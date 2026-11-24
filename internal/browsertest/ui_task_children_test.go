package browsertest_test

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chromedp/chromedp/kb"

	"floe/internal/store"
)

// closeKin closes the child dialog by Escape, as a person does.
func closeKin(b *browser) {
	b.t.Helper()

	b.pressKey(kb.Escape)
	b.until("the child dialog to close", "!document.getElementById('kin-dialog').open")
}

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
		s.floe(t, append([]string{"task", "--slug", slug}, args...)...)
	}
	task := func(project string, args ...string) {
		t.Helper()
		s.floe(t, append([]string{"task", "--project", project, "--task"}, args...)...)
	}
	page("proj", "p1", "--task", "#1", "--state", "doing", "--title", "the parent")
	page("proj", "p1b", "--task", "#1")
	page("proj", "c2", "--task", "#2", "--state", "doing", "--title", "earlier doing child")
	page("proj", "c3", "--task", "#3", "--state", "done", "--title", "archived child")
	page("proj", "c4", "--task", "#4", "--state", "doing", "--title", "doing child")
	for _, child := range []string{"#2", "#3", "#4"} {
		task("proj", child, "--parent", "#1")
	}
	task("proj", "#3", "--state", "archived")
	// No page of its own: its state says, as of any task.
	task("proj", "#10", "--state", "archived", "--title", "gone by its child")
	page("proj", "c11", "--task", "#11", "--state", "archived")
	task("proj", "#11", "--parent", "#10")
	task("proj", "#20", "--state", "done", "--title", "kept by its child")
	page("proj", "c21", "--task", "#21", "--state", "doing")
	task("proj", "#21", "--parent", "#20")
	// Its state says, a child at work or not.
	page("proj", "p30", "--task", "#30", "--state", "archived")
	page("proj", "c31", "--task", "#31", "--state", "doing")
	task("proj", "#31", "--parent", "#30")
	// Of the same name in another project: nothing of it shows here.
	page("other", "o1", "--task", "#1", "--state", "doing")
	task("other", "#5", "--parent", "#1")

	rows := func(view string) string {
		t.Helper()
		b.navigate("/tasks?project=proj&view=" + view)
		b.until("drawn", "!document.getElementById('tasks-none').hidden || document.querySelector('#list > li [data-testid=tname]')")
		if count, shown := b.eval("document.querySelectorAll('#list > li').length + ''"), b.eval("document.querySelector('#shelf [data-testid=views] button[data-view="+js(view)+"]').textContent.match(/\\d+/)[0]"); count != shown {
			t.Errorf("%s: %s rows, %s on the shelf", view, count, shown)
		}
		return b.eval("[...document.querySelectorAll('#list > li')].map((li) => li.querySelector('[data-testid=tname]').textContent + [...li.querySelectorAll('[data-testid=kin], [data-testid=task-pages]')].filter((e) => e.matches('[data-testid=kin]') || /child/.test(e.textContent)).map((e) => ' (' + e.textContent + ')').join('')).sort().join(', ')")
	}
	for _, c := range []struct{ view, want string }{
		{"doing", "#1 (3 child task(s)), #2 (Parent#1the parent), #21 (Parent#20kept by its child), #31 (Parent#30), #4 (Parent#1the parent)"},
		{"done", "#20 (1 child task(s))"},
		{"archived", "#10 (1 child task(s)), #11 (Parent#10gone by its child), #3 (Parent#1the parent), #30 (1 child task(s))"},
	} {
		if got := rows(c.view); got != c.want {
			t.Errorf("%s = %q, want %q", c.view, got, c.want)
		}
	}

	open := func(name string) {
		t.Helper()
		b.navigate("/tasks?project=proj&task=" + strings.ReplaceAll(name, "#", "%23"))
		b.until("the head", "!document.getElementById('task-head').hidden")
	}
	const kids = "[...document.querySelectorAll('#task-family [data-testid=kin-row]')].map((e) => e.dataset.task + ' ' + e.querySelector('[data-testid=kid-state]').textContent).join(', ')"

	// The children by view, the latest changed first within one; j and k stay on the pages, and a press goes to the child's page.
	open("#1")
	if got := b.eval(kids); got != "#4 In progress, #2 In progress, #3 Archive" {
		t.Errorf("the children: %q", got)
	}
	b.eval("document.querySelector('#list a[data-testid=title]').focus()")
	b.pressKey("j")
	b.pressKey("j")
	b.pressKey("j")
	if got := b.eval("document.activeElement.closest('li')?.dataset.slug ?? document.activeElement.id"); got != "p1b" {
		t.Errorf("j stays on the pages, at %q", got)
	}

	// The focus stays on a child as the list is read again, and comes back when it is drawn anew.
	b.eval("window.kidNode = document.querySelector('#task-family [data-testid=kin-row][data-task=\"#4\"]'); window.kidNode.focus()")
	b.untilRead(b.reread(), "the list read again", "true")
	b.still()
	if got := b.eval("(document.activeElement === window.kidNode) + ''"); got != "true" {
		t.Error("a child drawn anew with nothing changed")
	}
	task("proj", "#2", "--title", "renamed")
	b.until("drawn anew", "document.querySelector('#task-family').textContent.includes('renamed')")
	if got := b.eval("(document.activeElement.dataset.task ?? '') + ' ' + (document.activeElement === window.kidNode)"); got != "#4 false" {
		t.Errorf("the focus after a change: %q", got)
	}
	b.eval("document.querySelector('#task-family [data-testid=kin-row][data-task=\"#2\"]').click()")
	b.until("the child's page", "new URLSearchParams(location.search).get('task') === '#2' && !document.getElementById('task-family').hidden")

	// A child leads up to its parent, and its way back is to its own state.
	if got := b.eval("document.querySelector('#task-family h3').textContent + ' ' + document.querySelector('#task-family [data-testid=kin-row]').textContent + ' ' + document.querySelector('#task-family [data-testid=kin-row]').getAttribute('href') + ' ' + document.getElementById('to-tasks').getAttribute('href')"); got != "Parent task In progress#1the parent /tasks?project=proj&task=%231 /tasks?project=proj" {
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
		b.until("the dialog", "document.getElementById('kin-dialog').open")
	}
	child := func(name, title string) {
		t.Helper()
		// A new task asks a title: one is given where none is, the asking itself tested apart
		if title == "" {
			title = "a title"
		}
		b.fill("#kin-task", name)
		b.fill("#kin-title", title)
		b.eval("document.getElementById('kin-submit').click()")
	}
	open("#99")
	if got := b.eval(adds); got != "false" {
		t.Errorf("no way from a task of no page: %s", got)
	}
	openChild()
	child(" #100 ", "made here")
	b.until("saved", "!document.getElementById('kin-dialog').open")
	b.until("its row", "document.querySelector('#task-family [data-testid=kin-row][data-task=\"#100\"]')")
	if got := b.eval("new URLSearchParams(location.search).get('task') + ' ' + document.querySelector('[data-testid=toast-link]').textContent + ' ' + document.querySelector('[data-testid=toast-link]').getAttribute('href')"); got != "#99 Created the child task #100 /tasks?project=proj&task=%23100" {
		t.Errorf("after making: %q", got)
	}
	made, err := st.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	if got := made["proj"]["#100"]; got.Parent != "#99" || got.Title != "made here" || got.State != "todo" {
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
	b.until("refused", "!document.getElementById('kin-error').hidden")
	if got := b.eval("document.getElementById('kin-error').textContent + ' ' + document.getElementById('kin-dialog').open"); !strings.Contains(got, "task #99 has a parent itself (#1)") || !strings.HasSuffix(got, "true") {
		t.Errorf("refused: %q", got)
	}
	if made, err = st.ReadTasks(); err != nil {
		t.Fatal(err)
	}
	if _, ok := made["proj"]["#101"]; ok {
		t.Error("the refused child has a row")
	}
	closeKin(b)
	task("proj", "#99", "--parent", "")

	// By real presses: a new name makes a child, and the focus is back on the button.
	// Into view first: a modal taller than the window scrolls, and a button cut by its edge is pressed on the backdrop,
	// which closes it. The window without b.size is low enough for that.
	press := func(id string) {
		t.Helper()
		b.eval("document.getElementById(" + js(id) + ").scrollIntoView({ block: 'nearest' })")
		b.still()
		r := b.rect("document.getElementById(" + js(id) + ")")
		b.click(r.X+r.Width/2, r.Y+r.Height/2)
	}
	write := func(name, title string) {
		t.Helper()
		press("task-child")
		b.until("the dialog", "document.getElementById('kin-dialog').open")
		b.fill("#kin-task", name)
		// The title asked of a new name alone: hidden, and not to be filled, for a task there
		if title != "" {
			b.fill("#kin-title", title)
		}
		press("kin-submit")
	}
	open("#99")
	write("#103", "made")
	b.until("saved", "!document.getElementById('kin-dialog').open")
	b.until("its row", "document.querySelector('#task-family [data-testid=kin-row][data-task=\"#103\"]')")
	if got := b.eval("document.activeElement.id"); got != "task-child" {
		t.Errorf("the focus after making one: %q", got)
	}
	// A task on the list is offered, and said before it is done: chosen by a press, it moves from its parent, keeping its
	// state and its title with none written. One with children, typed, is refused with the dialog as written.
	press("task-child")
	b.until("the dialog", "document.getElementById('kin-dialog').open")
	if got := b.eval("document.getElementById('kin-submit').textContent"); got != "Create" {
		t.Errorf("the button of no name: %q", got)
	}
	field := func(value string) {
		t.Helper()
		b.fill("#kin-task", value)
	}
	field("#")
	offered := b.eval("[...document.getElementById('kin-task-names').children].map((e) => e.querySelector('[data-testid=names-word]').textContent.split(' ')[0]).join(' ')")
	for _, name := range []string{" #1 ", " #10 ", " #103 ", " #99 "} {
		if strings.Contains(" "+offered+" ", name) {
			t.Errorf("offered %s: %q", name, offered)
		}
	}
	if !strings.Contains(" "+offered+" ", " #4 ") {
		t.Errorf("not offered #4: %q", offered)
	}
	if got := b.eval("[...document.getElementById('kin-task-names').children].find((e) => e.textContent.startsWith('#4')).textContent"); !strings.Contains(got, "· In progress · parent #1") {
		t.Errorf("what is said of #4: %q", got)
	}
	// A name typed whole closes the names, which would cover what is said; no title goes with a task that has one.
	field("#4")
	const said = "[document.getElementById('kin-task-names').matches(':popover-open'), document.getElementById('kin-note').hidden ? '' : document.getElementById('kin-note').textContent, document.getElementById('kin-submit').textContent, document.getElementById('kin-title').closest('label').hidden].join(' | ')"
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
	if got := b.eval(said); got != "false |  | Create | false" {
		t.Errorf("a new name: %q", got)
	}
	// A title written for it, hidden as a task there is chosen, is not sent.
	b.fill("#kin-title", "not sent")
	// Chosen by a press, as typed.
	field("#")
	r := b.rect("[...document.getElementById('kin-task-names').children].find((e) => e.textContent.startsWith('#4'))")
	b.click(r.X+r.Width/2, r.Y+r.Height/2)
	if got := b.eval(said); got != "false | #4 doing child becomes a child, moving from its parent #1 | Make it a child | true" {
		t.Errorf("#4 chosen: %q", got)
	}
	press("kin-submit")
	b.until("saved", "!document.getElementById('kin-dialog').open")
	// Any of the toasts: the one of #103 made before may still show, before this one
	b.until("the toast", "[...document.querySelectorAll('[data-testid=toast-link]')].some((e) => e.textContent === 'Made #4 a child task')")
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
	b.until("closed", "!document.getElementById('kin-dialog').open")
	if got := b.eval("(document.querySelector('[data-testid=toast-link]')?.textContent ?? '') + '|' + document.activeElement.id"); got != "|task-child" {
		t.Errorf("a child here already: %q", got)
	}
	press("task-child")
	b.until("the dialog", "document.getElementById('kin-dialog').open")
	field("#4")
	if got := b.eval(said); got != "false |  | Make it a child | true" {
		t.Errorf("a child here already, typed: %q", got)
	}
	closeKin(b)
	open("#99")
	write("#20", "")
	b.until("refused", "!document.getElementById('kin-error').hidden")
	want := "Failed to save: task #20 has child tasks, such as #21, so it cannot have a parent: tasks have one level of children | #20 |  | true"
	if got := b.eval("[document.getElementById('kin-error').textContent, document.getElementById('kin-task').value, document.getElementById('kin-title').value, document.getElementById('kin-dialog').open].join(' | ')"); got != want {
		t.Errorf("a task with children: %q", got)
	}
	closeKin(b)
	task("proj", "#103", "--forget")

	// A new child asks a title: of spaces it is stopped, the spaces kept; opened again, nothing of that is left.
	press("task-child")
	b.until("the dialog", "document.getElementById('kin-dialog').open")
	field("#105")
	b.fill("#kin-title", "   ")
	press("kin-submit")
	const titleState = "(document.getElementById('kin-title').validity.customError ? document.getElementById('kin-title').validationMessage : '') + '|' + document.getElementById('kin-title').value + '|' + document.getElementById('kin-dialog').open"
	if got := b.eval(titleState); got != "Write more than spaces|   |true" {
		t.Errorf("a title of spaces: %q", got)
	}
	closeKin(b)
	press("task-child")
	b.until("the dialog", "document.getElementById('kin-dialog').open")
	if got := b.eval(titleState); got != "||true" {
		t.Errorf("opened again: %q", got)
	}
	// Made meanwhile under another parent: refused as stale, read again and said so; pressed again, it is taken.
	field("#106")
	b.fill("#kin-title", "mine")
	task("proj", "#106", "--parent", "#1")
	press("kin-submit")
	b.until("refused", "!document.getElementById('kin-error').hidden")
	if got := b.eval("document.getElementById('kin-error').textContent + '|' + document.getElementById('kin-note').textContent + '|' + document.getElementById('kin-title').closest('label').hidden"); got != "Failed to save: task #106 is there already; the tasks are read again, so check and save again|#106 becomes a child, moving from its parent #1|true" {
		t.Errorf("made meanwhile: %q", got)
	}
	press("kin-submit")
	b.until("saved", "!document.getElementById('kin-dialog').open")
	if made, err = st.ReadTasks(); err != nil {
		t.Fatal(err)
	}
	if got := made["proj"]["#106"]; got.Parent != "#99" || got.Title != "" {
		t.Errorf("#106 taken: %+v", got)
	}
	task("proj", "#106", "--parent", "")
	task("proj", "#106", "--forget")
	// Shown there, then gone and read before the press: sent as there, as shown, and refused, not made of no title.
	task("proj", "#107", "--title", "theirs")
	b.untilRead(b.reread(), "#107 read", "true")
	press("task-child")
	b.until("the dialog", "document.getElementById('kin-dialog').open")
	field("#")
	b.until("#107 offered", "[...document.getElementById('kin-task-names').children].some((e) => e.textContent.startsWith('#107'))")
	field("#107")
	task("proj", "#107", "--forget")
	b.untilRead(b.reread(), "#107 gone", "true")
	press("kin-submit")
	b.until("refused", "!document.getElementById('kin-error').hidden")
	if got := b.eval("document.getElementById('kin-error').textContent + '|' + document.getElementById('kin-title').closest('label').hidden"); got != "Failed to save: task #107 is no longer there; the tasks are read again, so check and save again|false" {
		t.Errorf("gone meanwhile: %q", got)
	}
	closeKin(b)

	// Removed by two presses and back at the list; refused, it says why and stays. A task with children has no button.
	pressForget := func() {
		t.Helper()
		press("task-delete")
		b.until("armed", "document.getElementById('task-delete').dataset.armed === 'true'")
		press("task-delete")
	}
	task("proj", "#104", "--title", "to remove", "--state", "doing")
	open("#104")
	if got := b.eval("((e) => { e.style.color = 'var(--color-danger)'; document.body.append(e); const want = getComputedStyle(e).color; e.remove(); return want !== '' && getComputedStyle(document.getElementById('task-delete')).color === want; })(document.createElement('i'))"); got != "true" {
		t.Error("the button is not of a delete's color")
	}
	b.eval("window.realFetch = window.fetch; window.fetch = (url, init) => (init?.method === 'DELETE' ? Promise.resolve(new Response('it has a page', { status: 400 })) : window.realFetch(url, init))")
	pressForget()
	b.until("refused", "!document.getElementById('task-delete-error').hidden")
	if got := b.eval("document.getElementById('task-delete-error').textContent + ' | ' + document.getElementById('task-delete').textContent"); got != "Failed to delete the task: it has a page | Delete the task" {
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
		all["proj"][name] = store.Task{Title: "", State: "done", At: at, URLs: nil, Categories: nil, Parent: parent, Rev: 0}
	}
	all["proj"]["proto"] = store.Task{Title: "", State: "done", At: at, URLs: nil, Categories: nil, Parent: "constructor", Rev: 0}
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
	got := rows("done")
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
	s.floe(t, "task", "--project", "proj", "--task", "pa", "--forget")

	// A task a page names with no row, as a hand edit leaves, is not there: its page says so.
	b.navigate("/tasks?project=proj&task=t-pages")
	b.until("not there", "!document.getElementById('task-missing').hidden")

	// Named from another task's page, it is new to the screen: made of the title, and a child.
	open("gp")
	openChild()
	child("t-pages", "")
	b.until("saved", "!document.getElementById('kin-dialog').open")
	if made, err = st.ReadTasks(); err != nil {
		t.Fatal(err)
	}
	if got := made["proj"]["t-pages"]; got.Parent != "gp" || got.Title != "a title" {
		t.Errorf("a page's task of no row named a child: %+v", got)
	}
}

