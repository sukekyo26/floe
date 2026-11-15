package browsertest_test

import "testing"

// TestRenderedFragment draws every diagram (fragments) in floe's page: part.css leaves no shape unpainted, and raw SVG
// is in the palette's colours it reads (diagram.md's rule of raw's colours). A page's colours written on the diagrams'
// roots win over the fragment's, before the fragment or after it.
func TestRenderedFragment(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	fragment, _ := fragments(t, s)
	if got := s.b.eval(unpainted); got != "" {
		t.Errorf("shapes part.css leaves unpainted in floe's page:\n%s", got)
	}
	if got := s.b.eval(rawOff); got != "" {
		t.Errorf("raw SVG not in the palette's colours it reads, in floe's page:\n%s", got)
	}
	const accent = "rgb(1, 2, 3)"
	rule := "<style>.floe-zoom-tools, .floe-zoom { --accent: " + accent + "; }</style>"
	stroke := "getComputedStyle(document.querySelector('.dg .box[data-floe-tone=accent] rect')).stroke"
	for _, page := range []struct{ name, head, body string }{
		{"before.html", rule, fragment},
		{"after.html", "", fragment + rule},
	} {
		openOwn(t, s, page.name, page.head, page.body)
		if got := s.b.eval(stroke); got != accent {
			t.Errorf("%s: the accent box's stroke %s, want the page's %s", page.name, got, accent)
		}
	}
}
