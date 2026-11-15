package browsertest_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/input"

	"floe/internal/store"
)

// frameBack says the back button of a page whole: where it goes, its word, what it says in full, the key's words.
const frameBack = `(() => { const e = document.getElementById('back');
  return [location.pathname, e.getAttribute('href'), document.getElementById('back-word').textContent, e.title, e.getAttribute('aria-label'), document.getElementById('keys-back').textContent, document.querySelector('.brand').getAttribute('href')].join(' | '); })()`

// The back button of a page whole, by every way a page is opened whole: real presses, the row's press again and a double press, the
// title over the page beside the rows, a pin, the page list's row; a reload keeps it, and a tab that kept nothing has the page list. h goes
// where it says. It shows at every width, a narrow screen with its mark alone.
func TestUIFrameBack(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	project := "proj"
	for _, slug := range []string{"a", "b", "pin", "loose"} {
		s.floe(t, "publish", s.page(t, slug+".yaml", "T "+slug, ""), "--slug", slug)
		if _, err := st.Edit(slug, store.Edit{Project: &project}); err != nil {
			t.Fatal(err)
		}
	}
	s.floe(t, "task", "--slug", "a", "--slug", "b", "--task", "#128", "--state", "doing")
	s.floe(t, "task", "--slug", "pin", "--pin")
	b.size(1400, 800, false)
	defer b.unsize()

	var out []string
	wholeAt := func(what, path string) {
		t.Helper()
		b.until(what+": the page whole", "location.pathname === "+js(path)+" && document.getElementById('frame-title').textContent")
		out = append(out, what+": "+b.eval(frameBack))
	}
	const task = "/tasks?project=proj&task=%23128"
	row := func(slug string) rect {
		t.Helper()
		b.until("the row of "+slug, "document.querySelector('#list li[data-slug="+slug+"] .title')")
		return b.rect("document.querySelector('#list li[data-slug=" + slug + "] .title')")
	}
	opened := func(path string) {
		t.Helper()
		b.navigate(path)
		b.until("the rows", "document.querySelector('#list > li') && document.getElementById('list').children.length > 0")
	}

	// A task's row, pressed twice as a person does.
	opened(task)
	r := row("a")
	b.click(r.X+10, r.Y+r.Height/2)
	b.until("the page beside", "picked?.slug === 'a'")
	r = row("a")
	b.click(r.X+10, r.Y+r.Height/2)
	wholeAt("task row, pressed again", "/a/a/")
	realKey(b, "h", 0)
	b.until("h to the task", "location.pathname + location.search === "+js(task))

	// A double press.
	opened(task)
	r = row("b")
	for _, count := range []int{1, 2} {
		b.mouse(input.DispatchMouseEventTypeMousePressed, r.X+10, r.Y+r.Height/2, count)
		b.mouse(input.DispatchMouseEventTypeMouseReleased, r.X+10, r.Y+r.Height/2, count)
	}
	wholeAt("task row, double press", "/a/b/")

	// The title over the page beside the rows.
	opened(task)
	r = row("a")
	b.click(r.X+10, r.Y+r.Height/2)
	b.until("the title", "picked?.slug === 'a' && document.getElementById('read-title').getAttribute('href')")
	b.moving("document.getElementById('read-title').click()")
	wholeAt("the title over the page", "/a/a/")

	// By Enter, and a reload there.
	opened(task)
	b.eval("document.querySelector('#list li[data-slug=b] .title').focus()")
	b.pressKey("\r")
	wholeAt("task row, Enter", "/a/b/")
	b.navigate("/a/b/")
	wholeAt("reloaded", "/a/b/")

	// A pin on the task list.
	opened("/tasks?project=proj")
	b.until("the pin", "document.querySelector('.pin')")
	b.moving("document.querySelector('.pin').click()")
	wholeAt("pin", "/a/pin/")
	realKey(b, "h", 0)
	b.until("h to the task list", "location.pathname + location.search === '/tasks?project=proj'")

	// The page list's row.
	b.navigate("/?project=proj")
	r = row("loose")
	b.click(r.X+10, r.Y+r.Height/2)
	b.until("the page beside", "picked?.slug === 'loose'")
	r = row("loose")
	b.click(r.X+10, r.Y+r.Height/2)
	wholeAt("page list row", "/a/loose/")

	// A tab that kept nothing.
	b.eval("sessionStorage.clear()")
	b.navigate("/a/a/")
	wholeAt("another tab", "/a/a/")

	// A narrow screen: the mark alone, its words there to read.
	b.navigate("/a/a/")
	b.size(800, 800, false)
	b.until("the bar", "document.getElementById('frame-title').textContent")
	b.size(360, 800, true)
	b.until("the narrow bar", "innerWidth === 360")
	out = append(out, "phone: "+b.eval("(document.querySelector('.brand').offsetParent !== null) + ' ' + document.querySelector('.brand').getAttribute('href') + ' ' + (document.querySelector('.brand svg').getBoundingClientRect().width === 26) + ' ' + (document.getElementById('frame-actions').getBoundingClientRect().right <= innerWidth) + ' ' + (document.documentElement.scrollWidth <= innerWidth)"))
	b.size(800, 800, false)
	out = append(out, "narrow: "+b.eval("(document.getElementById('back').offsetParent !== null) + ' ' + (document.getElementById('back-word').offsetParent === null) + ' ' + document.getElementById('back').getAttribute('aria-label')"))
	b.size(1400, 800, false)

	// A long title gives way: the mark and a short name stay whole.
	longTitle := strings.Repeat("A very long title of a page ", 8)
	if _, err := st.Edit("b", store.Edit{Title: &longTitle}); err != nil {
		t.Fatal(err)
	}
	opened(task)
	b.eval("document.querySelector('#list li[data-slug=b] .title').focus()")
	b.pressKey("\r")
	wholeAt("a long title", "/a/b/")
	for _, width := range []int64{1400, 1100, 900} {
		b.size(width, 800, false)
		whole := b.eval("(() => { const e = document.getElementById('back-word'); return e.scrollWidth <= e.clientWidth && document.querySelector('#back svg').getBoundingClientRect().width === 16; })()")
		if whole != "true" {
			t.Errorf("width %d: the back button gave way", width)
		}
	}
	b.size(1400, 800, false)

	// The other ways a page whole may be: of a version, its diff, and with its name changed there.
	opened(task)
	b.eval("document.querySelector('#list li[data-slug=a] .title').focus()")
	b.pressKey("\r")
	b.until("the page whole", "location.pathname === '/a/a/' && document.getElementById('frame-title').textContent")
	s.floe(t, "publish", s.write(t, "a.yaml", "template: report\nlang: en\ntitle: T a\nsections:\n  - heading: two\n"), "--slug", "a")
	b.navigate("/a/a/?v=1")
	wholeAt("of a version", "/a/a/")
	b.navigate("/a/a/?diff")
	b.until("the diff", "document.getElementById('back-word').textContent")
	out = append(out, "its diff: "+b.eval(frameBack))
	b.navigate("/a/a/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	b.moving("openEdit(items.find((a) => a.slug === 'a')); document.getElementById('edit-slug').value = 'a2'; document.getElementById('edit').requestSubmit()")
	wholeAt("renamed", "/a/a2/")
	b.moving("openEdit(items.find((a) => a.slug === 'a2')); document.getElementById('edit-slug').value = 'a'; document.getElementById('edit').requestSubmit()")
	wholeAt("named back", "/a/a/")

	// Come back to from the browser's memory, what was kept since is drawn.
	b.eval(`sessionStorage.setItem(ORDER_KEY, '{"order":["a"],"shown":["a"],"back":"/tasks?project=proj"}'); dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true }))`)
	out = append(out, "from the browser's memory: "+b.eval(frameBack))

	// A task of no project, and a list at a view.
	s.floe(t, "publish", s.page(t, "none.yaml", "T none", ""), "--slug", "none")
	none := ""
	if _, err := st.Edit("none", store.Edit{Project: &none}); err != nil {
		t.Fatal(err)
	}
	s.floe(t, "task", "--slug", "none", "--task", "#7", "--state", "doing")
	opened("/tasks?project=%2F&task=%237")
	b.eval("document.querySelector('#list li[data-slug=none] .title').focus()")
	b.pressKey("\r")
	wholeAt("a task of no project", "/a/none/")
	opened("/tasks?view=done&project=proj&task=%23128")
	b.navigate("/tasks?view=done&project=proj")
	b.until("the pin", "document.querySelector('.pin')")
	b.moving("document.querySelector('.pin').click()")
	wholeAt("a pin of a list at a view", "/a/pin/")

	// A long name is cut, with nothing past the edge, down to the narrowest width that has the word.
	long := "#" + strings.Repeat("a long task name ", 6)
	s.floe(t, "task", "--slug", "a", "--task", long)
	opened("/tasks?project=proj&task=" + url.QueryEscape(strings.TrimSpace(long)))
	b.eval("document.querySelector('#list li[data-slug=a] .title').focus()")
	b.pressKey("\r")
	b.until("the page whole", "location.pathname === '/a/a/' && document.getElementById('frame-title').textContent")
	for _, width := range []int64{1400, 1100, 900} {
		b.size(width, 800, false)
		last := b.rect("document.getElementById('frame-actions')")
		word := b.rect("document.getElementById('back-word')")
		if last.X+last.Width > float64(width) || word.Width > 12*16+1 || word.Width < 1 {
			t.Errorf("width %d: the bar ends at %v, the word is %v wide", width, last.X+last.Width, word.Width)
		}
	}
	b.size(1400, 800, false)
	if got := strings.Join(out, "\n"); got != frameBackWant {
		t.Errorf("the way back:\n%s\nwant:\n%s", got, frameBackWant)
	}
}

// frameBackWant is what TestUIFrameBack reads, in the order it opens them.
const frameBackWant = `task row, pressed again: /a/a/ | /tasks?project=proj&task=%23128 | #128 | Back to #128 | Back to #128 | Back to #128 | /
task row, double press: /a/b/ | /tasks?project=proj&task=%23128 | #128 | Back to #128 | Back to #128 | Back to #128 | /
the title over the page: /a/a/ | /tasks?project=proj&task=%23128 | #128 | Back to #128 | Back to #128 | Back to #128 | /
task row, Enter: /a/b/ | /tasks?project=proj&task=%23128 | #128 | Back to #128 | Back to #128 | Back to #128 | /
reloaded: /a/b/ | /tasks?project=proj&task=%23128 | #128 | Back to #128 | Back to #128 | Back to #128 | /
pin: /a/pin/ | /tasks?project=proj | Task list | Back to the task list | Back to the task list | Back to the task list | /
page list row: /a/loose/ | /?project=proj | Page list | Back to the page list | Back to the page list | Back to the page list | /?project=proj
another tab: /a/a/ | / | Page list | Back to the page list | Back to the page list | Back to the page list | /
phone: true / true true true
narrow: true true Back to the page list
a long title: /a/b/ | /tasks?project=proj&task=%23128 | #128 | Back to #128 | Back to #128 | Back to #128 | /
of a version: /a/a/ | /tasks?project=proj&task=%23128 | #128 | Back to #128 | Back to #128 | Back to #128 | /
its diff: /a/a/ | /tasks?project=proj&task=%23128 | #128 | Back to #128 | Back to #128 | Back to #128 | /
renamed: /a/a2/ | /tasks?project=proj&task=%23128 | #128 | Back to #128 | Back to #128 | Back to #128 | /
named back: /a/a/ | /tasks?project=proj&task=%23128 | #128 | Back to #128 | Back to #128 | Back to #128 | /
from the browser's memory: /a/a/ | /tasks?project=proj | Task list | Back to the task list | Back to the task list | Back to the task list | /
a task of no project: /a/none/ | /tasks?project=%2F&task=%237 | #7 | Back to #7 | Back to #7 | Back to #7 | /
a pin of a list at a view: /a/pin/ | /tasks?view=done&project=proj | Task list | Back to the task list | Back to the task list | Back to the task list | /`

// A page opened whole and gone back from has its row picked and focused on the list that opened it, by the back button, by h and by the
// browser's own way back, on a task's page and on the page list. Moved on by j, the row is the page last read. A row no longer there, or
// a list the page was not opened from, picks nothing. A page marked unread there is not read by coming back, and a pin has the focus itself.
// The logo and a URL are no way back; a version seen whole stays, the list folded gives the focus to what opens it, and the browser's memory goes the same way.
func TestUIFrameBackFocus(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
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
	s.floe(t, "task", "--slug", "a", "--slug", "b", "--slug", "c", "--task", "#1", "--state", "doing")
	b.size(1400, 800, false)
	defer b.unsize()

	const (
		task  = "/tasks?project=proj&task=%231"
		pages = "/?project=proj"
	)
	var out []string
	// whole waits for the page whole, read.
	whole := func(slug string) {
		t.Helper()
		b.until("the page whole", "location.pathname === '/a/"+slug+"/' && document.getElementById('frame-title').textContent && !document.querySelector('#frame-actions .mark-unread').hidden")
	}
	// open opens the row whole by a double press, as a person does.
	open := func(list, slug string) {
		t.Helper()
		b.navigate(list)
		sel := "document.querySelector('#list li[data-slug=" + slug + "] .title')"
		b.until("the row of "+slug, sel)
		r := b.rect(sel)
		for _, count := range []int{1, 2} {
			b.mouse(input.DispatchMouseEventTypeMousePressed, r.X+10, r.Y+r.Height/2, count)
			b.mouse(input.DispatchMouseEventTypeMouseReleased, r.X+10, r.Y+r.Height/2, count)
		}
		whole(slug)
	}
	// read says what the list came back to has, once its rows are drawn.
	read := func(what, list string) {
		t.Helper()
		b.until(what+": the list", "location.pathname + location.search === "+js(list)+" && document.querySelector('#list li[data-slug]')")
		out = append(out, what+": "+b.eval(`[document.activeElement.closest?.('#list li')?.dataset.slug ?? 'no focus', picked?.slug ?? 'nothing picked',
			[...document.querySelectorAll('#list li.picked')].map((li) => li.dataset.slug).join()].join(' | ')`))
	}
	pressBack := func() {
		t.Helper()
		r := b.rect("document.getElementById('back')")
		b.click(r.X+r.Width/2, r.Y+r.Height/2)
	}

	open(task, "b")
	pressBack()
	read("task page, the back button", task)
	open(task, "c")
	realKey(b, "h", 0)
	read("task page, h", task)
	open(task, "b")
	b.moving("history.back()")
	read("task page, the browser's way back", task)
	open(task, "a")
	realKey(b, "j", 0)
	whole("b")
	pressBack()
	read("task page, moved on by j", task)
	open(pages, "b")
	pressBack()
	read("page list, the back button", pages)
	open(pages, "b")
	b.moving("history.back()")
	read("page list, the browser's way back", pages)

	// Reloaded, the list is as it opens.
	b.navigate(pages)
	read("page list, reloaded", pages)
	// Opened from the task's page, the page list has no part in it.
	open(task, "b")
	b.navigate(pages)
	read("another list", pages)
	// A row that is no longer there.
	open(task, "c")
	s.floe(t, "task", "--slug", "c", "--task", "")
	pressBack()
	read("a row gone", task)

	// Marked unread there, it stays unread: the row has the focus, with nothing beside it to read it.
	open(pages, "b")
	b.eval("document.querySelector('#frame-actions .mark-unread').click()")
	b.until("marked unread", "document.querySelector('#frame-actions .mark-unread').hidden")
	pressBack()
	read("marked unread", pages)
	// One column has nothing beside the list.
	b.size(800, 800, false)
	b.navigate(pages)
	b.until("the row", "document.querySelector('#list li[data-slug=a] .title')")
	b.eval("document.querySelector('#list li[data-slug=a] .title').focus()")
	b.pressKey("\r")
	whole("a")
	b.moving("document.getElementById('back').click()")
	read("one column", pages)
	b.size(1400, 800, false)
	// A pin has no page beside it: the pin has the focus.
	s.floe(t, "task", "--slug", "a", "--pin")
	b.navigate("/tasks?project=proj")
	b.until("the pin", "document.querySelector('.pin')")
	b.moving("document.querySelector('.pin').click()")
	whole("a")
	pressBack()
	b.until("the task list", "location.pathname === '/tasks' && document.querySelector('.pin')")
	out = append(out, "a pin: "+b.eval("document.activeElement.getAttribute('href')"))

	// The logo and a URL to the same list are no way back.
	open(pages, "b")
	r := b.rect("document.querySelector('.brand')")
	b.click(r.X+r.Width/2, r.Y+r.Height/2)
	read("the logo", pages)
	open(pages, "b")
	b.navigate(pages)
	read("the same list's URL", pages)

	// The version seen whole is the one beside the list.
	s.floe(t, "publish", s.write(t, "b.yaml", "template: report\nlang: en\ntitle: T b\nsections:\n  - heading: two\n"), "--slug", "b")
	open(pages, "b")
	b.navigate("/a/b/?v=1")
	b.until("the version whole", "document.getElementById('frame-title').textContent")
	pressBack()
	read("of a version", pages)
	b.until("the version beside", "readLoaded.endsWith('v=1')")
	// The versions to choose from are asked apart from the page
	out = append(out, "its version: "+b.until("the versions", "document.getElementById('read-version').value && picked.v + ' ' + document.getElementById('read-version').value"))

	// The list folded leaves when a row is open: the focus goes to what opens it.
	b.navigate(task)
	sel := "document.querySelector('#list li[data-slug=a] .title')"
	b.until("the row", sel)
	r = b.rect(sel)
	b.click(r.X+10, r.Y+r.Height/2)
	b.until("the title", "picked?.slug === 'a' && document.getElementById('read-title').getAttribute('href')")
	b.eval("document.getElementById('list-hide').click()")
	b.moving("document.getElementById('read-title').click()")
	whole("a")
	pressBack()
	b.until("the list", "location.pathname === '/tasks' && picked?.slug === 'a'")
	out = append(out, "folded: "+b.eval("document.activeElement.id + ' ' + document.getElementById('listcol').offsetParent"))
	b.eval("document.getElementById('list-show').click()")

	// Come back to from the browser's memory: the page whole knows it is there, and the list picks the row as any way back does.
	b.navigate(pages)
	b.eval(`sessionStorage.setItem(ORDER_KEY, '{"order":["a"],"shown":["a"],"back":"/?project=proj","at":{"slug":"c","file":"","v":0}}')`)
	b.navigate("/a/a/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	b.eval(`sessionStorage.setItem(ORDER_KEY, '{"order":["a"],"shown":["a"],"back":"/?project=proj","at":{"slug":"c","file":"","v":0}}'); dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true }))`)
	out = append(out, "whole, from the memory: "+b.eval("JSON.parse(sessionStorage.getItem(ORDER_KEY)).at.slug"))
	b.navigate(pages)
	b.until("the rows", "document.querySelector('#list li[data-slug=a]')")
	b.eval(`sessionStorage.setItem(ORDER_KEY, '{"order":["a"],"shown":["a"],"back":"/?project=proj","at":{"slug":"a","file":"","v":0}}'); dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true }))`)
	read("list, from the memory", pages)

	if got := strings.Join(out, "\n"); got != frameBackFocusWant {
		t.Errorf("what the list has on the way back:\n%s\nwant:\n%s", got, frameBackFocusWant)
	}
}

const frameBackFocusWant = `task page, the back button: b | b | b
task page, h: c | c | c
task page, the browser's way back: b | b | b
task page, moved on by j: b | b | b
page list, the back button: b | b | b
page list, the browser's way back: b | b | b
page list, reloaded: no focus | nothing picked | 
another list: no focus | nothing picked | 
a row gone: no focus | nothing picked | 
marked unread: b | nothing picked | 
one column: a | nothing picked | 
a pin: /a/a/
the logo: no focus | nothing picked | 
the same list's URL: no focus | nothing picked | 
of a version: b | b | b
its version: 1 1
folded: list-show null
whole, from the memory: a
list, from the memory: a | a | a`
