package terminal

import (
	"game-engine/internal/logger"
	"unicode/utf8"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	barHeight = 40
	prompt    = "> "
	fontSize  = 20
	padding   = 8
)

// Terminal is the chat/terminal input bar at the bottom of the screen. It handles input and drawing.
type Terminal struct {
	log      *logger.Logger
	inputBuf string
}

// New returns a new Terminal that logs lines to log.
func New(log *logger.Logger) *Terminal {
	return &Terminal{log: log}
}

// Update handles keyboard input (typing, backspace, enter). Call once per frame.
func (t *Terminal) Update() {
	for {
		c := rl.GetCharPressed()
		if c == 0 {
			break
		}
		t.inputBuf += string(rune(c))
	}
	if rl.IsKeyPressed(rl.KeyBackspace) && len(t.inputBuf) > 0 {
		_, size := utf8.DecodeLastRuneInString(t.inputBuf)
		t.inputBuf = t.inputBuf[:len(t.inputBuf)-size]
	}
	if (rl.IsKeyPressed(rl.KeyEnter) || rl.IsKeyPressed(rl.KeyKpEnter)) && t.inputBuf != "" {
		t.log.Log(t.inputBuf)
		t.inputBuf = ""
	}
}

// Draw draws the terminal bar at the bottom of the screen. Call after ClearBackground in the render loop.
func (t *Terminal) Draw() {
	screenW := int(rl.GetScreenWidth())
	screenH := int(rl.GetScreenHeight())
	barY := screenH - barHeight

	rl.DrawRectangle(0, int32(barY), int32(screenW), int32(barHeight), rl.NewColor(40, 40, 40, 255))
	rl.DrawRectangle(0, int32(barY), int32(screenW), 1, rl.NewColor(80, 80, 80, 255))

	text := prompt + t.inputBuf + "|"
	rl.DrawText(text, int32(padding), int32(barY+padding), int32(fontSize), rl.White)
}
