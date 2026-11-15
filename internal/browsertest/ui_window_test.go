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
	const window = "innerWidth + ' ' + innerHeight"
	b.navigate("/")
	if got := b.eval(window); got != want {
		t.Errorf("as it opens: %s, want %s", got, want)
	}
	// A phone's window alone takes the width a page asks for in its viewport: given back, that is to end.
	b.eval("document.querySelector('meta[name=viewport]').content = 'width=600'")
	b.size(390, 844, true)
	b.until("a phone's window", "innerWidth === 600")
	b.unsize()
	b.still()
	if got := b.eval(window); got != want {
		t.Errorf("given back: %s, want %s", got, want)
	}
}
