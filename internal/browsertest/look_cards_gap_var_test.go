package browsertest_test

import "testing"

// cardsGapIn returns a measure that finds each element sel finds whose gap is not --cards-gap, read in it: a number
// copied, 0.4rem, as a gap a part chooses of its own, and one of no length (normal). It returns {seen, amiss}: those it
// measured, and for each otherwise its part ("look cards" for an element of no part that takes the look) and its gap.
func cardsGapIn(sel string) string {
	return `((sel) => {
  ` + partOfJS + pxJS + `
  let seen = 0;
  const amiss = [];
  for (const e of document.querySelectorAll(sel)) {
    if (!e.getClientRects().length) continue;
    seen++;
    const gap = parseFloat(getComputedStyle(e).rowGap), want = px(e, 'var(--cards-gap)');
    const at = e.closest('[data-floe-look~="cards"]:not([data-floe-part])') ? 'look cards' : partOf(e);
    if (!(Math.abs(gap - want) <= 0.1)) amiss.push({ at, text: (e.matches('.card') ? 'a card\'s' : 'an option\'s body\'s') + ' gap ' + gap + 'px against --cards-gap ' + want + 'px' });
  }
  return JSON.stringify({ seen, amiss });
})(` + mustJSON(sel) + `)`
}

// measureCardsGap is cardsGapIn of every card (.card) and every body of options' cards (.option > .body).
var measureCardsGap = cardsGapIn(".card, .option > .body")

// gappedTheme is a theme of one's own that sets --cards-gap on its cards.css's :scope, as a theme changes the gaps in
// a card.
var gappedTheme = map[string]string{"themes/gapped/cards.css": ":scope { --cards-gap: 1rem; }\n"}

const lookCardsGapVarFix = "space what a card holds by var(--cards-gap), as the look cards does: copy no 0.4rem, and choose no gap of your own"

// lookCardsGapVar draws every part (everyPart) and optionsPage in gappedTheme, whose --cards-gap is 1rem, as
// measureCardsGap measures: every gap in a card follows the variable, there a gap left 0.4rem tells a copy.
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
