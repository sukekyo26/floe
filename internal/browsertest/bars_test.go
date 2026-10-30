package browsertest_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/emulation"

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
// narrow, and a delete or unregister pressed once shown in words with the others but the clearing out of its way, at any
// width. The bar over the list holds what it has at every width, the page not scrolling sideways. On a narrow screen the
// header bar's second line holds the search, the bar over the list stuck under it, a row reached by ↑ shown under it.
func TestUIBars(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	inEveryLanguage(t, uiBars)
}

func uiBars(t *testing.T, lang i18n.Lang, msg func(string, ...any) string) {
	t.Helper()

	s := newSite(t)
	b := newBrowser(t, s.Base, lang)
	// No motion: a row rising in moves while it does.
	b.media("", &emulation.MediaFeature{Name: "prefers-reduced-motion", Value: "reduce"})
	s.floe(t, "publish", s.page(t, "p.yaml", "P", "<p>p</p>"), "--slug", "p")

	out, want := make([]string, 0, 32), make([]string, 0, 32)
	// The search: in the header bar of the list, not of a page's frame.
	b.navigate("/")
	out = append(out, b.eval("[document.getElementById('search').hidden, !!document.getElementById('search').closest('.bar')].join(' ')"))
	b.navigate("/a/p/")
	out = append(out, b.eval("String(document.getElementById('search').hidden)"))
	want = append(want, "false true", "true")

	// The bulk bar at each width: one line, within the bar over the list, which keeps its height, the list unmoved; on a
	// bar of 46rem or less its buttons' words out of sight but there to read; a delete pressed once in words, the rest but
	// the clearing gone.
	const bulk = `(() => {
		const bar = document.getElementById('bulk'), r = bar.getBoundingClientRect();
		const shown = [...bar.querySelectorAll('.btn')].filter((e) => e.offsetParent);
		const within = (x, y) => x.left >= y.left - 0.5 && x.right <= y.right + 0.5 && x.top >= y.top - 0.5 && x.bottom <= y.bottom + 0.5;
		const inside = within(r, document.querySelector('.toolbar').getBoundingClientRect()) && shown.every((e) => within(e.getBoundingClientRect(), r));
		const words = shown.map((e) => e.querySelector('.label')).filter((l) => l.getBoundingClientRect().width > 1).map((l) => l.textContent);
		return [shown.length, inside, words.join('/'), shown.every((e) => e.querySelector('.label').textContent !== '')].join(' ');
	})()`
	const first = "document.querySelector('#list li:not(.group)').getBoundingClientRect().top + ' ' + document.querySelector('.toolbar').getBoundingClientRect().height"
	// What the bar over the list holds, within it, the page not wider than the window (on a phone, the window widens to the page).
	const fits = `((width) => {
		const bar = document.querySelector('.toolbar').getBoundingClientRect();
		const shown = [...document.querySelector('.toolbar').children].filter((e) => e.offsetParent).map((e) => e.getBoundingClientRect());
		return shown.every((r) => r.left >= bar.left - 0.5 && r.right <= bar.right + 0.5) && document.documentElement.scrollWidth <= width;
	})(%d)`
	all := strings.Join([]string{msg("ui_select_page"), msg("ui_tags"), msg("ui_archive"), msg("ui_delete"), msg("ui_clear_selection")}, "/")
	b.navigate("about:blank")
	b.stub(barsList)
	for _, width := range []int64{320, 390, 660, 700, 768, 1000, 1100, 1440} {
		b.navigate("/")
		b.size(width, 900, width < 500)
		fit := b.eval(fmt.Sprintf(fits, width))
		before := b.eval(first)
		b.eval("for (const s of [...document.querySelectorAll('#list li:not(.group) .sel')].slice(0, 2)) s.click()")
		after := b.eval(first)
		selected := b.eval(bulk)
		b.eval("document.getElementById('bulk-delete').click()")
		armed := b.eval(bulk)
		b.eval("document.getElementById('bulk-clear').click(); document.querySelector('.tabs [data-tab=links]').click(); " +
			"for (const s of document.querySelectorAll('#list li:not(.group) .sel')) s.click(); document.getElementById('bulk-delete').click()")
		unlink := b.eval(bulk)
		out = append(out, fmt.Sprintf("%d %s %v / %s / %s / %s", width, fit, before == after, selected, armed, unlink))
		// The words show on a bar wider than 46rem, measured.
		words, clearing, confirm, unlinkConfirm := "", "", msg("ui_delete_bulk_confirm", 2), msg("ui_unlink_bulk_confirm", 2)
		if b.eval("String(document.querySelector('.toolbar').clientWidth > 46 * 16)") == "true" {
			words, clearing = all, "/"+msg("ui_clear_selection")
		}
		want = append(want, fmt.Sprintf("%d true true / 5 true %s true / 2 true %s%s true / 2 true %s%s true", width, words, confirm, clearing, unlinkConfirm, clearing))
	}
	b.unsize()

	// Narrow, the header bar is two lines, the search the second; scrolled, the bar over the list sits right under it.
	b.navigate("/")
	b.size(390, 844, true)
	b.eval("scrollTo(0, 400)")
	b.until("the page to scroll", "scrollY === 400")
	out = append(out, b.eval("(() => { const bar = document.querySelector('.bar').getBoundingClientRect(), q = document.getElementById('search').getBoundingClientRect(), "+
		"brand = document.querySelector('.brand').getBoundingClientRect(), tools = document.querySelector('.toolbar').getBoundingClientRect(); "+
		"return [q.top >= brand.bottom, q.bottom <= bar.bottom, Math.abs(tools.top - bar.bottom) <= 0.5].join(' '); })()"))
	// A row half under the bar over the list, reached by ↑, shows its title under the bar.
	key := func(k string) {
		b.eval("document.body.dispatchEvent(new KeyboardEvent('keydown', { key: '" + k + "', bubbles: true }))")
	}
	for range 30 {
		key("ArrowDown")
	}
	b.eval("scrollBy(0, [...document.querySelectorAll('#list .title')].find((a) => a.textContent === 'P28').getBoundingClientRect().top - " +
		"document.querySelector('.toolbar').getBoundingClientRect().bottom + 10)")
	key("ArrowUp")
	out = append(out, b.eval("[document.activeElement.textContent, document.activeElement.getBoundingClientRect().top >= "+
		"document.querySelector('.toolbar').getBoundingClientRect().bottom].join(' ')"))
	want = append(want, "true true true", "P28 true")
	b.unsize()

	check(t, "bars", out, strings.Join(want, "|"))
}
