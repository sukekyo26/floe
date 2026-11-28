package browsertest_test

import "testing"

// TestRenderedLayers draws the template bare of layersKit, which draws no ground, colour or face of its own: floe's
// base draws the page's in washi's, and notebook's ground of a pattern, its margin line and its rules, is drawn whole
// (its layers, their size, the colour under them).
func TestRenderedLayers(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	s.own(t, layersKit)
	file := s.write(t, "bare.yaml", "template: bare\nlang: en\n")
	for _, theme := range []string{"washi", "notebook"} {
		s.floe(t, "publish", file, "--slug", "bare-"+theme, "--theme", theme)
	}
	s.b.navigate("/a/bare-washi/?raw")
	check(t, "bare", []string{s.b.eval(`(() => { const r = getComputedStyle(document.documentElement), s = getComputedStyle(document.querySelector('p'));
		return [r.backgroundColor, getComputedStyle(document.body).backgroundColor, s.color, s.fontFamily.split(',')[0]].join(' | ') })()`)},
		`rgb(243, 239, 230) | rgba(0, 0, 0, 0) | rgb(31, 28, 25) | "Hiragino Mincho ProN"`)
	s.b.navigate("/a/bare-notebook/?raw")
	check(t, "rules", []string{s.b.eval(`(() => { const r = getComputedStyle(document.documentElement);
		return [r.backgroundImage.split('linear-gradient').length - 1, r.backgroundSize, r.backgroundColor].join(' | ') })()`)},
		"2 | auto, auto, auto | rgb(249, 246, 237)")
}
