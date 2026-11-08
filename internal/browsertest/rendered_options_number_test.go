package browsertest_test

import (
	"fmt"
	"testing"

	"github.com/sukekyo26/floe/internal/kit"
)

// TestRenderedOptionsNumber draws optionsPage in every theme at each of optionsWidths: a theme's number on a card, in
// the rows or laid out of them (notebook's tape, washi's seal), stands as far below the card's top on a card holding
// parts as on a card of options without parts (across, it stands by the card's width).
func TestRenderedOptionsNumber(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	optionsFiles(t, s)
	page := s.write(t, "options.yaml", optionsPage)
	for _, theme := range themes {
		s.floe(t, "publish", page, "--slug", "options-"+theme, "--theme", theme)
		optionsAt(t, s, "options-"+theme, func(width string) {
			var at [2]string
			for i, sel := range []string{"[data-floe-part=options]:has(.parts) > .card", "[data-floe-part=options]:not(:has(.parts)) > .card"} {
				mark, _ := s.b.pseudoBoxes(sel)
				card := s.b.rect("document.querySelector(" + mustJSON(sel) + ")")
				if mark != nil {
					at[i] = fmt.Sprintf("%.1f", mark[1]-card.Y)
				}
			}
			if at[0] != at[1] {
				t.Errorf("%s at %s: the number at %s on a card with parts, at %s without", theme, width, at[0], at[1])
			}
		})
	}
}
