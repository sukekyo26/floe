package browsertest_test

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/servetest"
)

// inEveryLanguage runs scenario once for each language with a catalog, with that language's messages, so that a
// scenario that reads the page's wording checks every catalog's.
func inEveryLanguage(t *testing.T, scenario func(t *testing.T, lang i18n.Lang, msg func(string, ...any) string)) {
	t.Helper()

	for _, lang := range i18n.Langs() {
		t.Run(string(lang), func(t *testing.T) {
			scenario(t, lang, i18n.New(lang).Msg)
		})
	}
}

// extrasList is the list the stub answers: pages, a favourite, links (one whose file is gone) and archived ones,
// newest first as the server lists them.
const extrasList = `[
	{ slug: 'b', title: 'Beta', description: '', createdAt: at(3), updatedAt: at(9) },
	{ slug: 'a', title: 'Alpha', description: 'old', createdAt: at(1), updatedAt: at(8) },
	{ slug: 'c', title: 'Gamma', description: '', createdAt: at(5), updatedAt: at(7), favorite: true },
	{ slug: 'z', title: 'Zeta', description: '', project: 'p', createdAt: at(2), updatedAt: at(6), favorite: true, archived: true },
	{ slug: 'y', title: 'Ypsilon', description: '', createdAt: at(2), updatedAt: at(5), archived: true },
	{ slug: 'l', title: 'Linked', description: '', path: '/tmp/x/l.html', createdAt: at(1), updatedAt: at(1) },
	{ slug: 'm', title: 'Missing', description: '', path: '/tmp/x/m.html', missing: true, createdAt: at(1), updatedAt: at(1) },
	{ slug: 'k', title: 'Kept', description: '', path: '/tmp/x/k.html', createdAt: at(1), updatedAt: at(1), archived: true },
]`

// managementPage tells whether the document is a management page (/ and /a/<slug>/, not a page shown in its frame), which
// stub fakes. It is ui.html's own test of a link the frame leaves for (leaveFrame): change the two together.
const managementPage = "/^\\/(a\\/[^/]+\\/?)?$/.test(location.pathname) && !new URLSearchParams(location.search).has('raw')"

// fake is the script that answers list, a JavaScript array at(day) may build, for the management page's list, counting
// its reads in window.gets, and records the writes in window.calls. The list is made first, so that a mistake in it
// fails wherever the script runs; it fakes nothing but a management page.
func fake(list string) string {
	return `(() => {
	const at = (d) => ` + "`2026-01-0${d}T00:00:00.000Z`" + `;
	const made = JSON.stringify(` + list + `);
	if (!(` + managementPage + `)) return;
	window.faked = true;
	window.gets ||= 0;
	window.calls = [];
	window.list = made;
	window.fetch = (url, init = {}) => {
		// A folder's directory: one directory, one page and more left out, so that each kind of its rows shows
		if (!init.method && String(url).includes('/pages')) {
			return Promise.resolve(new Response('{"dirs":["guide"],"pages":[{"file":"index.html","updatedAt":"' + at(3) + '"}],"more":1200}'));
		}
		if (!init.method && String(url).includes('/api/tasks')) return Promise.resolve(new Response('{}'));
		if (!init.method) {
			window.gets++;
			return Promise.resolve(new Response(window.list));
		}
		if (window.failWrites) return Promise.reject(new TypeError('Failed to fetch'));
		calls.push([init.method, url, init.body].filter(Boolean).join(' '));
		if (init.method === 'POST') return Promise.resolve(new Response('{"slug":"n"}', { status: 201 }));
		return Promise.resolve(new Response(null, { status: 204 }));
	};
	navigator.clipboard.writeText = (text) => {
		calls.push(` + "`copy ${text}`" + `);
		return Promise.resolve();
	};
})()`
}

// stub answers list (see fake) in every management page this tab opens from now on, by a script run before the page's
// own, so that each of its loads (the first, each poll, one after a move to another page) is answered so; navigate
// waits for such a page to draw it. A stub after the first also draws the management page open now with list. The first
// stub, and the first after unstub, comes on a blank tab, before the page it fakes opens: a page open has sent its own
// load already, whose answer would draw over the stub's.
func (b *browser) stub(list string) {
	b.t.Helper()

	script := fake(list)
	if b.faked == "" {
		if at := b.eval("location.href"); at != "about:blank" {
			b.t.Fatalf("the first stub comes on a blank tab, before the page it fakes opens: navigate to about:blank first, not on %s", at)
		}
		b.eval(script)
		b.faked = b.addScript(script)
		return
	}
	// The new script is added before the old one goes, so that a page opened in between takes one of them.
	old := b.faked
	b.faked = b.addScript(script)
	b.removeScript(old)
	b.until("the page to load", "document.readyState === 'complete'")
	if b.eval(managementPage) != "true" {
		return // no management page open: the next one opened takes list
	}
	if b.eval("window.faked === true") != "true" {
		b.t.Fatal("this page was opened before the first stub: stub on a blank tab before opening it")
	}
	b.eval(script)
	b.loadStubbed()
}

// loadStubbed waits until the management page open, faked, has read the list stubbed and drawn it: its own load may
// still be on the way after the page is loaded.
func (b *browser) loadStubbed() {
	b.t.Helper()

	got := b.eval("(async () => { const n = window.gets; await load(); " +
		"return window.gets > n || 'not read: ' + document.getElementById('error').textContent; })()")
	if got != "true" {
		b.t.Fatalf("the page did not load the list stubbed: %s", got)
	}
}

// unstub gives the pages this tab opens from now on the server's list again.
func (b *browser) unstub() {
	b.t.Helper()

	if b.faked == "" {
		return
	}
	b.removeScript(b.faked)
	b.faked = ""
}

func (b *browser) addScript(script string) page.ScriptIdentifier {
	b.t.Helper()

	added, err := chromedp.Call(b.ctx, page.AddScriptToEvaluateOnNewDocument, page.AddScriptToEvaluateOnNewDocumentParams{Source: script})
	if err != nil {
		b.t.Fatal(err)
	}
	return added.Identifier
}

func (b *browser) removeScript(id page.ScriptIdentifier) {
	b.t.Helper()

	if _, err := chromedp.Call(b.ctx, page.RemoveScriptToEvaluateOnNewDocument, page.RemoveScriptToEvaluateOnNewDocumentParams{Identifier: id}); err != nil {
		b.t.Fatal(err)
	}
}

// manyPages is a list of n pages to stub, newest first, alike but for their slug and title.
func manyPages(n int) string {
	return "Array.from({ length: " + strconv.Itoa(n) + " }, (_, i) => ({ slug: `p${i}`, title: `P${i}`, description: '', createdAt: at(1), updatedAt: at(1) }))"
}

// TestUIExtras filters favourites and the archived, counts each list, moves by keyboard, deletes and archives in bulk,
// edits in a modal, copies a path, adds a link only to the lists it goes on, clears an error once a change succeeds and
// tells an empty list, against a stubbed list.
func TestUIExtras(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	inEveryLanguage(t, uiExtras)
}

