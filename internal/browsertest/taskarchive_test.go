package browsertest_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/input"

	"github.com/sukekyo26/floe/internal/store"
)

// A task archived by its own button, of no page, of pages or of pages archived already: its state moves it, whatever its pages, and the same
// button gives it back to done with its pages. Its head says so where its state was. Of a parent, it is the parent's alone. A task forgotten
// elsewhere is refused and does not come back; a refused write leaves the pages; a page refused leaves the task as written, and the page on its row.
func TestUITaskArchive(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	project := "proj"
	page := func(slug string, args ...string) {
		t.Helper()
		s.floe(t, "publish", s.page(t, slug+".yaml", "T "+slug, ""), "--slug", slug)
		if _, err := st.Edit(slug, store.Edit{Project: &project}); err != nil {
			t.Fatal(err)
		}
		s.floe(t, append([]string{"work", slug}, args...)...)
	}
	task := func(args ...string) {
		t.Helper()
		s.floe(t, append([]string{"work", "--project", project, "--task"}, args...)...)
	}
	task("#none", "--state", "done", "--title", "no page")
	page("p1", "--task", "#pages", "--state", "done")
	page("p2", "--task", "#pages")
	page("m1", "--task", "#marked", "--state", "done")
	s.floe(t, "archive", "m1")
	page("w1", "--task", "#work", "--state", "doing")
	s.floe(t, "archive", "w1")
	task("#up", "--state", "done", "--title", "the parent")
	task("#kid", "--state", "doing", "--parent", "#up")
	task("#kid2", "--state", "archived", "--parent", "#up")
	task("#forgot", "--state", "done")
	page("f1", "--task", "#refused", "--state", "done")
	page("f2", "--task", "#part", "--state", "done")
	page("f3", "--task", "#part")
	defer b.unsize()
	b.size(1440, 900, false)

	var out []string
	open := func(name string) {
		t.Helper()
		b.navigate("/tasks?project=proj&task=" + strings.ReplaceAll(name, "#", "%23"))
		b.until("the head of "+name, "!document.getElementById('task-head').hidden && document.getElementById('task-title').textContent.startsWith("+js(name)+")")
	}
	const button = "document.getElementById('task-archive')"
	head := func(what string) {
		t.Helper()
		out = append(out, what+": "+b.eval(`[`+button+`.hidden ? 'no button' : `+button+`.textContent, document.getElementById('task-state').hidden ? 'no select' : document.getElementById('task-state').value,
			document.getElementById('task-archived').hidden ? 'no word' : document.getElementById('task-archived').textContent,
			'rows ' + [...document.querySelectorAll('#list li[data-slug]')].map((li) => li.dataset.slug + (items.find((a) => a.slug === li.dataset.slug).archived ? '*' : '')).join(), document.activeElement.id].join(' | ')`))
	}
	press := func(word string) {
		t.Helper()
		r := b.rect(button)
		b.click(r.X+r.Width/2, r.Y+r.Height/2)
		b.until("the button says "+word, button+".textContent === "+js(word))
		b.until("the press done", "!archivingTask")
	}
	toast := func() string {
		return b.eval("[...document.querySelectorAll('.toast')].at(-1).textContent")
	}
	shelf := func(what string) {
		t.Helper()
		b.navigate("/tasks?project=proj&view=done")
		b.until("the shelf", "document.querySelector('#list li .title .tname')")
		done := b.eval("[...document.querySelectorAll('#list li .title .tname')].map((e) => e.textContent).sort().join()")
		b.navigate("/tasks?project=proj&view=archived")
		b.until("the shelf", "!tasksRead.error && items.length > 0")
		archived := b.eval("[...document.querySelectorAll('#list li .title .tname')].map((e) => e.textContent).sort().join()")
		out = append(out, what+": done "+done+" | archive "+archived)
	}

	shelf("at first")

	// Of no page: the button is there, and the same button gives it back.
	open("#none")
	head("no page")
	press("Bring the archive back")
	head("no page, archived")
	out = append(out, "said: "+toast())
	out = append(out, "the way up: "+b.eval("document.getElementById('task-up').getAttribute('href')"))
	press("Archive the task")
	head("no page, given back")
	out = append(out, "said: "+toast())

	// Of no page, drawn by the press itself: the looks at the list are held, so that none draws it for it.
	task("#still", "--state", "done")
	open("#still")
	b.eval("pressedAt = performance.now() + 1e9; " + button + ".click()")
	b.until("the head drawn by the press", button+".textContent === 'Bring the archive back'")
	b.eval("pressedAt = -Infinity")

	// A double press: the second lands on the button turned the other way, and is no press more.
	task("#double", "--state", "done")
	open("#double")
	r := b.rect(button)
	x, y := r.X+r.Width/2, r.Y+r.Height/2
	b.mouse(input.DispatchMouseEventTypeMousePressed, x, y, 1)
	b.mouse(input.DispatchMouseEventTypeMouseReleased, x, y, 1)
	b.until("the press done", button+".textContent === 'Bring the archive back' && !archivingTask")
	b.mouse(input.DispatchMouseEventTypeMousePressed, x, y, 2)
	b.mouse(input.DispatchMouseEventTypeMouseReleased, x, y, 2)
	b.painted()
	b.until("no press more", "!archivingTask")
	out = append(out, "a double press: "+b.eval(button+".textContent + ' ' + tasksOf().find((x) => x.name === '#double').state"))

	// A page put on it since it was drawn goes with it, the pages read after the task is written.
	page("l1", "--task", "#late", "--state", "done")
	open("#late")
	b.eval("pressedAt = performance.now() + 1e9")
	page("l2", "--task", "#late")
	b.eval(button + ".click()")
	b.until("the pages", "!archivingTask && items.filter((a) => a.work?.task === '#late').length === 2")
	out = append(out, "a page since: "+b.eval("items.filter((a) => a.work?.task === '#late').map((a) => a.slug + (a.archived ? '*' : '')).sort().join()"))
	b.eval("pressedAt = -Infinity")

	// Of pages: the pages go with it, and come back with it.
	open("#pages")
	press("Bring the archive back")
	b.until("the pages", "items.filter((a) => a.work?.task === '#pages' && a.archived).length === 2")
	head("pages, archived")
	shelf("with #pages in the archive")
	open("#pages")
	press("Archive the task")
	b.until("the pages back", "items.filter((a) => a.work?.task === '#pages' && !a.archived).length === 2")
	head("pages, given back")

	// Its pages archived do not move it: it is done, and archived by its button.
	open("#marked")
	head("pages archived by floe archive")
	press("Bring the archive back")
	head("those, archived")

	// A page given back or published again leaves it in the archive, with its unread on its row.
	s.floe(t, "publish", s.page(t, "m1.yaml", "T m1", ""), "--slug", "m1")
	b.navigate("/tasks?project=proj&view=archived")
	b.until("the row", "[...document.querySelectorAll('#list li .title .tname')].some((e) => e.textContent === '#marked')")
	out = append(out, "published again: "+b.eval("[...document.querySelectorAll('#list li')].find((li) => li.querySelector('.title .tname').textContent === '#marked').querySelector('.sub').textContent"))

	// Not done: no button, however its pages are.
	open("#work")
	head("at work, its pages archived")

	// A parent's is its own. Its children keep theirs, the ones at work first.
	open("#up")
	const kids = "[...document.querySelectorAll('#task-family .kin-row')].map((e) => e.dataset.task + ' ' + e.querySelector('.kid-state').textContent).join()"
	b.until("the children", kids)
	out = append(out, "the children: "+b.eval(kids))
	press("Bring the archive back")
	out = append(out, "the parent archived: "+b.eval(kids))
	task("#kid3", "--state", "todo", "--parent", "#up")
	b.until("a child more", "document.querySelectorAll('#task-family .kin-row').length === 3")
	out = append(out, "a child added: "+b.eval(kids))
	out = append(out, "the child: "+b.eval("tasksOf().find((x) => x.name === '#kid').state"))
	open("#kid")
	out = append(out, "its parent, from the child: "+b.eval("document.querySelector('#task-family .kin-row').dataset.task"))
	shelf("with the parent in the archive")

	// Forgotten elsewhere: refused, as what was seen is not there, and not made again.
	open("#forgot")
	b.eval("window.pressed = " + button)
	task("#forgot", "--forget")
	b.eval("window.pressed.click()")
	b.until("the refusal", "!document.getElementById('error').hidden")
	out = append(out, "forgotten: "+b.eval("document.getElementById('error').textContent"))
	b.until("read again", "!document.getElementById('task-missing').hidden")
	if tasks, err := st.ReadTasks(); err != nil {
		t.Fatal(err)
	} else if _, there := tasks["proj"]["#forgot"]; there {
		t.Error("the task forgotten was made again")
	}

	// The task's write refused: its pages are not touched.
	open("#refused")
	b.eval(`window.realFetch = fetch; fetch = (u, o) => (o?.method === 'PATCH' && String(u).startsWith('/api/tasks') ? Promise.resolve(new Response('no', { status: 500 })) : realFetch(u, o)); ` + button + `.click()`)
	b.until("the refusal", "!document.getElementById('error').hidden")
	out = append(out, "refused: "+b.eval("document.getElementById('error').textContent"))
	head("refused, as it was")

	// The server not reached: said, and nothing written.
	open("#refused")
	b.eval(`window.realFetch = fetch; fetch = (u, o) => (o?.method === 'PATCH' ? Promise.reject(new Error('no server')) : realFetch(u, o)); ` + button + `.click()`)
	b.until("said", "!archivingTask && !document.getElementById('error').hidden")
	out = append(out, "not reached: "+b.eval("document.getElementById('error').textContent + ' | ' + tasksOf().find((x) => x.name === '#refused').state"))
	b.eval("fetch = realFetch; showError('')")

	// Every page refused: the task as written, said so, and each page on its row.
	page("f4", "--task", "#allpages", "--state", "done")
	open("#allpages")
	b.eval(`window.realFetch = fetch; fetch = (u, o) => (o?.method === 'PATCH' && String(u).startsWith('/api/artifacts') ? Promise.resolve(new Response('no', { status: 500 })) : realFetch(u, o)); ` + button + `.click()`)
	b.until("the refusal", "!archivingTask && !document.getElementById('error').hidden")
	out = append(out, "every page refused: "+b.eval("[...document.querySelectorAll('.toast')].at(-1).textContent + ' | ' + document.getElementById('error').textContent + ' | ' + tasksOf().find((x) => x.name === '#allpages').state"))
	b.eval("fetch = realFetch; showError('')")

	// A page refused: the task as written, and the page on its row.
	open("#part")
	b.eval(`window.realFetch = fetch; fetch = (u, o) => (o?.method === 'PATCH' && String(u).startsWith('/api/artifacts/f3') ? Promise.resolve(new Response('no', { status: 500 })) : realFetch(u, o)); ` + button + `.click()`)
	b.until("the refusal", "!archivingTask && !document.getElementById('error').hidden")
	b.until("the other page", "items.find((a) => a.slug === 'f2').archived")
	out = append(out, "a page refused: "+b.eval("document.getElementById('error').textContent"))
	b.until("the head", button+".textContent === 'Bring the archive back'")
	head("a page refused, the task archived")
	b.eval("fetch = realFetch")

	// Archived elsewhere with the choices open: they stay until closed, and the word takes their place then.
	task("#open", "--state", "done")
	open("#open")
	r = b.rect("document.getElementById('task-state')")
	b.click(r.X+r.Width/2, r.Y+r.Height/2)
	b.until("the choices", "document.getElementById('task-state').matches(':open')")
	task("#open", "--state", "archived")
	b.until("read", "tasksOf().find((x) => x.name === '#open').state === 'archived'")
	out = append(out, "archived elsewhere, open: "+b.eval("document.getElementById('task-state').hidden + ' ' + document.getElementById('task-archived').hidden"))
	b.pressKey("\x1b")
	b.until("the choices closed", "!document.getElementById('task-state').matches(':open')")
	b.until("the word", "!document.getElementById('task-archived').hidden")
	out = append(out, "closed: "+b.eval("document.getElementById('task-state').hidden + ' ' + document.getElementById('task-archived').hidden"))

	// Archived elsewhere with the focus on the choices, closed: it goes to the button beside the word.
	task("#focused", "--state", "doing")
	open("#focused")
	b.eval("document.getElementById('task-state').focus()")
	task("#focused", "--state", "archived")
	b.until("the word", "!document.getElementById('task-archived').hidden")
	out = append(out, "the focus on the choices: "+b.eval("document.activeElement.id"))

	// The archive's latest first, by when each went there.
	task("#first", "--state", "archived")
	time.Sleep(2 * time.Millisecond) // the times are of milliseconds, and the same two go by name
	task("#second", "--state", "archived")
	b.navigate("/tasks?project=proj&view=archived")
	b.until("the rows", "document.querySelector('#list li .title .tname')")
	out = append(out, "the latest first: "+b.eval("[...document.querySelectorAll('#list li .title .tname')].map((e) => e.textContent).slice(0, 3).join()"))

	// A child added by the button of a parent archived.
	open("#up")
	r = b.rect("document.getElementById('task-child')")
	b.click(r.X+r.Width/2, r.Y+r.Height/2)
	b.until("the dialog", "document.getElementById('kin-dialog').open")
	b.eval("document.getElementById('kin-task').value = '#added'; document.getElementById('kin-task').dispatchEvent(new Event('input', { bubbles: true })); document.getElementById('kin-title').value = 'added'")
	b.eval("document.getElementById('kin').requestSubmit()")
	b.until("saved", "!document.getElementById('kin-dialog').open && document.querySelector('#task-family .kin-row[data-task=\"#added\"]')")
	out = append(out, "a child by the button: "+b.eval(kids))

	// At every width the word and the button stay in the window.
	open("#none")
	press("Bring the archive back")
	for _, width := range []int64{1440, 1100, 900, 800, 360} {
		b.size(width, 900, width == 360)
		b.until("the width", fmt.Sprintf("innerWidth === %d", width))
		whole := b.eval(`(() => { const word = document.getElementById('task-archived').getBoundingClientRect(); const box = ` + button + `.getBoundingClientRect();
			return [word.width > 0, word.right <= innerWidth, word.height < 24, box.right <= innerWidth, document.documentElement.scrollWidth <= innerWidth].join(' '); })()`)
		if whole != "true true true true true" {
			t.Errorf("width %d: the head is out of shape: %s", width, whole)
		}
	}
	b.size(1440, 900, false)

	if got, want := strings.Join(out, "\n"), taskArchiveWant; got != want {
		t.Errorf("archiving tasks:\n%s\nwant:\n%s", got, want)
	}
}

