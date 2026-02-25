package physics

import (
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

// Step advances the simulation by dt seconds: apply gravity, integrate, then AABB collisions.
// No global floor: dynamic bodies can fall below Y=0 until they hit another body (e.g. a static plane).
func (w *World) Step(dt float32) {
	// Apply gravity and integrate for dynamic bodies
	for _, b := range w.Bodies {
		if b.Static {
			continue
		}
		b.Velocity[0] += w.Gravity[0] * dt
		b.Velocity[1] += w.Gravity[1] * dt
		b.Velocity[2] += w.Gravity[2] * dt
		b.Position[0] += b.Velocity[0] * dt
		b.Position[1] += b.Velocity[1] * dt
		b.Position[2] += b.Velocity[2] * dt
	}

	// AABB collision: resolve overlapping pairs (push apart along minimum penetration axis)
	for i := 0; i < len(w.Bodies); i++ {
		bi := w.Bodies[i]
		boxI := bodyAABB(bi)
		for j := i + 1; j < len(w.Bodies); j++ {
			bj := w.Bodies[j]
			if !rl.CheckCollisionBoxes(boxI, bodyAABB(bj)) {
				continue
			}
			boxJ := bodyAABB(bj)
			depth, axis := penetrationAxis(boxI, boxJ)
			if axis < 0 {
				continue
			}
			// Push apart: move along axis. Static doesn't move.
			totalMass := bi.Mass + bj.Mass
			if bi.Static {
				totalMass = bj.Mass
			}
			if bj.Static {
				totalMass = bi.Mass
			}
			var moveI, moveJ float32
			if bi.Static {
				moveI = 0
				moveJ = depth
			} else if bj.Static {
				moveI = -depth
				moveJ = 0
			} else {
				moveI = -depth * (bj.Mass / totalMass)
				moveJ = depth * (bi.Mass / totalMass)
			}
			switch axis {
			case 0:
				bi.Position[0] += moveI
				bj.Position[0] += moveJ
				if !bi.Static {
					bi.Velocity[0] = 0
				}
				if !bj.Static {
					bj.Velocity[0] = 0
				}
			case 1:
				bi.Position[1] += moveI
				bj.Position[1] += moveJ
				if !bi.Static {
					bi.Velocity[1] = 0
				}
				if !bj.Static {
					bj.Velocity[1] = 0
				}
			case 2:
				bi.Position[2] += moveI
				bj.Position[2] += moveJ
				if !bi.Static {
					bi.Velocity[2] = 0
				}
				if !bj.Static {
					bj.Velocity[2] = 0
				}
			}
			boxI = bodyAABB(bi) // update for next pair
		}
	}
}
