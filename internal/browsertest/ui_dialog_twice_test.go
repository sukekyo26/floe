package browsertest_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp/kb"

	"floe/internal/store"
)

// TestUIDialogTwice presses double, and holds a key, on what opens or closes a modal: what is left of the press or the key
// reaches neither what is then on top nor what is under it, while a press or a key begun after it does.
func TestUIDialogTwice(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	publish := func(slug, task string, more ...string) {
		t.Helper()
		s.floe(t, "publish", s.page(t, slug+".yaml", "T "+slug, ""), "--slug", slug)
		project := "proj"
		if _, err := st.Edit(slug, store.Edit{Project: &project}); err != nil {
			t.Fatal(err)
		}
		s.floe(t, append([]string{"task", "--slug", slug, "--task", task}, more...)...)
	}
	publish("own", "S300", "--new")
	publish("other-0", "S301", "--new")
	for i := 1; i < 8; i++ {
		publish(fmt.Sprintf("other-%d", i), "S301")
	}
	s.floe(t, "task", "--project", "proj", "--task", "S300", "--categories", "a", "--categories", "b", "--categories", "c", "--categories", "d", "--categories", "e", "--categories", "f")
	b.size(1280, 900, false)
	defer b.unsize()

	open := func() {
		t.Helper()
		b.navigate("/tasks?project=proj&task=S300")
		b.until("the rows", "document.querySelector('#list li[data-slug]')")
		b.eval("window.sent = []; window.realFetch ??= window.fetch; window.fetch = (u, i) => { if (i?.method === 'PATCH') sent.push(new URL(u, location).pathname); return realFetch(u, i); }")
	}
	center := func(sel string) (float64, float64) {
		t.Helper()
		r := b.rect("document.querySelector(" + js(sel) + ")")
		return r.X + r.Width/2, r.Y + r.Height/2
	}
	// What is on the screen at x, y: the modal, for its backdrop
	at := func(x, y float64) string {
		t.Helper()
		return b.eval(fmt.Sprintf("(() => { const e = document.elementFromPoint(%g, %g); return e.tagName === 'DIALOG' ? e.id : (e.closest('li[data-slug], li[id], button') ?? e).id; })()", x, y))
	}
	press := func(x, y float64, counts ...int) {
		t.Helper()
		for _, count := range counts {
			b.mouse(input.DispatchMouseEventTypeMousePressed, x, y, count)
			b.mouse(input.DispatchMouseEventTypeMouseReleased, x, y, count)
		}
	}
	pressAt := func(sel string, counts ...int) {
		t.Helper()
		b.still()
		x, y := center(sel)
		press(x, y, counts...)
	}
	focus := func(sel string) {
		t.Helper()
		b.eval("document.querySelector(" + js(sel) + ").focus()")
	}
	until := func(id string, opened bool) {
		t.Helper()
		not := "!"
		if opened {
			not = ""
		}
		b.until(id, not+"document.getElementById("+js(id)+").open")
	}
	out := []string{}
	// The modals open, the focus and how many writes were sent
	say := func(name string) {
		t.Helper()
		b.painted()
		out = append(out, name+": "+b.eval("[...document.querySelectorAll('dialog:open')].map((d) => d.id).join(' over ') + ' / ' + (document.activeElement.id || document.activeElement.tagName) + ' / sent ' + sent.length"))
	}
	shut := func() {
		t.Helper()
		for range 3 {
			if b.eval("document.querySelector('dialog:open') !== null") != "true" {
				return
			}
			b.pressKey(kb.Escape)
			b.painted()
		}
		b.until("all shut", "!document.querySelector('dialog:open')")
	}
	openAdd := func() {
		t.Helper()
		pressAt("#task-edit", 1)
		until("edit-task-dialog", true)
		pressAt("#edit-category-plus", 1)
		until("edit-add-dialog", true)
	}
	openMove := func() {
		t.Helper()
		pressAt("#task-add-page", 1)
		until("add-page-dialog", true)
		b.pressKey(kb.ArrowDown)
		b.pressKey(kb.Enter)
		until("move-dialog", true)
	}

	// What closes a modal, pressed once and double: the same
	open()
	for _, c := range []struct {
		name string
		prep func()
		sel  string
	}{
		{"help, close", func() { pressAt("#keys-open", 1); until("keys", true) }, "#keys [data-close]"},
		{"settings, close", func() { pressAt("#settings-open", 1); until("settings", true) }, "#settings [data-close]"},
		{"add a category, cancel", openAdd, "#edit-add-dialog [data-close]"},
		{"move, no", openMove, "#move-no"},
	} {
		for _, counts := range [][]int{{1}, {1, 2}} {
			c.prep()
			pressAt(c.sel, counts...)
			say(c.name + fmt.Sprint(counts))
			shut()
		}
	}
	for _, counts := range [][]int{{1}, {1, 2}} {
		openAdd()
		b.fill("#edit-add-new", "g"+fmt.Sprint(len(counts)))
		pressAt("#edit-add-submit", counts...)
		say("add a category, add" + fmt.Sprint(counts))
		shut()
	}
	// The backdrop, over a grip of the modal under it, which a press on its way (pointerdown, not dropped) leaves as it was
	openAdd()
	var x, y float64
	for i := 6; i >= 1; i-- {
		if cx, cy := center(fmt.Sprintf("#edit-categories li:nth-child(%d) [data-testid=category-grip]", i)); at(cx, cy) == "edit-add-dialog" {
			x, y = cx, cy
			break
		}
	}
	if x == 0 {
		t.Fatal("no grip under the backdrop")
	}
	press(x, y, 1, 2)
	say("add a category, backdrop double")
	out = append(out, "grips: "+b.eval("[...document.querySelectorAll('#edit-categories li')].some((li) => li.draggable)"))
	shut()

	// What opens a modal, pressed double: it stays open, its field focused
	for _, c := range []struct {
		name, sel, id string
	}{
		{"help", "#keys-open", "keys"},
		{"settings", "#settings-open", "settings"},
		{"add a page", "#task-add-page", "add-page-dialog"},
		{"edit", "#task-edit", "edit-task-dialog"},
	} {
		x, y = center(c.sel)
		press(x, y, 1)
		until(c.id, true)
		b.still()
		if got := at(x, y); got != c.id {
			t.Fatalf("%s: %q is under its button, not the modal's backdrop", c.name, got)
		}
		shut()
		press(x, y, 1, 2)
		say("open " + c.name)
		shut()
	}
	// A row of another task's page pressed double where the question's Yes comes: asked, nothing written; then Yes
	openMove()
	b.still()
	x, y = center("#move-yes")
	b.pressKey(kb.Escape)
	until("move-dialog", false)
	b.still()
	if got := at(x, y); !strings.HasPrefix(got, "add-page-") {
		t.Fatalf("no row where Yes comes: %q", got)
	}
	press(x, y, 1, 2)
	say("row double")
	press(x, y, 1)
	until("add-page-dialog", false)
	b.until("written", "sent.length === 1")
	say("then Yes")
	b.eval("sent.length = 0")

	// Not dropped: the backdrop's press, two presses on a field's words, a press on a row after a close
	pressAt("#task-add-page", 1)
	until("add-page-dialog", true)
	b.still()
	b.fill("#add-page-q", "words here")
	r := b.rect("document.getElementById('add-page-q')")
	press(r.X+30, r.Y+r.Height/2, 1, 2)
	out = append(out, "words picked: "+b.eval("getSelection().toString()"))
	press(5, 5, 1)
	until("add-page-dialog", false)
	say("then the backdrop")
	openMove()
	pressAt("#move-no", 1)
	until("move-dialog", false)
	b.still()
	press(x, y, 1)
	until("move-dialog", true)
	say("a row after a close")
	shut()

	// A key held: no repeat reaches what is on top
	for _, c := range []struct {
		name string
		prep func()
		sel  string
		key  string
	}{
		{"add a category, enter", func() { pressAt("#task-edit", 1); until("edit-task-dialog", true) }, "#edit-category-plus", kb.Enter},
		{"add a category, cancel by enter", openAdd, "#edit-add-dialog [data-close]", kb.Enter},
		{"add a category, escape", openAdd, "#edit-add-new", kb.Escape},
		{"move, no by enter", openMove, "#move-no", kb.Enter},
	} {
		c.prep()
		focus(c.sel)
		b.holdKey(c.key, 5)
		say("held: " + c.name)
		out = append(out, "said: "+b.eval("document.getElementById('edit-add-said').textContent"))
		shut()
	}
	// Not dropped: a key of its own after an opening one (the same key, another, Escape), and one held after it
	openAdd()
	focus("#edit-add-new")
	b.pressKey(kb.Enter)
	say("then enter again")
	out = append(out, "said: "+b.eval("document.getElementById('edit-add-said').textContent"))
	shut()
	focus("#task-add-page")
	b.holdKey(kb.Enter, 3)
	b.holdKey(kb.ArrowDown, 4)
	say("then down held")
	out = append(out, "reached: "+b.eval("document.getElementById('add-page-q').getAttribute('aria-activedescendant')"))
	focus("#add-page-q")
	b.pressKey(kb.Escape)
	say("then escape")

	check(t, "dialog twice", out, strings.Join([]string{
		"help, close[1]:  / keys-open / sent 0",
		"help, close[1 2]:  / keys-open / sent 0",
		"settings, close[1]:  / settings-open / sent 0",
		"settings, close[1 2]:  / settings-open / sent 0",
		"add a category, cancel[1]: edit-task-dialog / edit-category-plus / sent 0",
		"add a category, cancel[1 2]: edit-task-dialog / edit-category-plus / sent 0",
		"move, no[1]: add-page-dialog / add-page-q / sent 0",
		"move, no[1 2]: add-page-dialog / add-page-q / sent 0",
		"add a category, add[1]: edit-task-dialog / edit-category-plus / sent 0",
		"add a category, add[1 2]: edit-task-dialog / edit-category-plus / sent 0",
		"add a category, backdrop double: edit-task-dialog / edit-category-plus / sent 0",
		"grips: false",
		"open help: keys / BUTTON / sent 0",
		"open settings: settings / settings-title / sent 0",
		"open add a page: add-page-dialog / add-page-q / sent 0",
		"open edit: edit-task-dialog / edit-task-title / sent 0",
		"row double: move-dialog over add-page-dialog / move-no / sent 0",
		"then Yes:  / task-add-page / sent 1",
		"words picked: words",
		"then the backdrop:  / task-add-page / sent 0",
		"a row after a close: move-dialog over add-page-dialog / move-no / sent 0",
		"held: add a category, enter: edit-add-dialog over edit-task-dialog / edit-add-new / sent 0",
		"said: ",
		"held: add a category, cancel by enter: edit-task-dialog / edit-category-plus / sent 0",
		"said: ",
		"held: add a category, escape: edit-task-dialog / edit-category-plus / sent 0",
		"said: ",
		"held: move, no by enter: add-page-dialog / add-page-q / sent 0",
		"said: ",
		"then enter again: edit-add-dialog over edit-task-dialog / edit-add-new / sent 0",
		"said: Write the category's name",
		"then down held: add-page-dialog / add-page-q / sent 0",
		"reached: add-page-4",
		"then escape:  / task-add-page / sent 0",
	}, "|"))
}

