package link_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/sukekyo26/floe/internal/link"
	"github.com/sukekyo26/floe/internal/usererr"
)

// realTemp returns a temporary directory by its real path; on macOS t.TempDir is under the /var symlink.
func realTemp(tb testing.TB) string {
	tb.Helper()

	dir, err := filepath.EvalSymlinks(tb.TempDir())
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

	if err := os.Symlink(target, name); err != nil {
		tb.Fatal(err)
	}
}

func mkfifo(tb testing.TB, path string) {
	tb.Helper()

	if err := syscall.Mkfifo(path, 0o600); err != nil {
		tb.Fatal(err)
	}
}

// site builds a linked directory with dot paths, an escaping symlink, a FIFO and a name that one more
// decoding would turn into another file, and returns the registered file.
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
	mkfifo(tb, filepath.Join(dir, "pipe.txt"))
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
		{"pipe.txt", ""},
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
func TestDir(t *testing.T) {
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
		"a,b:c/odd.html":    "an odd name",
		"assets/logo.svg":   "no page",
	} {
		write(t, filepath.Join(folder, path), body)
	}
	write(t, filepath.Join(tmp, "secret.html"), "outside")
	symlink(t, filepath.Join(tmp, "secret.html"), filepath.Join(folder, "out.html"))
	symlink(t, filepath.Join(folder, "index.html"), filepath.Join(folder, "in.html"))
	symlink(t, filepath.Join(folder, "guide"), filepath.Join(folder, "linked-dir"))
	mkfifo(t, filepath.Join(folder, "pipe.html"))

	got, err := link.Dir(folder, "")
	if err != nil {
		t.Fatal(err)
	}
	files := func(l link.Listing) []string {
		out := make([]string, 0, len(l.Pages))
		for _, p := range l.Pages {
			out = append(out, p.File)
		}
		return out
	}
	if want := []string{"a,b:c", "assets", "guide"}; !slices.Equal(got.Dirs, want) || got.More != 0 {
		t.Errorf("Dir(top).Dirs = %q, more %d; want %q", got.Dirs, got.More, want)
	}
	if want := []string{"README.HTM", "in.html", "index.html"}; !slices.Equal(files(got), want) {
		t.Errorf("Dir(top).Pages = %q, want %q", files(got), want)
	}
	if got.Pages[0].Modified.IsZero() {
		t.Error("a page's time is not given")
	}
	for dir, want := range map[string][]string{"guide": {"guide/setup.html"}, "a,b:c": {"a,b:c/odd.html"}} {
		l, err := link.Dir(folder, dir)
		if err != nil || !slices.Equal(files(l), want) {
			t.Errorf("Dir(%q) = %q, %v; want %q", dir, files(l), err, want)
		}
	}
	for _, dir := range []string{"..", ".drafts", "guide/../..", "nope", "index.html", "guide/", "/guide"} {
		if _, err := link.Dir(folder, dir); !errors.Is(err, link.ErrNoDir) {
			t.Errorf("Dir(%q) = %v, want ErrNoDir", dir, err)
		}
	}
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
	if len(l.Dirs) != 2 || len(l.Pages) != link.MaxListed-2 || l.More != 6 || l.Pages[0].File != "p0000.html" {
		t.Errorf("Dir = %d dirs, %d pages (first %q), more %d; want 2, %d, p0000.html, 6",
			len(l.Dirs), len(l.Pages), l.Pages[0].File, l.More, link.MaxListed-2)
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
		for _, p := range strings.Split(rest, "/") {
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
	mkfifo(t, filepath.Join(realHome, "proj", "pipe.html"))
	// $HOME is a symlink: the real paths under it must still meet the home checks.
	symlink(t, realHome, home)
	// A path through a symlink is refused, whatever it points at.
	symlink(t, filepath.Join(realHome, ".secret"), filepath.Join(realHome, "proj", "alias"))
	// So is one with .., which the string checks would misread too.
	for _, p := range []string{realHome + "/proj/alias/x.html", realHome + "/proj/../top.html", ".secret/x.html", "top.html"} {
		if _, err := link.Check(p, home); err == nil || !strings.Contains(err.Error(), "is not its absolute real path") {
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
		if _, err := link.Check(p, home); err == nil || !strings.Contains(err.Error(), "would expose all of "+tmp) {
			t.Errorf("Check(%s) = %v, want it refused as exposing %s", p, err, tmp)
		}
	}
	tests := []struct {
		path string
		want string // a part of the message, "" when allowed
	}{
		{"proj/.agents/doc.html", ""},
		{"proj/ok.HTM", ""},
		{"proj/spec.YML", ""},
		{".secret/x.html", "is under a hidden directory of your home"},
		{"top.html", "would expose all of " + realHome},
		{".top.html", "would expose all of " + realHome},
		{"proj/page.txt", "is not a regular HTML file (.html, .htm), data file (.yaml, .yml) or folder"},
		{"proj/pipe.html", "is not a regular HTML file (.html, .htm), data file (.yaml, .yml) or folder"},
		{"proj/missing.html", "is not a regular HTML file (.html, .htm), data file (.yaml, .yml) or folder"},
		// A folder is checked as the directory it exposes: its own.
		{"proj", ""},
		{"proj/dir.html", ""},
		{"proj/.agents", ""},
		{"", "would expose all of " + realHome},
		{".secret", "is under a hidden directory of your home"},
		{".secret/sub", "is under a hidden directory of your home"},
	}
	for _, tt := range tests {
		_, err := link.Check(filepath.Clean(filepath.Join(realHome, tt.path)), home)
		if tt.want == "" {
			if err != nil {
				t.Errorf("Check(%s) = %v, want it allowed", tt.path, err)
			}
			continue
		}
		var ue *usererr.Error
		if !errors.As(err, &ue) || !strings.Contains(ue.Error(), tt.want) {
			t.Errorf("Check(%s) = %v, want a usererr containing %q", tt.path, err, tt.want)
		}
	}
}
