package primitives

// PrimitiveDef is the YAML definition for a default primitive (e.g. assets/primitives/cube.yaml).
// Used for default size/color; mesh generation is driven by Type in code for now.
type PrimitiveDef struct {
	Type  string     `yaml:"type"`
	Size  [3]float32 `yaml:"size,omitempty"`
	Color string     `yaml:"color,omitempty"`
}
