package files_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"floe/internal/files"
	"floe/internal/testfs"
)

// Remove, RemoveAll and Rename work on a name itself: a link in a directory removed, or a link removed or renamed,
// goes, and what it leads to stays.
func TestRemoveKeepsTarget(t *testing.T) {
	t.Parallel()

	target := t.TempDir()
	keep := filepath.Join(target, "keep.yaml")
	if err := os.WriteFile(keep, []byte("k"), 0o600); err != nil {
		t.Fatal(err)
	}
	stays := func(how string) {
		t.Helper()
		if _, err := os.Stat(keep); err != nil {
			t.Errorf("%s: what the link leads to is gone: %v", how, err)
		}
	}
	links := map[string]func(t *testing.T, link string){
		"symlink":  func(t *testing.T, link string) { t.Helper(); testfs.Symlink(t, target, link) },
		"junction": func(t *testing.T, link string) { t.Helper(); testfs.Junction(t, link, target) },
	}
	for kind, link := range links {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			page := filepath.Join(dir, "page")
			if err := os.Mkdir(page, 0o700); err != nil {
				t.Fatal(err)
			}
			link(t, filepath.Join(page, "out"))
			if err := files.RemoveAll(page); err != nil {
				t.Errorf("RemoveAll of a directory holding a %s = %v", kind, err)
			}
			stays("RemoveAll of a directory holding a " + kind)

			link(t, filepath.Join(dir, "gone"))
			if err := files.RemoveAll(filepath.Join(dir, "gone")); err != nil {
				t.Errorf("RemoveAll of a %s = %v", kind, err)
			}
			if _, err := os.Lstat(filepath.Join(dir, "gone")); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("RemoveAll of a %s left it: %v", kind, err)
			}
			stays("RemoveAll of a " + kind)

			link(t, filepath.Join(dir, "l"))
			if err := files.Rename(dir, "l", "m"); err != nil {
				t.Errorf("Rename of a %s = %v", kind, err)
			}
			fi, err := os.Lstat(filepath.Join(dir, "m"))
			if err != nil || fi.Mode()&(fs.ModeSymlink|fs.ModeIrregular) == 0 {
				t.Errorf("Rename of a %s moved %v, %v; want the link itself", kind, fi, err)
			}
			stays("Rename of a " + kind)
			if err := files.Remove(filepath.Join(dir, "m")); err != nil {
				t.Errorf("Remove of a %s = %v", kind, err)
			}
			stays("Remove of a " + kind)
		})
	}
	if err := files.RemoveAll(filepath.Join(t.TempDir(), "none")); err != nil {
		t.Errorf("RemoveAll of nothing = %v, want nil as os.RemoveAll", err)
	}
}

// Rename takes a name alone on each side: one with a separator, . or .. is refused, nothing renamed.
func TestRenameNames(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a"), []byte("a"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", ".", "..", "x/y", `x\y`, "../a", filepath.Join(dir, "b")} {
		for _, pair := range [][2]string{{"a", name}, {name, "a"}} {
			err := files.Rename(dir, pair[0], pair[1])
			if !errors.Is(err, fs.ErrInvalid) {
				t.Errorf("Rename(%q, %q) = %v, want fs.ErrInvalid", pair[0], pair[1], err)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "a")); err != nil {
		t.Errorf("a refused Rename moved a: %v", err)
	}
	if err := files.Rename(dir, "a", "b"); err != nil {
		t.Errorf("Rename(a, b) = %v", err)
	}
}

// MkdirTemp makes a new directory in dir and names it under dir as written.
func TestMkdirTemp(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	got, err := files.MkdirTemp(dir, "t-")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(got) != dir {
		t.Errorf("MkdirTemp = %s, want a name under %s as written", got, dir)
	}
	if fi, err := os.Stat(got); err != nil || !fi.IsDir() {
		t.Errorf("MkdirTemp made %v, %v", fi, err)
	}
}
