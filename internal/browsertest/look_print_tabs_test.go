package browsertest_test

import "testing"

// measureTabs finds each tabs whose titles are not drawn alike, read with the page drawn for print: on paper every
// tab shows under its title, so that the one picked on the screen is no more than the others. It returns {seen,
// amiss}: the tabs, and for each drawn apart how each title is drawn.
const measureTabs = `(() => {
  const amiss = [];
  const look = (e) => { const s = getComputedStyle(e);
    return [s.backgroundColor, s.backgroundImage, s.color, s.borderTop, s.borderBottom, s.boxShadow, s.borderRadius].join(' / '); };
  const all = document.querySelectorAll('[data-floe-part="tabs"]');
  for (const tabs of all) {
    const titles = [...tabs.querySelectorAll(':scope > label')].map(look);
    if (new Set(titles).size > 1) amiss.push({ at: 'tabs', text: titles.join(' | ') });
  }
  return JSON.stringify({ seen: all.length, amiss });
})()`

const lookPrintTabsFix = "draw every tab's title alike on paper: paint the one picked apart under @media screen alone"

// lookPrintTabs measures every page of floePages on paper (measureTabs), as tabs paints the picked title on the screen
// alone.
// LIMIT: tabs is the only part whose state print draws otherwise; check a part's that comes to do so too.
// LIMIT: it tells the picked title drawn apart alone, not a ground a theme gives every title alike outside @media screen.
func lookPrintTabs(t *testing.T, s *site) measured {
	t.Helper()

	onPaper(t, s)
	var m measured
	s.floePages(t, func(what string) { m.add(s.b.measure(what, measureTabs)) })
	return m
}

// brokenPrintTabs draws tabs in a theme of one's own whose tabs.css paints the first title apart on paper.
func brokenPrintTabs(t *testing.T, s *site) (string, measured) {
	t.Helper()

	onPaper(t, s)
	s.brokenPage(t, map[string]string{"themes/tabby/tabs.css": "@media print { :scope > label:first-of-type { color: rgb(200, 0, 0); } }\n"},
		themePage("tabby", "- tabs: [{ title: One, parts: [{ text: one }] }, { title: Two, parts: [{ text: two }] }]\n"))
	return "tabs", s.b.measure("broken", measureTabs)
}
