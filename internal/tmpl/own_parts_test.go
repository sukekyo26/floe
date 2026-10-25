package tmpl_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

// ownParts writes parts of one's own, files by their path under the directory of parts, and returns it.
func ownParts(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "parts")
	if err := os.Mkdir(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	for name, body := range files {
		path := filepath.Join(dir, name)
		if theme, ok := strings.CutPrefix(name, "themes/"); ok {
			path = filepath.Join(themesOf(dir), theme)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// themesOf is the directory of themes of one's own beside parts, where ownParts writes those of files
// under themes/.
func themesOf(parts string) string {
	return filepath.Join(filepath.Dir(parts), "themes")
}

// quote is a part of one's own, name, that draws a quote and who said it.
func quote(name string) map[string]string {
	return map[string]string{
		name + "/schema.json": `{"type": "object", "properties": {"text": {"type": "string"}, "source": {"type": "string"}}, ` +
			`"required": ["text"], "additionalProperties": false}`,
		name + "/part.html": `<figure data-floe-part="` + name + `"><blockquote>{{.text}}</blockquote>` +
			`{{with .source}}<figcaption class="source">{{word "by"}} {{.}}</figcaption>{{end}}</figure>`,
		name + "/words.yaml": "en: { by: by }\nja: { by: 出典 }\n",
		name + "/part.css":   "/* the " + name + " of one's own */\n:scope { margin: 0; }\n.source { color: var(--muted); }\n",
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
	pilled := tmpl.NewLibrary(memo(t, `"risks": {"$ref": "floe:///parts/pilled.json"}, "aside": {"type": "string"}`), parts, themesOf(parts), "")
	if page, err := draw(pilled, "template: memo\nlang: en\nrisks: { v: { pill: ok, tone: good } }\n"); err != nil ||
		!strings.Contains(page, `<p data-floe-part="pilled"><span class="good" data-floe-part="cell">ok</span></p>`) {
		t.Errorf("drawing a part that holds a cell by cell.json = %v:\n%s", err, page)
	}
	l := tmpl.NewLibrary(memo(t, `"risks": {"$ref": "floe:///parts/quote.json"}, "aside": {"$ref": "floe:///parts/table.json"}`), parts, themesOf(parts), "")
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
	l = tmpl.NewLibrary(memo(t, `"risks": {"$ref": "floe:///parts/compare.json"}, "aside": {"type": "string"}`), parts, themesOf(parts), "")
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
	l = tmpl.NewLibrary(memo(t, `"risks": {"$ref": "floe:///parts/boxed.json"}, "aside": {"type": "string"}`), ownParts(t, looked), "", "")
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
		// What floe cannot read, out of floe, wherever the $ref is (#90).
		"hosted/schema.json": `{"type": "object", "properties": {"text": {"anyOf": [{"$ref": "file:///etc/hosts"}]}}}`,
		"hosted/part.html":   `<div data-floe-part="hosted"></div>`,
		"hosted/part.css":    "",
		// Its own file, which floe does not follow to, but jsonschema does: usable.
		"defined/schema.json": `{"type": "object", "properties": {"t": {"$ref": "#/$defs/t"}}, "$defs": {"t": {"type": "string"}}}`,
		"defined/part.html":   `<div data-floe-part="defined">{{.t}}</div>`,
		"defined/part.css":    "",
		// A $ref that finds nothing where it points, in its own file or in a schema the parts share: at the $ref.
		"pointed/schema.json": `{"type": "object", "properties": {"t": {"$ref": "#/$defs/nope"}}}`,
		"pointed/part.html":   `<div data-floe-part="pointed"></div>`,
		"pointed/part.css":    "",
		"deadptr/schema.json": `{"type": "object", "properties": {"t": {"$ref": "tone.json#/nope"}}}`,
		"deadptr/part.html":   `<div data-floe-part="deadptr"></div>`,
		"deadptr/part.css":    "",
		"unread/schema.json":  `{"type": "object", "properties": {"t": {"$ref": "http://[::1"}}}`,
		"unread/part.html":    `<div data-floe-part="unread"></div>`,
		"unread/part.css":     "",
		"holder/schema.json":  `{"type": "object", "properties": {"p": {"$ref": "floe:///parts/panel.json"}}}`,
		"holder/part.html":    `<div data-floe-part="holder"></div>`,
		"holder/part.css":     "",
		"keyed/schema.json":   `{"type": "object"}`,
		"keyed/part.html":     `<div data-floe-part="keyed"></div>`,
		"keyed/part.css":      "",
		"keyed/part.yaml":     "lookz: [panel]\n",
		"boxlook/schema.json": `{"type": "object"}`,
		"boxlook/part.html":   `<div data-floe-part="boxlook"><i data-floe-part="panel"></i></div>`,
		"boxlook/part.css":    "",
		"halfway/part.yaml":   "looks: [panel]\n",
		"halfway/part.html":   `<div data-floe-part="halfway"></div>`,
		"twodocs/schema.json": `{"type": "object"}`,
		"twodocs/part.html":   `<div data-floe-part="twodocs"></div>`,
		"twodocs/part.css":    "",
		"twodocs/part.yaml":   "looks: [panel]\n---\nlooks: [rows]\n",
		"base/schema.json":    `{"type": "object"}`,
		"palette/schema.json": `{"type": "object"}`,
		"diagram/schema.json": `{"type": "object"}`,
		"code/schema.json":    `{"type": "object"}`,
		"code/part.html":      `<div data-floe-part="code"></div>`,
		"code/part.css":       "}\n",
		"loop/schema.json":    `{"type": "object", "properties": {"kids": {"type": "array", "items": {"$ref": "floe:///parts/loop.json"}}}}`,
		"loop/part.html":      `<div data-floe-part="loop"></div>`,
		"loop/part.css":       "",
		"ping/schema.json":    `{"type": "object", "properties": {"pong": {"$ref": "floe:///parts/pong.json"}}}`,
		"ping/part.html":      `<div data-floe-part="ping"></div>`,
		"ping/part.css":       "",
		"pong/schema.json":    `{"type": "object", "properties": {"ping": {"$ref": "floe:///parts/ping.json"}}}`,
		"pong/part.html":      `<div data-floe-part="pong"></div>`,
		"pong/part.css":       "",
		"Quote/schema.json":   `{"type": "object"}`,
		"blurb/schema.json":   `{"type": "object", "properties": {"body": {"type": "string", "format": "html"}}}`,
		"blurb/part.html":     `<div data-floe-part="blurb">{{.body}}</div>`,
		"blurb/part.css":      "",
		"gallery/schema.json": `{"type": "array", "items": {"type": "object", "properties": {"src": {"type": "string", "format": "image"}}}}`,
		"gallery/part.html":   `<div data-floe-part="gallery"></div>`,
		"gallery/part.css":    "",
		"marked/schema.json":  `{"type": "object", "properties": {"text": {"type": "string", "format": "markdown"}}}`,
		"marked/part.html":    `<div data-floe-part="marked">{{.text}}</div>`,
		"marked/part.css":     "",
		// A schema the parts share is drawn as the part's own items, its formats too.
		"anything/schema.json": `{"type": "object", "properties": {"inner": {"$ref": "any.json"}}}`,
		"anything/part.html":   `<div data-floe-part="anything"></div>`,
		"anything/part.css":    "",
		// Its $schema names a draft of JSON Schema, not a part.
		"drafted/schema.json": `{"$schema": "floe:///parts/table.json", "type": "object"}`,
		"drafted/part.html":   `<div data-floe-part="drafted"></div>`,
		"drafted/part.css":    "",
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
		// A look, which draws nothing, is told as a look, not as a part.
		"paneled/schema.json": `{"$ref": "floe:///parts/panel.json"}`,
		"paneled/part.html":   `<div data-floe-part="paneled"></div>`,
		"paneled/part.css":    "",
		// A name reserved for what is no part is not told as one.
		"based/schema.json": `{"$ref": "floe:///parts/base.json"}`,
		"based/part.html":   `<div data-floe-part="based"></div>`,
		"based/part.css":    "",
		// A schema the parts share at the root is the part's own value.
		"toned/schema.json": `{"$ref": "tone.json"}`,
		"toned/part.html":   `<b data-floe-part="toned">{{.}}</b>`,
		"toned/part.css":    "",
		// What compiling the schema finds, at its pointer: a pattern that is no regular expression.
		"regexed/schema.json": `{"type": "object", "properties": {"text": {"type": "string", "pattern": "(["}}}`,
		"regexed/part.html":   `<div data-floe-part="regexed">{{.text}}</div>`,
		"regexed/part.css":    "",
		// Named before regexed, whose compiling stops at regexed's: told as a part that cannot be used.
		"aregexed/schema.json": `{"type": "object", "properties": {"r": {"$ref": "regexed.json"}}}`,
		"aregexed/part.html":   `<div data-floe-part="aregexed"></div>`,
		"aregexed/part.css":    "",
		// A mistake of compiling does not hide those of its names.
		"unboxed/schema.json": `{"type": "object", "properties": {"text": {"type": "string", "pattern": "(["}}}`,
		"unboxed/part.html":   `<div class="x"></div>`,
		"unboxed/part.css":    "",
		// yb's $ref into ya, which finds nothing, is yb's: ya's compiling stops there too, which jsonschema tells
		// as ya's file.
		"ya/schema.json": `{"type": "object", "properties": {"b": {"$ref": "yb.json"}}}`,
		"ya/part.html":   `<div data-floe-part="ya"></div>`,
		"ya/part.css":    "",
		"yb/schema.json": `{"type": "object", "properties": {"t": {"$ref": "ya.json#/$defs/nope"}}}`,
		"yb/part.html":   `<div data-floe-part="yb"></div>`,
		"yb/part.css":    "",
		// What no check of a $ref sees, a $ref through an $id, in the part's own file, at its top, as a
		// template's would be, and a part that holds it, whose compiling stops there too, told as one that
		// holds a part that cannot be used.
		"idd/schema.json":  `{"$id": "https://example.com/p.json", "type": "object", "properties": {"t": {"$ref": "#/$defs/nope"}}}`,
		"idd/part.html":    `<div data-floe-part="idd"></div>`,
		"idd/part.css":     "",
		"aidd/schema.json": `{"type": "object", "properties": {"d": {"$ref": "idd.json"}}}`,
		"aidd/part.html":   `<div data-floe-part="aidd"></div>`,
		"aidd/part.css":    "",
		// zy's $ref into zx at what is no schema is zy's, though the metaschema refuses it under zx's file.
		"zx/schema.json": `{"type": "object", "properties": {"y": {"$ref": "zy.json"}}}`,
		"zx/part.html":   `<div data-floe-part="zx"></div>`,
		"zx/part.css":    "",
		"zy/schema.json": `{"type": "object", "properties": {"t": {"$ref": "zx.json#/type"}}}`,
		"zy/part.html":   `<div data-floe-part="zy"></div>`,
		"zy/part.css":    "",
		// A $ref to a value of its own enum, right as it is: at the $ref.
		"enumref/schema.json": `{"type": "object", "properties": {"t": {"$ref": "#/properties/u/enum/0"}, "u": {"enum": ["a"]}}}`,
		"enumref/part.html":   `<div data-floe-part="enumref"></div>`,
		"enumref/part.css":    "",
		// No URL to JSON Schema, though net/url reads it: told at the $ref, once, not by the metaschema too.
		"bslash/schema.json": `{"type": "object", "properties": {"t": {"$ref": "#/$defs/a\\b"}}, "$defs": {"a\\b": {"type": "string"}}}`,
		"bslash/part.html":   `<div data-floe-part="bslash"></div>`,
		"bslash/part.css":    "",
		// A $dynamicRef in a part, which another holds.
		"dyn/schema.json":  `{"type": "object", "properties": {"t": {"type": "object", "$dynamicRef": "https://example.com/x.json"}}}`,
		"dyn/part.html":    `<div data-floe-part="dyn"></div>`,
		"dyn/part.css":     "",
		"adyn/schema.json": `{"type": "object", "properties": {"t": {"type": "object", "$dynamicRef": "floe:///parts/dyn.json"}}}`,
		"adyn/part.html":   `<div data-floe-part="adyn"></div>`,
		"adyn/part.css":    "",
		// The mistakes of several files are told together, under the part.
		"wrongs/schema.json": `{"type": "object"}`,
		"wrongs/part.html":   `<div data-floe-part="wrongs"></div>`,
		"wrongs/part.css":    "}\n",
		"wrongs/part.yaml":   "lookz: [panel]\n",
		"wrongs/words.yaml":  "en: { a: A }\nja: {}\n",
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
		"unnamed": "the part unnamed cannot be used\n  " + filepath.Join(parts, "unnamed", "part.html") +
			`: no element of the part unnamed is its box, data-floe-part="unnamed"`,
		"brace": "the part brace cannot be used\n  " + filepath.Join(parts, "brace", "part.css") + ":2: this } closes no block",
		"lender": "the part lender cannot be used\n  " + filepath.Join(parts, "lender", "part.yaml") +
			":2: the part names the part nope, which there is none of",
		"base":     "the part base cannot be used\n  " + filepath.Join(parts, "base") + ": a part of your own cannot be named base",
		"palette":  "the part palette cannot be used\n  " + filepath.Join(parts, "palette") + ": a part of your own cannot be named palette",
		"diagram":  "the part diagram cannot be used\n  " + filepath.Join(parts, "diagram") + ": a part of your own cannot be named diagram",
		"script":   "the part script cannot be used\n  " + filepath.Join(parts, "script") + ": a part of your own cannot be named script",
		"tone":     "the part tone cannot be used\n  " + filepath.Join(parts, "tone") + ": a part of your own cannot be named tone",
		"borrower": "the part borrower cannot be used\n  " + filepath.Join(parts, "borrower", "part.yaml") + ":1: it names the part unnamed, or takes its look, which cannot be used",
		"ring":     "the part ring cannot be used\n  " + filepath.Join(parts, "ring") + ": the part ring takes its own look (ring → round → ring)",
		"round":    "the part round cannot be used\n  " + filepath.Join(parts, "round", "part.yaml") + ":1: it names the part ring, or takes its look, which cannot be used",
		"wordy": "the part wordy cannot be used\n  " + filepath.Join(parts, "wordy", "words.yaml") +
			": the look wordy holds no schema.json nor part.html",
		"holder": "the part holder cannot be used\n  " + filepath.Join(parts, "holder", "schema.json") +
			": #/properties/p: floe:///parts/panel.json is the look panel, which draws nothing",
		"keyed": "the part keyed cannot be used\n  " + filepath.Join(parts, "keyed", "part.yaml") + ":1:1: lookz is not a key of part.yaml",
		"boxlook": "the part boxlook cannot be used\n  " + filepath.Join(parts, "boxlook", "part.html") +
			":1: data-floe-part names the look panel, which has no box",
		"twodocs": "the part twodocs cannot be used\n  " + filepath.Join(parts, "twodocs", "part.yaml") + ":2:1: part.yaml holds one YAML document",
		"halfway": "the part halfway cannot be used\n  " + filepath.Join(parts, "halfway") +
			": a part holds schema.json, part.html and part.css",
		"code":     "the part code cannot be used\n  " + filepath.Join(parts, "code", "part.css") + ":1: this } closes no block",
		"diff":     "the part diff cannot be used\n  parts/diff/part.yaml:1: it names the part code, or takes its look, which cannot be used",
		"terminal": "the part terminal cannot be used\n  parts/terminal/part.yaml:1: it names the part code, or takes its look, which cannot be used",
		"pong": "the part pong cannot be used\n  " + filepath.Join(parts, "pong", "schema.json") +
			": #/properties/ping: the part ping cannot be used",
		"first": "the part first cannot be used\n  " + filepath.Join(parts, "first", "part.html") +
			`:1: data-floe-part="cards first" is not the name of one part`,
		"spaced": "the part spaced cannot be used\n  " + filepath.Join(parts, "spaced", "part.html") +
			`: no element of the part spaced is its box, data-floe-part="spaced"`,
		"hosted": "the part hosted cannot be used\n  " + filepath.Join(parts, "hosted", "schema.json") +
			": #/properties/text/anyOf/0: file:///etc/hosts: a schema refers to no other file or URL",
		"typo": "the part typo cannot be used\n  " + filepath.Join(parts, "typo", "schema.json") +
			": #/properties/c: floe:///parts/celll.json is not a part",
		// floe's that hold one of one's own that cannot be used (code) stay usable.
		"stack":   "",
		"tabs":    "",
		"details": "",
		"loop":    "the part loop cannot be used\n  " + filepath.Join(parts, "loop") + ": the part loop holds itself (loop → loop)",
		"ping":    "the part ping cannot be used\n  " + filepath.Join(parts, "ping") + ": the part ping holds itself (ping → pong → ping)",
		"Quote":   "the part Quote cannot be used\n  " + filepath.Join(parts, "Quote") + ": a name is lower case letters",
		"drafted": "the part drafted cannot be used\n  " + filepath.Join(parts, "drafted", "schema.json") +
			": $schema floe:///parts/table.json names no draft of JSON Schema",
		"blurb": "the part blurb cannot be used\n  " + filepath.Join(parts, "blurb", "schema.json") +
			`: #/properties/body: a part's schema takes no "format": "html"`,
		"anything": "the part anything cannot be used\n  " + filepath.Join(parts, "anything", "schema.json") +
			`: #/properties/inner → floe:///parts/any.json#/properties/html: a part's schema takes no "format": "html", which floe:///parts/any.json brings in`,
		"gallery": "the part gallery cannot be used\n  " + filepath.Join(parts, "gallery", "schema.json") +
			`: #/items/properties/src: a part's schema takes no "format": "image"`,
		"marked": "the part marked cannot be used\n  " + filepath.Join(parts, "marked", "schema.json") +
			`: #/properties/text: a part's schema takes no "format": "markdown"`,
		"grid": "the part grid cannot be used\n  " + filepath.Join(parts, "grid", "schema.json") +
			": the $ref at the root of the schema names the part table",
		"jotted": "the part jotted cannot be used\n  " + filepath.Join(parts, "jotted", "schema.json") +
			": the $ref at the root of the schema names the part note",
		"plotted": "the part plotted cannot be used\n  " + filepath.Join(parts, "plotted", "schema.json") +
			": the $ref at the root of the schema names the part diagram",
		"aliased": "the part aliased cannot be used\n  " + filepath.Join(parts, "aliased", "schema.json") +
			": #: the part unnamed cannot be used",
		"based": "the part based cannot be used\n  " + filepath.Join(parts, "based", "schema.json") +
			": #: the part base cannot be used",
		"zaliased": "the part zaliased cannot be used\n  " + filepath.Join(parts, "zaliased", "schema.json") +
			": #: the part unnamed cannot be used",
		"paneled": "the part paneled cannot be used\n  " + filepath.Join(parts, "paneled", "schema.json") +
			": #: floe:///parts/panel.json is the look panel, which draws nothing",
		"regexed": "the part regexed cannot be used\n  " + filepath.Join(parts, "regexed", "schema.json") +
			": #/properties/text/pattern: JSON Schema does not take this: '([' is not valid regex",
		"aregexed": "the part aregexed cannot be used\n  " + filepath.Join(parts, "aregexed", "schema.json") +
			": #/properties/r: the part regexed cannot be used",
		"unboxed": "the part unboxed cannot be used\n  " + filepath.Join(parts, "unboxed", "part.html") +
			`: no element of the part unboxed is its box, data-floe-part="unboxed"; write it on the part's element` + "\n  " +
			filepath.Join(parts, "unboxed", "schema.json") + ": #/properties/text/pattern: JSON Schema does not take this: '([' is not valid regex",
		"ya": "the part ya cannot be used\n  " + filepath.Join(parts, "ya", "schema.json") + ": #/properties/b: the part yb cannot be used\n    " +
			filepath.Join(parts, "yb", "schema.json") + ": #/properties/t: floe:///parts/ya.json#/$defs/nope points into a part",
		"yb": "the part yb cannot be used\n  " + filepath.Join(parts, "yb", "schema.json") +
			": #/properties/t: floe:///parts/ya.json#/$defs/nope points into a part",
		"idd": "the part idd cannot be used\n  " + filepath.Join(parts, "idd", "schema.json") +
			": #: a $ref to #/$defs/nope finds nothing in schema.json",
		"aidd": "the part aidd cannot be used\n  " + filepath.Join(parts, "aidd", "schema.json") + ": #/properties/d: the part idd cannot be used\n    " +
			filepath.Join(parts, "idd", "schema.json") + ": #: a $ref to #/$defs/nope finds nothing in schema.json",
		"zx": "the part zx cannot be used\n  " + filepath.Join(parts, "zx", "schema.json") + ": #/properties/y: the part zy cannot be used\n    " +
			filepath.Join(parts, "zy", "schema.json") + ": #/properties/t: floe:///parts/zx.json#/type points into a part",
		"zy": "the part zy cannot be used\n  " + filepath.Join(parts, "zy", "schema.json") +
			": #/properties/t: floe:///parts/zx.json#/type points into a part",
		"enumref": "the part enumref cannot be used\n  " + filepath.Join(parts, "enumref", "schema.json") +
			": #/properties/t: a $ref to #/properties/u/enum/0 points at what is no schema",
		"bslash": "the part bslash cannot be used\n  " + filepath.Join(parts, "bslash", "schema.json") +
			`: #/properties/t: the $ref #/$defs/a\b cannot be read as a URL (contains \)`,
		"dyn": "the part dyn cannot be used\n  " + filepath.Join(parts, "dyn", "schema.json") +
			": #/properties/t: floe does not follow $dynamicRef",
		"adyn": "the part adyn cannot be used\n  " + filepath.Join(parts, "adyn", "schema.json") +
			": #/properties/t: floe does not follow $dynamicRef",
		"pointed": "the part pointed cannot be used\n  " + filepath.Join(parts, "pointed", "schema.json") +
			": #/properties/t: a $ref to #/$defs/nope finds nothing in schema.json",
		"deadptr": "the part deadptr cannot be used\n  " + filepath.Join(parts, "deadptr", "schema.json") +
			": #/properties/t: a $ref to tone.json#/nope finds nothing in floe:///parts/tone.json: refer to what is there",
		"unread": "the part unread cannot be used\n  " + filepath.Join(parts, "unread", "schema.json") +
			": #/properties/t: the $ref http://[::1 cannot be read as a URL (missing ']' in host)",
		"wrongs": "the part wrongs cannot be used\n  " + filepath.Join(parts, "wrongs", "part.yaml") + ":1:1: lookz is not a key of part.yaml" +
			"; write looks:, the list of the looks the part takes, like looks: [panel], and a look alone's description:\n  " +
			filepath.Join(parts, "wrongs", "words.yaml") + ":2:1: ja leaves out a, which en has\n  " +
			filepath.Join(parts, "wrongs", "part.css") + ":1: this } closes no block",
	}
	for name, why := range whys {
		dir := memo(t, `"risks": {"$ref": "floe:///parts/`+name+`.json"}, "aside": {"type": "string"}`)
		_, mistakes := check(filepath.Join(dir, "memo"), parts, themesOf(parts))
		if why == "" || name == "diagram" || name == "script" || name == "tone" {
			// floe's stays: the schema is right, and only the example, which fits none of them, fails.
			example := filepath.Join(dir, "memo", "example.yaml") + ":"
			if mistakes == nil || slices.ContainsFunc(mistakes, func(e error) bool { return !strings.HasPrefix(e.Error(), example) }) {
				t.Errorf("Check of a template of %s = %v", name, mistakes)
			}
			continue
		}
		// Told as floe part check tells it, at the $ref, not in the words of jsonschema's loading.
		at := filepath.Join(dir, "memo", "schema.json") + ": #/properties/risks: "
		// A $ref is told once, at the $ref, not again by the compiling, at the top, or by the metaschema.
		if len(mistakes) != 1 || !strings.HasPrefix(mistakes[0].Error(), at+why) ||
			strings.Count(mistakes[0].Error(), "finds nothing") > 1 || strings.Contains(mistakes[0].Error(), "uri-reference") {
			t.Errorf("Check of a template of %s = %v, want %s", name, mistakes, why)
		}
	}
	// A data file of the template, as the page of its mistakes words it, in the reader's language.
	braced := memo(t, `"risks": {"$ref": "floe:///parts/brace.json"}, "aside": {"type": "string"}`)
	_, err := draw(tmpl.NewLibrary(braced, parts, themesOf(parts), ""), "template: memo\nlang: en\n")
	var mistake i18n.Localizer
	if !errors.As(err, &mistake) || !strings.HasPrefix(mistake.Localize(i18n.New("ja")), "data.yaml:1:11: 自分のテンプレート memo は使えません\n  "+
		filepath.Join(braced, "memo", "schema.json")+": #/properties/risks: 部品 brace は使えません\n    "+filepath.Join(parts, "brace", "part.css")+":2: ") {
		t.Errorf("Read of a template of brace = %v", err)
	}
	// By page.html.
	named := memo(t, memoItems)
	page := filepath.Join(named, "memo", "page.html")
	if err := os.WriteFile(page, []byte(`<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head>`+
		`<body><div data-floe-part="brace">{{.Data.risks}}</div></body></html>`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, mistakes := check(filepath.Join(named, "memo"), parts, themesOf(parts)); len(mistakes) != 1 || !strings.Contains(mistakes[0].Error(), whys["brace"]) {
		t.Errorf("Check of a page.html that names brace = %v", mistakes)
	}
	if err := os.WriteFile(page, []byte(`<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head>`+
		`<body><div data-floe-part="diagram">{{.Data.risks}}</div></body></html>`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, mistakes := check(filepath.Join(named, "memo"), parts, themesOf(parts)); len(mistakes) != 0 {
		t.Errorf("Check of a page.html that names floe's diagram = %v", mistakes)
	}
	// Named in a comment alone, which is no attribute, brace is warned of, not refused.
	if err := os.WriteFile(page, []byte(`<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head>`+
		`<body><!-- data-floe-part="brace" -->{{.Data.risks}}</body></html>`), 0o600); err != nil {
		t.Fatal(err)
	}
	if warnings, mistakes := check(filepath.Join(named, "memo"), parts, themesOf(parts)); len(mistakes) != 0 ||
		!slices.ContainsFunc(warnings, func(w *usererr.Error) bool { return strings.HasPrefix(w.Error(), whys["brace"]) }) {
		t.Errorf("Check of a page.html that names brace in a comment = %v, %v", warnings, mistakes)
	}
	// A template that uses none of them is drawn, its check warned of each.
	dir := memo(t, `"risks": {"$ref": "floe:///parts/quote.json"}, "aside": {"type": "string"}`)
	if _, err := draw(tmpl.NewLibrary(dir, parts, themesOf(parts), ""), "template: memo\nlang: en\nrisks: { text: Hi }\n"); err != nil {
		t.Errorf("drawing a template of the quote = %v", err)
	}
	dated := memo(t, `"risks": {"$ref": "floe:///parts/dated.json"}, "aside": {"type": "string"}`)
	if page, err := draw(tmpl.NewLibrary(dated, parts, themesOf(parts), ""), "template: memo\nlang: en\nrisks: { on: 2026-10-20 }\n"); err != nil ||
		!strings.Contains(page, `<div data-floe-part="dated">2026-10-20</div>`) {
		t.Errorf("drawing a part of a format of JSON Schema's = %v", err)
	}
	toned := memo(t, `"risks": {"$ref": "floe:///parts/toned.json"}, "aside": {"type": "string"}`)
	if page, err := draw(tmpl.NewLibrary(toned, parts, themesOf(parts), ""), "template: memo\nlang: en\nrisks: good\n"); err != nil ||
		!strings.Contains(page, `<b data-floe-part="toned">good</b>`) {
		t.Errorf("drawing a part of a tone = %v", err)
	}
	// floe's tone stays the tone of its parts.
	if page, err := draw(tmpl.NewLibrary(memo(t, memoItems), parts, themesOf(parts), ""), "template: memo\nlang: en\n"+
		"risks: { columns: [{title: Risk}], rows: [[Links]] }\naside: { text: Hi, tone: warn }\n"); err != nil ||
		!strings.Contains(page, `<div class="warn" data-floe-part="note">`) {
		t.Errorf("drawing a note of a tone = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "memo", "example.yaml"), []byte("template: memo\nlang: en\nrisks: { text: Hi }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	warnings, mistakes := check(filepath.Join(dir, "memo"), parts, themesOf(parts))
	var told []string
	for _, w := range warnings {
		if strings.Contains(w.Error(), "cannot be used") {
			told = append(told, w.Error())
		}
	}
	unusable := 0
	for _, why := range whys {
		if why != "" {
			unusable++
		}
	}
	if len(mistakes) != 0 || len(told) != unusable {
		t.Errorf("Check of a template of the quote = %v, %v", told, mistakes)
	}
}

// A part of floe's that holds one of one's own that cannot be used stays usable, and so do a part and a
// template that hold it: only the data that takes the one that cannot be used is refused, at its item.
func TestHeldThroughFloesPart(t *testing.T) {
	t.Parallel()

	parts := ownParts(t, map[string]string{
		"cell/schema.json": `{"$id": "https://example.com/p.json", "type": "object", "properties": {"t": {"$ref": "#/$defs/nope"}}}`,
		"cell/part.html":   `<div data-floe-part="cell"></div>`,
		"cell/part.css":    "",
		"mine/schema.json": `{"type": "object", "properties": {"t": {"$ref": "floe:///parts/table.json"}}}`,
		"mine/part.html":   `<div data-floe-part="mine"></div>`,
		"mine/part.css":    "",
	})
	dir := memo(t, `"risks": {"$ref": "floe:///parts/mine.json"}, "aside": {"type": "string"}`)
	if _, mistakes := check(filepath.Join(dir, "memo"), parts, themesOf(parts)); len(mistakes) != 0 {
		t.Errorf("Check of a template of mine = %v, want none", mistakes)
	}
	l := tmpl.NewLibrary(dir, parts, themesOf(parts), "")
	_, err := draw(l, "template: memo\nlang: en\nrisks:\n  t: { columns: [{title: A}], rows: [[x]] }\n")
	if want := "data.yaml:4:39: the part cell cannot be used\n  " + filepath.Join(parts, "cell", "schema.json"); err == nil ||
		!strings.HasPrefix(err.Error(), want) {
		t.Errorf("drawing a table of mine = %v, want %s", err, want)
	}
}

// A part's schema is compiled with the other parts of one's own standing as true: its own mistake is told
// whatever order jsonschema walks it in, and one that another's $ref into it meets is not taken for its own.
func TestPartCompileOrder(t *testing.T) {
	t.Parallel()

	parts := ownParts(t, map[string]string{
		// pa's under draft-04, whose metaschema checks no regular expression, met as it compiles, and qb's,
		// which its metaschema refuses as it is read, which compiling pa reads too.
		"pa/schema.json": `{"$schema": "http://json-schema.org/draft-04/schema#", "type": "object",` +
			` "properties": {"a": {"$ref": "qb.json"}, "z": {"patternProperties": {"([": {}}}}}`,
		"pa/part.html":   `<div data-floe-part="pa"></div>`,
		"pa/part.css":    "",
		"qb/schema.json": `{"type": "object", "properties": {"t": {"type": "string", "pattern": "[["}}}`,
		"qb/part.html":   `<div data-floe-part="qb"></div>`,
		"qb/part.css":    "",
		// xid's own $ref through its $id, which no check of a $ref resolves, finds nothing; yref, which xid
		// does not hold, points at the same place: xid's is still xid's. aref and zid are the two of them, the
		// other way round by name.
		"xid/schema.json":  `{"$id": "https://example.com/x.json", "type": "object", "properties": {"t": {"$ref": "#/$defs/nope"}}}`,
		"xid/part.html":    `<div data-floe-part="xid"></div>`,
		"xid/part.css":     "",
		"yref/schema.json": `{"type": "object", "properties": {"u": {"$ref": "xid.json#/$defs/nope"}}}`,
		"yref/part.html":   `<div data-floe-part="yref"></div>`,
		"yref/part.css":    "",
		"zid/schema.json":  `{"$id": "https://example.com/z.json", "type": "object", "properties": {"t": {"$ref": "#/$defs/nope"}}}`,
		"zid/part.html":    `<div data-floe-part="zid"></div>`,
		"zid/part.css":     "",
		"aref/schema.json": `{"type": "object", "properties": {"u": {"$ref": "zid.json#/$defs/nope"}}}`,
		"aref/part.html":   `<div data-floe-part="aref"></div>`,
		"aref/part.css":    "",
		// A mistake of a $ref of it, where floe does not follow to a part, does not hide one of its own file
		// the compiling finds.
		"xa/schema.json": `{"$id": "https://example.com/a.json", "type": "object",` +
			` "properties": {"t": {"$ref": "#/$defs/nope"}}, "$defs": {"q": {"$ref": "floe:///parts/note.json"}}}`,
		"xa/part.html": `<div data-floe-part="xa"></div>`,
		"xa/part.css":  "",
		// cell, in place of floe's, holds itself through yy and is left out: floe's table, which holds cell,
		// stays usable, and so do aa and zz, which hold table, whatever their names.
		"cell/schema.json": `{"type": "object", "properties": {"y": {"$ref": "yy.json"}}}`,
		"cell/part.html":   `<div data-floe-part="cell"></div>`,
		"cell/part.css":    "",
		"yy/schema.json":   `{"type": "object", "properties": {"c": {"$ref": "cell.json"}}}`,
		"yy/part.html":     `<div data-floe-part="yy"></div>`,
		"yy/part.css":      "",
		"aa/schema.json":   `{"type": "object", "properties": {"t": {"$ref": "table.json"}}}`,
		"aa/part.html":     `<div data-floe-part="aa"></div>`,
		"aa/part.css":      "",
		"zz/schema.json":   `{"type": "object", "properties": {"t": {"$ref": "table.json"}}}`,
		"zz/part.html":     `<div data-floe-part="zz"></div>`,
		"zz/part.css":      "",
	})
	want := []string{
		filepath.Join(parts, "pa", "schema.json") + ": #/properties/z/patternProperties: ",
		filepath.Join(parts, "xid", "schema.json") + ": #: a $ref to #/$defs/nope finds nothing",
		filepath.Join(parts, "zid", "schema.json") + ": #: a $ref to #/$defs/nope finds nothing",
		filepath.Join(parts, "yref", "schema.json") + ": #/properties/u: ",
		filepath.Join(parts, "aref", "schema.json") + ": #/properties/u: ",
		filepath.Join(parts, "xa", "schema.json") + ": #: a $ref to #/$defs/nope finds nothing",
		filepath.Join(parts, "xa", "schema.json") + ": #/$defs/q: ",
		"the part cell cannot be used",
	}
	first := ""
	for range 20 {
		warnings, err := tmpl.NewLibrary("", parts, themesOf(parts), "").Run().CheckOwnParts()
		got := fmt.Sprint(warnings, err)
		if first == "" {
			first = got
		}
		if got != first || slices.ContainsFunc(want, func(w string) bool { return !strings.Contains(got, w) }) ||
			strings.Contains(got, "the part table cannot be used") || strings.Contains(got, "the part aa cannot be used") {
			t.Fatalf("CheckOwnParts = %s, want the same every time, with %q", got, want)
		}
	}
}

// A theme of one's own is picked as floe's are. One of the name of one of floe's takes the place
// of its files of their names, the rest staying floe's. floe's themes do not style a part of one's own
// that took the place of floe's, whose rules would fit it ill: its file goes, for the parts that take its
// look too. One that cannot be used is refused to a data file that picks it; a file of a name
// starting with a dot, one not of CSS, one for a part of one's own that cannot be used, and one for no
// part, which is warned of, do not make it so.
func TestOwnPartsThemes(t *testing.T) {
	t.Parallel()

	files := quote("code")
	files["themes/brutal/code.css"] = "/* brutal's code of one's own */\n"
	files["themes/brutal/palette.css"] = "/* brutal's palette of one's own */\n:root { --brutal-yellow: #ff0; --brutal-head-bg: #c00; }\n"
	files["themes/mine/base.css"] = "/* mine */\nblockquote { color: var(--fg); }\n"
	files["themes/mine/palette.css"] = "/* mine's palette */\n:root { --bg: #123; }\n"
	files["themes/pale/palette.css"] = ":root { --bg: #fdf6e3; --font: serif; --rule: 2px solid var(--fg); }\n"
	files["themes/rules/palette.css"] = ":root { --bg: #fff; }\nbody { margin: 0; }\n"
	files["themes/default/palette.css"] = ":root { --accent: #c00; }\n"
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
	l := tmpl.NewLibrary(dir, parts, themesOf(parts), "")
	page, err := draw(l, "template: memo\nlang: en\ntheme: brutal\nrisks: { text: x }\naside: { text: y }\n")
	if err != nil {
		t.Fatal(err)
	}
	note, err := os.ReadFile(filepath.Join("themes", "brutal", "note.css"))
	if err != nil {
		t.Fatal(err)
	}
	// A palette of one's own goes over floe's of its name, which keeps the theme's variables it leaves out.
	if floe, mine := strings.Index(page, "/* Black on cream"), strings.Index(page, "/* brutal's palette of one's own */"); floe < 0 || mine < floe {
		t.Errorf("the palettes of brutal are at %d and %d:\n%s", floe, mine, page)
	}
	for want, n := range map[string]int{
		`@scope ([data-floe-part="code"]) to (:scope [data-floe-part] > *) {` + "\n/* brutal's code of one's own */": 1,
		string(note): 1,
		":scope .lang { border-radius: 0; border: 2px solid var(--fg)": 0,
	} {
		if strings.Count(page, want) != n {
			t.Errorf("the page holds %s %d times, want %d:\n%s", want, strings.Count(page, want), n, page)
		}
	}
	// The theme mine gives its palette and the parts' shape.
	if page, err = draw(l, "template: memo\nlang: en\ntheme: mine\nrisks: { text: x }\n"); err != nil ||
		!strings.Contains(page, "/* mine */") || !strings.Contains(page, "/* mine's palette */") {
		t.Errorf("drawing with the theme mine = %v:\n%s", err, page)
	}
	// A theme given over the data's gives both too, the data's left unread.
	read := func(src, given string) string {
		d, rerr := l.Read("data.yaml", []byte(src), given, cwd)
		if rerr != nil {
			t.Fatal(rerr)
		}
		out, rerr := d.Render("t")
		if rerr != nil {
			t.Fatal(rerr)
		}
		return string(out)
	}
	if page = read("template: memo\nlang: en\ntheme: brutal\nrisks: { text: x }\n", "mine"); !strings.Contains(page, "/* mine */") ||
		!strings.Contains(page, "/* mine's palette */") || strings.Contains(page, "/* Black on cream") {
		t.Errorf("drawing with --theme mine over theme: brutal = \n%s", page)
	}
	// Its palette goes over floe's default one, which keeps the variables it leaves out, and so does a
	// default of one's own, which does not take the place of floe's.
	under, mine := strings.Index(page, "@layer floe.palette {\n/* Every page's colours"), strings.Index(page, "@layer floe.palette {\n/* mine's palette */")
	if under < 0 || mine < under || strings.Contains(page, "--accent: #c00") {
		t.Errorf("the palettes of the theme mine are at %d and %d:\n%s", under, mine, page)
	}
	if page, err = draw(l, "template: memo\nlang: en\nrisks: { text: x }\n"); err != nil ||
		strings.Index(page, "/* Every page's colours") > strings.Index(page, ":root { --accent: #c00; }") {
		t.Errorf("drawing with default of one's own = %v:\n%s", err, page)
	}
	// palette.css holds the palette's variables alone.
	rules := "your own theme rules cannot be used\n  " + filepath.Join(themesOf(parts), "rules", "palette.css") +
		":2: palette.css holds only the palette's variables"
	if _, err = draw(l, "template: memo\nlang: en\ntheme: rules\nrisks: { text: x }\n"); err == nil || !strings.Contains(err.Error(), rules) {
		t.Errorf("drawing with the theme rules = %v, want %s", err, rules)
	}
	// A file of a name no part takes goes into no page, and leaves the theme to be used: tone.css too, a
	// part of one's own named tone left out or not.
	if _, err = draw(l, "template: memo\nlang: en\ntheme: bad\nrisks: { text: x }\n"); err != nil {
		t.Errorf("drawing with the theme bad = %v", err)
	}
	strays := []string{
		filepath.Join(themesOf(parts), "bad", "nope.css") + ": this file goes into no page",
		filepath.Join(themesOf(parts), "stray", "tone.css") + ": this file goes into no page",
	}
	why := "your own theme broke cannot be used\n  " + filepath.Join(themesOf(parts), "broke", "table.css") +
		":1: this } closes no block"
	if _, err = draw(l, "template: memo\nlang: en\ntheme: broke\nrisks: { text: x }\n"); err == nil ||
		!strings.HasPrefix(err.Error(), "data.yaml:3:8: "+why) {
		t.Errorf("drawing with the theme broke = %v, want %s", err, why)
	}
	// Told beside the data's other mistakes.
	if _, err = draw(l, "template: memo\nlang: en\ntheme: broke\nrisks: { text: x }\nleed: x\n"); err == nil ||
		!strings.Contains(err.Error(), "data.yaml:3:8: "+why) || !strings.Contains(err.Error(), "data.yaml:5:1: leed is not an item") {
		t.Errorf("drawing with the theme broke and an item of no name = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "memo", "example.yaml"), []byte("template: memo\nlang: en\nrisks: { text: x }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	warnings, mistakes := check(filepath.Join(dir, "memo"), parts, themesOf(parts))
	warned := func(prefix string) bool {
		return slices.ContainsFunc(warnings, func(w *usererr.Error) bool { return strings.HasPrefix(w.Error(), prefix) })
	}
	// A palette that sets colours for one look alone is not warned of: the page has no dark mode.
	for _, name := range []string{"pale", "brutal", "mine"} {
		if palette := filepath.Join(themesOf(parts), name, "palette.css"); warned(palette) {
			t.Errorf("Check = %v, want no warning of %s", warnings, palette)
		}
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
	l := tmpl.NewLibrary(memo(t, `"risks": {"$ref": "floe:///parts/level.json"}, "aside": {"$ref": "floe:///parts/progress.json"}`), parts, themesOf(parts), "")
	if page, err := draw(l, "template: memo\nlang: en\nrisks: { level: low }\n"); err != nil || !strings.Contains(page, ">Low</span>") {
		t.Errorf("drawing a word the part has = %v", err)
	}
	want := "data.yaml:3:10: risks: " + filepath.Join(parts, "level", "words.yaml") + `: words.yaml has no word "mid"`
	if _, err := draw(l, "template: memo\nlang: en\nrisks: { level: mid }\n"); err == nil || err.Error() != want {
		t.Errorf("drawing a word the part has not = %v, want %s", err, want)
	}
	_, err := draw(l, "template: memo\nlang: en\nrisks: { level: mid }\naside: [{ label: Pages, done: 5, of: 3 }]\n")
	if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "data.yaml:4:") {
		t.Errorf("drawing a word the part has not beside a check that fails = %v", err)
	}
}

// A check of a part that stops for want of a value the data gives none for is told at its expression in the
// part's checks.yaml, the item and the value named from the top of the data file.
func TestPartCheckLeftOut(t *testing.T) {
	t.Parallel()

	parts := ownParts(t, map[string]string{
		"pc/schema.json": `{"type": "object", "properties": {"a": {"type": "object", "properties": {"n": {"type": "number"}}}}}`,
		"pc/part.html":   `<p data-floe-part="pc"></p>`,
		"pc/part.css":    "",
		"pc/checks.yaml": "- expr: doc.a.n > 0\n  message: m\n",
	})
	l := tmpl.NewLibrary(memo(t, `"risks": {"$ref": "floe:///parts/pc.json"}`), parts, "", "")
	want := filepath.Join(parts, "pc", "checks.yaml") + ":1:9: risks of data.yaml cannot be checked, as data.yaml gives no value for risks.a that the expression uses; "
	if _, err := draw(l, "template: memo\nlang: en\nrisks: {}\n"); err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("checking what the data leaves out = %v, want %s", err, want)
	}
	// A part that is a list gives doc no keys: what the check reads from it is not the data's to give.
	parts = ownParts(t, map[string]string{
		"pl/schema.json": `{"type": "array", "items": {"type": "object"}}`,
		"pl/part.html":   `<p data-floe-part="pl"></p>`,
		"pl/part.css":    "",
		"pl/checks.yaml": "- expr: doc.max > 0\n  message: m\n",
	})
	l = tmpl.NewLibrary(memo(t, `"risks": {"$ref": "floe:///parts/pl.json"}`), parts, "", "")
	if _, err := draw(l, "template: memo\nlang: en\nrisks: [{ a: 1 }]\n"); err == nil || strings.Contains(err.Error(), "gives no value") {
		t.Errorf("checking a list's doc = %v", err)
	}
}

// A part of one's own whose part.html fails to draw an item is told at its part.html, naming the item, for
// every item it fails on, in the order of their keys: the part's mistake, which its maker fixes, not floe's.
func TestPartDrawMistake(t *testing.T) {
	t.Parallel()

	parts := ownParts(t, map[string]string{
		"pp/schema.json": `{"type": "object", "properties": {"text": {"type": "string"}}}`,
		"pp/part.html":   `<p data-floe-part="pp">{{if ne .text "ok"}}{{num .text}}{{end}}</p>`,
		"pp/part.css":    "",
	})
	l := tmpl.NewLibrary(memo(t, `"risks": {"$ref": "floe:///parts/pp.json"}, "aside": {"$ref": "floe:///parts/pp.json"}`), parts, "", "")
	if page, err := draw(l, "template: memo\nlang: en\nrisks: { text: ok }\n"); err != nil || !strings.Contains(page, `data-floe-part="pp"></p>`) {
		t.Errorf("drawing what the part takes = %v", err)
	}
	_, err := draw(l, "template: memo\nlang: en\nrisks: { text: hello }\naside: { text: there }\n")
	var all usererr.List
	failed := func(item, value string) string {
		return filepath.Join(parts, "pp", "part.html") + ": it cannot draw " + item + " of data.yaml: " +
			`template: part.html:1:45: executing "part.html" at <num .text>: error calling num: num: ` + value +
			" (string) is not a number; write it to draw every value its schema.json takes, such as an item left out (draw it in {{with}}) or a list or text shorter than it reads"
	}
	if !errors.As(err, &all) || len(all) != 2 || all[0].Error() != failed("aside", "there") || all[1].Error() != failed("risks", "hello") {
		t.Errorf("drawing what is not a number = %v, want\n%s\n%s", err, failed("aside", "there"), failed("risks", "hello"))
	}
	// What html/template cannot escape, whatever the data, makes the part unusable as it is read: told once, at
	// the line and column of the node html/template names, its words alone, and the second use as told above.
	parts = ownParts(t, map[string]string{
		"pp/schema.json": `{"type": "object", "properties": {"text": {"type": "string"}}}`,
		"pp/part.html":   "<p data-floe-part=\"pp\">\n<a href=\"{{if eq .text `a`}}/x/{{else}}/s?q={{end}}{{.text}}\">x</a></p>",
		"pp/part.css":    "",
	})
	memoDir := memo(t, `"risks": {"$ref": "floe:///parts/pp.json"}, "aside": {"$ref": "floe:///parts/pp.json"}`)
	l = tmpl.NewLibrary(memoDir, parts, "", "")
	schema := filepath.Join(memoDir, "memo", "schema.json")
	want := "data.yaml:1:11: your own template memo cannot be used\n" +
		"  " + schema + ": #/properties/aside: the part pp cannot be used\n" +
		"    " + filepath.Join(parts, "pp", "part.html") + ":2:54: this is not a valid Go template: {{.text}} appears in an ambiguous context within a URL\n" +
		"  " + schema + ": #/properties/risks: the part pp cannot be used, as told above"
	if _, err = draw(l, "template: memo\nlang: en\nrisks: { text: hello }\naside: { text: there }\n"); err == nil || err.Error() != want {
		t.Errorf("drawing what cannot be escaped = %v, want %s", err, want)
	}
	// One html/template places at a node alone is placed at that node too, without its file twice.
	parts = ownParts(t, map[string]string{
		"pp/schema.json": `{"type": "object", "properties": {"text": {"type": "string"}}}`,
		"pp/part.html":   "<p data-floe-part=\"pp\">\n<script>{{if eq .text `a`}}var a = '{{end}}</script></p>",
		"pp/part.css":    "",
	})
	l = tmpl.NewLibrary(memo(t, `"risks": {"$ref": "floe:///parts/pp.json"}`), parts, "", "")
	want = filepath.Join(parts, "pp", "part.html") + ":2:14: this is not a valid Go template: {{if}} branches end in different contexts: "
	if _, err = draw(l, "template: memo\nlang: en\nrisks: { text: hello }\n"); err == nil || !strings.Contains(err.Error(), "\n    "+want) {
		t.Errorf("drawing branches that end apart = %v, want %s", err, want)
	}
}

// An item holding one that fails to draw, or to pass its part's checks, or a diagram refused, is not drawn:
// its part, one's own or floe's, would draw it without what it holds and fail, or be told to.
func TestPartHoldingOneNotDrawn(t *testing.T) {
	t.Parallel()

	parts := ownParts(t, map[string]string{
		"inner/schema.json": `{"type": "object", "properties": {"text": {"type": "string"}}}`,
		"inner/part.html":   `<b data-floe-part="inner">{{num .text}}</b>`,
		"inner/part.css":    "",
		"outer/schema.json": `{"type": "object", "properties": {"in": {"$ref": "inner.json"}, "x": {"type": "array"}}}`,
		"outer/part.html":   `<p data-floe-part="outer">{{if not .in}}{{index .x 3}}{{end}}{{.in}}</p>`,
		"outer/part.css":    "",
		"cell/schema.json":  `{"type": "string"}`,
		"cell/part.html":    `<span data-floe-part="cell">{{num .}}</span>`,
		"cell/part.css":     "",
		"dd/schema.json":    `{"type": "object", "properties": {"plan": {"$ref": "floe:///parts/diagram.json"}}}`,
		"dd/part.html":      `<div data-floe-part="dd">{{if not .plan}}{{num "x"}}{{end}}{{.plan}}</div>`,
		"dd/part.css":       "",
	})
	items := `"risks": {"$ref": "floe:///parts/outer.json"}, "aside": {"$ref": "floe:///parts/table.json"}, "plan": {"$ref": "floe:///parts/dd.json"}`
	l := tmpl.NewLibrary(memo(t, items), parts, "", "")
	for data, failed := range map[string]string{
		"risks: { in: { text: hello } }\n":                         filepath.Join(parts, "inner", "part.html") + ": ",
		"aside: { columns: [{ title: Risk }], rows: [[hello]] }\n": filepath.Join(parts, "cell", "part.html") + ": ",
		"plan: { plan: { nodes: x } }\n":                           "data.yaml:3:17: diagram: unknown key",
	} {
		_, err := draw(l, "template: memo\nlang: en\n"+data)
		var all usererr.List
		if !errors.As(err, &all) || len(all) != 1 || !strings.HasPrefix(all[0].Error(), failed) {
			t.Errorf("drawing %s = %v, want the mistake of %s alone", data, err, failed)
		}
	}
}

// A diagram refused leaves the others drawn: the page is refused, and a part of one's own holding a right one
// is checked, its mistakes told with the diagram's.
func TestPartHoldingRightDiagram(t *testing.T) {
	t.Parallel()

	parts := ownParts(t, map[string]string{
		"dd/schema.json": `{"type": "object", "properties": {"plan": {"$ref": "floe:///parts/diagram.json"}}}`,
		"dd/part.html":   `<div data-floe-part="dd">{{.plan}}{{num "x"}}</div>`,
		"dd/part.css":    "",
	})
	items := `"risks": {"$ref": "floe:///parts/diagram.json"}, "plan": {"$ref": "floe:///parts/dd.json"}`
	l := tmpl.NewLibrary(memo(t, items), parts, "", "")
	_, err := draw(l, "template: memo\nlang: en\n"+
		"risks: { id: b, width: 100, height: 50, title: B, parts: [{ dot: { cx: a, cy: 1 } }] }\n"+
		"plan: { plan: { id: a, width: 100, height: 50, title: A, parts: [] } }\n")
	var all usererr.List
	if !errors.As(err, &all) || len(all) != 2 || !strings.HasPrefix(all[0].Error(), "data.yaml:3:") ||
		!strings.Contains(all[0].Error(), "dot: cx must be a number") ||
		!strings.HasPrefix(all[1].Error(), filepath.Join(parts, "dd", "part.html")+": ") {
		t.Errorf("drawing a wrong diagram and a part holding a right one = %v, want the mistakes of both", err)
	}
}

// An item whose drawing names a part that cannot be used, a table of Markdown of the data, is refused there, rather
// than published without its style; themes of one's own that cannot be read are warned of.
func TestBrokenPartInPage(t *testing.T) {
	t.Parallel()

	parts := ownParts(t, map[string]string{
		"table/schema.json": `{"type": "object"}`,
		"table/part.html":   `<div data-floe-part="table"></div>`,
		"table/part.css":    "}\n",
	})
	if err := os.Symlink(filepath.Join(parts, "nowhere"), themesOf(parts)); err != nil {
		t.Fatal(err)
	}
	dir := memo(t, `"risks": {"type": "string"}, "aside": {"type": "string", "format": "markdown"}`)
	l := tmpl.NewLibrary(dir, parts, themesOf(parts), "")
	if _, err := draw(l, "template: memo\nlang: en\nrisks: x\naside: |\n  | a |\n  | --- |\n  | b |\n"); err == nil ||
		!strings.HasPrefix(err.Error(), "data.yaml:4:8: the part table cannot be used\n  "+filepath.Join(parts, "table", "part.css")) {
		t.Errorf("drawing a table of Markdown = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "memo", "example.yaml"), []byte("template: memo\nlang: en\nrisks: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := "the directory of your own themes " + themesOf(parts) + " cannot be read"
	if warnings, _ := check(filepath.Join(dir, "memo"), parts, themesOf(parts)); !slices.ContainsFunc(warnings, func(w *usererr.Error) bool {
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
	l := tmpl.NewLibrary(memo(t, `"risks": {"$ref": "floe:///parts/table.json"}, "aside": {"type": "string"}`), parts, themesOf(parts), "")
	_, err := draw(l, "template: memo\nlang: en\nrisks: { columns: [{title: A}], rows: [[x]] }\n")
	// Under the template, which names it.
	want := "the part table cannot be used\n    " + filepath.Join(parts, "table") + ": a look alone cannot take the place of floe's part table"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("drawing a table with a look alone named table = %v, want %s", err, want)
	}
}

// The mistakes of a part's schema are told together, and those of its names with those of what it refers
// to.
func TestPartMistakesTogether(t *testing.T) {
	t.Parallel()

	parts := ownParts(t, map[string]string{
		"both/schema.json":  `{"$schema": "floe:///parts/table.json", "type": "object", "properties": {"b": {"type": "string", "format": "html"}}}`,
		"both/part.html":    `<div data-floe-part="both"></div>`,
		"both/part.css":     "",
		"named/schema.json": `{"type": "object", "properties": {"c": {"$ref": "floe:///parts/nope.json"}}}`,
		"named/part.html":   `<div class="x"></div>`,
		"named/part.css":    "",
	})
	_, mistakes := checkOwn(parts, "")
	lines := make([]string, 0, len(mistakes))
	for _, m := range mistakes {
		lines = append(lines, m.Error())
	}
	got := strings.Join(lines, "\n")
	for _, want := range []string{
		"\n  " + filepath.Join(parts, "both", "schema.json") + ": $schema floe:///parts/table.json names no draft of JSON Schema",
		"\n  " + filepath.Join(parts, "both", "schema.json") + `: #/properties/b: a part's schema takes no "format": "html"`,
		"\n  " + filepath.Join(parts, "named", "part.html") + ": no element of the part named is its box",
		"\n  " + filepath.Join(parts, "named", "schema.json") + ": #/properties/c: floe:///parts/nope.json is not a part",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("CheckOwnParts = %s\nwant %s", got, want)
		}
	}
}

// The paths of a part's checks.yaml are checked against its schema as a template's are: one misspelt finds
// nothing to check and is refused at its line, a field without for under the items of a part that is a list.
func TestOwnPartChecksDeclared(t *testing.T) {
	t.Parallel()

	parts := ownParts(t, map[string]string{
		"lines/schema.json": `{"type": "array", "items": {"type": "object", "additionalProperties": false, ` +
			`"properties": {"title": {"type": "string"}}}}`,
		"lines/part.html":   `<ul data-floe-part="lines"></ul>`,
		"lines/part.css":    "",
		"lines/checks.yaml": "- field: titel\n  expr: 'true'\n  message: m\n- field: title\n  expr: 'true'\n  message: m\n",
	})
	_, mistakes := checkOwn(parts, themesOf(parts))
	got := fmt.Sprint(mistakes)
	if len(mistakes) != 1 || !strings.Contains(got, "lines/checks.yaml:1:10: field: schema.json does not declare titel") ||
		strings.Contains(got, "declare title") {
		t.Errorf("CheckOwnParts = %s, want the misspelt field alone", got)
	}
}

// A part of one's own that cannot be used, in place of one of floe's that any.json or floe's parts hold,
// stops only the data that takes it, at its item, as a theme it picks does: its why under the first, and
// "as told above" under the next. The data that does not take it is drawn, and floe's parts that hold it
// stay usable, so floe part check tells it alone.
func TestUnusablePartWhereTaken(t *testing.T) {
	t.Parallel()

	parts := ownParts(t, map[string]string{
		"links/schema.json": `{"type": "array", "items": {"type": "object", "properties": {"u": {"pattern": "(["}}}}`,
		"links/part.html":   `<ul data-floe-part="links"></ul>`,
		"links/part.css":    "",
		"note/schema.json":  `{"type": 1}`,
		"note/part.html":    `<div data-floe-part="note"></div>`,
		"note/part.css":     "",
	})
	l := tmpl.NewLibrary("", parts, themesOf(parts), "")
	page := func(parts string) string {
		return "template: report\nlang: en\ntitle: t\nsections:\n  - heading: h\n    parts:\n" + parts
	}
	for _, src := range []string{
		page("      - text: hi\n"),
		page("      - details: { title: d, parts: [{ text: hi }] }\n"),
	} {
		if _, err := draw(l, src); err != nil {
			t.Errorf("drawing %q = %v, want it drawn", src, err)
		}
	}
	why := "  " + filepath.Join(parts, "links", "schema.json") + ": #/items/properties/u/pattern: "
	_, err := draw(l, page("      - links: [{ title: a, url: /a }]\n      - links: [{ title: b, url: /b }]\n"))
	if lines := strings.Split(fmt.Sprint(err), "\n"); len(lines) != 3 || lines[0] != "data.yaml:7:16: the part links cannot be used" ||
		!strings.HasPrefix(lines[1], why) || lines[2] != "data.yaml:8:16: the part links cannot be used, as told above" {
		t.Errorf("drawing two links = %v", err)
	}
	_, err = draw(l, page("      - details: { title: d, parts: [{ note: { text: hi } }] }\n"))
	if want := "data.yaml:7:48: the part note cannot be used\n  " + filepath.Join(parts, "note", "schema.json"); err == nil ||
		!strings.HasPrefix(err.Error(), want) {
		t.Errorf("drawing a note in details = %v, want %s", err, want)
	}
	_, mistakes := checkOwn(parts, themesOf(parts))
	if got := fmt.Sprint(mistakes); len(mistakes) != 2 || !strings.HasPrefix(got, "[the part links cannot be used") ||
		!strings.Contains(got, "the part note cannot be used") {
		t.Errorf("CheckOwnParts = %s, want links and note alone", got)
	}
}

// What html/template cannot escape in a part.html, whatever the data, is told by part check, at the line and
// column of the node html/template names, in its words alone; what drawing alone fails on is not.
func TestOwnPartsEscape(t *testing.T) {
	t.Parallel()

	files := map[string]string{}
	for name, html := range map[string]string{
		"href":   "<p data-floe-part=\"href\">\n<a href=\"{{if eq .text `a`}}/x/{{else}}/s?q={{end}}{{.text}}\">x</a></p>",
		"self":   `<div data-floe-part="self">{{with .items}}{{template "outline" .}}{{end}}</div>`,
		"jsstr":  `<div data-floe-part="jsstr"><script>var s = "{{.text}}</script></div>`,
		"branch": `<div data-floe-part="branch">{{if .pick}}<a href="{{else}}<b>{{end}}x</div>`,
		"define": `{{define "row"}}<a href="{{.}}{{end}}<div data-floe-part="define">{{template "row" .text}}</div>`,
		"attr":   `<p data-floe-part="attr" a"b="c">`,
		// Right, though no data draws them whole: they are drawing's to tell.
		"nil":   `<div data-floe-part="nil">{{.pick.name}}</div>`,
		"if":    `<div data-floe-part="if">{{if .pick}}{{.pick.name}}{{end}}{{.text}}</div>`,
		"range": `<ul data-floe-part="range">{{range .items}}<li>{{.}}</li>{{end}}</ul>`,
		"funcs": `<div data-floe-part="funcs">{{markdown .text}}{{with .n}}{{num .}}{{end}}{{with .items}}{{index . 0}}{{end}}</div>`,
		"html":  `<div data-floe-part="html">{{.text | html}}</div>`,
	} {
		files[name+"/schema.json"] = `{"type": "object", "properties": {"text": {"type": "string"}, "n": {"type": "number"}, ` +
			`"items": {"type": "array", "items": {"type": "string"}}, "pick": {"type": "object", "properties": {"name": {"type": "string"}}}}}`
		files[name+"/part.html"] = html
		files[name+"/part.css"] = ""
	}
	parts := ownParts(t, files)
	warnings, err := tmpl.NewLibrary("", parts, themesOf(parts), "").Run().CheckOwnParts()
	at := func(name, place, words string) string {
		return "the part " + name + " cannot be used\n  " + filepath.Join(parts, name, "part.html") + place +
			": this is not a valid Go template: " + words
	}
	got := fmt.Sprint(warnings, err)
	for _, want := range []string{
		at("attr", "", `"\"" in attribute name: " a\"b=\"c\">"`),
		at("branch", ":1:35", "{{if}} branches end in different contexts: {stateURL delimDoubleQuote"),
		at("define", "", "ends in a non-text context: {stateURL delimDoubleQuote urlPartPreQuery"),
		at("href", ":2:54", "{{.text}} appears in an ambiguous context within a URL"),
		at("jsstr", "", "ends in a non-text context: {stateJSDqStr"),
		at("self", ":1:54", `no such template "outline"`),
	} {
		if !strings.Contains(got, want) {
			t.Errorf("CheckOwnParts = %s, want %s", got, want)
		}
	}
	for _, right := range []string{"nil", "if", "range", "funcs", "html"} {
		if strings.Contains(got, "the part "+right+" ") {
			t.Errorf("CheckOwnParts = %s, want %s used", got, right)
		}
	}
}
