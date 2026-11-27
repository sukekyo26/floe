package store_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"floe/internal/i18n"
	"floe/internal/store"
	"floe/internal/usererr"
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
	got := change(store.TaskChange{TaskEdit: store.TaskEdit{Title: new("T"), New: new(true)}, Slugs: []string{"a"}, Task: new("#9")})
	if len(got.Tasks) != 1 || !got.Tasks[0].Made || got.Tasks[0].Row.Rev != 1 || got.Tasks[0].Row.Title != "T" || got.Tasks[0].Row.State != "todo" {
		t.Errorf("made: %+v", got.Tasks)
	}
	if m := s.ReadMeta("a"); m.Task == nil || m.Task.Name != "#9" {
		t.Errorf("the page: %+v", m.Task)
	}
	// On a task of a row, with nothing to change: as it was.
	got = change(store.TaskChange{Slugs: []string{"a"}, Task: new("#9")})
	if got.Tasks[0].Made || got.Tasks[0].Wrote || got.Tasks[0].Row.Rev != 1 {
		t.Errorf("as it was: %+v", got.Tasks)
	}
	// A category of the page, without --task, is added, and the rev moves; again, it is among them.
	got = change(store.TaskChange{Slugs: []string{"a"}, Category: new("x")})
	if r := row("p", "#9"); !slices.Equal(r.Categories, []string{"x"}) || r.Rev != 2 || !got.Tasks[0].Wrote {
		t.Errorf("a category added: %+v", r)
	}
	if got = change(store.TaskChange{Slugs: []string{"a"}, Category: new("x")}); got.Tasks[0].Wrote {
		t.Errorf("a category among them: %+v", got.Tasks)
	}
	// An order written, and a category not in it: at its end. The page moves within the write, so x may go.
	change(store.TaskChange{Categories: &[]string{"y"}, Slugs: []string{"a"}, Category: new("z")})
	if r := row("p", "#9"); !slices.Equal(r.Categories, []string{"y", "z"}) {
		t.Errorf("an order and a category: %v", r.Categories)
	}
	// Two pages on tasks of two projects: each task written, both in the answer.
	change(store.TaskChange{TaskEdit: store.TaskEdit{New: new(true)}, Slugs: []string{"b"}, Task: new("#9")})
	got = change(store.TaskChange{State: new("done"), Slugs: []string{"a", "b"}})
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
		{"a category with pages left out", store.TaskChange{Categories: &[]string{"y"}, TaskProject: new("p"), Task: new("#9")}, "category \"z\" of task #9 in project p has pages (a)", false},
		// Both tasks are there, as the screen says of one: the pages moved meanwhile, so it reads again
		{"what the screen saw, of two tasks", store.TaskChange{New: &no, Slugs: []string{"a", "b"}}, "task #9 has changed since it was read", true},
		{"a parent of two projects", store.TaskChange{Parent: new("#1"), Slugs: []string{"a", "b"}, Task: new("#9")}, "a parent goes with one task, and the pages are on tasks of 2 projects", false},
		{"a field of pages taken off", store.TaskChange{State: new("done"), Slugs: []string{"a"}, Task: new("")}, "takes pages off their task changes no task", false},
		{"a category of a page of no task", store.TaskChange{Slugs: []string{"c"}, Category: new("x")}, "page c is on no task", false},
		{"new, there", store.TaskChange{New: &yes, Title: new("t"), TaskProject: new("p"), Task: new("#9")}, "task #9 is there already", true},
		{"a page not there", store.TaskChange{Slugs: []string{"a", "none"}, Task: new("#1")}, `no page "none"`, false},
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
	if _, err := s.ChangeTasks(store.TaskChange{TaskProject: new("p"), Task: new("#5")}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("nothing of no row: %v", err)
	}
}

