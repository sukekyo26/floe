package browsertest_test

import "testing"

// measureScrollGround finds each element with a ground (a background colour or image) in a box that scrolls sideways,
// and does, that ends left of what it holds: the right of its elements and text, what an inner box clips (one whose
// overflow-x is not visible, the element itself too) counted to that box's own right, and visually hidden text not
// counted. It returns {seen, amiss}: the boxes that scroll and do, and for each ground that ends short the part of its
// box and how far.
// LIMIT: a frame (a border, --floe-inset's shadow), a ground of ::before or ::after and the ground of an element laid in
// a line (inline, inline-block and the like) are not measured; measure them too if a theme comes to paint them inside a box that scrolls.
const measureScrollGround = `(() => {
  ` + scrollsJS + `
  ` + partOfJS + `
  let seen = 0;
  const out = new Set();
  const reach = (e) => {
    let far = e.getBoundingClientRect().right;
    // A box that clips, as a pre that scrolls in a table, shows nothing past its own right.
    if (getComputedStyle(e).overflowX !== 'visible') return far;
    const walk = document.createTreeWalker(e, NodeFilter.SHOW_ELEMENT | NodeFilter.SHOW_TEXT, (n) => {
      if (n.nodeType === Node.TEXT_NODE) return NodeFilter.FILTER_ACCEPT;
      if (n.matches('.floe-hidden')) return NodeFilter.FILTER_REJECT;
      // An inner box clips what reaches past it: its own right is what shows.
      if (getComputedStyle(n).overflowX !== 'visible') {
        far = Math.max(far, n.getBoundingClientRect().right);
        return NodeFilter.FILTER_REJECT;
      }
      return NodeFilter.FILTER_ACCEPT;
    });
    for (let n; (n = walk.nextNode()); ) {
      if (n.nodeType === Node.TEXT_NODE) {
        const r = document.createRange();
        r.selectNodeContents(n);
        for (const c of r.getClientRects()) far = Math.max(far, c.right);
      } else {
        far = Math.max(far, n.getBoundingClientRect().right);
      }
    }
    return far;
  };
  for (const box of document.querySelectorAll('*')) {
    if (!scrolls(box) || box.scrollWidth <= box.clientWidth) continue;
    seen++;
    for (const e of box.querySelectorAll('*')) {
      const s = getComputedStyle(e);
      if ((s.backgroundColor === 'rgba(0, 0, 0, 0)' && s.backgroundImage === 'none') || s.display.startsWith('inline')) continue;
      const right = e.getBoundingClientRect().right, far = reach(e);
      if (far > right + 1) out.add(JSON.stringify({ at: partOf(box), text: e.tagName.toLowerCase() + ' in ' + (box.dataset.floePart || box.tagName.toLowerCase()) + ' ends ' + Math.round(far - right) + ' px short' }));
    }
  }
  return JSON.stringify({ seen, amiss: [...out].map((l) => JSON.parse(l)) });
})()`

const lookScrollGroundFix = "stretch a ground in a box that scrolls sideways to the end of what it holds (as tree's list: width: fit-content; min-width: 100%)"

// lookScrollGround measures wrapPage in every theme, as wide as a phone, as measureScrollGround does; its trees scroll
// in each, or nothing in them is measured.
func lookScrollGround(t *testing.T, s *site) measured {
	t.Helper()

	var m measured
	wrapThemes(t, s, func(theme string) {
		if got := s.b.eval("(() => { const trees = [...document.querySelectorAll('[data-floe-part=tree]')]; return trees.length > 0 && trees.every((e) => e.scrollWidth > e.clientWidth); })()"); got != "true" {
			t.Fatalf("%s: wrapPage has no tree, or one that does not scroll sideways", theme)
		}
		m.add(s.b.measure("wrapPage in "+theme, measureScrollGround))
	})
	return m
}

// brokenScrollGround draws a part of one's own that scrolls sideways, as the rules say, and holds a ground that stops at
// the box's width, short of its text.
func brokenScrollGround(t *testing.T, s *site) (string, measured) {
	t.Helper()

	html := `<div data-floe-part="groundy"><p class="g">` + longWord + `</p></div>`
	onPhone(t, s, 390)
	s.brokenPage(t, ownPart("groundy", html, "@media screen { :scope { overflow-x: auto; contain: inline-size; } }\n:scope .g { background: #ddd; white-space: nowrap; }\n"), htmlPage(html))
	return "groundy", s.b.measure("broken", measureScrollGround)
}
