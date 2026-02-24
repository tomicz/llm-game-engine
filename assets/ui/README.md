# UI assets

CSS and other UI data live here so they stay separate from other assets (skybox, etc.).

## CSS (`assets/ui/*.css`)

- **Primitive subset:** class (`.class`) and id (`#id`) selectors only.
- **Properties:** `background`, `color`, `border`, `width`, `height`, `left`, `top` (or `x`, `y`).
- **Values:** hex colors (`#RGB`, `#RRGGBB`), numbers with optional `px`.

The UI engine loads CSS per scene (see docs/ARCHITECTURE.md). `default.css` is used for the initial/test UI.
