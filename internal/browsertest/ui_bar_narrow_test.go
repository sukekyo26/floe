package browsertest_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/chromedp"

	"floe/internal/i18n"
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

// TestUIFrameBarRoom draws a page whole's bar with all it may hold, in both languages, across 56rem, where the switch of the two lists
// gives way to the back link's mark: of two versions and archived, opened from a list and from a task's page. Nothing runs past an edge,
// and the title, which alone gives way, keeps more than the room of its padding.
func TestUIFrameBarRoom(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	title := strings.TrimSpace(strings.Repeat("A title of many words ", 5))
	for _, body := range []string{"one", "two"} {
		s.floe(t, "publish", s.page(t, "a.yaml", title, "<p>"+body+"</p>"), "--slug", "a")
	}
	s.floe(t, "archive", "a")
	for _, lang := range []i18n.Lang{i18n.English, i18n.Lang("ja")} {
		b := newBrowser(t, s.Base, lang)
		defer b.unsize()
		for _, back := range []string{"/?view=fav", "/tasks?project=home&task=%2312345"} {
			b.navigate("/")
			b.eval(`sessionStorage.setItem('pages.order', '{"order":["a"],"shown":["a"],"back":"` + back + `"}')`)
			for _, width := range []int64{641, 768, 896, 897} {
				b.size(width, 800, false)
				b.navigate("/a/a/")
				b.until("the bar", fmt.Sprintf("innerWidth === %d && document.getElementById('frame-title').textContent && !document.getElementById('frame-version').hidden", width))
				got := b.settled(`(() => { const inner = document.querySelector('[data-testid=bar-inner]'), shown = (id) => document.getElementById(id).getClientRects().length > 0;
					return [...['screens', 'back', 'frame-version', 'frame-archived', 'frame-actions'].map((id) => id + ' ' + shown(id)),
						'past ' + (inner.scrollWidth - inner.clientWidth), 'title ' + (document.getElementById('frame-title').getBoundingClientRect().width > 14)].join(', '); })()`)
				screens := width > 896
				whole := strings.HasPrefix(back, "/tasks")
				want := fmt.Sprintf("screens %t, back %t, frame-version true, frame-archived true, frame-actions true, past 0, title true", screens, !screens || whole)
				if got != want {
					t.Errorf("%s at %d, opened from %s: %s", lang, width, back, got)
				}
			}
		}
	}
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
