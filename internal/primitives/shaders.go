package primitives

import rl "github.com/gen2brain/raylib-go/raylib"

// Lighting constants shared by every lit primitive.
var (
	// defaultAmbient is the ambient term (dim so shadowed areas aren't pure black).
	defaultAmbient = [4]float32{0.2, 0.22, 0.26, 1.0}
	// defaultLightColor is a soft warm-white for the directional light.
	defaultLightColor = [3]float32{1.0, 0.98, 0.95}
)

const (
	// defaultLightIntensity scales the directional diffuse (0–1).
	defaultLightIntensity = float32(0.75)
	// defaultSpecularPower controls highlight tightness (higher = smaller, sharper highlight).
	defaultSpecularPower = float32(48.0)
	// defaultSpecularStrength scales specular contribution (0–1).
	defaultSpecularStrength = float32(0.35)
)

// litMaterial is a material using one of the lit shaders, with its per-frame uniform locations cached
// so drawing does not look them up by name for every object.
type litMaterial struct {
	mtl         rl.Material
	viewPosLoc  int32
	lightDirLoc int32
	uvScaleLoc  int32 // -1 for the untextured shader
}

// newLitMaterial compiles the shader from source and uploads the lighting constants once; uniform
// values persist on the shader program, so only view position and light direction change per frame.
// If compilation fails, the material keeps raylib's default shader.
func newLitMaterial(fragmentShader string) litMaterial {
	m := litMaterial{mtl: rl.LoadMaterialDefault(), viewPosLoc: -1, lightDirLoc: -1, uvScaleLoc: -1}
	shader := rl.LoadShaderFromMemory(litVS, fragmentShader)
	if !rl.IsShaderValid(shader) {
		return m
	}
	m.mtl.Shader = shader
	m.viewPosLoc = rl.GetShaderLocation(shader, "viewPos")
	m.lightDirLoc = rl.GetShaderLocation(shader, "lightDir")
	m.uvScaleLoc = rl.GetShaderLocation(shader, "uvScale")
	setVec(shader, "ambient", defaultAmbient[:], rl.ShaderUniformVec4)
	setVec(shader, "lightColor", defaultLightColor[:], rl.ShaderUniformVec3)
	setVec(shader, "lightIntensity", []float32{defaultLightIntensity}, rl.ShaderUniformFloat)
	setVec(shader, "specularPower", []float32{defaultSpecularPower}, rl.ShaderUniformFloat)
	setVec(shader, "specularStrength", []float32{defaultSpecularStrength}, rl.ShaderUniformFloat)
	return m
}

// setView uploads the per-frame camera position and light direction.
func (m *litMaterial) setView(viewPos, lightDir [3]float32) {
	if m.viewPosLoc >= 0 {
		rl.SetShaderValueV(m.mtl.Shader, m.viewPosLoc, viewPos[:], rl.ShaderUniformVec3, 1)
	}
	if m.lightDirLoc >= 0 {
		rl.SetShaderValueV(m.mtl.Shader, m.lightDirLoc, lightDir[:], rl.ShaderUniformVec3, 1)
	}
}

// setUVScale sets how many times the albedo texture repeats (textured shader only).
func (m *litMaterial) setUVScale(uv [2]float32) {
	if m.uvScaleLoc >= 0 {
		rl.SetShaderValueV(m.mtl.Shader, m.uvScaleLoc, uv[:], rl.ShaderUniformVec2, 1)
	}
}

func setVec(shader rl.Shader, name string, v []float32, typ rl.ShaderUniformDataType) {
	if loc := rl.GetShaderLocation(shader, name); loc >= 0 {
		rl.SetShaderValueV(shader, loc, v, typ, 1)
	}
}

const (
	// litVS uses the same vertex attributes as raylib meshes: vertexPosition, vertexTexCoord, vertexNormal.
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
	// litFS does directional light + ambient + Blinn-Phong specular. colDiffuse is set by raylib's
	// DrawMesh from the material's albedo color.
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
	// litTexturedFS is litFS with the tint multiplied by the albedo texture (tiled by uvScale).
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
uniform vec2 uvScale;
out vec4 finalColor;
void main() {
  vec2 uv = fragTexCoord * uvScale;
  vec4 texColor = texture(albedoMap, uv);
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
