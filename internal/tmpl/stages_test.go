package tmpl_test

import (
	"cmp"
	"strconv"
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
	k, err := kit.Load(floeKit(t), src)
	if err != nil {
		t.Fatalf("kit.Load: %s", told(err))
	}
	return k
}

// drawn draws src, the data file d.yaml, with k, and returns what the command line tells of its mistakes, ""
// for none.
func drawn(t *testing.T, k *kit.Kit, src string, o tmpl.Options) string {
	t.Helper()
	if _, _, err := tmpl.Draw(k, "d.yaml", []byte(src), o); err != nil {
		return told(err)
	}
	return ""
}

// mine is a template of one's own: page.html, schema.json and an example.yaml, with the files more gives.
func mine(page, schema string, more map[string]string) map[string]string {
	files := map[string]string{"mine/page.html": page, "mine/schema.json": schema, "mine/example.yaml": "lang: en\ntemplate: mine\n"}
	for name, text := range more {
		files["mine/"+name] = text
	}
	return files
}

const minePage = "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head>\n<body>\n{{with .Data.note}}{{.}}{{end}}\n</body></html>\n"

const mineSchema = `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"n": {}, "a": {}, "items": {"type": "array"}, "note": {"$ref": "floe:///parts/note.json"}}}`

func lines(l ...string) string { return strings.Join(l, "\n") }

var none = tmpl.Options{Theme: "", Untitled: "", ImagesDir: ""}

