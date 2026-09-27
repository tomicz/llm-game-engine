package scene

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// newTestScene returns an empty scene rooted in a temp dir (so no scene file is loaded or written
// outside the test).
func newTestScene(t *testing.T) *Scene {
	t.Helper()
	t.Chdir(t.TempDir())
	return New()
}

func types(s *Scene) []string {
	var out []string
	for _, o := range s.objects {
		out = append(out, o.Type)
	}
	return out
}

func TestAddAssignsUniqueIDsAndPlaneThickness(t *testing.T) {
	s := newTestScene(t)
	ids := s.Add(Object{Type: "cube"}, Object{Type: "plane", Scale: [3]float32{10, 1, 10}}, Object{Type: "plane", Scale: [3]float32{2, 3, 2}})
	if len(ids) != 3 || ids[0] == ids[1] || ids[1] == ids[2] || ids[0] == 0 {
		t.Fatalf("ids = %v, want 3 distinct non-zero", ids)
	}
	p, _ := s.Object(ids[1])
	if p.Scale[1] != planeDefaultScaleY {
		t.Errorf("plane with Y scale 1 got Y %v, want %v", p.Scale[1], planeDefaultScaleY)
	}
	p2, _ := s.Object(ids[2])
	if p2.Scale[1] != 3 {
		t.Errorf("plane with explicit Y scale changed to %v", p2.Scale[1])
	}
}

func TestDeleteClearsSelectionAndKeepsOthersAddressable(t *testing.T) {
	s := newTestScene(t)
	ids := s.Add(Object{Type: "cube"}, Object{Type: "sphere"}, Object{Type: "cylinder"})
	s.Select(ids[2])
	if n := s.Delete(ids[0]); n != 1 {
		t.Fatalf("Delete = %d", n)
	}
	// IDs stay valid after earlier objects are removed (indices would have shifted).
	if o, ok := s.Selected(); !ok || o.Type != "cylinder" {
		t.Errorf("selection after unrelated delete = %v, %v; want cylinder", o, ok)
	}
	s.Delete(ids[2])
	if _, ok := s.Selected(); ok {
		t.Error("selection not cleared after deleting the selected object")
	}
	if _, err := s.RequireSelected(); err != ErrNoSelection {
		t.Errorf("RequireSelected err = %v, want ErrNoSelection", err)
	}
}

func TestUndoAdd(t *testing.T) {
	s := newTestScene(t)
	s.Add(Object{Type: "cube"})
	s.Add(Object{Type: "sphere"}, Object{Type: "cylinder"})
	if err := s.Undo(); err != nil {
		t.Fatal(err)
	}
	if got := types(s); !slices.Equal(got, []string{"cube"}) {
		t.Errorf("after undo = %v, want [cube]", got)
	}
	if err := s.Undo(); err == nil {
		t.Error("second undo succeeded; only one level is kept")
	}
}

func TestUndoDeleteAllRestoresEveryObjectInOrder(t *testing.T) {
	s := newTestScene(t)
	ids := s.Add(Object{Type: "cube", Name: "a"}, Object{Type: "sphere", Name: "b"}, Object{Type: "cylinder", Name: "c"}, Object{Type: "plane", Name: "d"})
	s.Delete(ids[1], ids[3], ids[0])
	if got := types(s); !slices.Equal(got, []string{"cylinder"}) {
		t.Fatalf("after delete = %v", got)
	}
	if err := s.Undo(); err != nil {
		t.Fatal(err)
	}
	if got := types(s); !slices.Equal(got, []string{"cube", "sphere", "cylinder", "plane"}) {
		t.Errorf("after undo = %v, want original order", got)
	}
	// Restored objects keep their IDs.
	if o, ok := s.Object(ids[1]); !ok || o.Name != "b" {
		t.Errorf("Object(%d) after undo = %v, %v", ids[1], o, ok)
	}
}

func TestGroupIsOneUndoStep(t *testing.T) {
	s := newTestScene(t)
	keep := s.Add(Object{Type: "plane"})
	s.Group(func() {
		s.Add(Object{Type: "cylinder"})
		s.Add(Object{Type: "sphere"})
		s.Delete(keep[0])
	})
	if err := s.Undo(); err != nil {
		t.Fatal(err)
	}
	if got := types(s); !slices.Equal(got, []string{"plane"}) {
		t.Errorf("after undoing group = %v, want [plane]", got)
	}
}

