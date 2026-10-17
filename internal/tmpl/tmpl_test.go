package tmpl_test

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// builtins are the built-in templates alone.
var builtins = tmpl.NewLibrary("", "")

func render(t *testing.T, src string) string {
	t.Helper()
	d, err := builtins.Read("data.yaml", []byte(src), "", "")
	if err != nil {
		t.Fatal(err)
	}
	b, err := d.Render("A title & more")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The report's example draws: Markdown text, HTML as it is, and the header from the data.
func TestReportExample(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("builtin/report/example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	d, err := builtins.Read("example.yaml", src, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if d.Template != "report" || d.Theme != "default" || d.Title != "Moving the team wiki" {
		t.Errorf("Read = %+v", d)
	}
	if d, err := builtins.Read("data.yaml", []byte("template: report\ntitle: ' T '\nlang: en\n"), "", ""); err != nil || d.Title != "T" {
		t.Errorf("Read of a title with spaces = %+v, %v", d, err)
	}
	got := render(t, string(src))
	for _, want := range []string{
		`<html lang="en">`,
		`<title>A title &amp; more</title>`,
		`<h1>A title &amp; more</h1>`,
		`<p class="eyebrow">Plan · 2030-04</p>`,
		`<h2>Steps</h2>`,
		`<p>The pages move over <strong>one weekend</strong>; the old wiki stays readable until the new one opens.</p>`,
		`<li class="doing" aria-current="step"><strong>Copy the pages and check the links</strong></li>`,
		`<tr class="same"><td class="what">Address</td>`,
		`<div class="floe-table"><table>`,
		`<th scope="col">Likelihood</th>`,
		`<td>Keep the old host for <strong>a month</strong></td>`,
		`<td class="num">120</td>`,
		`<td><span class="floe-pill warn">watching</span></td>`,
		`<div class="floe-card"><h3>Copy</h3><p>Copies the pages on <strong>Saturday</strong> and keeps a list of what failed.</p>`,
		`<div class="floe-note warn"><strong class="floe-note-title">Heads up</strong><p>Anyone editing during the freeze loses their changes.</p>`,
		`<details><summary>What the freeze covers</summary>`,
		`aria-label="The pages go from the old wiki to the new host"`,
		`--accent: #1d6670;`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the example lacks %s:\n%s", want, got)
		}
	}
	// The style of each part it uses is in its <head> once, so that a part is styled as it is drawn.
	head, _, _ := strings.Cut(got, "</head>")
	theme, rerr := os.ReadFile("builtin/report/themes/default.css")
	if rerr != nil {
		t.Fatal(rerr)
	}
	for _, style := range []string{".floe-table {", ".floe-cards {", ".floe-note {"} {
		if strings.Count(head, style) != 1+strings.Count(string(theme), style) || strings.Count(got, style) != strings.Count(head, style) {
			t.Errorf("the example holds %s %d times, %d in its <head>", style, strings.Count(got, style), strings.Count(head, style))
		}
	}
}

// Items the schema declares may be left out, and only html takes HTML: Markdown leaves it out.
func TestReportParts(t *testing.T) {
	t.Parallel()

	got := render(t, "template: report\nsections:\n  - parts:\n      - text: 'a <script>x()</script> <table> `</table>` **b**'\n  - heading: h\nlang: en\n")
	if strings.Contains(got, "<script>x()") || !strings.Contains(got, "<strong>b</strong>") || strings.Contains(got, `<div class="floe-table">`) {
		t.Errorf("Markdown with raw HTML = %s", got)
	}
	if strings.Contains(got, `class="eyebrow"`) || strings.Contains(got, `class="lead"`) {
		t.Errorf("an eyebrow or a lead is drawn without one: %s", got)
	}
	if strings.Contains(got, "dg-wrap") {
		t.Errorf("a page without diagrams holds their zoom: %s", got)
	}
}

