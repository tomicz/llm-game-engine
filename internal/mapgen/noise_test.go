package mapgen

import (
	"math"
	"testing"
)

func TestHash2DDeterministicAndInRange(t *testing.T) {
	for x := int32(-20); x <= 20; x++ {
		for y := int32(-20); y <= 20; y++ {
			a := hash2D(x, y, 42)
			b := hash2D(x, y, 42)
			if a != b {
				t.Fatalf("hash2D(%d,%d,42) not deterministic: %v vs %v", x, y, a, b)
			}
			if a < 0 || a > 1 {
				t.Fatalf("hash2D(%d,%d,42) = %v, out of [0,1]", x, y, a)
			}
		}
	}
}

func TestValueNoise2DMatchesLatticeAtIntegers(t *testing.T) {
	for x := int32(-5); x <= 5; x++ {
		for y := int32(-5); y <= 5; y++ {
			got := valueNoise2D(float32(x), float32(y), 7)
			want := hash2D(x, y, 7)
			if got != want {
				t.Errorf("valueNoise2D(%d,%d) = %v, want lattice value %v", x, y, got, want)
			}
		}
	}
}

func TestFractalValueNoise2D(t *testing.T) {
	const seed = 1234
	differs := false
	for i := range 50 {
		for j := range 50 {
			x, y := float32(i)*0.13, float32(j)*0.17
			a := fractalValueNoise2D(x, y, seed, 4, 2, 0.5)
			b := fractalValueNoise2D(x, y, seed, 4, 2, 0.5)
			if a != b {
				t.Fatalf("fractalValueNoise2D(%v,%v) not deterministic: %v vs %v", x, y, a, b)
			}
			if a < 0 || a > 1 || math.IsNaN(float64(a)) {
				t.Fatalf("fractalValueNoise2D(%v,%v) = %v, out of [0,1]", x, y, a)
			}
			if a != fractalValueNoise2D(x, y, seed+1, 4, 2, 0.5) {
				differs = true
			}
		}
	}
	if !differs {
		t.Error("different seeds produced identical noise over the whole grid")
	}
}

func TestFractalValueNoise2DZeroOctaves(t *testing.T) {
	if got := fractalValueNoise2D(1.5, 2.5, 1, 0, 2, 0.5); got != 0 {
		t.Errorf("zero octaves = %v, want 0", got)
	}
}

func TestSmoothStep(t *testing.T) {
	tests := []struct {
		in, want float32
	}{
		{-1, 0},
		{0, 0},
		{0.5, 0.5},
		{1, 1},
		{2, 1},
		{0.25, 0.15625},
	}
	for _, tt := range tests {
		if got := smoothStep(tt.in); math.Abs(float64(got-tt.want)) > 1e-6 {
			t.Errorf("smoothStep(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestLerp(t *testing.T) {
	tests := []struct {
		a, b, t, want float32
	}{
		{2, 4, 0, 2},
		{2, 4, 1, 4},
		{2, 4, 0.25, 2.5},
		{-1, 1, 0.5, 0},
	}
	for _, tt := range tests {
		if got := lerp(tt.a, tt.b, tt.t); got != tt.want {
			t.Errorf("lerp(%v,%v,%v) = %v, want %v", tt.a, tt.b, tt.t, got, tt.want)
		}
	}
}

func TestIsFinite(t *testing.T) {
	tests := []struct {
		in   float32
		want bool
	}{
		{0, true},
		{-3.5, true},
		{float32(math.NaN()), false},
		{float32(math.Inf(1)), false},
		{float32(math.Inf(-1)), false},
	}
	for _, tt := range tests {
		if got := isFinite(tt.in); got != tt.want {
			t.Errorf("isFinite(%v) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
