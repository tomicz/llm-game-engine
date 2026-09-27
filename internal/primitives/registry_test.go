package primitives

import (
	"math"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func near(a, b rl.Vector3) bool {
	const eps = 1e-4
	return math.Abs(float64(a.X-b.X)) < eps && math.Abs(float64(a.Y-b.Y)) < eps && math.Abs(float64(a.Z-b.Z)) < eps
}

func TestModelMatrixPlacesMeshCenterAtPosition(t *testing.T) {
	tests := []struct {
		name       string
		meshCenter rl.Vector3 // center of the unit mesh in model space
		offset     [3]float32
		pos, scale [3]float32
	}{
		{"cube at origin", rl.Vector3{}, [3]float32{}, [3]float32{0, 0, 0}, [3]float32{1, 1, 1}},
		{"cube moved and scaled", rl.Vector3{}, [3]float32{}, [3]float32{10, 2, -3}, [3]float32{2, 5, 0.5}},
		// raylib's cylinder has its base at Y=0, so its unit-mesh center is (0, 0.5, 0).
		{"cylinder unit", rl.Vector3{Y: 0.5}, shapeDefs[Cylinder].centerOffset, [3]float32{0, 0, 0}, [3]float32{1, 1, 1}},
		{"cylinder trunk off-origin", rl.Vector3{Y: 0.5}, shapeDefs[Cylinder].centerOffset, [3]float32{10, 1, -4}, [3]float32{0.3, 2, 0.3}},
		{"zero scale means 1", rl.Vector3{Y: 0.5}, shapeDefs[Cylinder].centerOffset, [3]float32{5, 0, 5}, [3]float32{0, 0, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rl.Vector3Transform(tt.meshCenter, modelMatrix(tt.pos, tt.scale, tt.offset))
			want := rl.Vector3{X: tt.pos[0], Y: tt.pos[1], Z: tt.pos[2]}
			if !near(got, want) {
				t.Errorf("mesh center -> %v, want %v", got, want)
			}
		})
	}
}

func TestModelMatrixScalesExtent(t *testing.T) {
	// The top of a unit cylinder (Y=1 in model space) must end up half the scaled height above the center.
	m := modelMatrix([3]float32{3, 4, 5}, [3]float32{1, 2, 1}, shapeDefs[Cylinder].centerOffset)
	got := rl.Vector3Transform(rl.Vector3{Y: 1}, m)
	if want := (rl.Vector3{X: 3, Y: 5, Z: 5}); !near(got, want) {
		t.Errorf("cylinder top -> %v, want %v", got, want)
	}
}

func TestIsShape(t *testing.T) {
	for _, s := range Shapes {
		if !IsShape(s) {
			t.Errorf("IsShape(%q) = false", s)
		}
		if _, ok := shapeDefs[s]; !ok {
			t.Errorf("shape %q has no definition", s)
		}
	}
	for _, s := range []string{Terrain, "", "cone", "Cube"} {
		if IsShape(s) {
			t.Errorf("IsShape(%q) = true", s)
		}
	}
}

func TestTintToColor(t *testing.T) {
	if got := tintToColor(nil); got != defaultPrimitiveColor {
		t.Errorf("nil tint = %v, want default %v", got, defaultPrimitiveColor)
	}
	if got, want := tintToColor(&[4]float32{1, 0, 0.5, 1}), rl.NewColor(255, 0, 127, 255); got != want {
		t.Errorf("tint = %v, want %v", got, want)
	}
}
