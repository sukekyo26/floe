package link_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"floe/internal/dirs"
	"floe/internal/files"
	"floe/internal/link"
	"floe/internal/testfs"
	"floe/internal/usererr"
)

// realTemp returns a temporary directory by its real path, as files.Real names it: on macOS t.TempDir is under the
// /var symlink, and on Windows a link's paths are \\?\C:\….
func realTemp(tb testing.TB) string {
	tb.Helper()

	dir, err := files.Real(tb.TempDir())
	if err != nil {
		tb.Fatal(err)
	}
	return dir
}

func write(tb testing.TB, path, body string) {
	tb.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		tb.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		tb.Fatal(err)
	}
}

func symlink(tb testing.TB, target, name string) {
	tb.Helper()

	testfs.Symlink(tb, target, name)
}

func mkfifo(tb testing.TB, path string) {
	tb.Helper()

	testfs.Mkfifo(tb, path)
}

// site builds a linked directory with dot paths, an escaping symlink and a name that one more decoding would turn
// into another file, and returns the registered file.
func site(tb testing.TB) string {
	tb.Helper()

	tmp := realTemp(tb)
	dir := filepath.Join(tmp, "site")
	for path, body := range map[string]string{
		"index.html":      "<h1>Linked</h1>",
		"s.css":           "p{}",
		"sub/p.html":      "<title>Sub</title>",
		".git/config":     "secret",
		"sub/.git/config": "secret",
		"a%41.html":       "encoded",
		"aA.html":         "decoded twice",
	} {
		write(tb, filepath.Join(dir, path), body)
	}
	write(tb, filepath.Join(tmp, "outside.txt"), "outside")
	symlink(tb, filepath.Join(tmp, "outside.txt"), filepath.Join(dir, "escape.txt"))
	symlink(tb, filepath.Join(dir, "s.css"), filepath.Join(dir, "inside.css"))
	return filepath.Join(dir, "index.html")
}

func TestResolve(t *testing.T) {
	t.Parallel()

	registered := site(t)
	dir := filepath.Dir(registered)
	tests := []struct {
		rest string
		want string // "" when refused
	}{
		{"", "index.html"},
		{"s.css", "s.css"},
		{"sub/p.html", "sub/p.html"},
		{"inside.css", "s.css"},
		{"a%41.html", "a%41.html"},
		{".git/config", ""},
		{"sub/.git/config", ""},
		{"../outside.txt", ""},
		{"sub/../s.css", ""},
		{"./s.css", ""},
		{"%2e%2e/outside.txt", ""},
		{"escape.txt", ""},
		{"sub", ""},
		{"sub/", ""},
		{"sub//p.html", ""},
		{"nope.css", ""},
	}
	for _, tt := range tests {
		got, ok := link.Resolve(registered, false, tt.rest)
		want := ""
		if tt.want != "" {
			want = filepath.Join(dir, tt.want)
		}
		if got != want || ok != (want != "") {
			t.Errorf("Resolve(%q) = %q, %v; want %q", tt.rest, got, ok, want)
		}
	}
	t.Run("a FIFO", func(t *testing.T) {
		t.Parallel()
		mkfifo(t, filepath.Join(dir, "pipe.txt"))
		if got, ok := link.Resolve(registered, false, "pipe.txt"); got != "" || ok {
			t.Errorf("Resolve(pipe.txt) = %q, %v, a FIFO served", got, ok)
		}
	})
}

func TestResolveFolder(t *testing.T) {
	t.Parallel()

	registered := site(t)
	folder := filepath.Join(filepath.Dir(registered), "sub")
	write(t, filepath.Join(filepath.Dir(registered), "sibling.html"), "beside the folder")
	tests := []struct {
		rest string
		want string // under the site, "" when refused
	}{
		{"", ""},
		{"p.html", "sub/p.html"},
		{"../sibling.html", ""},
		{"../index.html", ""},
		{".git/config", ""},
		{"p.html/", ""},
	}
	for _, tt := range tests {
		got, ok := link.Resolve(folder, true, tt.rest)
		want := ""
		if tt.want != "" {
			want = filepath.Join(filepath.Dir(registered), tt.want)
		}
		if got != want || ok != (want != "") {
			t.Errorf("Resolve(folder, %q) = %q, %v; want %q", tt.rest, got, ok, want)
		}
	}
}

