package browsertest_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/store"
)

// A task's family on the screens: a row says its parent or how many children, which change neither the rows nor their
// number; a parent's page lists its children by their view, off the rows j and k go through; a child's page leads to its
// parent; a parent with no page of its own goes by its children's; a parent written by hand that the store would not count
// makes no family, and a task of no row is no parent.
func TestUITaskChildren(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	b.navigate("/")
	page := func(project, slug string, args ...string) {
		t.Helper()
		s.floe(t, "publish", s.page(t, slug+".yaml", "T "+slug, ""), "--slug", slug)
		if _, err := st.Edit(slug, store.Edit{Project: &project}); err != nil {
			t.Fatal(err)
		}
		s.floe(t, append([]string{"work", slug}, args...)...)
	}
	task := func(project string, args ...string) {
		t.Helper()
		s.floe(t, append([]string{"work", "--project", project, "--task"}, args...)...)
	}
	page("proj", "p1", "--task", "#1", "--state", "doing", "--title", "the parent")
	page("proj", "p1b", "--task", "#1")
	page("proj", "c2", "--task", "#2", "--state", "waiting", "--title", "waiting child")
	page("proj", "c3", "--task", "#3", "--state", "done", "--title", "archived child")
	page("proj", "c4", "--task", "#4", "--state", "doing", "--title", "doing child")
	for _, child := range []string{"#2", "#3", "#4"} {
		task("proj", child, "--parent", "#1")
	}
	s.floe(t, "archive", "c3")
	// No page of its own: its children's say whether it is archived.
	task("proj", "#10", "--state", "done", "--title", "gone by its child")
	page("proj", "c11", "--task", "#11", "--state", "done")
	task("proj", "#11", "--parent", "#10")
	s.floe(t, "archive", "c11")
	task("proj", "#20", "--state", "done", "--title", "kept by its child")
	page("proj", "c21", "--task", "#21", "--state", "doing")
	task("proj", "#21", "--parent", "#20")
	// Its own page says, a child at work or not.
	page("proj", "p30", "--task", "#30", "--state", "done")
	page("proj", "c31", "--task", "#31", "--state", "doing")
	task("proj", "#31", "--parent", "#30")
	s.floe(t, "archive", "p30")
	// Of the same name in another project: nothing of it shows here.
	page("other", "o1", "--task", "#1", "--state", "doing")
	task("other", "#5", "--parent", "#1")

	rows := func(view string) string {
		t.Helper()
		b.navigate("/tasks?project=proj&view=" + view)
		b.until("drawn", "!document.getElementById('tasks-none').hidden || document.querySelector('#list > li .tname')")
		if count, shown := b.eval("document.querySelectorAll('#list > li').length + ''"), b.eval("document.querySelector('.views button[data-view="+js(view)+"]').textContent.match(/\\d+/)[0]"); count != shown {
			t.Errorf("%s: %s rows, %s on the shelf", view, count, shown)
		}
		return b.eval("[...document.querySelectorAll('#list > li')].map((li) => li.querySelector('.tname').textContent + [...li.querySelectorAll('.kin, .task-pages')].filter((e) => e.matches('.kin') || /child/.test(e.textContent)).map((e) => ' (' + e.textContent + ')').join('')).sort().join(', ')")
	}
	for _, c := range []struct{ view, want string }{
		{"waiting", "#2 (Parent#1the parent)"},
		{"doing", "#1 (3 child task(s)), #21 (Parent#20kept by its child), #31 (Parent#30), #4 (Parent#1the parent)"},
		{"done", "#20 (1 child task(s))"},
		{"archived", "#10 (1 child task(s)), #11 (Parent#10gone by its child), #3 (Parent#1the parent), #30 (1 child task(s))"},
	} {
		if got := rows(c.view); got != c.want {
			t.Errorf("%s = %q, want %q", c.view, got, c.want)
		}
	}

	// A child not done keeps a parent of no page from the archive, with none of its pages to say so.
	task("proj", "#12", "--state", "doing", "--title", "at work")
	task("proj", "#12", "--parent", "#10")
	if got := rows("done"); got != "#10 (2 child task(s)), #20 (1 child task(s))" {
		t.Errorf("a child at work under a parent of no page: %q", got)
	}
	task("proj", "#12", "--parent", "")
	task("proj", "#12", "--forget")

	open := func(name string) {
		t.Helper()
		b.navigate("/tasks?project=proj&task=" + strings.ReplaceAll(name, "#", "%23"))
		b.until("the head", "!document.getElementById('task-head').hidden")
	}
	const kids = "[...document.querySelectorAll('#task-family .kin-row')].map((e) => e.dataset.task + ' ' + e.querySelector('.kid-state').textContent).join(', ')"

	// The children by view; j and k stay on the pages, and a press goes to the child's page.
	open("#1")
	if got := b.eval(kids); got != "#2 Waiting, #4 In progress, #3 Archive" {
		t.Errorf("the children: %q", got)
	}
	b.eval("document.querySelector('#list a.title').focus()")
	b.pressKey("j")
	b.pressKey("j")
	b.pressKey("j")
	if got := b.eval("document.activeElement.closest('li')?.dataset.slug ?? document.activeElement.id"); got != "p1b" {
		t.Errorf("j stays on the pages, at %q", got)
	}

	// The focus stays on a child as the list is read again, and comes back when it is drawn anew.
	b.eval("window.kidNode = document.querySelector('#task-family .kin-row[data-task=\"#4\"]'); window.kidNode.focus()")
	b.eval("load()")
	b.still()
	if got := b.eval("(document.activeElement === window.kidNode) + ''"); got != "true" {
		t.Error("a child drawn anew with nothing changed")
	}
	task("proj", "#2", "--title", "renamed")
	b.until("drawn anew", "document.querySelector('#task-family').textContent.includes('renamed')")
	if got := b.eval("(document.activeElement.dataset.task ?? '') + ' ' + (document.activeElement === window.kidNode)"); got != "#4 false" {
		t.Errorf("the focus after a change: %q", got)
	}
	b.eval("document.querySelector('#task-family .kin-row[data-task=\"#2\"]').click()")
	b.until("the child's page", "new URLSearchParams(location.search).get('task') === '#2' && !document.getElementById('task-family').hidden")

	// A child leads up to its parent, and its way back is to its own state.
	if got := b.eval("document.querySelector('#task-family h3').textContent + ' ' + document.querySelector('#task-family .kin-row').textContent + ' ' + document.querySelector('#task-family .kin-row').getAttribute('href') + ' ' + document.getElementById('task-up').getAttribute('href')"); got != "Parent task #1the parentIn progress /tasks?project=proj&task=%231 /tasks?project=proj" {
		t.Errorf("a child's head: %q", got)
	}

	// A parent of no page: words in place of a command that would be refused; nothing beside it says to choose a row.
	open("#20")
	if got := b.eval("[document.getElementById('task-none').hidden, document.getElementById('task-delete-box').hidden, document.getElementById('task-none-kids').hidden, getComputedStyle(document.getElementById('read-blank')).display].join(' ')"); got != "false true false none" {
		t.Errorf("a parent of no page: %q", got)
	}
	task("proj", "#99", "--title", "no page, no child")
	open("#99")
	if got := b.eval("[document.getElementById('task-delete-box').hidden, document.getElementById('task-none-kids').hidden, document.getElementById('task-family').hidden].join(' ')"); got != "false true false" {
		t.Errorf("no page, no child: %q", got)
	}

	// A child made from a task's page, of no page: it stays there, with the new row and a way to it; of a task with a
	// parent there is no way, and one that comes to have one meanwhile is refused in the dialog.
	const adds = "document.getElementById('task-child').hidden + ''"
	openChild := func() {
		t.Helper()
		b.eval("document.getElementById('task-child').click()")
		b.until("the dialog", "document.getElementById('child-dialog').open")
	}
	child := func(name, title string) {
		t.Helper()
		b.eval("document.getElementById('child-task').value = " + js(name) + "; document.getElementById('child-title').value = " + js(title) + "; document.getElementById('child').requestSubmit()")
	}
	open("#99")
	if got := b.eval(adds); got != "false" {
		t.Errorf("no way from a task of no page: %s", got)
	}
	openChild()
	child(" #100 ", "made here")
	b.until("saved", "!document.getElementById('child-dialog').open")
	b.until("its row", "document.querySelector('#task-family .kin-row[data-task=\"#100\"]')")
	if got := b.eval("new URLSearchParams(location.search).get('task') + ' ' + document.querySelector('.toast-link').textContent + ' ' + document.querySelector('.toast-link').getAttribute('href')"); got != "#99 Made the child task #100 /tasks?project=proj&task=%23100" {
		t.Errorf("after making: %q", got)
	}
	made, err := st.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	if got := made["proj"]["#100"]; got.Parent != "#99" || got.Title != "made here" || got.State != "doing" {
		t.Errorf("made: %+v", got)
	}
	open("#100")
	if got := b.eval(adds); got != "true" {
		t.Errorf("a way from a child: %s", got)
	}
	task("proj", "#100", "--forget")
	open("#99")
	openChild()
	task("proj", "#99", "--parent", "#1")
	child("#101", "")
	b.until("refused", "!document.getElementById('child-error').hidden")
	if got := b.eval("document.getElementById('child-error').textContent + ' ' + document.getElementById('child-dialog').open"); !strings.Contains(got, "task #99 has a parent itself (#1)") || !strings.HasSuffix(got, "true") {
		t.Errorf("refused: %q", got)
	}
	if made, err = st.ReadTasks(); err != nil {
		t.Fatal(err)
	}
	if _, ok := made["proj"]["#101"]; ok {
		t.Error("the refused child has a row")
	}
	b.eval("document.getElementById('child-dialog').close()")
	task("proj", "#99", "--parent", "")

	// By real presses: a new name makes a child, and the focus is back on the button.
	press := func(id string) {
		t.Helper()
		b.still()
		r := b.rect("document.getElementById(" + js(id) + ")")
		b.click(r.X+r.Width/2, r.Y+r.Height/2)
	}
	write := func(name, title string) {
		t.Helper()
		press("task-child")
		b.until("the dialog", "document.getElementById('child-dialog').open")
		b.eval("document.getElementById('child-task').value = " + js(name) + "; document.getElementById('child-title').value = " + js(title))
		press("child-submit")
	}
	open("#99")
	write("#103", "")
	b.until("saved", "!document.getElementById('child-dialog').open")
	b.until("its row", "document.querySelector('#task-family .kin-row[data-task=\"#103\"]')")
	if got := b.eval("document.activeElement.id"); got != "task-child" {
		t.Errorf("the focus after making one: %q", got)
	}
	// A task on the list is offered, and said before it is done: chosen by a press, it moves from its parent, keeping its
	// state and its title with none written. One with children, typed, is refused with the dialog as written.
	press("task-child")
	b.until("the dialog", "document.getElementById('child-dialog').open")
	if got := b.eval("document.getElementById('child-submit').textContent"); got != "Make" {
		t.Errorf("the button of no name: %q", got)
	}
	field := func(value string) {
		t.Helper()
		b.eval("document.getElementById('child-task').value = " + js(value) + "; document.getElementById('child-task').dispatchEvent(new Event('input', { bubbles: true }))")
	}
	field("#")
	offered := b.eval("[...document.getElementById('child-task-names').children].map((e) => e.querySelector('.names-word').textContent.split(' ')[0]).join(' ')")
	for _, name := range []string{" #1 ", " #10 ", " #103 ", " #99 "} {
		if strings.Contains(" "+offered+" ", name) {
			t.Errorf("offered %s: %q", name, offered)
		}
	}
	if !strings.Contains(" "+offered+" ", " #4 ") {
		t.Errorf("not offered #4: %q", offered)
	}
	if got := b.eval("[...document.getElementById('child-task-names').children].find((e) => e.textContent.startsWith('#4')).textContent"); !strings.Contains(got, "· In progress · parent #1") {
		t.Errorf("what is said of #4: %q", got)
	}
	// A name typed whole closes the names, which would cover what is said; no title goes with a task that has one.
	field("#4")
	const said = "[document.getElementById('child-task-names').matches(':popover-open'), document.getElementById('child-note').hidden ? '' : document.getElementById('child-note').textContent, document.getElementById('child-submit').textContent, document.getElementById('child-title').closest('label').hidden].join(' | ')"
	if got := b.eval(said); got != "false | #4 doing child becomes a child, moving from its parent #1 | Make it a child | true" {
		t.Errorf("#4 typed: %q", got)
	}
	// One that cannot be a child is not said to be; a name of none is made, with a title.
	field("#20")
	if got := b.eval(said); got != "false |  | Make it a child | true" {
		t.Errorf("#20, with children: %q", got)
	}
	field("#99")
	if got := b.eval(said); got != "false |  | Make it a child | true" {
		t.Errorf("itself: %q", got)
	}
	field("#777")
	if got := b.eval(said); got != "false |  | Make | false" {
		t.Errorf("a new name: %q", got)
	}
	// Chosen by a press, as typed.
	field("#")
	r := b.rect("[...document.getElementById('child-task-names').children].find((e) => e.textContent.startsWith('#4'))")
	b.click(r.X+r.Width/2, r.Y+r.Height/2)
	if got := b.eval(said); got != "false | #4 doing child becomes a child, moving from its parent #1 | Make it a child | true" {
		t.Errorf("#4 chosen: %q", got)
	}
	b.eval("document.getElementById('child-title').value = 'not sent'")
	press("child-submit")
	b.until("saved", "!document.getElementById('child-dialog').open")
	b.until("the toast", "document.querySelector('.toast-link')?.textContent === 'Made #4 a child task'")
	if made, err = st.ReadTasks(); err != nil {
		t.Fatal(err)
	}
	if got := made["proj"]["#4"]; got.Parent != "#99" || got.State != "doing" || got.Title != "doing child" {
		t.Errorf("#4 taken: %+v", got)
	}
	task("proj", "#4", "--parent", "#1")
	// One that is a child here already is not said to be made one: the press writes nothing, and tells nothing.
	open("#1")
	write("#4", "")
	b.until("closed", "!document.getElementById('child-dialog').open")
	if got := b.eval("(document.querySelector('.toast-link')?.textContent ?? '') + '|' + document.activeElement.id"); got != "|task-child" {
		t.Errorf("a child here already: %q", got)
	}
	press("task-child")
	b.until("the dialog", "document.getElementById('child-dialog').open")
	field("#4")
	if got := b.eval(said); got != "false |  | Make it a child | true" {
		t.Errorf("a child here already, typed: %q", got)
	}
	b.eval("document.getElementById('child-dialog').close()")
	open("#99")
	write("#20", "")
	b.until("refused", "!document.getElementById('child-error').hidden")
	want := "Could not save: task #20 has child tasks, such as #21, so it cannot have a parent: tasks have one level of children | #20 |  | true"
	if got := b.eval("[document.getElementById('child-error').textContent, document.getElementById('child-task').value, document.getElementById('child-title').value, document.getElementById('child-dialog').open].join(' | ')"); got != want {
		t.Errorf("a task with children: %q", got)
	}
	b.eval("document.getElementById('child-dialog').close()")
	task("proj", "#103", "--forget")

	// Removed by two presses and back at the list; refused, it says why and stays. A task with children has no button.
	pressForget := func() {
		t.Helper()
		press("task-delete")
		b.until("armed", "document.getElementById('task-delete').classList.contains('armed')")
		press("task-delete")
	}
	task("proj", "#104", "--title", "to remove", "--state", "waiting")
	open("#104")
	if got := b.eval("getComputedStyle(document.getElementById('task-delete')).color === getComputedStyle(document.getElementById('bulk-delete')).color"); got != "true" {
		t.Error("the button is not of a delete's color")
	}
	b.eval("window.realFetch = window.fetch; window.fetch = (url, init) => (init?.method === 'DELETE' ? Promise.resolve(new Response('it has a page', { status: 400 })) : window.realFetch(url, init))")
	pressForget()
	b.until("refused", "!document.getElementById('task-delete-error').hidden")
	if got := b.eval("document.getElementById('task-delete-error').textContent + ' | ' + document.getElementById('task-delete').textContent"); got != "Could not remove it: it has a page | Remove the task" {
		t.Errorf("refused: %q", got)
	}
	// The connection lost says so too, and the next press takes the words off.
	b.eval("window.fetch = (url, init) => (init?.method === 'DELETE' ? Promise.reject(new Error('no way')) : window.realFetch(url, init))")
	pressForget()
	b.until("no way", "document.getElementById('task-delete-error').textContent.includes('no way')")
	b.eval("window.fetch = window.realFetch")
	pressForget()
	b.until("the list", "location.pathname === '/tasks' && !location.search.includes('task=')")
	if got := b.eval("location.search"); got != "?project=proj" {
		t.Errorf("after removing: %q", got)
	}
	if made, err = st.ReadTasks(); err != nil {
		t.Fatal(err)
	}
	if _, ok := made["proj"]["#104"]; ok {
		t.Error("#104 is still there")
	}

	// Written by hand: only what the store counts is a family.
	file := filepath.Join(s.Env["FLOE_DIR"], "tasks.json")
	text, err := os.ReadFile(file) //nolint:gosec // G304: a path the test names
	if err != nil {
		t.Fatal(err)
	}
	var all map[string]map[string]store.Task
	if err = json.Unmarshal(text, &all); err != nil {
		t.Fatal(err)
	}
	at := all["proj"]["#1"].At
	for name, parent := range map[string]string{
		"lost": "gone", "self": "self", "gp": "", "pa": "gp", "ch": "pa", "x": "y", "y": "x", "pagesonly": "t-pages",
	} {
		all["proj"][name] = store.Task{Title: "", State: "waiting", Step: "", At: at, URL: "", Steps: nil, Parent: parent}
	}
	all["proj"]["proto"] = store.Task{Title: "", State: "waiting", Step: "", At: at, URL: "", Steps: nil, Parent: "constructor"}
	text, err = json.Marshal(all)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(file, text, 0o600); err != nil { //nolint:gosec // G703: a path the test names
		t.Fatal(err)
	}
	page("proj", "tp", "--task", "t-pages")
	if err = os.WriteFile(file, text, 0o600); err != nil { //nolint:gosec // G703: a path the test names
		t.Fatal(err)
	}
	got := rows("waiting")
	for _, want := range []string{"ch, ", "gp (1 child task(s))", "lost, ", "pa (Parentgp), ", "pagesonly, ", "proto, ", "self, ", "x, y"} {
		if !strings.Contains(got, want) {
			t.Errorf("by hand: %q lacks %q", got, want)
		}
	}
	// What the store forgets is what the page offers to.
	open("pa")
	if got := b.eval("document.getElementById('task-delete-box').hidden + ''"); got != "false" {
		t.Errorf("a task the store forgets has no command")
	}
	open("gp")
	if got := b.eval("document.getElementById('task-delete-box').hidden + ''"); got != "true" {
		t.Errorf("a task the store keeps has its command")
	}
	s.floe(t, "work", "--project", "proj", "--task", "pa", "--forget")

	// A task of pages alone cannot be a parent, so that it has no way to make a child.
	open("t-pages")
	if got := b.eval(adds + " + document.getElementById('task-family').hidden"); got != "truetrue" {
		t.Errorf("a way from a task of no row: %s", got)
	}

	// Named from another task's page, it comes to have a row and be a child.
	open("gp")
	openChild()
	child("t-pages", "")
	b.until("saved", "!document.getElementById('child-dialog').open")
	if made, err = st.ReadTasks(); err != nil {
		t.Fatal(err)
	}
	if got := made["proj"]["t-pages"].Parent; got != "gp" {
		t.Errorf("a task of pages alone: %q", got)
	}
}

