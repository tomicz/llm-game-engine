package scene

import (
	"os"
	"path/filepath"

	"game-engine/internal/primitives"

	rl "github.com/gen2brain/raylib-go/raylib"
	"gopkg.in/yaml.v3"
)

const (
	gridExtent     = 50
	gridMinorStep  = 1
	gridMajorStep  = 10
	gridMinorAlpha = 50
	gridMajorAlpha = 120
	axisLineAlpha  = 220
	skyboxScale    = 1000
	// Y-drag: world units per pixel (screen-space mouse delta → vertical movement).
	yDragSensitivity = float32(0.015)
	// Gizmo arrows: visual-only length (no picking).
	gizmoArrowLength = float32(1.5)
)

// skyboxPaths are tried in order so the skybox is found whether run from repo root or cmd/game.
// Skybox assets live under assets/skybox/ to keep them separate from other future assets.
var skyboxPaths = []string{
	"assets/skybox/skybox.png",
	"assets/skybox/skybox.jpg",
	"../../assets/skybox/skybox.png",
	"../../assets/skybox/skybox.jpg",
}

// scenePaths are tried in order so the scene YAML is found whether run from repo root or cmd/game.
var scenePaths = []string{
	"assets/scenes/default.yaml",
	"../../assets/scenes/default.yaml",
}

// SceneData is the YAML format for a scene: list of object instances.
type SceneData struct {
	Objects []ObjectInstance `yaml:"objects"`
}

// ObjectInstance describes one object in the scene: type (e.g. cube), position, optional scale.
type ObjectInstance struct {
	Type     string     `yaml:"type"`
	Position [3]float32 `yaml:"position"`
	Scale    [3]float32 `yaml:"scale,omitempty"`
}

// Scene holds a 3D camera and draws the 3D world. Update runs camera logic (e.g. free camera);
// Draw renders between BeginMode3D and EndMode3D. Based on raylib examples/core/core_3d_camera_free.
type Scene struct {
	Camera      rl.Camera3D
	cursorDone  bool
	GridVisible bool
	// Scene objects loaded from YAML; drawn each frame. Not hardcoded.
	sceneData   SceneData
	scenePath   string // path we loaded from; Save writes here (or first scenePaths if never loaded)
	primitives  *primitives.Registry
	// Editor: when terminal is open (cursor visible), user can select and move primitives. -1 = no selection.
	selectedIndex int
	dragging      bool
	// Drag mode from selection box face: 0=none, 1=top/bottom (XZ), 2=side (Y). For Y we use mouse delta.
	dragMode        int
	dragStartObjY   float32
	lastMouseY      int32   // screen Y when Y drag started (total delta from this)
	dragOffsetX     float32 // XZ: offset from object center to click point so drag keeps that point under cursor
	dragOffsetZ     float32
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
	s.primitives = primitives.NewRegistry()
	s.selectedIndex = -1 // no selection until user selects in terminal mode
	s.loadScene()
	s.loadSkybox()
	return s
}

// loadScene reads the scene from the first existing path in scenePaths and unmarshals into sceneData.
// If no file is found or YAML is invalid, sceneData.Objects stays nil (no objects drawn).
func (s *Scene) loadScene() {
	var path string
	for _, p := range scenePaths {
		cleaned := filepath.Clean(p)
		if _, err := os.Stat(cleaned); err == nil {
			path = cleaned
			break
		}
	}
	if path == "" {
		return
	}
	s.scenePath = path
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var sd SceneData
	if err := yaml.Unmarshal(data, &sd); err != nil {
		return
	}
	s.sceneData = sd
}

// AddObject appends an object to the scene. It is drawn on the next frame.
// Use for runtime spawning (e.g. from the spawn command).
func (s *Scene) AddObject(obj ObjectInstance) {
	s.sceneData.Objects = append(s.sceneData.Objects, obj)
}

// AddPrimitive adds a primitive with the given position and scale. Default scale is [1,1,1].
// Position is the center of the primitive.
func (s *Scene) AddPrimitive(typ string, position, scale [3]float32) {
	s.AddObject(ObjectInstance{Type: typ, Position: position, Scale: scale})
}

// SelectedIndex returns the index of the selected object, or -1 if none.
func (s *Scene) SelectedIndex() int {
	return s.selectedIndex
}

// SelectedObject returns the currently selected object and true, or (zero, false) if none.
func (s *Scene) SelectedObject() (ObjectInstance, bool) {
	if s.selectedIndex < 0 || s.selectedIndex >= len(s.sceneData.Objects) {
		return ObjectInstance{}, false
	}
	return s.sceneData.Objects[s.selectedIndex], true
}

