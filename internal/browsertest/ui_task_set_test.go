package browsertest_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chromedp/chromedp/kb"

	"github.com/sukekyo26/floe/internal/store"
)

// A page's menu pins it and sets its task and step, in the order floe work writes; the task's own fields are its page's (TestUITaskEdit), so
// the dialog has none of its parent or URL; a name of no task is a new one; what is refused keeps the dialog open; off a task, the pin stays.
// The page list's rows lead to the task and the task list.
func TestUITaskSet(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
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
	s.floe(t, "work", "b", "--task", "#1", "--step", "design", "--url", "https://example.com/a/one")
	s.floe(t, "work", "c", "--task", "#2", "--step", "build")
	b.size(1400, 800, false)
	defer b.unsize()

	var out []string
	say := func(what, expr string) {
		t.Helper()
		out = append(out, what+": "+b.eval(expr))
	}
	work := func(slug string) string {
		t.Helper()
		got, err := json.Marshal(st.ReadMeta(slug).Work)
		if err != nil {
			t.Fatal(err)
		}
		return string(got)
	}
	tasks := func() string {
		t.Helper()
		all, err := st.ReadTasks()
		if err != nil {
			t.Fatal(err)
		}
		names := make([]string, 0, 4)
		for _, name := range []string{"#1", "#2", "#3"} {
			if row, ok := all["proj"][name]; ok {
				names = append(names, name+" "+row.State+" "+row.Step+" "+row.URL)
			}
		}
		return strings.Join(names, "; ")
	}
	const items = `[...document.querySelectorAll('#list li[data-slug=SLUG] .menu > *')].filter((e) => !e.hidden).map((e) => e.className.replace('icon-btn ', '')).join(' ')`
	menu := func(slug, item string) {
		t.Helper()
		b.eval("document.querySelector('#list li[data-slug=" + slug + "] .more').open = true")
		b.eval("document.querySelector('#list li[data-slug=" + slug + "] .menu ." + item + "').click()")
	}
	dialog := func(slug string) {
		t.Helper()
		menu(slug, "task-set")
		b.until("the dialog", "document.getElementById('set-dialog').open")
	}
	field := func(id, value string) {
		t.Helper()
		b.eval("document.getElementById(" + js(id) + ").value = " + js(value) + "; document.getElementById(" + js(id) + ").dispatchEvent(new Event('input', { bubbles: true }))")
	}
	const fields = "['set-task', 'set-step'].map((id) => document.getElementById(id).value).join(' / ') + ' / out ' + !document.getElementById('set-out').hidden"
	save := func() {
		t.Helper()
		// A new task asks a title: one is given where none is typed, the asking itself tested apart
		b.eval("const ti = document.getElementById('set-task-title'); if (!ti.closest('[hidden]') && !ti.value) ti.value = 'a title'")
		b.eval("document.getElementById('set').requestSubmit()")
		b.until("saved", "!document.getElementById('set-dialog').open")
	}

	// The rows' ways to the task list, and the items of the menu.
	b.navigate("/?project=proj")
	b.until("the rows", "document.querySelector('#list li[data-slug=b] .task-tag')")
	say("tags", "[...document.querySelectorAll('.task-tag')].map((e) => e.textContent + ' ' + e.getAttribute('href')).join(', ')")
	say("items of a page on a task", strings.ReplaceAll(items, "SLUG", "b"))
	say("items of a page on none", strings.ReplaceAll(items, "SLUG", "a"))
	say("open the task", "document.querySelector('#list li[data-slug=b] .task-open').textContent + ' ' + document.querySelector('#list li[data-slug=b] .task-open').getAttribute('href')")

	// Its items look alike, the pin's too.
	b.eval("document.querySelector('#list li[data-slug=b] .more').open = true")
	say("the items' looks", "[...new Set(['pin-item', 'task-set', 'edit'].map((name) => { const c = getComputedStyle(document.querySelector('#list li[data-slug=b] .menu .' + name)); return [c.borderTopWidth, c.borderTopColor, c.backgroundColor, c.paddingLeft, c.height, c.borderRadius].join(' '); }))].length")
	b.eval("document.querySelector('#list li[data-slug=b] .more').open = false")

	// A pin on and off, from the menu.
	menu("a", "pin-item")
	b.until("pinned", "items.find((x) => x.slug === 'a').work?.pin")
	b.until("drawn", "document.querySelector('#list li[data-slug=a] .task-tag')")
	say("pinned", "document.querySelector('#list li[data-slug=a] .task-tag').textContent + ' ' + document.querySelector('#list li[data-slug=a] .task-tag').getAttribute('href') + ' ' + document.querySelector('#list li[data-slug=a] .pin-item').textContent")
	out = append(out, "pinned: "+work("a"))

	// A name of no task: new, doing; the suggestions name what there is, and say it is new. The dialog has no field of a task's parent or URL.
	dialog("a")
	say("a page of none", fields)
	say("no parent nor URL", "String(document.querySelector('#set-dialog #set-parent, #set-dialog #set-url'))")
	field("set-task", "#3")
	b.until("the suggestions", "document.getElementById('set-task-names').matches(':popover-open') || (document.getElementById('set-task').focus(), false)")
	say("offered", "[...document.getElementById('set-task-names').children].map((e) => e.textContent).join(', ')")
	field("set-step", "first")
	save()
	out = append(out, "new task: "+work("a"), tasks())

	// A task written since the dialog opened: refused, and saved then, as the dialog shows it read again.
	dialog("b")
	say("a page of a task", fields)
	s.floe(t, "work", "--task", "#1", "--project", "proj", "--url", "https://example.com/later")
	b.eval("document.getElementById('set').requestSubmit()")
	b.until("refused", "!document.getElementById('set-error').hidden")
	say("written meanwhile", "document.getElementById('set-error').textContent")
	save()
	out = append(out, "written meanwhile, saved: "+tasks())

	// What is refused keeps the dialog open, says why; nothing of the page changes.
	was := work("b")
	dialog("b")
	b.eval(`window.realFetch = window.fetch; window.fetch = (url, init) => (String(url).includes('/api/tasks') && init?.method === 'PATCH' ? Promise.resolve(new Response('no', { status: 400 })) : window.realFetch(url, init))`)
	b.eval("document.getElementById('set').requestSubmit()")
	b.until("the refusal", "!document.getElementById('set-error').hidden")
	b.eval("window.fetch = window.realFetch")
	say("refused", "document.getElementById('set-dialog').open + ' ' + document.getElementById('set-error').textContent")
	if got := work("b"); got != was {
		t.Errorf("a refusal changed the page: %s", got)
	}
	b.eval("document.getElementById('set-dialog').close()")

	// A name left empty, or of spaces, saves nothing: taking a page off is the button's. No toast comes of it.
	dialog("b")
	b.eval("document.querySelectorAll('.toast').forEach((e) => e.remove())")
	field("set-task", "")
	b.eval("document.getElementById('set').requestSubmit()")
	say("an empty name", "document.getElementById('set-dialog').open + ' ' + document.getElementById('set-task').validity.valueMissing + ' ' + document.querySelectorAll('.toast').length")
	// Of spaces, the dialog stops it, the spaces kept, and sends nothing
	field("set-task", "   ")
	field("set-task-title", "a title")
	b.eval("document.getElementById('set').requestSubmit()")
	say("a name of spaces", "document.getElementById('set-dialog').open + ' ' + document.getElementById('set-task').validationMessage + ' ' + document.getElementById('set-task').value.length + ' ' + document.querySelectorAll('.toast').length")
	if got := work("b"); got != was {
		t.Errorf("an empty name changed the page: %s", got)
	}
	b.eval("document.getElementById('set-dialog').close()")

	// Saved in order: the page's write refused, the task's is there, and saving again finishes it.
	dialog("c")
	b.eval(`window.realFetch = window.fetch; window.fetch = (url, init) => (String(url).includes('/api/artifacts/c') && init?.method === 'PATCH' ? Promise.resolve(new Response('no', { status: 500 })) : window.realFetch(url, init))`)
	field("set-task", "#2")
	field("set-step", "review")
	b.eval("document.getElementById('set').requestSubmit()")
	b.until("the page refused", "!document.getElementById('set-error').hidden")
	out = append(out, "the page refused: "+tasks()+" | "+work("c"))
	b.eval("window.fetch = window.realFetch")
	save()
	out = append(out, "saved again: "+work("c"))

	// Off the task: the page's name and step go, its pin and the task's row stay.
	dialog("a")
	b.eval("document.getElementById('set-out').click()")
	b.until("off the task", "!document.getElementById('set-dialog').open")
	out = append(out, "off: "+work("a"), tasks())
	dialog("c")
	b.eval("document.getElementById('set-out').click()")
	b.until("off the task", "!document.getElementById('set-dialog').open")
	out = append(out, "off, no pin: "+work("c"))

	// As it opens, no suggestions over the fields; a suggestion pressed, as a person does, goes in and closes; a tag's too.
	dialog("b")
	say("as it opens", "document.getElementById('set-task-names').matches(':popover-open')")
	s.floe(t, "work", "--task", "#1", "--project", "proj", "--url", "https://example.com/a/one")
	s.floe(t, "work", "--task", "#2", "--project", "proj", "--url", "https://example.com/a/two")
	b.eval("document.getElementById('set-dialog').close()")
	b.navigate("/?project=proj")
	b.until("the rows", "document.querySelector('#list li[data-slug=b]')")
	dialog("b")
	input := b.rect("document.getElementById('set-task')")
	b.click(input.X+10, input.Y+input.Height/2)
	b.until("the suggestions", "document.getElementById('set-task-names').matches(':popover-open')")
	option := b.rect("[...document.getElementById('set-task-names').children].find((e) => e.querySelector('.names-word').textContent === '#2')")
	b.click(option.X+10, option.Y+option.Height/2)
	b.until("it closed", "!document.getElementById('set-task-names').matches(':popover-open')")
	say("a suggestion pressed", fields)
	say("the field keeps the focus", "document.activeElement.id")
	b.still()
	say("it stays closed", "document.getElementById('set-task-names').matches(':popover-open')")
	// What is offered holds what is written.
	field("set-task", "#9")
	say("offered of #9", "[...document.getElementById('set-task-names').children].map((e) => e.textContent).join(', ')")
	field("set-task", "#")
	say("offered of #", "[...document.getElementById('set-task-names').children].map((e) => e.textContent).join(', ')")
	// The suggestions are a listbox the field controls as a combobox, under the field: ↓ reaches one, which the field names, and
	// Escape closes them, not the dialog.
	say("roles", "(() => { const f = document.getElementById('set-task'), n = document.getElementById('set-task-names'); "+
		"return [f.getAttribute('role'), f.getAttribute('aria-autocomplete'), f.getAttribute('aria-controls') === n.id, n.getAttribute('role')].join(' '); })()")
	b.eval("document.getElementById('set-task').focus()")
	out = append(out, "under the field: "+b.until("the suggestions under the field", "document.getElementById('set-task-names').matches(':popover-open') && (() => { "+
		"const f = document.getElementById('set-task').getBoundingClientRect(), r = document.getElementById('set-task-names').getBoundingClientRect(); "+
		"return Math.abs(r.top - f.bottom - 4) < 0.5 && Math.abs(r.left - f.left) < 0.5 && 'yes'; })()"))
	b.pressKey(kb.ArrowDown)
	say("reached", "(() => { const o = document.querySelector('#set-task-names [aria-selected=true]'); "+
		"return document.getElementById('set-task').getAttribute('aria-activedescendant') === o?.id && o.querySelector('.names-word').textContent; })()")
	b.pressKey(kb.Escape)
	say("escape", "document.getElementById('set-task-names').matches(':popover-open') + ' ' + document.getElementById('set-dialog').open")
	// The list read again leaves the suggestions open, the one reached as it is.
	field("set-task", "#")
	b.until("the suggestions", "document.getElementById('set-task-names').matches(':popover-open')")
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.ArrowDown)
	b.eval("window.reread = false; drawn = ''; load().then(() => { window.reread = true; })")
	b.until("the list read again", "window.reread")
	say("read again", "document.getElementById('set-task-names').matches(':popover-open') + ' ' + "+
		"document.querySelector('#set-task-names [aria-selected=true] .names-word')?.textContent")
	b.pressKey(kb.Escape)
	// Opened as its dialog opens, the suggestions go under the field once the dialog stops moving.
	b.eval("document.getElementById('set-dialog').close()")
	dialog("b")
	b.eval("document.getElementById('set-task').focus()")
	field("set-task", "#")
	b.still()
	out = append(out, "under the field once still: "+b.until("the suggestions under the field once still", "document.getElementById('set-task-names').matches(':popover-open') && (() => { "+
		"const f = document.getElementById('set-task').getBoundingClientRect(), r = document.getElementById('set-task-names').getBoundingClientRect(); "+
		"return Math.abs(r.top - f.bottom - 4) < 0.5 && Math.abs(r.left - f.left) < 0.5 && 'yes'; })()"))
	b.pressKey(kb.Escape)
	field("set-task", "#1")
	save()
	out = append(out, "saved: "+tasks())

	// Saved with nothing of the step changed, the task's last step stays: the page's own is older.
	s.floe(t, "work", "c", "--task", "#1", "--step", "later")
	// Read before the dialog opens, which writes the task as read
	b.until("read", "tasksRead.tasks.proj['#1'].step === 'later'")
	dialog("b")
	save()
	out = append(out, "nothing changed: "+tasks()+" | "+work("b"))
	// While saving, what is typed next goes nowhere.
	dialog("b")
	b.eval(`window.realFetch = window.fetch; window.fetch = async (url, init) => { const res = await window.realFetch(url, init); if (String(url).includes('/api/tasks') && init?.method === 'PATCH') document.getElementById('set-task').value = '#2'; return res; }`)
	b.eval("document.getElementById('set').requestSubmit()")
	b.until("saved", "!document.getElementById('set-dialog').open")
	b.eval("window.fetch = window.realFetch")
	out = append(out, "typed while saving: "+work("b"))

	// An archived page has no pin to give: the shelf of pins leaves it out.
	s.floe(t, "archive", "a")
	b.navigate("/?view=archived&project=proj")
	b.until("the archive", "document.querySelector('#list li[data-slug=a]')")
	say("an archived page", "document.querySelector('#list li[data-slug=a] .pin-item').hidden + ' ' + document.querySelectorAll('#list li[data-slug=a] .task-tag').length")
	s.floe(t, "archive", "--undo", "a")
	if got := work("a"); got != `{"pin":true}` {
		t.Errorf("archived and back: %s", got)
	}

	// A pin says it was done, in the bar's menu, where no badge shows it; refused, it says why.
	b.navigate("/?project=proj")
	b.until("the rows", "document.querySelector('#list li[data-slug=b]')")
	menu("b", "pin-item")
	b.until("the toast", "document.querySelector('.toast')")
	say("pinned, told", "document.querySelector('.toast').textContent")
	b.eval(`window.realFetch = window.fetch; window.fetch = (url, init) => (init?.method === 'PATCH' ? Promise.resolve(new Response('no', { status: 500 })) : window.realFetch(url, init))`)
	menu("b", "pin-item")
	b.until("the refusal", "!document.getElementById('error').hidden")
	say("refused, told", "document.getElementById('error').textContent")
	b.eval("window.fetch = window.realFetch; showError('')")
	s.floe(t, "work", "b", "--unpin")

	// From a page whole, which reads the tasks as it opens: a task written since is that task as written.
	s.floe(t, "work", "b", "--task", "#1", "--step", "whole")
	b.navigate("/a/b/")
	b.until("the bar", "document.querySelector('#frame-actions .task-set')")
	b.eval("document.querySelector('#frame-actions .more').open = true; document.querySelector('#frame-actions .task-set').click()")
	b.until("the dialog", "document.getElementById('set-dialog').open")
	say("whole", fields)
	b.eval("document.getElementById('set-dialog').close()")

	// On the task's own page the way to it is not offered.
	b.navigate("/tasks?project=proj&task=%231")
	b.until("the row", "document.querySelector('#list li[data-slug=b]')")
	say("on its own task", "document.querySelector('#list li[data-slug=b] .task-open').hidden + ' ' + document.querySelectorAll('#list .task-tag').length")

	// The suggestions say what a task is in, cut to their box; put on a task, the toast says it too, and leads there.
	s.floe(t, "work", "--task", "#5", "--project", "proj", "--title", strings.Repeat("A very long title ", 6), "--state", "done")
	b.navigate("/?project=proj")
	b.until("the rows", "document.querySelector('#list li[data-slug=c]')")
	for _, width := range []int64{1400, 400} {
		b.size(width, 800, false)
		b.navigate("/?project=proj")
		b.until("the rows", "document.querySelector('#list li[data-slug=c]')")
		dialog("c")
		box := b.rect("document.getElementById('set-task')")
		b.click(box.X+10, box.Y+box.Height/2)
		b.until("the suggestions", "document.getElementById('set-task-names').matches(':popover-open')")
		b.still()
		within := b.eval("[...document.querySelectorAll('#set-task-names .names-note')].every((e) => e.getBoundingClientRect().right <= document.getElementById('set-task-names').getBoundingClientRect().right + 0.5 && e.scrollWidth <= e.clientWidth) && document.getElementById('set-task-names').getBoundingClientRect().right <= innerWidth")
		if within != "true" {
			t.Errorf("width %d: a suggestion's state is cut", width)
		}
		out = append(out, b.eval("[...document.getElementById('set-task-names').children].map((e) => e.textContent).join(', ')"))
		b.eval("document.getElementById('set-dialog').close()")
	}
	b.size(1400, 800, false)
	b.navigate("/?project=proj")
	b.until("the rows", "document.querySelector('#list li[data-slug=c]')")
	dialog("c")
	field("set-task", "#5")
	save()
	b.until("the toast", "document.querySelector('.toast-link')")
	say("put on a task", "document.querySelector('.toast-link').textContent + ' ' + document.querySelector('.toast-link').getAttribute('href')")
	say("its state as it was", "tasksRead.tasks.proj['#5'].state")
	out = append(out, "the state written: "+tasks())
	dialog("c")
	save()
	b.until("the toast", "[...document.querySelectorAll('.toast')].at(-1).textContent === 'Saved'")
	say("the same task", "[...document.querySelectorAll('.toast')].at(-1).textContent")
	b.navigate("/?project=proj")
	b.until("the rows", "document.querySelector('#list li[data-slug=c]')")
	dialog("c")
	field("set-task", "#5")
	field("set-task", "#2")
	save()
	b.until("the toast", "document.querySelector('.toast-link')")
	b.still()
	link := b.rect("document.querySelector('.toast-link')")
	b.click(link.X+link.Width/2, link.Y+link.Height/2)
	b.until("the task's page", "location.search === '?project=proj&task=%232' && !document.getElementById('task-head').hidden")
	say("up, come by the toast", "document.getElementById('task-up').getAttribute('href')")

	if got, want := strings.Join(out, "\n"), taskSetWant; got != want {
		t.Errorf("the task's dialog:\n%s\nwant:\n%s", got, want)
	}
}

