package browsertest_test

import (
	"testing"

	"github.com/sukekyo26/floe/internal/kit"
)

// optionsPage holds options with parts (a tall picture, a small one under a title of two lines, a card without
// parts), options without parts and options of parts alone, each as its own part.
const optionsPage = `template: report
lang: en
title: Options
sections:
  - heading: Options
    parts:
      - options:
          - title: Tall
            parts: [ { image: { file: tall.svg, alt: tall } } ]
            text: The words of the tall one.
            pros: [Seen whole]
          - title: A title long enough to wrap onto a second line in a card of a third of the column
            parts: [ { image: { file: small.svg, alt: small } } ]
            text: The words of the small one.
          - title: Bare
            text: No parts.
            cons: [Nothing to see]
      - options:
          - { title: One, text: first, pros: [a] }
          - { title: Two, text: second }
      - options:
          - { title: Seen, parts: [ { image: { file: small.svg, alt: seen } } ] }
          - { title: Also seen, parts: [ { image: { file: small.svg, alt: also } } ] }
`

// measureOptions returns, as lines, how the cards of each options lie: for each line of cards (by their tops), the
// tops of their words and their parts, apart (tops that differ), the room from each title to its parts and from
// the lowest parts of the line to the words, a parts box taller than what it holds, a card ending not at the parts it
// ends with, and the words of a card of no parts not the cards' gap (--cards-gap, as the options' box reads it) under
// its title in options of no parts, not --floe-room and the cards' gap under it on a line of no parts in options holding
// some (the row of the parts left empty, its gaps kept). A theme's number in the rows is the cards' gap above its title, and so are the pros below
// the words. It returns {seen, amiss}: the cards it measured, and what is amiss, at options.
const measureOptions = `(() => {
  const px = (v) => { const d = document.body.appendChild(document.createElement('div')); d.style.width = v; const w = d.getBoundingClientRect().width; d.remove(); return w; };
  const room = px('var(--floe-room)'), off = (a, b) => Math.abs(a - b) > 0.5;
  let seen = 0;
  const out = [];
  [...document.querySelectorAll('[data-floe-part=options]')].forEach((o, i) => {
    const probe = o.appendChild(document.createElement('div'));
    probe.style.cssText = 'position: absolute; width: var(--cards-gap)';
    const small = probe.getBoundingClientRect().width;
    probe.remove();
    const lines = new Map();
    for (const c of o.children) { const top = Math.round(c.getBoundingClientRect().top); lines.set(top, [...(lines.get(top) || []), c]); }
    for (const [top, cards] of lines) {
      const where = 'options ' + i + ' line at ' + top;
      const at = (sel) => cards.map((c) => c.querySelector(':scope > ' + sel)).filter(Boolean).map((e) => e.getBoundingClientRect());
      const words = at('.body'), parts = at('.parts'), titles = at('.title');
      if (words.some((r) => off(r.top, words[0].top))) out.push(where + ': words at ' + words.map((r) => r.top.toFixed(1)).join(' '));
      if (parts.some((r) => off(r.top, parts[0].top))) out.push(where + ': parts at ' + parts.map((r) => r.top.toFixed(1)).join(' '));
      for (const c of cards) {
        seen++;
        if (matchMedia('print').matches && c.querySelector(':scope > .parts') && getComputedStyle(c).breakInside !== 'avoid') out.push(where + ': a card breaks on paper');
        const t = c.querySelector(':scope > .title').getBoundingClientRect(), p = c.querySelector(':scope > .parts'), b = c.querySelector(':scope > .body');
        const before = getComputedStyle(c, '::before');
        if (before.content !== 'none' && before.position !== 'absolute' && before.display !== 'none') {
          const n = c.getBoundingClientRect().top + c.clientTop + parseFloat(getComputedStyle(c).paddingTop) + parseFloat(before.height);
          if (off(t.top - n, small)) out.push(where + ': title ' + (t.top - n).toFixed(1) + ' under its number');
        }
        if (b && b.children.length > 1) {
          const d = b.children[1].getBoundingClientRect().top - b.children[0].getBoundingClientRect().bottom;
          if (off(d, small)) out.push(where + ': pros ' + d.toFixed(1) + ' under the words');
        }
        if (p) {
          const r = p.getBoundingClientRect(), held = p.firstElementChild.getBoundingClientRect();
          if (off(r.top - t.bottom, room)) out.push(where + ': parts ' + (r.top - t.bottom).toFixed(1) + ' under the title');
          if (off(r.height, held.height)) out.push(where + ': parts ' + r.height.toFixed(1) + ' tall, holding ' + held.height.toFixed(1));
          const cs = getComputedStyle(c), end = c.getBoundingClientRect().bottom - parseFloat(cs.borderBottomWidth) - parseFloat(cs.paddingBottom);
          if (!b && !o.querySelector('.body') && off(end, r.bottom)) out.push(where + ': card ends ' + (end - r.bottom).toFixed(1) + ' under its parts');
        } else if (b) {
          const d = b.getBoundingClientRect().top - t.bottom, want = !o.querySelector('.parts') ? small : !parts.length ? room + small : null;
          if (want !== null && off(d, want)) out.push(where + ': words ' + d.toFixed(1) + ' under the title');
        }
      }
      if (parts.length && words.length) {
        const d = words[0].top - Math.max(...parts.map((r) => r.bottom));
        if (off(d, room)) out.push(where + ': words ' + d.toFixed(1) + ' under the lowest parts');
      }
    }
  });
  const img = document.querySelector('img[alt=small]');
  if (img && (off(img.clientWidth, 160) || off(img.clientHeight, 90))) out.push('small picture ' + img.clientWidth + ' by ' + img.clientHeight);
  return JSON.stringify({ seen, amiss: out.map((text) => ({ at: 'options', text })) });
})()`