// A task's time is its last write's: a page that comes to it or changes its category on it writes it, its time and rev
// moving, as a field does; a pin, a write that changes nothing, and the task a page leaves keep theirs.
func TestTaskWriteTime(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	pageIn(t, s, root, "a", "p")
	for _, name := range []string{"#1", "#2"} {
		if _, err := s.ChangeTasks(store.TaskChange{TaskEdit: store.TaskEdit{Categories: &[]string{"x", "y"}, New: new(true)}, TaskProject: new("p"), Task: new(name)}); err != nil {
			t.Fatal(err)
		}
	}
	long := hour(1)
	for _, c := range []struct {
		name         string
		w            store.TaskChange
		moved, still string
	}{
		{"a page come", store.TaskChange{Slugs: []string{"a"}, Task: new("#1"), Category: new("x")}, "#1", "#2"},
		{"the same again", store.TaskChange{Slugs: []string{"a"}, Task: new("#1"), Category: new("x")}, "", "#1"},
		{"another category of the task", store.TaskChange{Slugs: []string{"a"}, Category: new("y")}, "#1", "#2"},
		{"its pin", store.TaskChange{Slugs: []string{"a"}, Pin: new(true)}, "", "#1"},
		{"to another task", store.TaskChange{Slugs: []string{"a"}, Task: new("#2")}, "#2", "#1"},
		{"a title", store.TaskChange{Title: new("T"), TaskProject: new("p"), Task: new("#1")}, "#1", "#2"},
	} {
		// Of an hour no write makes, so that a write in the same millisecond is not taken for none
		all, err := s.ReadTasks()
		if err != nil {
			t.Fatal(err)
		}
		for name, row := range all["p"] {
			row.At = long
			all["p"][name] = row
		}
		text, err := json.Marshal(all)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(root, "tasks.json"), text, 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := s.ChangeTasks(c.w)
		if err != nil {
			t.Fatal(err)
		}
		after, err := s.ReadTasks()
		if err != nil {
			t.Fatal(err)
		}
		if c.moved != "" {
			wrote := slices.ContainsFunc(got.Tasks, func(w store.TaskWritten) bool { return w.Task == c.moved && w.Wrote })
			if row := after["p"][c.moved]; row.At == long || row.Rev != all["p"][c.moved].Rev+1 || !wrote {
				t.Errorf("%s: %s %+v, written %v", c.name, c.moved, row, wrote)
			}
		}
		if row := after["p"][c.still]; row.At != long || row.Rev != all["p"][c.still].Rev {
			t.Errorf("%s: %s moved: %+v", c.name, c.still, row)
		}
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
		{"from no task", store.TaskChange{TaskEdit: store.TaskEdit{New: new(true)}, Slugs: []string{"a"}, Task: new("#1"), Category: new("x")}, "/"},
		{"the same task, another category", store.TaskChange{Slugs: []string{"a"}, Task: new(" #1 "), Category: new("y")}, "/"},
		{"its pin", store.TaskChange{Slugs: []string{"a"}, Pin: new(true)}, "/"},
		{"another task, a category of the same name", store.TaskChange{TaskEdit: store.TaskEdit{New: new(true)}, Slugs: []string{"a"}, Task: new("#2"), Category: new("y")}, "#1/y"},
		{"another task, no category", store.TaskChange{TaskEdit: store.TaskEdit{New: new(true)}, Slugs: []string{"a"}, Task: new("#3")}, "#2/y"},
		{"off its task", store.TaskChange{Slugs: []string{"a"}, Task: new("")}, "#3/"},
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
	if _, err := s.ChangeTasks(store.TaskChange{TaskEdit: store.TaskEdit{New: new(true)}, Slugs: []string{"a"}, Task: new("#9"), Category: new("x"), Pin: new(true)}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(root, "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	// The same project written with spaces round it is the same project
	if _, err = s.Edit("a", store.Edit{Project: new(" p ")}); err != nil {
		t.Fatal(err)
	}
	if w := s.ReadMeta("a").Task; w == nil || w.Name != "#9" {
		t.Errorf("the same project: %+v", w)
	}
	edited, err := s.Edit("a", store.Edit{Project: new("q")})
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
	if _, err = s.Edit("a", store.Edit{Project: new("p")}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ChangeTasks(store.TaskChange{Slugs: []string{"a"}, Task: new("#9")}); err != nil {
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
		{TaskEdit: store.TaskEdit{Title: new("parent"), New: new(true)}, TaskProject: new("p"), Task: new("#P")},
		{TaskEdit: store.TaskEdit{New: new(true)}, Slugs: []string{"x"}, Task: new("#1")},
		{TaskEdit: store.TaskEdit{New: new(true)}, Slugs: []string{"y"}, Task: new("#2")},
		{Parent: new("#P"), Slugs: []string{"x", "y"}},
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

// A category with pages, left out of the order, is named quoted in the refusal, in either language, so that a name
// with a comma or a space ends where it does; a full-width space is said as it is.
func TestCategoryPagedNamed(t *testing.T) {
	t.Parallel()

	for _, c := range []struct{ name, en, ja string }{
		{"Designs, v2", `category "Designs, v2" of task #1 in project p has pages (a)`, `のカテゴリ "Designs, v2" にはページがあります`},
		{"設計　見本", `category "設計　見本" of task #1 in project p has pages (a)`, `のカテゴリ "設計　見本" にはページがあります`},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			root := filepath.Join(t.TempDir(), "S")
			s := store.New(root)
			if _, err := editTask(s, "p", "#1", store.TaskEdit{Categories: &[]string{c.name}, New: new(true)}); err != nil {
				t.Fatal(err)
			}
			pageOn(t, s, root, "a", "p", false)
			m := s.ReadMeta("a")
			m.Task.Category = c.name
			if err := s.WriteMeta("a", m); err != nil {
				t.Fatal(err)
			}
			_, err := editTask(s, "p", "#1", store.TaskEdit{Categories: &[]string{}})
			if err == nil {
				t.Fatal("written")
			}
			if en := err.Error(); !strings.Contains(en, c.en) {
				t.Errorf("in English: %q, want %q", en, c.en)
			}
			if ja := usererr.Tell(i18n.New("ja"), err).Render(""); !strings.Contains(ja, c.ja) {
				t.Errorf("in Japanese: %q, want %q", ja, c.ja)
			}
		})
	}
}

// A task's row is made only by a write that says the task is new: a field written or a page put on a task of no row
// is ErrNotFound and writes nothing; New makes it, and is refused for a task there is and for pages of two projects.
func TestTaskMadeOnlyNew(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	pageIn(t, s, root, "a", "p")
	pageIn(t, s, root, "b", "q")
	before := written(t, root)
	for _, w := range []store.TaskChange{
		{TaskEdit: store.TaskEdit{State: new("done")}, TaskProject: new("p"), Task: new("#1")},
		{Slugs: []string{"a"}, Task: new("#1")},
	} {
		if _, err := s.ChangeTasks(w); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("%+v without New: %v, want ErrNotFound", w, err)
		}
	}
	if got := written(t, root); got != before {
		t.Errorf("a refusal wrote:\n%s", got)
	}
	got, err := s.ChangeTasks(store.TaskChange{TaskEdit: store.TaskEdit{New: new(true)}, Slugs: []string{"a"}, Task: new("#1")})
	if err != nil || len(got.Tasks) != 1 || !got.Tasks[0].Made {
		t.Fatalf("New: %+v, %v", got, err)
	}
	_, err = s.ChangeTasks(store.TaskChange{TaskEdit: store.TaskEdit{New: new(true)}, TaskProject: new("p"), Task: new("#1")})
	if _, conflict := errors.AsType[*store.Conflict](err); !conflict || keyArgs(err) != "store_task_there [#1]" {
		t.Errorf("New of a task there is: %v", err)
	}
	_, err = s.ChangeTasks(store.TaskChange{TaskEdit: store.TaskEdit{New: new(true)}, Slugs: []string{"a", "b"}, Task: new("#2")})
	if keyArgs(err) != "store_task_new_projects [2]" {
		t.Errorf("New of pages of two projects: %v", err)
	}
}
