package assets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCandidates(t *testing.T) {
	got := Candidates("assets/ui/default.css")
	want := []string{"assets/ui/default.css", "../../assets/ui/default.css"}
	if len(got) != len(want) {
		t.Fatalf("Candidates = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != filepath.FromSlash(want[i]) {
			t.Errorf("Candidates[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if abs := Candidates("/tmp/x.png"); len(abs) != 1 || abs[0] != "/tmp/x.png" {
		t.Errorf("absolute path candidates = %v", abs)
	}
}

func TestFind(t *testing.T) {
	// Layout: <root>/assets/a.txt and <root>/cmd/game as the working directory.
	root := t.TempDir()
	game := filepath.Join(root, "cmd", "game")
	if err := os.MkdirAll(game, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", "a.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(game, "local.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(game)

	if p, ok := Find("assets/a.txt"); !ok || p != filepath.Join("..", "..", "assets", "a.txt") {
		t.Errorf("Find from cmd/game = %q, %v", p, ok)
	}
	if p, ok := Find("missing.txt", "local.txt"); !ok || p != "local.txt" {
		t.Errorf("Find fallback = %q, %v", p, ok)
	}
	if _, ok := Find("missing.txt"); ok {
		t.Error("Find(missing) = true")
	}
}
