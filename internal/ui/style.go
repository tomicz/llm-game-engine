package ui

import (
	"strconv"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Rule is a single CSS rule: one selector and a set of property values (raw strings).
type Rule struct {
	Selector string            // e.g. ".panel" or "#menu"
	Props    map[string]string // e.g. "background" -> "#333"
}

// Stylesheet is a list of rules (order matters: later overrides earlier).
type Stylesheet struct {
	Rules []Rule
}

// ComputedStyle holds resolved values used for drawing (raylib types where applicable).
// LeftPct/TopPct: 0–100 for percentage positioning; -1 means use Left/Top as pixels.
// Padding is the offset (in pixels) from the node's left/top when drawing text.
type ComputedStyle struct {
	Background rl.Color
	Color      rl.Color
	Border     rl.Color
	HasBorder  bool
	Width      int32
	Height     int32
	Left       int32
	Top        int32
	LeftPct    int32 // -1 = not set
	TopPct     int32 // -1 = not set
	Padding    int32 // text offset from node bounds (default 4)
}

// DefaultComputedStyle returns a minimal style (transparent background, white text, no border, zero size).
func DefaultComputedStyle() ComputedStyle {
	return ComputedStyle{
		Background: rl.NewColor(0, 0, 0, 0),
		Color:      rl.White,
		Border:     rl.Black,
		HasBorder:  false,
		Width:      0,
		Height:     0,
		Left:       0,
		Top:        0,
		LeftPct:    -1,
		TopPct:     -1,
		Padding:    4,
	}
}

// ParseHexColor parses #RGB or #RRGGBB into rl.Color (alpha 255). Returns rl.Black and false on parse error.
func ParseHexColor(s string) (rl.Color, bool) {
	s = strings.TrimSpace(s)
	if len(s) >= 4 && s[0] == '#' {
		hex := s[1:]
		var r, g, b uint8
		if len(hex) == 3 {
			// #RGB -> RR GG BB
			r = hexByte(hex[0]) * 17
			g = hexByte(hex[1]) * 17
			b = hexByte(hex[2]) * 17
		} else if len(hex) == 6 {
			r = hexByte(hex[0])<<4 + hexByte(hex[1])
			g = hexByte(hex[2])<<4 + hexByte(hex[3])
			b = hexByte(hex[4])<<4 + hexByte(hex[5])
		} else {
			return rl.Black, false
		}
		return rl.NewColor(r, g, b, 255), true
	}
	return rl.Black, false
}

func hexByte(c byte) uint8 {
	if c >= '0' && c <= '9' {
		return c - '0'
	}
	if c >= 'a' && c <= 'f' {
		return c - 'a' + 10
	}
	if c >= 'A' && c <= 'F' {
		return c - 'A' + 10
	}
	return 0
}

// ParsePx parses a number, with optional "px" suffix, to int32. Unitless is treated as pixels.
func ParsePx(s string) (int32, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "px")
	s = strings.TrimSpace(s)
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return int32(n), true
}

// ParsePct parses "N%" to int32 (0–100). Used for left/top percentage positioning.
func ParsePct(s string) (int32, bool) {
	s = strings.TrimSpace(s)
	if len(s) < 2 || s[len(s)-1] != '%' {
		return 0, false
	}
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil || n < 0 || n > 100 {
		return 0, false
	}
	return int32(n), true
}

// ResolveProps builds a ComputedStyle from a merged property map (e.g. from matching rules).
func ResolveProps(props map[string]string) ComputedStyle {
	out := DefaultComputedStyle()
	for k, v := range props {
		v = strings.TrimSpace(v)
		switch k {
		case "background":
			if c, ok := ParseHexColor(v); ok {
				out.Background = c
			}
		case "color":
			if c, ok := ParseHexColor(v); ok {
				out.Color = c
			}
		case "border":
			if c, ok := ParseHexColor(v); ok {
				out.Border = c
				out.HasBorder = true
			}
		case "width":
			if n, ok := ParsePx(v); ok {
				out.Width = n
			}
		case "height":
			if n, ok := ParsePx(v); ok {
				out.Height = n
			}
		case "left", "x":
			if pct, ok := ParsePct(v); ok {
				out.LeftPct = pct
			} else if n, ok := ParsePx(v); ok {
				out.Left = n
			}
		case "top", "y":
			if pct, ok := ParsePct(v); ok {
				out.TopPct = pct
			} else if n, ok := ParsePx(v); ok {
				out.Top = n
			}
		case "padding":
			if n, ok := ParsePx(v); ok && n >= 0 {
				out.Padding = n
			}
		}
	}
	return out
}
