package browsertest_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/store"
)

// A page's menu pins it and sets its task, in the order floe work writes; what a person did not change of the URL is not sent, so that one
// set since stays; a name of no task is a new one, and takes no URL; what is refused keeps the dialog open; off a task, the pin stays. The page
// list's rows lead to the task and the task list.
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
	const fields = "['set-task', 'set-step', 'set-url'].map((id) => document.getElementById(id).value).join(' / ') + ' / out ' + !document.getElementById('set-out').hidden"
	save := func() {
		t.Helper()
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

	// A pin on and off, from the menu.
	menu("a", "pin")
	b.until("pinned", "items.find((x) => x.slug === 'a').work?.pin")
	b.until("drawn", "document.querySelector('#list li[data-slug=a] .task-tag')")
	say("pinned", "document.querySelector('#list li[data-slug=a] .task-tag').textContent + ' ' + document.querySelector('#list li[data-slug=a] .task-tag').getAttribute('href') + ' ' + document.querySelector('#list li[data-slug=a] .pin').textContent")
	out = append(out, "pinned: "+work("a"))

	// A name of no task: new, doing, with no URL though the page before showed one; the suggestions name what there is, and say it is new.
	dialog("a")
	say("a page of none", fields)
	field("set-task", "#1")
	say("the name of a task brings its URL", fields)
	field("set-task", "#3")
	say("a new name takes it away", fields)
	b.until("the suggestions", "document.getElementById('set-task-names').matches(':popover-open') || (document.getElementById('set-task').focus(), false)")
	say("offered", "[...document.getElementById('set-task-names').children].map((e) => e.textContent).join(', ')")
	field("set-step", "first")
	save()
	out = append(out, "new task: "+work("a"), tasks())

	// The URL set since the dialog opened stays, as what was shown is not sent; a URL changed is.
	dialog("b")
	say("a page of a task", fields)
	s.floe(t, "work", "--task", "#1", "--project", "proj", "--url", "https://example.com/later")
	save()
	out = append(out, "a URL set meanwhile: "+tasks())
	dialog("b")
	field("set-url", "https://example.com/mine")
	save()
	out = append(out, "a URL changed: "+tasks())
	dialog("b")
	field("set-url", "")
	save()
	out = append(out, "a URL emptied: "+tasks())

	// What is refused keeps the dialog open, says why; nothing of the page changes.
	was := work("b")
	dialog("b")
	field("set-url", "javascript:alert(1)")
	b.eval("document.getElementById('set').requestSubmit()")
	b.until("the refusal", "!document.getElementById('set-error').hidden")
	say("refused", "document.getElementById('set-dialog').open + ' ' + document.getElementById('set-error').textContent")
	if got := work("b"); got != was {
		t.Errorf("a refusal changed the page: %s", got)
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

	// From a page whole, which reads the tasks as it opens.
	s.floe(t, "work", "--task", "#1", "--project", "proj", "--url", "https://example.com/a/whole")
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

	if got, want := strings.Join(out, "\n"), taskSetWant; got != want {
		t.Errorf("the task's dialog:\n%s\nwant:\n%s", got, want)
	}
}

// taskSetWant is what TestUITaskSet reads, in the order it does things.
const taskSetWant = `tags: Task #2 /tasks?project=proj&task=%232, Task #1 /tasks?project=proj&task=%231
items of a page on a task: fav-item select pin task-set task-open edit open download archive delete
items of a page on none: fav-item select pin task-set edit open download archive delete
open the task: Open task #1 /tasks?project=proj&task=%231
pinned: Pinned /tasks?project=proj Take the pin off
pinned: {"pin":true}
a page of none:  /  /  / out false
the name of a task brings its URL: #1 /  / https://example.com/a/one / out false
a new name takes it away: #3 /  /  / out false
offered: #1, #2, Make #3 a new task
new task: {"task":"#3","step":"first","pin":true}
#1 doing design https://example.com/a/one; #2 doing build ; #3 doing first 
a page of a task: #1 / design / https://example.com/a/one / out true
a URL set meanwhile: #1 doing design https://example.com/later; #2 doing build ; #3 doing first 
a URL changed: #1 doing design https://example.com/mine; #2 doing build ; #3 doing first 
a URL emptied: #1 doing design ; #2 doing build ; #3 doing first 
refused: true Could not save: "javascript:alert(1)" is not an http or https URL
the page refused: #1 doing design ; #2 doing review ; #3 doing first  | {"task":"#2","step":"build"}
saved again: {"task":"#2","step":"review"}
off: {"pin":true}
#1 doing design ; #2 doing review ; #3 doing first 
off, no pin: null
whole: #1 / design / https://example.com/a/whole / out true
on its own task: true 0`
