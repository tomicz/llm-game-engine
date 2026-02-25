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

- **`cmd/game/`** — Entry point; `main()` wires logger, terminal, scene, and graphics.
- **`internal/graphics/`** — Window, loop, clear. Calls `update`/`draw` each frame; no UI logic.
- **`internal/scene/`** — 3D scene: Camera3D and free-camera update. Draw uses BeginMode3D, **scene objects** (loaded from YAML; see **3D primitives and scene YAML** below), and a custom **editor-style grid** on the XZ plane (minor/major lines every 1/10 units, extent ±50) plus X/Y/Z axis lines (red/green/blue) through the origin; see `drawEditorGrid()` in `scene.go`.
- **`internal/primitives/`** — 3D primitive types (e.g. cube): registry, mesh cache (lazy after GL context), and draw. Scene objects reference types by name; no hardcoded primitives in the scene. See **3D primitives and scene YAML** below.
- **`internal/terminal/`** — Chat/terminal bar: input handling and drawing (uses logger and raylib). Submits lines starting with `cmd ` to the command registry; see **In-game command system** below.
- **`internal/commands/`** — In-game command system: subcommand registry, flag parsing (Go `flag.FlagSet` per command), and execution. Commands and flags are defined in code; no external config file.
- **`internal/debug/`** — Debugging overlays (e.g. FPS counter). All overlays are off by default; toggle via in-game terminal. See **Debug system** below.
- **`internal/engineconfig/`** — Engine-only preferences (debug overlays, grid visibility). Persisted to `config/engine.json`; loaded at startup, saved on every toggle. See **Engine config persistence** below.
- **`internal/logger/`** — Terminal lines (memory + file), engine/raylib log to file. See **Log files** below.
- **`internal/ui/`** — Primitive CSS-driven UI: parser, style resolution, and raylib draw. See **Primitive CSS UI system** below.
- **`docs/`** — Documentation (e.g. this file).
- **`assets/ui/`** — UI assets only (CSS files). Kept separate from other assets (skybox, etc.). See **Primitive CSS UI system** below.
- **`assets/primitives/`** — Default primitive definitions (YAML): type, default size/color per primitive. See **3D primitives and scene YAML** below.
- **`assets/scenes/`** — Scene files (YAML): list of object instances (type, position, scale). The scene loads one file (e.g. `default.yaml`) at startup and draws objects by metadata; not hardcoded.

Graphics, scene UI, and terminal are separate: graphics owns the window and loop; scene owns 3D camera and world; **UI** draws scene-based overlays from CSS; **terminal** is the chat/LLM bar and draws on top of everything when enabled. Add more `internal/*` packages as needed (e.g. `internal/input`).

---

## 3D primitives and scene YAML

**Scene data** is loaded from YAML (e.g. `assets/scenes/default.yaml`). The scene does not hardcode objects; it loads a list of **object instances** (type, position, optional scale) and draws each via **`internal/primitives/`**.

- **Primitive types:** Registered in code: `cube`, `sphere`, `cylinder` (raylib `GenMeshCube`, `GenMeshSphere`, `GenMeshCylinder`). Mesh and material are created **lazily** on first draw so GPU resources exist after the window/OpenGL context is ready. More primitives (plane, cone, etc.) can be added on demand.
- **Default size:** Cube 1×1×1, sphere diameter 1 (radius 0.5), cylinder diameter 1 and height 1 (radius 0.5). All share the same 1-unit extent for consistent defaults.
- **Origin at center:** Scene `position` is the **center** of each primitive. Cube and sphere meshes are already centered; the cylinder (raylib: base Y=0, top Y=height) gets a model-space offset so its center is at `position`.
- **Default primitives folder:** `assets/primitives/` holds YAML files (e.g. `cube.yaml`, `sphere.yaml`, `cylinder.yaml`) with type and default size/color. Used for defaults; mesh generation is driven by type name in the registry.
- **Scene file format:** YAML with `objects:` — list of `type`, `position` [x,y,z], optional `scale` [x,y,z]. Example: cube at center, sphere and cylinder beside it: `objects: [{ type: cube, position: [0,0,0], scale: [1,1,1] }, ...]`.
- **Parsing and persistence:** `gopkg.in/yaml.v3`; scene is loaded at startup from the first existing path in `scenePaths` (e.g. `assets/scenes/default.yaml`, `../../assets/scenes/default.yaml`). Saving the scene (e.g. from an editor) writes the same YAML format back. Scalable: add objects in YAML or new primitive types in code without changing the scene loader.

