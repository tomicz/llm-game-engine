package fonts

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestStripAssetsFontsPrefix(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"assets/fonts/Inter/Inter-Regular.ttf", "Inter/Inter-Regular.ttf"},
		{`assets\fonts\Inter\Inter.ttf`, `Inter\Inter.ttf`},
		{"  assets/fonts/x.ttf  ", "x.ttf"},
		{"Inter/Inter.ttf", "Inter/Inter.ttf"},
		{"other/assets/fonts/x.ttf", "other/assets/fonts/x.ttf"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := StripAssetsFontsPrefix(tt.in); got != tt.want {
				t.Errorf("StripAssetsFontsPrefix(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeForMatch(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Open Sans-Bold_x", "opensansboldx"},
		{"Inter", "inter"},
		{"  ", ""},
		{"Roboto/static/Roboto-Regular.ttf", "roboto/static/robotoregular.ttf"},
	}
	for _, tt := range tests {
		if got := normalizeForMatch(tt.in); got != tt.want {
			t.Errorf("normalizeForMatch(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSearchCandidates(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"Roboto", []string{"Roboto"}},
		{
			"Inter/Inter-Regular.ttf",
			[]string{"Inter/Inter-Regular.ttf", "Inter", "Inter/Inter", "Inter/Inter-Regular"},
		},
		{
			"GoogleSans-Regular.ttf",
			[]string{"GoogleSans-Regular.ttf", "GoogleSans", "GoogleSans-Regular", "Google Sans"},
		},
		{
			"Sans Google",
			[]string{"Sans Google", "Google Sans", "GoogleSans"},
		},
		{
			"Font.OTF",
			[]string{"Font.OTF", "Font"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := SearchCandidates(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SearchCandidates(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestScanDir(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"a.ttf", "sub/b.OTF", "sub/deeper/c.otf", "readme.txt", "sub/d.woff2"} {
		p := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	got, err := ScanDir(dir)
	if err != nil {
		t.Fatalf("ScanDir error: %v", err)
	}
	want := []string{"a.ttf", "sub/b.OTF", "sub/deeper/c.otf"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ScanDir = %v, want %v", got, want)
	}
}

func TestScanDirMissing(t *testing.T) {
	got, err := ScanDir(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Errorf("ScanDir(missing) error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("ScanDir(missing) = %v, want empty", got)
	}
}
