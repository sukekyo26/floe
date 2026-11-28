package cli_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"floe/internal/files"
	"floe/internal/testfs"
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

	dir := write(t, map[string]string{"a.yaml": good, "b.yaml": "lang: en\ntemplate: reprot\n", "c.yaml": "x: [\n", "page.html": "", "d.yaml": "a: |\n   one\n  two\n"})
	if code, stdout, stderr := run(dir, "check", "a.yaml", "./a.yaml"); code != 0 || stdout != "" || stderr != "" {
		t.Errorf("floe check a.yaml = %d, %q, %q", code, stdout, stderr)
	}
	t.Run("every mistake", func(t *testing.T) {
		t.Parallel()
		_, _, gone := files.Read(filepath.Join(dir, "gone.yaml"))
		code, _, stderr := run(dir, "check", "b.yaml", "a.yaml", "page.html", "", "gone.yaml", "c.yaml")
		want := []string{
			`b.yaml:2:11: unknown template "reprot"; use adr, handoff, postmortem, prd, report, runbook, slides, tech-spec`,
			"floe: page.html is not a data file (.yaml or .yml)",
			"floe: argument 4 is an empty path; give a data file (.yaml or .yml), and check that a variable written for it is set",
			"gone.yaml: " + testfs.Words(t, gone),
		}
		if code != 1 || !strings.HasPrefix(stderr, strings.Join(want, "\n")+"\nc.yaml:") {
			t.Errorf("floe check = %d, stderr\n%s\nwant first\n%s", code, stderr, strings.Join(want, "\n"))
		}
	})
	// A line less deep than the first of a | text: the library's words at its place, then how to mend the text.
	if code, _, stderr := run(dir, "check", "d.yaml"); code != 1 || !strings.HasPrefix(stderr, "d.yaml:3:3: did not find expected key: the | or > text above ends before this line") {
		t.Errorf("floe check d.yaml = %d, %q", code, stderr)
	}
	if code, _, stderr := run("", "check", "a.yaml"); code != 1 || !strings.Contains(stderr, "the relative path a.yaml means nothing") {
		t.Errorf("floe check under no directory = %d, %q", code, stderr)
	}
}

// floe check and render tell the warnings of the pages drawn, once each and after the mistakes of the others, and
// end well without a mistake.
func TestWarnings(t *testing.T) {
	t.Parallel()

	fenced := "lang: en\ntemplate: report\nsections:\n  - parts:\n      - text: \"```\\ncode\\n```\"\n"
	warning := "w.yaml:5:15: warning: this text holds a fenced block of code on its line 1, drawn as plain code: write code in a code part and commands in a terminal part, which show them under their file or prompt with a copy button\n"
	dir := write(t, map[string]string{"w.yaml": fenced, "b.yaml": "lang: en\ntemplate: reprot\n"})
	for _, tt := range []struct {
		args []string
		code int
		errs string
	}{
		{[]string{"check", "w.yaml", "./w.yaml"}, 0, warning},
		{[]string{"check", "b.yaml", "w.yaml"}, 1, `b.yaml:2:11: unknown template "reprot"; use adr, handoff, postmortem, prd, report, runbook, slides, tech-spec` + "\n" + warning},
		{[]string{"render", "w.yaml"}, 0, warning},
	} {
		if code, _, errs := run(dir, tt.args...); code != tt.code || errs != tt.errs {
			t.Errorf("floe %v = %d, %q; want %d, %q", tt.args, code, errs, tt.code, tt.errs)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "w.html")); err != nil {
		t.Errorf("floe render wrote no page: %v", err)
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
	t.Run("out-dir", func(t *testing.T) {
		t.Parallel()
		out := filepath.Join("pages", "x")
		if code, _, stderr := run(dir, "render", "--out-dir", out, "a.yaml", "sub/a.yaml"); code != 1 ||
			stderr != "floe: a.yaml and sub/a.yaml would both write "+filepath.Join(out, "a.html")+"; rename one of them\n" {
			t.Errorf("floe render of one page twice = %d, %q", code, stderr)
		}
		if code, stdout, _ := run(dir, "render", "--out-dir", out, "sub/a.yaml"); code != 0 || stdout != filepath.Join(out, "a.html")+"\n" {
			t.Errorf("floe render --out-dir = %d, %q", code, stdout)
		}
	})
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
	if code, stdout, stderr := run(dir, "render", fits+".yaml"); code != 0 || stdout != fits+".html\n" {
		t.Errorf("floe render of a name that fits = %d, %q, %q", code, stdout, stderr)
	}
	// The system's words differ from one system to another: what is told before them is floe's.
	cannotWrite := func(t *testing.T, file, page string) {
		t.Helper()
		code, _, stderr := run(dir, "render", file)
		if code != 1 || !strings.HasPrefix(stderr, page+": cannot be written (") ||
			!strings.HasSuffix(stderr, "): check that its directory exists and can be written to, and that its name is free and not too long\n") ||
			strings.Contains(stderr, dir) || strings.Count(stderr, "\n") != 1 {
			t.Errorf("floe render %.20s = %d, stderr %q", file, code, stderr)
		}
	}
	cannotWrite(t, "b.yaml", "b.html")
	cannotWrite(t, long+".yml", long+".html")
	t.Run("a directory it cannot write in", func(t *testing.T) {
		testfs.UnixOnly(t, "chmod keeps a directory from being written")
		t.Parallel()
		// root writes in a directory it may not.
		if os.Geteuid() == 0 {
			t.Skip("root writes in a directory it may not")
		}
		if err := os.Chmod(filepath.Join(dir, "ro"), 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "ro"), 0o700) }) //nolint:errcheck // for TempDir to remove it
		cannotWrite(t, "ro/c.yaml", filepath.Join("ro", "c.html"))
	})
}

