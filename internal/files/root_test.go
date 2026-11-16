package files_test

import (
	"errors"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"floe/internal/files"
	"floe/internal/testfs"
	"floe/internal/timebox"
	"floe/internal/usererr"
)

// fsOf returns the directory dir as a files.Root, closed at the end of the test, and as a files.DirFS.
func fsOf(t *testing.T, dir string) map[string]fs.FS {
	t.Helper()
	root := files.NewRoot(dir)
	t.Cleanup(func() { _ = root.Close() })
	return map[string]fs.FS{"Root": root, "DirFS": files.DirFS(dir)}
}

// Both keep to what an fs.FS promises, as testing/fstest checks it.
func TestFSContract(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for name, b := range map[string]string{"a.yaml": "a", "sub/b.png": "b"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(b), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for via, fsys := range fsOf(t, dir) {
		t.Run(via, func(t *testing.T) {
			t.Parallel()
			testfs.Pending(t, "S315-P4: Open takes sub\\b.png, for the names are not Localized")
			if err := fstest.TestFS(fsys, "a.yaml", "sub/b.png"); err != nil {
				t.Error(err)
			}
		})
		// fstest.TestFS tries bad names on Open and ReadFile alone.
		if _, err := fs.Stat(fsys, "../x"); !errors.Is(err, fs.ErrInvalid) {
			t.Errorf("%s: Stat(../x) = %v, want fs.ErrInvalid", via, err)
		}
	}
}

// A FIFO or a socket is refused by a read or a listing through either, unopened; a Stat tells of it without
// opening it.
func TestFSNotFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", filepath.Join(dir, "s"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	notFileIn(t, dir, "s")
	t.Run("a FIFO", func(t *testing.T) {
		t.Parallel()
		testfs.Mkfifo(t, filepath.Join(dir, "f"))
		notFileIn(t, dir, "f")
	})
}

// notFileIn checks a read, a listing and a Stat of name, under dir through either, as of no regular file.
func notFileIn(t *testing.T, dir, name string) {
	t.Helper()
	for via, fsys := range fsOf(t, dir) {
		timebox.Within(t, via+" "+name, func() {
			if _, err := fs.ReadFile(fsys, name); !errors.Is(err, usererr.ErrNotFile) {
				t.Errorf("%s: ReadFile(%s) = %v, want ErrNotFile", via, name, err)
			}
			if _, err := fs.ReadDir(fsys, name); !errors.Is(err, usererr.ErrNotFile) {
				t.Errorf("%s: ReadDir(%s) = %v, want ErrNotFile", via, name, err)
			}
			if fi, err := fs.Stat(fsys, name); err != nil || fi.Mode().IsRegular() || fi.IsDir() {
				t.Errorf("%s: Stat(%s) = %v, %v, want what it is", via, name, fi, err)
			}
		})
	}
}

// DirFS follows links as os.DirFS does, out of its directory too, and puts names after its directory by their
// letters, for the system to resolve link/.. as os.ReadDir does; Root keeps to its directory.
func TestFSLinks(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	for _, d := range []string{"home", "real/inner", "out"} {
		if err := os.MkdirAll(filepath.Join(tmp, d), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for name, b := range map[string]string{"home/x": "home", "real/x": "real", "out/o.yaml": "o"} {
		if err := os.WriteFile(filepath.Join(tmp, name), []byte(b), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	testfs.Symlink(t, filepath.Join(tmp, "real", "inner"), filepath.Join(tmp, "home", "link"))
	testfs.Symlink(t, filepath.Join(tmp, "out", "o.yaml"), filepath.Join(tmp, "home", "o.yaml"))
	t.Run("link/..", func(t *testing.T) {
		t.Parallel()
		// Win32 folds link\.. by its letters before it looks at the link, for os.ReadDir too.
		testfs.UnixOnly(t, "Windows folds link/.. as written, before the link")
		// By its letters: filepath.Join would fold link/.. itself.
		if b, err := fs.ReadFile(files.DirFS(filepath.Join(tmp, "home", "link")+"/.."), "x"); err != nil || string(b) != "real" {
			t.Errorf("DirFS(home/link/..) read x = %q, %v; want real, as os.ReadDir reads it", b, err)
		}
	})
	home := fsOf(t, filepath.Join(tmp, "home"))
	if b, err := fs.ReadFile(home["DirFS"], "o.yaml"); err != nil || string(b) != "o" {
		t.Errorf("DirFS read a link out = %q, %v", b, err)
	}
	if _, err := fs.ReadFile(home["Root"], "o.yaml"); err == nil {
		t.Error("Root read a link out of it")
	}
}

// DirFS("") opens nothing and says why, as os.DirFS("") does.
func TestDirFSEmpty(t *testing.T) {
	t.Parallel()

	if _, err := fs.ReadFile(files.DirFS(""), "a.yaml"); err == nil {
		t.Error("DirFS(\"\") read a.yaml")
	}
	if _, err := fs.Stat(files.DirFS(""), "a.yaml"); err == nil {
		t.Error("DirFS(\"\") stat a.yaml")
	}
}

// A Root whose directory cannot be opened says so (DirError), at an Open and a Stat alike.
func TestRootDirError(t *testing.T) {
	t.Parallel()

	root := files.NewRoot(filepath.Join(t.TempDir(), "none"))
	defer root.Close()
	if _, err := root.Open("a.png"); !errors.As(err, new(*files.DirError)) {
		t.Errorf("Open = %v, want a DirError", err)
	}
	if _, err := root.Stat("a.png"); !errors.As(err, new(*files.DirError)) {
		t.Errorf("Stat = %v, want a DirError", err)
	}
}
