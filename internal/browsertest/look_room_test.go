package browsertest_test

import (
	"encoding/json"
	"fmt"
	"testing"
)

// measurePartRoomAt finds each room between parts that is not --floe-room, read from the page (amiss), and
// how many boxes of a template's own Markdown it measured a room in (markdown). It
// measures the room between every two children one after another that are shown as blocks (not out of the flow),
// across when on one line, else down, in a place where parts stand: a box laid out as a grid or a flex that is a
// stack, holds cards, holds a part's box outside any part (a template's section or step), or is a text's box or the box of a
// template's own Markdown. Tabs are measured from
// the bottom of their names to the top of the panel shown, as their children are names and panels, not parts in a row.
// A part's Markdown stands in a text's box (MarkdownFlow), measured as a text's is.
// LIMIT: a flow in no box of Markdown (what a note drawn in HTML holds) and what a quote or a list holds (base.css's 0.6rem and 0.3rem, not room between parts) go unmeasured.
// LIMIT: the room before a template's own section (report's .sub, design's .decision), between a template's own
// section or header and a note it places (adr's record superseding it after its header, runbook's caution after its
// needs) and after the first child of an item whose mark stands in its
// grid's row mark (a step's head, the row as tall as the mark) is not measured, until the room around a template's
// heading is one rule. A cell is a pill in a line, not a box among parts, wherever it stands.
// LIMIT: what a tab not chosen holds is hidden, and goes unmeasured; the tab chosen is measured.
// It returns {seen, amiss, markdown}: the pairs it measured, and for each pair apart by other than --floe-room the later box's part (or the element) and how far.
const measurePartRoomAt = `(() => {
  let seen = 0;
  const probe = document.body.appendChild(document.createElement('div'));
  probe.style.width = 'var(--floe-room)';
  const room = probe.getBoundingClientRect().width;
  probe.remove();
  // A box of no height is no block: an empty text's box left in the grid would read as two rooms of 12px, not 24.
  const block = (e) => { const s = getComputedStyle(e); return e.getClientRects().length && e.getBoundingClientRect().height > 0 && !s.display.startsWith('inline') &&
    s.display !== 'contents' && !['absolute', 'fixed'].includes(s.position); };
  const box = (e) => e.matches('[data-floe-part]:not([data-floe-part=cell]), .card') && block(e);
  const name = (e) => e.tagName.toLowerCase() + (e.dataset.floePart ? '[' + e.dataset.floePart + ']' : '') +
    (e.dataset.floeLook ? '{' + e.dataset.floeLook + '}' : '') + (e.classList.length ? '.' + [...e.classList].join('.') : '');
  const out = new Set();
  let markdown = 0;
  for (const c of document.body.querySelectorAll('*')) {
    if (c.matches('[data-floe-part=tabs]') || !/grid|flex/.test(getComputedStyle(c).display)) continue;
    if (!(c.matches('[data-floe-part=stack], [data-floe-part=columns], [data-floe-part=columns] > div, [data-floe-text], [data-floe-markdown]') || [...c.children].some((k) => k.matches('.card')) ||
      ([...c.children].some(box) && !c.closest('[data-floe-part]')))) continue;
    const kids = [...c.children].filter(block), mark = getComputedStyle(c, '::before');
    if (c.matches('[data-floe-markdown]') && kids.length > 1) markdown++;
    const marked = mark.content !== 'none' && mark.display !== 'none' && mark.gridRowStart === 'mark';
    for (let i = 1; i < kids.length; i++) {
      const a = kids[i - 1], b = kids[i];
      seen++;
      const own = 'section:not([data-floe-part])';
      if (b.matches(own) || (a.matches(own + ', header') && b.matches('[data-floe-part=note]')) || (marked && i === 1)) continue;
      const r = a.getBoundingClientRect(), q = b.getBoundingClientRect();
      const d = q.top >= r.bottom - 0.5 ? q.top - r.bottom : q.left - r.right;
      if (Math.abs(d - room) > 0.5) out.add(JSON.stringify({ at: b.dataset.floePart || name(b), text: name(a) + ' + ' + name(b) + ' in ' + name(c) + ': ' + d.toFixed(1) }));
    }
  }
  for (const t of document.querySelectorAll('[data-floe-part=tabs]')) {
    const names = [...t.querySelectorAll(':scope > label')], panel = [...t.children].find((e) => e.matches('[data-floe-part]') && e.getClientRects().length);
    if (!names.length || !panel) continue;
    const d = panel.getBoundingClientRect().top - Math.max(...names.map((l) => l.getBoundingClientRect().bottom));
    seen++;
    if (Math.abs(d - room) > 0.5) out.add(JSON.stringify({ at: 'tabs', text: 'names + panel: ' + d.toFixed(1) }));
  }
  return JSON.stringify({ seen, amiss: [...out].sort().map((l) => JSON.parse(l)), markdown });
})()`

const lookRoomFix = "space parts and Markdown's blocks by --floe-room, and write no margin on a part's root"

// lookRoom measures every page of floePages, its details open, at 480 and 1100 px wide, as measurePartRoomAt does; a page of
// a template's own Markdown of two blocks or more must be among them.
func lookRoom(t *testing.T, s *site) measured {
	t.Helper()

	var m measured
	markdown := 0
	s.floePages(t, func(what string) {
		s.b.eval("document.querySelectorAll('details').forEach((d) => { d.open = true; })")
		for _, w := range []int64{480, 1100} {
			s.b.size(w, 900, false)
			m.add(s.b.measure(fmt.Sprintf("%s at %dpx", what, w), measurePartRoomAt))
			var got struct {
				Markdown int `json:"markdown"`
			}
			if err := json.Unmarshal([]byte(s.b.eval(measurePartRoomAt)), &got); err != nil {
				t.Fatal(err)
			}
			markdown += got.Markdown
			s.b.unsize()
		}
	})
	if markdown == 0 {
		t.Fatal("no box of a template's own Markdown measured: its mark or its grid changed, or no example's field holds two blocks")
	}
	return m
}

// brokenRoom draws a part of one's own that writes a margin on its root's top, after a text.
func brokenRoom(t *testing.T, s *site) (string, measured) {
	t.Helper()

	html := `<div data-floe-part="roomy"><p>r</p></div>`
	s.brokenPage(t, ownPart("roomy", html, ":scope { margin-top: 30px; }\n"), htmlPage(html))
	return "roomy", s.b.measure("broken", measurePartRoomAt)
}
