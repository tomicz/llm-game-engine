# Engine fonts

Place TTF (or OTF) font files here. The engine uses **one font for all UI**: inspector, terminal/chat, and debug overlays (FPS, memory). **Any font** you add under `assets/fonts/` is supported; Roboto is only the default when none is configured.

- Add font files in any structure (e.g. `Roboto/static/Roboto-Regular.ttf`, `DejaVuSans.ttf`, `MyFont/MyFont.otf`).
- **Multiple fonts:** Keep as many as you like under `assets/fonts/`; only one is **active** at a time.
- **Set active font:**
  - **In-game:** `cmd font <path or name>` — exact path relative to `assets/fonts/` (e.g. `Roboto/static/Roboto-Regular.ttf`) or a **search name** (e.g. `Inter`, `Google Sans`). The engine scans `assets/fonts/` for `.ttf`/`.otf` files and picks the first match; spaces and dashes are ignored for matching. Persisted to config.
  - **Config:** In `config/engine.json`, set `"font": "path/under/assets/fonts"` (e.g. `"Roboto/static/Roboto-Regular.ttf"`), then restart.
- **Default:** If `font` is missing or empty, the engine uses `Roboto/static/Roboto-Regular.ttf`.

If the configured path is not found, the engine falls back to raylib’s built-in pixel font.
