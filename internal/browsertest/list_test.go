package browsertest_test

import (
	"os"
	"path/filepath"
	"strconv"
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
	out = append(out, b.eval("(() => { const l = document.querySelector('#add .lead').getBoundingClientRect(), f = document.querySelector('#add .field').getBoundingClientRect(); return [l.bottom <= f.top, getComputedStyle(document.querySelector('#add .lead')).textAlign === 'center'].join(' '); })()"))
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
		"Today,Yesterday,Last 7 days", "2 new / Newest first · 2 new / 2",
		"Version\nthe words", "v ?q=the+words",
		"false true /none  ", "true false /a/r/?raw Read r", "/ /a/r/", "true",
		"true false /a/l/?raw Link l", "true false /a/n/?raw New n New",
		"true true true", "false true blank New ",
		"vn ?view=new", "vn", "1 new / 1", "New", "n", "true",
		`["v","n"]`, "/?view=new", "/a/v/",
		"true true true true true", "flex none none true", "/ false", "none true", "flex flex", "true false", "true",
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
	check(t, "reader", out, "true|a3|bar|row|true about:blank|false| about:blank|true")
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
	go1("n", "[]", "?template=report", "r")
	go1("l", "[]", "?kind=link", "d")
	// Not on the last list: all of its kind.
	go1("v", "[]", "?kind=link", "n")
	// Archived: the archived.
	go1("a", "[]", "?kind=link", "b")
	// At an end, it stays; k goes back.
	b.navigate("/")
	b.eval(`sessionStorage.setItem('artifacts.order', '["v","n"]')`)
	b.navigate("/a/n/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	b.eval("document.body.dispatchEvent(new KeyboardEvent('keydown', { key: 'j', bubbles: true })); window.stayed = true")
	check(t, "at an end", []string{b.eval("window.stayed === true && location.pathname")}, "/a/n/")
}
