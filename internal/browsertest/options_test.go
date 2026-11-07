package browsertest_test

import (
	"fmt"
	"os"
	"strings"
	"testing"
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
// its title in options of no parts, not --floe-room under it on a line of no parts in options holding some (the row
// of the parts left empty). A theme's number in the rows is the cards' gap above its title, and so are the pros below
// the words.
const measureOptions = `(() => {
  const px = (v) => { const d = document.body.appendChild(document.createElement('div')); d.style.width = v; const w = d.getBoundingClientRect().width; d.remove(); return w; };
  const room = px('var(--floe-room)'), off = (a, b) => Math.abs(a - b) > 0.5;
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
          const d = b.getBoundingClientRect().top - t.bottom, want = !o.querySelector('.parts') ? small : !parts.length ? room : null;
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
  if (off(img.clientWidth, 160) || off(img.clientHeight, 90)) out.push('small picture ' + img.clientWidth + ' by ' + img.clientHeight);
  return out.join('\n');
})()`

// TestUIOptionsRows draws options holding parts in every theme, at a laptop's width (three cards on a line), a
// phone's (one) and on paper, and options without parts beside them: the cards on a line start their parts and
// their words at one height, --floe-room from the title and from the lowest parts, a small picture keeps its size and
// its box is not stretched to the row's, a theme's number stands where it stands without parts, a card of parts alone
// ends at them; options without parts keep their words the cards' gap under the title. A theme of one's own setting
// --cards-gap on its cards.css's :scope sets the gaps in a card by it.
func TestUIOptionsRows(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	themes, err := os.ReadDir("../kit/floe/themes")
	if err != nil {
		t.Fatal(err)
	}
	s.write(t, "tall.svg", `<svg xmlns="http://www.w3.org/2000/svg" width="300" height="600"><rect width="300" height="600" fill="#ccc"/></svg>`)
	s.write(t, "small.svg", `<svg xmlns="http://www.w3.org/2000/svg" width="160" height="90"><rect width="160" height="90" fill="#999"/></svg>`)
	page := s.write(t, "options.yaml", optionsPage)
	s.own(t, map[string]string{"themes/gapped/cards.css": ":scope { --cards-gap: 1rem; }\n"})
	names := make([]string, 0, 1+len(themes))
	names = append(names, "gapped")
	for _, theme := range themes {
		names = append(names, theme.Name())
	}
	var amiss []string
	for _, name := range names {
		s.floe(t, "publish", page, "--slug", "options-"+name, "--theme", name)
		for _, w := range []struct {
			name  string
			width int64
			media string
		}{{"1440", 1440, ""}, {"400", 400, ""}, {"paper", paperWidth, "print"}} {
			b.size(w.width, 900, false)
			b.media(w.media)
			b.navigate("/a/options-" + name + "/?raw")
			if got := b.eval(measureOptions); got != "" {
				amiss = append(amiss, fmt.Sprintf("%s at %s:\n%s", name, w.name, got))
			}
			if gap := b.eval("getComputedStyle(document.querySelector('.option > .body')).rowGap"); name == "gapped" && gap != "16px" {
				amiss = append(amiss, fmt.Sprintf("gapped at %s: the words and the pros %s apart, not the theme's --cards-gap 16px", w.name, gap))
			}
			// A theme's number, in the rows or laid out of them (notebook's tape, washi's seal), stands as far below a
			// card's top as on a card of options without parts (across, it stands by the card's width).
			var at [2]string
			for i, sel := range []string{"[data-floe-part=options]:has(.parts) > .card", "[data-floe-part=options]:not(:has(.parts)) > .card"} {
				mark, _ := b.pseudoBoxes(sel)
				card := b.rect("document.querySelector(" + mustJSON(sel) + ")")
				if mark != nil {
					at[i] = fmt.Sprintf("%.1f", mark[1]-card.Y)
				}
			}
			if at[0] != at[1] {
				amiss = append(amiss, fmt.Sprintf("%s at %s: the number at %s on a card with parts, at %s without", name, w.name, at[0], at[1]))
			}
		}
	}
	b.media("")
	b.unsize()
	if len(amiss) > 0 {
		t.Error(strings.Join(amiss, "\n"))
	}
}
