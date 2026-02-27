module game-engine

go 1.25.6

require (
	github.com/gen2brain/raylib-go/raylib v0.55.1
	github.com/tomicz/speak-to-agent v0.0.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/ebitengine/purego v0.10.0 // indirect
	golang.org/x/exp v0.0.0-20260218203240-3dfff04db8fa // indirect
	golang.org/x/sys v0.41.0 // indirect
)

replace github.com/tomicz/speak-to-agent => ./modules/voice-to-text
