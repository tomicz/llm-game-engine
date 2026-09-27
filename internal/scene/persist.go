package scene

import (
	"fmt"
	"os"
	"path/filepath"

	"game-engine/internal/assets"

	"gopkg.in/yaml.v3"
)

// defaultScenePath is the scene file loaded at startup, relative to an assets search root.
const defaultScenePath = "assets/scenes/default.yaml"

// sceneFile is the YAML layout of a scene file.
type sceneFile struct {
	Objects []*Object `yaml:"objects"`
}

// load reads the default scene file. A missing file leaves the scene empty.
func (s *Scene) load() error {
	path, ok := assets.Find(defaultScenePath)
	if !ok {
		return nil
	}
	s.path = path
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var f sceneFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	for _, o := range f.Objects {
		if o != nil {
			s.insert(len(s.objects), o)
		}
	}
	return nil
}

// Save writes the current scene (including runtime-spawned objects) to the file it was loaded
// from, or to the default path if none was loaded.
func (s *Scene) Save() error {
	path := s.path
	if path == "" {
		path = filepath.Clean(defaultScenePath)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(sceneFile{Objects: s.objects})
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Clear removes all objects, the terrain mesh, the selection and undo history, then saves the
// now-empty scene.
func (s *Scene) Clear() error {
	s.objects = nil
	s.selected = 0
	s.undo.last, s.undo.pending = nil, nil // keep depth: Clear may run inside a Group
	s.prims.ClearTerrain()
	return s.Save()
}
