package handler

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yourname/qrdrop/server/model"
	"github.com/yourname/qrdrop/server/service"
	"github.com/yourname/qrdrop/server/storage"
)

// multipartOverhead covers MIME boundaries and part headers on top of the file
// payload so a file of exactly the configured limit still fits.
const multipartOverhead = 10 << 20

// UploadFile (POST /api/upload?session=…) accepts a multipart file upload.
// The body is streamed straight to disk; nothing is buffered in memory.
func (a *App) UploadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	token := r.URL.Query().Get("session")
	if !service.ValidToken(token) {
		writeError(w, http.StatusBadRequest, "invalid session")
		return
	}

	sess, err := a.Store.GetSession(token)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot load session")
		return
	}
	if sess.Status == model.StatusClosed {
		writeError(w, http.StatusGone, "session closed")
		return
	}
	if time.Now().After(sess.ExpiresAt) {
		writeError(w, http.StatusGone, "session expired")
		return
	}

	// Cap the request body before the multipart reader wraps it, otherwise the
	// limit would never be observed while streaming.
	r.Body = http.MaxBytesReader(w, r.Body, a.Cfg.MaxUploadSize+multipartOverhead)

	reader, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "expected multipart/form-data")
		return
	}

	saveRoot, err := a.Cfg.ResolveSaveDir(sess.SaveDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "save location unavailable: "+err.Error())
		return
	}

	dir := filepath.Join(saveRoot, token)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, "cannot prepare storage")
		return
	}

	saved := make([]model.FileMeta, 0, 1)
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if isTooLarge(err) {
				writeError(w, http.StatusRequestEntityTooLarge, "file exceeds size limit")
				return
			}
			writeError(w, http.StatusBadRequest, "malformed upload")
			return
		}

		if part.FormName() != "file" || part.FileName() == "" {
			_ = part.Close()
			continue
		}

		meta, status, err := a.storePart(token, dir, saveRoot, part)
		_ = part.Close()
		if err != nil {
			if isTooLarge(err) {
				writeError(w, http.StatusRequestEntityTooLarge, "file exceeds size limit")
				return
			}
			writeError(w, status, err.Error())
			return
		}
		saved = append(saved, *meta)

		// One file per request keeps the client flow and progress UI simple.
		break
	}

	if len(saved) == 0 {
		writeError(w, http.StatusBadRequest, "no file part found")
		return
	}

	for i := range saved {
		a.Hub.Broadcast(token, map[string]any{
			"type":    "file",
			"session": token,
			"file":    saved[i],
		})
	}

	writeJSON(w, http.StatusCreated, map[string]any{"ok": true, "files": saved})
}

// storePart writes a single multipart part to disk and records its metadata.
// relBase is the save-location root the stored path is recorded relative to.
func (a *App) storePart(token, dir, relBase string, part *multipart.Part) (*model.FileMeta, int, error) {
	name := SafeFileName(part.FileName())
	if !a.Cfg.Allowed(name) {
		return nil, http.StatusUnsupportedMediaType,
			fmt.Errorf("file type not allowed: %s", strings.TrimPrefix(filepath.Ext(name), "."))
	}

	handle, fullPath, err := createUnique(dir, name)
	if err != nil {
		return nil, http.StatusInternalServerError, errors.New("cannot create file")
	}

	written, copyErr := io.Copy(handle, part)
	closeErr := handle.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(fullPath)
		if isTooLarge(copyErr) {
			return nil, http.StatusRequestEntityTooLarge, errors.New("file exceeds size limit")
		}
		return nil, http.StatusInternalServerError, errors.New("cannot write file")
	}
	if written == 0 {
		_ = os.Remove(fullPath)
		return nil, http.StatusBadRequest, errors.New("empty file")
	}
	if written > a.Cfg.MaxUploadSize {
		_ = os.Remove(fullPath)
		return nil, http.StatusRequestEntityTooLarge, errors.New("file exceeds size limit")
	}

	relPath, err := filepath.Rel(relBase, fullPath)
	if err != nil {
		_ = os.Remove(fullPath)
		return nil, http.StatusInternalServerError, errors.New("cannot resolve storage path")
	}

	meta, err := a.Store.AddFile(token, name, filepath.ToSlash(relPath), written, time.Now())
	if err != nil {
		_ = os.Remove(fullPath)
		return nil, http.StatusInternalServerError, errors.New("cannot persist file metadata")
	}
	return meta, http.StatusCreated, nil
}

// SafeFileName strips any directory component and control characters so a
// hostile client cannot escape the upload directory.
func SafeFileName(raw string) string {
	cleaned := strings.ReplaceAll(raw, "\\", "/")
	cleaned = filepath.Base(cleaned)
	cleaned = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == '/' || r == '\\' {
			return -1
		}
		return r
	}, cleaned)
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" || cleaned == "." || cleaned == ".." {
		return "file"
	}
	return truncateName(cleaned, 180)
}

// truncateName caps the rune length while preserving the extension.
func truncateName(name string, limit int) string {
	runes := []rune(name)
	if len(runes) <= limit {
		return name
	}
	ext := filepath.Ext(name)
	extRunes := []rune(ext)
	keep := limit - len(extRunes)
	if keep < 1 {
		return string(runes[:limit])
	}
	return string(runes[:keep]) + ext
}

// createUnique opens path/name for writing, appending -1, -2, … on collision.
// O_EXCL makes the name reservation atomic, so concurrent uploads cannot clash.
func createUnique(dir, name string) (*os.File, string, error) {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)

	for i := 0; i < 1000; i++ {
		candidate := name
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d%s", stem, i, ext)
		}
		fullPath := filepath.Join(dir, candidate)

		handle, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			return handle, fullPath, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, "", err
		}
	}
	return nil, "", errors.New("too many files with the same name")
}

// isTooLarge detects the body-limit error raised by http.MaxBytesReader.
func isTooLarge(err error) bool {
	if err == nil {
		return false
	}
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return true
	}
	return strings.Contains(err.Error(), "http: request body too large")
}
