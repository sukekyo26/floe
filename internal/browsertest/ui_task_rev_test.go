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

	// The page's script: the looks stopped, and every write to the store kept as sent
	const watch = `window.realLoad = load; load = (polled) => (polled ? Promise.resolve() : realLoad(polled));
		window.sent = []; window.realFetch = fetch;
		fetch = (u, o) => { if (o?.method === 'PATCH') sent.push(String(u).split('?')[0] + ' ' + o.body); return realFetch(u, o); }; 'watched'`
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

	check(t, "task revs", out, strings.Join([]string{
		`written since: /api/tasks {"new":false,"rev":1,"state":"waiting"} | done | done | The state could not be changed: task #130 has changed since it was read`,
		`archived: /api/tasks {"new":false,"rev":2,"state":"archived"} | archived | pa as it was`,
		`given back: /api/tasks {"new":false,"rev":3,"state":"done"} | done | pa as it was`,
		`no rev: none | The state could not be changed: the task shown could not be read; load the page again`,
		`then off its parent: /api/tasks {"new":false,"rev":2,"parent":""} | no parent | no error`,
	}, "|"))
}
