package browsertest_test

import "testing"

// barsUnitPage holds a bar whose unit is in Japanese and one whose unit is of many words, each beside a bar of a whole.
const barsUnitPage = `template: report
lang: ja
title: Bars
sections:
  - heading: Bars
    parts:
      - bars:
          - { label: 取り込み, value: 1200, of: 1840, unit: ページ }
          - { label: 速さ, value: 30, of: 100, unit: pages per hour for each team on the new host }
`

// TestRenderedBarsUnit lays the units out on a phone: one in Japanese stays whole, as its text has no space to break at
// (without word-break: keep-all it breaks within the word), and one of many words wraps at its spaces rather than
// widening the column and the bar with it (white-space: nowrap keeps it whole).
func TestRenderedBarsUnit(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.write(t, "bars.yaml", barsUnitPage), "--slug", "bars")
	b.size(390, 900, true)
	b.navigate("/a/bars/?raw")

	lines := b.eval(`(() => { const [a, w] = [...document.querySelectorAll('.unit')].map((u) => u.getClientRects().length); return a + ' ' + (w > 1); })()`)
	if want := "1 true"; lines != want {
		t.Errorf("the lines of the Japanese unit and whether the long one wraps: %q, want %q", lines, want)
	}
}
