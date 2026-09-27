package agent

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"game-engine/internal/commands"
	"game-engine/internal/scene"
)

func TestParseActions(t *testing.T) {
	tests := []struct {
		name    string
		reply   string
		want    []string // action names
		wantErr bool
	}{
		{"actions array", `{"actions":[{"action":"a"},{"action":"b"}]}`, []string{"a", "b"}, false},
		{"markdown fence", "```json\n{\"actions\":[{\"action\":\"a\"}]}\n```", []string{"a"}, false},
		{"text around", `Sure! {"actions":[{"action":"a"}]} Hope that helps.`, []string{"a"}, false},
		{"braces inside strings", `{"actions":[{"action":"run_cmd","args":["name","{x}"]}]}`, []string{"run_cmd"}, false},
		{"actions as object", `{"actions":{"action":"a"}}`, []string{"a"}, false},
		{"single top-level action", `{"action":"add_objects","count":3}`, []string{"add_objects"}, false},
		{"non-object item kept for reporting", `{"actions":[1,{"action":"a"}]}`, []string{"", "a"}, false},
		{"no json", "I can't do that", nil, true},
		{"invalid json", `{"actions": [}`, nil, true},
		{"no actions", `{"foo":1}`, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseActions(tt.reply)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			var names []string
			for _, a := range got {
				n, _ := a["action"].(string)
				names = append(names, n)
			}
			if !slices.Equal(names, tt.want) {
				t.Errorf("actions = %q, want %q", names, tt.want)
			}
		})
	}
}

type fakeClient struct {
	reply              string
	err                error
	model, system, msg string
}

func (f *fakeClient) Complete(_ context.Context, model, system, msg string) (string, error) {
	f.model, f.system, f.msg = model, system, msg
	return f.reply, f.err
}

func testRegistry(ran *[][]string) *commands.Registry {
	reg := commands.NewRegistry()
	record := func(name string) func([]string) error {
		return func(args []string) error {
			*ran = append(*ran, append([]string{name}, args...))
			return nil
		}
	}
	reg.Register(commands.Command{Name: "grid", Usage: "--show | --hide", Help: "Show or hide the grid.", Run: record("grid")})
	reg.Register(commands.Command{Name: "model", Usage: "[name]", Help: "Set the AI model.", Manual: true, Run: record("model")})
	return reg
}

func TestPlan(t *testing.T) {
	var ran [][]string
	fc := &fakeClient{reply: `{"actions":[{"action":"run_cmd","args":["grid","--hide"]}]}`}
	a := New(fc, testRegistry(&ran))

	actions, err := a.Plan(context.Background(), "", "hide the grid", "Visible: 1. cube (center).")
	if err != nil || len(actions) != 1 {
		t.Fatalf("Plan = %v, %v", actions, err)
	}
	if fc.model != defaultModel {
		t.Errorf("model = %q, want default %q", fc.model, defaultModel)
	}
	if !strings.HasPrefix(fc.msg, "Current camera view: Visible") || !strings.HasSuffix(fc.msg, "User: hide the grid") {
		t.Errorf("user message = %q", fc.msg)
	}
	if len(ran) != 0 {
		t.Error("Plan must not apply actions")
	}

	fc.err = errors.New("offline")
	if _, err := a.Plan(context.Background(), "m", "x", ""); err == nil {
		t.Error("client error not returned")
	}
	fc.err, fc.reply = nil, "nope"
	if _, err := a.Plan(context.Background(), "m", "x", ""); err == nil || !strings.Contains(err.Error(), "LLM response invalid") {
		t.Errorf("bad reply err = %v", err)
	}
}

func TestSystemPromptListsCommandsFromRegistry(t *testing.T) {
	var ran [][]string
	fc := &fakeClient{reply: `{"actions":[]}`}
	a := New(fc, testRegistry(&ran))
	a.Plan(context.Background(), "m", "x", "")
	if !strings.Contains(fc.system, "- cmd grid --show | --hide — Show or hide the grid.") {
		t.Errorf("system prompt missing grid command:\n%s", fc.system)
	}
	if strings.Contains(fc.system, "cmd model") {
		t.Error("system prompt lists a manual command")
	}
}

