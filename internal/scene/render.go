package scene

import (
	"game-engine/internal/assets"
	"game-engine/internal/primitives"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Editor grid on the XZ plane: minor lines every gridMinorStep, major every gridMajorStep, out to
// ±gridExtent, plus X/Y/Z axis lines through the origin.
const (
	gridExtent     = 50
	gridMinorStep  = 1
	gridMajorStep  = 10
	gridMinorAlpha = 50
	gridMajorAlpha = 120
	axisLineAlpha  = 220
	// gizmoArrowLength is the length of the (visual-only) axis arrows on the selected object.
	gizmoArrowLength = float32(1.5)
)

var (
	gridMinorColor = rl.NewColor(128, 128, 128, gridMinorAlpha)
	gridMajorColor = rl.NewColor(160, 160, 160, gridMajorAlpha)
	axisXColor     = rl.NewColor(220, 80, 80, axisLineAlpha)
	axisYColor     = rl.NewColor(80, 220, 80, axisLineAlpha)
	axisZColor     = rl.NewColor(80, 80, 220, axisLineAlpha)
	gizmoXColor    = rl.NewColor(220, 80, 80, 255)
	gizmoYColor    = rl.NewColor(80, 220, 80, 255)
	gizmoZColor    = rl.NewColor(80, 80, 220, 255)
)

// Draw renders the 3D scene: skybox, terrain, objects, then the grid. In editor mode the selected
// object gets a yellow bounding box and axis arrows. Call between BeginDrawing and EndDrawing,
// before 2D overlays.
func (s *Scene) Draw(editorMode bool) {
	s.sky.ensureLoaded()
	rl.BeginMode3D(s.Camera)
	s.sky.draw(s.Camera.Position)

	cam := s.Camera.Position
	s.prims.SetView([3]float32{cam.X, cam.Y, cam.Z}, s.lightDir)
	now := rl.GetTime()
	for _, o := range s.objects {
		drawPos := o.drawPosition(now)
		if o.Type == primitives.Terrain {
			// The terrain mesh is built in world space around the origin; its object only carries
			// color, texture, and the selectable box.
			s.drawPrimitive(o, primitives.Terrain, [3]float32{}, [3]float32{1, 1, 1})
		} else {
			s.drawPrimitive(o, o.Type, drawPos, o.Scale)
		}
		if editorMode && o.ID == s.selected {
			rl.DrawBoundingBox(o.boundsAt(drawPos), rl.Yellow)
			drawGizmoArrows(drawPos)
		}
	}
	if s.GridVisible {
		drawEditorGrid()
	}
	rl.EndMode3D()
}

func (s *Scene) drawPrimitive(o *Object, shape string, pos, scale [3]float32) {
	if tex, ok := s.texture(o.Texture); ok {
		s.prims.DrawWithTexture(shape, pos, scale, tex, o.tint())
		return
	}
	s.prims.Draw(shape, pos, scale, o.tint())
}

// texture returns the GPU texture for an object's texture path, loading and caching it on first
// use. The path is tried as given and under assets/textures/.
func (s *Scene) texture(path string) (rl.Texture2D, bool) {
	if path == "" {
		return rl.Texture2D{}, false
	}
	if tex, ok := s.textures[path]; ok {
		return tex, rl.IsTextureValid(tex)
	}
	var tex rl.Texture2D
	if file, ok := assets.Find(path, "assets/textures/"+path); ok {
		tex = rl.LoadTexture(file)
	}
	// Cache failures too, so a missing file isn't retried every frame.
	s.textures[path] = tex
	return tex, rl.IsTextureValid(tex)
}

// drawGizmoArrows draws red (X), green (Y), blue (Z) arrows at pos. Visual only; no picking.
func drawGizmoArrows(pos [3]float32) {
	base := rl.NewVector3(pos[0], pos[1], pos[2])
	drawArrow(base, rl.NewVector3(1, 0, 0), rl.NewVector3(0, 0, 1), rl.NewVector3(0, 1, 0), gizmoXColor)
	drawArrow(base, rl.NewVector3(0, 1, 0), rl.NewVector3(0, 0, 1), rl.NewVector3(1, 0, 0), gizmoYColor)
	drawArrow(base, rl.NewVector3(0, 0, 1), rl.NewVector3(1, 0, 0), rl.NewVector3(0, 1, 0), gizmoZColor)
}

// drawArrow draws a line from base along dir with a four-line arrowhead spread along u and v.
func drawArrow(base, dir, u, v rl.Vector3, c rl.Color) {
	head := gizmoArrowLength * 0.2
	end := rl.Vector3Add(base, rl.Vector3Scale(dir, gizmoArrowLength))
	back := rl.Vector3Add(base, rl.Vector3Scale(dir, gizmoArrowLength-head))
	rl.DrawLine3D(base, end, c)
	for _, side := range []rl.Vector3{u, rl.Vector3Negate(u), v, rl.Vector3Negate(v)} {
		rl.DrawLine3D(end, rl.Vector3Add(back, rl.Vector3Scale(side, head)), c)
	}
}

// drawEditorGrid draws the XZ grid and the axis lines through the origin.
func drawEditorGrid() {
	const e = float32(gridExtent)
	for i := -gridExtent; i <= gridExtent; i += gridMinorStep {
		c := gridMinorColor
		if i%gridMajorStep == 0 {
			c = gridMajorColor
		}
		f := float32(i)
		rl.DrawLine3D(rl.NewVector3(f, 0, -e), rl.NewVector3(f, 0, e), c)
		rl.DrawLine3D(rl.NewVector3(-e, 0, f), rl.NewVector3(e, 0, f), c)
	}
	rl.DrawLine3D(rl.NewVector3(-e, 0, 0), rl.NewVector3(e, 0, 0), axisXColor)
	rl.DrawLine3D(rl.NewVector3(0, -e, 0), rl.NewVector3(0, e, 0), axisYColor)
	rl.DrawLine3D(rl.NewVector3(0, 0, -e), rl.NewVector3(0, 0, e), axisZColor)
}