func uiExtras(t *testing.T, lang i18n.Lang, msg func(string, ...any) string) {
	t.Helper()

	b := newBrowser(t, servetest.Start(t, nil).Base, lang)
	b.stub(extrasList)
	b.navigate("/")
	const titles = "[...document.querySelectorAll('#list .title')].map((a) => a.textContent).join(',')"
	// What the list counts, and the view in the URL: the kinds, the project chips and whether they show, the views,
	// the head and the titles.
	const counts = "[...document.querySelectorAll('#kinds .n')].map((n) => n.textContent)" +
		".concat([...document.querySelectorAll('#chips .chip')].map((c) => c.textContent), document.getElementById('shelf-projects').hidden, " +
		"[...document.querySelectorAll('.views .n')].map((n) => n.textContent).join(' '), document.getElementById('count').textContent, " + titles + ", " +
		"new URLSearchParams(location.search).get('view')).join(' ')"
	// Which empty states show, and their words.
	const empties = "['empty', 'all-archived', 'empty-new', 'empty-favorites', 'empty-archived', 'nomatch']" +
		".filter((id) => !document.getElementById(id).hidden).map((id) => id + ':' + document.getElementById(id).querySelector('strong').textContent).join(' ')"
	key := func(k string) {
		b.eval("document.body.dispatchEvent(new KeyboardEvent('keydown', { key: '" + k + "', bubbles: true }))")
	}

	out := make([]string, 0, 32)
	out = append(out, b.eval(titles))
	// The shelf's favourites narrow to them (kept in the URL); a row's star sends the change.
	b.eval("document.querySelector('.views [data-view=fav]').click()")
	out = append(out, b.eval(titles)+" "+b.eval("new URLSearchParams(location.search).get('view')"))
	b.eval("document.querySelector('.views [data-view=all]').click()")
	b.eval("document.querySelector('#list li:not(.group) .star').click()")
	out = append(out, b.until("the favourite", "calls.splice(0).join(',')"))
	// The title's tooltip has both the creation and the update.
	out = append(out, b.eval("document.querySelector('#list li:not(.group) .title').title.split('\\n').slice(1).map((l) => l.split(' ')[0]).join(' ')"))
	// ↓ moves through the rows by focusing their titles, x selects the focused row, the check boxes showing from then on alone.
	const boxes = "getComputedStyle(document.querySelector('#list li:not(.group) .pick')).display"
	out = append(out, b.eval(boxes))
	key("ArrowDown")
	key("ArrowDown")
	out = append(out, b.eval("document.activeElement.textContent"))
	key("x")
	out = append(out, b.eval("document.getElementById('bulk-count').textContent"), b.eval(boxes))
	// With Beta selected too by a real mouse click (nothing may sit on top of the checkbox), the first
	// click arms and the second deletes both.
	r := b.rect("document.querySelector('#list li:not(.group) .sel')")
	b.click(r.X+r.Width/2, r.Y+r.Height/2)
	out = append(out, b.until("the mouse click to select",
		"document.querySelector('#list li:not(.group) .sel').checked && document.getElementById('bulk-count').textContent"))
	b.eval("document.getElementById('bulk-delete').click()")
	out = append(out, b.eval("document.getElementById('bulk-delete').textContent"))
	b.eval("document.getElementById('bulk-delete').click()")
	b.until("the selection to clear", "document.getElementById('bulk').hidden")
	out = append(out, b.eval("calls.splice(0).sort().join(',')"), b.eval(boxes))
	// Links alone say they are unregistered.
	b.eval("document.querySelector('#list li[data-slug=l] .sel').click()")
	out = append(out, b.eval("document.getElementById('bulk-delete').textContent"))
	b.eval("document.getElementById('bulk-clear').click()")
	// The pencil opens a modal with the current description; saving sends the new one.
	b.eval("document.querySelector('#list li[data-slug=a] .edit').click()")
	out = append(out, b.eval("[document.getElementById('edit-dialog').open, document.getElementById('edit-desc').value].join(' ')"))
	b.eval("document.getElementById('edit-desc').value = 'new'; document.getElementById('edit').requestSubmit()")
	// Closed, the focus goes back to the row's menu button, as the dialog's close event comes.
	b.until("the edit modal to close", "!document.getElementById('edit-dialog').open && document.activeElement.matches('#list li[data-slug=a] summary')")
	out = append(out, b.eval("calls.splice(0).join(',')"),
		b.eval("document.getElementById('toasts').textContent.includes("+js(msg("ui_saved"))+")"))
	// Only linked files have a copy button for their path.
	out = append(out, b.eval("document.querySelector('#list li[data-slug=a] .copy').hidden"))
	b.eval("document.querySelector('#list li[data-slug=l] .copy').click()")
	out = append(out, b.until("the copy", "calls.join(',')"),
		b.eval("document.getElementById('toasts').textContent.includes("+js(msg("ui_path_copied"))+")"))
	// A link whose file is gone has no href, but ↓ still reaches it.
	b.eval("document.querySelector('#list li[data-slug=l] .title').focus()")
	key("ArrowDown")
	out = append(out, b.eval("document.activeElement.textContent"))
	// The add form sends only the fields filled in, trimmed: a field sent is given, empty or not.
	b.eval(`(() => {
		calls.length = 0;
		document.getElementById('add-open').click();
		document.getElementById('add-path').value = ' /tmp/x/n.html ';
		document.getElementById('add-desc').value = ' a link ';
		document.getElementById('add').requestSubmit();
	})()`)
	out = append(out, b.until("the add to be sent", "calls.splice(0).join(',')"))
	b.until("the add dialog to close", "!document.getElementById('add-dialog').open")

	// The archived are off every count and list but their own, which the switch shows (kept in the URL); a row's
	// menu and the bulk bar send the change, to archive or to give back, and say which; the menu closes.
	archiveWords := "(() => { const a = document.querySelector('#list li:not(.group) .archive'); document.getElementById('bulk-page').click(); " +
		"const words = [a.textContent.trim(), a.getAttribute('aria-label'), document.getElementById('bulk-archive').textContent]; " +
		"document.getElementById('bulk-clear').click(); return words.join(' / '); })()"
	const archiveRow = "document.querySelector('#list li:not(.group) .more summary').click(); document.querySelector('#list li:not(.group) .archive').click()"
	out = append(out, b.eval(counts), b.eval(archiveWords))
	b.eval(archiveRow)
	out = append(out, b.until("the archive", "calls.splice(0).join(',')"), b.eval("document.querySelectorAll('#list .more[open]').length"),
		b.eval("document.getElementById('toasts').textContent.includes("+js(msg("ui_archive_done", "Beta"))+")"))
	b.eval("document.querySelector('.views [data-view=archived]').click()")
	out = append(out, b.eval(counts), b.eval(archiveWords))
	b.eval(archiveRow)
	out = append(out, b.until("the give back", "calls.splice(0).join(',')"))
	// An item of the menu done by keys, the menu closes, focus going back to its button: there still when the change fails.
	b.eval("window.failWrites = true; document.querySelector('#list li:not(.group) .more summary').click(); " +
		"document.querySelector('#list li:not(.group) .archive').focus(); document.querySelector('#list li:not(.group) .archive').click()")
	out = append(out, b.until("the give back to fail", "!document.getElementById('error').hidden && "+
		"String(document.activeElement === document.querySelector('#list li:not(.group) .more summary'))"))
	b.eval("window.failWrites = false")
	b.eval("document.getElementById('bulk-page').click()")
	out = append(out, b.eval("document.getElementById('bulk-archive').textContent"))
	b.eval("document.getElementById('bulk-archive').click()")
	b.until("the selection to clear", "document.getElementById('bulk').hidden")
	out = append(out, b.eval("calls.splice(0).sort().join(',')"))
	// Empty lists: nothing archived; all archived, which leads to them; nothing at all; no new page.
	b.stub(`[{ slug: 'l', title: 'Linked', description: '', path: '/tmp/x/l.html', createdAt: at(1), updatedAt: at(1) }]`)
	// A link is added from the lists it goes on, not from the archived.
	out = append(out, b.eval(empties)+" "+b.eval("document.getElementById('add-open').hidden"))
	b.eval("document.querySelector('.views [data-view=all]').click()")
	out = append(out, b.eval("document.getElementById('add-open').hidden"))
	// A change that succeeds clears the error of one that failed before, a favourite's as an archive's.
	b.eval("window.failWrites = true; document.querySelector('#list li:not(.group) .star').click()")
	out = append(out, b.until("the favourite to fail", "!document.getElementById('error').hidden && document.getElementById('error').textContent"))
	b.eval("window.failWrites = false; document.querySelector('#list li:not(.group) .star').click()")
	out = append(out, b.until("the error to clear", "document.getElementById('error').hidden"))
	b.stub(`[{ slug: 'z', title: 'Zeta', description: '', createdAt: at(2), updatedAt: at(6), archived: true }]`)
	out = append(out, b.eval(empties))
	b.eval("document.getElementById('show-archived').click()")
	out = append(out, b.eval(titles+" + ' ' + new URLSearchParams(location.search).get('view')"))
	b.eval("document.querySelector('.views [data-view=new]').click()")
	out = append(out, b.eval(empties))
	b.eval("document.querySelector('.views [data-view=all]').click()")
	b.stub("[]")
	out = append(out, b.eval(empties), b.eval("document.getElementById('add-empty').offsetParent !== null"))

	// The time's tooltip lines start with the label of their format ("Created %s").
	label := func(key string) string { return strings.Fields(msg(key, ""))[0] }
	check(t, "extras", out, strings.Join([]string{
		"Beta,Alpha,Gamma,Linked,Missing", "Gamma fav",
		`PATCH /api/artifacts/b {"favorite":true}`, label("ui_created_at") + " " + label("ui_updated_at"), "none", "Alpha",
		msg("ui_selected_n", 1) + msg("ui_selected_more"), "grid", msg("ui_selected_n", 2) + msg("ui_selected_more"), msg("ui_delete_bulk_confirm", 2),
		"DELETE /api/artifacts/a,DELETE /api/artifacts/b", "none", msg("ui_unlink"),
		"true old", `PATCH /api/artifacts/a {"description":"new"}`, "true", "true",
		"copy /tmp/x/l.html", "true", "Missing", `POST /api/artifacts {"path":"/tmp/x/n.html","description":"a link"}`,
		"3 2 " + msg("ui_no_project") + "5 true 0 5 1 3 " + msg("ui_list_head", 0) + " Beta,Alpha,Gamma,Linked,Missing ",
		msg("ui_archive") + " / " + msg("ui_archive_named", "Beta") + " / " + msg("ui_archive"),
		`PATCH /api/artifacts/b {"archived":true}`, "0", "true",
		// The archived, by project: p's Zeta and those of none; two projects, so the shelf shows them.
		"2 1 p1 " + msg("ui_no_project") + "2 false 0 5 1 3 " + msg("ui_list_head", 0) + " Zeta,Ypsilon,Kept archived",
		msg("ui_unarchive") + " / " + msg("ui_unarchive_named", "Zeta") + " / " + msg("ui_unarchive"),
		`PATCH /api/artifacts/z {"archived":false}`, "true", msg("ui_unarchive"),
		`PATCH /api/artifacts/k {"archived":false},PATCH /api/artifacts/y {"archived":false},PATCH /api/artifacts/z {"archived":false}`,
		"empty-archived:" + msg("ui_none_archived") + " true", "false",
		msg("ui_favorite_failed", "Failed to fetch"), "true",
		"all-archived:" + msg("ui_all_archived", 1),
		"Zeta archived", "empty-new:" + msg("ui_no_new"),
		"empty:" + msg("ui_empty_pages"), "true",
	}, "|"))
}

