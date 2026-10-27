package files_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/sukekyo26/floe/internal/files"
	"github.com/sukekyo26/floe/internal/usererr"
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

	for file, want := range map[string]string{"Plan Notes.yaml": "plan-notes", "docs/a.b.yaml": "a-b", "--x--.yaml": "x"} {
		if got := files.SlugFromFile(file); got != want {
			t.Errorf("SlugFromFile(%q) = %q, want %q", file, got, want)
		}
	}
}

// WriteAtomic writes the whole file with its mode, and leaves no file beside it.
func TestWriteAtomic(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "p.html")
	for _, data := range []string{"one", "two"} {
		if err := files.WriteAtomic(path, []byte(data), 0o644, nil); err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(path); err != nil || string(got) != data {
			t.Errorf("read %q, %v, want %q", got, err, data)
		}
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 1 {
		t.Errorf("left %d files, %v", len(entries), err)
	}
	if err := files.WriteAtomic(filepath.Join(dir, "no", "p.html"), nil, 0o644, nil); err == nil {
		t.Error("wrote under no directory")
	}
}

func TestIsPage(t *testing.T) {
	t.Parallel()

	for file, want := range map[string]bool{"a.html": true, "a.HTML": true, "a.htm": false, "a.yaml": false, "html": false} {
		if got := files.IsPage(file); got != want {
			t.Errorf("IsPage(%q) = %v", file, got)
		}
	}
}

// Read reads a regular file, and refuses what is neither one nor a directory unopened: a FIFO would block the
// open, so that Read returning at all tells it was not opened. The system's errors stay its own.
func TestRead(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	file, fifo := filepath.Join(dir, "a.yaml"), filepath.Join(dir, "f.yaml")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	if b, info, err := files.Read(file); err != nil || string(b) != "x" || info.Size() != 1 {
		t.Errorf("Read(a regular file) = %q, %v, %v", b, info, err)
	}
	for _, path := range []string{fifo, "/dev/zero"} {
		var pe *fs.PathError
		if _, _, err := files.Read(path); !errors.Is(err, usererr.ErrNotFile) || !errors.As(err, &pe) {
			t.Errorf("Read(%s) = %v, want an *fs.PathError of ErrNotFile", path, err)
		}
	}
	if _, _, err := files.Read(filepath.Join(dir, "gone.yaml")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Read(no file) = %v, want fs.ErrNotExist", err)
	}
	if _, _, err := files.Read(dir); err == nil || errors.Is(err, usererr.ErrNotFile) {
		t.Errorf("Read(a directory) = %v, want the system's error", err)
	}
}

// A new file takes perm less the umask; one written in place of another keeps that one's permissions, not its
// setuid bit, under any umask.
func TestWriteAtomicPermissions(t *testing.T) { //nolint:paralleltest // sets the umask of the process
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)
	dir := t.TempDir()
	path := filepath.Join(dir, "p.html")
	if err := files.WriteAtomic(path, []byte("one"), 0o644, nil); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
		t.Errorf("a new file: %v, %v; want mode 0600 under the umask 077", info, err)
	}
	for _, mode := range []os.FileMode{0o664, 0o200, 0o4755} {
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
		before, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := files.WriteAtomic(path, []byte("two"), 0o644, before); err != nil {
			t.Fatal(err)
		}
		if info, err := os.Lstat(path); err != nil || info.Mode() != mode.Perm() {
			t.Errorf("in place of %v: %v, %v; want %v", mode, info.Mode(), err, mode.Perm())
		}
	}
}
