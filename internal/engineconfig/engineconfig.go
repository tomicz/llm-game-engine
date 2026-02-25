package engineconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// EngineConfigPath is the path to the engine config file, relative to the process working directory.
const EngineConfigPath = "config/engine.json"

// EnginePrefs holds engine-only preferences (debug overlays, grid, AI model, etc.). Persisted across runs.
// In-game save data is separate and handled elsewhere.
type EnginePrefs struct {
	ShowFPS      bool   `json:"show_fps"`
	ShowMemAlloc bool   `json:"show_memalloc"`
	GridVisible  bool   `json:"grid_visible"`
	AIModel      string `json:"ai_model,omitempty"`
}

// Default returns default engine preferences (debug overlays off, grid on).
func Default() EnginePrefs {
	return EnginePrefs{
		ShowFPS:      false,
		ShowMemAlloc: false,
		GridVisible:  true,
		AIModel:      "gpt-4o-mini",
	}
}

// Load reads engine preferences from config/engine.json. If the file is missing or invalid,
// returns Default() and does not create a file.
func Load() (EnginePrefs, error) {
	data, err := os.ReadFile(EngineConfigPath)
	if err != nil {
		return Default(), nil
	}
	var p EnginePrefs
	if err := json.Unmarshal(data, &p); err != nil {
		return Default(), nil
	}
	return p, nil
}

// Save writes engine preferences to config/engine.json, creating the config directory if needed.
func Save(p EnginePrefs) error {
	dir := filepath.Dir(EngineConfigPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "\t")
	if err != nil {
		return err
	}
	return os.WriteFile(EngineConfigPath, data, 0644)
}