// A task's head keeps its way back, title and Edit in one row, wide and narrow: the mark and Edit in the middle of the title however
// many lines it takes, the title's lines all right of the mark, and nothing past the window, a title with no space in it too. The way
// back is a mark alone, of an icon button's room, named; the archive stands by the state. 280px is narrower than a phone, as the tests'
// fonts are narrower than a person's.
func TestUITaskHeadRoom(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.navigate("/")
	tasks := []struct{ name, title, lines string }{
		{"#1", "short", "one"},
		{"#2", strings.TrimSpace(strings.Repeat("a title of many words ", 6)), "more"},
		{"#3", strings.Repeat("unbroken", 12), "more"},
	}
	for _, task := range tasks {
		s.floe(t, "task", "--project", "proj", "--task", task.name, "--title", task.title, "--state", "done")
	}
	defer b.unsize()
	for _, width := range []int64{1400, 400, 280} {
		b.size(width, 800, width != 1400)
		for _, task := range tasks {
			b.navigate("/tasks?project=proj&task=" + url.QueryEscape(task.name))
			b.until("the head", "!document.getElementById('task-head').hidden && !document.getElementById('task-archive').hidden")
			got := b.eval(`(() => {
  const at = (id) => document.getElementById(id), r = (id) => at(id).getBoundingClientRect(), mid = (id) => (r(id).top + r(id).bottom) / 2;
  const up = r('task-up'), title = r('task-title'), body = at('list-body'), said = [];
  if (Math.abs(mid('task-up') - mid('task-title')) > 0.5 || Math.abs(mid('task-edit') - mid('task-title')) > 0.5) said.push('off the middle of the title');
  if (title.left < up.right || title.right > r('task-edit').left) said.push('the title over the mark or Edit');
  if (at('task-title').scrollWidth > at('task-title').clientWidth) said.push('the title past its room');
  if (up.width !== 30 || up.height !== 30 || at('task-up').textContent !== '') said.push('the mark of another form');
  if (at('task-up').getAttribute('aria-label') !== 'Back to the task list' || at('task-up').title !== 'Back to the task list') said.push('the mark named otherwise');
  if (up.top - document.querySelector('[data-testid=bar]').getBoundingClientRect().bottom < 8) said.push('the mark by the bar');
  if (!['task-up', 'task-edit', 'task-archive', 'task-project', 'task-add-page'].every((id) => r(id).left >= 0 && r(id).right <= innerWidth)) said.push('past the window');
  if (document.documentElement.scrollWidth > innerWidth || body.scrollWidth > body.clientWidth) said.push('scrolls sideways');
  if (r('task-archive').top < Math.max(title.bottom, r('task-edit').bottom)) said.push('the archive off the state');
  const lines = Math.round(title.height / parseFloat(getComputedStyle(at('task-title')).lineHeight));
  return [lines > 1 ? 'more' : 'one', ...said].join(', ');
})()`)
			if got != task.lines {
				t.Errorf("%s at %d: %s, want %s", task.name, width, got, task.lines)
			}
		}
	}
}

