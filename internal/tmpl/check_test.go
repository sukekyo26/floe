package tmpl_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/version"
)

// The template of testdata/sizes uses every stage: its checks, its computed values, num, bytes, its
// assets and an image of the data.
func TestDrawEveryStage(t *testing.T) {
	t.Parallel()

	const file = "testdata/sizes/example.yaml"
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	l := tmpl.NewLibrary("testdata", "")
	d, err := l.Read(file, src, tmpl.Themes{}, "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := d.Render("t")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"<p><b>made</b> for tests</p>", "<td>1.5 KiB</td>", "<p>2,049,536 bytes, 1 of them",
		`<img src="data:image/svg&#43;xml;base64,`, `<img src="data:image/png;base64,iVBORw0KGgo`,
	} {
		if !strings.Contains(string(b), want) {
			t.Errorf("the page has no %s:\n%s", want, b)
		}
	}
	twice := strings.Replace(string(src), "b.png", "a.txt", 1)
	if _, err := l.Read(file, []byte(twice), tmpl.Themes{}, ""); err == nil || !strings.Contains(err.Error(), file+":8:11: files[1].name: give each file its own name") {
		t.Errorf("a check that fails = %v", err)
	}
}

// A right template passes, warned only that it does not name the floe it needs.
func TestCheck(t *testing.T) {
	t.Parallel()

	warnings, mistakes := tmpl.Check("testdata/sizes", "")
	if len(warnings) != 1 || len(mistakes) != 0 {
		t.Fatalf("Check = %v, %v", warnings, mistakes)
	}
	release := strings.SplitN(version.Version, ".", 3)
	if want := "testdata/sizes has no template.yaml: write floe: " + release[0] + "." + release[1] + " in it"; !strings.HasPrefix(warnings[0].Error(), want) {
		t.Errorf("the warning = %s, want %s", warnings[0], want)
	}

	dir := sizes(t, map[string]string{"template.yaml": "floe: " + release[0] + "." + release[1] + "\n"})
	if warnings, mistakes := tmpl.Check(dir, ""); len(warnings) != 0 || len(mistakes) != 0 {
		t.Errorf("Check with template.yaml = %v, %v", warnings, mistakes)
	}
}

