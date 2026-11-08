package browsertest_test

import "testing"

// TestRenderedNesting draws nestingPage in brutal and default: a stack or a note gives the boxes it holds no margins of
// its own but the note's gap between them, --floe-room next to a part's box, in brutal too, whose shadows take no room;
// the blocks a browser gives margins of its own (h4, hr) have none in a part's Markdown, spaced by its text's box, nor
// a <figure> box but the note's; and entries keep --floe-room around their parts in the text's line height.
func TestRenderedNesting(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	s.own(t, figPart)
	file := s.write(t, "nesting.yaml", nestingPage)
	for _, theme := range []string{"brutal", "default"} {
		s.floe(t, "publish", file, "--slug", "nesting-"+theme, "--theme", theme)
	}
	s.b.navigate("/a/nesting-brutal/?raw")
	check(t, "held", []string{s.b.eval(measureHeld)}, "12px 0px 0px | 0px | 0px")
	s.b.navigate("/a/nesting-default/?raw")
	check(t, "blocks", []string{s.b.eval(measureBlocks)}, "0px | 0px | 0px | 0px | 12px 0px 0px")
	check(t, "entries", []string{s.b.eval(measureEntries)}, "12 12 12 | 26.4px 26.4px")
}
