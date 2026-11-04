package browsertest_test

import (
	"strings"
	"testing"
)

// listList is the list the stub answers for the page list: what is new of each kind, a day each, newest first. day is the
// time so many days before today's noon.
const listList = `(() => {
	const day = (d) => new Date(new Date(new Date().setHours(12, 0, 0, 0)) - d * 86400000).toISOString();
	return [
		{ slug: 'v', title: 'Version', description: 'the words', news: 'version', version: 3, template: 'design', project: 'p', tags: ['t'], createdAt: day(9), updatedAt: day(0) },
		{ slug: 'n', title: 'New', description: '', news: 'new', version: 1, template: 'report', project: 'q', createdAt: day(1), updatedAt: day(1) },
		{ slug: 'r', title: 'Read', description: '', version: 2, template: 'report', project: 'p', createdAt: day(9), updatedAt: day(3) },
		{ slug: 'l', title: 'Link', description: '', path: '/tmp/x/l.html', createdAt: day(9), updatedAt: day(7) },
		{ slug: 'd', title: 'Data', description: '', path: '/tmp/x/d.yaml', template: 'handoff', createdAt: day(9), updatedAt: day(8) },
		{ slug: 'f', title: 'Folder', description: '', path: '/tmp/x/f', folder: true, createdAt: day(9), updatedAt: day(40) },
		{ slug: 'g', title: 'Gone', description: '', path: '/tmp/x/g.html', missing: true, createdAt: day(400), updatedAt: day(400) },
		{ slug: 'a', title: 'Kept', description: '', news: 'new', version: 1, archived: true, createdAt: day(9), updatedAt: day(9) },
	];
})()`

// What a row shows, in order, for each row of the list: its first column, its time's being there, its second line's parts.
const rowsDrawn = `[...document.querySelectorAll('#list > li')].map((li) => {
	if (li.classList.contains('group')) return '#';
	const lead = li.querySelector('.lead');
	const mark = lead.querySelector('.dot') ? 'dot' : lead.querySelector('.ring') ? 'ring' : lead.querySelector('svg') ? 'mark' : '?';
	return [li.dataset.slug, mark, getComputedStyle(li.querySelector('.title')).fontWeight, ...[...li.querySelector('.sub').children].map((e) => e.className + ':' + e.textContent)].join(' ');
}).join('|')`

