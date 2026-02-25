# Game Engine

A game engine that can **program itself from within the game**: you can extend and change behavior from inside the running game (in-game terminal, commands, and future tooling).

Built with [Go](https://go.dev/) and [raylib-go](https://github.com/gen2brain/raylib-go). 3D scene with free camera, optional skybox, editor-style grid, and in-game command system.

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

## API keys (natural-language / LLM)

For **natural-language** commands (e.g. “spawn 10 cubes”), the game can call an LLM (Groq, OpenAI, or Cursor). Setup:

1. Copy `.env.example` to `.env`: `cp .env.example .env`
2. Edit `.env` and add your API key(s), e.g. `GROQ_API_KEY=...` or `OPENAI_API_KEY=...`
3. **Do not commit or push `.env`** — it is listed in `.gitignore` so your keys stay local. Never add API keys to the repo or to code.

Priority: Groq (free) > Cursor > OpenAI. Model: `cmd model <name>` (e.g. `cmd model llama-3.3-70b-versatile`). See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) (Natural language and AI agent).

---

## Project layout

- **`cmd/game/`** — Entry point; wires logger, terminal, scene, graphics.
- **`internal/`** — Engine packages: `graphics`, `scene`, `terminal`, `commands`, `debug`, `logger`.
- **`assets/`** — Optional runtime assets (skybox under `assets/skybox/`, etc.).
- **`docs/`** — [ARCHITECTURE.md](docs/ARCHITECTURE.md) and other docs.

Details: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

---

## Assets

Optional assets live under **`assets/`**, grouped by purpose (e.g. **`assets/skybox/`** for skybox images). The game runs without them.

- **Skybox:** put `skybox.png` or `skybox.jpg` in `assets/skybox/`. Equirectangular (2:1) or cubemap layouts supported.
- Full list and sources (e.g. Poly Haven, CC0): [assets/README.md](assets/README.md).
