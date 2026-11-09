package browsertest_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/chromedp/chromedp/kb"

	"github.com/sukekyo26/floe/internal/store"
)

// TestUITaskArchivedInStep keeps a task's archived pages under their steps, after the rest of each, dimmed and marked: a step of
// archived pages alone is a step all the same, and no archived page is unseen, wherever its row is. Archived by the keys and left
// on the task's page, Escape on the way back goes back to its row; a row moved by its button leaves the focus on the one after it.
func TestUITaskArchivedInStep(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
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
		b.until(slug+" read", "document.querySelector('.toast.page')")
	}
	// In the order made, which orders the pages of a step: a0 is made after a1 and before a2, o1 before n1
	for _, p := range [][2]string{{"a1", "案"}, {"a0", "案"}, {"a2", "案"}, {"o1", "旧"}, {"n1", "新"}, {"v1", "設計"}, {"v0", "設計"}} {
		publish(p[0], "one")
		if _, err := st.Edit(p[0], store.Edit{Project: ptrTo("proj")}); err != nil {
			t.Fatal(err)
		}
		s.floe(t, "work", p[0], "--task", "T1", "--step", p[1])
	}
	s.write(t, "fold/one.html", "<title>one</title><p>one</p>")
	s.write(t, "fold/sub/two.html", "<title>two</title><p>two</p>")
	s.floe(t, "publish", "--link", filepath.Join(s.Env["HOME"], "fold"), "--slug", "fold")
	if _, err := st.Edit("fold", store.Edit{Project: ptrTo("proj"), Work: &store.WorkEdit{Task: ptrTo("T1")}}); err != nil {
		t.Fatal(err)
	}
	s.floe(t, "work", "--project", "proj", "--task", "T1", "--steps", "案,設計")
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
	const groups = "[...document.querySelectorAll('#list > li:not(.tree)')].map((li) => li.classList.contains('group') ? '[' + li.querySelector('[role=heading]').textContent + ']' : li.dataset.slug + (li.classList.contains('archived') ? '*' : '')).join(' ')"
	// A row's marks of what is unseen: its lead, the class, the title's weight, a new version's mark
	marks := func(sel string) string {
		return "(() => { const li = document.querySelector(" + js(sel) + "); return [li.querySelector('.lead span')?.className, li.classList.contains('unseen'), getComputedStyle(li.querySelector('.title')).fontWeight, Boolean(li.querySelector('.ver'))].join(' '); })()"
	}
	// Whether an element's colour is the palette's muted
	const muted = "((e) => { const m = document.createElement('span'); m.style.color = 'var(--muted)'; document.body.append(m); const c = getComputedStyle(m).color; m.remove(); return getComputedStyle(e).color === c; })"
	const badge = "((e) => { const s = getComputedStyle(e); return [e.textContent, s.paddingLeft, s.fontWeight].join(' '); })"
	open := func() {
		t.Helper()
		b.navigate("/tasks?project=proj&task=T1")
		b.until("the task's rows", "document.querySelector('#list li[data-slug=a1]') && document.querySelector('#list li.tree')")
	}

	// Under their steps, after the rest; a step of archived pages alone headed; dimmed, marked, never unseen; the folder's rows too.
	b.eval("localStorage.setItem('list.open', JSON.stringify(['fold']))")
	open()
	say("groups", groups)
	say("a0", marks("#list li[data-slug=a0]"))
	say("a1", marks("#list li[data-slug=a1]"))
	say("v0", marks("#list li[data-slug=v0]"))
	say("v1", marks("#list li[data-slug=v1]"))
	say("dimmed", "["+muted+"(document.querySelector('#list li[data-slug=a0] .title')), "+muted+"(document.querySelector('#list li[data-slug=a1] .title')), "+
		"...[...document.querySelectorAll('#list li.tree .title')].map((e) => e.tagName + ' ' + "+muted+"(e))].join(', ')")
	say("badge", badge+"(document.querySelector('#list li[data-slug=a0] .badge'))")
	say("count", "document.getElementById('news-n').textContent")
	// Unread is offered of a page read, not archived
	say("mark unread", "['o1', 'n1'].map((slug) => document.querySelector('#list li[data-slug=' + slug + '] .mark-unread').hidden).join(' ')")

	// The search: the same marks, the badge of the same size.
	b.eval("openFind()")
	b.until("the search", "document.getElementById('find').open")
	b.eval("const q = document.getElementById('find-q'); q.value = 'T a0'; q.dispatchEvent(new Event('input', { bubbles: true }))")
	b.until("the hit", "document.querySelector('#find-list li')")
	say("find", "(() => { const li = document.querySelector('#find-list li'); return [li.querySelector('.lead span')?.className, li.classList.contains('unseen'), "+muted+"(li.querySelector('.title')), "+badge+"(li.querySelector('.badge'))].join(' '); })()")
	b.pressKey(kb.Escape)
	b.until("the search closed", "!document.getElementById('find').open")

	// The steps Set task offers are the page's: archived pages count, in the order they were made.
	b.eval("const m = document.querySelector('#list li[data-slug=n1] .more'); m.open = true; m.querySelector('.task-set').click()")
	b.until("the dialog", "document.getElementById('set-dialog').open")
	b.eval("document.getElementById('set-step').value = ''")
	b.until("the steps", "document.getElementById('set-step-names').matches(':popover-open') || (document.getElementById('set-step').blur(), document.getElementById('set-step').focus(), false)")
	say("set steps", "[...document.getElementById('set-step-names').children].map((e) => e.querySelector('.names-word')?.textContent ?? e.textContent).join(', ')")
	b.eval("document.getElementById('set-dialog').close()")
	// Closed, the focus goes back to the menu it came from, as the dialog's close does
	b.until("closed", "!document.getElementById('set-dialog').open && document.activeElement.tagName === 'SUMMARY'")
	b.eval("document.querySelector('#list li[data-slug=n1] .more').open = false")

	// Archived from the keys and left dimmed: Escape on the way back goes back to its row.
	b.eval("const a = document.querySelector('#list li[data-slug=a1] .actions > .archive'); a.focus(); a.dispatchEvent(new MouseEvent('click', { bubbles: true, detail: 0 }))")
	b.until("archived", "document.querySelector('#list li[data-slug=a1]')?.classList.contains('archived') && document.activeElement.classList.contains('undo')")
	b.pressKey(kb.Escape)
	say("escape", "document.activeElement.closest('li')?.dataset.slug + ' ' + document.activeElement.className")
	say("archived a1", groups)
	// Given back by a pointer, a0 moves to the top of its step: the focus goes to the row that was after it.
	b.eval("const a = document.querySelector('#list li[data-slug=a0] .actions > .archive'); a.focus(); a.dispatchEvent(new MouseEvent('click', { bubbles: true, detail: 1 }))")
	b.until("given back", "!document.querySelector('#list li[data-slug=a0]').classList.contains('archived')")
	say("given back", groups)
	say("given back, focus", "document.activeElement.closest('li')?.dataset.slug + ' ' + document.activeElement.className")

	// The archive's shelf: never unseen there either.
	b.navigate("/?view=archived")
	b.until("the archive", "document.querySelector('#list li[data-slug=v0]')")
	say("shelf", marks("#list li[data-slug=v0]")+" + ' ' + document.querySelector('#list li[data-slug=v0]').classList.contains('archived')")

	// Opened whole, an archived page is read with no way back to unread; its changes are told all the same. One not archived has both.
	for _, slug := range []string{"v1", "v0"} {
		b.navigate("/a/" + slug + "/")
		b.until(slug+" told", "document.querySelector('.dnote.page-note')")
		say("opened "+slug, "Boolean(document.querySelector('.toast.page'))")
	}

	check(t, "task archived in step", out, strings.Join([]string{
		"groups: fold* [案] a1 a2 a0* [設計] v1 v0* [旧] o1* [新] n1",
		"a0: ring false 500 false",
		"a1: dot true 700 false",
		"v0: ring false 500 false",
		"v1: dot true 700 true",
		"dimmed: true, false, BUTTON true, A true",
		"badge: Archived 4.8px 400",
		"count: All unread 3",
		"mark unread: true false",
		"find: ring false true Archived 4.8px 400",
		"set steps: 案, 設計, 旧, 新",
		"escape: a1 title",
		"archived a1: fold* [案] a2 a1* a0* [設計] v1 v0* [旧] o1* [新] n1",
		"given back: fold* [案] a0 a2 a1* [設計] v1 v0* [旧] o1* [新] n1",
		"given back, focus: v1 title",
		"shelf: ring false 500 false false",
		"opened v1: true",
		"opened v0: false",
	}, "|"))
}
