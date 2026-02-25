package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// downloadImage fetches the image at url and saves it under dir (e.g. "assets/textures/downloaded").
// Returns the relative path to the saved file (e.g. "assets/textures/downloaded/abc.png") and an error.
// defaultUserAgent is sent so hosts that block non-browser clients (e.g. Freepik) allow the download.
const defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; rv:109.0) Gecko/20100101 Firefox/115.0"

func downloadImage(url string, dir string) (relPath string, err error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}
	ext := extensionFromContentType(resp.Header.Get("Content-Type"))
	if ext == "" {
		ext = extensionFromURL(url)
	}
	if ext == "" {
		ext = ".png"
	}
	name := filenameFromURL(url)
	if name == "" {
		name = "image"
	}
	name = sanitizeFilename(name) + ext
	fullPath := filepath.Join(dir, name)
	if err := mkdirAll(dir); err != nil {
		return "", err
	}
	out, err := createFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = removeFile(fullPath)
		return "", fmt.Errorf("write file: %w", err)
	}
	// Return path suitable for scene (same regardless of cwd).
	return filepath.Join(dir, name), nil
}

func extensionFromContentType(ct string) string {
	ct = strings.ToLower(strings.TrimSpace(ct))
	if idx := strings.Index(ct, ";"); idx >= 0 {
		ct = ct[:idx]
	}
	switch {
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "jpeg"), strings.Contains(ct, "jpg"):
		return ".jpg"
	case strings.Contains(ct, "gif"):
		return ".gif"
	case strings.Contains(ct, "webp"):
		return ".webp"
	}
	return ""
}

func extensionFromURL(url string) string {
	path := url
	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}
	ext := filepath.Ext(path)
	ext = strings.ToLower(ext)
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return ext
	}
	return ""
}

var safeNameRe = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)

func filenameFromURL(url string) string {
	path := url
	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	return base
}

func sanitizeFilename(name string) string {
	if name == "" {
		return "image"
	}
	name = safeNameRe.ReplaceAllString(name, "_")
	if len(name) > 64 {
		name = name[:64]
	}
	return name
}

// Wrappers so we can test or override; in production use os package.
var (
	mkdirAll   = func(path string) error { return os.MkdirAll(path, 0755) }
	createFile = func(path string) (io.WriteCloser, error) { return os.Create(path) }
	removeFile = os.Remove
)
