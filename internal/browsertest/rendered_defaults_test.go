package browsertest_test

import "testing"

// TestRenderedDefaults draws a page in three themes of one's own, read over floe's: one that sets nothing keeps
// floe's defaults, and one that sets them in palette.css or on base.css's :scope has its own, as floe.defaults lies
// under every palette. On paper floe's base keeps no room for the copy button, whatever the palette sets.
func TestRenderedDefaults(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	const set = "--floe-text-small: 0.7rem; --floe-row-pad: 0.5rem; --floe-copy-room: 5px;"
	s.own(t, map[string]string{
		"themes/bare/base.css":         "/* no rules */\n",
		"themes/inpalette/palette.css": ":scope { " + set + " }\n",
		"themes/inbase/base.css":       ":scope { " + set + " }\n",
	})
	const read = `(() => { const r = getComputedStyle(document.documentElement);
		return ['--floe-text-small', '--floe-row-pad', '--floe-copy-room'].map(n => r.getPropertyValue(n).trim()).join(' | ') })()`
	for _, tt := range []struct{ theme, want string }{
		{"bare", "0.8rem | 0.4rem | calc(30px + 2 * 0.4rem)"},
		{"inpalette", "0.7rem | 0.5rem | 5px"},
		{"inbase", "0.7rem | 0.5rem | 5px"},
	} {
		s.floe(t, "publish", s.write(t, tt.theme+".yaml", themePage(tt.theme, "- text: x\n")), "--slug", tt.theme)
		s.b.navigate("/a/" + tt.theme + "/?raw")
		check(t, tt.theme, []string{s.b.eval(read)}, tt.want)
	}
	s.b.media("print")
	defer s.b.media("")
	s.b.navigate("/a/inpalette/?raw")
	check(t, "inpalette on paper", []string{s.b.eval(read)}, "0.7rem | 0.5rem | 0px")
	// LIMIT (defaults.css): base.css's :scope is a layer over floe's print, so its room holds on paper too.
	s.b.navigate("/a/inbase/?raw")
	check(t, "inbase on paper", []string{s.b.eval(read)}, "0.7rem | 0.5rem | 5px")
}
