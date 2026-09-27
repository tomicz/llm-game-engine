package scene

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"slices"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// VisibleObject is a scene object currently in the camera's view.
type VisibleObject struct {
	Object       *Object
	Distance     float32    // from the camera position
	ScreenPos    rl.Vector2 // object center on screen
	DrawPosition [3]float32 // world position as drawn (with motion)
}

// ObjectsInView returns the objects in front of the camera whose center projects inside the
// screen, closest first.
func (s *Scene) ObjectsInView() []VisibleObject {
	if len(s.objects) == 0 {
		return nil
	}
	camPos := s.Camera.Position
	forward := rl.Vector3Normalize(rl.Vector3Subtract(s.Camera.Target, camPos))
	w, h := float32(rl.GetScreenWidth()), float32(rl.GetScreenHeight())
	now := rl.GetTime()
	const inFrontEpsilon = 0.01

	var out []VisibleObject
	for _, o := range s.objects {
		drawPos := o.drawPosition(now)
		center := rl.NewVector3(drawPos[0], drawPos[1], drawPos[2])
		toCenter := rl.Vector3Subtract(center, camPos)
		dist := rl.Vector3Length(toCenter)
		if dist < 1e-6 {
			continue
		}
		if rl.Vector3DotProduct(rl.Vector3Scale(toCenter, 1/dist), forward) < inFrontEpsilon {
			continue // behind the camera or outside the view cone
		}
		screen := rl.GetWorldToScreen(center, s.Camera)
		if screen.X < 0 || screen.X > w || screen.Y < 0 || screen.Y > h {
			continue
		}
		out = append(out, VisibleObject{Object: o, Distance: dist, ScreenPos: screen, DrawPosition: drawPos})
	}
	slices.SortFunc(out, func(a, b VisibleObject) int { return cmp.Compare(a.Distance, b.Distance) })
	return out
}

// Screen positions accepted by Query.Position.
var positionWords = []string{"left", "right", "top", "bottom", "closest", "farthest"}

// ColorNames maps color words accepted in queries to RGB (0–1).
var ColorNames = map[string][3]float32{
	"red": {1, 0, 0}, "green": {0, 1, 0}, "blue": {0, 0, 1}, "yellow": {1, 1, 0},
	"orange": {1, 0.5, 0}, "purple": {0.5, 0, 0.5}, "pink": {1, 0.75, 0.8},
	"white": {1, 1, 1}, "black": {0, 0, 0}, "gray": {0.5, 0.5, 0.5}, "grey": {0.5, 0.5, 0.5},
}

// Query describes visible objects by type, color, name, and screen position. Empty fields match
// anything; an empty Position picks the closest match.
type Query struct {
	Type     string      // primitive type, e.g. "cube"
	Color    *[3]float32 // approximate RGB; objects without a color never match
	Name     string      // case-insensitive name substring
	Position string      // left, right, top, bottom, closest, or farthest
}

// ParseQuery parses command arguments of the forms:
//
//	<position> | <type> | <name>
//	<type> <position> | <color> <type> | <name> <position>
//	<color> <type> <position>
func ParseQuery(args []string) (Query, bool) {
	a := make([]string, len(args))
	for i, s := range args {
		a[i] = strings.ToLower(s)
	}
	isPos := func(s string) bool { return slices.Contains(positionWords, s) }
	color := func(s string) (*[3]float32, bool) {
		c, ok := ColorNames[s]
		return &c, ok
	}
	switch len(a) {
	case 1:
		switch {
		case isPos(a[0]):
			return Query{Position: a[0]}, true
		case isKnownType(a[0]):
			return Query{Type: a[0]}, true
		default:
			return Query{Name: a[0]}, true
		}
	case 2:
		if isKnownType(a[0]) && isPos(a[1]) {
			return Query{Type: a[0], Position: a[1]}, true
		}
		if c, ok := color(a[0]); ok && isKnownType(a[1]) {
			return Query{Type: a[1], Color: c}, true
		}
		if isPos(a[1]) {
			return Query{Name: a[0], Position: a[1]}, true
		}
	case 3:
		if c, ok := color(a[0]); ok && isKnownType(a[1]) && isPos(a[2]) {
			return Query{Type: a[1], Color: c, Position: a[2]}, true
		}
	}
	return Query{}, false
}

// matches reports whether o satisfies the type, color, and name filters of q.
func (q Query) matches(o *Object) bool {
	if q.Type != "" && o.Type != q.Type {
		return false
	}
	if q.Color != nil {
		if !o.HasColor() {
			return false
		}
		const tolerance = 0.35
		for i := range 3 {
			if math.Abs(float64(o.Color[i]-q.Color[i])) > tolerance {
				return false
			}
		}
	}
	return q.Name == "" || strings.Contains(strings.ToLower(o.Name), strings.ToLower(q.Name))
}

// notFound describes a query that matched nothing.
func (q Query) notFound() error {
	switch {
	case q.Type != "" && q.Name != "":
		return fmt.Errorf("no %s matching %q in view", q.Type, q.Name)
	case q.Type != "" && q.Color != nil:
		return fmt.Errorf("no %s with that color in view (look at the object and try again)", q.Type)
	case q.Type != "":
		return fmt.Errorf("no %s in view (look at the object and try again)", q.Type)
	case q.Name != "":
		return fmt.Errorf("no objects matching %q in view", q.Name)
	default:
		return errors.New("no objects in view")
	}
}

