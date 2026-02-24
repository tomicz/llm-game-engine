package scene

import rl "github.com/gen2brain/raylib-go/raylib"

// Scene holds 3D content (plane, cube) and a camera. Draw renders it.
// The plane is created on first Draw() so raylib's OpenGL context is already active.
type Scene struct {
	camera rl.Camera3D
	plane  rl.Model
	ready  bool
}

// New returns a scene that will create its plane on first Draw (after window is open).
func New() *Scene {
	return &Scene{}
}

// Draw renders the 3D scene. Call after ClearBackground, before 2D overlay (e.g. terminal).
func (s *Scene) Draw() {
	if !s.ready {
		mesh := rl.GenMeshPlane(20, 20, 1, 1)
		s.plane = rl.LoadModelFromMesh(mesh)
		rl.UnloadMesh(&mesh)
		if s.plane.Materials != nil && s.plane.Materials.Maps != nil {
			s.plane.Materials.Maps.Color = rl.LightGray
		}
		s.camera.Position = rl.NewVector3(0, 12, 0)
		s.camera.Target = rl.NewVector3(0, 0, 0)
		s.camera.Up = rl.NewVector3(0, 0, -1)
		s.camera.Fovy = 60
		s.camera.Projection = rl.CameraPerspective
		s.ready = true
	}
	rl.BeginMode3D(s.camera)
	rl.DrawModelEx(s.plane, rl.NewVector3(0, 0, 0), rl.NewVector3(1, 0, 0), 0, rl.NewVector3(1, -1, 1), rl.White)
	rl.DrawGrid(20, 1)
	rl.DrawCube(rl.NewVector3(2, 0.75, 0), 1.5, 1.5, 1.5, rl.Maroon)
	rl.EndMode3D()
}
