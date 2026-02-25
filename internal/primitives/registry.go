package primitives

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

// cached holds mesh and material for a primitive type. Created lazily on first Draw.
// texturedMtl is used when drawing with an albedo texture (same mesh, different material).
type cached struct {
	mesh       rl.Mesh
	mtl        rl.Material
	texturedMtl rl.Material
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

// defaultPrimitiveColor is the albedo tint for cube and sphere (basic material).
var defaultPrimitiveColor = rl.NewColor(128, 128, 128, 255)

// defaultSphereRings and defaultSphereSlices control sphere mesh resolution.
const defaultSphereRings = 16
const defaultSphereSlices = 16

// defaultCylinderSlices controls cylinder mesh resolution.
const defaultCylinderSlices = 16

// defaultPlaneResX/Z: 1 subdivision = single quad (1×1 in XZ).
const defaultPlaneResX = 1
const defaultPlaneResZ = 1

// ensureCube creates the cube mesh and material if not yet cached.
// Uses a simple lighting shader (directional light + ambient) so the cube has visible shading.
func (r *Registry) ensureCube() {
	if _, ok := r.cache["cube"]; ok {
		return
	}
	mesh := rl.GenMeshCube(1, 1, 1)
	mtl := rl.LoadMaterialDefault()
	if albedo := mtl.GetMap(rl.MapAlbedo); albedo != nil {
		albedo.Color = defaultPrimitiveColor
	}
	shader := loadLitShader()
	if rl.IsShaderValid(shader) {
		mtl.Shader = shader
	}
	texturedMtl := rl.LoadMaterialDefault()
	if albedo := texturedMtl.GetMap(rl.MapAlbedo); albedo != nil {
		albedo.Color = rl.White
	}
	if ts := loadLitTexturedShader(); rl.IsShaderValid(ts) {
		texturedMtl.Shader = ts
	}
	r.cache["cube"] = cached{mesh: mesh, mtl: mtl, texturedMtl: texturedMtl}
}

// ensureSphere creates the sphere mesh and material if not yet cached.
// Reuses the same lit shader as the cube.
func (r *Registry) ensureSphere() {
	if _, ok := r.cache["sphere"]; ok {
		return
	}
	// Radius 0.5 so diameter = 1, matching cube side length (1) for same default size.
	mesh := rl.GenMeshSphere(0.5, defaultSphereRings, defaultSphereSlices)
	mtl := rl.LoadMaterialDefault()
	if albedo := mtl.GetMap(rl.MapAlbedo); albedo != nil {
		albedo.Color = defaultPrimitiveColor
	}
	shader := loadLitShader()
	if rl.IsShaderValid(shader) {
		mtl.Shader = shader
	}
	texturedMtl := rl.LoadMaterialDefault()
	if albedo := texturedMtl.GetMap(rl.MapAlbedo); albedo != nil {
		albedo.Color = rl.White
	}
	if ts := loadLitTexturedShader(); rl.IsShaderValid(ts) {
		texturedMtl.Shader = ts
	}
	r.cache["sphere"] = cached{mesh: mesh, mtl: mtl, texturedMtl: texturedMtl}
}

// ensureCylinder creates the cylinder mesh and material if not yet cached.
// Radius 0.5 and height 1 so diameter and height match cube side length (1). Reuses lit shader.
func (r *Registry) ensureCylinder() {
	if _, ok := r.cache["cylinder"]; ok {
		return
	}
	mesh := rl.GenMeshCylinder(0.5, 1, defaultCylinderSlices)
	mtl := rl.LoadMaterialDefault()
	if albedo := mtl.GetMap(rl.MapAlbedo); albedo != nil {
		albedo.Color = defaultPrimitiveColor
	}
	shader := loadLitShader()
	if rl.IsShaderValid(shader) {
		mtl.Shader = shader
	}
	texturedMtl := rl.LoadMaterialDefault()
	if albedo := texturedMtl.GetMap(rl.MapAlbedo); albedo != nil {
		albedo.Color = rl.White
	}
	if ts := loadLitTexturedShader(); rl.IsShaderValid(ts) {
		texturedMtl.Shader = ts
	}
	r.cache["cylinder"] = cached{mesh: mesh, mtl: mtl, texturedMtl: texturedMtl}
}

// ensurePlane creates the plane (quad) mesh and material if not yet cached.
// 1×1 in XZ, centered at origin (raylib plane is centered). Reuses lit shader.
func (r *Registry) ensurePlane() {
	if _, ok := r.cache["plane"]; ok {
		return
	}
	mesh := rl.GenMeshPlane(1, 1, defaultPlaneResX, defaultPlaneResZ)
	mtl := rl.LoadMaterialDefault()
	if albedo := mtl.GetMap(rl.MapAlbedo); albedo != nil {
		albedo.Color = defaultPrimitiveColor
	}
	shader := loadLitShader()
	if rl.IsShaderValid(shader) {
		mtl.Shader = shader
	}
	texturedMtl := rl.LoadMaterialDefault()
	if albedo := texturedMtl.GetMap(rl.MapAlbedo); albedo != nil {
		albedo.Color = rl.White
	}
	if ts := loadLitTexturedShader(); rl.IsShaderValid(ts) {
		texturedMtl.Shader = ts
	}
	r.cache["plane"] = cached{mesh: mesh, mtl: mtl, texturedMtl: texturedMtl}
}

// loadLitShader returns a shader that does simple directional light + ambient.
// Used by cube and sphere. Same vertex attributes as raylib meshes: vertexPosition, vertexTexCoord, vertexNormal.
func loadLitShader() rl.Shader {
	return rl.LoadShaderFromMemory(litVS, litFS)
}

// loadLitTexturedShader returns a shader that samples albedo texture and applies directional light + ambient.
// Used when drawing primitives with a texture (MapAlbedo set on material).
func loadLitTexturedShader() rl.Shader {
	return rl.LoadShaderFromMemory(litVS, litTexturedFS)
}

const (
	litVS = `#version 330
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
	litFS = `#version 330
in vec3 fragPosition;
in vec2 fragTexCoord;
in vec3 fragNormal;
uniform vec4 colDiffuse;
uniform vec3 viewPos;
uniform vec3 lightDir;
uniform vec4 ambient;
uniform vec3 lightColor;
uniform float lightIntensity;
uniform float specularPower;
uniform float specularStrength;
out vec4 finalColor;
void main() {
  vec4 tint = colDiffuse;
  vec3 N = normalize(fragNormal);
  vec3 L = normalize(lightDir);
  vec3 V = normalize(viewPos - fragPosition);
  float NdotL = max(dot(N, L), 0.0);
  vec3 diffuse = tint.rgb * NdotL * lightColor * lightIntensity;
  vec3 amb = ambient.rgb * tint.rgb;
  vec3 H = normalize(L + V);
  float NdotH = max(dot(N, H), 0.0);
  float spec = pow(NdotH, specularPower) * specularStrength;
  vec3 specular = lightColor * spec * (NdotL > 0.0 ? 1.0 : 0.0);
  finalColor = vec4(amb + diffuse + specular, tint.a);
}
`
	// litTexturedFS: same as litFS but tint from albedo texture * colDiffuse (for textured primitives).
	litTexturedFS = `#version 330
in vec3 fragPosition;
in vec2 fragTexCoord;
in vec3 fragNormal;
uniform vec4 colDiffuse;
uniform vec3 viewPos;
uniform vec3 lightDir;
uniform vec4 ambient;
uniform vec3 lightColor;
uniform float lightIntensity;
uniform float specularPower;
uniform float specularStrength;
uniform sampler2D albedoMap;
out vec4 finalColor;
void main() {
  vec4 texColor = texture(albedoMap, fragTexCoord);
  vec4 tint = texColor * colDiffuse;
  vec3 N = normalize(fragNormal);
  vec3 L = normalize(lightDir);
  vec3 V = normalize(viewPos - fragPosition);
  float NdotL = max(dot(N, L), 0.0);
  vec3 diffuse = tint.rgb * NdotL * lightColor * lightIntensity;
  vec3 amb = ambient.rgb * tint.rgb;
  vec3 H = normalize(L + V);
  float NdotH = max(dot(N, H), 0.0);
  float spec = pow(NdotH, specularPower) * specularStrength;
  vec3 specular = lightColor * spec * (NdotL > 0.0 ? 1.0 : 0.0);
  finalColor = vec4(amb + diffuse + specular, tint.a);
}
`
)

// defaultAmbient is the ambient term (dim so shadowed areas aren't pure black).
var defaultAmbient = [4]float32{0.2, 0.22, 0.26, 1.0}

// defaultLightColor is a soft warm-white for the directional light.
var defaultLightColor = [3]float32{1.0, 0.98, 0.95}

// defaultLightIntensity scales the directional diffuse (0–1).
const defaultLightIntensity = float32(0.75)

// defaultSpecularPower controls highlight tightness (higher = smaller, sharper highlight).
const defaultSpecularPower = float32(48.0)

// defaultSpecularStrength scales specular contribution (0–1).
const defaultSpecularStrength = float32(0.35)

// setLitShaderUniforms sets viewPos, lightDir, ambient, light color/intensity, and specular on the given shader (cgo-safe: local arrays).
func (r *Registry) setLitShaderUniforms(shader rl.Shader) {
	if !rl.IsShaderValid(shader) {
		return
	}
	viewPos := [3]float32{r.viewPos[0], r.viewPos[1], r.viewPos[2]}
	lightDir := [3]float32{r.lightDir[0], r.lightDir[1], r.lightDir[2]}
	amb := [4]float32{defaultAmbient[0], defaultAmbient[1], defaultAmbient[2], defaultAmbient[3]}
	lightColor := [3]float32{defaultLightColor[0], defaultLightColor[1], defaultLightColor[2]}
	if loc := rl.GetShaderLocation(shader, "viewPos"); loc >= 0 {
		rl.SetShaderValueV(shader, loc, viewPos[:], rl.ShaderUniformVec3, 1)
	}
	if loc := rl.GetShaderLocation(shader, "lightDir"); loc >= 0 {
		rl.SetShaderValueV(shader, loc, lightDir[:], rl.ShaderUniformVec3, 1)
	}
	if loc := rl.GetShaderLocation(shader, "ambient"); loc >= 0 {
		rl.SetShaderValueV(shader, loc, amb[:], rl.ShaderUniformVec4, 1)
	}
	if loc := rl.GetShaderLocation(shader, "lightColor"); loc >= 0 {
		rl.SetShaderValueV(shader, loc, lightColor[:], rl.ShaderUniformVec3, 1)
	}
	if loc := rl.GetShaderLocation(shader, "lightIntensity"); loc >= 0 {
		rl.SetShaderValue(shader, loc, []float32{defaultLightIntensity}, rl.ShaderUniformFloat)
	}
	if loc := rl.GetShaderLocation(shader, "specularPower"); loc >= 0 {
		rl.SetShaderValue(shader, loc, []float32{defaultSpecularPower}, rl.ShaderUniformFloat)
	}
	if loc := rl.GetShaderLocation(shader, "specularStrength"); loc >= 0 {
		rl.SetShaderValue(shader, loc, []float32{defaultSpecularStrength}, rl.ShaderUniformFloat)
	}
}

// drawCached draws a cached mesh with the given key at position and scale (scale 0 → 1).
// modelCenterOffset shifts the mesh in model space before scale/translate so the scene position
// is the primitive's center. Use (0,0,0) for cube/sphere (already centered); (0,-0.5,0) for cylinder
// (raylib cylinder has base at Y=0, top at Y=height, so offset -height/2 centers it).
func (r *Registry) drawCached(key string, position, scale [3]float32, modelCenterOffset [3]float32) {
	c, ok := r.cache[key]
	if !ok {
		return
	}
	r.setLitShaderUniforms(c.mtl.Shader)
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
	var transform rl.Matrix
	if modelCenterOffset[0] != 0 || modelCenterOffset[1] != 0 || modelCenterOffset[2] != 0 {
		offsetM := rl.MatrixTranslate(modelCenterOffset[0], modelCenterOffset[1], modelCenterOffset[2])
		// Order: offset (center mesh), then scale, then translate to position.
		transform = rl.MatrixMultiply(rl.MatrixMultiply(transM, scaleM), offsetM)
	} else {
		transform = rl.MatrixMultiply(scaleM, transM)
	}
	rl.DrawMesh(c.mesh, c.mtl, transform)
}

// drawCachedWithTexture draws a cached mesh with the given key using the textured material and the given albedo texture.
func (r *Registry) drawCachedWithTexture(key string, position, scale [3]float32, modelCenterOffset [3]float32, tex rl.Texture2D) {
	c, ok := r.cache[key]
	if !ok {
		return
	}
	rl.SetMaterialTexture(&c.texturedMtl, rl.MapAlbedo, tex)
	r.setLitShaderUniforms(c.texturedMtl.Shader)
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
	var transform rl.Matrix
	if modelCenterOffset[0] != 0 || modelCenterOffset[1] != 0 || modelCenterOffset[2] != 0 {
		offsetM := rl.MatrixTranslate(modelCenterOffset[0], modelCenterOffset[1], modelCenterOffset[2])
		transform = rl.MatrixMultiply(rl.MatrixMultiply(transM, scaleM), offsetM)
	} else {
		transform = rl.MatrixMultiply(scaleM, transM)
	}
	rl.DrawMesh(c.mesh, c.texturedMtl, transform)
}

// Draw draws one instance of the given type at position with scale.
// Must be called between BeginMode3D and EndMode3D.
// SetView must be called once per frame before drawing so lit primitives get shading.
// Unknown types are skipped. "cube", "sphere", "cylinder", and "plane" are created on first use.
func (r *Registry) Draw(primType string, position, scale [3]float32) {
	switch primType {
	case "cube":
		r.ensureCube()
		r.drawCached("cube", position, scale, [3]float32{0, 0, 0})
	case "sphere":
		r.ensureSphere()
		r.drawCached("sphere", position, scale, [3]float32{0, 0, 0})
	case "cylinder":
		r.ensureCylinder()
		// Raylib cylinder: base Y=0, top Y=height. Offset -height/2 so center is at position.
		r.drawCached("cylinder", position, scale, [3]float32{0, -0.5, 0})
	case "plane":
		r.ensurePlane()
		r.drawCached("plane", position, scale, [3]float32{0, 0, 0})
	default:
		// Unknown type; skip. More primitives added later on demand.
	}
}

// DrawWithTexture draws one instance of the given type at position with scale, using the given texture as albedo.
// Must be called between BeginMode3D and EndMode3D. SetView must be called once per frame before drawing.
func (r *Registry) DrawWithTexture(primType string, position, scale [3]float32, tex rl.Texture2D) {
	if !rl.IsTextureValid(tex) {
		r.Draw(primType, position, scale)
		return
	}
	switch primType {
	case "cube":
		r.ensureCube()
		r.drawCachedWithTexture("cube", position, scale, [3]float32{0, 0, 0}, tex)
	case "sphere":
		r.ensureSphere()
		r.drawCachedWithTexture("sphere", position, scale, [3]float32{0, 0, 0}, tex)
	case "cylinder":
		r.ensureCylinder()
		r.drawCachedWithTexture("cylinder", position, scale, [3]float32{0, -0.5, 0}, tex)
	case "plane":
		r.ensurePlane()
		r.drawCachedWithTexture("plane", position, scale, [3]float32{0, 0, 0}, tex)
	default:
		r.Draw(primType, position, scale)
	}
}
