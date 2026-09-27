package physics

import (
	"math"
	"testing"
)

const eps = 1e-4

func approx(a, b float32) bool {
	return math.Abs(float64(a-b)) < eps
}

func approx3(a, b [3]float32) bool {
	return approx(a[0], b[0]) && approx(a[1], b[1]) && approx(a[2], b[2])
}

func TestNewBody(t *testing.T) {
	tests := []struct {
		name     string
		mass     float32
		wantMass float32
	}{
		{"zero mass defaults to 1", 0, 1},
		{"negative mass defaults to 1", -2, 1},
		{"positive mass kept", 3, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos := [3]float32{1, 2, 3}
			scale := [3]float32{4, 5, 6}
			b := NewBody(pos, scale, tt.mass, true)
			if b.Mass != tt.wantMass {
				t.Errorf("Mass = %v, want %v", b.Mass, tt.wantMass)
			}
			if b.Position != pos || b.Scale != scale || !b.Static {
				t.Errorf("body = %+v, want pos %v scale %v static", b, pos, scale)
			}
			if b.Velocity != ([3]float32{}) {
				t.Errorf("Velocity = %v, want zero", b.Velocity)
			}
		})
	}
}

func TestNewWorldAndSetGravity(t *testing.T) {
	w := NewWorld()
	if w.Gravity != ([3]float32{0, -9.8, 0}) {
		t.Errorf("default gravity = %v, want [0 -9.8 0]", w.Gravity)
	}
	if len(w.Bodies) != 0 {
		t.Errorf("new world has %d bodies, want 0", len(w.Bodies))
	}
	w.SetGravity([3]float32{1, 2, 3})
	if w.Gravity != ([3]float32{1, 2, 3}) {
		t.Errorf("gravity after SetGravity = %v", w.Gravity)
	}
	b := NewBody([3]float32{}, [3]float32{1, 1, 1}, 1, false)
	w.AddBody(b)
	if len(w.Bodies) != 1 || w.Bodies[0] != b {
		t.Error("AddBody did not append the body")
	}
}

func TestStepDynamicFalls(t *testing.T) {
	w := NewWorld()
	b := NewBody([3]float32{0, 10, 0}, [3]float32{1, 1, 1}, 1, false)
	w.AddBody(b)

	w.Step(0.1)

	// Semi-implicit Euler: v += g*dt, then p += v*dt.
	if !approx3(b.Velocity, [3]float32{0, -0.98, 0}) {
		t.Errorf("velocity = %v, want [0 -0.98 0]", b.Velocity)
	}
	if !approx3(b.Position, [3]float32{0, 10 - 0.098, 0}) {
		t.Errorf("position = %v, want [0 9.902 0]", b.Position)
	}
}

func TestStepStaticDoesNotMove(t *testing.T) {
	w := NewWorld()
	b := NewBody([3]float32{0, 10, 0}, [3]float32{1, 1, 1}, 1, true)
	w.AddBody(b)

	for range 10 {
		w.Step(0.1)
	}

	if b.Position != ([3]float32{0, 10, 0}) {
		t.Errorf("static position = %v, want unchanged", b.Position)
	}
	if b.Velocity != ([3]float32{}) {
		t.Errorf("static velocity = %v, want zero", b.Velocity)
	}
}

func TestStepCustomGravity(t *testing.T) {
	w := NewWorld()
	w.SetGravity([3]float32{2, 0, -4})
	b := NewBody([3]float32{}, [3]float32{1, 1, 1}, 1, false)
	w.AddBody(b)

	w.Step(0.5)

	if !approx3(b.Velocity, [3]float32{1, 0, -2}) {
		t.Errorf("velocity = %v, want [1 0 -2]", b.Velocity)
	}
	if !approx3(b.Position, [3]float32{0.5, 0, -1}) {
		t.Errorf("position = %v, want [0.5 0 -1]", b.Position)
	}
}

func TestStepDynamicRestingOnStaticIsPushedUp(t *testing.T) {
	w := NewWorld()
	w.SetGravity([3]float32{})
	floor := NewBody([3]float32{0, 0, 0}, [3]float32{10, 1, 10}, 1, true)
	box := NewBody([3]float32{0, 0.9, 0}, [3]float32{1, 1, 1}, 1, false)
	box.Velocity = [3]float32{0, -3, 0}
	w.AddBody(floor)
	w.AddBody(box)

	w.Step(0)

	if !approx3(box.Position, [3]float32{0, 1.0, 0}) {
		t.Errorf("box position = %v, want [0 1 0] (pushed out along Y by 0.1)", box.Position)
	}
	if box.Velocity[1] != 0 {
		t.Errorf("box Y velocity = %v, want 0 after collision", box.Velocity[1])
	}
	if floor.Position != ([3]float32{}) {
		t.Errorf("floor moved to %v", floor.Position)
	}
}

func TestStepMinPenetrationAxisX(t *testing.T) {
	w := NewWorld()
	w.SetGravity([3]float32{})
	wall := NewBody([3]float32{0, 0, 0}, [3]float32{1, 1, 1}, 1, true)
	box := NewBody([3]float32{0.9, 0, 0}, [3]float32{1, 1, 1}, 1, false)
	box.Velocity = [3]float32{-1, 0, 0}
	w.AddBody(wall)
	w.AddBody(box)

	w.Step(0)

	if !approx3(box.Position, [3]float32{1.0, 0, 0}) {
		t.Errorf("box position = %v, want [1 0 0]", box.Position)
	}
	if box.Velocity[0] != 0 {
		t.Errorf("box X velocity = %v, want 0", box.Velocity[0])
	}
}

