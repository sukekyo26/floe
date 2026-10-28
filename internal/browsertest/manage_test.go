package browsertest_test

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/input"
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

// stub answers list, a JavaScript array at(day) may build, for the list, and records the writes in window.calls.
func stub(list string) string {
	return `(() => {
	window.calls = [];
	const at = (d) => ` + "`2026-01-0${d}T00:00:00.000Z`" + `;
	window.list = JSON.stringify(` + list + `);
	window.fetch = (url, init = {}) => {
		if (!init.method) return Promise.resolve(new Response(window.list));
		if (window.failWrites) return Promise.reject(new TypeError('Failed to fetch'));
		calls.push([init.method, url, init.body].filter(Boolean).join(' '));
		if (init.method === 'POST') return Promise.resolve(new Response('{"slug":"n"}', { status: 201 }));
		return Promise.resolve(new Response(null, { status: 204 }));
	};
	navigator.clipboard.writeText = (text) => {
		calls.push(` + "`copy ${text}`" + `);
		return Promise.resolve();
	};
	return load();
})()`
}

// manyPages is a stub of n pages, newest first, alike but for their slug and title.
func manyPages(n int) string {
	return stub("Array.from({ length: " + strconv.Itoa(n) + " }, (_, i) => ({ slug: `p${i}`, title: `P${i}`, description: '', createdAt: at(1), updatedAt: at(1) }))")
}

// TestUIExtras sorts, filters favourites and the archived, counts each list, moves by keyboard, deletes and
// archives in bulk, edits in a modal, copies a path, adds a link only to the list it goes on, clears an error once a
// change succeeds, tells an empty list, and steps through neighbours, against a stubbed list.
func TestUIExtras(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	inEveryLanguage(t, uiExtras)
}

