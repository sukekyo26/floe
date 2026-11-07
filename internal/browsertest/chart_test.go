package browsertest_test

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/chromedp"
)

const chartPage = `template: report
lang: en
title: Charts
sections:
  - heading: A
    parts:
      - chart:
          kind: line
          title: Lines
          unit: ms
          x: ['#401', '#410', '#421', '#428', '#433', '#436', '#439']
          series:
            - { name: Largest, values: [182, 178, 190, 240, 236, 228, 231] }
            - { name: Middle, values: [120, 122, 118, 126, 124, 121, 123] }
            - { name: Close, values: [60, 64, 58, 66, 70, 62, 122] }
            - { name: Goal, values: [200, 200, 180, 180, 150, 150, 121] }
      - chart:
          kind: bar
          title: Bars
          x: [List, Find, Edit, Set]
          series:
            - { name: This, values: [300, 200, 150, 90] }
            - { name: Last, values: [280, 240, 120, 70] }
      - table:
          columns: [Name, { title: This, num: true }, { title: Last, num: true }]
          rows:
            - [List, '300', '280']
            - [Find, '200', '240']
            - [Edit, '150', '120']
            - [Set, '90', '70']
      - chart:
          kind: line
          title: Dates
          x: ['11-01','11-02','11-03','11-04','11-05','11-06','11-07','11-08','11-09','11-10','11-11','11-12','11-13','11-14','11-15','11-16','11-17','11-18','11-19','11-20','11-21','11-22','11-23','11-24','11-25','11-26','11-27','11-28','11-29','11-30']
          series:
            - { name: A, values: [1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,27,28,29,30] }
      - chart:
          kind: bar
          title: Thin
          x: ['1','2','3','4','5','6','7','8','9','10','11','12','13','14','15','16','17','18','19','20','21','22','23','24','25','26','27','28','29','30']
          series:
            - { name: A, values: [1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,27,28,29,30] }
            - { name: B, values: [1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,27,28,29,30] }
            - { name: C, values: [1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,27,28,29,30] }
            - { name: D, values: [1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24,25,26,27,28,29,30] }
      - chart: { kind: line, title: A title long enough for the legend under it to go on a line of its own at any width of a page, x: [a, b], series: [ { name: One, values: [1, 2] } ] }
      - chart: { kind: line, title: Digits, x: [a, b], series: [ { name: Long name of a series, values: [123456789, 123499999] }, { name: B, values: [1234567890123, 12] } ] }
      - chart: { kind: line, title: Close, min: 1730000000000, x: [a, b], series: [ { name: A, values: [1730000000000, 1730000000900] } ] }
      - chart: { kind: line, title: Long ticks, x: ['aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb'], series: [ { name: A, values: [999999999, 1] } ] }
      - chart: { kind: line, title: Ends, x: [a, b], series: [ { name: A, values: [1.7e+308, -1.7e+308] } ] }
      - chart: { kind: bar, title: Tiny, x: [a, b], series: [ { name: A, values: [5.0e-324, 0.0] } ] }
      - columns:
          - parts: [ { chart: { kind: line, title: One, x: [a, b], series: [ { name: A, values: [123456789, 987654321] } ] } } ]
          - parts: [ { chart: { kind: bar, title: Two, x: [a, b], series: [ { name: A, values: [123456789, 987654321] } ] } } ]
          - parts: [ { chart: { kind: line, title: Three, x: ['2026-11-01', '2026-11-02', '2026-11-03', '2026-11-04'], series: [ { name: A, values: [100000, 200000, 150000, 180000] } ] } } ]
          - parts: [ { chart: { kind: line, title: Four, x: ['2026-11-01', '2026-11-02', '2026-11-03', '2026-11-04'], series: [ { name: A, values: [-400000000, -450000000, -420000000, -987654321] } ] } } ]
          - parts: [ { chart: { kind: line, title: Five, x: [a, b], series: [ { name: A, values: [1, 2] } ] } } ]
`

