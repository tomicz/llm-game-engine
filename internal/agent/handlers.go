package agent

import (
	"errors"
	"fmt"
	"math"
	"math/rand/v2"

	"game-engine/internal/primitives"
	"game-engine/internal/scene"
)

// maxBatch caps how many objects one add_objects action may create.
const maxBatch = 500

// RegisterSceneHandlers adds the add_object and add_objects actions, which spawn primitives in scn.
func RegisterSceneHandlers(a *Agent, scn *scene.Scene) {
	a.RegisterHandler("add_object", func(act Action) error {
		typ, _ := act["type"].(string)
		if !primitives.IsShape(typ) {
			return fmt.Errorf("unknown type %q", typ)
		}
		pos, err := float3(act["position"])
		if err != nil {
			return fmt.Errorf("position: %w", err)
		}
		scale, err := float3(act["scale"])
		if err != nil {
			scale = [3]float32{1, 1, 1}
		}
		o := scene.Object{Type: typ, Position: pos, Scale: scale}
		o.SetPhysics(boolOr(act["physics"], true))
		if c, err := float3(act["color"]); err == nil {
			o.Color = c
		}
		scn.Add(o)
		return nil
	})

	a.RegisterHandler("add_objects", func(act Action) error {
		typ, _ := act["type"].(string)
		randomType := typ == "random" || typ == "any"
		if !randomType && !primitives.IsShape(typ) {
			return fmt.Errorf("unknown type %q (use cube, sphere, cylinder, plane, or random)", typ)
		}
		count := 1
		if n, ok := act["count"].(float64); ok && n >= 1 {
			count = min(int(n), maxBatch)
		}
		spacing := float32(2)
		if s, err := float1(act["spacing"]); err == nil && s > 0 {
			spacing = s
		}
		origin, _ := float3(act["origin"])
		pattern, _ := act["pattern"].(string)
		scale := [3]float32{1, 1, 1}
		if s, err := float3(act["scale"]); err == nil {
			scale = s
		}
		scaleMin, errMin := float3(act["scale_min"])
		scaleMax, errMax := float3(act["scale_max"])
		scaleRange := errMin == nil && errMax == nil
		physics := boolOr(act["physics"], true)
		color, _ := float3(act["color"])
		colorRandom := boolOr(act["color_random"], false)

		objs := make([]scene.Object, count)
		for i := range objs {
			o := scene.Object{Type: typ, Position: layout(pattern, i, count, origin, spacing), Scale: scale, Color: color}
			if randomType {
				o.Type = primitives.Shapes[rand.IntN(len(primitives.Shapes))]
			}
			if scaleRange {
				for j := range 3 {
					lo, hi := min(scaleMin[j], scaleMax[j]), max(scaleMin[j], scaleMax[j])
					o.Scale[j] = max(lo+rand.Float32()*(hi-lo), 0.1)
				}
			}
			if colorRandom {
				// 0.35–1.0 per channel so colors stay visible.
				o.Color = [3]float32{0.35 + rand.Float32()*0.65, 0.35 + rand.Float32()*0.65, 0.35 + rand.Float32()*0.65}
			}
			o.SetPhysics(physics)
			objs[i] = o
		}
		scn.Add(objs...)
		return nil
	})
}

// layout returns the position of item i of count for a spawn pattern: "line" along +X, "random"
// (or "spread") scattered in a square around origin, otherwise a square grid in +X/+Z.
func layout(pattern string, i, count int, origin [3]float32, spacing float32) [3]float32 {
	switch pattern {
	case "line":
		return [3]float32{origin[0] + float32(i)*spacing, origin[1], origin[2]}
	case "random", "spread":
		half := max(spacing*float32(count)/4, 5)
		return [3]float32{
			origin[0] + (rand.Float32()*2-1)*half,
			origin[1],
			origin[2] + (rand.Float32()*2-1)*half,
		}
	default: // "grid"
		cols := int(math.Ceil(math.Sqrt(float64(count))))
		row, col := i/cols, i%cols
		return [3]float32{origin[0] + float32(col)*spacing, origin[1], origin[2] + float32(row)*spacing}
	}
}

// boolOr returns v if it is a JSON boolean, else def.
func boolOr(v any, def bool) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}

// float1 returns v as a number.
func float1(v any) (float32, error) {
	if n, ok := v.(float64); ok {
		return float32(n), nil
	}
	return 0, errors.New("expected number")
}

// float3 returns v as an [x,y,z] array of numbers (extra elements are ignored).
func float3(v any) ([3]float32, error) {
	var out [3]float32
	arr, ok := v.([]any)
	if !ok || len(arr) < 3 {
		return out, errors.New("expected [x,y,z]")
	}
	for i := range out {
		n, ok := arr[i].(float64)
		if !ok {
			return out, fmt.Errorf("element %d is not a number", i)
		}
		out[i] = float32(n)
	}
	return out, nil
}
