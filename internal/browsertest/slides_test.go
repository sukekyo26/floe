package browsertest_test

import (
	"testing"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

const acrossPage = "lang: en\ntemplate: slides\ntitle: Across\ndirection: across\nslides:\n" +
	"  - {heading: a, parts: [{text: t}]}\n  - {heading: b}\n  - {heading: c, parts: [{text: u}]}\n"

// TestUISlidesAcross lays slides across: the cover and each slide as wide as the window, none spilling down, and
// → and ← move a slide at a time with nothing clicked first.
func TestUISlidesAcross(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	s.floe(t, "publish", s.write(t, "across.yaml", acrossPage), "--slug", "across")
	b := s.b
	b.navigate("/a/across/?raw")
	b.until("the slides", "document.querySelectorAll('.slide').length === 4")
	if got := b.eval("[...document.querySelectorAll('.slide')].every((e) => e.getBoundingClientRect().width === innerWidth)"); got != "true" {
		t.Errorf("a slide is not as wide as the window: %s", b.eval("JSON.stringify([innerWidth, [...document.querySelectorAll('.slide')].map((e) => e.getBoundingClientRect().width)])"))
	}
	if got := b.eval("document.documentElement.clientHeight === innerHeight"); got != "true" {
		t.Errorf("a scrollbar takes the page's height: clientHeight %s, innerHeight %s", b.eval("document.documentElement.clientHeight"), b.eval("innerHeight"))
	}
	// The tab takes the focus a window in front has, without which this Chrome's keys scroll nothing; nothing is clicked,
	// as a click would make the slide clicked the scroller the keys move. The scroll a key starts and its snap move for a
	// while, which still and settled do not wait for, and a key pressed meanwhile is lost in it: each key waits for its slide
	if _, err := chromedp.Call(b.ctx, emulation.SetFocusEmulationEnabled, emulation.SetFocusEmulationEnabledParams{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	b.pressKey(kb.ArrowRight)
	b.until("one slide across", "scrollX === innerWidth")
	b.pressKey(kb.ArrowRight)
	b.until("two slides across", "scrollX === 2 * innerWidth")
	b.pressKey(kb.ArrowLeft)
	b.until("one slide back", "scrollX === innerWidth")
}