---

## 3D editor grid

The scene draws a Unity-style grid on the **XZ plane** (Y = 0, raylib Y-up):

- **Minor lines** every 1 unit, dim gray; **major lines** every 10 units, brighter gray; extent ±50 on X and Z.
- **Axis lines** through the origin: **X** red, **Y** green, **Z** blue.

Tunables live in `internal/scene/scene.go` as constants: `gridExtent`, `gridMinorStep`, `gridMajorStep`, and the alpha values for minor/major/axis lines. **Grid visibility** is controlled at runtime via the in-game terminal: `cmd grid --show` / `cmd grid --hide`. The scene exposes `GridVisible` and `SetGridVisible(bool)`; the grid is drawn only when `GridVisible` is true (default: true).

---

## Scene editor (terminal mode)

When the **terminal is open** (ESC; cursor visible), the scene runs in editor mode: you can **select** and **move** primitives. Skybox and grid are not selectable or movable.

- **Selection:** Click an object (ray vs object AABB). The selected object gets a **yellow bounding box** and **red (X), green (Y), blue (Z) direction arrows** at its center. The arrows are **visual only** (no picking); movement is by box face.
- **Drag mode from box face:** Which face you click decides how you move:
  - **Top or bottom face** (horizontal) → drag on the **XZ plane** (forward/back, left/right). The point you clicked stays under the cursor (offset from object center is stored so the object doesn’t teleport when you click an edge).
  - **Any of the four side faces** (vertical) → drag **up/down** (Y). Movement uses screen-space mouse delta and a sensitivity constant; mouse up = object up.
- **Implementation:** `internal/scene/scene.go`: `UpdateEditor(cursorVisible, terminalBarHeight)` handles pick and drag; face classification uses the ray–box hit normal (Y ≈ ±1 → top/bottom, else side). XZ drag uses `rayPlaneY` and `dragOffsetX`/`dragOffsetZ`; Y drag uses `lastMouseY` and `yDragSensitivity`. Draw calls `Draw(selectionVisible)` so the outline and arrows are only drawn when the terminal is open and an object is selected.
- **Commands:** `cmd spawn <type> <x> <y> <z> [sx sy sz]` adds a primitive; `cmd save` writes the current scene to YAML; `cmd newscene` clears and saves an empty scene.

---

## In-game command system

The terminal interprets lines that start with **`cmd `** (space required) as commands. The rest of the line is tokenized by spaces; the first token is the **subcommand** name, the rest are **flags and arguments** for that subcommand.

- **Parsing:** `commands.Parse(line)` returns `(args []string, ok bool)`. Example: `cmd grid --show` → `args = ["grid", "--show"]`, `ok = true`.
- **Registry:** `commands.NewRegistry()` creates an empty registry. Commands are registered in code with `reg.Register(name, *flag.FlagSet, run func() error)`. Each subcommand has its own Go `flag.FlagSet`, so you get standard flag syntax: `-flag`, `--flag`, `-flag=value`, etc.
- **Execution:** `reg.Execute(args)` looks up the subcommand, parses `args[1:]` with that command’s FlagSet, then calls its `Run()`. Errors (unknown command, bad flags) are returned and shown in the terminal.

**Adding a command:** In `cmd/game/main.go` (or wherever you wire the registry), create a `flag.NewFlagSet("subcommand", flag.ContinueOnError)`, define flags with `BoolVar`, `StringVar`, etc., and `reg.Register("subcommand", fs, func() error { ... })`. The closure can read the flag variables and call into scene/engine. No config file: commands and flags live in code and are fully extensible.

**Built-in commands:**

| Command | Flags | Effect |
|---------|-------|--------|
| `grid` | `--show` | Show the 3D editor grid (XZ plane). |
| `grid` | `--hide` | Hide the 3D editor grid. |
| `fps` | `--show` | Show FPS counter (top-right, green). Part of debugging; off by default. |
| `fps` | `--hide` | Hide the FPS counter. |
| `memalloc` | `--show` | Show memory allocation (under FPS, green). Off by default. |
| `memalloc` | `--hide` | Hide the memory allocation display. |
| `window` | `--fullscreen` | Switch to fullscreen. |
| `window` | `--windowed` | Switch to windowed mode. |
| `spawn` | `<type> <x> <y> <z> [sx sy sz]` | Add a primitive (cube, sphere, cylinder, plane) at position; optional scale. |
| `save` | *(none)* | Write current scene (including runtime-spawned objects) to the scene YAML file. |
| `newscene` | *(none)* | Clear all primitives and save an empty scene. |

