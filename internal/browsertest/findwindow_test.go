package browsertest_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

const findList = `[{ slug: 'a', title: 'Alpha plan', description: 'the words', news: 'new', version: 1, template: 'report', project: 'p', createdAt: at(1), updatedAt: at(9) },
	{ slug: 'z', title: 'Zed plan', description: '', version: 1, archived: true, project: 'q', createdAt: at(1), updatedAt: at(8) },
	{ slug: 'b', title: 'Beta', description: '', version: 1, favorite: true, createdAt: at(1), updatedAt: at(7) },
	{ slug: 'c', title: 'Gamma plan', description: '', path: '/tmp/x/c.html', createdAt: at(1), updatedAt: at(6) },
	{ slug: 'f', title: 'Folder', description: '', path: '/tmp/x/f', folder: true, createdAt: at(1), updatedAt: at(5) }]`

// What the window shows: open or not, the rows with the one reached in brackets, the title over its buttons, the foot's end.
const findDrawn = `(() => {
	const rows = [...document.querySelectorAll('#find-list li')].map((li) => {
		const slug = new URL(li.querySelector('a').href).pathname.split('/')[2];
		return li.classList.contains('reached') ? '[' + slug + ']' : slug;
	}).join('');
	return [document.getElementById('find').open, rows, document.getElementById('find-acts').hidden ? '-' : document.getElementById('find-on').textContent,
		document.getElementById('find-foot').hidden ? '-' : document.getElementById('find-cut').textContent].join(' / ');
})()`

// keyDown sends the key from what has the focus, as the page's handlers see it.
func keyDown(b *browser, init string) {
	b.t.Helper()
	b.eval("(document.activeElement ?? document.body).dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, " + init + " }))")
}

func typeFind(b *browser, words string) {
	b.t.Helper()
	b.eval("(() => { const q = document.getElementById('find-q'); q.value = " + js(words) + "; q.dispatchEvent(new Event('input')); })()")
}

