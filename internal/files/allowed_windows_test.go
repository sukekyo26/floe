package files_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"floe/internal/files"
	"floe/internal/testfs"
	"floe/internal/usererr"
)

// entry is one of files' ways to the system on a path, run on path; it returns what it told.
type entry struct {
	name string
	run  func(t *testing.T, path string) error
}

// entries are every one of files' functions that call the system on a path (TestEveryEntryAllowed keeps them all).
func entries() []entry {
	closeFile := func(f *os.File, err error) error {
		if f != nil {
			_ = f.Close()
		}
		return err
	}
	return []entry{
		{"Open", func(_ *testing.T, p string) error { f, _, err := files.Open(p); return closeFile(f, err) }},
		{"OpenAppend", func(_ *testing.T, p string) error { f, _, err := files.OpenAppend(p); return closeFile(f, err) }},
		{"Read", func(_ *testing.T, p string) error { _, _, err := files.Read(p); return err }},
		{"Real", func(_ *testing.T, p string) error { _, err := files.Real(p); return err }},
		{"RealAbove", func(_ *testing.T, p string) error { files.RealAbove(p); return nil }},
		{"LongName", func(_ *testing.T, p string) error { _, err := files.LongName(p); return err }},
		{"Mark", func(_ *testing.T, p string) error { files.Mark(p); return nil }},
		{"Stat", func(_ *testing.T, p string) error { _, err := files.Stat(p); return err }},
		{"Lstat", func(_ *testing.T, p string) error { _, err := files.Lstat(p); return err }},
		{"ReadDir", func(_ *testing.T, p string) error { _, err := files.ReadDir(p); return err }},
		{"OpenRoot", func(_ *testing.T, p string) error {
			r, err := files.OpenRoot(p)
			if r != nil {
				_ = r.Close()
			}
			return err
		}},
		{"OpenLock", func(_ *testing.T, p string) error { return closeFile(files.OpenLock(p)) }},
		{"MkdirAll", func(_ *testing.T, p string) error { return files.MkdirAll(p, 0o700) }},
		{"Mkdir", func(_ *testing.T, p string) error { return files.Mkdir(p, 0o700) }},
		{"WriteAtomic", func(_ *testing.T, p string) error { return files.WriteAtomic(p, []byte("w"), 0o600, nil) }},
		{"Remove", func(_ *testing.T, p string) error { return files.Remove(p) }},
		{"RemoveAll", func(_ *testing.T, p string) error { return files.RemoveAll(p) }},
		{"Rename", func(_ *testing.T, p string) error { return files.Rename(p, "a", "b") }},
		{"MkdirTemp", func(_ *testing.T, p string) error { _, err := files.MkdirTemp(p, "t-"); return err }},
		{"NewRoot", func(t *testing.T, p string) error {
			t.Helper()
			r := files.NewRoot(filepath.Dir(p))
			t.Cleanup(func() { _ = r.Close() })
			_, err := fs.ReadFile(r, filepath.Base(p))
			return err
		}},
		{"DirFS", func(_ *testing.T, p string) error {
			_, err := fs.ReadFile(files.DirFS(filepath.Dir(p)), filepath.Base(p))
			return err
		}},
	}
}

