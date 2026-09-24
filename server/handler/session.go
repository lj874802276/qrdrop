package handler

import (
	"errors"
	"net/http"

	"github.com/yourname/qrdrop/server/model"
	"github.com/yourname/qrdrop/server/service"
	"github.com/yourname/qrdrop/server/storage"
)

// CreateSession (POST /api/session) opens a new inbox and persists it.
func (a *App) CreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	sess, err := service.NewSession(a.Cfg.SessionTTL, a.Cfg.SaveDirPref)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot generate session token")
		return
	}
	if err := a.Store.CreateSession(sess); err != nil {
		writeError(w, http.StatusInternalServerError, "cannot persist session")
		return
	}

	writeJSON(w, http.StatusCreated, a.view(r, sess))
}

// CloseSession (POST /api/session/close) stops an inbox and notifies its hosts.
func (a *App) CloseSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	token := r.URL.Query().Get("session")
	if token == "" {
		_ = r.ParseForm()
		token = r.FormValue("session")
	}
	if !service.ValidToken(token) {
		writeError(w, http.StatusBadRequest, "invalid session")
		return
	}

	if err := a.Store.CloseSession(token); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot close session")
		return
	}

	a.Hub.Broadcast(token, map[string]any{
		"type":    "status",
		"session": token,
		"status":  string(model.StatusClosed),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"token":  token,
		"status": string(model.StatusClosed),
	})
}