// point moves the mouse onto (x, y) of the page.
func (b *browser) point(x, y float64) {
	b.t.Helper()

	p := input.DispatchMouseEventParams{Type: input.DispatchMouseEventTypeMouseMoved, X: x, Y: y}
	if _, err := chromedp.Call(b.ctx, input.DispatchMouseEvent, p); err != nil {
		b.t.Fatal(err)
	}
}

// TestUIShelf picks the list on the shelf, shows a row's actions where pointed at or reached, keeps a row's menu whole and
// gives focus back from it, adds a link from any list, folds the shelf into a row and a dialog on a narrow screen, and acts
// from the frame's bar, against a stubbed list.
func TestUIShelf(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	inEveryLanguage(t, uiShelf)
}

func uiShelf(t *testing.T, lang i18n.Lang, msg func(string, ...any) string) {
	t.Helper()

	b := newBrowser(t, servetest.Start(t, nil).Base, lang)
	b.stub(extrasList)
	b.navigate("/")
	if b.eval("matchMedia('(hover: hover)').matches") != "true" {
		t.Fatal("Chrome does not take the screen for one with a mouse: the flag blink-settings of startChrome no longer works")
	}
	b.point(0, 0)
	key := func(k string) {
		b.eval("document.body.dispatchEvent(new KeyboardEvent('keydown', { key: '" + k + "', bubbles: true }))")
	}
	const order = "[...document.getElementById('list').children].map((li) => li.classList.contains('group') ? 'H' : li.dataset.slug).join('')"
	const views = "[...document.querySelectorAll('.views button')].filter((b) => b.getAttribute('aria-pressed') === 'true').map((b) => b.dataset.view).join('') + ' ' + location.search"

	out := make([]string, 0, 40)
	// A heading is no row for a screen reader.
	out = append(out, b.eval(order), b.eval("[...document.querySelectorAll('#list li.group')].every((li) => "+
		"li.getAttribute('role') === 'none' && li.firstChild.getAttribute('role') === 'heading' && li.firstChild.getAttribute('aria-level') === '2')"))
	b.eval("document.querySelector('.views [data-view=fav]').click()")
	out = append(out, b.eval(order)+" "+b.eval(views))
	b.eval("document.querySelector('.views [data-view=archived]').click()")
	out = append(out, b.eval(order)+" "+b.eval(views))
	b.eval("document.querySelector('.views [data-view=all]').click()")
	// ↓ goes past a heading; selecting all selects its rows only.
	key("ArrowDown")
	key("ArrowDown")
	out = append(out, b.eval("document.activeElement.textContent"))
	b.eval("document.getElementById('bulk-page').click()")
	out = append(out, b.eval("document.getElementById('bulk-count').textContent"))
	// Shown before they hide, as on a slow machine, which clears the rows long after it selected them.
	b.until("the selected rows' actions to show", "getComputedStyle(document.querySelector('#list li[data-slug=c] .more')).opacity === '1'")
	b.eval("document.getElementById('bulk-clear').click(); document.activeElement.blur()")
	// With a mouse, a row's actions show only where pointed at, reached by keyboard, chosen or selected; a pressed star always.
	out = append(out, b.settled("['b', 'c'].map((slug) => ['.star', '.more'].map((part) => "+
		"getComputedStyle(document.querySelector(`#list li[data-slug=${slug}] ${part}`)).opacity).join('')).join('')"))
	r := b.rect("document.querySelector('#list li[data-slug=b]')")
	b.point(r.X+r.Width/2, r.Y+r.Height/2)
	out = append(out, b.until("the pointed row's actions", "getComputedStyle(document.querySelector('#list li[data-slug=b] .more')).opacity === '1' && 'shown'"))
	b.point(0, 0)
	b.until("the actions to hide", "getComputedStyle(document.querySelector('#list li[data-slug=b] .more')).opacity === '0'")
	b.eval("document.querySelector('#list li[data-slug=c] .title').focus()")
	out = append(out, b.until("the reached row's actions", "getComputedStyle(document.querySelector('#list li[data-slug=c] .more')).opacity === '1' && 'shown'"))
	b.eval("document.activeElement.blur(); document.querySelector('#list li[data-slug=b] .sel').click()")
	out = append(out, b.until("the selected row's actions", "getComputedStyle(document.querySelector('#list li[data-slug=b] .more')).opacity === '1' && 'shown'"))
	b.eval("document.getElementById('bulk-clear').click()")
	out = append(out, b.eval("[...document.querySelectorAll('#list li:not(.group)')].filter((li) => "+
		"['.star', '.archive', '.more summary'].some((s) => !li.querySelector(s).getAttribute('aria-label'))).length"))
	// The menu: its words, Escape and a click elsewhere close it, focus going back to its button.
	b.eval("document.querySelector('#list li[data-slug=b] summary').click()")
	out = append(out, b.eval("document.querySelector('#list li[data-slug=b] summary').getAttribute('aria-label')"),
		b.eval("[...document.querySelectorAll('#list li[data-slug=b] .menu > *')].filter((e) => getComputedStyle(e).display !== 'none').map((e) => e.textContent.trim()).join(' / ')"))
	b.eval("document.querySelector('#list li[data-slug=b] .edit').focus()")
	key("Escape")
	out = append(out, b.eval("[document.querySelector('#list li[data-slug=b] .more').open, "+
		"document.activeElement === document.querySelector('#list li[data-slug=b] summary')].join(' ')"))
	key("ArrowDown")
	out = append(out, b.eval("document.activeElement.textContent"))
	b.eval("document.querySelector('#list li[data-slug=b] summary').click(); document.querySelector('#list li[data-slug=b] .edit').focus()")
	b.eval("document.getElementById('count').click()")
	out = append(out, b.eval("[document.querySelector('#list li[data-slug=b] .more').open, "+
		"document.activeElement === document.querySelector('#list li[data-slug=b] summary')].join(' ')"))
	// One menu open at a time.
	b.eval("document.querySelector('#list li[data-slug=b] summary').click(); document.querySelector('#list li[data-slug=a] summary').click()")
	out = append(out, b.eval("[...document.querySelectorAll('#list .more')].filter((d) => d.open).length"))
	b.eval("document.querySelector('#list li[data-slug=a] summary').click()")
	// An item closes its menu; the edit dialog, cancelled or saved, gives focus back to the row's menu button.
	b.eval("document.querySelector('#list li[data-slug=b] summary').click(); document.querySelector('#list li[data-slug=b] .edit').click()")
	out = append(out, b.eval("[document.querySelector('#list li[data-slug=b] .more').open, document.getElementById('edit-dialog').open].join(' ')"))
	b.eval("document.querySelector('#edit-dialog [data-close]').click()")
	out = append(out, b.eval("document.activeElement === document.querySelector('#list li[data-slug=b] summary')"))
	b.eval(`(() => {
		document.querySelector('#list li[data-slug=b] summary').click();
		document.querySelector('#list li[data-slug=b] .edit').click();
		document.getElementById('edit-desc').value = 'new';
		document.getElementById('edit').requestSubmit();
	})()`)
	out = append(out, b.until("the edit dialog to close", "!document.getElementById('edit-dialog').open && "+
		"String(document.activeElement === document.querySelector('#list li[data-slug=b] summary'))"))
	// A row's menu is not cut off by the list's box: with one row, the menu reaches below it, its delete the topmost at its
	// middle, which deletes on a second click.
	b.stub(`[{ slug: 'a', title: 'Alpha', description: '', createdAt: at(1), updatedAt: at(8) }]`)
	// A window tall enough for the menu below the one row, whatever the test Chrome's own window is (CI's is shorter): the
	// width of that window, as the rest of the test sees it.
	b.size(780, 800, false)
	// Not scrollIntoView, which would scroll a box that cuts it off to bring it in.
	b.eval("document.querySelector('#list > li:last-child summary').click()")
	d := b.rect("document.querySelector('#list > li:last-child .delete')")
	out = append(out, b.eval("Boolean(document.elementFromPoint("+js(fmt.Sprint(d.X+d.Width/2))+", "+js(fmt.Sprint(d.Y+d.Height/2))+")?.closest('.delete'))"))
	b.eval("calls.length = 0; document.querySelector('#list > li:last-child .delete').click(); document.querySelector('#list > li:last-child .delete').click()")
	out = append(out, b.until("the delete", "calls.join(',')"))
	b.unsize()
	// The kinds show with two to choose between, the projects with two or one chosen; the templates with one.
	b.stub(`[{ slug: 'a', title: 'Alpha', description: '', project: 'p', createdAt: at(1), updatedAt: at(8) }]`)
	const shelves = "['shelf-projects', 'shelf-kinds', 'shelf-templates', 'shelf-tags'].map((id) => document.getElementById(id).hidden).join(' ')"
	out = append(out, b.eval(shelves))
	b.eval("toggleProject('p')")
	out = append(out, b.eval(shelves))
	b.eval("toggleProject('p')")
	b.stub(`[{ slug: 'a', title: 'Alpha', description: '', project: 'p', template: 'report', createdAt: at(1), updatedAt: at(8) },
		{ slug: 'l', title: 'L', description: '', path: '/tmp/x/l.yaml', template: 'report', createdAt: at(1), updatedAt: at(7) },
		{ slug: 'd', title: 'D', description: '', template: 'design', createdAt: at(1), updatedAt: at(6) }]`)
	out = append(out, b.eval(shelves), b.eval("[...document.querySelectorAll('#templates .chip')].map((c) => c.textContent).join(',')"))
	// Each part picks one, picked again lets it go; the parts narrow together; all in the URL.
	b.eval("document.querySelector('#templates [data-template=report]').click()")
	out = append(out, b.eval(order)+" "+b.eval("location.search"))
	b.eval("document.querySelector('#kinds [data-kind=link]').click()")
	out = append(out, b.eval(order)+" "+b.eval("location.search"))
	b.navigate("/?template=report&kind=link&view=fav&tab=links&sort=title&page=3")
	out = append(out, b.eval("[view, kind, template].join(' ')"))
	b.eval("document.querySelector('.views [data-view=fav]').click(); document.querySelector('#kinds [data-kind=link]').click(); document.querySelector('#templates [data-template=report]').click()")
	out = append(out, b.eval(order)+" "+b.eval("location.search"))
	// A project's dot is of its color by projectOrder: as many each, by name.
	b.stub(`[{ slug: 'a', title: 'Alpha', description: '', project: 'p', createdAt: at(1), updatedAt: at(8) },
		{ slug: 'b', title: 'Beta', description: '', project: 'floe', createdAt: at(1), updatedAt: at(7) }]`)
	out = append(out, b.eval("[...document.querySelectorAll('#chips .chip')].map((c) => c.dataset.project + ':' + c.style.getPropertyValue('--c')).join(',')"))
	// The favourites count their own projects, and none shown when there are none.
	b.stub(`[{ slug: 'c', title: 'Gamma', description: '', project: 'p', createdAt: at(1), updatedAt: at(9), favorite: true },
		{ slug: 'b', title: 'Beta', description: '', project: 'q', createdAt: at(1), updatedAt: at(8) }]`)
	b.eval("document.querySelector('.views [data-view=fav]').click()")
	out = append(out, b.eval("document.getElementById('shelf-projects').hidden"))
	const empties = "['empty', 'all-archived', 'empty-favorites', 'empty-archived', 'nomatch']" +
		".filter((id) => !document.getElementById(id).hidden).join(' ')"
	b.stub(`[{ slug: 'z', title: 'Zeta', description: '', createdAt: at(1), updatedAt: at(6), favorite: true, archived: true }]`)
	out = append(out, b.eval(empties))
	b.eval("document.querySelector('.views [data-view=all]').click()")
	out = append(out, b.eval(empties))
	b.eval("document.getElementById('show-archived').click()")
	out = append(out, b.eval(views))
	b.stub(extrasList)
	// A link is added from any list but the archived, and then shown on the whole list, unfiltered by a project or a search it
	// may not match, drawn once.
	b.eval("document.querySelector('.views [data-view=all]').click()")
	b.stub(`[{ slug: 'a', title: 'Alpha', description: '', project: 'p', createdAt: at(1), updatedAt: at(8) }]`)
	out = append(out, b.eval("document.getElementById('add-open').hidden"))
	b.eval("toggleProject('p'); (() => { const q = document.getElementById('q'); q.value = 'Al'; q.dispatchEvent(new Event('input')); })()")
	b.eval(`(() => {
		window.renders = 0;
		window.draw = render;
		render = () => { renders++; draw(); };
		// The server lists the link added.
		const post = window.fetch;
		window.fetch = (url, init = {}) => {
			if (init.method === 'POST') list = JSON.stringify([{ slug: 'n', title: 'New', description: '', path: '/tmp/x/n.html', createdAt: '2026-01-01T00:00:00.000Z', updatedAt: '2026-01-09T00:00:00.000Z' }, ...JSON.parse(list)]);
			return post(url, init);
		};
		document.getElementById('add-open').click();
		document.getElementById('add-path').value = '/tmp/x/n.html';
		document.getElementById('add').requestSubmit();
	})()`)
	// Waited for until drawn: the dialog closes before the list is fetched again.
	b.until("the link to be added", "!document.getElementById('add-dialog').open && document.getElementById('q').value === '' && !location.search.includes('project')")
	out = append(out, b.until("the link to show", "document.querySelector('#list li[data-slug=n]') && [location.search === '', renders, "+
		"[...document.querySelectorAll('#list .title')].map((a) => a.textContent)].join(' ')"))
	b.eval("render = draw")
	b.eval("document.querySelector('.views [data-view=archived]').click()")
	out = append(out, b.eval("document.getElementById('add-open').hidden"))
	b.eval("document.querySelector('.views [data-view=all]').click()")
	// An open menu passes under the head stuck at the top as the list scrolls, not over it.
	b.stub(manyPages(60))
	b.eval("document.querySelector('#list li[data-slug=p5] summary').click()")
	out = append(out, b.eval(`(() => {
		const bar = document.querySelector('.lhead').getBoundingClientRect();
		document.getElementById('list-body').scrollBy(0, document.querySelector('#list li[data-slug=p5] .menu').getBoundingClientRect().top - bar.top - 5);
		const menu = document.querySelector('#list li[data-slug=p5] .menu').getBoundingClientRect();
		return Boolean(document.elementFromPoint(menu.left + menu.width / 2, bar.top + 10)?.closest('.lhead'));
	})()`))
	b.eval("document.querySelector('#list li[data-slug=p5] summary').click(); document.getElementById('list-body').scrollTo(0, 0)")
	b.unsize()
	// The frame's bar: the title, archived or not, the star, and the same menu as a row's, which act as the list's do and tell
	// their failure.
	frame := "[document.getElementById('frame-title').textContent, document.getElementById('frame-archived').hidden, " +
		"document.querySelector('#frame-actions .star').getAttribute('aria-pressed'), document.querySelector('#frame-actions .star').getAttribute('aria-label'), " +
		"document.querySelector('#frame-actions .open').getAttribute('href'), document.querySelector('#frame-actions .menu .archive').textContent.trim()].join(' / ')"
	b.stub(extrasList)
	b.navigate("/a/z/")
	out = append(out, b.until("the frame's bar", "document.getElementById('frame-title').textContent && "+frame))
	b.eval("document.querySelector('#frame-actions > .archive').click()")
	out = append(out, b.until("the give back", "calls.splice(0).join(',')"))
	// Given back, as the server now lists it: the menu says so.
	b.eval("window.list = JSON.stringify(JSON.parse(window.list).map((a) => a.slug === 'z' ? { ...a, archived: false } : a)); load()")
	out = append(out, b.until("the menu to say so", "document.getElementById('frame-archived').hidden && "+frame))
	b.eval("window.failWrites = true; document.querySelector('#frame-actions .star').click()")
	out = append(out, b.until("the failure to show", "!document.getElementById('error').hidden && document.getElementById('error').offsetHeight > 0 && "+
		"document.getElementById('error').textContent"))
	b.eval("window.failWrites = false")
	// Renamed in the frame, the tab goes to the new name; removed, to the list.
	b.eval(`(() => {
		document.querySelector('#frame-actions .edit').click();
		document.getElementById('edit-slug').value = 'z2';
		document.getElementById('edit').requestSubmit();
	})()`)
	out = append(out, b.until("the new name", "location.pathname === '/a/z2/' && location.pathname"))
	b.navigate("/a/b/")
	b.until("the frame's bar", "document.getElementById('frame-title').textContent")
	b.eval("document.querySelector('#frame-actions .delete').click(); document.querySelector('#frame-actions .delete').click()")
	b.until("the list", "location.pathname === '/'")

	check(t, "shelf", out, strings.Join([]string{
		"Hbaclm", "true", "Hc fav ?view=fav", "Hzyk archived ?view=archived", "Alpha", msg("ui_selected_n", 5) + msg("ui_selected_more"),
		"0010", "shown", "shown", "shown", "0",
		msg("ui_more_named", "Beta"), msg("ui_mark_unread") + " / " + msg("ui_select") + " / " + msg("ui_pin") + " / " + msg("ui_task_set") + " / " + msg("ui_edit") + " / " + msg("ui_open_raw") + " / " + msg("ui_download") + " / " + msg("ui_archive") + " / " + msg("ui_delete"),
		"false true", "Alpha", "false true", "1", "false true", "true", "true",
		"true", "DELETE /api/artifacts/a",
		"true true true true", "false true true true", "false false false true", "report2,design1",
		"Hal ?template=report", "Hl ?kind=link&template=report", "fav link report", "Hald ",
		"floe:var(--pc0),p:var(--pc1)", "true", "empty-favorites", "all-archived", "archived ?view=archived",
		"false", "true 1 New,Alpha", "true", "true",
		"Zeta / false / true / " + msg("ui_favorite_named", "Zeta") + " / /a/z/?raw / " + msg("ui_unarchive"), `PATCH /api/artifacts/z {"archived":false}`,
		"Zeta / true / true / " + msg("ui_favorite_named", "Zeta") + " / /a/z/?raw / " + msg("ui_archive"),
		msg("ui_favorite_failed", "Failed to fetch"), "/a/z2/",
	}, "|"))
}

