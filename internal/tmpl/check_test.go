package tmpl_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

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
	l := tmpl.NewLibrary("testdata", "", "")
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

// A right template passes, warned that it does not name the floe it needs.
func TestCheck(t *testing.T) {
	t.Parallel()

	warnings, mistakes := tmpl.Check("testdata/sizes", "", "")
	if len(warnings) != 1 || len(mistakes) != 0 {
		t.Fatalf("Check = %v, %v", warnings, mistakes)
	}
	release := strings.SplitN(version.Version, ".", 3)
	if want := "testdata/sizes has no template.yaml: write floe: " + release[0] + "." + release[1] + " in it"; !strings.HasPrefix(warnings[0].Error(), want) {
		t.Errorf("the warning = %s, want %s", warnings[0], want)
	}

	// With template.yaml, nothing is told.
	dir := sizes(t, map[string]string{"template.yaml": "floe: " + release[0] + "." + release[1] + "\n"})
	if warnings, mistakes := tmpl.Check(dir, "", ""); len(warnings) != 0 || len(mistakes) != 0 {
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

			warnings, mistakes := tmpl.Check(sizes(t, tt.files), "", "")
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
	if _, mistakes := tmpl.Check(dir, "", ""); len(mistakes) != 1 || !strings.HasPrefix(mistakes[0].Error(), want) {
		t.Errorf("Check = %v, want %s", mistakes, want)
	}
}

// The template's CSS names the page's root as :scope, past a comment and values that name it otherwise, and
// page.html names parts floe has, past a name it puts together, an element the box of one, a mistake told
// at the line of the name.
func TestTemplateCSSAndParts(t *testing.T) {
	t.Parallel()

	dir := sizes(t, map[string]string{
		"page.css":           "/* not :root */\n:scope { --fg: #000; }\nb { background: url(html.svg); animation: html 1s; }\n:root .lead, a { color: red; }\n",
		"themes/default.css": "body { font-family: sans-serif; }\nhtml{ color: red; }\n",
		"page.html": "<body>|<body><span\n" + `data-floe-look="cell nope"></span><b data-floe-part="cell note"></b>` +
			`<u data-floe-part=" cell"></u><s data-floe-part="panel"></s><i data-floe-part="{{"cell"}}"></i>`,
	})
	_, mistakes := tmpl.Check(dir, "", "")
	got := make([]string, 0, len(mistakes))
	for _, m := range mistakes {
		got = append(got, strings.TrimPrefix(m.Error(), dir+string(filepath.Separator)))
	}
	want := []string{
		"page.css:4: write the root as :scope, not :root",
		filepath.Join("themes", "default.css") + ":2: write the root as :scope, not html",
		"page.html:5: page.html names the part nope, which there is none of; use bars, ",
		`page.html:5: data-floe-part="cell note" is not the name of one part`,
		`page.html:5: data-floe-part=" cell" is not the name of one part`,
		"page.html:5: data-floe-part names the look panel, which has no box",
	}
	if len(got) != len(want) {
		t.Fatalf("Check = %q, want %q", got, want)
	}
	for i := range want {
		if !strings.HasPrefix(got[i], want[i]) {
			t.Errorf("Check = %q, want %q", got[i], want[i])
		}
	}
}

// An @import of the template's CSS, which a browser does not read in floe's layer, is refused at its
// line by floe template check and by the drawing, past comments that name it or hold more than ASCII,
// and one the file ends without its ; too.
func TestCSSImportRefused(t *testing.T) {
	t.Parallel()

	dir := sizes(t, map[string]string{
		"page.css":           "/* 色の定義です。\n   @import は書かない */\nbody { color: red; }\n@IMPORT url(x.css);\n",
		"themes/default.css": "a { color: red; }\n@import url(y.css)",
	})
	want := []string{
		filepath.Join(dir, "page.css") + ":4: floe puts this CSS in a layer, where a browser reads no @import",
		filepath.Join(dir, "themes", "default.css") + ":2: floe puts this CSS in a layer",
	}
	_, mistakes := tmpl.Check(dir, "", "")
	if len(mistakes) != len(want) || !strings.HasPrefix(mistakes[0].Error(), want[0]) || !strings.HasPrefix(mistakes[1].Error(), want[1]) {
		t.Errorf("Check = %v, want %q", mistakes, want)
	}
	if _, err := draw(tmpl.NewLibrary(filepath.Dir(dir), "", ""), "template: sizes\nlang: en\nfiles: [{ name: a.txt, bytes: 1 }]\n"); err == nil || !strings.HasPrefix(err.Error(), want[0]) {
		t.Errorf("drawing = %v, want %s", err, want[0])
	}
}

// A } of the template's CSS that closes no block, which would close floe's layer around it, is refused at
// its line, past braces and an @import in strings and comments, and a /* in a string, which starts none.
func TestCSSBraceRefused(t *testing.T) {
	t.Parallel()

	dir := sizes(t, map[string]string{
		"page.css": "a::before { content: \"} @import x; /*\"; }\n@media print { p { color: red; } }\n}\n/* } */\nb { color: red; }\n",
	})
	want := filepath.Join(dir, "page.css") + ":3: this } closes no block"
	if _, mistakes := tmpl.Check(dir, "", ""); len(mistakes) != 1 || !strings.HasPrefix(mistakes[0].Error(), want) {
		t.Errorf("Check = %v, want %s", mistakes, want)
	}
	// The URL of url( runs to the first ), whatever it holds, as a browser reads it.
	url := sizes(t, map[string]string{"page.css": "a { b: url(x(y) } }\nc { color: red; }\n"})
	if _, mistakes := tmpl.Check(url, "", ""); len(mistakes) != 1 ||
		!strings.HasPrefix(mistakes[0].Error(), filepath.Join(url, "page.css")+":1: this } closes no block") {
		t.Errorf("Check of a } after url( = %v", mistakes)
	}
	// :root is told at its line, past the comments before it.
	root := sizes(t, map[string]string{"page.css": ".a, /* one\ntwo\nthree */ :root { color: red; }\n"})
	if _, mistakes := tmpl.Check(root, "", ""); len(mistakes) != 1 || !strings.Contains(mistakes[0].Error(), "page.css:3:") {
		t.Errorf("Check of :root after a comment = %v", mistakes)
	}
	// An escaped bracket is a name's, and leaves the } after it to be told.
	escaped := sizes(t, map[string]string{"page.css": ".a\\( { color: red; }\n}\n"})
	if _, mistakes := tmpl.Check(escaped, "", ""); len(mistakes) != 1 ||
		!strings.HasPrefix(mistakes[0].Error(), filepath.Join(escaped, "page.css")+":2: this } closes no block") {
		t.Errorf("Check of an escaped bracket = %v", mistakes)
	}
	// A block left open ends with the template's own <style>.
	ok := sizes(t, map[string]string{"page.css": "@media print { p { color: red; }\n"})
	if warnings, mistakes := tmpl.Check(ok, "", ""); len(mistakes) != 0 {
		t.Errorf("Check of a block left open = %v, %v", warnings, mistakes)
	}
}

// A </style of the template's CSS, in a comment or a string, is written <\/style: it closes no <style>,
// and what follows it stays CSS.
func TestCSSStyleEndEscaped(t *testing.T) {
	t.Parallel()

	dir := sizes(t, map[string]string{
		"page.css":           "/* </STYLE><b id=\"x\">x</b><style> */\n",
		"themes/default.css": "a::before { content: \"</style>\"; }\n",
	})
	if warnings, mistakes := tmpl.Check(dir, "", ""); len(mistakes) != 0 {
		t.Errorf("Check = %v, %v", warnings, mistakes)
	}
	page, err := draw(tmpl.NewLibrary(filepath.Dir(dir), "", ""), "template: sizes\nlang: en\nfiles: [{ name: a.txt, bytes: 1 }]\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page, `/* <\/STYLE><b id="x">x</b><style> */`) || !strings.Contains(page, `content: "<\/style>"`) {
		t.Errorf("drawn page = %s", page)
	}
}

// A colour that changes with the mode, set on :scope for light mode alone, is warned of at the file that
// sets it: over every palette, it would stay in dark mode too.
func TestCheckScopeColours(t *testing.T) {
	t.Parallel()

	release := strings.SplitN(version.Version, ".", 3)
	floe := "floe: " + release[0] + "." + release[1] + "\n"
	for page, want := range map[string]string{
		":scope { --surface: #fff; --accent: #c00; --page-width: 90rem; --rule: 2px solid var(--fg); }":    "sizes/page.css sets --accent, --surface on :scope for light mode alone",
		":scope { --surface: #fff; }\n@media (prefers-color-scheme: dark) { :scope { --surface: #222; } }": "",
		"body { --surface: #fff; }": "",
		"@media (prefers-color-scheme: dark) { :scope { --bg: #000; } }\n:scope { --bg: #fff; }":                                         "sizes/page.css sets --bg on :scope for light mode alone",
		":scope { --line: #fff; &.x { --q: 1; } --fg: #000; }\n@media (prefers-color-scheme: dark) { :scope { --line: #222; } }":         "sizes/page.css sets --fg on :scope",
		":scope { --bg: #fff; }\n@media screen and (prefers-color-scheme: dark) { :scope { --bg: #000; } }":                              "",
		":scope { --bg: #fff; }\n@media not (prefers-color-scheme: dark) { :scope { --bg: #eee; } }":                                     "sizes/page.css sets --bg on :scope",
		":scope { --bg: #fff; @media (prefers-color-scheme: dark) { --bg: #000; } }":                                                     "",
		"@layer a { :scope { --bg: #fff; } }\n@supports (color: red) { @media (prefers-color-scheme: dark) { :scope { --bg: #000; } } }": "",
		":scope { @media (prefers-color-scheme: dark) { --bg: #000; } --bg: #fff; }":                                                     "sizes/page.css sets --bg on :scope",
	} {
		warnings, mistakes := tmpl.Check(sizes(t, map[string]string{"template.yaml": floe, "page.css": page}), "", "")
		if len(mistakes) != 0 || want == "" && len(warnings) != 0 ||
			want != "" && (len(warnings) != 1 || !strings.Contains(warnings[0].Error(), want)) {
			t.Errorf("Check with %s = %v, %v, want %q", page, warnings, mistakes, want)
		}
	}
}

// page.css is told once, of what it sets for light mode alone with any theme, and with none when every
// theme is refused.
func TestCheckScopeColoursOnce(t *testing.T) {
	t.Parallel()

	release := strings.SplitN(version.Version, ".", 3)
	floe := "floe: " + release[0] + "." + release[1] + "\n"
	dir := sizes(t, map[string]string{
		"template.yaml": floe, "page.css": ":scope { --bg: #fff; --fg: #000; }\n",
		"themes/default.css": "@media (prefers-color-scheme: dark) { :scope { --bg: #000; } }\n", "themes/print.css": "",
	})
	warnings, mistakes := tmpl.Check(dir, "", "")
	if len(mistakes) != 0 || len(warnings) != 1 || !strings.Contains(warnings[0].Error(), "sizes/page.css sets --bg, --fg on :scope") {
		t.Errorf("Check of two themes = %v, %v", warnings, mistakes)
	}
	refused := sizes(t, map[string]string{
		"template.yaml": floe, "page.css": ":scope { --bg: #fff; }\n", "themes/default.css": ":root { --x: 1; }\n",
	})
	warnings, mistakes = tmpl.Check(refused, "", "")
	if len(mistakes) != 1 || len(warnings) != 1 || !strings.Contains(warnings[0].Error(), "sizes/page.css sets --bg on :scope") {
		t.Errorf("Check of a refused theme = %v, %v", warnings, mistakes)
	}
}

// A directory is checked as a template of its name, which must be one.
func TestCheckName(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "Sizes")
	if err := os.CopyFS(dir, os.DirFS("testdata/sizes")); err != nil {
		t.Fatal(err)
	}
	_, mistakes := tmpl.Check(dir, "", "")
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
	_, mistakes := tmpl.Check(dir, "", "")
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
	d, err := tmpl.NewLibrary(filepath.Dir(dir), "", "").Read(file, src, tmpl.Themes{}, "")
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
	_, mistakes := tmpl.Check(dir, "", "")
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
	_, mistakes := tmpl.Check(file, "", "")
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
	_, err := tmpl.NewLibrary(filepath.Dir(dir), "", "").Read("d.yaml", []byte("template: sizes\nlang: en\nfiles: []\n"), tmpl.Themes{}, "")
	if err == nil || !strings.Contains(err.Error(), "sizes/schema.json declares or requires lang, title, which floe reads") {
		t.Errorf("Read with a template whose schema declares lang and title = %v", err)
	}
}

