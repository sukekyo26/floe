package tmpl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/usererr"
)

// A floe fits a template from the version it needs up to the next major, or the next minor under
// major 0.
func TestFits(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		needs, have semver
		want        string
	}{
		{semver{1, 2, 0}, semver{1, 2, 0}, ""},
		{semver{1, 2, 0}, semver{1, 9, 3}, ""},
		{semver{1, 2, 0}, semver{1, 1, 9}, "template t needs floe 1.2.0 or later, and this is floe 1.1.9"},
		{semver{1, 2, 0}, semver{2, 0, 0}, "template t is written for floe 1, not for this floe 2.0.0"},
		{semver{2, 0, 0}, semver{1, 9, 0}, "template t needs floe 2.0.0 or later"},
		{semver{0, 2, 0}, semver{0, 2, 5}, ""},
		{semver{0, 2, 0}, semver{0, 3, 0}, "template t is written for floe 0.2, not for this floe 0.3.0"},
		{semver{0, 2, 1}, semver{0, 2, 0}, "template t needs floe 0.2.1 or later"},
	} {
		err := fits("t", tt.needs, tt.have)
		if tt.want == "" && err != nil || tt.want != "" && (err == nil || !strings.HasPrefix(err.Error(), tt.want)) {
			t.Errorf("fits(%v, %v) = %v, want %q", tt.needs, tt.have, err, tt.want)
		}
	}
}

// A template gone after it was found is an error, not a template without its optional files.
func TestReadOptionalOfATemplateGone(t *testing.T) {
	t.Parallel()

	gone := &Template{Name: "gone", Dir: filepath.Join(t.TempDir(), "gone"), Path: ""}
	if _, found, err := gone.readOptional("checks.yaml"); err == nil || found {
		t.Errorf("readOptional of a template gone = %v, %v", found, err)
	}
}

// This floe's version leaves out what follows its release.
func TestReleaseOf(t *testing.T) {
	t.Parallel()

	for s, want := range map[string]semver{"1.2.3": {1, 2, 3}, "1.2.3-rc.1": {1, 2, 3}, "0.4.0+build.5": {0, 4, 0}} {
		if got, ok := releaseOf(s); !ok || got != want {
			t.Errorf("releaseOf(%q) = %v, %v", s, got, ok)
		}
	}
	if _, ok := releaseOf("v1.2.3"); ok {
		t.Error("releaseOf took v1.2.3")
	}
}

// The version is read as written: YAML would read 1.10 as the number 1.1.
func TestNeedsFloe(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		src  string
		want semver
	}{
		{"floe: 1.10\n", semver{1, 10, 0}},
		{"floe: '2.3.4'\n", semver{2, 3, 4}},
		{"floe: 1\n", semver{1, 0, 0}},
	} {
		if got, err := needsFloe("template.yaml", []byte(tt.src)); err != nil || got != tt.want {
			t.Errorf("needsFloe(%q) = %v, %v; want %v", tt.src, got, err, tt.want)
		}
	}
	for _, src := range []string{"floe: 1.2.3.4\n", "floe: 01.2\n", "floe: -1\n", "floe: 1.x\n", "floe:\n"} {
		if _, err := needsFloe("template.yaml", []byte(src)); err == nil {
			t.Errorf("needsFloe(%q) took it", src)
		}
	}
}

// A run reads what its reads share once: a template found by name, the parts and themes of one's own and the
// mistakes of a template's files are the same each time, for the mistakes to be told once. Without a run,
// each read reads them again.
func TestRunReadsOnce(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	templates, themes := filepath.Join(dir, "templates"), filepath.Join(dir, "themes")
	write := func(name, text string) {
		t.Helper()
		file := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(text), 0o600); err != nil { //nolint:gosec // G703: under the test's own directory
			t.Fatal(err)
		}
	}
	write("themes/mine/palette.css", "")
	write("templates/m/schema.json", `{"type": "object", "additionalProperties": false, "properties": {}}`)
	write("templates/m/page.html", `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head><body></body></html>`)
	write("templates/m/computed.yaml", "x: nope\n")
	src := []byte("template: m\nlang: en\ntitle: t\n")

	l := NewLibrary(templates, "", themes, "").Run()
	m, err := l.Find("m")
	if again, err2 := l.Find("m"); err != nil || err2 != nil || again != m {
		t.Errorf("Find in a run = %p, %p, %v, %v, want the same template", m, again, err, err2)
	}
	report, err := l.Find("report")
	if err != nil {
		t.Fatal(err)
	}
	ownSet, err := m.parts()
	if reportSet, err2 := report.parts(); err != nil || err2 != nil || ownSet != reportSet {
		t.Errorf("the parts of two templates in a run = %p, %p, %v, %v, want one read", ownSet, reportSet, err, err2)
	}
	// Out of a run, every read reads anew: a file fixed in between is seen.
	fresh := NewLibrary(templates, "", themes, "")
	if got := mistakesOf(t, fresh, "a.yaml", src, dir); len(got) != 1 {
		t.Fatalf("reading with computed.yaml broken, out of a run = %v, want its mistake", got)
	}
	if err = os.Remove(filepath.Join(templates, "m", "computed.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err = fresh.Read("b.yaml", src, "", dir); err != nil {
		t.Errorf("reading after computed.yaml is fixed, out of a run = %v, want it read anew", err)
	}

	// The files of a template are read once in a run: broken after the first data file, the second still
	// draws with them as they were.
	files := map[string]string{
		"page.html": `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head>` +
			`<body>{{.Calc.twice}}</body></html>`,
		"computed.yaml": "twice: sum(n) * 2\n",
		"checks.yaml":   "- for: n\n  expr: item > 0\n  message: positive\n",
	}
	broken := map[string]string{
		"page.html": "<html><body></body></html>", "computed.yaml": "twice: nope\n",
		"checks.yaml": "- for: n\n  expr: item < 0\n  message: negative\n",
	}
	write("templates/k/schema.json", `{"type": "object", "properties": {"n": {"type": "array", "items": {"type": "number"}}}}`)
	src = []byte("template: k\nlang: en\ntitle: t\nn: [2]\n")
	for name := range broken {
		for n, text := range files {
			write("templates/k/"+n, text)
		}
		run := NewLibrary(templates, "", themes, "").Run()
		draw := func(file string) error {
			d, e := run.Read(file, src, "", dir)
			if e == nil {
				_, e = d.Render("t")
			}
			return e
		}
		if err = draw("a.yaml"); err != nil {
			t.Fatal(err)
		}
		write("templates/k/"+name, broken[name])
		if err = draw("b.yaml"); err != nil {
			t.Errorf("a data file drawn after %s is broken, in the run that read it = %v, want it read once", name, err)
		}
	}
}

func mistakesOf(t *testing.T, l *Library, file string, src []byte, cwd string) usererr.List {
	t.Helper()
	_, err := l.Read(file, src, "", cwd)
	var found usererr.List
	if failed := found.Add(err); failed != nil || found == nil {
		t.Fatalf("Read %s = %v, want mistakes", file, err)
	}
	return found
}
