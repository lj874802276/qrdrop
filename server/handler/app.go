package handler

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/yourname/qrdrop/server/config"
	"github.com/yourname/qrdrop/server/model"
	"github.com/yourname/qrdrop/server/service"
	"github.com/yourname/qrdrop/server/storage"
)

// App wires configuration, storage and the realtime hub into HTTP handlers.
type App struct {
	Cfg   *config.Config
	Store *storage.Store
	Hub   *service.Hub
	Web   fs.FS
}

// New builds the application.
func New(cfg *config.Config, store *storage.Store, hub *service.Hub, web fs.FS) *App {
	return &App{Cfg: cfg, Store: store, Hub: hub, Web: web}
}

// Page serves an embedded HTML page without caching.
func (a *App) Page(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			methodNotAllowed(w)
			return
		}
		data, err := fs.ReadFile(a.Web, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(data)
	}
}

// sessionView is the JSON payload shared by /api/session and /api/files.
type sessionView struct {
	Token       string           `json:"token"`
	Status      string           `json:"status"`
	ExpiresAt   int64            `json:"expires_at"`
	TTLSeconds  int64            `json:"ttl_seconds"`
	UploadURL   string           `json:"upload_url"`
	QRURL       string           `json:"qr_url"`
	SaveDir     string           `json:"save_dir"`
	MaxUploadMB int64            `json:"max_upload_mb"`
	AllowedExts []string         `json:"allowed_exts"`
	Files       []model.FileMeta `json:"files"`
}

func (a *App) view(r *http.Request, sess *model.Session) sessionView {
	base := a.baseURL(r)
	files := sess.Files
	if files == nil {
		files = []model.FileMeta{}
	}
	// The save location is the snapshotted preference resolved to its real path,
	// scoped to this inbox's token directory (files live at saveDir/{token}/...).
	saveLoc := filepath.Join(a.Cfg.DataDir, "received", sess.Token)
	if resolved, err := a.Cfg.ResolveSaveDir(sess.SaveDir); err == nil {
		saveLoc = filepath.Join(resolved, sess.Token)
	}

	return sessionView{
		Token:       sess.Token,
		Status:      string(sess.Status),
		ExpiresAt:   sess.ExpiresAt.UnixMilli(),
		TTLSeconds:  int64(a.Cfg.SessionTTL.Seconds()),
		UploadURL:   base + "/upload?session=" + sess.Token,
		QRURL:       base + "/api/qrcode?session=" + sess.Token,
		SaveDir:     saveLoc,
		MaxUploadMB: a.Cfg.MaxUploadMB(),
		AllowedExts: a.Cfg.AllowedExts(),
		Files:       files,
	}
}

// baseURL derives the externally reachable origin, honouring reverse proxies.
// Set QRDROP_BASE_URL to override it when the Host header is not trustworthy.
func (a *App) baseURL(r *http.Request) string {
	if a.Cfg.BaseURL != "" {
		return a.Cfg.BaseURL
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = firstValue(proto)
	}

	host := r.Host
	if forwarded := r.Header.Get("X-Forwarded-Host"); forwarded != "" {
		host = firstValue(forwarded)
	}

	return scheme + "://" + host
}

func firstValue(header string) string {
	if idx := strings.IndexByte(header, ','); idx >= 0 {
		header = header[:idx]
	}
	return strings.TrimSpace(header)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("handler: encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}
