package cli_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"floe/internal/i18n"
	"floe/internal/store"
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

// floe task writes the tasks first, once for each different task of the pages, and the pages then: checked first, so that
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
	run(0, "task", "--slug", "a", "--pin")
	if got := tasks(); len(got) != 0 {
		t.Errorf("--pin wrote a task: %v", got)
	}
	if got := workOf(t, env, "a"); got != `{"pin":true}` {
		t.Errorf("--pin: %s", got)
	}
	run(0, "task", "--slug", "a", "--unpin")
	if got := workOf(t, env, "a"); got != "null" {
		t.Errorf("--unpin: %s", got)
	}

	// A page on a task of no row, as a hand edit leaves: the next floe task of the page makes the row.
	if m := s.ReadMeta("e"); m == nil {
		t.Fatal("no page e")
	} else {
		m.Project, m.Work = "moved", &store.Work{Task: "#8", Category: "", Pin: false}
		if err := s.WriteMeta("e", m); err != nil {
			t.Fatal(err)
		}
	}
	if got := run(0, "task", "--slug", "e", "--pin"); got != "task #8 in project moved: made\npage e: task #8 in project moved, pinned\n" {
		t.Errorf("a page of a task of no row: %q", got)
	}
	run(0, "task", "--slug", "e", "--clear")
	run(0, "task", "--task", "#8", "--project", "moved", "--forget")

	// A category of spaces is no category: the page's flag alone, of a page on no task as well.
	run(0, "task", "--slug", "a", "--category", "  ")
	if got := tasks(); len(got) != 0 {
		t.Errorf("--category of spaces wrote a task: %v", got)
	}

	// Refused before anything is written: a page of no task, or none.
	if got := run(1, "task", "--slug", "a", "--slug", "b", "--state", "done"); got != "floe: page \"a\" is on no task: name one with --task\n" {
		t.Errorf("no task: %q", got)
	}
	if got := run(1, "task", "--slug", "a", "--slug", "none", "--task", "#1"); got != "floe: no page \"none\" (see floe list)\n" {
		t.Errorf("no page: %q", got)
	}
	if got := tasks(); len(got) != 0 {
		t.Errorf("a refusal wrote a task: %v", got)
	}
	if got := workOf(t, env, "a"); got != "null" {
		t.Errorf("a refusal wrote a page: %s", got)
	}

	// Each to its own task, of its own project.
	run(0, "task", "--slug", "a", "--task", "#1", "--category", "設計")
	run(0, "task", "--slug", "b", "--task", "#2")
	run(0, "task", "--slug", "c", "--task", "#1")
	run(0, "task", "--slug", "a", "--slug", "b", "--slug", "c", "--state", "done", "--url", "https://example.com/a/b", "--title", "T")
	for _, task := range []struct{ project, name string }{{"p", "#1"}, {"p", "#2"}, {"q", "#1"}} {
		got := tasks()[task.project][task.name]
		if got.State != "done" || !slices.Equal(got.URLs, []string{"https://example.com/a/b"}) || got.Title != "T" {
			t.Errorf("%v: %+v", task, got)
		}
	}
	if got := workOf(t, env, "a"); got != `{"task":"#1","category":"設計"}` {
		t.Errorf("a page's own work: %s", got)
	}
	// Named, all go to that task: of each page's project.
	run(0, "task", "--slug", "a", "--slug", "c", "--task", "#3", "--categories", "設計,実装,,設計")
	for _, project := range []string{"p", "q"} {
		if got := tasks()[project]["#3"].Categories; !slices.Equal(got, []string{"設計", "実装"}) {
			t.Errorf("%s: %v", project, got)
		}
	}
	if got := workOf(t, env, "c"); got != `{"task":"#3"}` {
		t.Errorf("c: %s", got)
	}
	// A category goes with a move to another task, stays on the same one, and is taken off the page alone.
	run(0, "task", "--slug", "a", "--task", "#9")
	if got := workOf(t, env, "a"); got != `{"task":"#9"}` {
		t.Errorf("--task to another task: %s", got)
	}
	run(0, "task", "--slug", "a", "--task", "#1", "--category", "設計")
	run(0, "task", "--slug", "a", "--task", "#1")
	if got := workOf(t, env, "a"); got != `{"task":"#1","category":"設計"}` {
		t.Errorf("--task keeps the category: %s", got)
	}
	run(0, "task", "--slug", "a", "--category", "")
	if got := workOf(t, env, "a"); got != `{"task":"#1"}` {
		t.Errorf(`--category "": %s`, got)
	}

	// A page of no project: its task is that of "/".
	if _, err := s.Edit("b", store.Edit{Project: ptr("")}); err != nil {
		t.Fatal(err)
	}
	run(0, "task", "--slug", "b", "--task", "#9", "--state", "done")
	if got := tasks()["/"]["#9"].State; got != "done" {
		t.Errorf("a page of no project: %v", tasks())
	}
	run(0, "task", "--slug", "b", "--task", "#2")
	if _, err := s.Edit("b", store.Edit{Project: ptr("p")}); err != nil {
		t.Fatal(err)
	}

	// A link goes the same way; off its task and its pin, the task stays.
	run(0, "task", "--slug", "e", "--task", "#1", "--pin")
	run(0, "task", "--slug", "e", "--clear")
	if got := workOf(t, env, "e"); got != "null" {
		t.Errorf("--clear: %s", got)
	}
	run(0, "task", "--slug", "b", "--task", "")
	if got := workOf(t, env, "b"); got != "null" {
		t.Errorf(`--task "": %s`, got)
	}
	if _, ok := tasks()["p"]["#2"]; !ok {
		t.Error(`--task "" took the task`)
	}

	// The task alone: of the working directory, of --project, of none.
	run(0, "task", "--task", "#4", "--state", "done")
	run(0, "task", "--task", "#5", "--project", "shop", "--title", "five")
	run(0, "task", "--task", "#6", "--project", "", "--title", "six")
	if tasks()["p"]["#4"].State != "done" || tasks()["shop"]["#5"].State != "todo" || tasks()["/"]["#6"].State != "todo" {
		t.Errorf("tasks alone: %v", tasks())
	}
	run(0, "task", "--task", "#a", "--state", "archived")
	if tasks()["p"]["#a"].State != "archived" {
		t.Errorf("a task archived: %v", tasks()["p"]["#a"])
	}
	if got := run(1, "task", "--task", "#4", "--state", "no"); got != "floe: unknown state \"no\": use todo, doing, done, archived\n" {
		t.Errorf("a state of none: %q", got)
	}
	if got := run(1, "task", "--task", "#4", "--url", "ftp://x/y"); got != "floe: \"ftp://x/y\" is not an http or https URL\n" {
		t.Errorf("a URL of none: %q", got)
	}
	if got := run(1, "task", "--task", "#4", "--url", ""); got != "floe: "+i18n.New("en").Msg("store_task_url", "")+"\n" {
		t.Errorf("an empty URL: %q", got)
	}

	// Forgotten once no page is on it; none, and one with a page, say so.
	run(0, "task", "--task", "#4", "--forget")
	if _, ok := tasks()["p"]["#4"]; ok {
		t.Error("--forget left #4")
	}
	if got := run(1, "task", "--task", "#4", "--forget"); got != "floe: no task #4 in project p: check the ID, or name its project with --project\n" {
		t.Errorf("no task: %q", got)
	}
	if got := run(1, "task", "--task", "#1", "--forget"); !strings.HasPrefix(got, "floe: task #1 of project p still has 1 page(s), such as a:") {
		t.Errorf("a task with a page: %q", got)
	}

	// A parent, named with --task alone, written and taken off; refused in the store's words.
	run(0, "task", "--task", "#20", "--title", "parent")
	run(0, "task", "--task", "#21", "--parent", "#20")
	if got := tasks()["p"]["#21"].Parent; got != "#20" {
		t.Errorf("--parent: %q", got)
	}
	if got := run(1, "task", "--task", "#20", "--forget"); !strings.Contains(got, "still has 1 child task(s), such as #21") {
		t.Errorf("--forget of a parent: %q", got)
	}
	run(0, "task", "--task", "#21", "--parent", "")
	if got := tasks()["p"]["#21"].Parent; got != "" {
		t.Errorf(`--parent "": %q`, got)
	}
	if got := run(1, "task", "--task", "#22", "--parent", ""); got != "floe: no task #22 in project p: check the ID, or name its project with --project\n" {
		t.Errorf(`--parent "" of no row: %q`, got)
	}
	if got := run(1, "task", "--task", "#22", "--parent", "#7"); !strings.Contains(got, "project p has no task #7 that can be a parent") {
		t.Errorf("a parent of no row: %q", got)
	}
	run(0, "task", "--task", "#22", "--title", "t", "--parent", "")
	run(0, "task", "--task", "#22", "--forget")
	run(0, "task", "--task", "#20", "--forget")
	run(0, "task", "--task", "#21", "--forget")

	// A parent with pages and --task: the task and the page in one write, refused in the store's words with the page as
	// it was. --parent "" alone of a task of no row has nothing to take off: the page goes on it, which makes its row.
	run(0, "task", "--task", "#30", "--title", "parent")
	run(0, "task", "--slug", "b", "--task", "#31", "--parent", "#30")
	if got := tasks()["p"]["#31"].Parent; got != "#30" {
		t.Errorf("--parent with a page: %q", got)
	}
	if got := workOf(t, env, "b"); got != `{"task":"#31"}` {
		t.Errorf("--parent with a page put it on: %s", got)
	}
	if got := run(1, "task", "--slug", "b", "--task", "#32", "--parent", "#7"); !strings.Contains(got, "project p has no task #7 that can be a parent") {
		t.Errorf("a parent of no row with a page: %q", got)
	}
	if got := workOf(t, env, "b"); got != `{"task":"#31"}` {
		t.Errorf("a parent refused moved the page: %s", got)
	}
	run(0, "task", "--slug", "b", "--task", "#33", "--parent", "")
	if got := workOf(t, env, "b"); got != `{"task":"#33"}` {
		t.Errorf(`--parent "" of no row with a page put it on: %s`, got)
	}
	if got, ok := tasks()["p"]["#33"]; !ok || got.Parent != "" {
		t.Errorf(`--parent "" of no row with a page: the page's task has no row of no parent: %v`, tasks()["p"])
	}
	run(0, "task", "--slug", "b", "--task", "#33", "--parent", "", "--state", "doing")
	if row, ok := tasks()["p"]["#33"]; !ok || row.State != "doing" || row.Parent != "" {
		t.Errorf(`--parent "" and --state with a page: %v, %v`, row, ok)
	}
	run(0, "task", "--slug", "b", "--clear")
	// Pages of two projects (c is of q) take no parent: refused before anything is written.
	if got := run(2, "task", "--slug", "b", "--slug", "c", "--task", "#34", "--parent", "#30"); got != "floe: --parent with --slug goes with pages of one project: "+
		"give the pages of each project in their own command: run floe --help\n" {
		t.Errorf("--parent with pages of two projects: %q", got)
	}
	if _, ok := tasks()["p"]["#34"]; ok {
		t.Errorf("--parent with pages of two projects wrote #34: %v", tasks()["p"]["#34"])
	}
	if got := workOf(t, env, "b"); got != "null" {
		t.Errorf("--parent with pages of two projects moved b: %s", got)
	}

	// With no --task, a flag but --project is no way to tell the open tasks (--state done is no filter of them): refused
	// with nothing told, and the refusal names --task and says which command line tells them.
	for _, args := range [][]string{
		{"task", "--state", "done"},
		{"task", "--title", "T"},
		{"task", "--forget"},
		{"task", "--pin"},
		{"task", "--project", "p", "--state", "done"},
	} {
		if code, out, errs := runIn(dir, env, args...); code != 2 || out != "" ||
			!strings.HasPrefix(errs, "floe: ") || !strings.Contains(errs, "--task") || !strings.Contains(errs, "no flag but --project") {
			t.Errorf("floe %v = %d, %q, %q", args, code, out, errs)
		}
	}
	// The command line that cannot mean anything.
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"task", "--task", "#1", "--pin"}, "floe: --pin changes pages: name them with --slug: run floe --help\n"},
		{[]string{"task", "--task", "#1", "--category", "s"}, "floe: --category changes pages: name them with --slug: run floe --help\n"},
		{[]string{"task", "--slug", "a", "--forget"}, "floe: --forget goes with a task alone, not with pages: give it without --slug: run floe --help\n"},
		{[]string{"task", "--slug", "a", "--project", "p"}, "floe: --project goes with a task alone, not with pages: give it without --slug: run floe --help\n"},
		// Not a flag of the task, --parent would be dropped here and say nothing.
		{[]string{"task", "--slug", "a", "--task", "", "--parent", "x"}, "floe: --task \"\" takes the pages off their task, so it cannot go with --parent: run floe --help\n"},
		{[]string{"task", "--slug", "a", "--parent", "x"}, "floe: --parent with --slug goes with --task: name the one task the pages go on: run floe --help\n"},
	} {
		if got := run(2, c.args...); got != c.want {
			t.Errorf("floe %v = %q, want %q", c.args, got, c.want)
		}
	}
	if got := run(2, "task", "--slug", "a", "--pin", "--unpin"); !strings.Contains(got, "run floe --help") {
		t.Errorf("--pin --unpin: %q", got)
	}
	// Each flag that writes a task's field goes with neither --forget nor --clear
	for _, flag := range []string{"--state", "--url", "--remove-url", "--title", "--categories", "--parent"} {
		if got := run(2, "task", "--task", "#1", "--forget", flag, "x"); !strings.Contains(got, "run floe --help") {
			t.Errorf("--forget %s: %q", flag, got)
		}
		if got := run(2, "task", "--slug", "a", "--clear", flag, "x"); !strings.Contains(got, "run floe --help") {
			t.Errorf("--clear %s: %q", flag, got)
		}
	}
	// The names categories had before are no flags.
	for _, flag := range []string{"--step", "--steps"} {
		if got := run(2, "task", "--slug", "a", "--task", "#1", flag, "x"); !strings.Contains(got, "unknown flag: "+flag) {
			t.Errorf("%s: %q", flag, got)
		}
	}
	for _, flag := range []string{"--state", "--category"} {
		want := "floe: --task \"\" takes the pages off their task, so it cannot go with " + flag + ": run floe --help\n"
		if got := run(2, "task", "--slug", "a", "--task", "", flag, "done"); got != want {
			t.Errorf("--task \"\" %s: %q", flag, got)
		}
	}
	if got := run(0, "task", "--help"); !strings.Contains(got, "the task's state: todo, doing, done, archived") {
		t.Errorf("help: %q", got)
	}
	if got := run(1, "task", "--slug", "a", "--task", "  "); got != "floe: a task needs an ID\n" {
		t.Errorf("a blank name: %q", got)
	}
}

