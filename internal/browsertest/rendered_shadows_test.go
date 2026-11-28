package browsertest_test

import "testing"

// TestRenderedShadows draws every-part in cyber and retro: a box in one that casts a shadow casts none (the colour is
// the box's alone; cyber's cells in its tables), and retro's cards keep the raised edge they draw inside themselves
// with --floe-inset.
func TestRenderedShadows(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	file := s.write(t, "every.yaml", s.everyPart(t)+shadowsExtra)
	for _, theme := range []string{"cyber", "retro"} {
		s.floe(t, "publish", file, "--slug", "shadows-"+theme, "--theme", theme)
	}
	s.b.navigate("/a/shadows-cyber/?raw")
	check(t, "held", []string{s.b.eval(measureCellShadows)}, "rgba(0, 0, 0, 0) in rgba(54, 255, 108, 0.4)")
	s.b.navigate("/a/shadows-retro/?raw")
	got := s.b.eval(`(() => { const s = getComputedStyle(document.querySelector('.card')).boxShadow; return (s.match(/inset/g) || []).length; })()`)
	check(t, "retro's raised edge", []string{got}, "3")
}
