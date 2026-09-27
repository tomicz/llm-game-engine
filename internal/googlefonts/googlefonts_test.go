package googlefonts

import (
	"reflect"
	"testing"
)

func TestNormalizeFamily(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"Inter", []string{"inter"}},
		{"Open Sans", []string{"opensans", "open-sans"}},
		{"  Roboto Mono  ", []string{"robotomono", "roboto-mono"}},
		{"Noto Sans JP", []string{"notosansjp", "noto-sans-jp"}},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := NormalizeFamily(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NormalizeFamily(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
