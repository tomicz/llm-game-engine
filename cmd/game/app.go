package main

import (
	"os"

	"game-engine/internal/agent"
	"game-engine/internal/assets"
	"game-engine/internal/commands"
	"game-engine/internal/debug"
	"game-engine/internal/engineconfig"
	"game-engine/internal/graphics"
	"game-engine/internal/logger"
	"game-engine/internal/scene"
	"game-engine/internal/terminal"
	"game-engine/internal/ui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// App wires the engine subsystems together and runs the frame loop. Everything except the
// background work started by background() runs on the main thread.
type App struct {
	log       *logger.Logger
	scene     *scene.Scene
	debug     *debug.Debug
	commands  *commands.Registry
	terminal  *terminal.Terminal
	ui        *ui.Engine
	inspector *ui.Inspector
	nodes     []*ui.Node // UI nodes for this frame; reused

	provider string // "ollama", "openai", or "groq"
	model    string
	font     string // path under assets/fonts/
	agent    *agent.Agent

	tasks chan func() // results of background work, run on the main thread
}

// NewApp creates the engine from the saved preferences. The window opens in Run.
func NewApp() *App {
	log := logger.New()
	rl.SetTraceLogCallback(log.LogEngine)
	prefs, err := engineconfig.Load()
	if err != nil {
		log.Log("config: " + err.Error())
	}

	a := &App{
		log:       log,
		scene:     scene.New(),
		debug:     debug.New(),
		commands:  commands.NewRegistry(),
		ui:        ui.New(),
		inspector: ui.NewInspector(),
		font:      prefs.Font,
		tasks:     make(chan func(), 16),
	}
	a.debug.SetShowFPS(prefs.ShowFPS)
	a.debug.SetShowMemAlloc(prefs.ShowMemAlloc)
	a.scene.SetGridVisible(prefs.GridVisible)
	if os.Getenv("CAMERA_AWARENESS") == "1" {
		a.scene.EnableViewAwareness(scene.NewViewAwarenessWithLogging())
	}

	a.registerCommands()
	a.terminal = terminal.New(log, a.commands)
	a.terminal.OnNaturalLanguage = a.handleNaturalLanguage

	// The agent's prompt lists the registered commands, so set up AI after registering them.
	provider := prefs.AIProvider
	if provider == "" {
		provider = detectProvider()
	}
	if err := a.setProvider(provider, prefs.AIModel); err != nil {
		log.Log("LLM: " + err.Error())
	}

	if path, ok := assets.Find("assets/ui/default.css"); ok {
		if err := a.ui.LoadCSS(path); err != nil {
			log.Log("UI: " + err.Error())
		}
	}
	a.savePrefs()
	return a
}

// Run opens the window and blocks until it is closed.
func (a *App) Run() {
	graphics.Run(a.init, a.update, a.draw)
}

// init runs once the window exists.
func (a *App) init() {
	if path, ok := assets.Find("assets/fonts/" + a.font); ok {
		_ = a.loadFont(a.font, path)
	}
}

func (a *App) update() {
	a.runTasks()
	a.terminal.Update()
	if !a.terminal.IsOpen() {
		a.scene.Update()
		return
	}
	// Clicks on the UI overlay must not also select or deselect objects behind it.
	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) && a.handleUIClick() {
		return
	}
	a.scene.UpdateEditor(a.terminal.BarTop())
}

// handleUIClick handles a left click on the UI overlay and reports whether it hit the overlay.
func (a *App) handleUIClick() bool {
	node, hit := a.ui.HitTest(rl.GetMouseX(), rl.GetMouseY())
	if !hit {
		return false
	}
	if node.Class == "inspector-physics" {
		if o, ok := a.scene.Selected(); ok {
			o.SetPhysics(!o.PhysicsEnabled())
		}
	}
	return true
}

func (a *App) draw() {
	editing := a.terminal.IsOpen()
	a.scene.Draw(editing)
	a.debug.Draw()

	a.nodes = a.nodes[:0]
	if o, ok := a.scene.Selected(); ok && editing {
		a.nodes = a.inspector.AppendNodes(a.nodes, ui.Selection{
			Name:     o.Type,
			Position: o.Position,
			Scale:    o.Scale,
			Physics:  o.PhysicsEnabled(),
			Texture:  o.Texture,
		})
	}
	a.ui.SetNodes(a.nodes)
	a.ui.Draw()
	a.terminal.Draw()
}

// background runs work on a new goroutine, then done with its result on the main thread at the
// start of a later frame. work must not touch raylib, the scene, or other main-thread state.
func background[T any](a *App, work func() (T, error), done func(T, error)) {
	go func() {
		v, err := work()
		a.tasks <- func() { done(v, err) }
	}()
}

// runTasks runs the completed background results queued since the last frame.
func (a *App) runTasks() {
	for {
		select {
		case fn := <-a.tasks:
			fn()
		default:
			return
		}
	}
}

// loadFont makes the font at path (rel is its name under assets/fonts/) the UI, terminal, and
// debug font.
func (a *App) loadFont(rel, path string) error {
	if err := a.ui.LoadFont(path); err != nil {
		return err
	}
	a.font = rel
	a.terminal.SetFont(a.ui.Font())
	a.debug.SetFont(a.ui.Font())
	return nil
}

func (a *App) savePrefs() {
	err := engineconfig.Save(engineconfig.EnginePrefs{
		ShowFPS:      a.debug.ShowFPS,
		ShowMemAlloc: a.debug.ShowMemAlloc,
		GridVisible:  a.scene.GridVisible,
		AIProvider:   a.provider,
		AIModel:      a.model,
		Font:         a.font,
	})
	if err != nil {
		a.log.Error("config: " + err.Error())
	}
}
