package browsertest_test

import (
	"strings"
	"testing"

	"github.com/chromedp/chromedp/kb"

	"floe/internal/store"
)

// TestUIEditFold: a page's "Edit" folds its slug and project under a row that says what they are, as typed, and opens closed. A field
// in the fold refused, by the browser's check or by the server, or a save that fails, opens it with that field focused; a refusal of a
// field outside it leaves it closed.
func TestUIEditFold(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	project := "proj"
	for _, slug := range []string{"a", "b"} {
		s.floe(t, "publish", s.page(t, "fold/"+slug+".yaml", "T "+slug, ""), "--slug", slug)
		if _, err := st.Edit(slug, store.Edit{Project: &project}); err != nil {
			t.Fatal(err)
		}
	}
	b.size(1400, 800, false)
	defer b.unsize()
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=a]')")

	var out []string
	say := func(what, expr string) {
		t.Helper()
		out = append(out, what+": "+b.eval(expr))
	}
	open := func() {
		t.Helper()
		b.eval("document.getElementById('edit-dialog').close()")
		b.eval("document.querySelector('#list li[data-slug=a] .more').open = true; document.querySelector('#list li[data-slug=a] .menu .edit').click()")
		b.until("the dialog", "document.getElementById('edit-dialog').open")
	}
	field := func(id, value string) {
		t.Helper()
		b.eval("const f = document.getElementById(" + js(id) + "); f.value = " + js(value) + "; f.dispatchEvent(new Event('input', { bubbles: true }))")
	}
	// Typed with the fold open, then folded and saved: saved open, a fold not opened by the refusal would show open all the same
	save := func(what string) {
		t.Helper()
		b.eval("document.getElementById('edit-more').open = false; document.getElementById('edit-error').hidden = true; document.getElementById('edit-submit').click()")
		b.until(what, "document.getElementById('edit-more').open || !document.getElementById('edit-error').hidden")
	}
	const fold = "document.getElementById('edit-more').open + ' / ' + document.getElementById('edit-more-now').textContent"
	const state = "document.getElementById('edit-more').open + ' / ' + document.activeElement.id + ' / ' + (document.getElementById('edit-error').hidden ? '-' : document.getElementById('edit-error').textContent)"

	open()
	say("opened", fold)
	field("edit-slug", " a-new ")
	say("typed", fold)
	field("edit-project", "  ")
	say("no project", fold)
	open()
	say("again", fold)

	field("edit-slug", "Bad Name")
	save("invalid")
	say("invalid", state+" + ' / ' + document.getElementById('edit-dialog').open")

	open()
	say("reopened after one opened", fold)
	field("edit-slug", "b")
	save("taken")
	say("taken", state)

	open()
	field("edit-project", "/")
	save("project refused")
	say("project refused", state)

	open()
	field("edit-slug", "a-x")
	b.eval("window.cutFetch = window.fetch; window.fetch = (url, init) => (init?.method === 'PATCH' ? Promise.reject(new TypeError('Failed to fetch')) : window.cutFetch(url, init))")
	save("not reached")
	say("not reached", state)

	// The title alone refused: nothing in the fold was sent, so it stays closed and the focus where it was
	open()
	field("edit-name", "Another")
	b.eval("window.fetch = (url, init) => (init?.method === 'PATCH' ? Promise.resolve(new Response('no', { status: 500 })) : window.cutFetch(url, init))")
	b.eval("document.getElementById('edit-name').focus()")
	save("title refused")
	say("title refused", state)
	b.eval("window.fetch = window.cutFetch")
	say("alert", "document.getElementById('edit-error').getAttribute('role')")

	// The keys: Enter and Space open and close it, the focus staying on the row; Escape closes the dialog.
	open()
	b.eval("document.querySelector('#edit-more > summary').focus()")
	b.pressKey(kb.Enter)
	say("enter", "document.getElementById('edit-more').open + ' / ' + document.activeElement.matches('#edit-more > summary')")
	b.pressKey(" ")
	say("space", "document.getElementById('edit-more').open + ' / ' + document.activeElement.matches('#edit-more > summary')")
	b.pressKey(kb.Escape)
	b.until("closed", "!document.getElementById('edit-dialog').open")

	// A page moved to another project is off its task, which is of its project: said so as it saves.
	s.floe(t, "work", "a", "--task", "#1")
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=a]')")
	open()
	field("edit-project", "else")
	b.eval("document.getElementById('edit-submit').click()")
	b.until("saved", "!document.getElementById('edit-dialog').open")
	say("moved", "[...document.querySelectorAll('#toasts .toast')].at(-1)?.textContent.trim()")

	check(t, "edit fold", out, strings.Join([]string{
		"opened: false / a · proj",
		"typed: false / a-new · proj",
		"no project: false / a-new · Not set",
		"again: false / a · proj",
		"invalid: true / edit-slug / - / true",
		"reopened after one opened: false / a · proj",
		`taken: true / edit-slug / Failed to save: HTTP 409: slug "b" is already in use: choose another slug`,
		`project refused: true / edit-project / Failed to save: HTTP 400: a project name cannot be just "/"`,
		"not reached: true / edit-slug / Failed to save: Failed to fetch",
		"title refused: false / edit-name / Failed to save: HTTP 500: no",
		"alert: alert",
		"enter: true / true",
		"space: false / true",
		"moved: Saved; off task #1, which is of the project it was in",
	}, "|"))
}
