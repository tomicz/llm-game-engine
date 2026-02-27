package main

import (
	"context"
	"flag"
	"fmt"
	"game-engine/internal/agent"
	"game-engine/internal/commands"
	"game-engine/internal/debug"
	"game-engine/internal/download"
	"game-engine/internal/engineconfig"
	"game-engine/internal/env"
	"game-engine/internal/fonts"
	"game-engine/internal/graphics"
	"game-engine/internal/googlefonts"
	"game-engine/internal/llm"
	"game-engine/internal/logger"
	"game-engine/internal/scene"
	"game-engine/internal/terminal"
	"game-engine/internal/ui"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tomicz/speak-to-agent/vttlib"
	rl "github.com/gen2brain/raylib-go/raylib"
)

func main() {
	// Load .env from repo root or cmd/game so API keys are available
	_ = env.Load(".env")
	_ = env.Load("../../.env")

	// Optional: enable heap/CPU profiling. Run with DEBUG_PPROF=1, then e.g. go tool pprof -http=:8080 http://localhost:6060/debug/pprof/heap
	if os.Getenv("DEBUG_PPROF") == "1" {
		go func() { _ = http.ListenAndServe("localhost:6060", nil) }()
	}

	log := logger.New()
	rl.SetTraceLogCallback(log.LogEngine) // capture raylib INFO/WARNING/ERROR to engine_log.txt

	scn := scene.New()
	dbg := debug.New()
	reg := commands.NewRegistry()

	// Optional: camera object-awareness — log when objects enter/leave view. Set CAMERA_AWARENESS=1 to enable.
	if os.Getenv("CAMERA_AWARENESS") == "1" {
		scn.EnableViewAwareness(scene.NewViewAwarenessWithLogging())
	}

	// Apply persisted engine prefs (debug overlays, grid, AI model). Save on every toggle.
	prefs, _ := engineconfig.Load()
	dbg.SetShowFPS(prefs.ShowFPS)
	dbg.SetShowMemAlloc(prefs.ShowMemAlloc)
	scn.SetGridVisible(prefs.GridVisible)
	currentAIModel := prefs.AIModel
	if currentAIModel == "" {
		currentAIModel = "gpt-4o-mini"
	}
	currentFontPath := prefs.Font
	if currentFontPath == "" {
		currentFontPath = "Roboto/static/Roboto-Regular.ttf"
	}
	saveEnginePrefs := func() {
		_ = engineconfig.Save(engineconfig.EnginePrefs{
			ShowFPS:      dbg.ShowFPS,
			ShowMemAlloc: dbg.ShowMemAlloc,
			GridVisible:  scn.GridVisible,
			AIModel:      currentAIModel,
			Font:         currentFontPath,
		})
	}
	// If only Groq is configured, default to a Groq model so natural language works without cmd model.
	if os.Getenv("GROQ_API_KEY") != "" && (currentAIModel == "" || currentAIModel == "gpt-4o-mini") {
		currentAIModel = "llama-3.3-70b-versatile"
		saveEnginePrefs()
	}

	// grid: --show / --hide to show or hide the editor grid
	var showGrid, hideGrid bool
	gridFS := flag.NewFlagSet("grid", flag.ContinueOnError)
	gridFS.BoolVar(&showGrid, "show", false, "show grid")
	gridFS.BoolVar(&hideGrid, "hide", false, "hide grid")
	reg.Register("grid", gridFS, func() error {
		s, h := showGrid, hideGrid
		showGrid, hideGrid = false, false
		if s {
			scn.SetGridVisible(true)
		}
		if h {
			scn.SetGridVisible(false)
		}
		saveEnginePrefs()
		return nil
	})

	// fps: --show / --hide to show or hide the FPS counter (top-right, green). Part of debugging; hidden by default.
	var showFPS, hideFPS bool
	fpsFS := flag.NewFlagSet("fps", flag.ContinueOnError)
	fpsFS.BoolVar(&showFPS, "show", false, "show FPS")
	fpsFS.BoolVar(&hideFPS, "hide", false, "hide FPS")
	reg.Register("fps", fpsFS, func() error {
		s, h := showFPS, hideFPS
		showFPS, hideFPS = false, false
		if s {
			dbg.SetShowFPS(true)
		}
		if h {
			dbg.SetShowFPS(false)
		}
		saveEnginePrefs()
		return nil
	})

	// memalloc: --show / --hide to show or hide the memory counter (under FPS, green). Hidden by default.
	var showMemAlloc, hideMemAlloc bool
	memallocFS := flag.NewFlagSet("memalloc", flag.ContinueOnError)
	memallocFS.BoolVar(&showMemAlloc, "show", false, "show memory allocation")
	memallocFS.BoolVar(&hideMemAlloc, "hide", false, "hide memory allocation")
	reg.Register("memalloc", memallocFS, func() error {
		s, h := showMemAlloc, hideMemAlloc
		showMemAlloc, hideMemAlloc = false, false
		if s {
			dbg.SetShowMemAlloc(true)
		}
		if h {
			dbg.SetShowMemAlloc(false)
		}
		saveEnginePrefs()
		return nil
	})

	// window: --fullscreen / --windowed to switch display mode (raylib ToggleFullscreen when needed).
	var wantFullscreen, wantWindowed bool
	windowFS := flag.NewFlagSet("window", flag.ContinueOnError)
	windowFS.BoolVar(&wantFullscreen, "fullscreen", false, "switch to fullscreen")
	windowFS.BoolVar(&wantWindowed, "windowed", false, "switch to windowed")
	reg.Register("window", windowFS, func() error {
		f, w := wantFullscreen, wantWindowed
		wantFullscreen, wantWindowed = false, false
		if f == w {
			return nil // no change if both or neither set
		}
		isFull := rl.IsWindowFullscreen()
		if f && !isFull {
			rl.ToggleFullscreen()
		}
		if w && isFull {
			rl.ToggleFullscreen()
		}
		return nil
	})

	// spawn: add a primitive at a position. Usage: cmd spawn <type> <x> <y> <z> [sx sy sz]
	// type: cube | sphere | cylinder | plane. Scale defaults to 1,1,1 if omitted.
	spawnFS := flag.NewFlagSet("spawn", flag.ContinueOnError)
	reg.Register("spawn", spawnFS, func() error {
		args := spawnFS.Args()
		if len(args) != 4 && len(args) != 7 {
			return fmt.Errorf("usage: cmd spawn <type> <x> <y> <z> [sx sy sz] (e.g. cmd spawn cube 0 0 0 or cmd spawn cube 0 0 0 2 1 1)")
		}
		typ := args[0]
		switch typ {
		case "cube", "sphere", "cylinder", "plane":
			// ok
		default:
			return fmt.Errorf("unknown type %q (use: cube, sphere, cylinder, plane)", typ)
		}
		var pos [3]float32
		for i := 0; i < 3; i++ {
			f, err := strconv.ParseFloat(args[1+i], 32)
			if err != nil {
				return fmt.Errorf("invalid position %q: %w", args[1+i], err)
			}
			pos[i] = float32(f)
		}
		scale := [3]float32{1, 1, 1}
		if len(args) == 7 {
			for i := 0; i < 3; i++ {
				f, err := strconv.ParseFloat(args[4+i], 32)
				if err != nil {
					return fmt.Errorf("invalid scale %q: %w", args[4+i], err)
				}
				scale[i] = float32(f)
			}
		}
		scn.AddPrimitive(typ, pos, scale)
		scn.RecordAdd(1)
		return nil
	})

	// save: write current scene (including runtime-spawned objects) to the scene YAML file.
	saveFS := flag.NewFlagSet("save", flag.ContinueOnError)
	reg.Register("save", saveFS, func() error {
		return scn.SaveScene()
	})

	// newscene: clear all primitives and save an empty scene (fresh start).
	newsceneFS := flag.NewFlagSet("newscene", flag.ContinueOnError)
	reg.Register("newscene", newsceneFS, func() error {
		return scn.NewScene()
	})

	// model: set AI model for natural-language commands. Usage: cmd model <name> (e.g. cmd model gpt-4o-mini)
	// When using Ollama, model cannot be changed (prevents voice/LLM from switching model by accident).
	var isOllama bool // set in LLM client switch below
	modelFS := flag.NewFlagSet("model", flag.ContinueOnError)
	reg.Register("model", modelFS, func() error {
		if isOllama {
			return fmt.Errorf("cannot change model when using Ollama (disabled to prevent voice/LLM from switching by accident)")
		}
		args := modelFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd model <name> (e.g. cmd model gpt-4o-mini)")
		}
		currentAIModel = args[0]
		saveEnginePrefs()
		return nil
	})

	// physics: enable or disable falling/collision for the selected object. Usage: cmd physics on | cmd physics off
	physicsFS := flag.NewFlagSet("physics", flag.ContinueOnError)
	reg.Register("physics", physicsFS, func() error {
		args := physicsFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd physics on | cmd physics off (select an object first)")
		}
		switch args[0] {
		case "on":
			return scn.SetSelectedPhysics(true)
		case "off":
			return scn.SetSelectedPhysics(false)
		default:
			return fmt.Errorf("use on or off (e.g. cmd physics off)")
		}
	})

	// delete: remove object(s). With camera awareness: delete plane | delete red cube | delete right | delete cube left | delete all cube | delete all building
	deleteFS := flag.NewFlagSet("delete", flag.ContinueOnError)
	reg.Register("delete", deleteFS, func() error {
		args := deleteFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd delete selected | look | random | name <name> | left|right|top|bottom | [color] <type> [position] | all [type|name]")
		}
		primTypes := map[string]bool{"cube": true, "sphere": true, "cylinder": true, "plane": true}
		positionWords := map[string]bool{"left": true, "right": true, "top": true, "bottom": true, "closest": true, "farthest": true}
		colorNames := map[string][3]float32{
			"red": {1, 0, 0}, "green": {0, 1, 0}, "blue": {0, 0, 1}, "yellow": {1, 1, 0},
			"orange": {1, 0.5, 0}, "purple": {0.5, 0, 0.5}, "pink": {1, 0.75, 0.8},
			"white": {1, 1, 1}, "black": {0, 0, 0}, "gray": {0.5, 0.5, 0.5}, "grey": {0.5, 0.5, 0.5},
		}

		switch args[0] {
		case "selected":
			return scn.DeleteSelected()
		case "look", "camera":
			return scn.DeleteAtCameraLook()
		case "random":
			return scn.DeleteRandom()
		case "name":
			if len(args) < 2 {
				return fmt.Errorf("usage: cmd delete name <name>")
			}
			_, err := scn.DeleteByName(args[1])
			return err
		case "all":
			// delete all [type] | delete all [color] [type] | delete all <name_substring>
			if len(args) < 2 {
				n, err := scn.DeleteAllVisibleByDescription("", nil, "")
				if err != nil {
					return err
				}
				fmt.Printf("Deleted %d object(s) in view.\n", n)
				return nil
			}
			if len(args) == 2 {
				a1 := strings.ToLower(args[1])
				if primTypes[a1] {
					n, err := scn.DeleteAllVisibleByDescription(a1, nil, "")
					if err != nil {
						return err
					}
					fmt.Printf("Deleted %d %s(s) in view.\n", n, a1)
					return nil
				}
				// name substring (e.g. "building")
				n, err := scn.DeleteAllVisibleByDescription("", nil, a1)
				if err != nil {
					return err
				}
				fmt.Printf("Deleted %d object(s) matching %q in view.\n", n, a1)
				return nil
			}
			if len(args) == 3 {
				colorName := strings.ToLower(args[1])
				typ := strings.ToLower(args[2])
				if primTypes[typ] {
					if c, ok := colorNames[colorName]; ok {
						n, err := scn.DeleteAllVisibleByDescription(typ, &c, "")
						if err != nil {
							return err
						}
						fmt.Printf("Deleted %d %s %s(s) in view.\n", n, colorName, typ)
						return nil
					}
				}
			}
			return fmt.Errorf("usage: cmd delete all [type] or delete all [color] [type] or delete all <name_substring>")
		}

		// Position-only: delete left | delete right | etc.
		if len(args) == 1 && positionWords[strings.ToLower(args[0])] {
			return scn.DeleteVisibleByPosition(strings.ToLower(args[0]))
		}

		// One word: type only
		if len(args) == 1 {
			typ := strings.ToLower(args[0])
			if primTypes[typ] {
				return scn.DeleteVisibleByDescription(typ, nil)
			}
			return fmt.Errorf("use selected, look, random, name <name>, left|right|top|bottom, [color] <type> [position], or all [type|name]")
		}

		// Two words: type + position, or color + type, or name substring + position
		if len(args) == 2 {
			a0, a1 := strings.ToLower(args[0]), strings.ToLower(args[1])
			if primTypes[a0] && positionWords[a1] {
				return scn.DeleteVisibleByDescriptionAndPosition(a0, nil, "", a1)
			}
			if c, ok := colorNames[a0]; ok && primTypes[a1] {
				return scn.DeleteVisibleByDescription(a1, &c)
			}
			// name substring + position (e.g. "building right")
			if positionWords[a1] {
				return scn.DeleteVisibleByDescriptionAndPosition("", nil, a0, a1)
			}
		}

		if len(args) == 3 {
			a0, a1, a2 := strings.ToLower(args[0]), strings.ToLower(args[1]), strings.ToLower(args[2])
			if c, ok := colorNames[a0]; ok && primTypes[a1] && positionWords[a2] {
				return scn.DeleteVisibleByDescriptionAndPosition(a1, &c, "", a2)
			}
		}

		return fmt.Errorf("use selected, look, random, name <name>, left|right|top|bottom, [color] <type> [position], or all [type|name]")
	})

	// select: choose object in view by description/position (updates scene selection).
	selectFS := flag.NewFlagSet("select", flag.ContinueOnError)
	reg.Register("select", selectFS, func() error {
		args := selectFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd select none | left|right|top|bottom|closest|farthest | [color] <type> [position] | <name_substring> [position]")
		}
		primTypes := map[string]bool{"cube": true, "sphere": true, "cylinder": true, "plane": true}
		positionWords := map[string]bool{"left": true, "right": true, "top": true, "bottom": true, "closest": true, "farthest": true}
		colorNames := map[string][3]float32{
			"red": {1, 0, 0}, "green": {0, 1, 0}, "blue": {0, 0, 1}, "yellow": {1, 1, 0},
			"orange": {1, 0.5, 0}, "purple": {0.5, 0, 0.5}, "pink": {1, 0.75, 0.8},
			"white": {1, 1, 1}, "black": {0, 0, 0}, "gray": {0.5, 0.5, 0.5}, "grey": {0.5, 0.5, 0.5},
		}

		switch strings.ToLower(args[0]) {
		case "none":
			scn.ClearSelection()
			return nil
		}

		// Position-only: select left | select right | etc.
		if len(args) == 1 && positionWords[strings.ToLower(args[0])] {
			return scn.SelectVisibleByPosition(strings.ToLower(args[0]))
		}

		// One word: type or name substring
		if len(args) == 1 {
			a0 := strings.ToLower(args[0])
			if primTypes[a0] {
				return scn.SelectVisibleByDescriptionAndPosition(a0, nil, "", "")
			}
			// treat as name substring (e.g. "building")
			return scn.SelectVisibleByDescriptionAndPosition("", nil, a0, "")
		}

		// Two words: type + position, or color + type, or name substring + position
		if len(args) == 2 {
			a0, a1 := strings.ToLower(args[0]), strings.ToLower(args[1])
			if primTypes[a0] && positionWords[a1] {
				return scn.SelectVisibleByDescriptionAndPosition(a0, nil, "", a1)
			}
			if c, ok := colorNames[a0]; ok && primTypes[a1] {
				return scn.SelectVisibleByDescriptionAndPosition(a1, &c, "", "")
			}
			// name substring + position (e.g. "building right")
			if positionWords[a1] {
				return scn.SelectVisibleByDescriptionAndPosition("", nil, a0, a1)
			}
		}

		// Three words: color + type + position
		if len(args) == 3 {
			a0, a1, a2 := strings.ToLower(args[0]), strings.ToLower(args[1]), strings.ToLower(args[2])
			if c, ok := colorNames[a0]; ok && primTypes[a1] && positionWords[a2] {
				return scn.SelectVisibleByDescriptionAndPosition(a1, &c, "", a2)
			}
		}

		return fmt.Errorf("usage: cmd select none | left|right|top|bottom|closest|farthest | [color] <type> [position] | <name_substring> [position]")
	})

	// look: point camera at a visible object by description/position (does not change selection).
	lookFS := flag.NewFlagSet("look", flag.ContinueOnError)
	reg.Register("look", lookFS, func() error {
		args := lookFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd look left|right|top|bottom|closest|farthest | [color] <type> [position] | <name_substring> [position]")
		}
		primTypes := map[string]bool{"cube": true, "sphere": true, "cylinder": true, "plane": true}
		positionWords := map[string]bool{"left": true, "right": true, "top": true, "bottom": true, "closest": true, "farthest": true}
		colorNames := map[string][3]float32{
			"red": {1, 0, 0}, "green": {0, 1, 0}, "blue": {0, 0, 1}, "yellow": {1, 1, 0},
			"orange": {1, 0.5, 0}, "purple": {0.5, 0, 0.5}, "pink": {1, 0.75, 0.8},
			"white": {1, 1, 1}, "black": {0, 0, 0}, "gray": {0.5, 0.5, 0.5}, "grey": {0.5, 0.5, 0.5},
		}

		// Position-only: look left | look right | etc.
		if len(args) == 1 && positionWords[strings.ToLower(args[0])] {
			return scn.FocusOnVisibleByPosition(strings.ToLower(args[0]))
		}

		// One word: type or name substring (closest)
		if len(args) == 1 {
			a0 := strings.ToLower(args[0])
			if primTypes[a0] {
				return scn.FocusOnVisibleByDescriptionAndPosition(a0, nil, "", "")
			}
			// treat as name substring
			return scn.FocusOnVisibleByDescriptionAndPosition("", nil, a0, "")
		}

		// Two words: type + position, or color + type, or name substring + position
		if len(args) == 2 {
			a0, a1 := strings.ToLower(args[0]), strings.ToLower(args[1])
			if primTypes[a0] && positionWords[a1] {
				return scn.FocusOnVisibleByDescriptionAndPosition(a0, nil, "", a1)
			}
			if c, ok := colorNames[a0]; ok && primTypes[a1] {
				return scn.FocusOnVisibleByDescriptionAndPosition(a1, &c, "", "")
			}
			if positionWords[a1] {
				return scn.FocusOnVisibleByDescriptionAndPosition("", nil, a0, a1)
			}
		}

		// Three words: color + type + position
		if len(args) == 3 {
			a0, a1, a2 := strings.ToLower(args[0]), strings.ToLower(args[1]), strings.ToLower(args[2])
			if c, ok := colorNames[a0]; ok && primTypes[a1] && positionWords[a2] {
				return scn.FocusOnVisibleByDescriptionAndPosition(a1, &c, "", a2)
			}
		}

		return fmt.Errorf("usage: cmd look left|right|top|bottom|closest|farthest | [color] <type> [position] | <name_substring> [position]")
	})

	// inspect: print details about an object (type, name, position, scale, color, physics, motion, texture).
	// Usage: cmd inspect            (selected, or closest visible if none selected)
	inspectFS := flag.NewFlagSet("inspect", flag.ContinueOnError)
	reg.Register("inspect", inspectFS, func() error {
		args := inspectFS.Args()
		printInfo := func(label string, obj scene.ObjectInstance) {
			fmt.Printf("%s: type=%s name=%q pos=[%.2f,%.2f,%.2f] scale=[%.2f,%.2f,%.2f] color=[%.2f,%.2f,%.2f] physics=%v motion=%q texture=%q\n",
				label,
				obj.Type, obj.Name,
				obj.Position[0], obj.Position[1], obj.Position[2],
				obj.Scale[0], obj.Scale[1], obj.Scale[2],
				obj.Color[0], obj.Color[1], obj.Color[2],
				scene.PhysicsEnabledForObject(obj), obj.Motion, obj.Texture)
		}
		if len(args) == 0 {
			if obj, ok := scn.SelectedObject(); ok {
				printInfo("Selected", obj)
				return nil
			}
			visible := scn.ObjectsInView()
			if len(visible) == 0 {
				return fmt.Errorf("no objects in view")
			}
			v := visible[0] // closest
			printInfo("Closest in view", v.Object)
			return nil
		}
		return fmt.Errorf("usage: cmd inspect (no arguments)")
	})

	// download: fetch image from URL in background and apply to selected object when done. Usage: cmd download image <url>
	type downloadResult struct {
		Index int
		Path  string
		Err   error
	}
	downloadDone := make(chan *downloadResult, 8)
	downloadFS := flag.NewFlagSet("download", flag.ContinueOnError)
	reg.Register("download", downloadFS, func() error {
		args := downloadFS.Args()
		if len(args) < 2 {
			return fmt.Errorf("usage: cmd download image <url> (select an object first)")
		}
		if args[0] != "image" {
			return fmt.Errorf("usage: cmd download image <url>")
		}
		url := args[1]
		if url == "" {
			return fmt.Errorf("url is required")
		}
		idx := scn.SelectedIndex()
		if idx < 0 {
			return fmt.Errorf("no object selected (click an object with terminal open)")
		}
		go func() {
			relPath, err := downloadImage(url, "assets/textures/downloaded")
			downloadDone <- &downloadResult{Index: idx, Path: relPath, Err: err}
		}()
		return nil
	})

	// texture: apply image file as texture to selected object. Usage: cmd texture <path> (e.g. cmd texture assets/textures/downloaded/foo.png)
	textureFS := flag.NewFlagSet("texture", flag.ContinueOnError)
	reg.Register("texture", textureFS, func() error {
		args := textureFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd texture <path> (e.g. cmd texture assets/textures/downloaded/foo.png)")
		}
		path := args[0]
		if path == "" {
			return fmt.Errorf("path is required")
		}
		if scn.SelectedIndex() < 0 {
			return fmt.Errorf("no object selected (click an object with terminal open)")
		}
		return scn.SetSelectedTexture(path)
	})

	// skybox: download image from URL in background and set as skybox when done. Usage: cmd skybox <url>
	type skyboxResult struct {
		Path string
		Err  error
	}
	skyboxDone := make(chan *skyboxResult, 4)
	skyboxFS := flag.NewFlagSet("skybox", flag.ContinueOnError)
	reg.Register("skybox", skyboxFS, func() error {
		args := skyboxFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd skybox <url> (e.g. cmd skybox https://example.com/panorama.jpg)")
		}
		url := args[0]
		if url == "" {
			return fmt.Errorf("url is required")
		}
		go func() {
			relPath, err := downloadImage(url, "assets/skybox/downloaded")
			skyboxDone <- &skyboxResult{Path: relPath, Err: err}
		}()
		return nil
	})

	// color: set RGB (0-1) on selected object. Usage: cmd color <r> <g> <b> (e.g. cmd color 1 0 0 for red)
	colorFS := flag.NewFlagSet("color", flag.ContinueOnError)
	reg.Register("color", colorFS, func() error {
		args := colorFS.Args()
		if len(args) < 3 {
			return fmt.Errorf("usage: cmd color <r> <g> <b> (0-1, e.g. cmd color 1 0 0)")
		}
		var c [3]float32
		for i := 0; i < 3; i++ {
			f, err := strconv.ParseFloat(args[i], 32)
			if err != nil || f < 0 || f > 1 {
				return fmt.Errorf("color components must be 0-1")
			}
			c[i] = float32(f)
		}
		return scn.SetSelectedColor(c)
	})

	// duplicate: clone selected object N times with offset. Usage: cmd duplicate [N] (default 1, offset 2 on X)
	duplicateFS := flag.NewFlagSet("duplicate", flag.ContinueOnError)
	reg.Register("duplicate", duplicateFS, func() error {
		n := 1
		if args := duplicateFS.Args(); len(args) >= 1 {
			if v, err := strconv.Atoi(args[0]); err == nil && v >= 1 {
				n = v
			}
		}
		offset := [3]float32{2, 0, 0}
		count, err := scn.DuplicateSelected(n, offset)
		if err != nil {
			return err
		}
		log.Log(fmt.Sprintf("Duplicated %d time(s).", count))
		return nil
	})

	// screenshot: capture current view to screenshot.png (in cwd)
	screenshotFS := flag.NewFlagSet("screenshot", flag.ContinueOnError)
	reg.Register("screenshot", screenshotFS, func() error {
		rl.TakeScreenshot("screenshot.png")
		log.Log("Screenshot saved: screenshot.png")
		return nil
	})

	// lighting: set time-of-day profile. Usage: cmd lighting noon | sunset | night
	lightingFS := flag.NewFlagSet("lighting", flag.ContinueOnError)
	reg.Register("lighting", lightingFS, func() error {
		args := lightingFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd lighting noon | sunset | night")
		}
		scn.SetLighting(args[0])
		return nil
	})

	// name: set name on selected object. Usage: cmd name <name>
	nameFS := flag.NewFlagSet("name", flag.ContinueOnError)
	reg.Register("name", nameFS, func() error {
		args := nameFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd name <name>")
		}
		return scn.SetSelectedName(args[0])
	})

	// motion: set motion on selected. Usage: cmd motion off | bob | spin (spin not yet implemented)
	motionFS := flag.NewFlagSet("motion", flag.ContinueOnError)
	reg.Register("motion", motionFS, func() error {
		args := motionFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd motion off | bob")
		}
		m := args[0]
		if m == "off" {
			m = ""
		}
		return scn.SetSelectedMotion(m)
	})

	// undo: revert last add or delete
	undoFS := flag.NewFlagSet("undo", flag.ContinueOnError)
	reg.Register("undo", undoFS, func() error {
		return scn.Undo()
	})

	// focus: point camera at selected object
	focusFS := flag.NewFlagSet("focus", flag.ContinueOnError)
	reg.Register("focus", focusFS, func() error {
		return scn.FocusOnSelected()
	})

	// view: list objects currently visible to the camera (object-awareness)
	viewFS := flag.NewFlagSet("view", flag.ContinueOnError)
	reg.Register("view", viewFS, func() error {
		visible := scn.ObjectsInView()
		if len(visible) == 0 {
			fmt.Println("No objects in view. Move the camera to look at primitives.")
			return nil
		}
		fmt.Printf("%d object(s) in view (closest first):\n", len(visible))
		for _, v := range visible {
			name := v.Object.Name
			if name == "" {
				name = fmt.Sprintf("#%d", v.Index)
			}
			fmt.Printf("  %s — %s — distance %.2f — screen (%.0f, %.0f)\n",
				name, v.Object.Type, v.Distance, v.ScreenPos.X, v.ScreenPos.Y)
		}
		return nil
	})

	// gravity: set gravity strength/direction. Usage: cmd gravity <y> (e.g. -9.8, 0, 4.9 for low, 0 for float)
	gravityFS := flag.NewFlagSet("gravity", flag.ContinueOnError)
	reg.Register("gravity", gravityFS, func() error {
		args := gravityFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd gravity <y> (e.g. cmd gravity -9.8 or 0 for zero-g)")
		}
		f, err := strconv.ParseFloat(args[0], 32)
		if err != nil {
			return fmt.Errorf("gravity must be a number")
		}
		scn.SetGravity([3]float32{0, float32(f), 0})
		return nil
	})

	// template: spawn a preset (e.g. tree). Usage: cmd template tree [x y z]
	templateFS := flag.NewFlagSet("template", flag.ContinueOnError)
	reg.Register("template", templateFS, func() error {
		args := templateFS.Args()
		if len(args) < 1 {
			return fmt.Errorf("usage: cmd template tree [x y z]")
		}
		x, y, z := 0.0, 0.0, 0.0
		if len(args) >= 4 {
			for i, s := range []*float64{&x, &y, &z} {
				if f, err := strconv.ParseFloat(args[1+i], 32); err == nil {
					*s = f
				}
			}
		}
		switch args[0] {
		case "tree":
			// Trunk (cylinder) + foliage (sphere)
			_ = reg.Execute([]string{"spawn", "cylinder", strconv.FormatFloat(x, 'f', -1, 32), strconv.FormatFloat(y, 'f', -1, 32), strconv.FormatFloat(z, 'f', -1, 32), "0.3", "2", "0.3"})
			_ = reg.Execute([]string{"spawn", "sphere", strconv.FormatFloat(x, 'f', -1, 32), strconv.FormatFloat(y+1.5, 'f', -1, 32), strconv.FormatFloat(z, 'f', -1, 32), "1.2", "1.2", "1.2"})
			log.Log("Spawned tree.")
		default:
			return fmt.Errorf("unknown template (use tree)")
		}
		return nil
	})

	term := terminal.New(log, reg)

	// Voice-to-text: Cmd+R to record; release to transcribe and send to LLM. Root path for vtt module.
	var vttRoot string
	for _, p := range []string{"modules/voice-to-text", "../../modules/voice-to-text"} {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			vttRoot, _ = filepath.Abs(p)
			break
		}
	}
	var voiceRecording bool
	var voiceRec *vttlib.Recorder
	var wasCmdRDown bool

	// LLM + agent: natural language -> structured actions -> scene/commands.
	// Priority: Groq (free) > Cursor (+ OpenAI fallback) > OpenAI > Ollama (local, e.g. Qwen 3 Coder).
	var ag *agent.Agent
	var client llm.Client
	groqKey := os.Getenv("GROQ_API_KEY")
	cursorKey := os.Getenv("CURSOR_API_KEY")
	openAIKey := os.Getenv("OPENAI_API_KEY")
	ollamaBase := os.Getenv("OLLAMA_BASE_URL")
	switch {
	case groqKey != "":
		client = llm.NewGroq(groqKey)
	case cursorKey != "" && openAIKey != "":
		client = &llm.Fallback{Primary: llm.NewCursor(cursorKey), Secondary: llm.NewOpenAI(openAIKey)}
	case cursorKey != "":
		client = llm.NewCursor(cursorKey)
	case openAIKey != "":
		client = llm.NewOpenAI(openAIKey)
	default:
		client = llm.NewOllama(ollamaBase)
		isOllama = true
		// Use Ollama default when no model set or when saved model is a cloud name (e.g. from when Groq was used).
		if currentAIModel == "" || currentAIModel == "gpt-4o-mini" || currentAIModel == "llama-3.3-70b-versatile" {
			currentAIModel = "qwen3-coder:30b"
			saveEnginePrefs()
		}
	}
	// Commands from the agent (e.g. window) must run on the main thread; queue them here.
	pendingRunCmd := make(chan []string, 64)
	if client != nil {
		ag = agent.New(client, func() string { return currentAIModel })
		agent.RegisterSceneHandlers(ag, scn, reg, pendingRunCmd)
	}
	if ag != nil {
		term.GetViewContext = func() string { return scn.GetViewContextSummary() }
		term.OnNaturalLanguage = func(line string, viewContext string) {
			log.Log("Thinking…")
			summary, err := ag.Run(context.Background(), line, viewContext)
			if err != nil {
				log.Log(err.Error())
			} else {
				log.Log(summary)
			}
		}
	}

	// UI: CSS-driven overlay (scene UI). Renders after debug, before terminal.
	uiEngine := ui.New()
	for _, path := range []string{"assets/ui/default.css", "../../assets/ui/default.css"} {
		if err := uiEngine.LoadCSS(path); err == nil {
			break
		}
	}
	// Base nodes: none (inspector is the only UI when shown)
	baseNodes := []*ui.Node{}
	inspector := ui.NewInspector()

	// font: set or show active UI font. Usage: cmd font [name]. If not found locally, downloads from Google Fonts (safe).
	fontFS := flag.NewFlagSet("font", flag.ContinueOnError)
	type fontDownloadResult struct {
		RelPath  string
		FullPath string
		Err      error
	}
	fontDownloadDone := make(chan *fontDownloadResult, 2)
	reg.Register("font", fontFS, func() error {
		args := fontFS.Args()
		if len(args) < 1 {
			log.Log("Current font: " + currentFontPath)
			return nil
		}
		rel := args[0]
		// If the LLM sent a path that already includes assets/fonts/, strip it so we don't double-prefix
		rel = fonts.StripAssetsFontsPrefix(rel)
		// Try direct path first (e.g. Roboto/static/Roboto-Regular.ttf)
		for _, p := range []string{"assets/fonts/" + rel, "../../assets/fonts/" + rel} {
			if err := uiEngine.LoadFont(p); err == nil {
				currentFontPath = rel
				term.SetFont(uiEngine.Font())
				dbg.SetFont(uiEngine.Font())
				saveEnginePrefs()
				log.Log("Font set: " + rel)
				return nil
			}
		}
		// Search assets/fonts for a .ttf/.otf matching the name (e.g. "Inter", "Google Sans")
		for _, search := range fonts.SearchCandidates(rel) {
			if foundRel, fullPath, findErr := fonts.FindFont(search); findErr == nil {
				if err := uiEngine.LoadFont(fullPath); err == nil {
					currentFontPath = foundRel
					term.SetFont(uiEngine.Font())
					dbg.SetFont(uiEngine.Font())
					saveEnginePrefs()
					log.Log("Font set: " + foundRel)
					return nil
				}
			}
		}
		// Not found locally: download from Google Fonts (by name only; no arbitrary URLs)
		go func() {
			res := &fontDownloadResult{}
			defer func() { fontDownloadDone <- res }()
			downloadURL, err := googlefonts.FetchDownloadURLByFamily(rel)
			if err != nil {
				res.Err = err
				return
			}
			var baseDir string
			for _, d := range []string{"assets/fonts", "../../assets/fonts"} {
				if err := os.MkdirAll(filepath.Join(d, "downloaded"), 0755); err == nil {
					baseDir = d
					break
				}
			}
			if baseDir == "" {
				res.Err = fmt.Errorf("cannot create assets/fonts/downloaded")
				return
			}
			// Save to downloaded/<family>/ so multiple fonts don't clash
			folder := googlefonts.NormalizeFamily(rel)[0]
			downloadDir := filepath.Join(baseDir, "downloaded", folder)
			if err := os.MkdirAll(downloadDir, 0755); err != nil {
				res.Err = err
				return
			}
			savedPath, err := download.Download(downloadURL, downloadDir)
			if err != nil {
				res.Err = err
				return
			}
			res.RelPath = filepath.ToSlash("downloaded/" + folder + "/" + filepath.Base(savedPath))
			res.FullPath = savedPath
		}()
		log.Log("Downloading font from Google Fonts…")
		return nil
	})

	// Load engine font from assets/fonts/ (config: prefs.Font, default Roboto). One font for UI, terminal, and debug.
	uiFontTried := false
	engineFontPaths := func() []string {
		rel := currentFontPath
		return []string{
			"assets/fonts/" + rel,
			"../../assets/fonts/" + rel,
		}
	}()

	update := func() {
		// Run any commands queued by the agent (e.g. window) on the main thread.
		for {
			select {
			case args := <-pendingRunCmd:
				if err := reg.Execute(args); err != nil {
					log.Log(err.Error())
				}
			default:
				goto done
			}
		}
	done:
		// Apply textures from background downloads (main thread only).
		for {
			select {
			case res := <-downloadDone:
				if res.Err != nil {
					log.Log(res.Err.Error())
				} else if err := scn.SetObjectTexture(res.Index, res.Path); err != nil {
					log.Log(err.Error())
				} else {
					log.Log("Texture applied: " + res.Path)
				}
			default:
				goto doneDownload
			}
		}
	doneDownload:
		// Set skybox from background downloads (main thread only).
		for {
			select {
			case res := <-skyboxDone:
				if res.Err != nil {
					log.Log(res.Err.Error())
				} else {
					scn.SetSkyboxPath(res.Path)
					log.Log("Skybox set: " + res.Path)
				}
			default:
				goto doneSkybox
			}
		}
	doneSkybox:
		// Apply font from URL download (main thread only).
		for {
			select {
			case res := <-fontDownloadDone:
				if res.Err != nil {
					log.Log(res.Err.Error())
				} else if err := uiEngine.LoadFont(res.FullPath); err != nil {
					log.Log(err.Error())
				} else {
					currentFontPath = res.RelPath
					term.SetFont(uiEngine.Font())
					dbg.SetFont(uiEngine.Font())
					saveEnginePrefs()
					log.Log("Font set: " + res.RelPath)
				}
			default:
				goto doneFontDownload
			}
		}
	doneFontDownload:
		term.Update()

		// Voice: hold Cmd+R to record; release to transcribe and send to chat (with logs).
		combo := (rl.IsKeyDown(rl.KeyLeftSuper) || rl.IsKeyDown(rl.KeyRightSuper)) && rl.IsKeyDown(rl.KeyR)
		if combo && !wasCmdRDown && !voiceRecording && vttRoot != "" {
			rec, err := vttlib.NewRecorder(vttRoot)
			if err != nil {
				log.Log("Voice: " + err.Error())
			} else if err := rec.Start(); err != nil {
				log.Log("Voice: " + err.Error())
			} else {
				voiceRec = rec
				voiceRecording = true
				log.Log("Voice: recording…")
			}
		}
		if combo && !wasCmdRDown && !voiceRecording && vttRoot == "" {
			log.Log("Voice: modules/voice-to-text not found (run from repo root?)")
		}
		if !combo && wasCmdRDown && voiceRecording {
			voiceRecording = false
			rec := voiceRec
			voiceRec = nil
			if rec != nil {
				if err := rec.Stop(); err != nil {
					log.Log("Voice: stop error: " + err.Error())
				} else {
					log.Log("Voice: stopped, transcribing…")
					viewCtx := ""
					if term.GetViewContext != nil {
						viewCtx = term.GetViewContext()
					}
					viewCtxCopy := viewCtx
					go func() {
						text, err := rec.Transcribe(context.Background())
						if err != nil {
							log.Log("Voice: " + err.Error())
							return
						}
						text = strings.TrimSpace(text)
						if text == "" {
							return
						}
						log.Log("Voice (transcript): " + text)
						// Skip sending very short / noise transcripts to avoid random LLM actions (e.g. "you").
						const minSendLen = 5
						if len(text) < minSendLen {
							log.Log("Voice (skipped, too short; not sent to chat): " + text)
							return
						}
						log.Log("Voice (sent to chat): " + text)
						if term.OnNaturalLanguage != nil {
							term.OnNaturalLanguage(text, viewCtxCopy)
						}
					}()
				}
			}
		}
		wasCmdRDown = combo

		if term.IsOpen() {
			// Cursor visible: allow selecting and moving primitives (not skybox/grid).
			scn.UpdateEditor(true, terminal.BarHeight)
			// Inspector physics toggle: left-click on the physics row toggles physics for selected object.
			if obj, ok := scn.SelectedObject(); ok {
				if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
					screenH := int32(rl.GetScreenHeight())
					mouseY := rl.GetMouseY()
					if mouseY < screenH-int32(terminal.BarHeight) {
						hitNode, hit := uiEngine.HitTest(rl.GetMouseX(), mouseY)
						if hit && hitNode != nil && hitNode.Class == "inspector-physics" {
							_ = scn.SetSelectedPhysics(!scene.PhysicsEnabledForObject(obj))
						}
					}
				}
			}
		} else {
			scn.Update()
		}
	}
	draw := func() {
		scn.Draw(term.IsOpen())
		dbg.Draw()
		var nodes []*ui.Node
		obj, ok := scn.SelectedObject()
		nodes = inspector.AppendNodes(baseNodes, term.IsOpen() && ok, ui.Selection{
			Name:     obj.Type,
			Position: obj.Position,
			Scale:    obj.Scale,
			Physics:  scene.PhysicsEnabledForObject(obj),
			Texture:  obj.Texture,
		})
		if !uiFontTried {
			uiFontTried = true
			for _, p := range engineFontPaths {
				if err := uiEngine.LoadFont(p); err == nil {
					term.SetFont(uiEngine.Font())
					dbg.SetFont(uiEngine.Font())
					break
				}
			}
		}
		uiEngine.SetNodes(nodes)
		uiEngine.Draw()
		// Recording indicator: only when chat is collapsed and voice is recording
		if !term.IsOpen() && voiceRecording {
			screenH := int(rl.GetScreenHeight())
			y := screenH - 32
			if !rl.IsWindowFullscreen() {
				y -= terminal.WindowedBarOffset
			}
			x := 16
			// Red dot
			rl.DrawCircle(int32(x+6), int32(y+8), 6, rl.Red)
			rl.DrawCircleLines(int32(x+6), int32(y+8), 6, rl.Maroon)
			// "Recording" text
			recText := "Recording..."
			if uiEngine.Font().Texture.ID != 0 {
				rl.DrawTextEx(uiEngine.Font(), recText, rl.NewVector2(float32(x+20), float32(y+2)), 18, 1, rl.Red)
			} else {
				rl.DrawText(recText, int32(x+20), int32(y+2), 18, rl.Red)
			}
		}
		term.Draw()
	}
	graphics.Run(update, draw)
}