// Each mistake of a template is reported, at its line and column where it has one; one in its files
// stops its example from being checked, which would only fail.
func TestCheckMistakes(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		files   map[string]string
		want    []string
		warning string
	}{
		{
			name:  "unknown format",
			files: map[string]string{"schema.json": `"format": "date"` + "|" + `"format": "datum"`},
			want:  []string{`sizes/schema.json: #/properties/files/items/properties/updated: floe does not know the format "datum"`},
		},
		{
			name:  "html out of reach",
			files: map[string]string{"schema.json": `"note": { "type": "string", "format": "html" }` + "|" + `"note": { "anyOf": [{ "type": "string", "format": "html" }] }`},
			want:  []string{"sizes/schema.json: #/properties/note/anyOf/0: the format html marks an item to draw"},
		},
		{
			name:    "open schema",
			files:   map[string]string{"schema.json": `"required": ["name", "bytes"],` + "\n" + `        "additionalProperties": false` + "|" + `"required": ["name", "bytes"]`},
			warning: `sizes/schema.json: #/properties/files/items declares properties without "additionalProperties": false`,
		},
		{
			name: "checks",
			files: map[string]string{"checks.yaml": "- for: file\n  expr: 'true'\n  message: m\n" +
				"- for: files.0\n  expr: 'true'\n  message: m\n" +
				"- for: files\n  field: size\n  expr: 'true'\n  message: m\n" +
				"- for: file\n  expr: itme\n  message: m\n"},
			want: []string{
				"sizes/checks.yaml:1:8: for: schema.json does not declare file",
				"sizes/checks.yaml:4:8: for: schema.json does not declare files.0",
				"sizes/checks.yaml:8:10: field: schema.json does not declare size under the items of files",
				"sizes/checks.yaml:11:8: for: schema.json does not declare file",
				"sizes/checks.yaml:12:9: itme cannot be read as an expression: unknown name itme",
			},
		},
		{
			name:    "open to any key",
			files:   map[string]string{"schema.json": `"additionalProperties": false` + "|" + `"unevaluatedProperties": true`},
			warning: `sizes/schema.json: #/properties/files/items declares properties without "additionalProperties": false`,
		},
		{
			name:  "anchors",
			files: map[string]string{"checks.yaml": "- for: files\n  expr: 'true'\n  message: &m m\n- for: files\n  expr: 'true'\n  message: *m\n"},
			want:  []string{"sizes/checks.yaml:3:12: anchors, aliases, merge keys (<<) and tags are not supported"},
		},
		{
			name:  "missing key",
			files: map[string]string{"checks.yaml": "- for: files\n  expr:\n  message: m\n"},
			want:  []string{"sizes/checks.yaml:2:3: write expr in the check"},
		},
		{
			name: "lists through anyOf and $ref",
			files: map[string]string{
				"schema.json": `"note": { "type": "string", "format": "html" },` + "|" +
					`"others": { "anyOf": [{ "type": "array", "items": { "$ref": "#/properties/files/items" } }, { "type": "null" }] },` +
					`"named": { "type": "array", "items": { "$ref": "#/properties/files/items" } },` +
					`"note": { "type": "string", "format": "html" },`,
				"checks.yaml": "- for: others\n  field: name\n  expr: 'true'\n  message: m\n" +
					"- for: named\n  field: name\n  expr: 'true'\n  message: m\n",
			},
		},
		{
			name:  "a broken check beside others",
			files: map[string]string{"checks.yaml": "- for: files\n  expr: 'true'\n- for: filez\n  expr: itme\n  message: m\n"},
			want: []string{
				"sizes/checks.yaml:1:3: write message in the check",
				"sizes/checks.yaml:3:8: for: schema.json does not declare filez",
				"sizes/checks.yaml:4:9: itme cannot be read as an expression",
			},
		},
		{
			name:  "a value and a name of computed.yaml",
			files: map[string]string{"computed.yaml": "in: [1]\nsum: 1\n"},
			want: []string{
				"sizes/computed.yaml:1:5: write computed.yaml as names and expressions",
				"sizes/computed.yaml:2:1: sum is not a name",
			},
		},
		{
			name:  "not a list",
			files: map[string]string{"checks.yaml": "- for: note\n  expr: 'true'\n  message: m\n"},
			want:  []string{"sizes/checks.yaml:1:8: for: schema.json does not declare note a list"},
		},
		{
			name:  "computed",
			files: map[string]string{"computed.yaml": "total: sum(files, .bytes)\nfiles: 1\n"},
			want:  []string{"sizes/computed.yaml:2:1: files is an item of the data too"},
		},
		{
			name:  "page",
			files: map[string]string{"page.html": "{{if}}"},
			want:  []string{"parse the template sizes: "},
		},
		{
			name:  "example",
			files: map[string]string{"example.yaml": "template: report\nfiles:\n  - name: a\nlang: en\n"},
			want: []string{
				"sizes/example.yaml:1:11: example.yaml names the template report; name it sizes",
				"sizes/example.yaml:3:5: files[0].bytes is required but missing",
			},
		},
		{
			name:  "example of another theme",
			files: map[string]string{"example.yaml": "template: report\ntheme: print\nfiles: []\nlang: en\n"},
			want: []string{
				"sizes/example.yaml:1:11: example.yaml names the template report; name it sizes",
				`sizes/example.yaml:2:8: template sizes has no theme "print"`,
			},
		},
		{
			name:  "example without a template",
			files: map[string]string{"example.yaml": "title: 3\nfiles: []\nlang: en\n"},
			want: []string{
				"sizes/example.yaml:1:1: name the template at the top level, like template: report (templates: sizes)",
				"sizes/example.yaml:1:8: title must be text",
			},
		},
		{
			name:  "an image of the example",
			files: map[string]string{"example.yaml": "chart: chart.png|chart: none.png"},
			want:  []string{`sizes/example.yaml:4:8: chart: there is no image "none.png" under `},
		},
		{
			name:  "an asset",
			files: map[string]string{"assets/notes.png": "not an image", "assets/.DS_Store": "x", "assets/README.md": "x"},
			want:  []string{`"notes.png" does not hold the image/png its name says`},
		},
		{
			name:  "an asset page.html names",
			files: map[string]string{"assets/logo.svg": "not svg"},
			want:  []string{`sizes/assets: "logo.svg" does not hold the image/svg+xml its name says`},
		},
		{
			name:  "an asset in a branch the example does not reach",
			files: map[string]string{"page.html": "<body>|<body>{{if false}}{{asset \"nope.svg\"}}{{else}}{{asset \"logo.svg\"}}{{end}}"},
			want:  []string{`sizes/page.html: there is no image "nope.svg" under `},
		},
		{
			name:    "a theme without the variables of diagrams",
			files:   map[string]string{"schema.json": `"chart":|"plan": { "$ref": "floe:///parts/diagram.json" },` + "\n    " + `"chart":`},
			warning: "sizes/themes/default.css does not define --bg, --surface, --fg, --muted, --line, --accent, --accent-soft",
		},
		{
			name:    "a theme without the variables of a Markdown table",
			files:   map[string]string{"schema.json": `"chart":|"aside": { "type": "string", "format": "markdown" },` + "\n    " + `"chart":`},
			warning: "sizes/themes/default.css does not define --accent-soft, --accent, --good-soft, --good, --warn-soft, --warn, --bad-soft, --bad, --surface, --line, --muted,",
		},
		{
			name: "formats of other types",
			files: map[string]string{"schema.json": `"note": { "type": "string", "format": "html" }|"note": { "type": "object", "format": "html", ` +
				`"properties": { "logo": { "type": "string", "format": "image" } } }, "plan": { "format": "diagram" }`},
			want: []string{
				`sizes/schema.json: #/properties/note: floe draws an item of the format html from a value of type string; declare "type": "string" alone`,
				"sizes/schema.json: #/properties/note/properties/logo: the format image marks an item to draw",
				`sizes/schema.json: #/properties/plan: a diagram is a part of floe's, not a format; write "$ref": "floe:///parts/diagram.json"`,
			},
		},
		{
			name: "a script out of reach, and Markdown of type object",
			files: map[string]string{"schema.json": `"chart":|"move": { "oneOf": [{ "$ref": "floe:///parts/script.json" }] }, ` +
				`"run": { "type": "object", "format": "markdown" },` + "\n    " + `"chart":`},
			want: []string{
				"sizes/schema.json: #/properties/move/oneOf/0: the part script draws an item",
				`sizes/schema.json: #/properties/run: floe draws an item of the format markdown from a value of type string`,
			},
		},
		{
			name: "a format inside a diagram",
			files: map[string]string{"schema.json": `"chart":|"plan": { "$ref": "floe:///parts/diagram.json", ` +
				`"properties": { "logo": { "type": "string", "format": "image" } } },` + "\n    " + `"chart":`},
			want: []string{"sizes/schema.json: #/properties/plan/properties/logo: the format image marks an item to draw"},
		},
		{
			name:  "an item floe reads itself",
			files: map[string]string{"schema.json": `"chart":|"lang": { "type": "string" },` + "\n    " + `"chart":`},
			want: []string{
				"sizes/schema.json declares or requires lang, which floe reads from every data file itself (lang, parts-theme, template, theme, title): give the items of the schema other names",
			},
		},
		{
			name: "an item floe reads itself beside another mistake",
			files: map[string]string{
				"schema.json": `"chart":|"title": { "type": "string" },` + "\n    " + `"chart":`, "page.html": "{{if}}",
			},
			want: []string{"sizes/schema.json declares or requires title, which floe reads", "parse the template sizes: "},
		},
		{
			name:  "an empty example",
			files: map[string]string{"example.yaml": ""},
			want: []string{
				"sizes/example.yaml:1:1: name the language of the page at the top level",
				"sizes/example.yaml:1:1: name the template at the top level, like template: report (templates: sizes)",
				"sizes/example.yaml:1:1: files is required but missing",
			},
		},
		{
			name:  "newer floe",
			files: map[string]string{"template.yaml": "floe: 999.0\n"},
			want:  []string{"template sizes needs floe 999.0.0 or later"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			warnings, mistakes := tmpl.Check(sizes(t, tt.files), "")
			lines := make([]string, 0, len(mistakes))
			for _, m := range mistakes {
				lines = append(lines, strings.Split(m.Error(), "\n")...)
			}
			if len(lines) != len(tt.want) {
				t.Fatalf("the mistakes =\n%s\nwant %d", strings.Join(lines, "\n"), len(tt.want))
			}
			for i, w := range tt.want {
				if !strings.Contains(lines[i], w) {
					t.Errorf("mistake %d = %s\nwant %s", i, lines[i], w)
				}
			}
			if tt.warning != "" && (len(warnings) < 2 || !strings.Contains(warnings[1].Error(), tt.warning)) {
				t.Errorf("the warnings = %v, want %s", warnings, tt.warning)
			}
		})
	}
}

