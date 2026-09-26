package handler

import (
	"errors"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/lj874802276/qrdrop/server/service"
	"github.com/lj874802276/qrdrop/server/storage"
)

// OpenFolder (POST /api/open-folder?session=&id=) reveals a received file (or the
// inbox's save directory when id is empty/"dir") in the system file manager on the
// machine running QRDrop — i.e. the host. Only meaningful for local/self-hosted
// use; on a remote server it fails safely.
func (a *App) OpenFolder(w http.ResponseWriter, r *http.Request) {
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

	saveRoot, err := a.Cfg.ResolveSaveDir(sess.SaveDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "save location unavailable: "+err.Error())
		return
	}

	idStr := r.URL.Query().Get("id")
	if idStr == "" || idStr == "dir" {
		// Open the inbox's save directory as a whole.
		if err := openInFileManager(saveRoot); err != nil {
			writeError(w, http.StatusInternalServerError, "cannot open folder: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": saveRoot})
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
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

	fullPath := filepath.Join(saveRoot, filepath.FromSlash(meta.Path))
	if !a.withinAllowedDirs(fullPath, saveRoot) {
		writeError(w, http.StatusForbidden, "path outside storage")
		return
	}

	if err := openInFileManager(fullPath); err != nil {
		writeError(w, http.StatusInternalServerError, "cannot open folder: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": fullPath})
}

// openInFileManager reveals path in the OS file manager.
func openInFileManager(path string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer.exe", "/select,"+path).Start()
	case "darwin":
		return exec.Command("open", "-R", path).Start()
	default:
		return exec.Command("xdg-open", filepath.Dir(path)).Start()
	}
}