Example: `cmd grid --hide` to hide the grid; `cmd fps --show` to show the FPS counter; `cmd memalloc --show` to show memory usage; `cmd window --windowed` to switch to windowed mode; `cmd window --fullscreen` to switch back to fullscreen.

---

## Debug system

**`internal/debug/`** provides runtime debugging overlays. All overlays are **hidden by default** and are toggled via the in-game terminal (e.g. `cmd fps --show` / `cmd fps --hide`).

- **FPS** — Frames per second drawn at the **top-right** of the screen in **green** when enabled. Uses raylib’s `GetFPS()`.

- **Mem** — Heap allocation (Go runtime) drawn **under FPS** in **green** when enabled (`cmd memalloc --show`). Uses `runtime.ReadMemStats()`; displayed as MiB.

The debug system is drawn after the 3D scene and before the terminal in the main loop. New debug overlays can be added as fields and draw logic in `internal/debug/debug.go`, with corresponding commands registered in `main.go`. FPS and Mem text are only recomputed every 30 frames to limit allocations.

**Memory profiling:** Run with `DEBUG_PPROF=1` to expose pprof on `http://localhost:6060`. Then e.g. `go tool pprof -http=:8080 http://localhost:6060/debug/pprof/heap` to inspect heap usage and find remaining allocation hotspots.

---

## Primitive CSS UI system

**`internal/ui/`** provides a minimal, CSS-driven UI layer. It is **primitive**: no shadows, no rounded corners, no layout engine—just selectors, a small property set, and explicit position/size.

- **Draw order:** Scene → Debug → **UI** → Terminal. So scene UI sits above the 3D view and debug, and the terminal (chat/LLM) always renders on top when enabled.
- **Assets:** CSS and other UI data live under **`assets/ui/`** so they stay separate from skybox and other assets. Example: `assets/ui/default.css`.
- **Selectors:** Only `.class` and `#id`. No combinators or pseudo-classes.
- **Properties:** `background`, `color`, `border`, `width`, `height`, `left`, `top` (or `x`, `y`). Values: hex colors (`#RGB`, `#RRGGBB`), numbers with optional `px`.
- **Model:** Nodes are created in code (type, class, id, optional text). The engine loads a stylesheet (e.g. from `assets/ui/default.css`), matches rules to nodes by class/id, resolves props to a computed style, and draws with raylib (rectangles for background/border, text for labels).
- **Scene binding (future):** A data layer (manifest or per-scene file) will map scene id → CSS file(s) so each scene can have its own styles; on scene switch, the engine will load that scene’s CSS.

---

## Engine config persistence

**`internal/engineconfig/`** persists engine-only preferences across runs. This is **not** for in-game save data (that is a separate, future system).

- **File:** `config/engine.json` (relative to the process working directory; e.g. `cmd/game/config/` when run from repo root). The directory is created on first save.
- **Contents:** `show_fps`, `show_memalloc`, `grid_visible` (JSON booleans). Defaults when the file is missing: FPS and memalloc off, grid on.
- **Load:** At startup, `engineconfig.Load()` is called; the returned prefs are applied to the debug and scene (e.g. `dbg.SetShowFPS(prefs.ShowFPS)`). If the file is missing or invalid, defaults are used.
- **Save:** After every `grid`, `fps`, or `memalloc` command that changes state, the current debug and scene state is written to `config/engine.json`. Saving on each toggle keeps state in sync even if the game exits without a clean shutdown.

Adding a new engine preference: add a field to `EnginePrefs` in `internal/engineconfig/engineconfig.go`, apply it after `Load()` in `main.go`, and call `saveEnginePrefs()` from the command that changes it.

---

## Log files

Logs are written under **`logs/`** (relative to the process working directory; e.g. `cmd/game/logs/` when run from repo root). Both files persist after the game exits.

| File | Purpose |
|------|---------|
| **`terminal.txt`** | Terminal/chat input only. Each line the user submits (Enter) is appended with a timestamp. Not cleared on start. |
| **`engine_log.txt`** | Engine and raylib output only. All raylib trace messages (INFO, WARNING, ERROR, etc.—e.g. init, display, textures, shaders) are captured via `SetTraceLogCallback` and appended with timestamp and level. Engine errors logged with `log.Error(...)` also go here. Not cleared on start; use for debugging and post-run inspection. |

---

## Version control

**Commit** and **push** are done by the user. Do not have an agent perform git commit or push unless the user explicitly asks for it.
