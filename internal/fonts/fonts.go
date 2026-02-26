package fonts

import (
	"os"
	"path/filepath"
	"strings"
)

// Extensions we consider as font files.
var Exts = []string{".ttf", ".otf"}

// BaseDirs returns candidate base directories for fonts (relative to process cwd).
// First that exists is typically used when scanning.
func BaseDirs() []string {
	return []string{"assets/fonts", "../../assets/fonts"}
}

// StripAssetsFontsPrefix removes a leading "assets/fonts/" or "assets\fonts\" from path
// so the LLM sending "assets/fonts/SansGoogle.ttf" doesn't produce a double prefix.
func StripAssetsFontsPrefix(path string) string {
	path = strings.TrimSpace(path)
	for _, prefix := range []string{"assets/fonts/", "assets\\fonts\\"} {
		if strings.HasPrefix(path, prefix) {
			return strings.TrimPrefix(path, prefix)
		}
	}
	return path
}

// ScanDir returns relative paths of all font files under dir (e.g. "Inter/Inter-Regular.ttf").
// Paths use forward slashes. Only .ttf and .otf are included.
func ScanDir(dir string) ([]string, error) {
	var out []string
	dir = filepath.Clean(dir)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		for _, e := range Exts {
			if ext == e {
				rel, err := filepath.Rel(dir, path)
				if err != nil {
					return err
				}
				out = append(out, filepath.ToSlash(rel))
				return nil
			}
		}
		return nil
	})
	return out, err
}

// normalizeForMatch lowercases and removes spaces, dashes, and underscores for fuzzy matching.
func normalizeForMatch(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	return s
}

// SearchCandidates returns search terms to try in order when the exact path failed.
// Example: "Inter/Inter-Regular.ttf" -> ["Inter/Inter-Regular.ttf", "Inter"]; "GoogleSans-Regular.ttf" -> ["GoogleSans-Regular.ttf", "GoogleSans"].
// So wrong paths from the LLM still resolve by family name.
func SearchCandidates(pathOrName string) []string {
	seen := map[string]bool{pathOrName: true}
	candidates := []string{pathOrName}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] {
			seen[s] = true
			candidates = append(candidates, s)
		}
	}
	// First path segment (e.g. "Inter" from "Inter/Inter-Regular.ttf")
	if i := strings.IndexAny(pathOrName, "/\\"); i > 0 {
		add(pathOrName[:i])
	}
	// Name before first hyphen (e.g. "GoogleSans" from "GoogleSans-Regular.ttf")
	if i := strings.Index(pathOrName, "-"); i > 0 {
		add(pathOrName[:i])
	}
	// Name without .ttf/.otf extension
	base := pathOrName
	for _, ext := range Exts {
		if strings.HasSuffix(strings.ToLower(base), ext) {
			base = base[:len(base)-len(ext)]
			add(strings.TrimSpace(base))
			break
		}
	}
	// "Sans Google" / "SansGoogle" -> also try "Google Sans" (folder is Google_Sans_Code)
	lower := strings.ToLower(pathOrName)
	if strings.Contains(lower, "sans") && strings.Contains(lower, "google") {
		add("Google Sans")
		add("GoogleSans")
	}
	return candidates
}

// FindFont searches BaseDirs for a font file whose path matches the search term.
// search can be a name like "Inter", "Google Sans", or a partial path like "Inter-Regular".
// Returns the relative path (e.g. "Inter/Inter-Regular.ttf") and the first full path that exists, or error if none match.
// When multiple files match, prefers one whose path contains "Regular" (e.g. Inter-Regular.ttf).
func FindFont(search string) (relPath string, fullPath string, err error) {
	norm := normalizeForMatch(search)
	if norm == "" {
		return "", "", os.ErrNotExist
	}
	var candidates []struct{ rel, full string }
	for _, base := range BaseDirs() {
		list, walkErr := ScanDir(base)
		if walkErr != nil || len(list) == 0 {
			continue
		}
		for _, rel := range list {
			relNorm := normalizeForMatch(rel)
			if strings.Contains(relNorm, norm) {
				full := base + "/" + rel
				if _, err := os.Stat(full); err == nil {
					candidates = append(candidates, struct{ rel, full string }{rel, full})
				}
			}
		}
	}
	if len(candidates) == 0 {
		return "", "", os.ErrNotExist
	}
	// Prefer path containing "regular" when multiple match
	for _, c := range candidates {
		if strings.Contains(strings.ToLower(c.rel), "regular") {
			return c.rel, c.full, nil
		}
	}
	return candidates[0].rel, candidates[0].full, nil
}
