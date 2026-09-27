package scene

import (
	"fmt"
	"log"

	"game-engine/internal/assets"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	// skyboxScale is the size of the cube drawn around the camera.
	skyboxScale = 1000
	// Images with a width/height ratio in this range are treated as equirectangular panoramas
	// (typically 2:1); anything else is loaded as a cubemap layout.
	equirectAspectMin = 1.8
	equirectAspectMax = 2.2
)

// skybox is an optional background drawn as a large cube around the camera: either a cubemap or
// an equirectangular panorama sampled by view direction.
type skybox struct {
	pending   string // image path waiting to be loaded once the GPU context exists
	loaded    bool
	equirect  bool
	tex       rl.Texture2D
	mesh      rl.Mesh
	mtl       rl.Material
	camPosLoc int32
	texLoc    int32
}

// findDefault queues assets/skybox/skybox.{png,jpg} if present.
func (sb *skybox) findDefault() {
	if p, ok := assets.Find("assets/skybox/skybox.png", "assets/skybox/skybox.jpg"); ok {
		sb.pending = p
	}
}

// SetSkyboxPath replaces the skybox with the image at path. Panoramas (≈2:1) and cubemap layouts
// are both supported. Before the window exists the load is deferred to the first Draw.
func (s *Scene) SetSkyboxPath(path string) error {
	s.sky.unload()
	s.sky.pending = path
	if !rl.IsWindowReady() {
		return nil
	}
	return s.sky.load()
}

// ensureLoaded loads a pending skybox; failures are logged and not retried.
func (sb *skybox) ensureLoaded() {
	if sb.pending == "" {
		return
	}
	if err := sb.load(); err != nil {
		log.Printf("skybox: %v", err)
	}
}

func (sb *skybox) load() error {
	path := sb.pending
	sb.pending = ""
	img := rl.LoadImage(path)
	if img == nil || img.Width <= 0 || img.Height <= 0 {
		return fmt.Errorf("cannot load image %s", path)
	}
	aspect := float32(img.Width) / float32(img.Height)
	sb.equirect = aspect >= equirectAspectMin && aspect <= equirectAspectMax

	if !sb.equirect {
		sb.tex = rl.LoadTextureCubemap(img, rl.CubemapLayoutAutoDetect)
		rl.UnloadImage(img)
		if !rl.IsTextureValid(sb.tex) {
			return fmt.Errorf("%s is neither a 2:1 panorama nor a cubemap layout", path)
		}
		sb.mtl = rl.LoadMaterialDefault()
		rl.SetMaterialTexture(&sb.mtl, rl.MapCubemap, sb.tex)
	} else {
		sb.tex = rl.LoadTextureFromImage(img)
		rl.UnloadImage(img)
		if !rl.IsTextureValid(sb.tex) {
			return fmt.Errorf("cannot create texture from %s", path)
		}
		shader := rl.LoadShaderFromMemory(equirectVS, equirectFS)
		if !rl.IsShaderValid(shader) {
			rl.UnloadTexture(sb.tex)
			return fmt.Errorf("equirect skybox shader failed to compile")
		}
		sb.mtl = rl.LoadMaterialDefault()
		sb.mtl.Shader = shader
		sb.camPosLoc = rl.GetShaderLocation(shader, "cameraPosition")
		sb.texLoc = rl.GetShaderLocation(shader, "skybox")
	}
	sb.mesh = rl.GenMeshCube(1, 1, 1)
	sb.loaded = true
	return nil
}

// unload releases the skybox's GPU resources. UnloadMaterial also frees a non-default shader and
// any texture bound in the material's maps (the cubemap), so only the panorama texture, which is
// bound as a uniform instead, is unloaded separately.
func (sb *skybox) unload() {
	if !sb.loaded {
		return
	}
	if sb.equirect {
		rl.UnloadTexture(sb.tex)
	}
	rl.UnloadMesh(&sb.mesh)
	rl.UnloadMaterial(sb.mtl)
	sb.loaded = false
}

// draw renders the skybox centered on camPos, behind everything (no depth writes).
func (sb *skybox) draw(camPos rl.Vector3) {
	if !sb.loaded {
		return
	}
	rl.DisableDepthMask()
	rl.DisableBackfaceCulling()
	transform := rl.MatrixMultiply(
		rl.MatrixScale(skyboxScale, skyboxScale, skyboxScale),
		rl.MatrixTranslate(camPos.X, camPos.Y, camPos.Z),
	)
	if sb.equirect {
		if sb.camPosLoc >= 0 {
			rl.SetShaderValueV(sb.mtl.Shader, sb.camPosLoc, []float32{camPos.X, camPos.Y, camPos.Z}, rl.ShaderUniformVec3, 1)
		}
		if sb.texLoc >= 0 {
			rl.SetShaderValueTexture(sb.mtl.Shader, sb.texLoc, sb.tex)
		}
	}
	rl.DrawMesh(sb.mesh, sb.mtl, transform)
	rl.EnableBackfaceCulling()
	rl.EnableDepthMask()
}

// Equirectangular skybox shader: samples a 2D panorama by view direction.
const (
	equirectVS = `#version 330
in vec3 vertexPosition;
uniform mat4 matProjection;
uniform mat4 matView;
uniform mat4 matModel;
out vec3 fragWorldPos;
void main() {
  vec4 worldPos = matModel * vec4(vertexPosition, 1.0);
  fragWorldPos = worldPos.xyz;
  gl_Position = matProjection * matView * worldPos;
}
`
	equirectFS = `#version 330
in vec3 fragWorldPos;
out vec4 finalColor;
uniform sampler2D skybox;
uniform vec3 cameraPosition;
void main() {
  vec3 dir = normalize(fragWorldPos - cameraPosition);
  float lon = atan(dir.z, dir.x);
  float lat = asin(clamp(dir.y, -1.0, 1.0));
  float u = lon / 6.28318530718 + 0.5;
  float v = 0.5 - lat / 3.14159265359;
  finalColor = texture(skybox, vec2(u, v));
}
`
)
