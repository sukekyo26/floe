package tmpl_test

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/tmpl"
)

// mapFS is files, by their paths, as an fs.FS.
func mapFS(files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, text := range files {
		fsys[name] = &fstest.MapFile{Data: []byte(text)}
	}
	return fsys
}

// ownKit reads parts and templates of one's own, by their paths under parts/ and templates/, over floe's.
func ownKit(t *testing.T, parts, templates map[string]string) *kit.Kit {
	t.Helper()
	src := kit.Sources{Parts: kit.Dir{FS: nil, Name: ""}, Themes: kit.Dir{FS: nil, Name: ""}, Templates: kit.Dir{FS: nil, Name: ""}}
	if parts != nil {
		src.Parts = kit.Dir{FS: mapFS(parts), Name: "parts"}
	}
	if templates != nil {
		src.Templates = kit.Dir{FS: mapFS(templates), Name: "templates"}
	}
	k, err := kit.Read(floeKit(t), src)
	if err != nil {
		t.Fatalf("kit.Read: %s", told(err))
	}
	return k
}

// drawn draws src, the data file d.yaml, with k, and returns what the command line tells of its mistakes, ""
// for none.
func drawn(t *testing.T, k *kit.Kit, src string, o tmpl.Options) string {
	t.Helper()
	if _, err := tmpl.Draw(k, "d.yaml", []byte(src), o); err != nil {
		return told(err)
	}
	return ""
}

// mine is a template of one's own: page.html and schema.json, with the files more gives.
func mine(page, schema string, more map[string]string) map[string]string {
	files := map[string]string{"mine/page.html": page, "mine/schema.json": schema}
	for name, text := range more {
		files["mine/"+name] = text
	}
	return files
}

const minePage = "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head>\n<body>\n{{with .Data.note}}{{.}}{{end}}\n</body></html>\n"

const mineSchema = `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"n": {}, "a": {}, "items": {"type": "array"}, "note": {"$ref": "floe:///parts/note.json"}}}`

func lines(l ...string) string { return strings.Join(l, "\n") }

var none = tmpl.Options{Theme: "", Untitled: ""}

// The template's checks, the parts' checks and the computed values run together: none uses what another gives.
func TestStageFour(t *testing.T) {
	t.Parallel()

	report := "lang: ja\ntemplate: report\ntitle: t\nsections:\n  - heading: a\n    parts:\n" +
		"      - table: {columns: [{title: x}], rows: [[a, b]]}\n" +
		"      - table: {columns: [{title: x}], rows: [[a, b]]}\n" +
		"      - progress: [{label: p, done: 5, of: 3}]\n" +
		"      - text: \"```\\ncode\\n```\"\n"
	if got, want := drawn(t, floeKit(t), report, none), lines(
		"d.yaml:7:46: write as many cells in each row as there are columns",
		"d.yaml:8:46: write as many cells in each row as there are columns",
		`d.yaml:9:37: write done no greater than of, the whole ("5")`,
		"d.yaml:10:15: write code in a code part and commands in a terminal part, which show them under their file or prompt",
	); got != want {
		t.Errorf("report told\n%s\nwant\n%s", got, want)
	}

	k := ownKit(t, nil, mine(minePage, mineSchema, map[string]string{
		"checks.yaml":   "- expr: len(doc.items) > 0\n  message: write an item\n",
		"computed.yaml": "less: n - 1\n",
	}))
	if got, want := drawn(t, k, "lang: en\ntemplate: mine\nn: abc\nitems: []\n", none), lines(
		"d.yaml: write an item",
		"templates/mine/computed.yaml:1:9: less cannot be computed from d.yaml: invalid operation: string - int",
	); got != want {
		t.Errorf("own told\n%s\nwant\n%s", got, want)
	}
}

