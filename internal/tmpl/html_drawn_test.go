package tmpl_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/tmpl"
)

// htmlIn returns a library of the template memo, whose page.html draws its item risks of format html
// where body puts {{.Data.risks}}, and aside after it if the data gives one.
func htmlIn(t *testing.T, body string) *tmpl.Library {
	t.Helper()

	dir := memo(t, `"risks": {"type": "string", "format": "html"}, "aside": {"type": "string", "format": "html"}`)
	page := `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head><body>` + body + `</body></html>`
	if err := os.WriteFile(filepath.Join(dir, "memo", "page.html"), []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}
	return tmpl.NewLibrary(dir, "", "")
}

// yamlText writes s as a YAML string.
func yamlText(s string) string {
	b, _ := json.Marshal(s) //nolint:errcheck,errchkjson // a string always encodes
	return string(b)
}

// An item of HTML is refused where it changes the page around it as a browser reads it, which depends on
// where page.html puts it: a <p> left open is fine in a <td>, which its end closes, and not in a <span>.
// What is left as it is passes wherever it is, and an item in a value is no markup.
func TestDrawnHTML(t *testing.T) {
	t.Parallel()

	contexts := []string{
		`<main><section>{{.Data.risks}}<p>after</p></section></main>`,
		`<div><span class=n>{{.Data.risks}}</span> after</div>`,
		`<ul><li>{{.Data.risks}} tail</li></ul>`,
		`<p>{{.Data.risks}} tail</p>`,
		`<table><tr><td>{{.Data.risks}}</td><td>next</td></tr></table>`,
		`<section>{{.Data.risks}}</section><section>next</section><p>x</p>`,
		`<div title="{{.Data.risks}}">x</div>`,
	}
	// By context: . passes, x is refused.
	for item, want := range map[string]string{
		"<b>bold</b>":                    ".......",
		"<p>one<p>two":                   ".xxx...",
		"<p>a":                           ".xxx...",
		"<svg><rect/>":                   "..xx...",
		"<svg><rect/></svg>":             ".......",
		"a <!-- note":                    "xxxxxx.",
		"<textarea>x":                    "xxxxxx.",
		"<table><tr><td>a":               "xxxxxx.",
		"<b>bold":                        "xxxx.x.",
		"<p><a href=x>link":              "xxxx.x.",
		"x</section>":                    "x......",
		"x</div>":                        ".x.....",
		"<li>a</li>":                     "..xx...",
		"<div>d</div>":                   "...x...",
		"<p><b>x</p></b>":                "...x...",
		"<div/>x</div>":                  "...x...",
		"<table><td>x</td></tr></table>": "...x...",
		"<br></br>":                      ".......",
		"<svg><text><![CDATA[ a > b </tspan> ]]></text></svg>": ".......",
		"x</":         "xxxxxx.",
		`<a title="x`: "xxxxxx.",
	} {
		for i, c := range contexts {
			_, err := draw(htmlIn(t, c), "template: memo\nlang: en\nrisks: "+yamlText(item)+"\n")
			if refused := err != nil; refused != (want[i] == 'x') {
				t.Errorf("%q in %s = %v, want refused %v", item, c, err, want[i] == 'x')
			}
			if err != nil && !strings.HasPrefix(err.Error(), "data.yaml:3:8: risks: HTML ") {
				t.Errorf("%q in %s = %v, want a mistake at risks", item, c, err)
			}
		}
	}
}

// A mistake names what an item leaves open, the outermost: an element, a comment, or a tag or a quote;
// one that closes the page's, or changes it after its end, is told so.
func TestDrawnHTMLNames(t *testing.T) {
	t.Parallel()

	l := htmlIn(t, `<ul><li>{{.Data.risks}} tail</li></ul>`)
	for item, want := range map[string]string{
		"<svg><g><rect/>": "HTML changes the page around it",
		"a <!-- note":     "HTML changes the page around it",
		"<textarea>x":     "HTML changes the page around it",
		`<a title="x`:     "HTML changes the page around it",
		"<b>bold":         "HTML changes the page around it",
		"<li>a</li>":      "HTML changes the page around it",
	} {
		if _, err := draw(l, "template: memo\nlang: en\nrisks: "+yamlText(item)+"\n"); err == nil ||
			!strings.HasPrefix(err.Error(), "data.yaml:3:8: risks: "+want) {
			t.Errorf("%q = %v, want %s", item, err, want)
		}
	}
}

