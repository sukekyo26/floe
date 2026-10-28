package browsertest_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
)

// groupsList is the list the stub answers for the groups: seven projects, more than the six colors, counted over every
// entry (beta's second is archived, gamma's a link); a favourite of alpha; a project named fav; and a page of none.
const groupsList = `[
	{ slug: 'a1', title: 'A1', description: '', project: 'alpha', createdAt: at(1), updatedAt: at(9) },
	{ slug: 'a2', title: 'A2', description: '', project: 'alpha', createdAt: at(1), updatedAt: at(8), favorite: true },
	{ slug: 'a3', title: 'A3', description: '', project: 'alpha', createdAt: at(1), updatedAt: at(7) },
	{ slug: 'b1', title: 'B1', description: '', project: 'beta', createdAt: at(1), updatedAt: at(6) },
	{ slug: 'b2', title: 'B2', description: '', project: 'beta', createdAt: at(1), updatedAt: at(6), archived: true },
	{ slug: 'g1', title: 'G1', description: '', project: 'gamma', createdAt: at(1), updatedAt: at(5) },
	{ slug: 'g2', title: 'G2', description: '', project: 'gamma', path: '/tmp/x/g2.html', createdAt: at(1), updatedAt: at(5) },
	{ slug: 'd1', title: 'D1', description: '', project: 'delta', createdAt: at(1), updatedAt: at(4) },
	{ slug: 'e1', title: 'E1', description: '', project: 'eps', createdAt: at(1), updatedAt: at(4) },
	{ slug: 'fv', title: 'Fav', description: '', project: 'a project of a long name', createdAt: at(1), updatedAt: at(3) },
	{ slug: 'f2', title: 'F2', description: '', project: 'fav', createdAt: at(1), updatedAt: at(3) },
	{ slug: 'n1', title: 'N1', description: '', createdAt: at(1), updatedAt: at(2) },
]`

