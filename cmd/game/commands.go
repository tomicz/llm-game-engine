package main

import (
	"fmt"

	"game-engine/internal/commands"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// registerCommands registers every terminal command. Registration order is the order shown by
// cmd help and in the AI agent's prompt.
func (a *App) registerCommands() {
	a.registerDisplayCommands()
	a.registerObjectCommands()
	a.registerAssetCommands()
	a.registerAICommands()
	a.commands.Register(commands.Command{
		Name: "help", Usage: "[command]", Manual: true,
		Help: "List commands, or show one command's usage.",
		Run: func(args []string) error {
			if len(args) > 0 {
				c, ok := a.commands.Lookup(args[0])
				if !ok {
					return fmt.Errorf("unknown command: %s", args[0])
				}
				a.log.Log(fmt.Sprintf("cmd %s — %s", c.Synopsis(), c.Help))
				return nil
			}
			for _, c := range a.commands.Commands() {
				a.log.Log("cmd " + c.Synopsis())
			}
			a.log.Log("Type cmd help <command> for details. Lines without cmd go to the AI.")
			return nil
		},
	})
}

// toggle registers a command taking --on or --off and calls set with the chosen value.
func (a *App) toggle(name, on, off, help string, set func(bool)) {
	c := commands.Command{Name: name, Usage: "--" + on + " | --" + off, Help: help}
	c.Run = func(args []string) error {
		v, ok, err := commands.ParseToggle(name, args, on, off)
		if err != nil {
			return err
		}
		if !ok {
			return c.UsageError()
		}
		set(v)
		return nil
	}
	a.commands.Register(c)
}

// registerDisplayCommands registers overlays, window, lighting, and screenshot commands.
func (a *App) registerDisplayCommands() {
	a.toggle("grid", "show", "hide", "Show or hide the 3D editor grid.", func(v bool) {
		a.scene.SetGridVisible(v)
		a.savePrefs()
	})
	a.toggle("fps", "show", "hide", "Show or hide the FPS counter.", func(v bool) {
		a.debug.SetShowFPS(v)
		a.savePrefs()
	})
	a.toggle("memalloc", "show", "hide", "Show or hide heap memory usage under the FPS counter.", func(v bool) {
		a.debug.SetShowMemAlloc(v)
		a.savePrefs()
	})
	a.toggle("window", "fullscreen", "windowed", "Switch between fullscreen and windowed mode.", func(full bool) {
		if full != rl.IsWindowFullscreen() {
			rl.ToggleFullscreen()
		}
	})

	lighting := commands.Command{Name: "lighting", Usage: "noon | sunset | night", Help: "Set the time-of-day light direction."}
	lighting.Run = func(args []string) error {
		if len(args) != 1 {
			return lighting.UsageError()
		}
		return a.scene.SetLighting(args[0])
	}
	a.commands.Register(lighting)

	a.commands.Register(commands.Command{
		Name: "screenshot", Help: "Save the current view to screenshot.png in the working directory.",
		Run: func([]string) error {
			rl.TakeScreenshot("screenshot.png")
			a.log.Log("Screenshot saved: screenshot.png")
			return nil
		},
	})
}
