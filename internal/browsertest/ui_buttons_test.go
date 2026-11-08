package browsertest_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/store"
)

// Every button of words is of one form, as drawn: 2rem high, 2.25rem at a modal's foot, its corners --corner-small. They
// are gathered by their class, .btn, in each place they show, every modal of the management page among them.
// LIMIT: a button of words that does not have .btn is not found: the rule in .claude/rules/ui.md tells its writer.
func TestUIButtons(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	b.navigate("/")
	project := "proj"
	for _, slug := range []string{"a", "b"} {
		s.floe(t, "publish", s.page(t, slug+".yaml", "T "+slug, ""), "--slug", slug)
		if _, err := st.Edit(slug, store.Edit{Project: &project}); err != nil {
			t.Fatal(err)
		}
	}
	s.floe(t, "work", "a", "--task", "#1", "--title", "one", "--state", "done", "--step", "design")
	s.floe(t, "work", "--project", project, "--task", "#2", "--title", "no page")

	const measure = `(() => {
		const small = getComputedStyle(document.documentElement).getPropertyValue('--corner-small').trim();
		const shown = [...document.querySelectorAll('.btn')].filter((e) => e.getClientRects().length && !e.closest('dialog:not([open])'));
		const bad = shown.filter((e) => {
			const r = e.getBoundingClientRect();
			return Math.abs(r.height - (e.closest('.dialog-actions') ? 36 : 32)) > 0.5 || getComputedStyle(e).borderTopLeftRadius !== small;
		}).map((e) => (e.id || e.className) + ' ' + Math.round(e.getBoundingClientRect().height) + ' ' + getComputedStyle(e).borderTopLeftRadius);
		return shown.length + '|' + bad.join(', ');
	})()`
	check := func(scene string) {
		t.Helper()
		b.still()
		n, bad, _ := strings.Cut(b.eval(measure), "|")
		if count, err := strconv.Atoi(n); err != nil || count == 0 || bad != "" {
			t.Errorf("%s: %s buttons, of another form: %s", scene, n, bad)
		}
	}
	opened := 0
	modal := func(id, open string) {
		t.Helper()
		b.eval(open)
		b.until(id, "document.getElementById("+js(id)+").open")
		check(id)
		b.eval("document.getElementById(" + js(id) + ").close()")
		opened++
	}
	menu := func(slug, item string) string {
		return "document.querySelector('#list li[data-slug=" + slug + "] .more').open = true; document.querySelector('#list li[data-slug=" + slug + "] .menu ." + item + "').click()"
	}

	b.size(1400, 800, false)
	defer b.unsize()
	b.navigate("/?project=proj")
	b.until("the rows", "document.querySelector('#list li[data-slug=b]')")
	check("the page list")
	modal("add-dialog", "document.getElementById('add-open').click()")
	modal("edit-dialog", menu("b", "edit"))
	modal("set-dialog", menu("b", "task-set"))
	modal("find", "openFind()")
	modal("keys", "openKeys()")
	b.eval("for (const s of [...document.querySelectorAll('#list li:not(.group) .sel')].slice(0, 2)) s.click()")
	b.until("the bar", "!document.getElementById('bulk').hidden")
	check("the bar of the selected")
	modal("tag-dialog", "document.getElementById('bulk-tag').click()")
	b.eval("document.getElementById('bulk-delete').click()")
	check("the delete asked again")
	b.eval("document.getElementById('bulk-clear').click()")
	b.eval("document.querySelector('#list li[data-slug=b] .actions > .archive').click()")
	b.until("the toast", "document.querySelector('.toast .btn')")
	// The toast comes before the list is read again, which draws the rows anew
	b.untilActed("the archive done", "true")
	check("the toast")

	b.navigate("/tasks?project=proj")
	b.until("the task list's button", "document.getElementById('task-add').getClientRects().length")
	check("the task list")

	b.navigate("/tasks?project=proj&task=%231")
	b.until("the head", "!document.getElementById('task-archive').hidden")
	check("a task done")
	modal("kin-dialog", "document.getElementById('task-child').click()")
	modal("edit-task-dialog", "document.getElementById('task-edit').click()")
	modal("add-page-dialog", "document.querySelector('#list .step-add').click()")
	b.navigate("/tasks?project=proj&task=%232")
	b.until("no pages", "!document.getElementById('task-none').hidden")
	check("a task of no page")

	b.size(400, 800, true)
	b.navigate("/?project=proj")
	b.until("the rows", "document.querySelector('#list li[data-slug=a]')")
	modal("filter-dialog", "document.getElementById('narrow-filter').click()")

	// A modal with no way here is a modal not measured.
	if got := b.eval("document.querySelectorAll('dialog').length + ''"); got != strconv.Itoa(opened) {
		t.Errorf("%s modals, %d opened", got, opened)
	}
}
