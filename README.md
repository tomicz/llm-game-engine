# Game Engine

A 3D game engine that **builds itself through an LLM**: you describe what you want in natural language, and the engine uses an AI model to turn that into game actions (spawn objects, run commands, change the scene). No scripting required—the LLM drives the engine from inside the running game.

You can also extend and control everything via an in-game terminal with explicit commands (`cmd ...`). The same internal APIs power both: natural language goes to the LLM and comes back as structured actions; commands call the same handlers directly.

Built with [Go](https://go.dev/) and [raylib-go](https://github.com/gen2brain/raylib-go).

**License:** Apache 2.0 — see [LICENSE](LICENSE).

---

## Run

From the repo root:

```bash
go run ./cmd/game
```

Or from `cmd/game`:

```bash
cd cmd/game && go run .
```

Assets (e.g. skybox, UI CSS) are loaded from `assets/`; see [assets/README.md](assets/README.md). Logs are written under `cmd/game/logs/` when run from `cmd/game`.

---

## Features

### 3D scene and primitives

- **Primitives:** `cube`, `sphere`, `cylinder`, `plane`. All use a common scale (e.g. 1×1×1 default); position is the **center** of each object.
- **Scene file:** YAML (e.g. `assets/scenes/default.yaml`) defines the list of objects (type, position, scale). The scene loads at startup and can be saved at runtime; runtime-spawned objects are included.
- **Physics:** Each object can have physics on (gravity, collision) or off (static). Set per object or globally via gravity command.

### Scene editor (terminal open)

When the terminal is open (ESC), the scene is in **editor mode**:

- **Select:** Click an object. Selected object shows a **yellow bounding box** and **red (X), green (Y), blue (Z) direction arrows**.
- **Move:** Drag by face. **Top/bottom face** → move on the XZ plane (horizontal). **Side face** → move vertically (Y). The point you click stays under the cursor.
- **Skybox and grid** are not selectable.

### Camera

- **Free camera:** Move and look around the 3D world (WASD / mouse or equivalent).
- **Focus:** Command to point the camera at the selected object (`cmd focus`; select an object first).

### Grid and debug

- **Editor grid:** XZ plane with minor lines every 1 unit, major every 10, axis lines (X red, Y green, Z blue). Toggle with `cmd grid --show` / `cmd grid --hide`.
- **FPS counter:** `cmd fps --show` / `cmd fps --hide` (top-right, green).
- **Memory usage:** `cmd memalloc --show` / `cmd memalloc --hide` (under FPS).  
  Debug overlays are off by default; state is persisted in `config/engine.json`.

### Window and display

- **Fullscreen / windowed:** `cmd window --fullscreen` / `cmd window --windowed`.
- **Screenshot:** `cmd screenshot` writes `screenshot.png` in the working directory.

### Objects: spawn, delete, duplicate, undo

- **Spawn one:** `cmd spawn <type> <x> <y> <z> [sx sy sz]` (e.g. `cmd spawn cube 0 0 0` or `cmd spawn sphere 1 0 1 2 2 2`).
- **Delete:** `cmd delete selected` | `cmd delete look` | `cmd delete random` | `cmd delete name <name>`.
- **Duplicate:** `cmd duplicate [N]` clones the selected object N times (default 1). Select first.
- **Undo:** `cmd undo` reverts the last add or delete (one level).

### Object properties (select first)

- **Color:** `cmd color <r> <g> <b>` (0–1, e.g. `cmd color 1 0 0` for red).
- **Name:** `cmd name <name>` (for reference and `delete name <name>`).
- **Motion:** `cmd motion bob` (gentle Y oscillation) or `cmd motion off`.
- **Physics:** `cmd physics on` / `cmd physics off` (gravity/collision on selected object).

### Lighting and skybox

- **Lighting:** `cmd lighting noon` | `cmd lighting sunset` | `cmd lighting night` (directional light profile).
- **Skybox (file):** Put `skybox.png` or `skybox.jpg` in `assets/skybox/` (equirectangular 2:1 or cubemap). Loaded at startup.
- **Skybox (URL):** `cmd skybox <url>` downloads an image in the background and sets it as the skybox (panorama or cubemap).

### Textures (select object first)

- **From URL:** `cmd download image <url>` downloads an image and applies it as texture to the selected object.
- **From file:** `cmd texture <path>` (e.g. `assets/textures/downloaded/foo.png`) applies an image file as texture.

### Physics

- **Gravity:** `cmd gravity <y>` (e.g. `cmd gravity -9.8` or `cmd gravity 0` for zero-g). Affects all dynamic objects.

### Presets (templates)

- **Tree:** `cmd template tree [x y z]` spawns a cylinder (trunk) and sphere (foliage) at the given position (or 0,0,0). Optional for quick placeholders; the LLM can instead compose trees from primitives.

### Natural language (LLM agent)

When you type a line **without** `cmd `, it is sent to an LLM (if an API key is configured). The model returns **structured actions**; the engine applies them. No code generation—the running process uses the LLM to decide what to do, then uses existing handlers.

**Agent actions:**

- **add_object** — One primitive: type (cube/sphere/cylinder/plane), position, scale, optional color, physics on/off.
- **add_objects** — Many primitives: type, count, pattern (grid/line/random), spacing, origin, optional scale_min/scale_max, color, color_random, physics. Use for “spawn 50 cubes”, “city with random heights”, “colorful buildings”, etc.
- **run_cmd** — Run any in-game command by args (e.g. `["grid","--hide"]`, `["lighting","sunset"]`, `["screenshot"]`).

**Examples the LLM can handle:**

- “Spawn 10 cubes”, “add 50 spheres in a grid”, “30 random objects spread around”.
- “Create a city”, “city with skyscrapers”, “buildings with random heights” → add_objects with cubes, scale_min/scale_max for height, physics false.
- “Colorful city”, “buildings in random colors” → same + color_random.
- “Forest”, “spawn trees” → LLM composes trees from cylinders (trunk) + spheres (foliage), physics false, multiple add_object actions.
- “Save the scene”, “hide grid”, “sunset lighting”, “zero gravity”, “take a screenshot”, “delete selected”, “undo”, “focus on selected”, “set model to gpt-4o-mini”, etc. → run_cmd with the right args.

**Available shapes** for the LLM are only **cube, sphere, cylinder, plane**. The LLM composes them to represent other things (e.g. tree = cylinder + sphere). Model choice is set with `cmd model <name>` and persisted.

### UI (CSS overlay)

- **Primitive CSS UI:** A minimal CSS-driven layer (see [docs/UI.md](docs/UI.md)). Styles live in `assets/ui/` (e.g. `default.css`). Selectors: `.class`, `#id`. Properties: background, color, border, width, height, left/top (pixels or %). Nodes are created in code; draw order is Scene → Debug → UI → Terminal (terminal on top when enabled).
- **Inspector:** Scene UI can show an inspector for the selected object; layout and content are driven by the same UI system.

### Config and logs

- **Engine config:** `config/engine.json` (relative to working directory) stores grid visibility, FPS/memalloc toggles, and AI model name. Loaded at startup; saved when you change those options.
- **Logs:** `cmd/game/logs/terminal.txt` (terminal input lines); `cmd/game/logs/engine_log.txt` (engine/raylib output and errors). Not cleared on start.

---

## LLM setup (how the engine “builds itself”)

The engine turns **natural-language** input into game actions by calling an LLM (Groq, OpenAI, Cursor, or Ollama). To enable this:

1. Copy `.env.example` to `.env`: `cp .env.example .env`
2. Add your API key(s) to `.env`, e.g. `GROQ_API_KEY=...` or `OPENAI_API_KEY=...`
3. **Do not commit `.env`** — it’s in `.gitignore`. Never put API keys in the repo.

**Provider priority:** Groq (free tier) → Cursor → OpenAI. Set the model in-game with `cmd model <name>` (e.g. `cmd model gpt-4o-mini` or `cmd model llama-3.3-70b-versatile`). See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) (Natural language and AI agent).