// taskSetWant is what TestUITaskSet reads, in the order it does things.
const taskSetWant = `tags: Task #2 /tasks?project=proj&task=%232, Task #1 /tasks?project=proj&task=%231
items of a page on a task: fav-item select tag-item pin-item task-set task-out task-open edit open download archive delete
items of a page on none: fav-item select tag-item pin-item task-set edit open download archive delete
open the task: Open task #1 /tasks?project=proj&task=%231
the items' looks: 1
pinned: Pinned /tasks?project=proj Take the pin off
pinned: {"pin":true}
a page of none:  /  / out false
no parent nor URL: null
offered: Make #3 a new task
new task: {"task":"#3","step":"first","pin":true}
#1 todo design https://example.com/a/one; #2 todo build ; #3 todo first 
a page of a task: #1 / design / out true
written meanwhile: Failed to save: task #1 has changed since it was read: the tasks are read again, so check and save again
written meanwhile, saved: #1 todo design https://example.com/later; #2 todo build ; #3 todo first 
refused: true Failed to save: no
an empty name: true true 0
a name of spaces: true Write more than spaces 3 0
the page refused: #1 todo design https://example.com/later; #2 todo review ; #3 todo first  | {"task":"#2","step":"build"}
saved again: {"task":"#2","step":"review"}
off: {"pin":true}
#1 todo design https://example.com/later; #2 todo review ; #3 todo first 
off, no pin: null
as it opens: false
a suggestion pressed: #2 / design / out true
the field keeps the focus: set-task
it stays closed: false
offered of #9: Make #9 a new task
offered of #: #1· To do, #2· To do, #3 a title· To do, Make # a new task
roles: combobox list true listbox
under the field: yes
reached: #1
escape: false true
read again: true #2
under the field once still: yes
saved: #1 todo design https://example.com/a/one; #2 todo review https://example.com/a/two; #3 todo first 
nothing changed: #1 todo later https://example.com/a/one; #2 todo review https://example.com/a/two; #3 todo first  | {"task":"#1","step":"design"}
typed while saving: {"task":"#1","step":"design"}
an archived page: true 0
pinned, told: Pinned “T b” to the task list
refused, told: Failed to save: HTTP 500: no
whole: #1 / whole / out true
on its own task: true 0
#1· To do, #2· To do, #3 a title· To do, #5 A very long title A very long title A very long title A very long title A very long title A very long title· Done
#1· To do, #2· To do, #3 a title· To do, #5 A very long title A very long title A very long title A very long title A very long title A very long title· Done
put on a task: Put on #5 (Done) /tasks?project=proj&task=%235
its state as it was: done
the state written: #1 todo whole https://example.com/a/one; #2 todo review https://example.com/a/two; #3 todo first 
the same task: Saved
up, come by the toast: /tasks?project=proj&view=todo`
