package browsertest_test

import (
	"strings"
	"testing"

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
	const list = JSON.stringify(` + list + `);
	window.fetch = (url, init = {}) => {
		if (!init.method) return Promise.resolve(new Response(list));
		calls.push([init.method, url, init.body].filter(Boolean).join(' '));
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
// archives in bulk, edits in a modal, copies a path, tells an empty list, and steps through neighbours, against a
// stubbed list.
func TestUIExtras(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	inEveryLanguage(t, uiExtras)
}

func uiExtras(t *testing.T, lang i18n.Lang, msg func(string, ...any) string) {
	t.Helper()

	b := newBrowser(t, servetest.Start(t, nil).Base, lang)
	b.navigate("/")
	b.eval(stub(extrasList))
	const titles = "[...document.querySelectorAll('#list .title')].map((a) => a.textContent).join(',')"
	// What the list counts, and the archived in the URL: the tabs, the project chips, the favourites, the archived,
	// the count and the titles.
	const counts = "[...document.querySelectorAll('.tabs .n')].map((n) => n.textContent)" +
		".concat([...document.querySelectorAll('#chips .chip')].map((c) => c.textContent), document.querySelector('#fav .n').textContent, " +
		"document.querySelector('#archived .n').textContent, document.getElementById('count').textContent, " + titles + ", " +
		"new URLSearchParams(location.search).get('archived')).join(' ')"
	// Which empty states show, and their words.
	const empties = "['empty', 'empty-links', 'all-archived', 'empty-archived', 'nomatch']" +
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
	// The ★ chip narrows to favourites (kept in the URL); a row's star sends the change.
	b.eval("document.getElementById('fav').click()")
	out = append(out, b.eval(titles)+" "+b.eval("new URLSearchParams(location.search).get('fav')"))
	b.eval("document.getElementById('fav').click()")
	b.eval("document.querySelector('#list li .star').click()")
	out = append(out, b.until("the favourite", "calls.splice(0).join(',')"))
	// The time's tooltip has both the creation and the update.
	out = append(out, b.eval("document.querySelector('#list li time').title.split('\\n').map((l) => l.split(' ')[0]).join(' ')"))
	// ↓ moves through the rows by focusing their titles, x selects the focused row.
	key("ArrowDown")
	key("ArrowDown")
	out = append(out, b.eval("document.activeElement.textContent"))
	key("x")
	out = append(out, b.eval("document.getElementById('bulk-count').textContent"))
	// With Alpha selected too by a real mouse click (nothing may sit on top of the checkbox), the first
	// click arms and the second deletes both.
	r := b.rect("document.querySelector('#list li .sel')")
	b.click(r.X+r.Width/2, r.Y+r.Height/2)
	out = append(out, b.until("the mouse click to select",
		"document.querySelector('#list li .sel').checked && document.getElementById('bulk-count').textContent"))
	b.eval("document.getElementById('bulk-delete').click()")
	out = append(out, b.eval("document.getElementById('bulk-delete').textContent"))
	b.eval("document.getElementById('bulk-delete').click()")
	b.until("the selection to clear", "document.getElementById('bulk').hidden")
	out = append(out, b.eval("calls.splice(0).sort().join(',')"))
	// The pencil opens a modal with the current description; saving sends the new one.
	b.eval("document.querySelector('#list li .edit').click()")
	out = append(out, b.eval("[document.getElementById('edit-dialog').open, document.getElementById('edit-desc').value].join(' ')"))
	b.eval("document.getElementById('edit-desc').value = 'new'; document.getElementById('edit').requestSubmit()")
	b.until("the edit modal to close", "!document.getElementById('edit-dialog').open")
	out = append(out, b.eval("calls.splice(0).join(',')"),
		b.eval("document.getElementById('toasts').textContent.includes("+js(msg("ui_saved"))+")"))
	// Only linked files have a copy button for their path.
	out = append(out, b.eval("document.querySelector('#list li .copy').hidden"))
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
	// button and the bulk bar send the change, to archive or to give back.
	out = append(out, b.eval(counts))
	b.eval("document.querySelector('#list li .archive').click()")
	out = append(out, b.until("the archive", "calls.splice(0).join(',')"),
		b.eval("document.getElementById('toasts').textContent.includes("+js(msg("ui_archive_done", "Alpha"))+")"))
	b.eval("document.getElementById('archived').click()")
	out = append(out, b.eval(counts))
	b.eval("document.querySelector('#list li .archive').click()")
	out = append(out, b.until("the give back", "calls.splice(0).join(',')"))
	b.eval("document.getElementById('bulk-page').click()")
	out = append(out, b.eval("document.getElementById('bulk-archive').textContent"))
	b.eval("document.getElementById('bulk-archive').click()")
	b.until("the selection to clear", "document.getElementById('bulk').hidden")
	out = append(out, b.eval("calls.splice(0).sort().join(',')"))
	// Empty lists: no archived link; all pages archived, which leads to them; nothing at all.
	b.eval("document.querySelector('.tabs [data-tab=links]').click()")
	b.eval(stub(`[{ slug: 'l', title: 'Linked', description: '', path: '/tmp/x/l.html', createdAt: at(1), updatedAt: at(1) }]`))
	out = append(out, b.eval(empties))
	b.eval("document.querySelector('.tabs [data-tab=pages]').click(); document.getElementById('archived').click()")
	b.eval(stub(`[{ slug: 'z', title: 'Zeta', description: '', createdAt: at(2), updatedAt: at(6), archived: true }]`))
	out = append(out, b.eval(empties))
	b.eval("document.getElementById('show-archived').click()")
	out = append(out, b.eval(titles+" + ' ' + new URLSearchParams(location.search).get('archived')"))
	b.eval("document.getElementById('archived').click()")
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
		"Beta,Alpha,Gamma", "Gamma,Beta,Alpha created", "Alpha,Beta,Gamma", "Gamma 1",
		`PATCH /api/artifacts/a {"favorite":true}`, label("ui_created_at") + " " + label("ui_updated_at"), "Beta",
		msg("ui_selected", 1), msg("ui_selected", 2), msg("ui_delete_bulk_confirm", 2),
		"DELETE /api/artifacts/a,DELETE /api/artifacts/b", "true old", `PATCH /api/artifacts/a {"description":"new"}`, "true", "true",
		"copy /tmp/x/l.html", "true", "Missing", `POST /api/artifacts {"path":"/tmp/x/n.html","description":"a link"}`,
		"3 2 " + msg("ui_all_projects") + "3 " + msg("ui_no_project") + "3 1 2 " + msg("ui_count_all", 3) + " Alpha,Beta,Gamma ",
		`PATCH /api/artifacts/a {"archived":true}`, "true",
		"2 1 " + msg("ui_all_projects") + "2 p1 " + msg("ui_no_project") + "1 1 2 " + msg("ui_count_all", 2) + " Ypsilon,Zeta 1",
		`PATCH /api/artifacts/y {"archived":false}`, msg("ui_unarchive"),
		`PATCH /api/artifacts/y {"archived":false},PATCH /api/artifacts/z {"archived":false}`,
		"empty-archived:" + msg("ui_links_none_archived"),
		"all-archived:" + msg("ui_pages_all_archived", 1),
		"Zeta 1",
		"empty:" + msg("ui_empty_pages"),
		"/a/a/ /a/c/ 2 / 3 /?sort=title",
		" /a/y/ 1 / 2 /?sort=title",
	}, "|"))
}
