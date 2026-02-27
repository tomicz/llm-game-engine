package scene

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"game-engine/internal/physics"
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

// textureBasePaths are tried as prefixes when loading an object's texture path (e.g. run from cmd/game vs repo root).
var textureBasePaths = []string{
	"",
	"assets/textures/",
	"../../assets/textures/",
}

// SceneData is the YAML format for a scene: list of object instances.
type SceneData struct {
	Objects []ObjectInstance `yaml:"objects"`
}

// ObjectInstance describes one object in the scene: type (e.g. cube), position, optional scale.
// Physics: nil or true = falls and collides; false = static (no gravity, still blocks others). Omit in YAML = physics on.
// Texture: optional path to an image file (e.g. assets/textures/downloaded/foo.png); loaded and applied as albedo when set.
// Color: optional RGB tint (0-1). When set, object is drawn with this tint; omit = default material color.
// Name: optional label for reference (e.g. "Tower"); used by delete name <name> and inspector.
// Motion: optional "spin" (rotate Y each frame) or "bob" (oscillate Y); omit = static.
type ObjectInstance struct {
	Type     string     `yaml:"type"`
	Position [3]float32 `yaml:"position"`
	Scale    [3]float32 `yaml:"scale,omitempty"`
	Physics  *bool      `yaml:"physics,omitempty"`
	Texture  string     `yaml:"texture,omitempty"`
	Color    [3]float32 `yaml:"color,omitempty"`    // RGB 0-1; zero = use default
	Name     string     `yaml:"name,omitempty"`
	Motion   string     `yaml:"motion,omitempty"` // "spin" | "bob" | ""
}

// VisibleObject describes one scene object currently in the camera's view.
// Used by camera object-awareness: ObjectsInView and ViewAwareness.
type VisibleObject struct {
	Index         int             // index in scene objects
	Object        ObjectInstance
	Distance      float32         // distance from camera position
	ScreenPos     rl.Vector2      // 2D position on screen (object center)
	DrawPosition  [3]float32      // world position used for drawing (e.g. with motion)
}

// ViewAwareness holds state for camera object-awareness and optional logging.
// When attached to a scene and updated each frame, it can detect when objects
// enter or leave the camera view and call OnEnterView/OnLeaveView.
type ViewAwareness struct {
	// lastVisible is the set of object indices that were in view last frame.
	lastVisible map[int]struct{}
	// OnEnterView is called when an object index enters the camera view (optional).
	OnEnterView func(index int, obj ObjectInstance, distance float32)
	// OnLeaveView is called when an object index leaves the camera view (optional).
	OnLeaveView func(index int, obj ObjectInstance)
	// OnUpdate is called every frame with the current list of visible objects (optional).
	// Use for logging or documenting "what the camera sees" at a given time.
	OnUpdate func(visible []VisibleObject)
}

// NewViewAwarenessWithLogging returns a ViewAwareness that logs enter/leave to the standard logger.
// Optionally set OnUpdate for per-frame "what the camera sees" (can be noisy).
func NewViewAwarenessWithLogging() *ViewAwareness {
	return &ViewAwareness{
		OnEnterView: func(index int, obj ObjectInstance, distance float32) {
			name := obj.Name
			if name == "" {
				name = fmt.Sprintf("#%d", index)
			}
			log.Printf("[camera] enter view: %s (%s) at %.2f", name, obj.Type, distance)
		},
		OnLeaveView: func(index int, obj ObjectInstance) {
			name := obj.Name
			if name == "" {
				name = fmt.Sprintf("#%d", index)
			}
			log.Printf("[camera] leave view: %s (%s)", name, obj.Type)
		},
	}
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
	// 3D physics: AABB bodies in 1:1 with scene objects. Stepped only when terminal is closed (game mode).
	physicsWorld *physics.World
	// textureCache: path -> GPU texture for object albedo. Loaded lazily in Draw when object has Texture set.
	textureCache map[string]rl.Texture2D
	// lightDir: direction to sun for primitive shading. Set by SetLighting(profile).
	lightDir [3]float32
	// lastUndo: one level of undo (add or delete).
	lastUndo *undoRecord
	// viewAwareness: optional camera object-awareness; when set, updated each frame and can log enter/exit.
	viewAwareness *ViewAwareness
}

// getLightDir returns the current light direction (normalized). Used by Draw.
func (s *Scene) getLightDir() [3]float32 {
	if s.lightDir[0] == 0 && s.lightDir[1] == 0 && s.lightDir[2] == 0 {
		return [3]float32{0.5, 1, 0.5}
	}
	return s.lightDir
}

// motionPosition returns the draw position for obj, applying motion (e.g. bob) when set.
func (s *Scene) motionPosition(obj ObjectInstance, index int) [3]float32 {
	pos := obj.Position
	if obj.Motion == "bob" {
		t := float32(rl.GetTime())
		pos[1] += 0.2 * float32(math.Sin(float64(t*2)))
	}
	return pos
}

