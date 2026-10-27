package files

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/sukekyo26/floe/internal/usererr"
)

// swapped is an Opener that runs swap on the file between looking at it and opening it.
type swapped struct {
	Opener
	swap func(name string) error
}

func (s swapped) OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error) {
	if err := s.swap(name); err != nil {
		return nil, err
	}
	return s.Opener.OpenFile(name, flag, perm) //nolint:wrapcheck // the system's
}

// A regular file made a FIFO between what Open looks at and what it opens is refused unread, at a path and under
// an os.Root alike: the open does not block, and what was opened is looked at again. Another regular file put in
// its place, as a save by rename does, is read.
func TestOpenSwapped(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for _, tt := range []struct {
		name string
		swap func(path string) error
		want error
	}{
		{"a FIFO", func(path string) error { return syscall.Mkfifo(path, 0o600) }, usererr.ErrNotFile},
		{"a regular file", func(path string) error { return os.WriteFile(path, []byte("y"), 0o600) }, nil},
	} {
		for _, o := range []struct {
			via  string
			o    Opener
			name string
		}{{"a path", paths{}, filepath.Join(dir, "a.yaml")}, {"an os.Root", root, "a.yaml"}} {
			path := filepath.Join(dir, "a.yaml")
			if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
			s := swapped{Opener: o.o, swap: func(string) error {
				if err := os.Remove(path); err != nil {
					return err //nolint:wrapcheck // the test's
				}
				return tt.swap(path)
			}}
			b, _, err := ReadIn(s, o.name)
			if !errors.Is(err, tt.want) || tt.want == nil && (err != nil || string(b) != "y") {
				t.Errorf("ReadIn via %s of a file made %s = %q, %v, want %v", o.via, tt.name, b, err, tt.want)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
		}
	}
}
