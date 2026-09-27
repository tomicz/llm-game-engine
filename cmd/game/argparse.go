package main

import (
	"fmt"
	"strconv"
)

// parseFloats parses each arg as a float32; what names the values in error messages.
func parseFloats(what string, args []string) ([]float32, error) {
	out := make([]float32, len(args))
	for i, s := range args {
		f, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid %s %q", what, s)
		}
		out[i] = float32(f)
	}
	return out, nil
}

// parseVec3 parses exactly three floats.
func parseVec3(what string, args []string) ([3]float32, error) {
	var v [3]float32
	if len(args) != 3 {
		return v, fmt.Errorf("%s needs 3 numbers", what)
	}
	f, err := parseFloats(what, args)
	if err != nil {
		return v, err
	}
	copy(v[:], f)
	return v, nil
}
