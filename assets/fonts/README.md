# Engine fonts

Place TTF (or OTF) font files here. The engine uses **one font for all UI**: inspector, terminal/chat, and debug overlays (FPS, memory). **Any font** you add under `assets/fonts/` is supported; Roboto is only the default when none is configured.

- Add font files in any structure (e.g. `Roboto/static/Roboto-Regular.ttf`, `DejaVuSans.ttf`, `MyFont/MyFont.otf`).
- **Multiple fonts:** Keep as many as you like under `assets/fonts/`; only one is **active** at a time.
- **Set active font:**
  - **In-game:** `cmd font <name>` — font family name (e.g. `Inter`, `Open Sans`, `Roboto`). If the font is already in `assets/fonts/`, it is used; otherwise the engine **downloads it from Google Fonts** (no user URLs; safe). Saved under `assets/fonts/downloaded/<name>/` and set as active. Fonts persist after the game is closed. The `downloaded/` folder is gitignored.
  - **Config:** In `config/engine.json`, set `"font": "path/under/assets/fonts"` (e.g. `"Roboto/static/Roboto-Regular.ttf"`), then restart.
- **Default:** If `font` is missing or empty, the engine uses `Roboto/static/Roboto-Regular.ttf`.

If the configured path is not found, the engine falls back to raylib’s built-in pixel font.
