package browsertest_test

import (
	"strconv"
	"strings"
	"testing"

	"floe/internal/store"
)

// A task's page writes the task as it drew it (rev): a write over a row written since is refused and the tasks read again, the archive button
// writes the task alone, a rev not drawn is not sent, and the family's button sends the rev drawn after a change of state. The looks at the
// list are kept stale, so that the screen keeps what it drew until a write reads it again.
func TestUITaskRev(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	project := "proj"
	s.floe(t, "publish", s.page(t, "pa.yaml", "T pa", ""), "--slug", "pa")
	if _, err := st.Edit("pa", store.Edit{Project: &project}); err != nil {
		t.Fatal(err)
	}
	s.floe(t, "task", "--slug", "pa", "--task", "#130", "--state", "doing")
	s.floe(t, "task", "--project", project, "--task", "#up", "--state", "doing")
	s.floe(t, "task", "--project", project, "--task", "#kid", "--state", "doing", "--parent", "#up")

	// The test's script: every write to the store kept as sent, and counted answered; down fails every read, pageDown a page's write
	const watch = `if (!window.sent) { window.sent = []; window.answered = 0; window.realFetch = fetch; window.down = false; window.pageDown = false;
		window.fetch = (u, o) => {
			if (!o?.method && window.down) return Promise.reject(new TypeError('down'));
			if (o?.method !== 'PATCH') return window.realFetch(u, o);
			if (window.pageDown && String(u).startsWith('/api/tasks') && String(o.body).includes('"slugs"')) return Promise.resolve(new Response('no', { status: 500 })).finally(() => { window.answered++; });
			window.sent.push(String(u).split('?')[0] + ' ' + o.body);
			return window.realFetch(u, o).finally(() => { window.answered++; });
		}; } 'watched'`
	open := func(name string) {
		t.Helper()
		b.navigate("/tasks?project=proj&task=" + strings.ReplaceAll(name, "#", "%23"))
		b.until("the head of "+name, "!document.getElementById('task-head').hidden && document.getElementById('task-title').textContent.startsWith("+js(name)+")")
		b.eval(watch)
	}
	var out []string
	say := func(what, expr string) {
		t.Helper()
		out = append(out, what+": "+b.eval(expr))
	}
	const (
		stateShown = "(document.getElementById('task-state').hidden ? 'archived' : document.getElementById('task-state').value)"
		errorShown = "(document.getElementById('error').hidden ? 'no error' : document.getElementById('error').textContent)"
		// The toasts come since seenToasts marks those there: the page's own toasts are left as they are.
		newUndo = "[...document.querySelectorAll('#toasts [data-testid=toast]')].filter((e) => !window.seenToasts?.has(e)).map((e) => e.querySelector('[data-testid=undo]')).find(Boolean)"
		seen    = "window.seenToasts = new WeakSet(document.querySelectorAll('#toasts [data-testid=toast]'))"
		parent  = "(document.querySelector('#task-up-rows [data-testid=kin-row]')?.dataset.task || 'no parent')"
	)
	// chosen chooses the state to, and waits until the page has drawn it written: n writes answered.
	chosen := func(to string, n int) {
		t.Helper()
		chooseValue(b, "task-state", to)
		b.until("the state "+to+" written", "window.answered === "+strconv.Itoa(n)+" && document.getElementById('task-state').dataset.state === "+js(to))
	}

	// Written elsewhere since it was drawn: refused, read again, said. The page's reads keep the tasks as drawn till then.
	open("#130")
	release := b.staleReads("/api/tasks", "")
	b.eval(`(async () => (await realFetch('/api/tasks', { method: 'PATCH', headers: { 'content-type': 'application/json' }, body: '{"taskProject":"proj","task":"#130","state":"done"}' })).status)()`)
	if got, err := st.ReadTasks(); err != nil || got["proj"]["#130"].Rev != 2 {
		t.Fatalf("written elsewhere: %+v, %v", got["proj"]["#130"], err)
	}
	begun := b.pollEnded()
	chooseValue(b, "task-state", "todo")
	b.untilRead(begun, "refused", "window.answered === 1 && !document.getElementById('error').hidden")
	b.staleOwn(begun)
	release()
	say("written since", "[window.sent.join(' ; ') || 'none', "+stateShown+", "+errorShown+"].join(' | ')")

	// The archive button, of the rev read again: the task alone, either way
	press := func(what, archivedShown string) {
		t.Helper()
		begun := b.acted("window.sent.length = 0; window.answered = 0; document.getElementById('task-archive').click()")
		b.untilRead(begun, what, "window.answered === 1 && document.getElementById('task-archived').hidden === "+archivedShown)
		say(what, "[window.sent.join(' ; '), "+stateShown+", ((r) => !r ? 'no pa' : r.dataset.archived === 'true' ? 'pa archived' : 'pa as it was')(document.querySelector('#list li[data-slug=pa]'))].join(' | ')")
	}
	press("archived", "false")
	press("given back", "true")

	// A rev not drawn: not sent
	release = b.staleReads("/api/tasks", "(text) => { const j = JSON.parse(text); j.proj['#130'].rev = 1.5; return JSON.stringify(j); }")
	b.untilRead(b.reread(), "a rev not whole read", "true")
	b.eval("window.sent.length = 0")
	chooseValue(b, "task-state", "todo")
	b.until("refused", "document.getElementById('error').textContent.includes('version')")
	release()
	say("no rev", "[window.sent.join(' ; ') || 'none', document.getElementById('error').textContent].join(' | ')")

	// The state changed, then the child taken off its parent: the row drawn again, of the rev written
	open("#kid")
	b.until("the parent's row", "document.querySelector('#task-up-rows [data-testid=unlink]')")
	chosen("todo", 1)
	b.eval("window.sent.length = 0; " + seen)
	b.eval("document.querySelector('#task-up-rows [data-testid=unlink]').click()")
	b.until("taken off", "window.answered === 2 && "+newUndo+" && !document.getElementById('task-parent-add').hidden")
	say("then off its parent", "[window.sent.join(' ; '), "+parent+", "+errorShown+"].join(' | ')")

	// The reads failing: each write is of the rev the one before it answered. The state twice, then the child off, back, and off again
	s.floe(t, "task", "--project", project, "--task", "#kid", "--parent", "#up")
	open("#kid")
	b.until("the parent's row", "document.querySelector('#task-up-rows [data-testid=unlink]')")
	b.eval("window.down = true")
	chosen("done", 1)
	chosen("doing", 2)
	say("reads failing, the state twice", "window.sent.join(' ; ') + ' | ' + "+stateShown)
	// The button drawn before the reads failed, which the page cannot draw again
	b.eval("window.sent.length = 0; window.answered = 0")
	off := func(n int) {
		t.Helper()
		b.eval(seen)
		b.eval("document.querySelector('#task-up-rows [data-testid=unlink]').click()")
		b.until("taken off", "window.answered === "+strconv.Itoa(n)+" && "+newUndo)
	}
	off(1)
	b.eval(newUndo + ".click()")
	b.until("given back", "window.answered === 2 && document.getElementById('toasts').textContent.includes('Undone')")
	off(3)
	all, err := st.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	after := all[project]["#kid"].Parent
	if after == "" {
		after = "no parent"
	}
	out = append(out, "reads failing, off, back, off: "+b.eval("window.sent.join(' ; ')")+" | "+after)
	b.eval("window.down = false")

	// The way back is of the screen as it was when taken off: a write since, its own state too, refuses it
	s.floe(t, "task", "--project", project, "--task", "#kid", "--parent", "#up")
	open("#kid")
	b.until("the parent's row", "document.querySelector('#task-up-rows [data-testid=unlink]')")
	b.eval(seen)
	r := b.rect("document.querySelector('#task-up-rows [data-testid=unlink]')")
	b.click(r.X+r.Width/2, r.Y+r.Height/2)
	b.until("taken off", "window.answered === 1 && "+newUndo+" && !document.getElementById('task-parent-add').hidden")
	chosen("todo", 2)
	b.eval(newUndo + ".click()")
	b.until("refused", "!document.getElementById('error').hidden")
	say("off, the state, back", "document.getElementById('error').textContent")

	// Set the task, its page's write refused: saving again is of the rev written, refused when another wrote it since, and else saved; a page
	// whole, which reads the tasks only as the dialog opens, too. Moved from another task, it is asked first, and Yes answered
	const setSave = `(async () => { const at = document.getElementById('set-error').textContent; const n = window.sent.length; document.getElementById('set-submit').click();
		const end = Date.now() + 5000;
		const yes = () => { if (document.getElementById('move-dialog').open) document.getElementById('move-yes').click(); };
		await new Promise((r) => { const look = () => (yes(), Date.now() < end && document.getElementById('set-dialog').open && document.getElementById('set-error').textContent === at ? setTimeout(look, 10) : r()); setTimeout(look, 10); });
		return document.getElementById('set-dialog').open ? document.getElementById('set-error').textContent + (Date.now() >= end ? ' (stuck: ' + window.sent.slice(n).join(' ; ') + ')' : '') : 'saved'; })()`
	setOnce := func(box, where string) {
		t.Helper()
		b.eval(watch)
		pressMenuItem(b, box, "task-set")
		b.until("the dialog", "document.getElementById('set-dialog').open")
		b.fill("#set-category", "next "+where)
		b.eval("window.pageDown = true")
		say(where+", the page refused", setSave)
		b.eval("window.pageDown = false")
	}
	b.navigate("/?project=proj")
	b.until("the list", "document.querySelector('#list li[data-slug=pa]')")
	setOnce("#list li[data-slug=pa]", "the list")
	// Written since and read by the page, the dialog showing what it wrote: refused
	s.floe(t, "task", "--project", project, "--task", "#130", "--title", "another's")
	b.untilRead(b.reread(), "read", "true")
	say("the list, written since", setSave)
	closeSet(b)
	// A new task, made by the save whose page's write is refused: saved again as there
	pressMenuItem(b, "#list li[data-slug=pa]", "task-set")
	b.until("the dialog", "document.getElementById('set-dialog').open")
	b.fill("#set-task", "#new")
	b.fill("#set-task-title", "made")
	b.eval("window.pageDown = true")
	say("a new task, the page refused", setSave)
	b.eval("window.pageDown = false")
	say("a new task, again", setSave)
	b.navigate("/a/pa/")
	b.until("the page", "document.querySelector('#frame-actions [data-testid=more]')")
	setOnce("#frame-actions", "whole")
	say("whole, again", setSave)

	check(t, "task revs", out, strings.Join([]string{
		`written since: /api/tasks {"taskProject":"proj","task":"#130","new":false,"rev":1,"state":"todo"} | done | Failed to change the state: task #130 has changed since it was read`,
		`archived: /api/tasks {"taskProject":"proj","task":"#130","new":false,"rev":2,"state":"archived"} | archived | pa as it was`,
		`given back: /api/tasks {"taskProject":"proj","task":"#130","new":false,"rev":3,"state":"done"} | done | pa as it was`,
		`no rev: none | Failed to change the state: the version of the task shown is not known; load the page again`,
		`then off its parent: /api/tasks {"taskProject":"proj","task":"#kid","new":false,"rev":2,"parent":""} | no parent | no error`,
		`reads failing, the state twice: /api/tasks {"taskProject":"proj","task":"#kid","new":false,"rev":4,"state":"done"} ; /api/tasks {"taskProject":"proj","task":"#kid","new":false,"rev":5,"state":"doing"} | doing`,
		`reads failing, off, back, off: /api/tasks {"taskProject":"proj","task":"#kid","new":false,"rev":6,"parent":""} ; /api/tasks {"taskProject":"proj","task":"#kid","new":false,"rev":7,"parent":"#up","parentNew":false} ; /api/tasks {"taskProject":"proj","task":"#kid","new":false,"rev":8,"parent":""} | no parent`,
		`off, the state, back: Failed to save: task #kid has changed since it was read`,
		`the list, the page refused: Failed to save: no`,
		`the list, written since: Failed to save: task #130 has changed since it was read; the tasks are read again, so check and save again`,
		`a new task, the page refused: Failed to save: no`,
		`a new task, again: saved`,
		`whole, the page refused: Failed to save: no`,
		`whole, again: saved`,
	}, "|"))
}
