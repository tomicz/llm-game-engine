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
	// Number of chat/log lines drawn above the input bar when terminal is open.
	maxLinesOnScreen = 14
	lineHeight       = fontSize + 4
)

var (
	// Reused every frame when drawing the terminal bar to avoid per-frame color allocations.
	termBarColor   = rl.NewColor(40, 40, 40, 255)
	termLineColor  = rl.NewColor(80, 80, 80, 255)
	termChatBgColor = rl.NewColor(24, 24, 24, 240)
)

// Terminal is the chat/terminal input bar at the bottom of the screen. It is shown/hidden with ESC.
// When open, it handles typing and drawing; when closed, nothing is drawn and the player can move (WASD).
// Lines starting with "cmd " are parsed as subcommand + flags and executed via the command registry.
// Other lines are treated as natural language; if OnNaturalLanguage is set, it is called in a goroutine.
// GetViewContext, if set, is called on the main thread when the user submits natural language; its result
// is passed as the second argument so the LLM can reason about what the camera sees (e.g. "delete the one on the right").
type Terminal struct {
	log               *logger.Logger
	reg               *commands.Registry
	inputBuf          string
	open              bool
	font              rl.Font // optional; when set, Draw uses DrawTextEx instead of default font
	GetViewContext    func() string       // optional; called on main thread when user submits NL
	OnNaturalLanguage func(line string, viewContext string) // called in a goroutine when user submits a non-cmd line
}

// New returns a new Terminal that logs lines and runs "cmd ..." through reg. It starts closed (hidden); press ESC to open.
func New(log *logger.Logger, reg *commands.Registry) *Terminal {
	return &Terminal{log: log, reg: reg}
}

// IsOpen returns true when the terminal is visible and capturing input (player cannot move).
func (t *Terminal) IsOpen() bool {
	return t.open
}

// SetFont sets the font used to draw the terminal bar (e.g. same as UI). Zero texture ID = use raylib default.
func (t *Terminal) SetFont(font rl.Font) {
	t.font = font
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
			viewCtx := ""
			if t.GetViewContext != nil {
				viewCtx = t.GetViewContext()
			}
			viewCtxCopy := viewCtx
			go t.OnNaturalLanguage(line, viewCtxCopy)
		} else {
			t.log.Log(line)
		}
	}
}

// Draw draws the terminal bar at the bottom when open, and the recent chat/log lines above it.
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

	// Chat history area above the bar: last maxLinesOnScreen lines
	chatHeight := maxLinesOnScreen * lineHeight
	chatY := barY - chatHeight
	if chatY < 0 {
		chatHeight = barY
		chatY = 0
	}
	if chatHeight > 0 {
		rl.DrawRectangle(0, int32(chatY), int32(screenW), int32(chatHeight), termChatBgColor)
	}
	lines := t.log.Lines()
	start := 0
	if len(lines) > maxLinesOnScreen {
		start = len(lines) - maxLinesOnScreen
	}
	for i := start; i < len(lines); i++ {
		y := chatY + (i-start)*lineHeight + padding
		line := lines[i]
		if len(line) > 200 {
			line = line[:197] + "..."
		}
		if t.font.Texture.ID != 0 {
			rl.DrawTextEx(t.font, line, rl.NewVector2(float32(padding), float32(y)), float32(fontSize), 1, rl.LightGray)
		} else {
			rl.DrawText(line, int32(padding), int32(y), int32(fontSize), rl.LightGray)
		}
	}

	// Input bar
	rl.DrawRectangle(0, int32(barY), int32(screenW), int32(BarHeight), termBarColor)
	rl.DrawRectangle(0, int32(barY), int32(screenW), 1, termLineColor)

	text := prompt + t.inputBuf + "|"
	if t.font.Texture.ID != 0 {
		rl.DrawTextEx(t.font, text, rl.NewVector2(float32(padding), float32(barY+padding)), float32(fontSize), 1, rl.White)
	} else {
		rl.DrawText(text, int32(padding), int32(barY+padding), int32(fontSize), rl.White)
	}
}
