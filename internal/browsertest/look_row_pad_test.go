package browsertest_test

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"floe/internal/kit"
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
  - fields:
      - { name: --port, text: The port }
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
// --floe-pad-inline, in floe.page over floe's defaults.
var rowPadKit = map[string]string{
	"templates/rowpad/page.html": "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><meta charset=\"utf-8\">" +
		"<title>{{.Title}}</title></head>\n<body>\n{{range .Data.parts}}{{range .}}{{with .}}{{.}}{{end}}{{end}}{{end}}\n</body></html>\n",
	"templates/rowpad/page.css": ":scope { --floe-row-pad: 1rem; --floe-pad-inline: 2rem; }\nbody { font-size: 16px; line-height: 1.85; }\n",
	"templates/rowpad/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"parts": {"type": "array", "items": {"$ref": "floe:///parts/any.json"}}}}`,
	"templates/rowpad/example.yaml": "lang: en\ntemplate: rowpad\nparts:\n  - text: x\n",
}

// rowsSel finds the rows of every part and look drawn as rows: a table's cells, its head's and its body's, but not
// its number columns, whose font a theme may change (swiss); the rows of the look rows; and facts' cards.
const rowsSel = `':is([data-floe-part], [data-floe-look~="table"]) :is(thead th, tbody td):not(.num), ' +
    '[data-floe-look~="rows"] > *, :is([data-floe-part="entries"], [data-floe-part="keys"], [data-floe-part="checklist"], [data-floe-part="links"]) > *, ' +
    '[data-floe-part="facts"] > .card'`

// rowOwnerJS defines owner, the part (or "look rows", "look table") a row is of.
const rowOwnerJS = `const owner = (e) => { const p = e.parentElement.closest('[data-floe-part], [data-floe-look]'); return p.dataset.floePart || 'look ' + p.dataset.floeLook; };`

// measureRowPad measures each row (rowsSel), and checklist's count, a row its box draws as ::after: each is padded
// --floe-row-pad above and below and, but a table's cell, whose sides are its columns' gaps a theme may change,
// --floe-pad-inline at the left and the right, read in the row, and those read there are want's (pads, as JSON), the
// page's.
// It returns {seen, amiss, drawn}: the rows it measured, for each padded otherwise its part (or "look rows", "look
// table", or "theme" for variables set otherwise) and how, and the parts and looks it found rows of.
func measureRowPad(want pads) string {
	return `((want) => {
  ` + padsJS + rowOwnerJS + `
  const off = (a, b) => Math.abs(a - b) > 0.1;
  let seen = 0;
  const amiss = [], drawn = new Set();
  const measure = (at, e, s, cell) => {
    seen++;
    drawn.add(at);
    const { Row: row, Inline: inline } = pads(e);
    if (off(row, want.Row) || off(inline, want.Inline)) amiss.push({ at: 'theme', text: at + ' reads --floe-row-pad, --floe-pad-inline ' + row + ' ' + inline + ', not the page\'s ' + want.Row + ' ' + want.Inline });
    const sides = [[s.paddingTop, row, 'above'], [s.paddingBottom, row, 'below']];
    if (!cell) sides.push([s.paddingLeft, inline, 'left'], [s.paddingRight, inline, 'right']);
    const wrong = sides.filter(([g, w]) => off(parseFloat(g), w));
    if (wrong.length) amiss.push({ at, text: (e.matches('thead th') ? 'head ' : '') + 'padded ' + wrong.map(([g, w, side]) => side + ' ' + g + ', not ' + w + 'px').join(', ') });
  };
  for (const e of document.querySelectorAll(` + rowsSel + `)) measure(owner(e), e, getComputedStyle(e), e.matches('th, td'));
  for (const list of document.querySelectorAll('[data-floe-part="checklist"]')) measure('checklist', list, getComputedStyle(list, '::after'), false);
  return JSON.stringify({ seen, amiss, drawn: [...drawn] });
})(` + mustJSON(want) + `)`
}

// rowsSection is a section of design's and postmortem's more holding an element of html that takes the look rows.
const rowsSection = "  - heading: Rows\n    parts:\n      - html: |\n          <div data-floe-look=\"rows\"><p>One</p><p>Two</p></div>\n"

// rowsOn measures the page open, told as what, as measureRowPad does, and returns what it found rows of too.
func rowsOn(t *testing.T, s *site, what string, want pads) (measured, []string) {
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

// lookRowPad measures rowParts in every theme, on the screen and printed, and design's and postmortem's examples, by
// defaults.css's variables (basePads), and rowParts in a template of one's own (rowPadKit) whose page.css sets them
// otherwise, by those, as measureRowPad does. Every part that takes the look rows or table, and table and facts, must be drawn as rows in
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
	base := basePads(t, s)
	file := s.write(t, "rows.yaml", rowPadPage)
	var m measured
	for _, theme := range themes {
		s.floe(t, "publish", file, "--slug", "rows-"+theme, "--theme", theme)
		s.b.navigate("/a/rows-" + theme + "/?raw")
		for _, media := range []string{"screen", "print"} {
			s.b.media(media)
			got, drawn := rowsOn(t, s, "rowParts in "+theme+" on "+media, base)
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
		got, _ := rowsOn(t, s, sample, base)
		m.add(got)
	}
	s.own(t, rowPadKit)
	s.floe(t, "publish", s.write(t, "own.yaml", "template: rowpad\nlang: en\ntitle: Rows\nparts:\n"+rowParts), "--slug", "rows-own")
	s.b.navigate("/a/rows-own/?raw")
	own := padsOn(t, s)
	if own.Row == base.Row || own.Inline == base.Inline {
		m.amiss = append(m.amiss, amiss{Page: "rowParts in rowpad", At: "rowpad", Text: fmt.Sprintf("its page.css's variables reach not the page: %+v, defaults.css's %+v", own, base)})
	}
	got, _ := rowsOn(t, s, "rowParts in rowpad", own)
	m.add(got)
	return m
}

// brokenRowPad draws links in a theme of one's own whose links.css pads its rows by a number.
func brokenRowPad(t *testing.T, s *site) (string, measured) {
	t.Helper()

	base := basePads(t, s)
	s.brokenPage(t, map[string]string{"themes/thin/links.css": ":scope > * { padding-block: 3px; }\n"},
		themePage("thin", "- links: [{ title: Docs, url: 'https://example.com/docs', text: read }]\n"))
	got, _ := rowsOn(t, s, "broken", base)
	return "links", got
}
