package browsertest_test

import (
	"fmt"
	"strings"
	"testing"
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
	// The shelf's projects: name, color, share of the list and count.
	out = append(out, b.eval("[...document.querySelectorAll('#chips .chip')].map((c) => [c.dataset.project, c.style.getPropertyValue('--c'), "+
		"c.querySelector('.share').style.getPropertyValue('--share'), c.querySelector('.n').textContent].join(' ')).join(', ')"))
	// A project picked, picked again, is let go.
	pick := "[...document.querySelectorAll('#chips .chip')].find((c) => c.dataset.project === 'beta').click()"
	b.eval(pick)
	out = append(out, b.eval("new URLSearchParams(location.search).get('project') + ' ' + [...document.querySelectorAll('#list li:not(.group)')].map((li) => li.dataset.slug).join('')"))
	b.eval(pick)
	out = append(out, b.eval("String(new URLSearchParams(location.search).get('project'))"))

	// The columns line up across groups whatever a row's project: the time's right and the star's left are one, the chip
	// left of the time; a row of none has no chip.
	const columns = "(() => { const r = (slug, s) => document.querySelector(`#list li[data-slug=${slug}] ${s}`).getBoundingClientRect(); " +
		"return [r('a1', 'time').right === r('fv', 'time').right, r('a1', '.star').left === r('n1', '.star').left, " +
		"r('fv', '.proj').right <= r('fv', 'time').left, document.querySelector('#list li[data-slug=n1] .proj').hidden].join(' '); })()"
	out = append(out, b.eval(columns))
	// Narrow, a row's project and time go under its title, its actions where they were.
	b.size(390, 844, true)
	out = append(out, b.eval("(() => { const r = (s) => document.querySelector(`#list li[data-slug=fv] ${s}`).getBoundingClientRect(); "+
		"return [r('.proj').top >= r('.title').bottom, r('time').top >= r('.title').bottom, r('.proj').left === r('.title').left, "+
		"r('.actions').top < r('.proj').top].join(' '); })()"))
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
	// A project picked whose last entry is gone stays on the shelf, of the faint color, with no share.
	b.eval("setProject('gone')")
	out = append(out, b.eval("(() => { const c = document.querySelector('#chips .chip[data-project=gone]'); "+
		"return [c.style.getPropertyValue('--c'), c.querySelector('.share').style.getPropertyValue('--share'), c.querySelector('.n').textContent].join(' '); })()"))
	b.eval("setProject('')")
	// The shelf's bars sit in one place whatever their counts, of one, two and three digits.
	b.eval(stub("[['one', 1], ['two', 12], ['three', 123]].flatMap(([p, n]) => Array.from({ length: n }, (_, i) => " +
		"({ slug: `${p}${i}`, title: `${p}${i}`, description: '', project: p, createdAt: at(1), updatedAt: at(1) })))"))
	out = append(out, b.eval("(() => { const bars = [...document.querySelectorAll('#chips .share')].map((s) => s.getBoundingClientRect()); "+
		"return [bars.length, bars.every((r) => Math.abs(r.left - bars[0].left) <= 0.5 && Math.abs(r.right - bars[0].right) <= 0.5)].join(' '); })()"))

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
		"alpha var(--pc0) 30% 3, beta var(--pc1) 10% 1, gamma var(--pc2) 10% 1, a project of a long name var(--pc3) 10% 1, " +
			"delta var(--pc4) 10% 1, eps var(--pc5) 10% 1, fav var(--pc0) 10% 1, / var(--faint) 10% 1",
		"beta b1", "null",
		"true true true true",
		"true true true true",
		"rgb(251, 250, 248) rgb(246, 245, 241)",
		"true",
		"var(--faint) 0% 0",
		"3 true",
		" /a/a1/ 1 / 10",
	}, "|"))
}
