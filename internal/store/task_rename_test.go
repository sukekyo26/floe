package store_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"floe/internal/store"
)

// renameTask is a write of the task (p, name) alone that changes its ID to to, with ed's fields.
func renameTask(s *store.Store, name, to string, ed store.TaskEdit) (store.TaskChangeResult, error) {
	project := "p"

	return s.ChangeTasks(store.TaskChange{TaskEdit: ed, Slugs: nil, TaskProject: &project, Task: &name, Category: nil, Pin: nil, Clear: false, Rename: &to})
}

// pageOnTask writes the page slug of project on the task as pt has it.
func pageOnTask(t *testing.T, s *store.Store, root, slug, project string, archived bool, pt store.PageTask) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(root, slug), 0o700); err != nil {
		t.Fatal(err)
	}
	m := &store.Meta{Project: project, Archived: archived, CreatedAt: hour(1), UpdatedAt: hour(1), Task: &pt}
	if err := s.WriteMeta(slug, m); err != nil {
		t.Fatal(err)
	}
}

// A task's ID changed moves its row, every page on it (the archived too) with its category and pin, and its children's
// parent, each child's time and rev moved as any row written; the task of the same ID in another project, and a page
// of another task, stay.
func TestTaskRename(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	for _, c := range []struct {
		project, name string
		ed            store.TaskEdit
	}{
		{"p", "#1", store.TaskEdit{State: new("doing"), Title: new("T"), Categories: new([]string{"a"}), New: new(true)}},
		{"p", "#2", store.TaskEdit{Parent: new("#1"), New: new(true)}},
		{"q", "#1", store.TaskEdit{State: new("todo"), New: new(true)}},
	} {
		if _, err := editTask(s, c.project, c.name, c.ed); err != nil {
			t.Fatal(err)
		}
	}
	pageOnTask(t, s, root, "on", "p", false, store.PageTask{Name: "#1", Category: "a", Pin: true})
	pageOnTask(t, s, root, "old", "p", true, store.PageTask{Name: "#1", Category: "a", Pin: false})
	pageOnTask(t, s, root, "other", "q", false, store.PageTask{Name: "#1", Category: "", Pin: false})
	pageOnTask(t, s, root, "kid", "p", false, store.PageTask{Name: "#2", Category: "", Pin: false})
	before, err := s.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	rev := before["p"]["#1"].Rev
	got, err := renameTask(s, "#1", " #7 ", store.TaskEdit{Rev: &rev, Title: new("U")})
	if err != nil {
		t.Fatal(err)
	}
	written := make([]string, 0, len(got.Tasks))
	for _, w := range got.Tasks {
		written = append(written, w.Task)
	}
	if !slices.Equal(written, []string{"#7", "#2"}) {
		t.Errorf("the tasks written: %v", written)
	}
	all, err := s.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	row, had := all["p"]["#7"]
	if _, old := all["p"]["#1"]; old || !had || row.Title != "U" || row.State != "doing" || !slices.Equal(row.Categories, []string{"a"}) || row.Rev != rev+1 {
		t.Errorf("the rows of p: %+v", all["p"])
	}
	if kid := all["p"]["#2"]; kid.Parent != "#7" || kid.Rev != before["p"]["#2"].Rev+1 || kid.At == before["p"]["#2"].At {
		t.Errorf("the child: %+v, was %+v", kid, before["p"]["#2"])
	}
	if !reflect.DeepEqual(all["q"], before["q"]) {
		t.Errorf("the other project's: %+v", all["q"])
	}
	for slug, want := range map[string]store.PageTask{
		"on":    {Name: "#7", Category: "a", Pin: true},
		"old":   {Name: "#7", Category: "a", Pin: false},
		"other": {Name: "#1", Category: "", Pin: false},
		"kid":   {Name: "#2", Category: "", Pin: false},
	} {
		if m := s.ReadMeta(slug); m == nil || m.Task == nil || *m.Task != want {
			t.Errorf("page %s: %+v, want %+v", slug, m, want)
		}
	}
	// The same ID, written with spaces round it, changes the fields alone
	if _, err = renameTask(s, "#7", " #7 ", store.TaskEdit{Title: new("V")}); err != nil {
		t.Fatal(err)
	}
	if all, err = s.ReadTasks(); err != nil || all["p"]["#7"].Title != "V" || len(all["p"]) != 2 {
		t.Errorf("to the same ID: %+v, %v", all["p"], err)
	}
	// The ID alone is a write of the row
	rev = all["p"]["#7"].Rev
	if _, err = renameTask(s, "#7", "#9", store.TaskEdit{}); err != nil {
		t.Fatal(err)
	}
	if all, err = s.ReadTasks(); err != nil || all["p"]["#9"].Rev != rev+1 {
		t.Errorf("the ID alone: %+v, %v", all["p"], err)
	}
}

