package ui

import (
	"os"

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

// SetStylesheet sets the stylesheet directly (e.g. from embedded or merged CSS).
func (e *Engine) SetStylesheet(sheet *Stylesheet) {
	e.sheet = sheet
	e.cacheValid = false
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

// AddNode appends a node. Nodes are drawn in order.
func (e *Engine) AddNode(n *Node) {
	e.nodes = append(e.nodes, n)
	e.cacheValid = false
}

// SetNodes replaces all nodes.
func (e *Engine) SetNodes(nodes []*Node) {
	e.nodes = nodes
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

// Draw draws all nodes: for each node, resolve style (cached), update bounds from style, then draw background, border, and text.
func (e *Engine) Draw() {
	screenW := int32(rl.GetScreenWidth())
	screenH := int32(rl.GetScreenHeight())
	if !e.cacheValid {
		e.cachedStyles = make([]ComputedStyle, len(e.nodes))
		for i, n := range e.nodes {
			props := e.resolveProps(n)
			e.cachedStyles[i] = ResolveProps(props)
			resolveBounds(n, e.cachedStyles[i])
		}
		e.cacheValid = true
	}
	for i, n := range e.nodes {
		style := e.cachedStyles[i]
		w := int32(n.Bounds.Width)
		h := int32(n.Bounds.Height)
		x := int32(n.Bounds.X)
		y := int32(n.Bounds.Y)
		if style.LeftPct >= 0 {
			x = (screenW - w) * style.LeftPct / 100
		}
		if style.TopPct >= 0 {
			y = (screenH - h) * style.TopPct / 100
		}

		// Background
		if style.Background.A > 0 {
			rl.DrawRectangle(x, y, w, h, style.Background)
		}
		// Border (1px)
		if style.HasBorder && w > 0 && h > 0 {
			rl.DrawRectangleLines(x, y, w, h, style.Border)
		}
		// Text (for label-type or any node with text)
		if n.Text != "" {
			pad := style.Padding
			if pad <= 0 {
				pad = 4
			}
			textX := x + pad
			textY := y + pad
			if e.font.Texture.ID != 0 {
				rl.DrawTextEx(e.font, n.Text, rl.NewVector2(float32(textX), float32(textY)), float32(defaultFontSize), 1, style.Color)
			} else {
				rl.DrawText(n.Text, textX, textY, defaultFontSize, style.Color)
			}
		}
	}
}

// HasStylesheet returns whether a CSS file has been loaded.
func (e *Engine) HasStylesheet() bool {
	return e.sheet != nil && len(e.sheet.Rules) > 0
}

// Stylesheet returns the current stylesheet (may be nil).
func (e *Engine) Stylesheet() *Stylesheet {
	return e.sheet
}