// SaveScene writes the current scene (including runtime-spawned objects) to the scene YAML file.
// Uses the path we loaded from, or the first path in scenePaths if none was loaded.
// Returns an error if the file cannot be written.
func (s *Scene) SaveScene() error {
	path := s.scenePath
	if path == "" {
		path = filepath.Clean(scenePaths[0])
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(&s.sceneData)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// NewScene clears all objects from the scene and saves immediately, marking a fresh start.
// The scene file is overwritten with an empty objects list.
func (s *Scene) NewScene() error {
	s.sceneData.Objects = nil
	return s.SaveScene()
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

// objectAABB returns the world-space AABB for a scene object (primitives are centered at position).
// Scale 0 is treated as 1 so we get a valid box.
func objectAABB(obj ObjectInstance) rl.BoundingBox {
	sx, sy, sz := obj.Scale[0], obj.Scale[1], obj.Scale[2]
	if sx == 0 {
		sx = 1
	}
	if sy == 0 {
		sy = 1
	}
	if sz == 0 {
		sz = 1
	}
	half := [3]float32{sx * 0.5, sy * 0.5, sz * 0.5}
	return rl.NewBoundingBox(
		rl.NewVector3(obj.Position[0]-half[0], obj.Position[1]-half[1], obj.Position[2]-half[2]),
		rl.NewVector3(obj.Position[0]+half[0], obj.Position[1]+half[1], obj.Position[2]+half[2]),
	)
}

// rayPlaneY returns the intersection of ray with the horizontal plane Y = planeY.
// Returns (hit point, true) if hit in front of the ray, otherwise (zero, false).
func rayPlaneY(ray rl.Ray, planeY float32) (rl.Vector3, bool) {
	dy := ray.Direction.Y
	if dy > -1e-6 && dy < 1e-6 {
		return rl.Vector3{}, false
	}
	t := (planeY - ray.Position.Y) / dy
	if t < 0 {
		return rl.Vector3{}, false
	}
	hit := rl.Vector3{
		X: ray.Position.X + t*ray.Direction.X,
		Y: ray.Position.Y + t*ray.Direction.Y,
		Z: ray.Position.Z + t*ray.Direction.Z,
	}
	return hit, true
}

// rayPlane returns the intersection of ray with a plane (point + normal). Returns (hit, true) if t >= 0.
func rayPlane(ray rl.Ray, planePoint rl.Vector3, planeNormal rl.Vector3) (rl.Vector3, bool) {
	dn := ray.Direction.X*planeNormal.X + ray.Direction.Y*planeNormal.Y + ray.Direction.Z*planeNormal.Z
	if dn > -1e-6 && dn < 1e-6 {
		return rl.Vector3{}, false
	}
	diffX := planePoint.X - ray.Position.X
	diffY := planePoint.Y - ray.Position.Y
	diffZ := planePoint.Z - ray.Position.Z
	t := (diffX*planeNormal.X + diffY*planeNormal.Y + diffZ*planeNormal.Z) / dn
	if t < 0 {
		return rl.Vector3{}, false
	}
	return rl.Vector3{
		X: ray.Position.X + t*ray.Direction.X,
		Y: ray.Position.Y + t*ray.Direction.Y,
		Z: ray.Position.Z + t*ray.Direction.Z,
	}, true
}

// UpdateEditor runs when the terminal is open (cursor visible). It handles selection and
// movement of scene primitives. terminalBarHeight is the height in pixels of the bar at
// the bottom; mouse events in that area are ignored so the terminal can receive input.
// Drag mode is chosen by which face of the selection box was hit: top/bottom → XZ (forward/sides),
// side faces → Y (up/down). Only scene objects are selectable and movable; skybox and grid are not.
func (s *Scene) UpdateEditor(cursorVisible bool, terminalBarHeight int) {
	if !cursorVisible {
		s.dragging = false
		s.dragMode = 0
		return
	}
	objs := s.sceneData.Objects
	if len(objs) == 0 {
		return
	}
	screenH := int32(rl.GetScreenHeight())
	mouseY := rl.GetMouseY()
	if mouseY >= screenH-int32(terminalBarHeight) {
		s.dragging = false
		s.dragMode = 0
		return
	}
	mousePos := rl.GetMousePosition()
	ray := rl.GetMouseRay(mousePos, s.Camera)

	if rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		s.dragging = false
		s.dragMode = 0
		return
	}

	// Y drag: move object up/down from screen-space mouse delta (total pixels since drag start)
	if s.dragMode == 2 && s.selectedIndex >= 0 && s.selectedIndex < len(objs) {
		obj := &objs[s.selectedIndex]
		deltaPixels := mouseY - s.lastMouseY
		obj.Position[1] = s.dragStartObjY - float32(deltaPixels)*yDragSensitivity
		return
	}

	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		// Box pick: find closest hit and use hit normal to choose drag mode
		bestIdx := -1
		bestDist := float32(1e30)
		var bestHit rl.RayCollision
		for i := range objs {
			box := objectAABB(objs[i])
			hit := rl.GetRayCollisionBox(ray, box)
			if hit.Hit && hit.Distance > 0 && hit.Distance < bestDist {
				bestDist = hit.Distance
				bestIdx = i
				bestHit = hit
			}
		}
		s.selectedIndex = bestIdx
		s.dragging = bestIdx >= 0
		if bestIdx >= 0 {
			// Top or bottom face only when normal is clearly vertical (Y ≈ ±1). All 4 side faces (Y ≈ 0) → Y drag.
			n := bestHit.Normal
			if n.Y > 0.99 || n.Y < -0.99 {
				s.dragMode = 1 // top or bottom: drag on horizontal plane (XZ)
				// Store offset from object center to click point so the clicked point stays under cursor
				s.dragOffsetX = bestHit.Point.X - objs[bestIdx].Position[0]
				s.dragOffsetZ = bestHit.Point.Z - objs[bestIdx].Position[2]
			} else {
				s.dragMode = 2 // any of the 4 side faces: drag up/down (Y)
				s.dragStartObjY = objs[bestIdx].Position[1]
				s.lastMouseY = mouseY // store so total delta = mouseY - lastMouseY each frame
			}
		} else {
			s.dragMode = 0
		}
		return
	}

	// XZ drag (top/bottom face): drag on horizontal plane at object Y, keeping click offset under cursor
	if s.dragMode == 1 && s.dragging && s.selectedIndex >= 0 && s.selectedIndex < len(objs) {
		obj := &objs[s.selectedIndex]
		hit, ok := rayPlaneY(ray, obj.Position[1])
		if ok {
			obj.Position[0] = hit.X - s.dragOffsetX
			obj.Position[2] = hit.Z - s.dragOffsetZ
		}
	}
}

// drawGizmoArrows draws red (X), green (Y), blue (Z) arrows at pos. Visual only; no picking.
func drawGizmoArrows(pos [3]float32) {
	length := gizmoArrowLength
	headSize := length * 0.2
	red := rl.NewColor(220, 80, 80, 255)
	green := rl.NewColor(80, 220, 80, 255)
	blue := rl.NewColor(80, 80, 220, 255)
	base := rl.NewVector3(pos[0], pos[1], pos[2])
	// X
	endX := rl.NewVector3(pos[0]+length, pos[1], pos[2])
	rl.DrawLine3D(base, endX, red)
	rl.DrawLine3D(endX, rl.NewVector3(pos[0]+length-headSize, pos[1], pos[2]+headSize), red)
	rl.DrawLine3D(endX, rl.NewVector3(pos[0]+length-headSize, pos[1], pos[2]-headSize), red)
	rl.DrawLine3D(endX, rl.NewVector3(pos[0]+length-headSize, pos[1]+headSize, pos[2]), red)
	rl.DrawLine3D(endX, rl.NewVector3(pos[0]+length-headSize, pos[1]-headSize, pos[2]), red)
	// Y
	endY := rl.NewVector3(pos[0], pos[1]+length, pos[2])
	rl.DrawLine3D(base, endY, green)
	rl.DrawLine3D(endY, rl.NewVector3(pos[0], pos[1]+length-headSize, pos[2]+headSize), green)
	rl.DrawLine3D(endY, rl.NewVector3(pos[0], pos[1]+length-headSize, pos[2]-headSize), green)
	rl.DrawLine3D(endY, rl.NewVector3(pos[0]+headSize, pos[1]+length-headSize, pos[2]), green)
	rl.DrawLine3D(endY, rl.NewVector3(pos[0]-headSize, pos[1]+length-headSize, pos[2]), green)
	// Z
	endZ := rl.NewVector3(pos[0], pos[1], pos[2]+length)
	rl.DrawLine3D(base, endZ, blue)
	rl.DrawLine3D(endZ, rl.NewVector3(pos[0]+headSize, pos[1], pos[2]+length-headSize), blue)
	rl.DrawLine3D(endZ, rl.NewVector3(pos[0]-headSize, pos[1], pos[2]+length-headSize), blue)
	rl.DrawLine3D(endZ, rl.NewVector3(pos[0], pos[1]+headSize, pos[2]+length-headSize), blue)
	rl.DrawLine3D(endZ, rl.NewVector3(pos[0], pos[1]-headSize, pos[2]+length-headSize), blue)
}

// Draw renders the 3D scene. Call after ClearBackground and before 2D overlay (e.g. terminal).
// Draws skybox first (if loaded), then a Unity-style grid on the XZ plane (Y=0) when GridVisible is true.
// selectionVisible should be true only when terminal is open (editor mode); the selection outline is drawn only then.
func (s *Scene) Draw(selectionVisible bool) {
	s.ensureSkyboxLoaded()
	rl.BeginMode3D(s.Camera)
	if s.skyboxLoaded {
		drawSkybox(s)
	}
	viewPos := [3]float32{s.Camera.Position.X, s.Camera.Position.Y, s.Camera.Position.Z}
	lightDir := [3]float32{0.5, 1, 0.5} // direction to light (above-right), for primitive shading
	s.primitives.SetView(viewPos, lightDir)
	for i, obj := range s.sceneData.Objects {
		s.primitives.Draw(obj.Type, obj.Position, obj.Scale)
		// Outline only in terminal mode and when this object is selected
		if selectionVisible && s.selectedIndex == i {
			box := objectAABB(obj)
			rl.DrawBoundingBox(box, rl.Yellow)
			drawGizmoArrows(obj.Position)
		}
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
