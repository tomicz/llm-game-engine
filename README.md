# Game Engine

A 3D game engine that **builds itself through an LLM**: you describe what you want in natural language, and the engine uses an AI model to turn that into game actions (spawn objects, run commands, change the scene). No scripting required—the LLM drives the engine from inside the running game.

You can also extend and control everything via an in-game terminal with explicit commands (`cmd ...`). The same internal APIs power both: natural language goes to the LLM and comes back as structured actions; commands call the same handlers directly.

Built with [Go](https://go.dev/) and [raylib-go](https://github.com/gen2brain/raylib-go). 3D scene with free camera, optional skybox, editor-style grid, scene editing (select/move primitives), and physics.

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

Assets (e.g. skybox) are loaded from `assets/`; see [assets/README.md](assets/README.md). Logs are written under `cmd/game/logs/` when run from `cmd/game`.

---

## LLM setup (how the engine “builds itself”)

The engine turns **natural-language** input into game actions by calling an LLM (Groq, OpenAI, Cursor, or Ollama). For example: “spawn 10 cubes”, “make a city with random heights”, “save the scene”. The LLM returns structured actions; the engine applies them. No code generation—the running process uses the LLM to decide *what* to do, then uses its existing handlers to do it.

To enable this:

1. Copy `.env.example` to `.env`: `cp .env.example .env`
2. Add your API key(s) to `.env`, e.g. `GROQ_API_KEY=...` or `OPENAI_API_KEY=...`
3. **Do not commit `.env`** — it’s in `.gitignore`. Never put API keys in the repo.

Provider priority: Groq (free tier) → Cursor → OpenAI. Set the model in-game with `cmd model <name>` (e.g. `cmd model llama-3.3-70b-versatile`). See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) (Natural language and AI agent).

---

## Project layout

- **`cmd/game/`** — Entry point; wires logger, terminal, scene, graphics.
- **`internal/`** — Engine packages: `graphics`, `scene`, `terminal`, `commands`, `agent`, `llm`, `debug`, `logger`.
- **`internal/agent/`** — Natural language → LLM → structured actions (e.g. `add_object`, `run_cmd`); dispatches to the same handlers used by `cmd` commands.
- **`internal/llm/`** — LLM client (Groq, OpenAI, Cursor, Ollama).
- **`assets/`** — Optional runtime assets (skybox under `assets/skybox/`, etc.).
- **`docs/`** — [ARCHITECTURE.md](docs/ARCHITECTURE.md) and other docs.

Details: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

---

## Assets

Optional assets live under **`assets/`**, grouped by purpose (e.g. **`assets/skybox/`** for skybox images). The game runs without them.

- **Skybox:** put `skybox.png` or `skybox.jpg` in `assets/skybox/`. Equirectangular (2:1) or cubemap layouts supported.
- Full list and sources (e.g. Poly Haven, CC0): [assets/README.md](assets/README.md).
