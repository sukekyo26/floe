package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/store"
)

func ptr[T any](v T) *T {
	return &v
}

// workOf is the work of the page slug in the store of env.
func workOf(t *testing.T, env map[string]string, slug string) string {
	t.Helper()

	m := store.New(env["FLOE_DIR"]).ReadMeta(slug)
	if m == nil {
		t.Fatalf("no page %s", slug)
	}
	b, err := json.Marshal(m.Work)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// floe work writes the tasks first, once for each different task of the pages, and the pages then: checked first, so that
// a page of no task or no page at all leaves the rest; flags of the pages alone make no row of a task.
func TestWorkCommand(t *testing.T) {
	t.Parallel()

	env, dir := served(t)
	other := filepath.Join(filepath.Dir(dir), "q")
	in := func(at string, want int, args ...string) string {
		t.Helper()
		code, out, errs := runIn(at, env, args...)
		if code != want {
			t.Fatalf("floe %v = %d, %q, %q", args, code, out, errs)
		}
		return out + errs
	}
	run := func(want int, args ...string) string {
		t.Helper()
		return in(dir, want, args...)
	}
	s := store.New(env["FLOE_DIR"])
	tasks := func() store.Tasks {
		t.Helper()
		all, err := s.ReadTasks()
		if err != nil {
			t.Fatal(err)
		}
		return all
	}
	in(dir, 0, "publish", "w.yaml", "--slug", "a")
	in(dir, 0, "publish", "d.yaml", "--slug", "b")
	in(other, 0, "publish", "w.yaml", "--slug", "c")
	in(dir, 0, "publish", "h.html", "--link", "--slug", "e")

	// Of the pages alone: no row.
	run(0, "work", "a", "--pin")
	if got := tasks(); len(got) != 0 {
		t.Errorf("--pin wrote a task: %v", got)
	}
	if got := workOf(t, env, "a"); got != `{"pin":true}` {
		t.Errorf("--pin: %s", got)
	}
	run(0, "work", "a", "--unpin")
	if got := workOf(t, env, "a"); got != "null" {
		t.Errorf("--unpin: %s", got)
	}

	// Nor for a page on a task of no row, as one moved to another project is.
	if _, err := s.Edit("e", store.Edit{Project: ptr("moved"), Work: &store.WorkEdit{Task: ptr("#8")}}); err != nil {
		t.Fatal(err)
	}
	run(0, "work", "e", "--pin")
	run(0, "work", "e", "--step", "")
	if got := tasks(); len(got) != 0 {
		t.Errorf("flags of the page wrote a task: %v", got)
	}
	if _, err := s.Edit("e", store.Edit{Work: &store.WorkEdit{Task: ptr(""), Pin: ptr(false)}}); err != nil {
		t.Fatal(err)
	}

	// A step of spaces is no step: the page's flag alone, of a page on no task as well.
	run(0, "work", "a", "--step", "  ")
	if got := tasks(); len(got) != 0 {
		t.Errorf("--step of spaces wrote a task: %v", got)
	}

	// Refused before anything is written: a page of no task, or none.
	if got := run(1, "work", "a", "b", "--state", "done"); got != "floe: page \"a\" is on no task: name one with --task\n" {
		t.Errorf("no task: %q", got)
	}
	if got := run(1, "work", "a", "none", "--task", "#1"); got != "floe: no page \"none\" (see floe list)\n" {
		t.Errorf("no page: %q", got)
	}
	if got := tasks(); len(got) != 0 {
		t.Errorf("a refusal wrote a task: %v", got)
	}
	if got := workOf(t, env, "a"); got != "null" {
		t.Errorf("a refusal wrote a page: %s", got)
	}

	// Each to its own task, of its own project.
	run(0, "work", "a", "--task", "#1", "--step", "設計")
	run(0, "work", "b", "--task", "#2")
	run(0, "work", "c", "--task", "#1")
	run(0, "work", "a", "b", "c", "--state", "done", "--url", "https://example.com/a/b", "--title", "T")
	for _, task := range []struct{ project, name string }{{"p", "#1"}, {"p", "#2"}, {"q", "#1"}} {
		got := tasks()[task.project][task.name]
		if got.State != "done" || got.URL != "https://example.com/a/b" || got.Title != "T" {
			t.Errorf("%v: %+v", task, got)
		}
	}
	if got := tasks()["p"]["#1"].Step; got != "設計" {
		t.Errorf("the step of #1: %q", got)
	}
	if got := workOf(t, env, "a"); got != `{"task":"#1","step":"設計"}` {
		t.Errorf("a page's own work: %s", got)
	}
	// Named, all go to that task: of each page's project.
	run(0, "work", "a", "c", "--task", "#3", "--steps", "設計,実装,,設計")
	for _, project := range []string{"p", "q"} {
		if got := tasks()[project]["#3"].Steps; !slices.Equal(got, []string{"設計", "実装"}) {
			t.Errorf("%s: %v", project, got)
		}
	}
	if got := workOf(t, env, "c"); got != `{"task":"#3"}` {
		t.Errorf("c: %s", got)
	}
	// A step goes with a move to another task, stays on the same one, and is taken off the page alone.
	run(0, "work", "a", "--task", "#9")
	if got := workOf(t, env, "a"); got != `{"task":"#9"}` {
		t.Errorf("--task to another task: %s", got)
	}
	run(0, "work", "a", "--task", "#1", "--step", "設計")
	run(0, "work", "a", "--task", "#1")
	if got := workOf(t, env, "a"); got != `{"task":"#1","step":"設計"}` {
		t.Errorf("--task keeps the step: %s", got)
	}
	run(0, "work", "a", "--step", "")
	if got := workOf(t, env, "a"); got != `{"task":"#1"}` {
		t.Errorf(`--step "": %s`, got)
	}
	if got := tasks()["p"]["#1"].Step; got != "設計" {
		t.Errorf(`--step "" of the task: %q`, got)
	}

	// A page of no project: its task is that of "/".
	if _, err := s.Edit("b", store.Edit{Project: ptr("")}); err != nil {
		t.Fatal(err)
	}
	run(0, "work", "b", "--task", "#9", "--state", "waiting")
	if got := tasks()["/"]["#9"].State; got != "waiting" {
		t.Errorf("a page of no project: %v", tasks())
	}
	run(0, "work", "b", "--task", "#2")
	if _, err := s.Edit("b", store.Edit{Project: ptr("p")}); err != nil {
		t.Fatal(err)
	}

	// A link goes the same way; off its task and its pin, the task stays.
	run(0, "work", "e", "--task", "#1", "--pin")
	run(0, "work", "e", "--clear")
	if got := workOf(t, env, "e"); got != "null" {
		t.Errorf("--clear: %s", got)
	}
	run(0, "work", "b", "--task", "")
	if got := workOf(t, env, "b"); got != "null" {
		t.Errorf(`--task "": %s`, got)
	}
	if _, ok := tasks()["p"]["#2"]; !ok {
		t.Error(`--task "" took the task`)
	}

	// The task alone: of the working directory, of --project, of none.
	run(0, "work", "--task", "#4", "--state", "waiting")
	run(0, "work", "--task", "#5", "--project", "shop")
	run(0, "work", "--task", "#6", "--project", "")
	if tasks()["p"]["#4"].State != "waiting" || tasks()["shop"]["#5"].State != "todo" || tasks()["/"]["#6"].State != "todo" {
		t.Errorf("tasks alone: %v", tasks())
	}
	run(0, "work", "--task", "#a", "--state", "archived")
	if tasks()["p"]["#a"].State != "archived" {
		t.Errorf("a task archived: %v", tasks()["p"]["#a"])
	}
	if got := run(1, "work", "--task", "#4", "--state", "no"); got != "floe: unknown state \"no\": use todo, doing, waiting, done, archived\n" {
		t.Errorf("a state of none: %q", got)
	}
	if got := run(1, "work", "--task", "#4", "--url", "ftp://x/y"); got != "floe: \"ftp://x/y\" is not an http or https URL\n" {
		t.Errorf("a URL of none: %q", got)
	}

	// Forgotten once no page is on it; none, and one with a page, say so.
	run(0, "work", "--task", "#4", "--forget")
	if _, ok := tasks()["p"]["#4"]; ok {
		t.Error("--forget left #4")
	}
	if got := run(1, "work", "--task", "#4", "--forget"); got != "floe: no task #4 in project p: --project names another\n" {
		t.Errorf("no task: %q", got)
	}
	if got := run(1, "work", "--task", "#1", "--forget"); !strings.HasPrefix(got, "floe: task #1 of project p still has 1 page(s), such as a:") {
		t.Errorf("a task with a page: %q", got)
	}

	// A parent, named with --task alone, written and taken off; refused in the store's words.
	run(0, "work", "--task", "#20", "--title", "parent")
	run(0, "work", "--task", "#21", "--parent", "#20")
	if got := tasks()["p"]["#21"].Parent; got != "#20" {
		t.Errorf("--parent: %q", got)
	}
	if got := run(1, "work", "--task", "#20", "--forget"); !strings.Contains(got, "still has 1 child task(s), such as #21") {
		t.Errorf("--forget of a parent: %q", got)
	}
	run(0, "work", "--task", "#21", "--parent", "")
	if got := tasks()["p"]["#21"].Parent; got != "" {
		t.Errorf(`--parent "": %q`, got)
	}
	if got := run(1, "work", "--task", "#22", "--parent", ""); got != "floe: no task #22 in project p: --project names another\n" {
		t.Errorf(`--parent "" of no row: %q`, got)
	}
	if got := run(1, "work", "--task", "#22", "--parent", "#7"); !strings.Contains(got, "project p has no task #7 that can be a parent") {
		t.Errorf("a parent of no row: %q", got)
	}
	run(0, "work", "--task", "#22", "--title", "t", "--parent", "")
	run(0, "work", "--task", "#22", "--forget")
	run(0, "work", "--task", "#20", "--forget")
	run(0, "work", "--task", "#21", "--forget")

	// A parent with pages and --task: the task and the page in one command, refused in the store's words with the page
	// as it was. --parent "" alone of a task of no row has nothing to take off: the page goes on it, which makes it, as
	// --task alone does; with --state it makes the row.
	run(0, "work", "--task", "#30", "--title", "parent")
	run(0, "work", "b", "--task", "#31", "--parent", "#30")
	if got := tasks()["p"]["#31"].Parent; got != "#30" {
		t.Errorf("--parent with a page: %q", got)
	}
	if got := workOf(t, env, "b"); got != `{"task":"#31"}` {
		t.Errorf("--parent with a page put it on: %s", got)
	}
	if got := run(1, "work", "b", "--task", "#32", "--parent", "#7"); !strings.Contains(got, "project p has no task #7 that can be a parent") {
		t.Errorf("a parent of no row with a page: %q", got)
	}
	if got := workOf(t, env, "b"); got != `{"task":"#31"}` {
		t.Errorf("a parent refused moved the page: %s", got)
	}
	run(0, "work", "b", "--task", "#33", "--parent", "")
	if got := workOf(t, env, "b"); got != `{"task":"#33"}` {
		t.Errorf(`--parent "" of no row with a page put it on: %s`, got)
	}
	if _, ok := tasks()["p"]["#33"]; ok {
		t.Errorf(`--parent "" of no row with a page made a row: %v`, tasks()["p"]["#33"])
	}
	run(0, "work", "b", "--task", "#33", "--parent", "", "--state", "doing")
	if row, ok := tasks()["p"]["#33"]; !ok || row.State != "doing" || row.Parent != "" {
		t.Errorf(`--parent "" and --state with a page: %v, %v`, row, ok)
	}
	run(0, "work", "b", "--clear")

	// The command line that cannot mean anything.
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"work"}, "floe: name a task with --task, or pages by their slugs: run floe --help\n"},
		{[]string{"work", "--state", "done"}, "floe: name a task with --task, or pages by their slugs: run floe --help\n"},
		{[]string{"work", "--task", "#1", "--pin"}, "floe: --pin changes pages: name them by their slugs: run floe --help\n"},
		{[]string{"work", "--task", "#1", "--step", "s"}, "floe: --step changes pages: name them by their slugs: run floe --help\n"},
		{[]string{"work", "a", "--forget"}, "floe: --forget goes with a task alone, not with pages: give it without slugs: run floe --help\n"},
		{[]string{"work", "a", "--project", "p"}, "floe: --project goes with a task alone, not with pages: give it without slugs: run floe --help\n"},
		// Not a flag of the task, --parent would be dropped here and say nothing.
		{[]string{"work", "a", "--task", "", "--parent", "x"}, "floe: --task \"\" takes the pages off their task, so it cannot go with --parent: run floe --help\n"},
		{[]string{"work", "a", "--parent", "x"}, "floe: --parent with slugs goes with --task: name the one task the pages go on: run floe --help\n"},
		{[]string{"work", "a"}, "floe: say what to change of the pages: --task, --step, --state, --url, --title, --steps, --parent, --pin, --unpin or --clear: run floe --help\n"},
	} {
		if got := run(2, c.args...); got != c.want {
			t.Errorf("floe %v = %q, want %q", c.args, got, c.want)
		}
	}
	if got := run(2, "work", "a", "--pin", "--unpin"); !strings.Contains(got, "run floe --help") {
		t.Errorf("--pin --unpin: %q", got)
	}
	if got := run(2, "work", "--task", "#1", "--forget", "--state", "done"); !strings.Contains(got, "run floe --help") {
		t.Errorf("--forget --state: %q", got)
	}
	if got := run(2, "work", "--task", "#1", "--forget", "--parent", "x"); !strings.Contains(got, "run floe --help") {
		t.Errorf("--forget --parent: %q", got)
	}
	if got := run(2, "work", "a", "--clear", "--parent", "x"); !strings.Contains(got, "run floe --help") {
		t.Errorf("--clear --parent: %q", got)
	}
	for _, flag := range []string{"--state", "--step"} {
		want := "floe: --task \"\" takes the pages off their task, so it cannot go with " + flag + ": run floe --help\n"
		if got := run(2, "work", "a", "--task", "", flag, "done"); got != want {
			t.Errorf("--task \"\" %s: %q", flag, got)
		}
	}
	if got := run(0, "work", "--help"); !strings.Contains(got, "the task's state: todo, doing, waiting, done, archived") {
		t.Errorf("help: %q", got)
	}
	if got := run(1, "work", "a", "--task", "  "); got != "floe: a task needs an ID\n" {
		t.Errorf("a blank name: %q", got)
	}
}

