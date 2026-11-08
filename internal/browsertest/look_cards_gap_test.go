package browsertest_test

import (
	"testing"

	"github.com/sukekyo26/floe/internal/kit"
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

const lookCardsGapFix = "write no gap on .card in a theme: set --cards-gap, which every card's gap reads"

// lookCardsGap draws every part (everyPart) and optionsPage in every theme, as measureCardsGap measures: a theme writes
// no gap on a card, and so every card's gap is --cards-gap.
func lookCardsGap(t *testing.T, s *site) measured {
	t.Helper()

	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	optionsFiles(t, s)
	pages := map[string]string{"every-part": s.write(t, "every-part.yaml", s.everyPart(t)), "options": s.write(t, "options.yaml", optionsPage)}
	var m measured
	for _, theme := range themes {
		for name, file := range pages {
			s.floe(t, "publish", file, "--slug", name+"-"+theme, "--theme", theme)
			s.b.navigate("/a/" + name + "-" + theme + "/?raw")
			m.add(s.b.measure(name+" in "+theme, measureCardsGap))
		}
	}
	return m
}

// brokenCardsGap draws stats in a theme of one's own whose cards.css writes a gap on .card.
func brokenCardsGap(t *testing.T, s *site) (string, measured) {
	t.Helper()

	s.brokenPage(t, map[string]string{"themes/gappy/cards.css": ":scope .card { gap: 3px; }\n"}, themePage("gappy", "- stats: [{ label: Pages, value: '1' }]\n"))
	return "stats", s.b.measure("broken", measureCardsGap)
}
