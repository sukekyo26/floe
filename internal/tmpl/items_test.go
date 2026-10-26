package tmpl_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/sukekyo26/floe/internal/tmpl"
)

// htmlSchema is a template's schema of an item h of format html, and t of plain text.
const htmlSchema = `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"h": {"type": "string", "format": "html"}, "g": {"type": "string", "format": "html"}, "t": {}}}`

// htmlPage is a page that draws h in a section's div, as report does.
func htmlPage(body string) string {
	return "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head>\n<body>\n" + body +
		"\n</body></html>\n"
}

// quoted is s in YAML's double quotes.
func quoted(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// An item of format html that changes the page drawn around it is refused at it, saying what to close; one that
// changes none is drawn as written, with no mark of floe's left in the page.
func TestHTMLItems(t *testing.T) {
	t.Parallel()

	section := htmlPage("<section><div>{{.Data.h}}</div><p>after</p></section>")
	for _, tt := range []struct {
		page, h, want string
	}{
		{section, `<p>a <b>b</b></p><svg viewBox="0 0 1 1"><path d="M0 0"/></svg><br><img src="x">`, ""},
		{section, "<DIV>x</div>", ""},
		{section, "<table><tr><td>x</table>", ""},
		{section, "<ul><li>a<li>b</ul>", ""},
		{section, "<b><i>x</b></i>", ""},
		{section, "</br>", ""},
		{section, "<template><b></template>", ""},
		{section, "<image src=x>", ""},
		{section, `<svg><circle r="1"/><foreignObject><div>x</div></foreignObject></svg>`, ""},
		{section, "<textarea><b></textarea>", ""},
		{section, "<script>console.log(1)</script>", ""},
		{section, "x <", ""},
		{section, `<!-- <meta charset="utf-8"> -->`, ""},
		{section, `<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Inter">`, ""},
		{section, "<div><b>x</div>", "d.yaml:3:4: HTML changes the page around it at <b>: close in it each element and comment it opens, and close no element it did not open"},
		{section, "<p><b>x</p>", "d.yaml:3:4: HTML changes the page around it at <b>: close in it each element and comment it opens, and close no element it did not open"},
		{section, "<div/>x", "d.yaml:3:4: HTML changes the page around it at <div>: close in it each element and comment it opens, and close no element it did not open"},
		{section, "x</section>", "d.yaml:3:4: HTML changes the page around it at </div> </section>: close in it each element and comment it opens, and close no element it did not open"},
		{section, "x <!-- y", "d.yaml:3:4: HTML changes the page around it at <!--: close in it each element and comment it opens, and close no element it did not open"},
		{section, `<b class="x>y</b>`, "d.yaml:3:4: HTML changes the page around it at <b class=\"x>y</b>: close in it each element and comment it opens, and close no element it did not open"},
		{section, "<script>x", "d.yaml:3:4: HTML changes the page around it at <script>: close in it each element and comment it opens, and close no element it did not open"},
		{section, "<svg><![CDATA[></svg>", "d.yaml:3:4: HTML changes the page around it at <svg>: close in it each element and comment it opens, and close no element it did not open"},
		{section, "<svg><title><!--</title></svg>", "d.yaml:3:4: HTML changes the page around it at <!--: close in it each element and comment it opens, and close no element it did not open"},
		{section, "<svg><script></section></script></svg>", "d.yaml:3:4: HTML changes the page around it at </div> </section>: close in it each element and comment it opens, and close no element it did not open"},
		{section, "<svg><title><b>x</title></svg>", "d.yaml:3:4: HTML changes the page around it at <svg> <title> <b>: close in it each element and comment it opens, and close no element it did not open"},
		{section, "a</b", "d.yaml:3:4: HTML changes the page around it at </b: close in it each element and comment it opens, and close no element it did not open"},
		{section, "<!DOCTYPE x", "d.yaml:3:4: HTML changes the page around it at <!DOCTYPE x: close in it each element and comment it opens, and close no element it did not open"},
		{section, `<META http-equiv="refresh" content="0">`, `d.yaml:2:11: the page drawn from this file has <meta http-equiv="refresh" content="0"/>, which page.html, a part's part.html or an item of format html writes: a page floe draws is static, and takes a <meta> for its charset or its viewport alone; leave it out`},
		{section, `<svg><title><meta http-equiv="refresh" content="0"></title></svg>`, `d.yaml:2:11: the page drawn from this file has <meta http-equiv="refresh" content="0"/>, which page.html, a part's part.html or an item of format html writes: a page floe draws is static, and takes a <meta> for its charset or its viewport alone; leave it out`},
		{section, "<style>p{}</style>", "d.yaml:2:11: the page drawn from this file has a <style>, which page.html, a part's part.html or an item of format html writes: write the CSS in the template's page.css, which floe puts into the page in a layer under the parts' and the theme's, or in the part's part.css"},
		{section, "<noscript><p>x</p></noscript>", "d.yaml:2:11: the page drawn from this file has a <noscript>, which page.html, a part's part.html or an item of format html writes: floe does not check what it holds, which works where scripts are off; leave it out"},
		{section, `<span data-floe-part="nope">x</span>`, "d.yaml:3:4: the page drawn from this file names the part nope, which there is none of; use bars, cards, cell, checklist, code, compare, decision, details, diagram, diff, facts, faq, fields, findings, glossary, keys, links, matrix, note, options, progress, risks, script, stack, stats, steps, table, tabs, terminal, timeline, tree"},
		{htmlPage("<p>{{.Data.h}}</p>"), "<table><tr><td>x</td></tr></table>", "d.yaml:3:4: HTML changes the page around it at </p>: close in it each element and comment it opens, and close no element it did not open"},
		{htmlPage("<p><b>{{.Data.t}}</p><div>{{.Data.h}}</div>"), "plain", ""},
		{htmlPage("<table><tbody>{{.Data.h}}</tbody></table>"), "<tr><td>x</td></tr>", ""},
		{htmlPage("<table><tbody>{{.Data.h}}</tbody></table>"), `<tr><td>x</td></tr><meta http-equiv="refresh" content="0">`, `d.yaml:2:11: the page drawn from this file has <meta http-equiv="refresh" content="0"/>, which page.html, a part's part.html or an item of format html writes: a page floe draws is static, and takes a <meta> for its charset or its viewport alone; leave it out`},
		{htmlPage("<svg><foreignObject>{{.Data.h}}</foreignObject></svg><p>after</p>"), `<meta http-equiv="refresh" content="0">`, `d.yaml:2:11: the page drawn from this file has <meta http-equiv="refresh" content="0"/>, which page.html, a part's part.html or an item of format html writes: a page floe draws is static, and takes a <meta> for its charset or its viewport alone; leave it out`},
		{htmlPage("<textarea>{{.Data.h}}</textarea>"), "<b>x</b>", "d.yaml:2:11: page.html puts the item h of format html in a text or an attribute of <textarea>, where HTML does not work: put it where elements go, or give the item another format"},
		{htmlPage("<xmp>{{.Data.h}}</xmp>"), "</xmp>x", "d.yaml:2:11: page.html puts the item h of format html in a text or an attribute of <xmp>, where HTML does not work: put it where elements go, or give the item another format"},
		{htmlPage(`<div>{{.Data.h | printf "%s"}}</div>`), "<b>x</b>", "d.yaml:2:11: page.html puts the item h of format html in a text or an attribute of <div>, where HTML does not work: put it where elements go, or give the item another format"},
		{htmlPage("<div>{{.Data.h}}</div><div>{{.Data.h}}</div>"), "<b>x</b>", ""},
		{htmlPage(`<a href="{{.Data.h}}">a</a>`), "<b>x</b>", "d.yaml:2:11: page.html puts the item h of format html in a text or an attribute of <a>, where HTML does not work: put it where elements go, or give the item another format"},
		{htmlPage(`<div title="{{.Data.h}}">a</div>`), "<b>x</b>", "d.yaml:2:11: page.html puts the item h of format html in a text or an attribute of <div>, where HTML does not work: put it where elements go, or give the item another format"},
		{htmlPage("<script>var h = {{.Data.h}};</script>"), "<b>x</b>", "d.yaml:2:11: page.html puts the item h of format html in a text or an attribute of <script>, where HTML does not work: put it where elements go, or give the item another format"},
		{section, "a\n</p x=", "d.yaml:3:4: HTML changes the page around it at </p x=: close in it each element and comment it opens, and close no element it did not open"},
		{section, `<meta name="referrer" content="no-referrer">`, `d.yaml:2:11: the page drawn from this file has <meta name="referrer" content="no-referr…, which page.html, a part's part.html or an item of format html writes: a page floe draws is static, and takes a <meta> for its charset or its viewport alone; leave it out`},
		{section, `<meta name="VIEWPORT" content="width=device-width">`, ""},
		{section, `<p title="&#9;&#10; &#9;">x</p><p title="&#9;&#10;  &#9;">y</p>`, ""},
		{section, "<meta name=\"description\" content=\"a\nb\">", `d.yaml:2:11: the page drawn from this file has <meta name="description" content="a b"/>, which page.html, a part's part.html or an item of format html writes: a page floe draws is static, and takes a <meta> for its charset or its viewport alone; leave it out`},
	} {
		k := ownKit(t, nil, mine(tt.page, htmlSchema, nil))
		data := "lang: en\ntemplate: mine\nh: " + quoted(tt.h) + "\nt: t\n"
		page, err := tmpl.Draw(k, "d.yaml", []byte(data), none)
		got := ""
		if err != nil {
			got = told(err)
		}
		if got != tt.want {
			t.Errorf("%s in %q: told\n%s\nwant\n%s", tt.h, tt.page, got, tt.want)
		}
		// The probes and the marks, comments, are taken out.
		if err == nil && (!strings.Contains(tt.h, "&#9;") && bytes.Contains(page, []byte("\t\n \t")) || !strings.Contains(tt.h, "<!--") && bytes.Contains(page, []byte("<!--"))) {
			t.Errorf("%s: a mark is left in the page\n%s", tt.h, page)
		}
	}
}

// Items are looked at in the order of the page: one that page.html puts in a <title> is refused, though it is
// drawn rightly elsewhere too, and past the first item that changes the page, none is.
func TestHTMLItemsInOrder(t *testing.T) {
	t.Parallel()

	k := ownKit(t, nil, mine(htmlPage("<div>{{.Data.h}}</div><div>{{.Data.g}}</div>"), htmlSchema, nil))
	if got, want := drawn(t, k, "lang: en\ntemplate: mine\nh: x<textarea>\ng: ok\n", none),
		"d.yaml:3:4: HTML changes the page around it at <textarea>: close in it each element and comment it opens, and close no element it did not open"; got != want {
		t.Errorf("told\n%s\nwant\n%s", got, want)
	}
	k = ownKit(t, nil, mine(strings.Replace(htmlPage("<div>{{.Data.h}}</div>"), "{{.Title}}", "{{.Data.h}}", 1), htmlSchema, nil))
	if got, want := drawn(t, k, "lang: en\ntemplate: mine\nh: <b>x</b>\n", none),
		"d.yaml:2:11: page.html puts the item h of format html in a text or an attribute of <title>, where HTML does not work: put it where elements go, or give the item another format"; got != want {
		t.Errorf("told\n%s\nwant\n%s", got, want)
	}
	k = ownKit(t, nil, mine(htmlPage("<div>{{.Data.h}}</div><div>{{.Data.g}}</div>"), htmlSchema, nil))
	if got, want := drawn(t, k, "lang: en\ntemplate: mine\nh: <b>x</b>\ng: a</b\n", none),
		"d.yaml:4:4: HTML changes the page around it at </b: close in it each element and comment it opens, and close no element it did not open"; got != want {
		t.Errorf("told\n%s\nwant\n%s", got, want)
	}
}

// A mistake page.html makes with an item of format html, as a function's, tells the item without the marks and
// the probes floe draws around it.
func TestHTMLItemInFunction(t *testing.T) {
	t.Parallel()

	k := ownKit(t, nil, mine(htmlPage("<div>{{num .Data.h}}</div>"), htmlSchema, nil))
	if got, want := drawn(t, k, "lang: en\ntemplate: mine\nh: <b>x</b>\n", none),
		"templates/mine/page.html:4:8: num takes a number, not <b>x</b> (template.HTML)"; got != want {
		t.Errorf("told\n%s\nwant\n%s", got, want)
	}
}

// An item of a format of floe's that is not text is refused at it, where the schema does not say its type.
func TestFormatNotText(t *testing.T) {
	t.Parallel()

	k := ownKit(t, nil, mine(htmlPage("<div>{{.Data.h}}{{.Data.i}}{{.Data.m}}</div>"), `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"h": {"format": "html"}, "i": {"format": "image"}, "m": {"format": "markdown"}}}`, nil))
	if got, want := drawn(t, k, "lang: en\ntemplate: mine\nh: 5\ni: [1]\nm: true\n", none), lines(
		"d.yaml:3:4: an item of format html takes text: write it as text, in quotes where YAML would read a number, true, false or null",
		"d.yaml:4:4: an item of format image takes text: write it as text, in quotes where YAML would read a number, true, false or null",
		"d.yaml:5:4: an item of format markdown takes text: write it as text, in quotes where YAML would read a number, true, false or null",
	); got != want {
		t.Errorf("told\n%s\nwant\n%s", got, want)
	}
}

// png is the least PNG http.DetectContentType takes as one.
var png = []byte("\x89PNG\r\n\x1a\n0000000000000000")

// An image the data names under its directory goes into the page as a data URI; one that names no file floe
// reads is refused at the item, and one that cannot be read at its path, once.
func TestImageItems(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for name, b := range map[string][]byte{
		"ok.png": png, ".hidden.png": png, "sub/in.png": png, "large.png": bytes.Repeat([]byte{0}, 1<<20+1),
		"ok.svg": []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), "x.bmp": png,
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, link := range [][2]string{{"ok.png", "in.png"}, {"/etc/hostname", "esc.png"}} {
		if err := os.Symlink(link[0], filepath.Join(dir, link[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "dir.png"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "fifo.png"), 0o600); err != nil {
		t.Fatal(err)
	}
	k := ownKit(t, nil, mine(htmlPage(`{{range .Data.i}}<img src="{{.}}">{{end}}`), `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"i": {"type": "array", "items": {"type": "string", "format": "image"}}}}`, nil))
	o := tmpl.Options{Theme: "", Untitled: "", ImagesDir: dir}
	page, err := tmpl.Draw(k, "x/d.yaml", []byte("lang: en\ntemplate: mine\ni: [ok.png, ok.svg, .hidden.png, in.png, sub/in.png]\n"), o)
	if err != nil {
		t.Fatalf("told\n%s", told(err))
	}
	if !bytes.Contains(page, []byte(`src="data:image/png;base64,iVBORw0KGgowMDAw`)) || !bytes.Contains(page, []byte(`src="data:image/svg+xml;base64,`)) {
		t.Errorf("no data URI in\n%s", page)
	}
	_, err = tmpl.Draw(k, "d.yaml", []byte("lang: en\ntemplate: mine\ni: [./ok.png]\n"), o)
	if got, want := told(err), "d.yaml:3:5: \"./ok.png\" is not a path floe reads under the directory of d.yaml: write it relative to that directory, with one / between names, without ./ (write pic.png for ./pic.png), .., or a / at either end"; got != want {
		t.Errorf("told\n%s\nwant\n%s", got, want)
	}
	data := "lang: en\ntemplate: mine\ni:\n  - missing.png\n  - missing.png\n  - esc.png\n  - dir.png\n  - fifo.png\n  - x.bmp\n  - ./ok.png\n  - ../ok.png\n  - large.png\n"
	_, err = tmpl.Draw(k, "x/d.yaml", []byte(data), o)
	if got, want := told(err), lines(
		"x/missing.png: no such file or directory",
		"x/esc.png: path escapes from parent",
		"x/d.yaml:7:5: \"dir.png\" is not a file of an image, but a directory or the like: name a regular file of the image",
		"x/d.yaml:8:5: \"fifo.png\" is not a file of an image, but a directory or the like: name a regular file of the image",
		"x/d.yaml:9:5: \"x.bmp\" is not an image floe takes: use png, jpeg, gif, webp or svg",
		"x/d.yaml:10:5: \"./ok.png\" is not a path floe reads under the directory of x/d.yaml: write it relative to that directory, with one / between names, without ./ (write pic.png for ./pic.png), .., or a / at either end",
		"x/d.yaml:11:5: \"../ok.png\" is not a path floe reads under the directory of x/d.yaml: write it relative to that directory, with one / between names, without ./ (write pic.png for ./pic.png), .., or a / at either end",
		"x/d.yaml:12:5: the image \"large.png\" is 1,048,577 bytes, over the 1,048,576 bytes (1 MiB) an image may be; make it smaller",
	); got != want {
		t.Errorf("told\n%s\nwant\n%s", got, want)
	}
}
