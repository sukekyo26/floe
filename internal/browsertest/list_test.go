package browsertest_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/accessibility"
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
		b.eval("document.body.dispatchEvent(new KeyboardEvent('keydown', { key: '" + k + "', bubbles: true }))")
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
	b.eval("document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))")
	b.until("the page whole", "location.pathname === '/a/v/'")
	out = append(out, b.eval("sessionStorage.getItem('artifacts.order')"), b.eval("document.querySelector('.brand').getAttribute('href')"))
	b.stub(strings.Replace(listList, "news: 'version', ", "", 1))
	b.until("the bar", "document.getElementById('frame-title').textContent")
	key("j")
	// The bar, not the URL alone: a key goes nowhere until the list is read.
	b.until("the next page", "location.pathname === '/a/n/' && document.getElementById('frame-title').textContent")
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
	b.eval("document.querySelector('#list li[data-slug=r] .title').click()")
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
	b.eval("document.querySelector('#list li[data-slug=r] .title').click()")
	b.until("the row whole", "location.pathname === '/a/r/'")
	out = append(out, b.until("the bar", "document.getElementById('frame-title').textContent && [getComputedStyle(document.getElementById('back')).display, getComputedStyle(document.getElementById('frame-actions')).display].join(' ')"))
	b.unsize()

	check(t, "list", out, strings.Join([]string{
		"#|v dot 700 ver:v3 · new version kindof:design proj:p tags:t|#|n dot 700 kindof:report proj:q|#|r ring 500 kindof:report proj:p|l mark 500 kindof:Link · HTML path:/tmp/x/l.html|#|d mark 500 kindof:Link · handoff path:/home/you/work/a (1)/v2.0/data-set_#1/deep/deep/deep/deep/deep/deep/deep/deep/deep/name-2.yaml|#|f mark 500 kindof:Folder path:/tmp/x/f|#|g mark 500 kindof:Link · HTML miss:File not found",
		"Today,Yesterday,Last 7 days", "2 unread / Newest first · 2 unread / 2",
		"Version\nthe words\np", "v ?q=the+words",
		"false true about:blank  ", "true false /a/r/?raw Read r", "/ /a/r/", "true",
		"true false /a/l/?raw Link l", "true false /a/n/?raw New n New",
		"true true true", "false true about:blank New ",
		"vn ?view=new", "vn", "1 unread / 1", "New", "n", "true",
		`["v","n"]`, "/?view=new", "/a/v/",
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
	check(t, "shelf marks", out, "10 true|true true true true|page:true link:true folder:true|true|true|true|true|true|true|true|true|true|Favorites 0,Archive|Favorites 1,Archive|Favorites 1,Archive 0")
}

// TestUIShelfCounts: each part of the shelf counts what the other parts, the view and the search leave, its own part let go; the
// views take the picks and the search, All the search alone; a row at 0 stays where it is, and the bar counts all there is.
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
	// The bar and the head count all there is.
	out = append(out, b.eval("document.getElementById('news-n').textContent + ' / ' + document.getElementById('count').textContent"))
	// A read row that stays on the view of the unread is not counted.
	b.navigate("/?view=new")
	b.eval("window.list = JSON.stringify(JSON.parse(window.list).map((x) => x.slug === 'a' ? { ...x, news: undefined } : x)); drawn = ''; load()")
	b.until("the row read", "document.querySelector('#list li[data-slug=a] .ring')")
	out = append(out, b.eval("[...document.querySelectorAll('#list > li:not(.group)')].length + ' ' + document.querySelector('.views [data-view=new] .n').textContent"))
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
		"2 unread / Newest first · 2 unread",
		"2 1",
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
		b.eval("sessionStorage.setItem('artifacts.order', " + js(order) + "); sessionStorage.setItem('artifacts.list', " + js(list) + ")")
		b.navigate("/a/" + from + "/")
		b.until("the bar", "document.getElementById('frame-title').textContent")
		b.eval("document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'j', bubbles: true }))")
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
	b.eval(`sessionStorage.setItem('artifacts.order', '["v","n"]')`)
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
	after, err := strconv.Atoi(b.eval(cut))
	if err != nil || after >= before {
		t.Errorf("the path is cut by %d at the wider list, %d before: %v", after, before, err)
	}
	mouse("list-grip", 1590, true)
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
		const bar = [...document.querySelectorAll('#bulk .btn')].every(within);
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
	// Folded and shown again, the focus on the other button; the list's words clear of the button.
	const shelf = "[getComputedStyle(document.querySelector('.shelf')).display, document.getElementById('shelf-show').hidden, document.getElementById('shelf-grip').hidden, document.activeElement.id, document.getElementById('shelf-show').getBoundingClientRect().right <= document.getElementById('count').getBoundingClientRect().left].join(' ')"
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
	// Both folded, both buttons in the head of the page, told apart by their words; the page nearly whole.
	b.eval("document.getElementById('shelf-hide').click()")
	out = append(out, b.eval("[...document.querySelectorAll('#read-head .icon-btn')].filter((e) => e.closest('.frame-actions') === null && e.getClientRects().length).map((e) => e.id + ':' + (e.title === e.getAttribute('aria-label'))).join(' ') + ' ' + document.activeElement.id + ' ' + Math.round(document.getElementById('reader').getBoundingClientRect().left)"))
	// Kept across a reload.
	b.navigate("/")
	out = append(out, b.eval("document.getElementById('manage').className"))
	b.eval("document.getElementById('list-show').click()")
	out = append(out, b.eval("document.activeElement.id + ' ' + document.getElementById('manage').className + ' ' + document.getElementById('shelf-tools').parentElement.className"))
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
	b.until("the list under 70rem", "matchMedia('(max-width: 70rem)').matches && getComputedStyle(document.getElementById('listcol')).display === 'block'")
	b.eval("document.querySelector('#list li[data-slug=a1] .title').click()")
	out = append(out, b.until("the head of the page", "document.getElementById('read-title').textContent && [getComputedStyle(document.getElementById('listcol')).display, ...['list-show', 'list-hide', 'list-grip'].map((id) => document.getElementById(id).getClientRects().length), document.getElementById('read-head').getClientRects().length].join(' ')"))
	b.size(1600, 900, false)
	out = append(out, b.until("the list folded", "!matchMedia('(max-width: 70rem)').matches && getComputedStyle(document.getElementById('listcol')).display"))

	// The three heads' buttons stand on one line, open or folded; the list's over the rows' menus, the filters' over their counts.
	b.size(1600, 900, false)
	b.until("the wide window", "!matchMedia('(max-width: 70rem)').matches")
	b.eval("localStorage.removeItem('list.columns')")
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=a1]')")
	b.eval("document.querySelector('#list li[data-slug=a1] .title').click()")
	const lines = `((ids) => ids.map((s) => { const r = document.querySelector(s).getBoundingClientRect(); return r.top + r.height / 2; }).join(' '))`
	const centre = "(s) => { const r = document.querySelector(s).getBoundingClientRect(); return r.left + r.width / 2; }"
	open := b.eval(lines + "(['#shelf-hide', '#list-hide', '#read-actions .star'])")
	b.eval("document.getElementById('list-hide').click(); document.getElementById('shelf-hide').click()")
	shut := b.eval(lines + "(['#shelf-show', '#list-show', '#read-actions .star'])")
	y, _, _ := strings.Cut(open, " ")
	out = append(out, strconv.FormatBool(open == y+" "+y+" "+y), strconv.FormatBool(shut == open))
	b.eval("document.getElementById('list-show').click(); document.getElementById('shelf-show').click()")
	out = append(out, b.eval("(() => { const at = "+centre+"; const n = document.querySelector('.views .n').getBoundingClientRect(), i = document.querySelector('#shelf-hide svg').getBoundingClientRect(); return [Math.abs(at('#list-hide') - at('#list li[data-slug=a1] summary')) <= 0.5, Math.abs(n.right - i.right) <= 1].join(' '); })()"))
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
		"true  true", "false Archive · Filter 2 true", "true", "grid",
		"block 1 0 0 16 2", "none 0 1 0 16 2",
		"true", "none", "grid shelf-hide", "160px 740px 160 740",
		"520px 740", "0 0 0 0", "block false 0 true  608", "none true 1 false list-show 200", "false true true", "0 ",
		"long", "a3 false", "shelf-show:true list-show:true shelf-show 0",
		"shell shelf-closed list-closed", "list-hide shell shelf-closed lhead", "block", "block",
		"block 0 0 0 1", "none", "true", "true", "true true", "true", "false true not (max-width: 70rem)",
		"200px true true", "200px true true", "200px true true", "320px 320px",
	}, "|"))
}
