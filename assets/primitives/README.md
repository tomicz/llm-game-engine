# Default primitives

YAML files here define default parameters for each primitive type (e.g. `cube.yaml`, `sphere.yaml`, `cylinder.yaml`: type, size/radius/height, color). Mesh generation is driven by the type name in `internal/primitives/`; these files provide defaults for authoring.

**Built-in types:** `cube` (1×1×1), `sphere` (diameter 1), `cylinder` (diameter 1, height 1). All use the same default extent (1 unit) and share a single lit shader. Scene `position` is the **center** of each primitive.

Add new YAML files here when new primitive types are added to the engine.
