package browsertest_test

import "testing"

// barsUnitPage holds bars whose units are a short one in Japanese, a long one in Japanese with no space in it, and one
// of many words, each beside a bar of a whole.
const barsUnitPage = `template: report
lang: ja
title: Bars
sections:
  - heading: Bars
    parts:
      - bars:
          - { label: 取り込み, value: 1200, of: 1840, unit: ページ }
          - { label: 速さ, value: 30, of: 100, unit: 新しいホストへのページ毎時 }
          - { label: Speed, value: 30, of: 100, unit: pages per hour for each team on the new host }
`

// TestRenderedBarsUnit lays the units out on a phone: a short unit in Japanese stays whole (without word-break: keep-all
// it breaks within the word), and a long one or one of many words wraps in the value's column, which keeps within 40% of
// the part and leaves the bar a length (white-space: nowrap, or a long unit nothing breaks, widens the column and
// squeezes the bar).
// LIMIT: Chrome breaks a long unit under keep-all by itself, so this cannot tell overflow-wrap: anywhere, which keeps the
// column's least width small in an engine that breaks it not; measure that engine if one comes to be run.
func TestRenderedBarsUnit(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.write(t, "bars.yaml", barsUnitPage), "--slug", "bars")
	b.size(390, 900, true)
	b.navigate("/a/bars/?raw")

	got := b.eval(`(() => {
		const part = document.querySelector('[data-floe-part=bars]');
		const lines = [...part.querySelectorAll('.unit')].map((u) => u.getClientRects().length);
		const column = Math.max(...[...part.querySelectorAll('.value')].map((v) => v.getBoundingClientRect().width)) / part.getBoundingClientRect().width;
		const bar = Math.min(...[...part.querySelectorAll('.track')].map((v) => v.getBoundingClientRect().width));
		return [lines[0], lines.slice(1).every((n) => n > 1), column <= 0.4, bar >= 60].join(' ');
	})()`)
	if want := "1 true true true"; got != want {
		t.Errorf("the short unit's lines, whether the long ones wrap, whether the value's column is within 40%% and the bar at least 60px: %q, want %q", got, want)
	}
}
