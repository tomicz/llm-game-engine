package scene

import rl "github.com/gen2brain/raylib-go/raylib"

// yDragSensitivity is world units of vertical movement per pixel of mouse movement.
const yDragSensitivity = float32(0.015)

type dragMode int

const (
	dragNone dragMode = iota
	dragXZ            // grabbed by the top or bottom face: move on the horizontal plane
	dragY             // grabbed by a side face: move up/down with the mouse
)

// editor is the state of an in-progress drag of the selected object.
type editor struct {
	mode dragMode
	// XZ drag: offset from the object center to the grab point, so that point stays under the cursor.
	offsetX, offsetZ float32
	// Y drag: object height and mouse Y when the drag started.
	startY      float32
	startMouseY int32
}

// UpdateEditor handles selection and dragging while the terminal is open. Mouse input at or
// below inputTopY (the terminal bar) is ignored. Clicking an object selects it; which face of its
// box was clicked picks the drag: top/bottom moves it on the XZ plane, sides move it up and down.
// Clicking empty space clears the selection.
func (s *Scene) UpdateEditor(inputTopY int32) {
	mouseY := rl.GetMouseY()
	if mouseY >= inputTopY || rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		s.editor.mode = dragNone
		return
	}
	ray := rl.GetScreenToWorldRay(rl.GetMousePosition(), s.Camera)

	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		o, hit, ok := s.raycast(ray)
		if !ok {
			s.selected = 0
			s.editor.mode = dragNone
			return
		}
		s.selected = o.ID
		// Top/bottom faces have a clearly vertical normal; the four sides have Y ≈ 0.
		if hit.Normal.Y > 0.99 || hit.Normal.Y < -0.99 {
			s.editor = editor{mode: dragXZ, offsetX: hit.Point.X - o.Position[0], offsetZ: hit.Point.Z - o.Position[2]}
		} else {
			s.editor = editor{mode: dragY, startY: o.Position[1], startMouseY: mouseY}
		}
		return
	}

	o, ok := s.Selected()
	if !ok {
		s.editor.mode = dragNone
		return
	}
	switch s.editor.mode {
	case dragY:
		o.Position[1] = s.editor.startY - float32(mouseY-s.editor.startMouseY)*yDragSensitivity
	case dragXZ:
		if p, ok := rayPlaneY(ray, o.Position[1]); ok {
			o.Position[0] = p.X - s.editor.offsetX
			o.Position[2] = p.Z - s.editor.offsetZ
		}
	}
}

// rayPlaneY returns where ray crosses the horizontal plane Y = y, if in front of the ray.
func rayPlaneY(ray rl.Ray, y float32) (rl.Vector3, bool) {
	dy := ray.Direction.Y
	if dy > -1e-6 && dy < 1e-6 {
		return rl.Vector3{}, false
	}
	t := (y - ray.Position.Y) / dy
	if t < 0 {
		return rl.Vector3{}, false
	}
	return rl.Vector3Add(ray.Position, rl.Vector3Scale(ray.Direction, t)), true
}
