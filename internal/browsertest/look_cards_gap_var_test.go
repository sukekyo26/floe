package browsertest_test

import "testing"

// measureCardsGap finds, on a page whose --cards-gap is not 0.4rem, each card (.card) and each body of options' cards
// (.option > .body) whose gap is 0.4rem, cards' gap copied as a number that does not follow the variable. A gap a part
// chooses for itself, of another length, is its own.
// LIMIT: a gap a part chooses of 0.4rem is told as a copy; writing it var(--cards-gap) mends it, as it is the same.
// It returns {seen, amiss}: those it measured where --cards-gap is not 0.4rem, and for each copy its part and its gap.
const measureCardsGap = `(() => {
  ` + partOfJS + pxJS + `
  let seen = 0;
  const amiss = [], off = (a, b) => Math.abs(a - b) > 0.1;
  for (const e of document.querySelectorAll('.card, .option > .body')) {
    if (!e.getClientRects().length) continue;
    const gap = parseFloat(getComputedStyle(e).rowGap), copy = px(e, '0.4rem'), want = px(e, 'var(--cards-gap)');
    if (!off(want, copy)) continue;
    seen++;
    if (!off(gap, copy)) amiss.push({ at: partOf(e), text: (e.matches('.card') ? 'a card\'s' : 'an option\'s body\'s') + ' gap ' + gap + 'px, 0.4rem, against --cards-gap ' + want + 'px' });
  }
  return JSON.stringify({ seen, amiss });
})()`

// gappedTheme is a theme of one's own that sets --cards-gap on its cards.css's :scope, as a theme changes the gaps in
// a card.
var gappedTheme = map[string]string{"themes/gapped/cards.css": ":scope { --cards-gap: 1rem; }\n"}

const lookCardsGapVarFix = "space what a card holds by var(--cards-gap), not by 0.4rem copied; a theme changes it by --cards-gap"

// lookCardsGapVar draws every part (everyPart) and optionsPage in gappedTheme, whose --cards-gap is 1rem, as
// measureCardsGap measures: no gap in a card stays 0.4rem when the variable is otherwise.
func lookCardsGapVar(t *testing.T, s *site) measured {
	t.Helper()

	s.own(t, gappedTheme)
	optionsFiles(t, s)
	var m measured
	for slug, data := range map[string]string{"every-part": s.everyPart(t), "options": optionsPage} {
		s.floe(t, "publish", s.write(t, slug+".yaml", data), "--slug", "gap-"+slug, "--theme", "gapped")
		s.b.navigate("/a/gap-" + slug + "/?raw")
		m.add(s.b.measure(slug+" in gapped", measureCardsGap))
	}
	return m
}

// brokenCardsGapVar draws, in gappedTheme, a part of one's own that takes the look cards and copies its gap, 0.4rem.
func brokenCardsGapVar(t *testing.T, s *site) (string, measured) {
	t.Helper()

	html := `<div data-floe-part="cardy"><div class="card"><p>a</p><p>b</p></div></div>`
	own := ownPart("cardy", html, ":scope .card { gap: 0.4rem; }\n")
	own["parts/cardy/part.yaml"] = "looks: [cards]\n"
	for k, v := range gappedTheme {
		own[k] = v
	}
	s.brokenPage(t, own, htmlPage(html)+"theme: gapped\n")
	return "cardy", s.b.measure("broken", measureCardsGap)
}
