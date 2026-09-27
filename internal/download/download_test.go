package download

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilenameFromContentDisposition(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"inline", ""},
		{"attachment; filename=foo.png", "foo.png"},
		{`attachment; filename="foo.png"`, "foo.png"},
		{"attachment; filename*=UTF-8''bar.jpg", "bar.jpg"},
		{"attachment; filename*=UTF-8''bar.jpg; size=10", "bar.jpg"},
		{"attachment; filename=plain.gif; size=10", "plain.gif"},
		{`attachment; filename="quoted.png"; size=3`, "quoted.png"},
		{`attachment; filename="with space.png"`, "with space.png"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := filenameFromContentDisposition(tt.in); got != tt.want {
				t.Errorf("filenameFromContentDisposition(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestExtensionFromContentType(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"image/png", ".png"},
		{"IMAGE/PNG", ".png"},
		{"image/jpeg", ".jpg"},
		{"image/jpg; charset=binary", ".jpg"},
		{"image/gif", ".gif"},
		{"image/webp", ".webp"},
		{"application/zip", ".zip"},
		{"application/x-zip-compressed", ".zip"},
		{"font/ttf", ".ttf"},
		{"font/otf", ".otf"},
		{"application/x-font-otf", ".otf"},
		{"application/x-font-ttf", ".ttf"},
		{"application/octet-stream", ""},
		{"text/html", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := extensionFromContentType(tt.in); got != tt.want {
				t.Errorf("extensionFromContentType(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestExtensionFromURL(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"https://example.com/a.png", ".png"},
		{"https://example.com/a.PNG?x=1", ".png"},
		{"https://example.com/a.jpeg", ".jpeg"},
		{"https://example.com/a.jpg", ".jpg"},
		{"https://example.com/a.webp", ".webp"},
		{"https://example.com/a.gif", ".gif"},
		{"https://example.com/font.ttf", ".ttf"},
		{"https://example.com/font.otf", ".otf"},
		{"https://example.com/pack.zip", ".zip"},
		{"https://example.com/a.exe", ""},
		{"https://example.com/noext", ""},
		{"https://example.com/noext?f=a.png", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := extensionFromURL(tt.in); got != tt.want {
				t.Errorf("extensionFromURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFilenameFromURL(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"https://example.com/dir/image.jpg", "image"},
		{"https://example.com/dir/image.jpg?w=100", "image"},
		{"https://example.com/file", "file"},
		{"https://example.com/archive.tar.gz", "archive.tar"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := filenameFromURL(tt.in); got != tt.want {
				t.Errorf("filenameFromURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name, in, want string
	}{
		{"empty", "", "download"},
		{"clean", "photo_01.png", "photo_01.png"},
		{"unsafe chars collapsed", "a b/c?.png", "a_b_c_.png"},
		{"run of unsafe chars", "a   b", "a_b"},
		{"path traversal", "../../etc/passwd", ".._.._etc_passwd"},
		{"truncated to 96", strings.Repeat("x", 200), strings.Repeat("x", 96)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeFilename(tt.in); got != tt.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestDownload(t *testing.T) {
	body := []byte("fake image bytes")
	mux := http.NewServeMux()
	mux.HandleFunc("/images/cat.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(body)
	})
	mux.HandleFunc("/file", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/gif")
		w.Header().Set("Content-Disposition", `attachment; filename="report.gif"`)
		w.Write(body)
	})
	mux.HandleFunc("/photo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(body)
	})
	mux.HandleFunc("/blob", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(body)
	})
	mux.HandleFunc("/ua", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != defaultUserAgent {
			http.Error(w, "bad ua", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Write(body)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tests := []struct {
		name     string
		path     string
		wantFile string
	}{
		{"name and ext from URL and content type", "/images/cat.png", "cat.png"},
		{"name from content disposition", "/file", "report.gif"},
		{"ext appended from content type", "/photo", "photo.jpg"},
		{"unknown type falls back to .bin", "/blob", "blob.bin"},
		{"sends user agent", "/ua", "ua.png"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			destDir := filepath.Join(t.TempDir(), "nested", "dir")
			saved, err := Download(srv.URL+tt.path, destDir)
			if err != nil {
				t.Fatalf("Download error: %v", err)
			}
			if want := filepath.Join(destDir, tt.wantFile); saved != want {
				t.Errorf("saved path = %q, want %q", saved, want)
			}
			got, err := os.ReadFile(saved)
			if err != nil {
				t.Fatalf("read saved file: %v", err)
			}
			if string(got) != string(body) {
				t.Errorf("saved content = %q, want %q", got, body)
			}
		})
	}
}

func TestDownloadNon200(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()

	destDir := t.TempDir()
	_, err := Download(srv.URL+"/missing.png", destDir)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("error = %q, want it to mention HTTP 404", err)
	}
	entries, _ := os.ReadDir(destDir)
	if len(entries) != 0 {
		t.Errorf("destDir has %d entries after failed download, want 0", len(entries))
	}
}

func TestDownloadBadURL(t *testing.T) {
	if _, err := Download("://bad", t.TempDir()); err == nil {
		t.Fatal("expected error for malformed URL")
	}
}
