// Package terminal is the chat/command bar at the bottom of the screen, toggled with ESC.
package terminal

import (
	"unicode/utf8"

	"game-engine/internal/commands"
	"game-engine/internal/logger"
	"game-engine/internal/ui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	barHeight = 40
	// windowedBarOffset lifts the bar in windowed mode so it isn't cut off by the taskbar/window bounds.
	windowedBarOffset = 56
	prompt            = "> "
	fontSize          = 20
	padding           = 8
	// maxLinesOnScreen is how many log lines are shown above the input bar.
	maxLinesOnScreen = 14
	lineHeight       = fontSize + 4
	maxLineLength    = 200
)

var (
	barColor    = rl.NewColor(40, 40, 40, 255)
	lineColor   = rl.NewColor(80, 80, 80, 255)
	chatBgColor = rl.NewColor(24, 24, 24, 240)
)

// Terminal is the input bar plus recent log lines. When open it captures typing and shows the
// cursor; when closed nothing is drawn and the camera has the mouse. Submitted lines starting with
// "cmd " run through the command registry; other lines go to OnNaturalLanguage.
type Terminal struct {
	log      *logger.Logger
	reg      *commands.Registry
	inputBuf string
	open     bool
	font     rl.Font
	// OnNaturalLanguage handles a submitted non-command line. It is called on the main thread and
	// must not block (start a goroutine for slow work).
	OnNaturalLanguage func(line string)
}

// New returns a closed terminal that logs lines to log and runs commands through reg.
func New(log *logger.Logger, reg *commands.Registry) *Terminal {
	return &Terminal{log: log, reg: reg}
}

// IsOpen reports whether the terminal is visible and capturing input.
func (t *Terminal) IsOpen() bool {
	return t.open
}

// SetFont sets the font used to draw the terminal. A zero font means raylib's default.
func (t *Terminal) SetFont(font rl.Font) {
	t.font = font
}

// BarTop is the screen Y of the top of the input bar; mouse input below it belongs to the terminal.
func (t *Terminal) BarTop() int32 {
	y := int32(rl.GetScreenHeight()) - barHeight
	if !rl.IsWindowFullscreen() {
		y -= windowedBarOffset
	}
	return y
}

// Update handles ESC (toggle) and, while open, typing, paste, backspace, and enter.
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
	// Paste: Ctrl+V (Windows/Linux) or Cmd+V (macOS).
	ctrl := rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyRightControl) ||
		rl.IsKeyDown(rl.KeyLeftSuper) || rl.IsKeyDown(rl.KeyRightSuper)
	if ctrl && rl.IsKeyPressed(rl.KeyV) {
		t.inputBuf += rl.GetClipboardText()
	} else {
		for c := rl.GetCharPressed(); c != 0; c = rl.GetCharPressed() {
			t.inputBuf += string(c)
		}
	}
	if (rl.IsKeyPressed(rl.KeyBackspace) || rl.IsKeyPressedRepeat(rl.KeyBackspace)) && t.inputBuf != "" {
		_, size := utf8.DecodeLastRuneInString(t.inputBuf)
		t.inputBuf = t.inputBuf[:len(t.inputBuf)-size]
	}
	if (rl.IsKeyPressed(rl.KeyEnter) || rl.IsKeyPressed(rl.KeyKpEnter)) && t.inputBuf != "" {
		line := t.inputBuf
		t.inputBuf = ""
		t.log.Log(line)
		t.submit(line)
	}
}

func (t *Terminal) submit(line string) {
	if args, isCmd := commands.Parse(line); isCmd {
		if err := t.reg.Execute(args); err != nil {
			t.log.Log(err.Error())
		}
		return
	}
	if t.OnNaturalLanguage != nil {
		t.OnNaturalLanguage(line)
	}
}

// Draw draws the input bar and, above it, the most recent log lines. Does nothing when closed.
func (t *Terminal) Draw() {
	if !t.open {
		return
	}
	screenW := int32(rl.GetScreenWidth())
	barY := t.BarTop()

	chatHeight := int32(maxLinesOnScreen * lineHeight)
	chatY := max(barY-chatHeight, 0)
	chatHeight = barY - chatY
	if chatHeight > 0 {
		rl.DrawRectangle(0, chatY, screenW, chatHeight, chatBgColor)
	}
	for i, line := range t.log.Tail(maxLinesOnScreen) {
		if len(line) > maxLineLength {
			line = line[:maxLineLength-3] + "..."
		}
		ui.DrawText(t.font, line, padding, chatY+int32(i*lineHeight)+padding, fontSize, rl.LightGray)
	}

	rl.DrawRectangle(0, barY, screenW, barHeight, barColor)
	rl.DrawRectangle(0, barY, screenW, 1, lineColor)
	ui.DrawText(t.font, prompt+t.inputBuf+"|", padding, barY+padding, fontSize, rl.White)
}
