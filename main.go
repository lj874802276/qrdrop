// Command qrdrop serves the QRDrop file drop service: scan a QR code, upload
// a file, watch it appear on the host screen — no accounts, no apps.
package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/lj874802276/qrdrop/server/config"
	"github.com/lj874802276/qrdrop/server/handler"
	"github.com/lj874802276/qrdrop/server/netutil"
	"github.com/lj874802276/qrdrop/server/service"
	"github.com/lj874802276/qrdrop/server/storage"
)

//go:embed web
var embeddedWeb embed.FS

// version and commit are injected at build time via -ldflags (see .goreleaser.yaml).
var (
	version = "dev"
	commit  = "none"
)

func main() {
	cfg := config.Load()

	webFS, err := fs.Sub(embeddedWeb, "web")
	if err != nil {
		log.Fatalf("qrdrop: mount web assets: %v", err)
	}

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatalf("qrdrop: create data dir: %v", err)
	}

	store, err := storage.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("qrdrop: open database: %v", err)
	}
	defer store.Close()

	hub := service.NewHub()
	app := handler.New(cfg, store, hub, webFS)

	// Detect the LAN address so the QR code points somewhere a phone can reach,
	// and so we can open the right URL automatically.
	lanIP := netutil.PrimaryLanIP()
	if lanIP != "" {
		app.LanBaseURL = "http://" + lanIP + cfg.Addr
	}

	stopCleanup := make(chan struct{})
	go service.CleanupLoop(store, cfg, stopCleanup)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           withMiddleware(httpRoutes(app, webFS)),
		ReadHeaderTimeout: 15 * time.Second,
	}

	go func() {
		log.Printf("QRDrop ready → http://localhost%s  (data dir: %s)", cfg.Addr, cfg.DataDir)
		if lanIP != "" {
			log.Printf("LAN address   → %s  (open this on your phone)", app.LanBaseURL)
		}
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("qrdrop: serve: %v", err)
		}
	}()

	// Best-effort: open the LAN URL in the default browser so the user lands on
	// the right page immediately. Set QRDROP_NO_BROWSER=1 to disable (servers,
	// headless runs, or when you prefer to open it yourself).
	if lanIP != "" && os.Getenv("QRDROP_NO_BROWSER") == "" {
		if err := netutil.OpenBrowser(app.LanBaseURL); err != nil {
			log.Printf("qrdrop: auto-open browser skipped: %v", err)
		}
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	<-signals

	close(stopCleanup)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("qrdrop: shutdown: %v", err)
	}
	log.Println("QRDrop stopped")
}

// httpRoutes registers the API endpoints ahead of the static file server so
// that asset lookups never shadow an API path.
func httpRoutes(app *handler.App, webFS fs.FS) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/session", app.CreateSession)
	mux.HandleFunc("/api/session/close", app.CloseSession)
	mux.HandleFunc("/api/files", app.ListFiles)
	mux.HandleFunc("/api/history", app.History)
	mux.HandleFunc("/api/upload", app.UploadFile)
	mux.HandleFunc("/api/download", app.DownloadFile)
	mux.HandleFunc("/api/qrcode", app.QRCode)
	mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			app.UpdateSettings(w, r)
			return
		}
		app.GetSettings(w, r)
	})
	mux.HandleFunc("/api/open-folder", app.OpenFolder)
	mux.HandleFunc("/ws", app.WebSocket)

	// Clean URL for the page the QR code points at.
	mux.HandleFunc("/upload", app.Page("upload.html"))

	// Everything else is a static asset, with index.html at the root.
	mux.Handle("/", http.FileServer(http.FS(webFS)))

	return mux
}

// withMiddleware adds request logging, panic recovery and baseline headers.
func withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")

		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic serving %s %s: %v", r.Method, r.URL.Path, rec)
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)

		if isAPIPath(r.URL.Path) {
			log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
		}
	})
}

func isAPIPath(path string) bool {
	return path == "/api" || strings.HasPrefix(path, "/api/") || path == "/ws"
}
