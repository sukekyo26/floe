package browsertest_test

import (
	"os"
	"strings"
	"testing"
)

// nestingPage draws a card of stats and the cards HTML nests so: in cards, in a panel, in an element of an empty
// look, in an element that takes cards and the panel, and in a part's box that takes cards. In brutal, a note and a
// code in a note's box, and in the stacks of details and tabs; and Markdown's h4 and hr in a note and in details,
// and a part of one's own whose box is a <figure> with no margins of its own, in a note's box.
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
`

// measureCards tells, of each card the HTML nests, the style that differs from the card of stats.
const measureCards = `(() => {
  const props = ['background-color', 'background-image', 'border-top', 'border-right', 'border-bottom', 'border-left',
    'border-radius', 'box-shadow', 'outline', 'padding', 'margin', 'transform'];
  const style = (el) => { const s = getComputedStyle(el); return props.map((p) => p + ': ' + s.getPropertyValue(p)); };
  const part = style(document.querySelector('[data-floe-part=stats] .card'));
  return ['plain', 'in-panel', 'in-empty', 'both', 'box'].map((id) => {
    const got = style(document.querySelector('#' + id + ' > .card'));
    const off = got.filter((p, i) => p !== part[i]);
    return id + (off.length ? ' ' + off.join('; ') : ' as stats');
  }).join(', ');
})()`

// measureHeld tells the margins of the boxes stack and a note hold.
const measureHeld = `['#in-note', '[data-floe-part=details] [data-floe-part=note]', '[data-floe-part=tabs] [data-floe-part=code]']
  .map((at) => getComputedStyle(document.querySelector(at)).margin).join(' | ')`

// measureBlocks tells the margins of Markdown's h4 and hr in a note and in a stack, and of a <figure> box in a note.
const measureBlocks = `['[data-floe-part=note] h4', '[data-floe-part=note] hr', '[data-floe-part=stack] h4', '[data-floe-part=stack] hr', '#fig']
  .map((at) => getComputedStyle(document.querySelector(at)).margin).join(' | ')`

// TestUINesting checks, in the browser, that what nests the outer wins over what it holds, in every theme: the
// cards a cards of the HTML holds are drawn as the cards of a part are, whatever looks nest around them and
// whatever the depth; and a stack or a note gives the boxes it holds no margins of its own but the note's gap between
// them, --floe-room next to a part's box, in brutal too, whose shadows take no room, which floe's base leaves alone in
// the blocks a browser gives margins of its own (h4, hr, figure).
func TestUINesting(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	themes, err := os.ReadDir("../kit/floe/themes")
	if err != nil {
		t.Fatal(err)
	}
	s.own(t, map[string]string{
		"parts/fig/schema.json": `{"type": "object"}`,
		"parts/fig/part.html":   `<figure data-floe-part="fig">f</figure>`,
		"parts/fig/part.css":    "",
	})
	file := s.write(t, "nesting.yaml", nestingPage)
	out, want := make([]string, 0, len(themes)), make([]string, 0, len(themes))
	for _, theme := range themes {
		name := theme.Name()
		s.floe(t, "publish", file, "--slug", "nesting-"+name, "--theme", name)
		b.navigate("/a/nesting-" + name + "/?raw")
		b.until("#box", "!!document.querySelector('#box > .card')")
		out = append(out, name+": "+b.eval(measureCards))
		want = append(want, name+": plain as stats, in-panel as stats, in-empty as stats, both as stats, box as stats")
	}
	check(t, "nesting", out, strings.Join(want, "|"))

	b.navigate("/a/nesting-brutal/?raw")
	check(t, "held", []string{b.eval(measureHeld)}, "12px 0px 0px | 0px | 0px")
	b.navigate("/a/nesting-default/?raw")
	check(t, "blocks", []string{b.eval(measureBlocks)}, "8px 0px 0px | 8px 0px 0px | 0px | 0px | 8px 0px 0px")
}
