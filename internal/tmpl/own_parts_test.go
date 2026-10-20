package tmpl_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

// ownParts writes parts of one's own, files by their path under the directory of parts, and returns it.
func ownParts(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// quote is a part of one's own, name, that draws a quote and who said it.
func quote(name string) map[string]string {
	return map[string]string{
		name + "/schema.json": `{"type": "object", "properties": {"text": {"type": "string"}, "source": {"type": "string"}}, ` +
			`"required": ["text"], "additionalProperties": false}`,
		name + "/part.html": `<figure data-floe-part="` + name + `"><blockquote>{{.text}}</blockquote>` +
			`{{with .source}}<figcaption class="source">{{word "by"}} {{.}}</figcaption>{{end}}</figure>`,
		name + "/words.yaml": "en: { by: by }\nja: { by: 出典 }\n",
		name + "/part.css":   "/* the " + name + " of one's own */\n:scope { margin: 0; }\n.source { color: var(--muted, #57635f); }\n",
	}
}

// A part of one's own is referred to as floe's are, drawn with its words in the page's language, and its
// style goes into the page once, in its scope; it holds a part by a $ref relative to its own schema too.
// One of the name of one of floe's takes its place whole, for the parts drawn in its look too. One takes
// the look of floe's by its part.yaml, the looks those take with it, and a look alone of one's own too.
func TestOwnParts(t *testing.T) {
	t.Parallel()

	files := quote("quote")
	for name, body := range quote("table") {
		files[name] = body
	}
	files["pilled/schema.json"] = `{"type": "object", "properties": {"v": {"$ref": "cell.json"}}}`
	files["pilled/part.html"] = `<p data-floe-part="pilled">{{.v}}</p>`
	files["pilled/part.css"] = ""
	parts := ownParts(t, files)
	pilled := tmpl.NewLibrary(memo(t, `"risks": {"$ref": "floe:///parts/pilled.json"}, "aside": {"type": "string"}`), parts, "")
	if page, err := draw(pilled, "template: memo\nlang: en\nrisks: { v: { pill: ok, tone: good } }\n"); err != nil ||
		!strings.Contains(page, `<p data-floe-part="pilled"><span class="good" data-floe-part="cell">ok</span></p>`) {
		t.Errorf("drawing a part that holds a cell by cell.json = %v:\n%s", err, page)
	}
	l := tmpl.NewLibrary(memo(t, `"risks": {"$ref": "floe:///parts/quote.json"}, "aside": {"$ref": "floe:///parts/table.json"}`), parts, "")
	page, err := draw(l, "template: memo\nlang: ja\nrisks: { text: Hi, source: Ann }\naside: { text: Rows }\n")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<figure data-floe-part="quote"><blockquote>Hi</blockquote><figcaption class="source">出典 Ann</figcaption></figure>`,
		`<figure data-floe-part="table"><blockquote>Rows</blockquote></figure>`,
		`@scope ([data-floe-part="quote"]) to (:scope [data-floe-part] > *) {` + "\n/* the quote of one's own */",
	} {
		if strings.Count(page, want) != 1 {
			t.Errorf("the page holds %s %d times:\n%s", want, strings.Count(page, want), page)
		}
	}
	// compare is drawn in the table's look: it gets the style of the table of one's own.
	l = tmpl.NewLibrary(memo(t, `"risks": {"$ref": "floe:///parts/compare.json"}, "aside": {"type": "string"}`), parts, "")
	page, err = draw(l, "template: memo\nlang: en\nrisks: { before: Was, after: Is, rows: [{ what: Host, before: a, after: b }] }\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page, "/* the table of one's own */") || strings.Contains(page, partStyle(t, "table")) {
		t.Errorf("compare is not drawn in the look of the table of one's own:\n%s", page)
	}
	// It takes no look of floe's then: the table of one's own has no part.yaml.
	if strings.Contains(page, partStyle(t, "panel")) {
		t.Errorf("compare takes the look of floe's panel through a table of one's own that does not:\n%s", page)
	}
	looked := quote("boxed")
	looked["boxed/part.yaml"] = "looks: [rows, shaded, 404]\n"
	looked["shaded/part.css"] = "/* a look of one's own */\n"
	looked["404/part.css"] = "/* a look named in digits */\n"
	l = tmpl.NewLibrary(memo(t, `"risks": {"$ref": "floe:///parts/boxed.json"}, "aside": {"type": "string"}`), ownParts(t, looked), "")
	if page, err = draw(l, "template: memo\nlang: en\nrisks: { text: Hi }\n"); err != nil {
		t.Fatal(err)
	}
	at := func(s string) int { return strings.Index(page, s) }
	for _, look := range []string{"panel", "rows"} {
		if want := `@scope ([data-floe-part="boxed"]` + partStyle(t, look); strings.Count(page, want) != 1 {
			t.Errorf("the page holds the look %s of boxed %d times, want once:\n%s", look, strings.Count(page, want), page)
		}
	}
	if order := []int{
		at(partStyle(t, "panel")), at(partStyle(t, "rows")), at("/* a look of one's own */"), at("/* a look named in digits */"),
		at("/* the boxed of one's own */"),
	}; !slices.IsSorted(order) || order[0] < 0 {
		t.Errorf("the looks and the part are at %v, want each look before what takes it:\n%s", order, page)
	}
}

// A part of one's own that cannot be used is refused, with why, to a template that uses it, by its
// schema or its page.html, and told as a warning to one that does not, which is drawn: one that takes the
// look of one left out too, or holds it, floe's among them, and one that holds itself, which floe would
// follow forever. One named as a part floe draws in Go or a schema the parts share leaves floe's in place, a
// symlink that leads nowhere too, and page.html may name floe's.
func TestBrokenOwnParts(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"unnamed/schema.json":  `{"type": "object"}`,
		"unnamed/part.html":    `<div class="x">{{.}}</div>`,
		"unnamed/part.css":     "",
		"brace/schema.json":    `{"type": "object"}`,
		"brace/part.html":      `<div data-floe-part="brace"></div>`,
		"brace/part.css":       ":scope { color: red; }\n}\n",
		"lender/schema.json":   `{"type": "object"}`,
		"lender/part.html":     `<div data-floe-part="lender"></div>`,
		"lender/part.css":      "",
		"lender/part.yaml":     "looks: [panel,\n  nope]\n",
		"first/schema.json":    `{"type": "object"}`,
		"first/part.html":      `<div data-floe-part="first"><b data-floe-part="cards first"></b></div>`,
		"first/part.css":       "",
		"spaced/schema.json":   `{"type": "object"}`,
		"spaced/part.html":     `<div data-floe-part="spaced "></div>`,
		"spaced/part.css":      "",
		"typo/schema.json":     `{"type": "object", "properties": {"c": {"$ref": "celll.json"}}}`,
		"typo/part.html":       `<div data-floe-part="typo"></div>`,
		"typo/part.css":        "",
		"borrower/schema.json": `{"type": "object"}`,
		"borrower/part.html":   `<div data-floe-part="borrower"></div>`,
		"borrower/part.css":    "",
		"borrower/part.yaml":   "looks: [unnamed]\n",
		"ring/part.css":        "",
		"ring/part.yaml":       "looks: [round]\n",
		"round/part.css":       "",
		"round/part.yaml":      "looks: [ring]\n",
		"wordy/part.css":       "",
		"wordy/words.yaml":     "en: { a: A }\n",
		"holder/schema.json":   `{"type": "object", "properties": {"p": {"$ref": "floe:///parts/panel.json"}}}`,
		"holder/part.html":     `<div data-floe-part="holder"></div>`,
		"holder/part.css":      "",
		"keyed/schema.json":    `{"type": "object"}`,
		"keyed/part.html":      `<div data-floe-part="keyed"></div>`,
		"keyed/part.css":       "",
		"keyed/part.yaml":      "lookz: [panel]\n",
		"boxlook/schema.json":  `{"type": "object"}`,
		"boxlook/part.html":    `<div data-floe-part="boxlook"><i data-floe-part="panel"></i></div>`,
		"boxlook/part.css":     "",
		"halfway/part.yaml":    "looks: [panel]\n",
		"halfway/part.html":    `<div data-floe-part="halfway"></div>`,
		"twodocs/schema.json":  `{"type": "object"}`,
		"twodocs/part.html":    `<div data-floe-part="twodocs"></div>`,
		"twodocs/part.css":     "",
		"twodocs/part.yaml":    "looks: [panel]\n---\nlooks: [rows]\n",
		"base/schema.json":     `{"type": "object"}`,
		"palette/schema.json":  `{"type": "object"}`,
		"diagram/schema.json":  `{"type": "object"}`,
		"code/schema.json":     `{"type": "object"}`,
		"code/part.html":       `<div data-floe-part="code"></div>`,
		"code/part.css":        "}\n",
		"loop/schema.json":     `{"type": "object", "properties": {"kids": {"type": "array", "items": {"$ref": "floe:///parts/loop.json"}}}}`,
		"loop/part.html":       `<div data-floe-part="loop"></div>`,
		"loop/part.css":        "",
		"ping/schema.json":     `{"type": "object", "properties": {"pong": {"$ref": "floe:///parts/pong.json"}}}`,
		"ping/part.html":       `<div data-floe-part="ping"></div>`,
		"ping/part.css":        "",
		"pong/schema.json":     `{"type": "object", "properties": {"ping": {"$ref": "floe:///parts/ping.json"}}}`,
		"pong/part.html":       `<div data-floe-part="pong"></div>`,
		"pong/part.css":        "",
		"Quote/schema.json":    `{"type": "object"}`,
		"blurb/schema.json":    `{"type": "object", "properties": {"body": {"type": "string", "format": "html"}}}`,
		"blurb/part.html":      `<div data-floe-part="blurb">{{.body}}</div>`,
		"blurb/part.css":       "",
		"gallery/schema.json":  `{"type": "array", "items": {"type": "object", "properties": {"src": {"type": "string", "format": "image"}}}}`,
		"gallery/part.html":    `<div data-floe-part="gallery"></div>`,
		"gallery/part.css":     "",
		"marked/schema.json":   `{"type": "object", "properties": {"text": {"type": "string", "format": "markdown"}}}`,
		"marked/part.html":     `<div data-floe-part="marked">{{.text}}</div>`,
		"marked/part.css":      "",
		// A format of JSON Schema's own is checked as the part's schema is, and stays.
		"dated/schema.json":   `{"type": "object", "properties": {"on": {"type": "string", "format": "date"}}}`,
		"dated/part.html":     `<div data-floe-part="dated">{{.on}}</div>`,
		"dated/part.css":      "",
		"grid/schema.json":    `{"$ref": "floe:///parts/table.json"}`,
		"grid/part.html":      `<div data-floe-part="grid"></div>`,
		"grid/part.css":       "",
		"jotted/schema.json":  `{"description": "A note again", "$ref": "note.json"}`,
		"jotted/part.html":    `<div data-floe-part="jotted"></div>`,
		"jotted/part.css":     "",
		"plotted/schema.json": `{"$ref": "floe:///parts/diagram.json"}`,
		"plotted/part.html":   `<div data-floe-part="plotted"></div>`,
		"plotted/part.css":    "",
		// Named before unnamed, which cannot be used, and after it: told the same whether unnamed is left out
		// yet or not.
		"aliased/schema.json":  `{"$ref": "floe:///parts/unnamed.json"}`,
		"aliased/part.html":    `<div data-floe-part="aliased"></div>`,
		"aliased/part.css":     "",
		"zaliased/schema.json": `{"$ref": "floe:///parts/unnamed.json"}`,
		"zaliased/part.html":   `<div data-floe-part="zaliased"></div>`,
		"zaliased/part.css":    "",
		// A name reserved for what is no part is not told as one.
		"based/schema.json": `{"$ref": "floe:///parts/base.json"}`,
		"based/part.html":   `<div data-floe-part="based"></div>`,
		"based/part.css":    "",
		// A schema the parts share at the root is the part's own value.
		"toned/schema.json": `{"$ref": "tone.json"}`,
		"toned/part.html":   `<b data-floe-part="toned">{{.}}</b>`,
		"toned/part.css":    "",
	}
	for _, name := range []string{"quote", "tone"} {
		for file, body := range quote(name) {
			files[file] = body
		}
	}
	parts := ownParts(t, files)
	if err := os.Symlink(filepath.Join(parts, "nowhere"), filepath.Join(parts, "script")); err != nil {
		t.Fatal(err)
	}
	whys := map[string]string{
		"unnamed": "the part unnamed cannot be used: " + filepath.Join(parts, "unnamed", "part.html") +
			`: no element of the part unnamed is its box, data-floe-part="unnamed"`,
		"brace": "the part brace cannot be used: " + filepath.Join(parts, "brace", "part.css") + ":2: this } closes no block",
		"lender": "the part lender cannot be used: " + filepath.Join(parts, "lender", "part.yaml") +
			":2: the part names the part nope, which there is none of",
		"base":     "the part base cannot be used: " + filepath.Join(parts, "base") + ": a part of your own cannot be named base",
		"palette":  "the part palette cannot be used: " + filepath.Join(parts, "palette") + ": a part of your own cannot be named palette",
		"diagram":  "the part diagram cannot be used: " + filepath.Join(parts, "diagram") + ": a part of your own cannot be named diagram",
		"script":   "the part script cannot be used: " + filepath.Join(parts, "script") + ": a part of your own cannot be named script",
		"tone":     "the part tone cannot be used: " + filepath.Join(parts, "tone") + ": a part of your own cannot be named tone",
		"borrower": "the part borrower cannot be used: it names the part unnamed, or takes its look, which cannot be used",
		"ring":     "the part ring cannot be used: " + filepath.Join(parts, "ring") + ": the part ring takes its own look (ring → round → ring)",
		"round":    "the part round cannot be used: it names the part ring, or takes its look, which cannot be used",
		"wordy": "the part wordy cannot be used: " + filepath.Join(parts, "wordy", "words.yaml") +
			": the look wordy holds no schema.json nor part.html",
		"holder": "the part holder cannot be used: " + filepath.Join(parts, "holder", "schema.json") +
			": the part refers to floe:///parts/panel.json, a look",
		"keyed": "the part keyed cannot be used: " + filepath.Join(parts, "keyed", "part.yaml") + ":1:1: lookz is not a key of part.yaml",
		"boxlook": "the part boxlook cannot be used: " + filepath.Join(parts, "boxlook", "part.html") +
			":1: data-floe-part names the look panel, which has no box",
		"twodocs": "the part twodocs cannot be used: " + filepath.Join(parts, "twodocs", "part.yaml") + ":2:1: part.yaml holds one YAML document",
		"halfway": "the part halfway cannot be used: " + filepath.Join(parts, "halfway") +
			": a part holds schema.json, part.html and part.css",
		"code":     "the part code cannot be used: " + filepath.Join(parts, "code", "part.css") + ":1: this } closes no block",
		"diff":     "the part diff cannot be used: it names the part code, or takes its look, which cannot be used",
		"terminal": "the part terminal cannot be used: it names the part code, or takes its look, which cannot be used",
		"pong":     "the part pong cannot be used: it holds the part ping, which cannot be used",
		"first": "the part first cannot be used: " + filepath.Join(parts, "first", "part.html") +
			`:1: data-floe-part="cards first" is not the name of one part`,
		"spaced": "the part spaced cannot be used: " + filepath.Join(parts, "spaced", "part.html") +
			`: no element of the part spaced is its box, data-floe-part="spaced"`,
		"typo": "the part typo cannot be used: " + filepath.Join(parts, "typo", "schema.json") +
			": the part refers to floe:///parts/celll.json, which is no part",
		"stack":   "the part stack cannot be used: it holds the part code, which cannot be used",
		"tabs":    "the part tabs cannot be used: it holds the part stack, which cannot be used",
		"details": "the part details cannot be used: it holds the part stack, which cannot be used",
		"loop":    "the part loop cannot be used: " + filepath.Join(parts, "loop") + ": the part loop holds itself (loop → loop)",
		"ping":    "the part ping cannot be used: " + filepath.Join(parts, "ping") + ": the part ping holds itself (ping → pong → ping)",
		"Quote":   "the part Quote cannot be used: " + filepath.Join(parts, "Quote") + ": a name is lower case letters",
		"blurb": "the part blurb cannot be used: " + filepath.Join(parts, "blurb", "schema.json") +
			`: #/properties/body: a part's schema takes no "format": "html"`,
		"gallery": "the part gallery cannot be used: " + filepath.Join(parts, "gallery", "schema.json") +
			`: #/items/properties/src: a part's schema takes no "format": "image"`,
		"marked": "the part marked cannot be used: " + filepath.Join(parts, "marked", "schema.json") +
			`: #/properties/text: a part's schema takes no "format": "markdown"`,
		"grid": "the part grid cannot be used: " + filepath.Join(parts, "grid", "schema.json") +
			": the $ref at the root of the schema names the part table",
		"jotted": "the part jotted cannot be used: " + filepath.Join(parts, "jotted", "schema.json") +
			": the $ref at the root of the schema names the part note",
		"plotted": "the part plotted cannot be used: " + filepath.Join(parts, "plotted", "schema.json") +
			": the $ref at the root of the schema names the part diagram",
		"aliased": "the part aliased cannot be used: " + filepath.Join(parts, "aliased", "schema.json") +
			": the $ref at the root of the schema names the part unnamed",
		"based": "the part based cannot be used: it holds the part base, which cannot be used",
		"zaliased": "the part zaliased cannot be used: " + filepath.Join(parts, "zaliased", "schema.json") +
			": the $ref at the root of the schema names the part unnamed",
	}
	for name, why := range whys {
		dir := memo(t, `"risks": {"$ref": "floe:///parts/`+name+`.json"}, "aside": {"type": "string"}`)
		_, mistakes := tmpl.Check(filepath.Join(dir, "memo"), parts, "")
		if name == "diagram" || name == "script" || name == "tone" {
			// floe's stays: the schema is right, and only the example, which fits none of them, fails.
			if len(mistakes) != 1 || strings.Contains(mistakes[0].Error(), "cannot be used") {
				t.Errorf("Check of a template of %s = %v", name, mistakes)
			}
			continue
		}
		if len(mistakes) != 1 || !strings.Contains(mistakes[0].Error(), why) {
			t.Errorf("Check of a template of %s = %v, want %s", name, mistakes, why)
		}
	}
	// By page.html.
	named := memo(t, memoItems)
	page := filepath.Join(named, "memo", "page.html")
	if err := os.WriteFile(page, []byte(`<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head>`+
		`<body><div data-floe-part="brace">{{.Data.risks}}</div></body></html>`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, mistakes := tmpl.Check(filepath.Join(named, "memo"), parts, ""); len(mistakes) != 1 || !strings.Contains(mistakes[0].Error(), whys["brace"]) {
		t.Errorf("Check of a page.html that names brace = %v", mistakes)
	}
	if err := os.WriteFile(page, []byte(`<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head>`+
		`<body><div data-floe-part="diagram">{{.Data.risks}}</div></body></html>`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, mistakes := tmpl.Check(filepath.Join(named, "memo"), parts, ""); len(mistakes) != 0 {
		t.Errorf("Check of a page.html that names floe's diagram = %v", mistakes)
	}
	// Named in a comment alone, which is no attribute, brace is warned of, not refused.
	if err := os.WriteFile(page, []byte(`<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head>`+
		`<body><!-- data-floe-part="brace" -->{{.Data.risks}}</body></html>`), 0o600); err != nil {
		t.Fatal(err)
	}
	if warnings, mistakes := tmpl.Check(filepath.Join(named, "memo"), parts, ""); len(mistakes) != 0 ||
		!slices.ContainsFunc(warnings, func(w *usererr.Error) bool { return strings.HasPrefix(w.Error(), whys["brace"]) }) {
		t.Errorf("Check of a page.html that names brace in a comment = %v, %v", warnings, mistakes)
	}
	// A template that uses none of them is drawn, its check warned of each.
	dir := memo(t, `"risks": {"$ref": "floe:///parts/quote.json"}, "aside": {"type": "string"}`)
	if _, err := draw(tmpl.NewLibrary(dir, parts, ""), "template: memo\nlang: en\nrisks: { text: Hi }\n"); err != nil {
		t.Errorf("drawing a template of the quote = %v", err)
	}
	dated := memo(t, `"risks": {"$ref": "floe:///parts/dated.json"}, "aside": {"type": "string"}`)
	if page, err := draw(tmpl.NewLibrary(dated, parts, ""), "template: memo\nlang: en\nrisks: { on: 2026-10-20 }\n"); err != nil ||
		!strings.Contains(page, `<div data-floe-part="dated">2026-10-20</div>`) {
		t.Errorf("drawing a part of a format of JSON Schema's = %v", err)
	}
	toned := memo(t, `"risks": {"$ref": "floe:///parts/toned.json"}, "aside": {"type": "string"}`)
	if page, err := draw(tmpl.NewLibrary(toned, parts, ""), "template: memo\nlang: en\nrisks: good\n"); err != nil ||
		!strings.Contains(page, `<b data-floe-part="toned">good</b>`) {
		t.Errorf("drawing a part of a tone = %v", err)
	}
	// floe's tone stays the tone of its parts.
	if page, err := draw(tmpl.NewLibrary(memo(t, memoItems), parts, ""), "template: memo\nlang: en\n"+
		"risks: { columns: [{title: Risk}], rows: [[Links]] }\naside: { text: Hi, tone: warn }\n"); err != nil ||
		!strings.Contains(page, `<div class="warn" data-floe-part="note">`) {
		t.Errorf("drawing a note of a tone = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "memo", "example.yaml"), []byte("template: memo\nlang: en\nrisks: { text: Hi }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	warnings, mistakes := tmpl.Check(filepath.Join(dir, "memo"), parts, "")
	var told []string
	for _, w := range warnings {
		if strings.Contains(w.Error(), "cannot be used") {
			told = append(told, w.Error())
		}
	}
	if len(mistakes) != 0 || len(told) != len(whys) {
		t.Errorf("Check of a template of the quote = %v, %v", told, mistakes)
	}
}

// A parts theme of one's own is picked as floe's are. One of the name of one of floe's takes the place
// of its files of their names, the rest staying floe's. floe's themes do not style a part of one's own
// that took the place of floe's, whose rules would fit it ill: its file goes, for the parts that take its
// look too. One that cannot be used is refused to a data file that picks it; a file of a name
// starting with a dot, one not of CSS, one for a part of one's own that cannot be used, and one for no
// part, which is warned of, do not make it so.
func TestOwnPartsThemes(t *testing.T) {
	t.Parallel()

	files := quote("code")
	files["themes/brutal/code.css"] = "/* brutal's code of one's own */\n"
	files["themes/mine/base.css"] = "/* mine */\n:root { --mine-ink: #123; }\n"
	files["themes/mine/palette.css"] = "/* mine's palette */\n:root { --bg: #123; }\n" +
		"@media (prefers-color-scheme: dark) { :root { --bg: #321; } }\n"
	files["themes/pale/palette.css"] = ":root { --bg: #fdf6e3; --font: serif; --rule: 2px solid var(--fg); }\n"
	files["themes/rules/palette.css"] = ":root { --bg: #fff; }\nbody { margin: 0; }\n"
	files["themes/default/palette.css"] = ":root { --accent: #c00; }\n@media (prefers-color-scheme: dark) { :root { --accent: #f66; } }\n"
	files["themes/bad/nope.css"] = ""
	files["themes/mine/.DS_Store"] = "x"
	files["themes/mine/README.md"] = "x"
	files["broken/schema.json"] = `{"type": "object"}`
	files["broken/part.html"] = `<div></div>`
	files["broken/part.css"] = ""
	files["themes/brutal/broken.css"] = ""
	files["tone/schema.json"] = `{"type": "object"}`
	files["themes/stray/tone.css"] = ""
	files["themes/broke/table.css"] = "}\n"
	files["themes/broke/palette.css"] = ":root { --bg: #fff; }\n"
	parts := ownParts(t, files)
	dir := memo(t, `"risks": {"$ref": "floe:///parts/code.json"}, "aside": {"$ref": "floe:///parts/note.json"}`)
	l := tmpl.NewLibrary(dir, parts, "")
	page, err := draw(l, "template: memo\nlang: en\nparts-theme: brutal\nrisks: { text: x }\naside: { text: y }\n")
	if err != nil {
		t.Fatal(err)
	}
	note, err := os.ReadFile(filepath.Join("parts", "themes", "brutal", "note.css"))
	if err != nil {
		t.Fatal(err)
	}
	for want, n := range map[string]int{
		`@scope ([data-floe-part="code"]) to (:scope [data-floe-part] > *) {` + "\n/* brutal's code of one's own */": 1,
		string(note): 1,
		":scope .lang { border-radius: 0; border: 2px solid var(--brutal-ink)": 0,
	} {
		if strings.Count(page, want) != n {
			t.Errorf("the page holds %s %d times, want %d:\n%s", want, strings.Count(page, want), n, page)
		}
	}
	if page, err = draw(l, "template: memo\nlang: en\nparts-theme: mine\nrisks: { text: x }\n"); err != nil ||
		!strings.Contains(page, "/* mine */") {
		t.Errorf("drawing with the parts theme mine = %v:\n%s", err, page)
	}
	// Its palette goes over floe's default one, which keeps the variables it leaves out, and so does a
	// default of one's own, which does not take the place of floe's.
	under, mine := strings.Index(page, "@layer floe.palette {\n/* Every page's colours"), strings.Index(page, "@layer floe.palette {\n/* mine's palette */")
	if under < 0 || mine < under || strings.Contains(page, "--accent: #c00") {
		t.Errorf("the palettes of the parts theme mine are at %d and %d:\n%s", under, mine, page)
	}
	if page, err = draw(l, "template: memo\nlang: en\nrisks: { text: x }\n"); err != nil ||
		strings.Index(page, "/* Every page's colours") > strings.Index(page, ":root { --accent: #c00; }") {
		t.Errorf("drawing with default of one's own = %v:\n%s", err, page)
	}
	// palette.css holds the palette's variables alone, and is warned of where it sets a colour for light
	// mode alone: dark mode would keep it.
	rules := "your own parts theme rules cannot be used: " + filepath.Join(parts, "themes", "rules", "palette.css") +
		":2: palette.css holds only the palette's variables"
	if _, err = draw(l, "template: memo\nlang: en\nparts-theme: rules\nrisks: { text: x }\n"); err == nil || !strings.Contains(err.Error(), rules) {
		t.Errorf("drawing with the parts theme rules = %v, want %s", err, rules)
	}
	// A file of a name no part takes goes into no page, and leaves the theme to be used: tone.css too, a
	// part of one's own named tone left out or not.
	if _, err = draw(l, "template: memo\nlang: en\nparts-theme: bad\nrisks: { text: x }\n"); err != nil {
		t.Errorf("drawing with the parts theme bad = %v", err)
	}
	strays := []string{
		filepath.Join(parts, "themes", "bad", "nope.css") + " goes into no page",
		filepath.Join(parts, "themes", "stray", "tone.css") + " goes into no page",
	}
	why := "your own parts theme broke cannot be used: " + filepath.Join(parts, "themes", "broke", "table.css") +
		":1: this } closes no block"
	if _, err = draw(l, "template: memo\nlang: en\nparts-theme: broke\nrisks: { text: x }\n"); err == nil ||
		!strings.HasPrefix(err.Error(), "data.yaml:3:14: "+why) {
		t.Errorf("drawing with the parts theme broke = %v, want %s", err, why)
	}
	// Told beside the data's other mistakes.
	if _, err = draw(l, "template: memo\nlang: en\ntheme: nope\nparts-theme: broke\nrisks: { text: x }\n"); err == nil ||
		!strings.Contains(err.Error(), "data.yaml:3:8: ") || !strings.Contains(err.Error(), "data.yaml:4:14: "+why) {
		t.Errorf("drawing with the theme nope and the parts theme broke = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "memo", "example.yaml"), []byte("template: memo\nlang: en\nrisks: { text: x }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	warnings, mistakes := tmpl.Check(filepath.Join(dir, "memo"), parts, "")
	warned := func(prefix string) bool {
		return slices.ContainsFunc(warnings, func(w *usererr.Error) bool { return strings.HasPrefix(w.Error(), prefix) })
	}
	pale := filepath.Join(parts, "themes", "pale", "palette.css") + " sets --bg for light mode alone"
	if !warned(pale) || warned(filepath.Join(parts, "themes", "mine")) || warned(filepath.Join(parts, "themes", "broke", "palette.css")) {
		t.Errorf("Check = %v, want %s alone of the palettes", warnings, pale)
	}
	for _, want := range append(strays, why) {
		if len(mistakes) != 0 || !slices.ContainsFunc(warnings, func(w *usererr.Error) bool { return strings.HasPrefix(w.Error(), want) }) {
			t.Errorf("Check = %v, %v, want %s", warnings, mistakes, want)
		}
	}
}

// A word a part's HTML names from the data (word .level) that its words.yaml has not is told at the
// data's item, beside the failures of the checks of the other items.
func TestPartWordAtItem(t *testing.T) {
	t.Parallel()

	parts := ownParts(t, map[string]string{
		"level/schema.json": `{"type": "object", "properties": {"level": {"type": "string"}}}`,
		"level/part.html":   `<span data-floe-part="level">{{word .level}}</span>`,
		"level/part.css":    "",
		"level/words.yaml":  "en: { low: Low }\n",
	})
	l := tmpl.NewLibrary(memo(t, `"risks": {"$ref": "floe:///parts/level.json"}, "aside": {"$ref": "floe:///parts/progress.json"}`), parts, "")
	if page, err := draw(l, "template: memo\nlang: en\nrisks: { level: low }\n"); err != nil || !strings.Contains(page, ">Low</span>") {
		t.Errorf("drawing a word the part has = %v", err)
	}
	want := "data.yaml:3:10: risks: " + filepath.Join(parts, "level", "words.yaml") + ` has no word "mid"`
	if _, err := draw(l, "template: memo\nlang: en\nrisks: { level: mid }\n"); err == nil || err.Error() != want {
		t.Errorf("drawing a word the part has not = %v, want %s", err, want)
	}
	_, err := draw(l, "template: memo\nlang: en\nrisks: { level: mid }\naside: [{ label: Pages, done: 5, of: 3 }]\n")
	if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "data.yaml:4:") {
		t.Errorf("drawing a word the part has not beside a check that fails = %v", err)
	}
}

// A page that names a part that cannot be used, by a table of Markdown of the data, is refused, rather
// than published without its style; themes of one's own that cannot be read are warned of.
func TestBrokenPartInPage(t *testing.T) {
	t.Parallel()

	parts := ownParts(t, map[string]string{
		"table/schema.json": `{"type": "object"}`,
		"table/part.html":   `<div data-floe-part="table"></div>`,
		"table/part.css":    "}\n",
	})
	if err := os.Symlink(filepath.Join(parts, "nowhere"), filepath.Join(parts, "themes")); err != nil {
		t.Fatal(err)
	}
	dir := memo(t, `"risks": {"type": "string"}, "aside": {"type": "string", "format": "markdown"}`)
	l := tmpl.NewLibrary(dir, parts, "")
	if _, err := draw(l, "template: memo\nlang: en\nrisks: x\naside: |\n  | a |\n  | --- |\n  | b |\n"); err == nil ||
		!strings.HasPrefix(err.Error(), "the part table cannot be used: "+filepath.Join(parts, "table", "part.css")) {
		t.Errorf("drawing a table of Markdown = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "memo", "example.yaml"), []byte("template: memo\nlang: en\nrisks: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := "the parts themes of your own in " + filepath.Join(parts, "themes") + " cannot be read"
	if warnings, _ := tmpl.Check(filepath.Join(dir, "memo"), parts, ""); !slices.ContainsFunc(warnings, func(w *usererr.Error) bool {
		return strings.HasPrefix(w.Error(), want)
	}) {
		t.Errorf("Check = %v, want %s", warnings, want)
	}
}

// A look alone of one's own cannot take the place of one of floe's that draws, which Markdown's tables,
// floe's other parts and templates draw by: it is refused, with what to do, and floe's is not used either.
func TestLookReplacesDrawing(t *testing.T) {
	t.Parallel()

	parts := ownParts(t, map[string]string{"table/part.css": ":scope { color: red; }\n"})
	l := tmpl.NewLibrary(memo(t, `"risks": {"$ref": "floe:///parts/table.json"}, "aside": {"type": "string"}`), parts, "")
	_, err := draw(l, "template: memo\nlang: en\nrisks: { columns: [{title: A}], rows: [[x]] }\n")
	want := "the part table cannot be used: " + filepath.Join(parts, "table") + ": a look alone cannot take the place of floe's part table"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("drawing a table with a look alone named table = %v, want %s", err, want)
	}
}
