package main

import (
	"game-engine/internal/graphics"
	"game-engine/internal/logger"
	"game-engine/internal/scene"
	"game-engine/internal/terminal"
)

func main() {
	log := logger.New()
	term := terminal.New(log)
	scn := scene.New()
	update := func() {
		scn.Update()
		term.Update()
	}
	draw := func() {
		scn.Draw()
		term.Draw()
	}
	graphics.Run(update, draw)
}
