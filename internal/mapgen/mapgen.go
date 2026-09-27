package mapgen

import (
	"errors"
	"math"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// HeightMapOptions controls procedural height map generation.
// Width/Depth are in tiles; TileSize is the world size of one tile on X/Z.
// HeightScale is the maximum height of the terrain in world units.
// Seed controls randomness; Seed == 0 uses a time-based seed.
// Octaves, Frequency, Lacunarity, and Gain control the fractal noise shape.
type HeightMapOptions struct {
	Width       int
	Depth       int
	TileSize    float32
	HeightScale float32

	Seed       int64
	Octaves    int
	Frequency  float32
	Lacunarity float32
	Gain       float32
}

// DefaultHeightMapOptions returns a sane default configuration.
func DefaultHeightMapOptions() HeightMapOptions {
	return HeightMapOptions{
		Width:       32,
		Depth:       32,
		TileSize:    1.0,
		HeightScale: 3.0,
		Seed:        0,
		Octaves:     4,
		Frequency:   0.08,
		Lacunarity:  2.0,
		Gain:        0.5,
	}
}

// withDefaults returns opts with unset or invalid fields replaced by the defaults
// and a time-based seed when Seed is 0.
func (opts HeightMapOptions) withDefaults() HeightMapOptions {
	def := DefaultHeightMapOptions()
	// Need at least a 2x2 grid for meaningful deformation.
	if opts.Width <= 1 {
		opts.Width = def.Width
	}
	if opts.Depth <= 1 {
		opts.Depth = def.Depth
	}
	if opts.TileSize <= 0 {
		opts.TileSize = def.TileSize
	}
	if opts.HeightScale <= 0 {
		opts.HeightScale = def.HeightScale
	}
	if opts.Octaves <= 0 {
		opts.Octaves = def.Octaves
	}
	if opts.Frequency <= 0 {
		opts.Frequency = def.Frequency
	}
	if opts.Lacunarity <= 0 {
		opts.Lacunarity = def.Lacunarity
	}
	if opts.Gain <= 0 {
		opts.Gain = def.Gain
	}
	if opts.Seed == 0 {
		opts.Seed = time.Now().UnixNano()
	}
	return opts
}

// GenerateTerrain builds a single heightmapped mesh from fractal noise. Like raylib's
// GenMeshHeightmap, the mesh spans 0..size on each axis. It returns the mesh and its world size
// (width, max height, depth).
// Must be called on the main thread after the window exists (it uploads the mesh to the GPU).
func GenerateTerrain(opts HeightMapOptions) (mesh rl.Mesh, size [3]float32, err error) {
	opts = opts.withDefaults()

	// Build a grayscale heightmap image using fractal noise, then let raylib
	// turn it into a heightmapped mesh. This avoids manual vertex pointer math.
	img := rl.GenImageColor(opts.Width, opts.Depth, rl.Black)
	for z := range opts.Depth {
		for x := range opts.Width {
			h := fractalValueNoise2D(float32(x)*opts.Frequency, float32(z)*opts.Frequency, opts.Seed, opts.Octaves, opts.Lacunarity, opts.Gain)
			if !isFinite(h) {
				h = 0
			}
			v := uint8(min(max(h, 0), 1) * 255)
			rl.ImageDrawPixel(img, int32(x), int32(z), rl.NewColor(v, v, v, 255))
		}
	}
	size = [3]float32{float32(opts.Width) * opts.TileSize, opts.HeightScale, float32(opts.Depth) * opts.TileSize}
	mesh = rl.GenMeshHeightmap(*img, rl.NewVector3(size[0], size[1], size[2]))
	rl.UnloadImage(img)
	if mesh.VertexCount == 0 {
		return rl.Mesh{}, size, errors.New("heightmap: mesh generation failed")
	}
	return mesh, size, nil
}

// fractalValueNoise2D is simple fractal value noise: layered smooth value noise with
// configurable octaves, lacunarity, and gain. Output is in [0,1].
func fractalValueNoise2D(x, y float32, seed int64, octaves int, lacunarity, gain float32) float32 {
	var sum float32
	var amplitude float32 = 1
	var maxAmp float32 = 0
	freq := float32(1)

	for i := 0; i < octaves; i++ {
		n := valueNoise2D(x*freq, y*freq, int32(seed)+int32(i))
		sum += n * amplitude
		maxAmp += amplitude
		amplitude *= gain
		freq *= lacunarity
	}
	if maxAmp == 0 {
		return 0
	}
	return sum / maxAmp
}

// valueNoise2D is smooth value noise in [0,1] using a hash-based lattice and bicubic-like easing.
func valueNoise2D(x, y float32, seed int32) float32 {
	x0 := int32(math.Floor(float64(x)))
	y0 := int32(math.Floor(float64(y)))
	tx := x - float32(x0)
	ty := y - float32(y0)

	// Lattice values at cell corners.
	v00 := hash2D(x0, y0, seed)
	v10 := hash2D(x0+1, y0, seed)
	v01 := hash2D(x0, y0+1, seed)
	v11 := hash2D(x0+1, y0+1, seed)

	// Smooth interpolation.
	sx := smoothStep(tx)
	sy := smoothStep(ty)

	ix0 := lerp(v00, v10, sx)
	ix1 := lerp(v01, v11, sx)
	return lerp(ix0, ix1, sy)
}

// hash2D maps integer lattice coordinates to a deterministic pseudo-random float in [0,1].
func hash2D(x, y, seed int32) float32 {
	n := x*374761393 + y*668265263 + seed*362437
	n = (n ^ (n >> 13)) * 1274126177
	n = n ^ (n >> 16)
	// Convert to [0,1]
	const invMaxInt = 1.0 / 2147483647.0
	return float32(n&0x7fffffff) * float32(invMaxInt)
}

func lerp(a, b, t float32) float32 {
	return a + (b-a)*t
}

// smoothStep is Perlin-style cubic easing: 3t^2 - 2t^3.
func smoothStep(t float32) float32 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	return t * t * (3 - 2*t)
}

func isFinite(f float32) bool {
	return !math.IsNaN(float64(f)) && !math.IsInf(float64(f), 0)
}
