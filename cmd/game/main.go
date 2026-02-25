package main

import (
	"context"
	"flag"
	"fmt"
	"game-engine/internal/agent"
	"game-engine/internal/commands"
	"game-engine/internal/debug"
	"game-engine/internal/engineconfig"
	"game-engine/internal/env"
	"game-engine/internal/graphics"
	"game-engine/internal/llm"
	"game-engine/internal/logger"
	"game-engine/internal/scene"
	"game-engine/internal/terminal"
	"game-engine/internal/ui"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"strconv"

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

	// Apply persisted engine prefs (debug overlays, grid, AI model). Save on every toggle.
	prefs, _ := engineconfig.Load()
	dbg.SetShowFPS(prefs.ShowFPS)
	dbg.SetShowMemAlloc(prefs.ShowMemAlloc)
	scn.SetGridVisible(prefs.GridVisible)
	currentAIModel := prefs.AIModel
	if currentAIModel == "" {
		currentAIModel = "gpt-4o-mini"
	}
	saveEnginePrefs := func() {
		_ = engineconfig.Save(engineconfig.EnginePrefs{
			ShowFPS:      dbg.ShowFPS,
			ShowMemAlloc: dbg.ShowMemAlloc,
			GridVisible:  scn.GridVisible,
			AIModel:      currentAIModel,
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
	modelFS := flag.NewFlagSet("model", flag.ContinueOnError)
	reg.Register("model", modelFS, func() error {
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

	term := terminal.New(log, reg)

	// LLM + agent: natural language -> structured actions -> scene/commands.
	// Priority: Groq (free), then Cursor (fallback to OpenAI if 404), then OpenAI.
	var ag *agent.Agent
	var client llm.Client
	groqKey := os.Getenv("GROQ_API_KEY")
	cursorKey := os.Getenv("CURSOR_API_KEY")
	openAIKey := os.Getenv("OPENAI_API_KEY")
	switch {
	case groqKey != "":
		client = llm.NewGroq(groqKey)
	case cursorKey != "" && openAIKey != "":
		client = &llm.Fallback{Primary: llm.NewCursor(cursorKey), Secondary: llm.NewOpenAI(openAIKey)}
	case cursorKey != "":
		client = llm.NewCursor(cursorKey)
	case openAIKey != "":
		client = llm.NewOpenAI(openAIKey)
	}
	if client != nil {
		ag = agent.New(client, func() string { return currentAIModel })
		agent.RegisterSceneHandlers(ag, scn, reg)
	}
	if ag != nil {
		term.OnNaturalLanguage = func(line string) {
			log.Log("Thinking…")
			summary, err := ag.Run(context.Background(), line)
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

	// Try loading a TTF font for UI (smooth text). Attempt once after first frame (window exists).
	uiFontTried := false
	uiFontPaths := func() []string {
		paths := []string{"assets/ui/fonts/default.ttf", "../../assets/ui/fonts/default.ttf"}
		switch runtime.GOOS {
		case "darwin":
			paths = append(paths, "/System/Library/Fonts/Supplemental/Arial.ttf", "/Library/Fonts/Arial.ttf", "/System/Library/Fonts/Helvetica.ttc")
		case "windows":
			paths = append(paths, "C:\\Windows\\Fonts\\arial.ttf", "C:\\Windows\\Fonts\\segoeui.ttf")
		default:
			paths = append(paths, "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", "/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf")
		}
		return paths
	}()

	update := func() {
		term.Update()
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
		})
		if !uiFontTried {
			uiFontTried = true
			for _, p := range uiFontPaths {
				if err := uiEngine.LoadFont(p); err == nil {
					break
				}
			}
		}
		uiEngine.SetNodes(nodes)
		uiEngine.Draw()
		term.Draw()
	}
	graphics.Run(update, draw)
}