func TestApply(t *testing.T) {
	var ran [][]string
	a := New(&fakeClient{}, testRegistry(&ran))
	a.RegisterHandler("fail", func(Action) error { return errors.New("broken") })

	got := a.Apply([]Action{
		{"action": "run_cmd", "args": []any{"grid", "--show"}},
		{"action": "run_cmd", "args": []any{"model", "gpt-x"}},
		{"action": "fail"},
		{"action": "teleport"},
		{},
	})
	if len(ran) != 1 || !slices.Equal(ran[0], []string{"grid", "--show"}) {
		t.Errorf("commands run = %q, want only grid --show", ran)
	}
	for _, want := range []string{
		"action 2 (run_cmd): model can only be changed manually",
		"action 3 (fail): broken",
		`action 4: unknown action "teleport"`,
		"action 5: missing action",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("summary %q missing %q", got, want)
		}
	}
	if got := a.Apply([]Action{{"action": "run_cmd", "args": []any{"grid", "--hide"}}}); got != "Done. Applied 1 action(s)." {
		t.Errorf("summary = %q", got)
	}
	if got := a.Apply(nil); got != "No actions to apply." {
		t.Errorf("summary = %q", got)
	}
}

func TestRunCmdArgValidation(t *testing.T) {
	var ran [][]string
	h := runCmdHandler(testRegistry(&ran))
	for _, act := range []Action{{}, {"args": []any{}}, {"args": []any{"grid", 3}}, {"args": "grid"}} {
		if err := h(act); err == nil {
			t.Errorf("run_cmd %v: want error", act)
		}
	}
}

func TestSceneHandlers(t *testing.T) {
	t.Chdir(t.TempDir())
	scn := scene.New()
	var ran [][]string
	a := New(&fakeClient{}, testRegistry(&ran))
	RegisterSceneHandlers(a, scn)

	summary := a.Apply([]Action{
		{"action": "add_object", "type": "cube", "position": []any{1.0, 2.0, 3.0}, "physics": false, "color": []any{1.0, 0.0, 0.0}},
		{"action": "add_objects", "type": "random", "count": 5.0, "pattern": "line", "spacing": 3.0, "scale_min": []any{1.0, 4.0, 1.0}, "scale_max": []any{2.0, 1.0, 2.0}},
		{"action": "add_object", "type": "cone", "position": []any{0.0, 0.0, 0.0}},
		{"action": "add_object", "type": "cube"},
	})
	if !strings.Contains(summary, `unknown type "cone"`) || !strings.Contains(summary, "position: expected [x,y,z]") {
		t.Errorf("summary = %q", summary)
	}
	objs := scn.Objects()
	if len(objs) != 6 {
		t.Fatalf("scene has %d objects, want 6", len(objs))
	}
	if c := objs[0]; c.Type != "cube" || c.Position != [3]float32{1, 2, 3} || c.PhysicsEnabled() || c.Color != [3]float32{1, 0, 0} {
		t.Errorf("add_object cube = %+v", *c)
	}
	for i, o := range objs[1:] {
		if o.Position != [3]float32{float32(i) * 3, 0, 0} || !o.PhysicsEnabled() {
			t.Errorf("line object %d = %+v", i, *o)
		}
		// scale_min/scale_max are swapped per axis when reversed.
		if o.Scale[1] < 1 || o.Scale[1] > 4 || o.Scale[0] < 1 || o.Scale[0] > 2 {
			t.Errorf("object %d scale %v outside range", i, o.Scale)
		}
	}
	if err := scn.Undo(); err != nil {
		t.Fatal(err)
	}
	if n := len(scn.Objects()); n != 1 {
		t.Errorf("after undo %d objects, want 1 (add_objects is one undo step)", n)
	}
}

func TestLayout(t *testing.T) {
	origin := [3]float32{10, 1, 10}
	if got := layout("line", 3, 5, origin, 2); got != [3]float32{16, 1, 10} {
		t.Errorf("line = %v", got)
	}
	// 5 items → 3 columns; item 4 is row 1, col 1.
	if got := layout("grid", 4, 5, origin, 2); got != [3]float32{12, 1, 12} {
		t.Errorf("grid = %v", got)
	}
	if got := layout("", 4, 5, origin, 2); got != [3]float32{12, 1, 12} {
		t.Errorf("default pattern = %v, want grid", got)
	}
	for range 50 {
		p := layout("random", 0, 4, origin, 2)
		if p[1] != 1 || p[0] < 5 || p[0] > 15 || p[2] < 5 || p[2] > 15 {
			t.Fatalf("random = %v, want within ±5 of origin", p)
		}
	}
}