// TestUIDialogTwicePage holds a key in a page's iframe after a modal closed: its page hands the first over, and the repeats, the focus
// having left the iframe, are not dropped as the rest of the key the modal closed by.
func TestUIDialogTwicePage(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	for i := range 6 {
		slug := fmt.Sprintf("d%d", i)
		s.floe(t, "publish", s.page(t, slug+".yaml", slug, keysPage), "--slug", slug)
	}
	b.size(1440, 900, false)
	defer b.unsize()
	b.navigate("about:blank")
	b.addScript(framedCatch)
	out := make([]string, 0, 3)
	for _, how := range []string{"never opened", "by its button", "by Escape"} {
		b.navigate("/")
		b.until("the rows", "document.querySelector('#list li[data-slug=d5] [data-testid=title]')")
		b.eval("document.querySelector('#list li[data-slug=d5] [data-testid=title]').click()")
		b.until("the page beside to be told the keys are taken", "window.ready")
		if how != "never opened" {
			b.eval("document.getElementById('keys-open').click()")
			b.until("the help", "document.getElementById('keys').open")
			if how == "by Escape" {
				b.pressKey(kb.Escape)
			} else {
				b.eval("document.querySelector('#keys [data-close]').click()")
			}
			b.until("the help closed", "!document.getElementById('keys').open")
		}
		focusIn(b, "read-frame")
		at := "document.getElementById('read-frame').dataset.slug"
		// Each down of j, the first in the page and the repeats on the body, as the screen draws the one before
		for i, next := range []string{"d4", "d3", "d2"} {
			b.keyDown("j", i > 0)
			b.until(how+", on to "+next, at+" === "+js(next))
		}
		b.keyUp("j")
		out = append(out, how+": "+b.eval(at))
	}
	check(t, "dialog twice, page", out, "never opened: d2|by its button: d2|by Escape: d2")
}
