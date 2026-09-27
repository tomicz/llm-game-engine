// Package assets resolves paths to files under the repository's assets/ directory.
//
// The engine is started either from the repository root or from cmd/game, so relative asset paths
// are looked up under each search root in order. Files the engine writes (downloads, logs, config)
// stay relative to the working directory.
package assets

import (
	"os"
	"path/filepath"
)

// searchRoots are the directories tried, in order, when resolving a relative path.
var searchRoots = []string{".", filepath.Join("..", "..")}

// Candidates returns rel under each search root, in search order. Absolute paths are returned as is.
func Candidates(rel string) []string {
	if filepath.IsAbs(rel) {
		return []string{filepath.Clean(rel)}
	}
	out := make([]string, len(searchRoots))
	for i, root := range searchRoots {
		out[i] = filepath.Join(root, rel)
	}
	return out
}

// Find returns the first existing path among the candidates of each rel, trying rels in order.
func Find(rels ...string) (string, bool) {
	for _, rel := range rels {
		for _, p := range Candidates(rel) {
			if _, err := os.Stat(p); err == nil {
				return p, true
			}
		}
	}
	return "", false
}
