package browsertest_test

import (
	"encoding/json"
	"fmt"
	"math"
	"testing"
)

// timelineDatesPage holds three timelines: dates of every length up to 14rem (the first, in English: a date alone, a
// time alone, with a time, seconds, a weekday, and an ISO 8601 with an offset), the dates of a Japanese page (the
// second, the longest with a weekday, 13.0rem), and dates over 14rem (the third: one with spaces, one with none, a time).
const timelineDatesPage = `template: report
lang: en
title: Dates
sections:
  - heading: Dates
    parts:
      - timeline:
          - { date: '2030-05-14', title: A date }
          - { date: '10:42', title: A time }
          - { date: '2030-05-14 10:42', title: A date and a time }
          - { date: '2030-05-14 10:42:07', title: With seconds }
          - { date: '2030-05-14 (Tue) 10:42', title: With a weekday }
          - { date: '2030-05-14T10:42:07+09:00', title: ISO 8601 with an offset, text: A text under the title. }
      - timeline:
          - { date: '11 月 2 日 10:12', title: A date and a time }
          - { date: '2030 年 5 月 14 日（火）10:42', title: With a weekday }
      - timeline:
          - { date: '2030-05-14 10:42 (UTC+9, Tuesday, in the night of the maintenance)', title: Over 14rem with spaces }
          - { date: 'abcdefghijklmnopqrstuvwxyz0123456789', title: Over 14rem with none }
          - { date: '10:44', title: A time }
`

// measureTimelineDates returns a line for what stands amiss, given the form the page should be in: "wide" (the date
// at the left of the dot) or "narrow" (over the title), or "" where either will do (paper). A date up to 14rem is in
// one line; where wide, none is wider than 14rem (narrow, its box is the text's column), the dates of a timeline end at
// one edge, its titles start at one, and a date over 14rem is folded; where narrow, the dates and the titles start at
// one edge and the date is over its title. The dots are measured by dotsIn.
const measureTimelineDates = `((want) => {
  const out = [], rem = parseFloat(getComputedStyle(document.documentElement).fontSize);
  const lines = (el) => {
    const r = document.createRange();
    r.selectNodeContents(el);
    return new Set([...r.getClientRects()].map((q) => Math.round(q.top))).size;
  };
  const lists = document.querySelectorAll('[data-floe-part=timeline]');
  if (lists.length !== 3) return 'timelines: ' + lists.length;
  lists.forEach((ol, i) => {
    const cs = getComputedStyle(ol);
    const wide = ol.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight) > 36 * rem;
    const form = wide ? 'wide' : 'narrow', name = 'timeline ' + i + ' (' + form + ')';
    if (want && form !== want) out.push(name + ' is not ' + want);
    const edges = {};
    const same = (key, label, x) => {
      if (edges[key] === undefined) edges[key] = x;
      else if (Math.abs(x - edges[key]) > 1) out.push(label + ': its ' + key + ' is ' + x + ', not ' + edges[key]);
    };
    for (const li of ol.querySelectorAll(':scope > li')) {
      const date = li.querySelector('.date'), d = date.getBoundingClientRect(), t = li.querySelector('strong').getBoundingClientRect();
      const label = name + ' "' + date.textContent + '"', n = lines(date);
      if (i < 2 && n !== 1) out.push(label + ' is in ' + n + ' lines');
      if (wide && d.width > 14 * rem + 0.5) out.push(label + ' is ' + d.width + ' px wide, over 14rem');
      if (i === 2 && wide && date.textContent.length > 20 && n < 2) out.push(label + ' is over 14rem and in one line');
      same('title start', label, t.left);
      if (wide) {
        same('date end', label, d.right);
      } else {
        same('date start', label, d.left);
        if (d.bottom > t.top + 1) out.push(label + ' is not over its title');
      }
    }
  });
  return out.join('\n');
})`

// tagTimelineRows gives each row of each timeline the attribute data-row (its timeline and its place), which a measure
// of the dot reaches it by, and returns as JSON the 1rem in px and, for each row, the tag, the timeline, whether it is
// wide, and the right of its date.
const tagTimelineRows = `(() => {
  const rem = parseFloat(getComputedStyle(document.documentElement).fontSize), rows = [];
  document.querySelectorAll('[data-floe-part=timeline]').forEach((ol, i) => {
    const cs = getComputedStyle(ol);
    const wide = ol.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight) > 36 * rem;
    ol.querySelectorAll(':scope > li').forEach((li, k) => {
      li.setAttribute('data-row', i + '-' + k);
      rows.push({ tag: i + '-' + k, list: i, wide, dateRight: li.querySelector('.date').getBoundingClientRect().right });
    });
  });
  return JSON.stringify({ rem, rows });
})()`

// dotsIn measures the dots (the ::before boxes, which DevTools gives: pseudoBoxes) of the page open: those of a
// timeline start at one edge, and where it is wide each is 0.5rem or more from the right of its date. A dot moved by a
// theme or put in another column, or a date that has lost its gap, is told.
func (b *browser) dotsIn(what string) []string {
	b.t.Helper()

	var got struct {
		Rem  float64
		Rows []struct {
			Tag       string
			List      int
			Wide      bool
			DateRight float64
		}
	}
	if err := json.Unmarshal([]byte(b.eval(tagTimelineRows)), &got); err != nil {
		b.t.Fatal(err)
	}
	var amiss []string
	left := map[int]float64{}
	for _, r := range got.Rows {
		dot, _ := b.pseudoBoxes(`[data-row="` + r.Tag + `"]`)
		if dot == nil {
			amiss = append(amiss, what+": timeline row "+r.Tag+" has no dot")
			continue
		}
		if first, ok := left[r.List]; !ok {
			left[r.List] = dot[0]
		} else if math.Abs(dot[0]-first) > 1 {
			amiss = append(amiss, fmt.Sprintf("%s: the dot of row %s starts at %.1f, not %.1f", what, r.Tag, dot[0], first))
		}
		if r.Wide && dot[0]-r.DateRight < 0.5*got.Rem {
			amiss = append(amiss, fmt.Sprintf("%s: the dot of row %s is %.1f px from its date, less than 0.5rem", what, r.Tag, dot[0]-r.DateRight))
		}
	}
	return amiss
}

// TestRenderedTimelineDates measures a timeline's dates in the default theme (it holds the font, and no theme draws a
// timeline of its own) on a screen as wide as a desktop and a phone, and on paper; the dots of every theme are
// TestRenderedMarks's.
func TestRenderedTimelineDates(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	s.floe(t, "publish", s.write(t, "timeline-dates.yaml", timelineDatesPage), "--slug", "timeline-dates")
	measure := func(what, want string) {
		s.b.navigate("/a/timeline-dates/?raw")
		if got := s.b.eval(measureTimelineDates + `("` + want + `")`); got != "" {
			t.Errorf("%s:\n%s", what, got)
		}
		for _, a := range s.b.dotsIn(what) {
			t.Error(a)
		}
	}
	s.b.size(1440, 900, false)
	measure("1440 px", "wide")
	onPhone(t, s, 390)
	laidOut(t, s, 390)
	measure("390 px", "narrow")
	onPaper(t, s)
	measure("paper", "")
}
