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

// A new task is doing; its time moves with its state alone, its step with what is not empty; a key it does not set is
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
	if first := taskIn(t, s, "p"); first.State != "doing" || first.At == "" {
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
		{"p", "#1", store.TaskEdit{State: ptr("WAITING")}, `unknown state "WAITING": use waiting, doing, done`},
		{"p", "#1", store.TaskEdit{URL: ptr("javascript:alert(1)")}, `"javascript:alert(1)" is not an http or https URL`},
		{"p", "#1", store.TaskEdit{URL: ptr("   ")}, `"   " is not an http or https URL`},
		{"p", "#1", store.TaskEdit{URL: ptr("https:")}, `"https:" is not an http or https URL`},
		{"", "#1", store.TaskEdit{}, `a task needs its project: "/" for none`},
		{"  ", "#1", store.TaskEdit{}, `a task needs its project: "/" for none`},
		{"p", " ", store.TaskEdit{}, "a task needs a name"},
		{"\xff", "#1", store.TaskEdit{}, "a task's project and name must be UTF-8"},
		{"p", "\xff", store.TaskEdit{}, "a task's project and name must be UTF-8"},
	} {
		err := s.EditTask(c.project, c.name, c.ed)
		if err == nil || !strings.Contains(told(err, root), c.want) {
			t.Errorf("%+v = %v, want %s", c, err, c.want)
		}
	}
	if got := taskIn(t, s, "p"); got.At != was.At || got.State != was.State {
		t.Errorf("after the refusals: %+v", got)
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
		"{ broken", "null", `{"p": null}`, `{"p": {"#1": {"state": "paused", "at": "x"}}}`,
		`{"p": {"#1": {"state": "doing", "at": "x", "url": "javascript:alert(1)"}}}`,
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
	if len(fresh) != 1 {
		t.Errorf("new tasks take %v", fresh)
	}
	if got := store.StateNames(); got != "waiting, doing, done" {
		t.Errorf("StateNames = %q", got)
	}
}