// TestUIGroups lists the favourites, then each project in projectOrder (most entries first, every entry counted), then
// those of none, each headed with its color and count; colors the projects so on the shelf and the rows, the seventh
// as the first; lets a project go when picked again; lines the rows' columns up across groups and puts a row's project
// under its title on a narrow screen; boxes an empty list; and steps the frame through the groups' order.
func TestUIGroups(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.navigate("/")
	b.size(1440, 900, false)
	b.eval(stub(groupsList))

	out := make([]string, 0, 16)
	// The groups and their rows: a heading as its color and words, a row as its slug and color.
	out = append(out, b.eval("[...document.getElementById('list').children].map((li) => li.classList.contains('group') ? "+
		"'\\n' + li.style.getPropertyValue('--c') + ' ' + li.textContent : li.dataset.slug + ':' + li.style.getPropertyValue('--c')).join(' ')"))
	// Every heading alike: no line over the card, the first's or the next's.
	out = append(out, b.eval("[...document.querySelectorAll('#list li.group')].map((li) => getComputedStyle(li).borderTopWidth).join(' ')"))
	// The shelf's projects: name, color, title and count.
	out = append(out, b.eval("[...document.querySelectorAll('#chips .chip')].map((c) => [c.dataset.project, c.style.getPropertyValue('--c'), "+
		"c.title, c.querySelector('.n').textContent].join(' ')).join(', ')"))
	// A project picked, picked again, is let go.
	pick := "[...document.querySelectorAll('#chips .chip')].find((c) => c.dataset.project === 'beta').click()"
	b.eval(pick)
	out = append(out, b.eval("new URLSearchParams(location.search).get('project') + ' ' + [...document.querySelectorAll('#list li:not(.group)')].map((li) => li.dataset.slug).join('')"))
	b.eval(pick)
	out = append(out, b.eval("String(new URLSearchParams(location.search).get('project'))"))

	// The columns line up across groups whatever a row's project: the time's right and the star's left are one, the chip
	// and the creation date left of the time; a row of none has no chip.
	const columns = "(() => { const r = (slug, s) => document.querySelector(`#list li[data-slug=${slug}] ${s}`).getBoundingClientRect(); " +
		"return [r('a1', 'time').right === r('fv', 'time').right, r('a1', '.star').left === r('n1', '.star').left, " +
		"r('fv', '.proj').right <= r('fv', '.made').left && r('fv', '.made').right <= r('fv', 'time').left, document.querySelector('#list li[data-slug=n1] .proj').hidden].join(' '); })()"
	out = append(out, b.eval(columns))
	// Narrow, a row's project, creation date and time go under its title, within the row (wrapping where they do not fit
	// on one line), the date and the time whole, the project's chip not squeezed away, its actions where they were.
	b.size(390, 844, true)
	out = append(out, b.eval("(() => { const li = document.querySelector('#list li[data-slug=fv]'), r = (s) => li.querySelector(s).getBoundingClientRect(); "+
		"const whole = (s) => li.querySelector(s).scrollWidth <= li.querySelector(s).clientWidth; "+
		"return [r('.proj').top >= r('.title').bottom, r('time').top >= r('.title').bottom, r('.proj').left === r('.title').left, "+
		"r('.actions').top < r('.proj').top, r('.made').right <= r('.actions').left && r('time').right <= r('.actions').left && r('.proj').width >= 48, "+
		"whole('.made') && whole('time')].join(' '); })()"))
	b.size(1440, 900, false)
	// A heading pointed at keeps its ground, as it is not pressed; a row pointed at takes the hover's.
	ground := func(selector string) string {
		r := b.rect("document.querySelector('" + selector + "')")
		b.point(r.X+r.Width/2, r.Y+r.Height/2)
		return b.until("the pointer on "+selector, "document.querySelector('"+selector+":hover') && "+
			"new Promise((done) => setTimeout(done, 300)).then(() => getComputedStyle(document.querySelector('"+selector+"')).backgroundColor)")
	}
	out = append(out, ground("#list li.group")+" "+ground("#list li[data-slug=a1]"))
	b.point(0, 0)

	// An empty list keeps the box the list had.
	b.eval(stub(`[{ slug: 'x', title: 'X', description: '', createdAt: at(1), updatedAt: at(1) }]`))
	b.eval("document.querySelector('.views [data-view=fav]').click()")
	out = append(out, b.eval("getComputedStyle(document.getElementById('empty-favorites')).backgroundColor === getComputedStyle(document.documentElement).getPropertyValue('--panel').trim().replace(/^#(..)(..)(..)$/, (_, r, g, b) => `rgb(${parseInt(r, 16)}, ${parseInt(g, 16)}, ${parseInt(b, 16)})`)"))
	b.eval("document.querySelector('.views [data-view=all]').click()")
	// A project picked whose last entry is gone stays on the shelf, of the faint color.
	b.eval("setProject('gone')")
	out = append(out, b.eval("(() => { const c = document.querySelector('#chips .chip[data-project=gone]'); "+
		"return [c.style.getPropertyValue('--c'), c.querySelector('.n').textContent].join(' '); })()"))
	b.eval("setProject('')")
	// The shelf's counts line up whatever their digits, one, two or three; and long names, a project's or a tag's,
	// English or Japanese, stay on one line, cut off, whole as the title, on the shelf and in a heading, its count unsplit.
	const long, longJa = "floe-management-page-redesign-experiments", "管理ページの見た目を作り直すための試しの長いプロジェクトの名前"
	// A list of projects of n entries each, each entry's tag its project's name and -tag.
	projects := func(ns string) string {
		return stub("[" + ns + "].flatMap(([p, n]) => Array.from({ length: n }, (_, i) => ({ slug: `${p.length}-${i}`, title: `${p}${i}`, " +
			"description: '', project: p, tags: i ? [] : [p + '-tag'], createdAt: at(1), updatedAt: at(1) })))")
	}
	b.eval(projects("['one', 1], ['two', 12], ['three', 123]"))
	out = append(out, b.eval("(() => { const n = [...document.querySelectorAll('#chips .n')].map((e) => e.getBoundingClientRect()); "+
		"return [n.length, n.every((r) => Math.abs(r.left - n[0].left) <= 0.5 && Math.abs(r.right - n[0].right) <= 0.5)].join(' '); })()"))
	b.eval(projects("['one', 1], [" + js(long) + ", 1], [" + js(longJa) + ", 1]"))
	oneLine := "(() => { const one = (sel, short) => { const e = document.querySelector(sel), s = document.querySelector(short); " +
		"return Math.abs(e.getBoundingClientRect().height - s.getBoundingClientRect().height) <= 0.5 && e.querySelector('.name').scrollWidth > e.querySelector('.name').clientWidth " +
		"&& (e.title || e.firstChild.title) !== ''; }; const heading = (name) => [...document.querySelectorAll('#list li.group')].find((li) => li.textContent.startsWith(name)); " +
		"const h = heading(" + js(longJa) + "), n = h.querySelector('.n'); " +
		"return [one('#chips .chip[data-project=' + JSON.stringify(" + js(long) + ") + ']', '#chips .chip[data-project=one]'), " +
		"one('#chips .chip[data-project=' + JSON.stringify(" + js(longJa) + ") + ']', '#chips .chip[data-project=one]'), " +
		"one('#tags .chip[data-tag=' + JSON.stringify(" + js(long+"-tag") + ") + ']', '#tags .chip[data-tag=one-tag]'), " +
		"h.firstChild.title === " + js(longJa) + ", n.getClientRects().length === 1 && n.getBoundingClientRect().height < 1.5 * parseFloat(getComputedStyle(n).fontSize)].join(' '); })()"
	out = append(out, b.eval(oneLine))
	b.size(390, 844, true)
	out = append(out, b.eval("(() => { const h = [...document.querySelectorAll('#list li.group')].find((li) => li.textContent.startsWith("+js(longJa)+")); "+
		"const s = [...document.querySelectorAll('#list li.group')].find((li) => li.textContent.startsWith('one')); const n = h.querySelector('.n'); "+
		"return [Math.abs(h.getBoundingClientRect().height - s.getBoundingClientRect().height) <= 0.5, n.getClientRects().length === 1 && "+
		"n.getBoundingClientRect().height < 1.5 * parseFloat(getComputedStyle(n).fontSize)].join(' '); })()"))
	// The whole window's width, however wide: the shelf at the left edge under the logo, the list to the bar's right end.
	for _, width := range []int64{1920, 2560} {
		b.size(width, 900, false)
		out = append(out, b.eval("(() => { const shelf = document.querySelector('.shelf .views button').getBoundingClientRect(), "+
			"list = document.querySelector('.panel').getBoundingClientRect(), logo = document.querySelector('.brand').getBoundingClientRect(), "+
			"end = document.getElementById('server').getBoundingClientRect(); "+
			"return [shelf.left, logo.left, document.body.getBoundingClientRect().right - list.right, document.body.getBoundingClientRect().right - end.right >= 16].join(' '); })()"))
	}
	b.size(1440, 900, false)

	// The frame steps through the groups as the list has them: the favourite, then alpha's.
	b.eval("sessionStorage.setItem('artifacts.list', '')")
	b.navigate("/a/a2/")
	b.eval(stub(groupsList))
	out = append(out, b.until("the neighbours", "document.getElementById('frame-pos').textContent && "+
		"['frame-prev', 'frame-next'].map((id) => document.getElementById(id).getAttribute('href')).concat(document.getElementById('frame-pos').textContent).join(' ')"))

	// A heading as the list draws it: its color, then its name and count with nothing between.
	h := func(color, name string, n int) string { return fmt.Sprintf("\n%s %s%d in all", color, name, n) }
	check(t, "groups", out, strings.Join([]string{
		h("var(--star)", "Favorites", 1) + " a2:var(--pc0) " +
			h("var(--pc0)", "alpha", 2) + " a1:var(--pc0) a3:var(--pc0) " +
			h("var(--pc1)", "beta", 1) + " b1:var(--pc1) " +
			h("var(--pc2)", "gamma", 1) + " g1:var(--pc2) " +
			h("var(--pc3)", "a project of a long name", 1) + " fv:var(--pc3) " +
			h("var(--pc4)", "delta", 1) + " d1:var(--pc4) " +
			h("var(--pc5)", "eps", 1) + " e1:var(--pc5) " +
			h("var(--pc0)", "fav", 1) + " f2:var(--pc0) " +
			h("var(--faint)", "None recorded", 1) + " n1:var(--faint)",
		strings.TrimSpace(strings.Repeat("0px ", 9)),
		"alpha var(--pc0) alpha 3, beta var(--pc1) beta 1, gamma var(--pc2) gamma 1, a project of a long name var(--pc3) a project of a long name 1, " +
			"delta var(--pc4) delta 1, eps var(--pc5) eps 1, fav var(--pc0) fav 1, / var(--faint) None recorded 1",
		"beta b1", "null",
		"true true true true",
		"true true true true true true",
		"rgb(251, 250, 248) rgb(246, 245, 241)",
		"true",
		"var(--faint) 0",
		"3 true", "true true true true true", "true true",
		"16 16 16 true", "16 16 16 true",
		" /a/a1/ 1 / 10",
	}, "|"))
}

