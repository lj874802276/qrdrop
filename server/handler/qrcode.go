package handler

import (
	"errors"
	"net/http"
	"strconv"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/lj874802276/qrdrop/server/service"
	"github.com/lj874802276/qrdrop/server/storage"
)

const (
	// qrSize is the rendered PNG edge length in pixels — crisp on a projector
	// screen and still fast to encode.
	qrSize = 512
)

// QRCode (GET /api/qrcode?session=…) renders the upload page URL as a PNG.
// Generation is server-side so the client needs no QR library.
func (a *App) QRCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w)
		return
	}

	token := r.URL.Query().Get("session")
	if !service.ValidToken(token) {
		writeError(w, http.StatusBadRequest, "invalid session")
		return
	}

	if _, err := a.Store.GetSession(token); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "cannot load session")
		return
	}

	target := a.baseURL(r) + "/upload?session=" + token

	// High recovery tolerance keeps the code scannable on a dim screen or a
	// slightly angled phone camera.
	png, err := qrcode.Encode(target, qrcode.High, qrSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot generate qr code")
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(png)))
	// The payload embeds the request host, so it must never be cached.
	w.Header().Set("Cache-Control", "no-store")

	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(png)
}