// ReadFS reads the images from the fs.FS given, as a server reads those a publish sent, and tells the
// images the data names; without an fs.FS it fails.
func TestReadFS(t *testing.T) {
	t.Parallel()

	png := mustRead(t, "testdata/sizes/chart.png")
	l := tmpl.NewLibrary("testdata", "", "")
	src := []byte("template: sizes\nlang: en\nfiles: [{ name: a, bytes: 1 }]\nchart: img/chart.png\n")
	d, named, err := l.ReadFS("sizes.yaml", src, fstest.MapFS{"img/chart.png": {Data: []byte(png)}}, "/plans", tmpl.Themes{})
	if err != nil || !slices.Equal(named, []string{"img/chart.png"}) {
		t.Fatalf("ReadFS = %v, %v; want img/chart.png named", named, err)
	}
	if images, imgErr := d.Images(); imgErr != nil || string(images["img/chart.png"]) != png {
		t.Errorf("the images = %d, %v; want img/chart.png as sent", len(images), imgErr)
	}
	if _, _, err = l.ReadFS("sizes.yaml", src, nil, "/plans", tmpl.Themes{}); err == nil {
		t.Error("ReadFS without an fs.FS = nil error")
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

// A name of no part page.html names is told with those the attribute may name: a look has no box.
func TestCheckPageNames(t *testing.T) {
	t.Parallel()

	for page, want := range map[string]bool{`<b data-floe-part="pannel"></b>`: false, `<b data-floe-look="pannel"></b>`: true} {
		_, mistakes := tmpl.Check(sizes(t, map[string]string{"page.html": "<body>|<body>" + page}), "", "")
		if got := fmt.Sprint(mistakes); !strings.Contains(got, "options, progress") && !strings.Contains(got, "options, panel, progress") ||
			strings.Contains(got, "options, panel, progress") != want {
			t.Errorf("Check of %s = %v, want panel among the names %v", page, mistakes, want)
		}
	}
}