// floe task says each task and page it wrote on a line of its own, in the order written: a task made, changed or as it
// was with the fields asked for as the store left them, and a page with its task, category and pin after the write.
func TestWorkSays(t *testing.T) {
	t.Parallel()

	env, dir := served(t)
	say := func(want string, args ...string) {
		t.Helper()
		if code, out, errs := runIn(dir, env, args...); code != 0 || out != want || errs != "" {
			t.Errorf("floe %v = %d, %q, %q; want %q", args, code, out, errs, want)
		}
	}
	if code, out, errs := runIn(dir, env, "publish", "d.yaml", "--slug", "a"); code != 0 {
		t.Fatalf("publish = %d, %q, %q", code, out, errs)
	}
	say("task t1 in project p: made; title \"the task\"\n", "task", "--task", "t1", "--title", "the task")
	say("task t1 in project p: unchanged; title \"the task\"\n", "task", "--task", "t1", "--title", "the task")
	say("task t1 in project p: changed; state doing, categories \"design,build\"\n",
		"task", "--task", "t1", "--state", "doing", "--categories", "design, build, design")
	say("task t2 in no project: made; urls \"https://example.com/2\" \"https://example.com/3\", no parent\n",
		"task", "--task", "t2", "--project", "", "--url", " https://example.com/2 ", "--url", "https://example.com/3", "--parent", "")
	// --remove-url alone is a write of the task
	say("task t2 in no project: changed; urls \"https://example.com/3\"\n",
		"task", "--task", "t2", "--project", "", "--remove-url", "https://example.com/2")
	say("task t2 in no project: changed; no urls\n", "task", "--task", "t2", "--project", "", "--remove-url", "https://example.com/3")
	// The task named is written, with nothing of it to change; a category is the page's alone.
	// A task of a row, named, with nothing of it to change, is not told; a category among its categories adds none.
	say("page a: task t1 in project p, category \"design\"\n", "task", "--slug", "a", "--task", "t1", "--category", "design")
	say("page a: task t1 in project p, category \"build\"\n", "task", "--slug", "a", "--category", "build")
	say("page a: task t1 in project p, category \"build\", pinned\n", "task", "--slug", "a", "--pin")
	say("page a: no task, pinned\n", "task", "--slug", "a", "--task", "")
	say("task t1 in project p: forgotten\n", "task", "--task", " t1 ", "--forget")
	say("task t3 in project p: made\npage a: task t3 in project p, pinned\n", "task", "--slug", "a", "--task", "t3")
	say("task t4 in project p: made; parent \"t3\"\n", "task", "--task", "t4", "--parent", "t3")
}

