package tmpl_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

// own writes a copy of the built-in report as the template name under dir, with page.html marked so
// that a page tells which report drew it.
func own(t *testing.T, dir, name string) string {
	t.Helper()

	at := filepath.Join(dir, name)
	if err := os.CopyFS(at, os.DirFS("templates/report")); err != nil {
		t.Fatal(err)
	}
	page := filepath.Join(at, "page.html")
	src, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(page, append(src, "<i>own</i>"...), 0o600); err != nil { //nolint:gosec // G703: the test's own directory
		t.Fatal(err)
	}
	return at
}

func draw(l *tmpl.Library, src string) (string, error) {
	d, err := l.Read("data.yaml", []byte(src), "", "")
	if err != nil {
		return "", err
	}
	b, err := d.Render("t")
	return string(b), err
}

// A template of one's own takes the place of the built-in one of its name, a symlinked one included,
// and the names list both kinds once; files and names starting with a dot are not templates.
func TestOwnTemplates(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	own(t, dir, "report")
	elsewhere := own(t, t.TempDir(), "memo")
	if err := os.Symlink(elsewhere, filepath.Join(dir, "memo")); err != nil {
		t.Fatal(err)
	}
	own(t, dir, ".installing")
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	l := tmpl.NewLibrary(dir, "", "", "")
	if got, err := l.Names(); err != nil || !slices.Equal(got, []string{"memo", "report"}) {
		t.Errorf("Names = %v, %v", got, err)
	}
	for _, name := range []string{"report", "memo"} {
		if page, err := draw(l, "template: "+name+"\nlang: en\n"); err != nil || !strings.Contains(page, "<i>own</i>") {
			t.Errorf("a page of %s = %v, drawn by the built-in one", name, err)
		}
	}
	if page, err := draw(tmpl.NewLibrary("", "", "", ""), "template: report\nlang: en\n"); err != nil || strings.Contains(page, "<i>own</i>") {
		t.Errorf("a page of the built-in report = %v", err)
	}
	// A file where the directory of templates would be holds none.
	if _, err := draw(tmpl.NewLibrary(filepath.Join(dir, "notes.txt"), "", "", ""), "template: report\nlang: en\n"); err != nil {
		t.Errorf("a page of report under a file in the way = %v", err)
	}
	for _, name := range []string{"../report", ".installing", "notes.txt", "a/b", "Report", "memo/"} {
		if _, err := draw(l, "template: "+name+"\nlang: en\n"); err == nil || !strings.Contains(err.Error(), "unknown template") {
			t.Errorf("template %s = %v, want unknown", name, err)
		}
	}
}

