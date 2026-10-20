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
		"<svg><g><rect/>": "HTML leaves <svg> open",
		"a <!-- note":     "HTML leaves <!-- open",
		"<textarea>x":     "HTML leaves <textarea> open",
		`<a title="x`:     "HTML ends in a tag or a quote",
		"<b>bold":         "HTML leaves <b> open",
		"<li>a</li>":      "HTML closes an element of the page, or changes the page after it",
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
		!strings.HasPrefix(err.Error(), "data.yaml:4:8: aside: HTML leaves <svg> open") {
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
		"<div>x</div>":            "HTML closes an element of the page, or changes the page after it",
		"hello":                   "HTML closes an element of the page, or changes the page after it",
		"<script>x":               "HTML leaves <script> open",
	} {
		_, err := draw(l, "template: memo\nlang: en\nrisks: "+yamlText(item)+"\n")
		if want == "" && err != nil || want != "" && (err == nil || !strings.HasPrefix(err.Error(), "data.yaml:3:8: risks: "+want)) {
			t.Errorf("%q in the head = %v, want %q", item, err, want)
		}
	}
}

// The item told is the first in the file that changes the page, whatever the order Go keeps them in; the
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
	titled := htmlIn(t, `{{if eq .Title "t"}}<ul><li>{{.Data.risks}} tail</li></ul>{{else}}<div>{{.Data.risks}}</div>{{end}}`)
	if _, err := draw(titled, "template: memo\nlang: en\nrisks: <b>bold\n"); err == nil {
		t.Error("an item checked under another title than the page's passes")
	}
	for _, page := range []string{`<noscript>{{.Data.risks}}</noscript><p>x</p>`, `<noscript>{{.Data.risks}}</noscript><p>{{.Data.risks}}</p>`} {
		if _, err := draw(htmlIn(t, page), "template: memo\nlang: en\nrisks: x</noscript><b>y</b>\n"); err == nil ||
			!strings.HasPrefix(err.Error(), "data.yaml:3:8: risks: HTML closes an element of the page") {
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
			refused && !strings.Contains(err.Error(), "HTML closes an element of the page, or changes the page after it") {
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
		!strings.Contains(err.Error(), "i01: HTML leaves <textarea> open") {
		t.Errorf("the second of eleven items = %v", err)
	}
}

// An item left open at the end of the page takes in the scripts floe puts after it, and is refused too.
func TestDrawnHTMLAtTheEnd(t *testing.T) {
	t.Parallel()

	if _, err := draw(htmlIn(t, `<p>x</p>{{.Data.risks}}`), "template: memo\nlang: en\nrisks: <svg>\n"); err == nil ||
		!strings.Contains(err.Error(), "HTML leaves <svg> open") {
		t.Errorf("an <svg> at the end = %v", err)
	}
}