// The item that changes the page is told, the second of two; one drawn twice is checked each time; one
// in a <title> is no markup; and one that writes a comment of the marks' form stands in for none.
func TestDrawnHTMLItems(t *testing.T) {
	t.Parallel()

	l := htmlIn(t, `<p>{{.Data.risks}}</p><ul><li>{{with .Data.aside}}{{.}}{{end}} tail</li></ul>`)
	if _, err := draw(l, "template: memo\nlang: en\nrisks: <b>a</b>\naside: <svg>\n"); err == nil ||
		!strings.HasPrefix(err.Error(), "data.yaml:4:8: aside: HTML changes the page around it") {
		t.Errorf("the second item's <svg> = %v", err)
	}
	twice := htmlIn(t, `<section>{{.Data.risks}}</section><p>{{.Data.risks}} tail</p>`)
	if _, err := draw(twice, "template: memo\nlang: en\nrisks: <div>d</div>\n"); err == nil {
		t.Error("an item that changes the page where it is drawn the second time passes")
	}
	dir := memo(t, `"risks": {"type": "string", "format": "html"}`)
	page := `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Data.risks}}</title></head><body><p>x</p></body></html>`
	if err := os.WriteFile(filepath.Join(dir, "memo", "page.html"), []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := draw(tmpl.NewLibrary(dir, "", ""), "template: memo\nlang: en\nrisks: <svg>\n"); err != nil {
		t.Errorf("an item in a <title> = %v", err)
	}
	inTitle := htmlIn(t, `<h1 title="{{.Data.risks}}">t</h1><p>x</p>`)
	if _, err := draw(inTitle, "template: memo\nlang: en\nrisks: <svg>\n"); err != nil {
		t.Errorf("an item in a value = %v", err)
	}
	l = htmlIn(t, `<ul><li>{{.Data.risks}} tail</li></ul>`)
	for _, item := range []string{"<!--/AAAA:0--><svg>", "<!--/AAAA:0-->"} {
		_, err := draw(l, "template: memo\nlang: en\nrisks: "+yamlText(item)+"\n")
		if refused := err != nil; refused != strings.Contains(item, "<svg>") {
			t.Errorf("%q = %v", item, err)
		}
	}
}

// An item page.html draws in the <head> is refused where it ends the head, or hides its </head>; one that
// belongs there passes.
func TestDrawnHTMLInHead(t *testing.T) {
	t.Parallel()

	dir := memo(t, `"risks": {"type": "string", "format": "html"}`)
	page := `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title>{{.Data.risks}}</head><body><p>x</p></body></html>`
	if err := os.WriteFile(filepath.Join(dir, "memo", "page.html"), []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}
	l := tmpl.NewLibrary(dir, "", "")
	for item, want := range map[string]string{
		"<meta name=a content=b>": "",
		"<noscript>x</noscript>":  "",
		"<div>x</div>":            "HTML changes the page around it",
		"hello":                   "HTML changes the page around it",
		"<script>x":               "HTML changes the page around it",
	} {
		_, err := draw(l, "template: memo\nlang: en\nrisks: "+yamlText(item)+"\n")
		if want == "" && err != nil || want != "" && (err == nil || !strings.HasPrefix(err.Error(), "data.yaml:3:8: risks: "+want)) {
			t.Errorf("%q in the head = %v, want %q", item, err, want)
		}
	}
}

// The item told is the first the page draws that changes the page, whatever the order Go keeps them in; the
// page is checked as it is drawn, with its title; and an item that ends an element of the page whose text
// hides its start is told as one that closes an element of the page.
func TestDrawnHTMLWhich(t *testing.T) {
	t.Parallel()

	l := htmlIn(t, `<ul><li>{{.Data.risks}} a</li><li>{{with .Data.aside}}{{.}}{{end}} b</li></ul>`)
	for range 20 {
		if _, err := draw(l, "template: memo\nlang: en\nrisks: <svg>\naside: <svg>\n"); err == nil ||
			!strings.HasPrefix(err.Error(), "data.yaml:3:8: risks: ") {
			t.Fatalf("two items that change the page = %v, want risks", err)
		}
	}
	// aside, later in the file, leaves open a comment or a <b> before risks, which is not told for it.
	before := htmlIn(t, `<p>{{.Data.aside}}</p><p>{{.Data.risks}}</p><p>z</p>`)
	for _, aside := range []string{"a <!-- note", "<b>x"} {
		if _, err := draw(before, "template: memo\nlang: en\nrisks: <b>x</b>\naside: "+yamlText(aside)+"\n"); err == nil ||
			!strings.HasPrefix(err.Error(), "data.yaml:4:8: aside: ") {
			t.Errorf("%q drawn before risks = %v, want aside refused", aside, err)
		}
	}
	titled := htmlIn(t, `{{if eq .Title "t"}}<ul><li>{{.Data.risks}} tail</li></ul>{{else}}<div>{{.Data.risks}}</div>{{end}}`)
	if _, err := draw(titled, "template: memo\nlang: en\nrisks: <b>bold\n"); err == nil {
		t.Error("an item checked under another title than the page's passes")
	}
	for _, page := range []string{`<noscript>{{.Data.risks}}</noscript><p>x</p>`, `<noscript>{{.Data.risks}}</noscript><p>{{.Data.risks}}</p>`} {
		if _, err := draw(htmlIn(t, page), "template: memo\nlang: en\nrisks: x</noscript><b>y</b>\n"); err == nil ||
			!strings.HasPrefix(err.Error(), "data.yaml:3:8: risks: HTML changes the page around it") {
			t.Errorf("an item that ends a <noscript> in %s = %v", page, err)
		}
	}
}

// An item that gives the page's <body> or <html> an attribute changes the page, which may hide it; one
// whose attribute the page's has keeps the page's.
func TestDrawnHTMLAttributes(t *testing.T) {
	t.Parallel()

	l := htmlIn(t, `<div>{{.Data.risks}}</div><p>after</p>`)
	for item, refused := range map[string]bool{
		`<body style="display:none">`: true,
		`<html dir=rtl hidden>`:       true,
		`<p class="a">x</p>`:          false,
	} {
		if _, err := draw(l, "template: memo\nlang: en\nrisks: "+yamlText(item)+"\n"); (err != nil) != refused ||
			refused && !strings.Contains(err.Error(), "HTML changes the page around it") {
			t.Errorf("%q = %v, want refused %v", item, err, refused)
		}
	}
}

// The mark of an item is told whole: the first of eleven, left open, is not taken for the tenth, whose end
// mark the page draws before it.
func TestDrawnHTMLManyItems(t *testing.T) {
	t.Parallel()

	props, body, data := make([]string, 0, 11), make([]string, 0, 11), make([]string, 0, 11)
	for i := range 11 {
		name := fmt.Sprintf("i%02d", i)
		props = append(props, `"`+name+`": {"type": "string", "format": "html"}`)
		data = append(data, name+": x")
	}
	body = append(body, "<p>{{.Data.i10}}</p>")
	for i := range 10 {
		body = append(body, fmt.Sprintf("<p>{{.Data.i%02d}}</p>", i))
	}
	dir := memo(t, strings.Join(props, ", "))
	page := `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head><body>` + strings.Join(body, "") + `</body></html>`
	if err := os.WriteFile(filepath.Join(dir, "memo", "page.html"), []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}
	data[1] = "i01: <textarea>x"
	if _, err := draw(tmpl.NewLibrary(dir, "", ""), "template: memo\nlang: en\n"+strings.Join(data, "\n")+"\n"); err == nil ||
		!strings.Contains(err.Error(), "i01: HTML changes the page around it") {
		t.Errorf("the second of eleven items = %v", err)
	}
}

// An item that ends where a tag starts is refused: the next item's text would go on with the tag, which
// the marks of the drawing that checks it part.
func TestDrawnHTMLTagStart(t *testing.T) {
	t.Parallel()

	l := htmlIn(t, `<div>{{.Data.risks}}{{.Data.aside}}</div><p>after</p>`)
	for _, risks := range []string{"x <", "x </", "x <!", "x <!-", "x <?"} {
		if _, err := draw(l, "template: memo\nlang: en\nrisks: "+yamlText(risks)+"\naside: div class=y\n"); err == nil ||
			!strings.HasPrefix(err.Error(), "data.yaml:3:8: risks: HTML changes the page around it") {
			t.Errorf("%q before a letter = %v", risks, err)
		}
	}
	if _, err := draw(l, "template: memo\nlang: en\nrisks: \"x < y\"\naside: div\n"); err != nil {
		t.Errorf("a < before a space = %v", err)
	}
}

// An item that leaves a <b> or an <a> open is refused where the text it would take in is the next item's,
// with no text of the page's between them, and the item told is the one that leaves it open.
func TestDrawnHTMLNextItem(t *testing.T) {
	t.Parallel()

	for _, page := range []string{
		`<ul><li>{{.Data.risks}}</li><li>{{.Data.aside}}</li></ul>`,
		`<p>{{.Data.risks}}</p><p>{{.Data.aside}}</p>`,
	} {
		for _, risks := range []string{"<b>x", "<a href=x>x"} {
			_, err := draw(htmlIn(t, page), "template: memo\nlang: en\nrisks: "+yamlText(risks)+"\naside: plain\n")
			if err == nil || !strings.HasPrefix(err.Error(), "data.yaml:3:8: risks: ") {
				t.Errorf("%q before another item in %s = %v, want risks refused", risks, page, err)
			}
		}
	}
}

// The other items are drawn as written: a row in a table, which the parser puts in a <tbody> of its own,
// and rows and list items next to each other pass; an item in the <head> after another is checked in the
// head. An item that leaves a <li> open for the next to close is refused (LIMIT in checkDrawnHTML).
func TestDrawnHTMLOthers(t *testing.T) {
	t.Parallel()

	for page, data := range map[string]string{
		"<table>\n{{.Data.risks}}\n</table>":                  "risks: <tr><td>a</td></tr>\n",
		"<table>{{.Data.risks}}{{.Data.aside}}</table>":       "risks: <tr><td>a</td></tr>\naside: <tr><td>b</td></tr>\n",
		"<ul>{{.Data.risks}}{{.Data.aside}}</ul><p>after</p>": "risks: <li>a</li>\naside: <li>b</li>\n",
	} {
		if _, err := draw(htmlIn(t, page), "template: memo\nlang: en\n"+data); err != nil {
			t.Errorf("%s with %q = %v", page, data, err)
		}
	}
	for page, data := range map[string]string{
		"<ul>{{.Data.risks}}{{.Data.aside}}</ul>":       "risks: <li>a\naside: <li>b\n",
		"<table>{{.Data.risks}}{{.Data.aside}}</table>": "risks: <td>a</td>\naside: <td>b</td>\n",
	} {
		if _, err := draw(htmlIn(t, page), "template: memo\nlang: en\n"+data); err == nil {
			t.Errorf("%s with %q, the first left open for the next, passes", page, data)
		}
	}
	dir := memo(t, `"risks": {"type": "string", "format": "html"}, "aside": {"type": "string", "format": "html"}`)
	page := `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title>{{.Data.aside}}{{.Data.risks}}</head><body><p>x</p></body></html>`
	if err := os.WriteFile(filepath.Join(dir, "memo", "page.html"), []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := draw(tmpl.NewLibrary(dir, "", ""), "template: memo\nlang: en\naside: <meta name=a content=b>\nrisks: <div>x</div>\n"); err == nil ||
		!strings.HasPrefix(err.Error(), "data.yaml:4:8: risks: HTML changes the page around it") {
		t.Errorf("a <div> in the head after a <meta> = %v", err)
	}
}

// An item stays in its place: one that closes the page's element around it and opens another is refused,
// though the page without it is the page without it.
func TestDrawnHTMLPlace(t *testing.T) {
	t.Parallel()

	for page, item := range map[string]string{
		"<table><tr><td>{{.Data.risks}}</td><td>next</td></tr></table>": "</td><td>evil",
		"<table><tr><td>a</td>{{.Data.risks}}<td>b</td></tr></table>":   "</tr><tr>",
		"<section>{{.Data.risks}}</section><p>after</p>":                "</section><section>evil",
		"<ul><li>{{.Data.risks}}</li></ul>":                             "</li><li>fake",
		"<div class=card>{{.Data.risks}}</div><p>after</p>":             "</div><div>out",
	} {
		if _, err := draw(htmlIn(t, page), "template: memo\nlang: en\nrisks: "+yamlText(item)+"\n"); err == nil ||
			!strings.HasPrefix(err.Error(), "data.yaml:3:8: risks: HTML changes the page around it") {
			t.Errorf("%q in %s = %v", item, page, err)
		}
	}
}

// Where the parser makes an element itself, around an item or after it, is no change of the page: a page
// that leaves out its <body>, and a cell straight in a <table>.
func TestDrawnHTMLImplied(t *testing.T) {
	t.Parallel()

	dir := memo(t, `"risks": {"type": "string", "format": "html"}`)
	page := `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head>{{.Data.risks}}<p>x</p></html>`
	if err := os.WriteFile(filepath.Join(dir, "memo", "page.html"), []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := draw(tmpl.NewLibrary(dir, "", ""), "template: memo\nlang: en\nrisks: hello\n"); err != nil {
		t.Errorf("text where the <body> starts = %v", err)
	}
	if _, err := draw(htmlIn(t, `<table>{{.Data.risks}}</table>`), "template: memo\nlang: en\nrisks: <td>z</td>\n"); err != nil {
		t.Errorf("a cell straight in a table = %v", err)
	}
}

// What the parser reads as text is the item's text, however html/template writes it there: a script's,
// a <textarea>'s with a character reference, a <pre>'s after the newline the parser drops. One that ends a
// <noscript> is refused (TestDrawnHTMLWhich).
func TestDrawnHTMLAsText(t *testing.T) {
	t.Parallel()

	for page, items := range map[string][]string{
		`<script type="application/json">{{.Data.risks}}</script><p>x</p>`: {"hello", "<b>a</b>"},
		`<script>var x = {{.Data.risks}};</script><p>x</p>`:                {"hello", "</script><b>"},
		`<textarea>{{.Data.risks}}</textarea><p>x</p>`:                     {"Q&amp;A", "<b>"},
		`<pre>{{.Data.risks}}</pre><p>x</p>`:                               {"\nfoo", "\n\nfoo", "a\nb"},
	} {
		for _, item := range items {
			if _, err := draw(htmlIn(t, page), "template: memo\nlang: en\nrisks: "+yamlText(item)+"\n"); err != nil {
				t.Errorf("%q in %s = %v", item, page, err)
			}
		}
	}
}

// An empty item is drawn as it is, so that page.html tests it as the page does: the item checked is where
// the page draws it.
func TestDrawnHTMLEmpty(t *testing.T) {
	t.Parallel()

	l := htmlIn(t, `{{if .Data.aside}}<table><tr><td>{{.Data.risks}}</td></tr></table>{{else}}<span>{{.Data.risks}}</span> tail{{end}}`)
	if _, err := draw(l, "template: memo\nlang: en\naside: \"\"\nrisks: <p>a\n"); err == nil ||
		!strings.HasPrefix(err.Error(), "data.yaml:4:8: risks: ") {
		t.Errorf("an item drawn where an empty one leads page.html = %v", err)
	}
}

// An item that hides the page's </head> is told as the data's: the page with it has no </head>.
func TestDrawnHTMLHidesHead(t *testing.T) {
	t.Parallel()

	dir := memo(t, `"risks": {"type": "string", "format": "html"}, "aside": {"type": "string", "format": "html"}`)
	page := `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title>{{.Data.aside}}{{.Data.risks}}</head><body><p>x</p></body></html>`
	if err := os.WriteFile(filepath.Join(dir, "memo", "page.html"), []byte(page), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := draw(tmpl.NewLibrary(dir, "", ""), "template: memo\nlang: en\naside: <meta name=a content=b>\nrisks: <script>x\n"); err == nil ||
		!strings.HasPrefix(err.Error(), "data.yaml:4:8: risks: ") {
		t.Errorf("a <script> that hides </head> = %v", err)
	}
}

// Looking for the item that hides the </head>, page.html draws the items not yet added as marks, taking
// the branches it takes with them; one that fails so looks into an item, and is told so. The item told
// is the first the page draws: aside's <script>, which takes in risks' comment.
func TestDrawnHTMLHidesHeadMarked(t *testing.T) {
	t.Parallel()

	dir := memo(t, `"risks": {"type": "string", "format": "html"}`)
	page := filepath.Join(dir, "memo", "page.html")
	for action, want := range map[string]string{
		`{{if not .Data.risks}}{{index .Lang 9}}{{end}}`:         "data.yaml:3:8: risks: HTML changes the page around it",
		`{{if gt (len .Data.risks) 20}}{{index .Lang 9}}{{end}}`: "looks into an item of format html",
	} {
		src := `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title>{{.Data.risks}}` + action + `</head><body><p>x</p></body></html>`
		if err := os.WriteFile(page, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := draw(tmpl.NewLibrary(dir, "", ""), "template: memo\nlang: en\nrisks: <script>x\n"); err == nil ||
			!strings.Contains(err.Error(), want) {
			t.Errorf("%s = %v, want %s", action, err, want)
		}
	}
	dir = memo(t, `"risks": {"type": "string", "format": "html"}, "aside": {"type": "string", "format": "html"}`)
	src := `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title>{{.Data.aside}}{{.Data.risks}}</head><body><p>x</p></body></html>`
	if err := os.WriteFile(filepath.Join(dir, "memo", "page.html"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := draw(tmpl.NewLibrary(dir, "", ""), "template: memo\nlang: en\nrisks: <!-- c\naside: <script>\n"); err == nil ||
		!strings.HasPrefix(err.Error(), "data.yaml:4:8: aside: ") {
		t.Errorf("a <script> drawn before a comment = %v, want aside", err)
	}
}

// An item is checked in the <head> or the <body>: one between them is the body's, with the attributes of
// the page's <body>; page.html putting one after the </body> is refused, as is one looking into an item.
func TestDrawnHTMLWherePageHasIt(t *testing.T) {
	t.Parallel()

	dir := memo(t, `"risks": {"type": "string", "format": "html"}`)
	page := filepath.Join(dir, "memo", "page.html")
	head := `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title></head>`
	for body, want := range map[string]string{
		`{{.Data.risks}}<body class="r"><p>x</p></body></html>`:                       "",
		`<body class="r"><p>x</p></body>{{.Data.risks}}</html>`:                       "outside its <head> and <body>",
		`<body><p>{{slice .Data.risks 0 40}}</p><p>{{.Data.risks}}</p></body></html>`: "looks into an item of format html",
	} {
		if err := os.WriteFile(page, []byte(head+body), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := draw(tmpl.NewLibrary(dir, "", ""), "template: memo\nlang: en\nrisks: <p>hello there, a paragraph longer than a mark</p>\n")
		if want == "" && err != nil || want != "" && (err == nil || !strings.Contains(err.Error(), want)) {
			t.Errorf("%s = %v, want %q", body, err, want)
		}
	}
}

// An item that ends what the parser made itself in the page, as the page's own would, passes: a <tfoot>
// after the rows. One that leaves a <form> open, whose inputs the page's after it would be, is refused.
func TestDrawnHTMLTableAndForm(t *testing.T) {
	t.Parallel()

	foot := htmlIn(t, `<table><tr><td>a</td></tr>{{.Data.risks}}</table>`)
	if _, err := draw(foot, "template: memo\nlang: en\nrisks: <tfoot><tr><td>s</td></tr></tfoot>\n"); err != nil {
		t.Errorf("a <tfoot> after the rows = %v", err)
	}
	l := htmlIn(t, `<div>{{.Data.risks}}</div><input name=q><button>go</button>`)
	for item, refused := range map[string]bool{
		"<form action=//e.x><input>":                  true,
		"<form action=//e.x><input></form>":           false,
		"<svg><form></form></svg><p>x</p>":            false,
		"<form action=//e.x></form><form action=//f>": true,
		"<template><form></form><form></template>":    false,
	} {
		if _, err := draw(l, "template: memo\nlang: en\nrisks: "+yamlText(item)+"\n"); (err != nil) != refused {
			t.Errorf("%q = %v, want refused %v", item, err, refused)
		}
	}
}

// An action of page.html that leaves out its </head> is told as the template's, before the data's items.
func TestDrawnHTMLAfterTheTemplate(t *testing.T) {
	t.Parallel()

	dir := memo(t, `"risks": {"type": "string", "format": "html"}`)
	page := filepath.Join(dir, "memo", "page.html")
	src := `<!doctype html><html lang="{{.Lang}}"><head><title>{{.Title}}</title>{{.Data.risks}}{{if false}}</head>{{end}}<body><p>x</p></body></html>`
	if err := os.WriteFile(page, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := draw(tmpl.NewLibrary(dir, "", ""), "template: memo\nlang: en\nrisks: <div>x</div>\n"); err == nil ||
		!strings.HasPrefix(err.Error(), "an action of "+page+" leaves out its </head>") {
		t.Errorf("a page.html whose action leaves out </head> = %v", err)
	}
}

// An item left open at the end of the page takes in the scripts floe puts after it, and is refused too.
func TestDrawnHTMLAtTheEnd(t *testing.T) {
	t.Parallel()

	if _, err := draw(htmlIn(t, `<p>x</p>{{.Data.risks}}`), "template: memo\nlang: en\nrisks: <svg>\n"); err == nil ||
		!strings.Contains(err.Error(), "HTML changes the page around it") {
		t.Errorf("an <svg> at the end = %v", err)
	}
}