// A page that is a symlink is written where it leads; one that leads nowhere is refused as a page that cannot
// be written, and the link stays.
func TestRenderThroughLink(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{"a.yaml": good, "b.yaml": good, "out/a.html": "old"})
	for name, to := range map[string]string{"a.html": filepath.Join("out", "a.html"), "b.html": filepath.Join("out", "b.html")} {
		testfs.Symlink(t, to, filepath.Join(dir, name))
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
	testfs.UnixOnly(t, "chmod keeps a directory from being written")
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
	testfs.UnixOnly(t, "Windows folds link/.. as written, before the link")
	t.Parallel()

	dir := write(t, map[string]string{"x.yaml": "lang: en\ntemplate: reprot\n", "real/x.yaml": good, "real/sub/y": ""})
	testfs.Symlink(t, filepath.Join("real", "sub"), filepath.Join(dir, "link"))
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

// renderLine is a command line of floe render and what it prints.
type renderLine struct {
	args []string
	want string
}

// renderEach renders each command line of tt in dir, its pages removed first, and wants what it prints.
func renderEach(t *testing.T, dir string, tt []renderLine) {
	t.Helper()
	for _, c := range tt {
		args, want := c.args, c.want
		for _, page := range []string{"d.html", "D.html", "e.html", "l.html"} {
			if err := os.Remove(filepath.Join(dir, page)); err != nil && !errors.Is(err, fs.ErrNotExist) {
				t.Fatal(err)
			}
		}
		if code, stdout, stderr := run(dir, append([]string{"render"}, args...)...); code != 0 || stdout != want || stderr != "" {
			t.Errorf("floe render %v = %d, %q, %q; want %q", args, code, stdout, stderr, want)
		}
	}
}

// One file named twice, as it is, through ./ or through a symlinked directory, is read once; other names of one
// file, linked hard or symbolically, are two, each with its page.
func TestRenderNamesOnce(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{"d.yaml": good})
	if err := os.Link(filepath.Join(dir, "d.yaml"), filepath.Join(dir, "e.yaml")); err != nil {
		t.Fatal(err)
	}
	testfs.Symlink(t, "d.yaml", filepath.Join(dir, "l.yaml"))
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	testfs.Symlink(t, "..", filepath.Join(dir, "sub", "up"))
	renderEach(t, dir, []renderLine{
		{[]string{"d.yaml", "e.yaml"}, "d.html\ne.html\n"},
		{[]string{"d.yaml", "l.yaml"}, "d.html\nl.html\n"},
		{[]string{"d.yaml", "d.yaml"}, "d.html\n"},
		{[]string{"./d.yaml", "d.yaml"}, "d.html\n"},
		{[]string{"d.yaml", "sub/up/d.yaml"}, "d.html\n"},
	})
}

