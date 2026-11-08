package browsertest_test

import "testing"

// measureCardsGap finds each card (.card) and each body of options' cards (.option > .body) whose gap is not
// --cards-gap, read in it: a number copied, 0.4rem, as a gap a part chooses of its own. It returns {seen, amiss}: those
// it measured, and for each otherwise its part and its gap.
const measureCardsGap = `(() => {
  ` + partOfJS + pxJS + `
  let seen = 0;
  const amiss = [];
  for (const e of document.querySelectorAll('.card, .option > .body')) {
    if (!e.getClientRects().length) continue;
    seen++;
    const gap = parseFloat(getComputedStyle(e).rowGap), want = px(e, 'var(--cards-gap)');
    if (Math.abs(gap - want) > 0.1) amiss.push({ at: partOf(e), text: (e.matches('.card') ? 'a card\'s' : 'an option\'s body\'s') + ' gap ' + gap + 'px against --cards-gap ' + want + 'px' });
  }
  return JSON.stringify({ seen, amiss });
})()`

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
