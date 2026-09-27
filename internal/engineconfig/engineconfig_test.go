package engineconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(EngineConfigPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(EngineConfigPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMissingFileGivesDefaults(t *testing.T) {
	t.Chdir(t.TempDir())
	p, err := Load()
	if err != nil || p != Default() {
		t.Errorf("Load = %+v, %v; want defaults", p, err)
	}
	if !p.GridVisible || p.Font != DefaultFont || p.AIModel != "" {
		t.Errorf("defaults = %+v", p)
	}
}

func TestLoadKeepsDefaultsForMissingKeys(t *testing.T) {
	t.Chdir(t.TempDir())
	writeConfig(t, `{"show_fps": true, "ai_provider": "groq"}`)
	p, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !p.ShowFPS || p.AIProvider != "groq" {
		t.Errorf("file values not applied: %+v", p)
	}
	if !p.GridVisible || p.Font != DefaultFont {
		t.Errorf("missing keys lost their defaults: %+v", p)
	}
}

func TestLoadExplicitFalseWins(t *testing.T) {
	t.Chdir(t.TempDir())
	writeConfig(t, `{"grid_visible": false}`)
	if p, _ := Load(); p.GridVisible {
		t.Error("grid_visible false in file was ignored")
	}
}

func TestLoadInvalidFile(t *testing.T) {
	t.Chdir(t.TempDir())
	writeConfig(t, `{not json`)
	p, err := Load()
	if err == nil {
		t.Error("invalid file: want error")
	}
	if p != Default() {
		t.Errorf("invalid file: got %+v, want defaults", p)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	t.Chdir(t.TempDir())
	want := EnginePrefs{ShowMemAlloc: true, AIProvider: "ollama", AIModel: "qwen3-coder:30b", Font: "Inter/Inter-Regular.ttf"}
	if err := Save(want); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(); err != nil || got != want {
		t.Errorf("round trip = %+v, %v; want %+v", got, err, want)
	}
}
