package browsertest_test

import "testing"

// TestRenderedLayers draws the template bare of layersKit, which draws no ground, colour or face of its own: floe's
// base draws the page's in washi's, and blueprint's ground of a pattern, its squares, is drawn whole (its layers, their
// size, the colour under them).
func TestRenderedLayers(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	s.own(t, layersKit)
	file := s.write(t, "bare.yaml", "template: bare\nlang: en\n")
	for _, theme := range []string{"washi", "blueprint"} {
		s.floe(t, "publish", file, "--slug", "bare-"+theme, "--theme", theme)
	}
	s.b.navigate("/a/bare-washi/?raw")
	check(t, "bare", []string{s.b.eval(`(() => { const r = getComputedStyle(document.documentElement), s = getComputedStyle(document.querySelector('p'));
		return [r.backgroundColor, getComputedStyle(document.body).backgroundColor, s.color, s.fontFamily.split(',')[0]].join(' | ') })()`)},
		`rgb(237, 228, 207) | rgba(0, 0, 0, 0) | rgb(29, 27, 24) | "Hiragino Mincho ProN"`)
	s.b.navigate("/a/bare-blueprint/?raw")
	check(t, "squares", []string{s.b.eval(`(() => { const r = getComputedStyle(document.documentElement);
		return [r.backgroundImage.split('linear-gradient').length - 1, r.backgroundSize, r.backgroundColor].join(' | ') })()`)},
		"2 | 16px 16px, 16px 16px, auto | rgb(238, 244, 252)")
}
