package cli_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

const notFile = ": this is neither a regular file nor a directory (a FIFO, a device or the like), which floe may never finish reading: put a regular file here\n"

// A page that is a link to what is no .html file, a data file among them, is refused, and nothing is written:
// not the file it leads to, nor the other pages. One to an .html file, in any case, and a hard link are written.
func TestRenderLinksToNoPage(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{
		"a.yaml": good, "b.yaml": good, "c.yaml": good, "d.yaml": good, "e.yaml": good, "x.txt": "secret",
	})
	if err := os.Mkdir(filepath.Join(dir, "somedir"), 0o755); err != nil {
		t.Fatal(err)
	}
	for page, to := range map[string]string{"a.html": "a.yaml", "b.html": "c.yaml", "d.html": "x.txt", "e.html": "/dev/zero"} {
		link(t, to, filepath.Join(dir, page))
	}
	link(t, "somedir", filepath.Join(dir, "s.html"))
	refused := func(page, to string) string {
		return page + ": this page is a link to " + to + ", which is no .html file: floe writes pages to .html files alone; point the link at one, or remove it\n"
	}
	for _, tt := range []struct {
		file, want string
	}{
		{"a.yaml", refused("a.html", filepath.Join(dir, "a.yaml"))},
		{"b.yaml", refused("b.html", filepath.Join(dir, "c.yaml"))},
		{"d.yaml", refused("d.html", filepath.Join(dir, "x.txt"))},
		{"e.yaml", refused("e.html", "/dev/zero")},
	} {
		for _, args := range [][]string{{"render", tt.file}, {"render", "--check", tt.file}, {"render", tt.file, "c.yaml"}} {
			if code, stdout, stderr := run(dir, args...); code != 1 || stdout != "" || stderr != tt.want {
				t.Errorf("floe %v = %d, %q, %q; want 1, %q", args, code, stdout, stderr, tt.want)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "s.yaml"), []byte(good), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := run(dir, "render", "s.yaml"); code != 1 || stderr != refused("s.html", filepath.Join(dir, "somedir")) {
		t.Errorf("a link to a directory = %d, %q", code, stderr)
	}
	for name, want := range map[string]string{"a.yaml": good, "c.yaml": good, "x.txt": "secret"} {
		if got, err := os.ReadFile(filepath.Join(dir, name)); err != nil || string(got) != want {
			t.Errorf("%s is now %.30q, %v", name, got, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "c.html")); err == nil {
		t.Error("wrote c.html beside a page refused")
	}

	ok := write(t, map[string]string{"l.yaml": good, "n.yaml": good, "out/L.HTML": "old"})
	link(t, filepath.Join("out", "L.HTML"), filepath.Join(ok, "l.html"))
	if err := os.Link(filepath.Join(ok, "n.yaml"), filepath.Join(ok, "n.html")); err != nil {
		t.Fatal(err)
	}
	if code, stdout, stderr := run(ok, "render", "l.yaml", "n.yaml"); code != 0 || stdout != "l.html\nn.html\n" || stderr != "" {
		t.Errorf("floe render l.yaml n.yaml = %d, %q, %q", code, stdout, stderr)
	}
	if got, err := os.ReadFile(filepath.Join(ok, "n.yaml")); err != nil || string(got) != good {
		t.Errorf("n.yaml, hard linked to its page, is now %.30q, %v", got, err)
	}
}

// What is neither a regular file nor a directory, a FIFO, is refused unopened wherever floe reads a path it is
// given: a data file, a page and an icon set's zip. Nothing is written beside a FIFO page.
func TestFIFOsRefused(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{"a.yaml": good, "b.yaml": good})
	for _, name := range []string{"a.html", "f.yaml", "z.zip"} {
		if err := syscall.Mkfifo(filepath.Join(dir, name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	env := map[string]string{"FLOE_ICONS_DIR": filepath.Join(dir, "icons")}
	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{"render", "a.yaml", "b.yaml"}, "a.html" + notFile},
		{[]string{"render", "--check", "a.yaml"}, "a.html" + notFile},
		{[]string{"check", "f.yaml"}, "f.yaml" + notFile},
		{[]string{"icons", "fetch", "aws", "z.zip"}, "z.zip" + notFile},
	} {
		if code, stdout, stderr := runEnv(dir, env, tt.args...); code != 1 || stdout != "" || stderr != tt.want {
			t.Errorf("floe %v = %d, %q, %q; want 1, %q", tt.args, code, stdout, stderr, tt.want)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "b.html")); err == nil {
		t.Error("wrote b.html beside a FIFO page")
	}
}

// A page written again keeps its permissions, one that cannot be read too.
func TestRenderKeepsPermissions(t *testing.T) {
	t.Parallel()

	for _, mode := range []os.FileMode{0o600, 0o664, 0o200} {
		dir := write(t, map[string]string{"a.yaml": good, "a.html": "old"})
		page := filepath.Join(dir, "a.html")
		if err := os.Chmod(page, mode); err != nil {
			t.Fatal(err)
		}
		if code, _, stderr := run(dir, "render", "a.yaml"); code != 0 {
			t.Fatalf("floe render = %d, %q", code, stderr)
		}
		if info, err := os.Stat(page); err != nil || info.Mode().Perm() != mode {
			t.Errorf("a page of mode %v is now %v, %v", mode, info.Mode(), err)
		}
	}
}
