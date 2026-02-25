package agent

import (
	"fmt"
	"math"
	"math/rand"

	"game-engine/internal/commands"
	"game-engine/internal/scene"
)

var primitiveTypes = []string{"cube", "sphere", "cylinder", "plane"}

// RegisterSceneHandlers registers add_object, add_objects, and run_cmd handlers that use the given scene and command registry.
func RegisterSceneHandlers(a *Agent, scn *scene.Scene, reg *commands.Registry) {
	a.RegisterHandler("add_object", func(payload map[string]interface{}) error {
		typ, _ := payload["type"].(string)
		if typ == "" {
			return fmt.Errorf("missing type")
		}
		switch typ {
		case "cube", "sphere", "cylinder", "plane":
		default:
			return fmt.Errorf("unknown type %q", typ)
		}
		pos, err := parseFloat3(payload["position"])
		if err != nil {
			return fmt.Errorf("position: %w", err)
		}
		scale, err := parseFloat3(payload["scale"])
		if err != nil {
			scale = [3]float32{1, 1, 1}
		}
		physics := parseBoolOpt(payload["physics"], true)
		scn.AddPrimitiveWithPhysics(typ, pos, scale, physics)
		return nil
	})
	a.RegisterHandler("add_objects", func(payload map[string]interface{}) error {
		typ, _ := payload["type"].(string)
		if typ == "" {
			return fmt.Errorf("missing type")
		}
		randomType := typ == "random" || typ == "any"
		if !randomType {
			switch typ {
			case "cube", "sphere", "cylinder", "plane":
			default:
				return fmt.Errorf("unknown type %q (use cube, sphere, cylinder, plane, or random)", typ)
			}
		}
		count := 1
		if n, ok := payload["count"].(float64); ok && n >= 1 {
			count = int(n)
		}
		if count > 500 {
			count = 500
		}
		spacing := float32(2)
		if s, err := parseFloat1(payload["spacing"]); err == nil && s > 0 {
			spacing = s
		}
		origin, _ := parseFloat3(payload["origin"])
		pattern, _ := payload["pattern"].(string)
		if pattern == "" {
			pattern = "grid"
		}
		scale := [3]float32{1, 1, 1}
		if s, err := parseFloat3(payload["scale"]); err == nil {
			scale = s
		}
		physics := parseBoolOpt(payload["physics"], true)
		for i := 0; i < count; i++ {
			var pos [3]float32
			switch pattern {
			case "line":
				pos = [3]float32{origin[0] + float32(i)*spacing, origin[1], origin[2]}
			case "random", "spread":
				half := spacing * float32(count) / 4
				if half < 5 {
					half = 5
				}
				pos = [3]float32{
					origin[0] + (rand.Float32()*2-1)*half,
					origin[1],
					origin[2] + (rand.Float32()*2-1)*half,
				}
			case "grid":
				cols := int(math.Ceil(math.Sqrt(float64(count))))
				row, col := i/cols, i%cols
				pos = [3]float32{origin[0] + float32(col)*spacing, origin[1], origin[2] + float32(row)*spacing}
			default:
				cols := int(math.Ceil(math.Sqrt(float64(count))))
				row, col := i/cols, i%cols
				pos = [3]float32{origin[0] + float32(col)*spacing, origin[1], origin[2] + float32(row)*spacing}
			}
			spawnTyp := typ
			if randomType {
				spawnTyp = primitiveTypes[rand.Intn(len(primitiveTypes))]
			}
			scn.AddPrimitiveWithPhysics(spawnTyp, pos, scale, physics)
		}
		return nil
	})
	a.RegisterHandler("run_cmd", func(payload map[string]interface{}) error {
		args, ok := payload["args"].([]interface{})
		if !ok || len(args) == 0 {
			return fmt.Errorf("missing or empty args")
		}
		strs := make([]string, 0, len(args))
		for _, v := range args {
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("args must be strings")
			}
			strs = append(strs, s)
		}
		return reg.Execute(strs)
	})
}

func parseBoolOpt(v interface{}, defaultVal bool) bool {
	if v == nil {
		return defaultVal
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return defaultVal
}

func parseFloat1(v interface{}) (float32, error) {
	if v == nil {
		return 0, fmt.Errorf("expected number")
	}
	switch n := v.(type) {
	case float64:
		return float32(n), nil
	case float32:
		return n, nil
	default:
		return 0, fmt.Errorf("expected number")
	}
}

func parseFloat3(v interface{}) ([3]float32, error) {
	var out [3]float32
	arr, ok := v.([]interface{})
	if !ok || len(arr) < 3 {
		return out, fmt.Errorf("expected [x,y,z]")
	}
	for i := 0; i < 3; i++ {
		switch n := arr[i].(type) {
		case float64:
			out[i] = float32(n)
		case float32:
			out[i] = n
		default:
			return out, fmt.Errorf("position[%d] not a number", i)
		}
	}
	return out, nil
}
