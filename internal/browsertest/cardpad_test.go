package browsertest_test

import (
	"os"
	"strings"
	"testing"
)

// cardPadPage draws every part that takes the look cards.
const cardPadPage = `template: report
lang: en
title: Cards
sections:
  - heading: Cards
    parts:
      - options:
          - { title: One, text: first }
          - { title: Two, text: second }
      - stats:
          - { label: Pages, value: '1' }
      - facts:
          - { name: Owner, value: A team }
      - links:
          - { title: Docs, url: "https://example.com/docs" }
      - decision:
          title: A decision
          text: Made.
`

// measureCardPad returns, a line to a part that takes cards, the padding of its first card.
const measureCardPad = `['options', 'stats', 'facts', 'links', 'decision'].map((p) => {
  const s = getComputedStyle(document.querySelector('[data-floe-part=' + p + '] .card'));
  return p + ' ' + [s.paddingTop, s.paddingRight, s.paddingBottom, s.paddingLeft].join(' ');
}).join('\n')`

// TestUICardPad measures the padding of a card of every part that takes cards in every theme, which is the part's
// alone: a theme draws its decorations over it and writes none, so every theme pads them as default, which writes
// none.
func TestUICardPad(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	themes, err := os.ReadDir("../kit/floe/themes")
	if err != nil {
		t.Fatal(err)
	}
	file := s.write(t, "cards.yaml", cardPadPage)
	pads := map[string]string{}
	for _, theme := range themes {
		name := theme.Name()
		s.floe(t, "publish", file, "--slug", "cards-"+name, "--theme", name)
		b.navigate("/a/cards-" + name + "/?raw")
		pads[name] = b.eval(measureCardPad)
	}
	var amiss []string
	for name, got := range pads {
		if got != pads["default"] {
			amiss = append(amiss, name+":\n"+got)
		}
	}
	if len(amiss) > 0 {
		t.Errorf("themes pad cards otherwise than default:\n%s\ndefault:\n%s", strings.Join(amiss, "\n"), pads["default"])
	}
}
