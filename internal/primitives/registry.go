package primitives

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

// cached holds mesh and material for a primitive type. Created lazily on first Draw.
type cached struct {
	mesh rl.Mesh
	mtl  rl.Material
}

// Registry maps primitive type names to mesh+material. Meshes are created on first use
// so that GPU resources are allocated after the window/OpenGL context exists.
type Registry struct {
	cache    map[string]cached
	viewPos  [3]float32 // camera position, set each frame for lighting
	lightDir [3]float32 // direction to light (normalized), set each frame
}

// NewRegistry returns a registry with no primitives. Cube is created on first Draw.
func NewRegistry() *Registry {
	return &Registry{
		cache:    make(map[string]cached),
		lightDir: [3]float32{0.5, 1, 0.5}, // default: from above-right
	}
}

// SetView sets camera position and direction-to-light for this frame. Call once per frame
// before drawing objects so lit primitives (e.g. cube) get correct shading.
func (r *Registry) SetView(viewPos, lightDir [3]float32) {
	r.viewPos = viewPos
	r.lightDir = lightDir
}

// defaultCubeColor is the albedo tint for the primitive cube (basic material).
var defaultCubeColor = rl.NewColor(128, 128, 128, 255)

// ensureCube creates the cube mesh and material if not yet cached.
// Uses a simple lighting shader (directional light + ambient) so the cube has visible shading.
func (r *Registry) ensureCube() {
	if _, ok := r.cache["cube"]; ok {
		return
	}
	mesh := rl.GenMeshCube(1, 1, 1)
	mtl := rl.LoadMaterialDefault()
	if albedo := mtl.GetMap(rl.MapAlbedo); albedo != nil {
		albedo.Color = defaultCubeColor
	}
	shader := loadCubeLightingShader()
	if rl.IsShaderValid(shader) {
		mtl.Shader = shader
	}
	r.cache["cube"] = cached{mesh: mesh, mtl: mtl}
}

// loadCubeLightingShader returns a shader that does simple directional light + ambient.
// Uses same vertex attributes as raylib meshes: vertexPosition, vertexTexCoord, vertexNormal.
func loadCubeLightingShader() rl.Shader {
	return rl.LoadShaderFromMemory(cubeLightingVS, cubeLightingFS)
}

const (
	cubeLightingVS = `#version 330
in vec3 vertexPosition;
in vec2 vertexTexCoord;
in vec3 vertexNormal;
uniform mat4 matProjection;
uniform mat4 matView;
uniform mat4 matModel;
out vec3 fragPosition;
out vec2 fragTexCoord;
out vec3 fragNormal;
void main() {
  vec4 worldPos = matModel * vec4(vertexPosition, 1.0);
  fragPosition = worldPos.xyz;
  fragTexCoord = vertexTexCoord;
  fragNormal = mat3(matModel) * vertexNormal;
  gl_Position = matProjection * matView * worldPos;
}
`
	cubeLightingFS = `#version 330
in vec3 fragPosition;
in vec2 fragTexCoord;
in vec3 fragNormal;
uniform vec4 colDiffuse;
uniform vec3 viewPos;
uniform vec3 lightDir;
uniform vec4 ambient;
out vec4 finalColor;
void main() {
  vec4 tint = colDiffuse;
  vec3 N = normalize(fragNormal);
  vec3 L = normalize(lightDir);
  float NdotL = max(dot(N, L), 0.0);
  vec3 diffuse = tint.rgb * NdotL;
  vec3 amb = ambient.rgb * tint.rgb;
  finalColor = vec4(amb + diffuse, tint.a);
}
`
)

// defaultAmbient is the ambient term for the cube lighting shader (dim so faces aren't black).
var defaultAmbient = [4]float32{0.25, 0.25, 0.25, 1.0}

// Draw draws one instance of the given type at position with scale.
// Must be called between BeginMode3D and EndMode3D.
// SetView must be called once per frame before drawing so lit primitives get shading.
// Unknown types are skipped. For "cube", mesh is created on first use.
func (r *Registry) Draw(primType string, position, scale [3]float32) {
	switch primType {
	case "cube":
		r.ensureCube()
		c := r.cache["cube"]
		shader := c.mtl.Shader
		if rl.IsShaderValid(shader) {
			// Copy to local arrays so cgo doesn't receive Go pointers into struct (avoids panic).
			viewPos := [3]float32{r.viewPos[0], r.viewPos[1], r.viewPos[2]}
			lightDir := [3]float32{r.lightDir[0], r.lightDir[1], r.lightDir[2]}
			amb := [4]float32{defaultAmbient[0], defaultAmbient[1], defaultAmbient[2], defaultAmbient[3]}
			if loc := rl.GetShaderLocation(shader, "viewPos"); loc >= 0 {
				rl.SetShaderValueV(shader, loc, viewPos[:], rl.ShaderUniformVec3, 1)
			}
			if loc := rl.GetShaderLocation(shader, "lightDir"); loc >= 0 {
				rl.SetShaderValueV(shader, loc, lightDir[:], rl.ShaderUniformVec3, 1)
			}
			if loc := rl.GetShaderLocation(shader, "ambient"); loc >= 0 {
				rl.SetShaderValueV(shader, loc, amb[:], rl.ShaderUniformVec4, 1)
			}
		}
		sx, sy, sz := scale[0], scale[1], scale[2]
		if sx == 0 {
			sx = 1
		}
		if sy == 0 {
			sy = 1
		}
		if sz == 0 {
			sz = 1
		}
		scaleM := rl.MatrixScale(sx, sy, sz)
		transM := rl.MatrixTranslate(position[0], position[1], position[2])
		transform := rl.MatrixMultiply(scaleM, transM)
		rl.DrawMesh(c.mesh, c.mtl, transform)
	default:
		// Unknown type; skip. More primitives added later on demand.
	}
}