func TestDuplicate(t *testing.T) {
	s := newTestScene(t)
	id := s.Add(Object{Type: "cube", Name: "Tower", Position: [3]float32{1, 0, 0}})[0]
	n, err := s.Duplicate(id, 2, [3]float32{2, 0, 0})
	if err != nil || n != 2 {
		t.Fatalf("Duplicate = %d, %v", n, err)
	}
	if len(s.objects) != 3 || s.objects[2].Position[0] != 5 || s.objects[1].Name != "" {
		t.Errorf("clones = %+v %+v", *s.objects[1], *s.objects[2])
	}
	if n, _ := s.Duplicate(id, 100, [3]float32{}); n != 20 {
		t.Errorf("Duplicate(100) = %d, want capped at 20", n)
	}
	s.Undo()
	if len(s.objects) != 3 {
		t.Errorf("undo of duplicate left %d objects, want 3", len(s.objects))
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	s := newTestScene(t)
	s.Add(
		Object{Type: "cube", Position: [3]float32{1, 2, 3}, Scale: [3]float32{1, 1, 1}, Name: "Tower", Color: [3]float32{1, 0, 0}},
		Object{Type: "sphere", Motion: "bob"},
	)
	s.objects[1].SetPhysics(false)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.FromSlash(defaultScenePath))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "id:") || strings.Contains(string(data), "body") {
		t.Errorf("runtime fields leaked into scene file:\n%s", data)
	}

	loaded := New() // same working directory
	if len(loaded.objects) != 2 {
		t.Fatalf("loaded %d objects, want 2", len(loaded.objects))
	}
	a, b := loaded.objects[0], loaded.objects[1]
	if a.Name != "Tower" || a.Position != [3]float32{1, 2, 3} || a.Color != [3]float32{1, 0, 0} || !a.PhysicsEnabled() {
		t.Errorf("loaded cube = %+v", *a)
	}
	if b.PhysicsEnabled() || b.Motion != "bob" || b.ID == 0 || b.ID == a.ID {
		t.Errorf("loaded sphere = %+v", *b)
	}
}

func TestClear(t *testing.T) {
	s := newTestScene(t)
	id := s.Add(Object{Type: "cube"})[0]
	s.Select(id)
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	if len(s.objects) != 0 || s.selected != 0 {
		t.Errorf("after Clear: %d objects, selected %d", len(s.objects), s.selected)
	}
	if err := s.Undo(); err == nil {
		t.Error("undo after Clear should have nothing to undo")
	}
	// Clearing inside a group must not break later grouping.
	s.Group(func() { s.Add(Object{Type: "cube"}); s.Clear() })
	s.Add(Object{Type: "sphere"})
	if err := s.Undo(); err != nil || len(s.objects) != 0 {
		t.Errorf("undo after grouped Clear: err=%v, %d objects left", err, len(s.objects))
	}
}

func TestStepPhysicsCubeLandsOnPlaneRegardlessOfOrder(t *testing.T) {
	for _, planeFirst := range []bool{true, false} {
		s := newTestScene(t)
		cube := Object{Type: "cube", Position: [3]float32{0, 3, 0}}
		plane := Object{Type: "plane", Scale: [3]float32{10, 1, 10}}
		plane.SetPhysics(false)
		if planeFirst {
			s.Add(plane, cube)
		} else {
			s.Add(cube, plane)
		}
		for range 300 {
			s.stepPhysics(1.0 / 60)
		}
		var got *Object
		for _, o := range s.objects {
			if o.Type == "cube" {
				got = o
			}
		}
		// Plane top is at 0.05, so the unit cube rests with its center at 0.55.
		if y := got.Position[1]; y < 0.54 || y > 0.56 {
			t.Errorf("planeFirst=%v: cube rests at y=%v, want 0.55", planeFirst, y)
		}
	}
}

func TestStepPhysicsStaticObjectsDoNotMove(t *testing.T) {
	s := newTestScene(t)
	o := Object{Type: "cube", Position: [3]float32{0, 5, 0}}
	o.SetPhysics(false)
	s.Add(o)
	s.stepPhysics(1)
	if s.objects[0].Position[1] != 5 {
		t.Errorf("static cube moved to %v", s.objects[0].Position)
	}
}

func TestObjectAtCameraCenter(t *testing.T) {
	s := newTestScene(t)
	if _, err := s.ObjectAtCameraCenter(); err == nil {
		t.Error("empty scene: want error")
	}
	s.Camera.Position = rl.NewVector3(0, 0, 10)
	s.Camera.Target = rl.NewVector3(0, 0, 0)
	s.Add(Object{Type: "cube", Name: "far", Position: [3]float32{0, 0, -5}}, Object{Type: "cube", Name: "near"}, Object{Type: "cube", Name: "aside", Position: [3]float32{5, 0, 5}})
	o, err := s.ObjectAtCameraCenter()
	if err != nil || o.Name != "near" {
		t.Errorf("ObjectAtCameraCenter = %v, %v; want near", o, err)
	}
}

func TestSetLighting(t *testing.T) {
	s := newTestScene(t)
	if err := s.SetLighting("sunset"); err != nil || s.lightDir != lightingProfiles["sunset"] {
		t.Errorf("sunset: %v %v", err, s.lightDir)
	}
	if err := s.SetLighting("dusk"); err == nil {
		t.Error("unknown profile accepted")
	}
}

func TestFindByNameAndLabel(t *testing.T) {
	s := newTestScene(t)
	ids := s.Add(Object{Type: "cube", Name: "Tower"}, Object{Type: "cube"})
	if o, err := s.FindByName("Tower"); err != nil || o.ID != ids[0] {
		t.Errorf("FindByName = %v, %v", o, err)
	}
	if _, err := s.FindByName("tower"); err == nil {
		t.Error("FindByName should be exact")
	}
	named, _ := s.Object(ids[0])
	unnamed, _ := s.Object(ids[1])
	if named.Label() != "Tower" || unnamed.Label() != fmt.Sprintf("#%d", ids[1]) {
		t.Errorf("Labels = %q, %q", named.Label(), unnamed.Label())
	}
}
