package browsertest_test

import (
	"os"
	"strings"
	"testing"
)

// rowParts are the parts drawn as rows, a row each that is a line: a table with a number column, a Markdown table,
// the parts that take the table's look, those that take the look rows, facts' tiles and an element of html that takes
// the look rows.
const rowParts = `  - table:
      columns: [Name, {title: Count, num: true}, State]
      rows:
        - [table, '12', { pill: done, tone: good }]
        - [cards, '3', open]
  - text: |
      | Name | Count |
      | --- | --: |
      | memo | 12 |
  - compare:
      before: Now
      after: Then
      rows:
        - { what: Host, before: Old, after: New }
  - fields:
      - { name: --port, text: The port }
  - findings:
      - { level: nit, text: A finding }
  - entries:
      - { title: Copy, tags: [Saturday] }
      - { title: Links }
  - keys:
      - { keys: [Ctrl, K], text: Find a page }
      - { keys: [J], text: Next }
  - checklist:
      - { text: CI passes, done: true }
      - text: Write the plan
  - faq:
      - { q: Where is it?, a: Here. }
      - { q: Does it run on Windows?, a: No. }
  - facts:
      - { name: Owner, value: Agent }
  - html: |
      <div data-floe-look="rows"><p>One</p><p>Two</p></div>
`

// rowPadPage draws rowParts in report.
var rowPadPage = "template: report\nlang: en\ntitle: Rows\nsections:\n  - heading: Rows\n    parts:\n" + indent(rowParts, "    ")

// indent puts by before each line of text.
func indent(text, by string) string {
	var b strings.Builder
	for line := range strings.Lines(text) {
		b.WriteString(by + line)
	}
	return b.String()
}

// rowPadKit is a template of one's own that draws its parts as report does, its page.css setting --floe-row-pad, in
// floe.page over floe.base.
var rowPadKit = map[string]string{
	"templates/rowpad/page.html": "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><meta charset=\"utf-8\">" +
		"<title>{{.Title}}</title></head>\n<body>\n{{range .Data.parts}}{{range .}}{{with .}}{{.}}{{end}}{{end}}{{end}}\n</body></html>\n",
	"templates/rowpad/page.css": ":scope { --floe-row-pad: 1rem; }\nbody { font-size: 16px; line-height: 1.85; }\n",
	"templates/rowpad/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"parts": {"type": "array", "items": {"$ref": "floe:///parts/any.json"}}}}`,
	"templates/rowpad/example.yaml": "lang: en\ntemplate: rowpad\nparts:\n  - text: x\n",
}

// measureRowPad returns, for each kind of row, its part's name (or the look it takes outside a part), its padding
// above and below, and its line height, "page" when it is the page's. A table's rows are its body's cells, not its
// head's, which a theme may pad (default) in a smaller font, nor its number columns, whose font a theme may change
// (swiss). A question of faq is its summary, which a click opens, the details around it unpadded. It adds whether
// checklist's mark is level with its row's text and whether checklist's edge is that of the box of html's rows in the
// same theme.
const measureRowPad = `(() => {
  const ratio = (e) => { const c = getComputedStyle(e); return parseFloat(c.lineHeight) / parseFloat(c.fontSize); };
  const page = ratio(document.body);
  const name = (e) => { const p = e.parentElement.closest('[data-floe-part], [data-floe-look]');
    return p.dataset.floePart || 'look ' + p.dataset.floeLook; };
  const edge = (e) => getComputedStyle(e).paddingTop + '/' + getComputedStyle(e).paddingBottom;
  const row = (e) => name(e) + (e.matches('[data-floe-part="faq"] > details') ? ' details' : '') + ' ' + edge(e) + ' ' +
    (Math.abs(ratio(e) - page) < 0.005 ? 'page' : ratio(e).toFixed(2));
  const rows = [...document.querySelectorAll(':is([data-floe-part], [data-floe-look~="table"]) tbody td:not(.num), ' +
    '[data-floe-look~="rows"] > *, :is([data-floe-part="entries"], [data-floe-part="keys"], [data-floe-part="checklist"]) > *, ' +
    '[data-floe-part="faq"] > *, [data-floe-part="faq"] summary, [data-floe-part="facts"] > .card')];
  const out = [...new Set(rows.map(row))].sort();
  for (const list of document.querySelectorAll('[data-floe-part="checklist"]')) {
    const li = list.querySelector('li');
    out.push('checklist mark ' + (getComputedStyle(li, '::before').top === getComputedStyle(li).paddingTop ? 'level' : 'off'));
    const box = document.querySelector('div[data-floe-look="rows"]');
    out.push('checklist edge ' + (!box || edge(list) === edge(box) ? 'as rows' : edge(list) + ', rows ' + edge(box)));
  }
  return out.join(', ');
})()`

