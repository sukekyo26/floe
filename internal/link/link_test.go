package link_test

import (
	"errors"
	"os"
	"path/filepath"
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
		got, ok := link.Resolve(registered, tt.rest)
		want := ""
		if tt.want != "" {
			want = filepath.Join(dir, tt.want)
		}
		if got != want || ok != (want != "") {
			t.Errorf("Resolve(%q) = %q, %v; want %q", tt.rest, got, ok, want)
		}
	}
}

func TestResolveRefusesARegisteredPathThatIsNoLongerReal(t *testing.T) {
	t.Parallel()

	registered := site(t)
	if err := os.Remove(registered); err != nil {
		t.Fatal(err)
	}
	if _, ok := link.Resolve(registered, ""); ok {
		t.Error("a registered file that is gone is served")
	}
	symlink(t, filepath.Join(filepath.Dir(registered), "s.css"), registered)
	for _, rest := range []string{"", "s.css"} {
		if got, ok := link.Resolve(registered, rest); ok {
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
		got, ok := link.Resolve(registered, rest)
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
	for _, path := range []string{"top.html", ".top.html", ".secret/x.html", "proj/.agents/doc.html", "proj/page.txt", "proj/ok.HTM"} {
		write(t, filepath.Join(realHome, path), "<p>x</p>")
	}
	if err := os.Mkdir(filepath.Join(realHome, "proj", "dir.html"), 0o750); err != nil {
		t.Fatal(err)
	}
	mkfifo(t, filepath.Join(realHome, "proj", "pipe.html"))
	// $HOME is a symlink: the real paths under it must still meet the home checks.
	symlink(t, realHome, home)

	tests := []struct {
		path string
		want string // a part of the message, "" when allowed
	}{
		{"proj/.agents/doc.html", ""},
		{"proj/ok.HTM", ""},
		{".secret/x.html", "is under a hidden directory of your home"},
		{"top.html", "would expose all of " + realHome},
		{".top.html", "would expose all of " + realHome},
		{"proj/page.txt", "is not an .html file"},
		{"proj/dir.html", "is not an .html file"},
		{"proj/pipe.html", "is not an .html file"},
		{"proj/missing.html", "is not an .html file"},
	}
	for _, tt := range tests {
		err := link.Check(filepath.Join(realHome, tt.path), home)
		if tt.want == "" {
			if err != nil {
				t.Errorf("Check(%s) = %v, want it allowed", tt.path, err)
			}
			continue
		}
		var ue *usererr.Error
		if !errors.As(err, &ue) || !strings.Contains(ue.Msg, tt.want) || ue.Ja == "" {
			t.Errorf("Check(%s) = %v, want a usererr containing %q", tt.path, err, tt.want)
		}
	}
}
