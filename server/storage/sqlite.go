// Package storage persists sessions and file metadata in a local SQLite file.
// It uses the pure-Go modernc.org/sqlite driver so QRDrop builds without CGO.
package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver

	"github.com/yourname/qrdrop/server/model"
)

// ErrNotFound is returned when a session or file does not exist.
var ErrNotFound = errors.New("not found")

const schema = `
CREATE TABLE IF NOT EXISTS sessions (
	token      TEXT PRIMARY KEY,
	status     TEXT    NOT NULL DEFAULT 'open',
	save_dir   TEXT    NOT NULL DEFAULT 'default',
	created_at INTEGER NOT NULL,
	expires_at INTEGER NOT NULL,
	closed_at  INTEGER
);

CREATE TABLE IF NOT EXISTS files (
	id            INTEGER PRIMARY KEY AUTOINCREMENT,
	session_token TEXT    NOT NULL REFERENCES sessions(token) ON DELETE CASCADE,
	name          TEXT    NOT NULL,
	size          INTEGER NOT NULL,
	path          TEXT    NOT NULL,
	uploaded_at   INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_files_session   ON files(session_token);
CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions(expires_at);
`

// Store wraps the SQLite database.
type Store struct {
	db *sql.DB
}

// Open connects to the database at path, applies pragmas and creates the schema.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite allows a single writer; one connection avoids SQLITE_BUSY entirely.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	for _, pragma := range []string{
		// DELETE journal (not WAL) avoids the -shm shared-memory lock that some
		// sandboxed/container filesystems do not implement, which would otherwise
		// deadlock every database call after the first write. With a single
		// connection this is just as safe and far more portable.
		"PRAGMA journal_mode = DELETE",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA foreign_keys = ON",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", pragma, err)
		}
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// CreateSession inserts a new inbox.
func (s *Store) CreateSession(sess *model.Session) error {
	_, err := s.db.Exec(
		`INSERT INTO sessions (token, status, save_dir, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`,
		sess.Token, string(sess.Status), sess.SaveDir, millis(sess.CreatedAt), millis(sess.ExpiresAt),
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

// GetSession loads an inbox together with its received files.
func (s *Store) GetSession(token string) (*model.Session, error) {
	var (
		status           string
		saveDir          string
		createdAt, expiry int64
		closedAt         sql.NullInt64
	)
	err := s.db.QueryRow(
		`SELECT status, save_dir, created_at, expires_at, closed_at FROM sessions WHERE token = ?`, token,
	).Scan(&status, &saveDir, &createdAt, &expiry, &closedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select session: %w", err)
	}

	files, err := s.ListFiles(token)
	if err != nil {
		return nil, err
	}

	sess := &model.Session{
		Token:     token,
		Status:    model.SessionStatus(status),
		SaveDir:   saveDir,
		CreatedAt: fromMillis(createdAt),
		ExpiresAt: fromMillis(expiry),
		Files:     files,
	}
	if closedAt.Valid {
		t := fromMillis(closedAt.Int64)
		sess.ClosedAt = &t
	}
	return sess, nil
}

// CloseSession marks an inbox as closed (recording when) so uploads stop. Files
// are intentionally preserved; cleanup never deletes them.
func (s *Store) CloseSession(token string) error {
	res, err := s.db.Exec(
		`UPDATE sessions SET status = ?, closed_at = ? WHERE token = ?`,
		string(model.StatusClosed), millis(time.Now()), token,
	)
	if err != nil {
		return fmt.Errorf("close session: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("close session: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// AddFile records an uploaded file.
func (s *Store) AddFile(token, name, relPath string, size int64, uploadedAt time.Time) (*model.FileMeta, error) {
	res, err := s.db.Exec(
		`INSERT INTO files (session_token, name, size, path, uploaded_at) VALUES (?, ?, ?, ?, ?)`,
		token, name, size, relPath, millis(uploadedAt),
	)
	if err != nil {
		return nil, fmt.Errorf("insert file: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("insert file: %w", err)
	}
	return &model.FileMeta{ID: id, Name: name, Size: size, Path: relPath, UploadedAt: uploadedAt}, nil
}

// ListFiles returns the files of an inbox in upload order.
func (s *Store) ListFiles(token string) ([]model.FileMeta, error) {
	rows, err := s.db.Query(
		`SELECT id, name, size, path, uploaded_at FROM files WHERE session_token = ? ORDER BY uploaded_at, id`,
		token,
	)
	if err != nil {
		return nil, fmt.Errorf("select files: %w", err)
	}
	defer rows.Close()

	files := make([]model.FileMeta, 0, 8)
	for rows.Next() {
		var (
			meta       model.FileMeta
			uploadedAt int64
		)
		if err := rows.Scan(&meta.ID, &meta.Name, &meta.Size, &meta.Path, &uploadedAt); err != nil {
			return nil, fmt.Errorf("scan file: %w", err)
		}
		meta.UploadedAt = fromMillis(uploadedAt)
		files = append(files, meta)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate files: %w", err)
	}
	return files, nil
}

// GetFile loads a single file scoped to its inbox.
func (s *Store) GetFile(token string, id int64) (*model.FileMeta, error) {
	var (
		meta       model.FileMeta
		uploadedAt int64
	)
	err := s.db.QueryRow(
		`SELECT id, name, size, path, uploaded_at FROM files WHERE session_token = ? AND id = ?`,
		token, id,
	).Scan(&meta.ID, &meta.Name, &meta.Size, &meta.Path, &uploadedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select file: %w", err)
	}
	meta.UploadedAt = fromMillis(uploadedAt)
	return &meta, nil
}

// ListHistory returns every inbox (open, closed or expired) with its files, most
// recent first. Received files are never purged, so this is the audit/trace view.
func (s *Store) ListHistory() ([]model.Session, error) {
	rows, err := s.db.Query(
		`SELECT token, status, save_dir, created_at, expires_at, closed_at FROM sessions ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("select history: %w", err)
	}

	// Buffer every session's metadata first and release the result set before
	// issuing any nested query. With a single SQLite connection a still-open
	// outer rows would block the inner ListFiles query and deadlock.
	type meta struct {
		token, status, saveDir string
		createdAt, expiry      int64
		closedAt               sql.NullInt64
	}
	metas := make([]meta, 0, 16)
	for rows.Next() {
		var m meta
		if err := rows.Scan(&m.token, &m.status, &m.saveDir, &m.createdAt, &m.expiry, &m.closedAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan history: %w", err)
		}
		metas = append(metas, m)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate history: %w", err)
	}
	rows.Close()

	sessions := make([]model.Session, 0, len(metas))
	for _, m := range metas {
		files, fErr := s.ListFiles(m.token)
		if fErr != nil {
			return nil, fErr
		}
		sess := model.Session{
			Token:     m.token,
			Status:    model.SessionStatus(m.status),
			SaveDir:   m.saveDir,
			CreatedAt: fromMillis(m.createdAt),
			ExpiresAt: fromMillis(m.expiry),
			Files:     files,
		}
		if m.closedAt.Valid {
			t := fromMillis(m.closedAt.Int64)
			sess.ClosedAt = &t
		}
		sessions = append(sessions, sess)
	}
	return sessions, nil
}

// ExpiredSessions lists inboxes that are expired or closed. It is retained for
// diagnostics only — cleanup no longer deletes them (files are user property).
func (s *Store) ExpiredSessions(now time.Time) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT token FROM sessions WHERE expires_at <= ? OR status = ?`,
		millis(now), string(model.StatusClosed),
	)
	if err != nil {
		return nil, fmt.Errorf("select expired: %w", err)
	}
	defer rows.Close()

	var tokens []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return nil, fmt.Errorf("scan expired: %w", err)
		}
		tokens = append(tokens, token)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expired: %w", err)
	}
	return tokens, nil
}

// DeleteSession removes an inbox and its file rows.
func (s *Store) DeleteSession(token string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin delete: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM files WHERE session_token = ?`, token); err != nil {
		return fmt.Errorf("delete files: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM sessions WHERE token = ?`, token); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return tx.Commit()
}

// Stats reports the current row counts (used by tests and diagnostics).
func (s *Store) Stats() (sessions int, files int, err error) {
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&sessions); err != nil {
		return 0, 0, err
	}
	if err = s.db.QueryRow(`SELECT COUNT(*) FROM files`).Scan(&files); err != nil {
		return 0, 0, err
	}
	return sessions, files, nil
}

func millis(t time.Time) int64      { return t.UnixMilli() }
func fromMillis(ms int64) time.Time { return time.UnixMilli(ms) }
