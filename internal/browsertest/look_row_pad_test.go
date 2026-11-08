package browsertest_test

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/kit"
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
  - links:
      - { title: Docs, url: "https://example.com/docs", text: read }
      - { title: Wiki, url: "https://example.com/wiki" }
  - checklist:
      - { text: CI passes, done: true }
      - text: Write the plan
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

// rowPadKit is a template of one's own that draws its parts as report does, its page.css setting --floe-row-pad and
// --floe-pad-inline, in floe.page over floe.base.
var rowPadKit = map[string]string{
	"templates/rowpad/page.html": "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><meta charset=\"utf-8\">" +
		"<title>{{.Title}}</title></head>\n<body>\n{{range .Data.parts}}{{range .}}{{with .}}{{.}}{{end}}{{end}}{{end}}\n</body></html>\n",
	"templates/rowpad/page.css": ":scope { --floe-row-pad: 1rem; --floe-pad-inline: 2rem; }\nbody { font-size: 16px; line-height: 1.85; }\n",
	"templates/rowpad/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"parts": {"type": "array", "items": {"$ref": "floe:///parts/any.json"}}}}`,
	"templates/rowpad/example.yaml": "lang: en\ntemplate: rowpad\nparts:\n  - text: x\n",
}

// measureRowPad measures each row of every part and look drawn as rows: a table's cells, its head's and its body's,
// but not its number columns, whose font a theme may change (swiss); the rows of the look rows (TestLookRule/marks
// measures checklist's mark on its first line); and checklist's count, a row its box draws as ::after. Each is padded
// --floe-row-pad above and below and, but a table's cell, whose sides are its columns' gaps a theme may change,
// --floe-pad-inline at the left and the right, read in the row, in the page's line height. The box of every part and
// element drawn as rows is padded as that of an element of html's rows on the page, when there is one. When want
// ([row, block, inline] in px, else null) is given, the variables read in each row must be those.
// It returns {seen, amiss, drawn}: the rows it measured, for each padded otherwise its part (or "look rows", "look
// table", or "theme" for variables set otherwise) and how, and the parts and looks it found rows of.
func measureRowPad(want string) string {
	return `((want) => {
  const ratio = (e) => { const c = getComputedStyle(e); return parseFloat(c.lineHeight) / parseFloat(c.fontSize); };
  const page = ratio(document.body);
  ` + pxJS + `
  const off = (a, b) => Math.abs(a - b) > 0.1;
  const owner = (e) => { const p = e.parentElement.closest('[data-floe-part], [data-floe-look]'); return p.dataset.floePart || 'look ' + p.dataset.floeLook; };
  let seen = 0;
  const amiss = [], drawn = new Set();
  const measure = (at, e, s, cell) => {
    seen++;
    drawn.add(at);
    const vars = [px(e, 'var(--floe-row-pad)'), px(e, 'var(--floe-pad-block)'), px(e, 'var(--floe-pad-inline)')];
    if (want && vars.some((v, i) => off(v, want[i]))) amiss.push({ at: 'theme', text: at + ' reads --floe-row-pad, --floe-pad-block, --floe-pad-inline ' + vars.join(' ') + ', not base.css\'s ' + want.join(' ') });
    const [row, , inline] = vars;
    const sides = [[s.paddingTop, row, 'above'], [s.paddingBottom, row, 'below']];
    if (!cell) sides.push([s.paddingLeft, inline, 'left'], [s.paddingRight, inline, 'right']);
    const wrong = sides.filter(([g, w]) => off(parseFloat(g), w));
    if (wrong.length) amiss.push({ at, text: (e.matches('thead th') ? 'head ' : '') + 'padded ' + wrong.map(([g, w, side]) => side + ' ' + g + ', not ' + w + 'px').join(', ') });
  };
  const rows = document.querySelectorAll(':is([data-floe-part], [data-floe-look~="table"]) :is(thead th, tbody td):not(.num), ' +
    '[data-floe-look~="rows"] > *, :is([data-floe-part="entries"], [data-floe-part="keys"], [data-floe-part="checklist"], [data-floe-part="links"]) > *, ' +
    '[data-floe-part="facts"] > .card');
  for (const e of rows) {
    const at = owner(e);
    measure(at, e, getComputedStyle(e), e.matches('th, td'));
    if (off(ratio(e), page)) amiss.push({ at, text: 'line height ' + ratio(e).toFixed(2) + ', the page\'s ' + page.toFixed(2) });
  }
  for (const list of document.querySelectorAll('[data-floe-part="checklist"]')) measure('checklist', list, getComputedStyle(list, '::after'), false);
  const html = document.querySelector('div[data-floe-look="rows"]'), box = (e) => getComputedStyle(e).padding;
  if (html) {
    for (const e of document.querySelectorAll('[data-floe-look~="rows"], :is([data-floe-part="entries"], [data-floe-part="keys"], [data-floe-part="checklist"], [data-floe-part="links"])')) {
      seen++;
      if (box(e) !== box(html)) amiss.push({ at: e.dataset.floePart || 'look rows', text: 'its box padded ' + box(e) + ', html\'s rows ' + box(html) });
    }
  }
  return JSON.stringify({ seen, amiss, drawn: [...drawn] });
})(` + want + `)`
}

