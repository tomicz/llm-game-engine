package debug

import (
	"fmt"
	"runtime"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	fpsFontSize   = 20
	fpsPadding    = 12
	fpsLineHeight = fpsFontSize + 4
)

// Debug holds runtime debugging features (e.g. FPS display). All overlays are off by default.
type Debug struct {
	ShowFPS     bool
	ShowMemAlloc bool
}

// New returns a Debug system with all overlays hidden.
func New() *Debug {
	return &Debug{}
}

// SetShowFPS sets whether the FPS counter is drawn (top-right, green).
func (d *Debug) SetShowFPS(show bool) {
	d.ShowFPS = show
}

// SetShowMemAlloc sets whether the memory allocation counter is drawn (top-right, under FPS).
func (d *Debug) SetShowMemAlloc(show bool) {
	d.ShowMemAlloc = show
}

// Draw renders any enabled debug overlays. Call after scene and terminal in the draw loop.
// FPS is drawn at the top-right in green when ShowFPS is true.
// Memory (heap alloc) is drawn under FPS when ShowMemAlloc is true.
func (d *Debug) Draw() {
	screenW := int32(rl.GetScreenWidth())
	y := int32(fpsPadding)

	if d.ShowFPS {
		text := fmt.Sprintf("FPS: %d", rl.GetFPS())
		w := rl.MeasureText(text, fpsFontSize)
		x := screenW - w - fpsPadding
		rl.DrawText(text, x, y, fpsFontSize, rl.Green)
		y += fpsLineHeight
	}

	if d.ShowMemAlloc {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		mb := float64(m.Alloc) / (1024 * 1024)
		text := fmt.Sprintf("Mem: %.2f MiB", mb)
		w := rl.MeasureText(text, fpsFontSize)
		x := screenW - w - fpsPadding
		rl.DrawText(text, x, y, fpsFontSize, rl.Green)
	}
}
