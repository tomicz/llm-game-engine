package ui

import rl "github.com/gen2brain/raylib-go/raylib"

// DrawText draws text with font, or with raylib's default font when font is not loaded
// (zero texture ID).
func DrawText(font rl.Font, text string, x, y int32, size float32, color rl.Color) {
	if font.Texture.ID != 0 {
		rl.DrawTextEx(font, text, rl.NewVector2(float32(x), float32(y)), size, 1, color)
		return
	}
	rl.DrawText(text, x, y, int32(size), color)
}

// MeasureText returns the width of text as DrawText would draw it.
func MeasureText(font rl.Font, text string, size float32) int32 {
	if font.Texture.ID != 0 {
		return int32(rl.MeasureTextEx(font, text, size, 1).X)
	}
	return rl.MeasureText(text, int32(size))
}
