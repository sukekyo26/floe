package cli_test

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
)

const good = "lang: en\ntemplate: report\ntitle: A plan\nsections:\n  - heading: One\n    parts:\n      - text: Hello.\n"

// write writes files, by their names, in a new directory, and returns it.
func write(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, text := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// floe check tells every mistake of every file, and nothing for files that are right.
func TestCheck(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{"a.yaml": good, "b.yaml": "lang: en\ntemplate: reprot\n", "c.yaml": "x: [\n", "page.html": ""})
	if code, stdout, stderr := run(dir, "check", "a.yaml", "./a.yaml"); code != 0 || stdout != "" || stderr != "" {
		t.Errorf("floe check a.yaml = %d, %q, %q", code, stdout, stderr)
	}
	code, _, stderr := run(dir, "check", "b.yaml", "a.yaml", "page.html", "", "gone.yaml", "c.yaml")
	want := []string{
		`b.yaml:2:11: unknown template "reprot"; use report`,
		"floe: page.html is not a data file (.yaml or .yml)",
		"floe: argument 4 is an empty path; give a data file (.yaml or .yml), and check that a variable written for it is set",
		"gone.yaml: no such file or directory",
	}
	if code != 1 || !strings.HasPrefix(stderr, strings.Join(want, "\n")+"\nc.yaml:") {
		t.Errorf("floe check = %d, stderr\n%s\nwant first\n%s", code, stderr, strings.Join(want, "\n"))
	}
	if code, _, stderr := run("", "check", "a.yaml"); code != 1 || !strings.Contains(stderr, "the relative path a.yaml means nothing") {
		t.Errorf("floe check under no directory = %d, %q", code, stderr)
	}
}

// floe render writes each page that changes and prints its path; --check tells those missing or not drawn.
func TestRender(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{"a.yaml": good, "sub/a.yaml": good})
	if code, stdout, stderr := run(dir, "render", "--check", "a.yaml"); code != 1 ||
		stderr != "floe: a.html is missing: write it from a.yaml with floe render\n" || stdout != "" {
		t.Errorf("floe render --check before = %d, %q, %q", code, stdout, stderr)
	}
	if code, stdout, stderr := run(dir, "render", "a.yaml"); code != 0 || stdout != "a.html\n" || stderr != "" {
		t.Errorf("floe render = %d, %q, %q", code, stdout, stderr)
	}
	page, err := os.ReadFile(filepath.Join(dir, "a.html"))
	if err != nil || !strings.Contains(string(page), `<meta http-equiv="Content-Security-Policy"`) ||
		!strings.Contains(string(page), "<title>A plan</title>") {
		t.Errorf("a.html = %q, %v", page, err)
	}
	if code, stdout, _ := run(dir, "render", "a.yaml"); code != 0 || stdout != "" {
		t.Errorf("floe render again = %d, %q, want nothing written", code, stdout)
	}
	if code, _, stderr := run(dir, "render", "--check", "a.yaml"); code != 0 || stderr != "" {
		t.Errorf("floe render --check = %d, %q", code, stderr)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.html"), []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := run(dir, "render", "--check", "a.yaml"); code != 1 ||
		stderr != "floe: a.html is not what a.yaml draws: write it again with floe render\n" {
		t.Errorf("floe render --check stale = %d, %q", code, stderr)
	}
	out := filepath.Join("pages", "x")
	if code, _, stderr := run(dir, "render", "--out-dir", out, "a.yaml", "sub/a.yaml"); code != 1 ||
		stderr != "floe: a.yaml and sub/a.yaml would both write "+filepath.Join(out, "a.html")+"; rename one of them\n" {
		t.Errorf("floe render of one page twice = %d, %q", code, stderr)
	}
	if code, stdout, _ := run(dir, "render", "--out-dir", out, "sub/a.yaml"); code != 0 || stdout != filepath.Join(out, "a.html")+"\n" {
		t.Errorf("floe render --out-dir = %d, %q", code, stdout)
	}
	if code, _, stderr := run(dir, "render", "--out-dir", "", "a.yaml"); code != 1 ||
		stderr != "floe: --out-dir is an empty path; give the directory to write the pages in, and check that a variable written for it is set\n" {
		t.Errorf("floe render --out-dir \"\" = %d, %q", code, stderr)
	}
}

