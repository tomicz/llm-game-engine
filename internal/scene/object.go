package scene

import (
	"fmt"
	"math"

	"game-engine/internal/physics"
	"game-engine/internal/primitives"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// ObjectID identifies an object for the lifetime of the process. IDs are assigned when objects are
// loaded or added, are never reused, and are not saved to the scene file.
type ObjectID uint64

// Object is one primitive instance in the scene. Position is the primitive's center.
type Object struct {
	ID       ObjectID   `yaml:"-"`
	Type     string     `yaml:"type"`
	Position [3]float32 `yaml:"position"`
	// Scale is the size in world units per axis; zero components mean 1.
	Scale [3]float32 `yaml:"scale,omitempty"`
	// Physics: nil or true = falls and collides; false = static (no gravity, still blocks others).
	Physics *bool `yaml:"physics,omitempty"`
	// Texture is an optional image path (e.g. assets/textures/downloaded/foo.png) used as albedo.
	Texture string `yaml:"texture,omitempty"`
	// Color is an optional RGB tint (0–1); all zero means the default material color.
	Color [3]float32 `yaml:"color,omitempty"`
	// Name is an optional label used by name-based commands (e.g. delete name <name>).
	Name string `yaml:"name,omitempty"`
	// Motion is "bob" (oscillate on Y) or "" (static).
	Motion string `yaml:"motion,omitempty"`

	body *physics.Body // created on the first physics step
}

// planeDefaultScaleY is the thickness given to planes so they render and collide as a thin slab.
const planeDefaultScaleY = 0.1

// PhysicsEnabled reports whether the object falls and collides (the default).
func (o *Object) PhysicsEnabled() bool {
	return o.Physics == nil || *o.Physics
}

// SetPhysics turns falling/collision on or off.
func (o *Object) SetPhysics(enabled bool) {
	o.Physics = &enabled
}

// HasColor reports whether a tint is set.
func (o *Object) HasColor() bool {
	return o.Color != [3]float32{}
}

// Label is the object's name, or "#<id>" when it has none.
func (o *Object) Label() string {
	if o.Name != "" {
		return o.Name
	}
	return fmt.Sprintf("#%d", o.ID)
}

// tint returns the RGBA draw tint, or nil for the default material color.
func (o *Object) tint() *[4]float32 {
	if !o.HasColor() {
		return nil
	}
	return &[4]float32{o.Color[0], o.Color[1], o.Color[2], 1}
}

// size returns Scale with zero components replaced by 1.
func (o *Object) size() [3]float32 {
	s := o.Scale
	for i := range s {
		if s[i] == 0 {
			s[i] = 1
		}
	}
	return s
}

// bodySize is the physics collider size: planes are always a thin slab.
func (o *Object) bodySize() [3]float32 {
	s := o.size()
	if o.Type == primitives.Plane {
		s[1] = planeDefaultScaleY
	}
	return s
}

// drawPosition is where the object is drawn at time t (seconds), with motion applied.
func (o *Object) drawPosition(t float64) [3]float32 {
	pos := o.Position
	if o.Motion == "bob" {
		pos[1] += 0.2 * float32(math.Sin(t*2))
	}
	return pos
}

// boundsAt is the world-space AABB of the object centered at pos.
func (o *Object) boundsAt(pos [3]float32) rl.BoundingBox {
	s := o.size()
	return rl.NewBoundingBox(
		rl.NewVector3(pos[0]-s[0]/2, pos[1]-s[1]/2, pos[2]-s[2]/2),
		rl.NewVector3(pos[0]+s[0]/2, pos[1]+s[1]/2, pos[2]+s[2]/2),
	)
}

func (o *Object) bounds() rl.BoundingBox {
	return o.boundsAt(o.Position)
}

// isKnownType reports whether typ is a primitive type an object can have (including terrain).
func isKnownType(typ string) bool {
	return primitives.IsShape(typ) || typ == primitives.Terrain
}
