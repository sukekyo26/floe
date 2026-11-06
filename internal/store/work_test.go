package store_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/store"
)

// taskIn is the row of the task #1, failing the test when there is none.
func taskIn(t *testing.T, s *store.Store, project string) store.Task {
	t.Helper()

	all, err := s.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	row, ok := all[project]["#1"]
	if !ok {
		t.Fatalf("no task #1 of %s in %v", project, all)
	}
	return row
}

// A new task is todo; its time moves with its state alone, its step with what is not empty; a key it does not set is
// written back; an order of steps is kept clean, and an empty URL or order takes it off.
func TestWorkWrite(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	if all, err := s.ReadTasks(); err != nil || len(all) != 0 {
		t.Fatalf("no tasks.json = %v, %v", all, err)
	}
	if err := s.EditTask("p", "#1", store.TaskEdit{}); err != nil {
		t.Fatal(err)
	}
	if first := taskIn(t, s, "p"); first.State != "todo" || first.At == "" {
		t.Errorf("new task %+v", first)
	}
	// By hand: a parent, which no write sets, and a time no write can make by chance.
	long := hour(1)
	hand := fmt.Sprintf(`{"p": {"#1": {"state": "doing", "at": %q, "parent": "#0"}}}`, long)
	if err := os.WriteFile(filepath.Join(root, "tasks.json"), []byte(hand), 0o600); err != nil {
		t.Fatal(err)
	}
	// As Edit reads a project: trimmed, so that this is the same task.
	if err := s.EditTask(" p ", " #1 ", store.TaskEdit{Step: ptr(" 設計 "), Title: ptr(" T ")}); err != nil {
		t.Fatal(err)
	}
	row := taskIn(t, s, "p")
	if row.Step != "設計" || row.Title != "T" || row.At != long {
		t.Errorf("after a step: %+v", row)
	}
	// An empty step leaves the last written, as does the same state, which leaves its time too.
	if err := s.EditTask("p", "#1", store.TaskEdit{Step: ptr(" "), State: ptr("doing")}); err != nil {
		t.Fatal(err)
	}
	if row = taskIn(t, s, "p"); row.Step != "設計" || row.At != long {
		t.Errorf("after an empty step: %+v", row)
	}
	if err := s.EditTask("p", "#1", store.TaskEdit{
		State: ptr("waiting"), URL: ptr("HTTPS://example.com/a"), Steps: ptr([]string{" 設計", "実装", "", "設計"}),
	}); err != nil {
		t.Fatal(err)
	}
	row = taskIn(t, s, "p")
	if row.State != "waiting" || row.Parent != "#0" || row.URL != "HTTPS://example.com/a" || !slices.Equal(row.Steps, []string{"設計", "実装"}) {
		t.Errorf("after the state: %+v", row)
	}
	if row.At == long {
		t.Error("the state's time stayed")
	}
	if err := s.EditTask("p", "#1", store.TaskEdit{URL: ptr(""), Steps: ptr([]string{""})}); err != nil {
		t.Fatal(err)
	}
	if row = taskIn(t, s, "p"); row.URL != "" || row.Steps != nil {
		t.Errorf("taken off: %+v", row)
	}
}

