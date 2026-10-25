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
		`<div data-floe-part="nope"><p>x</p></div>`:                    "data.yaml:3:8: risks: HTML names the part nope",
	} {
		_, err := draw(l, "template: memo\nlang: en\nrisks: "+yamlText(item)+"\n")
		if want == "" && err != nil || want != "" && (err == nil || !strings.HasPrefix(err.Error(), want)) {
			t.Errorf("%q = %v, want %q", item, err, want)
		}
	}
	// A name of no part an item gives is the data's mistake, read in its text, whether page.html draws the item
	// or not.
	if _, err := draw(l, "template: memo\nlang: en\nrisks: x\naside: "+yamlText(`<i data-floe-part="nope">x</i>`)+"\n"); err == nil ||
		!strings.HasPrefix(err.Error(), "data.yaml:4:8: aside: HTML names the part nope") {
		t.Errorf("a name of no part of an item not drawn = %v, want it told at the item", err)
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

// A name page.html draws by what an item holds is asked of the page drawn itself.
func TestDrawnReaderDrawnNames(t *testing.T) {
	t.Parallel()

	l, _ := memoPage(t, `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head><body>`+
		`<div data-floe-part="{{if eq (printf "%s" .Data.risks) "<b>x</b>"}}note table{{else}}note{{end}}">{{.Data.risks}}</div></body></html>`)
	_, err := draw(l, "template: memo\nlang: en\nrisks: <b>x</b>\n")
	if err == nil || !strings.HasPrefix(err.Error(), `data.yaml:1:11: the page has data-floe-part="note table", which is not the name of one part`) {
		t.Errorf("a name drawn by what an item holds = %v", err)
	}
}

// An item's mistakes are told together with another's that page.html puts outside, whichever comes first.
func TestDrawnReaderTogether(t *testing.T) {
	t.Parallel()

	l, _ := memoPage(t, `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head>{{.Data.aside}}`+
		`<body><svg>{{.Data.risks}}</svg></body></html>`)
	_, err := draw(l, "template: memo\nlang: en\nrisks: "+yamlText(`<title><g data-floe-part="note table"></g></title>`)+
		"\naside: <p>x</p>\n")
	if err == nil || !strings.Contains(err.Error(), `risks: the page has data-floe-part="note table"`) ||
		!strings.Contains(err.Error(), "outside its <head> and <body>") {
		t.Errorf("an item's name and another outside = %v", err)
	}
}

// An item not compared with the page, as the page cannot be drawn with the others as marks, leaves the
// items after it uncompared: what it does to the page, which theirs is compared with, is not known.
func TestDrawnReaderNotDrawn(t *testing.T) {
	t.Parallel()

	// Drawn with risks a mark, the others as written, alone.
	cond := `{{if and (gt (len (printf "%s" .Data.risks)) 30) (lt (len (printf "%s" .Data.aside)) 30)}}{{.Missing.x}}{{end}}`
	l, _ := memoPage(t, `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head><body><div>`+
		cond+`{{.Data.risks}}{{.Data.aside}}</div></body></html>`)
	_, err := draw(l, "template: memo\nlang: en\nrisks: <p>x\naside: <p>y</p>\n")
	if err == nil || !strings.Contains(err.Error(), `"Missing"`) || strings.Contains(err.Error(), "aside: HTML changes the page") {
		t.Errorf("an item after one not drawn = %v", err)
	}
}

// A <form> an item leaves open is told after an item page.html puts outside, as it is the item's own.
func TestDrawnReaderFormAfterOutside(t *testing.T) {
	t.Parallel()

	l, _ := memoPage(t, `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head>{{.Data.aside}}`+
		`<body><div>{{.Data.risks}}</div></body></html>`)
	_, err := draw(l, "template: memo\nlang: en\nrisks: <form>x\naside: <p>x</p>\n")
	if err == nil || !strings.Contains(err.Error(), "risks: HTML changes the page") {
		t.Errorf("a <form> left open after an item outside = %v", err)
	}
}

// An item's names are asked as it is read too, wherever page.html puts it: one outside is told them.
func TestDrawnReaderOutsideNames(t *testing.T) {
	t.Parallel()

	l, _ := memoPage(t, `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head>{{.Data.aside}}`+
		`<body><p>{{.Data.risks}}</p></body></html>`)
	_, err := draw(l, "template: memo\nlang: en\nrisks: x\naside: "+yamlText(`<p data-floe-part="note table">x</p>`)+"\n")
	if err == nil || !strings.Contains(err.Error(), `aside: the page has data-floe-part="note table"`) {
		t.Errorf("a name of an item outside = %v", err)
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
		`<template></head></template><body></body></html>`:                               "example.yaml:1:11: the page drawn from this file has no </head>",
		`</head><body><div data-floe-part="note&#32;table"><p>x</p></div></body></html>`: `example.yaml:1:11: the page has data-floe-part="note table", which is not the name of one part`,
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
	l := tmpl.NewLibrary(memo(t, `"risks": {"$ref": "floe:///parts/note.json"}, "aside": {"type": "string"}`), parts, "", "")
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

// The doctype is asked of the page drawn first, when the data has an item of format html: in quirks mode its
// items are not checked, and the page they broke is not asked for its </head>, which would tell them again.
// Without such an item, the doctype is not asked.
func TestDrawnPageDoctypeFirst(t *testing.T) {
	t.Parallel()

	l, _ := memoPage(t, `<html lang="{{.Lang}}"><head><title>{{.Title}}</title>{{if false}}</head>{{end}}<body><p>{{.Data.risks}}</p></body></html>`)
	_, err := draw(l, "template: memo\nlang: en\nrisks: <svg>\n")
	if err == nil || !strings.HasPrefix(err.Error(), "data.yaml:1:11: page.html does not start with <!doctype html>") ||
		strings.Contains(err.Error(), "</head>") {
		t.Errorf("a page in quirks mode = %v", err)
	}
	_, err = draw(l, "template: memo\nlang: en\n")
	if err == nil || !strings.HasPrefix(err.Error(), "data.yaml:1:11: the page drawn from this file has no </head>") ||
		strings.Contains(err.Error(), "doctype") {
		t.Errorf("a page in quirks mode without an item of format html = %v", err)
	}
	// A <style> is asked all the same: it uses neither.
	l, _ = memoPage(t, `<html lang="{{.Lang}}"><head><title>{{.Title}}</title></head><body><style>p {}</style><p>{{.Data.risks}}</p></body></html>`)
	_, err = draw(l, "template: memo\nlang: en\nrisks: <b>x</b>\n")
	if err == nil || !strings.Contains(err.Error(), "does not start with <!doctype html>") || !strings.Contains(err.Error(), "has a <style>") {
		t.Errorf("a page in quirks mode drawing a <style> = %v, want both", err)
	}
}

// What page.html draws is told at the template: of the data file that drew it, wherever that is written:
// the example is drawn by template check alone, so a data file that draws it otherwise passes.
func TestDrawnPageExampleAlone(t *testing.T) {
	t.Parallel()

	l, dir := memoPage(t, `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head><body>`+
		`{{if .Data.aside}}<div data-floe-part="nope"></div>{{end}}<p>x</p></body></html>`)
	if err := os.WriteFile(filepath.Join(dir, "example.yaml"), []byte("lang: en\ntemplate: memo\naside: <b>a</b>\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, mistakes := check(dir, "", "")
	if len(mistakes) != 1 || !strings.HasPrefix(mistakes[0].Error(), filepath.Join(dir, "example.yaml")+":2:11: the page drawn from this file names the part nope") {
		t.Errorf("floe template check = %v", mistakes)
	}
	if _, err := draw(l, "template: memo\nlang: en\n"); err != nil {
		t.Errorf("a data file that does not draw the name = %v", err)
	}
	if _, err := draw(l, "lang: en\n\ntemplate: memo\naside: <b>a</b>\n"); err == nil ||
		!strings.HasPrefix(err.Error(), "data.yaml:3:11: the page drawn from this file names the part nope") {
		t.Errorf("a data file that draws the name = %v", err)
	}
}

// A part that cannot be used, which the example's page names, as only its data puts the name together, is
// told once, as a mistake there, not again as a warning of template check.
func TestDrawnPageBrokenPartOnce(t *testing.T) {
	t.Parallel()

	parts := ownParts(t, map[string]string{"bad/schema.json": `{"type":`, "bad/part.html": `<div data-floe-part="bad"></div>`, "bad/part.css": ""})
	dir := filepath.Join(memo(t, `"kind": {"type": "string"}`), "memo")
	files := map[string]string{
		"page.html":    `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head><body><div data-floe-part="{{.Data.kind}}"></div></body></html>`,
		"example.yaml": "template: memo\nlang: en\nkind: bad\n",
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	warnings, mistakes := check(dir, parts, "")
	if len(mistakes) != 1 || !strings.Contains(mistakes[0].Error(), "bad") {
		t.Errorf("the mistakes = %v, want the part bad", mistakes)
	}
	if slices.ContainsFunc(warnings, func(e *usererr.Error) bool { return strings.Contains(e.Error(), "bad") }) {
		t.Errorf("the warnings = %v, want none of bad", warnings)
	}
}

// A name of no part a part draws is told at its item, as an item of format html's is; the raw SVG of a
// diagram, which has no item to tell it at, is the page's: a <style> it draws is told at the template:.
func TestDrawnPageOfPartsAndDiagrams(t *testing.T) {
	t.Parallel()

	parts := ownParts(t, map[string]string{
		"note/schema.json": `{"type": "object", "properties": {"kind": {"type": "string", "minLength": 1, "description": "x"}}, ` +
			`"required": ["kind"], "additionalProperties": false}`,
		"note/part.html": `<div data-floe-part="note"><span data-floe-part="{{.kind}}">x</span></div>`,
		"note/part.css":  `[data-floe-part="note"] { margin: 0; }`,
	})
	l := tmpl.NewLibrary(memo(t, `"risks": {"$ref": "floe:///parts/note.json"}, "aside": {"type": "string"}`), parts, "", "")
	if _, err := draw(l, "template: memo\nlang: en\nrisks: { kind: nope }\n"); err == nil ||
		!strings.HasPrefix(err.Error(), "data.yaml:3:10: risks: the part draws the name nope from this item, which is no part's") {
		t.Errorf("a name of no part a part draws = %v, want it at the part's item", err)
	}
	raw := "template: report\nlang: en\ntitle: r\nsections:\n  - heading: h\n    parts:\n      - diagram:\n" +
		"          id: d\n          width: 100\n          height: 50\n          title: t\n          parts:\n" +
		`            - raw: { svg: '<style>p {}</style>' }` + "\n"
	if _, err := draw(tmpl.NewLibrary("", "", "", ""), raw); err == nil ||
		!strings.HasPrefix(err.Error(), "data.yaml:1:11: the page drawn from this file has a <style>") {
		t.Errorf("a <style> of a diagram's raw SVG = %v", err)
	}
}
