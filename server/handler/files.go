package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lj874802276/qrdrop/server/service"
	"github.com/lj874802276/qrdrop/server/storage"
)

// contentTypeByExt pins the MIME types of whitelisted extensions so inline
// previews behave consistently regardless of the host OS mime database.
var contentTypeByExt = map[string]string{
	".pdf":  "application/pdf",
	".txt":  "text/plain; charset=utf-8",
	".md":   "text/plain; charset=utf-8",
	".csv":  "text/csv; charset=utf-8",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".bmp":  "image/bmp",
	".svg":  "image/svg+xml",
	".mp4":  "video/mp4",
	".mov":  "video/quicktime",
	".mp3":  "audio/mpeg",
	".zip":  "application/zip",
}

// History (GET /api/history) returns every inbox — open, closed or expired —
// together with its files. Received files are never purged, so this is the
// audit/trace view the host can browse, download and open from.
func (a *App) History(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	sessions, err := a.Store.ListHistory()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load history")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
}

// ListFiles (GET /api/files?session=…) returns the current state of an inbox.
// The host page uses it on load and as the polling fallback for WebSocket.
func (a *App) ListFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
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

	// Closed or expired inboxes still expose their history (files are retained),
	// so the host can browse, download and open previously received files.
	writeJSON(w, http.StatusOK, a.view(r, sess))
}

// DownloadFile (GET /api/download?session=…&id=…) streams a received file.
// Add inline=1 to preview in the browser instead of downloading.
func (a *App) DownloadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w)
		return
	}

	token := r.URL.Query().Get("session")
	if !service.ValidToken(token) {
		writeError(w, http.StatusBadRequest, "invalid session")
		return
	}

	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid file id")
		return
	}

	meta, err := a.Store.GetFile(token, id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot load file")
		return
	}

	sess, err := a.Store.GetSession(token)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot load session")
		return
	}
	saveRoot, err := a.Cfg.ResolveSaveDir(sess.SaveDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "save location unavailable: "+err.Error())
		return
	}

	fullPath := filepath.Join(saveRoot, filepath.FromSlash(meta.Path))
	if !a.withinAllowedDirs(fullPath, saveRoot) {
		writeError(w, http.StatusForbidden, "path outside storage")
		return
	}

	handle, err := os.Open(fullPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "file missing on disk")
		return
	}
	defer handle.Close()

	info, err := handle.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot stat file")
		return
	}

	disposition := "attachment"
	if r.URL.Query().Get("inline") == "1" {
		disposition = "inline"
	}

	w.Header().Set("Content-Type", contentTypeFor(meta.Name))
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	w.Header().Set("Content-Disposition", contentDisposition(disposition, meta.Name))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	if r.Method == http.MethodHead {
		return
	}
	if _, err := io.Copy(w, handle); err != nil {
		// The client aborted or the connection dropped; nothing useful to send.
		return
	}
}

// withinAllowedDirs is a defence-in-depth check that a resolved file path stays
// inside the inbox's save-location root even if the stored metadata were tampered
// with. root is the resolved save directory for the owning session.
func (a *App) withinAllowedDirs(path, root string) bool {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(rootAbs, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func contentTypeFor(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if mime, ok := contentTypeByExt[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}

// contentDisposition builds an RFC 6266 header that survives non-ASCII names
// (Chinese file names in particular) on every major browser.
func contentDisposition(disposition, name string) string {
	return fmt.Sprintf("%s; filename=%q; filename*=UTF-8''%s",
		disposition, asciiFallback(name), rfc5987(name))
}

// asciiFallback produces a plain-ASCII name for clients that ignore filename*.
func asciiFallback(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r < 0x20 || r > 0x7e || r == '"' || r == '\\' {
			b.WriteRune('_')
			continue
		}
		b.WriteRune(r)
	}
	if b.Len() == 0 {
		return "download"
	}
	return b.String()
}

// rfc5987 percent-encodes a UTF-8 value for the extended filename parameter.
func rfc5987(value string) string {
	const allowed = "!#$&+-.^_`|~"
	var b strings.Builder
	for _, c := range []byte(value) {
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			b.WriteByte(c)
		case strings.IndexByte(allowed, c) >= 0:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