// The way back from a task's page is a press's height, wide and narrow, clear of the bar's tabs.
func TestUITaskUpRoom(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.navigate("/")
	s.floe(t, "work", "--project", "proj", "--task", "#1")
	defer b.unsize()
	for _, width := range []int64{1400, 400} {
		b.size(width, 800, width == 400)
		b.navigate("/tasks?project=proj&task=%231")
		b.until("the head", "!document.getElementById('task-head').hidden")
		const up = "document.getElementById('task-up').getBoundingClientRect()"
		got := b.eval("[" + up + ".height >= 36, " + up + ".top - document.getElementById('screens').getBoundingClientRect().bottom >= 8].join(' ')")
		if got != "true true" {
			t.Errorf("at %d: tall, clear of the tabs = %s", width, got)
		}
	}
}

// The field of a parent: given the task's, changed by the name, sent only when it differs from what it was given; what
// may be a parent is offered; a refusal keeps the dialog and the page as they were.
func TestUITaskSetParent(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	project := "proj"
	for _, slug := range []string{"a", "b", "c"} {
		s.floe(t, "publish", s.page(t, slug+".yaml", "T "+slug, ""), "--slug", slug)
		if _, err := st.Edit(slug, store.Edit{Project: &project}); err != nil {
			t.Fatal(err)
		}
	}
	s.floe(t, "work", "a", "--task", "#1", "--title", "one")
	s.floe(t, "work", "b", "--task", "#2")
	s.floe(t, "work", "c", "--task", "#3")
	s.floe(t, "work", "--project", "proj", "--task", "#3", "--parent", "#1")
	s.floe(t, "work", "--project", "proj", "--task", "#12")
	s.floe(t, "work", "--project", "proj", "--task", "#AB")
	b.size(1400, 800, false)
	defer b.unsize()
	parents := func() string {
		t.Helper()
		all, err := st.ReadTasks()
		if err != nil {
			t.Fatal(err)
		}
		return all["proj"]["#1"].Parent + "|" + all["proj"]["#2"].Parent + "|" + all["proj"]["#3"].Parent
	}
	dialog := func(slug string) {
		t.Helper()
		b.navigate("/?project=proj")
		b.until("the rows", "document.querySelector('#list li[data-slug="+slug+"]')")
		b.eval("document.querySelector('#list li[data-slug=" + slug + "] .more').open = true")
		b.eval("document.querySelector('#list li[data-slug=" + slug + "] .menu .task-set').click()")
		b.until("the dialog", "document.getElementById('set-dialog').open")
	}
	field := func(id, value string) {
		t.Helper()
		b.eval("document.getElementById(" + js(id) + ").value = " + js(value) + "; document.getElementById(" + js(id) + ").dispatchEvent(new Event('input', { bubbles: true }))")
	}
	save := func() {
		t.Helper()
		b.eval("document.getElementById('set').requestSubmit()")
		b.until("saved", "!document.getElementById('set-dialog').open")
	}
	value := func(id string) string {
		t.Helper()
		return b.eval("document.getElementById(" + js(id) + ").value")
	}

	// What the task has, and on the name changing what that one has, and of a new name nothing.
	dialog("c")
	if got := value("set-parent"); got != "#1" {
		t.Errorf("as it opens: %q", got)
	}
	field("set-task", "#2")
	if got := value("set-parent"); got != "" {
		t.Errorf("of #2: %q", got)
	}
	field("set-task", "#3")
	if got := value("set-parent"); got != "#1" {
		t.Errorf("back on #3: %q", got)
	}

	// Offered: of a row, of no parent, not itself; no new name.
	field("set-parent", "#")
	if got := b.eval("[...document.getElementById('set-parent-names').children].map((e) => e.textContent.split('·')[0].trim()).join(', ')"); got != "#1 one, #12, #2, #AB" {
		t.Errorf("offered: %q", got)
	}
	open := func(value string) string {
		t.Helper()
		field("set-parent", value)
		return b.eval("document.getElementById('set-parent-names').matches(':popover-open') + ''")
	}
	// Closed on a whole name, in any case, but one that goes on to another.
	if got := open("#1"); got != "true" {
		t.Errorf("#1, of which #12 goes on: %s", got)
	}
	if got := open("#12"); got != "false" {
		t.Errorf("#12: %s", got)
	}
	if got := open("#ab"); got != "false" {
		t.Errorf("#ab: %s", got)
	}
	field("set-parent", "#9")
	if got := b.eval("document.getElementById('set-parent-names').matches(':popover-open') + ''"); got != "false" {
		t.Errorf("offered of #9: %s", got)
	}
	b.eval("document.getElementById('set-dialog').close()")

	// Set, and refused with the dialog open and nothing written.
	dialog("b")
	field("set-parent", " #1 ")
	save()
	if got := parents(); got != "|#1|#1" {
		t.Errorf("set: %q", got)
	}
	dialog("a")
	field("set-parent", "#2")
	b.eval("document.getElementById('set').requestSubmit()")
	b.until("refused", "!document.getElementById('set-error').hidden")
	if got := b.eval("document.getElementById('set-error').textContent"); !strings.Contains(got, "task #2 has a parent itself (#1)") {
		t.Errorf("refused: %q", got)
	}
	if got := parents(); got != "|#1|#1" {
		t.Errorf("after the refusal: %q", got)
	}
	b.eval("document.getElementById('set-dialog').close()")

	// Emptied, it comes off.
	dialog("b")
	field("set-parent", "")
	save()
	if got := parents(); got != "||#1" {
		t.Errorf("emptied: %q", got)
	}

	// A field left alone sends nothing: a parent written meanwhile stays, as a parent left in it does.
	dialog("b")
	s.floe(t, "work", "--project", "proj", "--task", "#2", "--parent", "#1")
	field("set-step", "later")
	save()
	if got := parents(); got != "|#1|#1" {
		t.Errorf("written meanwhile: %q", got)
	}

	// A new name, its parent typed and emptied, is refused nothing.
	dialog("a")
	field("set-task", "#7")
	field("set-parent", "#1")
	field("set-parent", "")
	save()
	if got := st.ReadMeta("a").Work.Task; got != "#7" {
		t.Errorf("a new name: %q", got)
	}
}
