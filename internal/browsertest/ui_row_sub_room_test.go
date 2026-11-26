package browsertest_test

import (
	"testing"

	"floe/internal/store"
)

// TestUIRowSubRoom opens the page list, a task's page, the search and Add a page, wide and narrow, with linked pages
// whose project, task and tag have long names: each of what the line under a row's title holds, and each tag in it,
// ends within that line, cut or put on a line of its own rather than run under the row's buttons or out of the row or
// the modal; and a link's path keeps the room it asks for, or the whole line when that is narrower.
func TestUIRowSubRoom(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	project := "a-project-of-a-very-long-name-that-a-person-may-have-for-a-repository"
	for _, slug := range []string{"tagged", "long-task"} {
		s.floe(t, "publish", "--link", s.write(t, "some/folders/of/a/repository/"+slug+".html", "<title>T "+slug+"</title>"), "--slug", slug)
		if _, err := st.Edit(slug, store.Edit{Project: &project}); err != nil {
			t.Fatal(err)
		}
	}
	s.floe(t, "tag", "tagged", "--add", "a-tag-with-a-rather-long-name-too")
	s.floe(t, "task", "--slug", "tagged", "--task", "#1", "--category", "one")
	s.floe(t, "task", "--slug", "long-task", "--task", "a-task-of-a-very-long-name-for-the-badge-on-its-row", "--category", "one")
	defer b.unsize()
	for _, width := range []int64{1400, 400, 280} {
		b.size(width, 800, width != 1400)
		for _, screen := range []struct{ what, path, rows string }{
			{"the page list", "/", "2"},
			{"a task's page", "/tasks?project=" + project + "&task=%231", "1"},
		} {
			b.navigate(screen.path)
			b.until(screen.what, "document.querySelectorAll('#list li[data-slug] [data-testid=tagpill]').length === 1 && document.querySelectorAll('#list li[data-slug]').length === "+screen.rows)
			if got := b.eval(within("#list li[data-slug]")); got != "" {
				t.Errorf("%s at %d: %s", screen.what, width, got)
			}
		}
		b.navigate("/")
		b.until("the page list", "document.querySelectorAll('#list li[data-slug]').length === 2")
		realKey(b, "k", 2)
		b.until("the search's rows", "document.querySelectorAll('#find-list li').length === 2")
		if got := b.eval(within("#find-list li")); got != "" {
			t.Errorf("the search at %d: %s", width, got)
		}
		b.navigate("/tasks?project=" + project + "&task=%231")
		b.until("the task's page", "document.querySelectorAll('#list li[data-slug]').length === 1")
		b.eval("document.getElementById('task-add-page').click()")
		b.until("Add a page's row", "document.querySelector('#add-page-list li [data-testid=badge]') !== null")
		if got := b.eval(within("#add-page-list li")); got != "" {
			t.Errorf("Add a page at %d: %s", width, got)
		}
	}
}

// within tells of the rows rows names what of the line under each title runs past it (each tag too), and a path
// narrower than its basis while the line is wider; empty when none.
func within(rows string) string {
	return `[...document.querySelectorAll(` + js(rows) + `)].flatMap((li) => {
  const sub = li.querySelector('[data-testid=sub]'), end = sub.getBoundingClientRect().right + 0.5;
  const said = [...sub.children, ...sub.querySelectorAll('[data-testid=tagpill]')]
    .filter((x) => x.getBoundingClientRect().right > end)
    .map((x) => (li.dataset.slug ?? li.id) + ' ' + (x.dataset.testid || x.tagName) + ' past the line');
  const path = sub.querySelector('[data-testid=path]');
  if (path && path.clientWidth + 0.5 < Math.min(parseFloat(getComputedStyle(path).flexBasis), sub.clientWidth)) said.push((li.dataset.slug ?? li.id) + ' path of ' + path.clientWidth + 'px');
  return said;
}).join(', ')`
}