// Hard links whose names differ only in case are two names, each with its page, its mistakes and its diagram:
// floe does not guess the file system ignores case. Where it does, the second link cannot be made, and the test
// is skipped.
func TestRenderCaseNames(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{"d.yaml": good, "x.yaml": "lang: en\ntemplate: nope\n"})
	code, spec, stderr := run(dir, "diagram", "example", "er")
	if code != 0 {
		t.Fatalf("floe diagram example er = %d, %q", code, stderr)
	}
	if err := os.WriteFile(filepath.Join(dir, "s.diagram.yaml"), []byte(spec), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, l := range [][2]string{{"d.yaml", "D.yaml"}, {"x.yaml", "X.yaml"}, {"s.diagram.yaml", "S.diagram.yaml"}} {
		err := os.Link(filepath.Join(dir, l[0]), filepath.Join(dir, l[1]))
		if errors.Is(err, fs.ErrExist) {
			t.Skip("the file system here ignores case: " + l[1] + " is " + l[0])
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "a"), 0o700); err != nil {
		t.Fatal(err)
	}
	renderEach(t, dir, []renderLine{
		{[]string{"d.yaml", "D.yaml"}, "d.html\nD.html\n"},
		{[]string{"d.yaml", "a/../D.yaml"}, "d.html\na/../D.html\n"},
	})
	// d.html there, D.html not.
	if err := os.Remove(filepath.Join(dir, "D.html")); err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	if code, _, stderr = run(dir, "render", "--check", "d.yaml", "D.yaml"); code != 1 || !strings.Contains(stderr, "D.html") {
		t.Errorf("floe render --check d.yaml D.yaml = %d, %q; want D.html told", code, stderr)
	}
	if code, _, stderr = run(dir, "check", "x.yaml", "X.yaml"); code != 1 ||
		!strings.Contains(stderr, "x.yaml:2:11: ") || !strings.Contains(stderr, "X.yaml:2:11: ") {
		t.Errorf("floe check x.yaml X.yaml = %d, %q; want the mistake of each", code, stderr)
	}
	code, stdout, stderr := run(dir, "diagram", "render", "--no-assets", "s.diagram.yaml", "S.diagram.yaml")
	if n := strings.Count(stdout, "<svg "); code != 0 || n != 2 {
		t.Errorf("floe diagram render of two names = %d, %q, drew %d diagrams; want 0 and 2", code, stderr, n)
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

// A page that is a link to no file, or a loop of links, is not written over, written or checked: it cannot be
// written, in the system's words.
func TestCannotWriteThroughLinks(t *testing.T) {
	t.Parallel()

	dir := write(t, map[string]string{"a.yaml": good, "b.yaml": good})
	for link, to := range map[string]string{"a.html": filepath.Join("out", "a.html"), "b.html": "b.html"} {
		testfs.Symlink(t, to, filepath.Join(dir, link))
	}
	tail := "): check that its directory exists and can be written to, and that its name is free and not too long\n"
	// The system's words of why each link leads nowhere (a directory not there, a loop), as it says them.
	words := map[string]string{}
	for file, link := range map[string]string{"a.yaml": "a.html", "b.yaml": "b.html"} {
		_, err := files.Stat(filepath.Join(dir, link))
		words[file] = testfs.Words(t, err)
	}
	told := map[string]string{}
	for file := range words {
		for _, mode := range [][]string{{"render"}, {"render", "--check"}} {
			args := append(slices.Clone(mode), file)
			code, _, stderr := run(dir, args...)
			page := strings.TrimSuffix(file, ".yaml") + ".html"
			if code != 1 || !strings.HasPrefix(stderr, page+": cannot be written (") || !strings.HasSuffix(stderr, tail) {
				t.Errorf("floe %v = %d, %q", args, code, stderr)
			}
			told[strings.Join(args, " ")] = stderr
		}
	}
	t.Run("in the system's words", func(t *testing.T) {
		t.Parallel()
		for args, stderr := range told {
			file := args[strings.LastIndex(args, " ")+1:]
			if want := strings.TrimSuffix(file, ".yaml") + ".html: cannot be written (" + words[file] + tail; stderr != want {
				t.Errorf("floe %s told %q, want %q", args, stderr, want)
			}
		}
	})
	for _, link := range []string{"a.html", "b.html"} {
		if info, err := os.Lstat(filepath.Join(dir, link)); err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%s is no longer the link: %v, %v", link, info, err)
		}
	}
}
