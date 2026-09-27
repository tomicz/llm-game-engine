package debug

import (
	"fmt"
	"runtime"

	"game-engine/internal/ui"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	fpsFontSize   = 20
	fpsPadding    = 12
	fpsLineHeight = fpsFontSize + 4
	// updateInterval: only refresh FPS/Mem text every N frames to reduce allocations.
	updateInterval = 30
)

// Debug holds runtime debugging features (e.g. FPS display). All overlays are off by default.
type Debug struct {
	ShowFPS      bool
	ShowMemAlloc bool
	font         rl.Font // optional; when set, Draw uses DrawTextEx instead of default font
	frameCount   uint32
	lastFpsText  string
	lastMemText  string
	lastMemStats runtime.MemStats
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

// SetFont sets the font used to draw FPS/Mem (e.g. same as UI). Zero texture ID = use raylib default.
func (d *Debug) SetFont(font rl.Font) {
	d.font = font
}

// Draw renders the enabled overlays at the top-right in green: FPS, then heap allocation below it.
// Text is only recomputed every updateInterval frames to limit allocations.
func (d *Debug) Draw() {
	d.frameCount++
	refresh := d.frameCount%updateInterval == 0
	y := int32(fpsPadding)
	if d.ShowFPS {
		if refresh || d.lastFpsText == "" {
			d.lastFpsText = fmt.Sprintf("FPS: %d", rl.GetFPS())
		}
		d.drawRight(d.lastFpsText, y)
		y += fpsLineHeight
	}
	if d.ShowMemAlloc {
		if refresh || d.lastMemText == "" {
			runtime.ReadMemStats(&d.lastMemStats)
			d.lastMemText = fmt.Sprintf("Mem: %.2f MiB", float64(d.lastMemStats.Alloc)/(1024*1024))
		}
		d.drawRight(d.lastMemText, y)
	}
}

// drawRight draws text right-aligned against the screen edge.
func (d *Debug) drawRight(text string, y int32) {
	x := int32(rl.GetScreenWidth()) - ui.MeasureText(d.font, text, fpsFontSize) - fpsPadding
	ui.DrawText(d.font, text, x, y, fpsFontSize, rl.Green)
}