const taskArchiveWant = `at first: done #forgot,#marked,#none,#pages,#part,#refused,#up | archive #kid2
no page: Archive the task | done | no word | rows  | 
no page, archived: Bring the archive back | no select | Archive | rows  | task-archive
said: Archived #none
the way up: /tasks?project=proj&view=archived
no page, given back: Archive the task | done | no word | rows  | task-archive
said: Brought #none back from the archive
a double press: Bring the archive back archived
a page since: l1*,l2*
pages, archived: Bring the archive back | no select | Archive | rows p1*,p2* | task-archive
with #pages in the archive: done #forgot,#marked,#none,#part,#refused,#up | archive #double,#kid2,#late,#pages,#still
pages, given back: Archive the task | done | no word | rows p1,p2 | task-archive
pages archived by floe archive: Archive the task | done | no word | rows m1* | 
those, archived: Bring the archive back | no select | Archive | rows m1* | task-archive
published again: 1 page(s)1 unread
at work, its pages archived: no button | doing | no word | rows w1* | 
the children: #kid In progress,#kid2 Archive
the parent archived: #kid In progress,#kid2 Archive
a child added: #kid3 To do,#kid In progress,#kid2 Archive
the child: doing
its parent, from the child: #up
with the parent in the archive: done #forgot,#none,#pages,#part,#refused | archive #double,#kid2,#late,#marked,#still,#up
forgotten: Could not save: task #forgot is no longer there
refused: Could not save: no
refused, as it was: Archive the task | done | no word | rows f1 | 
not reached: Could not save: no server | done
every page refused: Archived #allpages | 1 could not be changed (HTTP 500: no). | archived
a page refused: 1 could not be changed (HTTP 500: no).
a page refused, the task archived: Bring the archive back | no select | Archive | rows f3,f2* | 
archived elsewhere, open: false true
closed: true false
the focus on the choices: task-archive
the latest first: #second,#first,#focused
a child by the button: #added To do,#kid3 To do,#kid In progress,#kid2 Archive`

