package archive

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Unzip extracts zipPath into destDir, preserving directory structure.
// destDir is created if needed. Returns the list of extracted file paths (absolute), or an error.
func Unzip(zipPath, destDir string) (extracted []string, err error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("unzip: %w", err)
	}
	defer r.Close()
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, fmt.Errorf("unzip: %w", err)
	}
	for _, f := range r.File {
		dest := filepath.Join(destDir, f.Name)
		dest = filepath.Clean(dest)
		absDest, err := filepath.Abs(dest)
		if err != nil {
			return nil, fmt.Errorf("unzip: %w", err)
		}
		absDir, err := filepath.Abs(destDir)
		if err != nil {
			return nil, fmt.Errorf("unzip: %w", err)
		}
		if !strings.HasPrefix(absDest, absDir+string(os.PathSeparator)) && absDest != absDir {
			continue // skip path escape
		}
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(dest, 0755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return nil, fmt.Errorf("unzip: %w", err)
		}
		out, err := os.Create(dest)
		if err != nil {
			return nil, fmt.Errorf("unzip: %w", err)
		}
		rc, err := f.Open()
		if err != nil {
			out.Close()
			return nil, fmt.Errorf("unzip: %w", err)
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return nil, fmt.Errorf("unzip: %w", err)
		}
		extracted = append(extracted, dest)
	}
	return extracted, nil
}

// FindFontFiles returns paths (relative to baseDir) of .ttf and .otf files under dir.
// Prefers paths containing "Regular" (case-insensitive). baseDir should be the font root (e.g. assets/fonts).
func FindFontFilesInDir(dir, baseDir string) (relPaths []string, err error) {
	dir = filepath.Clean(dir)
	baseDir = filepath.Clean(baseDir)
	var regular []string
	var other []string
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".ttf" && ext != ".otf" {
			return nil
		}
		rel, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.Contains(strings.ToLower(rel), "regular") {
			regular = append(regular, rel)
		} else {
			other = append(other, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	relPaths = append(relPaths, regular...)
	relPaths = append(relPaths, other...)
	return relPaths, nil
}
