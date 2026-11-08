package browsertest_test

import "testing"

// measurePrint finds each element with a ground (a colour or an image) that print would leave out: one whose
// print-color-adjust is not exact, read with the page drawn for print. It returns {seen, amiss}: the elements with a
// ground, and for each left out its part and its print-color-adjust.
const measurePrint = `(() => {
  ` + partOfJS + groundedJS + `
  let seen = 0;
  const out = new Set();
  for (const e of [document.documentElement, ...document.querySelectorAll('*')]) {
    const s = getComputedStyle(e);
    if (!grounded(s)) continue;
    seen++;
    if (s.printColorAdjust !== 'exact') out.add(JSON.stringify({ at: partOf(e), text: e.tagName.toLowerCase() + [...e.classList].map((c) => '.' + c).join('') + ': ' + s.printColorAdjust }));
  }
  return JSON.stringify({ seen, amiss: [...out].map((l) => JSON.parse(l)) });
})()`

const lookPaperGroundFix = "leave print-color-adjust to base.css, which keeps every ground on paper (exact)"

// lookPaperGround measures every page of floePages on paper (measurePrint): print leaves a ground out unless asked, as
// a browser's print does by default, and a light text on a fill would be lost on white paper.
func lookPaperGround(t *testing.T, s *site) measured {
	t.Helper()

	onPaper(t, s)
	var m measured
	s.floePages(t, func(what string) { m.add(s.b.measure(what, measurePrint)) })
	return m
}

// brokenPaperGround draws a note in a theme of one's own whose note.css leaves its ground to print's economy.
func brokenPaperGround(t *testing.T, s *site) (string, measured) {
	t.Helper()

	onPaper(t, s)
	s.brokenPage(t, map[string]string{"themes/eco/note.css": ":scope { print-color-adjust: economy; }\n"}, themePage("eco", "- note: { text: a ground }\n"))
	return "note", s.b.measure("broken", measurePrint)
}
