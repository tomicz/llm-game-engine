package terminal

import (
	"game-engine/internal/commands"
	"game-engine/internal/logger"
	"unicode/utf8"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	BarHeight = 40
	// When windowed, move bar up by this many pixels so it stays visible (avoids being cut off by taskbar/window bounds).
	WindowedBarOffset = 56
	prompt            = "> "
	fontSize          = 20
	padding           = 8
)

var (
	// Reused every frame when drawing the terminal bar to avoid per-frame color allocations.
	termBarColor  = rl.NewColor(40, 40, 40, 255)
	termLineColor = rl.NewColor(80, 80, 80, 255)
)

// Terminal is the chat/terminal input bar at the bottom of the screen. It is shown/hidden with ESC.
// When open, it handles typing and drawing; when closed, nothing is drawn and the player can move (WASD).
// Lines starting with "cmd " are parsed as subcommand + flags and executed via the command registry.
// Other lines are treated as natural language; if OnNaturalLanguage is set, it is called in a goroutine.
type Terminal struct {
	log                *logger.Logger
	reg                *commands.Registry
	inputBuf           string
	open               bool
	OnNaturalLanguage  func(line string) // called in a goroutine when user submits a non-cmd line
}

// New returns a new Terminal that logs lines and runs "cmd ..." through reg. It starts closed (hidden); press ESC to open.
func New(log *logger.Logger, reg *commands.Registry) *Terminal {
	return &Terminal{log: log, reg: reg}
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
	// Paste: Ctrl+V (Windows/Linux) or Cmd+V (macOS)
	if rl.IsKeyPressed(rl.KeyV) && (rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyRightControl) || rl.IsKeyDown(rl.KeyLeftSuper) || rl.IsKeyDown(rl.KeyRightSuper)) {
		if pasted := rl.GetClipboardText(); pasted != "" {
			t.inputBuf += pasted
		}
	} else {
		for {
			c := rl.GetCharPressed()
			if c == 0 {
				break
			}
			t.inputBuf += string(rune(c))
		}
	}
	if rl.IsKeyPressed(rl.KeyBackspace) && len(t.inputBuf) > 0 {
		_, size := utf8.DecodeLastRuneInString(t.inputBuf)
		t.inputBuf = t.inputBuf[:len(t.inputBuf)-size]
	}
	if (rl.IsKeyPressed(rl.KeyEnter) || rl.IsKeyPressed(rl.KeyKpEnter)) && t.inputBuf != "" {
		line := t.inputBuf
		t.log.Log(line)
		t.inputBuf = ""

		if args, isCmd := commands.Parse(line); isCmd {
			if err := t.reg.Execute(args); err != nil {
				t.log.Log(err.Error())
			}
		} else if t.OnNaturalLanguage != nil {
			t.log.Log(line)
			go t.OnNaturalLanguage(line)
		} else {
			t.log.Log(line)
		}
	}
}

// Draw draws the terminal bar at the bottom when open. Call after ClearBackground in the render loop.
// Uses GetScreenWidth/GetScreenHeight so the bar matches the 2D overlay coordinate system (correct in fullscreen).
func (t *Terminal) Draw() {
	if !t.open {
		return
	}
	screenW := int(rl.GetScreenWidth())
	screenH := int(rl.GetScreenHeight())
	barY := screenH - BarHeight
	if !rl.IsWindowFullscreen() {
		barY -= WindowedBarOffset
	}

	rl.DrawRectangle(0, int32(barY), int32(screenW), int32(BarHeight), termBarColor)
	rl.DrawRectangle(0, int32(barY), int32(screenW), 1, termLineColor)

	text := prompt + t.inputBuf + "|"
	rl.DrawText(text, int32(padding), int32(barY+padding), int32(fontSize), rl.White)
}
