package browsertest_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/chromedp/chromedp/kb"

	"floe/internal/store"
)

// TestUITaskArchivedInCategory keeps a task's archived pages under their categories, after the rest of each, dimmed and marked: a category of
// archived pages alone is a category all the same, and no archived page is unseen, wherever its row is. Archived by the keys and left
// on the task's page, Escape on the way back goes back to its row; a row moved by its button leaves the focus on the one after it.
func TestUITaskArchivedInCategory(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	yes := true
	publish := func(slug, words string) {
		t.Helper()
		s.floe(t, "publish", s.page(t, slug+".yaml", "T "+slug, "<p>"+words+"</p>"), "--slug", slug)
	}
	readWhole := func(slug string) {
		t.Helper()
		b.navigate("/a/" + slug + "/")
		b.until(slug+" read", "document.querySelector('[data-testid=toast][data-page]')")
	}
	// In the order made, which orders the pages of a category: a0 is made after a1 and before a2, o1 before n1
	for _, p := range [][2]string{{"a1", "案"}, {"a0", "案"}, {"a2", "案"}, {"o1", "旧"}, {"n1", "新"}, {"v1", "設計"}, {"v0", "設計"}} {
		publish(p[0], "one")
		if _, err := st.Edit(p[0], store.Edit{Project: new("proj")}); err != nil {
			t.Fatal(err)
		}
		s.floe(t, "task", "--slug", p[0], "--task", "T1", "--category", p[1])
	}
	s.write(t, "fold/one.html", "<title>one</title><p>one</p>")
	s.write(t, "fold/sub/two.html", "<title>two</title><p>two</p>")
	s.floe(t, "publish", "--link", filepath.Join(s.Env["HOME"], "fold"), "--slug", "fold")
	if _, err := st.Edit("fold", store.Edit{Project: new("proj")}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ChangeTasks(store.TaskChange{Slugs: []string{"fold"}, Task: new("T1")}); err != nil {
		t.Fatal(err)
	}
	s.floe(t, "task", "--project", "proj", "--task", "T1", "--categories", "案", "--categories", "設計", "--categories", "旧", "--categories", "新")
	// Read: o1 and n1 as they are, v1 and v0 before a new version of each
	for _, slug := range []string{"o1", "n1", "v1", "v0"} {
		readWhole(slug)
	}
	publish("v1", "two")
	publish("v0", "two")
	for _, slug := range []string{"a0", "o1", "v0", "fold"} {
		if _, err := st.Edit(slug, store.Edit{Archived: &yes}); err != nil {
			t.Fatal(err)
		}
	}
	b.size(1400, 900, false)
	defer b.unsize()

	var out []string
	say := func(what, expr string) {
		t.Helper()
		out = append(out, what+": "+b.eval(expr))
	}
	const groups = "[...document.querySelectorAll('#list > li:not([data-testid=tree])')].map((li) => li.dataset.testid === 'group' ? '[' + li.querySelector('[role=heading]').textContent + ']' : li.dataset.slug + (li.dataset.archived === 'true' ? '*' : '')).join(' ')"
	// A row's marks of what is unseen: its lead, the class, the title's weight, a new version's mark
	marks := func(sel string) string {
		return "(() => { const li = document.querySelector(" + js(sel) + "); return [li.querySelector('[data-testid=lead] span')?.dataset.testid, li.dataset.unread === 'true', getComputedStyle(li.querySelector('[data-testid=title]')).fontWeight, Boolean(li.querySelector('[data-testid=ver]'))].join(' '); })()"
	}
	// Whether an element's colour is the palette's muted
	const muted = "((e) => { const m = document.createElement('span'); m.style.color = 'var(--color-muted)'; document.body.append(m); const c = getComputedStyle(m).color; m.remove(); return getComputedStyle(e).color === c; })"
	const badge = "((e) => { const s = getComputedStyle(e); return [e.textContent, s.paddingLeft, s.fontWeight].join(' '); })"
	open := func() {
		t.Helper()
		b.navigate("/tasks?project=proj&task=T1")
		b.until("the task's rows", "document.querySelector('#list li[data-slug=a1]') && document.querySelector('#list li[data-testid=tree]')")
	}

	// Under their categories, after the rest; a category of archived pages alone headed; dimmed, marked, never unseen; the folder's rows too.
	b.eval("localStorage.setItem('list.open', JSON.stringify(['fold']))")
	open()
	say("groups", groups)
	say("a0", marks("#list li[data-slug=a0]"))
	say("a1", marks("#list li[data-slug=a1]"))
	say("v0", marks("#list li[data-slug=v0]"))
	say("v1", marks("#list li[data-slug=v1]"))
	say("dimmed", "["+muted+"(document.querySelector('#list li[data-slug=a0] [data-testid=title]')), "+muted+"(document.querySelector('#list li[data-slug=a1] [data-testid=title]')), "+
		"...[...document.querySelectorAll('#list li[data-testid=tree] [data-testid=title]')].map((e) => e.tagName + ' ' + "+muted+"(e))].join(', ')")
	say("badge", badge+"(document.querySelector('#list li[data-slug=a0] [data-testid=badge]'))")
	say("count", "document.getElementById('news-n').textContent")
	// Unread is offered of a page read, not archived
	say("mark unread", "["+inMenuItem("#list li[data-slug=o1]", "mark-unread", "i.hidden")+", "+inMenuItem("#list li[data-slug=n1]", "mark-unread", "i.hidden")+"].join(' ')")

	// The search: the same marks, the badge of the same size.
	pressFindOpen(b)
	b.until("the search", "document.getElementById('find').open")
	b.fill("#find-q", "T a0")
	b.until("the hit", "document.querySelector('#find-list li')")
	say("find", "(() => { const li = document.querySelector('#find-list li'); return [li.querySelector('[data-testid=lead] span')?.dataset.testid, li.dataset.unread === 'true', "+muted+"(li.querySelector('[data-testid=title]')), "+badge+"(li.querySelector('[data-testid=badge]'))].join(' '); })()")
	b.pressKey(kb.Escape)
	b.until("the search closed", "!document.getElementById('find').open")

	// The categories Set task offers are the page's: archived pages count, in the order they were made.
	pressMenuItem(b, "#list li[data-slug=n1]", "task-set")
	b.until("the dialog", "document.getElementById('set-dialog').open")
	b.fill("#set-category", "")
	b.until("the categories", "document.getElementById('set-category-names').matches(':popover-open') || (document.getElementById('set-category').blur(), document.getElementById('set-category').focus(), false)")
	say("set categories", "[...document.getElementById('set-category-names').children].map((e) => e.querySelector('[data-testid=names-word]')?.textContent ?? e.textContent).join(', ')")
	// Closed, the focus goes back to the menu it came from
	b.eval("document.querySelector('#set-dialog [data-close]').click()")
	b.until("closed", "!document.getElementById('set-dialog').open && document.activeElement.tagName === 'SUMMARY'")
	b.eval("(() => { const m = document.querySelector('#list li[data-slug=n1] [data-testid=more]'); if (m.open) m.querySelector('summary').click(); })()")

	// Archived from the keys and left dimmed: Escape on the way back goes back to its row, before the list is read again too.
	release := b.holdReads(unsent, "/api/pages", "/api/tasks")
	b.eval("document.querySelector('#list li[data-slug=a1] [data-testid=actions] > [data-testid=archive]').focus()")
	b.pressKey(kb.Enter)
	b.until("the way back", "document.activeElement.dataset.testid === 'undo'")
	b.pressKey(kb.Escape)
	say("escape", "document.activeElement.closest('li')?.dataset.slug + ' ' + document.activeElement.dataset.testid")
	release()
	b.until("archived", "document.querySelector('#list li[data-slug=a1]')?.dataset.archived === 'true' && document.activeElement.closest('li')?.dataset.slug === 'a1'")
	say("archived a1", groups)
	// Archived by a pointer and staying where it was, v1 keeps the focus on its button.
	b.eval("const a = document.querySelector('#list li[data-slug=v1] [data-testid=actions] > [data-testid=archive]'); a.focus(); a.dispatchEvent(new MouseEvent('click', { bubbles: true, detail: 1 }))")
	b.until("v1 archived", "document.querySelector('#list li[data-slug=v1]').dataset.archived === 'true'")
	say("stayed", groups+" + ' / ' + document.activeElement.closest('li')?.dataset.slug + ' ' + document.activeElement.matches('[data-testid=actions] > [data-testid=archive]')")
	// Given back by a pointer, a0 moves to the top of its category: the focus goes to the row that was after it.
	b.eval("const a = document.querySelector('#list li[data-slug=a0] [data-testid=actions] > [data-testid=archive]'); a.focus(); a.dispatchEvent(new MouseEvent('click', { bubbles: true, detail: 1 }))")
	b.until("given back", "document.querySelector('#list li[data-slug=a0]').dataset.archived !== 'true'")
	say("given back", groups)
	say("given back, focus", "document.activeElement.closest('li')?.dataset.slug + ' ' + document.activeElement.dataset.testid")

	// The archive's shelf: never unseen there either.
	b.navigate("/?view=archived")
	b.until("the archive", "document.querySelector('#list li[data-slug=v0]')")
	say("shelf", marks("#list li[data-slug=v0]")+" + ' ' + (document.querySelector('#list li[data-slug=v0]').dataset.archived === 'true')")

	// Opened whole, an archived page is read with no way back to unread; its changes are told all the same. One not archived has both.
	if _, err := st.Edit("v1", store.Edit{Archived: new(bool)}); err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"v1", "v0"} {
		b.navigate("/a/" + slug + "/")
		b.until(slug+" told", "document.querySelector('[data-testid=page-note]')")
		say("opened "+slug, "Boolean(document.querySelector('[data-testid=toast][data-page]'))")
	}
	// The toast's way back to unread, pressed once the page is archived, does nothing.
	b.navigate("/a/a2/")
	b.until("the toast", pageToastButton)
	b.eval("window.deleted = 0; const was = window.fetch; window.fetch = (url, init) => { if (init?.method === 'DELETE') deleted += 1; return was(url, init); }")
	b.eval("document.querySelector('#frame-actions > [data-testid=archive]').click()")
	b.until("archived whole", "document.querySelector('#frame-actions > [data-testid=archive]').dataset.on === 'true'")
	b.eval(pageToastButton + ".click()")
	say("unread after archived", "(async () => deleted + ' ' + Boolean(await "+pageNews("a2")+"))()")

	check(t, "task archived in category", out, strings.Join([]string{
		"groups: [No category] fold* [案] a1 a2 a0* [設計] v1 v0* [旧] o1* [新] n1",
		"a0: ring false 500 false",
		"a1: dot true 700 false",
		"v0: ring false 500 false",
		"v1: dot true 700 true",
		"dimmed: true, false, BUTTON true, A true",
		"badge: Archived 4.8px 400",
		"count: All unread 3",
		"mark unread: true false",
		"find: ring false true Archived 4.8px 400",
		"set categories: 案, 設計, 旧, 新",
		"escape: a1 title",
		"archived a1: [No category] fold* [案] a2 a1* a0* [設計] v1 v0* [旧] o1* [新] n1",
		"stayed: [No category] fold* [案] a2 a1* a0* [設計] v1* v0* [旧] o1* [新] n1 / v1 true",
		"given back: [No category] fold* [案] a0 a2 a1* [設計] v1* v0* [旧] o1* [新] n1",
		"given back, focus: v1 title",
		"shelf: ring false 500 false false",
		"opened v1: true",
		"opened v0: false",
		"unread after archived: 0 false",
	}, "|"))
}