// optionsWidths are the widths options is measured at: a laptop's (three cards on a line), a phone's (one) and paper's.
var optionsWidths = []struct {
	name  string
	width int64
	media string
}{{"1440", 1440, ""}, {"400", 400, ""}, {"paper", paperWidth, "print"}}

// optionsFiles writes the pictures optionsPage shows.
func optionsFiles(t *testing.T, s *site) {
	t.Helper()

	s.write(t, "tall.svg", `<svg xmlns="http://www.w3.org/2000/svg" width="300" height="600"><rect width="300" height="600" fill="#ccc"/></svg>`)
	s.write(t, "small.svg", `<svg xmlns="http://www.w3.org/2000/svg" width="160" height="90"><rect width="160" height="90" fill="#999"/></svg>`)
}

// optionsAt visits the page slug at each of optionsWidths, and leaves the window as it was.
func optionsAt(t *testing.T, s *site, slug string, visit func(width string)) {
	t.Helper()

	defer s.b.unsize()
	defer s.b.media("")
	for _, w := range optionsWidths {
		s.b.size(w.width, 900, false)
		s.b.media(w.media)
		s.b.navigate("/a/" + slug + "/?raw")
		visit(w.name)
	}
}

const lookCardsGapFix = "lay out a card's rows by --cards-gap and --floe-room as options does; a theme writes no gap on .card, and sets --cards-gap instead"

// lookCardsGap draws options holding parts in every theme at each of optionsWidths, and options without parts beside
// them, as measureOptions measures: the cards on a line start their parts and their words at one height, --floe-room
// from the title and from the lowest parts, a small picture keeps its size and its box is not stretched to the row's,
// a card of parts alone ends at them; options without parts keep their words the cards' gap under the title.
func lookCardsGap(t *testing.T, s *site) measured {
	t.Helper()

	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	optionsFiles(t, s)
	page := s.write(t, "options.yaml", optionsPage)
	var m measured
	for _, theme := range themes {
		s.floe(t, "publish", page, "--slug", "options-"+theme, "--theme", theme)
		optionsAt(t, s, "options-"+theme, func(width string) { m.add(s.b.measure(theme+" at "+width, measureOptions)) })
	}
	return m
}

// brokenCardsGap draws options in a theme of one's own whose cards.css writes a gap on .card.
func brokenCardsGap(t *testing.T, s *site) (string, measured) {
	t.Helper()

	optionsFiles(t, s)
	s.brokenPage(t, map[string]string{"themes/gappy/cards.css": ":scope .card { gap: 3px; }\n"}, optionsPage+"theme: gappy\n")
	s.b.size(1440, 900, false)
	defer s.b.unsize()
	return "options", s.b.measure("broken", measureOptions)
}