// A <style> of page.html is refused at its line, as floe check and the drawing refuse it: floe puts a
// page's CSS together.
func TestPageStyleRefused(t *testing.T) {
	t.Parallel()

	dir := sizes(t, map[string]string{"page.html": "<body>|<body>\n<!-- <style> -->\n<style>p {}</style>"})
	want := filepath.Join(dir, "page.html") + ":6: page.html holds a <style>: write the template's CSS in page.css"
	if _, mistakes := tmpl.Check(dir, ""); len(mistakes) != 1 || !strings.HasPrefix(mistakes[0].Error(), want) {
		t.Errorf("Check = %v, want %s", mistakes, want)
	}
}

// The variables of diagrams that page.css defines, for every theme, are not missed in a theme.
func TestCheckVariablesInPage(t *testing.T) {
	t.Parallel()

	release := strings.SplitN(version.Version, ".", 3)
	dir := sizes(t, map[string]string{
		"template.yaml": "floe: " + release[0] + "." + release[1] + "\n",
		"schema.json":   `"chart":|"plan": { "$ref": "floe:///parts/diagram.json" },` + "\n    " + `"chart":`,
		"page.css":      ":root { " + strings.Join(diagram.Variables(), ": red; ") + ": red }",
	})
	if warnings, mistakes := tmpl.Check(dir, ""); len(warnings) != 0 || len(mistakes) != 0 {
		t.Errorf("Check = %v, %v", warnings, mistakes)
	}
}

