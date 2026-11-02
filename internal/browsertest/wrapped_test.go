package browsertest_test

import (
	"fmt"
	"strings"
	"testing"
)

// measureWrapped opens every details and shows each tab in turn, and in each of these states finds the rows that wrap
// (a flex row that wraps, and a grid whose children stand in fewer columns when the grid is 1px wide than when it is
// 100000px wide: auto-fit, auto-fill) and returns, for each, the kinds of its children (the same tag, part and look)
// that differ from one child to another in their borders' widths, styles and corners, padding, margins, or their
// ::before and ::after (content, borders, size), and those of one tone whose borders' colours differ.
// A child in a state marked by a class (options' pick) is of its siblings' kind, so that a class does not set the
// first child apart: a state is drawn by an outline, a ground or --floe-inset, not by a border or room, as a theme's
// decoration is. A disabled child is left out, and a side without a line is not read for its colour (a picked tab's).
const measureWrapped = `(() => {
  const cols = (e) => new Set([...e.children].filter((c) => c.getClientRects().length).map((c) => Math.round(c.getBoundingClientRect().left))).size
  const at = (e, width) => {
    const was = e.style.cssText
    for (const [k, v] of [['width', width], ['min-width', '0'], ['max-width', 'none']]) e.style.setProperty(k, v, 'important')
    const n = cols(e)
    e.style.cssText = was
    return n
  }
  const sides = ['Top', 'Right', 'Bottom', 'Left'], corners = ['TopLeft', 'TopRight', 'BottomRight', 'BottomLeft']
  const frame = (s) => sides.map((side) => [s['border' + side + 'Width'], s['border' + side + 'Style']].join(' ')).join(', ')
  const shape = (c) => {
    const s = getComputedStyle(c)
    const pseudo = ['::before', '::after'].map((p) => {
      const q = getComputedStyle(c, p)
      return p + ' ' + [q.content, frame(q), q.width, q.height].join(' ')
    })
    return [frame(s), 'corners ' + corners.map((k) => s['border' + k + 'Radius']).join(' '),
      'padding ' + sides.map((side) => s['padding' + side]).join(' '), 'margin ' + sides.map((side) => s['margin' + side]).join(' '),
      ...pseudo].join('; ')
  }
  const rows = () => [...document.querySelectorAll('body *')].filter((e) => {
    const s = getComputedStyle(e)
    return /flex/.test(s.display) ? s.flexWrap !== 'nowrap' : /grid/.test(s.display) && at(e, '1px') < at(e, '100000px')
  }).flatMap((e) => {
    const kinds = new Map(), add = (key, look) => kinds.set(key, (kinds.get(key) || new Set()).add(look))
    for (const c of e.children) {
      if (!c.getClientRects().length || c.matches(':disabled')) continue
      const kind = c.tagName.toLowerCase() + ['floePart', 'floeLook'].map((d) => c.dataset[d] ? '[' + c.dataset[d] + ']' : '').join('')
      const s = getComputedStyle(c)
      add(kind, shape(c))
      add(kind + ' of tone ' + (c.dataset.floeTone || 'none'), sides.map((side) => s['border' + side + 'Style'] === 'none' ? '' : s['border' + side + 'Color']).join(', '))
    }
    const part = e.closest('[data-floe-part]')
    const where = (part ? part.dataset.floePart + ' ' : '') + e.tagName.toLowerCase() + [...e.classList].map((x) => '.' + x).join('')
    return [...kinds].filter(([, looks]) => looks.size > 1).map(([key, looks]) => where + ' > ' + key + ': ' + [...looks].join(' / '))
  })
  document.querySelectorAll('details').forEach((d) => { d.open = true })
  const out = new Set()
  for (const tab of [null, ...document.querySelectorAll('[data-floe-part="tabs"] input[type="radio"]')]) {
    if (tab) tab.checked = true
    rows().forEach((l) => out.add(l))
  }
  return [...out].join('\n')
})()`

// TestUIWrappedAlike draws every part and floe's templates' examples in every theme at a phone's width and a wide
// screen's, and finds the children of one kind in every row that wraps drawn alike (measureWrapped): which child
// begins a line is the width's, so no CSS marks a child by its place in such a row.
// LIMIT: a row is found only where the samples draw it with two children or more, at these two widths, and one's own
// parts and templates are not drawn; a row a sample does not reach is seen on the page
func TestUIWrappedAlike(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.size(480, 900, false)
	defer b.unsize()
	var amiss []string
	s.floePages(t, func(what string) {
		for _, w := range []int64{480, 1600} {
			b.size(w, 900, false)
			if got := b.eval(measureWrapped); got != "" {
				for _, l := range strings.Split(got, "\n") {
					amiss = append(amiss, fmt.Sprintf("%s at %dpx: %s", what, w, l))
				}
			}
		}
		b.size(480, 900, false)
	})
	check(t, "wrapped rows", amiss, "")
}
