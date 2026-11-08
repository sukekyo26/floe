package browsertest_test

import (
	"fmt"
	"os"
	"testing"
)

// boxParts are the parts that draw a box of their own: those that take the look cards, and those that take the look
// panel or paint their own ground.
const boxParts = `  - asks:
      - { question: A question }
  - checks:
      - { title: A check, result: ok, actual: It held }
  - chart: { kind: bar, title: A chart, x: [a], series: [ { name: b, values: [1] } ] }
  - options:
      - { title: One, text: first }
      - { title: Two, text: second }
  - stats:
      - { label: Pages, value: '1' }
  - facts:
      - { name: Owner, value: A team }
  - decision:
      title: A decision
      text: Made.
  - steps:
      - { title: Freeze, state: done }
  - timeline:
      - { date: 2030-04-05, title: Freeze }
  - bars:
      - { label: Copied, value: 1 }
  - tree:
      - { path: pages/home.md }
  - note: { text: Ready. }
  - details:
      title: The log
      open: true
      parts:
        - text: Printed.
`

// boxPadKit is a template of one's own that draws its parts as report does, its page.css setting both variables of a
// box's padding, in floe.page over floe.base.
var boxPadKit = map[string]string{
	"templates/boxpad/page.html": "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><meta charset=\"utf-8\">" +
		"<title>{{.Title}}</title></head>\n<body>\n{{range .Data.parts}}{{range .}}{{with .}}{{.}}{{end}}{{end}}{{end}}\n</body></html>\n",
	"templates/boxpad/page.css": ":scope { --floe-pad-block: 1rem; --floe-pad-inline: 2rem; }\n",
	"templates/boxpad/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"parts": {"type": "array", "items": {"$ref": "floe:///parts/any.json"}}}}`,
	"templates/boxpad/example.yaml": "lang: en\ntemplate: boxpad\nparts:\n  - text: x\n",
}

// measureBoxPad measures the padding of each box the page draws of those it names (a part's, or a built-in template's),
// every one its selector finds: above and below --floe-pad-block and at the sides --floe-pad-inline, read in the box,
// but facts' cards, rows padded --floe-row-pad above and below, details' title, as tall as a row of its own (0.8rem)
// and so measured at the sides alone, and details' body, of no padding above. A tree whose list paints a ground of its
// own sits in a frame a theme draws around the box (default's): the list is measured as the box. The variables read in
// each box must be want's (pads, as JSON), those of the page. Each of drawn, the names of the boxes the page draws,
// that the page draws none of is amiss too.
// LIMIT: the padding of a theme's frame around a tree, and details' title above and below, are not measured: the one is
// the theme's decoration, as its border is, the other a row's height of details' own.
// It returns {seen, amiss}: the boxes it measured, and for each padded otherwise its part (or the template's box, or
// "theme" for variables set otherwise) and how.
func measureBoxPad(want pads, drawn []string) string {
	return `((want, drawn) => {
  const boxes = {
    options: '[data-floe-part=options] .card', asks: '[data-floe-part=asks] .card', checks: '[data-floe-part=checks]', chart: '[data-floe-part=chart]', stats: '[data-floe-part=stats] .card', facts: '[data-floe-part=facts] .card',
    decision: '[data-floe-part=decision] .card',
    steps: '[data-floe-part=steps]', timeline: '[data-floe-part=timeline]',
    bars: '[data-floe-part=bars]', tree: '[data-floe-part=tree]', note: '[data-floe-part=note]',
    details: '[data-floe-part=details] > summary', 'details body': '[data-floe-part=details] > div',
    runbook: '.needs, .rollback', handoff: '.board section',
  };
  ` + padsJS + `
  const ground = (e) => { const c = getComputedStyle(e); return c.backgroundColor !== 'rgba(0, 0, 0, 0)' || c.backgroundImage !== 'none'; };
  let seen = 0;
  const amiss = [], off = (a, b) => Math.abs(a - b) > 0.1;
  for (const name of drawn) if (!document.querySelector(boxes[name])) amiss.push({ at: name, text: 'no box of it drawn' });
  for (const [name, sel] of Object.entries(boxes)) {
    for (let e of document.querySelectorAll(sel)) {
      seen++;
      const list = name === 'tree' && e.querySelector(':scope > ul');
      if (list && ground(list)) e = list;
      const { Row: row, Block: block, Inline: inline } = pads(e);
      if (off(row, want.Row) || off(block, want.Block) || off(inline, want.Inline)) amiss.push({ at: 'theme', text: name + ' reads --floe-row-pad, --floe-pad-block, --floe-pad-inline ' + [row, block, inline].join(' ') + ', not the page\'s ' + [want.Row, want.Block, want.Inline].join(' ') });
      const s = getComputedStyle(e);
      const top = name === 'facts' ? row : name === 'details body' ? 0 : block, bottom = name === 'facts' ? row : block;
      const got = [s.paddingTop, s.paddingRight, s.paddingBottom, s.paddingLeft].map(parseFloat);
      const sides = [[got[0], top, 'above'], [got[1], inline, 'right'], [got[2], bottom, 'below'], [got[3], inline, 'left']]
        .filter(([, , side]) => name !== 'details' || side === 'right' || side === 'left');
      const wrong = sides.filter(([g, w]) => off(g, w));
      if (wrong.length) amiss.push({ at: e.closest('[data-floe-part]')?.dataset.floePart || name, text: name + ' padded ' + wrong.map(([g, w, side]) => side + ' ' + g + ', not ' + w).join(', ') });
    }
  }
  return JSON.stringify({ seen, amiss });
})(` + mustJSON(want) + `, ` + mustJSON(drawn) + `)`
}