// A page that cannot be written, as the system refuses it after the checks, leaves its task's row written, which a
// page needs anyway: the write breaks no rule, and the same command given again finishes it.
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
	if code, out, errs := runIn(dir, env, "task", "--slug", "a", "--task", "#1", "--state", "done"); code != 1 || out != "" {
		t.Fatalf("work on a page that cannot be written = %d, %q, %q; want refused, nothing told", code, out, errs)
	}
	s := store.New(env["FLOE_DIR"])
	all, err := s.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	if got := all["p"]["#1"].State; got != "done" {
		t.Errorf("the task was not written first: %v", all)
	}
	if got := workOf(t, env, "a"); got != "null" {
		t.Errorf("the page: %s", got)
	}
	if err := os.Chmod(page, 0o700); err != nil {
		t.Fatal(err)
	}
	if code, out, errs := runIn(dir, env, "task", "--slug", "a", "--task", "#1", "--state", "done"); code != 0 ||
		out != "task #1 in project p: unchanged; state done\npage a: task #1 in project p\n" {
		t.Fatalf("given again = %d, %q, %q", code, out, errs)
	}
	if got := workOf(t, env, "a"); got != `{"task":"#1"}` {
		t.Errorf("given again, the page: %s", got)
	}
}

// A value of floe task's flags that is no UTF-8 is told by its flag, before anything is asked of the server: a task's
// body and its name and project alike, and a page not there is not looked for.
func TestWorkNotUTF8(t *testing.T) {
	t.Parallel()

	env, dir := served(t)
	if code, out, errs := runIn(dir, env, "publish", "d.yaml", "--slug", "a"); code != 0 {
		t.Fatalf("publish = %d, %q, %q", code, out, errs)
	}
	bad := "a\xffb"
	// darwin's APFS names no file so, and the project of the working directory is not tried there.
	elsewhere := filepath.Join(filepath.Dir(dir), bad)
	if err := os.Mkdir(elsewhere, 0o700); err != nil {
		if runtime.GOOS != "darwin" {
			t.Fatal(err)
		}
		elsewhere = ""
	}
	for _, c := range []struct {
		at   string
		args []string
		told string
	}{
		{dir, []string{"task", "--slug", "a", "--task", "t1", "--category", bad}, "--category"},
		{dir, []string{"task", "--slug", "a", "--task", "t1", "--url", bad}, "--url"},
		{dir, []string{"task", "--task", "t1", "--remove-url", bad}, "--remove-url"},
		{dir, []string{"task", "--slug", "a", "--task", "t1", "--categories", "x," + bad}, "--categories"},
		{dir, []string{"task", "--task", "t1", "--parent", bad}, "--parent"},
		{dir, []string{"task", "--task", "t1", "--state", bad}, "--state"},
		{dir, []string{"task", "--task", "t1", "--title", bad}, "--title"},
		{dir, []string{"task", "--task", bad}, "--task"},
		{dir, []string{"task", "--slug", "a", "--task", bad}, "--task"},
		{dir, []string{"task", "--task", "t1", "--project", bad}, "--project"},
		{dir, []string{"task", "--task", "t1", "--project", bad, "--forget"}, "--project"},
		{dir, []string{"task", "--slug", "nosuch", "--task", "t1", "--url", bad}, "--url"},
		{
			elsewhere,
			[]string{"task", "--task", "t1"},
			"the project name (of the git remote, the repository's directory or the working directory)",
		},
	} {
		if c.at == "" {
			continue
		}
		want := "floe: " + c.told + " is not UTF-8; the server takes only UTF-8: rename it, or give it in UTF-8\n"
		if code, out, errs := runIn(c.at, env, c.args...); code != 1 || out+errs != want {
			t.Errorf("floe %q = %d, %q, %q; want %q", c.args, code, out, errs, want)
		}
	}
	if all, err := store.New(env["FLOE_DIR"]).ReadTasks(); err != nil || len(all) != 0 {
		t.Errorf("tasks written: %v, %v", all, err)
	}
	if got := workOf(t, env, "a"); got != "null" {
		t.Errorf("the page written: %s", got)
	}
}