// tagsList is the list the stub answers for tags: pages with tags, one whose tag has a comma in it, one with none, and
// an archived one with a tag of its own.
const tagsList = `[
	{ slug: 'a', title: 'Alpha', description: '', tags: ['設計', 'plan'], createdAt: at(1), updatedAt: at(9) },
	{ slug: 'b', title: 'Beta', description: '', tags: ['plan'], createdAt: at(1), updatedAt: at(8) },
	{ slug: 'c', title: 'Gamma', description: '', tags: ['設計、実装'], createdAt: at(1), updatedAt: at(7) },
	{ slug: 'd', title: 'Delta', description: '', createdAt: at(1), updatedAt: at(6) },
	{ slug: 'z', title: 'Zeta', description: '', tags: ['old', 'plan'], createdAt: at(1), updatedAt: at(5), archived: true },
]`

// TestUITags picks a tag on the shelf, by a row's tag, in the URL and on a narrow screen, counts the tags of the list
// looked at, keeps the tag picked at 0 and lets it go with the other filters, searches tags, gives them in the edit
// dialog and to those selected, and puts their names in as text, against a stubbed list.
func TestUITags(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	inEveryLanguage(t, uiTags)
}

func uiTags(t *testing.T, lang i18n.Lang, msg func(string, ...any) string) {
	t.Helper()

	b := newBrowser(t, servetest.Start(t, nil).Base, lang)
	b.stub(tagsList)
	b.navigate("/")
	const rows = "[...document.querySelectorAll('#list li:not(.group)')].map((li) => li.dataset.slug).join('') + ' ' + (new URLSearchParams(location.search).get('tag') ?? '-')"
	const shelf = "[...document.querySelectorAll('#tags .chip')].map((c) => c.textContent + ':' + c.getAttribute('aria-pressed')).join(',') + ' ' + document.getElementById('shelf-tags').hidden"
	// Presses the shelf's tag plan, which picks it or lets it go.
	pickPlan := func() {
		b.eval("[...document.querySelectorAll('#tags .chip')].find((c) => c.dataset.tag === 'plan').click()")
	}
	search := func(q string) {
		b.eval("(() => { const q = document.getElementById('q'); q.value = " + js(q) + "; q.dispatchEvent(new Event('input')); })()")
	}
	press := b.pressKey
	// The tag dialog's chips: name, state, the mark shown and count
	const boxes = "[...document.querySelectorAll('#tag-boxes .tag-box')].map((l) => { const c = l.querySelector('input'); " +
		"return [l.querySelector('.name').textContent, c.indeterminate ? 'some' : c.checked ? 'all' : 'none', " +
		"getComputedStyle(l.querySelector('.mark'), '::before').content.replaceAll('\"', ''), l.querySelector('.n')?.textContent ?? ''].join(':'); }).join(',')"
	const applies = "!document.getElementById('tag-apply').disabled"
	selectOnly := func(slugs ...string) {
		b.eval("document.getElementById('bulk-clear').click()")
		for _, slug := range slugs {
			b.eval("document.querySelector('#list li[data-slug=" + slug + "] .sel').click()")
		}
	}
	box := func(name string) string {
		return "[...document.querySelectorAll('#tag-boxes .tag-box')].find((l) => l.querySelector('.name').textContent === " + js(name) + ").querySelector('input')"
	}

	out := make([]string, 0, 40)
	// The shelf names the tags of the list looked at by name, with their counts.
	out = append(out, b.eval(shelf), b.eval(rows),
		b.eval("[...document.querySelectorAll('#list li[data-slug=a] .tagpill')].map((p) => p.textContent + ':' + p.getAttribute('aria-label')).join(',')"),
		b.eval("document.querySelector('#list li[data-slug=d] .tags') === null"))
	// Picked, the rows are its pages, as when the URL has it; picked again it is let go.
	pickPlan()
	out = append(out, b.eval(rows), b.eval(shelf))
	b.navigate("/?tag=plan")
	out = append(out, b.eval(rows))
	pickPlan()
	out = append(out, b.eval(rows))
	b.eval("document.querySelector('.views [data-view=archived]').click()")
	out = append(out, b.eval(shelf))
	b.eval("document.querySelector('.views [data-view=all]').click()")
	// A row's tag picks it; the tag picked already, the list stays, its selection too.
	b.eval("document.querySelector('#list li[data-slug=c] .tagpill').click()")
	out = append(out, b.eval(rows))
	b.eval("document.querySelector('#list li[data-slug=c] .sel').click(); document.querySelector('#list li[data-slug=c] .tagpill').click()")
	out = append(out, b.eval("document.getElementById('bulk-count').textContent"))
	b.eval("document.getElementById('bulk-clear').click()")
	// Searching finds a page by its tags.
	b.eval("document.getElementById('clear').click()")
	search("設計")
	out = append(out, b.eval(rows))
	search("")
	// A tag picked stays at 0, until the filters are cleared.
	pickPlan()
	b.stub(strings.ReplaceAll(tagsList, "'plan'", "'other'"))
	out = append(out, b.eval(shelf), b.eval("document.getElementById('nomatch').hidden"))
	b.eval("document.getElementById('clear').click()")
	out = append(out, b.eval(rows))
	b.stub(tagsList)

	// The edit dialog sends the tags written, split at commas; as opened, none, so that a name with a comma stays whole.
	b.eval("document.querySelector('#list li[data-slug=a] .edit').click()")
	out = append(out, b.eval("document.getElementById('edit-tags').value"))
	b.eval("document.getElementById('edit-tags').value = '計画、 設計,'; document.getElementById('edit').requestSubmit()")
	b.until("the edit dialog to close", "!document.getElementById('edit-dialog').open")
	out = append(out, b.eval("calls.splice(0).join(',')"))
	b.eval("document.querySelector('#list li[data-slug=c] .edit').click()")
	b.eval("document.getElementById('edit-name').value = 'G2'; document.getElementById('edit').requestSubmit()")
	b.until("the edit dialog to close", "!document.getElementById('edit-dialog').open")
	out = append(out, b.eval("calls.splice(0).join(',')"))

	// The tag dialog boxes every tag, those on the selected first: on all, on some (indeterminate, with how many), on none.
	selectOnly("a", "b", "d")
	b.eval("document.getElementById('bulk-tag').click()")
	out = append(out, b.eval("document.getElementById('tag-title').textContent"), b.eval(boxes), b.eval(applies))
	// With keys alone: Tab to a box, Space marks it. One on some goes some, all, none and back; the others all and none.
	b.until("the find field to have focus", "document.activeElement.id === 'tag-find'")
	press(kb.Tab)
	// The chip reached shows the ring of its hidden box's focus.
	out = append(out, b.eval("document.activeElement.closest('.tag-box').querySelector('.name').textContent + ' ' + "+
		"getComputedStyle(document.activeElement.closest('.tag-box')).outlineStyle"))
	marks := make([]string, 0, 4)
	for range 4 {
		press(" ")
		marks = append(marks, b.eval(box("plan")+".indeterminate ? 'some' : "+box("plan")+".checked ? 'all' : 'none'")+":"+b.eval(applies))
	}
	out = append(out, strings.Join(marks, " "))
	press(kb.Tab)
	press(" ")
	press(" ")
	press(kb.Tab)
	press(" ")
	out = append(out, b.eval(boxes), b.eval(applies))
	// Applied, each page whose tags change is sent them so, drawn once by the list read again before (changed since).
	b.eval("window.renders = 0; window.draw = render; render = () => { renders++; draw(); }; list = list.replace('Delta', 'Delta 1')")
	b.eval("document.getElementById('tag-apply').click()")
	out = append(out, b.until("the tags to be applied", "document.getElementById('bulk').hidden && calls.splice(0).sort().join(',')"),
		b.eval("document.getElementById('toasts').textContent.includes("+js(msg("ui_tag_bulk_done", 3))+")"), b.eval("renders"))
	b.eval("render = draw")
	// A tag left on some is left as each page has it.
	selectOnly("a", "c")
	b.eval("document.getElementById('bulk-tag').click(); " + box("old") + ".click(); document.getElementById('tag-apply').click()")
	out = append(out, b.until("the tag to be added", "document.getElementById('bulk').hidden && calls.splice(0).sort().join(',')"))
	// The find field narrows the boxes; a name no tag has can be made one, marked all, by Enter, sending nothing.
	selectOnly("b")
	b.eval("document.getElementById('bulk-tag').click()")
	b.until("the find field to have focus", "document.activeElement.id === 'tag-find'")
	b.eval("(() => { const f = document.getElementById('tag-find'); f.value = 'la'; f.dispatchEvent(new Event('input')); })()")
	out = append(out, b.eval(boxes)+" "+b.eval("document.getElementById('tag-new').hidden"))
	b.eval("(() => { const f = document.getElementById('tag-find'); f.value = 'new'; f.dispatchEvent(new Event('input')); })()")
	out = append(out, b.eval("document.getElementById('tag-new').textContent"))
	press(kb.Enter)
	out = append(out, b.eval(boxes), b.eval("[document.getElementById('tag-dialog').open, calls.length, "+applies+"].join(' ')"))
	// Cancelled after a change, the dialog opens again with nothing to apply.
	b.eval("document.querySelector('#tag-dialog [data-close]').click(); document.getElementById('bulk-tag').click()")
	out = append(out, b.eval(applies))
	b.eval("document.querySelector('#tag-dialog [data-close]').click()")
	// A page removed since the list was read leaves the selection unsent.
	selectOnly("b")
	b.eval("list = JSON.stringify(JSON.parse(list).filter((a) => a.slug !== 'b')); document.getElementById('bulk-tag').click(); " +
		box("old") + ".click(); document.getElementById('tag-apply').click()")
	out = append(out, b.until("the gone page to drop", "document.getElementById('bulk').hidden && "+
		"[document.getElementById('toasts').textContent.includes("+js(msg("ui_tag_bulk_none"))+"), calls.length].join(' ')"))
	b.stub(tagsList)
	// Narrowed to a tag, the edit dialog's field holds it whole: pressed, the field offers the other names (reported by the user).
	pickPlan()
	b.eval("document.querySelector('#list li[data-slug=b] .edit').click()")
	// The test's window is low enough to hide the field under the dialog's fold.
	b.still()
	b.eval("document.getElementById('edit-tags').scrollIntoView({ block: 'center' })")
	r := b.rect("document.getElementById('edit-tags')")
	b.click(r.X+r.Width-8, r.Y+r.Height/2)
	out = append(out, b.until("the names on a press", "document.getElementById('edit-tags-names').matches(':popover-open') && "+
		"[...document.querySelectorAll('#edit-tags-names [role=option]')].map((o) => o.textContent).join(',')"))
	press(kb.ArrowDown)
	press(kb.Enter)
	out = append(out, b.eval("document.getElementById('edit-tags').value"))
	// Pressed after a name begun, the names are of what is begun where the caret lands, after focus put it at the start.
	b.eval("document.getElementById('edit-tags').blur(); document.getElementById('edit-tags').value = 'plan, 設'")
	b.click(r.X+r.Width-8, r.Y+r.Height/2)
	out = append(out, b.until("the names where pressed", "document.getElementById('edit-tags-names').matches(':popover-open') && "+
		"[...document.querySelectorAll('#edit-tags-names [role=option]')].map((o) => o.textContent).join(',')"))
	b.eval("document.querySelector('#edit-dialog [data-close]').click()")
	pickPlan()

	// A narrow screen's dialog of the rest of the shelf has the tags, which pick and let go as the wide shelf's do.
	b.size(390, 844, true)
	b.eval("document.getElementById('narrow-filter').click()")
	plan := "[...document.querySelectorAll('#tags .chip')].find((c) => c.dataset.tag === 'plan')"
	out = append(out, b.eval("[...document.querySelectorAll('#tags .chip')].filter((c) => c.offsetParent).map((c) => c.dataset.tag).join(',')"))
	b.eval(plan + ".click()")
	out = append(out, b.eval(rows), b.eval(plan+".getAttribute('aria-pressed')"))
	out = append(out, b.eval("document.getElementById('narrow-filter-name').textContent"))
	b.eval(plan + ".click()")
	out = append(out, b.eval(rows))
	b.eval("document.querySelector('#filter-dialog [data-close]').click()")
	out = append(out, b.until("the dialog to close", "document.querySelector('#shelf-more').parentElement.id === 'shelf-body' && 'shelf'"))
	b.unsize()

	// A tag's name goes in as text, wherever it shows, and counts as any.
	b.stub(`[{ slug: 'a', title: 'Alpha', description: '', tags: ['<b>x</b>', '__proto__'], createdAt: at(1), updatedAt: at(9) }]`)
	out = append(out, b.eval("[...document.querySelectorAll('#tags .chip')].map((c) => c.textContent).sort().join(',')"),
		b.eval("[...document.querySelectorAll('#list .tagpill')].map((p) => p.textContent).join(',')"),
		b.eval("document.querySelectorAll('.shelf b, #list b').length"))
	selectOnly("a")
	b.eval("document.getElementById('bulk-tag').click()")
	out = append(out, b.eval(boxes)+" "+b.eval("document.querySelectorAll('#tag-boxes b').length"))
	b.eval("document.querySelector('#tag-dialog [data-close]').click(); document.getElementById('bulk-clear').click()")

	// The tags fields suggest the names of every page, as a combobox, with the keyboard alone or the mouse.
	b.stub(`[{ slug: 'a', title: 'Alpha', description: '', tags: ['設計', 'plan'], createdAt: at(1), updatedAt: at(9) },
		{ slug: 'b', title: 'Beta', description: '', tags: ['計画', '<b>x</b>', 'xa'], createdAt: at(1), updatedAt: at(8), archived: true }]`)
	write := func(v string) {
		b.eval("(() => { const f = document.getElementById('edit-tags'); f.value = " + js(v) + "; f.setSelectionRange(f.value.length, f.value.length); f.dispatchEvent(new Event('input')); })()")
	}
	const namesBox = "document.getElementById('edit-tags-names')"
	const names = "[...document.querySelectorAll('#edit-tags-names [role=option]')].map((o) => o.textContent).join(',')"
	const shown = namesBox + ".matches(':popover-open') + ' ' + document.getElementById('edit-tags').getAttribute('aria-expanded')"
	b.eval("document.querySelector('#list li[data-slug=a] .edit').click(); document.getElementById('edit-tags').value = ''; document.getElementById('edit-tags').focus()")
	out = append(out, b.eval("(() => { const f = document.getElementById('edit-tags'); "+
		"return [f.getAttribute('role'), f.getAttribute('aria-autocomplete'), f.getAttribute('aria-controls') === document.getElementById('edit-tags-names').id, "+
		"document.getElementById('edit-tags-names').getAttribute('role')].join(' '); })()"))
	// Reached, the empty field opens all the names, the archived page's too, as text; ↓ picks the first; Escape closes the names,
	// not the dialog.
	out = append(out, b.eval(names), b.eval(shown), b.eval(namesBox+".querySelector('b') === null"),
		// Under the field, once the dialog has done moving in.
		b.until("the names under the field", "(() => { const f = document.getElementById('edit-tags').getBoundingClientRect(); const r = "+namesBox+".getBoundingClientRect(); "+
			"return Math.abs(r.top - f.bottom - 4) < 0.5 && Math.abs(r.left - f.left) < 0.5 && Math.abs(r.width - f.width) < 0.5 && 'under'; })()"))
	press(kb.ArrowDown)
	out = append(out, b.eval("document.getElementById('edit-tags').getAttribute('aria-activedescendant') === "+namesBox+".querySelector('[aria-selected=true]').id && "+
		namesBox+".querySelector('[aria-selected=true]').textContent"))
	press(kb.Escape)
	out = append(out, b.eval(shown)+" "+b.eval("document.getElementById('edit-dialog').open"))
	// The name between the commas around the caret: those holding it, starting with it first, but none the field has.
	write("計")
	out = append(out, b.eval(names))
	write("x")
	out = append(out, b.eval(names))
	write("計画, ")
	out = append(out, b.eval(names))
	// ↓↓↓ and Enter put the third in, a comma after it for the next, sending nothing.
	press(kb.ArrowDown)
	press(kb.ArrowDown)
	press(kb.ArrowDown)
	press(kb.Enter)
	out = append(out, b.eval("document.getElementById('edit-tags').value"), b.eval(shown),
		b.eval("[document.getElementById('edit-dialog').open, calls.length].join(' ')"))
	// The list read again leaves the names open, the one picked as it is.
	press(kb.ArrowDown)
	press(kb.ArrowDown)
	b.eval("list = list.replace('Alpha', 'Alpha 2'); load()")
	b.until("the list to be read again", "document.querySelector('#list li[data-slug=a] .title').textContent === 'Alpha 2'")
	out = append(out, b.eval(shown)+" "+b.eval(namesBox+".querySelector('[aria-selected=true]')?.textContent"))
	// A click puts a name in, the field keeping its focus.
	o := b.rect(namesBox + ".querySelector('[role=option]:last-child')")
	b.click(o.X+o.Width/2, o.Y+o.Height/2)
	out = append(out, b.until("the name clicked", "document.getElementById('edit-tags').value.includes('設計') && document.getElementById('edit-tags').value"),
		b.eval("document.activeElement.id"))
	press(kb.ArrowDown)
	press(kb.Escape)
	press(kb.Escape)
	out = append(out, b.eval("document.getElementById('edit-dialog').open"))
	// Opened as its dialog opens, the names go under the field once the dialog stops moving.
	b.eval("document.querySelector('#list li[data-slug=a] .edit').click(); document.getElementById('edit-tags').value = ''; document.getElementById('edit-tags').focus()")
	b.still()
	out = append(out, b.until("the names under the moving field", "(() => { "+
		"const f = document.getElementById('edit-tags').getBoundingClientRect(); const r = document.getElementById('edit-tags-names').getBoundingClientRect(); "+
		"return Math.abs(r.top - f.bottom - 4) < 0.5 && Math.abs(r.left - f.left) < 0.5 && 'under'; })()"))

	check(t, "tags", out, strings.Join([]string{
		"plan2:false,設計1:false,設計、実装1:false false", "abcd -",
		"設計:" + msg("ui_tag_filter_named", "設計") + ",plan:" + msg("ui_tag_filter_named", "plan"), "true",
		"ab plan", "plan2:true,設計1:false,設計、実装1:false false", "ab plan", "abcd -",
		"old1:false,plan1:false false", "c 設計、実装", msg("ui_selected_n", 1) + msg("ui_selected_more"), "ac -",
		"other2:false,plan0:true,設計1:false,設計、実装1:false false", "false", "abcd -",
		"設計, plan", `PATCH /api/artifacts/a {"tags":["計画","設計"]}`, `PATCH /api/artifacts/c {"title":"G2"}`,
		msg("ui_tag_bulk_title", 3), "plan:some:−:2/3,設計:some:−:1/3,old:none:+:,設計、実装:none:+:", "false", "plan solid",
		"all:true none:true some:false all:true", "plan:all:✓:2/3,設計:none:+:1/3,old:all:✓:,設計、実装:none:+:", "true",
		`PATCH /api/artifacts/a {"tags":["plan","old"]},PATCH /api/artifacts/b {"tags":["plan","old"]},PATCH /api/artifacts/d {"tags":["old","plan"]}`, "true", "1",
		`PATCH /api/artifacts/a {"tags":["設計","plan","old"]},PATCH /api/artifacts/c {"tags":["設計、実装","old"]}`,
		"plan:all:✓: false", msg("ui_tag_create", "new"), "plan:all:✓:,new:all:✓:,old:none:+:,設計:none:+:,設計、実装:none:+:", "true 0 true", "false",
		"true 0",
		"old,設計,設計、実装", "plan, old, ", "設計,設計、実装",
		"plan,設計,設計、実装", "ab plan", "true", msg("ui_narrow_n", 1), "abcd -", "shelf",
		"<b>x</b>1,__proto__1", "<b>x</b>,__proto__", "0", "__proto__:all:✓:,<b>x</b>:all:✓: 0",
		"combobox list true listbox",
		"<b>x</b>,plan,xa,計画,設計", "true true", "true", "under", "<b>x</b>",
		"false false true",
		"計画,設計", "xa,<b>x</b>", "<b>x</b>,plan,xa,設計",
		"計画, xa, ", "false false", "true 0",
		"true true <b>x</b>",
		"計画, xa, 設計, ", "edit-tags",
		"false", "under",
	}, "|"))
}