// The template and the theme a data file names are told together with the mistakes of its shared keys; a theme
// given is told at no place, as no line of the data names it, and draws the page instead of the data's.
func TestNames(t *testing.T) {
	t.Parallel()

	k := floeKit(t)
	themes := "blueprint, brutal, default, editorial, ink, notebook, soft, swiss, washi"
	for _, tt := range []struct {
		name, data string
		o          tmpl.Options
		want       string
	}{
		{"unknown template", "lang: ja\ntemplate: reprot\n", none, `d.yaml:2:11: unknown template "reprot"; use report`},
		{"no template", "lang: ja\n", none, "d.yaml:1:1: name the template at the top level, like template: report (templates: report)"},
		{"template not text", "lang: ja\ntemplate: 5\n", none, "d.yaml:2:11: template must be text"},
		{"unknown theme", "lang: ja\ntemplate: report\ntheme: nope\n", none, `d.yaml:3:8: there is no theme "nope"; use ` + themes},
		{
			"no lang and an unknown template", "template: reprot\ntitle: t\n", none,
			lines("d.yaml:1:1: name the language of the page at the top level, like lang: en (a BCP 47 tag such as en, ja or pt-BR)", `d.yaml:1:11: unknown template "reprot"; use report`),
		},
		{"a theme given", "lang: ja\ntemplate: report\ntheme: 5\n", tmpl.Options{Theme: "ink", Untitled: ""}, ""},
		{"a theme given that is none", "lang: ja\ntemplate: report\ntheme: ink\n", tmpl.Options{Theme: "nope", Untitled: ""}, `floe: there is no theme "nope"; use ` + themes},
	} {
		if got := drawn(t, k, tt.data, tt.o); got != tt.want {
			t.Errorf("%s: told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
	}

	// The checks, the computed values and page.html see the theme the page is drawn with.
	k = ownKit(t, nil, mine(minePage, mineSchema, map[string]string{
		"checks.yaml": "- expr: doc.theme == \"ink\"\n  message: draw it in ink\n",
	}))
	data := "lang: en\ntemplate: mine\ntheme: washi\n"
	if got := drawn(t, k, data, tmpl.Options{Theme: "ink", Untitled: ""}); got != "" {
		t.Errorf("drawn in ink, told %s", got)
	}
	if got := drawn(t, k, data, none); got != "d.yaml: draw it in ink" {
		t.Errorf("drawn in washi, told %s", got)
	}
}

// A part's checks run once for each item it draws, and a mistake told the same is told once.
func TestPartChecksOnce(t *testing.T) {
	t.Parallel()

	k := ownKit(t, map[string]string{
		"box/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
		"box/part.html":   `<div data-floe-part="box"></div>`,
		"box/part.css":    "",
		"box/checks.yaml": "- for: rows\n  expr: true\n  message: m\n",
	}, mine(minePage, `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"boxes": {"type": "array", "items": {"$ref": "floe:///parts/box.json"}}}}`, nil))
	got := drawn(t, k, "lang: en\ntemplate: mine\nboxes: [{rows: 1}, {rows: 2}, {rows: 3}]\n", none)
	if want := "parts/box/checks.yaml:1:8: for: rows does not lead to a list in d.yaml"; got != want {
		t.Errorf("told\n%s\nwant\n%s", got, want)
	}
}

// A mistake of drawing is told where it is: in page.html at its line and column, as textpos counts them, the
// usererr a function gave or the library's words without their head; of a part, at the value it draws.
func TestDrawMistakes(t *testing.T) {
	t.Parallel()

	page := func(body string) string {
		return "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>t</title></head>\n<body>\n" + body + "\n</body></html>\n"
	}
	for _, tt := range []struct{ name, page, want string }{
		{"num", page("<p>{{.Data.n | num}}</p>"), "templates/mine/page.html:4:16: num takes a number, not abc (string)"},
		{"no key", page("あいう{{.Data.x.y}}"), `templates/mine/page.html:4:11: executing "page.html" at <.Data.x.y>: map has no entry for key "x"`},
		{"in a define", "{{define \"x\"}}\n\t{{num .}}{{end}}" + page("{{template \"x\" .Data.n}}"), `templates/mine/page.html:2:4: num takes a number, not abc (string)`},
	} {
		k := ownKit(t, nil, mine(tt.page, mineSchema, nil))
		if got := drawn(t, k, "lang: en\ntemplate: mine\nn: abc\n", none); got != tt.want {
			t.Errorf("%s: told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
	}

	k := ownKit(t, map[string]string{
		"big/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
		"big/part.html":   `<div data-floe-part="big">{{num .n}}</div>`,
		"big/part.css":    "",
	}, mine(minePage, `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"bigs": {"type": "array", "items": {"$ref": "floe:///parts/big.json"}}}}`, nil))
	got := drawn(t, k, "lang: en\ntemplate: mine\nbigs:\n  - n: abc\n  - n: xyz\n", none)
	if want := lines("d.yaml:4:5: num takes a number, not abc (string)", "d.yaml:5:5: num takes a number, not xyz (string)"); got != want {
		t.Errorf("a part told\n%s\nwant\n%s", got, want)
	}
}

// The page drawn is read as a browser reads it, and refused, at the data's template:, for what floe puts into
// it: a <style>, a <base>, a name that is no part or one look's box or not one part's, and no </head> where a
// <style> styles the page. What is not drawn is not refused.
func TestDrawnPage(t *testing.T) {
	t.Parallel()

	page := func(head, body string) string {
		return "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>t</title>" + head + "</head>\n<body>\n" + body + "\n</body></html>\n"
	}
	several := func(v string) string {
		return `d.yaml:2:11: the page has data-floe-part="` + v + `", which is not the name of one part: write data-floe-part="<name>", as written, and take the look of others with data-floe-look="<name> …"`
	}
	for _, tt := range []struct{ name, page, want string }{
		{"two names", page("", `<div data-floe-part="note table">x</div>`), several("note table")},
		{"spaces", page("", `<div data-floe-part=" note ">x</div>`), several(" note ")},
		{"a look's box", page("", `<div data-floe-part="panel">x</div>`), `d.yaml:2:11: the page has data-floe-part="panel", but panel is a look, which has no box: take it with data-floe-look="panel"`},
		{"no such part", page("", `<div data-floe-part="nope">x</div>`), "d.yaml:2:11: the page drawn from this file names the part nope, which there is none of; use "},
		{"base", page(`<base href="x">`, ""), "d.yaml:2:11: the page drawn from this file has a <base>, which page.html or a part writes and the page's CSP ignores (base-uri 'none'): write its links whole"},
		{"base not drawn", page(`{{if false}}<base href="x">{{end}}<!-- <base href="x"> -->`, ""), ""},
		{"style in svg", page("", `<svg><style>a{}</style></svg>`), "d.yaml:2:11: the page drawn from this file has a <style>, which page.html or a part writes: write the CSS in the template's page.css, which floe puts into the page in a layer under the parts' and the theme's, or in the part's part.css"},
		{"names as a browser reads them", page("", `<DIV DATA-FLOE-PART="note">a</DIV><div data-floe-part=note>b</div><div data-floe-part="&#110;ote">c</div>`), ""},
		{"a name in a script's text", page("", `<script>let x = 'data-floe-look="nope"'</script>`), ""},
		{"no </head> drawn", "<!doctype html>\n<html><head><title>t</title>{{if .Data.n}}</head>{{end}}<body></body></html>", "d.yaml:2:11: the page drawn from this file has no </head> where a <style> styles the page, before which floe puts the page's styles: page.html writes it in the head, outside {{if}}, {{with}}, {{range}}, <template>, <svg> and <math>, and nothing drawn before it, a part among them, leaves an element open"},
		{"</head> in a comment", "<!doctype html>\n<html><head><title>t</title><!-- </head> --></head><body></body></html>", ""},
	} {
		k := ownKit(t, nil, mine(tt.page, mineSchema, nil))
		got := drawn(t, k, "lang: en\ntemplate: mine\n", none)
		if strings.HasSuffix(tt.want, "; use ") {
			got, _, _ = strings.Cut(got, "; use ")
			got += "; use "
		}
		if got != tt.want {
			t.Errorf("%s: told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
	}
}
