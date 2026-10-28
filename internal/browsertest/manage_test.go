package browsertest_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"

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
	out = append(out, b.eval(titles)+" "+b.eval("new URLSearchParams(location.search).get('sort')"))
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
	// button and the bulk bar send the change, to archive or to give back, and say which.
	archiveWords := "(() => { const a = document.querySelector('#list li:not(.group) .archive'); document.getElementById('bulk-page').click(); " +
		"const words = [a.title, a.getAttribute('aria-label'), document.getElementById('bulk-archive').textContent]; " +
		"document.getElementById('bulk-clear').click(); return words.join(' / '); })()"
	out = append(out, b.eval(counts), b.eval(archiveWords))
	b.eval("document.querySelector('#list li:not(.group) .archive').click()")
	out = append(out, b.until("the archive", "calls.splice(0).join(',')"),
		b.eval("document.getElementById('toasts').textContent.includes("+js(msg("ui_archive_done", "Gamma"))+")"))
	b.eval("document.querySelector('.views [data-view=archived]').click()")
	out = append(out, b.eval(counts), b.eval(archiveWords))
	b.eval("document.querySelector('#list li:not(.group) .archive').click()")
	out = append(out, b.until("the give back", "calls.splice(0).join(',')"))
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
		"Gamma,Beta,Alpha", "Gamma,Beta,Alpha created", "Gamma,Alpha,Beta", "Gamma 1",
		`PATCH /api/artifacts/c {"favorite":false}`, label("ui_created_at") + " " + label("ui_updated_at"), "Alpha",
		msg("ui_selected", 1), msg("ui_selected", 2), msg("ui_delete_bulk_confirm", 2),
		"DELETE /api/artifacts/a,DELETE /api/artifacts/c", "true old", `PATCH /api/artifacts/a {"description":"new"}`, "true", "true",
		"copy /tmp/x/l.html", "true", "Missing", `POST /api/artifacts {"path":"/tmp/x/n.html","description":"a link"}`,
		"3 2 " + msg("ui_all_projects") + "3 " + msg("ui_no_project") + "3 true 3 1 2 " + msg("ui_count_all", 3) + " Gamma,Alpha,Beta ",
		msg("ui_archive") + " / " + msg("ui_archive_named", "Gamma") + " / " + msg("ui_archive"),
		`PATCH /api/artifacts/c {"archived":true}`, "true",
		"2 1 " + msg("ui_all_projects") + "2 p1 " + msg("ui_no_project") + "1 false 3 1 2 " + msg("ui_count_all", 2) + " Ypsilon,Zeta 1",
		msg("ui_unarchive") + " / " + msg("ui_unarchive_named", "Ypsilon") + " / " + msg("ui_unarchive"),
		`PATCH /api/artifacts/y {"archived":false}`, msg("ui_unarchive"),
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
	// All of the list heads its favourites and the others; a heading is no row for a screen reader.
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
	out = append(out, opacity("a", ".star")+opacity("a", ".archive")+opacity("a", ".more")+opacity("c", ".star")+opacity("c", ".archive"))
	r := b.rect("document.querySelector('#list li[data-slug=a] .body')")
	b.point(r.X+r.Width/2, r.Y+r.Height/2)
	out = append(out, b.until("the pointed row's actions", "getComputedStyle(document.querySelector('#list li[data-slug=a] .archive')).opacity === '1' && 'shown'"))
	b.point(0, 0)
	b.until("the actions to hide", "getComputedStyle(document.querySelector('#list li[data-slug=a] .archive')).opacity === '0'")
	b.eval("document.querySelector('#list li[data-slug=b] .title').focus()")
	out = append(out, b.until("the reached row's actions", "getComputedStyle(document.querySelector('#list li[data-slug=b] .archive')).opacity === '1' && 'shown'"))
	b.eval("document.activeElement.blur(); document.querySelector('#list li[data-slug=a] .sel').click()")
	out = append(out, b.until("the selected row's actions", "getComputedStyle(document.querySelector('#list li[data-slug=a] .archive')).opacity === '1' && 'shown'"))
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
	// Not scrollIntoView, which would scroll a box that cuts it off to bring it in.
	b.eval("document.querySelector('#list > li:last-child summary').click()")
	d := b.rect("document.querySelector('#list > li:last-child .delete')")
	out = append(out, b.eval("Boolean(document.elementFromPoint("+js(fmt.Sprint(d.X+d.Width/2))+", "+js(fmt.Sprint(d.Y+d.Height/2))+")?.closest('.delete'))"))
	b.eval("calls.length = 0; document.querySelector('#list > li:last-child .delete').click(); document.querySelector('#list > li:last-child .delete').click()")
	out = append(out, b.until("the delete", "calls.join(',')"))
	b.eval("scrollTo(0, 0)")
	// Only favourites, or none: no headings.
	b.eval(stub(`[{ slug: 'c', title: 'Gamma', description: '', createdAt: at(1), updatedAt: at(9), favorite: true },
		{ slug: 'd', title: 'Delta', description: '', createdAt: at(1), updatedAt: at(8), favorite: true }]`))
	out = append(out, b.eval(order))
	// The creation date under the title, of a link when registered, and none without one.
	date := b.eval("new Date('2026-01-01T00:00:00.000Z').toLocaleDateString(document.documentElement.lang, { dateStyle: 'medium' })")
	b.eval(stub(`[{ slug: 'a', title: 'Alpha', description: '', createdAt: at(1), updatedAt: at(8) },
		{ slug: 'o', title: 'Old', description: '', updatedAt: at(7) },
		{ slug: 'l', title: 'Linked', description: '', path: '/tmp/x/l.html', createdAt: at(1), updatedAt: at(1) }]`))
	out = append(out, b.eval("document.querySelector('#list li[data-slug=a] .made').textContent"),
		b.eval("document.querySelector('#list li[data-slug=o] .made').hidden"))
	b.eval("document.querySelector('.tabs [data-tab=links]').click()")
	out = append(out, b.eval("document.querySelector('#list li[data-slug=l] .made').textContent"))
	b.eval("document.querySelector('.tabs [data-tab=pages]').click()")
	// The kinds show with a link, the projects with two or one chosen.
	b.eval(stub(`[{ slug: 'a', title: 'Alpha', description: '', project: 'p', createdAt: at(1), updatedAt: at(8) }]`))
	out = append(out, b.eval("[document.getElementById('shelf-kind').hidden, document.getElementById('shelf-projects').hidden].join(' ')"))
	b.eval("setProject('p')")
	out = append(out, b.eval("document.getElementById('shelf-projects').hidden"))
	b.eval("setProject('')")
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
	out = append(out, b.eval("[document.getElementById('shelf-kind').hidden, document.getElementById('add-open').hidden].join(' ')"))
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
	b.eval(stub("Array.from({ length: 60 }, (_, i) => ({ slug: `p${i}`, title: `P${i}`, description: '', createdAt: at(1), updatedAt: at(1) }))"))
	out = append(out, b.eval("document.querySelectorAll('#list li').length + ' ' + document.getElementById('pageno').textContent"))
	// An open menu passes under the bars stuck at the top as the page scrolls, not over them.
	b.eval("document.querySelector('#list li[data-slug=p5] summary').click()")
	out = append(out, b.eval(`(() => {
		const bar = document.querySelector('.toolbar').getBoundingClientRect();
		scrollBy(0, document.querySelector('#list li[data-slug=p5] .menu').getBoundingClientRect().top - bar.top - 5);
		const menu = document.querySelector('#list li[data-slug=p5] .menu').getBoundingClientRect();
		return Boolean(document.elementFromPoint(menu.left + menu.width / 2, bar.top + 10)?.closest('.toolbar'));
	})()`))
	b.eval("document.querySelector('#list li[data-slug=p5] summary').click(); scrollTo(0, 0)")
	// A narrow screen: one select stands for the shelf, its values telling a view from a project of that name; choosing
	// is clicking the shelf, and an unchanged list does not rebuild it.
	metrics := emulation.SetDeviceMetricsOverrideParams{Width: 390, Height: 844, DeviceScaleFactor: 1, Mobile: true}
	if _, err := chromedp.Call(b.ctx, emulation.SetDeviceMetricsOverride, metrics); err != nil {
		t.Fatal(err)
	}
	b.eval(stub(`[{ slug: 'a', title: 'Alpha', description: '', project: 'archived', createdAt: at(1), updatedAt: at(8) },
		{ slug: 'b', title: 'Beta', description: '', project: 'p', createdAt: at(1), updatedAt: at(7) },
		{ slug: 'c', title: 'Gamma', description: '', project: 'a<b&"c', createdAt: at(1), updatedAt: at(6) }]`))
	out = append(out, b.eval("[getComputedStyle(document.querySelector('.shelf')).display, getComputedStyle(document.getElementById('view-select')).display].join(' ')"),
		b.eval("document.getElementById('view-select').options[0].textContent"),
		b.eval("[...document.getElementById('view-select').options].map((o) => o.value).join(',')"))
	b.eval("window.first = document.getElementById('view-select').options[0]; render()")
	out = append(out, b.eval("document.getElementById('view-select').options[0] === window.first"))
	choose := func(value string) {
		b.eval("(() => { const s = document.getElementById('view-select'); s.value = " + js(value) + "; s.dispatchEvent(new Event('change')); })()")
	}
	out = append(out, b.eval("[...document.getElementById('view-select').options].at(-1).textContent"))
	choose(`project:a<b&"c`)
	out = append(out, b.eval("new URLSearchParams(location.search).get('project')"))
	choose("project:archived")
	out = append(out, b.eval(views))
	// Choosing what is chosen goes back to the summary too.
	choose("view:all")
	out = append(out, b.eval("document.getElementById('view-select').selectedIndex"))
	if _, err := chromedp.Call(b.ctx, emulation.ClearDeviceMetricsOverride, cdp.Empty{}); err != nil {
		t.Fatal(err)
	}
	// The frame's bar: the title, archived or not, the star and the archive, which act as the list's do and tell their failure.
	frame := "[document.getElementById('frame-title').textContent, document.getElementById('frame-archived').hidden, " +
		"document.getElementById('frame-star').getAttribute('aria-pressed'), document.getElementById('frame-archive-label').textContent].join(' / ')"
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
		"HcHba", "none:heading:2:" + msg("ui_group_favorites") + ",none:heading:2:" + msg("ui_group_others"),
		"c fav ?fav=1", "zy archived ?archived=1",
		"Beta", msg("ui_selected", 3),
		"00010", "shown", "shown", "shown", "0",
		msg("ui_more_named", "Beta"), msg("ui_edit") + " / " + msg("ui_open_raw") + " / " + msg("ui_download") + " / " + msg("ui_delete"),
		"false true", "Alpha", "false true", "false false true", "false true", "true", "true",
		"true", "DELETE /api/artifacts/a", "cd",
		msg("ui_created_at", date), "true", msg("ui_registered_at", date),
		"true true", "false",
		msg("ui_count_all", 1) + " true",
		"empty-favorites", "all-archived", "archived ?archived=1", "archived ?archived=1",
		"true false", "?tab=links true 1 New", "true",
		"50 1 / 2", "true",
		"none block", msg("ui_view_all") + " (" + msg("ui_count_all", 3) + ")",
		`,view:all,view:fav,view:archived,project:,project:a<b&"c,project:archived,project:p`, "true", "p (1)", `a<b&"c`,
		"all ?project=archived", "0",
		"Zeta / false / true / " + msg("ui_unarchive"), `PATCH /api/artifacts/z {"archived":false}`,
		msg("ui_favorite_failed", "Failed to fetch"),
		"Beta / true / false / " + msg("ui_archive"),
		"  1 / 1", "false   1 / 1", "/a/c/  2 / 2",
	}, "|"))
}
