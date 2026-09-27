package main

import "testing"

func TestParseVec3(t *testing.T) {
	if v, err := parseVec3("position", []string{"1", "-2.5", "0"}); err != nil || v != [3]float32{1, -2.5, 0} {
		t.Errorf("parseVec3 = %v, %v", v, err)
	}
	for _, args := range [][]string{{"1", "2"}, {"1", "2", "x"}, {"1", "2", "3", "4"}} {
		if _, err := parseVec3("position", args); err == nil {
			t.Errorf("parseVec3(%q): want error", args)
		}
	}
}
