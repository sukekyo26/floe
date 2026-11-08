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
		b.moving("document.body.dispatchEvent(new KeyboardEvent('keydown', { key: '" + k + "', bubbles: true }))")
	}
	out := make([]string, 0, 24)
	out = append(out, b.eval(rowsDrawn))
	out = append(out, b.eval("[...document.querySelectorAll('#list li.group')].slice(0, 3).map((li) => li.textContent).join(',')"))
	// The bar and the head count the new off the archive, as the shelf does.
	out = append(out, b.eval("[document.getElementById('news-n').textContent, document.getElementById('count').textContent, document.querySelector('.views [data-view=new] .n').textContent].join(' / ')"))
	// The words of the description are the title's alone, searched too.
	out = append(out, b.eval("document.querySelector('#list li[data-slug=v] .title').title.split('\\n').slice(0, 3).join('\\n')"))
	b.eval("(() => { const q = document.getElementById('q'); q.value = 'the words'; q.dispatchEvent(new Event('input')); })()")
	out = append(out, b.eval("[...document.querySelectorAll('#list > li:not(.group)')].map((li) => li.dataset.slug).join('') + ' ' + location.search"))
	b.eval("(() => { const q = document.getElementById('q'); q.value = ''; q.dispatchEvent(new Event('input')); })()")

	// Nothing chosen at first, as opening one is reading it: the words how.
	const reader = "[document.getElementById('read-blank').hidden, document.getElementById('read-frame').hidden, readLoaded, document.getElementById('read-title').textContent, document.querySelector('#list li.picked')?.dataset.slug].join(' ')"
	out = append(out, b.eval(reader))
	// Pressed, a row opens beside the list, the tab where it is; the title over it leads to it whole.
	b.eval("document.querySelector('#list li[data-slug=r] .title').click()")
	out = append(out, b.eval(reader), b.eval("location.pathname + ' ' + document.getElementById('read-title').getAttribute('href')"))
	// The bar over it: its star, the page alone and the same menu as a row's.
	out = append(out, b.eval("[...document.querySelectorAll('#read-actions .menu > :not(.select)')].filter((e) => !e.hidden).length === [...document.querySelectorAll('#list li[data-slug=r] .menu > :not(.select)')].filter((e) => !e.hidden).length"))
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

	// The view of the new: a row read stays on it, as a ring, and j goes on from it; the view let go, it is gone on the way back.
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
	b.eval("document.querySelector('.views [data-view=new]').click(); document.querySelector('.views [data-view=new]').click()")
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
	b.moving("document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))")
	// The bar too: the logo's link is drawn with it, once the list is read.
	b.until("the page whole", "location.pathname === '/a/v/' && document.getElementById('frame-title').textContent")
	out = append(out, b.eval("sessionStorage.getItem('artifacts.order')"), b.eval("document.querySelector('.brand').getAttribute('href')"))
	b.stub(strings.Replace(listList, "news: 'version', ", "", 1))
	b.until("the bar", "document.getElementById('frame-title').textContent")
	key("j")
	// The bar, not the URL alone: a key goes nowhere until the list is read.
	b.until("the next page", "location.pathname === '/a/n/' && document.getElementById('frame-title').textContent")
	key("k")
	b.until("the page before", "location.pathname === '/a/v/'")
	// From the URL, on no such list: the list it was last on, else all of its kind.
	b.eval(`sessionStorage.setItem('artifacts.order', '{"order":[],"shown":[],"back":"/"}'); sessionStorage.setItem('artifacts.list', '')`)
	b.navigate("/a/r/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	key("j")
	b.until("the next page of all", "location.pathname === '/a/l/'")
	// An end stays where it is.
	b.navigate("/a/v/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	b.eval(`sessionStorage.setItem('artifacts.order', '{"order":["v"],"shown":["v"],"back":"/"}')`)
	b.navigate("/a/v/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	key("j")
	key("k")
	out = append(out, b.eval("location.pathname"))

	// The path is cut at its head, the file's name whole at the box's end and its own order kept, marks and numbers too.
	b.navigate("/")
	out = append(out, b.eval(`(() => {
		const box = document.querySelector('#list li[data-slug=d] .path'), text = box.querySelector('bdi').firstChild;
		const at = (from, to) => { const r = document.createRange(); r.setStart(text, from); r.setEnd(text, to); return r.getBoundingClientRect(); };
		const n = text.length, b = box.getBoundingClientRect();
		return [box.scrollWidth > box.clientWidth, Math.abs(at(n - 1, n).right - b.right) <= 1, at(0, 1).left < b.left, at(n - 11, n - 10).left < at(n - 1, n).left,
			// Pointed at, the row's title tells it whole.
			document.elementFromPoint(b.left + b.width / 2, b.top + b.height / 2).closest('a.title').title.split('\n').includes(text.data)].join(' ');
	})()`))
	// Beside the list above 56rem, the shelf a row under 70rem, the filter right after the views, the bar's count gone.
	b.size(1000, 700, false)
	const middle = "[getComputedStyle(document.getElementById('reader')).display, getComputedStyle(document.getElementById('shelf-more')).display, getComputedStyle(document.getElementById('news')).display, " +
		"document.getElementById('narrow-filter').getBoundingClientRect().left - document.querySelector('.views [data-view=archived]').getBoundingClientRect().right < 10].join(' ')"
	out = append(out, b.eval(middle))
	b.eval("document.querySelector('#list li[data-slug=r] .title').click()")
	out = append(out, b.eval("[location.pathname, document.getElementById('read-frame').hidden].join(' ')"))
	// At 56rem or less, one column, a row opening whole with the way back, the page's buttons and the search's field still there.
	b.size(800, 700, false)
	out = append(out, b.eval("[getComputedStyle(document.getElementById('reader')).display, getComputedStyle(document.querySelector('#q')).color !== 'rgba(0, 0, 0, 0)'].join(' ')"))
	b.moving("document.querySelector('#list li[data-slug=r] .title').click()")
	b.until("the row whole", "location.pathname === '/a/r/'")
	out = append(out, b.until("the bar", "document.getElementById('frame-title').textContent && [getComputedStyle(document.getElementById('back')).display, getComputedStyle(document.getElementById('frame-actions')).display].join(' ')"))
	b.unsize()

	// A dialog's words are a paragraph, not a row's first column: above the next field, however long.
	b.navigate("/")
	b.size(400, 860, true)
	b.eval("document.getElementById('add-open').click()")
	out = append(out, b.eval("(() => { const l = document.querySelector('#add .lead').getBoundingClientRect(), f = document.querySelector('#add .field').getBoundingClientRect(); const e = document.querySelector('#add .lead'); return [l.bottom <= f.top && e.scrollHeight <= e.clientHeight + 1, getComputedStyle(e).display].join(' '); })()"))
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
	b.moving("document.querySelector('#list li[data-slug=r] .title').click()")
	b.until("the row whole", "location.pathname === '/a/r/'")
	out = append(out, b.until("the bar", "document.getElementById('frame-title').textContent && [getComputedStyle(document.getElementById('back')).display, getComputedStyle(document.querySelector('#frame-actions .star')).display].join(' ')"))
	b.unsize()

	check(t, "list", out, strings.Join([]string{
		"#|v dot 700 ver:v3 · new version kindof:design proj proj-tint:p tags:t|#|n dot 700 kindof:report proj proj-tint:q|#|r ring 500 kindof:report proj proj-tint:p|l mark 500 kindof:Link · HTML path:/tmp/x/l.html|#|d mark 500 kindof:Link · handoff path:/home/you/work/a (1)/v2.0/data-set_#1/deep/deep/deep/deep/deep/deep/deep/deep/deep/name-2.yaml|#|f mark 500 kindof:Folder path:/tmp/x/f|#|g mark 500 kindof:Link · HTML miss:File not found",
		"Today,Yesterday,Last 7 days", "All unread 2 / Newest first · 2 unread / 2",
		"Version\nthe words\np", "v ?q=the+words",
		"false true about:blank  ", "true false /a/r/?raw Read r", "/ /a/r/", "true",
		"true false /a/l/?raw Link l", "true false /a/n/?raw New n New",
		"true true true", "false true about:blank New ",
		"vn ?view=new", "vn", "All unread 1 / 1", "New", "n", "true",
		`{"order":["v","n"],"shown":["v","n"],"back":"/?view=new","at":{"slug":"v","file":"","v":0}}`, "/?view=new", "/a/v/",
		"true true true true true", "flex none none true", "/ false", "none true", "flex flex", "true block", "true",
		"none none flex none true", "flex none",
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
	const state = `[[...document.querySelectorAll('.views button[aria-pressed=true], #shelf-more .chip[aria-pressed=true]')].map((c) => c.dataset.view ?? c.dataset.project ?? c.dataset.kind ?? c.dataset.template ?? c.dataset.tag).join('+'),
		[...document.querySelectorAll('#list > li:not(.group)')].map((li) => li.dataset.slug).sort().join(''), location.search, document.getElementById('q').value].join(' | ')`
	press := func(selector string) string {
		b.eval("document.querySelector('" + selector + "').click()")
		return b.eval(state)
	}
	const (
		all   = ".views [data-view=all]"
		fresh = ".views [data-view=new]"
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
	press(".views [data-view=fav]")
	step(press(".views [data-view=fav]"), "floe | ab | ?project=floe | ")
	press(".views [data-view=archived]")
	step(press(".views [data-view=archived]"), "floe | ab | ?project=floe | ")
	// Unlit, All lets go of a view and every kind of pick, and a reload lights it.
	b.navigate("/?view=fav&project=p&kind=page")
	step(press(all), "all | abcd |  | ")
	b.navigate("/?template=report&tag=x")
	step(b.eval(state), "report+x | a | ?template=report&tag=x | ")
	step(press(all), "all | abcd |  | ")
	b.navigate("/")
	step(b.eval(state), "all | abcd |  | ")
	// The search stays, and All stays lit with it.
	b.eval("(() => { const q = document.getElementById('q'); q.value = 'A'; q.dispatchEvent(new Event('input')); })()")
	step(b.eval(state), "all | a | ?q=A | A")
	press(floe)
	step(press(all), "all | a | ?q=A | A")
	// Lit, All does nothing: not even to the selection.
	step(b.eval("selected.add('a'); document.querySelector('"+all+"').click(); selected.has('a')"), "true")
	// The way out of no match keeps the view, and clears the search and the picks.
	b.navigate("/?view=fav&project=p&q=none")
	step(press("#clear"), "fav | d | ?view=fav | ")

	// Two columns: the picks are in a dialog, and the button to it, with their number, is lit for them.
	b.navigate("/?project=floe&tag=x")
	b.size(900, 800, false)
	const narrow = `[...document.querySelectorAll('.views button')].filter((b) => b.getAttribute('aria-pressed') === 'true').map((b) => b.dataset.view).join('+') + ' | ' +
		document.getElementById('narrow-filter').classList.contains('narrowed') + ' | ' + document.getElementById('narrow-filter-name').textContent`
	step(b.eval(narrow), " | true | Filter 2")
	b.eval("document.querySelector('" + all + "').click()")
	step(b.eval(narrow), "all | false | Filter")
	b.eval("document.querySelector('" + fresh + "').click()")
	b.eval("document.getElementById('narrow-filter').click(); document.querySelector('" + floe + "').click(); document.getElementById('filter-dialog').close()")
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
		const lefts = [...document.querySelectorAll('.views .label, #chips .name, #kinds .name')].map((e) => e.getBoundingClientRect().left);
		return lefts.length + ' ' + (Math.max(...lefts) - Math.min(...lefts) <= 0.5);
	})()`))
	// The archive's mark and the kinds' are quiet, not the star's color.
	out = append(out, b.eval(`[...document.querySelectorAll('.views [data-view=archived] svg, #kinds svg')].map((e) => getComputedStyle(e).color === getComputedStyle(document.querySelector('.views .n')).color).join(' ')`))
	// A link and a folder have the mark of their rows, and a page its own.
	out = append(out, b.eval(`[...document.querySelectorAll('#kinds .chip')].map((c) => {
		const path = c.querySelector('path').getAttribute('d'), row = document.querySelector('#list li[data-slug=' + { link: 'l', folder: 'f' }[c.dataset.kind] + '] .lead path');
		return c.dataset.kind + ':' + (row ? row.getAttribute('d') === path : Boolean(path));
	}).join(' ')`))
	// The row holds all, its button to the filters within the window.
	for _, w := range []int64{1000, 640, 400, 360} {
		b.size(w, 700, false)
		for _, at := range []string{"/?view=archived&project=floe", "/?view=new&project=floe"} {
			b.navigate(at)
			out = append(out, b.eval(`(() => {
				const v = document.querySelector('.views');
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
	out = append(out, b.eval("document.querySelector('.views [data-view=archived] .label').getBoundingClientRect().width <= 1"), named())
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
	const counts = `['.views button[data-view]', '#chips .chip', '#kinds .chip', '#templates .chip', '#tags .chip'].map((part) => [...document.querySelectorAll(part)]
		.map((c) => (c.dataset.view ?? c.dataset.project ?? c.dataset.kind ?? c.dataset.template ?? c.dataset.tag) + ':' + c.querySelector('.n').textContent).join(' ')).join(' / ')`
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
	const unread = "document.getElementById('news-n').textContent + ' / ' + document.getElementById('count').textContent + ' / ' + document.querySelector('.views [data-view=new] .n').textContent"
	out = append(out, b.eval(unread))
	b.navigate("/?project=floe")
	out = append(out, b.eval(unread))
	// The bar pressed lets go of the picks and the search: as many rows as all three say, and the view stays on pressed again.
	b.navigate("/?view=fav&project=floe&kind=page&template=report&tag=x&q=alpha")
	b.eval("document.getElementById('news').click()")
	const led = "[...document.querySelectorAll('#list > li:not(.group)')].length + ' ' + " + unread + " + ' ' + location.search + ' [' + document.getElementById('q').value + ']'"
	out = append(out, b.eval(led))
	b.eval("document.getElementById('news').click()")
	out = append(out, b.eval(led))
	// A read row that stays on the view of the unread is not counted.
	b.navigate("/?view=new")
	b.eval("window.list = JSON.stringify(JSON.parse(window.list).map((x) => x.slug === 'a' ? { ...x, news: undefined } : x)); drawn = ''; load()")
	b.until("the row read", "document.querySelector('#list li[data-slug=a] .ring')")
	out = append(out, b.eval("[...document.querySelectorAll('#list > li:not(.group)')].length + ' ' + document.querySelector('.views [data-view=new] .n').textContent"))
	// The bar pressed there: as many rows as it says, the read row gone.
	b.eval("document.getElementById('news').click()")
	out = append(out, b.eval("[...document.querySelectorAll('#list > li:not(.group)')].length + ' ' + "+unread+" + ' ' + location.search"))
	b.eval("window.list = JSON.stringify(JSON.parse(window.list).map((x) => x.slug === 'a' ? { ...x, news: 'new' } : x)); drawn = ''; load()")
	b.until("the row unread", "document.querySelector('#list li[data-slug=a] .dot')")
	// A row at 0 is quieter, but for the one picked, which reads as picked.
	b.navigate("/?view=archived&project=floe&kind=folder")
	out = append(out, b.eval(`(() => {
		const name = (c) => getComputedStyle(c.querySelector('.name')).color, quiet = getComputedStyle(document.querySelector('.views .n')).color;
		const folder = document.querySelector('#kinds [data-kind=folder]');
		return [folder.getAttribute('aria-pressed'), folder.querySelector('.n').textContent, name(folder) === quiet].join(' ');
	})()`))
	b.navigate("/?q=alpha")
	out = append(out, b.eval(`(() => {
		const link = document.querySelector('#kinds [data-kind=link]'), page = document.querySelector('#kinds [data-kind=page]'), quiet = getComputedStyle(document.querySelector('.views .n')).color;
		const words = (e, s) => getComputedStyle(e.querySelector(s)).color === quiet;
		return [link.querySelector('.n').textContent, words(link, '.name'), words(page, '.name'), words(document.querySelector('.views [data-view=archived]'), '.label')].join(' ');
	})()`))
	// In the dialog of two columns, the same.
	b.size(900, 800, false)
	b.navigate("/?project=floe")
	b.eval("document.getElementById('narrow-filter').click()")
	out = append(out, b.eval("document.querySelector('#filter-box #kinds [data-kind=link] .n').textContent + ' ' + document.querySelector('#filter-box #chips [data-project=p] .n').textContent"))
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

		b.eval("document.querySelector('#list li[data-slug=" + slug + "] .title').click()")
	}
	loaded := func(what, path string) {
		t.Helper()

		b.until(what, "window.heard.includes("+js(path)+")")
	}
	// A page in the iframe tells the path it is at as it loads.
	b.eval("window.heard = []; addEventListener('message', (e) => { if (e.data?.artifactPath) window.heard.push(e.data.artifactPath); })")
	out := make([]string, 0, 8)
	before := b.eval("history.length")
	for _, slug := range []string{"a1", "a2", "a3"} {
		press(slug)
		loaded("the page "+slug, "/a/"+slug+"/")
	}
	out = append(out, b.eval("history.length === "+before), b.eval("document.querySelector('#list li.picked').dataset.slug"))

	// Opened from the bar over the page, the edit dialog gives the focus back there; from a row, to the row's.
	b.eval("document.querySelector('#read-actions summary').click(); document.querySelector('#read-actions .edit').click()")
	b.eval("document.querySelector('#edit-dialog [data-close]').click()")
	out = append(out, b.until("the focus back", "document.activeElement.matches('#read-actions summary') && 'bar'"))
	// The list read again, as after a read, draws the bar anew with the focus where it was.
	b.eval("drawn = ''; load()")
	if got := b.eval("document.activeElement.matches('#read-actions summary')"); got != "true" {
		t.Errorf("the focus after the bar was drawn anew: %s", got)
	}
	b.eval("document.querySelector('#list li[data-slug=a1] summary').click(); document.querySelector('#list li[data-slug=a1] .edit').click()")
	b.eval("document.querySelector('#edit-dialog [data-close]').click()")
	out = append(out, b.until("the focus back", "document.activeElement.matches('#list li[data-slug=a1] summary') && 'row'"))

	// Renamed from its row, the focus goes back to the row under its new name.
	b.eval(`document.querySelector('#list li[data-slug=a2] summary').click(); document.querySelector('#list li[data-slug=a2] .edit').click();
		document.getElementById('edit-slug').value = 'a4'; document.getElementById('edit').requestSubmit()`)
	out = append(out, b.until("the focus on the row renamed", "document.activeElement.matches('#list li[data-slug=a4] summary') && 'renamed'"))

	// A link's file gone is not loaded beside the list, and is once it is back.
	file := s.write(t, "aside/h.html", "<title>h</title><p>h</p>")
	s.floe(t, "publish", "--link", file, "--slug", "h")
	b.until("the link", "document.querySelector('#list li[data-slug=h]')")
	if err := os.Rename(file, file+".moved"); err != nil {
		t.Fatal(err)
	}
	b.until("the file gone", "document.querySelector('#list li[data-slug=h] .miss')")
	press("h")
	out = append(out, b.eval("[document.getElementById('read-frame').hidden, readLoaded].join(' ')"))
	if err := os.Rename(file+".moved", file); err != nil {
		t.Fatal(err)
	}
	b.until("the file back", "!document.querySelector('#list li[data-slug=h] .miss')")
	loaded("the page of the file back", "/a/h/")
	out = append(out, b.eval("document.getElementById('read-frame').hidden"))

	// One column: nothing stays alive beside the list. A new version is not told read, nor is anything chosen back.
	b.eval("window.heard = []")
	press("a1")
	loaded("the page a1", "/a/a1/")
	b.size(800, 700, false)
	b.until("one column", "document.querySelector('#list li.picked') === null")
	out = append(out, b.eval("[picked, readLoaded].join(' ')"))
	before = s.floeRead(t)
	s.floe(t, "publish", s.page(t, "aside/a1.yaml", "a1", "<p id=\"p\">a1 again</p>"), "--slug", "a1")
	b.until("the new version", "document.querySelector('#list li[data-slug=a1] .ver')")
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
		b.eval("sessionStorage.setItem('artifacts.order', " + js(`{"order":`+order+`,"shown":`+order+`,"back":"/"}`) + "); sessionStorage.setItem('artifacts.list', " + js(list) + ")")
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
	b.eval(`sessionStorage.setItem('artifacts.order', '{"order":["v","n"],"shown":["v","n"],"back":"/"}')`)
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
	b.eval("document.documentElement.style.fontSize = '20px'; layoutColumns()")
	out = append(out, b.eval("getComputedStyle(document.documentElement).getPropertyValue('--shelf-width') + ' ' + getComputedStyle(document.documentElement).getPropertyValue('--list-width') + ' ' + document.getElementById('list-grip').getAttribute('aria-valuemin')"))
	b.eval("document.documentElement.style.fontSize = ''; layoutColumns()")
	const widths = "[getComputedStyle(document.documentElement).getPropertyValue('--shelf-width'), getComputedStyle(document.documentElement).getPropertyValue('--list-width'), Math.round(document.querySelector('.shelf').getBoundingClientRect().width), Math.round(document.getElementById('listcol').getBoundingClientRect().width)].join(' ')"
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
	const cut = "(() => { const p = document.querySelector('#list li[data-slug=long] .path'); return p.scrollWidth - p.clientWidth; })()"
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
		const menu = within(document.querySelector('#list li[data-slug=long] .menu'));
		document.querySelector('#list li[data-slug=long] summary').click();
		document.querySelector('#list li[data-slug=long] .sel').click();
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
	b.eval("document.querySelector('#list li[data-slug=a1] .title').click()")
	mouse("list-grip", 900, false)
	out = append(out, b.eval("getComputedStyle(document.getElementById('read-frame')).pointerEvents + ' ' + document.getElementById('list-grip').classList.contains('dragging')"))
	if err := chromedp.Do(b.ctx, chromedp.MouseEvent(chromedp.MouseReleased, 900, 300, chromedp.ButtonLeft, chromedp.ClickCount(1))); err != nil {
		t.Fatal(err)
	}
	out = append(out, b.until("the edge let go", "!document.getElementById('list-grip').classList.contains('dragging') && getComputedStyle(document.getElementById('read-frame')).pointerEvents"))
	// Folded and shown again, the focus on the other button, the two in one foot.
	const shelf = "[getComputedStyle(document.querySelector('.shelf')).display, document.getElementById('shelf-show').hidden, document.getElementById('shelf-grip').hidden, document.activeElement.id, document.getElementById('shelf-show').getBoundingClientRect().right <= document.getElementById('list-hide').getBoundingClientRect().left].join(' ')"
	b.eval("document.getElementById('shelf-hide').click()")
	out = append(out, b.eval(shelf), b.eval("Math.round(document.getElementById('listcol').getBoundingClientRect().left)"))
	// Folded, the head says what the filters narrow the list to, in its line beside the rest; pressed, they show.
	const what = "[document.getElementById('shelf-what').hidden, document.getElementById('shelf-what').textContent, document.querySelector('.lhead').scrollWidth <= document.querySelector('.lhead').clientWidth].join(' ')"
	out = append(out, b.eval(what))
	b.eval("setColumn('list', 500); setView('archived'); refilter({ kind: 'link', tag: 'x' })")
	out = append(out, b.eval(what), b.eval("(() => { const e = document.getElementById('shelf-what'), r = document.createRange(); r.selectNodeContents(e); return r.getBoundingClientRect().width <= e.getBoundingClientRect().width + 0.5; })()"))
	b.eval("setColumn('list', 740)")
	b.eval("document.getElementById('shelf-what').click()")
	out = append(out, b.eval("getComputedStyle(document.querySelector('.shelf')).display"))
	b.eval("setView('all'); refilter({ kind: '', tag: '' }); document.getElementById('shelf-hide').click()")
	// Folded across the widths: the row of the views and no button under 70rem, as a line of a list; folded still above it.
	b.size(1000, 700, false)
	const folded = "[getComputedStyle(document.querySelector('.shelf')).display, document.querySelector('.views').getClientRects().length, document.getElementById('shelf-show').getClientRects().length, document.getElementById('shelf-what').getClientRects().length, parseFloat(getComputedStyle(document.querySelector('.lhead')).paddingLeft), getComputedStyle(document.getElementById('manage')).gridTemplateColumns.split(' ').length].join(' ')"
	out = append(out, b.until("the row of the filters", "matchMedia('(max-width: 70rem)').matches && "+folded))
	b.size(1600, 900, false)
	out = append(out, b.until("the filters folded", "!matchMedia('(max-width: 70rem)').matches && "+folded))
	// Kept across a reload, folded too.
	kept := b.eval("localStorage.getItem('list.columns')")
	b.navigate("/")
	out = append(out, b.eval("localStorage.getItem('list.columns') === "+js(kept)), b.eval("getComputedStyle(document.querySelector('.shelf')).display"))
	b.eval("document.getElementById('shelf-show').click()")
	out = append(out, b.eval("[getComputedStyle(document.querySelector('.shelf')).display, document.activeElement.id].join(' ')"), b.eval(widths))
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
	b.eval("document.querySelector('#list li[data-slug=a1] .title').focus(); document.getElementById('list-hide').click()")
	out = append(out, b.eval(list), b.eval("document.getElementById('read-blank').hidden + ' ' + document.getElementById('read-actions').hidden + ' ' + document.getElementById('read-title').hidden"))
	// A page chosen and gone leaves no link to it in the head.
	b.eval("document.querySelector('#list li[data-slug=a2] .title').click()")
	b.eval("window.fetch = ((f) => async (u, i) => { const r = await f(u, i); if (i?.method || String(u) !== '/api/artifacts') return r; return new Response(JSON.stringify((await r.json()).filter((a) => a.slug !== 'a2'))); })(window.fetch); load()")
	out = append(out, b.until("the page gone", "!document.querySelector('#list li[data-slug=a2]') && [document.getElementById('read-title').getClientRects().length, picked].join(' ')"))
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=a2]')")
	// Folded, j and k go on through the list, the page changing beside the place it was.
	key := func(k string) {
		b.eval("document.body.dispatchEvent(new KeyboardEvent('keydown', { key: '" + k + "', bubbles: true }))")
	}
	key("j")
	out = append(out, b.eval("picked.slug"))
	key("j")
	out = append(out, b.eval("picked.slug + ' ' + document.getElementById('read-actions').hidden"))
	// Both folded, both buttons over the page, told apart by their words; the page whole.
	b.eval("document.getElementById('shelf-hide').click()")
	out = append(out, b.eval("[...document.querySelectorAll('#manage > .side-show')].filter((e) => e.getClientRects().length).map((e) => e.id + ':' + (e.title === e.getAttribute('aria-label'))).join(' ') + ' ' + document.activeElement.id + ' ' + Math.round(document.getElementById('reader').getBoundingClientRect().left)"))
	// Kept across a reload.
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=a1]')")
	out = append(out, b.eval("document.getElementById('manage').className"))
	b.eval("document.getElementById('list-show').click()")
	out = append(out, b.eval("document.activeElement.id + ' ' + document.getElementById('manage').className + ' ' + document.getElementById('shelf-what').parentElement.className"))
	b.eval("document.getElementById('shelf-show').click(); document.getElementById('list-hide').click()")
	// What the filters pick shows: folded, the list opens; the search too.
	b.eval("document.querySelector('.views [data-view=fav]').click()")
	out = append(out, b.eval("getComputedStyle(document.getElementById('listcol')).display"))
	b.eval("setView('all'); document.getElementById('list-hide').click(); const q = document.getElementById('q'); q.value = 'a'; q.dispatchEvent(new Event('input'))")
	out = append(out, b.eval("getComputedStyle(document.getElementById('listcol')).display"))
	b.eval("q.value = ''; q.dispatchEvent(new Event('input')); document.getElementById('list-hide').click()")
	// Folded across the widths: the list as ever, no buttons, under 70rem; folded still above it.
	// With a page chosen, its head shows.
	b.size(1000, 700, false)
	b.until("the list under 70rem", "matchMedia('(max-width: 70rem)').matches && getComputedStyle(document.getElementById('listcol')).display === 'flex'")
	b.eval("document.querySelector('#list li[data-slug=a1] .title').click()")
	out = append(out, b.until("the head of the page", "document.getElementById('read-title').textContent && [getComputedStyle(document.getElementById('listcol')).display, ...['list-show', 'list-hide', 'list-grip'].map((id) => document.getElementById(id).getClientRects().length), document.getElementById('read-head').getClientRects().length].join(' ')"))
	b.size(1600, 900, false)
	out = append(out, b.until("the list folded", "!matchMedia('(max-width: 70rem)').matches && getComputedStyle(document.getElementById('listcol')).display"))

	// The two feet's buttons stand on one line, and what opens each column stays on it, folded.
	b.size(1600, 900, false)
	b.until("the wide window", "!matchMedia('(max-width: 70rem)').matches")
	b.eval("localStorage.removeItem('list.columns')")
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=a1]')")
	b.eval("document.querySelector('#list li[data-slug=a1] .title').click()")
	const lines = `((ids) => ids.map((s) => { const r = document.querySelector(s).getBoundingClientRect(); return r.top + r.height / 2; }).join(' '))`
	open := b.eval(lines + "(['#shelf-hide', '#list-hide'])")
	b.eval("document.getElementById('list-hide').click(); document.getElementById('shelf-hide').click()")
	shut := b.eval(lines + "(['#shelf-show', '#list-show'])")
	y, _, _ := strings.Cut(open, " ")
	out = append(out, strconv.FormatBool(open == y+" "+y), strconv.FormatBool(shut == open))
	b.eval("document.getElementById('list-show').click(); document.getElementById('shelf-show').click()")
	// The rule of the folded columns leaves no width between it and the one under 70rem.
	out = append(out, b.eval("[...document.styleSheets[0].cssRules].some((r) => r.media?.mediaText === 'not (max-width: 70rem)')"))

	// Between the widths a zoomed window can fall, one set of rules or the other holds: not only "under 70rem", and not both.
	out = append(out, b.eval("[matchMedia('(max-width: 70rem)').matches, WIDE.matches, WIDE.media].join(' ')"))

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
		"520px 740", "0 0 0 0", "flex false 0 true  608", "none true 1 true list-show 200", "false true true", "0 ",
		"long", "a3 false", "shelf-show:true list-show:true shelf-show 0",
		"shell shelf-closed list-closed", "list-hide shell shelf-closed lhead", "flex", "flex",
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
	const panel = "getComputedStyle(document.documentElement).getPropertyValue('--panel').trim()"
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

		return seams(".shelf", ".listcol", ".reader")
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
	out = append(out, at("diff-toc-hide"), seams(".dtoc", ".dbody"))
	press("diff-toc-hide")
	out = append(out, seams(".dtoc", ".dbody"), at("diff-toc-show"), face("diff-toc-show"), b.eval("(() => { const r = document.getElementById('diff-body'); return [r.getBoundingClientRect().left, r.getBoundingClientRect().width, getComputedStyle(r).paddingBottom, getComputedStyle(r).borderLeftWidth].join(' '); })()"))
	press("diff-toc-show")
	out = append(out, b.eval("document.getElementById('diff-toc-nav').hidden + ' ' + document.activeElement.id"))
	// In a window too low for the contents and the words under them, the foot stays, and the page does not scroll.
	b.size(1440, 260, false)
	out = append(out, b.until("the low window", "innerHeight === 260 && (() => { const f = document.querySelector('.dtoc .col-foot').getBoundingClientRect(); return [f.bottom, document.scrollingElement.scrollHeight - innerHeight].join(' '); })()"))
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
	b.until("the rows", "document.querySelector('#list li:not(.group)')")
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
	b.until("the rows", "document.querySelector('#list li:not(.group)')")
	press("list-show")
	b.eval("document.querySelector('#list li:not(.group) .title').click()")
	b.until("the page", "!document.getElementById('read-frame').hidden")
	out = append(out, at("shelf-hide"), at("list-hide"), columns())
	out = append(out, b.eval(`(() => {
		const body = document.getElementById('list-body'), col = document.getElementById('listcol'), foot = document.querySelector('.listcol > .col-foot').getBoundingClientRect();
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
	out = append(out, b.until("the one row of filters", "matchMedia('(max-width: 70rem)').matches && ['#shelf-show', '#list-show', '.shelf > .col-foot', '.listcol > .col-foot'].map((q) => document.querySelector(q).getClientRects().length).join(' ')"))
	b.size(1440, 900, false)
	press("list-show")
	out = append(out, b.until("the list again", "!document.querySelector('.list-closed') && document.activeElement.id"))

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
	const named = `const name = (e) => ['diff-open', 'fav-item', 'mark-unread', 'select', 'tag-item', 'pin-item', 'task-set', 'task-open', 'edit', 'copy', 'download', 'open', 'archive', 'star', 'delete'].find((c) => e.classList.contains(c));`
	shown := func(box string) string {
		t.Helper()

		bar := b.eval(`(() => { ` + named + `
			const out = [...document.querySelector(` + js(box) + `).children].filter((e) => e.getClientRects().length).map((e) => (e.matches('.more-slot') ? (e.querySelector('.more') ? 'more' : '') : name(e)));
			return out.filter(Boolean).join(' ');
		})()`)
		menu := b.eval(`(() => { ` + named + `
			const more = document.querySelector(` + js(box+" .more") + `);
			if (!more) return '';
			more.open = true;
			const out = [...more.querySelectorAll('.menu > *')].filter((e) => e.getClientRects().length).map(name).join(' ');
			more.open = false;
			return out;
		})()`)
		return bar + " | " + menu
	}
	pick := func(slug string) {
		t.Helper()

		b.eval("pickRow({ slug: " + js(slug) + ", file: '' })")
		b.until(slug+" beside the list", "picked?.slug === "+js(slug))
	}
	out := make([]string, 0, 16)
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=two]')")
	for _, slug := range []string{"two", "one"} {
		pick(slug)
		b.until(slug+" read", "items.find((a) => a.slug === "+js(slug)+") && !items.find((a) => a.slug === "+js(slug)+").news")
		b.until("its menu", "!document.querySelector('#read-actions .mark-unread').hidden")
		out = append(out, shown("#read-actions"))
	}
	for _, slug := range []string{"link", "gone"} {
		pick(slug)
		out = append(out, shown("#read-actions"))
	}
	// A row's menu has all, and the bar's kinds in the foot of the modal that finds: a folder, which the list opens nothing of.
	out = append(out, b.eval(`(() => { `+named+`
		const more = document.querySelector('#list li[data-slug=one] .more');
		more.open = true;
		const got = [...more.querySelectorAll('.menu > *')].filter((e) => e.getClientRects().length).map(name).join(' ');
		more.open = false;
		return got;
	})()`))
	b.eval("openFind()")
	typeFind(b, "docs")
	b.until("the folder's row", "document.getElementById('find-actions').dataset.slug === 'docs'")
	out = append(out, shown("#find-actions"))
	typeFind(b, "two")
	b.until("the page's row", "document.getElementById('find-actions').dataset.slug === 'two'")
	out = append(out, shown("#find-actions"))
	b.eval("document.getElementById('find').close()")

	// The whole page: an archived one tells what a press does in its button, and is black while it is.
	b.navigate("/a/kept/")
	b.until("the bar", "!document.getElementById('frame-actions').hidden && document.querySelector('#frame-actions .star')")
	b.until("read", "!document.querySelector('#frame-actions .mark-unread').hidden")
	out = append(out, shown("#frame-actions"))
	const archive = "(() => { const a = document.querySelector('#frame-actions > .archive'); return [a.title, a.getAttribute('aria-label'), a.getAttribute('aria-pressed'), a.classList.contains('on'), getComputedStyle(a).backgroundColor === 'rgba(0, 0, 0, 0)'].join(' / '); })()"
	out = append(out, b.eval(archive))
	b.eval("document.querySelector('#frame-actions > .archive').focus(); document.querySelector('#frame-actions > .archive').click()")
	b.until("restored", "!document.querySelector('#frame-actions > .archive').classList.contains('on')")
	out = append(out, b.settled(archive), b.eval("document.activeElement.matches('#frame-actions > .archive')"))
	b.navigate("/a/two/")
	b.until("the bar", "document.querySelector('#frame-actions > .download[href]')")
	out = append(out, b.eval("(() => { const d = document.querySelector('#frame-actions > .download'); return [new URL(d.href).pathname, d.download].join(' '); })()"))
	b.navigate("/a/docs/?file=index.html")
	b.until("the bar", "!document.getElementById('frame-actions').hidden && document.querySelector('#frame-actions .star')")
	out = append(out, shown("#frame-actions"))
	// Pressed the other way, twice before the answer: one question and one toast, the words changed, the mark black with what is on it readable.
	b.navigate("/a/one/")
	b.until("the bar", "document.querySelector('#frame-actions > .archive[title]')")
	b.eval(`window.patched = 0; window.fetch = ((f) => (u, i) => { if (i?.method === 'PATCH') window.patched++; return f(u, i); })(window.fetch);
		document.querySelector('#frame-actions > .archive').focus(); document.querySelector('#frame-actions > .archive').click(); document.querySelector('#frame-actions > .archive').click()`)
	b.until("archived", "document.querySelector('#frame-actions > .archive').classList.contains('on')")
	said := b.until("the toast", "document.querySelector('.toast') && [window.patched, [...document.querySelectorAll('.toast')].map((e) => e.textContent).join(), document.activeElement.matches('#frame-actions > .archive')].join(' / ')")
	out = append(out, b.settled(archive), said)
	out = append(out, b.settled(`(() => {
		const lum = (c) => { const [r, g, bl] = c.match(/[\d.]+/g).map((x) => { const v = x / 255; return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4; }); return 0.2126 * r + 0.7152 * g + 0.0722 * bl; };
		const s = getComputedStyle(document.querySelector('#frame-actions > .archive')), a = lum(s.color), g = lum(s.backgroundColor);
		return (Math.max(a, g) + 0.05) / (Math.min(a, g) + 0.05) >= 4.5;
	})()`))
	b.eval("document.querySelector('#frame-actions > .archive').click()")
	b.until("given back", "!document.querySelector('#frame-actions > .archive').classList.contains('on')")
	// Between the widths the bar is as wide.
	b.size(800, 800, false)
	b.navigate("/a/two/")
	b.until("the bar", "innerWidth === 800 && document.querySelector('#frame-actions .more')")
	out = append(out, shown("#frame-actions"))

	// Narrow: the title takes the logo's room, the rest in the menu, favorites too, and f goes on.
	b.size(400, 800, false)
	b.navigate("/a/two/")
	b.until("the bar", "innerWidth === 400 && !document.getElementById('frame-actions').hidden && document.querySelector('#frame-actions .more')")
	out = append(out, shown("#frame-actions"))
	out = append(out, b.eval("[document.querySelector('.brand').getClientRects().length, document.getElementById('back').getClientRects().length, document.documentElement.scrollWidth <= innerWidth].join(' ')"))
	const fav = "(() => { const f = document.querySelector('#frame-actions .fav-item'); return [f.querySelector('.label').textContent, f.getAttribute('aria-label')].join(' / '); })()"
	out = append(out, b.eval(fav))
	b.pressKey("f")
	out = append(out, b.until("a favorite", "items.find((a) => a.slug === 'two').favorite && "+fav))
	b.eval("document.querySelector('#frame-actions summary').click(); document.querySelector('#frame-actions .fav-item').click()")
	out = append(out, b.until("no favorite", "!items.find((a) => a.slug === 'two').favorite && document.querySelector('#frame-actions .fav-item') && "+fav))
	// Pressed on the toast with the focus on it, a page marked unread gives the focus to the bar's menu.
	// The menu opens within the window at any width.
	for _, width := range []int64{400, 320} {
		b.size(width, 800, false)
		b.until("the width", "innerWidth === "+strconv.FormatInt(width, 10))
		b.eval("document.querySelector('#frame-actions .more').open = true")
		out = append(out, b.eval("(() => { const r = document.querySelector('#frame-actions .menu').getBoundingClientRect(); return r.left >= 0 && r.right <= innerWidth && document.documentElement.scrollWidth <= innerWidth; })()"))
		b.eval("document.querySelector('#frame-actions .more').open = false")
	}
	b.size(400, 800, false)
	// The other kinds, narrow.
	s.floe(t, "archive", "kept")
	for _, slug := range []string{"one", "link", "kept"} {
		b.navigate("/a/" + slug + "/")
		b.until(slug+"'s bar", "innerWidth === 400 && document.querySelector('#frame-actions .more')")
		out = append(out, shown("#frame-actions"))
	}
	b.navigate("/a/one/")
	b.until("the bar", "document.querySelector('#frame-actions .more')")
	b.eval("markUnread('one')")
	b.until("unread", "items.find((a) => a.slug === 'one').news")
	b.navigate("/a/one/")
	b.until("the toast", "document.querySelector('.toast.page .btn')")
	b.eval("document.querySelector('.toast.page .btn').focus(); document.querySelector('.toast.page .btn').click()")
	out = append(out, b.until("the focus on the menu", "document.activeElement.matches('#frame-actions summary') && 'on the menu'"))
	b.navigate("/a/docs/?file=index.html")
	b.until("the bar", "!document.getElementById('frame-actions').hidden && document.querySelector('#frame-actions .star')")
	out = append(out, shown("#frame-actions"))
	// The foot of the modal holds its buttons at this width.
	b.eval("openFind()")
	typeFind(b, "two")
	b.until("the page's row", "document.getElementById('find-actions').dataset.slug === 'two'")
	out = append(out, b.eval(`(() => {
		const box = document.getElementById('find').getBoundingClientRect(), close = document.querySelector('.find-foot [data-close]').getBoundingClientRect();
		const acts = [...document.querySelectorAll('#find-actions > *')].filter((e) => e.getClientRects().length).map((e) => e.getBoundingClientRect());
		return [acts.every((r) => r.left >= box.left && r.right <= close.left), close.right <= box.right].join(' ');
	})()`))

	check(t, "bar", out, strings.Join([]string{
		"diff-open download open archive star more | mark-unread tag-item pin-item task-set edit delete",
		"download open archive star more | mark-unread tag-item pin-item task-set edit delete",
		"download open archive star more | tag-item pin-item task-set edit copy delete",
		"archive star more | tag-item pin-item task-set edit copy delete",
		"mark-unread select tag-item pin-item task-set edit open download delete",
		"archive star more | tag-item pin-item task-set edit copy delete",
		"diff-open download open archive star more | mark-unread tag-item pin-item task-set edit delete",
		"download open archive star more | mark-unread tag-item task-set edit delete",
		"Restore / Restore Kept to the list /  / true / false",
		"Archive / Archive Kept /  / false / true", "true",
		"/api/artifacts/two/download ",
		"copy open | ",
		"Restore / Restore One to the list /  / true / false",
		"1 / Archived “One”Restore / true",
		"true",
		"diff-open download open archive star more | mark-unread tag-item pin-item task-set edit delete",
		"diff-open more | fav-item mark-unread tag-item pin-item task-set edit open download archive delete",
		"1 1 true",
		"Add to favorites / Add Two to favorites",
		"Remove from favorites / Remove Two from favorites",
		"Add to favorites / Add Two to favorites",
		"true", "true",
		"more | fav-item mark-unread tag-item pin-item task-set edit open download archive delete",
		"more | fav-item tag-item pin-item task-set edit copy open download archive delete",
		"more | fav-item mark-unread tag-item task-set edit open download archive delete",
		"on the menu",
		"copy open | ",
		"true true",
	}, "|"))
}

// rowPoint is where in the row of the selector a press lands on its link: at its title's words, or in the margin of the row.
func rowPoint(t *testing.T, b *browser, row string, margin bool) (float64, float64) {
	t.Helper()

	box, x, y := "document.querySelector("+js(row+" .title")+")", 12, 6
	if margin {
		box, x, y = "document.querySelector("+js(row)+")", 4, 2
	}
	got := b.eval(fmt.Sprintf(`(() => {
		const r = %s.getBoundingClientRect(), x = r.left + %d, y = r.top + %d, on = document.elementFromPoint(x, y);
		return on?.closest('a.title') ? JSON.stringify({ X: x, Y: y }) : 'not its link: ' + on?.outerHTML;
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
	const a2 = "#list li[data-slug=a2]"
	x, y := rowPoint(t, b, a2, false)
	b.click(x, y)
	b.until("picked", "picked?.slug === 'a2'")
	out := make([]string, 0, 7)
	out = append(out, b.eval(rowAgainPath))
	// Read and drawn so, as a redraw closes a menu open.
	b.until("read", "!document.querySelector("+js(a2)+").classList.contains('unseen') && !readsGoing")
	b.eval("load().then(() => true)")

	// A menu open, the press closes it.
	b.eval("document.querySelector(" + js(a2+" .more") + ").open = true")
	b.click(x, y)
	b.until("the menu closed", "!document.querySelector('.more[open]')")
	out = append(out, b.eval(rowAgainPath))
	// A delete waiting in it, the same.
	b.eval("document.querySelector(" + js(a2+" .more") + ").open = true; document.querySelector(" + js(a2+" .delete") + ").click()")
	b.until("armed", "document.querySelector("+js(a2+" .delete.armed")+")")
	b.click(x, y)
	b.until("the menu closed again", "!document.querySelector('.more[open]')")
	out = append(out, b.eval(rowAgainPath))
	// A row selected, the press leaves it selected.
	b.eval("document.querySelector('#list li[data-slug=a1] .sel').click()")
	x, y = rowPoint(t, b, a2, false)
	b.click(x, y)
	out = append(out, b.eval("[selected.size, picked.slug, "+rowAgainPath+"].join(' ')"))
	b.eval("document.querySelector('#list li[data-slug=a1] .sel').click()")
	// Modified, it is left to the link.
	out = append(out, b.eval(`(() => {
		const link = document.querySelector('#list li[data-slug=a2] .title');
		let kept;
		const note = (e) => { kept = !e.defaultPrevented; e.preventDefault(); };
		document.addEventListener('click', note, { once: true });
		link.dispatchEvent(new MouseEvent('click', { ctrlKey: true, bubbles: true, cancelable: true }));
		return kept;
	})()`))
	// No row past the one picked: k stays, and does not go whole.
	b.eval("document.querySelector('#list li[data-slug=a3] .title').focus()")
	b.pressKey("j")
	b.until("back beside", "picked?.slug === 'a2'")
	b.pressKey("k")
	b.until("the first", "picked?.slug === 'a3'")
	b.pressKey("k")
	out = append(out, b.eval("picked.slug + ' ' + "+rowAgainPath))

	// Pressed again, whole, the list's order kept for j.
	x, y = rowPoint(t, b, a2, false)
	b.click(x, y)
	b.until("a2 again", "picked?.slug === 'a2'")
	b.click(x, y)
	b.until("whole", "location.pathname === '/a/a2/'")
	// The URL is the new page's before its script has kept where it is; the frame's src comes after that
	b.until("its bar", "document.getElementById('frame').hasAttribute('src')")
	out = append(out, b.eval("sessionStorage.getItem('artifacts.order')"))
	b.pressKey("j")
	b.until("on through the list", "location.pathname === '/a/a1/'")

	// In the margin of the row.
	asideList(b)
	x, y = rowPoint(t, b, a2, true)
	b.click(x, y)
	b.until("picked by the margin", "picked?.slug === 'a2'")
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
	const folder, missing = "#list li[data-slug=docs]", "#list li[data-slug=gone].gone"
	b.until("the rows", "document.querySelector("+js(folder)+") && document.querySelector("+js(missing)+")")

	x, y := rowPoint(t, b, missing, false)
	b.click(x, y)
	b.until("the words why", "picked?.slug === 'gone' && !document.getElementById('read-blank').hidden")
	b.click(x, y)
	out := make([]string, 0, 2)
	out = append(out, b.eval("picked.slug + ' ' + "+rowAgainPath+" + ' ' + sessionStorage.getItem('artifacts.order')"))

	const expanded = ".querySelector('.title').getAttribute('aria-expanded')"
	x, y = rowPoint(t, b, folder, false)
	b.click(x, y)
	b.until("the tree", "document.querySelector("+js(folder)+")"+expanded+" === 'true' && document.querySelector('#list li.page')")
	const page = "#list li.page[data-file='index.html']"
	px, py := rowPoint(t, b, page, false)
	b.click(px, py)
	b.until("the page picked", "picked?.file === 'index.html'")
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
	// Every page read and drawn so, so that nothing else is left to change the answer.
	for _, slug := range []string{"a1", "a3", "a2"} {
		x, y := rowPoint(t, b, "#list li[data-slug="+slug+"]", false)
		b.click(x, y)
		b.until(slug+" read", "picked?.slug === "+js(slug)+" && !document.querySelector('#list li[data-slug="+slug+"]').classList.contains('unseen') && !readsGoing")
	}
	b.eval("load().then(() => true)")
	// A load whose answer differs from what is drawn: true if it left items and the rows as they are.
	loaded := func(call string) string {
		t.Helper()
		return b.eval("(async () => { const row = document.querySelector('#list li[data-slug=a2]'), had = items; drawn = ''; await " + call + "; return row.isConnected && items === had; })()")
	}
	const a3 = "#list li[data-slug=a3]"
	out := make([]string, 0, 7)

	// Released, nothing is held.
	x, y := rowPoint(t, b, "#list li[data-slug=a1]", false)
	b.click(x, y)
	b.until("a1 picked", "picked?.slug === 'a1'")
	out = append(out, loaded("load(true)"))

	x, y = rowPoint(t, b, a3, false)
	// A press on the row of another, the focus elsewhere: the poll's answer held back; released, the press counts and the next poll draws.
	b.eval("document.querySelector('#list li[data-slug=a1] .title').focus()")
	b.mouse(chromedp.MousePressed, x, y, 1)
	out = append(out, loaded("load(true)"))
	b.eval("window.heldRow = document.querySelector('#list li[data-slug=a2]')")
	b.mouse(chromedp.MouseReleased, x, y, 1)
	b.until("picked", "picked?.slug === 'a3'")
	b.until("drawn by the next poll", "!window.heldRow.isConnected")

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
	out = append(out, loaded("load(true)"))
	b.mouse(chromedp.MouseReleased, at.X, at.Y, 1)

	// The second press, whose release may never come: an action's load goes on, and the poll's only until the time is up.
	x, y = rowPoint(t, b, a3, false)
	b.mouse(chromedp.MousePressed, x, y, 2)
	out = append(out, loaded("load()"))
	out = append(out, loaded("load(true)"))
	b.until("the time up", "performance.now() - pressedAt > PRESS_MS")
	out = append(out, loaded("load(true)"))
	b.eval("drawn = ''; load().then(() => true)")
	x, y = rowPoint(t, b, a3, false)
	b.mouse(chromedp.MouseReleased, x, y, 2)

	// Pressed again with the poll in the middle of it, the page opens whole.
	x, y = rowPoint(t, b, a3, false)
	b.mouse(chromedp.MousePressed, x, y, 1)
	out = append(out, loaded("load(true)"))
	b.mouse(chromedp.MouseReleased, x, y, 1)
	b.until("whole", "location.pathname === '/a/a3/'")

	check(t, "row again, pressed", out, "false|true|false|false|true|false|true")
}
