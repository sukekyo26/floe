package tmpl

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

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

// isMistake reports whether err holds a mistake of the user's, as List.Add finds one.
func isMistake(err error) bool {
	var one *usererr.Error
	var more usererr.List
	return errors.As(err, &one) || errors.As(err, &more)
}

// floeFailure words mistakes as floe's failure, where they are kept, and leaves a failure as it is.
func TestFloeFailure(t *testing.T) {
	t.Parallel()

	mistakes := usererr.List{usererr.NewAt("parts/x/part.html", 2, 5, "tmpl_go_template", "bad")}
	err := floeFailure("read floe's parts", mistakes)
	if err == nil || isMistake(err) || !strings.HasPrefix(err.Error(), "read floe's parts: parts/x/part.html:2:5: ") {
		t.Errorf("floeFailure of mistakes = %v, want floe's failure at parts/x/part.html:2:5", err)
	}
	failed := errors.New("disk")
	if err := floeFailure("read floe's parts", failed); !errors.Is(err, failed) {
		t.Errorf("floeFailure of a failure = %v, want it wrapped", err)
	}
	if err := floeFailure("read floe's parts", nil); err != nil {
		t.Errorf("floeFailure of nothing = %v, want nil", err)
	}
}

// mapOf copies the files of fsys into a file system of its own, for a test to change.
func mapOf(t *testing.T, fsys fs.FS) fstest.MapFS {
	t.Helper()
	m := fstest.MapFS{}
	err := fs.WalkDir(fsys, ".", func(at string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(fsys, at)
		m[at] = &fstest.MapFile{Data: b}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// floe's parts and themes are checked as floe part check checks one's own: what it refuses, and a warning,
// are floe's failure, at the file: the user cannot fix it.
func TestCheckFloeParts(t *testing.T) {
	t.Parallel()

	schema := `{"type": "object", "properties": {"text": {"type": "string"}}}`
	html := `<p data-floe-part="x">{{.text}}</p>`
	for name, files := range map[string]map[string]string{
		"none":    {},
		"parse":   {"part.html": `<p data-floe-part="x">{{if .text}}</p>`},
		"escape":  {"part.html": `<p data-floe-part="x"><a href="{{if .text}}/x/{{else}}/s?q={{end}}{{.text}}">x</a></p>`},
		"schema":  {"schema.json": `{"type":`},
		"pattern": {"schema.json": `{"type": "object", "properties": {"text": {"type": "string", "pattern": "["}}}`},
		"ref":     {"schema.json": `{"type": "object", "properties": {"text": {"$ref": "floe:///parts/nosuch.json"}}}`},
		"unnamed": {"part.html": `<p>{{.text}}</p>`},
		"css":     {"part.css": "[data-floe-part=\"x\"] { } }"},
		"warning": {"part.css": "[data-floe-part=\"x\"] { color: red; }"},
		"theme":   {"theme": "}"},
	} {
		root, themes := mapOf(t, partFiles), mapOf(t, themeFiles)
		for file, text := range map[string]string{"schema.json": schema, "part.html": html, "part.css": ""} {
			if broken, ok := files[file]; ok {
				text = broken
			}
			root[path.Join("parts", "x", file)] = &fstest.MapFile{Data: []byte(text)}
		}
		want := "parts/x"
		if css, ok := files["theme"]; ok {
			want = "themes/default/base.css"
			base := themes[want]
			themes[want] = &fstest.MapFile{Data: append(slices.Clone(base.Data), css...)}
		}
		err := checkFloeParts(root, themes)
		switch {
		case name == "none" && err != nil:
			t.Errorf("checkFloeParts of floe's parts and a part with no mistake = %v", err)
		case name != "none" && (err == nil || isMistake(err) || !strings.Contains(err.Error(), want)):
			t.Errorf("%s: checkFloeParts = %v, want floe's failure at %s", name, err, want)
		}
	}
}

// libraryOf returns a library of the templates of one's own in dir and the parts and themes of one's own in
// own, whose built-in templates are those of floe.
func libraryOf(dir string, own ownDirs, floe fs.FS) *Library {
	l := NewLibrary(dir, own.parts, own.themes, "")
	l.floe = floe
	return l
}

// A built-in template whose files have a mistake of their own is not found but floe's failure: those of its
// schema that floe notes beside and of its compiling with floe's parts too, and of its computed.yaml read
// against its schema bound to floe's parts alone, whatever the parts of one's own.
func TestFindBrokenBuiltin(t *testing.T) {
	t.Parallel()

	read := func(file string) string {
		b, err := fs.ReadFile(templateFiles, path.Join("templates", "report", file))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	schema, page := read("schema.json"), read("page.html")
	// One directory for both: parts of one's own that cannot be read, as they overlap the themes.
	overlap := t.TempDir()
	for _, tt := range []struct {
		name, file, text string
		own              ownDirs
	}{
		{"checks", "checks.yaml", "- for: sections\n  expr: item ==\n  message: x\n", ownDirs{}},
		{"format", "schema.json", strings.Replace(schema, `"properties": {`, `"properties": {"zz": {"type": "string", "format": "nope"}, `, 1), ownDirs{}},
		{"pattern", "schema.json", strings.Replace(schema, `"properties": {`, `"properties": {"zz": {"type": "string", "pattern": "["}, `, 1), ownDirs{}},
		{"ref", "schema.json", strings.Replace(schema, "floe:///parts/any.json", "floe:///parts/nosuch.json", 1), ownDirs{}},
		{"parse", "page.html", page + "{{if .title}}", ownDirs{}},
		{"escape", "page.html", page + `<a href="{{if .title}}/x/{{else}}/s?q={{end}}{{.title}}">x</a>`, ownDirs{}},
		{"words", "words.yaml", "en: [\n", ownDirs{}},
		{"computed", "computed.yaml", "total: title +\n", ownDirs{parts: overlap, themes: overlap}},
	} {
		floe := mapOf(t, templateFiles)
		floe[path.Join("templates", "report", tt.file)] = &fstest.MapFile{Data: []byte(tt.text)}
		tpl, err := libraryOf("", tt.own, floe).Find("report")
		if tpl != nil || err == nil || isMistake(err) || !strings.Contains(err.Error(), "report/"+tt.file) {
			t.Errorf("%s: Find = %v, %v; want floe's failure at report/%s", tt.name, tpl, err, tt.file)
		}
	}
	// Checked once for a library and its runs: floe's files do not change while it runs.
	l := libraryOf("", ownDirs{}, fstest.MapFS{"templates/report/schema.json": &fstest.MapFile{Data: []byte(`{"type":`)}})
	_, first := l.Run().Find("report")
	l.floe = templateFiles
	if _, err := l.Run().Find("report"); first == nil || err == nil || err.Error() != first.Error() {
		t.Errorf("Find in another run = %v, want %v as checked first", err, first)
	}
}

// floe's check of a built-in template reads no template of one's own of its name, which would take its place.
func TestCheckFloeReadsNoOwn(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "report"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "report", "schema.json"), []byte(`{"type":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := libraryOf(dir, ownDirs{}, templateFiles).floeChecked("report"); err != nil {
		t.Errorf("floe's check of report beside a broken one of one's own = %v, want nil", err)
	}
}

// heldFS holds the opening of a file under at until open is closed, telling started when one waits.
type heldFS struct {
	fs.FS
	at      string
	started chan struct{}
	open    chan struct{}
	once    sync.Once
}

func (h *heldFS) Open(name string) (fs.File, error) {
	if strings.HasPrefix(name, h.at+"/") {
		h.once.Do(func() { close(h.started) })
		<-h.open
	}
	return h.FS.Open(name) //nolint:wrapcheck // the file system's own
}

// floe's check of a built-in template does not wait for that of another name.
func TestCheckFloeOfAnotherName(t *testing.T) {
	t.Parallel()

	floe := mapOf(t, templateFiles)
	for name, f := range floe {
		if rest, ok := strings.CutPrefix(name, "templates/report/"); ok {
			floe["templates/slow/"+rest] = f
		}
	}
	held := &heldFS{FS: floe, at: "templates/slow", started: make(chan struct{}), open: make(chan struct{}), once: sync.Once{}}
	l := libraryOf("", ownDirs{}, held)
	slow := make(chan error)
	go func() { slow <- l.floeChecked("slow") }()
	<-held.started
	report := make(chan error)
	go func() { report <- l.floeChecked("report") }()
	select {
	case err := <-report:
		if err != nil {
			t.Errorf("floe's check of report = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("floe's check of report waits for that of slow")
	}
	close(held.open)
	if err := <-slow; err != nil {
		t.Errorf("floe's check of slow = %v", err)
	}
}
