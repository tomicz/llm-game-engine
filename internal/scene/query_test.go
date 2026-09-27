package scene

import (
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestParseQuery(t *testing.T) {
	red := ColorNames["red"]
	tests := []struct {
		args []string
		want Query
		ok   bool
	}{
		{[]string{"left"}, Query{Position: "left"}, true},
		{[]string{"Cube"}, Query{Type: "cube"}, true},
		{[]string{"terrain"}, Query{Type: "terrain"}, true},
		{[]string{"building"}, Query{Name: "building"}, true},
		{[]string{"cube", "right"}, Query{Type: "cube", Position: "right"}, true},
		{[]string{"red", "cube"}, Query{Type: "cube", Color: &red}, true},
		{[]string{"building", "farthest"}, Query{Name: "building", Position: "farthest"}, true},
		{[]string{"red", "cube", "top"}, Query{Type: "cube", Color: &red, Position: "top"}, true},
		{nil, Query{}, false},
		{[]string{"foo", "bar"}, Query{}, false},
		{[]string{"red", "cube", "nowhere"}, Query{}, false},
		{[]string{"a", "b", "c", "d"}, Query{}, false},
	}
	for _, tt := range tests {
		got, ok := ParseQuery(tt.args)
		if ok != tt.ok || got.Type != tt.want.Type || got.Name != tt.want.Name || got.Position != tt.want.Position ||
			(got.Color == nil) != (tt.want.Color == nil) || (got.Color != nil && *got.Color != *tt.want.Color) {
			t.Errorf("ParseQuery(%q) = %+v, %v; want %+v, %v", tt.args, got, ok, tt.want, tt.ok)
		}
	}
}

func vis(name, typ string, color [3]float32, x, y, dist float32) VisibleObject {
	return VisibleObject{
		Object:    &Object{Name: name, Type: typ, Color: color},
		ScreenPos: rl.Vector2{X: x, Y: y},
		Distance:  dist,
	}
}

func TestQueryFilterAndPick(t *testing.T) {
	red, blue := [3]float32{0.9, 0.1, 0}, [3]float32{0, 0, 1}
	visible := []VisibleObject{ // sorted closest first, as ObjectsInView returns them
		vis("Building 1", "cube", red, 500, 300, 2),
		vis("tree", "sphere", [3]float32{}, 100, 100, 4),
		vis("Building 2", "cube", blue, 900, 500, 6),
		vis("", "cube", [3]float32{}, 300, 700, 8),
	}
	redC, blueC := ColorNames["red"], ColorNames["blue"]
	tests := []struct {
		name string
		q    Query
		want string // picked object name, or "-" for none
	}{
		{"default is closest", Query{}, "Building 1"},
		{"left", Query{Position: "left"}, "tree"},
		{"right", Query{Position: "right"}, "Building 2"},
		{"top", Query{Position: "top"}, "tree"},
		{"bottom", Query{Position: "bottom"}, ""},
		{"farthest", Query{Position: "farthest"}, ""},
		{"type", Query{Type: "sphere"}, "tree"},
		{"type + position", Query{Type: "cube", Position: "right"}, "Building 2"},
		{"color approx", Query{Type: "cube", Color: &redC}, "Building 1"},
		{"color exact", Query{Type: "cube", Color: &blueC}, "Building 2"},
		{"uncolored never matches color", Query{Type: "sphere", Color: &redC}, "-"},
		{"name substring case-insensitive", Query{Name: "BUILDING", Position: "farthest"}, "Building 2"},
		{"unknown type", Query{Type: "cone"}, "-"},
		{"unknown position", Query{Position: "behind"}, "-"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, ok := tt.q.pick(tt.q.filter(visible))
			got := "-"
			if ok {
				got = v.Object.Name
			}
			if got != tt.want {
				t.Errorf("picked %q, want %q", got, tt.want)
			}
		})
	}
}

func TestQueryNotFound(t *testing.T) {
	c := ColorNames["red"]
	tests := []struct {
		q    Query
		want string
	}{
		{Query{}, "no objects in view"},
		{Query{Type: "cube"}, "no cube in view (look at the object and try again)"},
		{Query{Type: "cube", Color: &c}, "no cube with that color in view (look at the object and try again)"},
		{Query{Name: "tower"}, `no objects matching "tower" in view`},
		{Query{Type: "cube", Name: "tower"}, `no cube matching "tower" in view`},
	}
	for _, tt := range tests {
		if got := tt.q.notFound().Error(); got != tt.want {
			t.Errorf("notFound(%+v) = %q, want %q", tt.q, got, tt.want)
		}
	}
}
