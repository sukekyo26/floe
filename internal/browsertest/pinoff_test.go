package browsertest_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/input"

	"github.com/sukekyo26/floe/internal/store"
)

// The × on a pin of the task list takes the pin off where it is, by a press, Enter or Space. The focus goes to the title of the pin in its
// place, or the help once none is left, and stays where it is if it is elsewhere. A double press takes one off, a refusal leaves the pin and
// says so, and the pins are drawn anew only as they change, with the focus back, and not while pressed. At every width a long title gives
// way and the × stays whole, inside its pin. The last pin of a project with no task leaves the screen of a project not on the list.
func TestUIPinOff(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	project := "proj"
	long := strings.Repeat("A long title of a pinned page ", 6)
	var slugs []string
	publish := func(slug, title string, on *string) {
		t.Helper()
		s.floe(t, "publish", s.page(t, slug+".yaml", title, ""), "--slug", slug)
		if _, err := st.Edit(slug, store.Edit{Project: on}); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= 8; i++ {
		slug, title := fmt.Sprintf("p%d", i), fmt.Sprintf("T p%d", i)
		if i == 2 {
			title = long
		}
		publish(slug, title, &project)
		slugs = append(slugs, slug)
	}
	solo := "solo"
	publish("alone", "T alone", &solo)
	s.floe(t, "work", "p1", "--task", "#1")
	s.floe(t, append(append([]string{"work"}, slugs...), "alone", "--pin")...)
	defer b.unsize()
	b.size(1440, 900, false)
	b.navigate("/tasks?project=proj")
	b.until("the pins", "document.querySelectorAll('.pin-off').length === 8")

	const chip = "document.querySelector('.pin-chip[data-slug=p2]')"
	for _, width := range []int64{1440, 1100, 900, 800, 360} {
		b.size(width, 900, width == 360)
		b.until("the width", fmt.Sprintf("innerWidth === %d", width))
		whole := b.eval(`(() => {
			const chip = ` + chip + `.getBoundingClientRect();
			const off = ` + chip + `.querySelector('.pin-off').getBoundingClientRect();
			const words = ` + chip + `.querySelector('.pin-title');
			return [off.width === 28, off.height === 28, off.right <= chip.right, off.left >= words.getBoundingClientRect().right, off.top >= chip.top, off.bottom <= chip.bottom,
				words.scrollWidth > words.clientWidth, document.documentElement.scrollWidth <= innerWidth].join(' ');
		})()`)
		if whole != "true true true true true true true true" {
			t.Errorf("width %d: the pin is out of shape: %s", width, whole)
		}
		if width == 360 {
			// Where the pins scroll, each keeps its width instead of pressing the others.
			if got := b.eval(chip + ".getBoundingClientRect().width"); got != "208" {
				t.Errorf("a long pin on a phone is %s wide, want 13rem", got)
			}
		}
	}
	b.size(1440, 900, false)
	b.until("the width", "innerWidth === 1440")

	var out []string
	off := func(slug string) string {
		return "document.querySelector('.pin-chip[data-slug=" + slug + "] .pin-off')"
	}
	gone := func(slug string) {
		t.Helper()
		b.until(slug+" off", "!document.querySelector('.pin-chip[data-slug="+slug+"]')")
	}
	say := func(what string) {
		t.Helper()
		out = append(out, what+": "+b.eval(`[[...document.querySelectorAll('.pin-chip')].map((c) => c.dataset.slug).join(), (document.activeElement.closest('.pin-chip')?.dataset.slug ?? '') + ' ' + (document.activeElement.id || document.activeElement.className || document.activeElement.tagName)].join(' | ')`))
	}
	press := func(slug string) {
		t.Helper()
		r := b.rect(off(slug))
		b.click(r.X+r.Width/2, r.Y+r.Height/2)
		gone(slug)
	}
	byKey := func(slug, key string) {
		t.Helper()
		b.eval(off(slug) + ".focus()")
		b.pressKey(key)
		gone(slug)
	}

	press("p1")
	say("the last, pressed")
	out = append(out, "its toast: "+b.eval("[...document.querySelectorAll('.toast')].at(-1).textContent")+", stays on the list: "+b.eval("location.pathname"))
	press("p8")
	say("the first, pressed")
	byKey("p7", "\r")
	say("Enter")
	byKey("p6", " ")
	say("Space")

	// A double press: the second lands on the next pin, moved under it.
	r := b.rect(off("p5"))
	x, y := r.X+r.Width/2, r.Y+r.Height/2
	b.mouse(input.DispatchMouseEventTypeMousePressed, x, y, 1)
	b.mouse(input.DispatchMouseEventTypeMouseReleased, x, y, 1)
	gone("p5")
	b.mouse(input.DispatchMouseEventTypeMousePressed, x, y, 2)
	b.mouse(input.DispatchMouseEventTypeMouseReleased, x, y, 2)
	b.painted()
	say("a double press")

	// The focus elsewhere stays there.
	b.eval("openFind()")
	b.until("the search", "document.activeElement.id === 'find-q'")
	b.eval(off("p4") + ".click()")
	gone("p4")
	say("from the search")
	b.eval("document.getElementById('find').close()")

	// A refusal: the pin and the focus stay, and it is said where a reader of the screen hears it.
	b.eval(`window.realFetch = fetch; fetch = (u, o) => (o?.method === 'PATCH' ? Promise.resolve(new Response('no such page', { status: 404 })) : realFetch(u, o)); ` + off("p3") + `.focus(); ` + off("p3") + `.click()`)
	b.until("the refusal", "!document.getElementById('error').hidden")
	say("refused")
	out = append(out, "said: "+b.eval("document.getElementById('error').textContent + ' ' + document.getElementById('error').getAttribute('role')"))
	b.eval("fetch = realFetch; showError('')")

	// Another session: nothing of the pins changed draws nothing; a pin changed draws them, with the focus back.
	b.eval("window.was = document.activeElement")
	publish("other", "T other", &project)
	b.until("the other page", "items.some((a) => a.slug === 'other')")
	out = append(out, "nothing of the pins changed: "+b.eval("document.activeElement === window.was"))
	title := "T p2"
	if _, err := st.Edit("p2", store.Edit{Title: &title}); err != nil {
		t.Fatal(err)
	}
	b.until("the pin drawn anew", "document.querySelector('.pin-chip[data-slug=p2]').textContent === 'T p2'")
	say("a pin changed")
	out = append(out, "drawn anew: "+b.eval("document.activeElement !== window.was"))

	// Held down, nothing draws them anew, so that the press is not lost.
	r = b.rect(off("p3"))
	x, y = r.X+r.Width/2, r.Y+r.Height/2
	b.eval("window.was = " + off("p3"))
	b.mouse(input.DispatchMouseEventTypeMousePressed, x, y, 1)
	title = "T p2 again"
	if _, err := st.Edit("p2", store.Edit{Title: &title}); err != nil {
		t.Fatal(err)
	}
	b.eval("window.looked = false; load(true).then(() => { window.looked = true; })")
	b.until("the look", "window.looked")
	out = append(out, "held down: "+b.eval("window.was.isConnected"))
	b.mouse(input.DispatchMouseEventTypeMouseReleased, x, y, 1)
	gone("p3")
	say("let go")

	press("p2")
	say("none left")
	out = append(out, "says: "+b.eval("document.querySelector('.pins-none').textContent")+", the page's task: "+b.eval("items.find((a) => a.slug === 'p1').work.task"))

	// A project of pins alone: the last gone, it is no project on the list.
	b.navigate("/tasks?project=solo")
	b.until("the pin", off("alone"))
	press("alone")
	b.until("the screen", "!document.getElementById('tasks-none').hidden")
	out = append(out, "the project's last: "+b.eval("[location.search, document.querySelector('#tasks-none strong').textContent, document.getElementById('tasks-other').hidden, document.activeElement.id].join(' | ')"))

	if got := strings.Join(out, "\n"); got != pinOffWant {
		t.Errorf("taking the pins off:\n%s\nwant:\n%s", got, pinOffWant)
	}
}

const pinOffWant = `the last, pressed: p8,p7,p6,p5,p4,p3,p2 | p2 pin unseen
its toast: Took the pin off “T p1”, stays on the list: /tasks
the first, pressed: p7,p6,p5,p4,p3,p2 | p7 pin unseen
Enter: p6,p5,p4,p3,p2 | p6 pin unseen
Space: p5,p4,p3,p2 | p5 pin unseen
a double press: p4,p3,p2 | p4 icon-btn pin-off
from the search: p3,p2 |  find-q
refused: p3,p2 | p3 icon-btn pin-off
said: Could not save: HTTP 404: no such page alert
nothing of the pins changed: true
a pin changed: p3,p2 | p3 icon-btn pin-off
drawn anew: true
held down: true
let go: p2 | p2 pin unseen
none left:  |  pins-help
says: None, the page's task: #1
the project's last: ?project=solo | No tasks in solo | false | pins-help`
