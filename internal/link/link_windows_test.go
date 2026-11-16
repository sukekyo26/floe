package link_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"

	"floe/internal/files"
	"floe/internal/link"
	"floe/internal/testfs"
)

// shortBase is the 8.3 name of path's last name, or skips t where the volume makes none.
func shortBase(tb testing.TB, path string) string {
	tb.Helper()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		tb.Fatal(err)
	}
	buf := make([]uint16, windows.MAX_PATH)
	n, err := windows.GetShortPathName(p, &buf[0], windows.MAX_PATH)
	if err != nil {
		tb.Fatal(err)
	}
	short := filepath.Base(windows.UTF16ToString(buf[:n]))
	if short == filepath.Base(path) {
		tb.Skip("this volume makes no short names")
	}
	return short
}

// realOf is files.Real of path: the folder a link registers; the tests make the files under it by the plain path,
// which cmd's mklink takes.
func realOf(tb testing.TB, path string) string {
	tb.Helper()
	got, err := files.Real(path)
	if err != nil {
		tb.Fatal(err)
	}
	return got
}

// A junction under a folder to a directory outside it is neither listed nor served (S315's W20), one to a directory
// inside it is, and a nested file is served under the folder's \\?\ path.
func TestJunctionOut(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	plain := filepath.Join(tmp, "f")
	write(t, filepath.Join(plain, "sub", "a.html"), "a")
	write(t, filepath.Join(tmp, "outside", "x.html"), "x")
	testfs.Junction(t, filepath.Join(plain, "j"), filepath.Join(tmp, "outside"))
	testfs.Junction(t, filepath.Join(plain, "in"), filepath.Join(plain, "sub"))
	folder := realOf(t, plain)
	if l, err := link.Dir(folder, "j"); !errors.Is(err, link.ErrNoDir) {
		t.Errorf("Dir(j) = %+v, %v, want ErrNoDir: j leads out of the folder", l, err)
	}
	if got, ok := link.Resolve(folder, true, "j/x.html"); ok {
		t.Errorf("Resolve(j/x.html) = %q, want it refused", got)
	}
	for rest, want := range map[string]string{"in/a.html": filepath.Join(folder, "sub", "a.html"), "sub/a.html": filepath.Join(folder, "sub", "a.html")} {
		if got, ok := link.Resolve(folder, true, rest); !ok || got != want {
			t.Errorf("Resolve(%q) = %q, %v, want %q", rest, got, ok, want)
		}
	}
	if _, err := link.Dir(folder, "in"); err != nil {
		t.Errorf("Dir(in) = %v, want it listed", err)
	}
}

// A path through a junction is not its real path: Check refuses it, a file under it and the junction itself alike.
func TestCheckJunction(t *testing.T) {
	t.Parallel()

	plain := t.TempDir()
	write(t, filepath.Join(plain, "target", "a.html"), "a")
	write(t, filepath.Join(plain, "proj", "b.html"), "b")
	testfs.Junction(t, filepath.Join(plain, "proj", "j"), filepath.Join(plain, "target"))
	home := realOf(t, plain)
	j := filepath.Join(home, "proj", "j")
	for _, p := range []string{j, filepath.Join(j, "a.html")} {
		if _, err := link.Check(p, home); err == nil || !strings.Contains(err.Error(), "is not its real path") {
			t.Errorf("Check(%s) = %v, want it refused as not real", p, err)
		}
	}
}

// A short name under a folder names its long one for the dot rule: SECRET~1 is .secret, refused; a junction is not
// followed for it, so one to a dot directory under the folder is served, as a symlink is on Unix.
func TestResolveShortName(t *testing.T) {
	t.Parallel()

	plain := filepath.Join(t.TempDir(), "f")
	write(t, filepath.Join(plain, ".secret", "id.html"), "s")
	write(t, filepath.Join(plain, "sub", ".git", "config"), "c")
	write(t, filepath.Join(plain, ".git", "config"), "c")
	write(t, filepath.Join(plain, ".pnpm", "pkg", "a.js"), "a")
	testfs.Junction(t, filepath.Join(plain, "j"), filepath.Join(plain, ".git"))
	testfs.Junction(t, filepath.Join(plain, "nm"), filepath.Join(plain, ".pnpm", "pkg"))
	folder := realOf(t, plain)
	for _, rest := range []string{"j/config", "nm/a.js"} {
		if got, ok := link.Resolve(folder, true, rest); !ok {
			t.Errorf("Resolve(%q) = %q, want it served", rest, got)
		}
	}
	secret := shortBase(t, filepath.Join(plain, ".secret"))
	git := shortBase(t, filepath.Join(plain, "sub", ".git"))
	for _, rest := range []string{secret + "/id.html", "sub/" + git + "/config"} {
		if got, ok := link.Resolve(folder, true, rest); ok {
			t.Errorf("Resolve(%q) = %q, want it refused as a dot name", rest, got)
		}
	}
	if _, err := link.Dir(folder, secret); !errors.Is(err, link.ErrNoDir) {
		t.Errorf("Dir(%s) = %v, want ErrNoDir", secret, err)
	}
}

// A trailing dot names no file under a folder's \\?\ path: Windows drops it only from a path without \\?\.
func TestResolveTrailingDot(t *testing.T) {
	t.Parallel()

	folder := filepath.Join(realTemp(t), "f")
	write(t, filepath.Join(folder, "a.html"), "a")
	if got, ok := link.Resolve(folder, true, "a.html."); ok {
		t.Errorf("Resolve(a.html.) = %q, want it refused", got)
	}
	if _, ok := link.Resolve(folder, true, "a.html"); !ok {
		t.Error("Resolve(a.html) refused")
	}
}
