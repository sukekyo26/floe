package browsertest_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
)

// barsList is the list the stub answers for the bars: pages enough to scroll, and links.
const barsList = `[
	...Array.from({ length: 40 }, (_, i) => ({ slug: ` + "`p${i}`" + `, title: ` + "`P${i}`" + `, description: '', createdAt: at(1), updatedAt: at(1) })),
	{ slug: 'l', title: 'Linked', description: '', path: '/tmp/x/l.html', createdAt: at(1), updatedAt: at(1) },
	{ slug: 'k', title: 'Kept', description: '', path: '/tmp/x/k.html', createdAt: at(1), updatedAt: at(1) },
]`

// TestUIBars puts the search in the header bar of the list alone, and the bulk bar in the bar over the list while items
// are selected: on one line at every width, the list unmoved, its buttons' words out of sight but read where the bar is
// narrow, and a delete or unregister pressed once shown in words with the others out of its way, at any width. On a narrow screen the
// header bar's second line holds the search, the bar over the list stuck under it.
func TestUIBars(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	inEveryLanguage(t, uiBars)
}

func uiBars(t *testing.T, lang i18n.Lang, msg func(string, ...any) string) {
	t.Helper()

	s := newSite(t)
	b := newBrowser(t, s.Base, lang)
	s.floe(t, "publish", s.page(t, "p.yaml", "P", "<p>p</p>"), "--slug", "p")

	out := make([]string, 0, 32)
	// The search: in the header bar of the list, not of a page's frame.
	b.navigate("/")
	out = append(out, b.eval("[document.getElementById('search').hidden, !!document.getElementById('search').closest('.bar')].join(' ')"))
	b.navigate("/a/p/")
	out = append(out, b.eval("String(document.getElementById('search').hidden)"))

	// The bulk bar at each width: one line, within the bar over the list, which keeps its height, the list unmoved; on a
	// bar under 46rem its buttons' words out of sight but there to read; a delete pressed once in words, the rest gone.
	const bulk = `(() => {
		const bar = document.getElementById('bulk'), r = bar.getBoundingClientRect();
		const shown = [...bar.querySelectorAll('.btn')].filter((e) => e.offsetParent);
		const within = (x, y) => x.left >= y.left - 0.5 && x.right <= y.right + 0.5 && x.top >= y.top - 0.5 && x.bottom <= y.bottom + 0.5;
		const inside = within(r, document.querySelector('.toolbar').getBoundingClientRect()) && shown.every((e) => within(e.getBoundingClientRect(), r));
		const words = shown.map((e) => e.querySelector('.label')).filter((l) => l.getBoundingClientRect().width > 1).map((l) => l.textContent);
		return [shown.length, inside, words.join('/'), shown.every((e) => e.querySelector('.label').textContent !== '')].join(' ');
	})()`
	const first = "document.querySelector('#list li:not(.group)').getBoundingClientRect().top + ' ' + document.querySelector('.toolbar').getBoundingClientRect().height"
	for _, width := range []int64{390, 660, 700, 768, 1000, 1100, 1440} {
		b.navigate("/")
		b.size(width, 900, width < 500)
		b.eval(stub(barsList))
		before := b.eval(first)
		b.eval("for (const s of [...document.querySelectorAll('#list li:not(.group) .sel')].slice(0, 2)) s.click()")
		after := b.eval(first)
		selected := b.eval(bulk)
		b.eval("document.getElementById('bulk-delete').click()")
		armed := b.eval(bulk)
		b.eval("document.getElementById('bulk-clear').click(); document.querySelector('.tabs [data-tab=links]').click(); " +
			"for (const s of document.querySelectorAll('#list li:not(.group) .sel')) s.click(); document.getElementById('bulk-delete').click()")
		unlink := b.eval(bulk)
		out = append(out, fmt.Sprintf("%d %v / %s / %s / %s", width, before == after, selected, armed, unlink))
	}
	b.unsize()

	// Narrow, the header bar is two lines, the search the second; scrolled, the bar over the list sits right under it.
	b.navigate("/")
	b.size(390, 844, true)
	b.eval(stub(barsList))
	b.eval("scrollTo(0, 400)")
	b.until("the page to scroll", "scrollY === 400")
	out = append(out, b.eval("(() => { const bar = document.querySelector('.bar').getBoundingClientRect(), q = document.getElementById('search').getBoundingClientRect(), "+
		"brand = document.querySelector('.brand').getBoundingClientRect(), tools = document.querySelector('.toolbar').getBoundingClientRect(); "+
		"return [q.top >= brand.bottom, q.bottom <= bar.bottom, Math.abs(tools.top - bar.bottom) <= 0.5].join(' '); })()"))
	b.unsize()

	// On a wide bar all the words show, on a narrow one none; a delete or unregister pressed once shows its words alone.
	all := strings.Join([]string{msg("ui_select_page"), msg("ui_tags"), msg("ui_archive"), msg("ui_delete"), msg("ui_clear_selection")}, "/")
	wide := func(w int) string {
		return fmt.Sprintf("%d true / 5 true %s true / 1 true %s true / 1 true %s true", w, all, msg("ui_delete_bulk_confirm", 2), msg("ui_unlink_bulk_confirm", 2))
	}
	narrow := func(w int) string {
		return fmt.Sprintf("%d true / 5 true  true / 1 true %s true / 1 true %s true", w, msg("ui_delete_bulk_confirm", 2), msg("ui_unlink_bulk_confirm", 2))
	}
	check(t, "bars", out, strings.Join([]string{
		"false true", "true",
		narrow(390), narrow(660), narrow(700), narrow(768), narrow(1000), wide(1100), wide(1440),
		"true true true",
	}, "|"))
}
