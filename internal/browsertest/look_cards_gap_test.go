package browsertest_test

import (
	"testing"

	"floe/internal/kit"
)

// optionsPage holds options with parts (a tall picture, a small one under a title of two lines, a card without
// parts), options without parts and options of parts alone, each as its own part.
const optionsPage = `template: report
lang: en
title: Options
sections:
  - heading: Options
    parts:
      - options:
          - title: Tall
            parts: [ { image: { file: tall.svg, alt: tall } } ]
            text: The words of the tall one.
            pros: [Seen whole]
          - title: A title long enough to wrap onto a second line in a card of a third of the column
            parts: [ { image: { file: small.svg, alt: small } } ]
            text: The words of the small one.
          - title: Bare
            text: No parts.
            cons: [Nothing to see]
      - options:
          - { title: One, text: first, pros: [a] }
          - { title: Two, text: second }
      - options:
          - { title: Seen, parts: [ { image: { file: small.svg, alt: seen } } ] }
          - { title: Also seen, parts: [ { image: { file: small.svg, alt: also } } ] }
`

// optionsWidths are the widths options is measured at: a laptop's (three cards on a line), a phone's (one) and paper's.
var optionsWidths = []struct {
	name  string
	width int64
	media string
}{{"1440", 1440, ""}, {"400", 400, ""}, {"paper", paperWidth, "print"}}

// optionsFiles writes the pictures optionsPage shows.
func optionsFiles(t *testing.T, s *site) {
	t.Helper()

	s.write(t, "tall.svg", `<svg xmlns="http://www.w3.org/2000/svg" width="300" height="600"><rect width="300" height="600" fill="#ccc"/></svg>`)
	s.write(t, "small.svg", `<svg xmlns="http://www.w3.org/2000/svg" width="160" height="90"><rect width="160" height="90" fill="#999"/></svg>`)
}

// optionsAt visits the page slug at each of optionsWidths, and leaves the window as it was.
func optionsAt(t *testing.T, s *site, slug string, visit func(width string)) {
	t.Helper()

	defer s.b.unsize()
	defer s.b.media("")
	for _, w := range optionsWidths {
		s.b.size(w.width, 900, false)
		s.b.media(w.media)
		s.b.navigate("/a/" + slug + "/?raw")
		visit(w.name)
	}
}

// cardsLook is an element of html that takes the look cards, its own cards holding two paragraphs and one.
const cardsLook = `<div data-floe-look="cards"><div class="card"><p>a</p><p>b</p></div><div class="card"><p>c</p></div></div>`

// measureLookCards is cardsGapIn of the cards of an element of no part that takes the look cards: the look's own,
// which a theme alone draws otherwise; a part's are cards-gap-var's.
var measureLookCards = cardsGapIn(`[data-floe-look~="cards"]:not([data-floe-part]) > .card`)

const lookCardsGapFix = "write no gap on .card in a theme: set --cards-gap, which every card's gap reads"

// lookCardsGap draws cardsLook in every theme, as measureLookCards measures: a theme writes no gap on a card, and so
// the look's cards keep --cards-gap.
func lookCardsGap(t *testing.T, s *site) measured {
	t.Helper()

	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	page := s.write(t, "cards.yaml", htmlPage(cardsLook))
	var m measured
	for _, theme := range themes {
		s.floe(t, "publish", page, "--slug", "cards-"+theme, "--theme", theme)
		s.b.navigate("/a/cards-" + theme + "/?raw")
		m.add(s.b.measure("cardsLook in "+theme, measureLookCards))
	}
	return m
}

// brokenCardsGap draws cardsLook in a theme of one's own whose cards.css writes a gap on .card.
func brokenCardsGap(t *testing.T, s *site) (string, measured) {
	t.Helper()

	s.brokenPage(t, map[string]string{"themes/gappy/cards.css": ":scope .card { gap: 3px; }\n"}, htmlPage(cardsLook)+"theme: gappy\n")
	return "look cards", s.b.measure("broken", measureLookCards)
}
