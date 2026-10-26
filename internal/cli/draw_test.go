package cli_test

import (
	"os"
	"path/filepath"
	"strings"
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
