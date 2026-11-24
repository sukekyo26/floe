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
	const lis = [...document.querySelectorAll('#find-list li')], slugOf = (li) => new URL(li.querySelector('a').href).pathname.split('/')[2];
	const rows = lis.map((li) => li.getAttribute('aria-selected') === 'true' ? '[' + slugOf(li) + ']' : slugOf(li)).join('');
	const acted = lis.find((li) => slugOf(li) === document.getElementById('find-actions').dataset.slug);
	return [document.getElementById('find').open, rows, document.getElementById('find-acts').hidden ? '-' : acted.querySelector('[data-testid=title]').textContent,
		document.getElementById('find-cut').textContent || '-'].join(' / ');
})()`

// keyDown sends the key from what has the focus, as the page's handlers see it.
func keyDown(b *browser, init string) {
	b.t.Helper()
	b.moving("(document.activeElement ?? document.body).dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, " + init + " }))")
}

func typeFind(b *browser, words string) {
	b.t.Helper()
	b.fill("#find-q", words)
}

// pressFindOpen opens the search modal by the bar's button, or by Ctrl+K on the page list, whose search field stands for the
// button (S422-P9).
func pressFindOpen(b *browser) {
	b.t.Helper()
	if b.eval("String(document.getElementById('find-open') !== null)") == "true" {
		b.eval("document.getElementById('find-open').click()")
	} else {
		keyDown(b, "key: 'k', ctrlKey: true")
	}
	b.until("the search modal", "document.getElementById('find').open")
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
	b.eval("document.getElementById('add-link').click()")
	keyDown(b, "key: 'k', ctrlKey: true")
	out = append(out, b.eval("document.getElementById('find').open + ' ' + document.getElementById('add-dialog').open"))
	b.pressKey(kb.Escape)
	b.until("the add modal closed", "!document.getElementById('add-dialog').open")
	b.eval("document.activeElement.blur()")
	keyDown(b, "key: 'k', ctrlKey: true")
	// Found by the description, by the path; the archived last, marked; none.
	b.fill("#find-q", "the words")
	out = append(out, b.eval(findDrawn))
	b.fill("#find-q", "/tmp/x/c")
	out = append(out, b.eval(findDrawn))
	b.fill("#find-q", "plan")
	out = append(out, b.eval(findDrawn),
		b.eval("document.querySelector('#find-list li:last-child [data-testid=badge]').textContent + ' ' + document.querySelectorAll('#find-list [data-testid=badge]').length"))
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
	out = append(out, b.eval("document.getElementById('find').open + ' ' + document.activeElement.matches('#find-actions > [data-testid=download]')"))
	b.eval("document.getElementById('find-q').focus()")
	// None: no rows, no buttons, no foot, the reason; Enter goes nowhere.
	b.fill("#find-q", "nothing like it")
	out = append(out, b.eval(findDrawn), b.eval("document.getElementById('find-none').hidden + ' ' + document.getElementById('find-none').textContent.includes('.')"))
	// Its way to close stays, as there is none else without a key.
	out = append(out, b.eval("document.querySelector('#find [data-close]').getBoundingClientRect().width > 0"))
	keyDown(b, "key: 'Enter'")
	out = append(out, b.eval("location.pathname"))
	// What goes to the input method: no row moves, nothing opens or closes.
	b.fill("#find-q", "plan")
	keyDown(b, "key: 'ArrowDown', isComposing: true")
	keyDown(b, "key: 'Enter', isComposing: true")
	keyDown(b, "key: 'Enter', keyCode: 229")
	keyDown(b, "key: 'k', ctrlKey: true, isComposing: true")
	out = append(out, b.eval(findDrawn), b.eval("location.pathname"))
	// A row that stays is the one reached as the rows change, though at another place; one gone, what has come to its place, or the last; none, nothing.
	b.fill("#find-q", "")
	keyDown(b, "key: 'ArrowDown'")
	keyDown(b, "key: 'ArrowDown'")
	drop := func(slugs string) {
		b.eval("window.list = JSON.stringify(JSON.parse(window.list).filter((a) => !" + js(slugs) + ".includes(a.slug)))")
		b.reread()
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
	// None and then rows, as when it opens before the list is read: the first is reached, and Enter goes.
	b.eval("window.list = " + js("[{\"slug\":\"a\",\"title\":\"A\",\"description\":\"\",\"updatedAt\":\"2026-01-01T00:00:00.000Z\"}]"))
	b.reread()
	b.until("the row", "document.querySelector('#find-list li')")
	out = append(out, b.eval(findDrawn))
	// Rows, none and then rows again: the place reached before none is reached again, not the first.
	three := js(`[{"slug":"a","title":"A","description":"","updatedAt":"2026-01-03T00:00:00.000Z"},` +
		`{"slug":"b","title":"B","description":"","updatedAt":"2026-01-02T00:00:00.000Z"},` +
		`{"slug":"c","title":"C","description":"","updatedAt":"2026-01-01T00:00:00.000Z"}]`)
	b.eval("window.list = " + three)
	b.reread()
	b.until("the rows", "document.querySelectorAll('#find-list li').length === 3")
	keyDown(b, "key: 'ArrowDown'")
	keyDown(b, "key: 'ArrowDown'")
	out = append(out, b.eval(findDrawn))
	drop("abc")
	out = append(out, b.eval(findDrawn))
	b.eval("window.list = " + three)
	b.reread()
	b.until("the rows back", "document.querySelectorAll('#find-list li').length === 3")
	out = append(out, b.eval(findDrawn))
	b.eval("document.querySelector('#find [data-close]').click()")
	out = append(out, b.eval("document.getElementById('find').open"))
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
		"true /  / - / 0 found", "false true", "true",
		"/",
		"true / [a]cz / Alpha plan / 3 found", "/",
		"true / ab[c]fz / Gamma plan / Newest first",
		"true / a[c]fz / Gamma plan / Newest first",
		"true / a[f]z / Folder / Newest first",
		"true / af[z] / Zed plan / Newest first",
		"true / a[f] / Folder / Newest first",
		"true /  / - / 0 found",
		"/ 0",
		"true / [a] / A / Newest first",
		"true / ab[c] / C / Newest first",
		"true /  / - / 0 found",
		"true / ab[c] / C / Newest first", "false",
	}, "|"))
}

// TestUIFindCut says that it shows the first rows only.
func TestUIFindCut(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.stub("Array.from({ length: 51 }, (_, i) => ({ slug: 's' + i, title: 'T' + i, description: '', createdAt: at(1), updatedAt: at(9) }))")
	b.navigate("/")
	keyDown(b, "key: 'k', ctrlKey: true")
	out := make([]string, 0, 2)
	out = append(out, b.eval("document.querySelectorAll('#find-list li').length + ' ' + document.getElementById('find-cut').textContent"))
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
	b.eval("window.errs = []; addEventListener('error', (e) => window.errs.push(e.message)); addEventListener('unhandledrejection', (e) => window.errs.push(String(e.reason)))")
	if b.eval("matchMedia('(hover: hover)').matches") != "true" {
		t.Fatal("this runs with a mouse")
	}
	change := func(slug, fields string) {
		b.eval("window.list = JSON.stringify(JSON.parse(window.list).map((a) => a.slug === " + js(slug) + " ? { ...a, " + fields + " } : a))")
		b.reread()
	}
	const menu = "#find-actions [data-testid=more]"
	out := make([]string, 0, 24)
	keyDown(b, "key: 'k', ctrlKey: true")
	keyDown(b, "key: 'ArrowDown'") // b
	// The star: the same button, the focus on it, what the row is told.
	b.eval("(() => { const s = document.querySelector('#find-actions [data-testid=star]'); s.focus(); s.click(); })()")
	change("b", "favorite: false")
	b.until("the star drawn", "document.querySelector('#find-actions [data-testid=star]').getAttribute('aria-pressed') === 'false'")
	out = append(out, b.eval("document.activeElement.matches('#find-actions [data-testid=star]')"))
	// The menu is whole, inside the window of any height it fits, and an answer does not close it.
	typeFind(b, "beta") // one row: a window far shorter than the menu
	for _, size := range [][2]int64{{1440, 900}, {400, 800}} {
		b.size(size[0], size[1], false)
		b.eval("document.querySelector('" + menu + " summary').click()")
		out = append(out, b.eval(`(() => { const r = document.querySelector('`+menu+` [data-testid=menu]').getBoundingClientRect();
			return r.bottom <= innerHeight && r.right <= innerWidth && r.left >= 0 && document.querySelector('`+menu+` [data-testid=menu]').contains(document.elementFromPoint(r.left + r.width / 2, r.bottom - 8)); })()`))
		b.eval("document.querySelector('" + menu + " summary').click()")
	}
	b.size(1440, 900, false)
	typeFind(b, "")
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
	b.eval("document.querySelector('" + menu + " summary').focus(); document.querySelector('" + menu + " summary').click(); document.querySelector('" + menu + " [data-testid=archive]').click()")
	change("b", "archived: true")
	b.until("the row moved", inMenuItem("#find-actions", "archive", "i.querySelector('[data-testid=label]').textContent")+" === 'Restore'")
	out = append(out, b.eval("document.activeElement.matches('"+menu+" summary')"))
	change("b", "archived: false")
	b.until("the row back", inMenuItem("#find-actions", "archive", "i.querySelector('[data-testid=label]').textContent")+" === 'Archive'")
	// A delete waiting stays through an answer.
	b.eval("document.querySelector('" + menu + " summary').click(); document.querySelector('" + menu + " [data-testid=delete]').focus(); document.querySelector('" + menu + " [data-testid=delete]').click()")
	change("a", "title: 'Alpha plan'")
	b.until("the rows drawn", "document.querySelector('#find-list a').textContent === 'Alpha plan'")
	out = append(out, b.eval("document.querySelector('"+menu+" [data-testid=delete]').dataset.armed === 'true'"))

	// The menu shut by its button lets it go (TestUIRowMenuDisarms); closed by its key with a delete waiting, on the row it opens
	// on, the search opens anew with the menu shut.
	typeFind(b, "")
	b.eval("(() => { const m = document.querySelector('" + menu + "'); if (m.open) m.querySelector('summary').click(); })()")
	menuItemShown(b, "#find-actions", "delete", "")
	out = append(out, b.eval("document.querySelector('"+menu+" [data-testid=delete]').hasAttribute('data-armed')"))
	b.eval("document.querySelector('" + menu + " [data-testid=delete]').click()")
	b.until("the delete armed", "document.querySelector('"+menu+" [data-testid=delete]').dataset.armed")
	keyDown(b, "key: 'k', ctrlKey: true")
	keyDown(b, "key: 'k', ctrlKey: true")
	out = append(out, b.eval("document.querySelector('"+menu+"').open"))
	// What comes of the buttons shows over the rest: a toast, a mistake; and they go back with the window.
	const seen = `((id) => { const e = document.getElementById(id); return document.getElementById('find').contains(e) && !e.hidden && e.getBoundingClientRect().height > 0; })`
	b.eval("document.querySelector('#find-actions [data-testid=star]').click()")
	b.until("the toast", seen+"('toasts') && document.querySelector('#toasts [data-testid=toast]')")
	b.eval("window.failWrites = true; document.querySelector('#find-actions [data-testid=star]').click()")
	b.until("the mistake", seen+"('error')")
	out = append(out, b.eval(`(() => { const r = document.getElementById('error').getBoundingClientRect(); return document.getElementById('error').contains(document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2)); })()`))
	b.eval("window.failWrites = false")
	escapeDialog(b, "find")
	b.until("back", "!document.getElementById('find').contains(document.getElementById('error')) && !document.getElementById('find').contains(document.getElementById('toasts'))")
	// The buttons on a narrow whole page, where those of its bar go.
	b.size(400, 800, false)
	b.navigate("/a/c/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	keyDown(b, "key: 'k', ctrlKey: true")
	out = append(out, b.eval("[getComputedStyle(document.getElementById('find-actions')).display !== 'none', getComputedStyle(document.querySelector('#frame-actions [data-testid=star]')).display].join(' ')"))
	b.size(1440, 900, false)
	// From a whole page: a delete of another page stays there, the focus going to the field.
	b.navigate("/a/c/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	b.eval("window.errs = []; addEventListener('error', (e) => window.errs.push(e.message)); addEventListener('unhandledrejection', (e) => window.errs.push(String(e.reason)))")
	keyDown(b, "key: 'k', ctrlKey: true")
	typeFind(b, "beta")
	b.eval("document.querySelector('" + menu + " summary').click(); document.querySelector('" + menu + " [data-testid=delete]').focus(); document.querySelector('" + menu + " [data-testid=delete]').click()")
	b.eval("window.list = JSON.stringify(JSON.parse(window.list).filter((a) => a.slug !== 'b'))")
	b.eval("document.querySelector('" + menu + " [data-testid=delete]').click()")
	b.until("the row gone", "document.querySelectorAll('#find-list li').length === 0")
	out = append(out, b.eval("[location.pathname, document.getElementById('find').open, document.activeElement.id].join(' ')"))
	// The edit closes the window, no dialog going over another, and it goes back with no mistake.
	typeFind(b, "alpha")
	b.eval("document.querySelector('" + menu + " summary').click(); document.querySelector('" + menu + " [data-testid=edit]').click()")
	out = append(out, b.eval("document.getElementById('find').open + ' ' + document.getElementById('edit-dialog').open"))
	// Another page's new name leaves this one here. The save is the form's submit, waited for its end: closed, and what it threw.
	openSlugField(b)
	b.fill("#edit-slug", "a2")
	begun := b.acted("document.getElementById('edit-submit').click()")
	b.untilRead(begun, "the save's end", "!document.getElementById('edit-dialog').open")
	out = append(out, b.eval("location.pathname + ' ' + window.errs"))
	// This page's: its URL moves, and deleted, it goes to the list.
	keyDown(b, "key: 'k', ctrlKey: true")
	typeFind(b, "gamma")
	b.eval("document.querySelector('" + menu + " summary').click(); document.querySelector('" + menu + " [data-testid=edit]').click()")
	openSlugField(b)
	b.fill("#edit-slug", "c2")
	b.moving("document.getElementById('edit-submit').click()")
	b.until("the new URL", "location.pathname === '/a/c2/'")
	b.navigate("/a/c/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	keyDown(b, "key: 'k', ctrlKey: true")
	typeFind(b, "gamma")
	b.pressed("document.querySelector('"+menu+" summary').click(); document.querySelector('"+menu+" [data-testid=delete]').focus(); document.querySelector('"+menu+" [data-testid=delete]').click()",
		"the delete armed", "document.querySelector('"+menu+" [data-testid=delete]').hasAttribute('data-armed')")
	b.moving("document.querySelector('" + menu + " [data-testid=delete]').click()")
	b.until("the list", "location.pathname === '/'")
	check(t, "actions", out, strings.Join([]string{
		"true", "true", "true", "true", "false true true", "false", "true", "true", "false", "false", "true", "true none",
		"/a/c/ true find-q", "false true", "/a/c/ ",
	}, "|"))
}

// TestUIFindTouch leaves out the buttons where there is no mouse: a press on a row opens it, and they could act on the first alone.
func TestUIFindTouch(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.stub("Array.from({ length: 40 }, (_, i) => ({ slug: 's' + i, title: 'T' + i, description: '', createdAt: at(1), updatedAt: at(9) }))")
	b.navigate("/")
	b.size(400, 800, false)
	acts := "getComputedStyle(document.getElementById('find-acts')).display"
	// The foot, with what it says, within the window and the screen.
	const foot = `(() => { const f = document.querySelector('#find [data-testid=find-foot]').getBoundingClientRect(), w = document.getElementById('find').getBoundingClientRect();
		return f.top >= w.top && f.bottom <= w.bottom + 0.5 && f.bottom <= innerHeight; })()`
	keyDown(b, "key: 'k', ctrlKey: true")
	out := make([]string, 0, 4)
	out = append(out, b.eval(acts), b.eval(foot))
	touch := func(on bool) {
		if _, err := chromedp.Call(b.ctx, emulation.SetTouchEmulationEnabled, emulation.SetTouchEmulationEnabledParams{Enabled: on, MaxTouchPoints: 1}); err != nil {
			t.Fatal(err)
		}
	}
	touch(true)
	defer touch(false)
	out = append(out, b.eval(acts), b.eval(foot))
	check(t, "touch", out, "flex|true|none|true")
}

// TestUIKeys opens the window of the keys and says what works where; f and x press what a row or the bar shows.
func TestUIKeys(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.stub(findList)
	const open = "document.getElementById('keys').open"
	const shown = "[...document.querySelectorAll('[data-testid=keys-part]')].map((p) => p.hidden).join(' ') + ' ' + [...document.querySelectorAll('#keys kbd')].filter((k) => k.offsetParent).map((k) => k.textContent).join('')"
	const ways = "[getComputedStyle(document.getElementById('keys-open')).display !== 'none', ((e) => e !== null && getComputedStyle(e).display !== 'none')(document.getElementById('find-open'))].join(' ')"
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
	// The bar's ways: of the help above 40rem, but not on a page whole; of finding on a narrow whole page alone.
	out = append(out, b.eval(ways))
	b.eval("document.getElementById('keys-open').click()")
	out = append(out, b.eval(open))
	b.pressKey(kb.Escape)
	b.until("the help closed", "!document.getElementById('keys').open")
	b.size(800, 700, false)
	out = append(out, b.eval(ways))
	b.size(400, 700, false)
	out = append(out, b.eval(ways))
	b.navigate("/a/a/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	out = append(out, b.eval(ways))
	b.eval("document.getElementById('find-open').click()")
	out = append(out, b.eval("document.getElementById('find').open"))
	b.pressKey(kb.Escape)
	b.until("the search closed", "!document.getElementById('find').open")
	b.size(1440, 900, false)
	out = append(out, b.eval(ways))
	// A whole page: only what works there; / finds, as there is no search; f presses the star shown, not one hidden.
	keyDown(b, "key: '?', shiftKey: true")
	out = append(out, b.eval(shown))
	// Each row's keys on one line, whatever the word between them.
	out = append(out, b.eval("[...document.querySelectorAll('#keys-frame dt')].every((d) => d.getBoundingClientRect().height < 32)"))
	b.pressKey(kb.Escape)
	b.until("the help closed", "!document.getElementById('keys').open")
	b.eval("document.activeElement.blur()")
	keyDown(b, "key: '/'")
	out = append(out, b.eval("document.getElementById('find').open"))
	b.pressKey(kb.Escape)
	b.until("the search closed", "!document.getElementById('find').open")
	b.eval("document.activeElement.blur()")
	b.eval("window.starred = 0; document.querySelector('#frame-actions [data-testid=star]').onclick = () => { window.starred++; }")
	keyDown(b, "key: 'f'")
	b.eval("document.querySelector('#frame-actions [data-testid=star]').hidden = true")
	keyDown(b, "key: 'f'")
	out = append(out, b.eval("window.starred"))
	// A page missing has the search: / goes there.
	b.navigate("/a/no-such/")
	b.until("the search", "!document.getElementById('search').hidden")
	keyDown(b, "key: '/'")
	out = append(out, b.eval("document.activeElement.id + ' ' + document.getElementById('find').open"))
	// In the list, f on a row whose star is hidden does nothing.
	b.navigate("/")
	b.eval("(() => { const li = document.querySelector('#list li[data-slug=a]'); window.starred = 0; const s = li.querySelector('[data-testid=star]'); s.hidden = true; s.onclick = () => { window.starred++; }; li.querySelector('[data-testid=title]').focus(); })()")
	keyDown(b, "key: 'f'")
	out = append(out, b.eval("window.starred"))
	// On a Mac, the keys as written there.
	b.addScript("Object.defineProperty(navigator, 'platform', { get: () => 'MacIntel' })")
	b.navigate("/")
	out = append(out, b.eval("document.querySelector('#keys-list [data-testid=mod]').textContent + ' ' + document.getElementById('keys-note-list').textContent.includes('⌘K')"))
	check(t, "keys", out, strings.Join([]string{
		"false", "true", "false true j↓k↑Enterxf/CtrlK?Esc",
		"false", "true false", "true", "true false", "false false", "false true", "true", "false false",
		"true false jkhf/CtrlK?Esc", "true", "true", "1", "q false", "0", "⌘ true",
	}, "|"))
}

// bulkFits says, with a list of the width and the entries selected, what the delete says; that the clearing is in the list, and
// each button once reached; and that the count's number is whole. How wide the words are is the font's, which differs by machine.
func bulkFits(b *browser, column int64, slugs ...string) string {
	b.t.Helper()
	b.eval("document.getElementById('bulk-clear').click()")
	b.drag("#list-grip", b.rect("document.getElementById('listcol')").X+float64(column))
	for _, slug := range slugs {
		b.eval("document.querySelector('#list li[data-slug=" + slug + "] [data-testid=sel]').click()")
	}
	return b.eval(`(() => { const box = document.getElementById('listcol').getBoundingClientRect(), c = document.getElementById('bulk-count');
		const within = (e) => { const r = e.getBoundingClientRect(); return r.left >= box.left - 0.5 && r.right <= box.right + 0.5; };
		const reached = [...document.querySelectorAll('#bulk button:not(#bulk-clear)')].every((e) => { e.scrollIntoView({ inline: 'nearest', block: 'nearest' }); return within(e) && within(document.getElementById('bulk-clear')); });
		document.getElementById('bulk').scrollLeft = 0;
		const n = document.getElementById('bulk-n').getBoundingClientRect();
		return [document.querySelector('#bulk-delete [data-testid=label]').textContent, reached, n.width > 0 && n.right <= c.getBoundingClientRect().right + 0.5].join(' '); })()`)
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
	out := make([]string, 0, 7)
	out = append(out, bulkFits(b, 320, "a"), bulkFits(b, 320, "l", "k"), bulkFits(b, 500, "l", "k"))
	// Words far too wide for it, in any font.
	b.eval("window.wide = document.head.appendChild(document.createElement('style')); wide.textContent = '#bulk, #bulk button { letter-spacing: 0.5em }'")
	out = append(out, bulkFits(b, 320, "l", "k"), b.eval("(() => { const bar = document.getElementById('bulk'); bar.scrollLeft = 1000; const scrolled = bar.scrollLeft > 0; bar.scrollLeft = 0; return scrolled; })()"))
	b.eval("wide.remove()")
	const looks = `(() => { const d = getComputedStyle(document.getElementById('bulk-delete')), h = getComputedStyle(document.querySelector('[data-testid=lhead]'));
		return [d.backgroundColor, d.color, h.backgroundColor, getComputedStyle(document.getElementById('bulk')).boxShadow, document.querySelectorAll('#bulk button:not(#bulk-clear) svg').length].join(' / '); })()`
	out = append(out, b.settled(looks))
	b.eval("document.getElementById('bulk-delete').click()")
	out = append(out, b.settled("getComputedStyle(document.getElementById('bulk-delete')).backgroundColor"))
	check(t, "bulk bar", out, strings.Join([]string{
		"Delete true true", "Unregister true true", "Unregister true true", "Unregister true true", "true",
		"rgba(0, 0, 0, 0) / rgb(184, 51, 43) / rgb(236, 235, 230) / none / 0",
		"rgb(184, 51, 43)",
	}, "|"))
}

// framedCatch catches what a page sends, for a press to be read, and while window.holdKeys keeps the keys a page hands over from the
// frame, so that they go nowhere.
const framedCatch = `window.got = []; window.seen = []; addEventListener('message', (e) => {
	if (typeof e.data?.pageKey === 'string') window.got.push(e.data.pageKey);
	if (window.holdKeys && e.data?.pageKey !== undefined) e.stopImmediatePropagation();
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
addEventListener('message', (e) => { if (e.data?.pageKeysWanted) parent.postMessage({ ready: 1 }, home); if (e.data?.run) { does[e.data.run](e.data.arg); parent.postMessage({ ran: e.data.n }, home); } });
const does = {
  focus: (id) => document.getElementById(id).focus(),
  design: () => { document.designMode = 'on'; },
  stop: () => { window.pageTakes = true; },
  rest: () => { window.pageTakes = false; document.designMode = 'off'; document.activeElement.blur(); },
  post: (keys) => { for (const k of keys) parent.postMessage({ pageKey: k }, home); },
  go: (url) => { setTimeout(() => { location.href = url; }); },
  open: () => parent.postMessage({ pageOpen: '/' }, home),
  self: () => postMessage({ pageKeysWanted: true }, '*'),
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
		fmt.Fprint(w, `<!doctype html><script>parent.postMessage({ pageKey: '/' }, '*'); parent.postMessage({ pageOpen: '/' }, '*'); parent.postMessage({ there: 1 }, '*');</script>`)
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
		b.eval("window.holdKeys = true")
		focusIn(b, frame)
		inPage(b, frame, "rest", "0")
	}
	// Pressed, then the focus back in the page as a key it told of itself: what the frame was given by then, the focus going
	// back to the frame with a key that it takes.
	press := func(frame, do, arg, k string, mods int64) string {
		b.eval("window.got = []; window.seen = []")
		if do != "" {
			inPage(b, frame, do, arg)
		}
		realKey(b, k, mods)
		inPage(b, frame, "rest", "0")
		focusIn(b, frame)
		return b.eval("window.got.join(',') + '.'")
	}
	out := make([]string, 0, 32)
	open("/a/d1/", "frame")
	for _, c := range []struct {
		do, arg, k string
		mods       int64
	}{
		{"", "", "j", 0},
		{"", "", "k", 0},
		{"", "", "/", 0},
		{"", "", "k", 2},
		{"", "", "k", 4},
		{"focus", "'c'", "j", 0},
		{"focus", "'r'", "j", 0},
		{"focus", "'f'", "j", 0},
		{"focus", "'t'", "j", 0},
		{"focus", "'t'", "k", 2},
		{"focus", "'a'", "j", 0},
		{"focus", "'s'", "j", 0},
		{"focus", "'e'", "j", 0},
		{"design", "0", "j", 0},
		{"stop", "0", "j", 0},
		{"", "", "f", 0},
		{"", "", "x", 0},
		{"", "", "j", 2},
		{"", "", "k", 1},
		{"", "", "K", 2 | 8},
		{"", "", "?", 8},
		{"", "", "h", 0},
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
	// What the frame does with them: h goes back to the list.
	b.eval("window.holdKeys = false")
	realKey(b, "h", 0)
	b.until("the list", "location.pathname === '/'")
	out = append(out, b.eval("location.pathname"))
	// A page floe did not draw: Ctrl+K alone, in a link and in a folder.
	for _, at := range []string{"/a/linked/", "/a/folder/?file=index.html"} {
		open(at, "frame")
		b.eval("window.got = []; window.seen = []")
		realKey(b, "j", 0)
		realKey(b, "h", 0)
		realKey(b, "k", 2)
		b.until("the page told", "window.seen.length === 3")
		out = append(out, b.eval("window.got.join(',') + ' ' + window.seen.join(',')"))
	}
	// Forged by a page: not of the list (f, whose star is counted), not while the focus is elsewhere. / opens the search modal.
	open("/a/d1/", "frame")
	b.eval("window.holdKeys = false; window.starred = 0; document.querySelector('#frame-actions [data-testid=star]').addEventListener('click', () => { window.starred++; })")
	inPage(b, "frame", "post", "['f', 'x', '__proto__', 'constructor', 5, null]")
	inPage(b, "frame", "post", "['/']")
	b.until("a key that is taken", "document.getElementById('find').open")
	b.pressKey(kb.Escape)
	b.until("the search closed", "!document.getElementById('find').open")
	b.eval("document.getElementById('back').focus(); window.got = []")
	// The key is told before the page says it ran (one frame's messages come in order): read once it has come
	inPage(b, "frame", "post", "['/']")
	b.until("the key told", "window.got.includes('/')")
	out = append(out, b.eval("window.starred + ' ' + document.getElementById('find').open"))
	// Another iframe, framed by the frame but not its own: nothing is taken from the page, and nothing it sends is.
	b.eval("window.got = []; window.seen = []; window.other = document.createElement('iframe'); other.id = 'other'; other.onload = () => { window.otherUp = true; }; other.src = '/a/d2/?raw'; other.style = 'position: fixed; inset: 0; width: 300px; height: 200px; z-index: 9'; document.body.append(other)")
	b.until("the other loaded", "window.otherUp")
	focusIn(b, "other")
	b.eval("window.seen = []")
	realKey(b, "/", 0)
	b.until("the other told", "window.seen.length > 0")
	inPage(b, "other", "post", "['/']")
	inPage(b, "other", "open", "0")
	// A page cannot take the keys for itself, the frame not having said so.
	inPage(b, "other", "self", "0")
	realKey(b, "/", 0)
	b.until("the other told again", "window.seen.length > 1")
	out = append(out, b.eval("window.seen.join(',') + ' ' + document.getElementById('find').open + ' ' + location.pathname"))
	b.eval("other.remove()")
	// The iframe gone out to another site: nothing it says is heard, the key (/, which would open the search modal) or the way to move.
	open("/a/d1/", "frame")
	b.eval("window.holdKeys = false")
	inPage(b, "frame", "go", js(ext.URL))
	b.until("the other site", "window.there")
	out = append(out, b.eval("document.getElementById('find').open + ' ' + location.pathname"))
	// Beside the list: j moves on through the rows, the focus in the page; the bar is what goes, f not at all.
	b.size(1440, 900, false)
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=d1] [data-testid=title]')")
	b.eval("document.querySelector('#list li[data-slug=d3] [data-testid=title]').click()")
	b.until("the page beside to be told the keys are taken", "window.ready")
	focusIn(b, "read-frame")
	// h has nothing to go back to here: nothing happens, and the focus stays in the page.
	b.eval("window.got = []; window.seen = []")
	realKey(b, "h", 0)
	b.until("h handed over", "window.got.at(-1) === 'h'")
	out = append(out, b.eval("document.activeElement.id + ' ' + location.pathname"))
	b.eval("window.got = []")
	realKey(b, "f", 0)
	realKey(b, "j", 0)
	b.until("the next row", "document.getElementById('read-frame').dataset.slug === 'd2'")
	out = append(out, b.eval("window.got.join(',')"))
	check(t, "page keys", out, strings.Join([]string{
		"j.", "k.", "/.", "Ctrl+K.", "Ctrl+K.",
		"j.", "j.", "j.",
		".", ".", ".", ".", ".",
		".", ".",
		".", ".", ".", ".", ".", "?.", "h.",
		"0 Escape:false",
		"/",
		"Ctrl+K j:false,h:false,k:true", "Ctrl+K j:false,h:false,k:true",
		"0 false", "/:false,/:false false /a/d1/", "false /a/d1/", "read-frame /", "j",
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
	b.fill("#q", "al")
	keyDown(b, "key: 'Escape', isComposing: true")
	out = append(out, b.eval("document.getElementById('q').value"))
	b.fill("#q", "")
	b.eval("document.activeElement.blur()")
	// No key of the list goes off.
	b.eval("document.querySelector('#list li[data-slug=a] [data-testid=title]').focus()")
	keyDown(b, "key: 'j', isComposing: true")
	keyDown(b, "key: '?', keyCode: 229")
	out = append(out, b.eval("document.activeElement.textContent + ' ' + document.getElementById('keys').open"))
	// The tags' field makes no tag of a word being composed.
	b.eval("document.querySelector('#list li[data-slug=a] [data-testid=sel]').click(); document.getElementById('bulk-tag').click()")
	b.until("the tags modal", "document.getElementById('tag-dialog').open")
	b.fill("#tag-find", "new")
	keyDown(b, "key: 'Enter', isComposing: true")
	out = append(out, b.eval("document.getElementById('tag-find').value"))
	keyDown(b, "key: 'Enter'")
	out = append(out, b.eval("document.getElementById('tag-find').value"))
	// A page missing: the search stays until its Enter is its own.
	b.navigate("/a/no-such/")
	b.until("the search", "!document.getElementById('search').hidden")
	b.fill("#q", "al")
	keyDown(b, "key: 'Enter', isComposing: true")
	out = append(out, b.eval("location.pathname"))
	check(t, "composing", out, "al|Alpha plan false|new||/a/no-such/")
}

// TestUIFindTrap: a page that keeps sending the key and taking the focus cannot keep the window open, which holds all else inert.
func TestUIFindTrap(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.page(t, "d.yaml", "d", keysPage+`<script>addEventListener('message', (e) => { if (e.data?.run === 'trap') setInterval(() => { window.focus(); document.getElementById('b').focus(); parent.postMessage({ pageKey: 'Ctrl+K' }, home); }, 50); });</script>`), "--slug", "d")
	b.navigate("about:blank")
	b.addScript(framedCatch)
	b.navigate("/a/d/")
	b.until("the page to be told the keys are taken", "window.ready")
	focusIn(b, "frame")
	b.eval("window.ranN = 1; document.getElementById('frame').contentWindow.postMessage({ run: 'trap' }, '*')")
	b.until("the window the page opens", "document.getElementById('find').open")
	if err := chromedp.Do(b.ctx, chromedp.KeyEvent(kb.Escape)); err != nil {
		t.Fatal(err)
	}
	b.until("the window closed", "!document.getElementById('find').open")
	// It stays so, the page going on; j still works, from the frame.
	out := make([]string, 0, 2)
	out = append(out, b.eval("new Promise((go) => setTimeout(() => go(document.getElementById('find').open + ' ' + (document.activeElement.id !== 'frame')), 500))"))
	check(t, "trap", out, "false true")
}

// TestUIBackKey goes from a page open whole to the list it was opened from, picks kept, from the page too, as the back button does;
// not typing in a field, and it is nothing on the list.
func TestUIBackKey(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.page(t, "d.yaml", "d", keysPage), "--slug", "d")
	b.navigate("about:blank")
	b.addScript(framedCatch)
	b.size(1440, 900, false)
	const whole = "document.getElementById('frame-title').textContent"
	out := make([]string, 0, 8)
	// On the list, nothing: not the URL, not a window.
	b.navigate("/?kind=page")
	keyDown(b, "key: 'h'")
	out = append(out, b.eval("location.pathname + location.search"))
	// Whole, from the list: back to it with its picks; in a field, the field's.
	b.until("the row", "document.querySelector('#list li[data-slug=d] a[data-testid=title]')")
	b.eval("document.querySelector('#list li[data-slug=d] a[data-testid=title]').focus()")
	keyDown(b, "key: 'Enter'")
	b.until("the page whole", "location.pathname === '/a/d/'")
	b.until("the bar", whole)
	out = append(out, b.eval("document.getElementById('back').getAttribute('href')"))
	keyDown(b, "key: 'h'")
	b.until("the list", "location.pathname === '/'")
	out = append(out, b.eval("location.search"))
	// With the focus in the page, the same.
	b.until("the row", "document.querySelector('#list li[data-slug=d] a[data-testid=title]')")
	b.eval("document.querySelector('#list li[data-slug=d] a[data-testid=title]').focus()")
	keyDown(b, "key: 'Enter'")
	b.until("the page whole", "location.pathname === '/a/d/'")
	b.until("the page to be told the keys are taken", "window.ready")
	focusIn(b, "frame")
	inPage(b, "frame", "focus", "'t'")
	realKey(b, "h", 0)
	inPage(b, "frame", "rest", "0")
	out = append(out, b.eval("location.pathname"))
	realKey(b, "h", 0)
	b.until("the list", "location.pathname === '/'")
	out = append(out, b.eval("location.search"))
	// A page missing goes back too; the row in the window of the keys is of a page whole alone.
	b.navigate("/a/no-such/")
	b.until("the search", "!document.getElementById('search').hidden")
	out = append(out, b.eval("[...document.querySelectorAll('#keys-frame kbd')].some((k) => k.textContent === 'h') + ' ' + [...document.querySelectorAll('#keys-list kbd')].some((k) => k.textContent === 'h')"))
	keyDown(b, "key: 'h'")
	b.until("the list", "location.pathname === '/'")
	check(t, "back", out, strings.Join([]string{"/?kind=page", "/?kind=page", "?kind=page", "/a/d/", "?kind=page", "true false"}, "|"))
}

// TestUINarrowFilter leaves the button to the filters its icon and number on a phone, its words read out.
func TestUINarrowFilter(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.stub(findList)
	const drawn = `(() => { const name = document.getElementById('narrow-filter-name'), n = document.getElementById('narrow-filter-n');
		return [name.textContent, name.getBoundingClientRect().width > 1, n.getBoundingClientRect().width > 0 ? n.textContent : '-', n.getAttribute('aria-hidden')].join(' '); })()`
	out := make([]string, 0, 3)
	for _, w := range []int64{500, 360} {
		b.size(w, 700, false)
		b.navigate("/?project=p&kind=page")
		out = append(out, b.eval(drawn))
	}
	b.navigate("/")
	out = append(out, b.eval(drawn))
	check(t, "narrow filter", out, "Filter 2 true - true|Filter 2 false 2 true|Filter false - true")
}

// TestUIModalShape gives the search modal the shape of the modal that adds, in the list, with the list folded and on a whole page,
// its field still as the rows go, its buttons on the left of the foot, and their menu up within it; the filters scroll inside.
func TestUIModalShape(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.stub(findList)
	// The left and right of the modal, its field and foot, and the heights of the field and the button that closes. The search's foot is
	// the find-foot.
	const shape = `(() => { const d = document.querySelector('dialog[open]'), r = (e) => e.getBoundingClientRect(), foot = d.querySelector(':is([data-testid=dialog-actions], [data-testid=find-foot])');
		const q = r(d.querySelector('input[data-testid=text]')), f = r(foot), m = r(d);
		return [m.left, m.right, q.left, q.right, q.height, f.left, f.right, r(foot.querySelector('[data-close]')).height].map(Math.round).join(' '); })()`
	const where = `(() => { const m = document.getElementById('find').getBoundingClientRect(); return Math.abs(m.top - (innerHeight - m.bottom)) <= 1 ? 'middle' : Math.round(m.top); })()`
	// The top of the field and the height of the modal, which the rows do not move.
	const still = `(() => { const q = document.getElementById('find-q').getBoundingClientRect(), m = document.getElementById('find').getBoundingClientRect(); return Math.round(q.top - m.top) + ' ' + Math.round(m.height); })()`
	out := make([]string, 0, 32)
	for _, w := range []int64{1440, 400} {
		b.size(w, 800, false)
		b.navigate("/")
		b.until("the rows", "document.querySelector('#list [data-testid=title]')")
		b.eval("document.getElementById('add-link').click()")
		add := b.settled(shape)
		b.pressKey(kb.Escape)
		b.until("the add modal closed", "!document.getElementById('add-dialog').open")
		pressFindOpen(b)
		find := b.settled(shape)
		out = append(out, fmt.Sprint(add == find), b.settled(where))
		first := b.eval(still)
		b.fill("#find-q", "plan")
		out = append(out, fmt.Sprint(b.eval(still) == first))
		// The rows as wide as the field; of the foot, the buttons left and the close right; a row's mark not taken for the head's line.
		out = append(out, b.eval(`(() => { const r = (e) => e.getBoundingClientRect(), f = r(document.querySelector('#find [data-testid=find-foot]'));
			return [Math.abs(r(document.querySelector('#find-list li')).right - r(document.getElementById('find-q')).right) < 1,
				Math.abs(r(document.querySelector('#find-actions > :not([hidden])')).left - f.left) < 12,
				Math.abs(r(document.querySelector('#find [data-close]')).right - f.right) < 1,
				r(document.querySelector('#find [data-testid=k]')).width > 0,
				getComputedStyle(document.querySelector('#find-list [data-testid=lead]')).marginTop].join(' '); })()`))
		b.fill("#find-q", "nothing like it")
		out = append(out, fmt.Sprint(b.eval(still) == first))
		out = append(out, b.eval(`[document.getElementById('find-list').hidden, document.getElementById('find-acts').hidden, getComputedStyle(document.querySelector('#find [data-testid=k]')).visibility, document.getElementById('find-cut').textContent].join(' ')`))
		b.pressKey(kb.Escape)
		b.until("the search closed", "!document.getElementById('find').open")
	}
	// Folded and whole, the same.
	b.size(1440, 800, false)
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list [data-testid=title]')")
	pressFindOpen(b)
	want := b.settled(shape)
	b.pressKey(kb.Escape)
	b.until("the search closed", "!document.getElementById('find').open")
	b.eval("document.getElementById('list-hide').click()")
	b.until("the list folded", "!document.getElementById('list-show').hidden")
	pressFindOpen(b)
	out = append(out, fmt.Sprint(b.settled(shape) == want))
	b.pressKey(kb.Escape)
	b.until("the search closed", "!document.getElementById('find').open")
	b.eval("document.getElementById('list-show').click()")
	b.until("the list open", "document.getElementById('list-show').hidden")
	b.navigate("/a/a/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	pressFindOpen(b)
	out = append(out, fmt.Sprint(b.settled(shape) == want))
	b.pressKey(kb.Escape)
	b.until("the search closed", "!document.getElementById('find').open")
	// The menu goes up from the foot, within the modal and the screen, and the rows stay as wide as the field with their scrollbar on the lower.
	const menu = `(() => { const r = (e) => e.getBoundingClientRect(), m = r(document.querySelector('#find-actions [data-testid=menu]')), d = r(document.getElementById('find')), o = r(document.querySelector('#find-actions summary'));
		return [m.bottom <= o.top, m.top >= d.top, m.top >= 0, m.left >= d.left, m.right <= d.right, Math.abs(r(document.querySelector('#find-list li')).right - r(document.getElementById('find-q')).right) < 1].join(' '); })()`
	for _, h := range []int64{620, 400} {
		b.size(1000, h, false)
		b.navigate("/")
		b.until("the rows", "document.querySelector('#list [data-testid=title]')")
		pressFindOpen(b)
		b.eval("document.querySelector('#find-actions summary').click()")
		out = append(out, b.settled(menu))
	}
	check(t, "search modal", out, strings.Join([]string{
		"true", "middle", "true", "true true true true 0px", "true", "true true hidden 0 found",
		"true", "16", "true", "true true true false 0px", "true", "true true hidden 0 found",
		"true", "true",
		"true true true true true true", "true true true true true true",
	}, "|"))

	// The filters on a low screen: the rows scroll, the foot stays, the focus on the first row.
	b.size(400, 420, false)
	b.navigate("/?project=p")
	b.until("the rows", "document.querySelector('#narrow-filter')")
	b.eval("document.getElementById('narrow-filter').click()")
	check(t, "filters", []string{b.settled(`(() => { const d = document.getElementById('filter-dialog'), m = document.getElementById('shelf-more'), c = d.querySelector('[data-close]').getBoundingClientRect();
		m.style.minHeight = ''; m.append(Object.assign(document.createElement('div'), { id: 'tall', style: 'height: 900px' }));
		return [d.scrollHeight === d.clientHeight, m.scrollHeight > m.clientHeight, c.bottom <= innerHeight, document.activeElement.tagName, !!d.querySelector('[data-testid=modal-head] svg')].join(' '); })()`)},
		"true true true BUTTON true")
	b.eval("document.getElementById('tall').remove()")
	b.pressKey(kb.Escape)
	b.until("the filters closed", "!document.getElementById('filter-dialog').open")

	// The shortcuts say in their head the note of where they are.
	b.size(1440, 800, false)
	const note = `['list', 'frame'].map((w) => document.getElementById('keys-note-' + w).getBoundingClientRect().height > 0).join(' ')`
	b.navigate("/")
	keyDown(b, "key: '?', shiftKey: true")
	b.until("the help", "document.getElementById('keys').open")
	list := b.eval(note)
	b.pressKey(kb.Escape)
	b.until("the help closed", "!document.getElementById('keys').open")
	b.navigate("/a/a/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	keyDown(b, "key: '?', shiftKey: true")
	b.until("the help", "document.getElementById('keys').open")
	check(t, "notes", []string{list, b.eval(note)}, "true false|false true")
}

// TestUIFindPress leaves the focus in the field after a press on what does nothing, so that the arrows still move over the rows;
// a drag in the field selects, the scrollbar scrolls, and what can be pressed takes its press.
func TestUIFindPress(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.stub("Array.from({ length: 40 }, (_, i) => ({ slug: 's' + i, title: 'T' + i, description: '', version: 1, createdAt: at(1), updatedAt: at(9) }))")
	b.size(1000, 700, false)
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list [data-testid=title]')")
	pressFindOpen(b)
	b.still()
	mouse := func(kind input.DispatchMouseEventType, x, y float64) {
		t.Helper()
		if kind == input.DispatchMouseEventTypeMousePressed {
			b.painted()
		}
		p := input.DispatchMouseEventParams{Type: kind, X: x, Y: y, Button: input.MouseButtonLeft, Buttons: 1, ClickCount: 1}
		if _, err := chromedp.Call(b.ctx, input.DispatchMouseEvent, p); err != nil {
			t.Fatal(err)
		}
	}
	drag := func(x0, y0, x1, y1 float64) {
		t.Helper()
		mouse(input.DispatchMouseEventTypeMousePressed, x0, y0)
		mouse(input.DispatchMouseEventTypeMouseMoved, x1, y1)
		mouse(input.DispatchMouseEventTypeMouseReleased, x1, y1)
	}
	at := func(expr string) (float64, float64) {
		t.Helper()
		var x, y float64
		if _, err := fmt.Sscan(b.eval(expr), &x, &y); err != nil {
			t.Fatal(err)
		}
		return x, y
	}
	down := func() {
		t.Helper()
		if err := chromedp.Do(b.ctx, chromedp.KeyEvent(kb.ArrowDown)); err != nil {
			t.Fatal(err)
		}
	}
	const reached = "document.activeElement.id + ' ' + document.getElementById('find-q').getAttribute('aria-activedescendant')"
	out := make([]string, 0, 12)
	// The heading, the line under it, the modal's edge, and the line over the foot: none of them takes the focus.
	for i, spot := range []string{"#find h2", "#find-lead", "#find", "#find [data-testid=find-foot]"} {
		x, y := at(`(() => { const r = document.querySelector(` + js(spot) + `).getBoundingClientRect(); return ` + map[bool]string{true: "(r.left + 8) + ' ' + (r.top + r.height / 2)", false: "(r.left + r.width / 2) + ' ' + (r.top + 8)"}[i == 2] + `; })()`)
		b.click(x, y)
		down()
		out = append(out, b.eval(reached))
	}
	// The field goes on taking what is typed.
	if err := chromedp.Do(b.ctx, chromedp.KeyEvent("T")); err != nil {
		t.Fatal(err)
	}
	out = append(out, b.eval("document.getElementById('find-q').value"))
	// A drag over its words selects them.
	x0, y0, x1 := func() (float64, float64, float64) {
		x, y := at("(() => { const r = document.getElementById('find-q').getBoundingClientRect(); return r.left + ' ' + (r.top + r.height / 2); })()")
		return x + 40, y, x + 4
	}()
	drag(x0, y0, x1, y0)
	out = append(out, b.eval("(() => { const q = document.getElementById('find-q'); return q.selectionEnd - q.selectionStart; })()"))
	b.fill("#find-q", "")
	// The scrollbar's thumb, dragged, moves the rows, and the arrows go on from where they were.
	x, y := at("(() => { const r = document.getElementById('find-list').getBoundingClientRect(); return (r.right - 5) + ' ' + (r.top + 10); })()")
	mouse(input.DispatchMouseEventTypeMousePressed, x, y)
	for i := 1.0; i <= 4; i++ {
		mouse(input.DispatchMouseEventTypeMouseMoved, x, y+20*i)
	}
	mouse(input.DispatchMouseEventTypeMouseReleased, x, y+80)
	b.until("the rows moved", "document.getElementById('find-list').scrollTop > 0")
	out = append(out, b.eval("document.activeElement.id"))
	// Tab goes from the field to the buttons of the foot, not to the box of rows that scrolls.
	b.eval("document.getElementById('find-q').focus()")
	if err := chromedp.Do(b.ctx, chromedp.KeyEvent("\t")); err != nil {
		t.Fatal(err)
	}
	out = append(out, b.eval("document.activeElement.matches('#find-actions > [data-testid=download]')"))
	b.eval("document.getElementById('find-q').focus()")
	// The words of an error can be selected, and a link keeps its own press.
	out = append(out, b.eval("['#error', '#find-list a'].every((e) => document.querySelector(e).dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true })))"))
	// What can be pressed takes the press: the star and the menu their focus, and a row opens.
	x, y = at("(() => { const r = document.querySelector('#find-actions [data-testid=star]').getBoundingClientRect(); return (r.left + r.width / 2) + ' ' + (r.top + r.height / 2); })()")
	b.click(x, y)
	out = append(out, b.eval("document.activeElement.matches('#find-actions [data-testid=star]')"))
	x, y = at("(() => { const r = document.querySelector('#find-actions summary').getBoundingClientRect(); return (r.left + r.width / 2) + ' ' + (r.top + r.height / 2); })()")
	b.click(x, y)
	b.until("the menu open", "document.querySelector('#find-actions [data-testid=more]').open")
	out = append(out, b.eval("document.activeElement.matches('#find-actions summary') && document.querySelector('#find-actions [data-testid=more]').open"))
	b.pressKey(kb.Escape)
	b.until("the menu closed", "!document.querySelector('#find-actions [data-testid=more]').open")
	x, y = at("(() => { const a = document.querySelector('#find-list a'); a.scrollIntoView(); const r = a.getBoundingClientRect(); return (r.left + 4) + ' ' + (r.top + r.height / 2); })()")
	b.click(x, y)
	b.until("the page", "location.pathname === '/a/s0/'")
	check(t, "press", out, strings.Join([]string{
		"find-q find-1", "find-q find-2", "find-q find-3", "find-q find-4", "T", "1", "find-q", "true", "true", "true", "true",
	}, "|"))
}

// TestUIFindRowGone: a row archived out of the rows shown takes the focus off the foot's buttons, which would go on to act on the row
// come to its place, one more archived by each press.
func TestUIFindRowGone(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	for i := range 51 {
		slug := fmt.Sprintf("p%d", i)
		s.floe(t, "publish", s.page(t, "many/"+slug+".yaml", slug, "<p>p</p>"), "--slug", slug)
	}
	b.navigate("/")
	b.until("the rows", "document.querySelectorAll('#list li[data-slug]').length === 51")
	pressFindOpen(b)
	keyDown(b, "key: 'ArrowUp'")
	b.until("the last row", "document.getElementById('find-actions').dataset.slug && document.querySelector('#find-actions > [data-testid=archive][title]')")
	const at = "document.querySelector('#find-actions').dataset.slug"
	slug := b.eval(at)
	b.eval("document.querySelector('#find-actions > [data-testid=archive]').focus(); document.querySelector('#find-actions > [data-testid=archive]').click()")
	b.until("the row gone", at+" !== "+js(slug))
	check(t, "gone", []string{b.eval("document.activeElement.id"), b.eval("fetch('/api/pages').then((r) => r.json()).then((all) => all.filter((a) => a.archived).map((a) => a.slug).join())")}, "find-q|"+slug)
}

// TestUIFindArrows moves over the rows by the arrows from the foot's buttons too, which keep the focus and say the row they act on,
// but not with a menu open; Enter stays the button's.
func TestUIFindArrows(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.stub(findList)
	b.size(1000, 700, false)
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list [data-testid=title]')")
	pressFindOpen(b)
	b.still()
	key := func(k string) {
		t.Helper()
		if err := chromedp.Do(b.ctx, chromedp.KeyEvent(k)); err != nil {
			t.Fatal(err)
		}
	}
	const q = "document.getElementById('find-q')"
	// The row reached, what has the focus (its id, test id or tag), and the name of the star.
	const state = `[` + q + `.getAttribute('aria-activedescendant'), document.activeElement.id || document.activeElement.dataset.testid || document.activeElement.tagName,
		document.querySelector('#find-actions [data-testid=star]').getAttribute('aria-label')].join(' / ')`
	press := func(k string) string {
		t.Helper()
		key(k)
		return b.eval(state)
	}
	out := make([]string, 0, 16)
	// By Tab to the star: down, up, and round past the first.
	key("\t")
	out = append(out, press(kb.ArrowDown), press(kb.ArrowUp), press(kb.ArrowUp))
	b.eval(q + ".focus()")
	key(kb.ArrowDown)
	// After a press on the star, by the mouse.
	x := b.eval("(() => { const r = document.querySelector('#find-actions [data-testid=star]').getBoundingClientRect(); return (r.left + r.width / 2) + ' ' + (r.top + r.height / 2); })()")
	var px, py float64
	if _, err := fmt.Sscan(x, &px, &py); err != nil {
		t.Fatal(err)
	}
	b.click(px, py)
	b.until("the star's toast", "document.querySelector('#toasts').textContent")
	out = append(out, press(kb.ArrowDown))
	// Enter is the star's still, and no row opens.
	b.eval("window.starred = 0; document.querySelector('#find-actions [data-testid=star]').addEventListener('click', () => { window.starred++; })")
	key(kb.Enter)
	out = append(out, b.eval("window.starred + ' ' + location.pathname"))
	// The button that opens a page keeps the focus; on a folder, which has none to open, the focus goes to the field. The rows run to the folder as z, f.
	b.eval("document.querySelector('#find-actions > [data-testid=open]').focus()")
	out = append(out, press(kb.ArrowDown), b.eval("document.querySelector('#find-actions > [data-testid=open]').getAttribute('aria-label')"))
	out = append(out, press(kb.ArrowDown))
	// From the close button too.
	b.eval("document.querySelector('#find [data-close]').focus()")
	out = append(out, press(kb.ArrowUp))
	// From the menu's button the focus goes to the field, as the menu is another row's.
	b.eval("document.querySelector('#find-actions summary').focus()")
	out = append(out, press(kb.ArrowUp))
	// With the menu open the rows stay, from its button and from within it.
	b.eval("document.querySelector('#find-actions summary').focus(); document.querySelector('#find-actions summary').click()")
	const menu = "document.querySelector('#find-actions [data-testid=more]').open"
	out = append(out, press(kb.ArrowDown), b.eval(menu))
	b.eval("document.querySelector('#find-actions [data-testid=menu] [data-testid=edit]').focus()")
	out = append(out, press(kb.ArrowDown), b.eval(menu))
	// Pressed by the keys, Enter and the space bar, the star keeps the focus through the rows drawn anew, and the arrows go on.
	// The menu shut by its own button, as a press elsewhere would.
	const shut = "(() => { const m = document.querySelector('#find-actions [data-testid=more]'); if (m.open) m.querySelector('summary').click(); })()"
	for i, k := range []string{kb.Enter, " "} {
		was := b.eval("window.starred")
		b.eval(shut + "; document.querySelector('#find-actions [data-testid=star]').focus()")
		key(k)
		b.until("the star pressed", "window.starred !== "+was)
		b.still()
		// The rows drawn anew: another row's title changed.
		title := fmt.Sprintf("Zed plan %d", i)
		b.eval("window.list = JSON.stringify(JSON.parse(window.list).map((a) => a.slug === 'z' ? { ...a, title: " + js(title) + " } : a))")
		b.reread()
		b.until("the rows drawn anew", "[...document.querySelectorAll('#find-list [data-testid=title]')].some((a) => a.textContent === "+js(title)+")")
		out = append(out, press(kb.ArrowDown))
	}
	// The rows gone with the focus on the star, it goes to the field.
	b.eval(shut + "; document.querySelector('#find-actions [data-testid=star]').focus(); window.list = '[]'")
	b.reread()
	b.until("the rows gone", "document.querySelectorAll('#find-list li').length === 0")
	out = append(out, b.eval("document.activeElement.id"))
	// The bar of a whole page says the same of its button.
	b.navigate("/a/a/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	out = append(out, b.eval("document.querySelector('#frame-actions > [data-testid=open]').getAttribute('aria-label')"))
	check(t, "arrows", out, strings.Join([]string{
		"find-1 / download / Add Beta to favorites", "find-0 / download / Add Alpha plan to favorites", "find-4 / download / Add Zed plan to favorites",
		"find-1 / star / Add Beta to favorites", "1 /",
		"find-2 / open / Add Gamma plan to favorites", "Open Gamma plan alone in a new tab", "find-3 / find-q / Add Folder to favorites",
		"find-2 / BUTTON / Add Gamma plan to favorites", "find-1 / find-q / Add Beta to favorites",
		"find-1 / SUMMARY / Add Beta to favorites", "true", "find-1 / edit / Add Beta to favorites", "true",
		"find-2 / star / Add Gamma plan to favorites", "find-3 / star / Add Folder to favorites",
		"find-q", "Open Alpha plan alone in a new tab",
	}, "|"))
}

// TestUIFindOrder leaves j and k to the list last on where the search modal opens a page, not the order an earlier list kept.
func TestUIFindOrder(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.stub("[1, 2, 3, 4, 5].map((n) => ({ slug: 'p' + n, title: 'P' + n, description: '', version: 1, tags: n % 2 ? ['t'] : [], createdAt: at(1), updatedAt: at(9 - n) }))")
	const row = "[...document.querySelectorAll('#find-list li')].find((li) => li.querySelector('a').textContent === 'P3')"
	// The order of all kept, as a list of all keeps it, under the key the page keeps it by.
	const keptAll = `sessionStorage.setItem('pages.order', '{"order":["p1","p2","p3","p4","p5"],"shown":["p1","p2","p3","p4","p5"],"back":"/"}')`
	open := map[string]func(){
		"enter": func() {
			for range 5 {
				if b.eval(row+".getAttribute('aria-selected')") == "true" {
					break
				}
				keyDown(b, "key: 'ArrowDown'")
			}
			if err := chromedp.Do(b.ctx, chromedp.KeyEvent(kb.Enter)); err != nil {
				t.Fatal(err)
			}
		},
		"press": func() { b.moving(row + ".querySelector('a').click()") },
	}
	out := make([]string, 0, 5)
	for _, how := range []string{"enter", "press"} {
		// On the list of the tag, whose next after P3 is P5, with the order of all kept, whose next is P4.
		b.navigate("/?tag=t")
		b.until("the rows", "document.querySelector('#list [data-testid=title]')")
		b.eval(keptAll)
		pressFindOpen(b)
		// A press beside the rows opens nothing, and the order stays.
		out = append(out, b.eval("document.getElementById('find-list').click(); sessionStorage.getItem('pages.order') !== null"))
		open[how]()
		b.until("the page", "location.pathname === '/a/p3/'")
		b.until("the bar", "document.getElementById('frame-title').textContent")
		keyDown(b, "key: 'j'")
		b.until("the next page", "location.pathname !== '/a/p3/'")
		out = append(out, b.eval("location.pathname"))
	}
	// Back on a list the order of the one before is over: a page opened by its URL goes through the list last on.
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list [data-testid=title]')")
	b.eval(keptAll)
	b.navigate("/?tag=t")
	b.until("the rows", "document.querySelector('#list [data-testid=title]')")
	b.navigate("/a/p3/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	keyDown(b, "key: 'j'")
	b.until("the next page", "location.pathname !== '/a/p3/'")
	out = append(out, b.eval("location.pathname"))
	check(t, "order", out, "true|/a/p5/|true|/a/p5/|/a/p5/")
}

// TestUIReadLoads reads the list again as soon as a read is told, so that the dot and the counts do not wait for the next look.
func TestUIReadLoads(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.page(t, "read/pv.yaml", "pv", "<p>pv</p>"), "--slug", "pv")
	b.addScript(`(() => { const f = window.fetch; window.told = 0; window.next = 0;
		window.fetch = (u, o) => { const read = String(u).endsWith('/read');
			if (window.told && !window.next && String(u) === '/api/pages') window.next = performance.now();
			return f(u, o).then((r) => { if (read) window.told = performance.now(); return r; }); }; })()`)
	b.navigate("/a/pv/")
	b.until("the read told and the list read", "window.told > 0 && window.next > 0")
	check(t, "at once", []string{b.eval("window.next - window.told < 500")}, "true")
}

// TestUIFindCloseFocus: the search shut by its keys from a button of its foot, which goes as it shuts, gives the focus back to what had it
// before.
func TestUIFindCloseFocus(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.page(t, "a.yaml", "T a", ""), "--slug", "a")
	b.navigate("/")
	const title = "document.querySelector('#list li[data-slug=a] [data-testid=title]')"
	b.until("the row", title)
	for _, w := range []struct {
		name string
		shut func()
	}{
		{"Escape", func() { b.pressKey(kb.Escape) }},
		{"Ctrl+K", func() { b.pressKey("k", kb.ModifierCtrl) }},
	} {
		b.eval(title + ".focus()")
		b.pressKey("k", kb.ModifierCtrl)
		b.until(w.name+": the search", "document.getElementById('find').open && document.querySelector('#find-actions [data-testid=star]')")
		b.eval("document.querySelector('#find-actions [data-testid=star]').focus()")
		w.shut()
		b.until(w.name+": the search shut", "!document.getElementById('find').open")
		b.until(w.name+": its foot gone", "document.getElementById('find-actions') === null")
		if got := b.eval("document.activeElement === " + title); got != "true" {
			t.Errorf("%s from the foot's star: the focus is on %s, want the row's title", w.name, b.eval("document.activeElement.outerHTML.slice(0, 120)"))
		}
	}
}

// TestUIRowSelect starts a selection from a row's menu, with no keys and on a touch screen, where the boxes then show for the rest;
// the menus of the pages' bars and the search modal have no such item.
func TestUIRowSelect(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.stub(findList)
	touch := func(on bool) {
		if _, err := chromedp.Call(b.ctx, emulation.SetTouchEmulationEnabled, emulation.SetTouchEmulationEnabledParams{Enabled: on, MaxTouchPoints: 1}); err != nil {
			t.Fatal(err)
		}
	}
	touch(true)
	defer touch(false)
	b.size(400, 700, false)
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=f]')")
	press := func(selector string) {
		t.Helper()
		var x, y float64
		if _, err := fmt.Sscan(b.eval("(() => { const e = document.querySelector("+js(selector)+"); e.scrollIntoView({ block: 'center' }); const r = e.getBoundingClientRect(); return (r.left + r.width / 2) + ' ' + (r.top + r.height / 2); })()"), &x, &y); err != nil {
			t.Fatal(err)
		}
		b.click(x, y)
	}
	state := "[...document.querySelectorAll('#list li[data-slug]')].filter((li) => li.querySelector('[data-testid=sel]')?.checked).map((li) => li.dataset.slug).sort().join('') + ' ' + (document.getElementById('list').dataset.selecting === 'true') + ' ' + !document.getElementById('bulk').hidden + ' ' + getComputedStyle(document.querySelector('#list li[data-slug=c] [data-testid=pick]')).display"
	out := make([]string, 0, 6)
	out = append(out, b.eval(state))
	press("#list li[data-slug=a] summary")
	press("#list li[data-slug=a] [data-testid=menu] [data-testid=select]")
	out = append(out, b.eval(state), b.eval("document.querySelector('#list li[data-slug=a] [data-testid=more]').open"))
	// A folder's row too; pressed again, a row stays selected.
	press("#list li[data-slug=f] summary")
	press("#list li[data-slug=f] [data-testid=menu] [data-testid=select]")
	press("#list li[data-slug=f] summary")
	press("#list li[data-slug=f] [data-testid=menu] [data-testid=select]")
	out = append(out, b.eval(state))
	// Named for a screen reader; none in the search modal or the bar of a page.
	out = append(out, b.eval(inMenuItem("#list li[data-slug=a]", "select", "i.getAttribute('aria-label')")))
	b.eval("document.getElementById('bulk-clear').click()")
	pressFindOpen(b)
	out = append(out, b.eval(inMenuItem("#find-actions", "select", "i.hidden")))
	b.pressKey(kb.Escape)
	b.until("the search closed", "!document.getElementById('find').open")
	touch(false)
	b.navigate("/a/a/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	out = append(out, b.eval(inMenuItem("#frame-actions", "select", "i.hidden")))
	check(t, "select", out, strings.Join([]string{
		" false false none", "a true true grid", "false", "af true true grid", "Select Alpha plan", "true", "true",
	}, "|"))
}
