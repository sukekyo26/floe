package browsertest_test

import (
	"fmt"
	"testing"
)

// A tab has the same window whatever the version of Chrome, as it opens and when its size is given back, a phone's too.
// LIMIT: on a Chrome whose own window is of this size, CI's for now, this passes without newBrowser's unsize: another version is what tells
func TestUIWindow(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	b := newSite(t).b
	want := fmt.Sprintf("%d %d", windowWidth, windowHeight)
	const (
		window = "innerWidth + ' ' + innerHeight"
		// A phone's scrollbars take no room of the box they scroll.
		scrollbar = "(() => { const d = document.createElement('div'); d.style.cssText = 'overflow: scroll; width: 100px; height: 100px'; document.body.append(d); const w = d.offsetWidth - d.clientWidth; d.remove(); return w; })()"
	)
	b.navigate("/")
	if got := b.eval(window); got != want {
		t.Errorf("as it opens: %s, want %s", got, want)
	}
	opened := b.eval(scrollbar)
	b.size(390, 844, true)
	if got := b.eval(scrollbar); got == opened {
		t.Errorf("a phone's scrollbars take %s, as the window's do: this measures nothing", got)
	}
	b.unsize()
	if got := b.eval(window); got != want {
		t.Errorf("given back: %s, want %s", got, want)
	}
	if got := b.eval(scrollbar); got != opened {
		t.Errorf("given back: scrollbars of %s, want %s", got, opened)
	}
}
