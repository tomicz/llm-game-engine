// Package scene holds the 3D world: its objects, camera, selection, undo, physics, and rendering.
// All methods must be called on the main (render) thread.
package scene

import (
	"errors"
	"fmt"
	"log"
	"slices"

	"game-engine/internal/physics"
	"game-engine/internal/primitives"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// ErrNoSelection is returned by operations on the selected object when nothing is selected.
var ErrNoSelection = errors.New("no object selected (click an object with terminal open)")

// Scene holds a free-fly camera and the objects of the world. Update runs camera and physics in
// game mode; UpdateEditor runs selection and dragging when the terminal is open; Draw renders.
type Scene struct {
	Camera      rl.Camera3D
	GridVisible bool

	objects  []*Object
	nextID   ObjectID
	selected ObjectID // 0 = none
	path     string   // scene file loaded from; Save writes here

	prims          *primitives.Registry
	physics        *physics.World
	textures       map[string]rl.Texture2D
	sky            skybox
	lightDir       [3]float32
	undo           undoLog
	editor         editor
	awareness      *ViewAwareness
	cursorCaptured bool
}

// New returns a scene with a perspective camera looking at the origin and the default scene file
// loaded (see Load). GPU resources are created lazily on first Draw.
func New() *Scene {
	s := &Scene{
		GridVisible: true,
		prims:       primitives.NewRegistry(),
		physics:     physics.NewWorld(),
		textures:    make(map[string]rl.Texture2D),
		lightDir:    lightingProfiles["noon"],
	}
	// Slightly off from center so the initial view isn't perfectly symmetric.
	s.Camera.Position = rl.NewVector3(11, 10.5, 9.5)
	s.Camera.Target = rl.NewVector3(0, 0, 0)
	s.Camera.Up = rl.NewVector3(0, 1, 0)
	s.Camera.Fovy = 45
	s.Camera.Projection = rl.CameraPerspective
	if err := s.load(); err != nil {
		log.Printf("scene: %v", err)
	}
	s.sky.findDefault()
	return s
}

// Objects returns the scene's objects in draw order. The slice is a copy; the objects are not.
func (s *Scene) Objects() []*Object {
	return slices.Clone(s.objects)
}

// Object returns the object with the given ID.
func (s *Scene) Object(id ObjectID) (*Object, bool) {
	if i := s.indexOf(id); i >= 0 {
		return s.objects[i], true
	}
	return nil, false
}

func (s *Scene) indexOf(id ObjectID) int {
	return slices.IndexFunc(s.objects, func(o *Object) bool { return o.ID == id })
}

// Add appends objects to the scene and returns their IDs. Planes with a Y scale of 1 get the
// default slab thickness. The whole call is one undo step.
func (s *Scene) Add(objs ...Object) []ObjectID {
	ids := make([]ObjectID, 0, len(objs))
	s.Group(func() {
		for _, o := range objs {
			if o.Type == primitives.Plane && o.Scale[1] == 1 {
				o.Scale[1] = planeDefaultScaleY
			}
			id := s.insert(len(s.objects), &o)
			s.undo.record(undoOp{added: id})
			ids = append(ids, id)
		}
	})
	return ids
}

// insert places o at index i with a fresh ID and no physics body.
func (s *Scene) insert(i int, o *Object) ObjectID {
	s.nextID++
	o.ID = s.nextID
	o.body = nil
	s.objects = slices.Insert(s.objects, min(i, len(s.objects)), o)
	return o.ID
}

// Delete removes the objects with the given IDs and returns how many were removed. The whole call
// is one undo step.
func (s *Scene) Delete(ids ...ObjectID) int {
	n := 0
	s.Group(func() {
		for _, id := range ids {
			i := s.indexOf(id)
			if i < 0 {
				continue
			}
			s.undo.record(undoOp{removed: s.objects[i], index: i})
			s.removeAt(i)
			n++
		}
	})
	return n
}

func (s *Scene) removeAt(i int) {
	o := s.objects[i]
	if o.ID == s.selected {
		s.selected = 0
	}
	o.body = nil // a restored object starts at rest
	s.objects = slices.Delete(s.objects, i, i+1)
}

// Duplicate clones the object n times (capped at 20), each shifted by a further offset.
// Clones have no name. Returns the number created.
func (s *Scene) Duplicate(id ObjectID, n int, offset [3]float32) (int, error) {
	o, ok := s.Object(id)
	if !ok {
		return 0, fmt.Errorf("object %d not found", id)
	}
	n = min(n, 20)
	clones := make([]Object, 0, max(n, 0))
	for i := range n {
		c := *o
		c.Name = ""
		for axis := range 3 {
			c.Position[axis] += offset[axis] * float32(i+1)
		}
		clones = append(clones, c)
	}
	s.Add(clones...)
	return len(clones), nil
}

// Selected returns the selected object.
func (s *Scene) Selected() (*Object, bool) {
	if s.selected == 0 {
		return nil, false
	}
	return s.Object(s.selected)
}

// RequireSelected returns the selected object, or ErrNoSelection.
func (s *Scene) RequireSelected() (*Object, error) {
	if o, ok := s.Selected(); ok {
		return o, nil
	}
	return nil, ErrNoSelection
}

// Select makes the object with the given ID the selection.
func (s *Scene) Select(id ObjectID) {
	s.selected = id
}

// ClearSelection deselects any object.
func (s *Scene) ClearSelection() {
	s.selected = 0
}

// LookAt points the camera at the object without moving the camera.
func (s *Scene) LookAt(o *Object) {
	s.Camera.Target = rl.NewVector3(o.Position[0], o.Position[1], o.Position[2])
}

// lightingProfiles maps time-of-day names to the direction toward the light.
var lightingProfiles = map[string][3]float32{
	"noon":   {0.5, 1, 0.5},
	"sunset": {0.8, 0.3, 0.2},   // warm, low
	"night":  {-0.3, 0.5, -0.5}, // dim, blue-ish
}

// SetLighting sets the directional light from a profile: "noon", "sunset", or "night".
func (s *Scene) SetLighting(profile string) error {
	dir, ok := lightingProfiles[profile]
	if !ok {
		return fmt.Errorf("unknown lighting profile %q (use noon, sunset, or night)", profile)
	}
	s.lightDir = dir
	return nil
}

// SetGravity sets the physics gravity vector (e.g. [0, -9.8, 0]).
func (s *Scene) SetGravity(g [3]float32) {
	s.physics.SetGravity(g)
}

// SetGridVisible sets whether the editor grid is drawn.
func (s *Scene) SetGridVisible(visible bool) {
	s.GridVisible = visible
}

// Update runs once per frame in game mode (terminal closed): free-fly camera with the cursor
// captured, then physics, then view awareness.
func (s *Scene) Update() {
	if !s.cursorCaptured {
		rl.DisableCursor()
		s.cursorCaptured = true
	}
	rl.UpdateCamera(&s.Camera, rl.CameraFree)
	s.stepPhysics(rl.GetFrameTime())
	s.updateViewAwareness()
}

// stepPhysics mirrors objects into physics bodies, advances the world, and copies dynamic body
// positions back. Object fields are the source of truth, so edits (drag, scale, physics toggle)
// take effect on the next step.
func (s *Scene) stepPhysics(dt float32) {
	bodies := s.physics.Bodies[:0]
	for _, o := range s.objects {
		if o.body == nil {
			o.body = physics.NewBody(o.Position, o.bodySize(), 1, !o.PhysicsEnabled())
		}
		o.body.Position = o.Position
		o.body.Scale = o.bodySize()
		o.body.Static = !o.PhysicsEnabled()
		bodies = append(bodies, o.body)
	}
	clear(bodies[len(bodies):cap(bodies)]) // drop references to removed objects' bodies
	s.physics.Bodies = bodies
	s.physics.Advance(dt)
	for _, o := range s.objects {
		if !o.body.Static {
			o.Position = o.body.Position
		}
	}
}
