package browsertest_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/chromedp/chromedp/kb"

	"floe/internal/store"
)

// A task's page heads every category, those of no page too, each with a way to add a page of the task's project: the task is written first,
// of the rev the modal opened at, then the page, which alone holds its category. A page comes off its task from its menu, the toast's way back putting it on the same
// task and category, made anew when the task went with it.
func TestUITaskCategories(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	yes := true
	for _, p := range [][3]string{
		{"plan-a", "proj", "S300 案"},
		{"build-a", "proj", "S300 実装"},
		{"review-a", "proj", "S300 レビュー"},
		{"note-a", "proj", "S300 "},
		{"old-a", "proj", "S300 案"},
		{"memo-a", "proj", ""},
		{"check-a", "proj", ""},
		{"other-a", "proj", "S301 設計"},
		{"g-a", "proj", ""},
		{"far-a", "else", ""},
	} {
		s.floe(t, "publish", s.page(t, p[0]+".yaml", "T "+p[0], ""), "--slug", p[0])
		project := p[1]
		if _, err := st.Edit(p[0], store.Edit{Project: &project}); err != nil {
			t.Fatal(err)
		}
		if task, category, ok := strings.Cut(p[2], " "); ok {
			args := []string{"work", p[0], "--task", task}
			if category != "" {
				args = append(args, "--category", category)
			}
			s.floe(t, args...)
		}
	}
	if _, err := st.Edit("old-a", store.Edit{Archived: &yes}); err != nil {
		t.Fatal(err)
	}
	s.floe(t, "work", "--project", "proj", "--task", "S300", "--title", "Categories", "--categories", "案,実装,リリース,レビュー")
	// G2 has no row: its one page alone makes it, written to the page alone
	g2, x := "G2", "x"
	if _, err := st.Edit("g-a", store.Edit{Work: &store.WorkEdit{Task: &g2, Category: &x}}); err != nil {
		t.Fatal(err)
	}
	b.size(1400, 900, false)
	defer b.unsize()

	var out []string
	say := func(what, expr string) {
		t.Helper()
		out = append(out, what+": "+b.eval(expr))
	}
	row := func(name string) string {
		t.Helper()
		all, err := st.ReadTasks()
		if err != nil {
			t.Fatal(err)
		}
		r, ok := all["proj"][name]
		if !ok {
			return "none"
		}
		return strconv.Itoa(r.Rev)
	}
	work := func(slug string) string {
		t.Helper()
		w := st.ReadMeta(slug).Work
		if w == nil {
			return "none"
		}
		return w.Task + "/" + w.Category
	}
	const groups = "[...document.querySelectorAll('#list li')].map((li) => li.classList.contains('group') ? '[' + li.querySelector('[role=heading]').textContent + (li.querySelector('.category-add') ? '+' : '') + (li.querySelector('.category-empty') ? ' none' : '') + ']' : li.dataset.slug).join(' ')"
	const options = "[...document.querySelectorAll('#add-page-list li')].map((li) => li.querySelector('.title').textContent.slice(2) + (li.querySelector('.badge') ? ' (' + li.querySelector('.badge').textContent + ')' : '')).join(', ')"
	const lastToast = "[...document.querySelectorAll('#toasts .toast')].at(-1)?.textContent.trim()"
	open := func(task string) {
		t.Helper()
		b.navigate("/tasks?project=proj&task=" + task)
		b.until("the task's rows", "document.querySelector('#list li[data-slug]') && document.getElementById('task-title').textContent.includes("+js(task)+")")
		// The writes, as sent, and how many the server has answered: one sent may not be written yet
		b.eval("window.sent = []; window.answered = 0; window.realFetch ??= window.fetch; window.fetch = (url, init) => { if (init?.method !== 'PATCH') return window.realFetch(url, init); sent.push(new URL(url, location).pathname + ' ' + init.body); return window.realFetch(url, init).finally(() => { answered += 1; }); }")
	}
	add := func(category string) {
		t.Helper()
		b.eval("const a = document.querySelector('#list li[data-key=\"group:' + " + js(category) + " + '\"] .category-add'); a.focus(); a.click()")
		b.until("the modal", "document.getElementById('add-page-dialog').open")
	}
	query := func(words string) {
		t.Helper()
		b.eval("const q = document.getElementById('add-page-q'); q.value = " + js(words) + "; q.dispatchEvent(new Event('input', { bubbles: true }))")
	}
	// The menu's item pressed by a pointer (detail 1) or from the keys (0)
	menu := func(slug, item string, keys bool) {
		t.Helper()
		detail := "1"
		if keys {
			detail = "0"
		}
		b.eval("const m = document.querySelector('#list li[data-slug=" + slug + "] .more'); m.open = true; m.querySelector('.menu ." + item + "').dispatchEvent(new MouseEvent('click', { bubbles: true, detail: " + detail + " }))")
	}

	// Every category a heading with its add button, one of no page saying so; the categories' none has none. The archived page is under its category,
	// after the rest.
	open("S300")
	say("groups", groups)
	say("keys apart", "(() => { const keys = [...document.querySelectorAll('#list li')].map((li) => li.dataset.key); return new Set(keys).size === keys.length; })()")
	say("named", "document.querySelector('#list li[data-key=\"group:リリース\"] .category-add').getAttribute('aria-label')")
	// The modal: none reached, so Enter puts nothing; the pages of the project, not archived, not of this category, each saying where it is.
	add("リリース")
	say("opened", "document.getElementById('add-page-title').textContent + ' / ' + document.getElementById('add-page-target').textContent + ' / ' + document.activeElement.id + ' / ' + document.getElementById('add-page-q').getAttribute('aria-activedescendant')")
	say("options", options)
	say("no link", "document.querySelectorAll('#add-page-list a').length")
	b.pressKey(kb.Enter)
	say("nothing reached", "sent.length + ' ' + document.getElementById('add-page-dialog').open")
	// Words narrow them; ↓ and Enter put the one reached: the task first, as read, then the page, which alone holds its category; closed, the focus on the category's button.
	query("check")
	say("narrowed", options)
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.Enter)
	b.until("closed", "!document.getElementById('add-page-dialog').open && document.querySelector('#list li[data-slug=check-a]')")
	say("put", "sent.join(' ; ')")
	say("put, told", lastToast)
	say("put, focus", "document.activeElement.getAttribute('aria-label')")
	out = append(out, "put, written: "+row("S300")+" / "+work("check-a"))
	say("put, groups", groups)
	query("nothing like it")

	// Written meanwhile: refused, nothing put, said in the modal; the second goes, of the rev read again.
	b.eval("sent.length = 0")
	add("リリース")
	s.floe(t, "work", "--project", "proj", "--task", "S300", "--title", "Later")
	// Read again meanwhile, as a look does: the modal still sends the rev it opened at
	b.eval("window.reread = false; readTasks().then((r) => { tasksRead = r; window.reread = true; })")
	b.until("the tasks read again", "window.reread")
	query("memo")
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.Enter)
	b.until("refused", "!document.getElementById('add-page-error').hidden")
	say("stale", "document.getElementById('add-page-error').textContent + ' / ' + document.getElementById('add-page-dialog').open + ' / ' + "+options)
	// Typed again, the refusal goes; Enter picks again
	query("memo")
	say("stale, typed", "document.getElementById('add-page-error').hidden")
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.Enter)
	b.until("closed", "!document.getElementById('add-page-dialog').open")
	say("stale, then", "sent.join(' ; ')")
	out = append(out, "stale, written: "+row("S300")+" / "+work("memo-a"))

	// The task's row checked and the page refused: said, and the next goes of the rev that check answered, the row unchanged.
	b.eval("sent.length = 0; window.failPage = true; const was = window.fetch; window.fetch = (url, init) => { if (window.failPage && String(url).includes('/api/artifacts/') && init?.method === 'PATCH') { window.failPage = false; return Promise.resolve(new Response('no', { status: 500 })); } return was(url, init); }")
	add("案")
	query("note")
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.Enter)
	b.until("refused", "!document.getElementById('add-page-error').hidden")
	say("page refused", "document.getElementById('add-page-error').textContent + ' / ' + document.getElementById('add-page-dialog').open")
	b.pressKey(kb.Enter)
	b.until("closed", "!document.getElementById('add-page-dialog').open")
	say("page refused, then", "sent.join(' ; ')")
	out = append(out, "page refused, written: "+row("S300")+" / "+work("note-a"))

	// Not reached: said in the modal, which stays open.
	b.eval("window.cutFetch = window.fetch; window.fetch = (url, init) => (String(url).startsWith('/api/tasks') && init?.method === 'PATCH' ? Promise.reject(new TypeError('Failed to fetch')) : window.cutFetch(url, init))")
	add("案")
	query("other")
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.Enter)
	b.until("not reached", "!document.getElementById('add-page-error').hidden")
	say("not reached", "document.getElementById('add-page-error').textContent + ' / ' + document.getElementById('add-page-dialog').open")
	b.eval("window.fetch = window.cutFetch")
	b.pressKey(kb.Escape)
	b.until("closed", "!document.getElementById('add-page-dialog').open")

	// From another task, and from another category of this one: said where it came from.
	add("案")
	query("other")
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.Enter)
	b.until("closed", "!document.getElementById('add-page-dialog').open")
	say("moved from a task", lastToast)
	add("リリース")
	query("build")
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.Enter)
	b.until("closed", "!document.getElementById('add-page-dialog').open")
	say("moved from a category", lastToast)
	say("moved, groups", groups)
	// A narrow screen: the modal fits.
	b.size(390, 844, true)
	add("リリース")
	say("narrow", "document.getElementById('add-page-dialog').scrollWidth <= document.getElementById('add-page-dialog').clientWidth")
	b.pressKey(kb.Escape)
	b.size(1400, 900, false)

	// Off its task by a pointer: the page alone written; its category of pages alone goes with it, the focus on the row after, or before; the way back
	// writes the page's work back alone, the task's row as it was.
	open("S300")
	menu("review-a", "task-out", false)
	b.until("off", "!document.querySelector('#list li[data-slug=review-a]')")
	say("off", "sent.join(' ; ')")
	say("off, told", lastToast)
	say("off, focus", "document.activeElement.closest('li')?.dataset.slug")
	say("off, groups", groups)
	b.eval("sent.length = 0; [...document.querySelectorAll('#toasts .undo')].at(-1).click()")
	b.until("back", "document.querySelector('#list li[data-slug=review-a]')")
	say("back", "sent.join(' ; ')")
	out = append(out, "back, written: "+row("S300")+" / "+work("review-a"))
	// From the keys: the focus on the way back; Escape leaves it on the row after the one taken off.
	menu("plan-a", "task-out", true)
	b.until("off", "!document.querySelector('#list li[data-slug=plan-a]')")
	say("keys, focus", "document.activeElement.className")
	b.pressKey(kb.Escape)
	say("keys, escape", "document.activeElement.closest('li')?.dataset.slug + ' ' + document.querySelectorAll('#toasts .undo').length")

	// A task of no row, its one page taken off, is gone; its way back, pressed before the list is read again too, brings it back of its page
	// alone, as it was, no row made.
	open("G2")
	b.eval("window.realLoad = load; load = async () => {}")
	menu("g-a", "task-out", false)
	b.until("the way back", "document.querySelector('#toasts .undo')")
	b.eval("document.querySelector('#toasts .undo').click()")
	b.until("written back", "answered === 2")
	b.eval("load = window.realLoad")
	say("back as it was", "sent.join(' ; ')")
	out = append(out, "back as it was, written: "+row("G2")+" / "+work("g-a"))

	// Only a page on a task has the item.
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=far-a]')")
	say("item", "document.querySelector('#list li[data-slug=far-a] .task-out').hidden + ' ' + document.querySelector('#list li[data-slug=build-a] .task-out').hidden + ' ' + document.querySelector('#list li[data-slug=build-a] .task-out').getAttribute('aria-label')")
	// From the task dialog, by the keys: closed first, then the focus on the way back; Escape leaves it on the menu it opened from.
	b.eval("const m = document.querySelector('#list li[data-slug=build-a] .more'); m.open = true; m.querySelector('.task-set').click()")
	b.until("the dialog", "document.getElementById('set-dialog').open")
	b.eval("document.getElementById('set-out').click()")
	b.until("closed", "!document.getElementById('set-dialog').open && document.activeElement.classList.contains('undo')")
	say("dialog, focus", "document.activeElement.className")
	b.pressKey(kb.Escape)
	say("dialog, escape", "document.activeElement.tagName + ' ' + document.activeElement.closest('li')?.dataset.slug")
	out = append(out, "dialog, written: "+work("build-a"))

	check(t, "task categories", out, strings.Join([]string{
		"groups: note-a [案+] plan-a old-a [実装+] build-a [リリース+ none] [レビュー+] review-a",
		"keys apart: true",
		"named: Add a page to リリース",
		"opened: Add a page to リリース / S300 Categories / add-page-q / null",
		"options: g-a (G2 · x), other-a (S301 · 設計), check-a, memo-a, note-a (this task's no category), review-a (this task's レビュー), build-a (this task's 実装), plan-a (this task's 案)",
		"no link: 0",
		"nothing reached: 0 true",
		"narrowed: check-a",
		`put: /api/tasks {"new":false,"rev":4} ; /api/artifacts/check-a {"work":{"task":"S300","category":"リリース"}}`,
		"put, told: Put T check-a under リリース",
		"put, focus: Add a page to リリース",
		"put, written: 4 / S300/リリース",
		"put, groups: note-a [案+] plan-a old-a [実装+] build-a [リリース+] check-a [レビュー+] review-a",
		"stale: Failed to save: task S300 has changed since it was read; the tasks are read again, so check and save again / true / memo-a",
		"stale, typed: true",
		`stale, then: /api/tasks {"new":false,"rev":4} ; /api/tasks {"new":false,"rev":5} ; /api/artifacts/memo-a {"work":{"task":"S300","category":"リリース"}}`,
		"stale, written: 5 / S300/リリース",
		"page refused: Failed to save: HTTP 500: no / true",
		`page refused, then: /api/tasks {"new":false,"rev":5} ; /api/tasks {"new":false,"rev":5} ; /api/artifacts/note-a {"work":{"task":"S300","category":"案"}}`,
		"page refused, written: 5 / S300/案",
		"not reached: Failed to save: Failed to fetch / true",
		"moved from a task: Moved T other-a from S301's 設計 to 案",
		"moved from a category: Moved T build-a from 実装 to リリース",
		"moved, groups: [案+] plan-a note-a other-a old-a [実装+ none] [リリース+] build-a memo-a check-a [レビュー+] review-a",
		"narrow: true",
		`off: /api/artifacts/review-a {"work":{"task":"","category":""}}`,
		"off, told: Took T review-a off task S300Undo",
		"off, focus: check-a",
		"off, groups: [案+] plan-a note-a other-a old-a [実装+ none] [リリース+] build-a memo-a check-a [レビュー+ none]",
		`back: /api/artifacts/review-a {"work":{"task":"S300","category":"レビュー"}}`,
		"back, written: 5 / S300/レビュー",
		"keys, focus: btn undo",
		"keys, escape: note-a 0",
		`back as it was: /api/artifacts/g-a {"work":{"task":"","category":""}} ; /api/artifacts/g-a {"work":{"task":"G2","category":"x"}}`,
		"back as it was, written: none / G2/x",
		"item: true false Take T build-a off its task",
		"dialog, focus: btn undo",
		"dialog, escape: SUMMARY build-a",
		"dialog, written: none",
	}, "|"))
}
