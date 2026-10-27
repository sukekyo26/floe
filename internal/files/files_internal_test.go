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
