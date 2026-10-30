package browsertest_test

import (
	"strings"
	"testing"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

// measurePrint returns a line for each element with a ground (a colour or an image) that print would leave out: one
// whose print-color-adjust is not exact, read with the page drawn for print.
const measurePrint = `(() => {
  const out = new Set();
  for (const e of [document.documentElement, ...document.querySelectorAll('*')]) {
    const s = getComputedStyle(e);
    if (s.backgroundColor === 'rgba(0, 0, 0, 0)' && s.backgroundImage === 'none') continue;
    if (s.printColorAdjust !== 'exact') out.add(e.tagName.toLowerCase() + [...e.classList].map((c) => '.' + c).join('') + ': ' + s.printColorAdjust);
  }
  return [...out].join('\n');
})()`

// TestUIPrint draws every page of floePages for print, and finds every ground kept on paper: print leaves a ground
// out unless asked, as a browser's print does by default, and a light text on a fill would be lost on white paper.
func TestUIPrint(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	if _, err := chromedp.Call(s.b.ctx, emulation.SetEmulatedMedia, emulation.SetEmulatedMediaParams{Media: "print"}); err != nil {
		t.Fatal(err)
	}
	s.floePages(t, func(what string) {
		if lost := s.b.eval(measurePrint); lost != "" {
			t.Errorf("%s draws a ground print leaves out:\n  %s", what, strings.ReplaceAll(lost, "\n", "\n  "))
		}
	})
}
