package files_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"floe/internal/files"
	"floe/internal/usererr"
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
	// ReadIn reads under an os.Root as Read does: a FIFO unopened.
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if b, _, err := files.ReadIn(root, "a.yaml"); err != nil || string(b) != "x" {
		t.Errorf("ReadIn(a regular file) = %q, %v", b, err)
	}
	if _, _, err := files.ReadIn(root, "f.yaml"); !errors.Is(err, usererr.ErrNotFile) {
		t.Errorf("ReadIn(a FIFO) = %v, want ErrNotFile", err)
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

// OpenAppend makes the file with no wider permissions than 0600 and appends to it, follows a link, and refuses what
// is not a regular file without opening it (a FIFO would block the open, with or without a reader) or the system's
// words for a directory.
func TestOpenAppend(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "a.log")
	for _, text := range []string{"one\n", "two\n"} {
		f, info, err := files.OpenAppend(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&^0o600 != 0 {
			t.Errorf("made with permissions %v, want no wider than 0600", info.Mode().Perm())
		}
		if _, err = f.WriteString(text); err != nil {
			t.Fatal(err)
		}
		if err = f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "one\ntwo\n" {
		t.Errorf("appended twice = %q, %v", b, err)
	}

	link := filepath.Join(dir, "link.log")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	f, _, err := files.OpenAppend(link)
	if err != nil {
		t.Fatalf("OpenAppend of a link to a file = %v", err)
	}
	_, _ = f.WriteString("three\n")                                  //nolint:errcheck // read back below
	_ = f.Close()                                                    //nolint:errcheck // as above
	if b, _ := os.ReadFile(path); string(b) != "one\ntwo\nthree\n" { //nolint:errcheck // as above
		t.Errorf("appended through a link = %q", b)
	}

	fifo := filepath.Join(dir, "fifo")
	if err = syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	fifoLink := filepath.Join(dir, "fifo-link")
	if err = os.Symlink(fifo, fifoLink); err != nil {
		t.Fatal(err)
	}
	// Opening one for writing with no reader blocks for good: the call must come back.
	for _, name := range []string{fifo, fifoLink} {
		got := make(chan error, 1)
		go func() {
			opened, _, openErr := files.OpenAppend(name)
			if opened != nil {
				_ = opened.Close() //nolint:errcheck // not wanted
			}
			got <- openErr
		}()
		select {
		case err = <-got:
			if !errors.Is(err, usererr.ErrNotFile) {
				t.Errorf("OpenAppend of %s = %v, want ErrNotFile", filepath.Base(name), err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("OpenAppend of %s did not come back: it opened a FIFO", filepath.Base(name))
		}
	}
	// With a reader, a plain open succeeds: it is refused all the same.
	reader, err := os.OpenFile(fifo, os.O_RDWR, 0) //nolint:gosec // G304: the test's own
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close() //nolint:errcheck // test
	if f, _, err = files.OpenAppend(fifo); !errors.Is(err, usererr.ErrNotFile) {
		if f != nil {
			_ = f.Close() //nolint:errcheck // not wanted
		}
		t.Errorf("OpenAppend of a FIFO with a reader = %v, want ErrNotFile", err)
	}

	if _, _, err = files.OpenAppend(dir); !errors.Is(err, syscall.EISDIR) {
		t.Errorf("OpenAppend of a directory = %v, want EISDIR", err)
	}
}
