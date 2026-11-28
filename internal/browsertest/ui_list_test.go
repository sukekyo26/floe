package browsertest_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"

	"floe/internal/store"
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
		{ slug: 'd', title: 'Data', description: '', path: '/home/you/work/a (1)/v2.0/data-set_#1/deep/deep/deep/deep/deep/deep/deep/deep/deep/name-2.yaml', template: 'handoff', createdAt: day(9), updatedAt: day(8) },
		{ slug: 'f', title: 'Folder', description: '', path: '/tmp/x/f', folder: true, createdAt: day(9), updatedAt: day(40) },
		{ slug: 'g', title: 'Gone', description: '', path: '/tmp/x/g.html', missing: true, createdAt: day(400), updatedAt: day(400) },
		{ slug: 'a', title: 'Kept', description: '', news: 'new', version: 1, archived: true, createdAt: day(9), updatedAt: day(9) },
		{ slug: 'b', title: 'Kept too', description: '', version: 1, archived: true, createdAt: day(9), updatedAt: day(10) },
	];
})()`

// What a row shows, in order, for each row of the list: its first column, its time's being there, its second line's parts.
const rowsDrawn = `[...document.querySelectorAll('#list > li')].map((li) => {
	if (li.dataset.testid === 'group') return '#';
	const lead = li.querySelector('[data-testid=lead]');
	const mark = lead.querySelector('[data-testid=dot]') ? 'dot' : lead.querySelector('[data-testid=ring]') ? 'ring' : lead.querySelector('svg') ? 'mark' : '?';
	return [li.dataset.slug, mark, getComputedStyle(li.querySelector('[data-testid=title]')).fontWeight, ...[...li.querySelector('[data-testid=sub]').children].map((e) => e.dataset.testid + ':' + e.textContent)].join(' ');
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
		b.moving("document.body.dispatchEvent(new KeyboardEvent('keydown', { key: '" + k + "', bubbles: true }))")
	}
	out := make([]string, 0, 24)
	out = append(out, b.eval(rowsDrawn))
	out = append(out, b.eval("[...document.querySelectorAll('#list li[data-testid=group]')].slice(0, 3).map((li) => li.textContent).join(',')"))
	// The bar and the head count the new off the archive, as the shelf does.
	const freshN = "document.querySelector('[data-testid=views] [data-view=new] [data-testid=n]').textContent"
	out = append(out, b.eval("[document.getElementById('news-n').textContent, document.getElementById('count').textContent, "+freshN+"].join(' / ')"))
	// The words of the description are the title's alone, searched too.
	out = append(out, b.eval("document.querySelector('#list li[data-slug=v] [data-testid=title]').title.split('\\n').slice(0, 3).join('\\n')"))
	b.fill("#q", "the words")
	out = append(out, b.eval("[...document.querySelectorAll('#list > li:not([data-testid=group])')].map((li) => li.dataset.slug).join('') + ' ' + location.search"))
	b.fill("#q", "")

	// Nothing chosen at first, as opening one is reading it: the words how.
	const closed = "[document.getElementById('read-blank').hidden, document.getElementById('read-frame').hidden, document.getElementById('read-frame').dataset.loaded, document.getElementById('read-head').hidden, document.querySelector('#list li[aria-current=true]')?.dataset.slug].join(' ')"
	const reader = "[document.getElementById('read-blank').hidden, document.getElementById('read-frame').hidden, document.getElementById('read-frame').dataset.loaded, document.getElementById('read-title').textContent, document.querySelector('#list li[aria-current=true]')?.dataset.slug].join(' ')"
	out = append(out, b.eval(reader))
	// Pressed, a row opens beside the list, the tab where it is; the title over it leads to it whole.
	b.eval("document.querySelector('#list li[data-slug=r] [data-testid=title]').click()")
	out = append(out, b.eval(reader), b.eval("location.pathname + ' ' + document.getElementById('read-title').getAttribute('href')"))
	// The bar over it: its star, the page alone and the same menu as a row's.
	const items = topsJS + "(m).filter((e) => !e.hidden && e.dataset.testid !== 'select').length"
	out = append(out, b.eval("((bar, row) => bar > 0 && bar === row)("+inMenu("#read-actions", items)+", "+inMenu("#list li[data-slug=r]", items)+")"))
	// j and k from the row chosen, the list's redraw keeping the row with the focus: another row's title changed, so that it is drawn anew.
	b.eval("document.querySelector('#list li[data-slug=r] [data-testid=title]').focus()")
	key("j")
	out = append(out, b.eval(reader))
	b.eval("window.list = JSON.stringify(JSON.parse(window.list).map((a) => a.slug === 'g' ? { ...a, title: 'Gone too' } : a))")
	b.reread()
	b.until("the list drawn anew", "document.querySelector('#list li[data-slug=g] [data-testid=title]').textContent === 'Gone too' && document.activeElement.textContent === 'Link'")
	key("k")
	key("ArrowUp")
	out = append(out, b.eval(reader)+" "+b.eval("document.activeElement.textContent"))
	// A link gone tells why beside the list.
	b.eval("document.querySelector('#list li[data-slug=g] [data-testid=title]').click()")
	out = append(out, b.eval("[document.getElementById('read-frame').hidden, document.getElementById('read-blank').textContent.includes('/tmp/x/g.html'), document.querySelector('#list li[data-slug=g] [data-testid=miss]') !== null].join(' ')"))
	// A page gone from the list closes.
	b.eval("document.querySelector('#list li[data-slug=n] [data-testid=title]').click()")
	b.eval("window.list = JSON.stringify(JSON.parse(window.list).filter((a) => a.slug !== 'n'))")
	b.reread()
	b.until("the page to close", "!document.querySelector('#list li[data-slug=n]')")
	// Closed, the head is hidden: the words a build may leave in it are not read
	out = append(out, b.eval(closed))

	// The view of the new: a row read stays on it, as a ring, and j goes on from it; the view let go, it is gone on the way back.
	const fresh = "document.querySelector('[data-testid=views] [data-view=new]')"
	b.stub(listList)
	b.eval(fresh + ".click()")
	out = append(out, b.eval("[...document.querySelectorAll('#list > li:not([data-testid=group])')].map((li) => li.dataset.slug).join('') + ' ' + location.search"))
	b.eval("window.list = JSON.stringify(JSON.parse(window.list).map((a) => a.slug === 'v' ? { ...a, news: undefined } : a))")
	b.reread()
	b.until("the row read", "document.querySelector('#list li[data-slug=v] [data-testid=ring]')")
	out = append(out, b.eval("[...document.querySelectorAll('#list > li:not([data-testid=group])')].map((li) => li.dataset.slug).join('')"),
		b.eval("[document.getElementById('news-n').textContent, "+freshN+"].join(' / ')"))
	b.eval("document.querySelector('#list li[data-slug=v] [data-testid=title]').focus()")
	key("j")
	out = append(out, b.eval("document.activeElement.textContent"))
	b.pressed(fresh+".click()", "the new pages' view left", fresh+".getAttribute('aria-pressed') === 'false'")
	b.eval(fresh + ".click()")
	out = append(out, b.eval("[...document.querySelectorAll('#list > li:not([data-testid=group])')].map((li) => li.dataset.slug).join('')"))
	// None new: the list says so.
	b.eval("window.list = JSON.stringify(JSON.parse(window.list).map((a) => ({ ...a, news: undefined })))")
	b.reread()
	b.until("the rows read", freshN+" === '0'")
	// The view let go and taken again, which leaves out the rows read on it.
	b.pressed(fresh+".click()", "the new pages' view left", fresh+".getAttribute('aria-pressed') === 'false'")
	b.eval(fresh + ".click()")
	b.until("none new", "!document.getElementById('empty-new').hidden")
	out = append(out, b.eval("document.getElementById('news').hidden"))
	b.eval("document.querySelector('[data-testid=views] [data-view=all]').click()")

	// Whole: j and k go through the list that opened it, from the view of the new too, though read it is off that view now.
	b.stub(listList)
	b.eval(fresh + ".click()")
	b.eval("document.querySelector('#list li[data-slug=v] [data-testid=title]').focus()")
	b.moving("document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))")
	// The bar too: the logo's link is drawn with it, once the list is read.
	b.until("the page whole", "location.pathname === '/a/v/' && document.getElementById('frame-title').textContent")
	out = append(out, b.eval("sessionStorage.getItem('pages.order')"), b.eval("document.querySelector('[data-testid=brand]').getAttribute('href')"))
	b.stub(strings.Replace(listList, "news: 'version', ", "", 1))
	b.until("the bar", "document.getElementById('frame-title').textContent")
	key("j")
	// The bar, not the URL alone: a key goes nowhere until the list is read.
	b.until("the next page", "location.pathname === '/a/n/' && document.getElementById('frame-title').textContent")
	key("k")
	b.until("the page before", "location.pathname === '/a/v/'")
	// From the URL, on no such list: the list it was last on, else all of its kind.
	b.eval(`sessionStorage.setItem('pages.order', '{"order":[],"shown":[],"back":"/"}'); sessionStorage.setItem('pages.list', '')`)
	b.navigate("/a/r/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	key("j")
	b.until("the next page of all", "location.pathname === '/a/l/'")
	// An end stays where it is.
	b.navigate("/a/v/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	b.eval(`sessionStorage.setItem('pages.order', '{"order":["v"],"shown":["v"],"back":"/"}')`)
	b.navigate("/a/v/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	key("j")
	key("k")
	out = append(out, b.eval("location.pathname"))

	// The path is cut at its head, the file's name whole at the box's end and its own order kept, marks and numbers too.
	b.navigate("/")
	out = append(out, b.eval(`(() => {
		const box = document.querySelector('#list li[data-slug=d] [data-testid=path]'), text = box.querySelector('bdi').firstChild;
		const at = (from, to) => { const r = document.createRange(); r.setStart(text, from); r.setEnd(text, to); return r.getBoundingClientRect(); };
		const n = text.length, b = box.getBoundingClientRect();
		return [box.scrollWidth > box.clientWidth, Math.abs(at(n - 1, n).right - b.right) <= 1, at(0, 1).left < b.left, at(n - 11, n - 10).left < at(n - 1, n).left,
			// Pointed at, the row's title tells it whole.
			document.elementFromPoint(b.left + b.width / 2, b.top + b.height / 2).closest('a[data-testid=title]').title.split('\n').includes(text.data)].join(' ');
	})()`))
	// Beside the list above 56rem, the shelf a row under 70rem, the filter right after the views, the bar's count gone.
	b.size(1000, 700, false)
	const middle = "[getComputedStyle(document.getElementById('reader')).display, getComputedStyle(document.getElementById('shelf-more')).display, getComputedStyle(document.getElementById('news')).display, " +
		"document.getElementById('narrow-filter').getBoundingClientRect().left - document.querySelector('[data-testid=views] [data-view=archived]').getBoundingClientRect().right < 10].join(' ')"
	out = append(out, b.eval(middle))
	b.eval("document.querySelector('#list li[data-slug=r] [data-testid=title]').click()")
	out = append(out, b.eval("[location.pathname, document.getElementById('read-frame').hidden].join(' ')"))
	// At 56rem or less, one column, a row opening whole with the way back, the page's buttons and the search's field still there.
	b.size(800, 700, false)
	out = append(out, b.eval("[getComputedStyle(document.getElementById('reader')).display, getComputedStyle(document.querySelector('#q')).color !== 'rgba(0, 0, 0, 0)'].join(' ')"))
	b.moving("document.querySelector('#list li[data-slug=r] [data-testid=title]').click()")
	b.until("the row whole", "location.pathname === '/a/r/'")
	out = append(out, b.until("the bar", "document.getElementById('frame-title').textContent && [("+wayBack+")?.id, getComputedStyle(document.getElementById('frame-actions')).display].join(' ')"))
	b.unsize()

	// A dialog's words are a paragraph, not a row's first column: above the next field, however long.
	b.navigate("/")
	b.size(400, 860, true)
	b.eval("document.getElementById('add-link').click()")
	out = append(out, b.eval("(() => { const l = document.querySelector('#add [data-testid=lead]').getBoundingClientRect(), f = document.querySelector('#add [data-testid=field]').getBoundingClientRect(); const e = document.querySelector('#add [data-testid=lead]'); return [l.bottom <= f.top && e.scrollHeight <= e.clientHeight + 1, getComputedStyle(e).display].join(' '); })()"))
	b.eval("document.getElementById('add-cancel').click()")
	// In the dialog of the rest of the shelf, its headings are the shelf's, under the dialog's own.
	b.eval("document.getElementById('narrow-filter').click()")
	out = append(out, b.eval("parseFloat(getComputedStyle(document.querySelector('#shelf-kinds h2')).fontSize) < parseFloat(getComputedStyle(document.getElementById('filter-title')).fontSize)"))
	b.eval("document.querySelector('#filter-dialog [data-close]').click()")
	b.unsize()

	// One column: the views in a row with the dialog of the rest, a row opening its page whole, the bar's way back.
	b.unsize()
	b.navigate("/")
	b.size(400, 860, true)
	out = append(out, b.eval("[getComputedStyle(document.getElementById('reader')).display, getComputedStyle(document.getElementById('shelf-more')).display, getComputedStyle(document.getElementById('narrow-filter')).display, getComputedStyle(document.getElementById('news')).display, document.documentElement.scrollWidth <= innerWidth].join(' ')"))
	b.moving("document.querySelector('#list li[data-slug=r] [data-testid=title]').click()")
	b.until("the row whole", "location.pathname === '/a/r/'")
	out = append(out, b.until("the bar", "document.getElementById('frame-title').textContent && [("+wayBack+")?.id, getComputedStyle(document.querySelector('#frame-actions [data-testid=star]')).display].join(' ')"))
	b.unsize()

	check(t, "list", out, strings.Join([]string{
		"#|v dot 700 ver:v3 · new version kindof:design proj:p tags:t|#|n dot 700 kindof:report proj:q|#|r ring 500 kindof:report proj:p|l mark 500 kindof:Link · HTML path:/tmp/x/l.html|#|d mark 500 kindof:Link · handoff path:/home/you/work/a (1)/v2.0/data-set_#1/deep/deep/deep/deep/deep/deep/deep/deep/deep/name-2.yaml|#|f mark 500 kindof:Folder path:/tmp/x/f|#|g mark 500 kindof:Link · HTML miss:File not found",
		"Today,Yesterday,Last 7 days", "All unread 2 / Newest first · 2 unread / 2",
		"Version\nthe words\np", "v ?q=the+words",
		"false true about:blank  ", "true false /a/r/?raw Read r", "/ /a/r/", "true",
		"true false /a/l/?raw Link l", "true false /a/n/?raw New n New",
		"true true true", "false true about:blank true ",
		"vn ?view=new", "vn", "All unread 1 / 1", "New", "n", "true",
		`{"order":["v","n"],"shown":["v","n"],"back":"/?view=new","at":{"slug":"v","file":"","v":0}}`, "/?view=new", "/a/v/",
		"true true true true true", "flex none none true", "/ false", "none true", "back flex", "true block", "true",
		"none none flex none true", "back none",
	}, "|"))
}

// asideSite is a server with pages of its own, for the iframe beside the list.
func asideSite(t *testing.T) *site {
	t.Helper()

	s := newSite(t)
	for _, slug := range []string{"a1", "a2", "a3"} {
		s.floe(t, "publish", s.page(t, "aside/"+slug+".yaml", slug, "<p id=\"p\">"+slug+"</p>"), "--slug", slug)
	}
	return s
}

// TestUIViewLit: All is lit with no view and no picks, whatever the search; a pick puts it out. Unlit, All lets go of the view
// and the picks, lit it does nothing; a view lit lets go of itself alone. A reload lights the same, and two columns do the same.
func TestUIViewLit(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.stub(`[{ slug: 'a', title: 'A', description: '', news: 'new', project: 'floe', template: 'report', tags: ['x'], createdAt: at(1), updatedAt: at(9) },
		{ slug: 'b', title: 'B', description: '', project: 'floe', createdAt: at(1), updatedAt: at(8) },
		{ slug: 'c', title: 'C', description: '', news: 'new', project: 'p', createdAt: at(1), updatedAt: at(7) },
		{ slug: 'd', title: 'D', description: '', project: 'p', favorite: true, createdAt: at(1), updatedAt: at(6) },
		{ slug: 'e', title: 'E', description: '', project: 'floe', archived: true, createdAt: at(1), updatedAt: at(5) }]`)
	b.navigate("/")
	b.size(1440, 900, false)
	// What is lit, the rows, the URL and the search.
	const state = `[[...document.querySelectorAll('[data-testid=views] button[aria-pressed=true], #shelf-more [data-testid=chip][aria-pressed=true]')].map((c) => c.dataset.view ?? c.dataset.project ?? c.dataset.kind ?? c.dataset.template ?? c.dataset.tag).join('+'),
		[...document.querySelectorAll('#list > li:not([data-testid=group])')].map((li) => li.dataset.slug).sort().join(''), location.search, document.getElementById('q').value].join(' | ')`
	press := func(selector string) string {
		b.eval("document.querySelector('" + selector + "').click()")
		return b.eval(state)
	}
	const (
		all   = "[data-testid=views] [data-view=all]"
		fresh = "[data-testid=views] [data-view=new]"
		floe  = "#chips [data-project=floe]"
	)
	want := make([]string, 0, 16)
	out := make([]string, 0, 16)
	step := func(got, expect string) {
		out = append(out, got)
		want = append(want, expect)
	}
	step(b.eval(state), "all | abcd |  | ")
	step(press(floe), "floe | ab | ?project=floe | ")
	// Unlit, All lets go of the picks.
	step(press(all), "all | abcd |  | ")
	// A view and a project narrow together; the view lit lets go of itself alone.
	press(fresh)
	step(press(floe), "new+floe | a | ?view=new&project=floe | ")
	b.navigate("/?view=new&project=floe")
	step(b.eval(state), "new+floe | a | ?view=new&project=floe | ")
	step(press(fresh), "floe | ab | ?project=floe | ")
	press("[data-testid=views] [data-view=fav]")
	step(press("[data-testid=views] [data-view=fav]"), "floe | ab | ?project=floe | ")
	press("[data-testid=views] [data-view=archived]")
	step(press("[data-testid=views] [data-view=archived]"), "floe | ab | ?project=floe | ")
	// Unlit, All lets go of a view and every kind of pick, and a reload lights it.
	b.navigate("/?view=fav&project=p&kind=page")
	step(press(all), "all | abcd |  | ")
	b.navigate("/?template=report&tag=x")
	step(b.eval(state), "report+x | a | ?template=report&tag=x | ")
	step(press(all), "all | abcd |  | ")
	b.navigate("/")
	step(b.eval(state), "all | abcd |  | ")
	// The search stays, and All stays lit with it.
	b.fill("#q", "A")
	step(b.eval(state), "all | a | ?q=A | A")
	press(floe)
	step(press(all), "all | a | ?q=A | A")
	// Lit, All does nothing: not even to the selection.
	const chosen = "document.querySelector('#list li[data-slug=a] [data-testid=sel]').checked && document.getElementById('list').hasAttribute('data-selecting')"
	b.eval("document.querySelector('#list li[data-slug=a] [data-testid=sel]').click()")
	b.until("the row selected", chosen)
	step(b.eval("document.querySelector('"+all+"').click(); "+chosen), "true")
	// The way out of no match keeps the view, and clears the search and the picks.
	b.navigate("/?view=fav&project=p&q=none")
	step(press("#clear"), "fav | d | ?view=fav | ")

	// Two columns: the picks are in a dialog, and the button to it, with their number, is lit for them.
	b.navigate("/?project=floe&tag=x")
	b.size(900, 800, false)
	const narrow = `[...document.querySelectorAll('[data-testid=views] button')].filter((b) => b.getAttribute('aria-pressed') === 'true').map((b) => b.dataset.view).join('+') + ' | ' +
		document.getElementById('narrow-filter').hasAttribute('data-narrowed') + ' | ' + document.getElementById('narrow-filter-name').textContent`
	step(b.eval(narrow), " | true | Filter 2")
	b.eval("document.querySelector('" + all + "').click()")
	step(b.eval(narrow), "all | false | Filter")
	b.eval("document.querySelector('" + fresh + "').click()")
	b.eval("document.getElementById('narrow-filter').click(); document.querySelector('" + floe + "').click()")
	b.pressKey(kb.Escape)
	b.until("the filters shut", "!document.getElementById('filter-dialog').open")
	step(b.eval(narrow), "new | true | Filter 1")
	b.eval("document.querySelector('" + fresh + "').click()")
	step(b.eval(narrow), " | true | Filter 1")
	b.eval("document.querySelector('" + all + "').click()")
	step(b.eval(narrow), "all | false | Filter")
	check(t, "lit", out, strings.Join(want, "|"))
}

// TestUIShelfMarks: the words of the views, the projects and the kinds start at one x, whichever mark they have, and the row of the views
// on a narrow screen holds the button to the filters, on every width, with the longest view picked.
func TestUIShelfMarks(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.stub(`[{ slug: 'a', title: 'A', description: '', news: 'new', project: 'floe', createdAt: at(1), updatedAt: at(9) },
		{ slug: 'b', title: 'B', description: '', project: 'p', favorite: true, createdAt: at(1), updatedAt: at(8) },
		{ slug: 'l', title: 'L', description: '', path: '/tmp/l.html', createdAt: at(1), updatedAt: at(7) },
		{ slug: 'f', title: 'F', description: '', path: '/tmp/f', folder: true, createdAt: at(1), updatedAt: at(6) }]`)
	b.navigate("/")
	b.size(1440, 900, false)
	out := make([]string, 0, 16)
	out = append(out, b.eval(`(() => {
		const lefts = [...document.querySelectorAll('[data-testid=views] [data-testid=label], #chips [data-testid=name], #kinds [data-testid=name]')].map((e) => e.getBoundingClientRect().left);
		return lefts.length + ' ' + (Math.max(...lefts) - Math.min(...lefts) <= 0.5);
	})()`))
	// The archive's mark and the kinds' are quiet, not the star's color.
	out = append(out, b.eval(`[...document.querySelectorAll('[data-testid=views] [data-view=archived] svg, #kinds svg')].map((e) => getComputedStyle(e).color === getComputedStyle(document.querySelector('[data-testid=views] [data-testid=n]')).color).join(' ')`))
	// A link and a folder have the mark of their rows, and a page its own.
	out = append(out, b.eval(`[...document.querySelectorAll('#kinds [data-testid=chip]')].map((c) => {
		const path = c.querySelector('path').getAttribute('d'), row = document.querySelector('#list li[data-slug=' + { link: 'l', folder: 'f' }[c.dataset.kind] + '] [data-testid=lead] path');
		return c.dataset.kind + ':' + (row ? row.getAttribute('d') === path : Boolean(path));
	}).join(' ')`))
	// The row holds all, its button to the filters within the window.
	for _, w := range []int64{1000, 640, 400, 360} {
		b.size(w, 700, false)
		for _, at := range []string{"/?view=archived&project=floe", "/?view=new&project=floe"} {
			b.navigate(at)
			out = append(out, b.eval(`(() => {
				const v = document.querySelector('[data-testid=views]');
				return v.scrollWidth <= v.clientWidth && document.getElementById('narrow-filter').getBoundingClientRect().right <= innerWidth;
			})()`))
		}
	}
	// The names as read out: the words hidden on the narrow are still read, and the count where it shows.
	named := func() string {
		tree, err := chromedp.Call(b.ctx, accessibility.GetFullAXTree, accessibility.GetFullAXTreeParams{})
		if err != nil {
			t.Fatal(err)
		}
		names := make([]string, 0, 4)
		for _, n := range tree.Nodes {
			if n.Name != nil && !n.Ignored && n.Role != nil && strings.Contains(string(n.Role.Value), "button") {
				var name string
				if err = json.Unmarshal(n.Name.Value, &name); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(name, "Favorites") || strings.Contains(name, "Archive") {
					names = append(names, name)
				}
			}
		}
		return strings.Join(names, ",")
	}
	out = append(out, b.eval("document.querySelector('[data-testid=views] [data-view=archived] [data-testid=label]').getBoundingClientRect().width <= 1"), named())
	b.size(1000, 700, false)
	b.navigate("/")
	out = append(out, named())
	b.size(1440, 900, false)
	b.navigate("/")
	out = append(out, named())
	check(t, "shelf marks", out, "10 true|true true true true|page:true link:true folder:true|true|true|true|true|true|true|true|true|true|Favorites 0,Archive,Archive A|Favorites 1,Archive,Archive A,Archive B,Archive L,Archive F|Favorites 1,Archive 0,Archive A,Archive B,Archive L,Archive F")
}

// TestUIShelfCounts: each part of the shelf counts what the other parts, the view and the search leave, its own part let go; the
// views take the picks and the search, All the search alone; a row at 0 stays where it is; the head says what the view of the unread does, and the bar counts all there is.
func TestUIShelfCounts(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.stub(`[{ slug: 'a', title: 'Alpha', description: '', news: 'new', project: 'floe', template: 'report', tags: ['x'], createdAt: at(1), updatedAt: at(9) },
		{ slug: 'b', title: 'Beta', description: '', path: '/tmp/b.html', project: 'floe', template: 'design', createdAt: at(1), updatedAt: at(8) },
		{ slug: 'c', title: 'Alpha two', description: '', news: 'new', project: 'p', template: 'report', favorite: true, createdAt: at(1), updatedAt: at(7) },
		{ slug: 'd', title: 'Delta', description: '', path: '/tmp/d.html', project: 'p', template: 'design', createdAt: at(1), updatedAt: at(6) },
		{ slug: 'e', title: 'Eta', description: '', project: 'floe', template: 'report', archived: true, createdAt: at(1), updatedAt: at(5) },
		{ slug: 'g', title: 'Gamma', description: '', path: '/tmp/g', folder: true, project: 'p', archived: true, createdAt: at(1), updatedAt: at(4) }]`)
	b.size(1440, 900, false)
	// The views, the projects, the kinds, the templates and the tags, as they stand, each with its count.
	const counts = `['[data-testid=views] button[data-view]', '#chips [data-testid=chip]', '#kinds [data-testid=chip]', '#templates [data-testid=chip]', '#tags [data-testid=chip]'].map((part) => [...document.querySelectorAll(part)]
		.map((c) => (c.dataset.view ?? c.dataset.project ?? c.dataset.kind ?? c.dataset.template ?? c.dataset.tag) + ':' + c.querySelector('[data-testid=n]').textContent).join(' ')).join(' / ')`
	at := func(url string) string {
		b.navigate(url)
		return b.eval(counts)
	}
	out := make([]string, 0, 12)
	out = append(out, at("/"))
	// Of floe: the kinds, the templates, the tags and the views narrow; the projects do not; All does not.
	out = append(out, at("/?project=floe"))
	// The view counts too, on its own rows: of the archive, a row of no entry of floe at 0.
	out = append(out, at("/?view=archived&project=floe"))
	// The search: rows at 0 stay in their order, of the view's numbers, and All takes it too.
	out = append(out, at("/?q=alpha"))
	// All is what its press leaves: the search, not the picks or the view.
	out = append(out, at("/?view=fav&kind=link&q=alpha"))
	// A kind narrows the projects, as a project does the kinds.
	out = append(out, at("/?kind=link"))
	// The bar counts all there is; the head says what the view of the unread does.
	const unread = "document.getElementById('news-n').textContent + ' / ' + document.getElementById('count').textContent + ' / ' + document.querySelector('[data-testid=views] [data-view=new] [data-testid=n]').textContent"
	out = append(out, b.eval(unread))
	b.navigate("/?project=floe")
	out = append(out, b.eval(unread))
	// The bar pressed lets go of the picks and the search: as many rows as all three say, and the view stays on pressed again.
	b.navigate("/?view=fav&project=floe&kind=page&template=report&tag=x&q=alpha")
	b.eval("document.getElementById('news').click()")
	const led = "[...document.querySelectorAll('#list > li:not([data-testid=group])')].length + ' ' + " + unread + " + ' ' + location.search + ' [' + document.getElementById('q').value + ']'"
	out = append(out, b.eval(led))
	b.eval("document.getElementById('news').click()")
	out = append(out, b.eval(led))
	// A read row that stays on the view of the unread is not counted.
	b.navigate("/?view=new")
	b.eval("window.list = JSON.stringify(JSON.parse(window.list).map((x) => x.slug === 'a' ? { ...x, news: undefined } : x))")
	b.reread()
	b.until("the row read", "document.querySelector('#list li[data-slug=a]:not([data-unread])')")
	out = append(out, b.eval("[...document.querySelectorAll('#list > li:not([data-testid=group])')].length + ' ' + document.querySelector('[data-testid=views] [data-view=new] [data-testid=n]').textContent"))
	// The bar pressed there: as many rows as it says, the read row gone.
	b.eval("document.getElementById('news').click()")
	out = append(out, b.eval("[...document.querySelectorAll('#list > li:not([data-testid=group])')].length + ' ' + "+unread+" + ' ' + location.search"))
	b.eval("window.list = JSON.stringify(JSON.parse(window.list).map((x) => x.slug === 'a' ? { ...x, news: 'new' } : x))")
	b.reread()
	b.until("the row unread", "document.querySelector('#list li[data-slug=a][data-unread=true]')")
	// A row at 0 is quieter, but for the one picked, which reads as picked.
	b.navigate("/?view=archived&project=floe&kind=folder")
	out = append(out, b.eval(`(() => {
		const name = (c) => getComputedStyle(c.querySelector('[data-testid=name]')).color, quiet = getComputedStyle(document.querySelector('[data-testid=views] [data-testid=n]')).color;
		const folder = document.querySelector('#kinds [data-kind=folder]');
		return [folder.getAttribute('aria-pressed'), folder.querySelector('[data-testid=n]').textContent, name(folder) === quiet].join(' ');
	})()`))
	b.navigate("/?q=alpha")
	out = append(out, b.eval(`(() => {
		const link = document.querySelector('#kinds [data-kind=link]'), page = document.querySelector('#kinds [data-kind=page]'), quiet = getComputedStyle(document.querySelector('[data-testid=views] [data-testid=n]')).color;
		const words = (e, s) => getComputedStyle(e.querySelector(s)).color === quiet;
		return [link.querySelector('[data-testid=n]').textContent, words(link, '[data-testid=name]'), words(page, '[data-testid=name]'), words(document.querySelector('[data-testid=views] [data-view=archived]'), '[data-testid=label]')].join(' ');
	})()`))
	// In the dialog of two columns, the same.
	b.size(900, 800, false)
	b.navigate("/?project=floe")
	b.eval("document.getElementById('narrow-filter').click()")
	out = append(out, b.eval("document.querySelector('#filter-box #kinds [data-kind=link] [data-testid=n]').textContent + ' ' + document.querySelector('#filter-box #chips [data-project=p] [data-testid=n]').textContent"))
	check(t, "shelf counts", out, strings.Join([]string{
		"new:2 all:4 fav:1 archived:2 / floe:2 p:2 / page:2 link:2 / design:2 report:2 / x:1",
		"new:1 all:4 fav:0 archived:1 / floe:2 p:2 / page:1 link:1 / design:1 report:1 / x:1",
		"new:1 all:4 fav:0 archived:1 / floe:1 p:1 / page:1 folder:0 / report:1 / ",
		"new:2 all:2 fav:1 archived:0 / floe:1 p:1 / page:2 link:0 / design:0 report:2 / x:1",
		"new:0 all:2 fav:0 archived:0 / p:0 / page:1 link:0 / report:0 / ",
		"new:0 all:4 fav:0 archived:0 / floe:1 p:1 / page:2 link:2 / design:2 report:0 / x:0",
		"All unread 2 / Newest first · 0 unread / 0",
		"All unread 2 / Newest first · 1 unread / 1",
		"2 All unread 2 / Newest first · 2 unread / 2 ?view=new []",
		"2 All unread 2 / Newest first · 2 unread / 2 ?view=new []",
		"2 1",
		"1 All unread 1 / Newest first · 1 unread / 1 ?view=new",
		"true 0 false",
		"0 true false true",
		"1 2",
	}, "|"))
}

// floeRead is reads.json of the site's store.
func (s *site) floeRead(t *testing.T) string {
	t.Helper()

	b, err := os.ReadFile(filepath.Join(s.Dir, "store", "reads.json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestUIReader keeps the iframe beside the list with the rows: going through them adds nothing to the tab's history, a link's
// missing file is not loaded and is once it is back, what the edit dialog was opened from has the focus again, and one column
// leaves nothing alive to be told read.
func TestUIReader(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := asideSite(t)
	b := s.b
	b.navigate("/")
	b.size(1440, 900, false)
	b.until("the rows", "document.querySelectorAll('#list li[data-slug]').length === 3")
	press := func(slug string) {
		t.Helper()

		b.eval("document.querySelector('#list li[data-slug=" + slug + "] [data-testid=title]').click()")
	}
	// The menu's item edit, of the bar over the page or of a row, pressed as a person does: the menu opened first.
	edit := func(scope string) {
		t.Helper()

		b.eval("document.querySelector('" + scope + " summary').click()")
		item := "document.querySelector('" + scope + " [data-testid=edit]')"
		b.until("the menu open", item+".checkVisibility()")
		b.eval(item + ".click()")
		b.until("the edit dialog", "document.getElementById('edit-dialog').open")
	}
	loaded := func(what, path string) {
		t.Helper()

		b.until(what, "window.heard.includes("+js(path)+")")
	}
	// A page in the iframe tells the path it is at as it loads.
	b.eval("window.heard = []; addEventListener('message', (e) => { if (e.data?.pagePath) window.heard.push(e.data.pagePath); })")
	out := make([]string, 0, 8)
	before := b.eval("history.length")
	for _, slug := range []string{"a1", "a2", "a3"} {
		press(slug)
		loaded("the page "+slug, "/a/"+slug+"/")
	}
	out = append(out, b.eval("history.length === "+before), b.eval("document.querySelector('#list li[aria-current=true]').dataset.slug"))

	// Opened from the bar over the page, the edit dialog gives the focus back there; from a row, to the row's.
	edit("#read-actions")
	b.eval("document.querySelector('#edit-dialog [data-close]').click()")
	out = append(out, b.until("the focus back", "document.activeElement.matches('#read-actions summary') && 'bar'"))
	// The list read again, as after a read, draws the bar anew with the focus where it was: another row's title changed, so that
	// it is drawn anew.
	title := "a1 renamed"
	if _, err := store.New(s.Env["FLOE_DIR"]).Edit("a1", store.Edit{Title: &title}); err != nil {
		t.Fatal(err)
	}
	b.reread()
	b.until("the list drawn anew", "document.querySelector('#list li[data-slug=a1] [data-testid=title]').textContent === 'a1 renamed'")
	if got := b.eval("document.activeElement.matches('#read-actions summary')"); got != "true" {
		t.Errorf("the focus after the bar was drawn anew: %s", got)
	}
	edit("#list li[data-slug=a1]")
	b.eval("document.querySelector('#edit-dialog [data-close]').click()")
	out = append(out, b.until("the focus back", "document.activeElement.matches('#list li[data-slug=a1] summary') && 'row'"))

	// Renamed from its row, the focus goes back to the row under its new name.
	edit("#list li[data-slug=a2]")
	b.eval("document.querySelector('#edit-more > summary').click()")
	b.until("the slug's field", "document.getElementById('edit-slug').checkVisibility()")
	b.fill("#edit-slug", "a4")
	b.eval("document.getElementById('edit-submit').click()")
	out = append(out, b.until("the focus on the row renamed", "document.activeElement.matches('#list li[data-slug=a4] summary') && 'renamed'"))

	// A link's file gone is not loaded beside the list, and is once it is back.
	file := s.write(t, "aside/h.html", "<title>h</title><p>h</p>")
	s.floe(t, "publish", "--link", file, "--slug", "h")
	b.until("the link", "document.querySelector('#list li[data-slug=h]')")
	if err := os.Rename(file, file+".moved"); err != nil {
		t.Fatal(err)
	}
	b.until("the file gone", "document.querySelector('#list li[data-slug=h] [data-testid=miss]')")
	press("h")
	out = append(out, b.eval("[document.getElementById('read-frame').hidden, document.getElementById('read-frame').dataset.loaded].join(' ')"))
	if err := os.Rename(file+".moved", file); err != nil {
		t.Fatal(err)
	}
	b.until("the file back", "!document.querySelector('#list li[data-slug=h] [data-testid=miss]')")
	loaded("the page of the file back", "/a/h/")
	out = append(out, b.eval("document.getElementById('read-frame').hidden"))

	// One column: nothing stays alive beside the list. A new version is not told read, nor is anything chosen back.
	b.eval("window.heard = []")
	press("a1")
	loaded("the page a1", "/a/a1/")
	b.size(800, 700, false)
	b.until("one column", "document.querySelector('#list li[aria-current=true]') === null")
	out = append(out, b.eval("[document.getElementById('read-frame').dataset.slug, document.getElementById('read-frame').dataset.loaded].join(' ')"))
	before = s.floeRead(t)
	s.floe(t, "publish", s.page(t, "aside/a1.yaml", "a1", "<p id=\"p\">a1 again</p>"), "--slug", "a1")
	b.until("the new version", "document.querySelector('#list li[data-slug=a1] [data-testid=ver]')")
	out = append(out, strconv.FormatBool(s.floeRead(t) == before))
	b.unsize()
	check(t, "reader", out, "true|a3|bar|row|renamed|true about:blank|false| about:blank|true")
}

// TestUIFrameOrder goes through a list by j in the frame: the list that opened it, a page gone since left out; opened from the
// URL, the list it was last on, else all of its kind, archived or not as it is; at an end it stays.
func TestUIFrameOrder(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.stub(listList)
	go1 := func(from, order, list, want string) {
		t.Helper()

		b.navigate("/")
		b.eval("sessionStorage.setItem('pages.order', " + js(`{"order":`+order+`,"shown":`+order+`,"back":"/"}`) + "); sessionStorage.setItem('pages.list', " + js(list) + ")")
		b.navigate("/a/" + from + "/")
		b.until("the bar", "document.getElementById('frame-title').textContent")
		b.moving("document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'j', bubbles: true }))")
		b.until(from+" to go to "+want, "location.pathname === "+js("/a/"+want+"/"))
	}
	// The list that opened it, a slug no entry has left out.
	go1("v", `["v","gone","r"]`, "", "r")
	// None such: the last list, of what it picks.
	go1("v", "[]", "?project=p", "r")
	go1("l", "[]", "?kind=link", "d")
	// Not on the last list: all of its kind, which is where a page ends before the links.
	go1("v", "[]", "?kind=link", "n")
	stays := func(from string) {
		t.Helper()

		b.navigate("/a/" + from + "/")
		b.until("the bar", "document.getElementById('frame-title').textContent")
		// Leaving is told at once: beforeunload runs as location is set.
		check(t, from+" at an end", []string{b.eval("addEventListener('beforeunload', () => { window.left = true; }); document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'j', bubbles: true })); window.left === true")}, "false")
	}
	stays("r")
	// Archived: the archived.
	go1("a", "[]", "?kind=link", "b")
	// At the end of the list that opened it, it stays.
	b.navigate("/")
	b.eval(`sessionStorage.setItem('pages.order', '{"order":["v","n"],"shown":["v","n"],"back":"/"}')`)
	stays("n")
}

// TestUIColumns drags the edges of the shelf and the list, by the mouse and the keys, within their bounds, the page beside the
// list taking no pointer meanwhile; folds the shelf and shows it again; keeps all of it across a reload; and shows more of a
// path as the list widens, all of it when wide enough. Under 70rem none of it shows.
func TestUIColumns(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := asideSite(t)
	b := s.b
	b.navigate("/")
	b.eval("localStorage.removeItem('list.columns')")
	file := s.write(t, "aside/a (1)/v2.0/data-set_#1/deep/deep/deep/deep/deep/deep/deep/deep/name-2.html", "<title>long</title><p>l</p>")
	s.floe(t, "publish", "--link", file, "--slug", "long")
	b.navigate("/")
	b.size(1600, 900, false)
	b.until("the rows and the window", "document.querySelector('#list li[data-slug=long]') && getComputedStyle(document.documentElement).getPropertyValue('--list-width') === '408px'")
	// Defaults and bounds go with the browser's text.
	out := make([]string, 0, 24)
	b.eval("document.documentElement.style.fontSize = '20px'; dispatchEvent(new Event('resize'))")
	out = append(out, b.eval("getComputedStyle(document.documentElement).getPropertyValue('--shelf-width') + ' ' + getComputedStyle(document.documentElement).getPropertyValue('--list-width') + ' ' + document.getElementById('list-grip').getAttribute('aria-valuemin')"))
	b.eval("document.documentElement.style.fontSize = ''; dispatchEvent(new Event('resize'))")
	const widths = "[getComputedStyle(document.documentElement).getPropertyValue('--shelf-width'), getComputedStyle(document.documentElement).getPropertyValue('--list-width'), Math.round(document.getElementById('shelf').getBoundingClientRect().width), Math.round(document.getElementById('listcol').getBoundingClientRect().width)].join(' ')"
	mouse := func(grip string, to float64, release bool) {
		t.Helper()

		r := b.rect("document.getElementById(" + js(grip) + ")")
		x, y := r.X+r.Width/2, r.Y+r.Height/2
		b.painted()
		events := []chromedp.Action[chromedp.Void]{
			chromedp.MouseEvent(chromedp.MousePressed, x, y, chromedp.ButtonLeft, chromedp.ClickCount(1)),
			chromedp.MouseEvent(chromedp.MouseMoved, (x+to)/2, y, chromedp.ButtonLeft),
			chromedp.MouseEvent(chromedp.MouseMoved, to, y, chromedp.ButtonLeft),
		}
		if release {
			events = append(events, chromedp.MouseEvent(chromedp.MouseReleased, to, y, chromedp.ButtonLeft, chromedp.ClickCount(1)))
		}
		for _, e := range events {
			if err := chromedp.Do(b.ctx, e); err != nil {
				t.Fatal(err)
			}
		}
	}
	// How much of the path is cut, its box's whole width else.
	const cut = "(() => { const p = document.querySelector('#list li[data-slug=long] [data-testid=path]'); return p.scrollWidth - p.clientWidth; })()"
	out = append(out, b.eval(widths))
	before, err := strconv.Atoi(b.eval(cut))
	if err != nil || before <= 0 {
		t.Fatalf("the path is not cut: %d, %v", before, err)
	}
	// The mouse: the shelf's edge, then the list's, from where the list starts. The list held off the page beside it.
	mouse("shelf-grip", 260, true)
	out = append(out, b.eval(widths))
	mouse("list-grip", 760, true)
	out = append(out, b.eval(widths))
	mouse("list-grip", 1590, true)
	// At the widest: a little wider, the path goes up onto the line of the rest, and has less room there than on its own line.
	after, err := strconv.Atoi(b.eval(cut))
	if err != nil || after >= before {
		t.Errorf("the path is cut by %d at the widest list, %d before: %v", after, before, err)
	}
	out = append(out, b.eval(widths+" + ' ' + (innerWidth - document.getElementById('reader').getBoundingClientRect().left)"))
	// Wide enough, all of it shows.
	b.size(3000, 900, false)
	b.until("the wide window", "Number(document.getElementById('list-grip').getAttribute('aria-valuemax')) > 2000")
	mouse("list-grip", 2400, true)
	out = append(out, b.eval(cut))
	b.size(1600, 900, false)
	b.until("the window back", "getComputedStyle(document.documentElement).getPropertyValue('--list-width') === '820px'")
	// The keys: 16 px, the bounds at Home and End, as the tree's edge.
	b.eval("document.getElementById('list-grip').focus()")
	for _, k := range []string{kb.Home, kb.ArrowRight} {
		if err := chromedp.Do(b.ctx, chromedp.KeyEvent(k)); err != nil {
			t.Fatal(err)
		}
	}
	out = append(out, b.eval(widths), b.settled(ringOf))
	// At its narrowest, the list holds a row's menu and the bulk bar, the filters folded as open.
	const whole = `(() => { const box = document.getElementById('listcol').getBoundingClientRect();
		const within = (e) => { const r = e.getBoundingClientRect(); return r.left >= box.left - 0.5 && r.right <= box.right + 0.5; };
		document.querySelector('#list li[data-slug=long] summary').click();
		const menu = within(document.querySelector('#list li[data-slug=long] [data-testid=menu]'));
		document.querySelector('#list li[data-slug=long] summary').click();
		document.querySelector('#list li[data-slug=long] [data-testid=sel]').click();
		const bar = within(document.getElementById('bulk-clear'));
		document.getElementById('bulk-clear').click();
		return [menu, bar].join(' '); })()`
	if err := chromedp.Do(b.ctx, chromedp.KeyEvent(kb.Home)); err != nil {
		t.Fatal(err)
	}
	out = append(out, b.eval(whole))
	if err := chromedp.Do(b.ctx, chromedp.KeyEvent(kb.ArrowRight)); err != nil {
		t.Fatal(err)
	}
	mouse("shelf-grip", 10, true)
	out = append(out, b.eval(widths))
	// Dragged, the page beside the list takes no pointer; let go, it does.
	b.eval("document.querySelector('#list li[data-slug=a1] [data-testid=title]').click()")
	mouse("list-grip", 900, false)
	out = append(out, b.eval("getComputedStyle(document.getElementById('read-frame')).pointerEvents + ' ' + (document.getElementById('list-grip').dataset.dragging === 'true')"))
	if err := chromedp.Do(b.ctx, chromedp.MouseEvent(chromedp.MouseReleased, 900, 300, chromedp.ButtonLeft, chromedp.ClickCount(1))); err != nil {
		t.Fatal(err)
	}
	out = append(out, b.until("the edge let go", "!document.getElementById('list-grip').dataset.dragging && getComputedStyle(document.getElementById('read-frame')).pointerEvents"))
	// Folded and shown again, the focus on the other button, the two in one foot.
	const shelf = "[getComputedStyle(document.getElementById('shelf')).display, document.getElementById('shelf-show').hidden, document.getElementById('shelf-grip').hidden, document.activeElement.id, document.getElementById('shelf-show').getBoundingClientRect().right <= document.getElementById('list-hide').getBoundingClientRect().left].join(' ')"
	b.eval("document.getElementById('shelf-hide').click()")
	out = append(out, b.eval(shelf), b.eval("Math.round(document.getElementById('listcol').getBoundingClientRect().left)"))
	// Folded, the head says what the filters narrow the list to, in its line beside the rest; pressed, they show.
	const what = "[document.getElementById('shelf-what').hidden, document.getElementById('shelf-what').textContent, document.querySelector('[data-testid=lhead]').scrollWidth <= document.querySelector('[data-testid=lhead]').clientWidth].join(' ')"
	out = append(out, b.eval(what))
	// The list's width from where the list starts; a tag no page has, so no chip to press, from the URL (what is folded is kept).
	listAt := func(w float64) float64 { return b.rect("document.getElementById('listcol')").X + w }
	b.drag("#list-grip", listAt(500))
	b.navigate("/?view=archived&kind=link&tag=x")
	// The words come with the list's first read, after the page has loaded
	out = append(out, b.until("the words of the picks", "document.getElementById('shelf-what').textContent && "+what), b.eval("(() => { const e = document.getElementById('shelf-what'), r = document.createRange(); r.selectNodeContents(e); return r.getBoundingClientRect().width <= e.getBoundingClientRect().width + 0.5; })()"))
	b.drag("#list-grip", listAt(740))
	b.eval("document.getElementById('shelf-what').click()")
	out = append(out, b.eval("getComputedStyle(document.getElementById('shelf')).display"))
	b.eval("document.querySelector('#shelf [data-view=all]').click(); document.getElementById('shelf-hide').click()")
	// Folded across the widths: the row of the views and no button under 70rem, as a line of a list; folded still above it.
	b.size(1000, 700, false)
	const folded = "[getComputedStyle(document.getElementById('shelf')).display, document.querySelector('[data-testid=views]').getClientRects().length, document.getElementById('shelf-show').getClientRects().length, document.getElementById('shelf-what').getClientRects().length, parseFloat(getComputedStyle(document.querySelector('[data-testid=lhead]')).paddingLeft), getComputedStyle(document.getElementById('manage')).gridTemplateColumns.split(' ').length].join(' ')"
	out = append(out, b.until("the row of the filters", "matchMedia('(max-width: 70rem)').matches && "+folded))
	b.size(1600, 900, false)
	out = append(out, b.until("the filters folded", "!matchMedia('(max-width: 70rem)').matches && "+folded))
	// Kept across a reload, folded too.
	kept := b.eval("localStorage.getItem('list.columns')")
	b.navigate("/")
	out = append(out, b.eval("localStorage.getItem('list.columns') === "+js(kept)), b.eval("getComputedStyle(document.getElementById('shelf')).display"))
	b.eval("document.getElementById('shelf-show').click()")
	out = append(out, b.eval("[getComputedStyle(document.getElementById('shelf')).display, document.activeElement.id].join(' ')"), b.eval(widths))
	// A window narrower holds the list under what is kept, which stays.
	b.size(1200, 800, false)
	out = append(out, b.until("the list held", "getComputedStyle(document.documentElement).getPropertyValue('--list-width') === '520px' && '520px ' + JSON.parse(localStorage.getItem('list.columns')).list"))
	// Under 70rem, none of it shows.
	b.size(1000, 700, false)
	out = append(out, b.until("the row of the shelf", "matchMedia('(max-width: 70rem)').matches && ['shelf-grip', 'list-grip', 'shelf-show', 'shelf-hide'].map((id) => document.getElementById(id).getClientRects().length).join(' ')"))
	// The list folds as the filters do, its button beside the page with no page chosen too; the focus in it goes to the button.
	b.size(1600, 900, false)
	b.until("the wide window", "!matchMedia('(max-width: 70rem)').matches")
	b.eval("localStorage.removeItem('list.columns')")
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=a1]')")
	const list = "[getComputedStyle(document.getElementById('listcol')).display, document.getElementById('list-grip').hidden, document.getElementById('list-show').getClientRects().length, document.getElementById('read-head').hidden, document.activeElement.id, Math.round(document.getElementById('reader').getBoundingClientRect().left)].join(' ')"
	out = append(out, b.eval(list))
	b.eval("document.querySelector('#list li[data-slug=a1] [data-testid=title]').focus(); document.getElementById('list-hide').click()")
	// Of the bar over a page, none shows with no page picked (a build may leave it out)
	out = append(out, b.eval(list), b.eval("document.getElementById('read-blank').hidden + ' ' + ['read-actions', 'read-title'].every((id) => !document.getElementById(id)?.checkVisibility())"))
	// A page chosen and gone leaves no link to it in the head.
	b.eval("document.querySelector('#list li[data-slug=a2] [data-testid=title]').click()")
	b.eval("window.fetch = ((f) => async (u, i) => { const r = await f(u, i); if (i?.method || String(u) !== '/api/pages') return r; return new Response(JSON.stringify((await r.json()).filter((a) => a.slug !== 'a2'))); })(window.fetch)")
	b.reread()
	// The frame's page as it shows: a build may leave the hidden frame as it was
	out = append(out, b.until("the page gone", "!document.querySelector('#list li[data-slug=a2]') && [document.getElementById('read-title').getClientRects().length, document.getElementById('read-frame').checkVisibility() ? document.getElementById('read-frame').dataset.slug : ''].join(' ')"))
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=a2]')")
	// Folded, j and k go on through the list, the page changing beside the place it was.
	key := func(k string) {
		b.eval("document.body.dispatchEvent(new KeyboardEvent('keydown', { key: '" + k + "', bubbles: true }))")
	}
	key("j")
	out = append(out, b.eval("document.getElementById('read-frame').dataset.slug"))
	key("j")
	out = append(out, b.eval("document.getElementById('read-frame').dataset.slug + ' ' + document.getElementById('read-actions').hidden"))
	// Both folded, both buttons over the page, told apart by their words; the page whole.
	b.eval("document.getElementById('shelf-hide').click()")
	out = append(out, b.eval("[...document.querySelectorAll('#shelf-show, #list-show')].filter((e) => e.getClientRects().length).map((e) => e.id + ':' + (e.title === e.getAttribute('aria-label'))).join(' ') + ' ' + document.activeElement.id + ' ' + Math.round(document.getElementById('reader').getBoundingClientRect().left)"))
	// Words of the filters in the page's head, however long, leave its buttons in it. The page list's words are a view and a
	// count of picks, short: long ones are set here, in the same task as the measure (a render puts them back). A pick from the URL
	// leaves the list folded; j opens its first row beside it, so the head has its buttons.
	b.navigate("/?kind=page")
	b.until("the rows", "document.querySelector('#list li[data-slug=a1]')")
	key("j")
	b.until("the words and the buttons in the page's head", "document.getElementById('shelf-what').parentElement.id === 'read-head' && document.getElementById('shelf-what').getClientRects().length && document.getElementById('read-frame').dataset.slug && !document.getElementById('read-actions').hidden")
	check(t, "the page's head", []string{b.eval(`(() => {
		document.getElementById('shelf-what').textContent = 'x'.repeat(300);
		const head = document.getElementById('read-head').getBoundingClientRect();
		const within = (e) => { const r = e.getBoundingClientRect(); return r.width > 0 && r.left >= head.left && r.right <= head.right + 0.5; };
		const shown = [...document.querySelectorAll('#read-actions > *')].filter((e) => e.getClientRects().length);
		return (shown.length > 0) + ' ' + shown.every(within);
	})()`)}, "true true")
	// Kept across a reload.
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=a1]')")
	const shows = "document.getElementById('shelf-show').hidden + ' ' + document.getElementById('list-show').hidden"
	out = append(out, b.eval(shows))
	b.eval("document.getElementById('list-show').click()")
	out = append(out, b.eval("document.activeElement.id + ' ' + "+shows+" + ' ' + document.getElementById('shelf-what').parentElement.dataset.testid"))
	b.eval("document.getElementById('shelf-show').click(); document.getElementById('list-hide').click()")
	// What the filters pick shows: folded, the list opens; the search too.
	b.eval("document.querySelector('#shelf [data-view=fav]').click()")
	out = append(out, b.eval("getComputedStyle(document.getElementById('listcol')).display"))
	b.eval("document.querySelector('#shelf [data-view=all]').click(); document.getElementById('list-hide').click()")
	b.fill("#q", "a")
	out = append(out, b.eval("getComputedStyle(document.getElementById('listcol')).display"))
	b.fill("#q", "")
	b.eval("document.getElementById('list-hide').click()")
	// Folded across the widths: the list as ever, no buttons, under 70rem; folded still above it.
	// With a page chosen, its head shows.
	b.size(1000, 700, false)
	b.until("the list under 70rem", "matchMedia('(max-width: 70rem)').matches && getComputedStyle(document.getElementById('listcol')).display === 'flex'")
	b.eval("document.querySelector('#list li[data-slug=a1] [data-testid=title]').click()")
	out = append(out, b.until("the head of the page", "document.getElementById('read-title').textContent && [getComputedStyle(document.getElementById('listcol')).display, ...['list-show', 'list-hide', 'list-grip'].map((id) => document.getElementById(id).getClientRects().length), document.getElementById('read-head').getClientRects().length].join(' ')"))
	b.size(1600, 900, false)
	out = append(out, b.until("the list folded", "!matchMedia('(max-width: 70rem)').matches && getComputedStyle(document.getElementById('listcol')).display"))

	// The two feet's buttons stand on one line, and what opens each column stays on it, folded.
	b.size(1600, 900, false)
	b.until("the wide window", "!matchMedia('(max-width: 70rem)').matches")
	b.eval("localStorage.removeItem('list.columns')")
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=a1]')")
	b.eval("document.querySelector('#list li[data-slug=a1] [data-testid=title]').click()")
	const lines = `((ids) => ids.map((s) => { const r = document.querySelector(s).getBoundingClientRect(); return r.top + r.height / 2; }).join(' '))`
	open := b.eval(lines + "(['#shelf-hide', '#list-hide'])")
	b.eval("document.getElementById('list-hide').click(); document.getElementById('shelf-hide').click()")
	shut := b.eval(lines + "(['#shelf-show', '#list-show'])")
	y, _, _ := strings.Cut(open, " ")
	out = append(out, strconv.FormatBool(open == y+" "+y), strconv.FormatBool(shut == open))
	b.eval("document.getElementById('list-show').click(); document.getElementById('shelf-show').click()")
	// The rule of the folded columns leaves no width between it and the one under 70rem: it is that one's negation. The React
	// build writes its rules in layers, so the rules are walked through.
	out = append(out, b.eval(`(() => {
		const texts = [], walk = (rules) => { for (const r of rules) { if (r.media) texts.push(r.media.mediaText); if (r.cssRules) walk(r.cssRules); } };
		for (const sheet of document.styleSheets) walk(sheet.cssRules);
		return texts.some((m) => m.includes('70rem') && !m.startsWith('not') && (texts.includes('not ' + m) || texts.includes('not all and ' + m)));
	})()`))

	// Between the widths a zoomed window can fall, one set of rules or the other holds: not only "under 70rem", and not both.
	out = append(out, b.eval("[matchMedia('(max-width: 70rem)').matches, matchMedia('not (max-width: 70rem)').matches, matchMedia('not (max-width: 70rem)').media].join(' ')"))

	// What is kept that cannot be a width is the default's, or within the bounds.
	for _, kept := range []string{"not json", `{"shelf":"x","listOpen":0}`, "null"} {
		b.eval("localStorage.setItem('list.columns', " + js(kept) + ")")
		b.navigate("/")
		out = append(out, b.eval("[getComputedStyle(document.documentElement).getPropertyValue('--shelf-width'), document.getElementById('shelf-show').hidden, document.getElementById('list-show').hidden].join(' ')"))
	}
	b.size(1600, 900, false)
	b.eval(`localStorage.setItem('list.columns', '{"shelf":99999,"list":-5}')`)
	b.navigate("/")
	out = append(out, b.until("the bounds", "[getComputedStyle(document.documentElement).getPropertyValue('--shelf-width'), getComputedStyle(document.documentElement).getPropertyValue('--list-width')].join(' ')"))
	// A plain press of a link of the bar, the logo and the way between the screens, on the same screen too, opens the
	// shelf and the list folded where it goes and keeps them so.
	b.size(1600, 900, false)
	// A task, as the task list with none lays out its column alone, with no shelf
	s.floe(t, "task", "--slug", "a1", "--task", "T1", "--new")
	shown := "(id) => getComputedStyle(document.getElementById(id)).display !== 'none'"
	pressed := make([]string, 0, 3)
	for _, link := range []string{"#to-list", "[data-testid=brand]", "#to-tasks"} {
		b.navigate("/api/pages")
		b.eval("localStorage.setItem('list.columns', JSON.stringify({ ...JSON.parse(localStorage.getItem('list.columns') ?? '{}'), open: false, listOpen: false }))")
		b.navigate("/")
		folded := b.until("the columns folded", "document.querySelector('#list li[data-slug=a1]') && ["+shown+"].map((s) => s('shelf') + ' ' + s('listcol'))[0]")
		b.eval("window.leaving = true; document.querySelector(" + js(link) + ").click()")
		pressed = append(pressed, link+" "+folded+" → "+b.until("the screen it goes to", "!window.leaving && document.getElementById('shelf')?.childElementCount && ["+shown+"].map((s) => location.pathname + ' ' + s('shelf') + ' ' + s('listcol'))[0] + ' kept ' + ((k) => k.open + ' ' + k.listOpen)(JSON.parse(localStorage.getItem('list.columns')))"))
	}
	check(t, "the bar's links", pressed, "#to-list false false → / true true kept true true|"+
		"[data-testid=brand] false false → / true true kept true true|#to-tasks false false → /tasks true true kept true true")
	b.unsize()
	ring := "solid 2px rgb(107, 105, 100) -2px seen"
	check(t, "columns", out, strings.Join([]string{
		"250px 510px 400", "200px 408px 200 408", "260px 408px 260 408", "260px 500px 260 500",
		"260px 820px 260 820 520", "0",
		"260px 336px 260 336", ring, "true true", "160px 336px 160 336",
		"none true", "auto",
		"none false true shelf-show true", "0",
		"true  true", "false Archive · Filter 2 true", "true", "flex",
		"block 1 0 0 16 2", "none 0 1 0 16 2",
		"true", "none", "flex shelf-hide", "160px 740px 160 740",
		"520px 740", "0 0 0 0", "flex false 0 true  608", "none true 1 true list-show 200", "false true", "0 ",
		"long", "a3 false", "shelf-show:true list-show:true shelf-show 0",
		"false false", "list-hide false true lhead", "flex", "flex",
		"flex 0 0 0 1", "none", "true", "true", "true", "false true not (max-width: 70rem)",
		"200px true true", "200px true true", "200px true true", "320px 320px",
	}, "|"))
}

// TestUIColumnFoot: the button that folds a column stands in its foot, and what opens it stands in the same place, with nothing of the
// column left: the filters, the list, a folder's tree and a diff's contents. The foot stays under what scrolls, its line reaching the
// column's edge past the scrollbar. Where a column cannot fold, neither shows.
func TestUIColumnFoot(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := diffSite(t)
	b := s.b
	s.write(t, "docs/index.html", "<!doctype html><title>t</title><p>page</p>")
	s.floe(t, "publish", "--link", filepath.Join(s.Env["HOME"], "docs"))
	b.size(1440, 900, false)
	defer b.unsize()
	b.navigate("/")
	b.eval("localStorage.clear()")
	at := func(id string) string {
		t.Helper()

		return b.eval("(() => { const r = document.getElementById(" + js(id) + ").getBoundingClientRect(); return r.left + ' ' + r.top; })()")
	}
	press := func(id string) {
		t.Helper()

		// A click where the button is seen, so that one under the page or another box takes nothing.
		r := b.rect("document.getElementById(" + js(id) + ")")
		b.mouse(input.DispatchMouseEventTypeMousePressed, r.X+r.Width/2, r.Y+r.Height/2, 1)
		b.mouse(input.DispatchMouseEventTypeMouseReleased, r.X+r.Width/2, r.Y+r.Height/2, 1)
	}
	// The ground and the shadow of what opens a column: over a page, its own; in the list's foot, none, as the button beside it.
	const panel = "getComputedStyle(document.documentElement).getPropertyValue('--color-panel').trim()"
	face := func(id string) string {
		t.Helper()

		// As it rests: the pointer on it after a press, its ground would be that of a hover
		b.point(0, 0)
		return b.settled(`(() => {
			const s = getComputedStyle(document.getElementById(` + js(id) + `)), c = document.createElement('i');
			c.style.color = ` + panel + `; document.body.append(c);
			const ground = getComputedStyle(c).color; c.remove();
			return [s.backgroundColor === ground ? 'panel' : s.backgroundColor, s.boxShadow === 'none' ? 'no shadow' : 'shadow'].join(' ');
		})()`)
	}
	// The line between each two faces shown, and at the two edges, of the boxes named in order: 1px between, none at an edge.
	seams := func(boxes ...string) string {
		t.Helper()

		return b.eval(`(() => {
			const shown = ` + js(strings.Join(boxes, ",")) + `.split(',').map((q) => document.querySelector(q)).filter((e) => e.getClientRects().length).map((e) => getComputedStyle(e));
			const between = shown.slice(1).map((s, i) => parseFloat(shown[i].borderRightWidth) + parseFloat(s.borderLeftWidth));
			return [parseFloat(shown[0].borderLeftWidth), ...between, parseFloat(shown.at(-1).borderRightWidth)].join(' ');
		})()`)
	}
	columns := func() string {
		t.Helper()

		return seams("#shelf", "#listcol", "#reader")
	}
	out := make([]string, 0, 40)

	// A folder's page: the page takes all the width, folded.
	b.navigate("/a/docs/?file=index.html")
	b.until("the tree", "!document.getElementById('side').hidden")
	out = append(out, at("side-hide"), seams("#side", "#frame"))
	press("side-hide")
	out = append(out, seams("#side", "#frame"), at("side-show"), face("side-show"), b.eval("document.activeElement.id + ' ' + document.getElementById('frame').getBoundingClientRect().width"))
	press("side-show")
	out = append(out, b.eval("document.getElementById('side').hidden + ' ' + document.activeElement.id"))

	// A diff: the words take all the width, with room under their last line for what opens the contents.
	openDiff(b, "/a/d/?diff")
	out = append(out, at("diff-toc-hide"), seams("#diff-toc-nav", "#diff-body"))
	press("diff-toc-hide")
	out = append(out, seams("#diff-toc-nav", "#diff-body"), at("diff-toc-show"), face("diff-toc-show"), b.eval("(() => { const r = document.getElementById('diff-body'); return [r.getBoundingClientRect().left, r.getBoundingClientRect().width, getComputedStyle(r).paddingBottom, getComputedStyle(r).borderLeftWidth].join(' '); })()"))
	press("diff-toc-show")
	out = append(out, b.eval("document.getElementById('diff-toc-nav').hidden + ' ' + document.activeElement.id"))
	// In a window too low for the contents and the words under them, the foot stays, and the page does not scroll.
	b.size(1440, 260, false)
	out = append(out, b.until("the low window", "innerHeight === 260 && (() => { const f = document.querySelector('#diff-toc-nav [data-testid=col-foot]').getBoundingClientRect(); return [f.bottom, document.scrollingElement.scrollHeight - innerHeight].join(' '); })()"))
	b.size(1440, 900, false)
	// Narrower, the tree's button and what opens it stay in one place; under 40rem the tree goes over the page.
	for _, width := range []int64{800, 400} {
		b.size(width, 900, false)
		b.navigate("/a/docs/?file=index.html")
		b.until("the frame", "!document.getElementById('frame').hidden && innerWidth === "+strconv.FormatInt(width, 10))
		if width == 400 {
			press("side-show")
		}
		out = append(out, at("side-hide"))
		press("side-hide")
		out = append(out, at("side-show"))
		if width == 800 {
			press("side-show")
		}
	}
	b.size(1440, 900, false)

	// The list's page, its rows more than it can hold: each column alone, then both.
	b.navigate("about:blank")
	b.stub(manyPages(60))
	defer b.unstub()
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li:not([data-testid=group])')")
	// With no page chosen, folding the list leaves no head, but for the words of the filters to hold.
	const head = "document.getElementById('read-head').getClientRects().length"
	press("list-hide")
	out = append(out, b.eval(head))
	press("shelf-hide")
	out = append(out, b.eval(head))
	b.navigate("/?view=fav")
	out = append(out, b.until("the words", "!document.getElementById('shelf-what').hidden && "+head))
	press("shelf-show")
	out = append(out, b.eval(head))
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li:not([data-testid=group])')")
	press("list-show")
	b.eval("document.querySelector('#list li:not([data-testid=group]) [data-testid=title]').click()")
	b.until("the page", "!document.getElementById('read-frame').hidden")
	out = append(out, at("shelf-hide"), at("list-hide"), columns())
	out = append(out, b.eval(`(() => {
		const body = document.getElementById('list-body'), col = document.getElementById('listcol'), foot = document.querySelector('#listcol > [data-testid=col-foot]').getBoundingClientRect();
		return [body.scrollHeight > body.clientHeight, foot.height, foot.bottom === innerHeight, body.getBoundingClientRect().bottom === foot.top, foot.right === col.getBoundingClientRect().left + col.clientLeft + col.clientWidth].join(' ');
	})()`))
	press("shelf-hide")
	out = append(out, at("shelf-show"), face("shelf-show"), at("list-hide"), b.eval("document.activeElement.id"), columns())
	press("shelf-show")
	press("list-hide")
	out = append(out, at("list-show"), face("list-show"), b.eval("document.activeElement.id"), columns())
	press("shelf-hide")
	out = append(out, at("shelf-show"), face("shelf-show"), at("list-show"), columns(), b.eval("(() => { const r = document.getElementById('read-frame').getBoundingClientRect(); return [r.left, r.width, document.getElementById('read-title').getBoundingClientRect().left].join(' '); })()"))
	// Over the page, a press opens the filters too; printed, nothing of it lies over the words.
	press("shelf-show")
	out = append(out, b.eval("document.activeElement.id"))
	press("shelf-hide")
	b.media("print")
	out = append(out, b.eval("['shelf-show', 'list-show'].map((id) => getComputedStyle(document.getElementById(id)).display).join()"))
	b.media("")
	// Nowhere to fold to, under 70rem: neither foot nor what opens, folded as they are kept.
	b.size(1100, 800, false)
	out = append(out, b.until("the one row of filters", "matchMedia('(max-width: 70rem)').matches && ['#shelf-show', '#list-show', '#shelf > [data-testid=col-foot]', '#listcol > [data-testid=col-foot]'].map((q) => document.querySelector(q).getClientRects().length).join(' ')"))
	b.size(1440, 900, false)
	press("list-show")
	out = append(out, b.until("the list again", "document.getElementById('list-show').hidden && document.activeElement.id"))

	check(t, "feet", out, strings.Join([]string{
		"6 863.5", "0 1 0", "0 0", "6 863.5", "panel shadow", "side-show 1440", "false side-hide",
		"6 863.5", "0 1 0", "0 0", "6 863.5", "panel shadow", "0 1440 44px 0px", "false diff-toc-hide",
		"260 0",
		"6 863.5", "6 863.5", "6 863.5", "6 863.5",
		"0", "0", "1", "0",
		"6 863.5", "206 863.5", "0 1 1 0", "true 44 true true true",
		"6 863.5", "rgba(0, 0, 0, 0) no shadow", "42 863.5", "shelf-show", "0 1 0",
		"206 863.5", "panel shadow", "list-show", "0 1 0",
		"6 863.5", "panel shadow", "42 863.5", "0 0", "0 1440 16",
		"shelf-hide", "none,none",
		"0 0 0 0", "list-hide",
	}, "|"))
}

// TestUIBarActions: what the bar over a page shows, its menu does not, and what leaves the bar is in its menu, for each kind of
// entry, at both widths where the bar changes. A row has no bar, and its menu has all.
func TestUIBarActions(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.page(t, "bar/two.yaml", "Two", "<p>one</p>"), "--slug", "two")
	s.floe(t, "publish", s.page(t, "bar/two.yaml", "Two", "<p>two</p>"), "--slug", "two")
	s.floe(t, "publish", s.page(t, "bar/one.yaml", "One", "<p>one</p>"), "--slug", "one")
	s.floe(t, "publish", s.page(t, "bar/kept.yaml", "Kept", "<p>kept</p>"), "--slug", "kept")
	s.floe(t, "archive", "kept")
	s.floe(t, "publish", "--link", s.write(t, "bar/l/link.html", "<title>L</title><p>l</p>"), "--slug", "link")
	gone := s.write(t, "bar/g/gone.html", "<title>G</title><p>g</p>")
	s.floe(t, "publish", "--link", gone, "--slug", "gone")
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	s.write(t, "bar/docs/index.html", "<!doctype html><title>t</title><p>page</p>")
	s.floe(t, "publish", "--link", filepath.Join(s.Env["HOME"], "bar/docs"), "--slug", "docs")
	b.size(1440, 900, false)
	defer b.unsize()

	// The bar of the box and its menu, opened and closed, by what each thing is: only what is seen.
	const named = `const name = (e) => ['diff-open', 'fav-item', 'mark-unread', 'select', 'tag-item', 'theme-item', 'pin-item', 'task-set', 'task-open', 'edit', 'copy', 'download', 'open', 'archive', 'star', 'delete'].find((c) => e.dataset.testid === c);`
	// A menu opened and shut as a person does, by its summary.
	const menuItems = `const menuOf = (more) => { more.querySelector('summary').click();
		const got = ` + topsJS + `(more.querySelector('[data-testid=menu]')).filter((e) => e.getClientRects().length).map(name).join(' ');
		more.querySelector('summary').click();
		return got; };`
	shown := func(box string) string {
		t.Helper()

		bar := b.eval(`(() => { ` + named + `
			const out = ` + topsJS + `(document.querySelector(` + js(box) + `)).filter((e) => e.getClientRects().length).map((e) => (e.dataset.testid === 'more' ? 'more' : name(e)));
			return out.filter(Boolean).join(' ');
		})()`)
		menu := b.eval(`(() => { ` + named + menuItems + `
			const more = document.querySelector(` + js(box+" [data-testid=more]") + `);
			return more ? menuOf(more) : '';
		})()`)
		return bar + " | " + menu
	}
	const picked = "document.querySelector('#list li[aria-current=true]')?.dataset.slug"
	pick := func(slug string) {
		t.Helper()

		b.eval("document.querySelector('#list li[data-slug=" + slug + "] [data-testid=title]').click()")
		b.until(slug+" beside the list", picked+" === "+js(slug))
	}
	// An item of the menu of box, pressed as a person does: the menu opened, then the item once it shows.
	pressItem := func(box, item string) {
		t.Helper()

		b.eval("document.querySelector('" + box + " [data-testid=more] > summary').click()")
		sel := "document.querySelector('" + box + " [data-testid=" + item + "]')"
		b.until("the item "+item, sel+".checkVisibility()")
		b.eval(sel + ".click()")
	}
	// The modal that finds, opened by its key and given words.
	find := func(words string) {
		t.Helper()

		if b.eval("document.getElementById('find').open") != "true" {
			keyDown(b, "key: 'k', ctrlKey: true")
			b.until("the modal that finds", "document.getElementById('find').open")
		}
		b.fill("#find-q", words)
	}
	// Asked from the page, of the server: the page has been read or not.
	serverNews := func(slug string) string {
		return "window.newsOf ??= {}, fetch('/api/pages').then((r) => r.json()).then((l) => { window.newsOf[" + js(slug) + "] = l.find((a) => a.slug === " + js(slug) + ").news ?? ''; }), window.newsOf[" + js(slug) + "]"
	}
	out := make([]string, 0, 16)
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=two][data-unread=true]')")
	for _, slug := range []string{"two", "one"} {
		pick(slug)
		b.until(slug+" read", "document.querySelector('#list li[data-slug="+slug+"]:not([data-unread])')")
		b.until("its menu", inMenuItem("#read-actions", "mark-unread", "i.hidden")+" === false")
		out = append(out, shown("#read-actions"))
	}
	for _, slug := range []string{"link", "gone"} {
		pick(slug)
		out = append(out, shown("#read-actions"))
	}
	// A row's menu has all, and the bar's kinds in the foot of the modal that finds: a folder, which the list opens nothing of.
	out = append(out, b.eval(`(() => { `+named+menuItems+`
		return menuOf(document.querySelector('#list li[data-slug=one] [data-testid=more]'));
	})()`))
	find("docs")
	b.until("the folder's row", "document.getElementById('find-actions').dataset.slug === 'docs'")
	out = append(out, shown("#find-actions"))
	find("two")
	b.until("the page's row", "document.getElementById('find-actions').dataset.slug === 'two'")
	out = append(out, shown("#find-actions"))
	b.pressKey(kb.Escape)
	b.until("the modal shut", "!document.getElementById('find').open")

	// The whole page: an archived one tells what a press does in its button, and is black while it is.
	b.navigate("/a/kept/")
	b.until("the bar", "!document.getElementById('frame-actions').hidden && document.querySelector('#frame-actions [data-testid=star]')")
	// Archived, it has no way back to unread, read or not
	b.until("read", serverNews("kept")+" === ''")
	out = append(out, shown("#frame-actions"))
	const archiveButton = "document.querySelector('#frame-actions > [data-testid=archive]')"
	const archive = "(() => { const a = " + archiveButton + "; return [a.title, a.getAttribute('aria-label'), a.getAttribute('aria-pressed'), a.dataset.on === 'true', getComputedStyle(a).backgroundColor === 'rgba(0, 0, 0, 0)'].join(' / '); })()"
	out = append(out, b.eval(archive))
	b.eval(archiveButton + ".focus(); " + archiveButton + ".click()")
	b.until("restored", "document.querySelector('#frame-actions > [data-testid=archive]:not([data-on])')")
	out = append(out, b.settled(archive), b.eval("document.activeElement.matches('#frame-actions > [data-testid=archive]')"))
	b.navigate("/a/two/")
	b.until("the bar", "document.querySelector('#frame-actions > [data-testid=download][href]')")
	out = append(out, b.eval("(() => { const d = document.querySelector('#frame-actions > [data-testid=download]'); return [new URL(d.href).pathname, d.download].join(' '); })()"))
	b.navigate("/a/docs/?file=index.html")
	b.until("the bar", "!document.getElementById('frame-actions').hidden && document.querySelector('#frame-actions [data-testid=star]')")
	out = append(out, shown("#frame-actions"))
	// Pressed the other way, twice before the answer: one question and one toast, the words changed, the mark black with what is on it readable.
	b.navigate("/a/one/")
	b.until("the bar", "document.querySelector('#frame-actions > [data-testid=archive][title]')")
	b.eval(`window.patched = 0; window.fetch = ((f) => (u, i) => { if (i?.method === 'PATCH') window.patched++; return f(u, i); })(window.fetch);
		` + archiveButton + `.focus(); ` + archiveButton + `.click(); ` + archiveButton + `.click()`)
	b.until("archived", "document.querySelector('#frame-actions > [data-testid=archive][data-on=true]')")
	said := b.until("the toast", "document.querySelector('[data-testid=toast]') && [window.patched, [...document.querySelectorAll('[data-testid=toast]')].map((e) => e.textContent).join(), document.activeElement.matches('#frame-actions > [data-testid=archive]')].join(' / ')")
	out = append(out, b.settled(archive), said)
	out = append(out, b.settled(`(() => {
		const lum = (c) => { const [r, g, bl] = c.match(/[\d.]+/g).map((x) => { const v = x / 255; return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4; }); return 0.2126 * r + 0.7152 * g + 0.0722 * bl; };
		const s = getComputedStyle(`+archiveButton+`), a = lum(s.color), g = lum(s.backgroundColor);
		return (Math.max(a, g) + 0.05) / (Math.min(a, g) + 0.05) >= 4.5;
	})()`))
	b.eval(archiveButton + ".click()")
	b.until("given back", "document.querySelector('#frame-actions > [data-testid=archive]:not([data-on])')")
	// Between the widths the bar is as wide.
	b.size(800, 800, false)
	b.navigate("/a/two/")
	b.until("the bar", "innerWidth === 800 && document.querySelector('#frame-actions [data-testid=more]')")
	out = append(out, shown("#frame-actions"))

	// Narrow: the title takes the logo's room, the rest in the menu, favorites too, and f goes on.
	b.size(400, 800, false)
	b.navigate("/a/two/")
	b.until("the bar", "innerWidth === 400 && !document.getElementById('frame-actions').hidden && document.querySelector('#frame-actions [data-testid=more]')")
	out = append(out, shown("#frame-actions"))
	out = append(out, b.eval("[document.querySelector('[data-testid=brand]').getClientRects().length, document.getElementById('back').getClientRects().length, document.documentElement.scrollWidth <= innerWidth].join(' ')"))
	fav := inMenuItem("#frame-actions", "fav-item", "[i.querySelector('[data-testid=label]').textContent, i.getAttribute('aria-label')].join(' / ')")
	const starred = "document.querySelector('#frame-actions [data-testid=star]').getAttribute('aria-pressed')"
	out = append(out, b.eval(fav))
	b.pressKey("f")
	out = append(out, b.until("a favorite", starred+" === 'true' && "+fav))
	pressItem("#frame-actions", "fav-item")
	out = append(out, b.until("no favorite", starred+" === 'false' && "+fav))
	// Pressed on the toast with the focus on it, a page marked unread gives the focus to the bar's menu.
	// The menu opens within the window at any width.
	for _, width := range []int64{400, 320} {
		b.size(width, 800, false)
		b.until("the width", "innerWidth === "+strconv.FormatInt(width, 10))
		b.eval("document.querySelector('#frame-actions [data-testid=more] > summary').click()")
		out = append(out, b.eval("(() => { const r = document.querySelector('#frame-actions [data-testid=menu]').getBoundingClientRect(); return r.left >= 0 && r.right <= innerWidth && document.documentElement.scrollWidth <= innerWidth; })()"))
		b.eval("document.querySelector('#frame-actions [data-testid=more] > summary').click()")
	}
	b.size(400, 800, false)
	// The other kinds, narrow.
	s.floe(t, "archive", "kept")
	for _, slug := range []string{"one", "link", "kept"} {
		b.navigate("/a/" + slug + "/")
		b.until(slug+"'s bar", "innerWidth === 400 && document.querySelector('#frame-actions [data-testid=more]')")
		out = append(out, shown("#frame-actions"))
	}
	b.navigate("/a/one/")
	b.until("the bar", "document.querySelector('#frame-actions [data-testid=more]')")
	pressItem("#frame-actions", "mark-unread")
	b.until("unread", serverNews("one"))
	b.navigate("/a/one/")
	const toastButton = "document.querySelector('[data-testid=toast][data-page] button')"
	b.until("the toast", toastButton)
	b.eval(toastButton + ".focus(); " + toastButton + ".click()")
	out = append(out, b.until("the focus on the menu", "document.activeElement.matches('#frame-actions summary') && 'on the menu'"))
	b.navigate("/a/docs/?file=index.html")
	b.until("the bar", "!document.getElementById('frame-actions').hidden && document.querySelector('#frame-actions [data-testid=star]')")
	out = append(out, shown("#frame-actions"))
	// The foot of the modal holds its buttons at this width.
	find("two")
	b.until("the page's row", "document.getElementById('find-actions').dataset.slug === 'two'")
	out = append(out, b.eval(`(() => {
		const box = document.getElementById('find').getBoundingClientRect(), close = document.querySelector('[data-testid=find-foot] [data-close]').getBoundingClientRect();
		const acts = [...document.querySelectorAll('#find-actions > *')].filter((e) => e.getClientRects().length).map((e) => e.getBoundingClientRect());
		return [acts.every((r) => r.left >= box.left && r.right <= close.left), close.right <= box.right].join(' ');
	})()`))

	check(t, "bar", out, strings.Join([]string{
		"diff-open download open archive star more | mark-unread tag-item theme-item pin-item task-set edit delete",
		"download open archive star more | mark-unread tag-item theme-item pin-item task-set edit delete",
		"download open archive star more | tag-item pin-item task-set edit copy delete",
		"archive star more | tag-item pin-item task-set edit copy delete",
		"mark-unread select tag-item theme-item pin-item task-set edit open download delete",
		"archive star more | tag-item pin-item task-set edit copy delete",
		"diff-open download open archive star more | mark-unread tag-item theme-item pin-item task-set edit delete",
		"download open archive star more | tag-item theme-item task-set edit delete",
		"Restore / Restore Kept to the list /  / true / false",
		"Archive / Archive Kept /  / false / true", "true",
		"/api/pages/two/download ",
		"copy open | ",
		"Restore / Restore One to the list /  / true / false",
		"1 / Archived “One”Restore / true",
		"true",
		"diff-open download open archive star more | mark-unread tag-item theme-item pin-item task-set edit delete",
		"diff-open more | fav-item mark-unread tag-item theme-item pin-item task-set edit open download archive delete",
		"1 1 true",
		"Add to favorites / Add Two to favorites",
		"Remove from favorites / Remove Two from favorites",
		"Add to favorites / Add Two to favorites",
		"true", "true",
		"more | fav-item mark-unread tag-item theme-item pin-item task-set edit open download archive delete",
		"more | fav-item tag-item pin-item task-set edit copy open download archive delete",
		"more | fav-item tag-item theme-item task-set edit open download archive delete",
		"on the menu",
		"copy open | ",
		"true true",
	}, "|"))
}

