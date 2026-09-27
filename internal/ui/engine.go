package ui

import (
	"os"
	"slices"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const defaultFontSize = 20

// Engine holds the current stylesheet and nodes, and draws them with raylib.
// Draw order is node order (first node drawn first, then on top the next).
// Resolved styles are cached and only recomputed when sheet or nodes change to avoid per-frame allocations.
// If font is loaded (LoadFont), text is drawn with that font; otherwise raylib's default (pixel) font is used.
type Engine struct {
	sheet        *Stylesheet
	nodes        []*Node
	cachedStyles []ComputedStyle
	cacheValid   bool
	font         rl.Font
}

// New creates an empty UI engine (no stylesheet, no nodes).
func New() *Engine {
	return &Engine{sheet: nil, nodes: nil}
}

// LoadCSS loads and parses a CSS file from path. Replaces the current stylesheet.
func (e *Engine) LoadCSS(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sheet, err := ParseCSS(string(data))
	if err != nil {
		return err
	}
	e.sheet = sheet
	e.cacheValid = false
	return nil
}

// LoadFont loads a TTF font from path for text rendering. If loading fails, the engine keeps using the default font.
// Call after the window/OpenGL context exists (e.g. after first frame or in draw).
func (e *Engine) LoadFont(path string) error {
	f := rl.LoadFont(path)
	if f.Texture.ID == 0 {
		return os.ErrNotExist
	}
	if e.font.Texture.ID != 0 {
		rl.UnloadFont(e.font)
	}
	e.font = f
	return nil
}

// Font returns the currently loaded font (for use by terminal, debug, etc.). Zero texture ID means no font loaded.
func (e *Engine) Font() rl.Font {
	return e.font
}

// SetNodes replaces all nodes. Styles are re-resolved only when the node list changes, so it is
// cheap to call every frame with the same nodes.
func (e *Engine) SetNodes(nodes []*Node) {
	if slices.Equal(e.nodes, nodes) {
		return
	}
	e.nodes = slices.Clone(nodes)
	e.cacheValid = false
}

// resolveProps returns merged properties for a node (class and id matched; last wins).
func (e *Engine) resolveProps(n *Node) map[string]string {
	merged := make(map[string]string)
	if e.sheet == nil {
		return merged
	}
	for _, rule := range e.sheet.Rules {
		sel := rule.Selector
		matches := false
		if len(sel) > 0 && sel[0] == '.' {
			class := sel[1:]
			if n.Class == class {
				matches = true
			}
		} else if len(sel) > 0 && sel[0] == '#' {
			id := sel[1:]
			if n.ID == id {
				matches = true
			}
		}
		if matches {
			for k, v := range rule.Props {
				merged[k] = v
			}
		}
	}
	return merged
}

// resolveBounds sets n.Bounds from style (left, top, width, height). If style has zero size, Bounds is unchanged.
func resolveBounds(n *Node, style ComputedStyle) {
	if style.Width > 0 {
		n.Bounds.Width = float32(style.Width)
	}
	if style.Height > 0 {
		n.Bounds.Height = float32(style.Height)
	}
	n.Bounds.X = float32(style.Left)
	n.Bounds.Y = float32(style.Top)
}

// resolve recomputes cached styles and bounds if the stylesheet or nodes changed.
func (e *Engine) resolve() {
	if e.cacheValid {
		return
	}
	e.cachedStyles = make([]ComputedStyle, len(e.nodes))
	for i, n := range e.nodes {
		e.cachedStyles[i] = ResolveProps(e.resolveProps(n))
		resolveBounds(n, e.cachedStyles[i])
	}
	e.cacheValid = true
}

// rect returns node i's on-screen rectangle, applying percentage positioning.
func (e *Engine) rect(i int, screenW, screenH int32) (x, y, w, h int32) {
	n, style := e.nodes[i], e.cachedStyles[i]
	w, h = int32(n.Bounds.Width), int32(n.Bounds.Height)
	x, y = int32(n.Bounds.X), int32(n.Bounds.Y)
	if style.LeftPct >= 0 {
		x = (screenW - w) * style.LeftPct / 100
	}
	if style.TopPct >= 0 {
		y = (screenH - h) * style.TopPct / 100
	}
	return x, y, w, h
}

// Draw draws all nodes in order: background, 1px border, then text.
func (e *Engine) Draw() {
	e.resolve()
	screenW, screenH := int32(rl.GetScreenWidth()), int32(rl.GetScreenHeight())
	for i, n := range e.nodes {
		style := e.cachedStyles[i]
		x, y, w, h := e.rect(i, screenW, screenH)
		if style.Background.A > 0 {
			rl.DrawRectangle(x, y, w, h, style.Background)
		}
		if style.HasBorder && w > 0 && h > 0 {
			rl.DrawRectangleLines(x, y, w, h, style.Border)
		}
		if n.Text != "" {
			pad := style.Padding
			if pad <= 0 {
				pad = 4
			}
			DrawText(e.font, n.Text, x+pad, y+pad, defaultFontSize, style.Color)
		}
	}
}

// HitTest returns the topmost node containing the screen point, using the layout of the last
// SetNodes call.
func (e *Engine) HitTest(screenX, screenY int32) (*Node, bool) {
	e.resolve()
	screenW, screenH := int32(rl.GetScreenWidth()), int32(rl.GetScreenHeight())
	for i := len(e.nodes) - 1; i >= 0; i-- {
		x, y, w, h := e.rect(i, screenW, screenH)
		if w > 0 && h > 0 && screenX >= x && screenX < x+w && screenY >= y && screenY < y+h {
			return e.nodes[i], true
		}
	}
	return nil, false
}
