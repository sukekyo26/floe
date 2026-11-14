package store_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"floe/internal/store"
)

// editTask is a write of the task (project, name) alone, its row as written.
func editTask(s *store.Store, project, name string, ed store.TaskEdit) (store.TaskWritten, error) {
	written, err := s.Work(store.WorkWrite{TaskEdit: ed, Slugs: nil, TaskProject: &project, Task: &name, Category: nil, Pin: nil, Clear: false})
	if err != nil {
		return store.TaskWritten{}, err
	}
	return written.Tasks[0], nil
}

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

// A new task is todo; its time moves with its state alone, its category with what is not empty; a key it does not set is
// written back; an order of categories is kept clean, and an empty order takes it off.
func TestWorkWrite(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	if all, err := s.ReadTasks(); err != nil || len(all) != 0 {
		t.Fatalf("no tasks.json = %v, %v", all, err)
	}
	if _, err := editTask(s, "p", "#1", store.TaskEdit{State: ptr("todo")}); err != nil {
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
	if _, err := editTask(s, " p ", " #1 ", store.TaskEdit{Title: ptr(" T ")}); err != nil {
		t.Fatal(err)
	}
	row := taskIn(t, s, "p")
	if row.Title != "T" || row.At != long {
		t.Errorf("after a title: %+v", row)
	}
	// The same state leaves its time.
	if _, err := editTask(s, "p", "#1", store.TaskEdit{State: ptr("doing")}); err != nil {
		t.Fatal(err)
	}
	if row = taskIn(t, s, "p"); row.At != long {
		t.Errorf("after the same state: %+v", row)
	}
	if _, err := editTask(s, "p", "#1", store.TaskEdit{
		State: ptr("done"), URLs: &store.URLsEdit{Add: []string{"HTTPS://example.com/a"}}, Categories: ptr([]string{" 設計", "実装", "", "設計"}),
	}); err != nil {
		t.Fatal(err)
	}
	row = taskIn(t, s, "p")
	if row.State != "done" || row.Parent != "#0" || !slices.Equal(row.URLs, []string{"HTTPS://example.com/a"}) || !slices.Equal(row.Categories, []string{"設計", "実装"}) {
		t.Errorf("after the state: %+v", row)
	}
	if row.At == long {
		t.Error("the state's time stayed")
	}
	if _, err := editTask(s, "p", "#1", store.TaskEdit{URLs: &store.URLsEdit{Remove: []string{"HTTPS://example.com/a"}}, Categories: ptr([]string{""})}); err != nil {
		t.Fatal(err)
	}
	if row = taskIn(t, s, "p"); row.URLs != nil || row.Categories != nil {
		t.Errorf("taken off: %+v", row)
	}
}

// What no row may hold is refused and leaves the file as it was: a state of no table, a URL of another scheme or of no
// host, a task of no project or name.
// Archived is a state as any: written, its time is when it went there, and read back.
func TestWorkArchived(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	if _, err := editTask(s, "p", "#1", store.TaskEdit{State: ptr("todo")}); err != nil {
		t.Fatal(err)
	}
	long := hour(1)
	if err := os.WriteFile(filepath.Join(root, "tasks.json"), []byte(fmt.Sprintf(`{"p": {"#1": {"state": "done", "at": %q}}}`, long)), 0o600); err != nil {
		t.Fatal(err)
	}
	state := "archived"
	if _, err := editTask(s, "p", "#1", store.TaskEdit{State: &state}); err != nil {
		t.Fatal(err)
	}
	if got := taskIn(t, s, "p"); got.State != "archived" || got.At == long {
		t.Errorf("archived %+v, its time before %s", got, long)
	}
}

// adding is a TaskEdit that adds urls.
func adding(urls ...string) store.TaskEdit {
	return store.TaskEdit{URLs: &store.URLsEdit{Add: urls}}
}

// URLs are added at the end and taken off, each once and trimmed as pasted, those in both staying; a write that would
// bring a task over 10 is refused and writes nothing, while a row over it by hand still takes the other writes.
func TestWorkURLs(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	u := func(n int) string { return fmt.Sprintf("https://example.com/%d", n) }
	urls := func() []string { return taskIn(t, s, "p").URLs }
	edit := func(ed store.URLsEdit) error {
		_, err := editTask(s, "p", "#1", store.TaskEdit{URLs: &ed})
		return err
	}
	if err := edit(store.URLsEdit{Add: []string{" " + u(1) + " ", u(2), u(1)}}); err != nil {
		t.Fatal(err)
	}
	if err := edit(store.URLsEdit{Add: []string{u(3), u(2)}, Remove: []string{"  " + u(1), u(2), "https://nowhere.example"}}); err != nil {
		t.Fatal(err)
	}
	if got, want := urls(), []string{u(2), u(3)}; !slices.Equal(got, want) {
		t.Fatalf("after adding and taking off: %q, want %q", got, want)
	}
	if err := edit(store.URLsEdit{Add: []string{u(4), u(5), u(6), u(7), u(8), u(9), u(10), u(11)}}); err != nil {
		t.Fatal(err)
	}
	was := taskIn(t, s, "p")
	err := edit(store.URLsEdit{Add: []string{u(12)}})
	if err == nil || !strings.Contains(told(err, root), "would hold 11 URLs") {
		t.Errorf("an 11th URL: %v", err)
	}
	// Swapping one for another keeps 10
	if err := edit(store.URLsEdit{Add: []string{u(12)}, Remove: []string{u(2)}}); err != nil {
		t.Errorf("one for another at 10: %v", err)
	}
	if got := taskIn(t, s, "p"); len(got.URLs) != 10 || got.Rev != was.Rev+1 {
		t.Errorf("after the refusal and the swap: %+v", got)
	}
	// Over 10 by hand: the state is written and a URL taken off; one added that leaves it over 10 is refused
	hand := fmt.Sprintf(`{"p": {"#1": {"state": "doing", "at": %q, "urls": [%s]}}}`, hour(1), quotedURLs(u, 12))
	if err := os.WriteFile(filepath.Join(root, "tasks.json"), []byte(hand), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := editTask(s, "p", "#1", store.TaskEdit{State: ptr("done")}); err != nil {
		t.Errorf("a state of a row of 12: %v", err)
	}
	if err := edit(store.URLsEdit{Remove: []string{u(1)}}); err != nil {
		t.Errorf("taking one off a row of 12: %v", err)
	}
	if err := edit(store.URLsEdit{Add: []string{u(13)}, Remove: []string{u(2), u(3)}}); err != nil {
		t.Errorf("down to 10: %v", err)
	}
	if err := edit(store.URLsEdit{Add: []string{u(14)}}); err == nil {
		t.Error("an 11th after the hand's")
	}
	if got := urls(); len(got) != 10 {
		t.Errorf("at the end: %q", got)
	}
	// 10 by hand and one of them twice: one more is an 11th
	hand = fmt.Sprintf(`{"p": {"#1": {"state": "doing", "at": %q, "urls": [%s, %q]}}}`, hour(1), quotedURLs(u, 10), u(1))
	if err := os.WriteFile(filepath.Join(root, "tasks.json"), []byte(hand), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := edit(store.URLsEdit{Add: []string{u(11)}}); err == nil || !strings.Contains(told(err, root), "would hold 11 URLs") {
		t.Errorf("an 11th to 10 and one twice: %v", err)
	}
}

// quotedURLs is the URLs u makes of 1 to n, quoted and comma-separated, as JSON has them in a list.
func quotedURLs(u func(int) string, n int) string {
	quoted := make([]string, n)
	for i := range n {
		quoted[i] = strconv.Quote(u(i + 1))
	}
	return strings.Join(quoted, ", ")
}

func TestWorkRefused(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	if _, err := editTask(s, "p", "#1", store.TaskEdit{State: ptr("todo")}); err != nil {
		t.Fatal(err)
	}
	was := taskIn(t, s, "p")
	for _, c := range []struct {
		project, name string
		ed            store.TaskEdit
		want          string
	}{
		{"p", "#1", store.TaskEdit{State: ptr("DOING")}, `unknown state "DOING": use todo, doing, done, archived`},
		{"p", "#1", store.TaskEdit{State: ptr("waiting")}, `unknown state "waiting": use todo, doing, done, archived`},
		{"p", "#1", adding("javascript:alert(1)"), `"javascript:alert(1)" is not an http or https URL`},
		{"p", "#1", adding("   "), `"" is not an http or https URL`},
		{"p", "#1", adding(""), `"" is not an http or https URL`},
		{"p", "#1", adding("https://example.com/ok", "https:"), `"https:" is not an http or https URL`},
		{"", "#1", store.TaskEdit{State: ptr("todo")}, `a task needs its project: "/" for none`},
		{"  ", "#1", store.TaskEdit{State: ptr("todo")}, `a task needs its project: "/" for none`},
		{"p", " ", store.TaskEdit{State: ptr("todo")}, "a task needs an ID"},
		{"\xff", "#1", store.TaskEdit{State: ptr("todo")}, "a task's project and ID must be UTF-8"},
		{"p", "\xff", store.TaskEdit{State: ptr("todo")}, "a task's project and ID must be UTF-8"},
	} {
		_, err := editTask(s, c.project, c.name, c.ed)
		if err == nil || !strings.Contains(told(err, root), c.want) {
			t.Errorf("%+v = %v, want %s", c, err, c.want)
		}
	}
	if got := taskIn(t, s, "p"); got.At != was.At || got.State != was.State {
		t.Errorf("after the refusals: %+v", got)
	}
	if got := taskIn(t, s, "p").URLs; got != nil {
		t.Errorf("URLs after the refusals: %q", got)
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
		`{"p": {"#1": {"state": "waiting", "at": "2026-01-02T00:00:00Z"}}}`,
		`{"p": {"#1": {"state": "doing", "at": "2026-01-02T00:00:00Z", "urls": ["https://example.com/a", "javascript:alert(1)"]}}}`,
		// A key floe does not know, as the URL of before is, one twice, and one of another case are refused, not dropped
		`{"p": {"#1": {"state": "doing", "at": "2026-01-02T00:00:00Z", "url": "https://example.com/a"}}}`,
		`{"p": {"#1": {"state": "doing", "at": "2026-01-02T00:00:00Z", "title": "a", "title": "b"}}}`,
		`{"p": {"#1": {"state": "doing", "at": "2026-01-02T00:00:00Z", "Title": "a"}}}`,
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
		if _, err := editTask(s, "p", "#2", store.TaskEdit{}); err == nil || !strings.Contains(told(err, root), "mend it by hand") {
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
		if _, err := editTask(s, project, "#1", store.TaskEdit{State: ptr("todo")}); err != nil {
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
	if _, err := editTask(s, "spaced ", "#1", store.TaskEdit{State: ptr("todo")}); err != nil {
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

// A page's work is changed a field at a time, its category going with its task and all of it with task and pin both;
// the list says its task's project, whatever the page's.
func TestPageWork(t *testing.T) {
	t.Parallel()

	s, _ := newStore(t)
	dw := picKit(t, "", "")
	publish(t, s, dw, "d.yaml", "lang: en\ntemplate: pic\n", nil, store.Options{})
	work := func(ed store.WorkEdit, want string) {
		t.Helper()
		if _, err := s.Work(store.WorkWrite{Slugs: []string{"d"}, Task: ed.Task, Category: ed.Category, Pin: ed.Pin}); err != nil {
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
	work(store.WorkEdit{Task: ptr(" #1 "), Category: ptr(" 設計 ")}, `{"task":"#1","category":"設計"}`)
	work(store.WorkEdit{Pin: ptr(true)}, `{"task":"#1","category":"設計","pin":true}`)
	work(store.WorkEdit{Task: ptr(" #1 ")}, `{"task":"#1","category":"設計","pin":true}`)
	work(store.WorkEdit{Task: ptr("#2"), Category: ptr("実装")}, `{"task":"#2","category":"実装","pin":true}`)
	work(store.WorkEdit{Task: ptr("#3")}, `{"task":"#3","pin":true}`)
	work(store.WorkEdit{Category: ptr("設計")}, `{"task":"#3","category":"設計","pin":true}`)
	work(store.WorkEdit{Category: ptr("")}, `{"task":"#3","pin":true}`)
	work(store.WorkEdit{Task: ptr("")}, `{"pin":true}`)
	// A category of a page on no task is refused, not dropped
	if _, err := s.Work(store.WorkWrite{Slugs: []string{"d"}, Category: ptr("off a task")}); err == nil {
		t.Error("a category of a page on no task")
	}
	work(store.WorkEdit{Pin: ptr(false)}, "null")
	work(store.WorkEdit{Task: ptr("#3"), Category: ptr("s")}, `{"task":"#3","category":"s"}`)
	for _, name := range []string{"   ", "\xff"} {
		if _, err := s.Work(store.WorkWrite{Slugs: []string{"d"}, Task: ptr(name)}); err == nil {
			t.Errorf("a page on the task %q", name)
		}
	}
	work(store.WorkEdit{}, `{"task":"#3","category":"s"}`)

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
	// A task is of its project: moved, the page is off it. Back in the project it is published from, it goes on again.
	if got := s.ReadMeta("d").Work; got != nil {
		t.Errorf("moved to another project: %+v", got)
	}
	if _, err := s.Edit("d", store.Edit{Project: ptr("p")}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Work(store.WorkWrite{Slugs: []string{"d"}, Task: ptr("#3"), Category: ptr("s")}); err != nil {
		t.Fatal(err)
	}

	// It goes on through what writes the page anew, and with its name.
	publish(t, s, dw, "d.yaml", "lang: en\ntemplate: pic\ntitle: two\n", nil, store.Options{})
	if got := s.ReadMeta("d").Work; got == nil || got.Task != "#3" || got.Category != "s" {
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
	if first := slices.DeleteFunc(slices.Clone(store.States), func(s store.State) bool { return !s.First }); len(first) != 1 || first[0].Name != "doing" {
		t.Errorf("the list opens on %v", first)
	}
	if len(fresh) != 1 {
		t.Errorf("new tasks take %v", fresh)
	}
	if got := store.StateNames(); got != "todo, doing, done, archived" {
		t.Errorf("StateNames = %q", got)
	}
	var open []string
	for _, s := range store.States {
		if s.Open {
			open = append(open, s.Name)
		}
		if (s.New || s.First) && !s.Open {
			t.Errorf("%s is where a new task or the list's first view is, and is not open", s.Name)
		}
	}
	if got := strings.Join(open, ", "); got != "todo, doing" {
		t.Errorf("the open states are %q", got)
	}
	// Open is for the CLI: the management page reads this table as JSON, and is not given it.
	if b, err := json.Marshal(store.States); err != nil || strings.Contains(strings.ToLower(string(b)), "open") {
		t.Errorf("the states as JSON, %s, %v", b, err)
	}
}

// The open tasks of a project come the latest change first, by name among those that changed at the same time, and
// the times are compared as times: T1 to T3 are in the reverse order as text of the order as times (a zone, and a
// fraction against none), and Z9 is the time of the six, written in another zone.
func TestTasksOpen(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	hand := `{
  "p": {
    "T1": {"state": "doing", "at": "2026-11-12T10:00:00+09:00", "title": "Fix login"},
    "T2": {"state": "todo", "at": "2026-11-12T09:30:00Z"},
    "T3": {"state": "todo", "at": "2026-11-12T09:30:00.500Z"},
    "#1": {"state": "todo", "at": "2026-11-11T09:00:00Z"},
    "#10": {"state": "todo", "at": "2026-11-11T09:00:00Z"},
    "#2": {"state": "todo", "at": "2026-11-11T09:00:00Z"},
    "B": {"state": "doing", "at": "2026-11-11T09:00:00Z"},
    "Z9": {"state": "todo", "at": "2026-11-11T18:00:00+09:00"},
    "a": {"state": "todo", "at": "2026-11-11T09:00:00Z"},
    "done": {"state": "done", "at": "2026-11-13T00:00:00Z"},
    "archived": {"state": "archived", "at": "2026-11-13T00:00:00Z"}
  },
  "q": {"Q1": {"state": "doing", "at": "2026-11-14T00:00:00Z"}}
}`
	if err := os.WriteFile(filepath.Join(root, "tasks.json"), []byte(hand), 0o600); err != nil {
		t.Fatal(err)
	}
	all, err := store.New(root).ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	opened := all.Open("p")
	names := make([]string, len(opened))
	for i, task := range opened {
		names[i] = task.Name
	}
	if got, want := strings.Join(names, " "), "T3 T2 T1 #1 #10 #2 B Z9 a"; got != want {
		t.Errorf("the open tasks of p = %s, want %s", got, want)
	}
	if row := opened[2]; row.Name != "T1" || row.State != "doing" || row.Title != "Fix login" {
		t.Errorf("a task of Open is %+v, not the row of T1", row)
	}
	if got := all.Open("none"); len(got) != 0 {
		t.Errorf("a project of no task has %v", got)
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
		if _, err := editTask(s, c.project, c.name, store.TaskEdit{State: ptr("todo")}); err != nil {
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
	if _, err := editTask(s, "p", "#2", store.TaskEdit{Parent: ptr(" #1 ")}); err != nil {
		t.Fatal(err)
	}
	if got := parentOf("#2"); got != "#1" {
		t.Errorf("a parent with spaces round it: %q", got)
	}
	// A child of no row yet comes to be, todo.
	if _, err := editTask(s, "p", "#3", store.TaskEdit{Parent: ptr("#1")}); err != nil {
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
		_, err := editTask(s, "p", c.name, store.TaskEdit{Parent: &c.parent})
		if err == nil || !strings.Contains(told(err, root), c.want) {
			t.Errorf("%s under %q = %v, want %s", c.name, c.parent, err, c.want)
		}
	}
	// A parent taken off a task of no row, a name mistyped, is not found; with anything else it is a new task.
	if _, err := editTask(s, "p", "#4", store.TaskEdit{Parent: ptr("")}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf(`"" of no row: %v`, err)
	}
	all, err := s.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := all["p"]["#4"]; ok || all["p"]["#1"].Parent != "" {
		t.Errorf("after the refusals: %v", all["p"])
	}
	if _, wrote := editTask(s, "p", "#5", store.TaskEdit{Title: ptr("t"), Parent: ptr("")}); wrote != nil {
		t.Errorf(`"" with a title, of no row: %v`, wrote)
	}
	// The same again, to another parent, and off, of a task with none too.
	at := all["p"]["#2"].At
	for _, c := range []struct{ name, parent, want string }{
		{"#2", "#1", "#1"}, {"#2", "#9", "#9"}, {"#2", "", ""}, {"#2", "", ""},
	} {
		if _, wrote := editTask(s, "p", c.name, store.TaskEdit{Parent: &c.parent}); wrote != nil {
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
		{"p", "#1", store.TaskEdit{State: ptr("todo")}},
		{"q", "#1", store.TaskEdit{State: ptr("todo")}},
		{"p", "#3", store.TaskEdit{Parent: ptr("#1")}},
		{"p", "#2", store.TaskEdit{Parent: ptr("#1")}},
	} {
		if _, err := editTask(s, c.project, c.name, c.ed); err != nil {
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
		"task #1 of project p still has 2 child task(s), such as #2: take them off it or delete them first",
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
	if _, err := editTask(s, "p", "#3", store.TaskEdit{Parent: ptr("")}); err != nil {
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
	if _, err := editTask(s, "p", "plain", store.TaskEdit{Title: ptr("t")}); err != nil {
		t.Errorf("another row: %v", err)
	}
	// A task of itself has no child, so that it takes another parent.
	if _, err := editTask(s, "p", "self", store.TaskEdit{Parent: ptr("plain")}); err != nil {
		t.Errorf("a task of itself: %v", err)
	}
	// A parent gone has no children for what names it, so that a task of that name takes a parent.
	if _, err := editTask(s, "p", "gone", store.TaskEdit{Parent: ptr("plain")}); err != nil {
		t.Errorf("a parent gone: %v", err)
	}
	if _, err := editTask(s, "p", "lost", store.TaskEdit{Parent: ptr("")}); err != nil {
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
	if _, err := editTask(s, "p", "#1", store.TaskEdit{Title: ptr("one")}); err != nil {
		t.Fatal(err)
	}
	pageOn(t, s, root, "kept", "p", true)
	refused := func(name string, ed store.TaskEdit, want string) {
		t.Helper()
		before, err := s.ReadTasks()
		if err != nil {
			t.Fatal(err)
		}
		_, err = editTask(s, "p", name, ed)
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
	if _, err := editTask(s, "p", "#2", store.TaskEdit{New: &yes, Title: ptr("two")}); err != nil {
		t.Errorf("a new one: %v", err)
	}
	if _, err := editTask(s, "p", "#2", store.TaskEdit{New: &no, Title: ptr("still")}); err != nil {
		t.Errorf("one there: %v", err)
	}
	if _, err := editTask(s, "p", "#2", store.TaskEdit{Title: ptr("by the CLI")}); err != nil || taskTitle(t, s, "#2") != "by the CLI" {
		t.Errorf("one with no word of new: %v", err)
	}
}

// A task is there of its row alone: one a page names with no row, as a hand edit leaves, is not there, so that a write
// that takes it to be new makes its row, and one that takes it to be there is refused.
func TestWorkThereOfRowAlone(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	yes, no := true, false
	pageOn(t, s, root, "kept", "p", true)
	_, err := editTask(s, "p", "#1", store.TaskEdit{New: &no, Title: ptr("t")})
	if _, conflict := errors.AsType[*store.Conflict](err); !conflict {
		t.Errorf("there, of a page alone: %v", err)
	}
	written, err := editTask(s, "p", "#1", store.TaskEdit{New: &yes, Title: ptr("t")})
	if err != nil || !written.Made || written.Row.Rev != 1 {
		t.Errorf("new, of a page alone: %+v, %v", written, err)
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

// A parent written from a screen says what the screen saw: the task's row read (rev), and whether the parent named is
// new, made then in the same write. Refused, nothing is written, a parent to make neither. Each row there is written
// once, of rev 1.
func TestWorkParentNew(t *testing.T) {
	t.Parallel()

	yes, no := true, false
	made := func(rev int) store.TaskEdit {
		return store.TaskEdit{New: &no, Rev: &rev, Parent: ptr("#200"), ParentNew: &yes, ParentTitle: ptr(" pay ")}
	}
	type row struct{ name, parent string }
	for _, c := range []struct {
		name  string
		rows  []row
		page  bool // a page on #1, of no row
		task  string
		ed    store.TaskEdit
		want  string // the refusal; "" for none
		stale bool   // refused as Conflict
		after map[string]string
	}{
		{name: "made", rows: []row{{"#130", ""}}, task: "#130", ed: made(1), after: map[string]string{"#130": "#200", "#200": ""}},
		{name: "a page's task of no row", page: true, task: "#1", ed: made(0), want: "task #1 is no longer there", stale: true},
		{name: "the parent there", rows: []row{{"#130", ""}, {"#200", ""}}, task: "#130", ed: made(1), want: "task #200 is there already", stale: true},
		{name: "a parent named by a page alone", rows: []row{{"#130", ""}}, page: true, task: "#130", ed: store.TaskEdit{New: &no, Rev: ptr(1), Parent: ptr("#1"), ParentNew: &yes, ParentTitle: ptr("x")}, after: map[string]string{"#130": "#1", "#1": ""}},
		{name: "with children", rows: []row{{"#130", ""}, {"#131", "#130"}}, task: "#130", ed: made(1), want: "task #130 has child tasks, such as #131"},
		{name: "the task gone", task: "#130", ed: made(1), want: "task #130 is no longer there", stale: true},
		{name: "another parent meanwhile", rows: []row{{"#180", ""}, {"#130", "#180"}}, task: "#130", ed: made(0), want: "task #130 has changed since it was read", stale: true},
		{name: "itself", rows: []row{{"#130", ""}}, task: "#130", ed: store.TaskEdit{New: &no, Rev: ptr(1), Parent: ptr("#130"), ParentNew: &yes, ParentTitle: ptr("x")}, want: "task #130 cannot be its own parent"},
		{name: "a title of spaces", rows: []row{{"#130", ""}}, task: "#130", ed: store.TaskEdit{New: &no, Rev: ptr(1), Parent: ptr("#200"), ParentNew: &yes, ParentTitle: ptr("  ")}, want: "parentNew and parentTitle go with a parent"},
		{name: "a title alone", rows: []row{{"#130", ""}}, task: "#130", ed: store.TaskEdit{ParentTitle: ptr("pay")}, want: "parentNew and parentTitle go with a parent"},
		{name: "taken off, said new", rows: []row{{"#123", ""}, {"#130", "#123"}}, task: "#130", ed: store.TaskEdit{Parent: ptr(""), ParentNew: &no}, want: "parentNew and parentTitle go with a parent"},
		// Of no row, it can be no parent.
		{name: "the parent gone", rows: []row{{"#130", ""}}, task: "#130", ed: store.TaskEdit{New: &no, Rev: ptr(1), Parent: ptr("#180"), ParentNew: &no}, want: "project p has no task #180 that can be a parent"},
		{name: "taken off, moved meanwhile", rows: []row{{"#123", ""}, {"#180", ""}, {"#130", "#180"}}, task: "#130", ed: store.TaskEdit{New: &no, Rev: ptr(0), Parent: ptr("")}, want: "task #130 has changed since it was read", stale: true},
		{name: "taken off", rows: []row{{"#123", ""}, {"#130", "#123"}}, task: "#130", ed: store.TaskEdit{New: &no, Rev: ptr(1), Parent: ptr("")}, after: map[string]string{"#123": "", "#130": ""}},
		{name: "a URL alone", rows: []row{{"#123", ""}, {"#130", "#123"}}, task: "#130", ed: adding("https://example.com/b"), after: map[string]string{"#123": "", "#130": "#123"}},
		{name: "as the CLI, of none", rows: []row{{"#130", ""}}, task: "#130", ed: store.TaskEdit{Parent: ptr("#200")}, want: "project p has no task #200 that can be a parent"},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			root := filepath.Join(t.TempDir(), "S")
			s := store.New(root)
			for _, r := range c.rows {
				ed := store.TaskEdit{Title: ptr("t")}
				if r.parent != "" {
					ed.Parent = ptr(r.parent)
				}
				if _, err := editTask(s, "p", r.name, ed); err != nil {
					t.Fatal(err)
				}
			}
			if c.page {
				pageOn(t, s, root, "kept", "p", false)
			}
			before, err := s.ReadTasks()
			if err != nil {
				t.Fatal(err)
			}
			_, err = editTask(s, "p", c.task, c.ed)
			all, readErr := s.ReadTasks()
			if readErr != nil {
				t.Fatal(readErr)
			}
			if c.want != "" {
				if _, conflict := errors.AsType[*store.Conflict](err); err == nil || !strings.Contains(told(err, root), c.want) || conflict != c.stale {
					t.Errorf("= %v, want %s (a Conflict: %v)", err, c.want, c.stale)
				}
				if fmt.Sprint(all) != fmt.Sprint(before) {
					t.Errorf("written: %v", all)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]string{}
			for name, task := range all["p"] {
				got[name] = task.Parent
			}
			if fmt.Sprint(got) != fmt.Sprint(c.after) {
				t.Errorf("parents %v, want %v", got, c.after)
			}
			if p, ok := all["p"]["#200"]; ok && (p.Title != "pay" || p.State != "todo") {
				t.Errorf("the parent made: %+v", p)
			}
		})
	}
}

// A row written by hand keeps its rev: a screen that read it before writes over it, as the LIMIT of EditTask says. A
// name of no row taken off its parent, and nothing else, is not found, said read or not: a name mistyped makes no row.
func TestWorkRevByHand(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "S")
	s := store.New(root)
	no := false
	for _, name := range []string{"#123", "#130"} {
		if _, err := editTask(s, "p", name, store.TaskEdit{Title: ptr("t")}); err != nil {
			t.Fatal(err)
		}
	}
	file := filepath.Join(root, "tasks.json")
	text, err := os.ReadFile(file) //nolint:gosec // G304: a path the test names
	if err != nil {
		t.Fatal(err)
	}
	var all map[string]map[string]map[string]any
	if err = json.Unmarshal(text, &all); err != nil {
		t.Fatal(err)
	}
	all["p"]["#130"]["parent"] = " #9"
	if text, err = json.Marshal(all); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(file, text, 0o600); err != nil { //nolint:gosec // G703: a path the test names
		t.Fatal(err)
	}
	if _, err = editTask(s, "p", "#77", store.TaskEdit{Parent: ptr(""), Rev: ptr(0)}); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("a parent taken off a name of no row: %v", err)
	}
	if _, err = editTask(s, "p", "#130", store.TaskEdit{New: &no, Rev: ptr(1), Parent: ptr("#123"), ParentNew: &no}); err != nil {
		t.Fatal(err)
	}
	got, err := s.ReadTasks()
	if err != nil || got["p"]["#130"].Parent != "#123" {
		t.Errorf("changed: %+v, %v", got["p"]["#130"], err)
	}
}

// A write from a screen sends the rev it read, 0 of no row: refused as Conflict when the row has been written since, of
// any field, and else written, its rev one more, which EditTask returns with the row. A write that changes nothing
// writes nothing, and keeps the rev. A parent made is of rev 1.
func TestWorkRev(t *testing.T) {
	t.Parallel()

	yes, no := true, false
	for _, c := range []struct {
		name   string
		page   bool // a page on #1, of no row
		task   string
		ed     store.TaskEdit
		want   string // the refusal; "" for none
		rev    int    // returned, and the row's after
		parent int    // the rev of #200 after; 0 for none
	}{
		{name: "the rev read", task: "#130", ed: store.TaskEdit{New: &no, Rev: ptr(2), State: ptr("done")}, rev: 3},
		{name: "written since", task: "#130", ed: store.TaskEdit{New: &no, Rev: ptr(1), State: ptr("done")}, want: "task #130 has changed since it was read"},
		{name: "written since, to the state it is in", task: "#130", ed: store.TaskEdit{New: &no, Rev: ptr(1), State: ptr("doing")}, want: "task #130 has changed since it was read"},
		{name: "no rev, as the CLI", task: "#130", ed: store.TaskEdit{State: ptr("done")}, rev: 3},
		// A page's task of no row, written by hand, is not there: a screen that takes it to be is refused, and reads again.
		{name: "a page's task of no row", page: true, task: "#1", ed: store.TaskEdit{New: &no, Rev: ptr(0), State: ptr("doing")}, want: "task #1 is no longer there"},
		{name: "new, 0", task: "#140", ed: store.TaskEdit{New: &yes, Rev: ptr(0), Title: ptr("x")}, rev: 1},
		{name: "new, 2", task: "#140", ed: store.TaskEdit{New: &yes, Rev: ptr(2)}, want: "task #140 has changed since it was read"},
		{name: "negative", task: "#130", ed: store.TaskEdit{Rev: ptr(-1), State: ptr("done")}, want: "task #130 has changed since it was read"},
		{name: "no change", task: "#130", ed: store.TaskEdit{New: &no, Rev: ptr(2), State: ptr("doing")}, rev: 2},
		{name: "no change, no rev", task: "#130", ed: store.TaskEdit{}, rev: 2},
		{name: "a parent made", task: "#130", ed: store.TaskEdit{New: &no, Rev: ptr(2), Parent: ptr("#200"), ParentNew: &yes, ParentTitle: ptr("pay")}, rev: 3, parent: 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			root := filepath.Join(t.TempDir(), "S")
			s := store.New(root)
			// #130 written twice, of rev 2, in doing
			for _, ed := range []store.TaskEdit{{Title: ptr("t")}, {State: ptr("doing")}} {
				if _, err := editTask(s, "p", "#130", ed); err != nil {
					t.Fatal(err)
				}
			}
			if c.page {
				pageOn(t, s, root, "kept", "p", false)
			}
			before, err := s.ReadTasks()
			if err != nil {
				t.Fatal(err)
			}
			written, err := editTask(s, "p", c.task, c.ed)
			rev := written.Row.Rev
			all, readErr := s.ReadTasks()
			if readErr != nil {
				t.Fatal(readErr)
			}
			if c.want != "" {
				if _, conflict := errors.AsType[*store.Conflict](err); !conflict || told(err, root) != "floe: "+c.want {
					t.Errorf("= %q (a Conflict: %v), want %s as a Conflict", told(err, root), conflict, c.want)
				}
				if fmt.Sprint(all) != fmt.Sprint(before) {
					t.Errorf("written: %v", all)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if rev != c.rev || all["p"][c.task].Rev != c.rev || all["p"]["#200"].Rev != c.parent {
				t.Errorf("rev %d, the row's %d, the parent's %d; want %d, %d", rev, all["p"][c.task].Rev, all["p"]["#200"].Rev, c.rev, c.parent)
			}
		})
	}
}