// floe task with --slug and no other flag tells each page as it is, as a write tells it, and writes nothing, not even
// the row of a task a page names that has none; a slug of no page is refused before any page is told.
func TestWorkReadsPages(t *testing.T) {
	t.Parallel()

	env, dir := served(t)
	run := func(want int, args ...string) string {
		t.Helper()
		code, out, errs := runIn(dir, env, args...)
		if code != want {
			t.Fatalf("floe %v = %d, %q, %q", args, code, out, errs)
		}
		return out + errs
	}
	// What a write would change: tasks.json and each page's meta.json.
	written := func() string {
		t.Helper()
		var all []string
		err := filepath.WalkDir(env["FLOE_DIR"], func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || (d.Name() != "meta.json" && d.Name() != "tasks.json") {
				return err
			}
			b, err := os.ReadFile(path) //nolint:gosec // G304: the test's own store
			all = append(all, path+"\n"+string(b))
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		return strings.Join(all, "\n")
	}
	run(0, "publish", "w.yaml", "--slug", "a")
	run(0, "publish", "d.yaml", "--slug", "b")
	run(0, "publish", "h.html", "--link", "--slug", "c")
	run(0, "task", "--slug", "a", "--task", "#1", "--category", "designs", "--pin")
	run(0, "task", "--slug", "c", "--task", "#1")
	run(0, "archive", "c")
	// The pages name a task of no row, as a hand edit leaves: a write would make it, a read leaves it out.
	if err := os.Remove(filepath.Join(env["FLOE_DIR"], "tasks.json")); err != nil {
		t.Fatal(err)
	}

	before := written()
	want := "page a: task #1 in project p, category \"designs\", pinned\n" +
		"page b: no task\n" +
		"page c: task #1 in project p\n"
	if got := run(0, "task", "--slug", "a", "--slug", "b", "--slug", "c"); got != want {
		t.Errorf("floe task --slug a --slug b --slug c = %q, want %q", got, want)
	}
	if got := run(0, "task", "--slug", "b"); got != "page b: no task\n" {
		t.Errorf("floe task --slug b = %q", got)
	}
	if got := run(1, "task", "--slug", "a", "--slug", "nope"); strings.Contains(got, "page a") || !strings.Contains(got, `"nope"`) {
		t.Errorf("floe task --slug a --slug nope = %q", got)
	}
	if after := written(); after != before {
		t.Errorf("reading wrote:\n%s\nwant\n%s", after, before)
	}
}

// A category is the page's alone: given without --task, it asks nothing of tasks.json; with --task, the task named is
// A page's category is always among its task's categories: added at the end, in the same write, which moves the
// task's rev and says so; an order written keeps a category with pages, and takes the page's at its end. A category
// needs the page on a task.
func TestWorkCategoryAdded(t *testing.T) {
	t.Parallel()

	env, dir := served(t)
	run := func(want int, args ...string) string {
		t.Helper()
		code, out, errs := runIn(dir, env, args...)
		if code != want {
			t.Fatalf("floe %v = %d, %q, %q", args, code, out, errs)
		}
		return out + errs
	}
	tasksFile := func() string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(env["FLOE_DIR"], "tasks.json"))
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		return string(b)
	}
	row := func() store.Task {
		t.Helper()
		all, err := store.New(env["FLOE_DIR"]).ReadTasks()
		if err != nil {
			t.Fatal(err)
		}
		return all["p"]["#1"]
	}
	run(0, "publish", "w.yaml", "--slug", "a")
	run(0, "publish", "d.yaml", "--slug", "b")
	run(0, "task", "--slug", "a", "--task", "#1")

	if got := run(0, "task", "--slug", "a", "--category", "x"); got != "task #1 in project p: changed; categories \"x\"\npage a: task #1 in project p, category \"x\"\n" {
		t.Errorf("--category alone = %q", got)
	}
	if got := row(); !slices.Equal(got.Categories, []string{"x"}) || got.Rev != 2 {
		t.Errorf("--category alone: %+v", got)
	}
	before := tasksFile()
	if got := run(0, "task", "--slug", "a", "--category", "x"); got != "page a: task #1 in project p, category \"x\"\n" {
		t.Errorf("--category among them = %q", got)
	}
	if got := tasksFile(); got != before {
		t.Errorf("--category among them wrote tasks.json:\n%s\nwant\n%s", got, before)
	}
	if got := run(0, "task", "--slug", "a", "--task", "#1", "--categories", "y", "--category", "z"); got != "task #1 in project p: changed; categories \"y,z\"\npage a: task #1 in project p, category \"z\"\n" {
		t.Errorf("--categories with another --category = %q", got)
	}
	// The page moves in the same write: an order without its old category is written.
	run(0, "task", "--slug", "a", "--categories", "n", "--category", "n")
	if got := row().Categories; !slices.Equal(got, []string{"n"}) {
		t.Errorf("moved within the write: %v", got)
	}
	// A category among them is not said, though the row is written for another field.
	if got := run(0, "task", "--slug", "a", "--category", "n", "--state", "doing"); got != "task #1 in project p: changed; state doing\npage a: task #1 in project p, category \"n\"\n" {
		t.Errorf("a category among them, with a field = %q", got)
	}
	before = tasksFile()
	if got := run(1, "task", "--task", "#1", "--categories", "m"); !strings.Contains(got, "category n of task #1 in project p has pages (a)") {
		t.Errorf("a category with pages left out = %q", got)
	}
	if got := tasksFile(); got != before {
		t.Errorf("a refusal wrote tasks.json:\n%s\nwant\n%s", got, before)
	}

	if got := run(1, "task", "--slug", "b", "--category", "x"); !strings.Contains(got, `"b" is on no task`) {
		t.Errorf("a page of no task = %q", got)
	}
	if got := run(2, "task", "--slug", "a", "--task", "", "--category", "x"); !strings.Contains(got, "cannot go with --category") {
		t.Errorf(`--task "" --category = %q`, got)
	}
	if got := workOf(t, env, "b"); got != "null" {
		t.Errorf("a refusal wrote b: %s", got)
	}
	if got := workOf(t, env, "a"); got != `{"task":"#1","category":"n"}` {
		t.Errorf("a refusal wrote a: %s", got)
	}
}

