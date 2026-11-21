package link_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"floe/internal/files"
	"floe/internal/link"
	"floe/internal/testfs"
	"floe/internal/usererr"
)

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
	secret := filepath.Base(testfs.ShortName(t, filepath.Join(plain, ".secret")))
	git := filepath.Base(testfs.ShortName(t, filepath.Join(plain, "sub", ".git")))
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
	// A trailing space alike: none names a.html; made under \\?\, a file named so is its own, and served.
	if got, ok := link.Resolve(folder, true, "a.html "); ok {
		t.Errorf("Resolve(a.html ) = %q, want it refused while there is no such file", got)
	}
	write(t, filepath.Join(folder, "a.html "), "s")
	if got, ok := link.Resolve(folder, true, "a.html "); !ok || filepath.Base(got) != "a.html " {
		t.Errorf("Resolve(a.html ) = %q, %v, want the file named so", got, ok)
	}
}

// Names Windows reads otherwise are not served: a stream after :, a reserved name (made as a plain file under \\?\),
// \ as a separator, a drive, and bytes that are not UTF-8 (Windows makes such a name with U+FFFD). The short name
// of a dot directory with a stream after it lists nothing (S315's W26). Reserved names with an extension are plain
// names on this Windows (build 26100), and served.
func TestResolveLocalize(t *testing.T) {
	t.Parallel()

	folder := filepath.Join(realTemp(t), "f")
	for _, name := range []string{"a.html", "a.html:x", "NUL", "CON", `sub\a.html`, "NUL.html", "nul.txt", `.secret\id.html`} {
		write(t, filepath.Join(folder, name), "x")
	}
	testfs.NotUTF8(t, folder, "a\xff.html", false)
	for _, rest := range []string{"a.html:x", "a.html::$DATA", "NUL", "nul", "CON", `sub\a.html`, "C:x", "a\xff.html"} {
		if got, ok := link.Resolve(folder, true, rest); ok {
			t.Errorf("Resolve(%q) = %q, want it refused", rest, got)
		}
	}
	for _, rest := range []string{"a.html", "sub/a.html", "NUL.html", "nul.txt", "a\uFFFD.html"} {
		if got, ok := link.Resolve(folder, true, rest); !ok {
			t.Errorf("Resolve(%q) = %q, want it served", rest, got)
		}
	}
	short := filepath.Base(testfs.ShortName(t, filepath.Join(folder, ".secret")))
	if l, err := link.Dir(folder, short+"::$INDEX_ALLOCATION"); !errors.Is(err, link.ErrNoDir) {
		t.Errorf("Dir(%s::$INDEX_ALLOCATION) = %+v, %v, want ErrNoDir", short, l, err)
	}
}

// mistakeKey is the key of the mistake err is, "" for none.
func mistakeKey(err error) string {
	if ue, ok := errors.AsType[*usererr.Error](err); ok {
		return ue.Key
	}
	return ""
}

// A drive's root is no folder to link: all of the drive would be served.
func TestCheckRoot(t *testing.T) {
	t.Parallel()

	home := filepath.Join(realTemp(t), "home")
	if err := os.Mkdir(home, 0o700); err != nil {
		t.Fatal(err)
	}
	root := realOf(t, os.Getenv("SystemDrive")+`\`)
	if _, err := link.Check(root, home); mistakeKey(err) != "link_exposes_dir" {
		t.Errorf("Check(%s) = %v, want link_exposes_dir", root, err)
	}
}

// A directory right under home with the hidden attribute is hidden, as a dot one is; one deeper, in a project, is not.
func TestCheckHiddenAttr(t *testing.T) {
	t.Parallel()

	home := filepath.Join(realTemp(t), "home")
	notes, hid := filepath.Join(home, "notes"), filepath.Join(home, "proj", "hid")
	write(t, filepath.Join(notes, "a.html"), "a")
	write(t, filepath.Join(hid, "a.html"), "a")
	testfs.Hide(t, notes)
	testfs.Hide(t, hid)
	if _, err := link.Check(filepath.Join(notes, "a.html"), home); mistakeKey(err) != "link_hidden_home" {
		t.Errorf("Check(notes/a.html) = %v, want link_hidden_home", err)
	}
	if _, err := link.Check(filepath.Join(hid, "a.html"), home); err != nil {
		t.Errorf("Check(proj/hid/a.html) = %v, want it allowed", err)
	}
}
