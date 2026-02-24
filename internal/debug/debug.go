package debug

import (
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	fpsFontSize = 20
	fpsPadding  = 12
)

// Debug holds runtime debugging features (e.g. FPS display). All overlays are off by default.
type Debug struct {
	ShowFPS bool
}

// New returns a Debug system with all overlays hidden.
func New() *Debug {
	return &Debug{}
}

// SetShowFPS sets whether the FPS counter is drawn (top-right, green).
func (d *Debug) SetShowFPS(show bool) {
	d.ShowFPS = show
}

// Draw renders any enabled debug overlays. Call after scene and terminal in the draw loop.
// FPS is drawn at the top-right in green when ShowFPS is true.
func (d *Debug) Draw() {
	if !d.ShowFPS {
		return
	}
	text := fmt.Sprintf("FPS: %d", rl.GetFPS())
	w := rl.MeasureText(text, fpsFontSize)
	screenW := int32(rl.GetScreenWidth())
	x := screenW - w - fpsPadding
	y := int32(fpsPadding)
	rl.DrawText(text, x, y, fpsFontSize, rl.Green)
}
