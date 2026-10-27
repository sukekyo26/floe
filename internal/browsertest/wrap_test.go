package browsertest_test

import (
	"strings"
	"testing"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

// wrapPage holds a path and a URL in cards, which have no place to break a line, words beside narrow
// columns (keys' text beside its keycaps, the language of code and the counts of a diff beside a long file
// name) and a path in a table and in a tree's note, which scroll instead.
const wrapPage = `template: report
lang: en
title: Wrap
sections:
  - heading: Wrap
    parts:
      - cards:
          - { title: path, text: 'See ` + "`internal/kit/floe/templates/report/page.html:216`" + ` here.' }
          - { title: url, text: 'See https://github.com/sukekyo26/floe/pull/117/files here.' }
      - keys:
          - { keys: [Control, Shift, Escape], text: open the task manager }
      - code: { file: internal/kit/floe/parts/findings/part_internal_test.go, lang: typescript, text: let a = 1 }
      - diff: { file: internal/kit/floe/parts/findings/part_internal_test.go, text: "+1\n+2\n+3\n+4\n+5\n+6\n+7\n+8\n+9\n+10\n+11\n+12\n-x\n" }
      - table:
          columns: [{ title: file }, { title: what }]
          rows:
            - ['` + "`internal/kit/floe/parts/findings/part_internal_test.go`" + `', a test of the findings]
      - tree:
          - { path: internal/kit/floe/parts/, note: internal/kit/floe/parts/findings/part_internal_test.go }
`

// measureWrap returns one line for each box a text overflows, out of those that scroll sideways, and for each
// word that breaks across lines in keys' text, code's language, a diff's counts, a table and a tree's note.
const measureWrap = `(() => {
  const out = [];
  // What is in a box that scrolls sideways may be wider than it.
  const scrolled = (e) => {
    for (let a = e.parentElement; a; a = a.parentElement) {
      if (['auto', 'scroll'].includes(getComputedStyle(a).overflowX)) return true;
    }
    return false;
  };
  for (const e of document.querySelectorAll('[data-floe-part] *')) {
    if (scrolled(e) || getComputedStyle(e).display === 'inline') continue;
    if (e.scrollWidth > e.clientWidth + 1) out.push('overflows: ' + e.textContent.trim().slice(0, 40));
  }
  const whole = '[data-floe-part=keys] dd, [data-floe-part=code] .lang, [data-floe-part=diff] figcaption > :not(.file), td, [data-floe-part=tree] .note';
  for (const e of document.querySelectorAll(whole)) {
    const walk = document.createTreeWalker(e, NodeFilter.SHOW_TEXT);
    for (let n; (n = walk.nextNode()); ) {
      for (const m of n.data.matchAll(/[\w+−]{2,}/g)) {
        const r = document.createRange();
        r.setStart(n, m.index);
        r.setEnd(n, m.index + m[0].length);
        if (new Set([...r.getClientRects()].map((c) => Math.round(c.top))).size > 1) out.push('breaks: ' + m[0]);
      }
    }
  }
  return out.join('\n');
})()`

// TestUIWrap draws a page as wide as a phone (390 px): a path or a URL breaks rather than overflow its card,
// and no other word breaks for it.
func TestUIWrap(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.write(t, "wrap.yaml", wrapPage), "--slug", "wrap")
	metrics := emulation.SetDeviceMetricsOverrideParams{Width: 390, Height: 844, DeviceScaleFactor: 1, Mobile: true}
	if _, err := chromedp.Call(b.ctx, emulation.SetDeviceMetricsOverride, metrics); err != nil {
		t.Fatal(err)
	}
	b.navigate("/a/wrap/?raw")
	if got := b.eval(measureWrap); got != "" {
		t.Errorf("at 390 px:\n  %s", strings.ReplaceAll(got, "\n", "\n  "))
	}
}
