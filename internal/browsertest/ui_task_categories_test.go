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
			args := []string{"task", "--slug", p[0], "--task", task}
			if category != "" {
				args = append(args, "--category", category)
			}
			if p[0] == "plan-a" || p[0] == "other-a" {
				args = append(args, "--new")
			}
			s.floe(t, args...)
		}
	}
	if _, err := st.Edit("old-a", store.Edit{Archived: &yes}); err != nil {
		t.Fatal(err)
	}
	s.floe(t, "task", "--project", "proj", "--task", "S300", "--title", "Categories", "--categories", "案", "--categories", "実装", "--categories", "リリース", "--categories", "レビュー")
	// G2 has no row, as a hand edit leaves it: its page says it, and no task is listed of it
	handMeta(t, st, "g-a", "proj", false, "G2", "x", false)
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
	pageTask := func(slug string) string {
		t.Helper()
		w := st.ReadMeta(slug).Task
		if w == nil {
			return "none"
		}
		return w.Name + "/" + w.Category
	}
	const groups = "[...document.querySelectorAll('#list li')].map((li) => li.dataset.testid === 'group' ? '[' + li.querySelector('[role=heading]').textContent + (li.querySelector('[data-testid=category-empty]') ? ' none' : '') + ']' : li.dataset.slug).join(' ')"
	const options = "[...document.querySelectorAll('#add-page-list li')].map((li) => li.querySelector('[data-testid=title]').textContent.slice(2) + (li.querySelector('[data-testid=badge]') ? ' (' + li.querySelector('[data-testid=badge]').textContent + ')' : '')).join(', ')"
	const lastToast = "[...document.querySelectorAll('#toasts [data-testid=toast]')].at(-1)?.textContent.trim()"
	// The writes, as sent, and how many the server has answered: one sent may not be written yet
	record := func() {
		t.Helper()
		b.eval("window.sent = []; window.answered = 0; window.realFetch ??= window.fetch; window.fetch = (url, init) => { if (init?.method !== 'PATCH') return window.realFetch(url, init); sent.push(new URL(url, location).pathname + ' ' + init.body); return window.realFetch(url, init).finally(() => { answered += 1; }); }")
	}
	open := func(task string) {
		t.Helper()
		b.navigate("/tasks?project=proj&task=" + task)
		b.until("the task's rows", "document.querySelector('#list li[data-slug]') && document.getElementById('task-title').textContent.includes("+js(task)+")")
		record()
	}
	// add opens Add a page from the task's head and chooses the category in it, none left as it opens
	add := func(category string) {
		t.Helper()
		b.eval("const a = document.getElementById('task-add-page'); a.focus(); a.click()")
		b.until("the modal", "document.getElementById('add-page-dialog').open")
		if category != "" {
			chooseValue(b, "add-page-category", category)
		}
	}
	query := func(words string) {
		t.Helper()
		b.fill("#add-page-q", words)
	}
	// The row's menu opened and its item pressed by a pointer, or from the keys
	menu := func(slug, item string, keys bool) {
		t.Helper()
		m := "#list li[data-slug=" + slug + "] [data-testid=more]"
		b.eval("document.querySelector(" + js(m+" > summary") + ").click()")
		got := "document.querySelector(" + js(m+" [data-testid="+item+"]") + ")"
		b.until("the item "+item, got+"?.checkVisibility()")
		if keys {
			b.eval(got + ".focus()")
			b.pressKey(kb.Enter)
			return
		}
		b.eval(got + ".scrollIntoView({ block: 'nearest' })")
		b.still()
		r := b.rect(got)
		b.click(r.X+r.Width/2, r.Y+r.Height/2)
	}

	// Every category a heading, one of no page saying so, no category's only as it has pages; no heading has a button, the task's head has one.
	// The archived page is under its category, after the rest.
	open("S300")
	say("groups", groups)
	say("keys apart", "(() => { const keys = [...document.querySelectorAll('#list li')].map((li) => li.dataset.key); return new Set(keys).size === keys.length; })()")
	say("named", "document.querySelectorAll('#list li[data-testid=group] button').length + ' ' + document.getElementById('task-add-page').textContent")
	// The modal, its category chosen in it, none at first: none reached, so Enter puts nothing; the pages of the project, not archived, not of this category, each saying where it is.
	add("リリース")
	say("opened", "document.getElementById('add-page-title').textContent + ' / ' + document.getElementById('add-page-target').textContent + ' / ' + document.activeElement.id + ' / ' + document.getElementById('add-page-q').getAttribute('aria-activedescendant')")
	say("category", "[...document.getElementById('add-page-category').options].map((o) => (o.selected ? '*' : '') + o.textContent).join(',')")
	say("options", options)
	say("no link", "document.querySelectorAll('#add-page-list a').length")
	b.pressKey(kb.Enter)
	say("nothing reached", "sent.length + ' ' + document.getElementById('add-page-dialog').open")
	// Words narrow them; ↓ and Enter put the one reached: the task first, as read, then the page, which alone holds its category; closed, the focus on the head's button.
	query("check")
	say("narrowed", options)
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.Enter)
	b.until("closed", "!document.getElementById('add-page-dialog').open && document.querySelector('#list li[data-slug=check-a]')")
	say("put", "sent.join(' ; ')")
	say("put, told", lastToast)
	say("put, focus", "document.activeElement.id")
	out = append(out, "put, written: "+row("S300")+" / "+pageTask("check-a"))
	say("put, groups", groups)

	// Written meanwhile: refused, nothing put, said in the modal; the second goes, of the rev read again.
	b.eval("sent.length = 0")
	add("リリース")
	s.floe(t, "task", "--project", "proj", "--task", "S300", "--title", "Later")
	// Read again meanwhile, as a look does: the modal still sends the rev it opened at
	b.untilRead(b.reread(), "the tasks read again", "true")
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
	out = append(out, "stale, written: "+row("S300")+" / "+pageTask("memo-a"))

	// The write refused, as the system may: said, nothing written, and the next goes of the rev the modal opened at.
	b.eval("sent.length = 0; window.failPage = true; const was = window.fetch; window.fetch = (url, init) => { if (window.failPage && String(url).includes('/api/tasks') && init?.method === 'PATCH') { window.failPage = false; return Promise.resolve(new Response('no', { status: 500 })); } return was(url, init); }")
	add("案")
	query("note")
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.Enter)
	b.until("refused", "!document.getElementById('add-page-error').hidden")
	say("page refused", "document.getElementById('add-page-error').textContent + ' / ' + document.getElementById('add-page-dialog').open")
	b.pressKey(kb.Enter)
	b.until("closed", "!document.getElementById('add-page-dialog').open")
	say("page refused, then", "sent.join(' ; ')")
	out = append(out, "page refused, written: "+row("S300")+" / "+pageTask("note-a"))

	// Not reached: said in the modal, which stays open.
	b.eval("window.cutFetch = window.fetch; window.fetch = (url, init) => (String(url).startsWith('/api/tasks') && init?.method === 'PATCH' ? Promise.reject(new TypeError('Failed to fetch')) : window.cutFetch(url, init))")
	add("案")
	query("other")
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.Enter)
	b.moveYes()
	b.until("not reached", "!document.getElementById('add-page-error').hidden")
	say("not reached", "document.getElementById('add-page-error').textContent + ' / ' + document.getElementById('add-page-dialog').open")
	b.eval("window.fetch = window.cutFetch")
	b.pressKey(kb.Escape)
	b.until("closed", "!document.getElementById('add-page-dialog').open")

	// From another task, asked first: No, or Escape, puts nothing and leaves the modal, the search box focused and the page still marked, so
	// that Enter asks again. From another category of this one, nothing is asked. Either is said where it came from.
	b.eval("sent.length = 0")
	add("案")
	query("other")
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.Enter)
	b.until("asked", "document.getElementById('move-dialog').open")
	say("asked", "document.getElementById('move-lead').textContent + ' / ' + document.activeElement.id")
	b.pressKey(kb.Escape)
	// No answered as the dialog closes
	b.until("no", "!document.getElementById('move-dialog').open && document.activeElement.id === 'add-page-q'")
	say("no", "sent.length + ' ' + document.getElementById('add-page-dialog').open + ' ' + document.activeElement.id + ' ' + Boolean(document.getElementById('add-page-q').getAttribute('aria-activedescendant'))")
	b.pressKey(kb.Enter)
	b.moveYes()
	b.until("closed", "!document.getElementById('add-page-dialog').open")
	say("moved from a task", lastToast)
	add("リリース")
	query("build")
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.Enter)
	b.until("closed", "!document.getElementById('add-page-dialog').open")
	say("moved from a category", lastToast)
	say("moved, groups", groups)
	// The list read before the page left another task: asked as the list has it, and told as the write found it, not of the task it left
	release := b.staleReads("/api/pages", "(text) => { const l = JSON.parse(text); l.find((a) => a.slug === 'other-a').task = { name: 'S301', category: '設計' }; return JSON.stringify(l); }")
	b.untilRead(b.reread(), "the list as read before", "!document.querySelector('#list li[data-slug=other-a]')")
	add("案")
	query("other")
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.Enter)
	b.moveYes()
	b.until("closed", "!document.getElementById('add-page-dialog').open")
	release()
	say("moved since, told", lastToast)
	// A narrow screen: the modal fits.
	b.size(390, 844, true)
	add("リリース")
	say("narrow", "document.getElementById('add-page-dialog').scrollWidth <= document.getElementById('add-page-dialog').clientWidth")
	b.pressKey(kb.Escape)
	b.size(1400, 900, false)

	// Off its task by a pointer: the page alone written; its category stays, of no page now, the focus on the row after, or before; the way back
	// puts the page back on the task as the toast shows it, its rev with it, the task's row as it was.
	open("S300")
	menu("review-a", "task-out", false)
	b.until("off", "!document.querySelector('#list li[data-slug=review-a]')")
	say("off", "sent.join(' ; ')")
	say("off, told", lastToast)
	say("off, focus", "document.activeElement.closest('li')?.dataset.slug")
	say("off, groups", groups)
	b.eval("sent.length = 0; [...document.querySelectorAll('#toasts [data-testid=undo]')].at(-1).click()")
	b.until("back", "document.querySelector('#list li[data-slug=review-a]')")
	say("back", "sent.join(' ; ')")
	out = append(out, "back, written: "+row("S300")+" / "+pageTask("review-a"))
	// The way back after another wrote the task meanwhile: refused as stale and said, the page left off.
	menu("review-a", "task-out", false)
	b.until("off again", "!document.querySelector('#list li[data-slug=review-a]')")
	s.floe(t, "task", "--project", "proj", "--task", "S300", "--title", "Categories again")
	b.eval("[...document.querySelectorAll('#toasts [data-testid=undo]')].at(-1).click()")
	b.until("refused", "!document.getElementById('error').hidden")
	say("back, stale", "document.getElementById('error').textContent")
	out = append(out, "back, stale, written: "+pageTask("review-a"))
	s.floe(t, "task", "--slug", "review-a", "--task", "S300", "--category", "レビュー")
	open("S300")
	// From the keys: the focus on the way back; Escape leaves it on the row after the one taken off.
	menu("plan-a", "task-out", true)
	b.until("off", "!document.querySelector('#list li[data-slug=plan-a]')")
	say("keys, focus", "document.activeElement.dataset.testid")
	b.pressKey(kb.Escape)
	say("keys, escape", "document.activeElement.closest('li')?.dataset.slug + ' ' + document.querySelectorAll('#toasts [data-testid=undo]').length")

	// A page on a task of no row, as a hand edit leaves, taken off from the page list; its way back, pressed before the list is read again
	// too, puts it back on the task, which makes the row.
	b.navigate("/?project=proj")
	b.until("the rows", "document.querySelector('#list li[data-slug=g-a]')")
	record()
	release = b.holdReads(unsent, "/api/pages", "/api/tasks")
	menu("g-a", "task-out", false)
	b.until("the way back", "document.querySelector('#toasts [data-testid=undo]')")
	b.eval("document.querySelector('#toasts [data-testid=undo]').click()")
	b.until("written back", "answered === 2")
	release()
	say("back as it was", "sent.join(' ; ')")
	out = append(out, "back as it was, written: "+row("G2")+" / "+pageTask("g-a"))

	// Off its task from a page whole's bar, whose load reads no tasks: the way back is of the task's row as read then.
	b.navigate("/a/review-a/")
	b.until("the bar", "document.querySelector('#frame-actions [data-testid=more]')")
	record()
	pressMenuItem(b, "#frame-actions", "task-out")
	b.until("the way back", "document.querySelector('#toasts [data-testid=undo]')")
	b.eval("sent.length = 0; document.querySelector('#toasts [data-testid=undo]').click()")
	b.until("written back", "answered === 2")
	say("whole, back", "sent.join(' ; ') + ' ' + document.getElementById('error').hidden")
	out = append(out, "whole, back, written: "+pageTask("review-a"))

	// Only a page on a task has the item.
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=far-a]')")
	say("item", inMenuItem("#list li[data-slug=far-a]", "task-out", "i.hidden")+" + ' ' + "+inMenuItem("#list li[data-slug=build-a]", "task-out", "i.hidden + ' ' + i.getAttribute('aria-label')"))
	// From the task dialog, by the keys: closed first, then the focus on the way back; Escape leaves it on the menu it opened from.
	pressMenuItem(b, "#list li[data-slug=build-a]", "task-set")
	b.until("the dialog", "document.getElementById('set-dialog').open")
	b.eval("document.getElementById('set-out').click()")
	b.until("closed", "!document.getElementById('set-dialog').open && document.activeElement.dataset.testid === 'undo'")
	say("dialog, focus", "document.activeElement.dataset.testid")
	b.pressKey(kb.Escape)
	say("dialog, escape", "document.activeElement.tagName + ' ' + document.activeElement.closest('li')?.dataset.slug")
	out = append(out, "dialog, written: "+pageTask("build-a"))

	// Added under no category, as the modal opens again whatever was chosen before: the page of a category of this task moves under it, sent
	// with the category empty, and no category's heading comes; closed, the focus back on the head's button.
	open("S300")
	// Not opened over another modal, as a script's press reaches the button under it
	b.eval("document.getElementById('find-open').click()")
	b.until("the search", "document.getElementById('find').open")
	b.eval("document.getElementById('task-add-page').click()")
	say("none, under the search", "document.getElementById('add-page-dialog').open")
	b.pressKey(kb.Escape)
	b.until("the search closed", "!document.getElementById('find').open")
	add("")
	say("none, opened", "document.getElementById('add-page-title').textContent + ' / ' + document.getElementById('add-page-category').value")
	query("review")
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.Enter)
	b.until("closed", "!document.getElementById('add-page-dialog').open && document.querySelector('#list li[data-key=\"group:\"] + li[data-slug=review-a]')")
	say("none, put", "sent.join(' ; ').replace(/\"rev\":\\d+/, '\"rev\":n')")
	say("none, focus", "document.activeElement.id")
	out = append(out, "none, written: "+pageTask("review-a"))

	check(t, "task categories", out, strings.Join([]string{
		"groups: [No category] note-a [案] plan-a old-a [実装] build-a [リリース none] [レビュー] review-a",
		"keys apart: true",
		"named: 0 Add",
		"opened: Add a page / S300 Categories / add-page-q / null",
		"category: No category,案,実装,*リリース,レビュー",
		"options: g-a (G2 · x), other-a (S301 · 設計), check-a, memo-a, note-a (this task's no category), review-a (this task's レビュー), build-a (this task's 実装), plan-a (this task's 案)",
		"no link: 0",
		"nothing reached: 0 true",
		"narrowed: check-a",
		`put: /api/tasks {"slugs":["check-a"],"task":"S300","category":"リリース","new":false,"rev":6}`,
		"put, told: Put T check-a under リリース",
		"put, focus: task-add-page",
		"put, written: 7 / S300/リリース",
		"put, groups: [No category] note-a [案] plan-a old-a [実装] build-a [リリース] check-a [レビュー] review-a",
		"stale: Failed to save: task S300 has changed since it was read; the tasks are read again, so check and save again / true / memo-a",
		"stale, typed: true",
		`stale, then: /api/tasks {"slugs":["memo-a"],"task":"S300","category":"リリース","new":false,"rev":7} ; /api/tasks {"slugs":["memo-a"],"task":"S300","category":"リリース","new":false,"rev":8}`,
		"stale, written: 9 / S300/リリース",
		"page refused: Failed to save: no / true",
		`page refused, then: /api/tasks {"slugs":["note-a"],"task":"S300","category":"案","new":false,"rev":9}`,
		"page refused, written: 10 / S300/案",
		"not reached: Failed to save: Failed to fetch / true",
		"asked: “T other-a” is on S301's 設計. Moving it to S300 takes it off S301. / move-no",
		"no: 0 true add-page-q true",
		"moved from a task: Moved T other-a from S301's 設計 to 案",
		"moved from a category: Moved T build-a from 実装 to リリース",
		"moved, groups: [案] plan-a note-a other-a old-a [実装 none] [リリース] build-a memo-a check-a [レビュー] review-a",
		"moved since, told: Put T other-a under 案",
		"narrow: true",
		`off: /api/tasks {"slugs":["review-a"],"task":"","category":""}`,
		"off, told: Took T review-a off task S300Undo",
		"off, focus: check-a",
		"off, groups: [案] plan-a note-a other-a old-a [実装 none] [リリース] build-a memo-a check-a [レビュー none]",
		`back: /api/tasks {"slugs":["review-a"],"task":"S300","category":"レビュー","new":false,"rev":12}`,
		"back, written: 13 / S300/レビュー",
		"back, stale: Failed to save: task S300 has changed since it was read; the tasks are read again, so check and save again",
		"back, stale, written: none",
		"keys, focus: undo",
		"keys, escape: note-a 0",
		`back as it was: /api/tasks {"slugs":["g-a"],"task":"","category":""} ; /api/tasks {"slugs":["g-a"],"task":"G2","category":"x","new":true}`,
		"back as it was, written: 1 / G2/x",
		`whole, back: /api/tasks {"slugs":["review-a"],"task":"S300","category":"レビュー","new":false,"rev":15} true`,
		"whole, back, written: S300/レビュー",
		"item: true false Take T build-a off its task",
		"dialog, focus: undo",
		"dialog, escape: SUMMARY build-a",
		"dialog, written: none",
		"none, under the search: false",
		"none, opened: Add a page / ",
		`none, put: /api/tasks {"slugs":["review-a"],"task":"S300","category":"","new":false,"rev":n}`,
		"none, focus: task-add-page",
		"none, written: S300/",
	}, "|"))
}
