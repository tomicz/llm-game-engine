# Go Project Layout & Best Practices

Summary of common Go folder structure and architecture conventions (from [golang-standards/project-layout](https://github.com/golang-standards/project-layout), [Go docs](https://go.dev/doc/modules/layout), and practical guides). **The Go team recommends starting simple**—add structure when you need it, not before.

---

## The only compiler-enforced rule: `internal/`

Packages under **`internal/`** cannot be imported from outside your module. Use it for:

- Private implementation details
- Code you don't want to promise as a stable API
- Clear boundary between "public contract" and "implementation"

---

## Core directories

| Directory   | Purpose |
|------------|---------|
| **`cmd/`** | Entry points for executables. One subdir per binary, e.g. `cmd/game/main.go`. Keep `main` thin: wire deps and call a `Run()` (or similar). |
| **`internal/`** | Private packages. Only code inside your module can import them. Use for app/engine logic, graphics, config, etc. |
| **`pkg/`** | Optional. Public, reusable packages. The Go team often skips it—import paths get longer and it's not required. Use only if you explicitly want "this is for external use." |

---

## Patterns by project type

- **Application only (CLI, game, server):**  
  `cmd/<binary>/main.go` + `internal/` for all implementation. No need for root-level packages.

- **Library only:**  
  Exportable code at repo root (or under one package). Use `internal/` for private helpers.

- **Library + CLI:**  
  Library at root (or one package), CLI in `cmd/<tool>/main.go`, shared private code in `internal/`.

---

## Best practices

1. **Start minimal:** `go.mod` + `main.go` is enough for small projects.
2. **Thin `main`:** In `cmd/*/main.go`, only parse flags, load config, and call into `internal` packages.
3. **Tests next to code:** `*_test.go` in the same package as the code under test.
4. **Test fixtures:** Use a `testdata/` directory in the package if needed.
5. **Name packages by responsibility:** Avoid catch-all names like `util`, `helpers`, `misc`. Prefer `graphics`, `input`, `config`.
6. **No `src/`:** Go projects don't use a top-level `src/` directory.
7. **One file, one job:** Split by responsibility (e.g. `client.go`, `types.go`, `errors.go`) rather than one giant file.

---

## What this project uses

### `cmd/game/` — the application

`main.go` only loads `.env`, optionally starts pprof, and calls `NewApp().Run()`. The rest of the package wires the engine together:

| File | Role |
|------|------|
| `app.go` | `App` struct (all subsystems), `NewApp` (load prefs, register commands, set up AI), the frame callbacks `init` / `update` / `draw`, the main-thread task queue (`background`, `runTasks`), prefs saving. |
| `ai.go` | LLM provider detection and client creation, `setProvider`, natural-language handling, `provider` / `model` commands. |
| `commands.go` | `registerCommands`, `help`, display commands (grid, fps, memalloc, window, lighting, screenshot). |
| `commands_objects.go` | Object commands: spawn, template, delete, select, look, focus, inspect, view, color, duplicate, name, motion, physics, gravity, undo, save, newscene. |
| `download.go` | Asset commands: download, texture, skybox, font, heightmap, terrain_repeat. |
| `argparse.go` | Small argument parsing helpers (`parseFloats`, `parseVec3`). |

### `internal/` — engine packages

| Package | Role |
|---------|------|
| **`graphics`** | Window and main loop: `Run(init, update, draw)`. Opens a fullscreen window, calls `init` once the GPU context exists, then `update` and `draw` every frame. |
| **`scene`** | The 3D world: objects, camera, selection, undo, physics sync, editor dragging, queries, and rendering (skybox, primitives, terrain, grid, gizmos). See **Scene package** below. |
| **`primitives`** | Built-in shapes (`cube`, `sphere`, `cylinder`, `plane`) and the terrain mesh, drawn with one shared lit shader. See **3D primitives and scene YAML**. |
| **`physics`** | Pure-Go AABB physics: bodies, gravity, collision. See [physics.md](physics.md). |
| **`mapgen`** | Procedural heightmap terrain: `GenerateTerrain(opts)` returns a mesh built from fractal value noise. |
| **`terminal`** | Chat/command bar: input handling and drawing. `cmd …` lines go to the command registry; other lines go to `OnNaturalLanguage`. |
| **`commands`** | Command registry: `Command`, `Registry`, `Parse`, `ParseToggle`, `NewFlagSet`. See **In-game command system**. |
| **`agent`** | Natural language → LLM → JSON actions → handlers. See **Natural language and AI agent**. |
| **`llm`** | Minimal chat clients: `OpenAICompat` (OpenAI, Groq) and `Ollama`. |
| **`assets`** | Resolves asset paths whether the game runs from the repo root or `cmd/game` (`Find`, `Candidates`). |
| **`ui`** | Primitive CSS UI (parser, styles, nodes, inspector) and the shared `DrawText` / `MeasureText` helpers. See [ui.md](ui.md). |
| **`debug`** | FPS and memory overlays. See **Debug system**. |
| **`engineconfig`** | Engine preferences persisted to `config/engine.json`. See **Engine config persistence**. |
| **`logger`** | Terminal log (memory + `logs/terminal.txt`) and engine log (`logs/engine_log.txt`). See **Log files**. |
| **`download`**, **`googlefonts`**, **`fonts`** | HTTP downloads (textures, skyboxes, fonts), Google Fonts lookup, and local font search. |
| **`env`** | Loads `.env` (API keys) at startup. |

### Other directories

- **`docs/`** — This file, [physics.md](physics.md), [ui.md](ui.md).
- **`assets/ui/`** — UI stylesheets (e.g. `default.css`).
- **`assets/scenes/`** — Scene files (YAML). `default.yaml` is loaded at startup.
- **`assets/skybox/`**, **`assets/fonts/`**, **`assets/textures/`** — Optional runtime assets.

Draw order each frame: **Scene → Debug → UI → Terminal**, so the terminal is always on top when open.

---

## Threading model

raylib and all engine state (scene, UI, terminal, App fields) are used **only on the main thread**. Slow work — LLM requests, image and font downloads — runs on goroutines started by `background` in `cmd/game/app.go`:

```go
background(a, func() (string, error) {
	return download.Download(url, textureDownloadDir) // goroutine: no raylib, no scene
}, func(path string, err error) {
	scn.SetTexture(o, path) // main thread, start of a later frame
})
```

`background` runs the work function on a goroutine and queues the `done` callback on the `App.tasks` channel; `update` drains that queue (`runTasks`) at the start of every frame. The logger is the only type shared with goroutines and is safe for concurrent use.

---

## Scene package

`internal/scene` is split by responsibility:

| File | Contents |
|------|----------|
| `scene.go` | `Scene` struct, `New`, object lifecycle (`Add`, `Delete`, `Duplicate`, `Object`, `Objects`), selection (`Selected`, `RequireSelected`, `Select`, `ClearSelection`), `LookAt`, lighting, gravity, `Update`, physics sync (`stepPhysics`). |
| `object.go` | `Object` (the YAML-serialized object), `ObjectID`, helpers (`PhysicsEnabled`, `SetPhysics`, `HasColor`, `Label`, bounds). |
| `undo.go` | One-level undo log, `Group`, `Undo`. |
| `persist.go` | Loading the default scene file, `Save`, `Clear`. |
| `query.go` | `ObjectsInView`, `Query` / `ParseQuery`, `FindVisible`, `FindAllVisible`, `ObjectAtCameraCenter`, `FindByName`, `Random`, `ViewSummary`. |
| `editor.go` | `UpdateEditor`: click-to-select and drag by box face. |
| `render.go` | `Draw`, texture cache and `SetTexture`, gizmo arrows, editor grid. |
| `skybox.go` | Skybox loading (panorama or cubemap), `SetSkyboxPath`, drawing. |
| `terrain.go` | `SetTerrain`, `SetTerrainTextureRepeat`. |
| `awareness.go` | `ViewAwareness`: optional enter/leave-view callbacks. |

### Object IDs

Every object gets an `ObjectID` when it is loaded or added. IDs are never reused and are **not** saved to YAML (`yaml:"-"`). Selection, undo, view awareness, and background work (e.g. a texture download) refer to objects by ID, so deleting other objects never makes them point at the wrong one. Undo restores objects with their original IDs.

### Undo

Undo is **one level**: `cmd undo` reverts the last step. A step is every add and delete made by one action: `Add(objs...)` and `Delete(ids...)` are each one step, and `Scene.Group(fn)` merges everything inside `fn` into one step. The app wraps each natural-language request in a group, so undo reverts a whole AI request (e.g. a forest), and `delete all` / `template` / `duplicate` each undo at once. Deleted objects are restored at their original position in the list. `Clear` (`cmd newscene`) empties the undo log.

---

## 3D primitives and scene YAML

**Scene data** is loaded from YAML (`assets/scenes/default.yaml`, found via `assets.Find` from the repo root or `cmd/game`). The scene does not hardcode objects; it loads a list of **objects** and draws each via **`internal/primitives/`**.

- **Primitive types:** `cube`, `sphere`, `cylinder`, `plane`, defined in a table in `internal/primitives/registry.go` (`primitives.Shapes`). Meshes are created **lazily** on first draw so GPU resources exist after the window/OpenGL context is ready. Adding a shape = one entry in `shapeDefs`.
- **Shared shader:** all primitives use one plain and one textured lit material (directional light + ambient + specular). Lighting constants are uploaded once; camera position and light direction once per frame (`Registry.SetView`). Uniform locations are cached.
- **Default size:** Cube 1×1×1, sphere diameter 1, cylinder diameter 1 and height 1, plane 1×1 in XZ. `scale` is the size in world units; zero components mean 1. Planes spawned with a Y scale of 1 get a thickness of 0.1.
- **Origin at center:** `position` is the **center** of each primitive. raylib's cylinder has its base at Y=0, so it has a model-space center offset; the model matrix applies center offset, then scale, then translation.
- **Terrain:** `cmd heightmap` builds a mesh with `mapgen.GenerateTerrain` and installs it with `Scene.SetTerrain`. The terrain is an object of type `terrain` (static, centered on the origin in XZ, base at Y=0, scale = terrain size) that carries its color, texture, collider, and selection box; the mesh is drawn to fill that box. Deleting it keeps the mesh so undo can restore it; `newscene` or a new heightmap releases it.
- **Scene file format:** YAML with `objects:` — a list of `type`, `position` [x,y,z], optional `scale` [x,y,z], optional `color` [r,g,b] (0-1), optional `name`, optional `motion` (`bob`), optional `physics` (`false` = static), optional `texture` (image path). Example: `objects: [{ type: cube, position: [0,0,0], scale: [1,1,1] }, ...]`.
- **Persistence:** `gopkg.in/yaml.v3`. `cmd save` writes the current scene (including runtime-spawned objects) back to the file it was loaded from, in the same format.

---

## 3D editor grid

The scene draws a Unity-style grid on the **XZ plane** (Y = 0, raylib Y-up):

- **Minor lines** every 1 unit, dim gray; **major lines** every 10 units, brighter gray; extent ±50 on X and Z.
- **Axis lines** through the origin: **X** red, **Y** green, **Z** blue.

Tunables are constants in `internal/scene/render.go`: `gridExtent`, `gridMinorStep`, `gridMajorStep`, and the alpha values for minor/major/axis lines. **Grid visibility** is toggled with `cmd grid --show` / `cmd grid --hide` (`Scene.SetGridVisible`) and persisted in the engine config.

---

## Scene editor (terminal mode)

When the **terminal is open** (ESC; cursor visible), the scene runs in editor mode: you can **select** and **move** primitives. Physics is not stepped. Skybox and grid are not selectable or movable.

- **Selection:** Click an object (ray vs object AABB). The selected object gets a **yellow bounding box** and **red (X), green (Y), blue (Z) direction arrows** at its center. The arrows are **visual only** (no picking); movement is by box face. Clicking empty space clears the selection.
- **Drag mode from box face:** Which face you click decides how you move:
  - **Top or bottom face** (horizontal) → drag on the **XZ plane**. The point you clicked stays under the cursor.
  - **Any of the four side faces** (vertical) → drag **up/down** (Y) from the vertical mouse movement.
- **UI first:** a click that lands on the UI overlay (e.g. the inspector) is handled by the UI and does not select or deselect scene objects. Clicking the inspector's **Physics** row toggles physics on the selected object.
- **Implementation:** `internal/scene/editor.go`: `UpdateEditor(inputTopY)` handles pick and drag, ignoring the mouse below `inputTopY` (the terminal bar, from `Terminal.BarTop`). Face classification uses the ray–box hit normal (Y ≈ ±1 → top/bottom, else side). `Draw(editorMode)` draws the outline and arrows only in editor mode.

---

## In-game command system

The terminal interprets lines that start with **`cmd `** (space required) as commands. The rest of the line is split on whitespace; the first token is the **command name**, the rest are its **arguments**. `cmd help` lists every command; `cmd help <name>` shows one command's usage and description.

- **Parsing:** `commands.Parse(line)` returns `(args []string, ok bool)`. Example: `cmd grid --show` → `args = ["grid", "--show"]`, `ok = true`.
- **Commands describe themselves:**

  ```go
  type Command struct {
  	Name   string
  	Usage  string // argument synopsis, e.g. "--show | --hide"
  	Help   string // one line; shown by cmd help and given to the AI agent
  	Manual bool   // user-only: the AI agent may not run it
  	Run    func(args []string) error
  }
  ```

- **Registry:** `reg.Register(cmd)` adds a command (duplicate names panic); `reg.Execute(args)` runs `args[0]` with `args[1:]`; `reg.Commands()` lists them in registration order (the order used by `cmd help` and the agent prompt).
- **Arguments:** `Run` receives the raw arguments, so positional values like `-9.8` work. Commands that take flags parse them per call: `commands.ParseToggle(name, args, "show", "hide")` for `--show`/`--hide` pairs, or `commands.NewFlagSet(name)` for anything else (e.g. `heightmap`). Standard Go flag syntax applies (`-flag`, `--flag`, `-flag=value`).
- **Errors** are returned and shown in the terminal. `cmd.UsageError()` builds a `usage: cmd …` error from the command's own `Usage`.

**Adding a command:** register it in the matching `cmd/game/commands*.go` file. The `Help` text is what the AI agent sees, so say when a command needs a selection.

```go
a.commands.Register(commands.Command{
	Name: "color", Usage: "<r> <g> <b>",
	Help: "Set the selected object's color, each component 0-1 (1 0 0 = red). Select an object first.",
	Run: a.withSelected(func(o *scene.Object, args []string) error {
		c, err := parseVec3("color", args)
		if err != nil {
			return err
		}
		o.Color = c
		return nil
	}),
})
```

`withSelected` returns `scene.ErrNoSelection` when nothing is selected. Mark a command `Manual: true` if the AI must not run it (currently `model`, `provider`, `help`).

**Built-in commands:**

| Command | Arguments | Effect |
|---------|-----------|--------|
| `help` | `[command]` | List commands, or show one command's usage. |
| `grid` | `--show` \| `--hide` | Show or hide the 3D editor grid (XZ plane). Persisted. |
| `fps` | `--show` \| `--hide` | Show or hide the FPS counter (top-right, green). Off by default. Persisted. |
| `memalloc` | `--show` \| `--hide` | Show or hide heap allocation (under FPS). Off by default. Persisted. |
| `window` | `--fullscreen` \| `--windowed` | Switch display mode. |
| `lighting` | `noon` \| `sunset` \| `night` | Set the directional light profile. |
| `screenshot` | — | Capture the current view to `screenshot.png` in the working directory. |
| `spawn` | `<type> <x> <y> <z> [sx sy sz]` | Add a primitive (cube, sphere, cylinder, plane) centered at the position; optional scale. |
| `template` | `tree [x y z]` | Spawn a preset (tree = cylinder trunk + sphere foliage). One undo step. |
| `delete` | `selected` \| `look` \| `random` \| `name <name>` \| `all [type \| color type \| name]` \| `<position>` \| `[color] <type> [position]` \| `<name> [position]` | Remove object(s). By position in view (`left`, `right`, `top`, `bottom`, `closest`, `farthest`), by type/color (`plane`, `red cube`), by type + position (`cube right`), by name substring (+ position), or in bulk (`all`, `all cube`, `all building`). `delete all` is one undo step. |
| `select` | `none` \| `<position>` \| `[color] <type> [position]` \| `<name> [position]` | Select a visible object without clicking. |
| `look` | same as `select` (without `none`) | Point the camera at a visible object (does not change the selection). |
| `focus` | — | Point the camera at the selected object. |
| `inspect` | — | Print type, name, position, scale, color, physics, motion, texture of the selected object (or the closest in view). |
| `view` | — | List objects in the camera view (label, type, distance, screen position), closest first. |
| `color` | `<r> <g> <b>` (0-1) | Set the selected object's color. |
| `duplicate` | `[N]` (default 1, max 20) | Clone the selected object N times, 2 units apart on X. One undo step. |
| `name` | `<name>` | Name the selected object (may contain spaces). |
| `motion` | `bob` \| `off` | Gentle Y oscillation on the selected object, or stop it. |
| `physics` | `on` \| `off` | Enable or disable physics (gravity/collision) on the selected object. |
| `gravity` | `<y>` (e.g. `-9.8`, `0`) | Set physics gravity along Y. |
| `undo` | — | Revert the last add or delete step (one level). |
| `save` | — | Write the current scene (including spawned objects) to its YAML file. |
| `newscene` | — | Remove every object (and the terrain) and save the empty scene. |
| `download` | `image <url>` | Download an image in the background and apply it as the selected object's texture. |
| `texture` | `<path>` | Apply an image file (e.g. `assets/textures/downloaded/foo.png`) as the selected object's texture. |
| `skybox` | `<url>` | Download an image in the background and set it as the skybox (2:1 panorama or cubemap layout). |
| `font` | `[name]` | Set the UI font by family name or path under `assets/fonts/`; downloads from Google Fonts if missing. No argument shows the current font. Persisted. |
| `heightmap` | `[--w N] [--d N] [--tile S] [--h H] [--seed N]` | Generate a static procedural terrain mesh (replaces existing terrain). |
| `terrain_repeat` | `<u> <v>` | Tile the terrain texture u×v times. |
| `provider` | `[ollama \| openai \| groq]` | Switch the AI provider (user-only). Persisted. |
| `model` | `[name]` | Set the AI model (user-only). Persisted. |

Examples: `cmd grid --hide`, `cmd fps --show`, `cmd color 1 0 0`, `cmd lighting sunset`, `cmd gravity -9.8`, `cmd template tree 4 0 4`, `cmd undo`.

---

## Natural language and AI agent

When the user types a line in the terminal that **does not** start with `cmd `, it is treated as **natural language** and sent to an LLM. The reply is parsed as JSON with an `actions` array; each action is applied through a **handler registry** that uses the same scene and command APIs as the terminal. The LLM never types into the terminal.

- **Flow:**
  1. The terminal calls `App.handleNaturalLanguage` on the main thread, which captures the current model and a **view summary** (`Scene.ViewSummary`, e.g. `Visible (left to right): 1. "Tower" (cube) (left), 2. plane (center)`).
  2. `Agent.Plan` runs in the background: it sends the system prompt, the view summary, and the request to the LLM and parses the actions. It does not touch game state. Requests time out after 3 minutes.
  3. On the main thread, `Agent.Apply` runs the actions inside `Scene.Group`, so the whole request is one undo step, and the summary (or per-action errors) is logged.
- **Actions:** `add_object` (one primitive), `add_objects` (count + pattern `grid` / `line` / `random`, optional random scale range and colors), and `run_cmd` (any terminal command by its args). New action types = `Agent.RegisterHandler`.
- **Prompt:** built in `internal/agent/prompt.go`. The list of commands the model may use is generated from the command registry (`Synopsis` + `Help`), so it always matches the real commands; `Manual` commands are left out and `run_cmd` refuses them. Hand-written rules map common phrasings ("create a city", "forest", "delete the one on the right") to actions.
- **Parsing:** the reply may be wrapped in markdown or text; the first JSON object is decoded. `{"actions": [...]}`, `{"actions": {...}}`, and a single top-level action are accepted.
- **Provider and model:** `cmd provider <name>` and `cmd model <name>`, both persisted in `config/engine.json`.

---

## Environment and API keys

API keys are read from a **`.env`** file. **`.env` is in `.gitignore`** — do not commit or push it; keep API keys local only.

- Copy `.env.example` to `.env` and set e.g. `GROQ_API_KEY=...` or `OPENAI_API_KEY=...`. Optionally set `OLLAMA_BASE_URL` (default `http://localhost:11434`).
- **Provider on first run** (no provider saved yet): Groq if `GROQ_API_KEY` is set, else OpenAI if `OPENAI_API_KEY` is set, else a local Ollama. After that the provider and model are persisted; switch with `cmd provider` / `cmd model`.
- Default models: Groq `llama-3.3-70b-versatile`, OpenAI `gpt-4o-mini`, Ollama `qwen3-coder:30b`.
- The game loads `.env` from the working directory and from `../../.env` (when run from `cmd/game`). Without a usable provider the game runs normally; natural-language lines log a hint instead of calling an LLM.
- **Never add API keys to the repository or to source code.**

---

## Debug system

**`internal/debug/`** provides runtime debugging overlays. All overlays are **hidden by default** and are toggled via the in-game terminal (e.g. `cmd fps --show` / `cmd fps --hide`).

- **FPS** — Frames per second drawn at the **top-right** of the screen in **green** when enabled. Uses raylib's `GetFPS()`.
- **Mem** — Heap allocation (Go runtime) drawn **under FPS** in **green** when enabled (`cmd memalloc --show`). Uses `runtime.ReadMemStats()`; displayed as MiB.

The debug system is drawn after the 3D scene and before the UI and terminal. New overlays go in `internal/debug/debug.go`, with their commands registered in `cmd/game/commands.go`. FPS and Mem text are only recomputed every 30 frames to limit allocations.

**Memory profiling:** Run with `DEBUG_PPROF=1` to expose pprof on `http://localhost:6060`. Then e.g. `go tool pprof -http=:8080 http://localhost:6060/debug/pprof/heap` to inspect heap usage and find allocation hotspots.

**Camera awareness logging:** Run with `CAMERA_AWARENESS=1` to log objects entering and leaving the view (to stderr and the engine log).

---

## Primitive CSS UI system

**`internal/ui/`** provides a minimal, CSS-driven UI layer. It is **primitive**: no shadows, no rounded corners, no layout engine—just selectors, a small property set, and explicit position/size. Full reference: [ui.md](ui.md).

- **Draw order:** Scene → Debug → **UI** → Terminal. So scene UI sits above the 3D view and debug, and the terminal always renders on top when open.
- **Assets:** CSS lives under **`assets/ui/`** (e.g. `assets/ui/default.css`, loaded at startup).
- **Selectors:** Only `.class` and `#id`. No combinators or pseudo-classes.
- **Properties:** `background`, `color`, `border`, `width`, `height`, `left`, `top` (or `x`, `y`), `padding`. Values: hex colors (`#RGB`, `#RRGGBB`), numbers with optional `px`, and `N%` for left/top.
- **Model:** Nodes are created in code (type, class, id, optional text). The app passes the frame's nodes to `SetNodes`; styles are re-resolved only when the node list changes. `HitTest` finds the node under the mouse.
- **Inspector:** `ui.Inspector` shows the selected object's type, position, scale, physics, and texture while the terminal is open.
- **Scene binding (future):** A data layer (manifest or per-scene file) will map scene id → CSS file(s) so each scene can have its own styles.

---

## Engine config persistence

**`internal/engineconfig/`** persists engine-only preferences across runs. This is **not** for in-game save data (that is a separate, future system).

- **File:** `config/engine.json` (relative to the process working directory; e.g. `cmd/game/config/` when run from `cmd/game`). The directory is created on first save.
- **Contents:** `show_fps`, `show_memalloc`, `grid_visible` (booleans), `ai_provider`, `ai_model`, `font` (strings).
- **Defaults:** overlays off, grid on, font `Roboto/static/Roboto-Regular.ttf`, provider detected from the environment, model = the provider's default. `Load()` reads the file **over** the defaults, so keys missing from the file keep their default values. An invalid file is reported in the terminal and the defaults are used.
- **Save:** the app saves after every command that changes a persisted value (`grid`, `fps`, `memalloc`, `provider`, `model`, `font`) and once at startup.

Adding a new preference: add a field to `EnginePrefs` (and its default to `Default()`), apply it in `NewApp` in `cmd/game/app.go`, include it in `App.savePrefs`, and call `a.savePrefs()` from the command that changes it.

---

## Log files

Logs are written under **`logs/`** (relative to the process working directory; e.g. `cmd/game/logs/` when run from `cmd/game`). Both files persist after the game exits.

| File | Purpose |
|------|---------|
| **`terminal.txt`** | Every terminal line (your input and engine responses), with a timestamp. Not cleared on start. |
| **`engine_log.txt`** | Engine and raylib output. All raylib trace messages (INFO, WARNING, ERROR, etc.) are captured via `SetTraceLogCallback`; engine errors (`log.Error`) and everything written to stderr — including Go crash dumps — also go here. Not cleared on start; check it first when the game crashes. |

---

## Testing

Unit tests live next to the code (`*_test.go`) and do not need a window: scene logic (add/delete/undo, queries, persistence, physics sync, ray picking), commands, the agent (with a fake LLM client), physics, primitives' transform math, LLM clients (against `httptest` servers), config, logging, downloads, and CSS parsing.

```bash
go test -race ./...
go vet ./...
```

Rendering itself (shaders, skybox, textures) is not covered by tests; check it by running the game.

---

## Version control

**Commit** and **push** are done by the user. Do not have an agent perform git commit or push unless the user explicitly asks for it.
