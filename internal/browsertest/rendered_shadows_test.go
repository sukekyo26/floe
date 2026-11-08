package browsertest_test

import "testing"

// TestRenderedShadows draws every-part in brutal and retro: a box in one that casts a shadow casts none (the colour is
// the box's alone; brutal's cells in its boxes), and retro's cards keep the stripes they draw inside themselves with
// --floe-inset.
func TestRenderedShadows(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	file := s.write(t, "every.yaml", s.everyPart(t)+shadowsExtra)
	for _, theme := range []string{"brutal", "retro"} {
		s.floe(t, "publish", file, "--slug", "shadows-"+theme, "--theme", theme)
	}
	s.b.navigate("/a/shadows-brutal/?raw")
	check(t, "held", []string{s.b.eval(measureCellShadows)}, "rgba(0, 0, 0, 0) in rgb(10, 10, 10)")
	s.b.navigate("/a/shadows-retro/?raw")
	got := s.b.eval(`(() => { const s = getComputedStyle(document.querySelector('.card')).boxShadow; return (s.match(/inset/g) || []).length; })()`)
	check(t, "retro's stripes", []string{got}, "3")
}