// A broken template of one's own stops, and does not give way to the built-in one: a file it lacks,
// a symlink that leads nowhere, and a file that leads out of it.
func TestBrokenOwnTemplates(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.Remove(filepath.Join(own(t, dir, "report"), "page.html")); err != nil {
		t.Fatal(err)
	}
	if _, err := draw(tmpl.NewLibrary(dir, "", "", ""), "template: report\nlang: en\n"); err == nil || !strings.Contains(err.Error(), "page.html") {
		t.Errorf("a template without page.html = %v", err)
	}

	dangling := t.TempDir()
	if err := os.Symlink(filepath.Join(dangling, "gone"), filepath.Join(dangling, "report")); err != nil {
		t.Fatal(err)
	}
	if _, err := draw(tmpl.NewLibrary(dangling, "", "", ""), "template: report\nlang: en\n"); err == nil || !strings.Contains(err.Error(), "your own template report (") {
		t.Errorf("a symlink that leads nowhere = %v", err)
	}

	leaky := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.css")
	if err := os.WriteFile(outside, []byte("body{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	css := filepath.Join(own(t, leaky, "report"), "page.css")
	if err := os.Remove(css); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, css); err != nil {
		t.Fatal(err)
	}
	if _, err := draw(tmpl.NewLibrary(leaky, "", "", ""), "template: report\nlang: en\n"); err == nil || !strings.Contains(err.Error(), "page.css") {
		t.Errorf("a page.css that leads out of the template = %v", err)
	}

	// A template of one's own named in another case is refused, not passed over for the built-in one.
	cased := t.TempDir()
	own(t, cased, "Report")
	if _, err := draw(tmpl.NewLibrary(cased, "", "", ""), "template: report\nlang: en\n"); err == nil || !strings.Contains(err.Error(), "is not template report: template names are lower case") {
		t.Errorf("a template of one's own named Report = %v", err)
	}

	// A file a template may leave out is broken, not missing, when it is a symlink that leads nowhere.
	for _, file := range []string{"template.yaml", "checks.yaml", "computed.yaml"} {
		dir := t.TempDir()
		path := filepath.Join(own(t, dir, "report"), file)
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			t.Fatal(err)
		}
		if err := os.Symlink("gone.yaml", path); err != nil {
			t.Fatal(err)
		}
		if _, err := draw(tmpl.NewLibrary(dir, "", "", ""), "template: report\nlang: en\n"); err == nil || !strings.Contains(err.Error(), file) {
			t.Errorf("a %s that leads nowhere = %v", file, err)
		}
	}

	// A directory of templates that cannot be read says so, rather than a template it may not hold.
	if os.Getuid() != 0 {
		locked := t.TempDir()
		if err := os.Chmod(locked, 0); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := os.Chmod(locked, 0o700); err != nil {
				t.Error(err)
			}
		})
		for _, src := range []string{"template: report\nlang: en\n", "title: no template\n"} {
			if _, err := draw(tmpl.NewLibrary(locked, "", "", ""), src); err == nil || !strings.Contains(err.Error(), "the directory of your own templates") {
				t.Errorf("%q under a directory of templates that cannot be read = %v", src, err)
			}
		}
	}
}

// template.yaml names the floe a template needs; a template for another major of floe, or for a later
// one, is refused before its data is checked.
func TestTemplateVersion(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ yaml, want string }{
		{"floe: 0\n", ""},
		{"floe: 0.0.0\n", ""},
		{"floe: 1.2\n", "template report needs floe 1.2.0 or later, and this is floe 0.0.0"},
		{"floe: 0.1\n", "template report needs floe 0.1.0 or later, and this is floe 0.0.0"},
		{"floe: 0.0.1\n", "template report needs floe 0.0.1 or later"},
		{"floe: x\n", "/report/template.yaml:1:7: write template.yaml as one key, floe:"},
		{"floe: [0]\n", "template.yaml:1:7: write template.yaml as one key, floe:"},
		{"flow: 0\n", "template.yaml:1:1: flow is not a key of template.yaml"},
		{"", "template.yaml:1:1: write template.yaml as one key, floe:"},
	} {
		// Linked from the directory of templates, a mistake is named where it was found.
		dir := t.TempDir()
		at := own(t, t.TempDir(), "report")
		if err := os.Symlink(at, filepath.Join(dir, "report")); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(at, "template.yaml"), []byte(tt.yaml), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := draw(tmpl.NewLibrary(dir, "", "", ""), "template: report\nleed: a typo\nlang: en\n")
		switch {
		case tt.want == "" && (err == nil || !strings.Contains(err.Error(), "leed is not an item")):
			t.Errorf("template.yaml %q = %v, want the data's mistake", tt.yaml, err)
		case tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)):
			t.Errorf("template.yaml %q = %v, want %s", tt.yaml, err, tt.want)
		case strings.HasPrefix(tt.want, "/") && !strings.Contains(err.Error(), "\n  "+dir):
			t.Errorf("template.yaml %q = %v, named where the symlink leads, not under %s", tt.yaml, err, dir)
		case tt.want != "" && !strings.HasPrefix(err.Error(), "data.yaml:1:11: your own template report cannot be used\n"):
			t.Errorf("template.yaml %q = %v, want the template cannot be used, at template:", tt.yaml, err)
		}
	}
}