// A table's cell is a line of Markdown: what would start a block stays text, its lines are joined,
// and raw HTML is left out; a pill is its text in its tone. A num column is aligned right. A card
// without text and a note without a tone leave them out.
func TestReportTableCardsNote(t *testing.T) {
	t.Parallel()

	cells := []struct{ yaml, html string }{
		{`"-"`, "-"},
		{`"1."`, "1."},
		{`"# x"`, "# x"},
		{`"> q"`, "&gt; q"},
		{`"1.50"`, "1.50"},
		{`"a <b>c</b> ~~d~~ [e](https://e.example)"`, `a <!-- raw HTML omitted -->c<!-- raw HTML omitted --> <del>d</del> <a href="https://e.example">e</a>`},
		{`"    120"`, "120"},
		{`"\tt"`, "t"},
		{`"**a\n\n  b**"`, "<strong>a\nb</strong>"},
		{`"c  \nd"`, "c\nd"},
		{`" \n "`, ""},
		{`""`, ""},
		{`{ pill: "#40 <b>", tone: good }`, `<span class="floe-pill good">#40 &lt;b&gt;</span>`},
		{`{ pill: x }`, `<span class="floe-pill">x</span>`},
		{`{ pill: y, tone: accent }`, `<span class="floe-pill accent">y</span>`},
	}
	src := "template: report\nsections:\n  - parts:\n      - table:\n          columns: [{title: Cell}, {title: '**N**', num: true}]\n          rows:\n"
	want := make([]string, 0, len(cells)+3)
	want = append(want, `<thead><tr><th scope="col">Cell</th><th scope="col" class="num"><strong>N</strong></th></tr></thead>`)
	for _, c := range cells {
		src += "            - [" + c.yaml + ", '9']\n"
		want = append(want, "<tr><td>"+c.html+`</td><td class="num">9</td></tr>`)
	}
	src += "      - cards:\n          - title: A & B\n      - note:\n          text: n\n      - note: { text: m, tone: good }\nlang: en\n"
	want = append(want, `<div class="floe-card"><h3>A &amp; B</h3></div>`, `<div class="floe-note"><p>n</p>`,
		`<div class="floe-note good"><p>m</p>`)
	got := render(t, src)
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("the page lacks %s:\n%s", w, got)
		}
	}
	// A number would lose how it was written (1.50 as 1.5, 1234567.5 as 1.2345675e+06): a cell is text or
	// a pill, which has its text and a tone floe knows.
	// A part with nothing in it is a slip.
	for src, want := range map[string]string{
		"table: { columns: [{title: a}], rows: [[1.50]] }": "data.yaml:4:49: sections[0].parts[0].table.rows[0][0]: write text or keys and values, not a number",
		"table: { columns: [{title: a}], rows: [[{tone: good}], [{pill: x, tone: ok}]] }": "data.yaml:4:50: sections[0].parts[0].table.rows[0][0].pill is required but missing\n" +
			`data.yaml:4:81: sections[0].parts[0].table.rows[1][0].tone: "ok" is not allowed; use accent, good, warn, bad`,
		"table: { columns: [], rows: [] }": "data.yaml:4:27: sections[0].parts[0].table.columns: the number of items must be at least 1\n" +
			"data.yaml:4:37: sections[0].parts[0].table.rows: the number of items must be at least 1",
		"table: { columns: [{title: a}, {title: b}], rows: [[x, y], [x]] }": "data.yaml:4:59: sections[0].parts[0].table.rows: write as many cells in each row as there are columns",
		"note: { text: '' }": "data.yaml:4:23: sections[0].parts[0].note.text: the length in characters must be at least 1",
	} {
		_, err := builtins.Read("data.yaml", []byte("template: report\nsections:\n  - parts:\n      - "+src+"\nlang: en\n"), "", "")
		if err == nil || err.Error() != want {
			t.Errorf("Read of %s = %v, want %s", src, err, want)
		}
	}
}

