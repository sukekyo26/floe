package browsertest_test

import (
	"strings"
	"testing"
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

// measureTabs returns a line for each tabs whose titles are not drawn alike, read with the page drawn for print: on
// paper every tab shows under its title, so that the one picked on the screen is no more than the others.
const measureTabs = `(() => {
  const out = [];
  const look = (e) => { const s = getComputedStyle(e);
    return [s.backgroundColor, s.backgroundImage, s.color, s.borderTop, s.borderBottom, s.boxShadow, s.borderRadius].join(' / '); };
  for (const tabs of document.querySelectorAll('[data-floe-part="tabs"]')) {
    const titles = [...tabs.querySelectorAll(':scope > label')].map(look);
    if (new Set(titles).size > 1) out.push('tabs: ' + titles.join(' | '));
  }
  return out.join('\n');
})()`

// TestUIPrint draws every page of floePages for print, and finds every ground kept on paper: print leaves a ground
// out unless asked, as a browser's print does by default, and a light text on a fill would be lost on white paper.
// It finds tabs whose picked title is drawn apart from the others, as a theme's style of tabs wins over tabs' own
// print but for a fill it keeps to the screen.
// LIMIT: tabs is the only part whose state print draws otherwise; check a part's that comes to do so too.
func TestUIPrint(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	s.b.media("print")
	s.floePages(t, func(what string) {
		if lost := s.b.eval(measurePrint); lost != "" {
			t.Errorf("%s draws a ground print leaves out:\n  %s", what, strings.ReplaceAll(lost, "\n", "\n  "))
		}
		if apart := s.b.eval(measureTabs); apart != "" {
			t.Errorf("%s draws a tab's title apart on paper:\n  %s", what, strings.ReplaceAll(apart, "\n", "\n  "))
		}
	})
}
