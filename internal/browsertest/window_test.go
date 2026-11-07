package browsertest_test

import (
	"fmt"
	"testing"
)

// A tab has the same window whatever the version of Chrome, as it opens and when its size is given back.
func TestUIWindow(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	b := newSite(t).b
	want := fmt.Sprintf("%d %d", windowWidth, windowHeight)
	const window = "innerWidth + ' ' + innerHeight"
	b.navigate("/")
	if got := b.eval(window); got != want {
		t.Errorf("as it opens: %s, want %s", got, want)
	}
	b.size(1000, 700, false)
	b.unsize()
	if got := b.eval(window); got != want {
		t.Errorf("given back: %s, want %s", got, want)
	}
}
