package browsertest_test

import (
	"strings"
	"testing"
)

// measurePartRoom returns, one to a line, each room between parts that is not --floe-room, read from the page. It
// measures, in each box laid out as a grid or a flex that is a stack or holds a part's box or a card, the room between
// each two children one after another that are shown as blocks (not out of the flow): across when on one line, else
// down. Tabs are measured from the bottom of their names to the top of the panel shown, as their children are names
// and panels, not parts in a row.
// LIMIT: a place of a template's that holds only text (a section of Markdown alone) is not known from the drawn page
// and goes unmeasured there; the same place holding a part elsewhere is measured.
// LIMIT: the room before a template's section in a section (report's .sub, design's .decision) and after the first
// child of an item whose mark stands in its grid's row mark (a step's head, the row as tall as the mark) is not
// measured: the room above a heading of a template's is S151's.
const measurePartRoom = `(() => {
  const probe = document.body.appendChild(document.createElement('div'));
  probe.style.width = 'var(--floe-room)';
  const room = probe.getBoundingClientRect().width;
  probe.remove();
  const block = (e) => { const s = getComputedStyle(e); return e.getClientRects().length && !s.display.startsWith('inline') &&
    s.display !== 'contents' && !['absolute', 'fixed'].includes(s.position); };
  const box = (e) => e.matches('[data-floe-part], .card') && block(e);
  const name = (e) => e.tagName.toLowerCase() + (e.dataset.floePart ? '[' + e.dataset.floePart + ']' : '') +
    (e.dataset.floeLook ? '{' + e.dataset.floeLook + '}' : '') + (e.classList.length ? '.' + [...e.classList].join('.') : '');
  const out = new Set();
  for (const c of document.body.querySelectorAll('*')) {
    if (c.matches('[data-floe-part=tabs]') || !/grid|flex/.test(getComputedStyle(c).display) || !(c.matches('[data-floe-part=stack]') || [...c.children].some(box))) continue;
    const kids = [...c.children].filter(block), mark = getComputedStyle(c, '::before');
    const marked = mark.content !== 'none' && mark.display !== 'none' && mark.gridRowStart === 'mark';
    for (let i = 1; i < kids.length; i++) {
      const a = kids[i - 1], b = kids[i];
      if (b.matches('section:not([data-floe-part])') || (marked && i === 1)) continue;
      const r = a.getBoundingClientRect(), q = b.getBoundingClientRect();
      const d = q.top >= r.bottom - 0.5 ? q.top - r.bottom : q.left - r.right;
      if (Math.abs(d - room) > 0.5) out.add(name(a) + ' + ' + name(b) + ' in ' + name(c) + ': ' + d.toFixed(1));
    }
  }
  for (const t of document.querySelectorAll('[data-floe-part=tabs]')) {
    const names = [...t.querySelectorAll(':scope > label')], panel = [...t.children].find((e) => e.matches('[data-floe-part]') && e.getClientRects().length);
    if (!names.length || !panel) continue;
    const d = panel.getBoundingClientRect().top - Math.max(...names.map((l) => l.getBoundingClientRect().bottom));
    if (Math.abs(d - room) > 0.5) out.add('tabs names + panel: ' + d.toFixed(1));
  }
  return [...out].sort().join('\n');
})()`

// TestUIRoom draws every part and floe's templates' examples in every theme: wherever parts stand one after another
// (a template's section, a step, a stack, cards, tabs' names and panel), the room between them, and between a heading
// or a paragraph and a part there, is --floe-room.
func TestUIRoom(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	s.b.size(1100, 900, false)
	defer s.b.unsize()
	var amiss []string
	s.floePages(t, func(what string) {
		if got := s.b.eval(measurePartRoom); got != "" {
			for _, l := range strings.Split(got, "\n") {
				amiss = append(amiss, what+": "+l)
			}
		}
	})
	check(t, "room", amiss, "")
}