// floe task --task with no field to change tells the task as the store has it, writing nothing; a task of no row is
// refused, and none is made.
func TestWorkReadsTask(t *testing.T) {
	t.Parallel()

	env, dir := served(t)
	tasksFile := func() string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(env["FLOE_DIR"], "tasks.json"))
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		return string(b)
	}
	if code, out, errs := runIn(dir, env, "task", "--task", "#1"); code != 1 || out != "" ||
		errs != "floe: no task #1 in project p: check the ID, or name its project with --project\n" {
		t.Errorf("no row = %d, %q, %q", code, out, errs)
	}
	if got := tasksFile(); got != "" {
		t.Errorf("reading made a row: %s", got)
	}
	if code, out, errs := runIn(dir, env, "task", "--task", "#1", "--title", "T", "--state", "doing", "--categories", "a,b"); code != 0 {
		t.Fatalf("written = %d, %q, %q", code, out, errs)
	}
	before := tasksFile()
	if code, out, errs := runIn(dir, env, "task", "--task", "#1"); code != 0 ||
		out != "task #1 in project p: state doing, title \"T\", categories \"a,b\"\n" || errs != "" {
		t.Errorf("read = %d, %q, %q", code, out, errs)
	}
	if got := tasksFile(); got != before {
		t.Errorf("reading wrote:\n%s\nwant\n%s", got, before)
	}
}

