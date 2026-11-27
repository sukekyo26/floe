package browsertest_test

import (
	"strings"
	"testing"

	"github.com/chromedp/chromedp/kb"

	"floe/internal/store"
)

// A refusal under a field of a task's page goes as the field is typed in, the same words again too: the add modal of the
// edit modal, Add a page, and the family's modal.
func TestUIRetyped(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	project := "proj"
	for _, slug := range []string{"plan", "memo"} {
		s.floe(t, "publish", s.page(t, slug+".yaml", "T "+slug, ""), "--slug", slug)
		if _, err := st.Edit(slug, store.Edit{Project: &project}); err != nil {
			t.Fatal(err)
		}
	}
	s.floe(t, "task", "--slug", "plan", "--task", "S1", "--category", "c", "--new")
	b.navigate("/tasks?project=proj&task=S1")
	b.until("the task's rows", "document.querySelector('#list li[data-slug=plan]')")
	// Every write fails, as when the server is not reached
	b.eval("window.realFetch ??= window.fetch; window.fetch = (url, init) => (init?.method === 'PATCH' ? Promise.reject(new TypeError('Failed to fetch')) : window.realFetch(url, init))")

	var out []string
	// field typed with words, and with the same again: whether the refusal shown by shown is there, then gone
	retyped := func(what, field, words, shown string) {
		t.Helper()
		b.until(what+" refused", shown)
		b.fill(field, words)
		out = append(out, what+": "+b.eval("!("+shown+")"))
	}

	b.eval("document.getElementById('task-edit').click()")
	b.until("the edit", "document.getElementById('edit-task-dialog').open")
	b.eval("document.getElementById('edit-url-plus').click()")
	b.until("the add modal", "document.activeElement.id === 'edit-add-new'")
	b.fill("#edit-add-new", "ftp://x")
	b.pressKey(kb.Enter)
	retyped("edit add", "#edit-add-new", "ftp://x", "document.getElementById('edit-add-said').textContent !== ''")
	b.pressKey(kb.Escape)
	b.until("the add modal closed", "!document.getElementById('edit-add-dialog').open")
	b.pressKey(kb.Escape)
	b.until("the edit closed", "!document.getElementById('edit-task-dialog').open")

	b.eval("document.getElementById('task-add-page').click()")
	b.until("add a page", "document.getElementById('add-page-dialog').open")
	chooseValue(b, "add-page-category", "c")
	b.fill("#add-page-q", "memo")
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.Enter)
	retyped("add a page", "#add-page-q", "memo", "!document.getElementById('add-page-error').hidden")
	b.pressKey(kb.Escape)
	b.until("add a page closed", "!document.getElementById('add-page-dialog').open")

	b.eval("document.getElementById('task-child').click()")
	b.until("the family's modal", "document.getElementById('kin-dialog').open")
	b.fill("#kin-task", "C1")
	b.until("the title's field", "!document.getElementById('kin-title').closest('[hidden]')")
	b.fill("#kin-title", "Child")
	b.eval("document.getElementById('kin-submit').click()")
	retyped("a child", "#kin-task", "C1", "!document.getElementById('kin-error').hidden")
	b.pressKey(kb.Escape)

	check(t, "retyped", out, strings.Join([]string{
		"edit add: true",
		"add a page: true",
		"a child: true",
	}, "|"))
}
