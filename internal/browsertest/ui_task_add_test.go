package browsertest_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/store"
)

// A task from the task list: its button where the add button of the page list is and not where a task made would not show, the dialog's words
// for a new ID and one there, the project asked with no task anywhere, and the list gone to the new row after.
func TestUITaskAdd(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	tasksFile := filepath.Join(s.Env["FLOE_DIR"], "tasks.json")
	rows := func() map[string]map[string]json.RawMessage {
		t.Helper()
		var got map[string]map[string]json.RawMessage
		data, err := os.ReadFile(tasksFile)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	const (
		seen   = "(id) => document.getElementById(id).getClientRects().length > 0"
		dialog = "document.getElementById('kin-dialog')"
		said   = "['kin-heading', 'kin-target', 'kin-note', 'kin-error'].map((id) => { const e = document.getElementById(id); return e.hidden ? '-' : e.textContent; }).join(' | ') + ' | title ' + !document.getElementById('kin-title').closest('[hidden]') + ' | ' + document.getElementById('kin-submit').textContent"
		list   = "location.search + ' ' + document.querySelector('#list li.fresh .tname')?.textContent + ' ' + (document.activeElement === document.querySelector('#list li.fresh .title')) + ' ' + [...document.querySelectorAll('#toasts .toast-link')].at(-1)?.textContent + ' ' + [...document.querySelectorAll('#toasts .toast-link')].at(-1)?.getAttribute('href')"
	)
	want := func(what, expr, wanted string) {
		t.Helper()
		if got := b.eval(expr); got != wanted {
			t.Errorf("%s:\n got %s\nwant %s", what, got, wanted)
		}
	}
	shown := func(id string) string { return "(" + seen + ")(" + js(id) + ") + ''" }
	open := func(button string) {
		t.Helper()
		b.until(button, shown(button)+" === 'true'")
		b.eval("document.getElementById(" + js(button) + ").click()")
		b.until("the dialog", dialog+".open")
	}
	name := func(id, title string) {
		t.Helper()
		b.eval("document.getElementById('kin-task').value = " + js(id) + "; document.getElementById('kin-task').dispatchEvent(new Event('input', { bubbles: true })); document.getElementById('kin-title').value = " + js(title))
	}
	saved := func() {
		t.Helper()
		b.eval("document.getElementById('kin').requestSubmit()")
		b.until("saved", "!"+dialog+".open")
	}
	press := func(selector string) {
		t.Helper()
		b.eval("document.querySelector(" + js(selector) + ").click()")
	}

	// No page, no task: the switch is there all the same, and leads to the words and the button.
	b.navigate("/")
	b.until("the list read", "!document.getElementById('empty').hidden")
	want("switch with nothing", shown("to-tasks"), "true")
	press("#to-tasks")
	b.until("the words", "!document.getElementById('tasks-empty').hidden")
	want("the words", "document.querySelector('#tasks-empty strong').textContent + ' | ' + document.querySelector('#tasks-empty p').textContent",
		"No tasks yet | Make one with Add task.")
	want("no command on the screens", "document.querySelectorAll('.state code').length + ''", "0")
	want("the head's button with no task", shown("add-task"), "false")
	want("the icon, the size of the others", "document.querySelector('#task-add-empty svg').getBoundingClientRect().width + ''", "13")

	// The screen of no task has the whole width, wide and narrow, the shelf folded or not, its words at the middle.
	for _, c := range []struct {
		width  int64
		mobile bool
		kept   string
	}{{1440, false, ""}, {1440, false, `{"open":false}`}, {400, true, ""}} {
		b.size(c.width, 800, c.mobile)
		b.eval("localStorage.setItem('list.columns', " + js(c.kept) + ")")
		b.navigate("/tasks")
		b.until("the words", "!document.getElementById('tasks-empty').hidden")
		b.still()
		want("the words at the middle", "(() => { const r = document.getElementById('tasks-empty').getBoundingClientRect(); return (Math.abs(r.width - innerWidth) < 2) + ' ' + (Math.abs(r.left + r.width / 2 - innerWidth / 2) < 2); })()", "true true")
	}
	b.unsize()
	b.eval("localStorage.removeItem('list.columns')")
	b.navigate("/tasks")
	b.until("the words", "!document.getElementById('tasks-empty').hidden")

	// With no project on a page, the one choice is none, and what comes of it is in no project.
	open("task-add-empty")
	want("no task, no page: the project field", "[...document.getElementById('kin-project').options].map((o) => o.value + '=' + o.textContent).join(', ')", "/=Not set")
	want("no task: the words", said, "Add task | - | - | - | title true | Make")
	name("#0", "first")
	saved()
	b.until("the row", "document.querySelector('#list li.fresh')")
	want("no task: after", list, `?view=todo&project=%2F #0 true Task #0 made /tasks?project=%2F&task=%230`)
	if _, ok := rows()["/"]["#0"]; !ok {
		t.Errorf("the row of no project: %v", rows())
	}
	s.floe(t, "work", "--task", "#0", "--project", "", "--forget")

	// With no task but pages, their projects are offered, the one with the most first; chosen, the list goes to it from the one in the URL.
	for slug, project := range map[string]string{"a1": "alpha", "b1": "beta", "b2": "beta", "n1": ""} {
		s.floe(t, "publish", s.page(t, slug+".yaml", "T "+slug, ""), "--slug", slug)
		if _, err := st.Edit(slug, store.Edit{Project: &project}); err != nil {
			t.Fatal(err)
		}
	}
	b.navigate("/tasks?project=alpha")
	b.until("the words", "!document.getElementById('tasks-empty').hidden")
	open("task-add-empty")
	want("the projects, in projectOrder", "[...document.getElementById('kin-project').options].map((o) => o.value).join(' ')", "/ beta alpha")
	want("the project of the URL first chosen", "document.getElementById('kin-project').value", "alpha")
	b.eval(dialog + ".close()")
	for _, path := range []string{"/tasks", "/tasks?project=typo"} {
		b.navigate(path)
		open("task-add-empty")
		want(path+": none chosen", "document.getElementById('kin-project').value", "/")
		b.eval(dialog + ".close()")
	}
	b.navigate("/tasks?project=alpha")
	open("task-add-empty")
	b.eval("const f = document.getElementById('kin-project'); f.value = 'beta'; f.dispatchEvent(new Event('change', { bubbles: true }))")
	name("#1", "one")
	saved()
	b.until("the row", "document.querySelector('#list li.fresh')")
	want("a project chosen: after", list, `?view=todo&project=beta #1 true Task #1 made /tasks?project=beta&task=%231`)
	if _, ok := rows()["beta"]["#1"]; !ok {
		t.Errorf("the row in the project chosen: %v", rows())
	}

	// The button under the projects of the shelf, wide; narrow, in the filter dialog, which closes for its dialog, the focus then
	// going back to the filter's button.
	b.size(1440, 800, false)
	b.navigate("/tasks?project=beta&view=doing")
	b.until("the button", shown("add-task")+" === 'true'")
	want("under the projects", "(document.getElementById('shelf-projects').nextElementSibling === document.getElementById('add-task').parentElement) + ''", "true")
	b.size(400, 800, true)
	b.navigate("/tasks?project=beta&view=doing")
	b.until("the filter's button", shown("narrow-filter")+" === 'true'")
	want("narrow, out of sight", shown("add-task"), "false")
	press("#narrow-filter")
	open("add-task")
	want("the filter closed", "document.getElementById('filter-dialog').open + ''", "false")
	want("the bar's switch fits", "document.documentElement.scrollWidth <= innerWidth ? 'true' : 'false'", "true")
	b.eval(dialog + ".close()")
	want("the focus back", "document.activeElement.id", "narrow-filter")
	b.unsize()
	b.navigate("/tasks?project=beta&view=doing")
	open("add-task")
	want("the dialog", said, "Add task | Project: beta | - | - | title true | Make")
	want("the project field hidden", shown("kin-project"), "false")
	name("#2", "two")

	// Made from another state's list: the list goes to the state it is in, the row marked until the rows are drawn anew.
	saved()
	b.until("the row", "document.querySelector('#list li.fresh')")
	want("from another state: after", list, `?view=todo&project=beta #2 true Task #2 made /tasks?project=beta&task=%232`)
	want("the old row is not marked", "document.querySelectorAll('#list li.fresh').length + ''", "1")
	press(".views button[data-view=doing]")
	b.until("another state", "!document.querySelector('#list li.fresh')")

	// An ID there: said, with no title asked, and pressed it opens the task, nothing written.
	before, err := os.ReadFile(tasksFile)
	if err != nil {
		t.Fatal(err)
	}
	open("add-task")
	name("#", "")
	want("nothing suggested", "document.querySelectorAll('.suggest:popover-open').length + ''", "0")
	name("#1", "")
	want("an ID there", said, "Add task | Project: beta | #1 one is already there (To do) | - | title false | Open the task")
	b.eval("document.getElementById('kin').requestSubmit()")
	b.until("the task's page", "location.search.includes('task=%231')")
	b.until("the head", "!document.getElementById('task-none').hidden || !document.getElementById('task-head').hidden")
	if after, err := os.ReadFile(tasksFile); err != nil || string(after) != string(before) {
		t.Errorf("opening wrote: %s (%v)", after, err)
	}
	want("not on a task's page", shown("add-task"), "false")

	// One of pages alone, archived: there all the same, and said so.
	s.floe(t, "work", "b2", "--task", "#20", "--title", "old", "--state", "archived")
	s.floe(t, "archive", "b2")
	b.navigate("/tasks?project=beta")
	open("add-task")
	name("#20", "")
	want("an archived ID", "document.getElementById('kin-note').textContent", "#20 old is already there (Archive)")
	b.eval(dialog + ".close()")

	// The same ID in another project is another task: the store's rows say so.
	s.floe(t, "work", "--project", "alpha", "--task", "#7")
	b.navigate("/tasks?project=alpha")
	open("add-task")
	name("#1", "alpha's one")
	want("the same ID in another project", said, "Add task | Project: alpha | - | - | title true | Make")
	saved()
	b.until("the row", "document.querySelector('#list li.fresh')")
	if _, ok := rows()["alpha"]["#1"]; !ok {
		t.Errorf("the row in alpha: %v", rows())
	}

	// A screen gone stale: made meanwhile, refused, read again, and the dialog stays with what is there now.
	open("add-task")
	name("#9", "mine")
	s.floe(t, "work", "--project", "alpha", "--task", "#9", "--title", "theirs")
	b.eval("document.getElementById('kin').requestSubmit()")
	b.until("refused", "!document.getElementById('kin-error').hidden")
	got := b.eval(said)
	if !strings.Contains(got, "#9 theirs is already there (To do)") || !strings.Contains(got, "Open the task") || !strings.Contains(got, "title false") {
		t.Errorf("stale: %s", got)
	}
	want("stale: the dialog stays", dialog+".open + ''", "true")
	name("#10", "")
	want("another ID: the refusal gone", "document.getElementById('kin-error').hidden + ''", "true")
	b.eval(dialog + ".close()")

	// Drawn once, with the new task: never the words of a list with none, from the tasks before.
	s.floe(t, "work", "--project", "gamma", "--task", "#40", "--state", "waiting")
	b.navigate("/tasks?project=gamma&view=waiting")
	open("add-task")
	b.eval("window.noneShown = false; new MutationObserver(() => { window.noneShown ||= !document.getElementById('tasks-none').hidden; }).observe(document.getElementById('tasks-none'), { attributes: true })")
	name("#41", "new")
	saved()
	b.until("the row", "document.querySelector('#list li.fresh')")
	want("drawn once", "window.noneShown + ''", "false")

	// A poll asked while the write's answer is on its way waits for it: drawn after, the list still goes to the new row.
	const holdWrite = "window.realFetch ??= window.fetch; window.fetch = (u, o) => (o?.method === 'PATCH' ? window.realFetch(u, o).then((r) => new Promise((go) => { window.letWrite = () => go(r); })) : window.realFetch(u, o))"
	s.floe(t, "work", "--project", "delta", "--task", "#50", "--state", "waiting")
	b.navigate("/tasks?project=delta&view=waiting")
	open("add-task")
	b.eval(holdWrite)
	name("#51", "polled")
	b.eval("document.getElementById('kin').requestSubmit()")
	b.until("the write", "window.letWrite")
	b.eval("window.polled = false; load(true).then(() => { window.polled = true; }); true")
	// Long past a read of the list: nothing drawn while the write goes
	want("a poll waits", "new Promise((done) => setTimeout(done, 300)).then(() => window.polled + ' ' + (document.querySelector('.views button[data-view=todo] .n')?.textContent === '1'))", "false false")
	b.eval("window.letWrite()")
	b.until("saved", "!"+dialog+".open && window.polled")
	want("a poll first: after", list, `?view=todo&project=delta #51 true Task #51 made /tasks?project=delta&task=%2351`)

	// A poll asked before the write and answered after it is of the tasks before: it draws neither the words of a list with none nor over the row.
	// The save's load waits for it, so it is let go once the write is answered.
	const holdReads = "window.realFetch ??= window.fetch; window.held = []; window.fetch = (u, o) => (o?.method ? window.realFetch(u, o) : window.realFetch(u, o).then((r) => new Promise((go) => { window.held.push(() => go(r)); })))"
	s.floe(t, "work", "--project", "epsilon", "--task", "#60", "--state", "waiting")
	b.navigate("/tasks?project=epsilon&view=waiting")
	open("add-task")
	b.eval("window.noneShown = false; new MutationObserver(() => { window.noneShown ||= !document.getElementById('tasks-none').hidden; }).observe(document.getElementById('tasks-none'), { attributes: true })")
	b.eval(holdReads)
	b.until("a poll held", "window.held.length >= 2")
	// The reads after go on; the save's answer is marked
	b.eval("window.patched = false; window.fetch = (u, o) => (o?.method === 'PATCH' ? window.realFetch(u, o).then((r) => { window.patched = true; return r; }) : window.realFetch(u, o))")
	name("#61", "late")
	b.eval("document.getElementById('kin').requestSubmit()")
	b.until("the write answered", "window.patched")
	b.eval("window.held.forEach((go) => go()); true")
	b.until("saved", "!"+dialog+".open")
	b.until("the row", "document.querySelector('#list li.fresh')")
	want("a poll late: the row and its mark stay", list, `?view=todo&project=epsilon #61 true Task #61 made /tasks?project=epsilon&task=%2361`)
	want("a poll late: never none", "window.noneShown + ''", "false")

	// A project that is on the list by a pin alone has its button; one not on the list has none: the name may be a mistake.
	project := "pinned"
	s.floe(t, "publish", s.page(t, "p1.yaml", "T p1", ""), "--slug", "p1")
	if _, err := st.Edit("p1", store.Edit{Project: &project}); err != nil {
		t.Fatal(err)
	}
	s.floe(t, "work", "p1", "--pin")
	b.navigate("/tasks?project=pinned")
	b.until("the words", "!document.getElementById('tasks-none').hidden")
	want("a project of a pin alone", shown("add-task"), "true")
	b.navigate("/tasks?project=typo")
	b.until("the words", "!document.getElementById('tasks-none').hidden")
	want("a project not there", shown("add-task"), "false")

	// Not where a task made would not show, nor on the page list.
	b.navigate("/tasks?project=alpha&view=archived")
	b.until("the archived list", "document.querySelector('.views button[data-view=archived]').getAttribute('aria-pressed') === 'true'")
	want("on the archived list", shown("add-task"), "false")
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug]')")
	want("on the page list", shown("add-task"), "false")
	want("the page list's own", shown("add-link"), "true")
	// Narrow, in the filter dialog, which closes for the add dialog, the focus then going back to the filter's button.
	b.size(400, 800, true)
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug]')")
	press("#narrow-filter")
	b.until("the button", shown("add-link")+" === 'true'")
	press("#add-link")
	b.until("the add dialog", "document.getElementById('add-dialog').open")
	want("the page list: the filter closed", "document.getElementById('filter-dialog').open + ''", "false")
	b.eval("document.getElementById('add-dialog').close()")
	want("the page list: the focus back", "document.activeElement.id", "narrow-filter")
	b.unsize()
	b.navigate("/tasks?project=alpha")
	b.until("the button", shown("add-task")+" === 'true'")
	want("the task list has not the page list's", shown("add-link"), "false")
	if err := os.WriteFile(tasksFile, []byte("{ broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	b.navigate("/tasks?project=alpha")
	b.until("the failure", "!document.getElementById('tasks-failed').hidden")
	want("unreadable", shown("add-task"), "false")
	want("unreadable: the switch stays", shown("to-list"), "true")
}
