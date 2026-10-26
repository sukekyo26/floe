package files_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sukekyo26/floe/internal/files"
)

func TestIsDataFile(t *testing.T) {
	t.Parallel()

	for file, want := range map[string]bool{"a.yaml": true, "a.YML": true, "a.html": false, "yaml": false} {
		if got := files.IsDataFile(file); got != want {
			t.Errorf("IsDataFile(%q) = %v", file, got)
		}
	}
}

func TestSlugFromFile(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ file, cwd, want string }{
		{"Plan Notes.yaml", "/w", "plan-notes"},
		{"docs/index.html", "/w", "docs"},
		{"index.htm", "/w/site", "site"},
		{"--x--.yaml", "", "x"},
	} {
		if got := files.SlugFromFile(tt.file, tt.cwd); got != tt.want {
			t.Errorf("SlugFromFile(%q, %q) = %q, want %q", tt.file, tt.cwd, got, tt.want)
		}
	}
}

// WriteAtomic writes the whole file with its mode, and leaves no file beside it.
func TestWriteAtomic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "p.html")
	for _, data := range []string{"one", "two"} {
		if err := files.WriteAtomic(path, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(path); err != nil || string(got) != data {
			t.Errorf("read %q, %v, want %q", got, err, data)
		}
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 1 {
		t.Errorf("left %d files, %v", len(entries), err)
	}
	if err := files.WriteAtomic(filepath.Join(dir, "no", "p.html"), nil, 0o644); err == nil {
		t.Error("wrote under no directory")
	}
}