// What no row may hold is refused and leaves the file as it was: a state of no table, a URL of another scheme or of no
// host, a task of no project or name.
func TestWorkRefused(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	if err := s.EditTask("p", "#1", store.TaskEdit{}); err != nil {
		t.Fatal(err)
	}
	was := taskIn(t, s, "p")
	for _, c := range []struct {
		project, name string
		ed            store.TaskEdit
		want          string
	}{
		{"p", "#1", store.TaskEdit{State: ptr("WAITING")}, `unknown state "WAITING": use todo, doing, waiting, done`},
		{"p", "#1", store.TaskEdit{URL: ptr("javascript:alert(1)")}, `"javascript:alert(1)" is not an http or https URL`},
		{"p", "#1", store.TaskEdit{URL: ptr("   ")}, `"   " is not an http or https URL`},
		{"p", "#1", store.TaskEdit{URL: ptr("https:")}, `"https:" is not an http or https URL`},
		{"", "#1", store.TaskEdit{}, `a task needs its project: "/" for none`},
		{"  ", "#1", store.TaskEdit{}, `a task needs its project: "/" for none`},
		{"p", " ", store.TaskEdit{}, "a task needs an ID"},
		{"\xff", "#1", store.TaskEdit{}, "a task's project and ID must be UTF-8"},
		{"p", "\xff", store.TaskEdit{}, "a task's project and ID must be UTF-8"},
	} {
		err := s.EditTask(c.project, c.name, c.ed)
		if err == nil || !strings.Contains(told(err, root), c.want) {
			t.Errorf("%+v = %v, want %s", c, err, c.want)
		}
	}
	if got := taskIn(t, s, "p"); got.At != was.At || got.State != was.State {
		t.Errorf("after the refusals: %+v", got)
	}
	if err := s.EditTask("p", "#1", store.TaskEdit{URL: ptr("  https://example.com/a  ")}); err != nil {
		t.Fatal(err)
	}
	if got := taskIn(t, s, "p").URL; got != "https://example.com/a" {
		t.Errorf("a URL as pasted: %q", got)
	}
	if err := s.ForgetTask("p", "\xff"); err == nil || !strings.Contains(told(err, root), "must be UTF-8") {
		t.Errorf("forget of no UTF-8: %v", err)
	}
}

// A tasks.json that cannot be read, or holds a state of no table, is refused to read and to write, and stays as it is
// for a person to mend.
func TestWorkBroken(t *testing.T) {
	t.Parallel()

	for _, text := range []string{
		"{ broken", "null", `{"p": null}`, `{"p": {"#1": {"state": "paused", "at": "2026-01-02T00:00:00Z"}}}`,
		`{"p": {"#1": {"state": "doing", "at": "2026-01-02T00:00:00Z", "url": "javascript:alert(1)"}}}`,
		`{"p": {"#1": {"state": "doing"}}}`, `{"p": {"#1": {"state": "doing", "at": "soon"}}}`,
	} {
		root := filepath.Join(t.TempDir(), "S")
		if err := os.MkdirAll(root, 0o700); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(root, "tasks.json")
		if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		s := store.New(root)
		if _, err := s.ReadTasks(); err == nil {
			t.Errorf("read %s", text)
		}
		if err := s.EditTask("p", "#2", store.TaskEdit{}); err == nil || !strings.Contains(told(err, root), "mend it by hand") {
			t.Errorf("write over %s: %v", text, err)
		}
		if err := s.ForgetTask("p", "#1"); err == nil {
			t.Errorf("forget in %s", text)
		}
		if b, err := os.ReadFile(file); err != nil || string(b) != text {
			t.Errorf("%s came to be %s, %v", text, b, err)
		}
	}
}

// pageOn writes a page of the project on the task #1.
func pageOn(t *testing.T, s *store.Store, root, slug, project string, archived bool) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(root, slug), 0o700); err != nil {
		t.Fatal(err)
	}
	m := &store.Meta{Project: project, Archived: archived, CreatedAt: hour(1), UpdatedAt: hour(1), Work: &store.Work{Task: "#1"}}
	if err := s.WriteMeta(slug, m); err != nil {
		t.Fatal(err)
	}
}

