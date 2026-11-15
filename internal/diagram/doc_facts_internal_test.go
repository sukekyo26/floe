package diagram

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"floe/internal/i18n"
)

// The doc of diagrams quotes the limits, sizes and mistakes of diagrams as floe makes them: each text below, made
// from floe's constants, defaults and catalog, stands in the doc as many times as said.
func TestDiagramDocFacts(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("../../docs/agents/diagrams.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(src)
	en := i18n.EnglishCatalog()
	for _, q := range []struct {
		text  string
		times int
	}{
		{"more than " + strconv.Itoa(maxNodes) + " boxes, icons, groups and lanes", 1},
		{"more than " + strconv.Itoa(maxPassed) + " ranks", 1},
		{"and " + strconv.Itoa(canvasPad) + " more", 2},
		{"| `r` | a size | " + num(dotR(&dot{})) + " |", 1},
		{"| `size` | a size | " + num(iconSize(&icon{})) + " |", 1},
		{strconv.Itoa(iconLabelY-labelTop) + " under it and " + strconv.Itoa(labelH) + " tall", 1},
		{num(labelWidth(Text{}, false)/2) + " wider each side than its text", 1},
		{en.Msg("diagram_too_narrow", `box "a"`, "A long title here", "", "<n>"), 1},
		{en.Msg("diagram_outside", `label "HTTPS-LONG-LABEL"`, "<w>", "<h>"), 1},
		{en.Msg("diagram_runs_through", `edge "a"→"c"`, `box "b"`), 1},
		{en.Msg("diagram_overlap", `box "a"`, `the heading of frame "F"`), 1},
		{en.Msg("diagram_overlap", `the icon of frame "G"`, `box "a"`), 1},
		{en.Msg("diagram_straddle", `box "a"`, `frame "F"`, "x", "<n>", "<n>", "<n>"), 1},
	} {
		if got := strings.Count(doc, q.text); got != q.times {
			t.Errorf("docs/agents/diagrams.md holds %q %d times, floe makes it %d: write it as floe makes it", q.text, got, q.times)
		}
	}
}
