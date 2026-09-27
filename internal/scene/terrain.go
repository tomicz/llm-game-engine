package scene

import (
	"slices"

	"game-engine/internal/primitives"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// SetTerrain installs mesh (spanning 0..size on each axis, as raylib's heightmap generator builds
// it) as the scene's terrain, replacing any previous terrain mesh. size is (width, max height,
// depth). The terrain object is centered on the origin in XZ with its base at Y=0 and gets size as
// its scale; the mesh is drawn to fill that box, so it can be selected by clicking anywhere on it
// and its collider matches. The terrain is static and not recorded for undo. The scene takes
// ownership of mesh.
func (s *Scene) SetTerrain(mesh rl.Mesh, size [3]float32) {
	s.prims.SetTerrainMesh(mesh)
	center := [3]float32{0, size[1] / 2, 0}
	if i := slices.IndexFunc(s.objects, func(o *Object) bool { return o.Type == primitives.Terrain }); i >= 0 {
		s.objects[i].Scale = size
		s.objects[i].Position = center
		return
	}
	t := &Object{Type: primitives.Terrain, Position: center, Scale: size}
	t.SetPhysics(false)
	s.insert(len(s.objects), t)
}

// SetTerrainTextureRepeat sets how many times the terrain texture tiles in U (X) and V (Z).
// (1,1), the default, stretches it once across the terrain.
func (s *Scene) SetTerrainTextureRepeat(u, v float32) {
	s.prims.SetTerrainUVScale(u, v)
}
