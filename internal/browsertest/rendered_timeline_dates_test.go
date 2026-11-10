package browsertest_test

import "testing"

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
// one edge, its titles start at one, no date reaches its title, and a date over 14rem is folded; where narrow, the dates
// and the titles start at one edge and the date is over its title.
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
    const wide = ol.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight) >= 36 * rem;
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
        if (d.right > t.left) out.push(label + ' reaches its title');
      } else {
        same('date start', label, d.left);
        if (d.bottom > t.top + 1) out.push(label + ' is not over its title');
      }
    }
  });
  return out.join('\n');
})`

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
	}
	s.b.size(1440, 900, false)
	measure("1440 px", "wide")
	onPhone(t, s, 390)
	laidOut(t, s, 390)
	measure("390 px", "narrow")
	onPaper(t, s)
	measure("paper", "")
}
