# Assets

Optional runtime assets. The game runs without these; they enhance the scene when present.

Assets are grouped by purpose. Skybox files live in **`assets/skybox/`** so they stay separate from other assets you may add later.

## Skybox (`assets/skybox/`)

- **Files:** `skybox.png` or `skybox.jpg` (place in `assets/skybox/`)
- **Formats supported:**
  - **Equirectangular panorama** (e.g. 2:1 wide sky image) — used automatically. No conversion needed.
  - **Cubemap** — if the image is in a cubemap layout (6×1 vertical, 1×6 horizontal, or 3×4 / 4×3 cross), it is loaded as a cubemap.

If a valid skybox image is present in `assets/skybox/`, it is drawn as the 3D sky.

### Recommended source: Poly Haven (CC0)

**License:** [CC0 (Public Domain)](https://creativecommons.org/publicdomain/zero/1.0/).  
**Source:** [Poly Haven](https://polyhaven.com/) — https://polyhaven.com/license

- You can use Poly Haven HDRIs for any purpose, including commercial; no attribution required (attribution appreciated).
- They provide **equirectangular** panoramas. Download a sky HDRI (JPG or HDR), save as `skybox.png` or `skybox.jpg` in `assets/skybox/`. The engine supports equirectangular skybox images; no conversion to cubemap needed.

**Suggested credit (optional but appreciated):**  
*Sky from Poly Haven (polyhaven.com) — CC0*