const chartSlides = `template: slides
lang: en
title: Charts
slides:
  - heading: Lines
    parts:
      - chart: { kind: line, title: Lines, x: [a, b, c], series: [ { name: One, values: [1, 2, 3] }, { name: Two, values: [3, 2, 1] } ] }
  - heading: Bars
    parts:
      - chart: { kind: bar, title: Bars, x: [a, b, c], series: [ { name: One, values: [1, 2, 3] } ] }
`

// chartInSlide tells a chart leaving its slide or running over what follows it on the page.
const chartInSlide = `(() => { const out = [];
  for (const c of document.querySelectorAll('[data-floe-part=chart]')) { const slide = c.closest('.slide').getBoundingClientRect(), r = c.getBoundingClientRect();
    if (r.left < slide.left - 0.5 || r.right > slide.right + 0.5 || r.bottom > slide.bottom + 0.5) out.push('out of its slide: ' + c.textContent.slice(0, 10)); }
  if (document.documentElement.scrollWidth > document.documentElement.clientWidth) out.push('the page scrolls sideways');
  return out.join('\n'); })()`

// chartNames is the names of the images a screen reader is told of.
func chartNames(t *testing.T, b *browser) []string {
	t.Helper()
	tree, err := chromedp.Call(b.ctx, accessibility.GetFullAXTree, accessibility.GetFullAXTreeParams{})
	if err != nil {
		t.Fatal(err)
	}
	text := func(v *accessibility.Value) string {
		if v == nil {
			return ""
		}
		var s string
		if json.Unmarshal(v.Value, &s) != nil {
			return string(v.Value)
		}
		return s
	}
	var names []string
	for _, n := range tree.Nodes {
		if !n.Ignored && text(n.Role) == "image" {
			names = append(names, text(n.Name))
		}
	}
	return names
}

