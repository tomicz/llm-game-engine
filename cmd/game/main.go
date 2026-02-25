package main

import (
	"flag"
	"game-engine/internal/commands"
	"game-engine/internal/debug"
	"game-engine/internal/engineconfig"
	"game-engine/internal/graphics"
	"game-engine/internal/logger"
	"game-engine/internal/scene"
	"game-engine/internal/terminal"
	"game-engine/internal/ui"
	"net/http"
	_ "net/http/pprof"
	"os"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func main() {
	// Optional: enable heap/CPU profiling. Run with DEBUG_PPROF=1, then e.g. go tool pprof -http=:8080 http://localhost:6060/debug/pprof/heap
	if os.Getenv("DEBUG_PPROF") == "1" {
		go func() { _ = http.ListenAndServe("localhost:6060", nil) }()
	}

	logger := logger.New()
	rl.SetTraceLogCallback(logger.LogEngine) // capture raylib INFO/WARNING/ERROR to engine_log.txt

	scn := scene.New()
	dbg := debug.New()
	reg := commands.NewRegistry()

	// Apply persisted engine prefs (debug overlays, grid). Save on every toggle.
	prefs, _ := engineconfig.Load()
	dbg.SetShowFPS(prefs.ShowFPS)
	dbg.SetShowMemAlloc(prefs.ShowMemAlloc)
	scn.SetGridVisible(prefs.GridVisible)
	saveEnginePrefs := func() {
		_ = engineconfig.Save(engineconfig.EnginePrefs{
			ShowFPS:      dbg.ShowFPS,
			ShowMemAlloc: dbg.ShowMemAlloc,
			GridVisible:  scn.GridVisible,
		})
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

	term := terminal.New(logger, reg)

	// UI: CSS-driven overlay (scene UI). Renders after debug, before terminal.
	uiEngine := ui.New()
	for _, path := range []string{"assets/ui/default.css", "../../assets/ui/default.css"} {
		if err := uiEngine.LoadCSS(path); err == nil {
			break
		}
	}
	uiEngine.AddNode(ui.NewNode("panel", "panel", "", ""))
	uiEngine.AddNode(ui.NewNode("label", "title", "", "UI"))
	uiEngine.AddNode(ui.NewNode("label", "label", "", "CSS-driven"))

	update := func() {
		term.Update()
		if !term.IsOpen() {
			scn.Update()
		}
	}
	draw := func() {
		scn.Draw()
		dbg.Draw()
		uiEngine.Draw()
		term.Draw()
	}
	graphics.Run(update, draw)
}
