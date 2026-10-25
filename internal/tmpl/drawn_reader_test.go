package tmpl_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/tmpl"
)

// memoPage returns a library of the template memo, whose page.html is src, its items risks and aside of
// format html.
func memoPage(t *testing.T, src string) (*tmpl.Library, string) {
	t.Helper()

	dir := memo(t, `"risks": {"type": "string", "format": "html"}, "aside": {"type": "string", "format": "html"}`)
	if err := os.WriteFile(filepath.Join(dir, "memo", "page.html"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	return tmpl.NewLibrary(dir, "", "", ""), filepath.Join(dir, "memo")
}

// An item is read as a browser reads it: a tag its quote leaves open, or never closed, is no element, and
// the item is told where it is for changing the page, not the page for the name it would give; character
// references are read, so that two names are refused at the item, and one brings its part's style. A name
// in a <noscript> is text, as a sandboxed page runs its scripts (a regression taken: no extra reading).
func TestDrawnReaderItems(t *testing.T) {
	t.Parallel()

	l := htmlIn(t, `<div>{{.Data.risks}}</div>`)
	for item, want := range map[string]string{
		`<p>x</p><div data-floe-part='note>`:                           "data.yaml:3:8: risks: HTML changes the page around it",
		`<p>x</p><div data-floe-part="note table"`:                     "data.yaml:3:8: risks: HTML changes the page around it",
		`<div data-floe-part="note&#32;table"><p>x</p></div>`:          `data.yaml:3:8: risks: the page has data-floe-part="note table", which is not the name of one part`,
		`<noscript><div data-floe-part="note table"></div></noscript>`: "",
	} {
		_, err := draw(l, "template: memo\nlang: en\nrisks: "+yamlText(item)+"\n")
		if want == "" && err != nil || want != "" && (err == nil || !strings.HasPrefix(err.Error(), want)) {
			t.Errorf("%q = %v, want %q", item, err, want)
		}
	}
	page, err := draw(l, "template: memo\nlang: en\nrisks: "+yamlText(`<div data-floe-part="&#110;ote"><p>x</p></div>`)+"\n")
	if err != nil || !strings.Contains(page, `@scope ([data-floe-part="note"])`) {
		t.Errorf("a box named &#110;ote = %v, want the style of note", err)
	}
}

// An item's name is read where the item is: a <title> in an <svg> holds elements, whose names count.
func TestDrawnReaderItemInPlace(t *testing.T) {
	t.Parallel()

	l := htmlIn(t, `<svg>{{.Data.risks}}</svg>`)
	_, err := draw(l, "template: memo\nlang: en\nrisks: "+yamlText(`<title><g data-floe-part="note table"></g></title>`)+"\n")
	if err == nil || !strings.HasPrefix(err.Error(), `data.yaml:3:8: risks: the page has data-floe-part="note table"`) {
		t.Errorf("a name in an <svg>'s <title> = %v", err)
	}
}

// The styles go before the </head> where a <style> styles the page, asked of the page drawn itself: a
// </head> in a value, which page.html draws by what an item holds, is not it.
func TestDrawnReaderStylesPlace(t *testing.T) {
	t.Parallel()

	head := `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title>`
	// The branch changes a value alone, which the items' check leaves out: the page is the same tree.
	l, _ := memoPage(t, head+`<meta name="m" content="{{if eq (printf "%s" .Data.risks) "<b>x</b>"}}</head>{{end}}"></head><body><p>{{.Data.risks}}</p></body></html>`)
	page, err := draw(l, "template: memo\nlang: en\nrisks: <b>x</b>\n")
	if err != nil || !strings.Contains(page, `<meta name="m" content="</head>"><style>`) {
		t.Errorf("a </head> in a value drawn by what an item holds = %v, %s", err, page)
	}
	for src, refused := range map[string]bool{
		head + `<div>x</div></head><body><p>y</p></body></html>`:          false,
		head + `<template></head></template><body><p>y</p></body></html>`: true,
		head + `<frameset></head><frame>`:                                 true,
	} {
		l, _ := memoPage(t, src)
		if _, err := draw(l, "template: memo\nlang: en\n"); (err != nil) != refused {
			t.Errorf("%q = %v, want refused %v", src, err, refused)
		}
	}
}

// When the page cannot be drawn with the items as marks, nothing of the template's is asked of it: no
// </head> is told missing that page.html has.
func TestDrawnReaderNoMarked(t *testing.T) {
	t.Parallel()

	l, _ := memoPage(t, `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head><body><p>{{slice (printf "%s" .Data.risks) 0 40}}</p></body></html>`)
	_, err := draw(l, "template: memo\nlang: en\nrisks: "+strings.Repeat("a", 50)+"\n")
	if err == nil || strings.Contains(err.Error(), "</head>") {
		t.Errorf("a page that cannot be drawn with marks = %v", err)
	}
}

// floe template check draws example.yaml: a page.html whose only </head> is in a <template> is refused.
// A value's character references are read in page.html too, as the drawn page reads them.
func TestDrawnReaderTemplateCheck(t *testing.T) {
	t.Parallel()

	head := `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title>`
	for body, want := range map[string]string{
		`<template></head></template><body></body></html>`:                               "leaves out its </head>",
		`</head><body><div data-floe-part="note&#32;table"><p>x</p></div></body></html>`: `data-floe-part="note table" is not the name of one part`,
	} {
		_, dir := memoPage(t, head+body)
		if err := os.WriteFile(filepath.Join(dir, "example.yaml"), []byte("template: memo\nlang: en\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, mistakes := check(dir, "", "")
		if !slices.ContainsFunc(mistakes, func(e error) bool { return strings.Contains(e.Error(), want) }) {
			t.Errorf("floe template check of %q = %v, want %q", body, mistakes, want)
		}
	}
}

// A name a part's part.html puts together is told at the part's item, the not-one-name too.
func TestDrawnReaderPartNames(t *testing.T) {
	t.Parallel()

	parts := ownParts(t, map[string]string{
		"note/schema.json": `{"type": "object", "properties": {"text": {"type": "string", "minLength": 1, "description": "x"}, ` +
			`"kind": {"type": "string", "minLength": 1, "description": "x"}}, "required": ["text", "kind"], "additionalProperties": false}`,
		"note/part.html": `<div data-floe-part="note"><span data-floe-part="{{.kind}}">{{.text}}</span></div>`,
		"note/part.css":  `[data-floe-part="note"] { margin: 0; }`,
	})
	l := tmpl.NewLibrary(memo(t, `"risks": {"$ref": "floe:///parts/note.json"}`), parts, "", "")
	_, err := draw(l, "template: memo\nlang: en\nrisks: { text: x, kind: note table }\n")
	if err == nil || !strings.HasPrefix(err.Error(), "data.yaml:3:") || !strings.Contains(err.Error(), "risks: the page has") ||
		!strings.Contains(err.Error(), `data-floe-part="note table", which is not the name of one part`) {
		t.Errorf("a name a part puts together = %v", err)
	}
}

// A part's box written with a character reference names the part, as a browser reads it.
func TestDrawnReaderPartBox(t *testing.T) {
	t.Parallel()

	parts := ownParts(t, map[string]string{
		"note/schema.json": `{"type": "object", "properties": {"text": {"type": "string", "minLength": 1, "description": "x"}}, ` +
			`"required": ["text"], "additionalProperties": false}`,
		"note/part.html": `<div data-floe-part="&#110;ote">{{.text}}</div>`,
		"note/part.css":  `[data-floe-part="note"] { margin: 0; }`,
	})
	if _, mistakes := checkOwn(parts, ""); len(mistakes) != 0 {
		t.Errorf("a box named &#110;ote = %v", mistakes)
	}
}