// rowsSection is a section of design's and postmortem's more holding an element of html that takes the look rows.
const rowsSection = "  - heading: Rows\n    parts:\n      - html: |\n          <div data-floe-look=\"rows\"><p>One</p><p>Two</p></div>\n"

// rowsOn measures the page open, told as what, as measureRowPad does, and returns what it found rows of too.
func rowsOn(t *testing.T, s *site, what, want string) (measured, []string) {
	t.Helper()

	raw := s.b.eval(measureRowPad(want))
	var got struct{ Drawn []string }
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	return s.b.measuredOf(what, raw), got.Drawn
}

const lookRowPadFix = "pad a row by --floe-row-pad above and below and --floe-pad-inline at the sides: take the look rows or table, " +
	"and write no padding of a row; a theme sets neither"

// lookRowPad measures rowParts in every theme, on the screen and printed, design's and postmortem's examples (their
// pages' line heights are not report's) and rowParts in a template of one's own setting the variables in its page.css,
// as measureRowPad does. Every part that takes the look rows or table, and table and facts, must be drawn as rows in
// rowParts.
func lookRowPad(t *testing.T, s *site) measured {
	t.Helper()

	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	parts, _, themes, _ := k.Names()
	rowed := []string{"table", "facts"}
	for _, name := range parts {
		if taken := k.Taken(name); slices.Contains(taken, "rows") || slices.Contains(taken, "table") {
			rowed = append(rowed, name)
		}
	}
	file := s.write(t, "rows.yaml", rowPadPage)
	var m measured
	for _, theme := range themes {
		s.floe(t, "publish", file, "--slug", "rows-"+theme, "--theme", theme)
		s.b.navigate("/a/rows-" + theme + "/?raw")
		for _, media := range []string{"screen", "print"} {
			s.b.media(media)
			got, drawn := rowsOn(t, s, "rowParts in "+theme+" on "+media, basePad)
			m.add(got)
			for _, name := range rowed {
				if !slices.Contains(drawn, name) {
					m.amiss = append(m.amiss, amiss{Page: "rowParts in " + theme, At: name, Text: "no row of it drawn: add it to rowParts"})
				}
			}
		}
		s.b.media("")
	}
	for _, sample := range []string{"design", "postmortem"} {
		data, err := os.ReadFile("../corpus/testdata/example/" + sample + ".yaml")
		if err != nil {
			t.Fatal(err)
		}
		s.floe(t, "publish", s.write(t, sample+".yaml", string(data)+rowsSection), "--slug", "rows-"+sample)
		s.b.navigate("/a/rows-" + sample + "/?raw")
		got, _ := rowsOn(t, s, sample, basePad)
		m.add(got)
	}
	s.own(t, rowPadKit)
	s.floe(t, "publish", s.write(t, "own.yaml", "template: rowpad\nlang: en\ntitle: Rows\nparts:\n"+rowParts), "--slug", "rows-own")
	s.b.navigate("/a/rows-own/?raw")
	got, _ := rowsOn(t, s, "rowParts in rowpad", "null")
	m.add(got)
	return m
}

// brokenRowPad draws links in a theme of one's own whose links.css pads its rows by a number.
func brokenRowPad(t *testing.T, s *site) (string, measured) {
	t.Helper()

	s.brokenPage(t, map[string]string{"themes/thin/links.css": ":scope > * { padding-block: 3px; }\n"},
		themePage("thin", "- links: [{ title: Docs, url: 'https://example.com/docs', text: read }]\n"))
	got, _ := rowsOn(t, s, "broken", basePad)
	return "links", got
}