// measureChart is what no other test measures of a chart at this width: its colours and text on its ground, its text in
// its box and not over each other, its marks' sizes, and its table as the part table draws one.
const measureChart = `(() => {
  const cv = document.createElement('canvas').getContext('2d', { willReadFrequently: true });
  const rgb = (c) => { cv.clearRect(0, 0, 1, 1); cv.fillStyle = c; cv.fillRect(0, 0, 1, 1); return [...cv.getImageData(0, 0, 1, 1).data]; };
  const lum = (c) => { const [r, g, b] = rgb(c).slice(0, 3).map((v) => { v /= 255; return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4; }); return 0.2126 * r + 0.7152 * g + 0.0722 * b; };
  const ratio = (a, b) => { const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p); return (x + 0.05) / (y + 0.05); };
  const out = new Set();
  const charts = [...document.querySelectorAll('[data-floe-part=chart]')];
  if (document.documentElement.scrollWidth > document.documentElement.clientWidth) out.add('the page scrolls sideways');
  for (const c of charts) {
    c.querySelectorAll('details').forEach((d) => { d.open = true; });
    let g = c.querySelector('svg'); while (g && rgb(getComputedStyle(g).backgroundColor)[3] === 0) g = g.parentElement;
    const ground = g ? getComputedStyle(g).backgroundColor : 'rgb(255,255,255)';
    for (const i of [1, 2, 3, 4]) { const e = document.createElement('i'); e.style.color = 'var(--chart-' + i + ')'; c.append(e); const v = getComputedStyle(e).color; e.remove();
      if (ratio(v, ground) < 3) out.add('--chart-' + i + ' ' + ratio(v, ground).toFixed(2) + ' on its ground'); }
    const box = c.getBoundingClientRect(), svg = c.querySelector('svg');
    const texts = [...svg.querySelectorAll('text')].filter((t) => t.getBoundingClientRect().width > 0);
    const rem = parseFloat(getComputedStyle(document.documentElement).fontSize);
    for (const t of texts) { const r = t.getBoundingClientRect(), f = getComputedStyle(t).color;
      if (t.matches('.x') && r.width > 10 * rem) out.add('x not cut near 8rem: ' + t.textContent);
      if (ratio(f, ground) < 4.5) out.add('text ' + t.textContent + ' ' + ratio(f, ground).toFixed(2));
      if (r.left < box.left - 0.5 || r.right > box.right + 0.5) out.add('text out of its box: ' + t.textContent); }
    const seen = texts.filter((t) => t.matches('.x, .end, .val, .tick'));
    for (let i = 0; i < seen.length; i++) for (let j = i + 1; j < seen.length; j++) { const a = seen[i].getBoundingClientRect(), b = seen[j].getBoundingClientRect();
      if (a.left < b.right - 0.5 && b.left < a.right - 0.5 && a.top < b.bottom - 2 && b.top < a.bottom - 2) out.add('text over text: ' + seen[i].textContent + ' / ' + seen[j].textContent); }
    for (const d of svg.querySelectorAll('.dot')) { const r = d.getBoundingClientRect(), s = getComputedStyle(d);
      if (Math.abs(r.width - r.height) > 0.5) out.add('a dot not round');
      if (parseFloat(s.r) > 5 || parseFloat(s.strokeWidth) > 2) out.add('a dot of ' + s.r + ' and ' + s.strokeWidth); }
    for (const g of svg.querySelectorAll(':scope > g')) { const end = g.querySelector('.end'), dots = g.querySelectorAll('.dot'); if (!end || !dots.length) continue;
      if (dots[dots.length - 1].getBoundingClientRect().right > end.getBoundingClientRect().left + 0.5) out.add('a dot over its label: ' + end.textContent); }
    for (const l of svg.querySelectorAll('.seg')) if (parseFloat(getComputedStyle(l).strokeWidth) > 3) out.add('a line of ' + getComputedStyle(l).strokeWidth);
    for (const r of svg.querySelectorAll('rect')) { const s = parseFloat(getComputedStyle(r).strokeWidth);
      if (s > 2) out.add('a bar edge of ' + s); }
    const top = svg.matches('.thin') ? null : svg.querySelector('.s1 rect');
    if (top) { const y = top.getBoundingClientRect().top + parseFloat(getComputedStyle(top).strokeWidth) / 2;
      const line = [...svg.querySelectorAll('text.tick')].find((t) => t.textContent === '300');
      if (line && Math.abs(y - line.getBoundingClientRect().top - line.getBoundingClientRect().height / 2) > 1.5) out.add('a bar of 300 off its tick'); }
    for (const r of svg.querySelectorAll('rect')) { const st = getComputedStyle(r), line = st.stroke === 'none' ? 0 : parseFloat(st.strokeWidth);
      if (line > 0 && r.getBoundingClientRect().width - line < 4) out.add('a bar under 4px with a frame'); }
    if (svg.getBoundingClientRect().width - parseFloat(getComputedStyle(svg).paddingLeft) - parseFloat(getComputedStyle(svg).paddingRight) < 30) out.add('no room for the plot: ' + c.textContent.slice(0, 20));

    const names = [...svg.querySelectorAll('.name')].filter((n) => n.getBoundingClientRect().width > 0).length;
    const pad = getComputedStyle(c), narrow = c.clientWidth - parseFloat(pad.paddingLeft) - parseFloat(pad.paddingRight) < 34 * rem;
    if ([...c.querySelectorAll('.legend li')].filter((l) => l.getBoundingClientRect().width > 0).length !== svg.querySelectorAll(':scope > g').length) out.add('a series not in the legend: ' + c.textContent.slice(0, 20));
    const legend = c.querySelector('.legend li').getBoundingClientRect(), title = c.querySelector('.title').getBoundingClientRect();
    if (legend.top >= title.bottom && Math.abs(legend.left - title.left) > 0.5) out.add('a legend on its own line off the title by ' + (legend.left - title.left));
    if (svg.querySelector('.name') && (narrow ? names !== 0 : names === 0)) out.add('names shown ' + names + ' at ' + c.clientWidth);
  }
  if ([...document.querySelectorAll('[data-floe-part=chart] svg *')].some((e) => [...e.attributes].some((a) => /NaN|Inf/.test(a.value)))) out.add('a place not a number');
  const look = (e) => { if (!e) return 'none'; const s = getComputedStyle(e); return [s.backgroundColor, s.borderBottom, s.borderLeft, s.borderRadius, s.padding, s.fontWeight, s.color].join('/'); };
  const mine = charts[1].querySelector('table'), real = [...document.querySelectorAll('[data-floe-part=table] table')].find((t) => !t.closest('[data-floe-part=chart]'));
  for (const q of ['thead th:first-child', 'thead th:last-child', 'tbody tr:nth-child(1) td:first-child', 'tbody tr:nth-child(2) td:first-child', 'tbody tr:nth-child(4) td:first-child', 'tbody tr:nth-child(4) td:last-child']) {
    if (look(mine.querySelector(q)) !== look(real.querySelector(q))) out.add('table ' + q + ': ' + look(mine.querySelector(q)) + ' != ' + look(real.querySelector(q))); }
  return [...out].join('\n');
})`

