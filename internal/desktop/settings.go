package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings are the program's own preferences, kept in the user data
// directory. Kept deliberately small: anything that belongs to a measurement
// or a vehicle lives elsewhere.
type Settings struct {
	Lang string `json:"lang,omitempty"`
}

// LoadSettings reads the settings file; a missing or damaged file gives the
// defaults rather than an error — preferences are never worth refusing to
// start over.
func LoadSettings(path string) Settings {
	var s Settings
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &s)
	}
	return s
}

// SaveSettings writes the settings file atomically.
func SaveSettings(path string, s Settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