// A script is an element to draw in and its function called with it and its data, the text of JSON
// that no text ends the script in, parsed; a ; or a comment after the function still parses. A js the
// page would read the end of its script in is refused, as is a script without js or with keys it does
// not take.
func TestReportScript(t *testing.T) {
	t.Parallel()

	got := render(t, "template: report\nsections:\n  - parts:\n      - script:\n          data: { a: '</script><b>&', n: 1.50 }\n"+
		"          js: |\n            function (el, data) { el.textContent = data.a; }; // a comment\n"+
		"      - script:\n          js: (el) => el.append('x');\nlang: en\n")
	for _, want := range []string{
		"<div></div><script>{const floeScript = function (el, data) { el.textContent = data.a; }; // a comment\n\n" +
			";floeScript(document.currentScript.previousElementSibling, " +
			`JSON.parse("{\"a\":\"\\u003c/script\\u003e\\u003cb\\u003e\\u0026\",\"n\":1.5}"));}</script>`,
		"<div></div><script>{const floeScript = (el) => el.append('x');\n" +
			`;floeScript(document.currentScript.previousElementSibling, JSON.parse("null"));}</script>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the page lacks %s:\n%s", want, got)
		}
	}
	for src, want := range map[string]string{
		"script: { js: f, run: 1 }":                  "data.yaml:4:26: sections[0].parts[0].script.run: a script takes js and data, not run",
		"script: { data: 1 }":                        "data.yaml:4:19: sections[0].parts[0].script: write js, the function to run with the element it draws in and data: function (el, data) { … }",
		"script: { js: 1 }":                          "data.yaml:4:23: sections[0].parts[0].script.js: write js, the function to run with the element it draws in and data: function (el, data) { … }",
		"script: { js: ' ' }":                        "data.yaml:4:23: sections[0].parts[0].script.js: write js, the function to run with the element it draws in and data: function (el, data) { … }",
		"script: { js: \"f('</SCRIPT>')\" }":         "data.yaml:4:23: sections[0].parts[0].script.js: js holds </SCRIPT, where the page would end its script; split it in a string, as '<' + '/SCRIPT'",
		"script: { js: \"f('<!--', '</script>')\" }": "data.yaml:4:23: sections[0].parts[0].script.js: js holds <!--, where the page would end its script; split it in a string, as '<' + '!--'",
	} {
		_, err := builtins.Read("data.yaml", []byte("template: report\nsections:\n  - parts:\n      - "+src+"\nlang: en\n"), "", "")
		if err == nil || err.Error() != want {
			t.Errorf("Read of %s = %v, want %s", src, err, want)
		}
	}
}

// Each refusal of Read points at the line and column it is about.
func TestReadRefusals(t *testing.T) {
	t.Parallel()

	for _, c := range []struct{ src, want string }{
		{"title: t\nlang: en\n", "data.yaml:1:1: name the template at the top level, like template: report (templates: report)"},
		{"", "data.yaml:1:1: name the language of the page at the top level"},
		{"template: ''\nlang: en\n", "data.yaml:1:11: name the template at the top level"},
		{"- template: report\n", "data.yaml:1:1: write the top level as keys and values"},
		{"template: report\n---\ntitle: t\nlang: en\n", "data.yaml:2:1: a data file holds one YAML document"},
		{"template: report\n...\nlead: x\nlang: en\n", "data.yaml:3:1: a data file holds one YAML document"},
		{"template: report\ntemplate: report\nlang: en\n", `data.yaml:2:1: this is not valid YAML: mapping key "template" already defined`},
		{"template: report\n---\ntitle: 'a\nlang: en\n", "data.yaml:3:8: this is not valid YAML: "},
		{"template: report\nlead: &a x\nlang: en\n", "data.yaml:2:7: anchors, aliases, merge keys (<<) and tags are not supported"},
		{"template: report\n<<: {lead: x}\nlang: en\n", "data.yaml:2:1: anchors, aliases, merge keys (<<) and tags are not supported"},
		{"template:\nlang: en\n", "data.yaml:1:1: template must be text"},
		{"template: report\n0x10: x\nlang: en\n", "data.yaml:2:1: a key is a name: write 0x10 as text, in quotes"},
		{"template: report\nsections:\n  - ~: x\nlang: en\n", "data.yaml:3:5: a key is a name: write ~ as text, in quotes"},
		{"template: report\nsections:\n  -\nlang: en\n", "data.yaml:3:3: sections[0]: write keys and values, not nothing"},
		{"template: report\nsections: [~]\nlang: en\n", "data.yaml:2:12: sections[0]: write keys and values, not nothing"},
		{"template: report\nlead: .inf\nlang: en\n", "data.yaml:2:7: .inf is not a value a data file can hold"},
		{"template: report\nsections:\n  - heading: .NaN\nlang: en\n", "data.yaml:3:14: .NaN is not a value a data file can hold"},
		// The shared keys and the schema are reported together.
		{"template: report\ntheme: print\nleed: x\nlang: en\n", "data.yaml:2:8: template report has no theme \"print\"; use default, editorial, ink, soft, tinted\ndata.yaml:3:1: leed is not an item"},
		{"template: deck\nlang: en\n", `data.yaml:1:11: unknown template "deck"; use report`},
		{"template: report\ntheme: print\nlang: en\n", `data.yaml:2:8: template report has no theme "print"; use default, editorial, ink, soft, tinted`},
		{"template: report\ntitle: [a]\nlang: en\n", "data.yaml:2:8: title must be text"},
		{"template: report\ntitle: 'a\nlang: en\n", "data.yaml:2:8: this is not valid YAML: "},
	} {
		_, err := builtins.Read("data.yaml", []byte(c.src), "", "")
		var one *yamlpos.Error
		var all yamlpos.Problems
		if !errors.As(err, &one) && !errors.As(err, &all) || !strings.HasPrefix(err.Error(), c.want) {
			t.Errorf("Read %q = %v, want %s", c.src, err, c.want)
		}
	}
	// An empty file misses both keys every data file needs.
	if _, err := builtins.Read("data.yaml", nil, "", ""); err == nil || !strings.Contains(err.Error(), "\ndata.yaml:1:1: name the template at the top level") {
		t.Errorf("Read of an empty file = %v", err)
	}
	// An explicit key is found by its text after ?.
	if _, err := builtins.Read("data.yaml", []byte("template: report\n? lead\n: 3\nlang: en\n"), "", ""); err == nil || err.Error() != "data.yaml:3:3: lead: write text, not a number" {
		t.Errorf("Read of an explicit key = %v", err)
	}
	if _, err := builtins.Read("data.yaml", []byte("%YAML 1.2\n---\ntemplate: report\nlang: en\n"), "", ""); err != nil {
		t.Errorf("Read of a document after a %%YAML directive = %v", err)
	}
	// Data that passed its schema draws; a template that does not fit it is refused while drawing.
	d, err := builtins.Read("data.yaml", []byte("template: report\nlang: en\n"), "", "")
	if err != nil {
		t.Fatal(err)
	}
	d.Values = map[string]any{"sections": "no"}
	if _, err := d.Render("t"); err == nil || !strings.HasPrefix(err.Error(), "data.yaml does not fit the template report: ") {
		t.Errorf("Render of data of the wrong shape = %v", err)
	}
}

// Every mistake against the schema is reported at once, in the order of the file, each at its line
// and column, worded in the reader's language.
func TestSchemaMistakes(t *testing.T) {
	t.Parallel()

	src := `template: report
title: Bad
leed: a typo
sections:
  - heading: 1
    parts:
      - txt: hello
      - text: a
        html: <b>b</b>
      - {}
  - parts: nope
lang: en
`
	_, err := builtins.Read("bad.yaml", []byte(src), "", "")
	var p yamlpos.Problems
	if !errors.As(err, &p) {
		t.Fatalf("Read = %v, want problems", err)
	}
	want := []string{
		"bad.yaml:3:1: leed is not an item of the template report (a typo?); use eyebrow, lang, lead, sections, template, theme, title",
		"bad.yaml:5:14: sections[0].heading: write text, not a number",
		"bad.yaml:7:9: sections[0].parts[0].txt is not an item of the template report (a typo?); use cards, checklist, code, compare, diagram, diff, facts, html, links, note, options, progress, script, stats, steps, table, terminal, text, tree",
		"bad.yaml:8:9: sections[0].parts[1]: the number of keys must be at most 1",
		"bad.yaml:10:9: sections[0].parts[2]: the number of keys must be at least 1",
		"bad.yaml:11:12: sections[1].parts: write a list, not text",
	}
	if got := p.Error(); got != strings.Join(want, "\n") {
		t.Errorf("the mistakes =\n%s\nwant\n%s", got, strings.Join(want, "\n"))
	}
	if ja := p.Localize(i18n.New("ja")); !strings.Contains(ja, "bad.yaml:11:12: sections[1].parts: 文字列ではなくリストで書いてください") {
		t.Errorf("the mistakes in Japanese:\n%s", ja)
	}
}

// Every built-in template has a page, a schema, an example that draws, and a default theme, and passes
// floe template check but for the template.yaml a built-in one does without.
func TestBuiltins(t *testing.T) {
	t.Parallel()

	names, err := builtins.Names()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		for _, f := range []string{"page.html", "schema.json", "example.yaml", "themes/default.css"} {
			if _, err := fs.Stat(os.DirFS("builtin/"+name), f); err != nil {
				t.Errorf("template %s: %v", name, err)
			}
		}
		src, err := os.ReadFile("builtin/" + name + "/example.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if render(t, string(src)) == "" {
			t.Errorf("the example of %s draws nothing", name)
		}
		if warnings, mistakes := tmpl.Check("builtin/"+name, ""); len(warnings) != 1 || len(mistakes) != 0 {
			t.Errorf("Check of %s = %v, %v", name, warnings, mistakes)
		}
	}
}

// A data file without its template is told so beside the other mistakes of the keys every template
// shares, and a theme: that is not text leaves the theme the default one.
func TestUnnamedBesideOtherMistakes(t *testing.T) {
	t.Parallel()

	_, err := tmpl.NewLibrary("", "").Read("d.yaml", []byte("title: 3\ntheme: [x]\nlang: ja_JP\n"), "", "")
	want := "d.yaml:1:1: name the template at the top level, like template: report (templates: report)\n" +
		"d.yaml:1:8: title must be text\nd.yaml:2:8: theme must be text\n" +
		`d.yaml:3:7: invalid language "ja_JP": write lang: as a BCP 47 tag such as en, ja or pt-BR`
	if err == nil || err.Error() != want {
		t.Errorf("Read = %v\nwant %s", err, want)
	}
	_, err = tmpl.NewLibrary("", "").Read("d.yaml", []byte("template: report\ntheme: [x]\nlang: en\n"), "", "")
	if want = "d.yaml:2:8: theme must be text"; err == nil || err.Error() != want {
		t.Errorf("Read of a theme not text = %v\nwant %s", err, want)
	}
	// The language is the data's own: one left out or not text is a mistake.
	for src, want := range map[string]string{
		"template: report\n":             "d.yaml:1:1: name the language of the page at the top level, like lang: en (a BCP 47 tag such as en, ja or pt-BR)",
		"template: report\nlang: [ja]\n": "d.yaml:2:7: lang must be text",
	} {
		if _, err := tmpl.NewLibrary("", "").Read("d.yaml", []byte(src), "", ""); err == nil || err.Error() != want {
			t.Errorf("Read %q = %v\nwant %s", src, err, want)
		}
	}
	// It goes into <html lang> as it is, so only a BCP 47 tag passes.
	for _, bad := range []string{"", "x", "ja_JP", "en-", `ja"><script>`} {
		src := "template: report\nlang: '" + bad + "'\n"
		want := `d.yaml:2:7: invalid language "` + bad + `": write lang: as a BCP 47 tag such as en, ja or pt-BR`
		if _, err := tmpl.NewLibrary("", "").Read("d.yaml", []byte(src), "", ""); err == nil || err.Error() != want {
			t.Errorf("Read %q = %v\nwant %s", src, err, want)
		}
	}
}
