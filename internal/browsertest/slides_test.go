package browsertest_test

import (
	"testing"

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
		t.Errorf("the page spills down: clientHeight %s, innerHeight %s", b.eval("document.documentElement.clientHeight"), b.eval("innerHeight"))
	}
	// A click on the page first: a tab of this Chrome sent the keys after navigating moved nothing until a click, though
	// a tab opened on the page moves at once. The scroll a key starts and its snap move for a while, which still and
	// settled do not wait for, and a key pressed meanwhile is lost in it: each key waits for its slide
	b.click(40, 40)
	b.pressKey(kb.ArrowRight)
	b.until("one slide across", "scrollX === innerWidth")
	b.pressKey(kb.ArrowRight)
	b.until("two slides across", "scrollX === 2 * innerWidth")
	b.pressKey(kb.ArrowLeft)
	b.until("one slide back", "scrollX === innerWidth")
}