// filter returns the visible objects matching q's type, color, and name, in input order.
func (q Query) filter(visible []VisibleObject) []VisibleObject {
	if q.Type != "" && !isKnownType(q.Type) {
		return nil
	}
	var out []VisibleObject
	for _, v := range visible {
		if q.matches(v.Object) {
			out = append(out, v)
		}
	}
	return out
}

// pick returns the one visible object at q.Position (closest when empty).
func (q Query) pick(visible []VisibleObject) (VisibleObject, bool) {
	var cmpFn func(a, b VisibleObject) int
	switch q.Position {
	case "left":
		cmpFn = func(a, b VisibleObject) int { return cmp.Compare(a.ScreenPos.X, b.ScreenPos.X) }
	case "right":
		cmpFn = func(a, b VisibleObject) int { return cmp.Compare(b.ScreenPos.X, a.ScreenPos.X) }
	case "top":
		cmpFn = func(a, b VisibleObject) int { return cmp.Compare(a.ScreenPos.Y, b.ScreenPos.Y) }
	case "bottom":
		cmpFn = func(a, b VisibleObject) int { return cmp.Compare(b.ScreenPos.Y, a.ScreenPos.Y) }
	case "", "closest":
		cmpFn = func(a, b VisibleObject) int { return cmp.Compare(a.Distance, b.Distance) }
	case "farthest":
		cmpFn = func(a, b VisibleObject) int { return cmp.Compare(b.Distance, a.Distance) }
	default:
		return VisibleObject{}, false
	}
	if len(visible) == 0 {
		return VisibleObject{}, false
	}
	return slices.MinFunc(visible, cmpFn), true
}

// FindVisible returns the visible object matching q.
func (s *Scene) FindVisible(q Query) (*Object, error) {
	v, ok := q.pick(q.filter(s.ObjectsInView()))
	if !ok {
		return nil, q.notFound()
	}
	return v.Object, nil
}

// FindAllVisible returns every visible object matching q's type, color, and name (Position is
// ignored), closest first.
func (s *Scene) FindAllVisible(q Query) []*Object {
	var out []*Object
	for _, v := range q.filter(s.ObjectsInView()) {
		out = append(out, v.Object)
	}
	return out
}

// ObjectAtCameraCenter returns the first object hit by a ray from the camera through its target.
func (s *Scene) ObjectAtCameraCenter() (*Object, error) {
	if len(s.objects) == 0 {
		return nil, errors.New("no objects in scene")
	}
	dir := rl.Vector3Normalize(rl.Vector3Subtract(s.Camera.Target, s.Camera.Position))
	o, _, ok := s.raycast(rl.Ray{Position: s.Camera.Position, Direction: dir})
	if !ok {
		return nil, errors.New("no object in view (camera not looking at any object)")
	}
	return o, nil
}

// raycast returns the closest object whose AABB the ray hits, and the hit.
func (s *Scene) raycast(ray rl.Ray) (*Object, rl.RayCollision, bool) {
	var best *Object
	var bestHit rl.RayCollision
	for _, o := range s.objects {
		hit := rl.GetRayCollisionBox(ray, o.bounds())
		if hit.Hit && hit.Distance > 0 && (best == nil || hit.Distance < bestHit.Distance) {
			best, bestHit = o, hit
		}
	}
	return best, bestHit, best != nil
}

// FindByName returns the first object whose name is exactly name.
func (s *Scene) FindByName(name string) (*Object, error) {
	if name == "" {
		return nil, errors.New("name is required")
	}
	if i := slices.IndexFunc(s.objects, func(o *Object) bool { return o.Name == name }); i >= 0 {
		return s.objects[i], nil
	}
	return nil, fmt.Errorf("no object named %q", name)
}

// Random returns a random object.
func (s *Scene) Random() (*Object, error) {
	if len(s.objects) == 0 {
		return nil, errors.New("no objects in scene")
	}
	return s.objects[rand.IntN(len(s.objects))], nil
}

// ViewSummary describes what the camera sees for the AI agent, e.g.
// `Visible (left to right): 1. "Tower" (cube) (left), 2. plane (center), 3. sphere (right).`
func (s *Scene) ViewSummary() string {
	visible := s.ObjectsInView()
	if len(visible) == 0 {
		return "No objects in view."
	}
	slices.SortFunc(visible, func(a, b VisibleObject) int { return cmp.Compare(a.ScreenPos.X, b.ScreenPos.X) })
	minX, maxX := visible[0].ScreenPos.X, visible[len(visible)-1].ScreenPos.X
	midX := (minX + maxX) / 2
	parts := make([]string, len(visible))
	for i, v := range visible {
		side := "center"
		if maxX > minX {
			if v.ScreenPos.X < midX-20 {
				side = "left"
			} else if v.ScreenPos.X > midX+20 {
				side = "right"
			}
		}
		name := v.Object.Type
		if v.Object.Name != "" {
			name = fmt.Sprintf("%q (%s)", v.Object.Name, v.Object.Type)
		}
		parts[i] = fmt.Sprintf("%d. %s (%s)", i+1, name, side)
	}
	return "Visible (left to right): " + strings.Join(parts, ", ") + "."
}
