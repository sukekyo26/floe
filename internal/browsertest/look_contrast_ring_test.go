package browsertest_test

import (
	"testing"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

// focused gives s's tab the focus, so that a focus there is :focus-visible, as measureRingsAt needs.
func focused(t *testing.T, s *site) {
	t.Helper()

	if _, err := chromedp.Call(s.b.ctx, emulation.SetFocusEmulationEnabled, emulation.SetFocusEmulationEnabledParams{Enabled: true}); err != nil {
		t.Fatal(err)
	}
}

const lookContrastRingFix = "draw a focus ring 3:1 off the ground under it, unclipped by the boxes around it, on everything a key reaches"

// lookContrastRing measures every page of floePages and of bandPages as measureRingsAt does: a focus there can be
// seen, in a band too.
func lookContrastRing(t *testing.T, s *site) measured {
	t.Helper()

	focused(t, s)
	var m measured
	visit := func(what string) { m.add(s.b.measure(what, measureRingsAt)) }
	s.floePages(t, visit)
	bandPages(t, s, visit)
	return m
}

// brokenContrastRing draws links in a theme of one's own whose links.css draws the ring near white on white.
func brokenContrastRing(t *testing.T, s *site) (string, measured) {
	t.Helper()

	focused(t, s)
	s.brokenPage(t, map[string]string{"themes/faint/links.css": ":scope a { outline-color: #fefefe !important; }\n"},
		themePage("faint", "- links: [{ title: a link, url: 'https://example.com/' }]\n"))
	return "links", s.b.measure("broken", measureRingsAt)
}
