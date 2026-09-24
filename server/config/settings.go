package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings is the user-editable runtime configuration persisted to disk. It is
// deliberately small: today it holds only the file save-location preference.
type Settings struct {
	SaveDirPref string `json:"save_dir"`
}

// DefaultSettings returns the built-in default settings.
func DefaultSettings() *Settings {
	return &Settings{SaveDirPref: SavePrefDefault}
}

// loadSettings reads settings.json from path. A missing file is not an error; it
// simply means the built-in defaults apply.
func loadSettings(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultSettings(), nil
		}
		return nil, err
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if s.SaveDirPref == "" {
		s.SaveDirPref = SavePrefDefault
	}
	return &s, nil
}

// SaveSettings writes settings to path, creating parent directories as needed.
func SaveSettings(path string, s *Settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
