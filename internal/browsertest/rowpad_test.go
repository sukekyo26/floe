package browsertest_test

import (
	"os"
	"slices"
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
// above and below, and its line height, "page" when it is the page's: a table's cells, its head's ("head") and its
// body's, but not its number columns, whose font a theme may change (swiss); the rows of the look rows, and faq's
// questions ("question"), which take none of their own. It adds whether checklist's mark is level with its row's text
// (within the 1/64 px a layout rounds to), the padding of checklist's count, a row its box draws as ::after, and
// whether the box of every part and element drawn as rows is padded on every side as that of html's rows in the same
// page.
const measureRowPad = `(() => {
  const ratio = (e) => { const c = getComputedStyle(e); return parseFloat(c.lineHeight) / parseFloat(c.fontSize); };
  const page = ratio(document.body);
  const name = (e) => { const p = e.parentElement.closest('[data-floe-part], [data-floe-look]');
    return (p.dataset.floePart || 'look ' + p.dataset.floeLook) + (e.matches('thead th') ? ' head' : e.matches('summary') ? ' question' : ''); };
  const edge = (e) => getComputedStyle(e).paddingTop + '/' + getComputedStyle(e).paddingBottom;
  const row = (e) => name(e) + ' ' + edge(e) + ' ' + (Math.abs(ratio(e) - page) < 0.005 ? 'page' : ratio(e).toFixed(2));
  const rows = [...document.querySelectorAll(':is([data-floe-part], [data-floe-look~="table"]) :is(thead th, tbody td):not(.num), ' +
    '[data-floe-look~="rows"] > *, :is([data-floe-part="entries"], [data-floe-part="keys"], [data-floe-part="checklist"], ' +
    '[data-floe-part="faq"]) > *, [data-floe-part="faq"] summary, [data-floe-part="facts"] > .card')];
  const out = [...new Set(rows.map(row))].sort();
  for (const list of document.querySelectorAll('[data-floe-part="checklist"]')) {
    const li = list.querySelector('li');
    const off = parseFloat(getComputedStyle(li, '::before').top) - parseFloat(getComputedStyle(li).paddingTop);
    out.push('checklist mark ' + (Math.abs(off) < 0.1 ? 'level' : 'off by ' + off.toFixed(2)));
    const count = getComputedStyle(list, '::after');
    out.push('checklist count ' + count.paddingTop + '/' + count.paddingBottom);
  }
  const html = document.querySelector('div[data-floe-look="rows"]'), box = (e) => getComputedStyle(e).padding;
  const odd = [...document.querySelectorAll('[data-floe-look~="rows"], ' +
    ':is([data-floe-part="entries"], [data-floe-part="keys"], [data-floe-part="checklist"], [data-floe-part="faq"])')]
    .filter((e) => box(e) !== box(html)).map((e) => (e.dataset.floePart || e.className) + ' ' + box(e));
  out.push('boxes ' + (odd.length ? odd.join(' ') + ', html rows ' + box(html) : 'as html rows'));
  return out.join(', ');
})()`

// rowPadWant is what measureRowPad returns for rowParts, each row padded pad above and below, facts' top padded
// factsTop.
func rowPadWant(pad, factsTop string) string {
	both := pad + "/" + pad + " page"
	rows := []string{
		"checklist " + both, "compare " + both, "compare head " + both, "entries " + both, "facts " + factsTop + "/" + pad + " page",
		"faq " + both, "faq question 0px/0px page", "fields " + both, "findings " + both, "keys " + both, "look rows " + both,
		"table " + both, "table head " + both,
	}
	slices.Sort(rows)
	return strings.Join(append(rows, "checklist mark level", "checklist count "+pad+"/"+pad, "boxes as html rows"), ", ")
}

// rowsSection is a section of design's and postmortem's more holding an element of html that takes the look rows.
const rowsSection = "  - heading: Rows\n    parts:\n      - html: |\n          <div data-floe-look=\"rows\"><p>One</p><p>Two</p></div>\n"

// TestUIRowPad measures the rows of every part and look drawn as rows: each is padded --floe-row-pad (0.4rem) above
// and below, in the page's line height, in every theme, on the screen and printed, which leaves the variable as it is
// (a theme's base.css could set it, being over floe.base); retro's tiles keep the room of their stripes on top. A
// template whose page's line height is not report's (design: its decided list and its table; postmortem: its table)
// pads them the same; and a template of one's own setting the variable in its page.css pads them all by it.
func TestUIRowPad(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	themes, err := os.ReadDir("../kit/floe/themes")
	if err != nil {
		t.Fatal(err)
	}
	file := s.write(t, "rows.yaml", rowPadPage)
	out, want := make([]string, 0, 2*len(themes)+3), make([]string, 0, 2*len(themes)+3)
	measure := func(media string) {
		b.media(media)
		for _, theme := range themes {
			name := theme.Name()
			b.navigate("/a/rows-" + name + "/?raw")
			out = append(out, media+" "+name+": "+b.eval(measureRowPad))
			top := "6.4px"
			if name == "retro" {
				top = "25.6px"
			}
			want = append(want, media+" "+name+": "+rowPadWant("6.4px", top))
		}
	}
	for _, theme := range themes {
		s.floe(t, "publish", file, "--slug", "rows-"+theme.Name(), "--theme", theme.Name())
	}
	measure("screen")
	measure("print")
	b.media("")

	for _, sample := range []struct{ name, want string }{
		{"design", "look rows 6.4px/6.4px page, look table 6.4px/6.4px page, look table head 6.4px/6.4px page, boxes as html rows"},
		{"postmortem", "facts 6.4px/6.4px page, look rows 6.4px/6.4px page, look table 6.4px/6.4px page, look table head 6.4px/6.4px page, boxes as html rows"},
	} {
		data, err := os.ReadFile("../corpus/testdata/example/" + sample.name + ".yaml")
		if err != nil {
			t.Fatal(err)
		}
		s.floe(t, "publish", s.write(t, sample.name+".yaml", string(data)+rowsSection), "--slug", "rows-"+sample.name)
		b.navigate("/a/rows-" + sample.name + "/?raw")
		out = append(out, sample.name+": "+b.eval(measureRowPad))
		want = append(want, sample.name+": "+sample.want)
	}

	s.own(t, rowPadKit)
	s.floe(t, "publish", s.write(t, "own.yaml", "template: rowpad\nlang: en\ntitle: Rows\nparts:\n"+rowParts), "--slug", "rows-own")
	b.navigate("/a/rows-own/?raw")
	out = append(out, "own: "+b.eval(measureRowPad))
	want = append(want, "own: "+rowPadWant("16px", "16px"))
	check(t, "rows", out, strings.Join(want, "|"))
}
