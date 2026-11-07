package browsertest_test

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/store"
)

// taskPageRows says the rows of a task's page: a step's heading in brackets, a page by its title.
const taskPageRows = `[...document.querySelectorAll('#list > li')].map((li) => li.classList.contains('group') ? '[' + li.textContent + ']' : li.querySelector('.title').textContent).join(' | ')`

// taskPageAt says where the focus and the page chosen are.
const taskPageAt = `(document.activeElement.closest?.('li')?.querySelector('.title')?.textContent ?? 'none') + ' / ' + (picked?.slug ?? 'none')`

// A task's page: nothing read as it opens, its pages under their steps in the order written, j and k through them, a page
// whole back to where it came from; the state changed and what that does to the list and the focus on the way back; what is
// done archived and given back; and the pages the list cannot show.
func TestUITaskPage(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	b.navigate("/")
	page := func(slug string, args ...string) {
		t.Helper()
		s.floe(t, "publish", s.page(t, slug+".yaml", "T "+slug, ""), "--slug", slug)
		project := "proj"
		if _, err := st.Edit(slug, store.Edit{Project: &project}); err != nil {
			t.Fatal(err)
		}
		s.floe(t, append([]string{"work", slug}, args...)...)
	}
	page("c1", "--task", "#128", "--state", "waiting")
	page("n1", "--task", "#130", "--state", "waiting")
	page("d1", "--task", "#130", "--step", "design")
	page("r1", "--task", "#130", "--step", "review")
	page("i1", "--task", "#130", "--step", "build")
	page("y1", "--task", "#142", "--state", "waiting")
	page("x1", "--task", "#120", "--state", "done")
	page("p1", "--pin")
	page("p2", "--pin")
	page("loose", "--pin")
	s.floe(t, "work", "loose", "--unpin")
	s.write(t, "fold/one.html", "<title>one</title><p>one</p>")
	s.floe(t, "publish", "--link", filepath.Join(s.Env["HOME"], "fold"), "--slug", "fold")
	if _, err := st.Edit("fold", store.Edit{Project: ptrTo("proj"), Work: &store.WorkEdit{Task: ptrTo("#1")}}); err != nil {
		t.Fatal(err)
	}
	s.floe(t, "work", "--task", "#1", "--project", "proj", "--state", "done")
	s.floe(t, "work", "--task", "#130", "--project", "proj", "--steps", "design,build,review,nothing")
	s.floe(t, "work", "--task", "#99", "--project", "proj")

	var out []string
	say := func(what, expr string) {
		t.Helper()
		out = append(out, what+": "+b.eval(expr))
	}
	key := func(k string) {
		t.Helper()
		b.moving("document.body.dispatchEvent(new KeyboardEvent('keydown', { key: " + js(k) + ", bubbles: true }))")
	}
	enter := func() {
		t.Helper()
		b.moving("document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))")
	}
	const whole = "document.getElementById('back').getAttribute('href') + ' ' + document.querySelector('.brand').getAttribute('href') + ' ' + sessionStorage.getItem('artifacts.order')"
	opened := func(task, path string) {
		t.Helper()
		b.navigate(path)
		b.until(task+"'s rows", taskPageShown)
	}

	b.size(1400, 800, false)
	defer b.unsize()
	b.navigate("/tasks?project=proj")
	b.until("the rows", "document.querySelector('#list > li')")
	unread := b.eval("document.getElementById('news-n').textContent")
	key("j")
	key("j")
	say("list j j", "document.activeElement.querySelector('.tname').textContent")
	enter()
	b.until("#130's page", "location.search === '?project=proj&task=%23130' && "+taskPageShown)

	// Open, nothing is, and nothing is read, as the whole URL stays.
	say("rows", taskPageRows)
	say("nothing open", taskPageAt)
	say("url", "location.pathname + location.search")
	say("the list's keys", "sessionStorage.getItem('artifacts.list') + ' ' + sessionStorage.getItem('artifacts.tasks')")
	say("head", "[document.getElementById('task-title').textContent, document.getElementById('task-state').value, document.getElementById('task-state').className, document.getElementById('task-project').textContent, document.getElementById('task-archive').hidden, document.getElementById('task-up').getAttribute('href')].join(' / ')")
	say("nothing to select", "document.querySelectorAll('#list .sel').length + ' ' + [...document.querySelectorAll('#list .menu .select')].every((e) => e.hidden)")
	b.still()
	if got := b.eval("document.getElementById('news-n').textContent"); got != unread {
		t.Errorf("opening the task read a page: %s, was %s", got, unread)
	}

	// j and k through its rows, in the order of its steps.
	moved := make([]string, 0, 4)
	for range 4 {
		key("j")
		moved = append(moved, b.eval(taskPageAt))
	}
	out = append(out, "j: "+strings.Join(moved, ", "))

	// Whole: back to the task, by the rows' order.
	enter()
	b.until("the page whole", "location.pathname === '/a/r1/' && document.getElementById('frame-title').textContent")
	say("whole", whole)
	key("k")
	b.until("the page before", "location.pathname === '/a/i1/' && document.getElementById('frame-title').textContent")
	say("k", whole)
	key("h")
	b.until("the task's page again", "location.search === '?project=proj&task=%23130' && "+taskPageShown)

	// The state changed: back to the list of that state, the task's row with the focus.
	b.eval("document.getElementById('task-state').focus(); document.getElementById('task-state').value = 'doing'; document.getElementById('task-state').dispatchEvent(new Event('change'))")
	b.until("the state", "document.getElementById('task-state').className.includes('doing')")
	// By the keyboard, whose keys a select keeps: the focus has left it.
	realKey(b, "h", 0)
	b.until("the list", "location.pathname === '/tasks' && document.querySelector('#list > li') && document.activeElement.closest('#list')")
	say("after the state changed", "location.search + ' ' + [...document.querySelectorAll('#list .tname')].map((e) => e.textContent).join(' | ') + ' focus ' + document.activeElement.querySelector('.tname').textContent")
	// A task of the same state, its row has the focus.
	b.navigate("/tasks?project=proj")
	b.until("the rows", "document.querySelector('#list > li')")
	b.eval("[...document.querySelectorAll('#list .title')].at(-1).focus()")
	enter()
	b.until("#142's page", "location.search === '?project=proj&task=%23142' && "+taskPageShown)
	key("h")
	b.until("the list", "location.pathname === '/tasks' && document.activeElement.closest('#list')")
	say("the row come back to", "document.activeElement.querySelector('.tname').textContent")

	// A pin: through the pins, back to the list with its filter.
	b.navigate("/tasks?view=doing&project=proj")
	b.until("the rows", "document.querySelector('#list > li')")
	say("the list kept", "sessionStorage.getItem('artifacts.tasks')")
	b.moving("document.querySelector('.pin').click()")
	b.until("the pin whole", "location.pathname.startsWith('/a/p') && document.getElementById('frame-title').textContent")
	say("pin", "location.pathname + ' ' + "+whole)
	key("h")
	b.until("the filtered list", "location.pathname + location.search === '/tasks?view=doing&project=proj'")
	b.until("the rows", "document.querySelector('#list > li')")
	b.moving("[...document.querySelectorAll('#list .title')].find((a) => a.textContent.startsWith('#130')).click()")
	b.until("the task's page", "location.search.includes('task=') && "+taskPageShown)
	say("up goes to the task's state", "document.getElementById('task-up').getAttribute('href')")

	// In a folder's tree on the task's page; a page named whole there; a page of no rows.
	opened("#1", "/tasks?project=proj&task=%231")
	b.eval("document.querySelector('#list li[data-slug=fold] .title').click()")
	b.until("the tree", "document.querySelector('#list li.tree .title')")
	b.eval("document.querySelector('#list li.tree .title').focus()")
	enter()
	b.until("the folder's page", "location.pathname === '/a/fold/' && location.search === '?file=one.html' && document.getElementById('frame-title').textContent")
	say("in the folder", whole)
	opened("#128", "/tasks?project=proj&task=%23128")
	b.eval("document.querySelector('#list .title').focus()")
	enter()
	b.until("the page whole", "location.pathname === '/a/c1/' && document.getElementById('frame-title').textContent")
	b.moving("openEdit(items.find((a) => a.slug === 'c1')); document.getElementById('edit-slug').value = 'c2'; document.getElementById('edit').requestSubmit()")
	b.until("the new name", "location.pathname === '/a/c2/' && document.getElementById('frame-title').textContent")
	say("renamed whole", whole)
	b.eval("sessionStorage.removeItem('artifacts.order')")
	b.navigate("/a/loose/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	say("of no rows", whole)

	// Done: its pages archived and given back, as the shelf's archive sees it.
	opened("#120", "/tasks?project=proj&task=%23120")
	const archive = "(() => { const e = document.getElementById('task-archive'); return e.hidden ? 'none' : e.textContent; })()"
	say("done", archive)
	b.eval("document.getElementById('task-archive').click()")
	b.until("archived", "document.getElementById('task-archive').textContent === 'Bring the archive back'")
	say("archived", taskPageRows)
	b.navigate("/tasks?view=archived")
	b.until("the archive", "document.querySelector('#list > li')")
	say("the shelf's archive", "[...document.querySelectorAll('#list .tname')].map((e) => e.textContent).join(' | ')")
	opened("#120", "/tasks?project=proj&task=%23120")
	b.eval("document.getElementById('task-archive').click()")
	b.until("given back", "document.getElementById('task-archive').textContent === 'Archive the task'")

	// A task's page with the list folded on the page list: the rows until a page is chosen.
	b.eval("localStorage.setItem('list.columns', JSON.stringify({ listOpen: false }))")
	opened("#128", "/tasks?project=proj&task=%23128")
	say("folded, none chosen", "(document.getElementById('listcol').offsetParent !== null) + ' ' + document.getElementById('list-hide').hidden")
	b.eval("document.querySelector('#list .title').click()")
	b.until("the page chosen", "picked && document.getElementById('listcol').offsetParent === null")
	b.eval("localStorage.removeItem('list.columns')")

	// A state changed while the choices are open is drawn once they close.
	b.navigate("/tasks?project=proj&task=%23142")
	b.until("#142's page", taskPageShown)
	state := b.rect("document.getElementById('task-state')")
	b.click(state.X+state.Width/2, state.Y+state.Height/2)
	b.until("the choices", "document.getElementById('task-state').matches(':open')")
	s.floe(t, "work", "--task", "#142", "--project", "proj", "--state", "done")
	b.until("a look at the tasks", "tasksRead.raw.includes('done') && tasksRead.tasks.proj['#142'].state === 'done'")
	say("while open", "document.getElementById('task-state').value")
	b.pressKey("\x1b")
	b.until("the choices closed", "!document.getElementById('task-state').matches(':open')")
	b.until("the state drawn", "document.getElementById('task-state').value === 'done'")
	s.floe(t, "work", "--task", "#142", "--project", "proj", "--state", "waiting")

	// A task of no pages is removed by two presses, its name and project whatever they hold.
	s.floe(t, "work", "--task", "it's $(touch made)", "--project", "")
	b.navigate("/tasks?project=%2F&task=" + url.QueryEscape("it's $(touch made)"))
	b.until("no pages", "!document.getElementById('task-none').hidden")
	for range 2 {
		b.still()
		r := b.rect("document.getElementById('task-delete')")
		b.click(r.X+r.Width/2, r.Y+r.Height/2)
	}
	b.until("the list", "!location.search.includes('task=')")
	if all, readErr := st.ReadTasks(); readErr != nil || len(all["/"]) != 0 {
		t.Errorf("removing: %v, %v", all["/"], readErr)
	}

	// A tag on a task's page has nothing to narrow.
	if _, err := st.Edit("y1", store.Edit{Tags: &[]string{"t"}}); err != nil {
		t.Fatal(err)
	}
	opened("#142", "/tasks?project=proj&task=%23142")
	say("a tag", "document.querySelector('#list .tagpill').tagName + ' ' + document.querySelector('#list .tagpill').textContent")

	// What is not there, and what cannot be written.
	opened("#128", "/tasks?project=proj&task=%23128")
	say("waiting has no archive button", archive)
	b.navigate("/tasks?project=proj&task=%2399")
	b.until("no pages", "!document.getElementById('task-none').hidden")
	say("no pages", "document.querySelector('#task-none strong').textContent + ' ' + document.getElementById('task-delete').textContent + ' ' + document.getElementById('task-archive').hidden")
	// Unreadable, what was drawn stays: it does not say there are no pages, or no such task. A page opened anew says why alone.
	file := filepath.Join(s.Env["FLOE_DIR"], "tasks.json")
	mended, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(file, []byte("{ broken"), 0o600); err != nil { //nolint:gosec // G703: a path the test names
		t.Fatal(err)
	}
	b.until("the error", "!document.getElementById('error').hidden")
	say("unreadable, drawn", "document.getElementById('task-head').hidden + ' ' + document.getElementById('task-none').hidden + ' ' + document.getElementById('task-missing').hidden")
	b.navigate("/tasks?project=proj&task=%2399")
	b.until("the error", "!document.getElementById('error').hidden")
	say("unreadable, first", "document.getElementById('task-head').hidden + ' ' + document.getElementById('task-none').hidden + ' ' + document.getElementById('task-missing').hidden")
	// Of a task done, whose pages are known still: its archive can be pressed.
	if err = os.WriteFile(file, mended, 0o600); err != nil { //nolint:gosec // G703: a path the test names
		t.Fatal(err)
	}
	opened("#120", "/tasks?project=proj&task=%23120")
	if err = os.WriteFile(file, []byte("{ broken"), 0o600); err != nil { //nolint:gosec // G703: a path the test names
		t.Fatal(err)
	}
	b.until("the error", "!document.getElementById('error').hidden")
	say("unreadable, done", archive)
	if err = os.WriteFile(file, mended, 0o600); err != nil { //nolint:gosec // G703: a path the test names
		t.Fatal(err)
	}
	b.navigate("/tasks?project=proj&task=%23999")
	b.until("no task", "!document.getElementById('task-missing').hidden")
	say("no task", "document.querySelector('#task-missing strong').textContent + ' ' + document.getElementById('task-head').hidden + ' ' + document.querySelectorAll('#list > li').length")
	opened("#128", "/tasks?project=proj&task=%23128")
	if err := os.WriteFile(filepath.Join(s.Env["FLOE_DIR"], "tasks.json"), []byte("{ broken"), 0o600); err != nil { //nolint:gosec // G703: a path the test names
		t.Fatal(err)
	}
	b.eval("document.getElementById('task-state').value = 'done'; document.getElementById('task-state').dispatchEvent(new Event('change'))")
	b.until("the refusal", "!document.getElementById('error').hidden")
	// The pages are told still, so a done task's archive can be pressed while its state cannot be read.
	say("refused", "document.getElementById('task-state').value + ' ' + document.getElementById('task-state').hidden + ' ' + document.getElementById('error').textContent.includes('mend it by hand')")

	// The widths: the column beside the page; the column alone. No shelf in any.
	for _, c := range []struct {
		width  int64
		beside bool
	}{{1400, true}, {960, true}, {880, false}} {
		b.size(c.width, 800, false)
		beside := b.eval("document.getElementById('reader').offsetParent !== null") == "true"
		shelf := b.eval("document.getElementById('shelf').offsetParent !== null") == "true"
		over := b.eval("document.documentElement.scrollWidth > innerWidth") == "true"
		if beside != c.beside || shelf || over {
			t.Errorf("width %d: beside %t, shelf %t, past the edge %t", c.width, beside, shelf, over)
		}
	}
	if got, want := strings.Join(out, "\n"), taskPageWant; got != want {
		t.Errorf("the task's page:\n%s\nwant:\n%s", got, want)
	}
}

// taskPageShown tells a task's page drew its rows.
const taskPageShown = "!document.getElementById('task-head').hidden && document.querySelector('#list > li')"

func ptrTo[T any](v T) *T {
	return &v
}

// taskPageWant is what TestUITaskPage reads, in the order it reads it.
const taskPageWant = `list j j: #130
rows: T n1 | [design] | T d1 | [build] | T i1 | [review] | T r1
nothing open: none / none
url: /tasks?project=proj&task=%23130
the list's keys:  ?project=proj
head: #130 / waiting / ver-pick st waiting / proj / true / /tasks?project=proj
nothing to select: 0 true
j: T n1 / n1, T d1 / d1, T i1 / i1, T r1 / r1
whole: /tasks?project=proj&task=%23130 / {"order":["n1","d1","i1","r1"],"shown":["n1","d1","i1","r1"],"back":"/tasks?project=proj&task=%23130","at":{"slug":"r1","file":"","v":0}}
k: /tasks?project=proj&task=%23130 / {"order":["n1","d1","i1","r1"],"shown":["n1","d1","i1","r1"],"back":"/tasks?project=proj&task=%23130","at":{"slug":"i1","file":"","v":0}}
after the state changed: ?view=doing&project=proj #130 focus #130
the row come back to: #142
the list kept: ?view=doing&project=proj
pin: /a/p2/ /tasks?view=doing&project=proj / {"order":["p2","p1"],"shown":["p2","p1"],"back":"/tasks?view=doing&project=proj","at":{"slug":"p2","file":"","v":0}}
up goes to the task's state: /tasks?project=proj&view=doing
in the folder: /tasks?project=proj&task=%231 / {"order":[],"shown":["fold"],"back":"/tasks?project=proj&task=%231","at":{"slug":"fold","file":"one.html","v":0}}
renamed whole: /tasks?project=proj&task=%23128 / {"order":["c2"],"shown":["c2"],"back":"/tasks?project=proj&task=%23128","at":{"slug":"c2","file":"","v":0}}
of no rows: / / null
done: Archive the task
archived: [Archive] | T x1
the shelf's archive: #120
folded, none chosen: true true
while open: waiting
a tag: SPAN t
waiting has no archive button: none
no pages: This task has no pages Remove the task true
unreadable, drawn: false false true
unreadable, first: true true true
unreadable, done: Archive the task
no task: No task #999 true 0
refused: waiting false true`

// The way up of a task's page is its project's list at the state the task is in now, whatever way it was come to; a state changed on
// the page takes it along, and the row the list opens with the focus on is the task's. The tab's list is left as it was.
func TestUITaskUp(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
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
	s.floe(t, "work", "a", "b", "--task", "#1", "--state", "doing")
	s.floe(t, "work", "c", "--task", "#2", "--state", "done")
	b.size(1400, 800, false)
	defer b.unsize()

	var out []string
	const up = "document.getElementById('task-up').getAttribute('href')"
	say := func(what string) {
		t.Helper()
		out = append(out, what+": "+b.eval(up))
	}
	press := func(expr string) {
		t.Helper()
		r := b.rect(expr)
		b.click(r.X+r.Width/2, r.Y+r.Height/2)
	}
	const task1 = "location.search === '?project=proj&task=%231' && " + taskPageShown
	const task2 = "location.search === '?project=proj&task=%232' && " + taskPageShown

	// By a row of the list, at that state: through a reload, j and k, and a page whole and back.
	b.navigate("/tasks?view=doing&project=proj")
	b.until("the rows", "document.querySelector('#list > li .title')")
	press("document.querySelector('#list > li .title')")
	b.until("the task's page", task1)
	say("a row")
	b.navigate("/tasks?project=proj&task=%231")
	b.until("reloaded", task1)
	say("reloaded")
	b.pressKey("j")
	b.until("the page beside", "picked?.slug")
	b.pressKey("j")
	b.until("the next", "picked?.slug === 'b'")
	say("j j")
	b.moving("document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))")
	b.until("the page whole", "location.pathname === '/a/b/' && document.getElementById('frame-title').textContent")
	out = append(out, "the page whole: "+b.eval("document.getElementById('back').getAttribute('href')"))
	realKey(b, "h", 0)
	b.until("the task's page", task1)
	say("back from it")

	// The state changed there: to the list it has come to be on, its row with the focus; the tab's list stays what was last seen.
	b.navigate("/tasks?view=doing&project=proj")
	b.until("the rows", "document.querySelector('#list > li .title')")
	press("document.querySelector('#list > li .title')")
	b.until("the task's page", task1)
	b.eval("document.getElementById('task-state').value = 'done'; document.getElementById('task-state').dispatchEvent(new Event('change'))")
	b.until("the state", "document.getElementById('task-state').className.includes('done')")
	say("the state changed")
	out = append(out, "the tab's list: "+b.eval("document.getElementById('to-tasks').getAttribute('href')"))
	realKey(b, "h", 0)
	b.until("the list", "location.search === '?project=proj&view=done' || location.search === '?view=done&project=proj'")
	b.until("the focus", "document.activeElement.closest('#list')")
	out = append(out, "on it: "+b.eval("location.search + ' ' + document.activeElement.querySelector('.tname').textContent"))
	// Changed elsewhere, as it is read.
	s.floe(t, "work", "--task", "#1", "--project", "proj", "--state", "doing")
	b.navigate("/tasks?project=proj&task=%231")
	b.until("the task's page", task1)
	say("changed elsewhere")
	s.floe(t, "work", "--task", "#1", "--project", "proj", "--state", "waiting")
	b.until("read again", up+" === '/tasks?project=proj'")
	say("changed elsewhere, while open")

	// By the URL, of a task of the other states.
	b.navigate("/tasks?project=proj&task=%232")
	b.until("the task's page", task2)
	say("the URL, of a task done")

	// By a badge on the page list, and by the way in a page's menu.
	s.floe(t, "work", "--task", "#1", "--project", "proj", "--state", "doing")
	b.navigate("/?project=proj")
	b.until("the page list", "document.querySelector('#list li[data-slug=a] .task-tag')")
	press("document.querySelector('#list li[data-slug=a] .task-tag')")
	b.until("the task's page", task1)
	say("a badge")
	press("document.querySelector('#task-up')")
	b.until("the list", "location.pathname === '/tasks' && document.activeElement.closest('#list')")
	out = append(out, "up from there: "+b.eval("location.search + ' ' + document.activeElement.querySelector('.tname').textContent + ' ' + document.getElementById('to-tasks').getAttribute('href')"))
	b.navigate("/tasks?view=done&project=proj")
	b.until("the rows", "document.querySelector('#list > li')")
	out = append(out, "no focus opened otherwise: "+b.eval("String(document.activeElement.closest('#list') !== null)"))
	b.navigate("/?project=proj")
	b.until("the rows", "document.querySelector('#list li[data-slug=c]')")
	b.eval("document.querySelector('#list li[data-slug=c] .more').open = true")
	press("document.querySelector('#list li[data-slug=c] .task-open')")
	b.until("the task's page", task2)
	say("the menu")

	if got, want := strings.Join(out, "\n"), taskUpWant; got != want {
		t.Errorf("the way up:\n%s\nwant:\n%s", got, want)
	}
}

//nolint:gosec // G101: where the way up goes, not a secret
const taskUpWant = `a row: /tasks?project=proj&view=doing
reloaded: /tasks?project=proj&view=doing
j j: /tasks?project=proj&view=doing
the page whole: /tasks?project=proj&task=%231
back from it: /tasks?project=proj&view=doing
the state changed: /tasks?project=proj&view=done
the tab's list: /tasks?view=doing&project=proj
on it: ?view=done&project=proj #1
changed elsewhere: /tasks?project=proj&view=doing
changed elsewhere, while open: /tasks?project=proj
the URL, of a task done: /tasks?project=proj&view=done
a badge: /tasks?project=proj&view=doing
up from there: ?view=doing&project=proj #1 /tasks?view=doing&project=proj
no focus opened otherwise: false
the menu: /tasks?project=proj&view=done`
