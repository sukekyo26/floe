package browsertest_test

import (
	"strings"
	"testing"

	"floe/internal/store"
)

// A task's ID changed in its Edit: the screen moves to the task's page by the new ID, with its pages and its child;
// an ID the project has is refused in the dialog, which stays open over the task.
func TestUITaskRename(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	project := "proj"
	s.floe(t, "publish", s.page(t, "a.yaml", "T a", ""), "--slug", "a")
	if _, err := st.Edit("a", store.Edit{Project: &project}); err != nil {
		t.Fatal(err)
	}
	s.floe(t, "task", "--slug", "a", "--task", "#1", "--category", "design", "--state", "doing")
	s.floe(t, "task", "--project", "proj", "--task", "#2", "--parent", "#1")
	b.size(1400, 800, false)
	defer b.unsize()

	edit := func() {
		t.Helper()
		b.eval("document.getElementById('task-edit').click()")
		b.until("the dialog", "document.getElementById('edit-task-dialog').open")
	}
	b.navigate("/tasks?project=proj&task=%231")
	b.until("the page of #1", taskPageShown)
	edit()
	if got := b.eval("document.getElementById('edit-task-name').value"); got != "#1" {
		t.Errorf("the ID field: %q", got)
	}
	b.fill("#edit-task-name", " #7 ")
	b.eval("document.getElementById('edit-task-submit').click()")
	b.until("the page of #7", "location.search === '?project=proj&task=%237'")
	b.until("its page and child", taskPageShown+" && document.querySelector('#task-family [data-testid=kin-row][data-task=\"#2\"]')")
	if got := b.eval(taskPageRows); got != "[design] | T a" {
		t.Errorf("the pages of #7: %s", got)
	}
	all, err := st.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	if _, old := all["proj"]["#1"]; old || all["proj"]["#2"].Parent != "#7" {
		t.Errorf("the rows: %+v", all["proj"])
	}
	if m := st.ReadMeta("a"); m == nil || m.Task == nil || m.Task.Name != "#7" || m.Task.Category != "design" {
		t.Errorf("the page: %+v", m)
	}

	edit()
	b.fill("#edit-task-name", "#2")
	b.eval("document.getElementById('edit-task-submit').click()")
	b.until("the refusal", "!document.getElementById('edit-task-error').hidden")
	if got := b.eval("document.getElementById('edit-task-error').textContent"); !strings.Contains(got, "#2") {
		t.Errorf("the refusal: %q", got)
	}
	if got := b.eval("document.getElementById('edit-task-dialog').open + ' ' + location.search"); got != "true ?project=proj&task=%237" {
		t.Errorf("after the refusal: %s", got)
	}
}