// A change of ID is refused, writing nothing, to an ID of the project's or none, with pages, of a task of no row, and of
// a row written since the screen read it.
func TestTaskRenameRefused(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	for _, name := range []string{"#1", "#2"} {
		if _, err := editTask(s, "p", name, store.TaskEdit{State: new("todo"), New: new(true)}); err != nil {
			t.Fatal(err)
		}
	}
	pageOnTask(t, s, root, "on", "p", false, store.PageTask{Name: "#1", Category: "", Pin: false})
	tasks := filepath.Join(root, "tasks.json")
	was, err := os.ReadFile(tasks)
	if err != nil {
		t.Fatal(err)
	}
	stale := 0
	for _, c := range []struct {
		what, to string
		ed       store.TaskEdit
		want     string
	}{
		{"taken", "#2", store.TaskEdit{}, "task #2 is there already"},
		{"none", "", store.TaskEdit{}, "a task needs an ID"},
		{"spaces", "  ", store.TaskEdit{}, "a task needs an ID"},
		{"stale", "#3", store.TaskEdit{Rev: &stale}, "task #1"},
	} {
		_, err = renameTask(s, "#1", c.to, c.ed)
		if err == nil || !strings.Contains(told(err, root), c.want) {
			t.Errorf("%s: %v, want %s", c.what, err, c.want)
		}
		if c.what == "stale" {
			if _, conflict := errors.AsType[*store.Conflict](err); !conflict {
				t.Errorf("stale: %v, want a Conflict", err)
			}
		}
	}
	project, name, to := "p", "#1", "#3"
	_, err = s.ChangeTasks(store.TaskChange{Slugs: []string{"on"}, TaskProject: &project, Task: &name, Rename: &to})
	if err == nil || !strings.Contains(told(err, root), "changes a task's ID puts no pages on a task") {
		t.Errorf("with pages: %v", err)
	}
	// With a field, which would make the row of a task alone
	for _, ed := range []store.TaskEdit{{}, {Title: new("t")}} {
		if _, err = renameTask(s, "#9", "#3", ed); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("of no row, %+v: %v", ed, err)
		}
	}
	if now, err := os.ReadFile(tasks); err != nil || string(now) != string(was) {
		t.Errorf("tasks.json after the refusals:\n%s\nwas\n%s", now, was)
	}
	if m := s.ReadMeta("on"); m == nil || m.Task == nil || m.Task.Name != "#1" {
		t.Errorf("the page after the refusals: %+v", m)
	}
}

// A change of ID with a parent made in the same write makes the parent's row, which the task names by its new ID; the
// parent may not be the new ID.
func TestTaskRenameParentNew(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	if _, err := editTask(s, "p", "#5", store.TaskEdit{State: new("todo"), New: new(true)}); err != nil {
		t.Fatal(err)
	}
	if _, err := renameTask(s, "#5", "#8", store.TaskEdit{Parent: new("#8"), ParentNew: new(true), ParentTitle: new("P")}); err == nil ||
		!strings.Contains(told(err, root), "task #8 cannot be its own parent") {
		t.Errorf("the new ID as its parent: %v", err)
	}
	got, err := renameTask(s, "#5", "#6", store.TaskEdit{Parent: new("#P"), ParentNew: new(true), ParentTitle: new("P")})
	if err != nil {
		t.Fatal(err)
	}
	all, err := s.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	if up, ok := all["p"]["#P"]; !ok || up.Title != "P" || all["p"]["#6"].Parent != "#P" || len(all["p"]) != 2 {
		t.Errorf("the rows: %+v", all["p"])
	}
	if len(got.Tasks) != 1 || got.Tasks[0].Task != "#6" || got.Tasks[0].Row.Parent != "#P" {
		t.Errorf("the tasks written: %+v", got.Tasks)
	}
}