// pipeForms are the paths that reach the pipe server named srv (\\.\pipe\…) by name or by a link, made under a
// directory of their own: P1 to P7 of the design (floe-s315-p4).
func pipeForms(t *testing.T, srv string) map[string]string {
	t.Helper()
	dir := t.TempDir()
	v := `\\?\` + dir
	rest := srv[len(`\\.\pipe\`):]
	testfs.Symlink(t, srv, filepath.Join(dir, "p.html"))
	testfs.Symlink(t, `\\.\pipe\`, filepath.Join(dir, "pd"))
	testfs.Symlink(t, srv, filepath.Join(dir, "pf"))
	// Named as Win32 would not leave them: made through \\?\.
	testfs.Symlink(t, `\\.\pipe\`, v+`\link `)
	testfs.Symlink(t, srv, v+`\p.html.`)
	return map[string]string{
		"P1 the pipe":                   srv,
		"P2 \\\\?\\pipe":                `\\?\pipe\` + rest,
		"P2 \\??\\pipe":                 `\??\pipe\` + rest,
		"P2 //./pipe":                   `//./pipe/` + rest,
		"P3 a link to the pipe":         filepath.Join(dir, "p.html"),
		"P4 a name under a link":        filepath.Join(dir, "pd", rest),
		"P5 a link ending in \\":        filepath.Join(dir, "pf") + `\`,
		"P6 a name under link-space":    dir + `\link \` + rest,
		"P7 \\\\?\\ a link ending in .": v + `\p.html.`,
	}
}

// No entry connects to a pipe, by its name or by a link to it, whatever it tells: the server sees no client.
func TestAllowedEntries(t *testing.T) {
	t.Parallel()

	srv, connected := testfs.PipeServer(t)
	for _, e := range entries() {
		for form, path := range pipeForms(t, srv) {
			before := connected()
			err := e.run(t, path)
			if got := connected() - before; got != 0 {
				t.Errorf("%s of %s (%s) connected to the pipe %d times; told %v", e.name, form, path, got, err)
			}
			if err != nil && strings.Contains(err.Error(), "a link to nothing") {
				t.Errorf("%s of %s told files' own error: %v", e.name, form, err)
			}
		}
	}
}

// wantNotLocal checks Read refuses path as not local, unopened.
func wantNotLocal(t *testing.T, path string) {
	t.Helper()
	if _, _, err := files.Read(path); !errors.Is(err, usererr.ErrNotLocal) {
		t.Errorf("Read(%s) = %v, want ErrNotLocal", path, err)
	}
}

// wantRead checks Read reads path.
func wantRead(t *testing.T, path, want string) {
	t.Helper()
	if b, _, err := files.Read(path); err != nil || string(b) != want {
		t.Errorf("Read(%s) = %q, %v, want %q", path, b, err, want)
	}
}

// writeFile writes text to path, a test's own.
func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// A device by its name, by a reserved name in a path or by a link, is refused unopened; a name reserved without an
// extension alone (Windows 11) is an ordinary file's (D1 to D6).
func TestAllowedDevice(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for _, p := range []string{
		`\\.\NUL`, filepath.Join(dir, "NUL"), filepath.Join(dir, "NUL."), filepath.Join(dir, "NUL "), `\\.\CON`,
		filepath.Join(dir, "CON"), filepath.Join(dir, "CONIN$"), filepath.Join(dir, "COM1"),
		filepath.Join(dir, "NUL", "x"), `\\?\GLOBALROOT\Device\Null`, `\\.\GLOBALROOT\Device\Null`,
		`\\?\` + filepath.Join(dir, "NUL"),
	} {
		wantNotLocal(t, p)
	}
	n := filepath.Join(dir, "n.html")
	testfs.Symlink(t, `\\.\NUL`, n)
	wantNotLocal(t, n)
	// Windows 11 reads these as ordinary names; Windows 10 as devices, where this fails.
	for _, name := range []string{"nul.txt", "COM1.html", "NUL .txt"} {
		writeFile(t, filepath.Join(dir, name), name)
		wantRead(t, filepath.Join(dir, name), name)
	}
}

// A share, as written, through a network drive, a subst of it, or a link to it, is refused unopened; a subst of a
// local directory is read (S1 to S6).
func TestAllowedShare(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.html"), "a")
	shared, mapped := testfs.MapShare(t, dir)
	for _, p := range []string{
		filepath.Join(shared, "a.html"), `\\?\UNC\` + filepath.Join(shared, "a.html")[2:],
		`\??\UNC\` + filepath.Join(shared, "a.html")[2:], filepath.ToSlash(filepath.Join(shared, "a.html")),
		filepath.Join(mapped, "a.html"), testfs.SubstShare(t, dir) + `\a.html`,
	} {
		wantNotLocal(t, p)
	}
	wantRead(t, testfs.Subst(t, dir)+`\a.html`, "a")
	other := t.TempDir()
	testfs.Symlink(t, shared, filepath.Join(other, "s"))
	testfs.Symlink(t, filepath.Join(shared, "a.html"), filepath.Join(other, "f.html"))
	testfs.Symlink(t, filepath.Join(mapped, "a.html"), filepath.Join(other, "m.html"))
	for _, p := range []string{filepath.Join(other, "s", "a.html"), filepath.Join(other, "f.html"), filepath.Join(other, "m.html")} {
		wantNotLocal(t, p)
	}
	if _, err := fs.ReadFile(files.DirFS(other), "s/a.html"); !errors.Is(err, usererr.ErrNotLocal) {
		t.Errorf("DirFS of a link to a share = %v, want ErrNotLocal", err)
	}
	// MkdirAll follows its last name (os.MkdirAll stats it first): a directory to make that is a link to a share.
	if err := files.MkdirAll(filepath.Join(other, "s"), 0o700); !errors.Is(err, usererr.ErrNotLocal) {
		t.Errorf("MkdirAll of a link to a share = %v, want ErrNotLocal", err)
	}
	// RealAbove does not climb above what allowed refuses.
	for _, p := range []string{filepath.Join(shared, "a.html", "x"), filepath.Join(other, "s", "x")} {
		if got, ok := files.RealAbove(p); ok {
			t.Errorf("RealAbove(%s) = %q, want false", p, got)
		}
	}
}

// Volumes of no drive letter, \\?\ with a /, and the device of a drive are refused; \\?\X:\…, its root, a lower case
// letter, / for \ and a long path are read, a file named a. made through \\?\ by its letters (V1 to V6).
func TestAllowedVolume(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.html"), "a")
	guid := testfs.VolumeGUID(t, dir)
	j := filepath.Join(t.TempDir(), "j")
	testfs.Junction(t, j, guid)
	for _, p := range []string{
		guid + `\a.html`, filepath.Join(j, "a.html"), `\\.\` + filepath.Join(dir, "a.html"), `1:\a.html`,
		"//?/" + filepath.ToSlash(filepath.Join(dir, "a.html")), `\\?/` + filepath.Join(dir, "a.html"),
		`\\?\` + filepath.VolumeName(dir), `\??\` + filepath.Join(dir, "a.html"), `\??\pipe\x`,
	} {
		wantNotLocal(t, p)
	}
	long := dir
	for len(long) < 300 {
		long = filepath.Join(long, strings.Repeat("d", 40))
	}
	writeFile(t, filepath.Join(long, "a.html"), "long")
	if err := os.WriteFile(`\\?\`+filepath.Join(dir, "b."), []byte("b."), 0o600); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]string{
		`\\?\` + filepath.Join(dir, "a.html"): "a", strings.ToLower(dir[:1]) + dir[1:] + `\a.html`: "a",
		filepath.ToSlash(filepath.Join(dir, "a.html")): "a", filepath.Join(long, "a.html"): "long",
		`\\?\` + filepath.Join(dir, "b."): "b.",
	} {
		wantRead(t, p, want)
	}
	if _, err := files.Stat(`\\?\` + filepath.VolumeName(dir) + `\`); err != nil {
		t.Errorf("Stat of a drive's root = %v", err)
	}
	if _, err := files.Stat(`\\?\` + dir + `\x\..\a.html`); !errors.Is(err, fs.ErrInvalid) {
		t.Errorf("Stat of \\\\?\\ with .. = %v, want fs.ErrInvalid", err)
	}
	if _, err := files.Stat(""); err == nil || errors.Is(err, usererr.ErrNotLocal) {
		t.Errorf("Stat(\"\") = %v, want the system's error", err)
	}
}

// Links to local files and directories are followed (absolute, relative, a junction in, out or to a root, a chain of
// 63); a 64th link, a root-relative target, one above the root, one to a device or a stream are refused, a target
// ending in a dot is read by its letters (L1 to L10).
func TestAllowedLinks(t *testing.T) {
	t.Parallel()

	dir, other := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(dir, "a.html"), "a")
	writeFile(t, filepath.Join(dir, "in", "b.html"), "b")
	writeFile(t, filepath.Join(other, "c.html"), "c")
	testfs.Symlink(t, filepath.Join(dir, "a.html"), filepath.Join(dir, "abs.html"))
	testfs.Symlink(t, "a.html", filepath.Join(dir, "rel.html"))
	testfs.Symlink(t, `..\`+filepath.Base(dir)+`\a.html`, filepath.Join(dir, "up.html"))
	testfs.Junction(t, filepath.Join(dir, "jin"), filepath.Join(dir, "in"))
	testfs.Junction(t, filepath.Join(dir, "jout"), other)
	testfs.Symlink(t, "abs.html", filepath.Join(dir, "chain.html"))
	if err := os.WriteFile(`\\?\`+filepath.Join(dir, "evil."), []byte("evil."), 0o600); err != nil {
		t.Fatal(err)
	}
	// Through \\?\: os.Symlink makes a target as Win32 reads it, its trailing dot dropped, otherwise.
	testfs.Symlink(t, `\\?\`+filepath.Join(dir, "evil."), filepath.Join(dir, "e.html"))
	for p, want := range map[string]string{
		"abs.html": "a", "rel.html": "a", "up.html": "a", `jin\b.html`: "b", `jout\c.html`: "c", "chain.html": "a",
		"e.html": "evil.",
	} {
		wantRead(t, filepath.Join(dir, p), want)
	}
	// A name ending in a separator is followed, as os.Lstat follows it: the junction's directory, not the junction.
	if fi, err := files.Lstat(filepath.Join(dir, "jout") + `\`); err != nil || !fi.IsDir() {
		t.Errorf("Lstat(jout\\) = %v, %v, want the directory it leads to", fi, err)
	}
	if fi, err := files.Lstat(filepath.Join(dir, "jout")); err != nil || fi.IsDir() {
		t.Errorf("Lstat(jout) = %v, %v, want the junction itself", fi, err)
	}
	root := filepath.Join(t.TempDir(), "root")
	testfs.Junction(t, root, filepath.VolumeName(dir)+`\`)
	if _, err := files.ReadDir(root); err != nil {
		t.Errorf("ReadDir of a junction to a drive's root = %v", err)
	}
	chain := func(n int) string {
		d := t.TempDir()
		writeFile(t, filepath.Join(d, "l0"), "x")
		for i := 1; i <= n; i++ {
			testfs.Symlink(t, fmt.Sprintf("l%d", i-1), filepath.Join(d, fmt.Sprintf("l%d", i)))
		}
		return filepath.Join(d, fmt.Sprintf("l%d", n))
	}
	wantRead(t, chain(63), "x")
	if _, _, err := files.Read(chain(64)); !errors.Is(err, syscall.ELOOP) {
		t.Errorf("Read of a chain of 64 links = %v, want ELOOP", err)
	}
	refused := t.TempDir()
	for name, to := range map[string]string{
		"rootrel.html": filepath.Join(dir, "a.html")[len("C:"):], "above.html": strings.Repeat(`..\`, 40) + "a.html",
		"nul.html": "NUL", "stream.html": "a.html:x",
	} {
		testfs.Symlink(t, to, filepath.Join(refused, name))
		wantNotLocal(t, filepath.Join(refused, name))
	}
}

// A link to nothing: what makes is refused (ELOOP), nothing made where it leads; what reads tells it is not there;
// files' own error never leaves it. Under a link to a directory that is there, what is not there yet is made
// (L11 to L14, L13b).
func TestAllowedDangling(t *testing.T) {
	t.Parallel()

	making := map[string]bool{"OpenAppend": true, "OpenLock": true, "MkdirAll": true, "Mkdir": true, "MkdirTemp": true}
	free := testfs.FreeDrive(t)
	for _, e := range entries() {
		dir := t.TempDir()
		nowhere := filepath.Join(t.TempDir(), "gone")
		testfs.Symlink(t, filepath.Join(nowhere, "x"), filepath.Join(dir, "log"))
		testfs.Symlink(t, nowhere, filepath.Join(dir, "d"))
		testfs.Symlink(t, free+`\`, filepath.Join(dir, "q"))
		// A drive not there is not there, told without opening it (R3), as a path and as a link's target.
		if err := e.run(t, free+`\x`); err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("%s of %s = %v, want it not there", e.name, free+`\x`, err)
		}
		for _, p := range []string{filepath.Join(dir, "log"), filepath.Join(dir, "d", "x"), filepath.Join(dir, "q", "x")} {
			err := e.run(t, p)
			if err != nil && strings.Contains(err.Error(), "a link to nothing") {
				t.Errorf("%s of %s told files' own error: %v", e.name, p, err)
			}
			// Mkdir does not follow its last name: the link log is there to make no directory over.
			lastOfMkdir := e.name == "Mkdir" && strings.HasSuffix(p, "log")
			if making[e.name] && !lastOfMkdir && !errors.Is(err, syscall.ELOOP) {
				t.Errorf("%s of %s = %v, want ELOOP", e.name, p, err)
			}
			if _, err := os.Lstat(nowhere); err == nil {
				t.Errorf("%s of %s made where the link leads", e.name, p)
			}
		}
	}
	if m := files.Mark(filepath.Join(t.TempDir(), "x")); m != "-" {
		t.Errorf("Mark of none = %q", m)
	}
	// Under a link to a directory that is there, a name not there yet is the path's own: made.
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "real"), 0o700); err != nil {
		t.Fatal(err)
	}
	testfs.Symlink(t, filepath.Join(dir, "real"), filepath.Join(dir, "link"))
	if err := files.MkdirAll(filepath.Join(dir, "link", "new", "deeper"), 0o700); err != nil {
		t.Errorf("MkdirAll under a link = %v", err)
	}
	f, _, err := files.OpenAppend(filepath.Join(dir, "link", "a.log"))
	if err != nil {
		t.Errorf("OpenAppend under a link = %v", err)
	} else {
		_ = f.Close()
	}
	l, err := files.OpenLock(filepath.Join(dir, "link", ".lock"))
	if err != nil {
		t.Errorf("OpenLock under a link = %v", err)
	} else {
		_ = l.Close()
	}
	// RealAbove follows the path's own link that leads to nothing to where it leads: a → r there, b → q\c, c not there.
	writeFile(t, filepath.Join(dir, "q", "keep"), "")
	testfs.Symlink(t, filepath.Join(dir, "real"), filepath.Join(dir, "a"))
	testfs.Symlink(t, filepath.Join(dir, "q", "c"), filepath.Join(dir, "real", "b"))
	q, err := files.Real(filepath.Join(dir, "q"))
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := files.RealAbove(filepath.Join(dir, "a", "b", "x")); !ok || got != filepath.Join(q, "c", "x") {
		t.Errorf("RealAbove(a\\b\\x) = %q, %v, want %q", got, ok, filepath.Join(q, "c", "x"))
	}
}

// What is opened through a link is named by the path allowed followed to, as the system names it (\\?\…).
func TestOpenName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.html"), "a")
	testfs.Symlink(t, filepath.Join(dir, "a.html"), filepath.Join(dir, "l.html"))
	want := `\\?\` + filepath.Join(dir, "a.html")
	f, _, err := files.Open(filepath.Join(dir, "l.html"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, _, err := files.OpenAppend(filepath.Join(dir, "l.html"))
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	for _, got := range []string{f.Name(), g.Name()} {
		if !strings.EqualFold(got, want) {
			t.Errorf("opened through a link as %s, want %s", got, want)
		}
	}
}
