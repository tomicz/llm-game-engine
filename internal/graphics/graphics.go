// Package graphics owns the window and the main loop.
package graphics

import rl "github.com/gen2brain/raylib-go/raylib"

// WindowTitle is the title of the engine window.
const WindowTitle = "LLM Game Engine"

// Run opens a fullscreen window at the primary monitor's resolution and runs the main loop until the
// window is closed. init runs once after the window (and GPU context) exists; then each frame calls
// update, clears the screen, and calls draw. ESC does not quit (the terminal uses it).
func Run(init, update, draw func()) {
	rl.SetConfigFlags(rl.FlagFullscreenMode)
	// Zero size means "use the primary monitor's resolution".
	rl.InitWindow(0, 0, WindowTitle)
	defer rl.CloseWindow()

	rl.SetExitKey(rl.KeyNull)
	rl.SetTargetFPS(60)
	init()

	for !rl.WindowShouldClose() {
		update()

		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)
		draw()
		rl.EndDrawing()
	}
}
