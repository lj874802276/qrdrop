package handler

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/lj874802276/qrdrop/server/service"
	"github.com/lj874802276/qrdrop/server/storage"
)

// wsUpgrader only accepts same-origin handshakes, which blocks cross-site
// pages from subscribing to someone else's inbox.
var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     sameOrigin,
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// Non-browser clients (curl, tests) send no Origin header.
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Host, r.Host)
}

// WebSocket (GET /ws?session=…) pushes realtime inbox updates to the host page.
func (a *App) WebSocket(w http.ResponseWriter, r *http.Request) {
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

	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade already wrote an HTTP error response.
		return
	}

	// Send the current state before the pumps start so a freshly loaded page
	// renders without waiting for the next upload.
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := conn.WriteJSON(map[string]any{
		"type":       "snapshot",
		"session":    token,
		"status":     string(sess.Status),
		"expires_at": sess.ExpiresAt.UnixMilli(),
		"files":      sess.Files,
	}); err != nil {
		_ = conn.Close()
		return
	}
	_ = conn.SetWriteDeadline(time.Time{})

	a.Hub.Serve(token, conn)
}
