package files_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"floe/internal/files"
	"floe/internal/testfs"
)

// A file on a path's way is told apart from a directory not there yet, on Windows too (W12), with the words Unix
// says; looked up from the directory of a file read, a leaf made since is not taken for one on the way.
func TestNotDir(t *testing.T) {
	t.Parallel()

	d := t.TempDir()
	f := filepath.Join(d, "f")
	if err := os.WriteFile(f, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := files.OpenRoot(d)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	stat := func(p string) error { _, err := files.Stat(p); return err }
	readDir := func(p string) error { _, err := files.ReadDir(p); return err }
	read := func(p string) error { _, _, err := files.Read(p); return err }
	for _, c := range []struct {
		name, from string
		err        error
		notDir     bool
	}{
		{"stat under a file", filepath.Join(f, "x"), stat(filepath.Join(f, "x")), true},
		{"stat two under a file", filepath.Join(f, "x", "y"), stat(filepath.Join(f, "x", "y")), true},
		{"list a file", f, readDir(f), true},
		{"make under a file", filepath.Join(f, "x", "y"), files.MkdirAll(filepath.Join(f, "x", "y"), 0o700), true},
		{"read under a file", f, read(filepath.Join(f, "x.yaml")), true},
		{"in a root under a file", f, func() error { _, _, err := files.ReadIn(root, "f/x.svg"); return err }(), true},
		{"not there", filepath.Join(d, "gone"), stat(filepath.Join(d, "gone")), false},
		{"under what is not there", filepath.Join(d, "gone", "x"), stat(filepath.Join(d, "gone", "x")), false},
		{"two under what is not there", filepath.Join(d, "gone", "x", "y"), stat(filepath.Join(d, "gone", "x", "y")), false},
		{"no error", d, nil, false},
		{"another error", d, errors.New("odd"), false},
	} {
		got := files.NotDir(c.from, c.err)
		if (got != nil) != c.notDir || got != nil && (!errors.Is(got, files.ErrNotDir) || errors.Is(got, fs.ErrNotExist)) {
			t.Errorf("%s: NotDir(%s, %v) = %v", c.name, c.from, c.err, got)
		}
		if pe, ok := errors.AsType[*fs.PathError](c.err); got != nil && (!ok || got.Error() != pe.Op+" "+pe.Path+": not a directory") {
			t.Errorf("%s: told %q", c.name, got)
		}
	}

	t.Run("a leaf made since", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, "sub", "x.yaml")
		_, _, readErr := files.Read(file)
		if err := os.Mkdir(filepath.Join(dir, "sub"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if got := files.NotDir(filepath.Dir(file), readErr); got != nil {
			t.Errorf("NotDir of %s read before it was made = %v", file, got)
		}
	})
	t.Run("not a path error", func(t *testing.T) {
		t.Parallel()
		got := files.NotDir(f, syscall.ENOTDIR)
		if pe, ok := errors.AsType[*fs.PathError](got); !ok || pe.Op != "stat" || pe.Path != f || !errors.Is(got, files.ErrNotDir) {
			t.Errorf("NotDir of a bare ENOTDIR = %v", got)
		}
	})
	t.Run("stops where it cannot look", func(t *testing.T) {
		testfs.UnixOnly(t, "chmod keeps a directory from being looked in")
		t.Parallel()
		dir := t.TempDir()
		locked := filepath.Join(dir, "locked")
		if err := os.Mkdir(locked, 0o700); err != nil {
			t.Fatal(err)
		}
		// Windows' error, ENOTDIR and not there both, on a path whose look up meets a directory it may not look in.
		err := &fs.PathError{Op: "stat", Path: filepath.Join(locked, "a", "b"), Err: errnoBoth{}}
		if err := os.Chmod(locked, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(locked, 0o700) }) //nolint:errcheck // for TempDir to remove it
		if os.Geteuid() == 0 {
			t.Skip("root looks in any directory")
		}
		if got := files.NotDir(filepath.Join(locked, "a"), err); got != nil {
			t.Errorf("NotDir under a directory it cannot look in = %v", got)
		}
	})
}

// errnoBoth is an error both ENOTDIR and not there, as Windows' ERROR_PATH_NOT_FOUND is to Go.
type errnoBoth struct{}

func (errnoBoth) Error() string { return "the system cannot find the path specified" }

func (errnoBoth) Is(target error) bool { return target == syscall.ENOTDIR || target == fs.ErrNotExist }

// A file read is refused when it is a directory, with the words Unix says reading one, through each way files reads
// one; a directory is still opened to list it.
func TestReadDirectory(t *testing.T) {
	t.Parallel()

	d := t.TempDir()
	sub := filepath.Join(d, "sub")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	root := files.NewRoot(d)
	t.Cleanup(func() { _ = root.Close() })
	osRoot, err := files.OpenRoot(d)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = osRoot.Close() })
	open := func() error {
		f, _, err := files.Open(sub)
		if f != nil {
			_ = f.Close()
		}
		return err
	}
	// Each told at the name it was given: a path, or the name under the directory of an fs.FS.
	for name, c := range map[string]struct {
		err  error
		path string
	}{
		"Open":               {open(), sub},
		"Read":               {func() error { _, _, err := files.Read(sub); return err }(), sub},
		"ReadIn":             {func() error { _, _, err := files.ReadIn(osRoot, "sub"); return err }(), "sub"},
		"fs.ReadFile(DirFS)": {func() error { _, err := fs.ReadFile(files.DirFS(d), "sub"); return err }(), "sub"},
		"fs.ReadFile(Root)":  {func() error { _, err := fs.ReadFile(root, "sub"); return err }(), "sub"},
	} {
		pe, ok := errors.AsType[*fs.PathError](c.err)
		if !errors.Is(c.err, syscall.EISDIR) || !ok || pe.Err.Error() != "is a directory" || pe.Path != c.path {
			t.Errorf("%s of a directory = %v, want EISDIR at %s", name, c.err, c.path)
		}
	}
	for name, fsys := range map[string]fs.FS{"DirFS": files.DirFS(d), "Root": root} {
		if entries, err := fs.ReadDir(fsys, "."); err != nil || len(entries) != 1 {
			t.Errorf("fs.ReadDir(%s) = %v, %v", name, entries, err)
		}
		if err := fs.WalkDir(fsys, ".", func(string, fs.DirEntry, error) error { return nil }); err != nil {
			t.Errorf("fs.WalkDir(%s) = %v", name, err)
		}
	}
}
