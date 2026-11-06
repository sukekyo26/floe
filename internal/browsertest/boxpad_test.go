package browsertest_test

import (
	"os"
	"strings"
	"testing"
)

// boxParts are the parts that draw a box of their own: those that take the look cards, and those that take the look
// panel or paint their own ground.
const boxParts = `  - asks:
      - { question: A question }
  - options:
      - { title: One, text: first }
      - { title: Two, text: second }
  - stats:
      - { label: Pages, value: '1' }
  - facts:
      - { name: Owner, value: A team }
  - links:
      - { title: Docs, url: "https://example.com/docs" }
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

// measureBoxPad returns, for each of the boxes named (the parts' or a built-in template's), its padding (top right
// bottom left), the first element its selector finds. A tree whose list paints a ground of its own is a frame a theme
// draws around the box (default's): it returns the frame's padding too ("frame"), and the list's as the box's.
func measureBoxPad(names string) string {
	return `((names) => {
  const boxes = {
    options: '[data-floe-part=options] .card', asks: '[data-floe-part=asks] .card', stats: '[data-floe-part=stats] .card', facts: '[data-floe-part=facts] .card',
    links: '[data-floe-part=links] .card', decision: '[data-floe-part=decision] .card',
    steps: '[data-floe-part=steps]', timeline: '[data-floe-part=timeline]',
    bars: '[data-floe-part=bars]', tree: '[data-floe-part=tree]', note: '[data-floe-part=note]',
    'details title': '[data-floe-part=details] > summary', 'details body': '[data-floe-part=details] > div',
    'design aside': 'aside[data-floe-look=panel]', 'runbook needs': '.needs', 'runbook rollback': '.rollback',
    'handoff board': '.board section',
  };
  const ground = (e) => { const c = getComputedStyle(e); return c.backgroundColor !== 'rgba(0, 0, 0, 0)' || c.backgroundImage !== 'none'; };
  const out = [];
  for (const name of names.split(', ')) {
    let e = document.querySelector(boxes[name]);
    const pad = (e) => { const s = getComputedStyle(e); return [s.paddingTop, s.paddingRight, s.paddingBottom, s.paddingLeft].join(' '); };
    const list = name === 'tree' && e.querySelector(':scope > ul');
    if (list && ground(list)) { out.push(name + ' frame ' + pad(e) + ', ' + pad(list)); continue; }
    const s = getComputedStyle(e);
    out.push(name + ' ' + [s.paddingTop, s.paddingRight, s.paddingBottom, s.paddingLeft].join(' '));
  }
  return out.join(', ');
})('` + names + `')`
}

// partBoxes are the boxes boxParts draw.
const partBoxes = "options, asks, stats, facts, links, decision, steps, timeline, bars, tree, note, details title, details body"

// boxPadWant is what measureBoxPad returns for boxParts, each box padded block above and below and inline at the
// left and the right: facts' cards are rows (TestUIRowPad), details' title is as tall as a row of its own, and frame
// is the padding of the frame a theme draws around the tree, as measureBoxPad returns it ("" when it draws none).
func boxPadWant(block, inline, frame string) string {
	box := block + " " + inline + " " + block + " " + inline
	return strings.Join([]string{
		"options " + box, "asks " + box, "stats " + box, "facts 6.4px " + inline + " 6.4px " + inline, "links " + box, "decision " + box,
		"steps " + box, "timeline " + box, "bars " + box, "tree " + frame + box, "note " + box,
		"details title 12.8px " + inline + " 12.8px " + inline, "details body 0px " + inline + " " + block + " " + inline,
	}, ", ")
}

// TestUIBoxPad measures the padding of the box of every part that draws one, in every theme: it is the variables'
// of floe's base.css (1.1rem above and below, 1.3rem at the sides), which a theme leaves as they are and writes no
// padding over (a theme's decorations lie over it), so a theme that set either variable or padded a box fails. The
// built-in templates' boxes are padded the same in every theme, and a template of one's own setting the variables in its
// page.css pads every part's box by them.
func TestUIBoxPad(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	themes, err := os.ReadDir("../kit/floe/themes")
	if err != nil {
		t.Fatal(err)
	}
	page := s.write(t, "boxes.yaml", "template: report\nlang: en\ntitle: Boxes\nsections:\n  - heading: Boxes\n    parts:\n"+
		indent(boxParts, "    "))
	samples := map[string]string{"design": "design aside", "runbook": "runbook needs, runbook rollback", "handoff": "handoff board"}
	files := map[string]string{}
	for name := range samples {
		data, err := os.ReadFile("../corpus/testdata/example/" + name + ".yaml")
		if err != nil {
			t.Fatal(err)
		}
		files[name] = s.write(t, name+".yaml", string(data))
	}
	// frames are the themes that frame the tree, by the padding of the frame: default's grey, --default-pad.
	frames := map[string]string{"default": "frame 6px 6px 6px 6px, "}
	out, want := make([]string, 0, 4*len(themes)+1), make([]string, 0, 4*len(themes)+1)
	for _, theme := range themes {
		name := theme.Name()
		s.floe(t, "publish", page, "--slug", "boxes-"+name, "--theme", name)
		b.navigate("/a/boxes-" + name + "/?raw")
		out = append(out, name+": "+b.eval(measureBoxPad(partBoxes)))
		want = append(want, name+": "+boxPadWant("17.6px", "20.8px", frames[name]))
		for _, sample := range []string{"design", "runbook", "handoff"} {
			slug := "boxes-" + sample + "-" + name
			s.floe(t, "publish", files[sample], "--slug", slug, "--theme", name)
			b.navigate("/a/" + slug + "/?raw")
			out = append(out, name+" "+sample+": "+b.eval(measureBoxPad(samples[sample])))
			boxes := strings.Split(samples[sample], ", ")
			for i := range boxes {
				boxes[i] += " 17.6px 20.8px 17.6px 20.8px"
			}
			want = append(want, name+" "+sample+": "+strings.Join(boxes, ", "))
		}
	}

	s.own(t, boxPadKit)
	s.floe(t, "publish", s.write(t, "own.yaml", "template: boxpad\nlang: en\ntitle: Boxes\nparts:\n"+boxParts), "--slug", "boxes-own")
	b.navigate("/a/boxes-own/?raw")
	out = append(out, "own: "+b.eval(measureBoxPad(partBoxes)))
	want = append(want, "own: "+boxPadWant("16px", "32px", frames["default"]))
	check(t, "boxes", out, strings.Join(want, "|"))
}
