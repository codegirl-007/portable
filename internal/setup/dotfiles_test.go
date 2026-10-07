package setup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFilterExisting(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitconfig"), []byte("[user]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".config/nvim"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := FilterExisting(dir, []string{".gitconfig", ".config/nvim", ".tmux.conf"})
	if len(got) != 2 {
		t.Fatalf("FilterExisting = %v, want 2 paths", got)
	}
}

func TestDotfileSuggestionsIncludesGitconfig(t *testing.T) {
	opts := DotfileSuggestions(false)
	if len(opts) < 2 || opts[0].Path != ".gitconfig" {
		t.Fatalf("opts = %+v", opts)
	}
}
