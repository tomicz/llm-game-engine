package scene

import rl "github.com/gen2brain/raylib-go/raylib"

// Scene holds a 3D camera and draws the 3D world. Update runs camera logic (e.g. free camera);
// Draw renders between BeginMode3D and EndMode3D. Based on raylib examples/core/core_3d_camera_free.
type Scene struct {
	Camera     rl.Camera3D
	cursorDone bool
}

// New returns a scene with a perspective camera looking at the origin.
// Camera: position (10,10,10), target (0,0,0), up (0,1,0), fovy 45°.
func New() *Scene {
	s := &Scene{}
	s.Camera.Position = rl.NewVector3(10, 10, 10)
	s.Camera.Target = rl.NewVector3(0, 0, 0)
	s.Camera.Up = rl.NewVector3(0, 1, 0)
	s.Camera.Fovy = 45
	s.Camera.Projection = rl.CameraPerspective
	return s
}

// Update runs once per frame. Uses raylib UpdateCamera with CameraFree so the user can
// move the camera with mouse (zoom, pan) and keyboard. Cursor is disabled so the mouse
// is captured for camera control.
func (s *Scene) Update() {
	if !s.cursorDone {
		rl.DisableCursor()
		s.cursorDone = true
	}
	rl.UpdateCamera(&s.Camera, rl.CameraFree)
}

// Draw renders the 3D scene. Call after ClearBackground and before 2D overlay (e.g. terminal).
func (s *Scene) Draw() {
	rl.BeginMode3D(s.Camera)
	rl.DrawGrid(10, 1)
	rl.EndMode3D()
}