// TestUIShelfStays keeps the shelf and the page beside the list where they sit, the list scrolling in itself, and the window
// not scrolling, whether the list is long enough to scroll or not.
func TestUIShelfStays(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.stub(manyPages(60))
	b.navigate("/")
	b.size(1440, 900, false)
	const place = "['.shelf', '.reader', '.lhead'].map((s) => document.querySelector(s).getBoundingClientRect().top).join(' ')"
	out := make([]string, 0, 5)
	out = append(out, b.eval("document.getElementById('list-body').scrollHeight > document.getElementById('list-body').clientHeight"),
		b.eval("document.documentElement.scrollHeight === innerHeight"))
	long := b.eval(place)
	b.eval("document.getElementById('list-body').scrollTo(0, 300)")
	b.until("the list to scroll", "document.getElementById('list-body').scrollTop === 300")
	out = append(out, b.eval(place+" === "+js(long)))
	b.stub(manyPages(1))
	out = append(out, b.eval("document.documentElement.scrollHeight === innerHeight"), b.eval(place+" === "+js(long)))
	check(t, "shelf stays", out, "true|true|true|true|true")
}

// TestUIShelfScrolls scrolls the shelf in itself when taller than the window, with room for its focus rings, its long names on
// one line, cut off, its height kept with the bulk bar; a press of the wheel over it leaves the list where it is.
func TestUIShelfScrolls(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	// Thirty projects and a tag of a long name with nowhere to break, on a list long enough to scroll.
	const shelfList = "Array.from({ length: 60 }, (_, i) => ({ slug: `p${i}`, title: `P${i}`, description: '', project: `project${i % 30}`, " +
		"tags: i ? [] : ['averylongtagnamewithoutanyspacesorhyphens'], createdAt: at(1), updatedAt: at(1) }))"
	b.stub(shelfList)
	b.navigate("/")
	b.size(1440, 600, false)
	const near = "const shelf = document.getElementById('shelf-body'); const r = shelf.getBoundingClientRect();"
	out := make([]string, 0, 6)
	out = append(out, b.eval("(() => { "+near+" return [Math.abs(document.querySelector('.shelf').getBoundingClientRect().bottom - innerHeight) <= 0.5, shelf.scrollHeight > shelf.clientHeight].join(' '); })()"))
	// The wheel scrolls the shelf, not the list.
	sr := b.rect("document.querySelector('.shelf')")
	// The protocol wants deltaX sent, which a 0 is left out as: a sliver of a pixel moves nothing sideways.
	e := input.DispatchMouseEventParams{Type: input.DispatchMouseEventTypeMouseWheel, X: sr.X + sr.Width/2, Y: sr.Y + sr.Height/2, DeltaX: 1e-6, DeltaY: 100}
	if _, err := chromedp.Call(b.ctx, input.DispatchMouseEvent, e); err != nil {
		t.Fatal(err)
	}
	out = append(out, b.until("the shelf to scroll", "document.getElementById('shelf-body').scrollTop > 0 && `list at ${document.getElementById('list-body').scrollTop}`"))
	// The focus ring's room each side, and a long name on one line, cut off, not scrolling sideways.
	out = append(out, b.eval("(() => { "+near+" const a = document.querySelector('.views button').getBoundingClientRect(); "+
		"const long = document.querySelector('#tags .chip .name'); "+
		"return [a.left - r.left >= 4, r.left + shelf.clientWidth - a.right >= 4, shelf.scrollWidth === shelf.clientWidth, "+
		"long.scrollWidth > long.clientWidth && long.getClientRects().length === 1].join(' '); })()"))
	// Pointed at, the shelf is laid out again for Chrome to draw its thumb; nothing in it moves.
	hover := "document.querySelector('.views button').getBoundingClientRect().right"
	b.point(800, 300)
	away := b.eval(hover)
	b.point(sr.X+sr.Width/2, sr.Y+sr.Height/2)
	on := b.until("the shelf to be pointed at", "document.querySelector('.shelf:hover') && ("+hover+")")
	b.point(800, 300)
	out = append(out, strconv.FormatBool(away == on))
	// With the bulk bar, in the head over the list, the shelf keeps its height.
	tall := b.eval("document.querySelector('.shelf').getBoundingClientRect().height")
	b.eval("document.querySelector('#list li:not(.group) .sel').click()")
	out = append(out, b.until("the bar to show", "(() => { const bar = document.getElementById('bulk').getBoundingClientRect(); "+
		"return bar.height > 0 && `${document.getElementById('bulk').closest('.lhead') !== null} ${document.querySelector('.shelf').getBoundingClientRect().height === "+tall+"}`; })()"))
	b.eval("document.getElementById('bulk-clear').click()")
	// A shelf that fits does not scroll.
	b.size(1440, 900, false)
	b.stub(manyPages(60))
	out = append(out, b.eval("(() => { "+near+" return shelf.scrollHeight === shelf.clientHeight; })()"))
	check(t, "shelf scrolls", out, "true true|list at 0|true true true true|true|true true|true")
}