// TestUIRowColumns lines up a row's project chip, creation date and time from row to row, however long its time reads:
// minutes and hours of one digit and of two, days, months and years ago, in every language.
func TestUIRowColumns(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	inEveryLanguage(t, uiRowColumns)
}

func uiRowColumns(t *testing.T, lang i18n.Lang, _ func(string, ...any) string) {
	t.Helper()

	s := newSite(t)
	b := newBrowser(t, s.Base, lang)
	b.navigate("/")
	b.size(1440, 900, false)
	b.eval(stub(`[[2, 'p'], [12, 'p'], [180, 'a project of a long name'], [780, 'p'], [4320, 'p'], [480000, 'p'], [1100000, 'p']].map(([m, p], i) => ({
		slug: 'r' + i, title: 'R' + i, description: '', project: p, createdAt: at(1), updatedAt: new Date(Date.now() - m * 60000).toISOString() }))`))
	out := b.eval(`(() => {
		const rows = [...document.querySelectorAll('#list li:not(.group)')], r = (li, s) => li.querySelector(s).getBoundingClientRect();
		const same = (f) => rows.every((li) => Math.abs(f(li) - f(rows[0])) <= 0.5);
		return [rows.length, new Set(rows.map((li) => li.querySelector('time').textContent.length)).size > 2,
			same((li) => r(li, '.proj').right), same((li) => r(li, '.made').left), same((li) => r(li, '.made').right), same((li) => r(li, 'time').right)].join(' ');
	})()`)
	b.unsize()
	check(t, "row columns", []string{out}, "7 true true true true true")
}