// A directory is checked as a template of its name, which must be one.
func TestCheckName(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "Sizes")
	if err := os.CopyFS(dir, os.DirFS("testdata/sizes")); err != nil {
		t.Fatal(err)
	}
	_, mistakes := tmpl.Check(dir, "")
	var e *usererr.Error
	if len(mistakes) != 1 || !errors.As(mistakes[0], &e) || e.Key != "tmpl_new_name" {
		t.Errorf("Check of %s = %v", dir, mistakes)
	}
}

// A file a template needs is named when it is missing.
func TestCheckMissingFile(t *testing.T) {
	t.Parallel()

	dir := sizes(t, nil)
	if err := os.Remove(filepath.Join(dir, "example.yaml")); err != nil {
		t.Fatal(err)
	}
	_, mistakes := tmpl.Check(dir, "")
	var e *usererr.Error
	if len(mistakes) != 1 || !errors.As(mistakes[0], &e) || e.Key != "tmpl_file_missing" {
		t.Fatalf("Check without example.yaml = %v", mistakes)
	}
	if ja := e.Localize(i18n.New("ja")); !strings.Contains(ja, "page.html、schema.json、example.yaml、themes/default.css") {
		t.Errorf("the mistake in Japanese = %s", ja)
	}
}

// An image of assets/ that is not right stops the drawing as a mistake of the template's assets/, as
// floe template check tells it, in the language of the messages.
func TestRenderBadAsset(t *testing.T) {
	t.Parallel()

	dir := sizes(t, map[string]string{"assets/logo.svg": "not svg"})
	file := filepath.Join(dir, "example.yaml")
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	d, err := tmpl.NewLibrary(filepath.Dir(dir), "").Read(file, src, tmpl.Themes{}, "")
	if err != nil {
		t.Fatal(err)
	}
	_, err = d.Render("t")
	var e *usererr.Error
	if !errors.As(err, &e) || e.Key != "tmpl_item" {
		t.Fatalf("Render = %v", err)
	}
	if ja := e.Localize(i18n.New("ja")); ja != filepath.Join(dir, "assets")+": 「logo.svg」の中身は、名前が示す image/svg+xml ではありません" {
		t.Errorf("the mistake in Japanese = %s", ja)
	}
}