// floe task with no slug and no flag tells the open tasks of the working directory's project (--project names another),
// a line each as floe task --task tells the task, the latest change first and by name among the same time, writing
// nothing. The tasks are written by hand, with times that are in one order as text and another as times, and ones that
// are done, archived or of another project.
func TestWorkListsOpenTasks(t *testing.T) {
	t.Parallel()

	env, dir := served(t)
	hand := `{
  "p": {
    "T1": {"state": "doing", "at": "2026-11-12T10:00:00+09:00", "title": "Fix login", "urls": ["https://example.com/1", "https://example.com/2"], "parent": "#1", "categories": ["a", "b"]},
    "T2": {"state": "todo", "at": "2026-11-12T09:30:00Z"},
    "T3": {"state": "todo", "at": "2026-11-12T09:30:00.500Z", "title": "Third"},
    "#1": {"state": "todo", "at": "2026-11-11T09:00:00Z"},
    "#10": {"state": "todo", "at": "2026-11-11T09:00:00Z"},
    "#2": {"state": "todo", "at": "2026-11-11T09:00:00Z"},
    "B": {"state": "doing", "at": "2026-11-11T09:00:00Z"},
    "Z9": {"state": "todo", "at": "2026-11-11T18:00:00+09:00"},
    "a": {"state": "todo", "at": "2026-11-11T09:00:00Z"},
    "done": {"state": "done", "at": "2026-11-13T00:00:00Z"},
    "archived": {"state": "archived", "at": "2026-11-13T00:00:00Z"}
  },
  "q": {"Q1": {"state": "doing", "at": "2026-11-14T00:00:00Z"}},
  "/": {"N1": {"state": "todo", "at": "2026-11-14T00:00:00Z"}}
}`
	file := filepath.Join(env["FLOE_DIR"], "tasks.json")
	if err := os.WriteFile(file, []byte(hand), 0o600); err != nil {
		t.Fatal(err)
	}
	// The line of each task is what floe task --task tells of it, not a text written here.
	told := func(project string, names ...string) string {
		t.Helper()
		var lines string
		for _, name := range names {
			code, out, errs := runIn(dir, env, "task", "--task", name, "--project", project)
			if code != 0 || errs != "" {
				t.Fatalf("floe task --task %s = %d, %q, %q", name, code, out, errs)
			}
			lines += out
		}
		return lines
	}
	want := told("p", "T3", "T2", "T1", "#1", "#10", "#2", "B", "Z9", "a")
	if !strings.Contains(want, `title "Fix login", urls "https://example.com/1" "https://example.com/2", categories "a,b", parent "#1"`) {
		t.Fatalf("the lines to compare have no task with every field: %s", want)
	}
	if code, out, errs := runIn(dir, env, "task"); code != 0 || out != want || errs != "" {
		t.Errorf("floe task = %d, %q, %q; want the lines\n%s", code, out, errs, want)
	}
	// The project is told as tasks.json keys it: "" is the key "/", and a name is trimmed, as ProjectKey does.
	for _, c := range []struct{ flag, project, task string }{{"q", "q", "Q1"}, {" q ", "q", "Q1"}, {"", "", "N1"}} {
		if code, out, errs := runIn(dir, env, "task", "--project", c.flag); code != 0 || out != told(c.project, c.task) || errs != "" {
			t.Errorf("floe task --project %q = %d, %q, %q", c.flag, code, out, errs)
		}
	}
	if b, err := os.ReadFile(file); err != nil || string(b) != hand {
		t.Errorf("telling the tasks wrote tasks.json: %s, %v", b, err)
	}
}

