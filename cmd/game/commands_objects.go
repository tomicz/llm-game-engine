package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"game-engine/internal/commands"
	"game-engine/internal/primitives"
	"game-engine/internal/scene"
)

const queryUsage = "<position> | [<color>] <type> [<position>] | <name> [<position>]"

// registerObjectCommands registers commands that create, find, edit, and remove scene objects.
func (a *App) registerObjectCommands() {
	scn := a.scene
	reg := func(c commands.Command) { a.commands.Register(c) }

	spawn := commands.Command{
		Name: "spawn", Usage: "<type> <x> <y> <z> [<sx> <sy> <sz>]",
		Help: "Add a primitive (" + strings.Join(primitives.Shapes, ", ") + ") centered at x y z, optionally scaled.",
	}
	spawn.Run = func(args []string) error {
		if len(args) != 4 && len(args) != 7 {
			return spawn.UsageError()
		}
		if !primitives.IsShape(args[0]) {
			return fmt.Errorf("unknown type %q (use: %s)", args[0], strings.Join(primitives.Shapes, ", "))
		}
		pos, err := parseVec3("position", args[1:4])
		if err != nil {
			return err
		}
		scale := [3]float32{1, 1, 1}
		if len(args) == 7 {
			if scale, err = parseVec3("scale", args[4:7]); err != nil {
				return err
			}
		}
		scn.Add(scene.Object{Type: args[0], Position: pos, Scale: scale})
		return nil
	}
	reg(spawn)

	template := commands.Command{Name: "template", Usage: "tree [<x> <y> <z>]", Help: "Spawn a preset built from primitives: tree = cylinder trunk + sphere foliage."}
	template.Run = func(args []string) error {
		if len(args) != 1 && len(args) != 4 {
			return template.UsageError()
		}
		var p [3]float32
		if len(args) == 4 {
			var err error
			if p, err = parseVec3("position", args[1:]); err != nil {
				return err
			}
		}
		if args[0] != "tree" {
			return errors.New("unknown template (use tree)")
		}
		scn.Add(
			scene.Object{Type: primitives.Cylinder, Position: p, Scale: [3]float32{0.3, 2, 0.3}},
			scene.Object{Type: primitives.Sphere, Position: [3]float32{p[0], p[1] + 1.5, p[2]}, Scale: [3]float32{1.2, 1.2, 1.2}},
		)
		a.log.Log("Spawned tree.")
		return nil
	}
	reg(template)

	del := commands.Command{
		Name:  "delete",
		Usage: "selected | look | random | name <name> | all [<type> | <color> <type> | <name>] | " + queryUsage,
		Help: "Delete objects. Positions are on screen: left, right, top, bottom, closest, farthest. " +
			"'look' deletes what the camera center points at; 'all' deletes every matching object in view.",
	}
	del.Run = func(args []string) error {
		if len(args) == 0 {
			return del.UsageError()
		}
		var target *scene.Object
		var err error
		switch args[0] {
		case "selected":
			target, err = scn.RequireSelected()
		case "look", "camera":
			target, err = scn.ObjectAtCameraCenter()
		case "random":
			target, err = scn.Random()
		case "name":
			if len(args) < 2 {
				return errors.New("usage: cmd delete name <name>")
			}
			target, err = scn.FindByName(strings.Join(args[1:], " "))
		case "all":
			return a.deleteAllVisible(args[1:])
		default:
			q, ok := scene.ParseQuery(args)
			if !ok {
				return del.UsageError()
			}
			target, err = scn.FindVisible(q)
		}
		if err != nil {
			return err
		}
		scn.Delete(target.ID)
		return nil
	}
	reg(del)

	sel := commands.Command{Name: "select", Usage: "none | " + queryUsage, Help: "Select a visible object without clicking it."}
	sel.Run = func(args []string) error {
		if len(args) == 1 && strings.EqualFold(args[0], "none") {
			scn.ClearSelection()
			return nil
		}
		o, err := a.findVisible(sel, args)
		if err != nil {
			return err
		}
		scn.Select(o.ID)
		return nil
	}
	reg(sel)

	look := commands.Command{Name: "look", Usage: queryUsage, Help: "Point the camera at a visible object (does not change the selection)."}
	look.Run = func(args []string) error {
		o, err := a.findVisible(look, args)
		if err != nil {
			return err
		}
		scn.LookAt(o)
		return nil
	}
	reg(look)

	reg(commands.Command{
		Name: "focus", Help: "Point the camera at the selected object. Select an object first.",
		Run: a.withSelected(func(o *scene.Object, _ []string) error {
			scn.LookAt(o)
			return nil
		}),
	})

	reg(commands.Command{
		Name: "inspect", Help: "Print the selected object's details (or the closest object in view if none is selected).",
		Run: func([]string) error {
			if o, ok := scn.Selected(); ok {
				a.log.Log(describe("Selected", o))
				return nil
			}
			o, err := scn.FindVisible(scene.Query{})
			if err != nil {
				return err
			}
			a.log.Log(describe("Closest in view", o))
			return nil
		},
	})

	reg(commands.Command{
		Name: "view", Help: "List the objects in the camera view, closest first.",
		Run: func([]string) error {
			visible := scn.ObjectsInView()
			if len(visible) == 0 {
				a.log.Log("No objects in view. Move the camera to look at primitives.")
				return nil
			}
			a.log.Log(fmt.Sprintf("%d object(s) in view (closest first):", len(visible)))
			for _, v := range visible {
				a.log.Log(fmt.Sprintf("  %s — %s — distance %.2f — screen (%.0f, %.0f)",
					v.Object.Label(), v.Object.Type, v.Distance, v.ScreenPos.X, v.ScreenPos.Y))
			}
			return nil
		},
	})

	reg(commands.Command{
		Name: "color", Usage: "<r> <g> <b>", Help: "Set the selected object's color, each component 0-1 (1 0 0 = red). Select an object first.",
		Run: a.withSelected(func(o *scene.Object, args []string) error {
			c, err := parseVec3("color", args)
			if err != nil {
				return err
			}
			for _, v := range c {
				if v < 0 || v > 1 {
					return errors.New("color components must be 0-1")
				}
			}
			o.Color = c
			return nil
		}),
	})

	reg(commands.Command{
		Name: "duplicate", Usage: "[<n>]", Help: "Clone the selected object n times (default 1, max 20), each 2 units further along X. Select an object first.",
		Run: a.withSelected(func(o *scene.Object, args []string) error {
			n := 1
			if len(args) > 0 {
				if v, err := strconv.Atoi(args[0]); err == nil && v >= 1 {
					n = v
				}
			}
			count, err := scn.Duplicate(o.ID, n, [3]float32{2, 0, 0})
			if err != nil {
				return err
			}
			a.log.Log(fmt.Sprintf("Duplicated %d time(s).", count))
			return nil
		}),
	})

	reg(commands.Command{
		Name: "name", Usage: "<name>", Help: "Name the selected object (used by name-based commands). Select an object first.",
		Run: a.withSelected(func(o *scene.Object, args []string) error {
			if len(args) == 0 {
				return errors.New("usage: cmd name <name>")
			}
			o.Name = strings.Join(args, " ")
			return nil
		}),
	})

	reg(commands.Command{
		Name: "motion", Usage: "bob | off", Help: "Animate the selected object: bob moves it gently up and down; off stops it. Select an object first.",
		Run: a.withSelected(func(o *scene.Object, args []string) error {
			switch strings.Join(args, " ") {
			case "bob":
				o.Motion = "bob"
			case "off":
				o.Motion = ""
			default:
				return errors.New("usage: cmd motion bob | off")
			}
			return nil
		}),
	})

	reg(commands.Command{
		Name: "physics", Usage: "on | off", Help: "Turn falling and collision on or off for the selected object. Select an object first.",
		Run: a.withSelected(func(o *scene.Object, args []string) error {
			if len(args) != 1 || (args[0] != "on" && args[0] != "off") {
				return errors.New("usage: cmd physics on | off")
			}
			o.SetPhysics(args[0] == "on")
			return nil
		}),
	})

	gravity := commands.Command{Name: "gravity", Usage: "<y>", Help: "Set gravity along Y: -9.8 is normal, 0 is zero-g, positive pulls up."}
	gravity.Run = func(args []string) error {
		if len(args) != 1 {
			return gravity.UsageError()
		}
		g, err := parseFloats("gravity", args)
		if err != nil {
			return err
		}
		scn.SetGravity([3]float32{0, g[0], 0})
		return nil
	}
	reg(gravity)

	reg(commands.Command{Name: "undo", Help: "Revert the last add or delete (one level; an AI request counts as one step).", Run: func([]string) error { return scn.Undo() }})
	reg(commands.Command{Name: "save", Help: "Save the scene, including spawned objects, to its YAML file.", Run: func([]string) error { return scn.Save() }})
	reg(commands.Command{Name: "newscene", Help: "Remove every object and save the empty scene.", Run: func([]string) error { return scn.Clear() }})
}