// A file named assets is not the directory of assets.
func TestCheckAssetsFile(t *testing.T) {
	t.Parallel()

	dir := sizes(t, nil)
	if err := os.RemoveAll(filepath.Join(dir, "assets")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "assets"), nil, 0o600); err != nil { //nolint:gosec // G703: the test's own directory
		t.Fatal(err)
	}
	_, mistakes := tmpl.Check(dir, "")
	var e *usererr.Error
	if len(mistakes) != 1 || !errors.As(mistakes[0], &e) || e.Key != "tmpl_assets_not_dir" {
		t.Errorf("Check with a file named assets = %v", mistakes)
	}
}

// A path that is not a directory is not a template.
func TestCheckFile(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "sizes")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, mistakes := tmpl.Check(file, "")
	var e *usererr.Error
	if len(mistakes) != 1 || !errors.As(mistakes[0], &e) || e.Key != "tmpl_check_not_dir" {
		t.Errorf("Check of a file = %v", mistakes)
	}
}

// A schema that declares a key floe reads from every data file itself, as an item or only as required,
// is refused wherever the template is read: its item would always be empty, or always missing.
func TestSchemaOfSharedKeys(t *testing.T) {
	t.Parallel()

	dir := sizes(t, map[string]string{"schema.json": `"chart":|"title": { "type": "string" },` + "\n    " + `"chart":`})
	if err := os.WriteFile(filepath.Join(dir, "schema.json"), []byte(strings.Replace(mustRead(t, filepath.Join(dir, "schema.json")),
		`"required": ["files"]`, `"required": ["files", "lang"]`, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := tmpl.NewLibrary(filepath.Dir(dir), "").Read("d.yaml", []byte("template: sizes\nlang: en\nfiles: []\n"), tmpl.Themes{}, "")
	if err == nil || !strings.Contains(err.Error(), "sizes/schema.json declares or requires lang, title, which floe reads") {
		t.Errorf("Read with a template whose schema declares lang and title = %v", err)
	}
}

func mustRead(t *testing.T, file string) string {
	t.Helper()

	b, err := os.ReadFile(file) //nolint:gosec // G304: the test's own file
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// sizes copies testdata/sizes into a directory sizes, with files written over: a whole file, or old|new
// for old to be replaced with new.
func sizes(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "sizes")
	if err := os.CopyFS(dir, os.DirFS("testdata/sizes")); err != nil {
		t.Fatal(err)
	}
	for name, src := range files {
		at := filepath.Join(dir, name)
		if old, repl, ok := strings.Cut(src, "|"); ok {
			b, err := os.ReadFile(at)
			if err != nil || !strings.Contains(string(b), old) {
				t.Fatalf("%s has no %s: %v", name, old, err)
			}
			src = strings.Replace(string(b), old, repl, 1)
		}
		if err := os.WriteFile(at, []byte(src), 0o600); err != nil { //nolint:gosec // G703: the test's own directory
			t.Fatal(err)
		}
	}
	return dir
}
