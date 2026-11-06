package browsertest_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/input"

	"github.com/sukekyo26/floe/internal/store"
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
	s.floe(t, "work", "a", "b", "--task", "#128", "--state", "waiting")
	s.floe(t, "work", "pin", "--pin")
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
	b.eval("document.getElementById('read-title').click()")
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
	b.eval("document.querySelector('.pin').click()")
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
	out = append(out, "phone: "+b.eval("(document.querySelector('.brand').offsetParent !== null) + ' ' + document.querySelector('.brand').getAttribute('href') + ' ' + (document.querySelector('.brand svg').getBoundingClientRect().width === 26) + ' ' + (document.getElementById('server').getBoundingClientRect().right <= innerWidth) + ' ' + (document.documentElement.scrollWidth <= innerWidth)"))
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
	b.eval("openEdit(items.find((a) => a.slug === 'a')); document.getElementById('edit-slug').value = 'a2'; document.getElementById('edit').requestSubmit()")
	wholeAt("renamed", "/a/a2/")
	b.eval("openEdit(items.find((a) => a.slug === 'a2')); document.getElementById('edit-slug').value = 'a'; document.getElementById('edit').requestSubmit()")
	wholeAt("named back", "/a/a/")

	// Come back to from the browser's memory, what was kept since is drawn.
	b.eval(`sessionStorage.setItem('artifacts.order', '{"order":["a"],"shown":["a"],"back":"/tasks?project=proj"}'); dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true }))`)
	out = append(out, "from the browser's memory: "+b.eval(frameBack))

	// A task of no project, and a list at a view.
	s.floe(t, "publish", s.page(t, "none.yaml", "T none", ""), "--slug", "none")
	none := ""
	if _, err := st.Edit("none", store.Edit{Project: &none}); err != nil {
		t.Fatal(err)
	}
	s.floe(t, "work", "none", "--task", "#7", "--state", "doing")
	opened("/tasks?project=%2F&task=%237")
	b.eval("document.querySelector('#list li[data-slug=none] .title').focus()")
	b.pressKey("\r")
	wholeAt("a task of no project", "/a/none/")
	opened("/tasks?view=doing&project=proj&task=%23128")
	b.navigate("/tasks?view=doing&project=proj")
	b.until("the pin", "document.querySelector('.pin')")
	b.eval("document.querySelector('.pin').click()")
	wholeAt("a pin of a list at a view", "/a/pin/")

	// A long name is cut, with nothing past the edge, down to the narrowest width that has the word.
	long := "#" + strings.Repeat("a long task name ", 6)
	s.floe(t, "work", "a", "--task", long)
	opened("/tasks?project=proj&task=" + url.QueryEscape(strings.TrimSpace(long)))
	b.eval("document.querySelector('#list li[data-slug=a] .title').focus()")
	b.pressKey("\r")
	b.until("the page whole", "location.pathname === '/a/a/' && document.getElementById('frame-title').textContent")
	for _, width := range []int64{1400, 1100, 900} {
		b.size(width, 800, false)
		last := b.rect("document.getElementById('server')")
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
a pin of a list at a view: /a/pin/ | /tasks?view=doing&project=proj | Task list | Back to the task list | Back to the task list | Back to the task list | /`
