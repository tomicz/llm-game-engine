package ui

import (
	"reflect"
	"testing"
)

func TestParseCSS(t *testing.T) {
	tests := []struct {
		name string
		css  string
		want []Rule
	}{
		{
			name: "empty",
			css:  "",
			want: nil,
		},
		{
			name: "class and id selectors",
			css:  ".panel { background: #333; width: 100px; }\n#menu { color: #fff }",
			want: []Rule{
				{Selector: ".panel", Props: map[string]string{"background": "#333", "width": "100px"}},
				{Selector: "#menu", Props: map[string]string{"color": "#fff"}},
			},
		},
		{
			name: "comments stripped",
			css:  "/* header */ .a { /* inline */ color: #000; }",
			want: []Rule{
				{Selector: ".a", Props: map[string]string{"color": "#000"}},
			},
		},
		{
			name: "invalid selectors skipped",
			css:  "body { color: #fff; } .ok { left: 10px; } div.x { top: 1px; } . { a: b; }",
			want: []Rule{
				{Selector: ".ok", Props: map[string]string{"left": "10px"}},
			},
		},
		{
			name: "declarations without colon or key ignored",
			css:  ".a { junk; : nokey; height: 20 }",
			want: []Rule{
				{Selector: ".a", Props: map[string]string{"height": "20"}},
			},
		},
		{
			name: "duplicate selectors kept in order",
			css:  ".a { color: #111; } .a { color: #222; }",
			want: []Rule{
				{Selector: ".a", Props: map[string]string{"color": "#111"}},
				{Selector: ".a", Props: map[string]string{"color": "#222"}},
			},
		},
		{
			name: "unbalanced brace stops parsing",
			css:  ".a { color: #111; } .b { color: #222;",
			want: []Rule{
				{Selector: ".a", Props: map[string]string{"color": "#111"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sheet, err := ParseCSS(tt.css)
			if err != nil {
				t.Fatalf("ParseCSS error: %v", err)
			}
			if !reflect.DeepEqual(sheet.Rules, tt.want) {
				t.Errorf("rules = %#v, want %#v", sheet.Rules, tt.want)
			}
		})
	}
}

func TestResolvePropsLaterRuleWins(t *testing.T) {
	sheet, err := ParseCSS(".a { color: #111; width: 10px; } #x { color: #222; } .a { color: #333; }")
	if err != nil {
		t.Fatal(err)
	}
	e := &Engine{sheet: sheet}

	got := e.resolveProps(NewNode("label", "a", "x", ""))
	want := map[string]string{"color": "#333", "width": "10px"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("class+id: got %v, want %v", got, want)
	}

	got = e.resolveProps(NewNode("label", "", "x", ""))
	want = map[string]string{"color": "#222"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("id only: got %v, want %v", got, want)
	}

	got = e.resolveProps(NewNode("label", "nomatch", "", ""))
	if len(got) != 0 {
		t.Errorf("no match: got %v, want empty", got)
	}

	if got := (&Engine{}).resolveProps(NewNode("label", "a", "", "")); len(got) != 0 {
		t.Errorf("nil sheet: got %v, want empty", got)
	}
}

func TestStripCSSComments(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{"a { b }", "a { b }"},
		{"x/* c */y", "xy"},
		{"/* a *//* b */z", "z"},
		{"keep /* unterminated", "keep "},
		{"keep /* unterminated x", "keep "},
		{"a /* nested /* still comment */ b", "a  b"},
	}
	for _, tt := range tests {
		if got := stripCSSComments(tt.in); got != tt.want {
			t.Errorf("stripCSSComments(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
