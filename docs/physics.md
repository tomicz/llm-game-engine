# 3D Physics

The engine includes a minimal **3D physics** layer: gravity, AABB (axis-aligned bounding box) collision, and per-object enable/disable. Physics runs only when the **terminal is closed** (game mode); when the terminal is open (editor mode), objects can be moved by hand and physics is not stepped.

---

## Overview

| Component | Location | Role |
|-----------|----------|------|
| **Physics world** | `internal/physics/` | Bodies, gravity, integration, AABB collision resolution |
| **Scene integration** | `internal/scene/scene.go` | 1:1 bodies with scene objects, sync, step only in game mode |
| **Per-object flag** | `ObjectInstance.Physics` | Enable or disable physics (falling/collision) per object |

- **Gravity** is applied along **-Y** by default (`[0, -9.8, 0]`). There is **no global floor**: dynamic objects can fall below Y=0 until they hit another body (e.g. a static plane).
- **Static** bodies (physics disabled) do not move and are not affected by gravity but **still collide**: they block falling objects.
- **Dynamic** bodies (physics enabled) get gravity, velocity integration, and collision response (push apart, velocity zeroed on collision axis).

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
- **Bodies** – slice of bodies in the same order as scene objects.

**Step(dt)**:

1. Applies gravity to non-static bodies.
2. Integrates velocity into position.
3. Resolves AABB vs AABB collisions: finds overlapping pairs, computes minimum penetration axis, pushes bodies apart (static bodies do not move), and zeroes velocity on that axis for both.

No ground plane or world bounds: bodies only stop when they hit another body.

---

## Scene integration

- The scene keeps a **physics World** and maintains **one body per scene object** (same order).
- **ensurePhysicsBodies()** – Ensures `len(Bodies) == len(Objects)`; adds bodies for new objects. Static/dynamic is set from each object’s **Physics** flag.
- **syncSceneToPhysics()** – Copies each object’s position, scale, and physics flag into the corresponding body (including `Static = !physicsEnabled(obj)`).
- **syncPhysicsToScene()** – Copies dynamic body positions back to scene objects (static bodies are not written back).

Each frame in **game mode** (terminal closed), `Update()` runs:

1. `ensurePhysicsBodies()`
2. `syncSceneToPhysics()`
3. `physicsWorld.Step(rl.GetFrameTime())`
4. `syncPhysicsToScene()`

When the **terminal is open**, physics is not stepped; the editor can move objects and the next time you close the terminal, the last positions are synced into the physics world and simulation continues from there.

---

## Per-object physics (enable / disable)

Every object can have **physics on** (falls, collides) or **off** (static: no movement, still blocks others).

### Data

- **ObjectInstance.Physics** – `*bool`, YAML: `physics: true` or `physics: false`, optional.
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

- **SetPhysicsForIndex(index int, enabled bool) error** – Set physics on/off for the object at `index`. Returns an error if index is out of range.
- **SetSelectedPhysics(enabled bool) error** – Set physics for the currently selected object. Returns an error if no object is selected.
- **PhysicsEnabledForObject(obj ObjectInstance) bool** – Returns whether the object has physics enabled (for display or logic).

Persist changes with **SaveScene()** (or the `cmd save` command).

---

## Summary

- **Physics** = pure Go, AABB-based, in `internal/physics`. No global floor; objects fall until they hit another body.
- **Per-object** = `Physics` on each object; default on, set to `false` for static (e.g. floor).
- **Control** = YAML `physics: true/false`, terminal `cmd physics on/off`, or inspector click on the Physics row.
- **When it runs** = Only when the terminal is closed (game mode); editor mode does not step physics.
