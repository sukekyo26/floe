package browsertest_test

import (
	"strconv"
	"testing"

	"floe/internal/store"
)

// A press is dropped only while what it asks is asked already (pressOnce): asked the other way and back before the list is
// read again, it is written each time. A task taken off its parent, put back and taken off; archived and restored; a page
// archived, restored and archived. The reads are held throughout.
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
		b.eval("window.answers = 0; { const f = window.fetch; window.fetch = (u, i) => f(u, i).then((r) => { if (i?.method === 'PATCH') window.answers++; return r; }); }")
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
	b.until("off", "!"+unlink)
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
}
