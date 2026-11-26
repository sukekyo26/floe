package browsertest_test

import (
	"testing"

	"floe/internal/store"
)

// TestUIRowSubRoom opens the page list and a task's page, wide and narrow, with rows whose project, task and tag have
// long names: each of what the line under a row's title holds, and each tag in it, ends within that line, cut rather
// than run under the row's buttons or out of the row.
func TestUIRowSubRoom(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	project := "a-project-of-a-very-long-name-that-a-person-may-have-for-a-repository"
	for _, slug := range []string{"tagged", "long-task"} {
		s.floe(t, "publish", s.page(t, slug+".yaml", "T "+slug, ""), "--slug", slug)
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
			got := b.eval(`[...document.querySelectorAll('#list li[data-slug]')].flatMap((li) => {
  const sub = li.querySelector('[data-testid=sub]'), end = sub.getBoundingClientRect().right + 0.5;
  return [...sub.children, ...sub.querySelectorAll('[data-testid=tagpill]')]
    .filter((x) => x.getBoundingClientRect().right > end)
    .map((x) => li.dataset.slug + ' ' + (x.dataset.testid || x.tagName));
}).join(', ')`)
			if got != "" {
				t.Errorf("%s at %d: past the line under the title: %s", screen.what, width, got)
			}
		}
	}
}