// The set dialog's fields and a new task's title: asked of a new name alone, of more than spaces; stale either way, refused and read again;
// unreadable, nothing is sent. It has no field of the task's parent or URL, which its page writes.
func TestUITaskSetTitle(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
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
	s.floe(t, "task", "--slug", "a", "--task", "#1", "--title", "one")
	s.floe(t, "task", "--slug", "b", "--task", "#2")
	s.floe(t, "task", "--slug", "c", "--task", "#3")
	s.floe(t, "task", "--project", "proj", "--task", "#3", "--parent", "#1")
	s.floe(t, "task", "--project", "proj", "--task", "#12", "--state", "todo")
	s.floe(t, "task", "--project", "proj", "--task", "#AB", "--state", "todo")
	b.size(1400, 800, false)
	defer b.unsize()
	dialog := func(slug string) {
		t.Helper()
		b.navigate("/?project=proj")
		b.until("the rows", "document.querySelector('#list li[data-slug="+slug+"]')")
		pressMenuItem(b, "#list li[data-slug="+slug+"]", "task-set")
		b.until("the dialog", "document.getElementById('set-dialog').open")
	}
	field := func(id, value string) {
		t.Helper()
		b.fill("#"+id, value)
	}
	save := func() {
		t.Helper()
		// A new task asks a title: one is given where none is typed, the asking itself tested apart
		titleOpen(b)
		submitSet(b)
		b.until("saved", "!document.getElementById('set-dialog').open")
	}

	// The fields' words, and the example the catalog gives.
	dialog("a")
	if got := b.eval("[document.getElementById('set-task').placeholder, document.getElementById('set-task').closest('label').textContent.trim(), document.getElementById('set-task-title').closest('label').textContent.trim(), document.getElementById('set-category').placeholder, document.querySelectorAll('#set-dialog input').length].join('|')"); got != "e.g. #128|Task ID|Title|e.g. proposals, designs, samples|3" {
		t.Errorf("the fields: %q", got)
	}
	closeSet(b)

	// A new name is made, its title given.
	dialog("a")
	field("set-task", "#7")
	save()
	if got := st.ReadMeta("a").Task.Name; got != "#7" {
		t.Errorf("a new name: %q", got)
	}

	// A new task's title of spaces is stopped, kept; a name there hides the field and its mark, and saves.
	shown := "(document.getElementById('set-task-title').closest('[hidden]') === null)"
	dialog("b")
	field("set-task", "#77")
	field("set-task-title", "   ")
	b.eval("document.getElementById('set-submit').click()") // stopped: neither saved nor asked
	if got := b.eval("document.getElementById('set-task-title').validationMessage + '|' + " + shown); got != "Write more than spaces|true" {
		t.Errorf("a title of spaces: %q", got)
	}
	field("set-task", "#1")
	if got := b.eval("document.getElementById('set-task-title').validity.customError + '|' + " + shown); got != "false|false" {
		t.Errorf("a name there: %q", got)
	}
	save()
	if got := st.ReadMeta("b").Task.Name; got != "#1" {
		t.Errorf("saved on #1: %q", got)
	}
	// Opened again, the title typed before is gone, and a task there asks none.
	dialog("b")
	field("set-task", "#88")
	field("set-task-title", "typed")
	closeSet(b)
	dialog("c")
	if got := b.eval("document.getElementById('set-task-title').value + '|' + " + shown); got != "|false" {
		t.Errorf("opened again: %q", got)
	}
	closeSet(b)

	// Stale either way: refused, read again, and saved as it is now at the second press.
	stale := func(slug, name, made, gone string, then func()) {
		t.Helper()
		dialog(slug)
		field("set-task", name)
		if made != "" {
			field("set-task-title", made)
		}
		then()
		submitSet(b)
		b.until("refused", "!document.getElementById('set-error').hidden")
		if got := b.eval("document.getElementById('set-error').textContent"); !strings.Contains(got, gone) {
			t.Errorf("%s: %q", name, got)
		}
		save()
	}
	stale("c", "#90", "mine", "task #90 is there already; the tasks are read again", func() {
		s.floe(t, "task", "--project", project, "--task", "#90", "--title", "theirs")
	})
	s.floe(t, "task", "--project", project, "--task", "#91", "--state", "todo")
	b.untilRead(b.reread(), "#91 read", "true")
	stale("c", "#91", "", "task #91 is no longer there; the tasks are read again", func() {
		s.floe(t, "task", "--project", project, "--task", "#91", "--forget")
	})
	all, err := st.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	if all["proj"]["#90"].Title != "theirs" || all["proj"]["#91"].Title != "a title" {
		t.Errorf("after the stale: %+v %+v", all["proj"]["#90"], all["proj"]["#91"])
	}
	// Shown new, then made and read before the press: sent as new, as shown, and refused, not put on theirs unseen.
	// Refused, the title is not asked of it any more.
	dialog("c")
	field("set-task", "#93")
	field("set-task-title", "mine")
	s.floe(t, "task", "--project", project, "--task", "#93", "--title", "theirs", "--url", "https://example.com/theirs", "--parent", "#1")
	b.untilRead(b.reread(), "#93 read", "true")
	submitSet(b)
	b.until("refused", "!document.getElementById('set-error').hidden")
	if got := b.eval("document.getElementById('set-error').textContent.includes('task #93 is there already') + '|' + " + shown); got != "true|false" {
		t.Errorf("made meanwhile: %q", got)
	}
	closeSet(b)

	// A page whole reads no tasks as its list is read: refused, it reads them itself.
	b.navigate("/a/a/")
	b.until("the bar", "document.querySelector('#frame-actions [data-testid=task-set]')")
	pressMenuItem(b, "#frame-actions", "task-set")
	b.until("the dialog", "document.getElementById('set-dialog').open")
	field("set-task", "#92")
	field("set-task-title", "mine")
	s.floe(t, "task", "--project", project, "--task", "#92", "--state", "todo")
	submitSet(b)
	b.until("refused", "!document.getElementById('set-error').hidden")
	save()

	// Unreadable, nothing is asked or sent: it says so.
	file := filepath.Join(s.Env["FLOE_DIR"], "tasks.json")
	good, err := os.ReadFile(file) //nolint:gosec // G304: a path the test names
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(file, []byte("{ broken"), 0o600); err != nil { //nolint:gosec // G703: a path the test names
		t.Fatal(err)
	}
	dialog("c")
	field("set-task", "#93")
	b.eval("document.getElementById('set-submit').click()") // nothing sent: the error the opening said stays
	if got := b.eval("document.getElementById('set-dialog').open + '|' + !document.getElementById('set-error').hidden + '|' + document.getElementById('set-task-title').required"); got != "true|true|false" {
		t.Errorf("unreadable: %q", got)
	}
	closeSet(b)
	if err = os.WriteFile(file, good, 0o600); err != nil { //nolint:gosec // G703: a path the test names
		t.Fatal(err)
	}
}
