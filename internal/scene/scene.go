package scene

import (
	"os"
	"path/filepath"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	gridExtent  = 50
	gridMinorStep = 1
	gridMajorStep = 10
	gridMinorAlpha = 50
	gridMajorAlpha = 120
	axisLineAlpha  = 220
	skyboxScale    = 1000
)

// skyboxPaths are tried in order so the skybox is found whether run from repo root or cmd/game.
// Skybox assets live under assets/skybox/ to keep them separate from other future assets.
var skyboxPaths = []string{
	"assets/skybox/skybox.png",
	"assets/skybox/skybox.jpg",
	"../../assets/skybox/skybox.png",
	"../../assets/skybox/skybox.jpg",
}

// Scene holds a 3D camera and draws the 3D world. Update runs camera logic (e.g. free camera);
// Draw renders between BeginMode3D and EndMode3D. Based on raylib examples/core/core_3d_camera_free.
type Scene struct {
	Camera      rl.Camera3D
	cursorDone  bool
	GridVisible bool
	// Skybox: optional texture drawn first in 3D mode. Cubemap or equirectangular panorama.
	skyboxTex       rl.Texture2D
	skyboxMesh      rl.Mesh
	skyboxMtl       rl.Material
	skyboxLoaded    bool
	skyboxPending   bool   // true = path known, GPU load deferred until first Draw (after window/GL exists)
	skyboxPath      string // set when pending; used to load texture on first frame
	skyboxEquirect  bool   // true = panorama (2D texture + shader), false = cubemap
	skyboxShader    rl.Shader
	skyboxCamPosLoc int32
	skyboxTexLoc    int32
}

// New returns a scene with a perspective camera looking at the origin.
// Camera: position (10,10,10), target (0,0,0), up (0,1,0), fovy 45°. Grid is visible by default.
// Tries to load skybox from assets/skybox/ (see skyboxPaths); see assets/README.md.
func New() *Scene {
	s := &Scene{}
	s.Camera.Position = rl.NewVector3(10, 10, 10)
	s.Camera.Target = rl.NewVector3(0, 0, 0)
	s.Camera.Up = rl.NewVector3(0, 1, 0)
	s.Camera.Fovy = 45
	s.Camera.Projection = rl.CameraPerspective
	s.GridVisible = true
	s.loadSkybox()
	return s
}

// equirectAspectMin/Max: width/height ratio for equirectangular panorama (typically 2:1).
const equirectAspectMin = 1.8
const equirectAspectMax = 2.2

// loadSkybox finds the skybox file and decides cubemap vs equirect. GPU loading is deferred to
// ensureSkyboxLoaded (called from Draw) so it runs after the window/OpenGL context exists.
func (s *Scene) loadSkybox() {
	var path string
	for _, p := range skyboxPaths {
		cleaned := filepath.Clean(p)
		if _, err := os.Stat(cleaned); err == nil {
			path = cleaned
			break
		}
	}
	if path == "" {
		return
	}
	img := rl.LoadImage(path)
	if img == nil || img.Width <= 0 || img.Height <= 0 {
		return
	}
	aspect := float32(img.Width) / float32(img.Height)
	s.skyboxEquirect = aspect >= equirectAspectMin && aspect <= equirectAspectMax
	rl.UnloadImage(img)

	s.skyboxPath = path
	s.skyboxPending = true
}

