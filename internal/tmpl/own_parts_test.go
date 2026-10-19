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
// style goes into the page once, in its scope. One of the name of one of floe's takes its place whole,
// for the parts drawn in its look too.
func TestOwnParts(t *testing.T) {
	t.Parallel()

	files := quote("quote")
	for name, body := range quote("table") {
		files[name] = body
	}
	parts := ownParts(t, files)
	l := tmpl.NewLibrary(memo(t, `"risks": {"$ref": "floe:///parts/quote.json"}, "aside": {"$ref": "floe:///parts/table.json"}`), parts, "")
	page, err := draw(l, "template: memo\nlang: ja\nrisks: { text: Hi, source: Ann }\naside: { text: Rows }\n")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<figure data-floe-part="quote"><blockquote>Hi</blockquote><figcaption class="source">出典 Ann</figcaption></figure>`,
		`<figure data-floe-part="table"><blockquote>Rows</blockquote></figure>`,
		`@scope ([data-floe-part~="quote"]) to (:scope [data-floe-part] > *) {` + "\n/* the quote of one's own */",
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
}

// A part of one's own that cannot be used is refused, with why, to a template that uses it, by its
// schema or its page.html, and told as a warning to one that does not, which is drawn: one that takes the
// look of one left out too, floe's among them, and one that holds itself, which floe would follow
// forever. One named as a part floe draws in Go or a schema the parts share leaves floe's in place, a
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
		"lender/part.html":     `<div data-floe-part="nope lender"></div>`,
		"lender/part.css":      "",
		"borrower/schema.json": `{"type": "object"}`,
		"borrower/part.html":   `<div data-floe-part="unnamed borrower"></div>`,
		"borrower/part.css":    "",
		"base/schema.json":     `{"type": "object"}`,
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
			`: the part unnamed names itself in no data-floe-part`,
		"brace": "the part brace cannot be used: " + filepath.Join(parts, "brace", "part.css") + ":2: this } closes no block",
		"lender": "the part lender cannot be used: " + filepath.Join(parts, "lender", "part.html") +
			":1: the part takes the look of the part nope, which there is none of",
		"base":     "the part base cannot be used: " + filepath.Join(parts, "base") + ": a part of your own cannot be named base",
		"diagram":  "the part diagram cannot be used: " + filepath.Join(parts, "diagram") + ": a part of your own cannot be named diagram",
		"script":   "the part script cannot be used: " + filepath.Join(parts, "script") + ": a part of your own cannot be named script",
		"tone":     "the part tone cannot be used: " + filepath.Join(parts, "tone") + ": a part of your own cannot be named tone",
		"borrower": "the part borrower cannot be used: it takes the look of your own part unnamed, which cannot be used",
		"code":     "the part code cannot be used: " + filepath.Join(parts, "code", "part.css") + ":1: this } closes no block",
		"diff":     "the part diff cannot be used: it takes the look of your own part code, which cannot be used",
		"loop":     "the part loop cannot be used: " + filepath.Join(parts, "loop") + ": the part loop holds itself (loop → loop)",
		"ping":     "the part ping cannot be used: " + filepath.Join(parts, "ping") + ": the part ping holds itself (ping → pong → ping)",
		"Quote":    "the part Quote cannot be used: " + filepath.Join(parts, "Quote") + ": a name is lower case letters",
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
	// pong holds ping, which is left out.
	pong := memo(t, `"risks": {"$ref": "floe:///parts/pong.json"}, "aside": {"type": "string"}`)
	if _, mistakes := tmpl.Check(filepath.Join(pong, "memo"), parts, ""); len(mistakes) != 1 || !strings.Contains(mistakes[0].Error(), whys["ping"]) {
		t.Errorf("Check of a template of pong = %v", mistakes)
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
	// A template that uses none of them is drawn, its check warned of each.
	dir := memo(t, `"risks": {"$ref": "floe:///parts/quote.json"}, "aside": {"type": "string"}`)
	if _, err := draw(tmpl.NewLibrary(dir, parts, ""), "template: memo\nlang: en\nrisks: { text: Hi }\n"); err != nil {
		t.Errorf("drawing a template of the quote = %v", err)
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
// that took the place of floe's, whose rules would fit it ill: its file goes, and its parts.css does not
// reach the part. One that cannot be used is refused to a data file that picks it; a file of a name
// starting with a dot, one not of CSS, and one for a part of one's own that cannot be used do not make
// it so.
func TestOwnPartsThemes(t *testing.T) {
	t.Parallel()

	files := quote("code")
	files["themes/brutal/code.css"] = "/* brutal's code of one's own */\n"
	files["themes/mine/base.css"] = "/* mine */\n:root { --mine-ink: #123; }\n"
	files["themes/bad/nope.css"] = ""
	files["themes/mine/.DS_Store"] = "x"
	files["themes/mine/README.md"] = "x"
	files["broken/schema.json"] = `{"type": "object"}`
	files["broken/part.html"] = `<div></div>`
	files["broken/part.css"] = ""
	files["themes/brutal/broken.css"] = ""
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
		`@scope ([data-floe-part~="code"]) to (:scope [data-floe-part] > *) {` + "\n/* brutal's code of one's own */":          1,
		`@scope ([data-floe-part]:not([data-floe-part="code"], [data-floe-part$=" code"])) to (:scope [data-floe-part] > *) {`: 1,
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
	why := "your own parts theme bad cannot be used: " + filepath.Join(parts, "themes", "bad", "nope.css") +
		": a file of a parts theme is base.css, parts.css or the style of a part"
	if _, err = draw(l, "template: memo\nlang: en\nparts-theme: bad\nrisks: { text: x }\n"); err == nil ||
		!strings.HasPrefix(err.Error(), "data.yaml:3:14: "+why) {
		t.Errorf("drawing with the parts theme bad = %v, want %s", err, why)
	}
	// Told beside the data's other mistakes.
	if _, err = draw(l, "template: memo\nlang: en\ntheme: nope\nparts-theme: bad\nrisks: { text: x }\n"); err == nil ||
		!strings.Contains(err.Error(), "data.yaml:3:8: ") || !strings.Contains(err.Error(), "data.yaml:4:14: "+why) {
		t.Errorf("drawing with the theme nope and the parts theme bad = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "memo", "example.yaml"), []byte("template: memo\nlang: en\nrisks: { text: x }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	warnings, mistakes := tmpl.Check(filepath.Join(dir, "memo"), parts, "")
	if len(mistakes) != 0 || !slices.ContainsFunc(warnings, func(w *usererr.Error) bool { return strings.HasPrefix(w.Error(), why) }) {
		t.Errorf("Check = %v, %v, want %s", warnings, mistakes, why)
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
