# 3D Physics

The engine includes a minimal **3D physics** layer: gravity, AABB (axis-aligned bounding box) collision, and per-object enable/disable. Physics runs only when the **terminal is closed** (game mode); when the terminal is open (editor mode), objects can be moved by hand and physics is not stepped.

---

## Overview

| Component | Location | Role |
|-----------|----------|------|
| **Physics world** | `internal/physics/` | Bodies, gravity, integration, AABB collision resolution |
| **Scene integration** | `internal/scene/scene.go` (`stepPhysics`) | Each object owns its body; sync, step only in game mode |
| **Per-object flag** | `scene.Object.Physics` | Enable or disable physics (falling/collision) per object |

- **Gravity** is applied along **-Y** by default (`[0, -9.8, 0]`). There is **no global floor**: dynamic objects can fall below Y=0 until they hit another body (e.g. a static plane).
- **Static** bodies (physics disabled) do not move and are not affected by gravity but **still collide**: they block falling objects.
- **Dynamic** bodies (physics enabled) get gravity, velocity integration, and collision response (pushed away from the other body, velocity zeroed on the collision axis).

---

## Package `internal/physics`

### Body

A **Body** has:

- **Position**, **Velocity**, **Scale** (used to build an AABB)
- **Mass** (used for collision response; default 1)
- **Static**: if true, the body does not move and ignores gravity; it still participates in collision so other bodies are pushed away.

Bodies are created by the scene; you do not create them directly unless extending the system.

### World

- **Gravity** – vector, default `[0, -9.8, 0]`. Change with `SetGravity([3]float32)`.
- **Bodies** – the bodies to simulate; the scene sets this list every step.

**Advance(dt)** is what the scene calls once per frame. It caps the frame time at 0.25 s (so a stall such as dragging the window doesn't launch everything) and splits it into `Step`s of at most 1/60 s, so fast bodies don't tunnel through thin colliders after a slow frame. At 60 FPS that is one step per frame.

**Step(dt)**:

1. Applies gravity to non-static bodies.
2. Integrates velocity into position.
3. Resolves AABB vs AABB collisions: finds overlapping pairs, computes the minimum penetration axis, and pushes each body **away from the other's center** along that axis (static bodies do not move; two dynamic bodies split the push by mass). Velocity on that axis is zeroed for dynamic bodies. The result does not depend on the order of bodies in the list.

No ground plane or world bounds: bodies only stop when they hit another body.

---

## Scene integration

- The scene keeps a **physics World**. Each scene object **owns its body**, created on the first physics step after the object is added. Deleting an object drops its body, so no invisible colliders are left behind; an object restored by undo starts at rest.
- Objects are the source of truth. Each frame in **game mode** (terminal closed), `Scene.Update()` calls `stepPhysics(rl.GetFrameTime())`, which:
  1. copies each object's position, collider size, and physics flag into its body (`Static = !obj.PhysicsEnabled()`),
  2. calls `World.Advance(dt)`,
  3. copies dynamic body positions back to their objects (static bodies are not written back).
- **Collider size** is the object's scale (zero components = 1). Planes always use a thickness of 0.1. The terrain's collider is its full box (width × max height × depth).

When the **terminal is open**, physics is not stepped; the editor can move objects and the next time you close the terminal, simulation continues from their new positions.

---

## Per-object physics (enable / disable)

Every object can have **physics on** (falls, collides) or **off** (static: no movement, still blocks others).

### Data

- **Object.Physics** – `*bool`, YAML: `physics: true` or `physics: false`, optional.
- **Default**: if `Physics` is **nil** (omitted in YAML), it is treated as **on** (dynamic). So existing scenes without the key behave as before.

### YAML

In scene files (e.g. `assets/scenes/default.yaml`):

```yaml
objects:
  - type: plane
    position: [0, 0, 0]
    scale: [10, 1, 10]
    physics: false
  - type: cube
    position: [0, 0.5, 0]
    scale: [1, 1, 1]
    # physics omitted = on (falls)
```

- **physics: false** – static (e.g. floor plane).
- **physics: true** or omit – dynamic (falls and collides).

### Terminal command

- **`cmd physics on`** – Enable physics for the **selected** object.
- **`cmd physics off`** – Disable physics for the **selected** object.

Requires an object to be selected (click it with the terminal open). Use **`cmd save`** to persist the scene after toggling.

### Inspector toggle

With the terminal open and an object selected, the inspector shows **Physics: On** or **Physics: Off**. **Left-click that row** to toggle physics for the selected object. Same effect as `cmd physics on/off`.

---

## Scene API (for LLM or scripts)

- **obj.SetPhysics(enabled bool)** – Turn physics on or off for an object (e.g. one from `Scene.RequireSelected()`, `Scene.Object(id)`, or a query).
- **obj.PhysicsEnabled() bool** – Whether the object has physics enabled (nil counts as on).
- **Scene.SetGravity([3]float32)** – Set the world gravity (`cmd gravity <y>`).

Persist changes with **Scene.Save()** (or the `cmd save` command).

---

## Summary

- **Physics** = pure Go, AABB-based, in `internal/physics`. No global floor; objects fall until they hit another body.
- **Per-object** = `Physics` on each object; default on, set to `false` for static (e.g. floor).
- **Control** = YAML `physics: true/false`, terminal `cmd physics on/off`, or inspector click on the Physics row.
- **When it runs** = Only when the terminal is closed (game mode); editor mode does not step physics.
