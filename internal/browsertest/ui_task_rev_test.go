package browsertest_test

import (
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/store"
)

// A task's page writes the task as it drew it (rev): a write over a row written since is refused and the tasks read again, the archive button
// writes the task alone, a rev not drawn is not sent, and the family's button sends the rev drawn after a change of state. The looks at the
// list are stopped, so that the screen keeps what it drew until a write reads it again.
func TestUITaskRev(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	project := "proj"
	s.floe(t, "publish", s.page(t, "pa.yaml", "T pa", ""), "--slug", "pa")
	if _, err := st.Edit("pa", store.Edit{Project: &project}); err != nil {
		t.Fatal(err)
	}
	s.floe(t, "work", "pa", "--task", "#130", "--state", "doing")
	s.floe(t, "work", "--project", project, "--task", "#up", "--state", "doing")
	s.floe(t, "work", "--project", project, "--task", "#kid", "--state", "doing", "--parent", "#up")

	// The page's script: the looks stopped, every write to the store kept as sent; down fails every read, pageDown a page's write
	const watch = `if (!window.realLoad) { window.realLoad = load; load = (polled) => (polled ? Promise.resolve() : realLoad(polled));
		window.sent = []; window.realFetch = fetch; window.down = false; window.pageDown = false;
		fetch = (u, o) => {
			if (!o?.method && down) return Promise.reject(new TypeError('down'));
			if (o?.method === 'PATCH' && pageDown && String(u).startsWith('/api/artifacts/')) return Promise.resolve(new Response('no', { status: 500 }));
			if (o?.method === 'PATCH') sent.push(String(u).split('?')[0] + ' ' + o.body);
			return realFetch(u, o);
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
	const state = `(async () => { const sel = document.getElementById('task-state'); sel.value = 'waiting'; await changeTaskState();
		return [sent.join(' ; ') || 'none', tasksOf().find((x) => x.name === '#130').state, document.getElementById('task-state').hidden ? 'archived' : document.getElementById('task-state').value,
			document.getElementById('error').hidden ? 'no error' : document.getElementById('error').textContent].join(' | '); })()`

	// Written elsewhere since it was drawn: refused, read again, said
	open("#130")
	b.eval(`(async () => (await realFetch('/api/tasks?project=proj&task=%23130', { method: 'PATCH', headers: { 'content-type': 'application/json' }, body: '{"state":"done"}' })).status)()`)
	if got, err := st.ReadTasks(); err != nil || got["proj"]["#130"].Rev != 2 {
		t.Fatalf("written elsewhere: %+v, %v", got["proj"]["#130"], err)
	}
	say("written since", state)

	// The archive button, of the rev read again: the task alone, either way
	const press = `(async () => { sent.length = 0; document.getElementById('task-archive').click(); await new Promise((r) => { const look = () => (archivingTask ? setTimeout(look, 10) : r()); setTimeout(look, 10); });
		return [sent.join(' ; '), tasksOf().find((x) => x.name === '#130').state, items.find((a) => a.slug === 'pa').archived ? 'pa archived' : 'pa as it was'].join(' | '); })()`
	say("archived", press)
	say("given back", press)

	// A rev not drawn: not sent
	say("no rev", `(async () => { sent.length = 0; showError(''); delete document.getElementById('task-state').dataset.rev; const sel = document.getElementById('task-state'); sel.value = 'waiting'; await changeTaskState();
		return [sent.join(' ; ') || 'none', document.getElementById('error').textContent].join(' | '); })()`)

	// The state changed, then the child taken off its parent: the row drawn again, of the rev written
	open("#kid")
	b.until("the parent's row", "document.querySelector('#task-up-rows .unlink')")
	say("then off its parent", `(async () => { const sel = document.getElementById('task-state'); sel.value = 'waiting'; await changeTaskState(); sent.length = 0;
		document.querySelector('#task-up-rows .unlink').click();
		await new Promise((r) => { const look = () => (unlinking.size ? setTimeout(look, 10) : r()); setTimeout(look, 10); });
		return [sent.join(' ; '), tasksOf().find((x) => x.name === '#kid').parent || 'no parent', document.getElementById('error').hidden ? 'no error' : document.getElementById('error').textContent].join(' | '); })()`)

	// The reads failing: each write is of the rev the one before it answered. The state twice, then the child off, back, and off again
	s.floe(t, "work", "--project", project, "--task", "#kid", "--parent", "#up")
	open("#kid")
	b.eval("down = true")
	const twice = `(async () => { sent.length = 0; const sel = document.getElementById('task-state');
		for (const to of ['done', 'doing']) { sel.value = to; await changeTaskState(); }
		return sent.join(' ; ') + ' | ' + tasksRead.tasks.proj['#kid'].state; })()`
	say("reads failing, the state twice", twice)
	const offBackOff = `(async () => { sent.length = 0; showError('');
		// The button drawn before the reads failed, which the page cannot draw again
		const off = async () => {
			const n = sent.length;
			document.querySelector('#task-up-rows .unlink').click();
			await new Promise((r) => { const look = () => (sent.length === n || unlinking.size ? setTimeout(look, 10) : r()); look(); });
		};
		await off();
		document.querySelector('#toasts .undo').click();
		await new Promise((r) => { const look = () => (sent.length < 2 ? setTimeout(look, 10) : setTimeout(r, 50)); look(); });
		await off();
		return sent.join(' ; ') + ' | ' + (tasksRead.tasks.proj['#kid'].parent || 'no parent'); })()`
	say("reads failing, off, back, off", offBackOff)
	b.eval("down = false; document.getElementById('toasts').replaceChildren()")

	// The way back is of the screen as it was when taken off: a write since, its own state too, refuses it
	s.floe(t, "work", "--project", project, "--task", "#kid", "--parent", "#up")
	open("#kid")
	say("off, the state, back", `(async () => { sent.length = 0; showError('');
		await unlinkTask(tasksOf().find((x) => x.name === '#kid'), '#up', false);
		const sel = document.getElementById('task-state'); sel.value = 'waiting'; await changeTaskState();
		document.querySelector('#toasts .undo').click();
		await new Promise((r) => { const look = () => (document.getElementById('error').hidden ? setTimeout(look, 10) : r()); look(); });
		return document.getElementById('error').textContent; })()`)

	// Set the task, its page's write refused: saving again is of the rev written, refused when another wrote it since, and else saved; a page
	// whole, which reads the tasks only as the dialog opens, too
	const setSave = `(async () => { const at = document.getElementById('set-error').textContent; const n = sent.length; document.getElementById('set').requestSubmit();
		const end = Date.now() + 5000;
		await new Promise((r) => { const look = () => (Date.now() < end && document.getElementById('set-dialog').open && document.getElementById('set-error').textContent === at ? setTimeout(look, 10) : r()); setTimeout(look, 10); });
		return document.getElementById('set-dialog').open ? document.getElementById('set-error').textContent + (Date.now() >= end ? ' (stuck: ' + sent.slice(n).join(' ; ') + ')' : '') : 'saved'; })()`
	setOnce := func(where string) {
		t.Helper()
		b.eval(watch)
		b.eval("openTaskSet(items.find((a) => a.slug === 'pa'))")
		b.until("the dialog", "document.getElementById('set-dialog').open")
		b.eval("document.getElementById('set-step').value = 'next " + where + "'; pageDown = true")
		say(where+", the page refused", setSave)
		b.eval("pageDown = false")
	}
	b.navigate("/?project=proj")
	b.until("the list", "items.some((a) => a.slug === 'pa')")
	setOnce("the list")
	// Written since and read by the page, the dialog showing what it wrote: refused
	s.floe(t, "work", "--project", project, "--task", "#130", "--title", "another's")
	b.eval("(async () => { await realLoad(); return 'read'; })()")
	say("the list, written since", setSave)
	b.eval("document.getElementById('set-dialog').close()")
	// A new task, made by the save whose page's write is refused: saved again as there
	b.eval("openTaskSet(items.find((a) => a.slug === 'pa'))")
	b.until("the dialog", "document.getElementById('set-dialog').open")
	b.eval("document.getElementById('set-task').value = '#new'; document.getElementById('set-task').dispatchEvent(new Event('input', { bubbles: true })); document.getElementById('set-task-title').value = 'made'; pageDown = true")
	say("a new task, the page refused", setSave)
	b.eval("pageDown = false")
	say("a new task, again", setSave)
	b.navigate("/a/pa/")
	b.until("the page", "items.some((a) => a.slug === 'pa')")
	setOnce("whole")
	say("whole, again", setSave)

	check(t, "task revs", out, strings.Join([]string{
		`written since: /api/tasks {"new":false,"rev":1,"state":"waiting"} | done | done | Failed to change the state: task #130 has changed since it was read`,
		`archived: /api/tasks {"new":false,"rev":2,"state":"archived"} | archived | pa as it was`,
		`given back: /api/tasks {"new":false,"rev":3,"state":"done"} | done | pa as it was`,
		`no rev: none | Failed to change the state: the version of the task shown is not known; load the page again`,
		`then off its parent: /api/tasks {"new":false,"rev":2,"parent":""} | no parent | no error`,
		`reads failing, the state twice: /api/tasks {"new":false,"rev":4,"state":"done"} ; /api/tasks {"new":false,"rev":5,"state":"doing"} | doing`,
		`reads failing, off, back, off: /api/tasks {"new":false,"rev":6,"parent":""} ; /api/tasks {"new":false,"rev":7,"parent":"#up","parentNew":false} ; /api/tasks {"new":false,"rev":8,"parent":""} | no parent`,
		`off, the state, back: Failed to save: task #kid has changed since it was read`,
		`the list, the page refused: Failed to save: HTTP 500: no`,
		`the list, written since: Failed to save: task #130 has changed since it was read; the tasks are read again, so check and save again`,
		`a new task, the page refused: Failed to save: HTTP 500: no`,
		`a new task, again: saved`,
		`whole, the page refused: Failed to save: HTTP 500: no`,
		`whole, again: saved`,
	}, "|"))
}
