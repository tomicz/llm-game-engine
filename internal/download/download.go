package download

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

const defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; rv:109.0) Gecko/20100101 Firefox/115.0"

// Download fetches url and saves it under destDir. Filename is derived from the URL path
// or Content-Disposition; extension from URL or Content-Type. Returns the path to the saved file
// (destDir + filename). destDir is created if needed.
func Download(url string, destDir string) (savedPath string, err error) {
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}
	ext := extensionFromContentType(resp.Header.Get("Content-Type"))
	if ext == "" {
		ext = extensionFromURL(url)
	}
	if ext == "" {
		ext = ".bin"
	}
	name := filenameFromContentDisposition(resp.Header.Get("Content-Disposition"))
	if name == "" {
		name = filenameFromURL(url)
	}
	if name == "" {
		name = "download"
	}
	name = sanitizeFilename(name)
	if !strings.HasSuffix(strings.ToLower(name), ext) {
		name = name + ext
	}
	savedPath = filepath.Join(destDir, name)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	out, err := os.Create(savedPath)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = os.Remove(savedPath)
		return "", fmt.Errorf("download: %w", err)
	}
	return savedPath, nil
}

func filenameFromContentDisposition(cd string) string {
	cd = strings.TrimSpace(cd)
	// filename="..."; or filename*=UTF-8''...
	if i := strings.Index(cd, "filename*=UTF-8''"); i >= 0 {
		s := cd[i+len("filename*=UTF-8''"):]
		if j := strings.IndexAny(s, ";\r\n"); j >= 0 {
			s = s[:j]
		}
		return strings.Trim(s, "\"")
	}
	if i := strings.Index(cd, "filename="); i >= 0 {
		s := cd[i+len("filename="):]
		s = strings.Trim(s, "\" ")
		if j := strings.IndexAny(s, ";\r\n"); j >= 0 {
			s = s[:j]
		}
		return s
	}
	return ""
}

func extensionFromContentType(ct string) string {
	ct = strings.ToLower(strings.TrimSpace(ct))
	if idx := strings.Index(ct, ";"); idx >= 0 {
		ct = ct[:idx]
	}
	switch {
	case strings.Contains(ct, "zip"):
		return ".zip"
	case strings.Contains(ct, "font") || strings.Contains(ct, "ttf"):
		return ".ttf"
	case strings.Contains(ct, "otf"):
		return ".otf"
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "jpeg"), strings.Contains(ct, "jpg"):
		return ".jpg"
	case strings.Contains(ct, "gif"):
		return ".gif"
	case strings.Contains(ct, "webp"):
		return ".webp"
	case strings.Contains(ct, "octet-stream"):
		return ""
	}
	return ""
}

func extensionFromURL(url string) string {
	path := url
	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".zip" || ext == ".ttf" || ext == ".otf" || ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif" || ext == ".webp" {
		return ext
	}
	return ""
}

func filenameFromURL(url string) string {
	path := url
	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

var safeNameRe = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)

func sanitizeFilename(name string) string {
	if name == "" {
		return "download"
	}
	name = safeNameRe.ReplaceAllString(name, "_")
	if len(name) > 96 {
		name = name[:96]
	}
	return name
}