// partBoxes are the names of measureBoxPad's boxes boxParts draws.
var partBoxes = []string{"options", "asks", "checks", "chart", "stats", "facts", "decision", "steps", "timeline", "bars", "tree", "note", "details", "details body"}

const lookBoxPadFix = "pad a box between parts by --floe-pad-block and --floe-pad-inline, not by numbers; a theme sets neither and pads no box"

// lookBoxPad measures the box of every part that draws one (boxParts), and runbook's and handoff's examples, in every
// theme (measureBoxPad): padded by base.css's variables (basePads), which a theme leaves as they are and writes no
// padding over (a theme's decorations lie over it). A template of one's own (boxPadKit) whose page.css sets both
// variables otherwise pads every part's box by them.
func lookBoxPad(t *testing.T, s *site) measured {
	t.Helper()

	themes, err := os.ReadDir("../kit/floe/themes")
	if err != nil {
		t.Fatal(err)
	}
	page := s.write(t, "boxes.yaml", "template: report\nlang: en\ntitle: Boxes\nsections:\n  - heading: Boxes\n    parts:\n"+
		indent(boxParts, "    "))
	files := map[string]string{}
	for _, name := range []string{"runbook", "handoff"} {
		data, err := os.ReadFile("../corpus/testdata/example/" + name + ".yaml")
		if err != nil {
			t.Fatal(err)
		}
		files[name] = s.write(t, name+".yaml", string(data))
	}
	base := basePads(t, s)
	var m measured
	open := func(what, file, slug, theme string, drawn []string) {
		s.floe(t, "publish", file, "--slug", slug, "--theme", theme)
		s.b.navigate("/a/" + slug + "/?raw")
		m.add(s.b.measure(what, measureBoxPad(base, drawn)))
	}
	for _, theme := range themes {
		name := theme.Name()
		open("boxParts in "+name, page, "boxes-"+name, name, partBoxes)
		for _, sample := range []string{"runbook", "handoff"} {
			open(sample+" in "+name, files[sample], "boxes-"+sample+"-"+name, name, []string{sample})
		}
	}
	s.own(t, boxPadKit)
	s.floe(t, "publish", s.write(t, "own.yaml", "template: boxpad\nlang: en\ntitle: Boxes\nparts:\n"+boxParts), "--slug", "boxes-own")
	s.b.navigate("/a/boxes-own/?raw")
	own := padsOn(t, s)
	if own.Block == base.Block || own.Inline == base.Inline {
		m.amiss = append(m.amiss, amiss{Page: "boxParts in boxpad", At: "boxpad", Text: fmt.Sprintf("its page.css's variables reach not the page: %+v, base.css's %+v", own, base)})
	}
	m.add(s.b.measure("boxParts in boxpad", measureBoxPad(own, partBoxes)))
	return m
}

// brokenBoxPad draws a note in a theme of one's own whose note.css pads it by a number.
func brokenBoxPad(t *testing.T, s *site) (string, measured) {
	t.Helper()

	base := basePads(t, s)
	s.brokenPage(t, map[string]string{"themes/thin/note.css": ":scope { padding: 3px; }\n"}, themePage("thin", "- note: { text: Ready. }\n"))
	return "note", s.b.measure("broken", measureBoxPad(base, []string{"note"}))
}