// withSelected adapts a command body that operates on the selected object.
func (a *App) withSelected(fn func(o *scene.Object, args []string) error) func([]string) error {
	return func(args []string) error {
		o, err := a.scene.RequireSelected()
		if err != nil {
			return err
		}
		return fn(o, args)
	}
}

// findVisible resolves query arguments (see scene.ParseQuery) to a visible object.
func (a *App) findVisible(c commands.Command, args []string) (*scene.Object, error) {
	q, ok := scene.ParseQuery(args)
	if !ok {
		return nil, c.UsageError()
	}
	return a.scene.FindVisible(q)
}

// deleteAllVisible deletes every object in view matching args: nothing (all), a type, a color
// and type, or a name substring.
func (a *App) deleteAllVisible(args []string) error {
	var q scene.Query
	if len(args) > 0 {
		var ok bool
		q, ok = scene.ParseQuery(args)
		if !ok || q.Position != "" {
			return errors.New("usage: cmd delete all [<type> | <color> <type> | <name_substring>]")
		}
	}
	objs := a.scene.FindAllVisible(q)
	if len(objs) == 0 {
		if q.Name != "" {
			return fmt.Errorf("no objects matching %q in view", q.Name)
		}
		return errors.New("no matching objects in view")
	}
	ids := make([]scene.ObjectID, len(objs))
	for i, o := range objs {
		ids[i] = o.ID
	}
	n := a.scene.Delete(ids...)
	a.log.Log(fmt.Sprintf("Deleted %d object(s) in view.", n))
	return nil
}

func describe(label string, o *scene.Object) string {
	return fmt.Sprintf("%s: type=%s name=%q pos=[%.2f,%.2f,%.2f] scale=[%.2f,%.2f,%.2f] color=[%.2f,%.2f,%.2f] physics=%v motion=%q texture=%q",
		label, o.Type, o.Name,
		o.Position[0], o.Position[1], o.Position[2],
		o.Scale[0], o.Scale[1], o.Scale[2],
		o.Color[0], o.Color[1], o.Color[2],
		o.PhysicsEnabled(), o.Motion, o.Texture)
}
