package files

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"floe/internal/testfs"
	"floe/internal/timebox"
	"floe/internal/usererr"
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

// readAll reads name of o through openRead, which Root and DirFS open through.
func readAll(o Opener, name string) ([]byte, fs.FileInfo, error) {
	f, err := openRead(o, name)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	return b, nil, err //nolint:wrapcheck // the system's
}

// A regular file made a FIFO between what Open (ReadIn) or openRead looks at and what it opens is refused unread,
// at a path, under a DirFS and under an os.Root alike: the open does not block, and what was opened is looked at
// again. Another regular file put in its place, as a save by rename does, is read.
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
		{"a FIFO", testfs.MakeFIFO, usererr.ErrNotFile},
		{"a regular file", func(path string) error { return os.WriteFile(path, []byte("y"), 0o600) }, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.want != nil {
				testfs.UnixOnly(t, "a FIFO")
			}
			openSwapped(t, dir, root, tt.name, tt.swap, tt.want)
		})
	}
}

// openSwapped reads the file a.yaml of dir, which swap makes what it is named, through each opener.
func openSwapped(t *testing.T, dir string, root *os.Root, name string, swap func(string) error, want error) {
	t.Helper()
	for _, o := range []struct {
		via  string
		o    Opener
		name string
		read func(Opener, string) ([]byte, fs.FileInfo, error)
	}{
		{"ReadIn at a path", paths{}, filepath.Join(dir, "a.yaml"), ReadIn},
		{"ReadIn under an os.Root", root, "a.yaml", ReadIn},
		// openRead takes a name under a directory, as fs.ValidPath does.
		{"openRead under a DirFS", dirFS(dir), "a.yaml", readAll},
		{"openRead under an os.Root", root, "a.yaml", readAll},
	} {
		path := filepath.Join(dir, "a.yaml")
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		s := swapped{Opener: o.o, swap: func(string) error {
			if err := os.Remove(path); err != nil {
				return err //nolint:wrapcheck // the test's
			}
			return swap(path)
		}}
		timebox.Within(t, o.via, func() {
			b, _, err := o.read(s, o.name)
			if !errors.Is(err, want) || want == nil && (err != nil || string(b) != "y") {
				t.Errorf("%s of a file made %s = %q, %v, want %v", o.via, name, b, err, want)
			}
		})
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
}

// A regular file made a FIFO between what OpenAppend looks at and what it opens, with a reader there to let the open
// through, is refused after it is opened: looking before is not enough.
func TestOpenAppendSwapped(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "a.log")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	var reader *os.File
	s := swapped{Opener: paths{}, swap: func(string) error {
		if err := os.Remove(path); err != nil {
			return err //nolint:wrapcheck // the test's
		}
		if err := testfs.MakeFIFO(path); err != nil {
			return err //nolint:wrapcheck // the test's
		}
		var err error
		reader, err = os.OpenFile(path, os.O_RDWR, 0) //nolint:gosec // G304: the test's own
		return err                                    //nolint:wrapcheck // the test's
	}}
	f, _, err := openWith(s, path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if reader != nil {
		defer reader.Close() //nolint:errcheck // test
	}
	if f != nil {
		_ = f.Close() //nolint:errcheck // not wanted
	}
	if !errors.Is(err, usererr.ErrNotFile) {
		t.Errorf("openWith of a file made a FIFO = %v, want ErrNotFile", err)
	}
}