// The template's checks, the parts' checks and the computed values run together: none uses what another gives.
func TestStageFour(t *testing.T) {
	t.Parallel()

	report := "lang: ja\ntemplate: report\ntitle: t\nsections:\n  - heading: a\n    parts:\n" +
		"      - table: {columns: [{title: x}], rows: [[a, b]]}\n" +
		"      - table: {columns: [{title: x}], rows: [[a, b]]}\n" +
		"      - progress: [{label: p, done: 5, of: 3}]\n"
	if got, want := drawn(t, floeKit(t), report, none), lines(
		"d.yaml:7:46: write as many cells in each row as there are columns",
		"d.yaml:8:46: write as many cells in each row as there are columns",
		`d.yaml:9:37: write done no greater than of, the whole ("5")`,
	); got != want {
		t.Errorf("report told\n%s\nwant\n%s", got, want)
	}

	schema := `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"n": {}, "items": {"type": "array"}, "t": {"$ref": "floe:///parts/table.json"}}}`
	k := ownKit(t, nil, mine(minePage, schema, map[string]string{
		"checks.yaml":   "- expr: len(doc.items) > 0\n  message: write an item\n",
		"computed.yaml": "less: n - 1\n",
	}))
	data := "lang: en\ntemplate: mine\nn: abc\nitems: []\nt: {columns: [{title: x}], rows: [[a, b]]}\n"
	if got, want := drawn(t, k, data, none), lines(
		"d.yaml: write an item",
		"d.yaml:5:34: write as many cells in each row as there are columns",
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
	themes := "aurora, blueprint, brutal, default, editorial, ink, notebook, retro, soft, swiss, washi"
	for _, tt := range []struct {
		name, data string
		o          tmpl.Options
		want       string
	}{
		{"unknown template", "lang: ja\ntemplate: reprot\n", none, `d.yaml:2:11: unknown template "reprot"; use adr, design, handoff, postmortem, report, runbook, slides`},
		{"no template", "lang: ja\n", none, "d.yaml:1:1: name the template at the top level, like template: report (templates: adr, design, handoff, postmortem, report, runbook, slides)"},
		{"template not text", "lang: ja\ntemplate: 5\n", none, "d.yaml:2:11: template must be text"},
		{"template null", "lang: ja\ntemplate: ~\n", none, "d.yaml:2:11: name the template at the top level, like template: report (templates: adr, design, handoff, postmortem, report, runbook, slides)"},
		// Empty, as null, names none: the template is told missing, and the theme is the default.
		{"template empty", "lang: ja\ntemplate: ''\n", none, "d.yaml:2:11: name the template at the top level, like template: report (templates: adr, design, handoff, postmortem, report, runbook, slides)"},
		{"theme empty", "lang: ja\ntemplate: report\ntheme: \"\"\n", none, ""},
		{"template of spaces", "lang: ja\ntemplate: ' '\n", none, "d.yaml:2:11: name the template at the top level, like template: report (templates: adr, design, handoff, postmortem, report, runbook, slides)"},
		{"theme of spaces", "lang: ja\ntemplate: report\ntheme: ' '\n", none, ""},
		{"a theme with spaces", "lang: ja\ntemplate: report\ntheme: ' ink '\n", none, ""},
		{"unknown theme", "lang: ja\ntemplate: report\ntheme: nope\n", none, `d.yaml:3:8: there is no theme "nope"; use ` + themes},
		{
			"no lang and an unknown template", "template: reprot\ntitle: t\n", none,
			lines("d.yaml:1:1: name the language of the page at the top level, like lang: en (a BCP 47 tag such as en, ja or pt-BR)", `d.yaml:1:11: unknown template "reprot"; use adr, design, handoff, postmortem, report, runbook, slides`),
		},
		{"a theme given", "lang: ja\ntemplate: report\ntheme: 5\n", tmpl.Options{Theme: "ink", Untitled: "", ImagesDir: ""}, ""},
		{"a theme given with spaces", "lang: ja\ntemplate: report\n", tmpl.Options{Theme: " ink ", Untitled: "", ImagesDir: ""}, ""},
		{"spaces given", "lang: ja\ntemplate: report\ntheme: nope\n", tmpl.Options{Theme: " ", Untitled: "", ImagesDir: ""}, `d.yaml:3:8: there is no theme "nope"; use ` + themes},
		{"a theme given that is none", "lang: ja\ntemplate: report\ntheme: ink\n", tmpl.Options{Theme: "nope", Untitled: "", ImagesDir: ""}, `floe: there is no theme "nope"; use ` + themes},
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
	if got := drawn(t, k, data, tmpl.Options{Theme: "ink", Untitled: "", ImagesDir: ""}); got != "" {
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
		{"num", page("<p>{{.Data.n | num}}</p>"), `templates/mine/page.html:4:16: num takes a number, not "abc"`},
		{"no key", page("あいう{{.Data.x.y}}"), `templates/mine/page.html:4:11: executing "page.html" at <.Data.x.y>: map has no entry for key "x"`},
		{"in a define", "{{define \"x\"}}\n\t{{num .}}{{end}}" + page("{{template \"x\" .Data.n}}"), `templates/mine/page.html:2:4: num takes a number, not "abc"`},
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
	if want := lines(`d.yaml:4:5: num takes a number, not "abc"`, `d.yaml:5:5: num takes a number, not "xyz"`); got != want {
		t.Errorf("a part told\n%s\nwant\n%s", got, want)
	}
}

// The page drawn is read as a browser reads it, and refused, at the data's template:, for what floe puts into
// it: a <style>, a <base>, a name that is no part or one look's box or not one part's. What is not drawn is
// not refused; a page without </head> gets its styles in the head the parser makes.
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
		{"base", page(`<base href="x">`, ""), "d.yaml:2:11: the page drawn from this file has a <base>, which page.html, a part's part.html or an item of format html writes and the page's CSP ignores (base-uri 'none'): write its links whole"},
		{"base not drawn", page(`{{if false}}<base href="x">{{end}}<!-- <base href="x"> -->`, ""), ""},
		{"style in svg", page("", `<svg><style>a{}</style></svg>`), "d.yaml:2:11: the page drawn from this file has a <style>, which page.html, a part's part.html or an item of format html writes: write the CSS in the template's page.css, which floe puts into the page in a layer under the parts' and the theme's, or in the part's part.css"},
		{"names as a browser reads them", page("", `<DIV DATA-FLOE-PART="note">a</DIV><div data-floe-part=note>b</div><div data-floe-part="&#110;ote">c</div>`), ""},
		{"a name in a script's text", page("", `<script>let x = 'data-floe-look="nope"'</script>`), ""},
		{"no </head> written", "<!doctype html>\n<html><title>t</title><body></body></html>", ""},
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

// A template or a part of one's own that cannot be used stops the Kit it is read in, every reason told
// together: nothing is drawn with it.
func TestLoadStops(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		src   kit.Sources
		wants []string
	}{
		{"templates", kit.Sources{
			Parts: kit.Dir{FS: nil, Name: ""}, Themes: kit.Dir{FS: nil, Name: ""},
			Templates: kit.Dir{FS: mapFS(map[string]string{
				"nopage/schema.json":  mineSchema,
				"escape/schema.json":  mineSchema,
				"escape/page.html":    "<!doctype html><html><head></head><body><a {{if .Data.n}}href{{end}}>x</a></body></html>",
				"escape/example.yaml": "lang: en\ntemplate: escape\n",
				"nopage/example.yaml": "lang: en\ntemplate: nopage\n",
			}), Name: "templates"},
		}, []string{
			"templates/escape/page.html:1:49: {{if}} branches end in different contexts",
			"templates/nopage: page.html is missing: a template has page.html, schema.json and example.yaml",
		}},
		{"a part", kit.Sources{
			Parts: kit.Dir{FS: mapFS(map[string]string{
				"bare/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
				"bare/part.css":    "",
			}), Name: "parts"},
			Themes: kit.Dir{FS: nil, Name: ""}, Templates: kit.Dir{FS: nil, Name: ""},
		}, []string{"parts/bare: a part holds schema.json, part.html and part.css"}},
	} {
		k, err := kit.Load(floeKit(t), tt.src)
		got := told(err)
		for _, want := range tt.wants {
			if k != nil || !strings.Contains(got, want) {
				t.Errorf("%s: Load = %v, told\n%s\nwant a line with %q", tt.name, k, got, want)
			}
		}
	}
}

// An empty data-floe-part, whose boundary would keep the parts' styles from what it holds, is refused, and an
// empty data-floe-look, which is no boundary, is not.
func TestNotDrawn(t *testing.T) {
	t.Parallel()

	page := "<!doctype html>\n<html><head><title>t</title></head><body>\n<div data-floe-part=\"\">a</div><p data-floe-look=\" \">b</p>\n</body></html>\n"
	k := ownKit(t, nil, mine(page, mineSchema, nil))
	want := `d.yaml:2:11: the page has data-floe-part="", which is not the name of one part: write data-floe-part="<name>", as written, and take the look of others with data-floe-look="<name> …"`
	if got := drawn(t, k, "lang: en\ntemplate: mine\n", none); got != want {
		t.Errorf("told\n%s\nwant\n%s", got, want)
	}
}

// A mistake a part's file names itself, a word its words.yaml has not in the page's language, is told where it
// says (the part's part.html, which draws the word), once whatever uses the part.
func TestPartWordsAtTheirFile(t *testing.T) {
	t.Parallel()

	k := ownKit(t, map[string]string{
		"w/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
		"w/part.html":   `<div data-floe-part="w">{{word "x"}}</div>`,
		"w/part.css":    "",
		"w/words.yaml":  "en: {x: X}\nja: {y: Y}\n",
	}, mine(minePage, `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"ws": {"type": "array", "items": {"$ref": "floe:///parts/w.json"}}}}`, nil))
	got := drawn(t, k, "lang: ja\ntemplate: mine\nws: [{}, {}]\n", none)
	if want := `parts/w/part.html:1:27: words.yaml has no word "x" in ja`; got != want {
		t.Errorf("told\n%s\nwant\n%s", got, want)
	}
}

// A heading with an empty id, which names nothing, is given one as one without.
func TestEmptyHeadingID(t *testing.T) {
	t.Parallel()

	page := "<!doctype html>\n<html><head><title>t</title></head><body><h2 id=\"\">Plan</h2></body></html>\n"
	out, _, err := tmpl.Draw(ownKit(t, nil, mine(page, mineSchema, nil)), "d.yaml", []byte("lang: en\ntemplate: mine\n"), none)
	if err != nil || !strings.Contains(string(out), `<h2 id="plan">Plan</h2>`) {
		t.Errorf("drawn %q, %v", out, err)
	}
}

// A theme's palette.css goes into the page as written, rules and !important too, in the layer floe.palette alone,
// which the other layers are over.
func TestPaletteInItsLayer(t *testing.T) {
	t.Parallel()

	const palette = ":root { --x: red !important }\nbody { color: blue }\n"
	k, err := kit.Load(floeKit(t), kit.Sources{
		Parts:     kit.Dir{FS: nil, Name: ""},
		Themes:    kit.Dir{FS: mapFS(map[string]string{"t/palette.css": palette}), Name: "themes"},
		Templates: kit.Dir{FS: mapFS(mine(minePage, mineSchema, nil)), Name: "templates"},
	})
	if err != nil {
		t.Fatalf("kit.Load: %s", told(err))
	}
	page, _, err := tmpl.Draw(k, "d.yaml", []byte("lang: en\ntemplate: mine\ntheme: t\n"), none)
	if err != nil {
		t.Fatal(told(err))
	}
	if html := string(page); !strings.Contains(html, "@layer floe.palette {\n"+palette+"}\n") || strings.Count(html, "body { color: blue }") != 1 {
		t.Errorf("the palette is not in floe.palette alone:\n%s", html)
	}
}

// A computed value named in any letters both expr and page.html read is drawn by its name.
func TestComputedNamedInJapanese(t *testing.T) {
	t.Parallel()

	page := strings.Replace(minePage, "{{with .Data.note}}{{.}}{{end}}", "<p>{{.Calc.合計}}</p>", 1)
	k := ownKit(t, nil, mine(page, mineSchema, map[string]string{"computed.yaml": "合計: 1 + 2\n"}))
	got, _, err := tmpl.Draw(k, "d.yaml", []byte("lang: en\ntemplate: mine\n"), none)
	if err != nil || !strings.Contains(string(got), "<p>3</p>") {
		t.Errorf("drew %s, %v; want <p>3</p>", got, told(err))
	}
}

// A word page.html names that its language has not is told at page.html's line and column, as a mistake of
// page.html made with no file is.
func TestPageWordsAtTheirLine(t *testing.T) {
	t.Parallel()

	page := strings.Replace(minePage, "{{with .Data.note}}{{.}}{{end}}", `<p>{{word "zz"}}</p>`, 1)
	k := ownKit(t, nil, mine(page, mineSchema, map[string]string{"words.yaml": "en: {a: A}\n"}))
	if got, want := drawn(t, k, "lang: en\ntemplate: mine\n", none), `templates/mine/page.html:4:6: words.yaml has no word "zz" in en`; got != want {
		t.Errorf("told %q, want %q", got, want)
	}
}

// A page x/net/html leaves the rest of out, past a <template> in an <svg> or a <math>, is refused at the data's
// template:, whether page.html or a part writes it; a <template> elsewhere, and a page ending in text the
// template does not follow, are drawn.
func TestPageCutShort(t *testing.T) {
	t.Parallel()

	const cut = "d.yaml:2:11: the page drawn was cut short: HTML's reader leaves out the rest of a page past a <template> inside an <svg> or a <math>; take the <template> out of it (page.html, a part.html or an item of format html)"
	part := map[string]string{
		"t/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
		"t/part.html":   `<div data-floe-part="t"><math><mi><template>x</template></mi></math></div>`,
		"t/part.css":    "",
	}
	const cutPage = "<svg><foreignObject><template>x</template></foreignObject></svg>"
	for _, tt := range []struct {
		name, body, h, want string
	}{
		{"foreignObject", "<svg><foreignObject><template>x</template></foreignObject></svg><p>after</p>", "", cut},
		// html/template and the parser read a <title> as text, the <template> too.
		{"svg's title", "<svg><title><template>x</template></title></svg><p>after</p>", "", ""},
		{"a part's math", "{{with .Data.t}}{{.}}{{end}}<p>after</p>", "", cut},
		{"a template outside", "<template>x</template><svg><text>t</text></svg><template>y</template><p>after</p>", "", ""},
		{"ending in xmp", "<xmp>x", "", ""},
		{"an item's HTML", "{{.Data.h}}<p>after</p>", "", cut},
		{"page.html ending in xmp past the cut", "<svg><foreignObject><template>x</template></foreignObject></svg><xmp>x", "", cut},
		{"page.html cut, then an item's open attribute", cutPage + "{{.Data.h}}<p>after</p>", `<b class="x>`, cut},
		{"page.html cut, then an item's xmp", cutPage + "{{.Data.h}}<p>after</p>", "<xmp>", cut},
		{"page.html cut, then an item's svg style", cutPage + "{{.Data.h}}<p>after</p>", "<svg><style>", cut},
		{"page.html ending in plaintext past the cut", "<svg><foreignObject><template>x</template></foreignObject></svg><plaintext>x", "", cut},
	} {
		schema := `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "properties": {"t": {"$ref": "floe:///parts/t.json"}, "h": {"type": "string", "format": "html"}}}`
		page := "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head>\n<body>\n" + tt.body + "\n</body></html>\n"
		k := ownKit(t, part, mine(page, schema, nil))
		h := cmp.Or(tt.h, "<svg><foreignObject><template>x</template></foreignObject></svg>")
		data := "lang: en\ntemplate: mine\nt: {}\nh: " + strconv.Quote(h) + "\n"
		out, _, err := tmpl.Draw(k, "d.yaml", []byte(data), none)
		got := ""
		if err != nil {
			got = told(err)
		}
		if got != tt.want {
			t.Errorf("%s: told %q, want %q", tt.name, got, tt.want)
		}
		if err == nil && strings.Contains(tt.body, "after") && !strings.Contains(string(out), "<p>after</p>") {
			t.Errorf("%s: the page drawn has no <p>after</p>", tt.name)
		}
	}
}
