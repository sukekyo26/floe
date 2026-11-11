package browsertest_test

import (
	"testing"

	"floe/internal/store"
)

// TestUIProjectPills opens the page list of a store of no page, of pages of no project and of pages of one project, and
// reads the project column of the shelf and the project pill of each row: the column shows with a single pill to choose
// (the "no project" one too) and none with no page; a row of a project has its pill, in the list and in the search, and
// one of no project has none; the rows of a task's page have none, the task's project being in its head.
func TestUIProjectPills(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	b.size(1400, 800, false)
	defer b.unsize()
	var out []string
	say := func(what, expr string) {
		t.Helper()
		out = append(out, what+": "+b.eval(expr))
	}
	const shelf = "[document.getElementById('shelf-projects').hidden, document.querySelectorAll('#chips .chip').length].join(' ')"
	const pills = "[...document.querySelectorAll('#list li[data-slug]')].map((li) => li.querySelector('.proj-name')?.textContent ?? '-').join(',')"

	b.navigate("/")
	b.until("the empty list", "document.getElementById('empty') && !document.getElementById('empty').hidden")
	say("no page", shelf)

	// A page is published into the project of the directory it is published from: taken out of it, it has none.
	none := ""
	for _, slug := range []string{"a", "b", "c"} {
		s.floe(t, "publish", s.page(t, slug+".yaml", "T "+slug, ""), "--slug", slug)
		if _, err := st.Edit(slug, store.Edit{Project: &none}); err != nil {
			t.Fatal(err)
		}
	}
	b.navigate("/")
	b.until("the rows", "document.querySelectorAll('#list li[data-slug]').length === 3")
	say("no project", "["+shelf+", "+pills+"].join(' ')")

	project := "floe"
	for _, slug := range []string{"a", "b", "c"} {
		if _, err := st.Edit(slug, store.Edit{Project: &project}); err != nil {
			t.Fatal(err)
		}
	}
	b.navigate("/")
	b.until("the rows", "document.querySelectorAll('#list li[data-slug]').length === 3 && document.querySelectorAll('#chips .chip').length > 0")
	say("one project", "["+shelf+", "+pills+", [...document.querySelectorAll('#chips .chip')].map((c) => c.dataset.project).join(',')].join(' ')")
	realKey(b, "k", 2)
	b.until("the search's rows", "document.querySelectorAll('#find-list li').length === 3")
	say("in the search", "[...document.querySelectorAll('#find-list li')].map((li) => li.querySelector('.proj-name')?.textContent ?? '-').join(',')")

	s.floe(t, "work", "a", "--task", "#1", "--category", "one")
	b.navigate("/tasks?project=floe&task=%231")
	b.until("the task's page", "document.querySelector('#list li[data-slug=a]') && document.querySelector('#task-project .proj-name')")
	say("on a task's page", "document.querySelector('#task-project .proj-name').textContent+' '+document.querySelectorAll('#list li[data-slug] .proj').length")

	// The pages to add to it are of its project too: none has a pill.
	b.eval("openAddPage(tasksOf().find((x) => x.name === '#1'), 'one')")
	b.until("the candidates", "document.querySelectorAll('#add-page-list li').length === 2")
	say("to add", "document.querySelectorAll('#add-page-list .proj').length")

	check(t, "project pills", out, "no page: true 0|no project: false 1 -,-,-|one project: false 1 floe,floe,floe floe|in the search: floe,floe,floe|on a task's page: floe 0|to add: 0")
}