// rowPoint is where in the row of the selector a press lands on its link: at its title's words, or in the margin of the row.
func rowPoint(t *testing.T, b *browser, row string, margin bool) (float64, float64) {
	t.Helper()

	box, x, y := "document.querySelector("+js(row+" [data-testid=title]")+")", 12, 6
	if margin {
		box, x, y = "document.querySelector("+js(row)+")", 4, 2
	}
	got := b.eval(fmt.Sprintf(`(() => {
		const r = %s.getBoundingClientRect(), x = r.left + %d, y = r.top + %d, on = document.elementFromPoint(x, y);
		return on?.closest('a[data-testid=title]') ? JSON.stringify({ X: x, Y: y }) : 'not its link: ' + on?.outerHTML;
	})()`, box, x, y))
	var at struct{ X, Y float64 }
	if err := json.Unmarshal([]byte(got), &at); err != nil {
		t.Fatalf("%s: %s", row, got)
	}
	return at.X, at.Y
}

// mouse sends the press or release of the main button at x, y.
func (b *browser) mouse(typ input.DispatchMouseEventType, x, y float64, count int) {
	b.t.Helper()

	if typ == input.DispatchMouseEventTypeMousePressed {
		b.painted()
	}
	if err := chromedp.Do(b.ctx, chromedp.MouseEvent(typ, x, y, chromedp.ButtonLeft, chromedp.ClickCount(count))); err != nil {
		b.t.Fatal(err)
	}
}

