package files

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/sukekyo26/floe/internal/usererr"
)

// A regular file made a FIFO between what Open looks at and what it opens is refused unread: the open does not
// block, and what was opened is looked at again. Another regular file put in its place, as a save by rename
// does, is read.
func TestOpenSwapped(t *testing.T) { //nolint:paralleltest // replaces openFile
	path := filepath.Join(t.TempDir(), "a.yaml")
	defer func() { openFile = os.OpenFile }()
	for _, tt := range []struct {
		name string
		swap func() error
		want error
	}{
		{"a FIFO", func() error { return syscall.Mkfifo(path, 0o600) }, usererr.ErrNotFile},
		{"a regular file", func() error { return os.WriteFile(path, []byte("y"), 0o600) }, nil},
	} {
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		openFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
			if err := os.Remove(name); err != nil {
				t.Fatal(err)
			}
			if err := tt.swap(); err != nil {
				t.Fatal(err)
			}
			return os.OpenFile(name, flag, perm) //nolint:gosec // G304: the test's own file
		}
		f, _, err := Open(path)
		if f != nil {
			_ = f.Close()
		}
		if !errors.Is(err, tt.want) || tt.want == nil && err != nil {
			t.Errorf("Open of a file made %s = %v, want %v", tt.name, err, tt.want)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
}

// swapRoot is an *os.Root that puts swap's file in the place of the one opened, between the Stat and the open.
type swapRoot struct {
	*os.Root
	swap func(name string) error
}

func (r swapRoot) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	if err := r.swap(name); err != nil {
		return nil, err
	}
	return r.Root.OpenFile(name, flag, perm) //nolint:wrapcheck // the system's
}

// Under an os.Root too, a regular file made a FIFO between the Stat and the open is refused unread.
func TestReadInSwapped(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.WriteFile(filepath.Join(dir, "a.svg"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	fifo := swapRoot{Root: root, swap: func(name string) error {
		if err := root.Remove(name); err != nil {
			return err //nolint:wrapcheck // the test's
		}
		return syscall.Mkfifo(filepath.Join(dir, name), 0o600) //nolint:wrapcheck // the test's
	}}
	if _, _, err := readWith(fifo, "a.svg"); !errors.Is(err, usererr.ErrNotFile) {
		t.Errorf("ReadIn of a file made a FIFO = %v, want ErrNotFile", err)
	}
}
