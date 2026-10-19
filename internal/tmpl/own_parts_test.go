package tmpl_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/tmpl"
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
// schema or its page.html, and told as a warning to one that does not, which is drawn. One named as a
// part floe draws in Go leaves floe's in place.
func TestBrokenOwnParts(t *testing.T) {
	t.Parallel()

	files := map[string]string{
		"unnamed/schema.json": `{"type": "object"}`,
		"unnamed/part.html":   `<div class="x">{{.}}</div>`,
		"unnamed/part.css":    "",
		"brace/schema.json":   `{"type": "object"}`,
		"brace/part.html":     `<div data-floe-part="brace"></div>`,
		"brace/part.css":      ":scope { color: red; }\n}\n",
		"lender/schema.json":  `{"type": "object"}`,
		"lender/part.html":    `<div data-floe-part="nope lender"></div>`,
		"lender/part.css":     "",
		"base/schema.json":    `{"type": "object"}`,
		"diagram/schema.json": `{"type": "object"}`,
	}
	for name, body := range quote("quote") {
		files[name] = body
	}
	parts := ownParts(t, files)
	whys := map[string]string{
		"unnamed": "your own part unnamed cannot be used: " + filepath.Join(parts, "unnamed", "part.html") +
			`: the part unnamed names itself in no data-floe-part`,
		"brace": "your own part brace cannot be used: " + filepath.Join(parts, "brace", "part.css") + ":2: this } closes no block",
		"lender": "your own part lender cannot be used: " + filepath.Join(parts, "lender", "part.html") +
			":1: the part takes the look of the part nope, which there is none of",
		"base":    "your own part base cannot be used: " + filepath.Join(parts, "base") + ": a part of your own cannot be named base",
		"diagram": "your own part diagram cannot be used: " + filepath.Join(parts, "diagram") + ": a part of your own cannot be named diagram",
	}
	for name, why := range whys {
		dir := memo(t, `"risks": {"$ref": "floe:///parts/`+name+`.json"}, "aside": {"type": "string"}`)
		_, mistakes := tmpl.Check(filepath.Join(dir, "memo"), parts, "")
		if name == "diagram" {
			// floe's diagram stays: the schema is right, and only the example, which is no diagram, fails.
			if len(mistakes) != 1 || strings.Contains(mistakes[0].Error(), "cannot be used") {
				t.Errorf("Check of a template of the diagram = %v", mistakes)
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
	// A template that uses none of them is drawn, its check warned of each.
	dir := memo(t, `"risks": {"$ref": "floe:///parts/quote.json"}, "aside": {"type": "string"}`)
	if _, err := draw(tmpl.NewLibrary(dir, parts, ""), "template: memo\nlang: en\nrisks: { text: Hi }\n"); err != nil {
		t.Errorf("drawing a template of the quote = %v", err)
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
