package store_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"floe/internal/store"
)

// pageIn writes a page of project, on no task.
func pageIn(t *testing.T, s *store.Store, root, slug, project string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(root, slug), 0o700); err != nil {
		t.Fatal(err)
	}
	m := &store.Meta{Project: project, CreatedAt: hour(1), UpdatedAt: hour(1)}
	if err := s.WriteMeta(slug, m); err != nil {
		t.Fatal(err)
	}
}

// written is the bytes of tasks.json and every page's meta.json, for a refusal to leave as they were.
func written(t *testing.T, root string) string {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join(root, "*", "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	all := make([]string, 0, len(paths)+1)
	for _, path := range append(paths, filepath.Join(root, "tasks.json")) {
		b, err := os.ReadFile(path) //nolint:gosec // G304: the test's own store
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		all = append(all, path+"\n"+string(b))
	}
	return strings.Join(all, "\n")
}

// ChangeTasks writes pages and their tasks in one write: a task a page comes to be on has a row, made of the write's time and
// title when it had none, and the page's category among its categories, added at the end; a row that changes moves its
// rev on; the answer has every task the write is on. A refusal writes nothing.
func TestTaskWriteOne(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	pageIn(t, s, root, "a", "p")
	pageIn(t, s, root, "b", "q")
	change := func(w store.TaskChange) store.TaskChangeResult {
		t.Helper()
		got, err := s.ChangeTasks(w)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	row := func(project, name string) store.Task {
		t.Helper()
		all, err := s.ReadTasks()
		if err != nil {
			t.Fatal(err)
		}
		return all[project][name]
	}

	// A page on a task of no row makes it: todo, of the title given, rev 1.
	got := change(store.TaskChange{TaskEdit: store.TaskEdit{Title: ptr("T")}, Slugs: []string{"a"}, Task: ptr("#9")})
	if len(got.Tasks) != 1 || !got.Tasks[0].Made || got.Tasks[0].Row.Rev != 1 || got.Tasks[0].Row.Title != "T" || got.Tasks[0].Row.State != "todo" {
		t.Errorf("made: %+v", got.Tasks)
	}
	if m := s.ReadMeta("a"); m.Task == nil || m.Task.Name != "#9" {
		t.Errorf("the page: %+v", m.Task)
	}
	// On a task of a row, with nothing to change: as it was.
	got = change(store.TaskChange{Slugs: []string{"a"}, Task: ptr("#9")})
	if got.Tasks[0].Made || got.Tasks[0].Wrote || got.Tasks[0].Row.Rev != 1 {
		t.Errorf("as it was: %+v", got.Tasks)
	}
	// A category of the page, without --task, is added, and the rev moves; again, it is among them.
	got = change(store.TaskChange{Slugs: []string{"a"}, Category: ptr("x")})
	if r := row("p", "#9"); !slices.Equal(r.Categories, []string{"x"}) || r.Rev != 2 || !got.Tasks[0].Wrote {
		t.Errorf("a category added: %+v", r)
	}
	if got = change(store.TaskChange{Slugs: []string{"a"}, Category: ptr("x")}); got.Tasks[0].Wrote {
		t.Errorf("a category among them: %+v", got.Tasks)
	}
	// An order written, and a category not in it: at its end. The page moves within the write, so x may go.
	change(store.TaskChange{TaskEdit: store.TaskEdit{Categories: &[]string{"y"}}, Slugs: []string{"a"}, Category: ptr("z")})
	if r := row("p", "#9"); !slices.Equal(r.Categories, []string{"y", "z"}) {
		t.Errorf("an order and a category: %v", r.Categories)
	}
	// Two pages on tasks of two projects: each task written, both in the answer.
	change(store.TaskChange{Slugs: []string{"b"}, Task: ptr("#9")})
	got = change(store.TaskChange{TaskEdit: store.TaskEdit{State: ptr("done")}, Slugs: []string{"a", "b"}})
	if len(got.Tasks) != 2 || row("p", "#9").State != "done" || row("q", "#9").State != "done" {
		t.Errorf("two tasks: %+v", got.Tasks)
	}

	no, yes := false, true
	for _, c := range []struct {
		name     string
		w        store.TaskChange
		want     string
		conflict bool
	}{
		{"a category with pages left out", store.TaskChange{TaskEdit: store.TaskEdit{Categories: &[]string{"y"}}, TaskProject: ptr("p"), Task: ptr("#9")}, "category z of task #9 in project p has pages (a)", false},
		// Both tasks are there, as the screen says of one: the pages moved meanwhile, so it reads again
		{"what the screen saw, of two tasks", store.TaskChange{TaskEdit: store.TaskEdit{New: &no}, Slugs: []string{"a", "b"}}, "task #9 has changed since it was read", true},
		{"a parent of two projects", store.TaskChange{TaskEdit: store.TaskEdit{Parent: ptr("#1")}, Slugs: []string{"a", "b"}, Task: ptr("#9")}, "a parent goes with one task, and the pages are on tasks of 2 projects", false},
		{"a field of pages taken off", store.TaskChange{TaskEdit: store.TaskEdit{State: ptr("done")}, Slugs: []string{"a"}, Task: ptr("")}, "takes pages off their task changes no task", false},
		{"a category of a page of no task", store.TaskChange{Slugs: []string{"c"}, Category: ptr("x")}, "page c is on no task", false},
		{"new, there", store.TaskChange{TaskEdit: store.TaskEdit{New: &yes, Title: ptr("t")}, TaskProject: ptr("p"), Task: ptr("#9")}, "task #9 is there already", true},
		{"a page not there", store.TaskChange{Slugs: []string{"a", "none"}, Task: ptr("#1")}, `no page "none"`, false},
	} {
		pageIn(t, s, root, "c", "p")
		before := written(t, root)
		_, err := s.ChangeTasks(c.w)
		if _, conflict := errors.AsType[*store.Conflict](err); err == nil || !strings.Contains(told(err, root), c.want) || conflict != c.conflict {
			t.Errorf("%s: %v, want %s (a Conflict: %v)", c.name, err, c.want, c.conflict)
		}
		if after := written(t, root); after != before {
			t.Errorf("%s: written", c.name)
		}
	}
	// Nothing to write of a task of no row is not found, and makes none.
	if _, err := s.ChangeTasks(store.TaskChange{TaskProject: ptr("p"), Task: ptr("#5")}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("nothing of no row: %v", err)
	}
}

// A page that leaves its task, for another or none, is answered with the task and category it left; one that stays on
// its task, or comes from none, with none.
func TestTaskWriteOffTask(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	pageIn(t, s, root, "a", "p")
	off := func(w store.TaskChange) string {
		t.Helper()
		got, err := s.ChangeTasks(w)
		if err != nil {
			t.Fatal(err)
		}
		return got.Pages[0].OffTask + "/" + got.Pages[0].OffCategory
	}
	for _, c := range []struct {
		name string
		w    store.TaskChange
		want string
	}{
		{"from no task", store.TaskChange{Slugs: []string{"a"}, Task: ptr("#1"), Category: ptr("x")}, "/"},
		{"the same task, another category", store.TaskChange{Slugs: []string{"a"}, Task: ptr(" #1 "), Category: ptr("y")}, "/"},
		{"its pin", store.TaskChange{Slugs: []string{"a"}, Pin: ptr(true)}, "/"},
		{"another task, a category of the same name", store.TaskChange{Slugs: []string{"a"}, Task: ptr("#2"), Category: ptr("y")}, "#1/y"},
		{"another task, no category", store.TaskChange{Slugs: []string{"a"}, Task: ptr("#3")}, "#2/y"},
		{"off its task", store.TaskChange{Slugs: []string{"a"}, Task: ptr("")}, "#3/"},
		{"cleared, from no task", store.TaskChange{Slugs: []string{"a"}, Clear: true}, "/"},
	} {
		if got := off(c.w); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

// A page whose project changes is taken off its task, its pin kept, the tasks as they were; put on a task again, it is
// one of the new project.
func TestEditProjectTakesOffTask(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	pageIn(t, s, root, "a", "p")
	if _, err := s.ChangeTasks(store.TaskChange{Slugs: []string{"a"}, Task: ptr("#9"), Category: ptr("x"), Pin: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	// The same project written with spaces round it is the same project
	if _, err = s.Edit("a", store.Edit{Project: ptr(" p ")}); err != nil {
		t.Fatal(err)
	}
	if w := s.ReadMeta("a").Task; w == nil || w.Name != "#9" {
		t.Errorf("the same project: %+v", w)
	}
	edited, err := s.Edit("a", store.Edit{Project: ptr("q")})
	if err != nil || edited.OffTask != "#9" || edited.OffCategory != "x" {
		t.Errorf("moved: %+v, %v; want it said off #9, category x", edited, err)
	}
	if w := s.ReadMeta("a").Task; w == nil || w.Name != "" || w.Category != "" || !w.Pin {
		t.Errorf("moved: %+v", w)
	}
	after, err := os.ReadFile(filepath.Join(root, "tasks.json"))
	if err != nil || string(after) != string(before) {
		t.Errorf("moved, tasks.json written: %s", after)
	}
	if _, err = s.Edit("a", store.Edit{Project: ptr("p")}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ChangeTasks(store.TaskChange{Slugs: []string{"a"}, Task: ptr("#9")}); err != nil {
		t.Fatal(err)
	}
	if w := s.ReadMeta("a").Task; w == nil || w.Name != "#9" {
		t.Errorf("moved and named: %+v", w)
	}
}

// A parent goes on the tasks of pages of one project, however many tasks they are on; of two projects it is refused.
func TestTaskParentOfPages(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	pageIn(t, s, root, "x", "p")
	pageIn(t, s, root, "y", "p")
	for _, w := range []store.TaskChange{
		{TaskEdit: store.TaskEdit{Title: ptr("parent")}, TaskProject: ptr("p"), Task: ptr("#P")},
		{Slugs: []string{"x"}, Task: ptr("#1")},
		{Slugs: []string{"y"}, Task: ptr("#2")},
		{TaskEdit: store.TaskEdit{Parent: ptr("#P")}, Slugs: []string{"x", "y"}},
	} {
		if _, err := s.ChangeTasks(w); err != nil {
			t.Fatal(err)
		}
	}
	all, err := s.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	if all["p"]["#1"].Parent != "#P" || all["p"]["#2"].Parent != "#P" {
		t.Errorf("two tasks of one project: %v", all["p"])
	}
}
