package browsertest_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/chromedp"
)

// TestUIBarNarrow draws the bar at a phone's widths: at 24rem (384px) or less the logo is its mark alone on every screen, which is
// what lets the links of the two screens, the longest in Japanese, and the search fit beside it at 280px; its word stays the link's name.
func TestUIBarNarrow(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	s.floe(t, "publish", s.page(t, "a.yaml", "T a", ""), "--slug", "a")
	s.floe(t, "task", "--slug", "a", "--task", "#1", "--title", "One")
	s.floe(t, "task", "--project", "home", "--task", "#1", "--state", "doing")
	b := newBrowser(t, s.Base, "ja")
	defer b.unsize()

	screens := []string{"/", "/tasks?project=home", "/tasks?project=home&task=%231"}
	const bar = `(() => { const logo = document.querySelector('[data-testid=brand]'), inner = document.querySelector('[data-testid=bar-inner]');
		return [logo.textContent, getComputedStyle(logo).fontSize === '0px' ? 'mark' : 'word', logo.querySelector('svg').getBoundingClientRect().width,
			document.documentElement.scrollWidth <= innerWidth, inner.scrollWidth <= inner.clientWidth].join(' '); })()`
	out, want := make([]string, 0, 16), make([]string, 0, 16)
	for _, width := range []int64{280, 320, 384, 385} {
		for _, path := range screens {
			b.size(width, 800, false)
			b.navigate(path)
			b.until("the bar", fmt.Sprintf("innerWidth === %d && document.querySelector('[data-testid=brand]') && document.querySelector('#list li')", width))
			b.still()
			logo := "mark"
			if width > 384 {
				logo = "word"
			}
			out = append(out, fmt.Sprintf("%d: %s", width, b.eval(bar)))
			want = append(want, fmt.Sprintf("%d: floe %s 26 true true", width, logo))
		}
	}
	// The name that a screen reader reads, at the width of the mark
	b.size(280, 800, false)
	b.navigate("/")
	b.until("the bar", "innerWidth === 280 && document.querySelector('[data-testid=brand]') && document.querySelector('#list li')")
	out = append(out, brandName(t, b))
	want = append(want, "floe")
	check(t, "the bar", out, strings.Join(want, "|"))
}

// brandName is the name of the bar's logo as the browser computes it for a screen reader.
func brandName(t *testing.T, b *browser) string {
	t.Helper()

	tree, err := chromedp.Call(b.ctx, accessibility.GetFullAXTree, accessibility.GetFullAXTreeParams{})
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range tree.Nodes {
		if n.Ignored || n.Role == nil || n.Name == nil {
			continue
		}
		var role, name string
		if json.Unmarshal(n.Role.Value, &role) == nil && role == "link" && json.Unmarshal(n.Name.Value, &name) == nil {
			return name
		}
	}
	return ""
}