// TestUIFind opens the window from the list, a whole page and a field, finds in everything an entry has with the archived last,
// goes through its rows, keeps the row reached as the rows change, and leaves what goes to the input method to it.
func TestUIFind(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.stub(findList)
	b.navigate("/")
	b.size(1440, 900, false)
	b.eval("window.errs = []; addEventListener('error', (e) => window.errs.push(e.message))")
	out := make([]string, 0, 32)
	// / is the search above where there is one.
	keyDown(b, "key: '/'")
	out = append(out, b.eval("document.getElementById('find').open + ' ' + document.activeElement.id"))
	// Ctrl+K from that field too; the same keys close it.
	keyDown(b, "key: 'k', ctrlKey: true")
	out = append(out, b.eval(findDrawn), b.eval("document.activeElement.id"))
	keyDown(b, "key: 'k', metaKey: true")
	out = append(out, b.eval("document.getElementById('find').open"))
	// Not with another key of k, nor over another dialog.
	keyDown(b, "key: 'K', ctrlKey: true, shiftKey: true")
	keyDown(b, "key: 'k', ctrlKey: true, altKey: true")
	out = append(out, b.eval("document.getElementById('find').open"))
	b.eval("document.getElementById('add-open').click()")
	keyDown(b, "key: 'k', ctrlKey: true")
	out = append(out, b.eval("document.getElementById('find').open + ' ' + document.getElementById('add-dialog').open"))
	b.eval("document.getElementById('add-dialog').close(); document.body.focus()")
	keyDown(b, "key: 'k', ctrlKey: true")
	// Found by the description, by the path; the archived last, marked; none.
	typeFind(b, "the words")
	out = append(out, b.eval(findDrawn))
	typeFind(b, "/tmp/x/c")
	out = append(out, b.eval(findDrawn))
	typeFind(b, "plan")
	out = append(out, b.eval(findDrawn),
		b.eval("document.querySelector('#find-list li:last-child .badge').textContent + ' ' + document.querySelectorAll('#find-list .badge').length"))
	// The arrows, round the ends; the row reached is told, and it is what the buttons act on. A row is a link out of the Tab's way.
	keyDown(b, "key: 'ArrowDown'")
	out = append(out, b.eval(findDrawn), b.eval("document.getElementById('find-q').getAttribute('aria-activedescendant')"))
	keyDown(b, "key: 'ArrowUp'")
	keyDown(b, "key: 'ArrowUp'")
	out = append(out, b.eval(findDrawn))
	out = append(out, b.eval("(() => { const a = document.querySelector('#find-list a'); return a.tabIndex + ' ' + new URL(a.href).pathname; })()"))
	// Tab stays in the window, on the buttons.
	if err := chromedp.Do(b.ctx, chromedp.KeyEvent(kb.Tab)); err != nil {
		t.Fatal(err)
	}
	out = append(out, b.eval("document.getElementById('find').open + ' ' + document.activeElement.matches('#find-actions .star')"))
	b.eval("document.getElementById('find-q').focus()")
	// None: no rows, no buttons, no foot, the reason; Enter goes nowhere.
	typeFind(b, "nothing like it")
	out = append(out, b.eval(findDrawn), b.eval("document.getElementById('find-none').hidden + ' ' + document.getElementById('find-none').textContent.includes('.')"))
	keyDown(b, "key: 'Enter'")
	out = append(out, b.eval("location.pathname"))
	// What goes to the input method: no row moves, nothing opens or closes.
	typeFind(b, "plan")
	keyDown(b, "key: 'ArrowDown', isComposing: true")
	keyDown(b, "key: 'Enter', isComposing: true")
	keyDown(b, "key: 'Enter', keyCode: 229")
	keyDown(b, "key: 'k', ctrlKey: true, isComposing: true")
	out = append(out, b.eval(findDrawn), b.eval("location.pathname"))
	// A row that stays is the one reached as the rows change, though at another place; one gone, what has come to its place, or the last; none, nothing.
	typeFind(b, "")
	keyDown(b, "key: 'ArrowDown'")
	keyDown(b, "key: 'ArrowDown'")
	reload := "drawn = ''; load()"
	drop := func(slugs string) {
		b.eval("window.list = JSON.stringify(JSON.parse(window.list).filter((a) => !" + js(slugs) + ".includes(a.slug))); " + reload)
		b.until("the rows drawn anew", "document.querySelectorAll('#find-list li').length === JSON.parse(window.list).length")
	}
	out = append(out, b.eval(findDrawn))
	drop("b")
	out = append(out, b.eval(findDrawn))
	drop("c")
	out = append(out, b.eval(findDrawn))
	keyDown(b, "key: 'ArrowDown'")
	out = append(out, b.eval(findDrawn))
	drop("z")
	out = append(out, b.eval(findDrawn))
	drop("afz")
	out = append(out, b.eval(findDrawn))
	keyDown(b, "key: 'Enter'")
	out = append(out, b.eval("location.pathname + ' ' + window.errs.length"))
	// Enter opens the row whole, wherever the window was.
	b.stub(findList)
	b.navigate("/")
	keyDown(b, "key: 'k', ctrlKey: true")
	keyDown(b, "key: 'ArrowDown'")
	keyDown(b, "key: 'Enter'")
	b.until("the page whole", "location.pathname === '/a/b/'")
	check(t, "find", out, strings.Join([]string{
		"false q",
		"true / [a]bcfz / Alpha plan / Newest first", "find-q",
		"false", "false", "false true",
		"true / [a] / Alpha plan / 1 found",
		"true / [c] / Gamma plan / 1 found",
		"true / [a]cz / Alpha plan / 3 found", "Archived 1",
		"true / a[c]z / Gamma plan / 3 found", "find-1",
		"true / ac[z] / Zed plan / 3 found", "-1 /a/a/",
		"true true",
		"true /  / - / -", "false true",
		"/",
		"true / [a]cz / Alpha plan / 3 found", "/",
		"true / ab[c]fz / Gamma plan / Newest first",
		"true / a[c]fz / Gamma plan / Newest first",
		"true / a[f]z / Folder / Newest first",
		"true / af[z] / Zed plan / Newest first",
		"true / a[f] / Folder / Newest first",
		"true /  / - / -",
		"/ 0",
	}, "|"))
}

// TestUIFindCut says that it shows the first rows only.
func TestUIFindCut(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.stub("Array.from({ length: 51 }, (_, i) => ({ slug: 's' + i, title: 'T' + i, description: '', createdAt: at(1), updatedAt: at(9) }))")
	b.navigate("/")
	keyDown(b, "key: 'k', ctrlKey: true")
	out := []string{b.eval("document.querySelectorAll('#find-list li').length + ' ' + document.getElementById('find-cut').textContent")}
	b.stub("Array.from({ length: 50 }, (_, i) => ({ slug: 's' + i, title: 'T' + i, description: '', createdAt: at(1), updatedAt: at(9) }))")
	b.navigate("/")
	keyDown(b, "key: 'k', ctrlKey: true")
	out = append(out, b.eval("document.querySelectorAll('#find-list li').length + ' ' + document.getElementById('find-cut').textContent"))
	check(t, "cut", out, "50 The first 50|50 Newest first")
}

