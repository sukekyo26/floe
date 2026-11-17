package files_test

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"floe/internal/files"
	"floe/internal/testfs"
)

// mustReal is files.Real of path, which must be there.
func mustReal(tb testing.TB, path string) string {
	tb.Helper()
	got, err := files.Real(path)
	if err != nil {
		tb.Fatal(err)
	}
	return got
}

// Real names a file there by the same real path whatever names it, through a symlink too, and again from its own
// result; it fails for a file not there and for a relative path.
func TestReal(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file := filepath.Join(dir, "a.html")
	write(t, file, "a")
	got := mustReal(t, file)
	if !filepath.IsAbs(got) || filepath.Base(got) != "a.html" {
		t.Errorf("Real(%q) = %q, want an absolute path to a.html", file, got)
	}
	if again := mustReal(t, got); again != got {
		t.Errorf("Real of its own result %q = %q", got, again)
	}
	if _, err := files.Real(filepath.Join(dir, "none.html")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Real of a file not there = %v, want fs.ErrNotExist", err)
	}
	// A relative path to a file there (this test's own, from the package's directory) is refused all the same.
	if rel, err := files.Real("real_test.go"); err == nil {
		t.Errorf("Real of a relative path = %q, want an error", rel)
	}
	t.Run("through a symlink", func(t *testing.T) {
		t.Parallel()
		link := filepath.Join(t.TempDir(), "link.html")
		testfs.Symlink(t, file, link)
		if through := mustReal(t, link); through != got {
			t.Errorf("Real of a symlink = %q, want %q", through, got)
		}
	})
}

// RealAbove takes a path not there as the real path of the directory above it that is, with the names after it as
// written; a .. after a name not there gives false on Unix (dotDotAbove), after a / as after the system's separator.
func TestRealAbove(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	base := mustReal(t, tmp)
	sep := string(filepath.Separator)
	for _, tt := range []struct {
		name, path, want string
		ok               bool
	}{
		{"there", tmp, base, true},
		{"names not there", filepath.Join(tmp, "x", "new"), filepath.Join(base, "x", "new"), true},
		// x is not there either: on Windows, a .. after a name there is folded by its letters before it is opened.
		{".. after a name not there, by the separator", tmp + sep + "x" + sep + "new" + sep + ".." + sep + "parts", dotDotAbove(base), dotDotAbove(base) != ""},
		{".. after a name not there, by /", tmp + "/x/new/../parts", dotDotAbove(base), dotDotAbove(base) != ""},
		{"empty", "", "", false},
		{"relative", "parts", "", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got, ok := files.RealAbove(tt.path); got != tt.want || ok != tt.ok {
				t.Errorf("RealAbove(%q) = %q, %v, want %q, %v", tt.path, got, ok, tt.want, tt.ok)
			}
		})
	}
	t.Run("through a symlink", func(t *testing.T) {
		t.Parallel()
		at := t.TempDir()
		testfs.Symlink(t, tmp, filepath.Join(at, "link"))
		if got, ok := files.RealAbove(filepath.Join(at, "link", "new")); got != filepath.Join(base, "new") || !ok {
			t.Errorf("RealAbove through a symlink = %q, %v, want %q", got, ok, filepath.Join(base, "new"))
		}
	})
}

// LongName gives a name already long back as it is.
func TestLongNameOfLong(t *testing.T) {
	t.Parallel()

	file := filepath.Join(mustReal(t, t.TempDir()), "a.html")
	write(t, file, "a")
	if got, err := files.LongName(file); err != nil || got != file {
		t.Errorf("LongName(%q) = %q, %v, want it as it is", file, got, err)
	}
}
