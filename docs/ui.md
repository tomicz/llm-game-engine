# UI system

The engine’s UI is a **primitive CSS-driven** layer: no shadows, no rounded corners, no layout engine. You write `.css` files and create nodes in code; the UI engine matches rules by class/id, resolves styles, and draws with raylib.

---

## Overview

| What | Where |
|------|--------|
| **Engine code** | `internal/ui/` (parser, style, nodes, draw) |
| **Assets** | `assets/ui/` (CSS and other UI data; separate from skybox etc.) |
| **Draw order** | Scene → Debug → **UI** → Terminal (terminal is always on top when enabled) |

---

## CSS

### File location

Put stylesheets under **`assets/ui/`**, e.g. `assets/ui/default.css`. Load at runtime with `uiEngine.LoadCSS(path)`. When running from repo root use `assets/ui/...`; from `cmd/game` you may need `../../assets/ui/...` (same pattern as other assets).

### Selectors

Only two kinds:

- **`.class`** — matches nodes whose `Class` equals the part after the dot.
- **`#id`** — matches nodes whose `ID` equals the part after the hash.

No combinators (`.a .b`), no pseudo-classes (`:hover`), no tag selectors. Later rules override earlier for the same selector.

### Properties

| Property | Meaning | Values |
|----------|---------|--------|
| `background` | Fill color of the node’s rectangle | `#RGB` or `#RRGGBB` |
| `color` | Text color | `#RGB` or `#RRGGBB` |
| `border` | 1px rectangle outline color (enables border) | `#RGB` or `#RRGGBB` |
| `width` | Width in pixels | Number, optional `px` (e.g. `200` or `200px`) |
| `height` | Height in pixels | Same |
| `left`, `x` | Horizontal position | Pixels or **`N%`** (0–100; relative to screen width, node centered when width known) |
| `top`, `y` | Vertical position | Pixels or **`N%`** (0–100; relative to screen height, node centered when height known) |

Anything else is ignored. No `padding`, `margin`, `box-shadow`, `border-radius`, or units other than `px`/`%` for left/top.

### Example

```css
.panel {
  background: #2a2a2a;
  width: 320px;
  height: 120px;
  left: 24px;
  top: 24px;
  border: #444;
}

.title {
  color: #eee;
  left: 32px;
  top: 32px;
}
```

---

## Nodes

Nodes are created in **code**, not in HTML or another markup format. Each node has:

- **Type** — e.g. `"panel"`, `"label"` (for your own use; the engine only uses it to know what to draw).
- **Class** — matched by `.class` in CSS.
- **ID** — matched by `#id` in CSS.
- **Bounds** — set from the resolved style (`left`/`top`/`width`/`height` or `left%`/`top%`).
- **Text** — optional; if set, drawn with `color` at a small offset from the node’s position.

Create with `ui.NewNode(typ, class, id, text)`, then `uiEngine.AddNode(n)` (or `SetNodes`). Draw order is the order of nodes in the list (first = back, last = front).

---

## Engine API (summary)

- **`ui.New()`** — New engine (no stylesheet, no nodes).
- **`LoadCSS(path string) error`** — Load and parse a `.css` file; replaces current stylesheet.
- **`SetStylesheet(sheet *Stylesheet)`** — Set stylesheet directly.
- **`AddNode(n *Node)`** / **`SetNodes(nodes []*Node)`** — Add one node or replace all.
- **`Draw()`** — Resolve styles for each node and draw (background rect, optional 1px border, optional text). Call once per frame after debug and before terminal.

---

## Percentage positioning

For **`left`** or **`top`** you can use **`N%`** (e.g. `50%`). The engine interprets this as a percentage of the current screen width or height and positions the node so that its left/top edge is at that percentage. With `50%` and a known width/height, the node is centered horizontally or vertically.

---

## Scene binding (future)

A data layer (manifest or per-scene file) will map **scene id → CSS file(s)**. On scene switch, the engine will load that scene’s stylesheet(s) so each scene can have its own UI look without mixing assets.
