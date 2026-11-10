package browsertest_test

import (
	"fmt"
	"strings"
	"testing"

	"floe/internal/kit"
)

// measureOptionsLook returns a line for each thing options draws wrong in its cards on a line: their words or their
// parts not starting at one height, a parts box taller than what it holds, a card of options of no words ending not
// at its parts, a card holding parts that breaks on paper, and a small picture not at its size (160 by 90).
const measureOptionsLook = `(() => {
  const off = (a, b) => Math.abs(a - b) > 0.5, out = [];
  [...document.querySelectorAll('[data-floe-part=options]')].forEach((o, i) => {
    const lines = new Map();
    for (const c of o.children) { const top = Math.round(c.getBoundingClientRect().top); lines.set(top, [...(lines.get(top) || []), c]); }
    for (const [top, cards] of lines) {
      const where = 'options ' + i + ' line at ' + top;
      const at = (sel) => cards.map((c) => c.querySelector(':scope > ' + sel)).filter(Boolean).map((e) => e.getBoundingClientRect());
      const words = at('.body'), parts = at('.parts');
      if (words.some((r) => off(r.top, words[0].top))) out.push(where + ': words at ' + words.map((r) => r.top.toFixed(1)).join(' '));
      if (parts.some((r) => off(r.top, parts[0].top))) out.push(where + ': parts at ' + parts.map((r) => r.top.toFixed(1)).join(' '));
      for (const c of cards) {
        const p = c.querySelector(':scope > .parts');
        if (!p) continue;
        if (matchMedia('print').matches && getComputedStyle(c).breakInside !== 'avoid') out.push(where + ': a card breaks on paper');
        const r = p.getBoundingClientRect(), held = p.firstElementChild.getBoundingClientRect();
        if (off(r.height, held.height)) out.push(where + ': parts ' + r.height.toFixed(1) + ' tall, holding ' + held.height.toFixed(1));
        const cs = getComputedStyle(c), end = c.getBoundingClientRect().bottom - parseFloat(cs.borderBottomWidth) - parseFloat(cs.paddingBottom);
        if (!o.querySelector('.body') && off(end, r.bottom)) out.push(where + ': card ends ' + (end - r.bottom).toFixed(1) + ' under its parts');
      }
    }
  });
  const img = document.querySelector('img[alt=small]');
  if (!img) out.push('no small picture');
  else if (off(img.clientWidth, 160) || off(img.clientHeight, 90)) out.push('small picture ' + img.clientWidth + ' by ' + img.clientHeight);
  return out.join('\n');
})()`

// measureOptions measures, in each options holding parts, the room its rows keep by --cards-gap and --floe-room, read
// in the options: from each title to its parts, and from the lowest parts of a line to the words, the more of the two;
// and on a line of no parts, from the title to the words, --floe-room and --cards-gap (the row of the parts left empty,
// its gaps kept). It returns a line for each kept otherwise.
const measureOptions = `(() => {
  ` + pxJS + `
  const off = (a, b) => Math.abs(a - b) > 0.5;
  const out = [];
  [...document.querySelectorAll('[data-floe-part=options]:has(> .option > .parts)')].forEach((o, i) => {
    const room = px(o, 'var(--floe-room)'), gap = px(o, 'var(--cards-gap)'), most = Math.max(room, gap);
    const lines = new Map();
    for (const c of o.children) { const top = Math.round(c.getBoundingClientRect().top); lines.set(top, [...(lines.get(top) || []), c]); }
    for (const [top, cards] of lines) {
      const where = 'options ' + i + ' line at ' + top;
      const parts = cards.map((c) => c.querySelector(':scope > .parts')).filter(Boolean).map((e) => e.getBoundingClientRect());
      const words = cards.map((c) => c.querySelector(':scope > .body')).filter(Boolean).map((e) => e.getBoundingClientRect());
      for (const c of cards) {
        const t = c.querySelector(':scope > .title').getBoundingClientRect(), p = c.querySelector(':scope > .parts'), b = c.querySelector(':scope > .body');
        if (p) {
          const d = p.getBoundingClientRect().top - t.bottom;
          if (off(d, most)) out.push(where + ': parts ' + d.toFixed(1) + ' under the title, not ' + most.toFixed(1));
        } else if (b && !parts.length) {
          const d = b.getBoundingClientRect().top - t.bottom;
          if (off(d, room + gap)) out.push(where + ': words ' + d.toFixed(1) + ' under the title, not ' + (room + gap).toFixed(1));
        }
      }
      if (parts.length && words.length) {
        const d = words[0].top - Math.max(...parts.map((r) => r.bottom));
        if (off(d, most)) out.push(where + ': words ' + d.toFixed(1) + ' under the lowest parts, not ' + most.toFixed(1));
      }
    }
  });
  return out.join('\n');
})()`

// TestRenderedOptions draws optionsPage in every theme and in gappedTheme, whose --cards-gap is 1rem, at each of
// optionsWidths: options' rows holding parts keep their room by --cards-gap and --floe-room (measureOptions); the cards on a line start their parts
// and their words at one height, a small picture keeps its size and its box is not stretched to the row's, a card of
// parts alone ends at them, a card holding parts goes whole to the next page on paper (measureOptionsLook); and a
// theme's number on a card, in the rows or laid out of them (notebook's tape, washi's seal), stands as far below the
// card's top on a card holding parts as on a card of options without parts (across, it stands by the card's width).
func TestRenderedOptions(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	optionsFiles(t, s)
	page := s.write(t, "options.yaml", optionsPage)
	s.own(t, gappedTheme)
	for _, theme := range append(themes, "gapped") {
		s.floe(t, "publish", page, "--slug", "options-"+theme, "--theme", theme)
		optionsAt(t, s, "options-"+theme, func(width string) {
			if got := s.b.eval(measureOptions); got != "" {
				t.Errorf("%s at %s:\n  %s", theme, width, strings.ReplaceAll(got, "\n", "\n  "))
			}
			if got := s.b.eval(measureOptionsLook); got != "" {
				t.Errorf("%s at %s:\n  %s", theme, width, strings.ReplaceAll(got, "\n", "\n  "))
			}
			var at [2]string
			for i, sel := range []string{"[data-floe-part=options]:has(.parts) > .card", "[data-floe-part=options]:not(:has(.parts)) > .card"} {
				mark, _ := s.b.pseudoBoxes(sel)
				card := s.b.rect("document.querySelector(" + mustJSON(sel) + ")")
				if mark != nil {
					at[i] = fmt.Sprintf("%.1f", mark[1]-card.Y)
				}
			}
			if at[0] != at[1] {
				t.Errorf("%s at %s: the number at %s on a card with parts, at %s without", theme, width, at[0], at[1])
			}
		})
	}
}
