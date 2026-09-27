package commands

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		line   string
		want   []string
		wantOK bool
	}{
		{"cmd grid --show", []string{"grid", "--show"}, true},
		{"cmd   spawn  cube 0 -1  0 ", []string{"spawn", "cube", "0", "-1", "0"}, true},
		{"cmd ", nil, true},
		{"cmdgrid", nil, false},
		{"Cmd grid", nil, false},
		{"make a cube", nil, false},
	}
	for _, tt := range tests {
		got, ok := Parse(tt.line)
		if ok != tt.wantOK || !slices.Equal(got, tt.want) {
			t.Errorf("Parse(%q) = %q, %v; want %q, %v", tt.line, got, ok, tt.want, tt.wantOK)
		}
	}
}

func TestExecute(t *testing.T) {
	r := NewRegistry()
	var got []string
	r.Register(Command{Name: "gravity", Usage: "<y>", Run: func(args []string) error { got = args; return nil }})
	boom := errors.New("boom")
	r.Register(Command{Name: "fail", Run: func([]string) error { return boom }})

	// Negative numbers are positional arguments, not flags.
	if err := r.Execute([]string{"gravity", "-9.8"}); err != nil || !slices.Equal(got, []string{"-9.8"}) {
		t.Errorf("Execute gravity: err=%v args=%q", err, got)
	}
	if err := r.Execute([]string{"fail"}); !errors.Is(err, boom) {
		t.Errorf("Execute fail = %v, want boom", err)
	}
	if err := r.Execute([]string{"nope"}); err == nil || !strings.Contains(err.Error(), "unknown command: nope") {
		t.Errorf("unknown command err = %v", err)
	}
	if err := r.Execute(nil); err == nil {
		t.Error("empty args: want error")
	}
}

func TestRegistryOrderAndLookup(t *testing.T) {
	r := NewRegistry()
	for _, n := range []string{"b", "a", "c"} {
		r.Register(Command{Name: n, Run: func([]string) error { return nil }})
	}
	var names []string
	for _, c := range r.Commands() {
		names = append(names, c.Name)
	}
	if !slices.Equal(names, []string{"b", "a", "c"}) {
		t.Errorf("Commands order = %v, want registration order", names)
	}
	if _, ok := r.Lookup("a"); !ok {
		t.Error("Lookup(a) failed")
	}
	defer func() {
		if recover() == nil {
			t.Error("duplicate Register did not panic")
		}
	}()
	r.Register(Command{Name: "a", Run: func([]string) error { return nil }})
}

func TestUsage(t *testing.T) {
	c := Command{Name: "grid", Usage: "--show | --hide"}
	if got := c.UsageError().Error(); got != "usage: cmd grid --show | --hide" {
		t.Errorf("UsageError = %q", got)
	}
	if got := (Command{Name: "save"}).Synopsis(); got != "save" {
		t.Errorf("Synopsis = %q", got)
	}
}

func TestParseToggle(t *testing.T) {
	tests := []struct {
		args             []string
		wantVal, wantSet bool
		wantErr          bool
	}{
		{[]string{"--show"}, true, true, false},
		{[]string{"-show"}, true, true, false},
		{[]string{"--hide"}, false, true, false},
		{nil, false, false, false},
		{[]string{"--show", "--hide"}, false, false, true},
		{[]string{"--bogus"}, false, false, true},
	}
	for _, tt := range tests {
		v, set, err := ParseToggle("grid", tt.args, "show", "hide")
		if v != tt.wantVal || set != tt.wantSet || (err != nil) != tt.wantErr {
			t.Errorf("ParseToggle(%q) = %v, %v, %v", tt.args, v, set, err)
		}
	}
}
