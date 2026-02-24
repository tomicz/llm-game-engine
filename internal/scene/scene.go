package scene

import rl "github.com/gen2brain/raylib-go/raylib"

const (
	gridExtent      = 50
	gridMinorStep   = 1
	gridMajorStep   = 10
	gridMinorAlpha  = 50
	gridMajorAlpha  = 120
	axisLineAlpha   = 220
)

// Scene holds a 3D camera and draws the 3D world. Update runs camera logic (e.g. free camera);
// Draw renders between BeginMode3D and EndMode3D. Based on raylib examples/core/core_3d_camera_free.
type Scene struct {
	Camera      rl.Camera3D
	cursorDone  bool
	GridVisible bool
}

// New returns a scene with a perspective camera looking at the origin.
// Camera: position (10,10,10), target (0,0,0), up (0,1,0), fovy 45°. Grid is visible by default.
func New() *Scene {
	s := &Scene{}
	s.Camera.Position = rl.NewVector3(10, 10, 10)
	s.Camera.Target = rl.NewVector3(0, 0, 0)
	s.Camera.Up = rl.NewVector3(0, 1, 0)
	s.Camera.Fovy = 45
	s.Camera.Projection = rl.CameraPerspective
	s.GridVisible = true
	return s
}

// SetGridVisible sets whether the editor grid is drawn.
func (s *Scene) SetGridVisible(visible bool) {
	s.GridVisible = visible
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
// Draws a Unity-style grid on the XZ plane (Y=0) when GridVisible is true.
func (s *Scene) Draw() {
	rl.BeginMode3D(s.Camera)
	if s.GridVisible {
		drawEditorGrid()
	}
	rl.EndMode3D()
}

// drawEditorGrid draws an infinite-style grid on the XZ plane with major/minor lines and axis lines.
func drawEditorGrid() {
	minor := rl.NewColor(128, 128, 128, gridMinorAlpha)
	major := rl.NewColor(160, 160, 160, gridMajorAlpha)
	axisX := rl.NewColor(220, 80, 80, axisLineAlpha)
	axisY := rl.NewColor(80, 220, 80, axisLineAlpha)
	axisZ := rl.NewColor(80, 80, 220, axisLineAlpha)

	// Grid lines on XZ plane (Y=0): lines along X (varying Z) and along Z (varying X)
	for x := -gridExtent; x <= gridExtent; x += gridMinorStep {
		c := major
		if x%gridMajorStep != 0 {
			c = minor
		}
		start := rl.NewVector3(float32(x), 0, float32(-gridExtent))
		end := rl.NewVector3(float32(x), 0, float32(gridExtent))
		rl.DrawLine3D(start, end, c)
	}
	for z := -gridExtent; z <= gridExtent; z += gridMinorStep {
		c := major
		if z%gridMajorStep != 0 {
			c = minor
		}
		start := rl.NewVector3(float32(-gridExtent), 0, float32(z))
		end := rl.NewVector3(float32(gridExtent), 0, float32(z))
		rl.DrawLine3D(start, end, c)
	}

	// Axis lines through origin (X=red, Y=green, Z=blue)
	rl.DrawLine3D(rl.NewVector3(float32(-gridExtent), 0, 0), rl.NewVector3(float32(gridExtent), 0, 0), axisX)
	rl.DrawLine3D(rl.NewVector3(0, float32(-gridExtent), 0), rl.NewVector3(0, float32(gridExtent), 0), axisY)
	rl.DrawLine3D(rl.NewVector3(0, 0, float32(-gridExtent)), rl.NewVector3(0, 0, float32(gridExtent)), axisZ)
}
