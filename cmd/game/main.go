package main

import (
	"game-engine/internal/commands"
	"game-engine/internal/debug"
	"game-engine/internal/engineconfig"
	"game-engine/internal/env"
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
	_ = env.Load(".env")
	_ = env.Load("../../.env")

	if os.Getenv("DEBUG_PPROF") == "1" {
		go func() { _ = http.ListenAndServe("localhost:6060", nil) }()
	}

	log := logger.New()
	rl.SetTraceLogCallback(log.LogEngine)

	scn := scene.New()
	dbg := debug.New()
	reg := commands.NewRegistry()

	if os.Getenv("CAMERA_AWARENESS") == "1" {
		scn.EnableViewAwareness(scene.NewViewAwarenessWithLogging())
	}

	// Apply persisted engine prefs.
	prefs, _ := engineconfig.Load()
	dbg.SetShowFPS(prefs.ShowFPS)
	dbg.SetShowMemAlloc(prefs.ShowMemAlloc)
	scn.SetGridVisible(prefs.GridVisible)

	// Resolve provider: use persisted value, or auto-detect from env on first run.
	provider := prefs.AIProvider
	if provider == "" {
		provider = detectProvider()
	}

	model := prefs.AIModel
	if model == "" {
		model = DefaultModelForProvider(provider)
	}

	currentFont := prefs.Font
	if currentFont == "" {
		currentFont = "Roboto/static/Roboto-Regular.ttf"
	}

	app := &App{
		Log:              log,
		Scene:            scn,
		Debug:            dbg,
		Registry:         reg,
		UI:               ui.New(),
		Inspector:        ui.NewInspector(),
		CurrentProvider:  provider,
		CurrentAIModel:   model,
		CurrentFont:      currentFont,
		DownloadDone:     make(chan *downloadResult, 8),
		SkyboxDone:       make(chan *skyboxResult, 4),
		FontDownloadDone: make(chan *fontDownloadResult, 2),
		PendingRunCmd:    make(chan []string, 64),
		baseNodes:        []*ui.Node{},
	}

	registerCommands(app)

	// Build LLM client from provider config.
	client, err := BuildLLMClient(app.CurrentProvider)
	if err != nil {
		log.Log("LLM: " + err.Error())
	} else {
		app.Client = client
	}

	app.Terminal = terminal.New(log, reg)
	app.RebuildAgent()
	app.SaveEnginePrefs()

	// UI: CSS-driven overlay.
	for _, path := range []string{"assets/ui/default.css", "../../assets/ui/default.css"} {
		if err := app.UI.LoadCSS(path); err == nil {
			break
		}
	}

	// Resolve engine font paths for lazy loading in Draw.
	app.engineFontPaths = []string{
		"assets/fonts/" + app.CurrentFont,
		"../../assets/fonts/" + app.CurrentFont,
	}

	graphics.Run(app.Update, app.Draw)
}

// detectProvider picks the best available provider based on env vars.
func detectProvider() string {
	switch {
	case os.Getenv("GROQ_API_KEY") != "":
		return "groq"
	case os.Getenv("OPENAI_API_KEY") != "":
		return "openai"
	default:
		return "ollama"
	}
}
