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

// pageIn writes a page of project, on task with category when task is not "", as a hand edit or an older floe left it.
func pageIn(t *testing.T, s *store.Store, root, slug, project, task, category string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(root, slug), 0o700); err != nil {
		t.Fatal(err)
	}
	m := &store.Meta{Project: project, CreatedAt: hour(1), UpdatedAt: hour(1)}
	if task != "" {
		m.Work = &store.Work{Task: task, Category: category}
	}
	if err := s.WriteMeta(slug, m); err != nil {
		t.Fatal(err)
	}
}

// written is the bytes of tasks.json and every page's meta.json, for a refusal to leave as they were.
func written(t *testing.T, root string) string {
	t.Helper()

	var all []string
	paths, err := filepath.Glob(filepath.Join(root, "*", "meta.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range append(paths, filepath.Join(root, "tasks.json")) {
		b, err := os.ReadFile(path) //nolint:gosec // G304: the test's own store
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		all = append(all, path+"\n"+string(b))
	}
	return strings.Join(all, "\n")
}

// Work writes pages and their tasks in one write: a task a page comes to be on has a row, made of the write's time and
// title when it had none, and the page's category among its categories, added at the end; a row that changes moves its
// rev on; the answer has every task the write is on. A refusal writes nothing.
func TestWorkWriteOne(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	pageIn(t, s, root, "a", "p", "", "")
	pageIn(t, s, root, "b", "q", "", "")
	work := func(w store.WorkWrite) store.WorkWritten {
		t.Helper()
		got, err := s.Work(w)
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
	got := work(store.WorkWrite{TaskEdit: store.TaskEdit{Title: ptr("T")}, Slugs: []string{"a"}, Task: ptr("#9")})
	if len(got.Tasks) != 1 || !got.Tasks[0].Made || got.Tasks[0].Row.Rev != 1 || got.Tasks[0].Row.Title != "T" || got.Tasks[0].Row.State != "todo" {
		t.Errorf("made: %+v", got.Tasks)
	}
	if m := s.ReadMeta("a"); m.Work == nil || m.Work.Task != "#9" {
		t.Errorf("the page: %+v", m.Work)
	}
	// On a task of a row, with nothing to change: as it was.
	got = work(store.WorkWrite{Slugs: []string{"a"}, Task: ptr("#9")})
	if got.Tasks[0].Made || got.Tasks[0].Wrote || got.Tasks[0].Row.Rev != 1 {
		t.Errorf("as it was: %+v", got.Tasks)
	}
	// A category of the page, without --task, is added, and the rev moves; again, it is among them.
	got = work(store.WorkWrite{Slugs: []string{"a"}, Category: ptr("x")})
	if r := row("p", "#9"); !slices.Equal(r.Categories, []string{"x"}) || r.Rev != 2 || !got.Tasks[0].Wrote {
		t.Errorf("a category added: %+v", r)
	}
	if got = work(store.WorkWrite{Slugs: []string{"a"}, Category: ptr("x")}); got.Tasks[0].Wrote {
		t.Errorf("a category among them: %+v", got.Tasks)
	}
	// An order written, and a category not in it: at its end. The page moves within the write, so x may go.
	work(store.WorkWrite{TaskEdit: store.TaskEdit{Categories: &[]string{"y"}}, Slugs: []string{"a"}, Category: ptr("z")})
	if r := row("p", "#9"); !slices.Equal(r.Categories, []string{"y", "z"}) {
		t.Errorf("an order and a category: %v", r.Categories)
	}
	// Two pages on tasks of two projects: each task written, both in the answer.
	work(store.WorkWrite{Slugs: []string{"b"}, Task: ptr("#9")})
	got = work(store.WorkWrite{TaskEdit: store.TaskEdit{State: ptr("done")}, Slugs: []string{"a", "b"}})
	if len(got.Tasks) != 2 || row("p", "#9").State != "done" || row("q", "#9").State != "done" {
		t.Errorf("two tasks: %+v", got.Tasks)
	}

	no, yes := false, true
	for _, c := range []struct {
		name     string
		w        store.WorkWrite
		want     string
		conflict bool
	}{
		{"a category with pages left out", store.WorkWrite{TaskEdit: store.TaskEdit{Categories: &[]string{"y"}}, Project: ptr("p"), Task: ptr("#9")}, "category z of task #9 in project p has pages (a)", false},
		{"a rev of two tasks", store.WorkWrite{TaskEdit: store.TaskEdit{New: &no, Rev: ptr(3)}, Slugs: []string{"a", "b"}}, "has changed since it was read", true},
		{"a parent of two projects", store.WorkWrite{TaskEdit: store.TaskEdit{Parent: ptr("#1")}, Slugs: []string{"a", "b"}, Task: ptr("#9")}, "a parent goes with one task, and the pages are on tasks of 2 projects", false},
		{"a field of pages taken off", store.WorkWrite{TaskEdit: store.TaskEdit{State: ptr("done")}, Slugs: []string{"a"}, Task: ptr("")}, "takes pages off their task changes no task", false},
		{"a category of a page of no task", store.WorkWrite{Slugs: []string{"c"}, Category: ptr("x")}, "page c is on no task", false},
		{"new, there", store.WorkWrite{TaskEdit: store.TaskEdit{New: &yes, Title: ptr("t")}, Project: ptr("p"), Task: ptr("#9")}, "task #9 is there already", true},
		{"a page not there", store.WorkWrite{Slugs: []string{"a", "none"}, Task: ptr("#1")}, `no page "none"`, false},
	} {
		pageIn(t, s, root, "c", "p", "", "")
		before := written(t, root)
		_, err := s.Work(c.w)
		if _, conflict := errors.AsType[*store.Conflict](err); err == nil || !strings.Contains(told(err, root), c.want) || conflict != c.conflict {
			t.Errorf("%s: %v, want %s (a Conflict: %v)", c.name, err, c.want, c.conflict)
		}
		if after := written(t, root); after != before {
			t.Errorf("%s: written", c.name)
		}
	}
	// Nothing to write of a task of no row is not found, and makes none.
	if _, err := s.Work(store.WorkWrite{Project: ptr("p"), Task: ptr("#5")}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("nothing of no row: %v", err)
	}
}

// A page whose project changes is taken off its task, its pin kept, the tasks as they were; unless the same edit names
// its task, which is then of the new project.
func TestEditProjectTakesOffTask(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	pageIn(t, s, root, "a", "p", "", "")
	if _, err := s.Work(store.WorkWrite{Slugs: []string{"a"}, Task: ptr("#9"), Category: ptr("x"), Pin: ptr(true)}); err != nil {
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
	if w := s.ReadMeta("a").Work; w == nil || w.Task != "#9" {
		t.Errorf("the same project: %+v", w)
	}
	if _, err = s.Edit("a", store.Edit{Project: ptr("q")}); err != nil {
		t.Fatal(err)
	}
	if w := s.ReadMeta("a").Work; w == nil || w.Task != "" || w.Category != "" || !w.Pin {
		t.Errorf("moved: %+v", w)
	}
	if after, err := os.ReadFile(filepath.Join(root, "tasks.json")); err != nil || string(after) != string(before) {
		t.Errorf("moved, tasks.json written: %s", after)
	}
	if _, err = s.Edit("a", store.Edit{Project: ptr("p"), Work: &store.WorkEdit{Task: ptr("#9")}}); err != nil {
		t.Fatal(err)
	}
	if w := s.ReadMeta("a").Work; w == nil || w.Task != "#9" {
		t.Errorf("moved and named: %+v", w)
	}
}
