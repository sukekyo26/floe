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
// block, and what was opened is looked at again.
func TestOpenSwapped(t *testing.T) { //nolint:paralleltest // replaces openFile
	path := filepath.Join(t.TempDir(), "a.yaml")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	defer func() { openFile = os.OpenFile }()
	openFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
		if err := os.Remove(name); err != nil {
			t.Fatal(err)
		}
		if err := syscall.Mkfifo(name, 0o600); err != nil {
			t.Fatal(err)
		}
		return os.OpenFile(name, flag, perm) //nolint:gosec // G304: the test's own file
	}
	if f, _, err := Open(path); !errors.Is(err, usererr.ErrNotFile) {
		if f != nil {
			_ = f.Close()
		}
		t.Errorf("Open of a file made a FIFO = %v, want ErrNotFile", err)
	}
}
