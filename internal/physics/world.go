package physics

import (
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// World holds a set of bodies and runs a simple 3D physics step: gravity, integration, AABB collision.
type World struct {
	Gravity [3]float32
	Bodies  []*Body
}

// NewWorld returns a new physics world with default gravity (0, -9.8, 0) in Y-down style.
// Your scene uses Y-up; we use negative Y so "down" is -Y.
func NewWorld() *World {
	return &World{
		Gravity: [3]float32{0, -9.8, 0},
		Bodies:  nil,
	}
}

// SetGravity sets the gravity vector (e.g. [0, -9.8, 0] for down in -Y).
func (w *World) SetGravity(g [3]float32) {
	w.Gravity = g
}

// AddBody appends a body to the world. Order is preserved for syncing with scene objects.
func (w *World) AddBody(b *Body) {
	w.Bodies = append(w.Bodies, b)
}

// bodyAABB returns the AABB for a body (center position, half extents from scale).
func bodyAABB(b *Body) rl.BoundingBox {
	sx, sy, sz := b.Scale[0], b.Scale[1], b.Scale[2]
	if sx == 0 {
		sx = 1
	}
	if sy == 0 {
		sy = 1
	}
	if sz == 0 {
		sz = 1
	}
	half := [3]float32{sx * 0.5, sy * 0.5, sz * 0.5}
	return rl.NewBoundingBox(
		rl.NewVector3(b.Position[0]-half[0], b.Position[1]-half[1], b.Position[2]-half[2]),
		rl.NewVector3(b.Position[0]+half[0], b.Position[1]+half[1], b.Position[2]+half[2]),
	)
}

// penetrationAxis returns the overlap amount and axis index (0=X, 1=Y, 2=Z) for the minimum penetration.
// If no overlap, returns (0, -1).
func penetrationAxis(a, b rl.BoundingBox) (depth float32, axis int) {
	overlapX := min(a.Max.X, b.Max.X) - max(a.Min.X, b.Min.X)
	overlapY := min(a.Max.Y, b.Max.Y) - max(a.Min.Y, b.Min.Y)
	overlapZ := min(a.Max.Z, b.Max.Z) - max(a.Min.Z, b.Min.Z)
	if overlapX <= 0 || overlapY <= 0 || overlapZ <= 0 {
		return 0, -1
	}
	depth = overlapX
	axis = 0
	if overlapY < depth {
		depth = overlapY
		axis = 1
	}
	if overlapZ < depth {
		depth = overlapZ
		axis = 2
	}
	return depth, axis
}

// Timestep limits for Advance. Frame times are split into sub-steps of at most maxStep seconds so
// fast bodies don't tunnel through thin colliders, and capped at maxFrameTime so a long stall
// (e.g. the window being dragged) doesn't launch everything at once.
const (
	maxStep      = float32(1.0 / 60)
	maxFrameTime = float32(0.25)
)

// Advance simulates dt seconds of frame time in one or more Steps of at most maxStep.
func (w *World) Advance(dt float32) {
	dt = min(dt, maxFrameTime)
	if dt <= 0 {
		return
	}
	n := int(math.Ceil(float64(dt / maxStep)))
	for range n {
		w.Step(dt / float32(n))
	}
}

// Step advances the simulation by dt seconds: apply gravity, integrate, then resolve AABB overlaps.
// No global floor: dynamic bodies can fall below Y=0 until they hit another body (e.g. a static plane).
func (w *World) Step(dt float32) {
	for _, b := range w.Bodies {
		if b.Static {
			continue
		}
		for i := range 3 {
			b.Velocity[i] += w.Gravity[i] * dt
			b.Position[i] += b.Velocity[i] * dt
		}
	}

	// Resolve overlapping pairs by pushing them apart along the axis of least penetration.
	for i, bi := range w.Bodies {
		boxI := bodyAABB(bi)
		for _, bj := range w.Bodies[i+1:] {
			if bi.Static && bj.Static {
				continue
			}
			depth, axis := penetrationAxis(boxI, bodyAABB(bj))
			if axis < 0 {
				continue
			}
			// Push each body away from the other's center; on a tie, bj goes toward +axis.
			dir := float32(1)
			if bj.Position[axis] < bi.Position[axis] {
				dir = -1
			}
			var shareI, shareJ float32 // fraction of depth each body moves
			switch {
			case bi.Static:
				shareJ = 1
			case bj.Static:
				shareI = 1
			default:
				total := bi.Mass + bj.Mass
				shareI, shareJ = bj.Mass/total, bi.Mass/total
			}
			bi.Position[axis] -= dir * depth * shareI
			bj.Position[axis] += dir * depth * shareJ
			if !bi.Static {
				bi.Velocity[axis] = 0
			}
			if !bj.Static {
				bj.Velocity[axis] = 0
			}
			boxI = bodyAABB(bi) // bi may have moved; use its new box for the remaining pairs
		}
	}
}
