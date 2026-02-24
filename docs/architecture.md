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
- **`internal/scene/`** — 3D scene: Camera3D and free-camera update; Draw uses BeginMode3D/DrawGrid/EndMode3D.
- **`internal/terminal/`** — Chat/terminal bar: input handling and drawing (uses logger and raylib).
- **`internal/logger/`** — Stores and persists lines (e.g. to `logs/terminal.txt`).
- **`docs/`** — Documentation (e.g. this file).

Graphics and UI (terminal) are separate: graphics owns the window and loop; scene owns 3D camera and world; terminal owns its state, input, and draw. Add more `internal/*` packages as needed (e.g. `internal/input`).