// asideList opens the list of asideSite with room for the page beside it.
func asideList(b *browser) {
	b.t.Helper()

	b.navigate("/")
	b.size(1440, 900, false)
	b.until("the rows", "document.querySelectorAll('#list li[data-slug]').length === 3")
}

const rowAgainPath = "location.pathname + location.search"

// TestUIRowAgain opens the page of the row picked whole as it is pressed again, at its title or in its margin, or pressed double, in
// the order of the list; a press that only closes a menu, or may have missed a box with rows selected, moves nowhere. So does the
// first, a modified one, which is the link's, and j and k finding no row past the one picked.
func TestUIRowAgain(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := asideSite(t)
	b := s.b
	asideList(b)
	b.eval(readMarks)
	const a2 = "#list li[data-slug=a2]"
	x, y := rowPoint(t, b, a2, false)
	b.click(x, y)
	b.until("picked", besideSlug+" === 'a2'")
	out := make([]string, 0, 7)
	out = append(out, b.eval(rowAgainPath))
	// Read and drawn so, as a redraw closes a menu open.
	b.until("read", "document.querySelector("+js(a2)+").dataset.unread !== 'true'")
	b.marksDone(1)
	b.untilRead(b.reread(), "drawn", "true")

	// A menu open, the press closes it.
	const menuOpen = "document.querySelector('#list [data-testid=more][open]')"
	b.eval("document.querySelector(" + js(a2+" [data-testid=more] > summary") + ").click()")
	b.until("the menu open", "document.querySelector("+js(a2+" [data-testid=delete]")+").checkVisibility()")
	b.click(x, y)
	b.until("the menu closed", "!"+menuOpen)
	out = append(out, b.eval(rowAgainPath))
	// A delete waiting in it, the same.
	pressMenuItem(b, a2, "delete")
	b.until("armed", menuOpen+" && document.querySelector("+js(a2+" [data-testid=delete][data-armed=true]")+")")
	b.click(x, y)
	b.until("the menu closed again", "!"+menuOpen)
	out = append(out, b.eval(rowAgainPath))
	// A row selected, the press leaves it selected.
	b.eval("document.querySelector('#list li[data-slug=a1] [data-testid=sel]').click()")
	x, y = rowPoint(t, b, a2, false)
	b.click(x, y)
	out = append(out, b.eval("[document.querySelectorAll('#list [data-testid=sel]:checked').length, "+besideSlug+", "+rowAgainPath+"].join(' ')"))
	b.eval("document.querySelector('#list li[data-slug=a1] [data-testid=sel]').click()")
	// Modified, it is left to the link.
	out = append(out, b.eval(`(() => {
		const link = document.querySelector('#list li[data-slug=a2] [data-testid=title]');
		let kept;
		const note = (e) => { kept = !e.defaultPrevented; e.preventDefault(); };
		document.addEventListener('click', note, { once: true });
		link.dispatchEvent(new MouseEvent('click', { ctrlKey: true, bubbles: true, cancelable: true }));
		return kept;
	})()`))
	// No row past the one picked: k stays, and does not go whole.
	b.eval("document.querySelector('#list li[data-slug=a3] [data-testid=title]').focus()")
	b.pressKey("j")
	b.until("back beside", besideSlug+" === 'a2'")
	b.pressKey("k")
	b.until("the first", besideSlug+" === 'a3'")
	b.pressKey("k")
	out = append(out, b.eval(besideSlug+" + ' ' + "+rowAgainPath))

	// Pressed again, whole, the list's order kept for j.
	x, y = rowPoint(t, b, a2, false)
	b.click(x, y)
	b.until("a2 again", besideSlug+" === 'a2'")
	b.click(x, y)
	b.until("whole", "location.pathname === '/a/a2/'")
	// The URL is the new page's before its script has kept where it is; the frame's src comes after that
	b.until("its bar", "document.getElementById('frame').hasAttribute('src')")
	out = append(out, b.eval("sessionStorage.getItem('pages.order')"))
	b.pressKey("j")
	b.until("on through the list", "location.pathname === '/a/a1/'")

	// In the margin of the row.
	asideList(b)
	x, y = rowPoint(t, b, a2, true)
	b.click(x, y)
	b.until("picked by the margin", besideSlug+" === 'a2'")
	b.click(x, y)
	b.until("whole by the margin", "location.pathname === '/a/a2/'")

	// Pressed double, as a person does.
	asideList(b)
	x, y = rowPoint(t, b, "#list li[data-slug=a3]", false)
	b.click(x, y)
	b.painted()
	if err := chromedp.Do(b.ctx, chromedp.MouseClickXY(x, y, chromedp.ClickCount(2))); err != nil {
		t.Fatal(err)
	}
	b.until("whole by a double press", "location.pathname === '/a/a3/'")

	check(t, "row again", out, strings.Join([]string{
		"/", "/", "/", "1 a2 /", "true", "a3 /", `{"order":["a3","a2","a1"],"shown":["a3","a2","a1"],"back":"/","at":{"slug":"a2","file":"","v":0}}`,
	}, "|"))
}