// New returns a scene with a perspective camera looking at the origin.
// Camera: position (10,10,10), target (0,0,0), up (0,1,0), fovy 45°. Grid is visible by default.
// Tries to load skybox from assets/skybox/ (see skyboxPaths); see assets/README.md.
func New() *Scene {
	s := &Scene{}
	// Slightly off from center so the initial view isn't perfectly symmetric.
	s.Camera.Position = rl.NewVector3(11, 10.5, 9.5)
	s.Camera.Target = rl.NewVector3(0, 0, 0)
	s.Camera.Up = rl.NewVector3(0, 1, 0)
	s.Camera.Fovy = 45
	s.Camera.Projection = rl.CameraPerspective
	s.GridVisible = true
	s.primitives = primitives.NewRegistry()
	s.selectedIndex = -1 // no selection until user selects in terminal mode
	s.physicsWorld = physics.NewWorld()
	s.textureCache = make(map[string]rl.Texture2D)
	s.lightDir = [3]float32{0.5, 1, 0.5}
	s.loadScene()
	s.ensurePhysicsBodies()
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

// planeDefaultScaleY is the default Y scale (height) for plane primitives so they render and collide as a thin slab.
const planeDefaultScaleY = 0.1

// AddPrimitive adds a primitive with the given position and scale. Default scale is [1,1,1].
// Plane uses Y scale 0.1 by default when scale[1] is 1. Position is the center of the primitive. Physics defaults to on.
func (s *Scene) AddPrimitive(typ string, position, scale [3]float32) {
	scale = applyPlaneDefaultScale(typ, scale)
	s.AddObject(ObjectInstance{Type: typ, Position: position, Scale: scale})
}

// AddPrimitiveWithPhysics adds a primitive with the given position, scale, and physics flag.
// color is optional (nil = default material); name and motion can be set via SetSelected* after add.
func (s *Scene) AddPrimitiveWithPhysics(typ string, position, scale [3]float32, physics bool, color *[3]float32) {
	scale = applyPlaneDefaultScale(typ, scale)
	obj := ObjectInstance{Type: typ, Position: position, Scale: scale, Physics: &physics}
	if color != nil {
		obj.Color = *color
	}
	s.AddObject(obj)
}

// applyPlaneDefaultScale returns scale with Y set to planeDefaultScaleY when typ is "plane" and scale[1] is 1.
func applyPlaneDefaultScale(typ string, scale [3]float32) [3]float32 {
	if typ == "plane" && scale[1] == 1 {
		scale[1] = planeDefaultScaleY
	}
	return scale
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

// SetPhysicsForIndex sets whether the object at index has physics (falling/collision) enabled.
// Returns an error if index is out of range. Persist with SaveScene.
func (s *Scene) SetPhysicsForIndex(index int, enabled bool) error {
	if index < 0 || index >= len(s.sceneData.Objects) {
		return fmt.Errorf("object index %d out of range (0..%d)", index, len(s.sceneData.Objects)-1)
	}
	s.sceneData.Objects[index].Physics = &enabled
	return nil
}

// SetSelectedPhysics sets physics on or off for the currently selected object.
// Returns an error if no object is selected.
func (s *Scene) SetSelectedPhysics(enabled bool) error {
	idx := s.SelectedIndex()
	if idx < 0 {
		return fmt.Errorf("no object selected (click an object with terminal open)")
	}
	return s.SetPhysicsForIndex(idx, enabled)
}

// DeleteObjectAtIndex removes the object at index i and the corresponding physics body.
// Adjusts selectedIndex if needed (clears or decrements). Returns error if index out of range.
func (s *Scene) DeleteObjectAtIndex(i int) error {
	objs := s.sceneData.Objects
	if i < 0 || i >= len(objs) {
		return fmt.Errorf("object index %d out of range (0..%d)", i, len(objs)-1)
	}
	s.sceneData.Objects = append(objs[:i], objs[i+1:]...)
	bodies := s.physicsWorld.Bodies
	if i < len(bodies) {
		s.physicsWorld.Bodies = append(bodies[:i], bodies[i+1:]...)
	}
	if s.selectedIndex == i {
		s.selectedIndex = -1
	} else if s.selectedIndex > i {
		s.selectedIndex--
	}
	return nil
}

// DeleteSelected removes the currently selected object. Returns error if none selected.
func (s *Scene) DeleteSelected() error {
	idx := s.SelectedIndex()
	if idx < 0 {
		return fmt.Errorf("no object selected (click an object with terminal open)")
	}
	s.RecordDelete([]ObjectInstance{s.sceneData.Objects[idx]})
	return s.DeleteObjectAtIndex(idx)
}

// DeleteAtCameraLook casts a ray from the camera position through the camera target and removes
// the first object hit. Returns error if no object is hit.
func (s *Scene) DeleteAtCameraLook() error {
	objs := s.sceneData.Objects
	if len(objs) == 0 {
		return fmt.Errorf("no objects in scene")
	}
	dir := rl.Vector3Subtract(s.Camera.Target, s.Camera.Position)
	dir = rl.Vector3Normalize(dir)
	ray := rl.Ray{Position: s.Camera.Position, Direction: dir}
	bestIdx := -1
	bestDist := float32(1e30)
	for i := range objs {
		box := objectAABB(objs[i])
		hit := rl.GetRayCollisionBox(ray, box)
		if hit.Hit && hit.Distance > 0 && hit.Distance < bestDist {
			bestDist = hit.Distance
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return fmt.Errorf("no object in view (camera not looking at any object)")
	}
	s.RecordDelete([]ObjectInstance{s.sceneData.Objects[bestIdx]})
	return s.DeleteObjectAtIndex(bestIdx)
}

// DeleteRandom removes a random object from the scene. Returns error if scene is empty.
func (s *Scene) DeleteRandom() error {
	objs := s.sceneData.Objects
	if len(objs) == 0 {
		return fmt.Errorf("no objects in scene")
	}
	i := rand.Intn(len(objs))
	s.RecordDelete([]ObjectInstance{objs[i]})
	return s.DeleteObjectAtIndex(i)
}

// DeleteVisibleByDescription deletes the closest object in the camera view that matches the given type
// and optional color. typ must be one of: cube, sphere, cylinder, plane. If colorOptional is nil,
// any color matches. If set, the object's Color (0-1 RGB) must be approximately equal (per-channel
// tolerance 0.35); objects with no color set (zero) do not match a color filter.
// Returns error if no matching object is in view.
func (s *Scene) DeleteVisibleByDescription(typ string, colorOptional *[3]float32) error {
	visible := s.ObjectsInView()
	for _, v := range visible {
		if v.Object.Type != typ {
			continue
		}
		if colorOptional != nil {
			c := v.Object.Color
			// Object must have some color set (at least one channel > 0) to match a color request
			if c[0] == 0 && c[1] == 0 && c[2] == 0 {
				continue
			}
			const tol = 0.35
			if math.Abs(float64(c[0]-colorOptional[0])) > tol ||
				math.Abs(float64(c[1]-colorOptional[1])) > tol ||
				math.Abs(float64(c[2]-colorOptional[2])) > tol {
				continue
			}
		}
		s.RecordDelete([]ObjectInstance{s.sceneData.Objects[v.Index]})
		return s.DeleteObjectAtIndex(v.Index)
	}
	if colorOptional != nil {
		return fmt.Errorf("no %s with that color in view (look at the object and try again)", typ)
	}
	return fmt.Errorf("no %s in view (look at the object and try again)", typ)
}

// visibleMatchFilters returns visible objects that match type (or any if typ empty), optional color, and optional name substring.
func visibleMatchFilters(visible []VisibleObject, typ string, colorOptional *[3]float32, nameSubstring string) []VisibleObject {
	primTypes := map[string]bool{"cube": true, "sphere": true, "cylinder": true, "plane": true}
	if typ != "" && !primTypes[typ] {
		return nil
	}
	nameLower := strings.ToLower(nameSubstring)
	var out []VisibleObject
	for _, v := range visible {
		if typ != "" && v.Object.Type != typ {
			continue
		}
		if colorOptional != nil {
			c := v.Object.Color
			if c[0] == 0 && c[1] == 0 && c[2] == 0 {
				continue
			}
			const tol = 0.35
			if math.Abs(float64(c[0]-colorOptional[0])) > tol ||
				math.Abs(float64(c[1]-colorOptional[1])) > tol ||
				math.Abs(float64(c[2]-colorOptional[2])) > tol {
				continue
			}
		}
		if nameLower != "" {
			if !strings.Contains(strings.ToLower(v.Object.Name), nameLower) {
				continue
			}
		}
		out = append(out, v)
	}
	return out
}

// visiblePickByPosition returns the one visible object at the given position: left (min X), right (max X),
// top (min Y), bottom (max Y), closest (min distance), farthest (max distance). Empty string = closest.
func visiblePickByPosition(visible []VisibleObject, position string) (VisibleObject, bool) {
	if len(visible) == 0 {
		return VisibleObject{}, false
	}
	pos := strings.ToLower(strings.TrimSpace(position))
	if pos == "" {
		pos = "closest"
	}
	best := visible[0]
	switch pos {
	case "left":
		for _, v := range visible[1:] {
			if v.ScreenPos.X < best.ScreenPos.X {
				best = v
			}
		}
	case "right":
		for _, v := range visible[1:] {
			if v.ScreenPos.X > best.ScreenPos.X {
				best = v
			}
		}
	case "top":
		for _, v := range visible[1:] {
			if v.ScreenPos.Y < best.ScreenPos.Y {
				best = v
			}
		}
	case "bottom":
		for _, v := range visible[1:] {
			if v.ScreenPos.Y > best.ScreenPos.Y {
				best = v
			}
		}
	case "closest":
		for _, v := range visible[1:] {
			if v.Distance < best.Distance {
				best = v
			}
		}
	case "farthest":
		for _, v := range visible[1:] {
			if v.Distance > best.Distance {
				best = v
			}
		}
	default:
		return VisibleObject{}, false
	}
	return best, true
}

// DeleteVisibleByPosition deletes the one visible object at the given position (left, right, top, bottom, closest, farthest).
func (s *Scene) DeleteVisibleByPosition(position string) error {
	visible := s.ObjectsInView()
	best, ok := visiblePickByPosition(visible, position)
	if !ok {
		return fmt.Errorf("no objects in view")
	}
	s.RecordDelete([]ObjectInstance{s.sceneData.Objects[best.Index]})
	return s.DeleteObjectAtIndex(best.Index)
}

// DeleteVisibleByDescriptionAndPosition deletes the visible object matching type/color/name and at the given position.
// typ can be "" for any type. nameSubstring "" = any name. position: left, right, top, bottom, closest, farthest, or "" for closest.
func (s *Scene) DeleteVisibleByDescriptionAndPosition(typ string, colorOptional *[3]float32, nameSubstring string, position string) error {
	visible := s.ObjectsInView()
	filtered := visibleMatchFilters(visible, typ, colorOptional, nameSubstring)
	best, ok := visiblePickByPosition(filtered, position)
	if !ok {
		if typ != "" && nameSubstring != "" {
			return fmt.Errorf("no %q matching %q in view", typ, nameSubstring)
		}
		if typ != "" {
			return fmt.Errorf("no %s in view", typ)
		}
		return fmt.Errorf("no matching object in view")
	}
	s.RecordDelete([]ObjectInstance{s.sceneData.Objects[best.Index]})
	return s.DeleteObjectAtIndex(best.Index)
}

// DeleteAllVisibleByDescription deletes all visible objects matching type (or any if ""), optional color, and optional name substring.
// Returns the number deleted. typ "" means any type; nameSubstring "" means any name.
func (s *Scene) DeleteAllVisibleByDescription(typ string, colorOptional *[3]float32, nameSubstring string) (int, error) {
	visible := s.ObjectsInView()
	filtered := visibleMatchFilters(visible, typ, colorOptional, nameSubstring)
	if len(filtered) == 0 {
		if nameSubstring != "" {
			return 0, fmt.Errorf("no objects matching %q in view", nameSubstring)
		}
		return 0, fmt.Errorf("no matching objects in view")
	}
	// Delete in reverse index order so indices remain valid
	indices := make([]int, len(filtered))
	for i, v := range filtered {
		indices[i] = v.Index
	}
	sort.Sort(sort.Reverse(sort.IntSlice(indices)))
	for _, idx := range indices {
		s.RecordDelete([]ObjectInstance{s.sceneData.Objects[idx]})
		_ = s.DeleteObjectAtIndex(idx)
	}
	return len(indices), nil
}

// ClearSelection clears any selected object in the scene.
func (s *Scene) ClearSelection() {
	s.selectedIndex = -1
}

// SelectVisibleByPosition selects the one visible object at the given position (left, right, top, bottom, closest, farthest).
func (s *Scene) SelectVisibleByPosition(position string) error {
	visible := s.ObjectsInView()
	best, ok := visiblePickByPosition(visible, position)
	if !ok {
		return fmt.Errorf("no objects in view")
	}
	s.selectedIndex = best.Index
	return nil
}

// SelectVisibleByDescriptionAndPosition selects the visible object matching type/color/name and at the given position.
// typ can be "" for any type. nameSubstring "" = any name. position: left, right, top, bottom, closest, farthest, or "" for closest.
func (s *Scene) SelectVisibleByDescriptionAndPosition(typ string, colorOptional *[3]float32, nameSubstring string, position string) error {
	visible := s.ObjectsInView()
	filtered := visibleMatchFilters(visible, typ, colorOptional, nameSubstring)
	best, ok := visiblePickByPosition(filtered, position)
	if !ok {
		if typ != "" && nameSubstring != "" {
			return fmt.Errorf("no %q matching %q in view", typ, nameSubstring)
		}
		if typ != "" {
			return fmt.Errorf("no %s in view", typ)
		}
		if nameSubstring != "" {
			return fmt.Errorf("no objects matching %q in view", nameSubstring)
		}
		return fmt.Errorf("no matching object in view")
	}
	s.selectedIndex = best.Index
	return nil
}

// FocusOnVisibleByPosition points the camera target at the visible object at the given position
// (left, right, top, bottom, closest, farthest) without changing the camera position.
func (s *Scene) FocusOnVisibleByPosition(position string) error {
	visible := s.ObjectsInView()
	best, ok := visiblePickByPosition(visible, position)
	if !ok {
		return fmt.Errorf("no objects in view")
	}
	obj := s.sceneData.Objects[best.Index]
	s.Camera.Target = rl.NewVector3(obj.Position[0], obj.Position[1], obj.Position[2])
	return nil
}

// FocusOnVisibleByDescriptionAndPosition points the camera target at the visible object matching
// type/color/name and at the given position. typ/nameSubstring semantics match SelectVisibleByDescriptionAndPosition.
func (s *Scene) FocusOnVisibleByDescriptionAndPosition(typ string, colorOptional *[3]float32, nameSubstring string, position string) error {
	visible := s.ObjectsInView()
	filtered := visibleMatchFilters(visible, typ, colorOptional, nameSubstring)
	best, ok := visiblePickByPosition(filtered, position)
	if !ok {
		if typ != "" && nameSubstring != "" {
			return fmt.Errorf("no %q matching %q in view", typ, nameSubstring)
		}
		if typ != "" {
			return fmt.Errorf("no %s in view", typ)
		}
		if nameSubstring != "" {
			return fmt.Errorf("no objects matching %q in view", nameSubstring)
		}
		return fmt.Errorf("no matching object in view")
	}
	obj := s.sceneData.Objects[best.Index]
	s.Camera.Target = rl.NewVector3(obj.Position[0], obj.Position[1], obj.Position[2])
	return nil
}

// GetViewContextSummary returns a short text summary of what the camera currently sees, for the LLM.
// Format: "Visible (left to right): 1. cube 'Tower' (left), 2. plane (center), 3. sphere (right)."
func (s *Scene) GetViewContextSummary() string {
	visible := s.ObjectsInView()
	if len(visible) == 0 {
		return "No objects in view."
	}
	// Sort by screen X for left-to-right order
	byX := make([]VisibleObject, len(visible))
	copy(byX, visible)
	sort.Slice(byX, func(i, j int) bool { return byX[i].ScreenPos.X < byX[j].ScreenPos.X })
	minX, maxX := byX[0].ScreenPos.X, byX[len(byX)-1].ScreenPos.X
	midX := (minX + maxX) / 2
	var parts []string
	for i, v := range byX {
		posLabel := "center"
		if maxX > minX {
			if v.ScreenPos.X < midX-20 {
				posLabel = "left"
			} else if v.ScreenPos.X > midX+20 {
				posLabel = "right"
			}
		}
		name := v.Object.Name
		if name == "" {
			name = v.Object.Type
		} else {
			name = fmt.Sprintf("%q (%s)", name, v.Object.Type)
		}
		parts = append(parts, fmt.Sprintf("%d. %s (%s)", i+1, name, posLabel))
	}
	return "Visible (left to right): " + strings.Join(parts, ", ") + "."
}

// EnsureTexture loads and caches a texture from path. Path is tried as-is and with textureBasePaths.
// Returns the texture and true if loaded or already cached; (zero, false) if path is empty or load failed.
// Safe to call from Draw (loads on first use when GL context exists).
func (s *Scene) EnsureTexture(path string) (rl.Texture2D, bool) {
	if path == "" {
		return rl.Texture2D{}, false
	}
	if tex, ok := s.textureCache[path]; ok && rl.IsTextureValid(tex) {
		return tex, true
	}
	var fullPath string
	for _, base := range textureBasePaths {
		candidate := filepath.Join(base, path)
		if base == "" {
			candidate = path
		}
		candidate = filepath.Clean(candidate)
		if _, err := os.Stat(candidate); err == nil {
			fullPath = candidate
			break
		}
	}
	if fullPath == "" {
		// path as-is (absolute or cwd-relative)
		if _, err := os.Stat(path); err == nil {
			fullPath = filepath.Clean(path)
		}
	}
	if fullPath == "" {
		return rl.Texture2D{}, false
	}
	tex := rl.LoadTexture(fullPath)
	if !rl.IsTextureValid(tex) {
		return rl.Texture2D{}, false
	}
	s.textureCache[path] = tex
	return tex, true
}

// SetSelectedTexture sets the texture path on the currently selected object. Path is stored as-is (e.g. assets/textures/downloaded/foo.png).
// Returns an error if no object is selected.
func (s *Scene) SetSelectedTexture(path string) error {
	idx := s.SelectedIndex()
	if idx < 0 {
		return fmt.Errorf("no object selected (click an object with terminal open)")
	}
	s.sceneData.Objects[idx].Texture = path
	return nil
}

// SetObjectTexture sets the texture path on the object at the given index. Used when a background download completes.
func (s *Scene) SetObjectTexture(index int, path string) error {
	if index < 0 || index >= len(s.sceneData.Objects) {
		return fmt.Errorf("object index out of range")
	}
	s.sceneData.Objects[index].Texture = path
	return nil
}

// SetSelectedColor sets the RGB color (0-1) on the currently selected object.
func (s *Scene) SetSelectedColor(c [3]float32) error {
	idx := s.SelectedIndex()
	if idx < 0 {
		return fmt.Errorf("no object selected")
	}
	s.sceneData.Objects[idx].Color = c
	return nil
}

// SetSelectedName sets the name on the currently selected object.
func (s *Scene) SetSelectedName(name string) error {
	idx := s.SelectedIndex()
	if idx < 0 {
		return fmt.Errorf("no object selected")
	}
	s.sceneData.Objects[idx].Name = name
	return nil
}

// SetSelectedMotion sets motion on the selected object ("", "spin", "bob").
func (s *Scene) SetSelectedMotion(motion string) error {
	idx := s.SelectedIndex()
	if idx < 0 {
		return fmt.Errorf("no object selected")
	}
	s.sceneData.Objects[idx].Motion = motion
	return nil
}

// SetLighting sets the directional light from a profile: "noon" (default), "sunset", "night".
func (s *Scene) SetLighting(profile string) {
	switch profile {
	case "sunset":
		s.lightDir = [3]float32{0.8, 0.3, 0.2} // warm, low
	case "night":
		s.lightDir = [3]float32{-0.3, 0.5, -0.5} // dim, blue-ish
	default:
		s.lightDir = [3]float32{0.5, 1, 0.5} // noon
	}
}

// DuplicateSelected clones the selected object n times with a small position offset. Returns count duplicated.
func (s *Scene) DuplicateSelected(n int, offset [3]float32) (int, error) {
	idx := s.SelectedIndex()
	if idx < 0 {
		return 0, fmt.Errorf("no object selected")
	}
	if n <= 0 {
		return 0, nil
	}
	if n > 20 {
		n = 20
	}
	obj := s.sceneData.Objects[idx]
	for i := 0; i < n; i++ {
		clone := obj
		clone.Position[0] += offset[0] * float32(i+1)
		clone.Position[1] += offset[1] * float32(i+1)
		clone.Position[2] += offset[2] * float32(i+1)
		clone.Name = "" // avoid duplicate names
		s.sceneData.Objects = append(s.sceneData.Objects, clone)
	}
	s.syncSceneToPhysics()
	return n, nil
}

// undoRecord holds one level of undo (either added indices or deleted objects).
type undoRecord struct {
	addCount    int              // last N objects added at end of list
	deletedObjs []ObjectInstance // objects that were deleted
}

// RecordAdd records that count objects were just added at the end (for undo).
func (s *Scene) RecordAdd(count int) {
	if count <= 0 {
		return
	}
	s.lastUndo = &undoRecord{addCount: count}
}

// RecordDelete records the given objects as deleted (for undo). Call before actually removing them.
func (s *Scene) RecordDelete(objs []ObjectInstance) {
	if len(objs) == 0 {
		return
	}
	s.lastUndo = &undoRecord{deletedObjs: objs}
}

// Undo reverts the last add or delete. Returns nil on success.
func (s *Scene) Undo() error {
	if s.lastUndo == nil {
		return fmt.Errorf("nothing to undo")
	}
	if s.lastUndo.addCount > 0 {
		n := len(s.sceneData.Objects) - s.lastUndo.addCount
		if n < 0 {
			n = 0
		}
		s.sceneData.Objects = s.sceneData.Objects[:n]
		s.syncSceneToPhysics()
		if s.selectedIndex >= len(s.sceneData.Objects) {
			s.selectedIndex = len(s.sceneData.Objects) - 1
		}
	} else {
		s.sceneData.Objects = append(s.sceneData.Objects, s.lastUndo.deletedObjs...)
		s.syncSceneToPhysics()
	}
	s.lastUndo = nil
	return nil
}

// SetGravity sets the physics world gravity vector (e.g. [0, -9.8, 0] for down).
func (s *Scene) SetGravity(g [3]float32) {
	s.physicsWorld.SetGravity(g)
}

// FocusOnSelected sets the camera target to the selected object's position.
func (s *Scene) FocusOnSelected() error {
	idx := s.SelectedIndex()
	if idx < 0 {
		return fmt.Errorf("no object selected")
	}
	obj := s.sceneData.Objects[idx]
	s.Camera.Target = rl.NewVector3(obj.Position[0], obj.Position[1], obj.Position[2])
	return nil
}

// DeleteByName removes the first object whose name matches. Returns true if one was removed.
func (s *Scene) DeleteByName(name string) (bool, error) {
	if name == "" {
		return false, fmt.Errorf("name is required")
	}
	for i := range s.sceneData.Objects {
		if s.sceneData.Objects[i].Name == name {
			s.RecordDelete([]ObjectInstance{s.sceneData.Objects[i]})
			return true, s.DeleteObjectAtIndex(i)
		}
	}
	return false, fmt.Errorf("no object named %q", name)
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
// The scene file is overwritten with an empty objects list. Physics bodies are cleared.
func (s *Scene) NewScene() error {
	s.sceneData.Objects = nil
	s.physicsWorld.Bodies = nil
	return s.SaveScene()
}

// equirectAspectMin/Max: width/height ratio for equirectangular panorama (typically 2:1).
const equirectAspectMin = 1.8
const equirectAspectMax = 2.2

// loadSkybox finds the skybox file from skyboxPaths. GPU loading and equirect detection are deferred to
// ensureSkyboxLoaded (called from Draw) so they run after the window/OpenGL context exists.
func (s *Scene) loadSkybox() {
	for _, p := range skyboxPaths {
		cleaned := filepath.Clean(p)
		if _, err := os.Stat(cleaned); err == nil {
			s.skyboxPath = cleaned
			s.skyboxPending = true
			return
		}
	}
}

// ensureSkyboxLoaded runs the first time we Draw with a pending skybox; it loads GPU resources
// (texture, mesh, material, shader) so that LoadTexture/LoadTextureCubemap run after the window/GL context exists.
// Only clears pending/path on success so a failed load (e.g. GL not ready on first frame) will retry next frame.
// Detects equirect vs cubemap from image aspect ratio when loading from a dynamically set path.
func (s *Scene) ensureSkyboxLoaded() {
	if !s.skyboxPending || s.skyboxPath == "" {
		return
	}
	path := s.skyboxPath
	img := rl.LoadImage(path)
	if img == nil || img.Width <= 0 || img.Height <= 0 {
		return
	}
	aspect := float32(img.Width) / float32(img.Height)
	s.skyboxEquirect = aspect >= equirectAspectMin && aspect <= equirectAspectMax

	if !s.skyboxEquirect {
		s.skyboxTex = rl.LoadTextureCubemap(img, rl.CubemapLayoutAutoDetect)
		rl.UnloadImage(img)
		if !rl.IsTextureValid(s.skyboxTex) {
			return
		}
		s.skyboxMesh = rl.GenMeshCube(1, 1, 1)
		s.skyboxMtl = rl.LoadMaterialDefault()
		rl.SetMaterialTexture(&s.skyboxMtl, rl.MapCubemap, s.skyboxTex)
		s.skyboxPending = false
		s.skyboxPath = ""
		s.skyboxLoaded = true
		return
	}

	rl.UnloadImage(img)
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
	s.skyboxPending = false
	s.skyboxPath = ""
	s.skyboxLoaded = true
}

// UnloadSkybox releases GPU resources for the current skybox. Call before setting a new skybox path.
// UnloadMaterial unloads the material's attached shader, so we must not call UnloadShader separately (double-free).
func (s *Scene) UnloadSkybox() {
	if !s.skyboxLoaded {
		return
	}
	rl.UnloadTexture(s.skyboxTex)
	rl.UnloadMesh(&s.skyboxMesh)
	rl.UnloadMaterial(s.skyboxMtl)
	s.skyboxLoaded = false
}

// SetSkyboxPath sets the skybox to the given image path (e.g. from a downloaded file). Loads in the next Draw.
// Supports equirectangular panoramas (2:1 aspect) and cubemaps. Call UnloadSkybox is not required; SetSkyboxPath unloads the current skybox first.
func (s *Scene) SetSkyboxPath(path string) {
	s.UnloadSkybox()
	s.skyboxPath = path
	s.skyboxPending = true
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

// ensurePhysicsBodies keeps physics world bodies in 1:1 with scene objects. Adds bodies for new objects.
// Static = physics disabled (no fall); dynamic = physics enabled (falls, collides). Scale 0 is treated as 1.
func (s *Scene) ensurePhysicsBodies() {
	objs := s.sceneData.Objects
	for len(s.physicsWorld.Bodies) < len(objs) {
		i := len(s.physicsWorld.Bodies)
		obj := objs[i]
		scale := scaleForPhysicsBody(obj)
		static := !physicsEnabled(obj)
		s.physicsWorld.AddBody(physics.NewBody(obj.Position, scale, 1, static))
	}
}

// physicsEnabled returns true if the object should be simulated (fall, collide). nil or true = on; false = off.
func physicsEnabled(obj ObjectInstance) bool {
	if obj.Physics == nil {
		return true
	}
	return *obj.Physics
}

// PhysicsEnabledForObject returns true if the given object has physics (falling/collision) enabled.
// Used by the inspector and callers that need to display or reason about physics state.
func PhysicsEnabledForObject(obj ObjectInstance) bool {
	return physicsEnabled(obj)
}

// scaleForPhysics returns scale with zeros replaced by 1 so AABB is valid.
func scaleForPhysics(s [3]float32) [3]float32 {
	out := s
	if out[0] == 0 {
		out[0] = 1
	}
	if out[1] == 0 {
		out[1] = 1
	}
	if out[2] == 0 {
		out[2] = 1
	}
	return out
}

// scaleForPhysicsBody returns the scale used for the physics AABB. Planes use Y = planeDefaultScaleY (0.1) for a thin collider.
func scaleForPhysicsBody(obj ObjectInstance) [3]float32 {
	s := scaleForPhysics(obj.Scale)
	if obj.Type == "plane" {
		s[1] = planeDefaultScaleY
	}
	return s
}

// syncSceneToPhysics copies each scene object's position, scale, and physics flag into the corresponding physics body.
func (s *Scene) syncSceneToPhysics() {
	bodies := s.physicsWorld.Bodies
	objs := s.sceneData.Objects
	for i := 0; i < len(bodies) && i < len(objs); i++ {
		bodies[i].Position = objs[i].Position
		bodies[i].Scale = scaleForPhysicsBody(objs[i])
		bodies[i].Static = !physicsEnabled(objs[i])
	}
}

// syncPhysicsToScene copies dynamic body positions back to scene objects.
func (s *Scene) syncPhysicsToScene() {
	bodies := s.physicsWorld.Bodies
	objs := s.sceneData.Objects
	for i := 0; i < len(bodies) && i < len(objs); i++ {
		if !bodies[i].Static {
			objs[i].Position = bodies[i].Position
		}
	}
}

// Update runs once per frame. Uses raylib UpdateCamera with CameraFree so the user can
// move the camera with mouse (zoom, pan) and keyboard. Cursor is disabled so the mouse
// is captured for camera control. When terminal is closed (game mode), runs 3D physics:
// sync scene→bodies, Step(dt), sync bodies→scene.
func (s *Scene) Update() {
	if !s.cursorDone {
		rl.DisableCursor()
		s.cursorDone = true
	}
	rl.UpdateCamera(&s.Camera, rl.CameraFree)
	s.ensurePhysicsBodies()
	s.syncSceneToPhysics()
	s.physicsWorld.Step(rl.GetFrameTime())
	s.syncPhysicsToScene()
	s.UpdateViewAwareness()
}

// objectAABB returns the world-space AABB for a scene object (primitives are centered at position).
func objectAABB(obj ObjectInstance) rl.BoundingBox {
	return objectAABBAt(obj, obj.Position)
}

// objectAABBAt returns the AABB for obj using the given center position (e.g. with motion applied).
func objectAABBAt(obj ObjectInstance, pos [3]float32) rl.BoundingBox {
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
		rl.NewVector3(pos[0]-half[0], pos[1]-half[1], pos[2]-half[2]),
		rl.NewVector3(pos[0]+half[0], pos[1]+half[1], pos[2]+half[2]),
	)
}

// ObjectsInView returns all scene objects currently visible to the camera:
// in front of the camera and with their center projected inside the screen bounds.
// Results are sorted by distance (closest first). Uses current camera and screen size.
func (s *Scene) ObjectsInView() []VisibleObject {
	objs := s.sceneData.Objects
	if len(objs) == 0 {
		return nil
	}
	camPos := s.Camera.Position
	forward := rl.Vector3Subtract(s.Camera.Target, camPos)
	forward = rl.Vector3Normalize(forward)
	w := float32(rl.GetScreenWidth())
	h := float32(rl.GetScreenHeight())
	const inFrontEpsilon = 0.01

	var out []VisibleObject
	for i := range objs {
		obj := objs[i]
		drawPos := s.motionPosition(obj, i)
		center := rl.NewVector3(drawPos[0], drawPos[1], drawPos[2])
		toCenter := rl.Vector3Subtract(center, camPos)
		dist := rl.Vector3Length(toCenter)
		if dist < 1e-6 {
			continue
		}
		dirToCenter := rl.Vector3Scale(toCenter, 1/dist)
		if rl.Vector3DotProduct(dirToCenter, forward) < inFrontEpsilon {
			continue // behind or to the side (outside view cone)
		}
		screen := rl.GetWorldToScreen(center, s.Camera)
		if screen.X < 0 || screen.X > w || screen.Y < 0 || screen.Y > h {
			continue
		}
		out = append(out, VisibleObject{
			Index:        i,
			Object:       obj,
			Distance:     dist,
			ScreenPos:    screen,
			DrawPosition: drawPos,
		})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Distance < out[b].Distance })
	return out
}

// EnableViewAwareness attaches a ViewAwareness to the scene. It will be updated each frame in Update.
// Pass nil to disable. The caller can set OnEnterView, OnLeaveView, OnUpdate on the provided awareness.
func (s *Scene) EnableViewAwareness(a *ViewAwareness) {
	s.viewAwareness = a
}

// UpdateViewAwareness updates view-awareness state and invokes enter/leave/update callbacks.
// Called automatically from Update when viewAwareness is set.
func (s *Scene) UpdateViewAwareness() {
	if s.viewAwareness == nil {
		return
	}
	visible := s.ObjectsInView()
	cur := make(map[int]struct{})
	for _, v := range visible {
		cur[v.Index] = struct{}{}
	}
	last := s.viewAwareness.lastVisible
	// Enter: in cur but not in last
	for idx := range cur {
		if last == nil {
			break
		}
		if _, was := last[idx]; was {
			continue
		}
		for _, v := range visible {
			if v.Index == idx {
				if s.viewAwareness.OnEnterView != nil {
					s.viewAwareness.OnEnterView(idx, v.Object, v.Distance)
				}
				break
			}
		}
	}
	// Leave: in last but not in cur
	if last != nil {
		for idx := range last {
			if _, now := cur[idx]; now {
				continue
			}
			objs := s.sceneData.Objects
			if idx >= 0 && idx < len(objs) && s.viewAwareness.OnLeaveView != nil {
				s.viewAwareness.OnLeaveView(idx, objs[idx])
			}
		}
	}
	s.viewAwareness.lastVisible = cur
	if s.viewAwareness.OnUpdate != nil {
		s.viewAwareness.OnUpdate(visible)
	}
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
	lightDir := s.getLightDir()
	s.primitives.SetView(viewPos, lightDir)
	for i, obj := range s.sceneData.Objects {
		drawPos := s.motionPosition(obj, i)
		var tint *[4]float32
		if obj.Color[0] != 0 || obj.Color[1] != 0 || obj.Color[2] != 0 {
			t := [4]float32{obj.Color[0], obj.Color[1], obj.Color[2], 1}
			tint = &t
		}
		if obj.Texture != "" {
			if tex, ok := s.EnsureTexture(obj.Texture); ok {
				s.primitives.DrawWithTexture(obj.Type, drawPos, obj.Scale, tex, tint)
			} else {
				s.primitives.Draw(obj.Type, drawPos, obj.Scale, tint)
			}
		} else {
			s.primitives.Draw(obj.Type, drawPos, obj.Scale, tint)
		}
		// Outline only in terminal mode and when this object is selected
		if selectionVisible && s.selectedIndex == i {
			box := objectAABBAt(obj, drawPos)
			rl.DrawBoundingBox(box, rl.Yellow)
			drawGizmoArrows(drawPos)
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