func uiExtras(t *testing.T, lang i18n.Lang, msg func(string, ...any) string) {
	t.Helper()

	b := newBrowser(t, servetest.Start(t, nil).Base, lang)
	b.navigate("/")
	b.eval(stub(extrasList))
	const titles = "[...document.querySelectorAll('#list .title')].map((a) => a.textContent).join(',')"
	// What the list counts, and the archived in the URL: the kinds, the project chips and whether they show, the views,
	// the count and the titles.
	const counts = "[...document.querySelectorAll('.tabs .n')].map((n) => n.textContent)" +
		".concat([...document.querySelectorAll('#chips .chip')].map((c) => c.textContent), document.getElementById('shelf-projects').hidden, " +
		"[...document.querySelectorAll('.views .n')].map((n) => n.textContent).join(' '), document.getElementById('count').textContent, " + titles + ", " +
		"new URLSearchParams(location.search).get('archived')).join(' ')"
	// Which empty states show, and their words.
	const empties = "['empty', 'empty-links', 'all-archived', 'empty-favorites', 'empty-archived', 'nomatch']" +
		".filter((id) => !document.getElementById(id).hidden).map((id) => id + ':' + document.getElementById(id).querySelector('strong').textContent).join(' ')"
	sortBy := func(v string) {
		b.eval("(() => { const s = document.getElementById('sort'); s.value = '" + v + "'; s.dispatchEvent(new Event('change')); })()")
	}
	key := func(k string) {
		b.eval("document.body.dispatchEvent(new KeyboardEvent('keydown', { key: '" + k + "', bubbles: true }))")
	}

	out := make([]string, 0, 32)
	out = append(out, b.eval(titles))
	sortBy("created")
	// Sorted by creation too, the time is the update's: the creation date has its own place beside it.
	out = append(out, b.eval(titles)+" "+b.eval("new URLSearchParams(location.search).get('sort')")+" "+
		b.eval("[...document.querySelectorAll('#list li:not(.group)')].every((li) => li.querySelector('time').dateTime === items.find((a) => a.slug === li.dataset.slug).updatedAt)"))
	sortBy("title")
	out = append(out, b.eval(titles))
	// The shelf's favourites narrow to them (kept in the URL); a row's star sends the change.
	b.eval("document.querySelector('.views [data-view=fav]').click()")
	out = append(out, b.eval(titles)+" "+b.eval("new URLSearchParams(location.search).get('fav')"))
	b.eval("document.querySelector('.views [data-view=all]').click()")
	b.eval("document.querySelector('#list li:not(.group) .star').click()")
	out = append(out, b.until("the favourite", "calls.splice(0).join(',')"))
	// The time's tooltip has both the creation and the update.
	out = append(out, b.eval("document.querySelector('#list li:not(.group) time').title.split('\\n').map((l) => l.split(' ')[0]).join(' ')"))
	// ↓ moves through the rows by focusing their titles, x selects the focused row.
	key("ArrowDown")
	key("ArrowDown")
	out = append(out, b.eval("document.activeElement.textContent"))
	key("x")
	out = append(out, b.eval("document.getElementById('bulk-count').textContent"))
	// With Alpha selected too by a real mouse click (nothing may sit on top of the checkbox), the first
	// click arms and the second deletes both.
	r := b.rect("document.querySelector('#list li:not(.group) .sel')")
	b.click(r.X+r.Width/2, r.Y+r.Height/2)
	out = append(out, b.until("the mouse click to select",
		"document.querySelector('#list li:not(.group) .sel').checked && document.getElementById('bulk-count').textContent"))
	b.eval("document.getElementById('bulk-delete').click()")
	out = append(out, b.eval("document.getElementById('bulk-delete').textContent"))
	b.eval("document.getElementById('bulk-delete').click()")
	b.until("the selection to clear", "document.getElementById('bulk').hidden")
	out = append(out, b.eval("calls.splice(0).sort().join(',')"))
	// The pencil opens a modal with the current description; saving sends the new one.
	b.eval("document.querySelector('#list li[data-slug=a] .edit').click()")
	out = append(out, b.eval("[document.getElementById('edit-dialog').open, document.getElementById('edit-desc').value].join(' ')"))
	b.eval("document.getElementById('edit-desc').value = 'new'; document.getElementById('edit').requestSubmit()")
	b.until("the edit modal to close", "!document.getElementById('edit-dialog').open")
	out = append(out, b.eval("calls.splice(0).join(',')"),
		b.eval("document.getElementById('toasts').textContent.includes("+js(msg("ui_saved"))+")"))
	// Only linked files have a copy button for their path.
	out = append(out, b.eval("document.querySelector('#list li:not(.group) .copy').hidden"))
	b.eval("document.querySelector('.tabs [data-tab=links]').click()")
	b.eval("document.querySelector('#list li .copy').click()")
	out = append(out, b.until("the copy", "calls.join(',')"),
		b.eval("document.getElementById('toasts').textContent.includes("+js(msg("ui_path_copied"))+")"))
	// A link whose file is gone has no href, but ↓ still reaches it.
	b.eval("document.activeElement.blur()")
	key("ArrowDown")
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
	b.eval("document.getElementById('add-cancel').click()")
	b.eval("document.querySelector('.tabs [data-tab=pages]').click()")

	// The archived are off every count and list but their own, which the switch shows (kept in the URL); a row's
	// menu and the bulk bar send the change, to archive or to give back, and say which; the menu closes.
	archiveWords := "(() => { const a = document.querySelector('#list li:not(.group) .archive'); document.getElementById('bulk-page').click(); " +
		"const words = [a.textContent.trim(), a.getAttribute('aria-label'), document.getElementById('bulk-archive').textContent]; " +
		"document.getElementById('bulk-clear').click(); return words.join(' / '); })()"
	const archiveRow = "document.querySelector('#list li:not(.group) .more summary').click(); document.querySelector('#list li:not(.group) .archive').click()"
	out = append(out, b.eval(counts), b.eval(archiveWords))
	b.eval(archiveRow)
	out = append(out, b.until("the archive", "calls.splice(0).join(',')"), b.eval("document.querySelectorAll('#list .more[open]').length"),
		b.eval("document.getElementById('toasts').textContent.includes("+js(msg("ui_archive_done", "Gamma"))+")"))
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
	// Empty lists: no archived link; all pages archived, which leads to them; nothing at all.
	b.eval("document.querySelector('.tabs [data-tab=links]').click()")
	b.eval(stub(`[{ slug: 'l', title: 'Linked', description: '', path: '/tmp/x/l.html', createdAt: at(1), updatedAt: at(1) }]`))
	// A link is added from the list it goes on, not from the archived.
	out = append(out, b.eval(empties)+" "+b.eval("document.getElementById('add-open').hidden"))
	b.eval("document.querySelector('.views [data-view=all]').click()")
	out = append(out, b.eval("document.getElementById('add-open').hidden"))
	// A change that succeeds clears the error of one that failed before, a favourite's as an archive's.
	b.eval("window.failWrites = true; document.querySelector('#list li:not(.group) .star').click()")
	out = append(out, b.until("the favourite to fail", "!document.getElementById('error').hidden && document.getElementById('error').textContent"))
	b.eval("window.failWrites = false; document.querySelector('#list li:not(.group) .star').click()")
	out = append(out, b.until("the error to clear", "document.getElementById('error').hidden"))
	b.eval("document.querySelector('.tabs [data-tab=pages]').click()")
	b.eval(stub(`[{ slug: 'z', title: 'Zeta', description: '', createdAt: at(2), updatedAt: at(6), archived: true }]`))
	out = append(out, b.eval(empties))
	b.eval("document.getElementById('show-archived').click()")
	out = append(out, b.eval(titles+" + ' ' + new URLSearchParams(location.search).get('archived')"))
	b.eval("document.querySelector('.views [data-view=all]').click()")
	b.eval(stub("[]"))
	out = append(out, b.eval(empties))

	// A page opened from the list steps to its neighbours in that list's order, and the logo returns to
	// that list. An archived page opened from elsewhere steps through the archived.
	b.eval("sessionStorage.setItem('artifacts.list', '?sort=title')")
	neighbours := "document.getElementById('frame-pos').textContent && " +
		"['frame-prev', 'frame-next'].map((id) => document.getElementById(id).getAttribute('href'))" +
		".concat(document.getElementById('frame-pos').textContent, document.querySelector('.brand').getAttribute('href')).join(' ')"
	b.navigate("/a/b/")
	b.eval(stub(extrasList))
	out = append(out, b.until("the neighbours", neighbours))
	b.navigate("/a/z/")
	b.eval(stub(extrasList))
	out = append(out, b.until("the archived neighbours", neighbours))

	// The time's tooltip lines start with the label of their format ("Created %s").
	label := func(key string) string { return strings.Fields(msg(key, ""))[0] }
	check(t, "extras", out, strings.Join([]string{
		"Gamma,Beta,Alpha", "Gamma,Beta,Alpha created true", "Gamma,Alpha,Beta", "Gamma 1",
		`PATCH /api/artifacts/c {"favorite":false}`, label("ui_created_at") + " " + label("ui_updated_at"), "Alpha",
		msg("ui_selected", 1), msg("ui_selected", 2), msg("ui_delete_bulk_confirm", 2),
		"DELETE /api/artifacts/a,DELETE /api/artifacts/c", "true old", `PATCH /api/artifacts/a {"description":"new"}`, "true", "true",
		"copy /tmp/x/l.html", "true", "Missing", `POST /api/artifacts {"path":"/tmp/x/n.html","description":"a link"}`,
		"3 2 " + msg("ui_no_project") + "3 true 3 1 2 " + msg("ui_count_all", 3) + " Gamma,Alpha,Beta ",
		msg("ui_archive") + " / " + msg("ui_archive_named", "Gamma") + " / " + msg("ui_archive"),
		`PATCH /api/artifacts/c {"archived":true}`, "0", "true",
		// The archived, by project: p's Zeta, then Ypsilon of none; two projects, so the shelf shows them.
		"2 1 p1 " + msg("ui_no_project") + "1 false 3 1 2 " + msg("ui_count_all", 2) + " Zeta,Ypsilon 1",
		msg("ui_unarchive") + " / " + msg("ui_unarchive_named", "Zeta") + " / " + msg("ui_unarchive"),
		`PATCH /api/artifacts/z {"archived":false}`, "true", msg("ui_unarchive"),
		`PATCH /api/artifacts/y {"archived":false},PATCH /api/artifacts/z {"archived":false}`,
		"empty-archived:" + msg("ui_links_none_archived") + " true", "false",
		msg("ui_favorite_failed", "Failed to fetch"), "true",
		"all-archived:" + msg("ui_pages_all_archived", 1),
		"Zeta 1",
		"empty:" + msg("ui_empty_pages"),
		"/a/a/  3 / 3 /?sort=title",
		" /a/y/ 1 / 2 /?sort=title",
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

// TestUIShelf picks the list on the shelf, heads the favourites, shows a row's actions where pointed at or reached,
// keeps a row's menu whole and gives focus back from it, adds a link from any list, folds the shelf into a select on
// a narrow screen, and acts from the frame's bar, against a stubbed list.
func TestUIShelf(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	inEveryLanguage(t, uiShelf)
}

func uiShelf(t *testing.T, lang i18n.Lang, msg func(string, ...any) string) {
	t.Helper()

	b := newBrowser(t, servetest.Start(t, nil).Base, lang)
	b.navigate("/")
	if b.eval("matchMedia('(hover: hover)').matches") != "true" {
		t.Fatal("Chrome does not take the screen for one with a mouse: the flag blink-settings of startChrome no longer works")
	}
	b.point(0, 0)
	b.eval(stub(extrasList))
	key := func(k string) {
		b.eval("document.body.dispatchEvent(new KeyboardEvent('keydown', { key: '" + k + "', bubbles: true }))")
	}
	const order = "[...document.getElementById('list').children].map((li) => li.classList.contains('group') ? 'H' : li.dataset.slug).join('')"
	const views = "[...document.querySelectorAll('.views button')].filter((b) => b.getAttribute('aria-pressed') === 'true').map((b) => b.dataset.view).join('') + ' ' + location.search"
	opacity := func(slug, part string) string {
		return b.eval("getComputedStyle(document.querySelector('#list li[data-slug=" + slug + "] " + part + "')).opacity")
	}

	out := make([]string, 0, 40)
	// All of the list heads its favourites and its projects, with how many each has; a heading is no row for a screen reader.
	out = append(out, b.eval(order), b.eval("[...document.querySelectorAll('#list li.group')].map((li) => "+
		"[li.getAttribute('role'), li.firstChild.getAttribute('role'), li.firstChild.getAttribute('aria-level'), li.textContent].join(':')).join(',')"))
	b.eval("document.querySelector('.views [data-view=fav]').click()")
	out = append(out, b.eval(order)+" "+b.eval(views))
	b.eval("document.querySelector('.views [data-view=archived]').click()")
	out = append(out, b.eval(order)+" "+b.eval(views))
	b.eval("document.querySelector('.views [data-view=all]').click()")
	// ↓ goes past a heading; selecting the page selects its rows only.
	key("ArrowDown")
	key("ArrowDown")
	out = append(out, b.eval("document.activeElement.textContent"))
	b.eval("document.getElementById('bulk-page').click()")
	out = append(out, b.eval("document.getElementById('bulk-count').textContent"))
	b.eval("document.getElementById('bulk-clear').click(); document.activeElement.blur()")
	// With a mouse, a row's actions show only where pointed at, reached by keyboard or selected; a pressed star always.
	out = append(out, opacity("a", ".star")+opacity("a", ".more")+opacity("c", ".star")+opacity("c", ".more"))
	r := b.rect("document.querySelector('#list li[data-slug=a] .body')")
	b.point(r.X+r.Width/2, r.Y+r.Height/2)
	out = append(out, b.until("the pointed row's actions", "getComputedStyle(document.querySelector('#list li[data-slug=a] .more')).opacity === '1' && 'shown'"))
	b.point(0, 0)
	b.until("the actions to hide", "getComputedStyle(document.querySelector('#list li[data-slug=a] .more')).opacity === '0'")
	b.eval("document.querySelector('#list li[data-slug=b] .title').focus()")
	out = append(out, b.until("the reached row's actions", "getComputedStyle(document.querySelector('#list li[data-slug=b] .more')).opacity === '1' && 'shown'"))
	b.eval("document.activeElement.blur(); document.querySelector('#list li[data-slug=a] .sel').click()")
	out = append(out, b.until("the selected row's actions", "getComputedStyle(document.querySelector('#list li[data-slug=a] .more')).opacity === '1' && 'shown'"))
	b.eval("document.getElementById('bulk-clear').click()")
	out = append(out, b.eval("[...document.querySelectorAll('#list li:not(.group)')].filter((li) => "+
		"['.star', '.archive', '.more summary'].some((s) => !li.querySelector(s).getAttribute('aria-label'))).length"))
	// The menu: its words, Escape and a click elsewhere close it, focus going back to its button.
	b.eval("document.querySelector('#list li[data-slug=b] summary').click()")
	out = append(out, b.eval("document.querySelector('#list li[data-slug=b] summary').getAttribute('aria-label')"),
		b.eval("[...document.querySelectorAll('#list li[data-slug=b] .menu > *')].filter((e) => !e.hidden).map((e) => e.textContent.trim()).join(' / ')"))
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
	out = append(out, b.eval("[...document.querySelectorAll('#list .more')].map((d) => d.open).join(' ')"))
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
	b.eval(stub(`[{ slug: 'a', title: 'Alpha', description: '', createdAt: at(1), updatedAt: at(8) }]`))
	// A window tall enough for the menu below the one row, whatever the test Chrome's own window is (CI's is shorter): the
	// width of that window, as the rest of the test sees it.
	b.size(780, 800, false)
	// Not scrollIntoView, which would scroll a box that cuts it off to bring it in.
	b.eval("document.querySelector('#list > li:last-child summary').click()")
	d := b.rect("document.querySelector('#list > li:last-child .delete')")
	out = append(out, b.eval("Boolean(document.elementFromPoint("+js(fmt.Sprint(d.X+d.Width/2))+", "+js(fmt.Sprint(d.Y+d.Height/2))+")?.closest('.delete'))"))
	b.eval("calls.length = 0; document.querySelector('#list > li:last-child .delete').click(); document.querySelector('#list > li:last-child .delete').click()")
	out = append(out, b.until("the delete", "calls.join(',')"))
	b.eval("scrollTo(0, 0)")
	// Only favourites: their project's heading, in the favourites' list as in all of it.
	b.eval(stub(`[{ slug: 'c', title: 'Gamma', description: '', createdAt: at(1), updatedAt: at(9), favorite: true },
		{ slug: 'd', title: 'Delta', description: '', createdAt: at(1), updatedAt: at(8), favorite: true }]`))
	out = append(out, b.eval(order))
	// The creation date beside the time, left of it, of a link when registered, and none without one; nothing under the title for it.
	date := b.eval("new Date('2026-01-01T00:00:00.000Z').toLocaleDateString(document.documentElement.lang, { dateStyle: 'medium' })")
	b.eval(stub(`[{ slug: 'a', title: 'Alpha', description: '', createdAt: at(1), updatedAt: at(8) },
		{ slug: 'o', title: 'Old', description: '', updatedAt: at(7) },
		{ slug: 'l', title: 'Linked', description: '', path: '/tmp/x/l.html', createdAt: at(1), updatedAt: at(1) }]`))
	out = append(out, b.eval("(() => { const li = document.querySelector('#list li[data-slug=a]'), m = li.querySelector('.made'); "+
		"return [m.textContent, m.parentElement.classList.contains('meta'), m.getBoundingClientRect().right <= li.querySelector('time').getBoundingClientRect().left, li.querySelector('.sub').hidden].join(' '); })()"),
		b.eval("document.querySelector('#list li[data-slug=o] .made').hidden"))
	b.eval("document.querySelector('.tabs [data-tab=links]').click()")
	out = append(out, b.eval("document.querySelector('#list li[data-slug=l] .made').textContent"))
	b.eval("document.querySelector('.tabs [data-tab=pages]').click()")
	// The kinds show with a link, the projects with two or one chosen.
	b.eval(stub(`[{ slug: 'a', title: 'Alpha', description: '', project: 'p', createdAt: at(1), updatedAt: at(8) }]`))
	out = append(out, b.eval("[document.getElementById('kinds').hidden, document.getElementById('shelf-projects').hidden].join(' ')"))
	b.eval("setProject('p')")
	out = append(out, b.eval("document.getElementById('shelf-projects').hidden"))
	b.eval("setProject('')")
	// A project's dot is of its color by projectOrder: as many each, by name.
	b.eval(stub(`[{ slug: 'a', title: 'Alpha', description: '', project: 'p', createdAt: at(1), updatedAt: at(8) },
		{ slug: 'b', title: 'Beta', description: '', project: 'floe', createdAt: at(1), updatedAt: at(7) }]`))
	out = append(out, b.eval("[...document.querySelectorAll('#chips .chip')].map((c) => c.dataset.project + ':' + c.style.getPropertyValue('--c')).join(',')"))
	// The favourites count their own: their count, their projects, and none shown when there are none.
	b.eval(stub(`[{ slug: 'c', title: 'Gamma', description: '', project: 'p', createdAt: at(1), updatedAt: at(9), favorite: true },
		{ slug: 'b', title: 'Beta', description: '', project: 'q', createdAt: at(1), updatedAt: at(8) }]`))
	b.eval("document.querySelector('.views [data-view=fav]').click()")
	out = append(out, b.eval("[document.getElementById('count').textContent, document.getElementById('shelf-projects').hidden].join(' ')"))
	const empties = "['empty', 'empty-links', 'all-archived', 'empty-favorites', 'empty-archived', 'nomatch']" +
		".filter((id) => !document.getElementById(id).hidden).join(' ')"
	b.eval(stub(`[{ slug: 'z', title: 'Zeta', description: '', createdAt: at(1), updatedAt: at(6), favorite: true, archived: true }]`))
	out = append(out, b.eval(empties))
	b.eval("document.querySelector('.views [data-view=all]').click()")
	out = append(out, b.eval(empties))
	b.eval("document.getElementById('show-archived').click()")
	out = append(out, b.eval(views))
	b.navigate("/?fav=1&archived=1")
	b.eval(stub(extrasList))
	out = append(out, b.eval(views))
	// A link is added from any list but the archived, the first one too, and then shown on the links' list, unfiltered by a
	// project or a search it may not match, drawn once.
	b.eval("document.querySelector('.views [data-view=all]').click()")
	b.eval(stub(`[{ slug: 'a', title: 'Alpha', description: '', project: 'p', createdAt: at(1), updatedAt: at(8) }]`))
	out = append(out, b.eval("[document.getElementById('kinds').hidden, document.getElementById('add-open').hidden].join(' ')"))
	b.eval("setProject('p'); (() => { const q = document.getElementById('q'); q.value = 'Al'; q.dispatchEvent(new Event('input')); })()")
	b.eval(`(() => {
		window.renders = 0;
		window.draw = render;
		render = () => { renders++; draw(); };
		// The server lists the link added.
		const post = window.fetch;
		window.fetch = (url, init = {}) => {
			if (init.method === 'POST') list = JSON.stringify([...JSON.parse(list), { slug: 'n', title: 'New', description: '', path: '/tmp/x/n.html', createdAt: '2026-01-01T00:00:00.000Z', updatedAt: '2026-01-09T00:00:00.000Z' }]);
			return post(url, init);
		};
		document.getElementById('add-open').click();
		document.getElementById('add-path').value = '/tmp/x/n.html';
		document.getElementById('add').requestSubmit();
	})()`)
	// Waited for until drawn: the dialog closes before the list is fetched again.
	b.until("the link to be added", "!document.getElementById('add-dialog').open && location.search.includes('tab=links')")
	out = append(out, b.eval("[location.search, document.getElementById('q').value === '', renders, "+
		"[...document.querySelectorAll('#list .title')].map((a) => a.textContent)].join(' ')"))
	b.eval("render = draw")
	b.eval("document.querySelector('.views [data-view=archived]').click()")
	out = append(out, b.eval("document.getElementById('add-open').hidden"))
	b.eval("document.querySelector('.views [data-view=all]').click(); document.querySelector('.tabs [data-tab=pages]').click()")
	// 50 rows a page unless the viewer chose otherwise.
	b.eval(manyPages(60))
	out = append(out, b.eval("document.querySelectorAll('#list li:not(.group)').length + ' ' + document.getElementById('pageno').textContent"))
	// An open menu passes under the bars stuck at the top as the page scrolls, not over them.
	b.eval("document.querySelector('#list li[data-slug=p5] summary').click()")
	out = append(out, b.eval(`(() => {
		const bar = document.querySelector('.toolbar').getBoundingClientRect();
		scrollBy(0, document.querySelector('#list li[data-slug=p5] .menu').getBoundingClientRect().top - bar.top - 5);
		const menu = document.querySelector('#list li[data-slug=p5] .menu').getBoundingClientRect();
		return Boolean(document.elementFromPoint(menu.left + menu.width / 2, bar.top + 10)?.closest('.toolbar'));
	})()`))
	b.eval("document.querySelector('#list li[data-slug=p5] summary').click(); scrollTo(0, 0)")
	// A narrow screen: the shelf is one row of its buttons, scrolled sideways rather than wrapped, its h2 and add link out of
	// sight; a view and a project of that name are told apart, pressed as the wide shelf's are.
	b.size(390, 844, true)
	b.eval(stub(`[{ slug: 'a', title: 'Alpha', description: '', project: 'archived', createdAt: at(1), updatedAt: at(8) },
		{ slug: 'b', title: 'Beta', description: '', project: 'p', createdAt: at(1), updatedAt: at(7) },
		{ slug: 'c', title: 'Gamma', description: '', project: 'a<b&"c', createdAt: at(1), updatedAt: at(6) }]`))
	out = append(out, b.eval("(() => { const s = document.querySelector('.shelf'); return [getComputedStyle(s).display, getComputedStyle(s).position, "+
		"s.scrollWidth > s.clientWidth, document.documentElement.scrollWidth === document.documentElement.clientWidth].join(' '); })()"),
		b.eval("[...document.querySelectorAll('.shelf button, .shelf h2')].filter((e) => e.offsetParent).map((e) => e.dataset.view ?? e.dataset.project ?? e.tagName).join(',')"))
	press := func(value string) {
		b.eval("[...document.querySelectorAll('#chips .chip')].find((c) => c.dataset.project === " + js(value) + ").click()")
	}
	press(`a<b&"c`)
	out = append(out, b.eval("new URLSearchParams(location.search).get('project')"))
	press("archived")
	out = append(out, b.eval(views))
	press("archived")
	out = append(out, b.eval("String(new URLSearchParams(location.search).get('project'))"))
	b.unsize()
	// The frame's bar: the title, archived or not, the star and the archive, which act as the list's do and tell their failure.
	// The archive, as the star, is an icon alone: pressed while archived, its name in its label and title, no words in it.
	frame := "[document.getElementById('frame-title').textContent, document.getElementById('frame-archived').hidden, " +
		"document.getElementById('frame-star').getAttribute('aria-pressed'), document.getElementById('frame-archive').getAttribute('aria-pressed'), " +
		"document.getElementById('frame-archive').getAttribute('aria-label'), document.getElementById('frame-archive').title, " +
		"JSON.stringify(document.getElementById('frame-archive').textContent.trim())].join(' / ')"
	b.navigate("/a/z/")
	b.eval(stub(extrasList))
	out = append(out, b.until("the frame's bar", "document.getElementById('frame-title').textContent && "+frame))
	b.eval("document.getElementById('frame-archive').click()")
	out = append(out, b.until("the give back", "calls.splice(0).join(',')"))
	b.eval("window.failWrites = true; document.getElementById('frame-star').click()")
	out = append(out, b.until("the failure to show", "!document.getElementById('error').hidden && document.getElementById('error').offsetHeight > 0 && "+
		"document.getElementById('error').textContent"))
	b.navigate("/a/b/")
	b.eval(stub(extrasList))
	out = append(out, b.until("the frame's bar", "document.getElementById('frame-title').textContent && "+frame))
	// The frame steps through the list it was opened from, which starring it from the bar does not change; a page gone drops out.
	neighbours := "document.getElementById('frame-pos').textContent && ['frame-prev', 'frame-next']" +
		".map((id) => document.getElementById(id).getAttribute('href')).concat(document.getElementById('frame-pos').textContent).join(' ')"
	b.eval("sessionStorage.setItem('artifacts.list', '?fav=1')")
	b.navigate("/a/c/")
	b.eval(stub(extrasList))
	out = append(out, b.until("the favourites' neighbours", neighbours))
	b.eval(stub(strings.Replace(extrasList, "updatedAt: at(7), favorite: true", "updatedAt: at(7)", 1)))
	out = append(out, b.eval("document.getElementById('frame-star').getAttribute('aria-pressed') + ' ' + ("+neighbours+")"))
	b.eval("sessionStorage.setItem('artifacts.list', '')")
	b.navigate("/a/b/")
	b.eval(stub(extrasList))
	b.until("the neighbours", neighbours)
	b.eval(stub(strings.Replace(extrasList, "{ slug: 'a', title: 'Alpha', description: 'old', createdAt: at(1), updatedAt: at(8) },", "", 1)))
	out = append(out, b.until("the gone page to drop out", "document.getElementById('frame-pos').textContent !== '2 / 3' && "+neighbours))

	check(t, "shelf", out, strings.Join([]string{
		"HcHba", "none:heading:2:" + msg("ui_group_favorites") + msg("ui_count_all", 1) + ",none:heading:2:" + msg("ui_no_project") + msg("ui_count_all", 2),
		"Hc fav ?fav=1", "HzHy archived ?archived=1",
		"Beta", msg("ui_selected", 3),
		"0010", "shown", "shown", "shown", "0",
		msg("ui_more_named", "Beta"), msg("ui_edit") + " / " + msg("ui_open_raw") + " / " + msg("ui_download") + " / " + msg("ui_archive") + " / " + msg("ui_delete"),
		"false true", "Alpha", "false true", "false false true", "false true", "true", "true",
		"true", "DELETE /api/artifacts/a", "Hcd",
		msg("ui_created_at", date) + " true true true", "true", msg("ui_registered_at", date),
		"true true", "false", "floe:var(--pc0),p:var(--pc1)",
		msg("ui_count_all", 1) + " true",
		"empty-favorites", "all-archived", "archived ?archived=1", "archived ?archived=1",
		"true false", "?tab=links true 1 New", "true",
		"50 1 / 2", "true",
		"flex static true true", `all,fav,archived,a<b&"c,archived,p`, `a<b&"c`,
		"all ?project=archived", "null",
		"Zeta / false / true / true / " + msg("ui_unarchive") + " / " + msg("ui_unarchive") + ` / ""`, `PATCH /api/artifacts/z {"archived":false}`,
		msg("ui_favorite_failed", "Failed to fetch"),
		"Beta / true / false / false / " + msg("ui_archive") + " / " + msg("ui_archive") + ` / ""`,
		"  1 / 1", "false   1 / 1", "/a/c/  2 / 2",
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
	b.navigate("/")
	b.eval(stub(tagsList))
	const rows = "[...document.querySelectorAll('#list li:not(.group)')].map((li) => li.dataset.slug).join('') + ' ' + (new URLSearchParams(location.search).get('tag') ?? '-')"
	const shelf = "[...document.querySelectorAll('#tags .chip')].map((c) => c.textContent + ':' + c.getAttribute('aria-pressed')).join(',') + ' ' + document.getElementById('shelf-tags').hidden"
	// Presses the shelf's tag plan, which picks it or lets it go.
	pickPlan := func() {
		b.eval("[...document.querySelectorAll('#tags .chip')].find((c) => c.dataset.tag === 'plan').click()")
	}
	search := func(q string) {
		b.eval("(() => { const q = document.getElementById('q'); q.value = " + js(q) + "; q.dispatchEvent(new Event('input')); })()")
	}
	// A key as a keyboard presses it: its text on the key down, which a page's preventDefault then stops too
	// (chromedp.KeyEvent sends the text as an event of its own, which nothing stops).
	press := func(k string) {
		t.Helper()

		for _, e := range kb.Encode([]rune(k)[0]) {
			if e.Type == kb.KeyChar {
				continue
			}
			if e.Type == kb.KeyDown {
				e.Text = map[string]string{kb.Enter: "\r", " ": " "}[k]
			}
			if _, err := chromedp.Call(b.ctx, input.DispatchKeyEvent, *e); err != nil {
				t.Fatal(err)
			}
		}
	}
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
		b.eval("document.querySelector('#list li[data-slug=d] .tags').hidden"))
	// Picked, the rows are its pages, as when the URL has it; picked again it is let go.
	pickPlan()
	out = append(out, b.eval(rows), b.eval(shelf))
	b.navigate("/?tag=plan")
	b.eval(stub(tagsList))
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
	b.eval(stub(strings.ReplaceAll(tagsList, "'plan'", "'other'")))
	out = append(out, b.eval(shelf), b.eval("document.getElementById('nomatch').hidden"))
	b.eval("document.getElementById('clear').click()")
	out = append(out, b.eval(rows))
	b.eval(stub(tagsList))

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
	b.eval(stub(tagsList))
	// Narrowed to a tag, the edit dialog's field holds it whole: pressed, the field offers the other names (reported by the user).
	pickPlan()
	b.eval("document.querySelector('#list li[data-slug=b] .edit').click()")
	// The test's window is low enough to hide the field under the dialog's fold.
	b.until("the edit dialog to stop moving", "document.getElementById('edit-dialog').getAnimations().length === 0")
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

	// A narrow screen's row of the shelf has the tags, which pick and let go as the wide shelf's do.
	b.size(390, 844, true)
	plan := "[...document.querySelectorAll('#tags .chip')].find((c) => c.dataset.tag === 'plan')"
	out = append(out, b.eval("[...document.querySelectorAll('#tags .chip')].filter((c) => c.offsetParent).map((c) => c.dataset.tag).join(',')"))
	b.eval(plan + ".click()")
	out = append(out, b.eval(rows), b.eval(plan+".getAttribute('aria-pressed')"))
	b.eval(plan + ".click()")
	out = append(out, b.eval(rows))
	b.unsize()

	// A tag's name goes in as text, wherever it shows, and counts as any.
	b.eval(stub(`[{ slug: 'a', title: 'Alpha', description: '', tags: ['<b>x</b>', '__proto__'], createdAt: at(1), updatedAt: at(9) }]`))
	out = append(out, b.eval("[...document.querySelectorAll('#tags .chip')].map((c) => c.textContent).sort().join(',')"),
		b.eval("[...document.querySelectorAll('#list .tagpill')].map((p) => p.textContent).join(',')"),
		b.eval("document.querySelectorAll('.shelf b, #list b').length"))
	selectOnly("a")
	b.eval("document.getElementById('bulk-tag').click()")
	out = append(out, b.eval(boxes)+" "+b.eval("document.querySelectorAll('#tag-boxes b').length"))
	b.eval("document.querySelector('#tag-dialog [data-close]').click(); document.getElementById('bulk-clear').click()")

	// The tags fields suggest the names of every page, as a combobox, with the keyboard alone or the mouse.
	b.eval(stub(`[{ slug: 'a', title: 'Alpha', description: '', tags: ['設計', 'plan'], createdAt: at(1), updatedAt: at(9) },
		{ slug: 'b', title: 'Beta', description: '', tags: ['計画', '<b>x</b>', 'xa'], createdAt: at(1), updatedAt: at(8), archived: true }]`))
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
	out = append(out, b.until("the names under the moving field", "document.getElementById('edit-dialog').getAnimations().length === 0 && (() => { "+
		"const f = document.getElementById('edit-tags').getBoundingClientRect(); const r = document.getElementById('edit-tags-names').getBoundingClientRect(); "+
		"return Math.abs(r.top - f.bottom - 4) < 0.5 && Math.abs(r.left - f.left) < 0.5 && 'under'; })()"))

	check(t, "tags", out, strings.Join([]string{
		"plan2:false,設計1:false,設計、実装1:false false", "abcd -",
		"設計:" + msg("ui_tag_filter_named", "設計") + ",plan:" + msg("ui_tag_filter_named", "plan"), "true",
		"ab plan", "plan2:true,設計1:false,設計、実装1:false false", "ab plan", "abcd -",
		"old1:false,plan1:false false", "c 設計、実装", msg("ui_selected", 1), "ac -",
		"other2:false,plan0:true,設計1:false,設計、実装1:false false", "false", "abcd -",
		"設計, plan", `PATCH /api/artifacts/a {"tags":["計画","設計"]}`, `PATCH /api/artifacts/c {"title":"G2"}`,
		msg("ui_tag_bulk_title", 3), "plan:some:−:2/3,設計:some:−:1/3,old:none:+:,設計、実装:none:+:", "false", "plan solid",
		"all:true none:true some:false all:true", "plan:all:✓:2/3,設計:none:+:1/3,old:all:✓:,設計、実装:none:+:", "true",
		`PATCH /api/artifacts/a {"tags":["plan","old"]},PATCH /api/artifacts/b {"tags":["plan","old"]},PATCH /api/artifacts/d {"tags":["old","plan"]}`, "true", "1",
		`PATCH /api/artifacts/a {"tags":["設計","plan","old"]},PATCH /api/artifacts/c {"tags":["設計、実装","old"]}`,
		"plan:all:✓: false", msg("ui_tag_create", "new"), "plan:all:✓:,new:all:✓:,old:none:+:,設計:none:+:,設計、実装:none:+:", "true 0 true", "false",
		"true 0",
		"old,設計,設計、実装", "plan, old, ", "設計,設計、実装",
		"plan,設計,設計、実装", "ab plan", "true", "abcd -",
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

// TestUIShelfStays keeps the shelf where it sits, whether the list is long enough to scroll or not, and there once
// scrolled: sticky only holds it, never pushes it.
func TestUIShelfStays(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.navigate("/")
	// The test Chrome's own window is near the width that hides the shelf. The size holds for this test's tab alone.
	b.size(1440, 900, false)
	// The shelf's top, then where it sits below what is over it: the shell's top, or the bar's bottom once scrolled,
	// and the shell's padding.
	place := func(over, edge string) string {
		return b.eval("(() => { const pad = parseFloat(getComputedStyle(document.querySelector('.shell')).paddingTop); " +
			"return document.querySelector('.shelf').getBoundingClientRect().top + ' ' + " +
			"(document.querySelector('" + over + "').getBoundingClientRect()." + edge + " + pad); })()")
	}

	out := make([]string, 0, 6)
	b.eval(manyPages(60))
	out = append(out, b.eval("document.documentElement.scrollHeight > innerHeight"), b.eval("document.querySelector('.shelf').getBoundingClientRect().height > 0"))
	long := place(".shell", "top")
	b.eval("scrollTo(0, 300)")
	b.until("the page to scroll", "scrollY === 300")
	scrolled := place(".bar", "bottom")
	b.eval("scrollTo(0, 0)")
	b.eval(manyPages(1))
	short := place(".shell", "top")
	// Each the shelf's top and where it sits, alike.
	for _, p := range []string{long, scrolled, short} {
		top, sits, _ := strings.Cut(p, " ")
		out = append(out, top, sits)
	}
	got := strings.Join(out, "|")
	want := strings.Join([]string{"true", "true", out[3], out[3], out[5], out[5], out[7], out[7]}, "|")
	check(t, "shelf stays", []string{got}, want)
}

// TestUIShelfScrolls holds the shelf to the window under it, scrolling in itself when taller, where it sits however far the
// page scrolls, its height kept with the bulk bar, with room for its focus rings, its long names on one line, cut off; and a shelf that fits passes the
// wheel to the page.
func TestUIShelfScrolls(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.navigate("/")
	// A real wheel over the shelf's middle.
	wheel := func() {
		t.Helper()

		r := b.rect("document.querySelector('.shelf')")
		// The protocol wants deltaX sent, which a 0 is left out as: a sliver of a pixel moves nothing sideways.
		e := input.DispatchMouseEventParams{Type: input.DispatchMouseEventTypeMouseWheel, X: r.X + r.Width/2, Y: r.Y + r.Height/2, DeltaX: 1e-6, DeltaY: 100}
		if _, err := chromedp.Call(b.ctx, input.DispatchMouseEvent, e); err != nil {
			t.Fatal(err)
		}
	}
	// Within half a pixel of each other, or how far apart.
	const near = "const near = (x, y, d = 0.5) => Math.abs(x - y) <= d || `${x} != ${y}`; " +
		"const shelf = document.querySelector('.shelf'); const r = shelf.getBoundingClientRect(); " +
		"const shell = parseFloat(getComputedStyle(document.querySelector('.shell')).paddingTop);"
	b.size(1440, 600, false)
	// Thirty projects and a tag of a long name with nowhere to break, on a list long enough to scroll.
	b.eval(stub("Array.from({ length: 60 }, (_, i) => ({ slug: `p${i}`, title: `P${i}`, description: '', project: `project${i % 30}`, " +
		"tags: i ? [] : ['averylongtagnamewithoutanyspacesorhyphens'], createdAt: at(1), updatedAt: at(1) }))"))
	out := make([]string, 0, 10)
	// Where #226 put it, down to the window's bottom less the room above it, scrolling in itself.
	edges := b.eval("(() => { " + near + " return [r.top, r.bottom].join(' '); })()")
	out = append(out, b.eval("(() => { "+near+" return [near(r.top, document.querySelector('.shell').getBoundingClientRect().top + shell), "+
		"near(r.bottom, innerHeight - shell), shelf.scrollHeight > shelf.clientHeight, document.documentElement.scrollHeight > innerHeight].join(' '); })()"))
	// Scrolled to the page's end, the shelf is not pushed up.
	b.eval("scrollTo(0, document.documentElement.scrollHeight)")
	b.until("the page to scroll to its end", "Math.abs(scrollY + innerHeight - document.documentElement.scrollHeight) < 1")
	out = append(out, b.eval("(() => { "+near+" const [top, bottom] = "+js(edges)+".split(' ').map(Number); return [near(r.top, top, 1), near(r.bottom, bottom, 1)].join(' '); })()"))
	b.eval("scrollTo(0, 0)")
	b.until("the page to scroll back", "scrollY === 0")
	// The wheel scrolls the shelf, not the page; at its end, its last item shows in the window, its focus ring room below (less
	// what the shelf's whole-pixel scroll leaves of a fraction).
	wheel()
	out = append(out, b.until("the shelf to scroll", "document.querySelector('.shelf').scrollTop > 0 && `page at ${scrollY}`"))
	b.eval("document.querySelector('.shelf').scrollTop = 1e6")
	out = append(out, b.eval("(() => { "+near+" const a = document.getElementById('add-open').getBoundingClientRect(); "+
		"return [a.bottom <= innerHeight - shell, r.bottom - a.bottom >= 3.5].join(' '); })()"))
	// The focus ring's room each side, and a long name on one line, cut off, not scrolling sideways.
	out = append(out, b.eval("(() => { "+near+" const a = document.querySelector('.views button').getBoundingClientRect(); "+
		"const long = document.querySelector('#tags .chip .name'); "+
		"return [a.left - r.left >= 4, r.left + shelf.clientWidth - a.right >= 4, shelf.scrollWidth === shelf.clientWidth, "+
		"long.scrollWidth > long.clientWidth && long.getClientRects().length === 1].join(' '); })()"))
	// Pointed at, the shelf is laid out again (its right room 4.01px) for Chrome to draw its thumb; nothing in it moves.
	hover := "document.querySelector('.views button').getBoundingClientRect().right + ' ' + getComputedStyle(document.querySelector('.shelf')).paddingRight"
	b.point(800, 300)
	away := b.eval(hover)
	sr := b.rect("document.querySelector('.shelf')")
	b.point(sr.X+sr.Width/2, sr.Y+sr.Height/2)
	on := b.until("the shelf to be pointed at", "document.querySelector('.shelf:hover') && ("+hover+")")
	b.point(800, 300)
	out = append(out, away+" / "+on+" / "+b.until("the shelf to be left", "!document.querySelector('.shelf:hover') && ("+hover+")"))
	// With the bulk bar, the shelf ends above it; in a window too low for that, it keeps 10rem, reaching below.
	// With the bulk bar, in the bar over the list, the shelf keeps its height.
	tall := b.eval("document.querySelector('.shelf').getBoundingClientRect().height")
	b.eval("document.querySelector('#list li:not(.group) .sel').click()")
	out = append(out, b.until("the bar to show", "(() => { const bar = document.getElementById('bulk').getBoundingClientRect(); "+
		"return bar.height > 0 && `${document.getElementById('bulk').closest('.toolbar') !== null} ${document.querySelector('.shelf').getBoundingClientRect().height === "+tall+"}`; })()"))
	b.size(1440, 240, false)
	out = append(out, b.eval("Math.abs(document.querySelector('.shelf').getBoundingClientRect().height - 10 * parseFloat(getComputedStyle(document.documentElement).fontSize)) <= 0.5"))
	b.size(1440, 600, false)
	b.eval("document.getElementById('bulk-clear').click()")
	// A shelf that fits does not scroll, and passes the wheel to the page.
	b.size(1440, 900, false)
	b.eval(manyPages(60))
	out = append(out, b.eval("(() => { "+near+" return [shelf.scrollHeight === shelf.clientHeight, document.documentElement.scrollHeight > innerHeight].join(' '); })()"))
	wheel()
	out = append(out, b.until("the page to scroll", "scrollY > 0 && 'scrolled'"))
	check(t, "shelf scrolls", out, strings.Join([]string{
		"true true true true", "true true", "page at 0", "true true", "true true true true", "238 4px / 238 4.01px / 238 4px", "true true", "true", "true true", "scrolled",
	}, "|"))
}

// TestUIScrollbar keeps the list where it is whether or not it scrolls, the scrollbar's room kept on the right alone,
// and the frame, a page's or a missing one's, without that room, as the server draws it so from the start.
func TestUIScrollbar(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.page(t, "p.yaml", "P", "<p>p</p>"), "--slug", "p")
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
	// What the body is narrower than the window: the room kept for the scrollbar. Not innerWidth - clientWidth, which
	// counts the room only while the scrollbar is there.
	const room = "innerWidth - document.body.getBoundingClientRect().width"
	// The shelf's first button, not its box, which reaches 4px out for its focus rings.
	const where = "['.shelf .views button', '.toolbar'].map((s) => document.querySelector(s).getBoundingClientRect().left)" +
		".concat(document.querySelector('.panel').getBoundingClientRect().right).join(' ')"
	out := make([]string, 0, 8)
	b.size(1800, 900, false)
	b.eval(manyPages(60))
	out = append(out, b.eval("document.documentElement.scrollHeight > innerHeight"))
	long := b.eval(where)
	b.eval(manyPages(1))
	out = append(out, b.eval("document.documentElement.scrollHeight > innerHeight"), b.eval(where+" === "+js(long)), b.eval(room))
	// The room is on the right alone.
	b.size(1200, 900, false)
	b.eval(manyPages(1))
	out = append(out, b.eval(where))
	for _, path := range []string{"/a/p/", "/a/none/"} {
		b.navigate(path)
		out = append(out, b.eval(room))
	}
	// The frame makes its URLs of the slug escaped again: one read from /a/x%3Fy/ is x?y.
	b.navigate("/a/x%3Fy/")
	out = append(out, b.eval("document.getElementById('frame-download').getAttribute('href')"))
	// The scrollbar is the management page's own, 10px wide.
	check(t, "scrollbar", out, strings.Join([]string{"true", "false", "true", "10", fmt.Sprintf("16 272 %d", 1200-16-width), "0", "0", "/api/artifacts/x%3Fy/download"}, "|"))
}