// TestUIChart draws charts in every theme at the widths that turn what they show, down to five columns at their least.
func TestUIChart(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	themes, err := os.ReadDir("../kit/floe/themes")
	if err != nil {
		t.Fatal(err)
	}
	file := s.write(t, "chart.yaml", chartPage)
	slides := s.write(t, "chart-slides.yaml", chartSlides)
	amiss := make([]string, 0, len(themes))
	for _, th := range themes {
		s.floe(t, "publish", file, "--slug", "chart-"+th.Name(), "--theme", th.Name())
		b.navigate("/a/chart-" + th.Name() + "/?raw")
		for _, w := range []int64{1440, 640, 400, 320} {
			b.size(w, 900, false)
			if got := b.eval(measureChart + "()"); got != "" {
				amiss = append(amiss, fmt.Sprintf("%s at %d:\n%s", th.Name(), w, got))
			}
		}
		// Five columns at their least width.
		b.size(1440, 900, false)
		var least int64
		if _, err := fmt.Sscan(b.eval(`(() => { const c = document.querySelector('[data-floe-part=columns]'), g = parseFloat(getComputedStyle(c).columnGap),
		  rem = parseFloat(getComputedStyle(document.documentElement).fontSize);
		  return String(Math.ceil(5 * 16 * rem + 4 * g + innerWidth - c.getBoundingClientRect().width)); })()`), &least); err != nil {
			t.Fatal(err)
		}
		b.size(least, 900, false)
		b.until("the columns at their least", `Math.abs(document.querySelector('[data-floe-part=columns] > div').getBoundingClientRect().width - 16 * parseFloat(getComputedStyle(document.documentElement).fontSize)) < 1`)
		if got := b.eval(measureChart + "()"); got != "" {
			amiss = append(amiss, fmt.Sprintf("%s at the columns' least, %d:\n%s", th.Name(), least, got))
		}
		b.unsize()
		if th.Name() == "notebook" || th.Name() == "blueprint" {
			if got := b.eval(`(() => { const s = document.querySelector('[data-floe-part=chart] svg'), probe = document.createElement('i');
			  probe.style.background = 'var(--surface)'; s.parentElement.append(probe); const surface = getComputedStyle(probe).backgroundColor; probe.remove();
			  return getComputedStyle(s).backgroundColor === surface ? '' : getComputedStyle(s).backgroundColor + ' under the plot, not ' + surface; })()`); got != "" {
				amiss = append(amiss, th.Name()+": "+got)
			}
		}
		s.floe(t, "publish", slides, "--slug", "chart-slides-"+th.Name(), "--theme", th.Name())
		b.navigate("/a/chart-slides-" + th.Name() + "/?raw")
		for _, w := range []int64{1440, 400} {
			b.size(w, 900, false)
			if got := b.eval(chartInSlide); got != "" {
				amiss = append(amiss, fmt.Sprintf("%s's slides at %d:\n%s", th.Name(), w, got))
			}
		}
		b.unsize()
	}
	b.navigate("/a/chart-default/?raw")
	if got := chartNames(t, b); !slices.Contains(got, "Line chart: Lines (ms)") || !slices.Contains(got, "Bar chart: Bars") {
		t.Errorf("the charts are named %q to a screen reader", got)
	}
	if len(amiss) > 0 {
		t.Error(strings.Join(amiss, "\n"))
	}
}