// A task is forgotten once no page is on it, an archived one and one of no project too; a page of another project of
// the same name is not on it. None is not found.
func TestWorkForget(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	if err := s.ForgetTask("p", "#1"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("of no store: %v", err)
	}
	for _, project := range []string{"p", "/"} {
		if err := s.EditTask(project, "#1", store.TaskEdit{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ForgetTask("p", "#2"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("of no task: %v", err)
	}
	pageOn(t, s, root, "kept", "p", true)
	pageOn(t, s, root, "none", "", false)
	pageOn(t, s, root, "other", "q", false)
	// A directory's name may end in a space, as no task's project does.
	if err := s.EditTask("spaced ", "#1", store.TaskEdit{}); err != nil {
		t.Fatal(err)
	}
	pageOn(t, s, root, "spaced", "spaced ", false)
	refused := func(project, want string) {
		t.Helper()
		if err := s.ForgetTask(project, "#1"); err == nil || !strings.Contains(told(err, root), want) {
			t.Errorf("forget %s: %v, want %s", project, err, want)
		}
		taskIn(t, s, project)
	}
	refused("p", "task #1 of project p still has 1 page(s), such as kept")
	refused("/", "task #1 of project / still has 1 page(s), such as none")
	refused("spaced", "task #1 of project spaced still has 1 page(s), such as spaced")
	if err := os.RemoveAll(filepath.Join(root, "kept")); err != nil {
		t.Fatal(err)
	}
	// Only the page of q, which is another task, of the same name.
	if err := s.ForgetTask("p", "#1"); err != nil {
		t.Error(err)
	}
	all, err := s.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := all["p"]; ok {
		t.Errorf("a project of no task left: %v", all)
	}
	taskIn(t, s, "/")
}

// A page's work is changed a field at a time, its step going with its task and all of it with task and pin both;
// the list says its task's project, whatever the page's.
func TestPageWork(t *testing.T) {
	t.Parallel()

	s, _ := newStore(t)
	dw := picKit(t, "", "")
	publish(t, s, dw, "d.yaml", "lang: en\ntemplate: pic\n", nil, store.Options{})
	work := func(ed store.WorkEdit, want string) {
		t.Helper()
		if _, err := s.Edit("d", store.Edit{Work: &ed}); err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(s.ReadMeta("d").Work)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != want {
			t.Errorf("%+v: %s, want %s", ed, b, want)
		}
	}
	work(store.WorkEdit{Task: ptr(" #1 "), Step: ptr(" 設計 ")}, `{"task":"#1","step":"設計"}`)
	work(store.WorkEdit{Pin: ptr(true)}, `{"task":"#1","step":"設計","pin":true}`)
	work(store.WorkEdit{Task: ptr(" #1 ")}, `{"task":"#1","step":"設計","pin":true}`)
	work(store.WorkEdit{Task: ptr("#2"), Step: ptr("実装")}, `{"task":"#2","step":"実装","pin":true}`)
	work(store.WorkEdit{Task: ptr("#3")}, `{"task":"#3","pin":true}`)
	work(store.WorkEdit{Step: ptr("設計")}, `{"task":"#3","step":"設計","pin":true}`)
	work(store.WorkEdit{Step: ptr("")}, `{"task":"#3","pin":true}`)
	work(store.WorkEdit{Task: ptr("")}, `{"pin":true}`)
	work(store.WorkEdit{Step: ptr("off a task")}, `{"pin":true}`)
	work(store.WorkEdit{Pin: ptr(false)}, "null")
	work(store.WorkEdit{Task: ptr("#3"), Step: ptr("s")}, `{"task":"#3","step":"s"}`)
	for _, name := range []string{"   ", "\xff"} {
		if _, err := s.Edit("d", store.Edit{Work: &store.WorkEdit{Task: ptr(name)}}); err == nil {
			t.Errorf("a page on the task %q", name)
		}
	}
	work(store.WorkEdit{}, `{"task":"#3","step":"s"}`)

	project := func(want string) {
		t.Helper()
		list, err := s.List(viewer)
		if err != nil {
			t.Fatal(err)
		}
		if got := list[0].TaskProject; got != want {
			t.Errorf("TaskProject %q, want %q", got, want)
		}
	}
	project("p")
	for _, c := range []struct{ set, want string }{{"shop", "shop"}, {"", "/"}} {
		if _, err := s.Edit("d", store.Edit{Project: ptr(c.set)}); err != nil {
			t.Fatal(err)
		}
		project(c.want)
	}

	// It goes on through what writes the page anew, and with its name.
	publish(t, s, dw, "d.yaml", "lang: en\ntemplate: pic\ntitle: two\n", nil, store.Options{})
	if got := s.ReadMeta("d").Work; got == nil || got.Task != "#3" || got.Step != "s" {
		t.Errorf("published again: %+v", got)
	}
	if err := s.Rebuild(picKit(t, "v2", "")); err != nil {
		t.Fatal(err)
	}
	if got := s.ReadMeta("d").Work; got == nil || got.Task != "#3" {
		t.Errorf("drawn again: %+v", got)
	}
	if _, err := s.Edit("d", store.Edit{Slug: ptr("e")}); err != nil {
		t.Fatal(err)
	}
	if got := s.ReadMeta("e").Work; got == nil || got.Task != "#3" {
		t.Errorf("renamed: %+v", got)
	}
}

// A page of no project, and a directory's name with spaces round it, are each one project in tasks.json.
func TestProjectKey(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{"": "/", "  ": "/", "p": "p", " p ": "p"} {
		if got := store.ProjectKey(in); got != want {
			t.Errorf("ProjectKey(%q) = %q, want %q", in, got, want)
		}
	}
}

// The table says the one state a new task takes.
func TestStates(t *testing.T) {
	t.Parallel()

	fresh := slices.DeleteFunc(slices.Clone(store.States), func(s store.State) bool { return !s.New })
	if first := slices.DeleteFunc(slices.Clone(store.States), func(s store.State) bool { return !s.First }); len(first) != 1 || first[0].Name != "waiting" {
		t.Errorf("the list opens on %v", first)
	}
	if len(fresh) != 1 {
		t.Errorf("new tasks take %v", fresh)
	}
	if got := store.StateNames(); got != "todo, doing, waiting, done" {
		t.Errorf("StateNames = %q", got)
	}
}

// A parent is a task of the same project with a row and no parent of its own, of a task with no children; what is
// refused writes nothing, a new task's row too. The same again changes nothing, and "" takes a parent off a row that is
// there.
func TestWorkParent(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	for _, c := range []struct{ project, name string }{{"p", "#1"}, {"p", "#2"}, {"p", "#9"}, {"q", "#8"}} {
		if err := s.EditTask(c.project, c.name, store.TaskEdit{}); err != nil {
			t.Fatal(err)
		}
	}
	parentOf := func(name string) string {
		t.Helper()
		all, err := s.ReadTasks()
		if err != nil {
			t.Fatal(err)
		}
		return all["p"][name].Parent
	}
	if err := s.EditTask("p", "#2", store.TaskEdit{Parent: ptr(" #1 ")}); err != nil {
		t.Fatal(err)
	}
	if got := parentOf("#2"); got != "#1" {
		t.Errorf("a parent with spaces round it: %q", got)
	}
	// A child of no row yet comes to be, todo.
	if err := s.EditTask("p", "#3", store.TaskEdit{Parent: ptr("#1")}); err != nil {
		t.Fatal(err)
	}
	if got := parentOf("#3"); got != "#1" {
		t.Errorf("a new child: %q", got)
	}
	for _, c := range []struct {
		name, parent, want string
	}{
		{"#1", " #1 ", "task #1 cannot be its own parent"},
		{"#4", "#7", "project p has no task #7 that can be a parent"},
		{"#4", "#8", "project p has no task #8 that can be a parent"},
		{"#4", "#2", "task #2 has a parent itself (#1), so it cannot be a parent"},
		{"#1", "#2", "task #2 has a parent itself (#1), so it cannot be a parent"},
		{"#1", "#9", "task #1 has child tasks, such as #2, so it cannot have a parent"},
		{"#4", "  ", "a task needs an ID"},
		{"#4", "\xff", "a task's project and ID must be UTF-8"},
	} {
		err := s.EditTask("p", c.name, store.TaskEdit{Parent: &c.parent})
		if err == nil || !strings.Contains(told(err, root), c.want) {
			t.Errorf("%s under %q = %v, want %s", c.name, c.parent, err, c.want)
		}
	}
	// A parent taken off a task of no row, a name mistyped, is not found; with anything else it is a new task.
	if err := s.EditTask("p", "#4", store.TaskEdit{Parent: ptr("")}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf(`"" of no row: %v`, err)
	}
	all, err := s.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := all["p"]["#4"]; ok || all["p"]["#1"].Parent != "" {
		t.Errorf("after the refusals: %v", all["p"])
	}
	if wrote := s.EditTask("p", "#5", store.TaskEdit{Title: ptr("t"), Parent: ptr("")}); wrote != nil {
		t.Errorf(`"" with a title, of no row: %v`, wrote)
	}
	// The same again, to another parent, and off, of a task with none too.
	at := all["p"]["#2"].At
	for _, c := range []struct{ name, parent, want string }{
		{"#2", "#1", "#1"}, {"#2", "#9", "#9"}, {"#2", "", ""}, {"#2", "", ""},
	} {
		if wrote := s.EditTask("p", c.name, store.TaskEdit{Parent: &c.parent}); wrote != nil {
			t.Errorf("%s under %q: %v", c.name, c.parent, wrote)
		}
		if got := parentOf(c.name); got != c.want {
			t.Errorf("%s under %q came to be under %q", c.name, c.parent, got)
		}
	}
	if all, err = s.ReadTasks(); err != nil || all["p"]["#2"].At != at {
		t.Errorf("its time moved with its parent: %v, %v", all["p"]["#2"], err)
	}
}

// A parent keeps its row while it has a child, whose row may go; the pages on it are told with its children. A page of a
// task and a child of it are of one task, and of no other project's of the same name.
func TestWorkForgetParent(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	for _, c := range []struct {
		project, name string
		ed            store.TaskEdit
	}{
		{"p", "#1", store.TaskEdit{}},
		{"q", "#1", store.TaskEdit{}},
		{"p", "#3", store.TaskEdit{Parent: ptr("#1")}},
		{"p", "#2", store.TaskEdit{Parent: ptr("#1")}},
	} {
		if err := s.EditTask(c.project, c.name, c.ed); err != nil {
			t.Fatal(err)
		}
	}
	pageOn(t, s, root, "kept", "p", false)
	err := s.ForgetTask("p", "#1")
	got := ""
	if err != nil {
		got = told(err, root)
	}
	for _, want := range []string{
		"task #1 of project p still has 1 page(s), such as kept",
		"task #1 of project p still has 2 child task(s), such as #2: take them off it or remove them first",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("forget a parent = %q, want %s", got, want)
		}
	}
	if err := s.ForgetTask("q", "#1"); err != nil {
		t.Errorf("of another project: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(root, "kept")); err != nil {
		t.Fatal(err)
	}
	if err := s.ForgetTask("p", "#2"); err != nil {
		t.Errorf("a child: %v", err)
	}
	if err := s.EditTask("p", "#3", store.TaskEdit{Parent: ptr("")}); err != nil {
		t.Fatal(err)
	}
	if err := s.ForgetTask("p", "#1"); err != nil {
		t.Errorf("with none left: %v", err)
	}
}

// A parent written by hand is read as it is, keeps no other row from being written, and is mended by a write; what
// childrenOfTask does not count as a child keeps nothing: a task of itself, one of a parent with a parent, either of a
// pair each the other's, one of a parent gone.
func TestWorkParentByHand(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	row := func(parent string) string {
		return `{"state": "doing", "at": "2026-01-02T00:00:00Z", "parent": "` + parent + `"}`
	}
	if err := os.WriteFile(filepath.Join(root, "tasks.json"), []byte(`{"p": {
		"self": `+row("self")+`,
		"a": {"state": "doing", "at": "2026-01-02T00:00:00Z"}, "b": `+row("a")+`, "c": `+row("b")+`,
		"x": `+row("y")+`, "y": `+row("x")+`,
		"lost": `+row("gone")+`,
		"plain": {"state": "doing", "at": "2026-01-02T00:00:00Z"}
	}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s := store.New(root)
	all, err := s.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	if all["p"]["lost"].Parent != "gone" {
		t.Errorf("read: %+v", all["p"]["lost"])
	}
	if err := s.EditTask("p", "plain", store.TaskEdit{Title: ptr("t")}); err != nil {
		t.Errorf("another row: %v", err)
	}
	// A task of itself has no child, so that it takes another parent.
	if err := s.EditTask("p", "self", store.TaskEdit{Parent: ptr("plain")}); err != nil {
		t.Errorf("a task of itself: %v", err)
	}
	// A parent gone has no children for what names it, so that a task of that name takes a parent.
	if err := s.EditTask("p", "gone", store.TaskEdit{Parent: ptr("plain")}); err != nil {
		t.Errorf("a parent gone: %v", err)
	}
	if err := s.EditTask("p", "lost", store.TaskEdit{Parent: ptr("")}); err != nil {
		t.Errorf("mended: %v", err)
	}
	// a has b for a child, which has a parent: c is no child of b, so that b goes.
	if err := s.ForgetTask("p", "a"); err == nil || !strings.Contains(told(err, root), "such as b") {
		t.Errorf("a parent: %v", err)
	}
	for _, name := range []string{"b", "x", "y"} {
		if err := s.ForgetTask("p", name); err != nil {
			t.Errorf("forget %s: %v", name, err)
		}
	}
}

// A write that says whether it takes the task to be new is refused, as a Conflict that writes nothing, when the task is
// there or not otherwise: there is of a row, or of a page on it with no row. One that says nothing is as the CLI's.
func TestWorkNew(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	yes, no := true, false
	if err := s.EditTask("p", "#1", store.TaskEdit{Title: ptr("one")}); err != nil {
		t.Fatal(err)
	}
	pageOn(t, s, root, "kept", "p", true)
	refused := func(name string, ed store.TaskEdit, want string) {
		t.Helper()
		before, err := s.ReadTasks()
		if err != nil {
			t.Fatal(err)
		}
		err = s.EditTask("p", name, ed)
		if _, conflict := errors.AsType[*store.Conflict](err); !conflict || !strings.Contains(told(err, root), want) {
			t.Errorf("%s %+v = %v, want a Conflict of %s", name, ed, err, want)
		}
		if after, readErr := s.ReadTasks(); readErr != nil || fmt.Sprint(after) != fmt.Sprint(before) {
			t.Errorf("%s written: %v, %v", name, after, readErr)
		}
	}
	// #1 of a row, and pageOn's task #1 of p: drop the row to have a task of a page alone.
	refused("#1", store.TaskEdit{New: &yes, Title: ptr("other")}, "task #1 is there already")
	if err := s.ForgetTask("p", "#1"); err == nil {
		t.Fatal("a task with a page was forgotten")
	}
	refused("#9", store.TaskEdit{New: &no, Parent: ptr("#1")}, "task #9 is no longer there")
	if err := s.EditTask("p", "#2", store.TaskEdit{New: &yes, Title: ptr("two")}); err != nil {
		t.Errorf("a new one: %v", err)
	}
	if err := s.EditTask("p", "#2", store.TaskEdit{New: &no, Title: ptr("still")}); err != nil {
		t.Errorf("one there: %v", err)
	}
	if err := s.EditTask("p", "#2", store.TaskEdit{Title: ptr("by the CLI")}); err != nil || taskTitle(t, s, "#2") != "by the CLI" {
		t.Errorf("one with no word of new: %v", err)
	}
}

// A task of pages alone, of no row, is there.
func TestWorkNewOfPages(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	yes, no := true, false
	pageOn(t, s, root, "kept", "p", true)
	err := s.EditTask("p", "#1", store.TaskEdit{New: &yes, Title: ptr("t")})
	if _, conflict := errors.AsType[*store.Conflict](err); !conflict {
		t.Errorf("new, of a page alone: %v", err)
	}
	if all, readErr := s.ReadTasks(); readErr != nil || len(all) != 0 {
		t.Errorf("written: %v, %v", all, readErr)
	}
	if err = s.EditTask("p", "#1", store.TaskEdit{New: &no, Parent: ptr("")}); err != nil {
		t.Errorf("there, of a page alone: %v", err)
	}
}

func taskTitle(t *testing.T, s *store.Store, name string) string {
	t.Helper()
	all, err := s.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	return all["p"][name].Title
}