// A page floe cannot write is told at it, that it cannot be written and what to look at, then in the system's
// words: a directory of its name, a directory it cannot write in, and a name too long. A name as long as the
// directory takes is written: the file written whole beside it has a short name of its own.
func TestRenderCannotWrite(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("x", 251) // .yml is a name the directory takes, .html one too long
	fits := strings.Repeat("y", 250)
	dir := write(t, map[string]string{"b.yaml": good, long + ".yml": good, fits + ".yaml": good, "b.html/x": "", "ro/c.yaml": good})
	if err := os.Chmod(filepath.Join(dir, "ro"), 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "ro"), 0o700) }) //nolint:errcheck // for TempDir to remove it
	if code, stdout, stderr := run(dir, "render", fits+".yaml"); code != 0 || stdout != fits+".html\n" {
		t.Errorf("floe render of a name that fits = %d, %q, %q", code, stdout, stderr)
	}
	// The system's words differ from one system to another: what is told before them is floe's.
	cases := []struct{ file, page string }{{"b.yaml", "b.html"}, {long + ".yml", long + ".html"}}
	// root writes in a directory it may not.
	if os.Geteuid() != 0 {
		cases = append(cases, struct{ file, page string }{"ro/c.yaml", filepath.Join("ro", "c.html")})
	}
	for _, tt := range cases {
		code, _, stderr := run(dir, "render", tt.file)
		if code != 1 || !strings.HasPrefix(stderr, tt.page+": cannot be written (") ||
			!strings.HasSuffix(stderr, "): check that its directory exists and can be written to, and that its name is free and not too long\n") ||
			strings.Contains(stderr, dir) || strings.Count(stderr, "\n") != 1 {
			t.Errorf("floe render %.20s = %d, stderr %q", tt.file, code, stderr)
		}
	}
}

// A page that is a symlink is written where it leads; one that leads nowhere is refused as a page that cannot
// be written, and the link stays.
func TestRenderThroughLink(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{"a.yaml": good, "b.yaml": good, "out/a.html": "old"})
	for name, to := range map[string]string{"a.html": filepath.Join("out", "a.html"), "b.html": filepath.Join("out", "b.html")} {
		if err := os.Symlink(to, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	code, stdout, stderr := run(dir, "render", "a.yaml", "b.yaml")
	if code != 1 || stdout != "" || !strings.HasPrefix(stderr, "b.html: cannot be written (") {
		t.Errorf("floe render = %d, %q, %q", code, stdout, stderr)
	}
	if code, stdout, stderr := run(dir, "render", "a.yaml"); code != 0 || stdout != "a.html\n" || stderr != "" {
		t.Errorf("floe render a.yaml = %d, %q, %q", code, stdout, stderr)
	}
	for _, name := range []string{"a.html", "b.html"} {
		if info, err := os.Lstat(filepath.Join(dir, name)); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s is no longer the link: %v, %v", name, info, err)
		}
	}
	if page, err := os.ReadFile(filepath.Join(dir, "out", "a.html")); err != nil || !strings.Contains(string(page), "<title>A plan</title>") {
		t.Errorf("out/a.html = %.40q, %v", page, err)
	}
}

// Every page that can be written is written, and every one that cannot is told.
func TestRenderWritesTheOthers(t *testing.T) {
	t.Parallel()

	// root writes in a directory it may not.
	if os.Geteuid() == 0 {
		t.Skip("root writes in a directory it may not")
	}
	dir := write(t, map[string]string{"a.yaml": good, "ro/b.yaml": good, "ro/c.yaml": good})
	if err := os.Chmod(filepath.Join(dir, "ro"), 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "ro"), 0o700) }) //nolint:errcheck // for TempDir to remove it
	code, stdout, stderr := run(dir, "render", "a.yaml", "ro/b.yaml", "ro/c.yaml")
	if code != 1 || stdout != "a.html\n" || strings.Count(stderr, "cannot be written") != 2 ||
		!strings.HasPrefix(stderr, "ro/b.html: ") || !strings.Contains(stderr, "\nro/c.html: ") {
		t.Errorf("floe render = %d, %q, %q", code, stdout, stderr)
	}
}

// A .. after a symlink leads where the system says: from where the link leads, not back from the link's name.
func TestDotDotAfterLink(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{"x.yaml": "lang: en\ntemplate: reprot\n", "real/x.yaml": good, "real/sub/y": ""})
	if err := os.Symlink(filepath.Join("real", "sub"), filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	file := "link/../x.yaml" // as written: filepath.Join would fold it
	if code, _, stderr := run(dir, "check", file); code != 0 || stderr != "" {
		t.Errorf("floe check %s = %d, %q: it read the x.yaml beside link, not real/x.yaml", file, code, stderr)
	}
	if code, stdout, stderr := run(dir, "render", file); code != 0 || stdout != "link/../x.html\n" || stderr != "" {
		t.Errorf("floe render %s = %d, %q, %q", file, code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "real", "x.html")); err != nil {
		t.Errorf("real/x.html: %v", err)
	}
}

