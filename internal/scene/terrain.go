package scene

import (
	"slices"

	"game-engine/internal/primitives"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// SetTerrain installs mesh (built in world space around the origin, base at Y=0) as the scene's
// terrain, replacing any previous terrain mesh. size is (width, max height, depth); the terrain
// object gets it as scale so it can be selected by clicking anywhere on the mesh. The terrain is
// static and not recorded for undo. The scene takes ownership of mesh.
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