func TestStepTwoDynamicSplitByMass(t *testing.T) {
	w := NewWorld()
	w.SetGravity([3]float32{})
	a := NewBody([3]float32{0, 0, 0}, [3]float32{1, 1, 1}, 1, false)
	b := NewBody([3]float32{0.8, 0, 0}, [3]float32{1, 1, 1}, 1, false)
	w.AddBody(a)
	w.AddBody(b)

	w.Step(0)

	// Overlap 0.2 on X split evenly between equal masses.
	if !approx(a.Position[0], -0.1) || !approx(b.Position[0], 0.9) {
		t.Errorf("positions = %v / %v, want X -0.1 / 0.9", a.Position, b.Position)
	}
}

func TestStepNoOverlapNoChange(t *testing.T) {
	w := NewWorld()
	w.SetGravity([3]float32{})
	a := NewBody([3]float32{0, 0, 0}, [3]float32{1, 1, 1}, 1, false)
	b := NewBody([3]float32{5, 0, 0}, [3]float32{1, 1, 1}, 1, false)
	w.AddBody(a)
	w.AddBody(b)

	w.Step(0.1)

	if a.Position != ([3]float32{}) || b.Position != ([3]float32{5, 0, 0}) {
		t.Errorf("positions changed without overlap: %v %v", a.Position, b.Position)
	}
}

func TestBodyAABBZeroScaleTreatedAsOne(t *testing.T) {
	b := NewBody([3]float32{1, 2, 3}, [3]float32{0, 0, 0}, 1, false)
	box := bodyAABB(b)
	if !approx(box.Min.X, 0.5) || !approx(box.Max.X, 1.5) ||
		!approx(box.Min.Y, 1.5) || !approx(box.Max.Y, 2.5) ||
		!approx(box.Min.Z, 2.5) || !approx(box.Max.Z, 3.5) {
		t.Errorf("bodyAABB = %+v, want unit box around [1 2 3]", box)
	}
}

func TestPenetrationAxis(t *testing.T) {
	unit := func(x, y, z, sx, sy, sz float32) *Body {
		return NewBody([3]float32{x, y, z}, [3]float32{sx, sy, sz}, 1, false)
	}
	tests := []struct {
		name      string
		a, b      *Body
		wantDepth float32
		wantAxis  int
	}{
		{"separated", unit(0, 0, 0, 1, 1, 1), unit(3, 0, 0, 1, 1, 1), 0, -1},
		{"touching is not overlap", unit(0, 0, 0, 1, 1, 1), unit(1, 0, 0, 1, 1, 1), 0, -1},
		{"x smallest", unit(0, 0, 0, 1, 1, 1), unit(0.9, 0, 0, 1, 1, 1), 0.1, 0},
		{"y smallest", unit(0, 0, 0, 10, 1, 10), unit(0, 0.8, 0, 1, 1, 1), 0.2, 1},
		{"z smallest", unit(0, 0, 0, 1, 1, 1), unit(0, 0, -0.7, 1, 1, 1), 0.3, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			depth, axis := penetrationAxis(bodyAABB(tt.a), bodyAABB(tt.b))
			if axis != tt.wantAxis || !approx(depth, tt.wantDepth) {
				t.Errorf("penetrationAxis = (%v, %d), want (%v, %d)", depth, axis, tt.wantDepth, tt.wantAxis)
			}
		})
	}
}

// Resolution must depend on where bodies are, not on their order in the world.
func TestStepPushDirectionIndependentOfOrder(t *testing.T) {
	for _, dynamicFirst := range []bool{false, true} {
		w := NewWorld()
		w.SetGravity([3]float32{})
		floor := NewBody([3]float32{0, 0, 0}, [3]float32{10, 1, 10}, 1, true)
		box := NewBody([3]float32{0, 0.9, 0}, [3]float32{1, 1, 1}, 1, false)
		if dynamicFirst {
			w.AddBody(box)
			w.AddBody(floor)
		} else {
			w.AddBody(floor)
			w.AddBody(box)
		}

		w.Step(0)

		if !approx3(box.Position, [3]float32{0, 1.0, 0}) {
			t.Errorf("dynamicFirst=%v: box position = %v, want [0 1 0] (pushed up out of the floor)", dynamicFirst, box.Position)
		}
	}
}

func TestStepDynamicBelowStaticIsPushedDown(t *testing.T) {
	w := NewWorld()
	w.SetGravity([3]float32{})
	ceiling := NewBody([3]float32{0, 0, 0}, [3]float32{10, 1, 10}, 1, true)
	box := NewBody([3]float32{0, -0.9, 0}, [3]float32{1, 1, 1}, 1, false)
	w.AddBody(ceiling)
	w.AddBody(box)

	w.Step(0)

	if !approx3(box.Position, [3]float32{0, -1.0, 0}) {
		t.Errorf("box position = %v, want [0 -1 0]", box.Position)
	}
}

func TestAdvanceSubsteps(t *testing.T) {
	tests := []struct {
		name      string
		dt        float32
		wantSteps float32 // effective simulated time
	}{
		{"one frame at 60fps", 1.0 / 60, 1.0 / 60},
		{"slow frame is split", 0.1, 0.1},
		{"stall is capped", 5, maxFrameTime},
		{"zero does nothing", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := NewWorld()
			w.SetGravity([3]float32{0, -1, 0})
			b := NewBody([3]float32{}, [3]float32{1, 1, 1}, 1, false)
			w.AddBody(b)

			w.Advance(tt.dt)

			// Constant gravity: final velocity is g*t regardless of how t is split.
			if !approx(b.Velocity[1], -tt.wantSteps) {
				t.Errorf("velocity = %v, want %v", b.Velocity[1], -tt.wantSteps)
			}
		})
	}
}
