package browsertest_test

import (
	"fmt"
	"strconv"
	"testing"

	"floe/internal/store"
)

// A press is dropped only while what it asks is asked already (pressOnce): asked the other way and back before the list is
// read again, it is written each time. A task taken off its parent, put back and taken off; archived and restored; a page
// archived, restored and archived. Asked the same before that (a star, a pin, a page off its task, the delete of the
// selected), it is written once. The reads are held throughout.
func TestUIPressOnce(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	project := "proj"
	s.floe(t, "publish", s.page(t, "p1.yaml", "T p1", ""), "--slug", "p1")
	if _, err := st.Edit("p1", store.Edit{Project: &project}); err != nil {
		t.Fatal(err)
	}
	s.floe(t, "task", "--slug", "p1", "--task", "#1", "--title", "kid")
	for _, slug := range []string{"p2", "p3", "p4"} {
		s.floe(t, "publish", s.page(t, slug+".yaml", "T "+slug, ""), "--slug", slug)
		if _, err := st.Edit(slug, store.Edit{Project: &project}); err != nil {
			t.Fatal(err)
		}
	}
	s.floe(t, "task", "--slug", "p2", "--task", "#1")
	for _, args := range [][]string{{"--task", "#3", "--title", "up"}, {"--task", "#1", "--parent", "#3"}, {"--task", "#9", "--state", "done"}} {
		s.floe(t, append([]string{"task", "--project", project}, args...)...)
	}
	const undo = "[...document.querySelectorAll('#toasts [data-testid=undo]')].at(-1)"
	// hold opens path and, with its reads held, counts the writes answered
	hold := func(path, ready string) func() {
		t.Helper()
		b.navigate(path)
		b.until("ready", ready)
		b.still()
		release := b.holdReads(unsent, "/api/pages", "/api/tasks")
		b.eval("window.answers = window.sent = 0; { const f = window.fetch; window.fetch = (u, i) => { if (i?.method) window.sent++; return f(u, i).then((r) => { if (i?.method === 'PATCH') window.answers++; return r; }); }; }")
		return release
	}
	press := func(what, expr string, answers int) {
		t.Helper()
		b.eval(expr + ".click()")
		b.until(what, "window.answers === "+strconv.Itoa(answers))
	}

	const unlink = "document.querySelector('#task-up-rows [data-testid=unlink]')"
	release := hold("/tasks?project=proj&task=%231", unlink)
	press("off", unlink, 1)
	b.until("the way back", undo)
	press("put back", undo, 2)
	b.until("the row back", unlink)
	press("off again", unlink, 3)
	release()
	all, err := st.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	if got := all[project]["#1"].Parent; got != "" {
		t.Errorf("off, back and off: parent %q", got)
	}

	const button = "document.getElementById('task-archive')"
	release = hold("/tasks?project=proj&task=%239", button+" && !"+button+".hidden")
	press("archived", button, 1)
	b.until("the button turned", button+".textContent === 'Restore'")
	press("restored", button, 2)
	b.until("the button turned back", button+".textContent === 'Archive'")
	release()
	if all, err = st.ReadTasks(); err != nil {
		t.Fatal(err)
	}
	if got := all[project]["#9"].State; got != "done" {
		t.Errorf("archived and restored: state %q", got)
	}

	const row = "document.querySelector('#list li[data-slug=p1] [data-testid=actions] > [data-testid=archive]')"
	release = hold("/", row)
	press("the page archived", row, 1)
	b.until("the way back", undo)
	press("the page restored", undo, 2)
	// Its row is drawn by the read, so it stands as it was
	press("the page archived again", row, 3)
	release()
	b.until("the row gone", "!"+row)
	if !st.ReadMeta("p1").Archived {
		t.Error("archived, restored and archived: not archived")
	}

	// Written, the read held: pressed again for the same, nothing is sent; taken off, put back and off, each is.
	// An item of its menu, opened for each press: the press before shut it
	const item = "(() => { const more = document.querySelector('#list li[data-slug=p2] [data-testid=more]'); if (!more.open) more.querySelector('summary').click(); return more.querySelector('[data-testid=%s]'); })()"
	star := "document.querySelector('#list li[data-slug=p2] [data-testid=actions] > [data-testid=star]')"
	pin, out := fmt.Sprintf(item, "pin-item"), fmt.Sprintf(item, "task-out")
	release = hold("/?project=proj", star)
	again := func(what, expr string, answers int) {
		t.Helper()
		press(what, expr, answers)
		b.eval(expr + ".click()")
		b.eval("new Promise((go) => setTimeout(go, 100))")
		if got := b.eval("window.sent"); got != strconv.Itoa(answers) {
			t.Errorf("%s, pressed again: %s writes sent, want %d", what, got, answers)
		}
	}
	again("starred", star, 1)
	again("pinned", pin, 2)
	again("taken off", out, 3)
	press("put back", undo, 4)
	again("off again", out, 5)
	release()
	if m := st.ReadMeta("p2"); !m.Favorite || m.Task.Name != "" {
		t.Errorf("starred, off, back and off: %+v", m)
	}

	// The selected: a delete pressed again as it goes sends nothing, and stays armed.
	release = hold("/?project=proj", "document.querySelector('#list li[data-slug=p3]')")
	b.eval("for (const x of ['p3', 'p4']) document.querySelector(`#list li[data-slug=${x}] [data-testid=sel]`).click(); 1")
	b.until("the bar", "!document.getElementById('bulk').hidden")
	// One held, so that the delete goes on, and refused at last, so that the bar stays
	b.eval("window.refuse = null; { const f = window.fetch; window.fetch = (u, i) => (i?.method === 'DELETE' && String(u).endsWith('/p4') ? (window.sent++, new Promise((go) => { window.refuse = () => go(new Response('no', { status: 500 })); })) : f(u, i)); }")
	const del = "document.getElementById('bulk-delete')"
	b.eval(del + ".click()")
	b.until("armed", del+".dataset.armed")
	b.eval(del + ".click()")
	b.until("the delete goes", "window.refuse")
	b.eval(del + ".click()")
	b.eval("new Promise((go) => setTimeout(go, 100))")
	if got := b.eval("window.sent + ' ' + " + del + ".dataset.armed"); got != "2 true" {
		t.Errorf("deleted, pressed again as it goes: %q, want 2 writes and the button armed", got)
	}
	b.eval("window.refuse()")
	b.until("the refusal", "!document.getElementById('error').hidden")
	release()
}