// TestUIFindActions acts on a row from the window: the row's own buttons, kept while pressed, the focus kept; a whole page stays
// unless it is the one acted on; the menu is whole, and Escape closes it first.
func TestUIFindActions(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.stub(findList)
	b.navigate("/")
	b.size(1440, 900, false)
	b.eval("window.errs = []; addEventListener('error', (e) => window.errs.push(e.message))")
	if b.eval("matchMedia('(hover: hover)').matches") != "true" {
		t.Fatal("this runs with a mouse")
	}
	change := func(slug, fields string) {
		b.eval("window.list = JSON.stringify(JSON.parse(window.list).map((a) => a.slug === " + js(slug) + " ? { ...a, " + fields + " } : a)); drawn = ''; load()")
	}
	const menu = "#find-actions .more"
	out := make([]string, 0, 24)
	keyDown(b, "key: 'k', ctrlKey: true")
	keyDown(b, "key: 'ArrowDown'") // b
	// The star: the same button, the focus on it, what the row is told.
	b.eval("(() => { const s = document.querySelector('#find-actions .star'); s.focus(); s.click(); })()")
	change("b", "favorite: false")
	b.until("the star drawn", "document.querySelector('#find-actions .star').getAttribute('aria-pressed') === 'false'")
	out = append(out, b.eval("document.activeElement.matches('#find-actions .star')"))
	// The menu is whole, inside the window of any height it fits, and an answer does not close it.
	for _, size := range [][2]int64{{1440, 900}, {400, 800}} {
		b.size(size[0], size[1], false)
		b.eval("document.querySelector('" + menu + " summary').click()")
		out = append(out, b.eval(`(() => { const r = document.querySelector('`+menu+` .menu').getBoundingClientRect();
			return r.bottom <= innerHeight && r.right <= innerWidth && r.left >= 0 && document.querySelector('`+menu+` .menu').contains(document.elementFromPoint(r.left + r.width / 2, r.bottom - 8)); })()`))
		b.eval("document.querySelector('" + menu + " summary').click()")
	}
	b.size(1440, 900, false)
	b.eval("document.querySelector('" + menu + " summary').click()")
	change("a", "title: 'Alpha plan!'")
	b.until("the rows drawn", "document.querySelector('#find-list a').textContent === 'Alpha plan!'")
	out = append(out, b.eval("document.querySelector('"+menu+"').open"))
	// Escape: the menu, its button getting the focus; then the window.
	b.eval("document.querySelector('" + menu + " summary').focus()")
	if err := chromedp.Do(b.ctx, chromedp.KeyEvent(kb.Escape)); err != nil {
		t.Fatal(err)
	}
	out = append(out, b.eval("[document.querySelector('"+menu+"').open, document.getElementById('find').open, document.activeElement.matches('"+menu+" summary')].join(' ')"))
	if err := chromedp.Do(b.ctx, chromedp.KeyEvent(kb.Escape)); err != nil {
		t.Fatal(err)
	}
	out = append(out, b.eval("document.getElementById('find').open"))
	keyDown(b, "key: 'k', ctrlKey: true")
	keyDown(b, "key: 'ArrowDown'")
	// Archived from the menu: the focus on the button of the menu as drawn, its words changed.
	b.eval("document.querySelector('" + menu + " summary').focus(); document.querySelector('" + menu + " summary').click(); document.querySelector('" + menu + " .archive').click()")
	change("b", "archived: true")
	b.until("the row moved", "document.querySelector('#find-actions .archive .label').textContent === 'Restore'")
	out = append(out, b.eval("document.activeElement.matches('"+menu+" summary')"))
	change("b", "archived: false")
	b.until("the row back", "document.querySelector('#find-actions .archive .label').textContent === 'Archive'")
	// A delete waiting stays through an answer.
	b.eval("document.querySelector('" + menu + " summary').click(); document.querySelector('" + menu + " .delete').focus(); document.querySelector('" + menu + " .delete').click()")
	change("a", "title: 'Alpha plan'")
	b.until("the rows drawn", "document.querySelector('#find-list a').textContent === 'Alpha plan'")
	out = append(out, b.eval("document.querySelector('"+menu+" .delete').classList.contains('armed')"))

	// From a whole page: a delete of another page stays there, the focus going to the field.
	b.navigate("/a/c/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	b.eval("window.errs = []; addEventListener('error', (e) => window.errs.push(e.message))")
	keyDown(b, "key: 'k', ctrlKey: true")
	typeFind(b, "beta")
	b.eval("document.querySelector('" + menu + " summary').click(); document.querySelector('" + menu + " .delete').focus(); document.querySelector('" + menu + " .delete').click()")
	b.eval("window.list = JSON.stringify(JSON.parse(window.list).filter((a) => a.slug !== 'b'))")
	b.eval("document.querySelector('" + menu + " .delete').click()")
	b.until("the row gone", "document.querySelectorAll('#find-list li').length === 0")
	out = append(out, b.eval("[location.pathname, document.getElementById('find').open, document.activeElement.id].join(' ')"))
	// The edit closes the window, no dialog going over another, and it goes back with no mistake.
	typeFind(b, "alpha")
	b.eval("document.querySelector('" + menu + " summary').click(); document.querySelector('" + menu + " .edit').click()")
	out = append(out, b.eval("document.getElementById('find').open + ' ' + document.getElementById('edit-dialog').open"))
	// Another page's new name leaves this one here.
	b.eval("document.getElementById('edit-slug').value = 'a2'; document.getElementById('edit-submit').click()")
	b.until("the edit closed", "!document.getElementById('edit-dialog').open")
	out = append(out, b.eval("location.pathname + ' ' + window.errs"))
	// This page's: its URL moves, and deleted, it goes to the list.
	keyDown(b, "key: 'k', ctrlKey: true")
	typeFind(b, "gamma")
	b.eval("document.querySelector('" + menu + " summary').click(); document.querySelector('" + menu + " .edit').click()")
	b.eval("document.getElementById('edit-slug').value = 'c2'; document.getElementById('edit-submit').click()")
	b.until("the new URL", "location.pathname === '/a/c2/'")
	b.navigate("/a/c/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	keyDown(b, "key: 'k', ctrlKey: true")
	typeFind(b, "gamma")
	b.eval("document.querySelector('" + menu + " summary').click(); document.querySelector('" + menu + " .delete').focus(); document.querySelector('" + menu + " .delete').click(); document.querySelector('" + menu + " .delete').click()")
	b.until("the list", "location.pathname === '/'")
	check(t, "actions", out, strings.Join([]string{
		"true", "true", "true", "true", "false true true", "false", "true", "true",
		"/a/c/ true find-q", "false true", "/a/c/ ",
	}, "|"))
}

// TestUIFindTouch leaves out the buttons where there is no mouse: a press on a row opens it, and they could act on the first alone.
func TestUIFindTouch(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.stub(findList)
	b.navigate("/")
	acts := "getComputedStyle(document.getElementById('find-acts')).display"
	keyDown(b, "key: 'k', ctrlKey: true")
	out := []string{b.eval(acts)}
	touch := func(on bool) {
		if _, err := chromedp.Call(b.ctx, emulation.SetTouchEmulationEnabled, emulation.SetTouchEmulationEnabledParams{Enabled: on, MaxTouchPoints: 1}); err != nil {
			t.Fatal(err)
		}
	}
	touch(true)
	defer touch(false)
	out = append(out, b.eval(acts))
	check(t, "touch", out, "flex|none")
}

// TestUIKeys opens the window of the keys and says what works where; f and x press what a row or the bar shows.
func TestUIKeys(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.stub(findList)
	const open = "document.getElementById('keys').open"
	const shown = "[...document.querySelectorAll('.keys-part')].map((p) => p.hidden).join(' ') + ' ' + [...document.querySelectorAll('#keys kbd')].filter((k) => k.offsetParent).map((k) => k.textContent).join('')"
	const ways = "[getComputedStyle(document.getElementById('keys-open')).display !== 'none', getComputedStyle(document.getElementById('find-open')).display !== 'none'].join(' ')"
	out := make([]string, 0, 16)
	b.navigate("/")
	b.size(1440, 900, false)
	// Not in a field.
	b.eval("document.getElementById('q').focus()")
	keyDown(b, "key: '?', shiftKey: true")
	out = append(out, b.eval(open))
	b.eval("document.activeElement.blur()")
	keyDown(b, "key: '?', shiftKey: true")
	out = append(out, b.eval(open), b.eval(shown))
	if err := chromedp.Do(b.ctx, chromedp.KeyEvent(kb.Escape)); err != nil {
		t.Fatal(err)
	}
	out = append(out, b.eval(open))
	// The bar's ways: of the keys above 56rem, of finding on a narrow whole page alone.
	out = append(out, b.eval(ways))
	b.eval("document.getElementById('keys-open').click()")
	out = append(out, b.eval(open))
	b.eval("document.getElementById('keys').close()")
	b.size(800, 700, false)
	out = append(out, b.eval(ways))
	b.size(400, 700, false)
	out = append(out, b.eval(ways))
	b.navigate("/a/a/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	out = append(out, b.eval(ways))
	b.eval("document.getElementById('find-open').click()")
	out = append(out, b.eval("document.getElementById('find').open"))
	b.eval("document.getElementById('find').close()")
	b.size(1440, 900, false)
	out = append(out, b.eval(ways))
	// A whole page: only what works there; / finds, as there is no search; f presses the star shown, not one hidden.
	keyDown(b, "key: '?', shiftKey: true")
	out = append(out, b.eval(shown))
	b.eval("document.getElementById('keys').close(); document.activeElement.blur()")
	keyDown(b, "key: '/'")
	out = append(out, b.eval("document.getElementById('find').open"))
	b.eval("document.getElementById('find').close(); document.activeElement.blur()")
	b.eval("window.starred = 0; document.querySelector('#frame-actions .star').onclick = () => { window.starred++; }")
	keyDown(b, "key: 'f'")
	b.eval("document.querySelector('#frame-actions .star').hidden = true")
	keyDown(b, "key: 'f'")
	out = append(out, b.eval("window.starred"))
	// A page missing has the search: / goes there.
	b.navigate("/a/no-such/")
	b.until("the search", "!document.getElementById('search').hidden")
	keyDown(b, "key: '/'")
	out = append(out, b.eval("document.activeElement.id + ' ' + document.getElementById('find').open"))
	// In the list, f on a row whose star is hidden does nothing.
	b.navigate("/")
	b.eval("(() => { const li = document.querySelector('#list li[data-slug=a]'); window.starred = 0; const s = li.querySelector('.star'); s.hidden = true; s.onclick = () => { window.starred++; }; li.querySelector('.title').focus(); })()")
	keyDown(b, "key: 'f'")
	out = append(out, b.eval("window.starred"))
	// On a Mac, the keys as written there.
	b.addScript("Object.defineProperty(navigator, 'platform', { get: () => 'MacIntel' })")
	b.navigate("/")
	out = append(out, b.eval("document.querySelector('#keys-list .mod').textContent + ' ' + document.querySelector('#keys-list .keys-note').textContent.includes('⌘K')"))
	check(t, "keys", out, strings.Join([]string{
		"false", "true", "false true j↓k↑Enterxf/CtrlK?Esc",
		"false", "true false", "true", "false false", "false false", "false true", "true", "true false",
		"true false jkf/CtrlK?Esc", "true", "1", "q false", "0", "⌘ true",
	}, "|"))
}

// bulkFits says whether the bulk bar holds it all, its count whole, with a list of the width and the entries selected.
func bulkFits(b *browser, column int64, slugs ...string) string {
	b.t.Helper()
	b.eval("document.getElementById('bulk-clear').click(); setColumn('list', " + fmt.Sprint(column) + ")")
	for _, slug := range slugs {
		b.eval("document.querySelector('#list li[data-slug=" + slug + "] .sel').click()")
	}
	return b.eval(`(() => { const bar = document.getElementById('bulk'), c = document.getElementById('bulk-count');
		return [document.querySelector('#bulk-delete .label').textContent, bar.scrollWidth <= bar.clientWidth, document.getElementById('bulk-n').scrollWidth <= c.clientWidth].join(' '); })()`)
}

// TestUIBulkBar draws the bar as a part of the head: words alone, the head of the selected rows' ground, red ground only for
// the delete waiting, and all of it with the count readable on the narrowest list, in the longest words.
func TestUIBulkBar(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.stub(`[{ slug: 'l', title: 'L', description: '', path: '/tmp/l.html', createdAt: at(1), updatedAt: at(9) },
		{ slug: 'k', title: 'K', description: '', path: '/tmp/k.html', createdAt: at(1), updatedAt: at(8) },
		{ slug: 'a', title: 'A', description: '', createdAt: at(1), updatedAt: at(7) }]`)
	b.navigate("/")
	b.size(1440, 900, false)
	out := []string{bulkFits(b, 320, "a"), bulkFits(b, 320, "l", "k"), bulkFits(b, 500, "l", "k")}
	const looks = `(() => { const d = getComputedStyle(document.getElementById('bulk-delete')), h = getComputedStyle(document.querySelector('.lhead'));
		return [d.backgroundColor, d.color, h.backgroundColor, getComputedStyle(document.getElementById('bulk')).boxShadow, document.querySelectorAll('#bulk .btn:not(.clear) svg').length].join(' / '); })()`
	out = append(out, b.settled(looks))
	b.eval("document.getElementById('bulk-delete').click()")
	out = append(out, b.settled("getComputedStyle(document.getElementById('bulk-delete')).backgroundColor"))
	check(t, "bulk bar", out, strings.Join([]string{
		"Delete true true", "Unregister true true", "Unregister true true",
		"rgba(0, 0, 0, 0) / rgb(184, 51, 43) / rgb(236, 235, 230) / none / 0",
		"rgb(184, 51, 43)",
	}, "|"))
}

// frameKeys makes a whole page's keys say that they came, and catches what a page sends, for a press to be read.
const framedCatch = `window.got = []; window.seen = []; addEventListener('message', (e) => {
	if (typeof e.data?.artifactKey === 'string') window.got.push(e.data.artifactKey);
	if (e.data?.seen) window.seen.push(e.data.seen + ':' + e.data.prevented);
	if (e.data?.ran) window.ran = e.data.ran;
	if (e.data?.ready) window.ready = true;
	if (e.data?.there) window.there = true;
}, true);`

// The page: fields of every kind and what can be asked of it from the test, its keys told back after its own frame's script.
const keysPage = `<input id="t"><input id="c" type="checkbox"><input id="r" type="radio"><textarea id="a"></textarea><select id="s"><option>1</select>
<div id="e" contenteditable="true">a<span id="f" contenteditable="false" tabindex="0">b</span></div><button id="b">b</button>
<script>
const home = location.protocol + '//' + location.host;
addEventListener('keydown', (e) => { if (window.pageTakes) e.preventDefault(); setTimeout(() => parent.postMessage({ seen: e.key, prevented: e.defaultPrevented }, home)); });
addEventListener('message', (e) => { if (e.data?.artifactKeysWanted) parent.postMessage({ ready: 1 }, home); if (e.data?.run) { does[e.data.run](e.data.arg); parent.postMessage({ ran: e.data.n }, home); } });
const does = {
  focus: (id) => document.getElementById(id).focus(),
  design: () => { document.designMode = 'on'; },
  stop: () => { window.pageTakes = true; },
  rest: () => { window.pageTakes = false; document.designMode = 'off'; document.activeElement.blur(); },
  post: (keys) => { for (const k of keys) parent.postMessage({ artifactKey: k }, home); },
  go: (url) => { setTimeout(() => { location.href = url; }); },
};
</script>`

// inPage runs code in the page and waits until it has.
func inPage(b *browser, frame, do, arg string) {
	b.t.Helper()
	b.eval("window.ranN = (window.ranN ?? 0) + 1; document.getElementById(" + js(frame) + ").contentWindow.postMessage({ run: " + js(do) + ", arg: " + arg + ", n: window.ranN }, '*')")
	b.until("the page to run it", "window.ran === window.ranN")
}

// focusIn clicks into the iframe until a key reaches its page: Chrome routes input into a frame only once it has drawn, and a
// click or a key sent before is lost.
func focusIn(b *browser, frame string) {
	b.t.Helper()
	r := b.rect("document.getElementById(" + js(frame) + ")")
	var last string
	for deadline := time.Now().Add(waitLimit); time.Now().Before(deadline); {
		b.click(r.X+r.Width-20, r.Y+r.Height-20)
		realKey(b, "q", 0)
		time.Sleep(50 * time.Millisecond)
		if last = b.eval("document.activeElement.id === " + js(frame) + " && window.seen.includes('q:false')"); last == "true" {
			return
		}
	}
	b.t.Fatalf("timed out waiting for a key to reach the page in %s: last %q", frame, last)
}

// realKey presses k as a keyboard does, with the modifiers (1 Alt, 2 Ctrl, 4 Meta, 8 Shift).
func realKey(b *browser, k string, modifiers int64) {
	b.t.Helper()
	for _, e := range kb.Encode([]rune(k)[0]) {
		if e.Type == kb.KeyChar {
			continue
		}
		e.Modifiers = modifiers
		if modifiers != 0 && modifiers != 8 {
			e.Text = ""
			e.UnmodifiedText = ""
		}
		if _, err := chromedp.Call(b.ctx, input.DispatchKeyEvent, *e); err != nil {
			b.t.Fatal(err)
		}
	}
}

// TestUIPageKeys presses real keys with the focus in a real page: those that move on or open a window reach the frame, those
// in a field or changing an entry do not; a page floe did not draw hands over Ctrl+K alone, and a frame that does not say it
// takes them gets none. The frame takes them only from its own iframe, of the sandbox, with the focus there, of its list.
func TestUIPageKeys(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	ext := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><script>parent.postMessage({ artifactKey: 'j' }, '*'); parent.postMessage({ artifactOpen: '/' }, '*'); parent.postMessage({ there: 1 }, '*');</script>`)
	}))
	defer ext.Close()
	s := newSite(t)
	b := s.b
	for _, slug := range []string{"d1", "d2", "d3"} {
		s.floe(t, "publish", s.page(t, slug+".yaml", slug, keysPage), "--slug", slug)
	}
	s.floe(t, "publish", "--link", s.write(t, "site/report.html", "<!doctype html><title>r</title><body>"+keysPage), "--slug", "linked")
	s.write(t, "dir/index.html", "<!doctype html><title>i</title><body>"+keysPage)
	s.floe(t, "publish", "--link", s.Env["HOME"]+"/dir", "--slug", "folder")
	b.navigate("about:blank")
	b.addScript(framedCatch)
	open := func(path, frame string) {
		b.navigate(path)
		b.until("the page to be told the keys are taken", "window.ready")
		// The key's own work is not this test's, but that it is told: the keys go nowhere, and the page stays.
		b.eval("for (const k of ['j', 'k', '?', '/', 'Ctrl+K', 'f']) KEYS[k] = () => { window.done = (window.done ?? '') + k + ' '; }")
		focusIn(b, frame)
		inPage(b, frame, "rest", "0")
	}
	// Pressed, then ? as the mark of what has come before it: what the frame was given by then.
	press := func(frame, do, arg, k string, mods int64) string {
		b.eval("window.got = []; window.seen = []; window.done = ''")
		if do != "" {
			inPage(b, frame, do, arg)
		}
		realKey(b, k, mods)
		inPage(b, frame, "rest", "0")
		realKey(b, "?", 8)
		b.until("the mark", "window.seen.at(-1) === '?:true' || window.seen.at(-1) === '?:false'")
		b.until("the frame told", "window.seen.at(-1) === '?:false' || window.got.at(-1) === '?'")
		return b.eval("window.got.join(',')")
	}
	out := make([]string, 0, 32)
	open("/a/d1/", "frame")
	for _, c := range []struct {
		do, arg, k string
		mods       int64
	}{
		{"", "", "j", 0}, {"", "", "k", 0}, {"", "", "/", 0}, {"", "", "k", 2}, {"", "", "k", 4},
		{"focus", "'c'", "j", 0}, {"focus", "'r'", "j", 0}, {"focus", "'f'", "j", 0},
		{"focus", "'t'", "j", 0}, {"focus", "'t'", "k", 2}, {"focus", "'a'", "j", 0}, {"focus", "'s'", "j", 0}, {"focus", "'e'", "j", 0},
		{"design", "0", "j", 0}, {"stop", "0", "j", 0},
		{"", "", "f", 0}, {"", "", "x", 0}, {"", "", "j", 2}, {"", "", "k", 1}, {"", "", "K", 2 | 8},
	} {
		out = append(out, press("frame", c.do, c.arg, c.k, c.mods))
	}
	// Escape is the page's own.
	b.eval("window.got = []; window.seen = []")
	if err := chromedp.Do(b.ctx, chromedp.KeyEvent(kb.Escape)); err != nil {
		t.Fatal(err)
	}
	b.until("the page told", "window.seen.length > 0")
	out = append(out, b.eval("window.got.length + ' ' + window.seen.join(',')"))
	// What the frame did with them.
	out = append(out, b.eval("window.done.trim()"))
	// A page floe did not draw: Ctrl+K alone, in a link and in a folder.
	for _, at := range []string{"/a/linked/", "/a/folder/?file=index.html"} {
		open(at, "frame")
		b.eval("window.got = []; window.seen = []")
		realKey(b, "j", 0)
		realKey(b, "k", 2)
		b.until("the page told", "window.seen.length === 2")
		out = append(out, b.eval("window.got.join(',') + ' ' + window.seen.join(',')"))
	}
	// Forged by a page: not of the list, not while the focus is elsewhere.
	open("/a/d1/", "frame")
	b.eval("window.done = ''; KEYS.f = () => { window.done += 'f '; }")
	inPage(b, "frame", "post", "['f', 'x', '__proto__', 'constructor', 5, null]")
	inPage(b, "frame", "post", "['j']")
	b.until("a key that is taken", "window.done === 'j '")
	b.eval("window.done = ''; document.getElementById('back').focus()")
	inPage(b, "frame", "post", "['j']")
	out = append(out, b.eval("window.done + '.'"))
	// Another iframe, framed by the frame but not its own: nothing is taken from the page, and nothing it sends is.
	b.eval("window.got = []; window.seen = []; window.done = ''; window.other = document.createElement('iframe'); other.id = 'other'; other.onload = () => { window.otherUp = true; }; other.src = '/a/d2/?raw'; other.style = 'position: fixed; inset: 0; width: 300px; height: 200px; z-index: 9'; document.body.append(other)")
	b.until("the other loaded", "window.otherUp")
	focusIn(b, "other")
	b.eval("window.seen = []")
	realKey(b, "/", 0)
	b.until("the other told", "window.seen.length > 0")
	inPage(b, "other", "post", "['j']")
	out = append(out, b.eval("window.seen.join(',') + ' ' + window.done + '.'"))
	b.eval("other.remove()")
	// The iframe gone out to another site: nothing it says is heard, the key or the way to move.
	open("/a/d1/", "frame")
	b.eval("window.done = ''")
	inPage(b, "frame", "go", js(ext.URL))
	b.until("the other site", "window.there")
	out = append(out, b.eval("window.done + '. ' + location.pathname"))
	// Beside the list: j moves on through the rows, the focus in the page; the bar is what goes, f not at all.
	b.size(1440, 900, false)
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=d1] .title')")
	b.eval("document.querySelector('#list li[data-slug=d3] .title').click()")
	b.until("the page beside to be told the keys are taken", "window.ready")
	focusIn(b, "read-frame")
	b.eval("window.got = []")
	realKey(b, "f", 0)
	realKey(b, "j", 0)
	b.until("the next row", "picked?.slug === 'd2'")
	out = append(out, b.eval("window.got.join(',')"))
	check(t, "page keys", out, strings.Join([]string{
		"j,?", "k,?", "/,?", "Ctrl+K,?", "Ctrl+K,?",
		"j,?", "j,?", "j,?",
		"?", "?", "?", "?", "?",
		"?", "?",
		"?", "?", "?", "?", "?",
		"0 Escape:false",
		"?",
		"Ctrl+K j:false,k:true", "Ctrl+K j:false,k:true",
		".", "/:false .", ". /a/d1/", "j",
	}, "|"))
}

// TestUIComposing leaves to the input method what goes to it, in every field that takes a key: the tags', the search of a page
// missing, and the search of the list.
func TestUIComposing(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.stub(findList)
	b.navigate("/")
	b.size(1440, 900, false)
	out := make([]string, 0, 6)
	// The search keeps its words through an Escape that ends the composing.
	b.eval("document.getElementById('q').value = 'al'; document.getElementById('q').focus()")
	keyDown(b, "key: 'Escape', isComposing: true")
	out = append(out, b.eval("document.getElementById('q').value"))
	b.eval("document.getElementById('q').value = ''; document.activeElement.blur()")
	// No key of the list goes off.
	b.eval("document.querySelector('#list li[data-slug=a] .title').focus()")
	keyDown(b, "key: 'j', isComposing: true")
	keyDown(b, "key: '?', keyCode: 229")
	out = append(out, b.eval("document.activeElement.textContent + ' ' + document.getElementById('keys').open"))
	// The tags' field makes no tag of a word being composed.
	b.eval("document.querySelector('#list li[data-slug=a] .sel').click(); document.getElementById('bulk-tag').click(); document.getElementById('tag-find').value = 'new'; document.getElementById('tag-find').focus(); document.getElementById('tag-find').dispatchEvent(new Event('input'))")
	keyDown(b, "key: 'Enter', isComposing: true")
	out = append(out, b.eval("document.getElementById('tag-find').value"))
	keyDown(b, "key: 'Enter'")
	out = append(out, b.eval("document.getElementById('tag-find').value"))
	// A page missing: the search stays until its Enter is its own.
	b.navigate("/a/no-such/")
	b.until("the search", "!document.getElementById('search').hidden")
	b.eval("document.getElementById('q').value = 'al'; document.getElementById('q').focus()")
	keyDown(b, "key: 'Enter', isComposing: true")
	out = append(out, b.eval("location.pathname"))
	check(t, "composing", out, "al|Alpha plan false|new||/a/no-such/")
}
