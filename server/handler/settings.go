package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/lj874802276/qrdrop/server/config"
)

// GetSettings (GET /api/settings) returns the current save-location preference
// and the concrete directory it resolves to, plus whether it is currently valid.
func (a *App) GetSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}
	pref := a.Cfg.SaveDirPref
	resolved, err := a.Cfg.ResolveSaveDir(pref)
	resp := map[string]any{
		"save_dir_pref": pref,
		"resolved_dir":  resolved,
		"valid":        err == nil,
	}
	if err != nil {
		resp["error"] = err.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}

// UpdateSettings (POST /api/settings) changes the file save-location preference.
// The new value is validated first (directory created if missing, write
// permission and path legality checked). On failure the previous setting is kept
// and a precise error is returned so the UI can roll back. On success it is
// persisted to settings.json and applies to every inbox created afterwards.
func (a *App) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var body struct {
		SaveDirPref string `json:"save_dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	newPref := strings.TrimSpace(body.SaveDirPref)
	if newPref == "" {
		writeError(w, http.StatusBadRequest, "empty save location")
		return
	}

	// Validate before adopting — never silently apply an unusable preference.
	resolved, vErr := a.Cfg.ResolveSaveDir(newPref)
	if vErr != nil {
		writeError(w, http.StatusBadRequest, "invalid path: "+vErr.Error())
		return
	}

	// Persist first; if that fails we do not change the running configuration.
	if err := config.SaveSettings(a.Cfg.SettingsPath(), &config.Settings{SaveDirPref: newPref}); err != nil {
		writeError(w, http.StatusInternalServerError, "cannot persist settings: "+err.Error())
		return
	}
	a.Cfg.SaveDirPref = newPref

	writeJSON(w, http.StatusOK, map[string]any{
		"save_dir_pref": newPref,
		"resolved_dir":  resolved,
		"valid":         true,
	})
}
