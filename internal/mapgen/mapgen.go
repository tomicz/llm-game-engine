package mapgen

import (
	"math"
	"time"

	"game-engine/internal/scene"
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

// GenerateHeightMapCubes builds a height map as a grid of cube primitives sitting on Y=0.
// Each tile becomes one cube whose Y scale is derived from fractal noise. The cubes are
// centered around the world origin on XZ.
func GenerateHeightMapCubes(opts HeightMapOptions) []scene.ObjectInstance {
	if opts.Width <= 0 || opts.Depth <= 0 {
		return nil
	}
	if opts.TileSize <= 0 {
		opts.TileSize = 1
	}
	if opts.HeightScale <= 0 {
		opts.HeightScale = 1
	}
	if opts.Octaves <= 0 {
		opts.Octaves = 1
	}
	if opts.Frequency <= 0 {
		opts.Frequency = 0.05
	}
	if opts.Lacunarity <= 0 {
		opts.Lacunarity = 2.0
	}
	if opts.Gain <= 0 {
		opts.Gain = 0.5
	}
	seed := opts.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	// Center the map around the origin. First cube center is at (-extentX + halfTile, -extentZ + halfTile).
	halfTile := opts.TileSize * 0.5
	extentX := float32(opts.Width) * opts.TileSize * 0.5
	extentZ := float32(opts.Depth) * opts.TileSize * 0.5
	startX := -extentX + halfTile
	startZ := -extentZ + halfTile

	objs := make([]scene.ObjectInstance, 0, opts.Width*opts.Depth)
	// All heightmap tiles should be static terrain (no gravity).

	baseFreq := opts.Frequency
	for z := 0; z < opts.Depth; z++ {
		for x := 0; x < opts.Width; x++ {
			nx := float32(x)
			nz := float32(z)
			// Sample fractal noise in a continuous domain; use X/Z indices scaled by base frequency.
			h := fractalValueNoise2D(nx*baseFreq, nz*baseFreq, seed, opts.Octaves, opts.Lacunarity, opts.Gain)
			// Map [0,1] noise to [minHeight, HeightScale].
			minHeight := float32(0.15)
			height := minHeight + h*(opts.HeightScale-minHeight)
			// Keep height positive and finite.
			if !isFinite(height) || height <= 0 {
				height = minHeight
			}

			worldX := startX + float32(x)*opts.TileSize
			worldZ := startZ + float32(z)*opts.TileSize
			worldY := height * 0.5 // bottom at Y=0

			static := false

			objs = append(objs, scene.ObjectInstance{
				Type: "cube",
				Position: [3]float32{
					worldX,
					worldY,
					worldZ,
				},
				Scale: [3]float32{
					opts.TileSize,
					height,
					opts.TileSize,
				},
				Physics: &static,
			})
		}
	}

	return objs
}

// ApplyHeightmapTerrain generates a single deformed plane mesh using fractal noise and
// installs it as optimized terrain in the given scene. This avoids thousands of cubes
// and is much faster to render.
func ApplyHeightmapTerrain(scn *scene.Scene, opts HeightMapOptions) error {
	if opts.Width <= 1 || opts.Depth <= 1 {
		// Need at least a 2x2 grid for meaningful deformation.
		if opts.Width <= 1 {
			opts.Width = 32
		}
		if opts.Depth <= 1 {
			opts.Depth = 32
		}
	}
	if opts.TileSize <= 0 {
		opts.TileSize = 1
	}
	if opts.HeightScale <= 0 {
		opts.HeightScale = 3
	}
	if opts.Octaves <= 0 {
		opts.Octaves = 4
	}
	if opts.Frequency <= 0 {
		opts.Frequency = 0.08
	}
	if opts.Lacunarity <= 0 {
		opts.Lacunarity = 2.0
	}
	if opts.Gain <= 0 {
		opts.Gain = 0.5
	}
	seed := opts.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}

	// World size of the plane; centered at origin.
	widthWorld := float32(opts.Width) * opts.TileSize
	depthWorld := float32(opts.Depth) * opts.TileSize

	// Build a grayscale heightmap image using fractal noise, then let raylib
	// turn it into a heightmapped mesh. This avoids manual vertex pointer math.
	img := rl.GenImageColor(opts.Width, opts.Depth, rl.Black)
	baseFreq := opts.Frequency
	for z := 0; z < opts.Depth; z++ {
		for x := 0; x < opts.Width; x++ {
			nx := float32(x)
			nz := float32(z)
			h := fractalValueNoise2D(nx*baseFreq, nz*baseFreq, seed, opts.Octaves, opts.Lacunarity, opts.Gain)
			if !isFinite(h) {
				h = 0
			}
			if h < 0 {
				h = 0
			}
			if h > 1 {
				h = 1
			}
			v := uint8(h * 255)
			c := rl.NewColor(v, v, v, 255)
			rl.ImageDrawPixel(img, int32(x), int32(z), c)
		}
	}
	size := rl.NewVector3(widthWorld, opts.HeightScale, depthWorld)
	mesh := rl.GenMeshHeightmap(*img, size)
	rl.UnloadImage(img)
	if mesh.VertexCount == 0 {
		return nil
	}

	terrainSize := [3]float32{widthWorld, opts.HeightScale, depthWorld}
	scn.EnableTerrain(mesh, terrainSize)
	return nil
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