// ensureSkyboxLoaded runs the first time we Draw with a pending skybox; it loads GPU resources
// (texture, mesh, material, shader) so that LoadTexture/LoadTextureCubemap run after the window/GL context exists.
func (s *Scene) ensureSkyboxLoaded() {
	if !s.skyboxPending || s.skyboxPath == "" {
		return
	}
	path := s.skyboxPath
	s.skyboxPending = false
	s.skyboxPath = ""

	if !s.skyboxEquirect {
		img := rl.LoadImage(path)
		if img == nil || img.Width <= 0 || img.Height <= 0 {
			return
		}
		s.skyboxTex = rl.LoadTextureCubemap(img, rl.CubemapLayoutAutoDetect)
		rl.UnloadImage(img)
		if !rl.IsTextureValid(s.skyboxTex) {
			return
		}
		s.skyboxMesh = rl.GenMeshCube(1, 1, 1)
		s.skyboxMtl = rl.LoadMaterialDefault()
		rl.SetMaterialTexture(&s.skyboxMtl, rl.MapCubemap, s.skyboxTex)
		s.skyboxLoaded = true
		return
	}

	s.skyboxTex = rl.LoadTexture(path)
	if !rl.IsTextureValid(s.skyboxTex) {
		return
	}
	shader := loadEquirectSkyboxShader()
	if !rl.IsShaderValid(shader) {
		rl.UnloadTexture(s.skyboxTex)
		return
	}
	s.skyboxMesh = rl.GenMeshCube(1, 1, 1)
	s.skyboxMtl = rl.LoadMaterialDefault()
	s.skyboxMtl.Shader = shader
	s.skyboxCamPosLoc = rl.GetShaderLocation(shader, "cameraPosition")
	s.skyboxTexLoc = rl.GetShaderLocation(shader, "skybox")
	s.skyboxShader = shader
	s.skyboxLoaded = true
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

func loadEquirectSkyboxShader() rl.Shader {
	return rl.LoadShaderFromMemory(equirectVS, equirectFS)
}

// SetGridVisible sets whether the editor grid is drawn.
func (s *Scene) SetGridVisible(visible bool) {
	s.GridVisible = visible
}

// Update runs once per frame. Uses raylib UpdateCamera with CameraFree so the user can
// move the camera with mouse (zoom, pan) and keyboard. Cursor is disabled so the mouse
// is captured for camera control.
func (s *Scene) Update() {
	if !s.cursorDone {
		rl.DisableCursor()
		s.cursorDone = true
	}
	rl.UpdateCamera(&s.Camera, rl.CameraFree)
}

// Draw renders the 3D scene. Call after ClearBackground and before 2D overlay (e.g. terminal).
// Draws skybox first (if loaded), then a Unity-style grid on the XZ plane (Y=0) when GridVisible is true.
func (s *Scene) Draw() {
	s.ensureSkyboxLoaded()
	rl.BeginMode3D(s.Camera)
	if s.skyboxLoaded {
		drawSkybox(s)
	}
	if s.GridVisible {
		drawEditorGrid()
	}
	rl.EndMode3D()
}

// drawSkybox draws the skybox as a large cube centered on the camera (cubemap or equirect).
func drawSkybox(s *Scene) {
	rl.DisableDepthMask()
	rl.DisableBackfaceCulling()
	pos := s.Camera.Position
	scale := rl.MatrixScale(skyboxScale, skyboxScale, skyboxScale)
	trans := rl.MatrixTranslate(pos.X, pos.Y, pos.Z)
	transform := rl.MatrixMultiply(scale, trans)
	if s.skyboxEquirect {
		if s.skyboxCamPosLoc >= 0 {
			camPos := []float32{pos.X, pos.Y, pos.Z}
			rl.SetShaderValueV(s.skyboxMtl.Shader, s.skyboxCamPosLoc, camPos, rl.ShaderUniformVec3, 1)
		}
		if s.skyboxTexLoc >= 0 {
			rl.SetShaderValueTexture(s.skyboxMtl.Shader, s.skyboxTexLoc, s.skyboxTex)
		}
	}
	rl.DrawMesh(s.skyboxMesh, s.skyboxMtl, transform)
	rl.EnableBackfaceCulling()
	rl.EnableDepthMask()
}

// drawEditorGrid draws an infinite-style grid on the XZ plane with major/minor lines and axis lines.
// Reuses start/end vectors to avoid per-frame allocations in the hot loop.
func drawEditorGrid() {
	minor := rl.NewColor(128, 128, 128, gridMinorAlpha)
	major := rl.NewColor(160, 160, 160, gridMajorAlpha)
	axisX := rl.NewColor(220, 80, 80, axisLineAlpha)
	axisY := rl.NewColor(80, 220, 80, axisLineAlpha)
	axisZ := rl.NewColor(80, 80, 220, axisLineAlpha)

	var start, end rl.Vector3
	// Grid lines on XZ plane (Y=0): lines along X (varying Z) and along Z (varying X)
	for x := -gridExtent; x <= gridExtent; x += gridMinorStep {
		c := major
		if x%gridMajorStep != 0 {
			c = minor
		}
		start.X, start.Y, start.Z = float32(x), 0, float32(-gridExtent)
		end.X, end.Y, end.Z = float32(x), 0, float32(gridExtent)
		rl.DrawLine3D(start, end, c)
	}
	for z := -gridExtent; z <= gridExtent; z += gridMinorStep {
		c := major
		if z%gridMajorStep != 0 {
			c = minor
		}
		start.X, start.Y, start.Z = float32(-gridExtent), 0, float32(z)
		end.X, end.Y, end.Z = float32(gridExtent), 0, float32(z)
		rl.DrawLine3D(start, end, c)
	}

	// Axis lines through origin (X=red, Y=green, Z=blue)
	start.X, start.Y, start.Z = float32(-gridExtent), 0, 0
	end.X, end.Y, end.Z = float32(gridExtent), 0, 0
	rl.DrawLine3D(start, end, axisX)
	start.X, start.Y, start.Z = 0, float32(-gridExtent), 0
	end.X, end.Y, end.Z = 0, float32(gridExtent), 0
	rl.DrawLine3D(start, end, axisY)
	start.X, start.Y, start.Z = 0, 0, float32(-gridExtent)
	end.X, end.Y, end.Z = 0, 0, float32(gridExtent)
	rl.DrawLine3D(start, end, axisZ)
}
