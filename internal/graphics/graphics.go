package graphics

import rl "github.com/gen2brain/raylib-go/raylib"

// Run starts the window and main loop. Each frame it calls update (e.g. input), then clears the screen and calls draw (e.g. UI).
// This keeps the graphics layer separate from chat/terminal or other screen content.
// Window is fullscreen (FlagFullscreenMode). ESC toggles terminal; close via window button.
func Run(update, draw func()) {
	rl.SetConfigFlags(rl.FlagFullscreenMode)
	rl.InitWindow(int32(rl.GetMonitorWidth(0)), int32(rl.GetMonitorHeight(0)), "raylib [core] example - basic window")
	defer rl.CloseWindow()

	rl.SetExitKey(rl.KeyNull) // ESC is used to toggle terminal, not to quit; close via window button
	rl.SetTargetFPS(60)

	for !rl.WindowShouldClose() {
		update()

		rl.BeginDrawing()
		rl.ClearBackground(rl.Black)
		draw()
		rl.EndDrawing()
	}
}
