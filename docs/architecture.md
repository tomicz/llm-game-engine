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
- **`internal/scene/`** — 3D scene: Camera3D and free-camera update. Draw uses BeginMode3D and a custom **editor-style grid** on the XZ plane (minor/major lines every 1/10 units, extent ±50) plus X/Y/Z axis lines (red/green/blue) through the origin; see `drawEditorGrid()` in `scene.go`.
- **`internal/terminal/`** — Chat/terminal bar: input handling and drawing (uses logger and raylib).
- **`internal/logger/`** — Terminal lines (memory + file), engine/raylib log to file. See **Log files** below.
- **`docs/`** — Documentation (e.g. this file).

Graphics and UI (terminal) are separate: graphics owns the window and loop; scene owns 3D camera and world; terminal owns its state, input, and draw. Add more `internal/*` packages as needed (e.g. `internal/input`).

---

## 3D editor grid

The scene draws a Unity-style grid on the **XZ plane** (Y = 0, raylib Y-up):

- **Minor lines** every 1 unit, dim gray; **major lines** every 10 units, brighter gray; extent ±50 on X and Z.
- **Axis lines** through the origin: **X** red, **Y** green, **Z** blue.

Tunables live in `internal/scene/scene.go` as constants: `gridExtent`, `gridMinorStep`, `gridMajorStep`, and the alpha values for minor/major/axis lines.

---

## Log files

Logs are written under **`logs/`** (relative to the process working directory; e.g. `cmd/game/logs/` when run from repo root). Both files persist after the game exits.

| File | Purpose |
|------|---------|
| **`terminal.txt`** | Terminal/chat input only. Each line the user submits (Enter) is appended with a timestamp. Not cleared on start. |
| **`engine_log.txt`** | Engine and raylib output only. All raylib trace messages (INFO, WARNING, ERROR, etc.—e.g. init, display, textures, shaders) are captured via `SetTraceLogCallback` and appended with timestamp and level. Engine errors logged with `log.Error(...)` also go here. Not cleared on start; use for debugging and post-run inspection. |
