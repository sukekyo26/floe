package browsertest_test

import "testing"

// measureContain finds each element that scrolls sideways and tells its content's width outside, with no contain of
// inline-size (size and strict hold it); a form's control scrolls by the browser's own sheet. It returns {seen,
// amiss}: the elements that scroll sideways, and for each with no such contain its part and its text.
const measureContain = `(() => {
  ` + scrollsJS + `
  ` + partOfJS + `
  let seen = 0;
  const amiss = [];
  for (const e of document.querySelectorAll('*')) {
    if (e.matches('textarea, select, input') || !scrolls(e)) continue;
    seen++;
    if (!getComputedStyle(e).contain.split(' ').some((v) => ['inline-size', 'size', 'strict'].includes(v))) amiss.push({ at: partOf(e), text: e.tagName.toLowerCase() + ' ' + e.textContent.trim().slice(0, 40) });
  }
  return JSON.stringify({ seen, amiss });
})()`

const lookScrollContainFix = "give a box that scrolls sideways on screen contain: inline-size, written with its overflow-x: auto under @media screen"

// lookScrollContain measures wrapPage in every theme, as wide as a phone, as measureContain does.
func lookScrollContain(t *testing.T, s *site) measured {
	t.Helper()

	var m measured
	wrapThemes(t, s, func(theme string) { m.add(s.b.measure("wrapPage in "+theme, measureContain)) })
	return m
}

// brokenScrollContain draws a part of one's own that scrolls sideways on screen with no contain.
func brokenScrollContain(t *testing.T, s *site) (string, measured) {
	t.Helper()

	html := `<div data-floe-part="scroller"><p>` + longWord + `</p></div>`
	onPhone(t, s, 390)
	s.brokenPage(t, ownPart("scroller", html, "@media screen { :scope { overflow-x: auto; } }\n:scope p { white-space: nowrap; }\n"), htmlPage(html))
	return "scroller", s.b.measure("broken", measureContain)
}
