package tmpl_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/sukekyo26/floe/internal/files"
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
		{section, `<span data-floe-part="nope">x</span>`, "d.yaml:3:4: the page drawn from this file names the part nope, which there is none of; use bars, cards, cell, checklist, code, compare, decision, details, diagram, diff, facts, faq, fields, findings, glossary, image, keys, links, matrix, note, options, progress, risks, script, stack, stats, steps, table, tabs, terminal, timeline, tree"},
		{htmlPage("<p>{{.Data.h}}</p>"), "<table><tr><td>x</td></tr></table>", "d.yaml:3:4: HTML changes the page around it at </p>: close in it each element and comment it opens, and close no element it did not open"},
		{htmlPage("<p><b>{{.Data.t}}</p><div>{{.Data.h}}</div>"), "plain", ""},
		{htmlPage("<table><tbody>{{.Data.h}}</tbody></table>"), "<tr><td>x</td></tr>", ""},
		{htmlPage("<table><tbody>{{.Data.h}}</tbody></table>"), `<tr><td>x</td></tr><meta http-equiv="refresh" content="0">`, `d.yaml:2:11: the page drawn from this file has <meta http-equiv="refresh" content="0"/>, which page.html, a part's part.html or an item of format html writes: a page floe draws is static, and takes a <meta> for its charset or its viewport alone; leave it out`},
		{htmlPage("<svg><foreignObject>{{.Data.h}}</foreignObject></svg><p>after</p>"), `<meta http-equiv="refresh" content="0">`, `d.yaml:2:11: the page drawn from this file has <meta http-equiv="refresh" content="0"/>, which page.html, a part's part.html or an item of format html writes: a page floe draws is static, and takes a <meta> for its charset or its viewport alone; leave it out`},
		{htmlPage("<textarea>{{.Data.h}}</textarea>"), "<b>x</b>", "d.yaml:2:11: page.html, or the part.html of a part holding it, puts the item h of format html in a text or an attribute of <textarea>, where HTML does not work: put it, or the part holding it, where elements go, or give the item another format"},
		{htmlPage("<xmp>{{.Data.h}}</xmp>"), "</xmp>x", "d.yaml:2:11: page.html, or the part.html of a part holding it, puts the item h of format html in a text or an attribute of <xmp>, where HTML does not work: put it, or the part holding it, where elements go, or give the item another format"},
		{htmlPage(`<div>{{.Data.h | printf "%s"}}</div>`), "<b>x</b>", "d.yaml:2:11: page.html, or the part.html of a part holding it, puts the item h of format html in a text or an attribute of <div>, where HTML does not work: put it, or the part holding it, where elements go, or give the item another format"},
		{htmlPage("<div>{{.Data.h}}</div><div>{{.Data.h}}</div>"), "<b>x</b>", ""},
		{htmlPage(`<a href="{{.Data.h}}">a</a>`), "<b>x</b>", "d.yaml:2:11: page.html, or the part.html of a part holding it, puts the item h of format html in a text or an attribute of <a>, where HTML does not work: put it, or the part holding it, where elements go, or give the item another format"},
		{htmlPage(`<div title="{{.Data.h}}">a</div>`), "<b>x</b>", "d.yaml:2:11: page.html, or the part.html of a part holding it, puts the item h of format html in a text or an attribute of <div>, where HTML does not work: put it, or the part holding it, where elements go, or give the item another format"},
		{htmlPage("<script>var h = {{.Data.h}};</script>"), "<b>x</b>", "d.yaml:2:11: page.html, or the part.html of a part holding it, puts the item h of format html in a text or an attribute of <script>, where HTML does not work: put it, or the part holding it, where elements go, or give the item another format"},
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
		"d.yaml:2:11: page.html, or the part.html of a part holding it, puts the item h of format html in a text or an attribute of <title>, where HTML does not work: put it, or the part holding it, where elements go, or give the item another format"; got != want {
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

// placed is the mistake of the item at of format html put where HTML does not work, in the element in.
func placed(at, in string) string {
	return "d.yaml:2:11: page.html, or the part.html of a part holding it, puts the item " + at + " of format html in a text or an attribute of " + in + ", where HTML does not work: put it, or the part holding it, where elements go, or give the item another format"
}

// changes is the mistake of an item of format html at pos that changes the page around it at where.
func changes(pos, where string) string {
	return "d.yaml:" + pos + ": HTML changes the page around it at " + where + ": close in it each element and comment it opens, and close no element it did not open"
}

// A part's schema takes floe's formats as a template's does: its items of format html are checked in the page
// drawn as a template's are, at their place in the part's value, though the part's part.html begins or ends with
// one, the part is in another part, or the item is white space alone; Markdown is drawn as markdown draws it.
func TestPartHTMLItems(t *testing.T) {
	t.Parallel()

	const head = `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "properties": `
	partFiles := func(name, schema, page string) map[string]string {
		return map[string]string{name + "/schema.json": head + schema + "}", name + "/part.html": page, name + "/part.css": ""}
	}
	parts := func(ph, po string) map[string]string {
		all := partFiles("ph", `{"h": {"type": "string", "format": "html"}, "md": {"type": "string", "format": "markdown"}}`, ph)
		for k, v := range partFiles("po", `{"in": {"$ref": "floe:///parts/ph.json"}}`, po) {
			all[k] = v
		}
		return all
	}
	schema := head + `{"p": {"$ref": "floe:///parts/ph.json"}, "o": {"$ref": "floe:///parts/po.json"}}}`
	whole := `<div data-floe-part="ph">{{.h}}<section>{{.md}}</section></div>`
	for _, tt := range []struct {
		name, ph, po, body, data, want, drawn string
	}{
		{"drawn", whole, "{{.in}}", "{{.Data.p}}<p>after</p>", `p: {h: "<b>ok</b>", md: "**bold** <i>raw</i>"}`, "", "<b>ok</b><section><p><strong>bold</strong> "},
		{"a style", whole, "{{.in}}", "{{.Data.p}}<p>after</p>", `p: {h: "<style>p{}</style>"}`, "d.yaml:2:11: the page drawn from this file has a <style>, which page.html, a part's part.html or an item of format html writes: write the CSS in the template's page.css, which floe puts into the page in a layer under the parts' and the theme's, or in the part's part.css", ""},
		{"left open", whole, "{{.in}}", "{{.Data.p}}<p>after</p>", `p: {h: "<b>x"}`, changes("3:8", "<b>"), ""},
		{"only the item, an element after", "{{.h}}", "{{.in}}", "<div>{{.Data.p}}<p>after</p></div>", `p: {h: "<b>ok</b>"}`, "", "<div><b>ok</b><p>after</p></div>"},
		{"only the item, text after", "{{.h}}", "{{.in}}", "<div>{{.Data.p}}after</div>", `p: {h: "<b>ok</b>"}`, "", "<div><b>ok</b>after</div>"},
		{"only the item, in formatting", "{{.h}}", "{{.in}}", "<b>{{.Data.p}}</b>", `p: {h: "<b>ok</b>"}`, "", "<b><b>ok</b></b>"},
		{"the item last", "<p>a</p>{{.h}}", "{{.in}}", "<div>{{.Data.p}}<p>after</p></div>", `p: {h: "<b>ok</b>"}`, "", "<div><p>a</p><b>ok</b><p>after</p></div>"},
		{"only the item, left open", "{{.h}}", "{{.in}}", "<div>{{.Data.p}}<p>after</p></div>", `p: {h: "<b>x"}`, changes("3:8", "<b>"), ""},
		{"in a part in a part", "{{.h}}", "{{.in}}", "<div>{{.Data.o}}<p>after</p></div>", `o: {in: {h: "<b>ok</b>"}}`, "", "<div><b>ok</b><p>after</p></div>"},
		{"in a part in a part, left open", "{{.h}}", "{{.in}}", "<div>{{.Data.o}}<p>after</p></div>", `o: {in: {h: "<b>x"}}`, changes("3:13", "<b>"), ""},
		{"white space alone", "{{.h}}", "{{.in}}", "<div>a{{.Data.p}}b</div>", `p: {h: "  "}`, "", "<div>a  b</div>"},
		{"in an attribute by part.html", `<div data-floe-part="ph" title="{{.h}}">x</div>`, "{{.in}}", "{{.Data.p}}", `p: {h: "<b>x</b>"}`, placed("p.h", "<div>"), ""},
		{"in an attribute by page.html", whole, "{{.in}}", `<p title="{{.Data.p}}">x</p>`, `p: {h: "<b>x</b>"}`, placed("p.h", "<p>"), ""},
		{"in a function", `<div data-floe-part="ph">{{num .h}}</div>`, "{{.in}}", "{{.Data.p}}", `p: {h: "<b>x</b>"}`, "d.yaml:3:5: num takes a number, not <b>x</b> (template.HTML)", ""},
	} {
		k := ownKit(t, parts(tt.ph, tt.po), mine(htmlPage(tt.body), schema, nil))
		page, err := tmpl.Draw(k, "d.yaml", []byte("lang: en\ntemplate: mine\n"+tt.data+"\n"), none)
		got := ""
		if err != nil {
			got = told(err)
		}
		if got != tt.want {
			t.Errorf("%s: told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
		if err == nil && !bytes.Contains(page, []byte(tt.drawn)) {
			t.Errorf("%s: the page does not hold %q\n%s", tt.name, tt.drawn, page)
		}
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
	images := files.NewRoot(dir)
	t.Cleanup(func() { _ = images.Close() })
	o := tmpl.Options{Theme: "", Untitled: "", Images: images, ImagesDir: ""}
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

// A directory of images that cannot be opened is told once, at it, however many images the data names; a page
// that names none draws.
func TestImagesDirUnreadable(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root opens a directory it may not read")
	}
	dir := filepath.Join(t.TempDir(), "x")
	if err := os.Mkdir(dir, 0o311); err != nil {
		t.Fatal(err)
	}
	k := ownKit(t, nil, mine(htmlPage(`{{range .Data.i}}<img src="{{.}}">{{end}}`), `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"i": {"type": "array", "items": {"type": "string", "format": "image"}}}}`, nil))
	images := files.NewRoot(dir)
	t.Cleanup(func() { _ = images.Close() })
	o := tmpl.Options{Theme: "", Untitled: "", Images: images, ImagesDir: ""}
	_, err := tmpl.Draw(k, "x/d.yaml", []byte("lang: en\ntemplate: mine\ni: [a.png, b.png]\n"), o)
	if got, want := told(err), "x: permission denied"; got != want {
		t.Errorf("told\n%s\nwant\n%s", got, want)
	}
	if _, err := tmpl.Draw(k, "x/d.yaml", []byte("lang: en\ntemplate: mine\ni: []\n"), o); err != nil {
		t.Errorf("a page of no image: told\n%s", told(err))
	}
}

// An HTML <select> may hold no element whose content is read as text, nor svg or math, written by an item or
// by page.html: a parser of the old "in select" mode reads them apart, its select closed by a tag held as text
// here (<input>, <keygen>), so that a <style> or a <meta> lives there unchecked. What closes the select in an
// item is told by the item's marks first; a <style> or a <noscript> by its own words alone.
func TestSelectText(t *testing.T) {
	t.Parallel()

	in := func(name string) string {
		return "d.yaml:2:11: the page drawn from this file has <" + name + "> inside a <select>, which may hold no element whose content is read as text, nor svg or math: move it out of the <select>"
	}
	style := "d.yaml:2:11: the page drawn from this file has a <style>, which page.html, a part's part.html or an item of format html writes: write the CSS in the template's page.css, which floe puts into the page in a layer under the parts' and the theme's, or in the part's part.css"
	section := htmlPage("<section><div>{{.Data.h}}</div><p>after</p></section>")
	for _, tt := range []struct{ h, want string }{
		{"<select><xmp><input><style>main{display:none}</style></xmp></select>", in("xmp")},
		{"<select><xmp><keygen><meta http-equiv=refresh content=0></xmp></select>", in("xmp")},
		{"<select><svg><![CDATA[><input><style>main{display:none}</style>]]></svg></select>", in("svg")},
		{"<select><title><style>x{}</style></title></select>", in("title")},
		{"<select><textarea>x</textarea></select>", in("textarea")},
		{"<select><script>1</script></select>", in("script")},
		{"<select><option><xmp>x</xmp></option></select>", in("xmp")},
		{"<SELECT><IFRAME>x</IFRAME></SELECT>", in("iframe")},
		{"<select><math><mi>x</mi></math></select>", in("math")},
		{`<select><option><svg width="8" height="8"><circle r="4"/></svg>a</option></select>`, in("svg")},
		{"<svg><foreignObject><select><xmp>x</xmp></select></foreignObject></svg>", in("xmp")},
		{"<select><template><xmp>x</xmp></template></select>", in("xmp")},
		{"<select><noembed>x</noembed></select>", in("noembed")},
		{"<select><noframes>x</noframes></select>", in("noframes")},
		{"<SELECT><XMP><style>x{}</style></XMP></SELECT>", in("xmp")},
		{"<select><div><xmp>x</xmp></div></select>", in("xmp")},
		{"<table><tr><td><select><xmp>x</xmp></select></td></tr></table>", in("xmp")},
		{"<select><svg><foreignObject><xmp>x</xmp></foreignObject></svg></select>", in("svg") + "\n" + in("xmp")},
		{"<select><style>x{}</style></select>", style},
		{"<select><noscript>x</noscript></select>", "d.yaml:2:11: the page drawn from this file has a <noscript>, which page.html, a part's part.html or an item of format html writes: floe does not check what it holds, which works where scripts are off; leave it out"},
		{"<select><plaintext>x</select>", "d.yaml:3:4: HTML changes the page around it at <plaintext>: close in it each element and comment it opens, and close no element it did not open"},
		{"<select><xmp></select><meta http-equiv=refresh content=0></xmp>", "d.yaml:3:4: HTML changes the page around it at <select>: close in it each element and comment it opens, and close no element it did not open"},
		{`<select><option>a</option><optgroup label="g"><option>b</option></optgroup></select>`, ""},
		{"<xmp><b>x</b></xmp>", ""},
		{"<svg><select><xmp>x</xmp></select></svg>", ""},
	} {
		k := ownKit(t, nil, mine(section, htmlSchema, nil))
		_, err := tmpl.Draw(k, "d.yaml", []byte("lang: en\ntemplate: mine\nh: "+quoted(tt.h)+"\nt: t\n"), none)
		got := ""
		if err != nil {
			got = told(err)
		}
		if got != tt.want {
			t.Errorf("%s: told\n%s\nwant\n%s", tt.h, got, tt.want)
		}
	}
	// What page.html writes, the closed select and the CDATA that html/template escapes, too.
	for body, want := range map[string]string{
		"<select><plaintext><style>x{}</style></select>":                             in("plaintext"),
		"<select><xmp></select><meta http-equiv=refresh content=0></xmp>":            in("xmp"),
		"<select><svg><![CDATA[><style>main{display:none}</style>]]></svg></select>": in("svg") + "\n" + style,
	} {
		page := "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>t</title></head>\n<body>" + body + "</body></html>\n"
		k := ownKit(t, nil, mine(page, mineSchema, nil))
		if got := drawn(t, k, "lang: en\ntemplate: mine\n", none); got != want {
			t.Errorf("page.html %s: told\n%s\nwant\n%s", body, got, want)
		}
	}
	// And what a part's part.html writes.
	k := ownKit(t, map[string]string{
		"sel/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
		"sel/part.html":   `<div data-floe-part="sel"><select><xmp>x</xmp></select></div>`,
		"sel/part.css":    "",
	}, mine("<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>t</title></head>\n<body>{{.Data.sel}}</body></html>\n",
		`{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "properties": {"sel": {"$ref": "floe:///parts/sel.json"}}}`, nil))
	if got := drawn(t, k, "lang: en\ntemplate: mine\nsel: {}\n", none); got != in("xmp") {
		t.Errorf("part.html: told\n%s\nwant\n%s", got, in("xmp"))
	}
}

// A value html/template does not take where page.html, a part's part.html or a diagram puts it, which it writes
// as ZgotmplZ without an error, is told at template: with the start tag of that attribute alone: in a style, in
// a name or a part of one, in a URL, in a value without quotes, in a <meta> the page may have. Not where
// html/template writes no ZgotmplZ (text, script, on…), nor a name of a heading's link, lower case.
func TestZgotmplZ(t *testing.T) {
	t.Parallel()

	in := func(tag string) string {
		return "d.yaml:2:11: the page drawn from this file has ZgotmplZ in " + tag + ": html/template wrote it for a value it does not take there; change the value, or where the template puts it"
	}
	const schema = `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"c": {"type": "string", "format": "html"}, "s": {}}}`
	page := func(head, body string) string {
		return "<!doctype html>\n<html lang=\"{{.Lang}}\"><head>" + head + "<title>t</title></head>\n<body>" + body + "</body></html>\n"
	}
	href := in(`<a href="#ZgotmplZ">`)
	for _, tt := range []struct{ head, body, s, want string }{
		{"", `<p style="color: {{.Data.c}}">x</p>`, "", in(`<p style="color: ZgotmplZ">`)},
		{"", `<p {{.Data.c}}>x</p>`, "", in(`<p zgotmplz="">`)},
		{"", `<p style="color: {{.Data.s}}">x</p>`, "red; background: url(x)", in(`<p style="color: ZgotmplZ">`)},
		{"", `<p style="font-family: {{.Data.s}}">x</p>`, `"Noto Sans"`, in(`<p style="font-family: ZgotmplZ">`)},
		{"", `<a href="{{.Data.s}}">x</a>`, "javascript:alert(1)", href},
		{"", `<a href="{{.Data.s}}">x</a>`, " https://x", href},
		{"", `<a href="{{.Data.s}}">x</a>`, "#a:b", href},
		{"", `<a href="{{.Data.s}}">x</a>`, "?q=a:b", href},
		{"", `<a href="{{.Data.s}}">x</a>`, "a b:c", href},
		{"", `<a href="{{.Data.s}}">x</a>`, "\thttp://x", href},
		{"", `<img srcset="{{.Data.s}} 2x" alt="">`, "javascript:x", in(`<img srcset="#ZgotmplZ 2x"/>`)},
		{"", `<p title={{.Data.s}}>x</p>`, "", in(`<p title="ZgotmplZ">`)},
		{"", `<p {{.Data.s}}="x">x</p>`, "href", in(`<p zgotmplz="x">`)},
		{"", `<p {{.Data.s}}="x">x</p>`, "data-x", in(`<p zgotmplz="x">`)},
		{"", `<p data-{{.Data.s}}="x">x</p>`, "<b>", in(`<p data-zgotmplz="x">`)},
		{"<meta name=viewport content={{.Data.s}}>", "", "", in(`<meta content="ZgotmplZ"/>`)},
		{"", `<div class="callout" data-floe-x="note" style="color: {{.Data.s}}"><p>x</p></div>`, "a;b", in(`<div style="color: ZgotmplZ">`)},
		{"", "<p style=\"margin: 0; padding: 0;\n color: {{.Data.s}}\">x</p>", "a;b", in(`<p style="margin: 0; padding: 0;  color: ZgotmplZ">`)},
		{"", `<svg width="8" height="8"><circle r="4" style="fill: {{.Data.s}}"/></svg>`, "a;b", in(`<circle style="fill: ZgotmplZ">`)},
		// One mistake for an element refused already.
		{"<meta http-equiv=refresh content={{.Data.s}}>", "", "", `d.yaml:2:11: the page drawn from this file has <meta http-equiv="refresh" content="Zgot…, which page.html, a part's part.html or an item of format html writes: a page floe draws is static, and takes a <meta> for its charset or its viewport alone; leave it out`},
		// Written as it is: refused all the same.
		{"", `<p title="{{.Data.s}}">x</p>`, "ZgotmplZ", in(`<p title="ZgotmplZ">`)},
		{"", `<p data-zgotmplz-x="1">x</p>`, "", in(`<p data-zgotmplz-x="1">`)},
		// Where html/template writes no ZgotmplZ.
		{"", `<p>{{.Data.s}}</p>`, "ZgotmplZ", ""},
		{"", `<a href="#{{.Data.s}}">x</a>`, "zgotmplz", ""},
		{"", `<p style="font-family: {{.Data.s}}">x</p>`, "Noto Sans, serif", ""},
		{"", `<script>var a = {{.Data.s}};</script>`, "x", ""},
		{"", `<p onclick="f({{.Data.s}})">x</p>`, "x", ""},
		{"", `<xmp><p title="ZgotmplZ"></p></xmp>`, "", ""},
	} {
		k := ownKit(t, nil, mine(page(tt.head, tt.body), schema, nil))
		data := "lang: en\ntemplate: mine\nc: \"<b>red</b>\"\ns: " + quoted(tt.s) + "\n"
		if got := drawn(t, k, data, none); got != tt.want {
			t.Errorf("%s%s with %q: told\n%s\nwant\n%s", tt.head, tt.body, tt.s, got, tt.want)
		}
	}
	// What a part's part.html writes.
	k := ownKit(t, map[string]string{
		"st/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
		"st/part.html":   `<div data-floe-part="st" style="color: {{.s}}">x</div>`,
		"st/part.css":    "",
	}, mine(page("", "{{.Data.st}}"),
		`{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "properties": {"st": {"$ref": "floe:///parts/st.json"}}}`, nil))
	if got, want := drawn(t, k, "lang: en\ntemplate: mine\nst: {s: \"a;b\"}\n", none), in(`<div style="color: ZgotmplZ">`); got != want {
		t.Errorf("part.html: told\n%s\nwant\n%s", got, want)
	}
	// floe's links, which leaves URLs to this, two told once, and a diagram's title.
	report := "lang: en\ntemplate: report\ntitle: t\nsections:\n  - heading: h\n    parts:\n"
	for src, want := range map[string]string{
		report + "      - links: [{title: x, url: ' https://x'}, {title: y, url: 'javascript:x'}]\n":                             href,
		report + "      - links: [{title: x, url: 'https://x'}, {title: y, url: 'p/a.html#b'}]\n":                                "",
		diagramPage("en", `{width: 300, height: 100, title: ZgotmplZ, parts: [{box: {x: 20, y: 20, w: 80, h: 40, rows: [A]}}]}`): in(`<svg aria-label="ZgotmplZ">`),
	} {
		if got := drawn(t, floeKit(t), src, none); got != want {
			t.Errorf("%s: told\n%s\nwant\n%s", src, got, want)
		}
	}
}