// TestDir lists one directory of a folder at a time, as Resolve would serve it.
func TestDir(t *testing.T) { //nolint:tparallel // the subtests change the folder in turn, before it is moved
	t.Parallel()

	tmp := realTemp(t)
	folder := filepath.Join(tmp, "docs")
	for path, body := range map[string]string{
		"index.html":        "top",
		"README.HTM":        "upper case",
		"notes.yaml":        "not a page",
		"style.css":         "not a page",
		".drafts/a.html":    "hidden",
		".x.html":           "hidden",
		"guide/setup.html":  "in a directory",
		"guide/deep/x.html": "deeper",
		"assets/logo.svg":   "no page",
	} {
		write(t, filepath.Join(folder, path), body)
	}
	write(t, filepath.Join(tmp, "secret.html"), "outside")
	symlink(t, filepath.Join(tmp, "secret.html"), filepath.Join(folder, "out.html"))
	symlink(t, filepath.Join(folder, "index.html"), filepath.Join(folder, "in.html"))
	symlink(t, filepath.Join(folder, "guide"), filepath.Join(folder, "linked-dir"))

	got, err := link.Dir(folder, "")
	if err != nil {
		t.Fatal(err)
	}
	files := func(l link.Listing) []string {
		out := make([]string, 0, len(l.Files))
		for _, p := range l.Files {
			out = append(out, p.File)
		}
		return out
	}
	if want := []string{"assets", "guide"}; !slices.Equal(got.Dirs, want) || got.More != 0 {
		t.Errorf("Dir(top).Dirs = %q, more %d; want %q", got.Dirs, got.More, want)
	}
	if want := []string{"README.HTM", "in.html", "index.html"}; !slices.Equal(files(got), want) {
		t.Errorf("Dir(top).Files = %q, want %q", files(got), want)
	}
	if got.Files[0].Modified.IsZero() {
		t.Error("a page's time is not given")
	}
	if l, err := link.Dir(folder, "guide"); err != nil || !slices.Equal(files(l), []string{"guide/setup.html"}) {
		t.Errorf("Dir(guide) = %q, %v", files(l), err)
	}
	for _, dir := range []string{"..", ".drafts", "guide/../..", "nope", "index.html", "guide/", "/guide"} {
		if _, err := link.Dir(folder, dir); !errors.Is(err, link.ErrNoDir) {
			t.Errorf("Dir(%q) = %v, want ErrNoDir", dir, err)
		}
	}
	t.Run("an odd name", func(t *testing.T) { //nolint:paralleltest // in the folder, before it is moved below
		testfs.UnixOnly(t, "a name with : (Windows takes none)")
		write(t, filepath.Join(folder, "a,b:c", "odd.html"), "an odd name")
		if l, err := link.Dir(folder, ""); err != nil || !slices.Equal(l.Dirs, []string{"a,b:c", "assets", "guide"}) {
			t.Errorf("Dir(top).Dirs = %q, %v", l.Dirs, err)
		}
		if l, err := link.Dir(folder, "a,b:c"); err != nil || !slices.Equal(files(l), []string{"a,b:c/odd.html"}) {
			t.Errorf("Dir(a,b:c) = %q, %v", files(l), err)
		}
	})
	t.Run("a FIFO", func(t *testing.T) { //nolint:paralleltest // as above
		mkfifo(t, filepath.Join(folder, "pipe.html"))
		if l, err := link.Dir(folder, ""); err != nil || !slices.Equal(files(l), []string{"README.HTM", "in.html", "index.html"}) {
			t.Errorf("Dir(top).Files = %q, %v", files(l), err)
		}
	})
	if err := os.Rename(folder, folder+"-moved"); err != nil {
		t.Fatal(err)
	}
	symlink(t, folder+"-moved", folder)
	if _, err := link.Dir(folder, ""); !errors.Is(err, link.ErrNoDir) {
		t.Errorf("Dir of a folder that became a symlink = %v, want ErrNoDir", err)
	}
}

