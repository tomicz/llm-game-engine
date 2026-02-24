package main

import (
	"flag"
	"game-engine/internal/commands"
	"game-engine/internal/graphics"
	"game-engine/internal/logger"
	"game-engine/internal/scene"
	"game-engine/internal/terminal"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func main() {
	logger := logger.New()
	rl.SetTraceLogCallback(logger.LogEngine) // capture raylib INFO/WARNING/ERROR to engine_log.txt

	scn := scene.New()
	reg := commands.NewRegistry()

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
		return nil
	})

	term := terminal.New(logger, reg)
	update := func() {
		term.Update()
		if !term.IsOpen() {
			scn.Update()
		}
	}
	draw := func() {
		scn.Draw()
		term.Draw()
	}
	graphics.Run(update, draw)
}
