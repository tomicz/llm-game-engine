// Package engineconfig persists engine-only preferences (overlays, grid, AI provider, font) across
// runs. In-game save data is separate.
package engineconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// EngineConfigPath is the config file, relative to the working directory.
const EngineConfigPath = "config/engine.json"

// DefaultFont is the UI font, relative to assets/fonts/.
const DefaultFont = "Roboto/static/Roboto-Regular.ttf"

// EnginePrefs holds engine preferences.
type EnginePrefs struct {
	ShowFPS      bool   `json:"show_fps"`
	ShowMemAlloc bool   `json:"show_memalloc"`
	GridVisible  bool   `json:"grid_visible"`
	AIProvider   string `json:"ai_provider,omitempty"` // "ollama", "openai", "groq"; empty = detect from env
	AIModel      string `json:"ai_model,omitempty"`    // empty = the provider's default model
	Font         string `json:"font,omitempty"`        // path under assets/fonts/
}

// Default returns the preferences used when there is no config file: overlays off, grid on,
// provider and model chosen at startup, Roboto font.
func Default() EnginePrefs {
	return EnginePrefs{GridVisible: true, Font: DefaultFont}
}

// Load reads the config file over the defaults, so keys missing from the file keep their default
// values. A missing file is not an error; an unreadable or invalid one returns the defaults and
// the error.
func Load() (EnginePrefs, error) {
	p := Default()
	data, err := os.ReadFile(EngineConfigPath)
	if errors.Is(err, fs.ErrNotExist) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return Default(), fmt.Errorf("%s: %w", EngineConfigPath, err)
	}
	return p, nil
}

// Save writes the preferences, creating the config directory if needed.
func Save(p EnginePrefs) error {
	if err := os.MkdirAll(filepath.Dir(EngineConfigPath), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(EngineConfigPath, data, 0o644)
}