// TestUIList draws the list newest first under its days, a dot and a version's mark as news says, a link and a folder with
// neither, opens a row beside the list, goes through the rows by j and k, keeps a row read on the view of the new ones, keeps
// the shelf's picks in the URL, and on one column opens a row whole.
func TestUIList(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.stub(listList)
	b.navigate("/")
	b.size(1440, 900, false)
	key := func(k string) {
		b.eval("document.body.dispatchEvent(new KeyboardEvent('keydown', { key: '" + k + "', bubbles: true }))")
	}
	out := make([]string, 0, 24)
	out = append(out, b.eval(rowsDrawn))
	out = append(out, b.eval("[...document.querySelectorAll('#list li.group')].slice(0, 3).map((li) => li.textContent).join(',')"))
	// The bar and the head count the new off the archive, as the shelf does.
	out = append(out, b.eval("[document.getElementById('news-n').textContent, document.getElementById('count').textContent, document.querySelector('.views [data-view=new] .n').textContent].join(' / ')"))
	// The words of the description are the title's alone, searched too.
	out = append(out, b.eval("document.querySelector('#list li[data-slug=v] .title').title"))
	b.eval("(() => { const q = document.getElementById('q'); q.value = 'the words'; q.dispatchEvent(new Event('input')); })()")
	out = append(out, b.eval("[...document.querySelectorAll('#list > li:not(.group)')].map((li) => li.dataset.slug).join('') + ' ' + location.search"))
	b.eval("(() => { const q = document.getElementById('q'); q.value = ''; q.dispatchEvent(new Event('input')); })()")

	// Nothing chosen at first, as opening one is reading it: the words how.
	const reader = "[document.getElementById('read-blank').hidden, document.getElementById('read-frame').hidden, new URL(document.getElementById('read-frame').src || 'http://x/none').pathname + new URL(document.getElementById('read-frame').src || 'http://x/').search, document.getElementById('read-title').textContent, document.querySelector('#list li.picked')?.dataset.slug].join(' ')"
	out = append(out, b.eval(reader))
	// Pressed, a row opens beside the list, the tab where it is; the title over it leads to it whole.
	b.eval("document.querySelector('#list li[data-slug=r] .title').click()")
	out = append(out, b.eval(reader), b.eval("location.pathname + ' ' + document.getElementById('read-title').getAttribute('href')"))
	// The bar over it: its star, the page alone and the same menu as a row's.
	out = append(out, b.eval("[...document.querySelectorAll('#read-actions .menu > *')].filter((e) => !e.hidden).length === [...document.querySelectorAll('#list li[data-slug=r] .menu > *')].filter((e) => !e.hidden).length"))
	// j and k from the row chosen, the list's redraw keeping the row with the focus.
	b.eval("document.querySelector('#list li[data-slug=r] .title').focus()")
	key("j")
	out = append(out, b.eval(reader))
	b.eval("drawn = ''; load()")
	b.until("the list drawn anew", "document.activeElement.textContent === 'Link'")
	key("k")
	key("ArrowUp")
	out = append(out, b.eval(reader)+" "+b.eval("document.activeElement.textContent"))
	// A link gone tells why beside the list.
	b.eval("document.querySelector('#list li[data-slug=g] .title').click()")
	out = append(out, b.eval("[document.getElementById('read-frame').hidden, document.getElementById('read-blank').textContent.includes('/tmp/x/g.html'), document.querySelector('#list li[data-slug=g] .miss') !== null].join(' ')"))
	// A page gone from the list closes.
	b.eval("document.querySelector('#list li[data-slug=n] .title').click()")
	b.eval("window.list = JSON.stringify(JSON.parse(window.list).filter((a) => a.slug !== 'n')); load()")
	b.until("the page to close", "!document.querySelector('#list li[data-slug=n]')")
	out = append(out, b.eval(reader))

	// The view of the new: a row read stays on it, as a ring, and j goes on from it; picked again, it goes.
	b.stub(listList)
	b.eval("document.querySelector('.views [data-view=new]').click()")
	out = append(out, b.eval("[...document.querySelectorAll('#list > li:not(.group)')].map((li) => li.dataset.slug).join('') + ' ' + location.search"))
	b.eval("window.list = JSON.stringify(JSON.parse(window.list).map((a) => a.slug === 'v' ? { ...a, news: undefined } : a)); load()")
	b.until("the row read", "document.querySelector('#list li[data-slug=v] .ring')")
	out = append(out, b.eval("[...document.querySelectorAll('#list > li:not(.group)')].map((li) => li.dataset.slug).join('')"),
		b.eval("[document.getElementById('news-n').textContent, document.querySelector('.views [data-view=new] .n').textContent].join(' / ')"))
	b.eval("document.querySelector('#list li[data-slug=v] .title').focus()")
	key("j")
	out = append(out, b.eval("document.activeElement.textContent"))
	b.eval("document.querySelector('.views [data-view=new]').click()")
	out = append(out, b.eval("[...document.querySelectorAll('#list > li:not(.group)')].map((li) => li.dataset.slug).join('')"))
	// None new: the list says so.
	b.eval("window.list = JSON.stringify(JSON.parse(window.list).map((a) => ({ ...a, news: undefined }))); load()")
	b.until("the rows read", "document.querySelector('.views [data-view=new] .n').textContent === '0'")
	b.eval("setView('new')")
	b.until("none new", "!document.getElementById('empty-new').hidden")
	out = append(out, b.eval("document.getElementById('news').hidden"))
	b.eval("setView('all')")

	// Whole: j and k go through the list that opened it, from the view of the new too, though read it is off that view now.
	b.stub(listList)
	b.eval("document.querySelector('.views [data-view=new]').click()")
	b.eval("document.querySelector('#list li[data-slug=v] .title').focus()")
	b.eval("document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))")
	b.until("the page whole", "location.pathname === '/a/v/'")
	out = append(out, b.eval("sessionStorage.getItem('artifacts.order')"), b.eval("document.querySelector('.brand').getAttribute('href')"))
	b.stub(strings.Replace(listList, "news: 'version', ", "", 1))
	b.until("the bar", "document.getElementById('frame-title').textContent")
	key("j")
	b.until("the next page", "location.pathname === '/a/n/'")
	key("k")
	b.until("the page before", "location.pathname === '/a/v/'")
	// From the URL, on no such list: the list it was last on, else all of its kind.
	b.eval("sessionStorage.setItem('artifacts.order', '[]'); sessionStorage.setItem('artifacts.list', '')")
	b.navigate("/a/r/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	key("j")
	b.until("the next page of all", "location.pathname === '/a/l/'")
	// An end stays where it is.
	b.navigate("/a/v/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	b.eval("sessionStorage.setItem('artifacts.order', '[\"v\"]')")
	b.navigate("/a/v/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	key("j")
	key("k")
	out = append(out, b.eval("location.pathname"))

	// One column: the views in a row with the dialog of the rest, a row opening its page whole, the bar's way back.
	b.unsize()
	b.navigate("/")
	b.size(400, 860, true)
	out = append(out, b.eval("[getComputedStyle(document.getElementById('reader')).display, getComputedStyle(document.getElementById('shelf-more')).display, getComputedStyle(document.getElementById('narrow-filter')).display, getComputedStyle(document.getElementById('news')).display, document.documentElement.scrollWidth <= innerWidth].join(' ')"))
	b.eval("document.querySelector('#list li[data-slug=r] .title').click()")
	b.until("the row whole", "location.pathname === '/a/r/'")
	out = append(out, b.eval("getComputedStyle(document.getElementById('back')).display"))
	b.unsize()

	check(t, "list", out, strings.Join([]string{
		"#|v dot 700 ver:v3 · new version kindof:design proj:p tags:t|#|n dot 700 kindof:report proj:q|#|r ring 500 kindof:report proj:p|l mark 500 kindof:Link · HTML path:/tmp/x/l.html|#|d mark 500 kindof:Link · handoff path:/tmp/x/d.yaml|#|f mark 500 kindof:Folder path:/tmp/x/f|#|g mark 500 kindof:Link · HTML miss:File not found",
		"Today,Yesterday,Last 7 days", "2 new / Newest first · 2 new / 2",
		"Version\nthe words", "v ?q=the+words",
		"false true /none  ", "true false /a/r/?raw Read r", "/ /a/r/", "true",
		"true false /a/l/?raw Link l", "true false /a/n/?raw New n New",
		"true true true", "false true blank New ",
		"vn ?view=new", "vn", "1 new / 1", "New", "n", "true",
		`["v","n"]`, "/?view=new", "/a/v/",
		"none none flex none true", "flex",
	}, "|"))
}