// What a template's files, or a part's, hold whatever the data is told once for two data files drawn with
// them, though drawing finds it.
func TestTemplateMistakesOnce(t *testing.T) {
	t.Parallel()

	head := `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head>`
	part := `"risks": {"$ref": "floe:///parts/pp.json"}`
	html := `"risks": {"type": "string", "format": "html"}`
	for name, tt := range map[string]struct {
		items, data string
		files       map[string]string // of the template memo
		parts       map[string]string
		unusable    bool // the template cannot be used: the second data file is told so, as above
	}{
		"an expression of checks.yaml": {
			items: `"n": {"type": "array", "items": {"type": "number"}}`, data: "n: [1]\n", unusable: true,
			files: map[string]string{"checks.yaml": "- for: n\n  expr: item +++ (\n  message: m\n"},
		},
		"page.html html/template cannot escape": {
			items: html, data: "",
			files: map[string]string{"page.html": head + `<body>{{if .Title}}<a href="{{end}}</body></html>`},
		},
		"part.html html/template cannot escape": {
			items: part, data: "risks: { text: hello }\n",
			parts: map[string]string{
				"pp/schema.json": `{"type": "object", "properties": {"text": {"type": "string"}}}`,
				"pp/part.html":   "<p data-floe-part=\"pp\">\n<script>{{if eq .text `a`}}var a = '{{end}}</script></p>",
				"pp/part.css":    "",
			},
		},
		"page.html without <!doctype html>": {
			items: html, data: "risks: <p>a</p>\n",
			files: map[string]string{"page.html": `<html lang="{{.Lang}}"><head><title>{{.Title}}</title></head>` +
				`<body><div>{{.Data.risks}}</div></body></html>`},
		},
		"page.html drawing an item outside the head and the body": {
			items: html, data: "risks: <p>hello there, a paragraph longer than a mark</p>\n",
			files: map[string]string{"page.html": head + `{{.Data.risks}}<body class="r"><p>x</p></body></html>`},
		},
		"an image page.html asks for": {
			items: html, data: "",
			files: map[string]string{"page.html": head + `<body><img src="{{asset "nope.png"}}" alt=""></body></html>`},
		},
		"page.html naming two parts as one": {
			items: html, data: "",
			files: map[string]string{"page.html": head + `<body><div data-floe-part="note table"></div></body></html>`},
		},
		"a word the data names that words.yaml has not": {
			items: `"level": {"type": "string"}`, data: "level: high\n",
			files: map[string]string{
				"page.html": head + `<body>{{word .Data.level}}</body></html>`, "words.yaml": "en:\n  low: Low\n",
			},
		},
	} {
		dir := memo(t, tt.items)
		for file, text := range tt.files {
			if err := os.WriteFile(filepath.Join(dir, "memo", file), []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		parts := ""
		if tt.parts != nil {
			parts = ownParts(t, tt.parts)
		}
		l := tmpl.NewLibrary(dir, parts, "", "").Run()
		var all usererr.List
		var first string
		for _, file := range []string{"a.yaml", "b.yaml"} {
			d, err := l.Read(file, []byte("template: memo\nlang: en\ntitle: t\n"+tt.data), "", "")
			if err == nil {
				_, err = d.Render("t")
			}
			if err == nil {
				t.Fatalf("%s: drawing %s = nil, want mistakes", name, file)
			}
			if first == "" {
				first = usererr.Tell(i18n.EnglishCatalog(), err).Render("")
			}
			if failed := all.Add(err); failed != nil {
				t.Fatalf("%s: drawing %s = %v, want mistakes", name, file, err)
			}
		}
		want := first
		if tt.unusable {
			want += "\nb.yaml:1:11: your own template memo cannot be used, as told above"
		}
		if got := usererr.Tell(i18n.EnglishCatalog(), all).Render(""); got != want {
			t.Errorf("%s: the mistakes of two data files =\n%s\nwant\n%s", name, got, want)
		}
	}
}