// TestUIScrollbar draws the list's scrollbar as the management page's own, 10px wide, within the list's box, the window
// scrolling neither way, and makes the frame's URLs of the slug escaped again.
func TestUIScrollbar(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.page(t, "p.yaml", "P", "<p>p</p>"), "--slug", "p")
	b.stub(manyPages(60))
	b.navigate("/")
	sb := b.eval(`(() => {
		const d = Object.assign(document.createElement('div'), { style: 'overflow: scroll; width: 100px; height: 100px; position: absolute' });
		document.body.append(d);
		const width = d.offsetWidth - d.clientWidth;
		d.remove();
		return width;
	})()`)
	width, err := strconv.Atoi(sb)
	switch {
	case err != nil:
		t.Fatalf("the scrollbar's width %q", sb)
	case width == 0 && runtime.GOOS == "darwin":
		t.Skip("macOS draws its scrollbars over the page, taking no room")
	case width == 0:
		t.Fatal("Chrome draws no scrollbar: the flag hide-scrollbars of startChrome no longer works")
	}
	out := make([]string, 0, 3)
	out = append(out, sb, b.eval("document.documentElement.scrollWidth === innerWidth && document.documentElement.scrollHeight === innerHeight"))
	// The server's list for the frames.
	b.unstub()
	// The frame makes its URLs of the slug escaped again: one read from /a/x%3Fy/ is x?y. No page of it: its way to look for one.
	b.navigate("/a/x%3Fy/")
	out = append(out, b.until("the state of a missing page", "!document.getElementById('notfound').hidden && document.getElementById('notfound-detail').textContent.includes('x?y') && document.getElementById('news').hidden !== undefined && 'told'"))
	check(t, "scrollbar", out, "10|true|told")
}