// With no open task, floe task says so, and is not refused: for a project of no task at all (which makes no tasks.json),
// of done tasks only, and of no project.
func TestWorkListsNone(t *testing.T) {
	t.Parallel()

	env, dir := served(t)
	file := filepath.Join(env["FLOE_DIR"], "tasks.json")
	if code, out, errs := runIn(dir, env, "task"); code != 0 || out != "no open task in project p\n" || errs != "" {
		t.Errorf("no task = %d, %q, %q", code, out, errs)
	}
	if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("telling no task made tasks.json: %v", err)
	}
	// A task is made with a field to write: --task alone only reads.
	if code, out, errs := runIn(dir, env, "task", "--task", "#1", "--state", "done"); code != 0 {
		t.Fatalf("done task = %d, %q, %q", code, out, errs)
	}
	if code, out, errs := runIn(dir, env, "task"); code != 0 || out != "no open task in project p\n" || errs != "" {
		t.Errorf("done tasks only = %d, %q, %q", code, out, errs)
	}
	if code, out, errs := runIn(dir, env, "task", "--project", ""); code != 0 || out != "no open task in no project\n" || errs != "" {
		t.Errorf("no project = %d, %q, %q", code, out, errs)
	}
}

// The project of the list is the one floe publish puts a data file in, and floe task --task names: the repository's
// name by its origin, not the directory's.
func TestWorkListsOfPublishedProject(t *testing.T) {
	t.Parallel()

	env, home := served(t)
	repo := filepath.Join(filepath.Dir(home), "r")
	if err := os.MkdirAll(repo, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "d.yaml"), []byte("lang: en\ntemplate: pic\ntitle: D\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "https://example.com/o/named.git"}} {
		git := exec.CommandContext(t.Context(), "git", args...)
		git.Dir = repo
		git.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
		if out, err := git.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if code, _, errs := runIn(repo, env, "publish", "d.yaml"); code != 0 || !strings.Contains(errs, "page in project named") {
		t.Fatalf("publish = %d, %q: the project is not the origin's name", code, errs)
	}
	for _, args := range [][]string{
		{"--task", "X", "--state", "todo"},
		{"--task", "Y", "--state", "todo", "--project", "r"},
	} {
		if code, out, errs := runIn(repo, env, append([]string{"task"}, args...)...); code != 0 {
			t.Fatalf("floe task %v = %d, %q, %q", args, code, out, errs)
		}
	}
	if code, out, errs := runIn(repo, env, "task"); code != 0 || out != "task X in project named: state todo\n" || errs != "" {
		t.Errorf("floe task = %d, %q, %q; want the tasks of project named, not of r", code, out, errs)
	}
}

// A link whose project comes back to its directory's, after an edit emptied it, is taken off its task, and says so.
func TestLinkOffTask(t *testing.T) {
	t.Parallel()

	env, dir := served(t)
	if code, out, errs := runIn(dir, env, "publish", "--link", "h.html", "--slug", "l"); code != 0 {
		t.Fatalf("link = %d, %q, %q", code, out, errs)
	}
	if code, out, errs := runIn(dir, env, "task", "--slug", "l", "--task", "#1"); code != 0 {
		t.Fatalf("work = %d, %q, %q", code, out, errs)
	}
	s := store.New(env["FLOE_DIR"])
	if _, err := s.Edit("l", store.Edit{Project: ptr("")}); err != nil {
		t.Fatal(err)
	}
	// The edit moved it to no project: off its task already; put back on that project's task
	if code, out, errs := runIn(dir, env, "task", "--slug", "l", "--task", "#1"); code != 0 {
		t.Fatalf("work again = %d, %q, %q", code, out, errs)
	}
	code, _, errs := runIn(dir, env, "publish", "--link", "h.html", "--slug", "l")
	if code != 0 || !strings.Contains(errs, "page in project p, off task #1") {
		t.Errorf("linked again = %d, %q", code, errs)
	}
	if w := s.ReadMeta("l").Work; w != nil {
		t.Errorf("linked again, the page's work: %+v", w)
	}
}

// floe task takes its pages by --slug, not as arguments: an argument is refused before anything is asked of the server,
// whatever the flags; floe work, its name before, is no command.
func TestTaskTakesNoArgs(t *testing.T) {
	t.Parallel()

	env, dir := served(t)
	for _, args := range [][]string{{"task", "a"}, {"task", "a", "--task", "#1"}, {"task", "--task", "#1", "a"}} {
		code, out, errs := runIn(dir, env, args...)
		if code != 2 || out != "" || !strings.Contains(errs, `unknown command "a" for "floe task"`) {
			t.Errorf("floe %v = %d, %q, %q", args, code, out, errs)
		}
	}
	if code, _, errs := runIn(dir, env, "work"); code != 2 || !strings.Contains(errs, `unknown command "work" for "floe"`) {
		t.Errorf("floe work = %d, %q", code, errs)
	}
	if _, err := os.Stat(filepath.Join(env["FLOE_DIR"], "tasks.json")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("an argument wrote tasks.json: %v", err)
	}
}