---

## Project layout

- **`cmd/game/`** — Entry point; wires logger, terminal, scene, graphics, agent, and commands.
- **`internal/`** — Engine packages: `graphics`, `scene`, `primitives`, `terminal`, `commands`, `agent`, `llm`, `debug`, `engineconfig`, `logger`, `ui`, `env`.
- **`internal/agent/`** — Natural language → LLM → structured actions (`add_object`, `add_objects`, `run_cmd`); dispatches to the same handlers used by `cmd` commands.
- **`internal/llm/`** — LLM client (Groq, OpenAI, Cursor, Ollama).
- **`assets/`** — Optional runtime assets: skybox under `assets/skybox/`, UI under `assets/ui/`, primitives/scenes under `assets/primitives/`, `assets/scenes/`.
- **`docs/`** — [ARCHITECTURE.md](docs/ARCHITECTURE.md), [UI.md](docs/UI.md), and other docs.

Details: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

---

## Assets

Optional assets live under **`assets/`**, grouped by purpose.

- **Skybox:** Put `skybox.png` or `skybox.jpg` in `assets/skybox/`. Equirectangular (2:1) or cubemap layouts supported. Or set at runtime with `cmd skybox <url>`.
- **UI:** CSS and related assets in `assets/ui/` (e.g. `default.css`). See [docs/UI.md](docs/UI.md).
- **Scenes:** YAML in `assets/scenes/` (e.g. `default.yaml`). Primitives’ default definitions in `assets/primitives/`.

Full list and sources (e.g. Poly Haven, CC0): [assets/README.md](assets/README.md).
