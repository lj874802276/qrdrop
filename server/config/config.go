// Package config loads QRDrop runtime settings from environment variables,
// falling back to sensible defaults for self-hosting.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

// defaultExts is the MVP file type whitelist (training materials + common docs).
const defaultExts = "pptx,ppt,pdf,doc,docx,xls,xlsx,txt,md,csv,png,jpg,jpeg,gif,webp,bmp,svg,zip,rar,7z,mp4,mov,mp3"

// SaveDir preference values understood by ResolveSaveDir.
const (
	SavePrefDefault = "default"
	SavePrefDesktop = "desktop"
)

// Config holds all runtime configuration.
type Config struct {
	Addr          string
	BaseURL       string
	DataDir       string
	DBPath        string
	SessionTTL    time.Duration
	MaxUploadSize int64

	// SaveDirPref is the user's chosen file save location: "desktop",
	// "default" (DataDir/received), or an absolute/custom path.
	SaveDirPref string

	allowedExts map[string]bool
}

// Load builds a Config from the environment.
// Precedence: explicit env var > settings.json (runtime user setting) > default.
func Load() *Config {
	dataDir := env("QRDROP_DATA_DIR", "data")
	cfg := &Config{
		Addr:          env("QRDROP_ADDR", ":8080"),
		BaseURL:       strings.TrimRight(env("QRDROP_BASE_URL", ""), "/"),
		DataDir:       dataDir,
		DBPath:        filepath.Join(dataDir, "qrdrop.db"),
		SessionTTL:    envDuration("QRDROP_SESSION_TTL", 30*time.Minute),
		MaxUploadSize: envInt64("QRDROP_MAX_UPLOAD_MB", 1024) * 1024 * 1024,
		allowedExts:   parseExts(env("QRDROP_ALLOWED_EXTS", defaultExts)),
		SaveDirPref:   SavePrefDefault,
	}

	// Runtime user setting (settings.json) overrides the built-in default.
	if s, err := loadSettings(filepath.Join(dataDir, "settings.json")); err == nil && s.SaveDirPref != "" {
		cfg.SaveDirPref = s.SaveDirPref
	}
	// An explicit environment variable is the strongest override (ops-level).
	if v := strings.TrimSpace(os.Getenv("QRDROP_SAVE_DIR")); v != "" {
		cfg.SaveDirPref = v
	}
	return cfg
}

// SettingsPath is where the user-editable settings are persisted.
func (c *Config) SettingsPath() string {
	return filepath.Join(c.DataDir, "settings.json")
}

// ResolveSaveDir turns a save-location preference into a concrete, validated,
// existing directory. "desktop" uses the OS Desktop folder, "default" uses
// <DataDir>/received, and anything else is treated as a custom path. It creates
// missing directories and verifies write permission; an error means the caller
// must not adopt the preference.
func (c *Config) ResolveSaveDir(pref string) (string, error) {
	if pref == "" {
		pref = SavePrefDefault
	}
	var dir string
	switch pref {
	case SavePrefDefault:
		dir = filepath.Join(c.DataDir, "received")
	case SavePrefDesktop:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		dir = filepath.Join(home, "Desktop")
	default:
		dir = filepath.Clean(pref)
	}
	if !filepath.IsAbs(dir) {
		// Tolerate a relative custom path by anchoring it under DataDir.
		dir = filepath.Join(c.DataDir, dir)
	}
	if err := ValidateDir(dir); err != nil {
		return "", err
	}
	return dir, nil
}

// ValidateDir ensures dir exists, is a directory and is writable. It creates the
// directory (and parents) when missing. A descriptive error is returned on
// failure so the UI can surface a precise reason and the caller can roll back.
func ValidateDir(dir string) error {
	clean := filepath.Clean(dir)
	if clean == "" || clean == "." || clean == string(filepath.Separator) {
		return fmt.Errorf("invalid path: %q", dir)
	}
	if runtime.GOOS == "windows" && isWindowsDeviceName(clean) {
		return fmt.Errorf("invalid path %q: reserved device name", dir)
	}

	info, err := os.Stat(clean)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("cannot access path %q: %w", dir, err)
		}
		if mkErr := os.MkdirAll(clean, 0o755); mkErr != nil {
			return fmt.Errorf("cannot create path %q: %w", dir, mkErr)
		}
		info, err = os.Stat(clean)
		if err != nil {
			return fmt.Errorf("cannot stat created path %q: %w", dir, err)
		}
	}
	if !info.IsDir() {
		return fmt.Errorf("path %q exists but is not a directory", dir)
	}

	// Probe write permission with a throwaway file.
	tmp := filepath.Join(clean, ".qrdrop-write-test-"+strconv.FormatInt(time.Now().UnixNano(), 36))
	if wErr := os.WriteFile(tmp, []byte("ok"), 0o600); wErr != nil {
		return fmt.Errorf("path %q is not writable: %w", dir, wErr)
	}
	_ = os.Remove(tmp)
	return nil
}

// isWindowsDeviceName reports whether base is a reserved DOS device name.
func isWindowsDeviceName(p string) bool {
	base := strings.ToUpper(filepath.Base(p))
	for _, dev := range []string{
		"CON", "PRN", "AUX", "NUL",
		"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
		"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
	} {
		if base == dev {
			return true
		}
	}
	return false
}

// Allowed reports whether a file name matches the extension whitelist.
func (c *Config) Allowed(name string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	if ext == "" {
		return false
	}
	return c.allowedExts[ext]
}

// AllowedExts returns the whitelist as a sorted slice (for the UI).
func (c *Config) AllowedExts() []string {
	exts := make([]string, 0, len(c.allowedExts))
	for ext := range c.allowedExts {
		exts = append(exts, ext)
	}
	sort.Strings(exts)
	return exts
}

// MaxUploadMB is the per-file size limit in whole megabytes.
func (c *Config) MaxUploadMB() int64 { return c.MaxUploadSize / (1024 * 1024) }

func parseExts(raw string) map[string]bool {
	set := make(map[string]bool)
	for _, part := range strings.Split(raw, ",") {
		ext := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(part, ".")))
		if ext != "" {
			set[ext] = true
		}
	}
	return set
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt64(key string, fallback int64) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(os.Getenv(key)), 10, 64)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v, err := time.ParseDuration(strings.TrimSpace(os.Getenv(key)))
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