// rowPadWant is what measureRowPad returns for rowParts, each row padded pad above and below, facts' top padded
// factsTop, faq's details not at all.
func rowPadWant(pad, factsTop string) string {
	return strings.Join([]string{
		"checklist " + pad + "/" + pad + " page", "compare " + pad + "/" + pad + " page", "entries " + pad + "/" + pad + " page",
		"facts " + factsTop + "/" + pad + " page", "faq " + pad + "/" + pad + " page", "faq details 0px/0px page", "fields " + pad + "/" + pad + " page",
		"findings " + pad + "/" + pad + " page", "keys " + pad + "/" + pad + " page", "look rows " + pad + "/" + pad + " page",
		"table " + pad + "/" + pad + " page", "checklist mark level", "checklist edge as rows",
	}, ", ")
}

// TestUIRowPad measures the rows of every part and look drawn as rows: each is padded --floe-row-pad (0.4rem) above
// and below, in the page's line height, in every theme, which leaves the variable as it is (a theme's base.css could
// set it, being over floe.base); retro's tiles keep the room of their stripes on top. A template whose page's line
// height is not report's (design: its decided list and its table) pads them the same; a template of one's own setting
// the variable in its page.css pads them all by it; and print pads them as the screen does.
func TestUIRowPad(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	themes, err := os.ReadDir("../kit/floe/themes")
	if err != nil {
		t.Fatal(err)
	}
	file := s.write(t, "rows.yaml", rowPadPage)
	out, want := make([]string, 0, len(themes)+3), make([]string, 0, len(themes)+3)
	for _, theme := range themes {
		name := theme.Name()
		s.floe(t, "publish", file, "--slug", "rows-"+name, "--theme", name)
		b.navigate("/a/rows-" + name + "/?raw")
		out = append(out, name+": "+b.eval(measureRowPad))
		top := "6.4px"
		if name == "retro" {
			top = "25.6px"
		}
		want = append(want, name+": "+rowPadWant("6.4px", top))
	}

	design, err := os.ReadFile("../corpus/testdata/example/design.yaml")
	if err != nil {
		t.Fatal(err)
	}
	s.floe(t, "publish", s.write(t, "design.yaml", string(design)), "--slug", "rows-design")
	b.navigate("/a/rows-design/?raw")
	out = append(out, "design: "+b.eval(measureRowPad))
	want = append(want, "design: look rows 6.4px/6.4px page, look table 6.4px/6.4px page")

	s.own(t, rowPadKit)
	s.floe(t, "publish", s.write(t, "own.yaml", "template: rowpad\nlang: en\ntitle: Rows\nparts:\n"+rowParts), "--slug", "rows-own")
	b.navigate("/a/rows-own/?raw")
	out = append(out, "own: "+b.eval(measureRowPad))
	want = append(want, "own: "+rowPadWant("16px", "16px"))

	b.media("print")
	b.navigate("/a/rows-default/?raw")
	out = append(out, "printed: "+b.eval(measureRowPad))
	want = append(want, "printed: "+rowPadWant("6.4px", "6.4px"))
	check(t, "rows", out, strings.Join(want, "|"))
}
