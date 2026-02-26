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
	// updateInterval: only refresh FPS/Mem text every N frames to reduce allocations.
	updateInterval = 30
)

// Debug holds runtime debugging features (e.g. FPS display). All overlays are off by default.
type Debug struct {
	ShowFPS       bool
	ShowMemAlloc  bool
	font          rl.Font // optional; when set, Draw uses DrawTextEx instead of default font
	frameCount    uint32
	lastFpsText   string
	lastMemText   string
	lastMemStats  runtime.MemStats
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

// Draw renders any enabled debug overlays. Call after scene and terminal in the draw loop.
// FPS is drawn at the top-right in green when ShowFPS is true.
// Memory (heap alloc) is drawn under FPS when ShowMemAlloc is true.
// Text is only recomputed every updateInterval frames to limit allocations.
func (d *Debug) Draw() {
	d.frameCount++
	update := (d.frameCount % updateInterval) == 0
	if d.ShowFPS && d.lastFpsText == "" {
		update = true
	}
	if d.ShowMemAlloc && d.lastMemText == "" {
		update = true
	}

	screenW := int32(rl.GetScreenWidth())
	y := int32(fpsPadding)

	if d.ShowFPS {
		if update {
			d.lastFpsText = fmt.Sprintf("FPS: %d", rl.GetFPS())
		}
		text := d.lastFpsText
		if text != "" {
			if d.font.Texture.ID != 0 {
				sz := float32(fpsFontSize)
				pos := rl.NewVector2(float32(screenW)-rl.MeasureTextEx(d.font, text, sz, 1).X-float32(fpsPadding), float32(y))
				rl.DrawTextEx(d.font, text, pos, sz, 1, rl.Green)
			} else {
				w := rl.MeasureText(text, fpsFontSize)
				x := screenW - w - fpsPadding
				rl.DrawText(text, x, y, fpsFontSize, rl.Green)
			}
		}
		y += fpsLineHeight
	}

	if d.ShowMemAlloc {
		if update {
			runtime.ReadMemStats(&d.lastMemStats)
			mb := float64(d.lastMemStats.Alloc) / (1024 * 1024)
			d.lastMemText = fmt.Sprintf("Mem: %.2f MiB", mb)
		}
		text := d.lastMemText
		if text != "" {
			if d.font.Texture.ID != 0 {
				sz := float32(fpsFontSize)
				pos := rl.NewVector2(float32(screenW)-rl.MeasureTextEx(d.font, text, sz, 1).X-float32(fpsPadding), float32(y))
				rl.DrawTextEx(d.font, text, pos, sz, 1, rl.Green)
			} else {
				w := rl.MeasureText(text, fpsFontSize)
				x := screenW - w - fpsPadding
				rl.DrawText(text, x, y, fpsFontSize, rl.Green)
			}
		}
	}
}