func TestDirKeepsTheFirstByName(t *testing.T) {
	t.Parallel()

	folder := filepath.Join(realTemp(t), "big")
	for i := range link.MaxListed + 4 {
		write(t, filepath.Join(folder, fmt.Sprintf("p%04d.html", i)), "")
	}
	for _, d := range []string{"a", "b"} {
		if err := os.Mkdir(filepath.Join(folder, d), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	l, err := link.Dir(folder, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Dirs) != 2 || len(l.Files) != link.MaxListed-2 || l.More != 6 || l.Files[0].File != "p0000.html" {
		t.Errorf("Dir = %d dirs, %d pages (first %q), more %d; want 2, %d, p0000.html, 6",
			len(l.Dirs), len(l.Files), l.Files[0].File, l.More, link.MaxListed-2)
	}
}

func TestResolveRefusesARegisteredPathThatIsNoLongerReal(t *testing.T) {
	t.Parallel()

	registered := site(t)
	if err := os.Remove(registered); err != nil {
		t.Fatal(err)
	}
	if _, ok := link.Resolve(registered, false, ""); ok {
		t.Error("a registered file that is gone is served")
	}
	symlink(t, filepath.Join(filepath.Dir(registered), "s.css"), registered)
	for _, rest := range []string{"", "s.css"} {
		if got, ok := link.Resolve(registered, false, rest); ok {
			t.Errorf("Resolve(%q) = %q after the registered file became a symlink", rest, got)
		}
	}
}

// FuzzResolve checks the boundary for any rest: what is served is a regular file under the directory,
// reached through no dot element.
func FuzzResolve(f *testing.F) {
	registered := site(f)
	dir := filepath.Dir(registered)
	// A FIFO where there are FIFOs; the rest is fuzzed on Windows too.
	if err := testfs.MakeFIFO(filepath.Join(dir, "pipe.txt")); errors.Is(err, testfs.ErrNoFIFO) {
		f.Log("no FIFO here; the rest is fuzzed")
	} else if err != nil {
		f.Fatal(err)
	}
	for _, seed := range []string{
		"", "s.css", "sub/p.html", "../outside.txt", "sub/../../outside.txt", ".git/config",
		"sub/.git/config", "escape.txt", "pipe.txt", "a%41.html", "sub/", "/etc/passwd", "s.css\x00",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, rest string) {
		got, ok := link.Resolve(registered, false, rest)
		if !ok {
			return
		}
		rel, err := filepath.Rel(dir, got)
		if err != nil || strings.HasPrefix(rel, "..") {
			t.Fatalf("Resolve(%q) = %q, outside %s", rest, got, dir)
		}
		for p := range strings.SplitSeq(rest, "/") {
			if p == "" && rest != "" || strings.HasPrefix(p, ".") {
				t.Fatalf("Resolve(%q) = %q through an empty or dot element", rest, got)
			}
		}
		if fi, err := os.Stat(got); err != nil || !fi.Mode().IsRegular() {
			t.Fatalf("Resolve(%q) = %q, not a regular file", rest, got)
		}
	})
}

func TestCheck(t *testing.T) {
	t.Parallel()

	tmp := realTemp(t)
	realHome := filepath.Join(tmp, "realhome")
	home := filepath.Join(tmp, "home")
	for _, path := range []string{"top.html", ".top.html", ".secret/x.html", "proj/.agents/doc.html", "proj/page.txt", "proj/ok.HTM", "proj/spec.YML"} {
		write(t, filepath.Join(realHome, path), "<p>x</p>")
	}
	for _, dir := range []string{"proj/dir.html", ".secret/sub"} {
		if err := os.MkdirAll(filepath.Join(realHome, dir), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	// $HOME is a symlink: the real paths under it must still meet the home checks.
	symlink(t, realHome, home)
	// A path through a symlink is refused, whatever it points at.
	symlink(t, filepath.Join(realHome, ".secret"), filepath.Join(realHome, "proj", "alias"))
	// So is one with .., which the string checks would misread too. Written as a user writes them (no \\?\ on
	// Windows, where the system reads no .. after it), with the system's separator.
	sep, written := string(filepath.Separator), dirs.Shown(realHome)
	for _, p := range []string{
		written + sep + "proj" + sep + "alias" + sep + "x.html", written + sep + "proj" + sep + ".." + sep + "top.html",
		".secret/x.html", "top.html",
	} {
		if _, err := link.Check(p, home); err == nil || !strings.Contains(err.Error(), "is not its real path") {
			t.Errorf("Check(%s) = %v, want it refused as not real", p, err)
		}
	}
	// A home that cannot be resolved leaves the real one unknown, so nothing passes.
	for _, h := range []string{"", filepath.Join(tmp, "no-such-home"), filepath.Join(realHome, "top.html")} {
		if _, err := link.Check(filepath.Join(realHome, "proj", "ok.HTM"), h); err == nil ||
			!strings.Contains(err.Error(), "cannot resolve the home directory") {
			t.Errorf("Check with home %q = %v, want it refused", h, err)
		}
	}

	// What home is under would serve every home's files: refused for a folder, and for a file in it.
	write(t, filepath.Join(tmp, "beside.html"), "<p>x</p>")
	for _, p := range []string{tmp, filepath.Join(tmp, "beside.html")} {
		if _, err := link.Check(p, home); err == nil || !strings.Contains(err.Error(), "would expose all of "+dirs.Shown(tmp)) {
			t.Errorf("Check(%s) = %v, want it refused as exposing %s", p, err, tmp)
		}
	}
	tests := []struct {
		path string
		key  string // the mistake's key, "" when allowed
		part string // a part of its message that matters, if any
	}{
		{"proj/.agents/doc.html", "", ""},
		{"proj/ok.HTM", "", ""},
		{"proj/spec.YML", "", ""},
		{".secret/x.html", "link_hidden_home", ""},
		{"top.html", "link_exposes_dir", "would expose all of " + dirs.Shown(realHome)},
		{".top.html", "link_exposes_dir", "would expose all of " + dirs.Shown(realHome)},
		{"proj/page.txt", "link_not_linkable", ""},
		// Real cannot tell where a file not there is (publishing a link, before, tells it cannot be read).
		{"proj/missing.html", "link_not_real", ""},
		// A folder is checked as the directory it exposes: its own.
		{"proj", "", ""},
		{"proj/dir.html", "", ""},
		{"proj/.agents", "", ""},
		{"", "link_exposes_dir", "would expose all of " + dirs.Shown(realHome)},
		{".secret", "link_hidden_home", ""},
		{".secret/sub", "link_hidden_home", ""},
	}
	for _, tt := range tests {
		_, err := link.Check(filepath.Clean(filepath.Join(realHome, tt.path)), home)
		if tt.key == "" {
			if err != nil {
				t.Errorf("Check(%s) = %v, want it allowed", tt.path, err)
			}
			continue
		}
		var ue *usererr.Error
		if !errors.As(err, &ue) || ue.Key != tt.key || !strings.Contains(ue.Error(), tt.part) {
			t.Errorf("Check(%s) = %v, want %s containing %q", tt.path, err, tt.key, tt.part)
		}
	}
	t.Run("a FIFO", func(t *testing.T) {
		t.Parallel()
		pipe := filepath.Join(realHome, "proj", "pipe.html")
		mkfifo(t, pipe)
		want := "is not a regular HTML file (.html, .htm), data file (.yaml, .yml) or folder"
		_, err := link.Check(pipe, home)
		if ue := (*usererr.Error)(nil); !errors.As(err, &ue) || !strings.Contains(ue.Error(), want) {
			t.Errorf("Check(proj/pipe.html) = %v, want a usererr containing %q", err, want)
		}
	})
}

// A folder whose own path has a dot directory (one under a project's .agents) serves its files: the dot rule reads
// the names under the folder alone.
func TestResolveDotBase(t *testing.T) {
	t.Parallel()

	folder := filepath.Join(realTemp(t), "proj", ".agents", "docs")
	write(t, filepath.Join(folder, "a.html"), "a")
	write(t, filepath.Join(folder, "sub", "b.html"), "b")
	for _, rest := range []string{"a.html", "sub/b.html"} {
		if got, ok := link.Resolve(folder, true, rest); !ok || got != filepath.Join(folder, filepath.FromSlash(rest)) {
			t.Errorf("Resolve(%q) = %q, %v, want it served", rest, got, ok)
		}
	}
	if _, err := link.Dir(folder, "sub"); err != nil {
		t.Errorf("Dir(sub) = %v", err)
	}
}

// A link under a folder to a dot file of it is served as the file it leads to: the dot rule reads the names as
// written, not where a link leads (as node_modules/x -> .pnpm/x).
func TestResolveLinkToDot(t *testing.T) {
	t.Parallel()

	folder := filepath.Join(realTemp(t), "f")
	env := filepath.Join(folder, ".env")
	write(t, env, "e")
	symlink(t, env, filepath.Join(folder, "x"))
	if got, ok := link.Resolve(folder, true, "x"); !ok || got != env {
		t.Errorf("Resolve(x) = %q, %v, want %q", got, ok, env)
	}
}

// A name that is not UTF-8 is not served nor listed by its bytes: the URL cannot name it. (A folder's listing gives it
// with U+FFFD, as encoding/json writes it; Windows names it so on disk too.)
func TestResolveInvalidUTF8(t *testing.T) {
	t.Parallel()

	folder := realTemp(t)
	testfs.NotUTF8(t, folder, "a\xff.html", false)
	testfs.NotUTF8(t, folder, "d\xff", true)
	if got, ok := link.Resolve(folder, true, "a\xff.html"); ok {
		t.Errorf("Resolve(a\\xff.html) = %q, want it refused", got)
	}
	if l, err := link.Dir(folder, "d\xff"); !errors.Is(err, link.ErrNoDir) {
		t.Errorf("Dir(d\\xff) = %+v, %v, want ErrNoDir", l, err)
	}
}
