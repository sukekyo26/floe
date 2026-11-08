package browsertest_test

import (
	"testing"

	"github.com/sukekyo26/floe/internal/kit"
)

// nestingPage draws a card of stats and the cards HTML nests so: in cards, in a panel, in an element of an empty
// look, in an element that takes cards and the panel, and in a part's box that takes cards. In brutal, a note and a
// code in a note's box, and in the stacks of details and tabs; and Markdown's h4 and hr in a note and in details,
// a part of one's own whose box is a <figure> with no margins of its own, in a note's box; and an item of entries that
// holds parts, and one of parts alone.
const nestingPage = `template: report
lang: en
sections:
  - parts:
      - stats: [{ label: a, value: "1" }]
      - html: |
          <div id="plain" data-floe-look="cards"><div class="card" data-floe-look="panel"><p class="title">a</p></div></div>
          <section data-floe-look="panel"><div id="in-panel" data-floe-look="cards"><div class="card" data-floe-look="panel"><p class="title">b</p></div></div></section>
          <div data-floe-look=""><div id="in-empty" data-floe-look="cards"><div class="card" data-floe-look="panel"><p class="title">c</p></div></div></div>
          <div id="both" data-floe-look="cards panel"><div class="card" data-floe-look="panel"><p class="title">d</p></div></div>
          <div id="box" data-floe-part="text" data-floe-look="cards"><div class="card" data-floe-look="panel"><p class="title">e</p></div></div>
          <div data-floe-part="note"><p>n</p><div id="in-note" data-floe-part="note"><p>m</p></div><figure id="fig" data-floe-part="fig">f</figure></div>
      - note: { text: "a\n\n#### h\n\nb\n\n---\n\nc" }
      - details:
          title: d
          open: true
          parts:
            - text: "#### h\n\nb\n\n---\n\nc"
            - note: { text: x }
      - tabs:
          - title: t
            parts:
              - text: t
              - code: { text: x }
          - title: u
            parts:
              - text: u
      - entries:
          - title: e
            text: a
            parts:
              - code: { text: x }
              - text: b
              - code: { text: x }
          - title: f
            parts:
              - code: { text: x }
`

// measureEntries tells, of an item of entries, the room between its text and its first part and between its last part
// and the item's end, and under the title of an item of parts alone, in px, and the line heights of its text and of
// a text in its parts.
const measureEntries = `(() => {
  const at = (s) => document.querySelector('[data-floe-part=entries] ' + s), box = (e) => e.getBoundingClientRect();
  const row = at('> div'), codes = row.querySelectorAll('[data-floe-part=code]'), texts = row.querySelectorAll('[data-floe-text] p');
  const next = at('> div + div');
  return [box(codes[0]).top - box(texts[0]).bottom, box(row).bottom - box(codes[1]).bottom,
    box(next.querySelector('[data-floe-part=code]')).top - box(next.querySelector('dt')).bottom].map(Math.round).join(' ') +
    ' | ' + [...texts].map((p) => getComputedStyle(p).lineHeight).join(' ');
})()`

// measureCards finds, of the card each element of ids holds (nestingPage's: plain, in a panel, in an empty look, in
// both, in a part's box), the style that differs from the card of stats. It returns {seen, amiss}: the cards, and each
// drawn otherwise, at cards.
const measureCards = `((ids) => {
  const props = ['background-color', 'background-image', 'border-top', 'border-right', 'border-bottom', 'border-left',
    'border-radius', 'box-shadow', 'outline', 'padding', 'margin', 'transform'];
  const style = (el) => { const s = getComputedStyle(el); return props.map((p) => p + ': ' + s.getPropertyValue(p)); };
  const part = style(document.querySelector('[data-floe-part=stats] .card'));
  const amiss = [];
  for (const id of ids) {
    const card = document.querySelector('#' + id + ' > .card');
    if (!card) { amiss.push({ at: 'cards', text: '#' + id + ': no card' }); continue; }
    const off = style(card).filter((p, i) => p !== part[i]);
    if (off.length) amiss.push({ at: 'cards', text: '#' + id + ': ' + off.join('; ') });
  }
  return JSON.stringify({ seen: ids.length, amiss });
})`

// nestedCards are the elements of nestingPage that hold a card.
var nestedCards = mustJSON([]string{"plain", "in-panel", "in-empty", "both", "box"})

// measureHeld tells the margins of the boxes stack and a note hold.
const measureHeld = `['#in-note', '[data-floe-part=details] [data-floe-part=note]', '[data-floe-part=tabs] [data-floe-part=code]']
  .map((at) => getComputedStyle(document.querySelector(at)).margin).join(' | ')`

// measureBlocks tells the margins of Markdown's h4 and hr in a note and in a stack, and of a <figure> box in a note.
const measureBlocks = `['[data-floe-part=note] h4', '[data-floe-part=note] hr', '[data-floe-part=stack] h4', '[data-floe-part=stack] hr', '#fig']
  .map((at) => getComputedStyle(document.querySelector(at)).margin).join(' | ')`

// figPart is a part of one's own drawn as a <figure>, a box a browser gives margins of its own.
var figPart = map[string]string{
	"parts/fig/schema.json": `{"type": "object"}`,
	"parts/fig/part.html":   `<figure data-floe-part="fig">f</figure>`,
	"parts/fig/part.css":    "",
}

const lookNestingFix = "leave a box nested to the outer look's rules: floe's layers order them, a part's CSS stops at a part it holds, " +
	"and no !important wins over them"

// lookNesting draws nestingPage in every theme, as measureCards measures: the cards a cards of the HTML holds are drawn
// as the cards of a part are, whatever looks nest around them and whatever the depth.
func lookNesting(t *testing.T, s *site) measured {
	t.Helper()

	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	s.own(t, figPart)
	file := s.write(t, "nesting.yaml", nestingPage)
	var m measured
	for _, theme := range themes {
		s.floe(t, "publish", file, "--slug", "nesting-"+theme, "--theme", theme)
		s.b.navigate("/a/nesting-" + theme + "/?raw")
		s.b.until("#box", "!!document.querySelector('#box > .card')")
		m.add(s.b.measure("nestingPage in "+theme, measureCards+"("+nestedCards+")"))
	}
	return m
}

// brokenNesting draws a card of cards that is a part of one's own, whose part.css pads it by !important, which wins
// over the outer cards' rules, beside a card of stats.
func brokenNesting(t *testing.T, s *site) (string, measured) {
	t.Helper()

	s.brokenPage(t, ownPart("tile", `<div class="card" data-floe-part="tile" data-floe-look="panel"></div>`, ":scope { padding: 3px !important; }\n"),
		"template: report\nlang: en\ntitle: Broken\nsections:\n  - parts:\n      - stats: [{ label: a, value: \"1\" }]\n"+
			"      - html: <div id=\"own\" data-floe-look=\"cards\"><div class=\"card\" data-floe-part=\"tile\" data-floe-look=\"panel\"><p class=\"title\">a</p></div></div>\n")
	return "cards", s.b.measure("broken", measureCards+"("+mustJSON([]string{"own"})+")")
}