// TestUIStubSlowServer keeps the list stubbed though the server's answer comes late: none of the page's loads, the first
// or a poll, reaches the server, so that its list is never drawn, not even for a moment before a poll draws the stub's
// again.
func TestUIStubSlowServer(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.page(t, "real.yaml", "Real", "<p>r</p>"), "--slug", "real")
	// The list's answer alone, slower than a poll: the page itself comes at once.
	slow := network.EmulateNetworkConditionsByRuleParams{MatchedNetworkConditions: []*network.Conditions{
		{URLPattern: s.Base + "/api/artifacts", Latency: 3500, DownloadThroughput: -1, UploadThroughput: -1},
	}}
	if _, err := chromedp.Call(b.ctx, network.EmulateNetworkConditionsByRule, slow); err != nil {
		t.Fatal(err)
	}
	b.stub(`[{ slug: 'a', title: 'Alpha', description: '', createdAt: at(1), updatedAt: at(1) }]`)
	b.navigate("/")
	b.eval("window.seen = new Set([...document.querySelectorAll('#list .title')].map((a) => a.textContent)); " +
		"new MutationObserver(() => { for (const a of document.querySelectorAll('#list .title')) seen.add(a.textContent); })" +
		".observe(document.getElementById('list'), { childList: true, subtree: true })")
	// Past when the server's answer would have come, a poll between: the page's clock, from its start.
	b.until("the server's answer to be late", "performance.now() > 5000 && window.gets >= 3")
	check(t, "titles drawn", []string{b.eval("[...seen].join(',')")}, "Alpha")
}
