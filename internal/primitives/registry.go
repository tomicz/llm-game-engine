// Package primitives draws the engine's built-in 3D shapes (cube, sphere, cylinder, plane) and the
// runtime-generated terrain mesh with a shared lit shader.
package primitives

import (
	"slices"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Shape names. Terrain is not spawnable: its mesh is generated at runtime (see SetTerrainMesh).
const (
	Cube     = "cube"
	Sphere   = "sphere"
	Cylinder = "cylinder"
	Plane    = "plane"
	Terrain  = "terrain"
)

// Shapes lists the spawnable primitive types in display order.
var Shapes = []string{Cube, Sphere, Cylinder, Plane}

// IsShape reports whether name is a spawnable primitive type (terrain excluded).
func IsShape(name string) bool {
	return slices.Contains(Shapes, name)
}

// Mesh resolution for curved shapes.
const (
	sphereRings    = 16
	sphereSlices   = 16
	cylinderSlices = 16
)

// shapeDef describes how to build a unit-sized shape. All shapes span 1 unit on each axis
// (plane: 1×1 in XZ) so that scale is the object's size in world units.
type shapeDef struct {
	gen func() rl.Mesh
	// centerOffset moves the mesh in model space so the object's position is its center
	// (raylib's cylinder has its base at Y=0).
	centerOffset [3]float32
}

var shapeDefs = map[string]shapeDef{
	Cube:     {gen: func() rl.Mesh { return rl.GenMeshCube(1, 1, 1) }},
	Sphere:   {gen: func() rl.Mesh { return rl.GenMeshSphere(0.5, sphereRings, sphereSlices) }},
	Cylinder: {gen: func() rl.Mesh { return rl.GenMeshCylinder(0.5, 1, cylinderSlices) }, centerOffset: [3]float32{0, -0.5, 0}},
	Plane:    {gen: func() rl.Mesh { return rl.GenMeshPlane(1, 1, 1, 1) }},
}

// defaultPrimitiveColor is the albedo for objects without a color.
var defaultPrimitiveColor = rl.NewColor(128, 128, 128, 255)

// Registry owns primitive meshes and the two shared lit materials (plain and textured). GPU
// resources are created lazily on first use so they exist only after the window/OpenGL context.
type Registry struct {
	meshes      map[string]rl.Mesh
	lit         litMaterial
	litTextured litMaterial
	ready       bool
	terrainUV   [2]float32 // texture tiling for the terrain mesh
}

// NewRegistry returns an empty registry. Nothing touches the GPU until SetView or Draw.
func NewRegistry() *Registry {
	return &Registry{
		meshes:    make(map[string]rl.Mesh),
		terrainUV: [2]float32{1, 1},
	}
}

func (r *Registry) ensureMaterials() {
	if r.ready {
		return
	}
	r.lit = newLitMaterial(litFS)
	r.litTextured = newLitMaterial(litTexturedFS)
	r.ready = true
}

// SetView uploads the camera position and direction-to-light for this frame. Call once per frame,
// inside BeginMode3D, before drawing primitives.
func (r *Registry) SetView(viewPos, lightDir [3]float32) {
	r.ensureMaterials()
	r.lit.setView(viewPos, lightDir)
	r.litTextured.setView(viewPos, lightDir)
}

// SetTerrainMesh installs mesh as the terrain shape, releasing any previous terrain mesh.
// The registry takes ownership of mesh.
func (r *Registry) SetTerrainMesh(mesh rl.Mesh) {
	r.ClearTerrain()
	r.meshes[Terrain] = mesh
	r.terrainUV = [2]float32{1, 1}
}

// ClearTerrain releases the terrain mesh, if any.
func (r *Registry) ClearTerrain() {
	if m, ok := r.meshes[Terrain]; ok {
		rl.UnloadMesh(&m)
		delete(r.meshes, Terrain)
	}
}

// HasTerrain reports whether a terrain mesh is installed.
func (r *Registry) HasTerrain() bool {
	_, ok := r.meshes[Terrain]
	return ok
}

// SetTerrainUVScale sets how many times the terrain texture repeats across the X/Z extent.
// For example, (4,4) tiles the texture 4x4; (1,1) stretches it once. Non-positive values mean 1.
func (r *Registry) SetTerrainUVScale(u, v float32) {
	if u <= 0 {
		u = 1
	}
	if v <= 0 {
		v = 1
	}
	r.terrainUV = [2]float32{u, v}
}

// mesh returns the mesh for a shape, generating built-in shapes on first use.
func (r *Registry) mesh(name string) (rl.Mesh, bool) {
	if m, ok := r.meshes[name]; ok {
		return m, true
	}
	def, ok := shapeDefs[name]
	if !ok {
		return rl.Mesh{}, false
	}
	m := def.gen()
	r.meshes[name] = m
	return m, true
}

// Draw draws one instance of a shape at position (its center) with scale; zero scale components
// mean 1. tint is RGBA 0–1, or nil for the default gray. Unknown shapes are skipped.
// Must be called between BeginMode3D and EndMode3D, after SetView.
func (r *Registry) Draw(name string, position, scale [3]float32, tint *[4]float32) {
	r.draw(name, position, scale, tint, nil)
}

// DrawWithTexture is Draw with tex as the albedo map. Invalid textures fall back to Draw.
func (r *Registry) DrawWithTexture(name string, position, scale [3]float32, tex rl.Texture2D, tint *[4]float32) {
	if !rl.IsTextureValid(tex) {
		r.draw(name, position, scale, tint, nil)
		return
	}
	r.draw(name, position, scale, tint, &tex)
}

func (r *Registry) draw(name string, position, scale [3]float32, tint *[4]float32, tex *rl.Texture2D) {
	mesh, ok := r.mesh(name)
	if !ok {
		return
	}
	r.ensureMaterials()
	m := &r.lit
	if tex != nil {
		m = &r.litTextured
		uv := [2]float32{1, 1}
		if name == Terrain {
			// Terrain UVs go beyond 0–1 when tiled, so the texture must repeat.
			rl.SetTextureWrap(*tex, rl.TextureWrapRepeat)
			uv = r.terrainUV
		}
		rl.SetMaterialTexture(&m.mtl, rl.MapAlbedo, *tex)
		m.setUVScale(uv)
	}
	// DrawMesh uploads the albedo color as the shader's colDiffuse uniform. Untinted textures are
	// drawn as-is (white); untinted plain shapes use the default gray.
	if albedo := m.mtl.GetMap(rl.MapAlbedo); albedo != nil {
		albedo.Color = tintToColor(tint)
		if tex != nil && tint == nil {
			albedo.Color = rl.White
		}
	}
	rl.DrawMesh(mesh, m.mtl, modelMatrix(position, scale, shapeDefs[name].centerOffset))
}

// modelMatrix returns the transform that centers the unit mesh (centerOffset), scales it, then
// moves it to position. Zero scale components are treated as 1.
func modelMatrix(position, scale, centerOffset [3]float32) rl.Matrix {
	for i := range scale {
		if scale[i] == 0 {
			scale[i] = 1
		}
	}
	// raylib's MatrixMultiply(a, b) applies a first, then b.
	m := rl.MatrixMultiply(
		rl.MatrixTranslate(centerOffset[0], centerOffset[1], centerOffset[2]),
		rl.MatrixScale(scale[0], scale[1], scale[2]),
	)
	return rl.MatrixMultiply(m, rl.MatrixTranslate(position[0], position[1], position[2]))
}

// tintToColor converts RGBA 0–1 to rl.Color; nil means the default gray.
func tintToColor(tint *[4]float32) rl.Color {
	if tint == nil {
		return defaultPrimitiveColor
	}
	return rl.NewColor(uint8(tint[0]*255), uint8(tint[1]*255), uint8(tint[2]*255), uint8(tint[3]*255))
}
