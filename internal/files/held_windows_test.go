package files_test

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"

	"floe/internal/files"
)

// holder opens a file of dir/sub/page.html and keeps it open; parent is the directory a rename of which Windows
// refuses while it is held (the file's directory, or, of a Root, a directory under the Root's).
type holder struct {
	name string
	open func(t *testing.T, dir string) (io.ReadSeekCloser, error)
}

func holders() []holder {
	return []holder{
		{"Open", func(_ *testing.T, dir string) (io.ReadSeekCloser, error) {
			f, _, err := files.Open(filepath.Join(dir, "sub", "page.html"))
			return f, err
		}},
		{"DirFS", func(_ *testing.T, dir string) (io.ReadSeekCloser, error) {
			f, err := files.DirFS(filepath.Join(dir, "sub")).Open("page.html")
			if err != nil {
				return nil, err
			}
			if rs, ok := f.(io.ReadSeekCloser); ok {
				return rs, nil
			}
			return nil, errors.New("not seekable")
		}},
		{"Root", func(t *testing.T, dir string) (io.ReadSeekCloser, error) {
			t.Helper()
			r := files.NewRoot(dir)
			t.Cleanup(func() { _ = r.Close() })
			f, err := r.Open("sub/page.html")
			if err != nil {
				return nil, err
			}
			if rs, ok := f.(io.ReadSeekCloser); ok {
				return rs, nil
			}
			return nil, errors.New("not seekable")
		}},
	}
}

// heldTree makes dir/sub/page.html, reading "old", with a new one beside it, and opens it with h.
func heldTree(t *testing.T, h holder) (string, io.ReadSeekCloser) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "page.html"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	f, err := h.open(t, dir)
	if err != nil || f == nil {
		t.Fatalf("%s: %v", h.name, err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return dir, f
}

func readAll(t *testing.T, f io.ReadSeeker) string {
	t.Helper()
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// What floe holds open to read can be written over (WriteAtomic) and removed, its directory too, while it reads on
// what it opened, as on Unix; its directory cannot be renamed (files.Rename's LIMIT), nor the directory a Root holds
// removed (the Root's handle).
func TestHeldOpen(t *testing.T) {
	t.Parallel()

	for _, h := range holders() {
		// Renamed over, after its directory is refused a rename (the file is in it still).
		dir, f := heldTree(t, h)
		page := filepath.Join(dir, "sub", "page.html")
		if err := files.Rename(dir, "sub", "moved"); !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			t.Errorf("%s: Rename of its directory = %v, want Access is denied (the LIMIT)", h.name, err)
		}
		if err := files.WriteAtomic(page, []byte("new"), 0o600, nil); err != nil {
			t.Errorf("%s: WriteAtomic over it = %v", h.name, err)
		}
		if got := readAll(t, f); got != "old" {
			t.Errorf("%s: the holder reads %q after the write, want old", h.name, got)
		}
		// Removed.
		dir, f = heldTree(t, h)
		if err := files.Remove(filepath.Join(dir, "sub", "page.html")); err != nil {
			t.Errorf("%s: Remove of it = %v", h.name, err)
		}
		if got := readAll(t, f); got != "old" {
			t.Errorf("%s: the holder reads %q after the removal, want old", h.name, got)
		}
		// Its directory removed.
		dir, _ = heldTree(t, h)
		if err := files.RemoveAll(filepath.Join(dir, "sub")); err != nil {
			t.Errorf("%s: RemoveAll of its directory = %v", h.name, err)
		}
		if _, err := os.Lstat(filepath.Join(dir, "sub")); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s: its directory is left: %v", h.name, err)
		}
		if h.name == "Root" {
			if err := files.RemoveAll(dir); !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
				t.Errorf("Root: RemoveAll of the Root's directory = %v, want it held (the Root's handle)", err)
			}
		}
	}
}

// An editor's save by ReplaceFile goes through while floe reads the file.
func TestHeldByEditor(t *testing.T) {
	t.Parallel()

	dir, f := heldTree(t, holders()[0])
	page := filepath.Join(dir, "sub", "page.html")
	saved := filepath.Join(dir, "sub", "page.html.new")
	if err := os.WriteFile(saved, []byte("saved"), 0o600); err != nil {
		t.Fatal(err)
	}
	replaced, err := windows.UTF16PtrFromString(page)
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := windows.UTF16PtrFromString(saved)
	if err != nil {
		t.Fatal(err)
	}
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("ReplaceFileW")
	if r, _, err := proc.Call(uintptr(unsafe.Pointer(replaced)), uintptr(unsafe.Pointer(replacement)), 0, 0, 0, 0); r == 0 {
		t.Errorf("ReplaceFile while floe reads the file = %v", err)
	}
	if b, err := os.ReadFile(page); err != nil || string(b) != "saved" {
		t.Errorf("the file after the save = %q, %v; want saved", b, err)
	}
	if got := readAll(t, f); got != "old" {
		t.Errorf("floe reads %q after the save, want old", got)
	}
}