// The state changed on a task forgotten elsewhere is refused as what was seen is not there, says so and reads the tasks again, and does
// not make the task again; on a task of pages alone, of no row, it is there.
func TestUITaskStateGone(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	project := "proj"
	s.floe(t, "work", "--project", project, "--task", "#gone", "--state", "done")
	s.floe(t, "publish", s.page(t, "p.yaml", "T p", ""), "--slug", "p")
	if _, err := st.Edit("p", store.Edit{Project: &project}); err != nil {
		t.Fatal(err)
	}
	s.floe(t, "work", "p", "--task", "#pages")
	// No row for #pages, which the command made.
	if err := os.WriteFile(filepath.Join(s.Env["FLOE_DIR"], "tasks.json"), []byte(`{"proj": {"#gone": {"state": "done", "at": "2026-01-01T00:00:00Z"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	const change = "(() => { const sel = document.getElementById('task-state'); sel.value = 'doing'; sel.dispatchEvent(new Event('change', { bubbles: true })); })()"

	b.navigate("/tasks?project=proj&task=%23gone")
	b.until("the head", "!document.getElementById('task-head').hidden")
	s.floe(t, "work", "--project", project, "--task", "#gone", "--forget")
	b.eval(change)
	b.until("the refusal", "!document.getElementById('error').hidden")
	b.until("read again", "!document.getElementById('task-missing').hidden")
	if got, want := b.eval("document.getElementById('error').textContent"), "The state could not be changed: task #gone is no longer there"; got != want {
		t.Errorf("forgotten: %q, want %q", got, want)
	}
	tasks, err := st.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	if _, there := tasks["proj"]["#gone"]; there {
		t.Error("the task forgotten was made again")
	}

	// A write refused by anything else reads the tasks: the page shows the state another session wrote since, with the looks at the list held
	// so that none reads them for it, and the refusal stays.
	const refuse = "window.realFetch = fetch; fetch = (u, o) => (o?.method === 'PATCH' ? Promise.resolve(new Response('no', { status: 500 })) : realFetch(u, o)); pressedAt = performance.now() + 1e9; "
	const free = "fetch = realFetch; pressedAt = -Infinity; showError('')"
	s.floe(t, "work", "--project", project, "--task", "#other", "--state", "done")
	b.navigate("/tasks?project=proj&task=%23other")
	b.until("the state", "document.getElementById('task-state').value === 'done'")
	b.eval(refuse)
	s.floe(t, "work", "--project", project, "--task", "#other", "--state", "waiting")
	b.eval(change)
	b.until("the refusal", "!document.getElementById('error').hidden")
	if got, want := b.eval("document.getElementById('task-state').value + ' | ' + document.getElementById('error').textContent"), "waiting | The state could not be changed: no"; got != want {
		t.Errorf("refused by another reason: %q, want %q", got, want)
	}
	b.eval(free)

	// The archive button the same.
	s.floe(t, "work", "--project", project, "--task", "#other", "--state", "done")
	b.navigate("/tasks?project=proj&task=%23other")
	b.until("the button", "!document.getElementById('task-archive').hidden")
	b.eval(refuse)
	s.floe(t, "work", "--project", project, "--task", "#other", "--state", "waiting")
	b.eval("document.getElementById('task-archive').click()")
	b.until("the refusal", "!archivingTask && !document.getElementById('error').hidden")
	if got, want := b.eval("document.getElementById('task-archive').hidden + ' | ' + document.getElementById('error').textContent"), "true | Could not save: no"; got != want {
		t.Errorf("the button refused by another reason: %q, want %q", got, want)
	}
	b.eval(free)

	b.navigate("/tasks?project=proj&task=%23pages")
	b.until("the head", "!document.getElementById('task-head').hidden")
	b.eval(change)
	b.until("the state", "tasksRead.tasks.proj?.['#pages']?.state === 'doing'")
	if got := b.eval("document.getElementById('error').hidden"); got != "true" {
		t.Errorf("a task of pages alone: error shown is %s", got)
	}
}
