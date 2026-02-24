package main

import (
	"game-engine/internal/graphics"
	"game-engine/internal/logger"
	"game-engine/internal/terminal"
)

func main() {
	log := logger.New()
	term := terminal.New(log)
	draw := func() {
		term.Draw()
	}
	graphics.Run(term.Update, draw)
}
