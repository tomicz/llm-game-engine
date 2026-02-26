package googlefonts

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	apiBase = "https://api.github.com/repos/google/fonts/contents/ofl"
)

// Only these hosts are used; no user-supplied URLs.
var allowedRawPrefix = "https://raw.githubusercontent.com/google/fonts/"

type githubFile struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	DownloadURL string `json:"download_url"`
}

// NormalizeFamily converts a display name to a folder name used in google/fonts ofl.
// e.g. "Inter" -> "inter", "Open Sans" -> "opensans". Also try "open-sans" if needed.
func NormalizeFamily(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	lower := strings.ToLower(name)
	noSpaces := strings.ReplaceAll(lower, " ", "")
	withHyphens := strings.ReplaceAll(lower, " ", "-")
	out := []string{noSpaces}
	if withHyphens != noSpaces {
		out = append(out, withHyphens)
	}
	return out
}

// FetchDownloadURL returns the raw download URL for a TTF file in the given folder.
// Prefers a file whose name does not contain "Italic". Only returns URLs from google/fonts (safe).
func FetchDownloadURL(folder string) (downloadURL string, err error) {
	u := apiBase + "/" + url.PathEscape(folder)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("google fonts: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("font %q not found on Google Fonts", folder)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("google fonts: HTTP %d", resp.StatusCode)
	}
	var files []githubFile
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return "", fmt.Errorf("google fonts: %w", err)
	}
	var preferred, fallback string
	for _, f := range files {
		if f.Type != "file" || f.DownloadURL == "" {
			continue
		}
		lower := strings.ToLower(f.Name)
		if !strings.HasSuffix(lower, ".ttf") && !strings.HasSuffix(lower, ".otf") {
			continue
		}
		if !strings.HasPrefix(f.DownloadURL, allowedRawPrefix) {
			continue
		}
		if strings.Contains(lower, "italic") {
			if fallback == "" {
				fallback = f.DownloadURL
			}
			continue
		}
		preferred = f.DownloadURL
		break
	}
	if preferred != "" {
		return preferred, nil
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", fmt.Errorf("no .ttf/.otf file found for %q on Google Fonts", folder)
}

// FetchDownloadURLByFamily tries NormalizeFamily(name) variants and returns the first successful download URL.
func FetchDownloadURLByFamily(name string) (downloadURL string, err error) {
	candidates := NormalizeFamily(name)
	if len(candidates) == 0 {
		return "", fmt.Errorf("invalid font name")
	}
	var lastErr error
	for _, folder := range candidates {
		u, err := FetchDownloadURL(folder)
		if err == nil {
			return u, nil
		}
		lastErr = err
	}
	return "", lastErr
}
