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

// Terminal is the chat/terminal input bar at the bottom of the screen. It is shown/hidden with ESC.
// When open, it handles typing and drawing; when closed, nothing is drawn and the player can move (WASD).
type Terminal struct {
	log      *logger.Logger
	inputBuf string
	open     bool
}

// New returns a new Terminal that logs lines to log. It starts closed (hidden); press ESC to open.
func New(log *logger.Logger) *Terminal {
	return &Terminal{log: log}
}

// IsOpen returns true when the terminal is visible and capturing input (player cannot move).
func (t *Terminal) IsOpen() bool {
	return t.open
}

// Update handles ESC (toggle open/closed), and when open: typing, backspace, enter. Call once per frame.
func (t *Terminal) Update() {
	if rl.IsKeyPressed(rl.KeyEscape) {
		t.open = !t.open
		if t.open {
			rl.EnableCursor()
		} else {
			rl.DisableCursor()
		}
	}
	if !t.open {
		return
	}
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

// Draw draws the terminal bar at the bottom when open. Call after ClearBackground in the render loop.
func (t *Terminal) Draw() {
	if !t.open {
		return
	}
	screenW := int(rl.GetScreenWidth())
	screenH := int(rl.GetScreenHeight())
	barY := screenH - barHeight

	rl.DrawRectangle(0, int32(barY), int32(screenW), int32(barHeight), rl.NewColor(40, 40, 40, 255))
	rl.DrawRectangle(0, int32(barY), int32(screenW), 1, rl.NewColor(80, 80, 80, 255))

	text := prompt + t.inputBuf + "|"
	rl.DrawText(text, int32(padding), int32(barY+padding), int32(fontSize), rl.White)
}