// A page that cannot be written leaves its task written, and the same command given again finishes it.
func TestWorkCommandOrder(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root writes under a directory of no write permission")
	}
	env, dir := served(t)
	if code, out, errs := runIn(dir, env, "publish", "w.yaml", "--slug", "a"); code != 0 {
		t.Fatalf("publish = %d, %q, %q", code, out, errs)
	}
	page := filepath.Join(env["FLOE_DIR"], "a")
	if err := os.Chmod(page, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(page, 0o700) }) //nolint:errcheck // for the temporary directory's removal
	if code, out, errs := runIn(dir, env, "work", "a", "--task", "#1", "--state", "waiting"); code != 1 {
		t.Fatalf("work on a page that cannot be written = %d, %q, %q", code, out, errs)
	}
	s := store.New(env["FLOE_DIR"])
	all, err := s.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	if got := all["p"]["#1"].State; got != "waiting" {
		t.Errorf("the task was not written first: %v", all)
	}
	if got := workOf(t, env, "a"); got != "null" {
		t.Errorf("the page: %s", got)
	}
	if err := os.Chmod(page, 0o700); err != nil {
		t.Fatal(err)
	}
	if code, out, errs := runIn(dir, env, "work", "a", "--task", "#1", "--state", "waiting"); code != 0 {
		t.Fatalf("given again = %d, %q, %q", code, out, errs)
	}
	if got := workOf(t, env, "a"); got != `{"task":"#1"}` {
		t.Errorf("given again, the page: %s", got)
	}
}