// TestUIRowAgainKinds opens a folder's page whole by its own URL, closes a folder's tree as its row is pressed again, and leaves a
// link whose file is gone, which has nowhere to go, and the order with it.
func TestUIRowAgainKinds(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.write(t, "docs/guide/setup.html", "<title>t</title><p>setup</p>")
	s.write(t, "docs/index.html", "<title>t</title><p>index</p>")
	s.floe(t, "publish", "--link", filepath.Join(s.Env["HOME"], "docs"), "--slug", "docs")
	gone := s.write(t, "away/gone.html", "<title>t</title><p>gone</p>")
	s.floe(t, "publish", "--link", gone, "--slug", "gone")
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	b.navigate("/")
	b.size(1440, 900, false)
	const folder, missing = "#list li[data-slug=docs]", "#list li[data-slug=gone]:has([data-testid=miss])"
	b.until("the rows", "document.querySelector("+js(folder)+") && document.querySelector("+js(missing)+")")
	const picked = "document.querySelector('#list li[aria-current=true]')"

	x, y := rowPoint(t, b, missing, false)
	b.click(x, y)
	b.until("the words why", picked+"?.dataset.slug === 'gone' && !document.getElementById('read-blank').hidden")
	b.click(x, y)
	out := make([]string, 0, 2)
	out = append(out, b.eval(picked+".dataset.slug + ' ' + "+rowAgainPath+" + ' ' + sessionStorage.getItem('pages.order')"))

	const expanded = ".querySelector('[data-testid=title]').getAttribute('aria-expanded')"
	x, y = rowPoint(t, b, folder, false)
	b.click(x, y)
	b.until("the tree", "document.querySelector("+js(folder)+")"+expanded+" === 'true' && document.querySelector('#list li[data-testid=tree][data-file]')")
	const page = "#list li[data-testid=tree][data-file='index.html']"
	px, py := rowPoint(t, b, page, false)
	b.click(px, py)
	b.until("the page picked", picked+"?.dataset.file === 'index.html'")
	x, y = rowPoint(t, b, folder, false)
	b.click(x, y)
	b.until("the tree closed", "document.querySelector("+js(folder)+")"+expanded+" === 'false'")
	out = append(out, b.eval(rowAgainPath))
	b.click(x, y)
	b.until("the tree back", "document.querySelector("+js(folder)+")"+expanded+" === 'true' && document.querySelector("+js(page)+")")
	px, py = rowPoint(t, b, page, false)
	b.click(px, py)
	b.until("whole by its own URL", "location.pathname + location.search === '/a/docs/?file=index.html'")

	check(t, "row again, kinds", out, "gone / null|/")
}

