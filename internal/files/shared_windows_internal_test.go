package files

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floe/internal/testfs"
)

// openedAs tells what open made of path: its error's Op, errno and whether it is fs.ErrNotExist, or the file's name,
// mode, size and contents (a directory's names and kinds).
func openedAs(open func(string) (*os.File, error), path string) string {
	f, err := open(path)
	if err != nil {
		var pe *fs.PathError
		if !errors.As(err, &pe) {
			return "not a PathError: " + err.Error()
		}
		return fmt.Sprintf("op %s, %v, not there %v", pe.Op, pe.Err, errors.Is(err, fs.ErrNotExist))
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return "stat: " + err.Error()
	}
	got := fmt.Sprintf("name %v, mode %v, size %d", f.Name() == path, fi.Mode(), fi.Size())
	if fi.IsDir() {
		// The names and kinds, summed: a drive's root names no file here.
		entries, readErr := f.ReadDir(-1)
		sum := sha256.New()
		for _, e := range entries {
			sum.Write([]byte(e.Name() + " " + e.Type().String() + "\n"))
		}
		return got + fmt.Sprintf(", %d entries %x, %v", len(entries), sum.Sum(nil)[:8], readErr)
	}
	b := make([]byte, 8)
	n, err := f.Read(b)
	return got + fmt.Sprintf(", read %q, %v", b[:n], err)
}

// openShared answers as os.OpenFile does to read, each kind of name: the errors refuseDir and NotDir read after it
// keep their form.
func TestOpenSharedSame(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	write := func(path, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "a.yaml"), "aaa")
	write(filepath.Join(dir, "sub", "b.yaml"), "bbb")
	long := dir
	for len(long) < 300 {
		long = filepath.Join(long, strings.Repeat("d", 40))
	}
	write(filepath.Join(long, "c.yaml"), "ccc")
	testfs.Symlink(t, filepath.Join(dir, "a.yaml"), filepath.Join(dir, "lf"))
	testfs.Symlink(t, filepath.Join(dir, "sub"), filepath.Join(dir, "ld"))
	testfs.Symlink(t, filepath.Join(dir, "gone"), filepath.Join(dir, "lg"))
	testfs.Junction(t, filepath.Join(dir, "j"), filepath.Join(dir, "sub"))
	same := func(t *testing.T, name, path string) {
		t.Helper()
		plain := openedAs(func(p string) (*os.File, error) { return os.OpenFile(p, os.O_RDONLY, 0) }, path)
		shared := openedAs(func(p string) (*os.File, error) { return openShared(checked{s: p}) }, path)
		if shared != plain {
			t.Errorf("%s:\n  openShared %s\n  os.OpenFile %s", name, shared, plain)
		}
	}
	for name, path := range map[string]string{
		"a file": filepath.Join(dir, "a.yaml"), "a directory": filepath.Join(dir, "sub"),
		"none": filepath.Join(dir, "none"), "none's name": filepath.Join(dir, "none", "x"),
		"a file's name": filepath.Join(dir, "a.yaml", "x"), "a symlink to a file": filepath.Join(dir, "lf"),
		"a symlink to a directory": filepath.Join(dir, "ld"), "a symlink to nothing": filepath.Join(dir, "lg"),
		"a junction": filepath.Join(dir, "j"), "a long path": filepath.Join(long, "c.yaml"),
		`a long \\?\ path`: `\\?\` + filepath.Join(long, "c.yaml"), `a \\?\ file`: `\\?\` + filepath.Join(dir, "a.yaml"),
		`a \\?\ directory`: `\\?\` + filepath.Join(dir, "sub"),
		"the drive's root": `\\?\` + filepath.VolumeName(dir) + `\`,
	} {
		same(t, name, path)
	}
	// Of its own: where there is no alias, the rows above run all the same.
	t.Run("an app alias", func(t *testing.T) {
		t.Parallel()
		same(t, "the AppExecLink", testfs.AppExecLink(t))
	})
}
