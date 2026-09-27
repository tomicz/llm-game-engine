package ui

import (
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func TestParseHexColor(t *testing.T) {
	tests := []struct {
		in     string
		want   rl.Color
		wantOK bool
	}{
		{"#fff", rl.NewColor(255, 255, 255, 255), true},
		{"#000", rl.NewColor(0, 0, 0, 255), true},
		{"#f80", rl.NewColor(255, 136, 0, 255), true},
		{"#1a2B3c", rl.NewColor(0x1a, 0x2b, 0x3c, 255), true},
		{"  #FFFFFF  ", rl.NewColor(255, 255, 255, 255), true},
		{"fff", rl.Black, false},
		{"#ff", rl.Black, false},
		{"#ffff", rl.Black, false},
		{"#1234567", rl.Black, false},
		{"", rl.Black, false},
		{"red", rl.Black, false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, ok := ParseHexColor(tt.in)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("ParseHexColor(%q) = %v, %v; want %v, %v", tt.in, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestParsePx(t *testing.T) {
	tests := []struct {
		in     string
		want   int32
		wantOK bool
	}{
		{"10", 10, true},
		{"10px", 10, true},
		{" 12 px ", 12, true},
		{"0", 0, true},
		{"-5", -5, true},
		{"1.5", 0, false},
		{"abc", 0, false},
		{"", 0, false},
		{"px", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, ok := ParsePx(tt.in)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("ParsePx(%q) = %d, %v; want %d, %v", tt.in, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestParsePct(t *testing.T) {
	tests := []struct {
		in     string
		want   int32
		wantOK bool
	}{
		{"50%", 50, true},
		{"0%", 0, true},
		{"100%", 100, true},
		{" 25% ", 25, true},
		{"101%", 0, false},
		{"-1%", 0, false},
		{"%", 0, false},
		{"50", 0, false},
		{"abc%", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, ok := ParsePct(tt.in)
			if ok != tt.wantOK || got != tt.want {
				t.Errorf("ParsePct(%q) = %d, %v; want %d, %v", tt.in, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestResolveProps(t *testing.T) {
	def := DefaultComputedStyle()

	tests := []struct {
		name  string
		props map[string]string
		want  func() ComputedStyle
	}{
		{
			name:  "empty gives defaults",
			props: nil,
			want:  func() ComputedStyle { return def },
		},
		{
			name:  "colors",
			props: map[string]string{"background": "#102030", "color": "#fff"},
			want: func() ComputedStyle {
				s := def
				s.Background = rl.NewColor(0x10, 0x20, 0x30, 255)
				s.Color = rl.NewColor(255, 255, 255, 255)
				return s
			},
		},
		{
			name:  "border sets HasBorder",
			props: map[string]string{"border": "#f00"},
			want: func() ComputedStyle {
				s := def
				s.Border = rl.NewColor(255, 0, 0, 255)
				s.HasBorder = true
				return s
			},
		},
		{
			name:  "invalid color ignored",
			props: map[string]string{"border": "red", "background": "nope"},
			want:  func() ComputedStyle { return def },
		},
		{
			name:  "size",
			props: map[string]string{"width": "200px", "height": " 50 "},
			want: func() ComputedStyle {
				s := def
				s.Width, s.Height = 200, 50
				return s
			},
		},
		{
			name:  "left/top pixels",
			props: map[string]string{"left": "10px", "top": "20"},
			want: func() ComputedStyle {
				s := def
				s.Left, s.Top = 10, 20
				return s
			},
		},
		{
			name:  "left/top percent",
			props: map[string]string{"left": "50%", "top": "100%"},
			want: func() ComputedStyle {
				s := def
				s.LeftPct, s.TopPct = 50, 100
				return s
			},
		},
		{
			name:  "x/y aliases",
			props: map[string]string{"x": "7", "y": "25%"},
			want: func() ComputedStyle {
				s := def
				s.Left = 7
				s.TopPct = 25
				return s
			},
		},
		{
			name:  "padding zero allowed",
			props: map[string]string{"padding": "0"},
			want: func() ComputedStyle {
				s := def
				s.Padding = 0
				return s
			},
		},
		{
			name:  "negative padding ignored",
			props: map[string]string{"padding": "-3px"},
			want:  func() ComputedStyle { return def },
		},
		{
			name:  "unknown property ignored",
			props: map[string]string{"box-shadow": "1px"},
			want:  func() ComputedStyle { return def },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, want := ResolveProps(tt.props), tt.want(); got != want {
				t.Errorf("ResolveProps(%v) = %+v, want %+v", tt.props, got, want)
			}
		})
	}
}

func TestDefaultComputedStyle(t *testing.T) {
	s := DefaultComputedStyle()
	if s.Background.A != 0 {
		t.Errorf("background alpha = %d, want 0 (transparent)", s.Background.A)
	}
	if s.Color != rl.White {
		t.Errorf("color = %v, want white", s.Color)
	}
	if s.HasBorder {
		t.Error("HasBorder = true, want false")
	}
	if s.LeftPct != -1 || s.TopPct != -1 {
		t.Errorf("LeftPct/TopPct = %d/%d, want -1/-1", s.LeftPct, s.TopPct)
	}
	if s.Padding != 4 {
		t.Errorf("Padding = %d, want 4", s.Padding)
	}
}
