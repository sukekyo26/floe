package server_test

import (
	"strings"
	"testing"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/store"
)

// wrapPage holds a path and a URL in cards, which have no place to break a line, and words beside narrow
// columns: keys' text beside its keycaps, a code's language beside a long file name.
const wrapPage = `template: report
lang: en
title: Wrap
sections:
  - heading: Wrap
    parts:
      - cards:
          - { title: path, text: 'See ` + "`internal/tmpl/templates/report/page.html:216`" + ` here.' }
          - { title: url, text: 'See https://github.com/sukekyo26/floe/pull/117/files here.' }
      - keys:
          - { keys: [Control, Shift, Escape], text: open the task manager }
      - code: { file: internal/tmpl/parts/findings/part_internal_test.go, lang: typescript, text: let a = 1 }
`

// measureWrap returns one line for each box a text overflows and for each word of keys' text or of
// code's language that breaks across lines.
const measureWrap = `(() => {
  const out = [];
  for (const e of document.querySelectorAll('[data-floe-part] *')) {
    if (e.closest('pre') || getComputedStyle(e).display === 'inline') continue;
    if (e.scrollWidth > e.clientWidth + 1) out.push('overflows: ' + e.textContent.trim().slice(0, 40));
  }
  for (const e of document.querySelectorAll('[data-floe-part=keys] dd, [data-floe-part=code] .lang')) {
    const n = e.firstChild;
    for (const m of n.data.matchAll(/\w{2,}/g)) {
      const r = document.createRange();
      r.setStart(n, m.index);
      r.setEnd(n, m.index + m[0].length);
      if (new Set([...r.getClientRects()].map((c) => Math.round(c.top))).size > 1) out.push('breaks: ' + m[0]);
    }
  }
  return out.join('\n');
})()`

// TestUIWrap draws a page as wide as a phone (390 px): a path or a URL breaks rather than overflow its card,
// and no other word breaks for it.
func TestUIWrap(t *testing.T) { //nolint:paralleltest // the UI tests share one Chrome, one at a time
	e := newEnv(t)
	b := newBrowser(t, e.base, i18n.English)
	slug := "wrap"
	e.publish(t, e.write(t, "wrap.yaml", wrapPage), store.Options{Slug: &slug})
	metrics := emulation.SetDeviceMetricsOverrideParams{Width: 390, Height: 844, DeviceScaleFactor: 1, Mobile: true}
	if _, err := chromedp.Call(b.ctx, emulation.SetDeviceMetricsOverride, metrics); err != nil {
		t.Fatal(err)
	}
	b.navigate("/a/" + slug + "/?raw")
	b.until("the parts", "!!document.querySelector('[data-floe-part=code]')")
	if got := b.eval(measureWrap); got != "" {
		t.Errorf("at 390 px:\n  %s", strings.ReplaceAll(got, "\n", "\n  "))
	}
}