// Two pages are one only when the system says so: on Linux a.html and A.html are two files, both written.
func TestRenderCaseIsTheSystems(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "linux" {
		t.Skip("a.html and A.html are two files on Linux, one on macOS")
	}
	dir := write(t, map[string]string{"a.yaml": good, "A.yaml": good})
	if code, stdout, stderr := run(dir, "render", "a.yaml", "A.yaml"); code != 0 || stdout != "a.html\nA.html\n" || stderr != "" {
		t.Errorf("floe render = %d, %q, %q", code, stdout, stderr)
	}
}

// A data file of no name but its extension is refused: its page would be a hidden .html titled by nothing.
func TestDataFileWithoutName(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{".yaml": good})
	for _, cmd := range []string{"check", "render"} {
		code, stdout, stderr := run(dir, cmd, ".yaml")
		if code != 1 || stdout != "" ||
			stderr != "floe: .yaml has no name before its extension, which names its page: name it, like plan.yaml\n" {
			t.Errorf("floe %s .yaml = %d, %q, %q", cmd, code, stdout, stderr)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".html")); err == nil {
		t.Error(".html was written")
	}
}

// A file refused by its name writes no page, which another would be told to clash with.
func TestRefusedNamesDoNotClash(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{".yaml": good, ".yml": good, "a.txt": "", "a.md": ""})
	for _, args := range [][]string{{".yaml", ".yml"}, {"a.txt", "a.md"}} {
		code, _, stderr := run(dir, append([]string{"render"}, args...)...)
		if code != 1 || strings.Contains(stderr, "would both write") || strings.Count(stderr, "\n") != 2 {
			t.Errorf("floe render %v = %d, %q", args, code, stderr)
		}
	}
}

// --check tells what render would: a page, or --out-dir, that is a link to no file or a loop of links, or a
// directory where a file goes, or a file where a directory goes, cannot be written, in the system's words.
func TestCannotWriteThroughLinks(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{"a.yaml": good, "b.yaml": good, "c.yaml": good, "c.html/x": "", "e.yaml": good, "file": ""})
	for link, to := range map[string]string{"a.html": filepath.Join("out", "a.html"), "b.html": "b.html", "nowhere": "missing", "loop": "loop"} {
		if err := os.Symlink(to, filepath.Join(dir, link)); err != nil {
			t.Fatal(err)
		}
	}
	tail := "): check that its directory exists and can be written to, and that its name is free and not too long\n"
	for _, tt := range []struct {
		args []string
		want string
	}{
		{[]string{"a.yaml"}, "a.html: cannot be written (" + syscall.ENOENT.Error() + tail},
		{[]string{"b.yaml"}, "b.html: cannot be written (" + syscall.ELOOP.Error() + tail},
		{[]string{"c.yaml"}, "c.html: cannot be written (" + syscall.EISDIR.Error() + tail},
		{[]string{"--out-dir", "nowhere", "e.yaml"}, "nowhere: cannot be written (" + syscall.ENOENT.Error() + tail},
		{[]string{"--out-dir", "loop", "e.yaml"}, "loop: cannot be written (" + syscall.ELOOP.Error() + tail},
		{[]string{"--out-dir", "file", "e.yaml"}, "file: cannot be written (" + syscall.ENOTDIR.Error() + tail},
	} {
		for _, mode := range [][]string{{"render"}, {"render", "--check"}} {
			args := slices.Concat(mode, tt.args)
			if code, _, stderr := run(dir, args...); code != 1 || stderr != tt.want {
				t.Errorf("floe %v = %d, %q, want %q", args, code, stderr, tt.want)
			}
		}
	}
	for _, link := range []string{"a.html", "b.html", "nowhere", "loop"} {
		if info, err := os.Lstat(filepath.Join(dir, link)); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s is no longer the link: %v, %v", link, info, err)
		}
	}
}

// An --out-dir that cannot be written to is told with the mistakes of the data files.
func TestOutDirToldWithTheFiles(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{"e.yaml": "lang: en\ntemplate: reprot\n"})
	if err := os.Symlink("missing", filepath.Join(dir, "nowhere")); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := run(dir, "render", "--out-dir", "nowhere", "e.yaml")
	if code != 1 || !strings.HasPrefix(stderr, "nowhere: cannot be written (") || !strings.Contains(stderr, `e.yaml:2:11: unknown template "reprot"`) {
		t.Errorf("floe render = %d, %q", code, stderr)
	}
}