// TestUIRowAgainPressed holds off drawing the rows anew from a poll's answer while one is pressed, as the press would be lost, for 2
// seconds, as a press may never tell its release, and draws them at the next poll. A press outside the list holds nothing, nor is an
// action's own load held, and what is held does not reach items.
func TestUIRowAgainPressed(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := asideSite(t)
	b := s.b
	asideList(b)
	b.eval(readMarks)
	// Every page read and drawn so, so that nothing else is left to change the answer.
	for i, slug := range []string{"a1", "a3", "a2"} {
		x, y := rowPoint(t, b, "#list li[data-slug="+slug+"]", false)
		b.click(x, y)
		b.until(slug+" read", besideSlug+" === "+js(slug)+" && document.querySelector('#list li[data-slug="+slug+"]').dataset.unread !== 'true'")
		b.marksDone(i + 1)
	}
	b.untilRead(b.reread(), "drawn", "true")
	// a1's title changes on the server (anew, which returns the JS that keeps the row of a2) before each read checked, so that its
	// answer differs from what is drawn. window.served is the title the last answer gave, read from a copy of it.
	b.eval(`(() => { const f = window.fetch; window.a1Title = null; window.served = null;
		window.fetch = async (url, init) => {
			const r = await f(url, init);
			if (init?.method || new URL(String(url), location.href).pathname !== '/api/pages') return r;
			window.served = (await r.clone().json()).find((a) => a.slug === 'a1').title;
			return r;
		}; })()`)
	st := store.New(s.Env["FLOE_DIR"])
	retitle := func(title string) {
		t.Helper()

		if _, err := st.Edit("a1", store.Edit{Title: &title}); err != nil {
			t.Fatal(err)
		}
	}
	turn := 0
	nextTitle := func() string {
		t.Helper()

		turn++
		title := "a1 #" + strconv.Itoa(turn)
		retitle(title)
		return title
	}
	anew := func() string {
		t.Helper()

		return "window.rowBefore = document.querySelector('#list li[data-slug=a2]'); window.a1Title = " + js(nextTitle()) + "; "
	}
	const (
		poll    = "document.dispatchEvent(new Event('visibilitychange'))"
		a1Drawn = "document.querySelector('#list li[data-slug=a1] [data-testid=title]').textContent"
	)
	// After a read of an answer anew: true if it left the rows as they are, the answer not drawn.
	held := func() string {
		t.Helper()
		got := b.eval("window.served !== window.a1Title ? 'no answer anew was read' : !document.getElementById('error').hidden ? document.getElementById('error').textContent : String(window.rowBefore.isConnected && " + a1Drawn + " !== window.a1Title)")
		if got != "true" && got != "false" {
			t.Fatal(got)
		}
		return got
	}
	polled := func() string {
		t.Helper()
		b.untilRead(b.acted(anew()+poll), "the poll read", "true")
		return held()
	}
	const a3 = "#list li[data-slug=a3]"
	out := make([]string, 0, 8)

	// Released, nothing is held.
	x, y := rowPoint(t, b, "#list li[data-slug=a1]", false)
	b.click(x, y)
	b.until("a1 picked", besideSlug+" === 'a1'")
	out = append(out, polled())

	x, y = rowPoint(t, b, a3, false)
	// A press on the row of another, the focus elsewhere: the poll's answer held back; released, the press counts and the next poll draws.
	b.eval("document.querySelector('#list li[data-slug=a1] [data-testid=title]').focus()")
	b.pollEnded()
	b.mouse(chromedp.MousePressed, x, y, 1)
	b.pressList(anew() + poll)
	out = append(out, held())
	// What is held does not reach what the page holds of the list either: the answers go back to what is drawn, and the search,
	// drawn from what the page holds, lists the title drawn, not the one held.
	retitle(b.eval(a1Drawn))
	b.mouse(chromedp.MouseReleased, x, y, 1)
	b.until("picked", besideSlug+" === 'a3'")
	pressFindOpen(b)
	out = append(out, b.eval("[...document.querySelectorAll('#find-list li [data-testid=title]')].map((e) => e.textContent).filter((t) => t.startsWith('a1')).join()"))
	escapeDialog(b, "find")
	// The title the next poll answers, drawn: a build may keep the rows it draws again
	b.eval("window.a1Title = " + js(nextTitle()))
	b.until("drawn by the next poll", a1Drawn+" === window.a1Title")

	// Outside the list, in the top document: an iframe's press does not reach it.
	r := b.eval(`(() => {
		const r = document.getElementById('read-head').getBoundingClientRect();
		const x = r.left + r.width / 2, y = r.top + 2, on = document.elementFromPoint(x, y);
		return on.closest('#read-head') && !on.closest('a, button, summary') ? JSON.stringify({ X: x, Y: y }) : 'on ' + on.outerHTML;
	})()`)
	var at struct{ X, Y float64 }
	if err := json.Unmarshal([]byte(r), &at); err != nil {
		t.Fatal(r)
	}
	b.mouse(chromedp.MousePressed, at.X, at.Y, 1)
	out = append(out, polled())
	b.mouse(chromedp.MouseReleased, at.X, at.Y, 1)

	// A press whose release may never come: an action's load goes on (the star's, pressed again after to put it back), and the
	// poll's only until the time is up, 2 s from the press (taken right after it).
	const star = "document.querySelector('#list li[data-slug=a2] [data-testid=star]')"
	starred := b.eval(star + ".getAttribute('aria-pressed')")
	b.pollEnded()
	b.pressList(anew() + "window.pressAt = performance.now(); " + star + ".click()")
	out = append(out, held())
	b.pollEnded()
	b.pressList(anew() + "window.pressAt = performance.now(); " + poll)
	out = append(out, held())
	b.until("the time up", "performance.now() - window.pressAt > 2000")
	out = append(out, polled())
	b.eval(star + ".click()")
	b.until("the star put back", star+".getAttribute('aria-pressed') === "+js(starred))
	b.letGoList()

	// Pressed again with the poll in the middle of it, the page opens whole.
	x, y = rowPoint(t, b, a3, false)
	b.pollEnded()
	b.mouse(chromedp.MousePressed, x, y, 1)
	b.pressList(anew() + poll)
	out = append(out, held())
	b.mouse(chromedp.MouseReleased, x, y, 1)
	b.until("whole", "location.pathname === '/a/a3/'")

	check(t, "row again, pressed", out, "false|true|a1 #1|false|false|true|false|true")
}
