package browsertest_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"

	"github.com/sukekyo26/floe/internal/store"
)

// A task's parent from its page: added, a task there or one made with it; changed, with a way back while the one before
// makes a family; taken off, from the child or the parent, by the mouse or the keys, with a way back. What the page saw
// is what it writes: a parent moved meanwhile is refused and read again.
func TestUITaskParent(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	b.navigate("/")
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
	page("p1", "--task", "#1", "--title", "free")
	page("p2", "--task", "#2", "--title", "a child")
	task("#3", "--title", "up")
	task("#2", "--parent", "#3")
	page("p4", "--task", "#4", "--title", "a parent")
	task("#5", "--title", "its child")
	task("#5", "--parent", "#4")
	page("p7", "--task", "#7", "--title", "by hand")
	parentOf := func(name string) string {
		t.Helper()
		all, err := st.ReadTasks()
		if err != nil {
			t.Fatal(err)
		}
		return all[project][name].Parent
	}
	open := func(name string) {
		t.Helper()
		b.navigate("/tasks?project=proj&task=" + strings.ReplaceAll(name, "#", "%23"))
		b.until("the head", "!document.getElementById('task-head').hidden")
		b.still()
	}
	press := func(expr string) {
		t.Helper()
		b.eval(expr + ".scrollIntoView({ block: 'nearest' })")
		b.still()
		r := b.rect(expr)
		b.click(r.X+r.Width/2, r.Y+r.Height/2)
	}
	const shown = "(id) => !document.getElementById(id).closest('[hidden]')"
	parts := func() string {
		t.Helper()
		return b.eval("['task-up-part', 'task-parent-add', 'task-parent-change', 'task-kids-part', 'task-child'].map(" + shown + ").join(' ')")
	}
	kin := func(name, title string) {
		t.Helper()
		b.until("the dialog", "document.getElementById('kin-dialog').open")
		b.eval("document.getElementById('kin-task').value = " + js(name) + "; document.getElementById('kin-task').dispatchEvent(new Event('input', { bubbles: true })); document.getElementById('kin-title').value = " + js(title))
	}
	const said = "[document.getElementById('kin-heading').textContent, document.getElementById('kin-target').textContent, document.getElementById('kin-note').hidden ? '' : document.getElementById('kin-note').textContent, document.getElementById('kin-title').closest('label').hidden, document.getElementById('kin-submit').textContent].join('|')"
	const toasts = "[...document.querySelectorAll('#toasts .toast')].map((e) => e.textContent).join(' / ')"
	saved := func() {
		t.Helper()
		b.eval("document.getElementById('kin').requestSubmit()")
		b.until("saved", "!document.getElementById('kin-dialog').open")
	}

	// The parts each task shows: both for one of no family, the parent's for a child, the children's for a parent.
	for _, c := range []struct{ name, want string }{
		{"#1", "true true false true true"},
		{"#2", "true false true false false"},
		{"#4", "false false false true true"},
	} {
		open(c.name)
		if got := parts(); got != c.want {
			t.Errorf("%s: parts %s, want %s", c.name, got, c.want)
		}
	}
	if got := b.eval("[...document.querySelectorAll('#task-family .unlink')].map((e) => e.dataset.task + ' ' + e.getAttribute('aria-label')).join(', ')"); got != "#5 Take #5 out of the children" {
		t.Errorf("a parent's buttons: %q", got)
	}

	// Added, a task there: said before; the toast leads to it.
	open("#1")
	b.eval("document.getElementById('task-parent-add').click()")
	kin("#3", "")
	if got := b.eval(said); got != "Add a parent task|Child: #1 free|#3 up becomes the parent|true|Make it the parent" {
		t.Errorf("a parent there: %q", got)
	}
	saved()
	if got := b.eval(toasts); got != "Gave it the parent task #3" || parentOf("#1") != "#3" {
		t.Errorf("added: %q, %q", got, parentOf("#1"))
	}
	if got := b.eval("document.activeElement.id"); got != "task-parent-change" {
		t.Errorf("the focus after adding: %q", got)
	}
	b.eval("document.getElementById('toasts').replaceChildren()")

	// The same parent: closed, nothing written. Changed to one made with it: in one write; its way back writes the one before.
	b.until("drawn", "!document.getElementById('task-parent-change').hidden")
	b.eval("document.getElementById('task-parent-change').click()")
	kin("#3", "")
	if got := b.eval(said); got != "Change the parent task|Child: #1 free||true|Make it the parent" {
		t.Errorf("the same parent: %q", got)
	}
	saved()
	b.eval("document.getElementById('task-parent-change').click()")
	kin("#40", "made")
	if got := b.eval(said); got != "Change the parent task|Child: #1 free||false|Make" {
		t.Errorf("a new parent: %q", got)
	}
	saved()
	all, err := st.ReadTasks()
	if err != nil {
		t.Fatal(err)
	}
	if all[project]["#40"].Title != "made" || all[project]["#1"].Parent != "#40" {
		t.Errorf("changed to a new one: %+v %+v", all[project]["#40"], all[project]["#1"])
	}
	if got := b.eval(toasts); got != "Changed the parent from #3 to #40Undo" {
		t.Errorf("changed: %q", got)
	}
	b.eval("document.querySelector('#toasts .undo').click()")
	b.until("given back", "document.querySelector('#toasts').textContent.includes('Undone')")
	b.until("drawn", "document.querySelector('#task-up-rows .kin-row[data-task=\"#3\"]')")
	if parentOf("#1") != "#3" {
		t.Errorf("given back to %q", parentOf("#1"))
	}
	b.eval("document.getElementById('toasts').replaceChildren()")

	// The button keeps its place unseen until the row is pointed at or reached: the row's end, the unread dot before it, does not move.
	b.size(1440, 900, false)
	defer b.unsize()
	open("#1")
	b.mouse(input.DispatchMouseEventTypeMouseMoved, 1, 1, 0)
	const look = "getComputedStyle(document.querySelector('#task-up-rows .unlink')).opacity + ' ' + Math.round(document.querySelector('#task-up-rows .kin-row > :last-child').getBoundingClientRect().right)"
	away := b.eval(look)
	b.eval("document.querySelector('#task-up-rows .unlink').focus()")
	b.still()
	near := b.eval(look)
	if a, n := strings.Fields(away), strings.Fields(near); a[0] != "0" || n[0] != "1" || a[1] != n[1] {
		t.Errorf("the button away %q, reached %q", away, near)
	}

	// Taken off by the mouse: the row gone, the focus on its part's button; the way back writes it again.
	press("document.querySelector('#task-up-rows .unlink')")
	b.until("taken off", "document.querySelector('#toasts').textContent.includes('Took')")
	b.until("drawn", "!document.getElementById('task-parent-add').hidden")
	if got := b.eval(toasts + " + ' | ' + document.activeElement.id"); got != "Took #1 out of the children of #3Undo | task-parent-add" || parentOf("#1") != "" {
		t.Errorf("taken off: %q, %q", got, parentOf("#1"))
	}
	b.eval("document.querySelector('#toasts .undo').click()")
	b.until("given back", "document.querySelector('#task-up-rows .unlink[data-task=\"#3\"]')")
	if got := b.eval("document.activeElement.dataset.task + ' ' + document.activeElement.classList.contains('unlink')"); got != "#3 true" || parentOf("#1") != "#3" {
		t.Errorf("given back: focus %q, parent %q", got, parentOf("#1"))
	}
	b.eval("document.getElementById('toasts').replaceChildren()")

	// By the keys: the focus to the way back at once. Escape before the page is read again leaves it for the button there is, and when that
	// goes with the drawing, for the one that takes its place. The reads after the write are held, to come after the key.
	const toastsLeft = "document.querySelectorAll('#toasts .undo').length + ' ' + document.activeElement.id"
	byKeys := func() {
		t.Helper()
		b.eval("document.querySelector('#task-up-rows .unlink').focus()")
		b.pressKey(kb.Enter)
		b.until("the way back", "document.activeElement.matches('#toasts .undo')")
	}
	b.eval("window.realFetch = window.fetch; window.fetch = (u, o) => (o?.method === 'PATCH' ? window.realFetch(u, o).then((r) => { window.holding = true; return r; }) : window.realFetch(u, o).then((r) => (window.holding ? new Promise((go) => { window.heldAnswers = [...(window.heldAnswers ?? []), () => go(r)]; }) : r)))")
	byKeys()
	b.pressKey(kb.Escape)
	if got := b.eval(toastsLeft); got != "0 task-parent-change" {
		t.Errorf("Escape before the reading: %q", got)
	}
	b.eval("window.fetch = window.realFetch; window.holding = false; (window.heldAnswers ?? []).forEach((go) => go()), 1")
	b.until("left", "document.activeElement.id === 'task-parent-add'")
	if parentOf("#1") != "" {
		t.Errorf("taken off: parent %q", parentOf("#1"))
	}

	// The reading failing: the task is off its parent, so the way back is there and works, and Escape still leaves the focus on a button.
	const unread = "window.realFetch = window.fetch; window.fetch = (u, o) => (o?.method ? window.realFetch(u, o) : Promise.reject(new TypeError('down'))), 1"
	s.floe(t, "work", "--project", "proj", "--task", "#1", "--parent", "#3")
	open("#1")
	b.eval(unread)
	byKeys()
	b.until("the reading failed", "!document.getElementById('error').hidden")
	b.pressKey(kb.Enter)
	b.until("given back", "document.querySelector('#toasts').textContent.includes('Undone')")
	if parentOf("#1") != "#3" {
		t.Errorf("given back with the reading failing: parent %q", parentOf("#1"))
	}
	// The page read again: its row is of the rev the way back wrote, which a page that cannot read does not know, and would send refused
	open("#1")
	b.eval(unread)
	byKeys()
	b.until("the reading failed", "!document.getElementById('error').hidden")
	b.pressKey(kb.Escape)
	if got := b.eval(toastsLeft); got != "0 task-parent-change" {
		t.Errorf("Escape with the reading failed: %q", got)
	}
	b.eval("window.fetch = window.realFetch, 1")
	b.until("read again", "document.activeElement.id === 'task-parent-add'")

	// From the parent's page, a child taken off; the focus on a button stays as the family is drawn anew.
	open("#4")
	b.eval("document.querySelector('#task-family-rows .unlink').focus()")
	task("#5", "--title", "renamed")
	b.until("drawn anew", "document.getElementById('task-family-rows').textContent.includes('renamed')")
	if got := b.eval("document.activeElement.dataset.task + ' ' + document.activeElement.classList.contains('unlink')"); got != "#5 true" {
		t.Errorf("the focus as drawn anew: %q", got)
	}
	press("document.querySelector('#task-family-rows .unlink')")
	b.until("taken off", "document.querySelector('#toasts').textContent.includes('Took')")
	if got := b.eval(toasts); got != "Took #5 out of the children of #4Undo" || parentOf("#5") != "" {
		t.Errorf("a child taken off: %q, %q", got, parentOf("#5"))
	}
	b.eval("document.getElementById('toasts').replaceChildren()")

	// Moved meanwhile, the list read before (held, as a poll would read it): refused as changed, not taken off the other; read again.
	hold := func() {
		t.Helper()
		b.eval(`window.realFetch ??= window.fetch; window.held = false;
			window.fetch = async (url, init) => {
				if (init?.method === 'PATCH') window.held = false;
				if (window.held && String(url).startsWith('/api/tasks') && !init?.method) return new Response(window.heldTasks);
				return window.realFetch(url, init);
			};
			window.realFetch('/api/tasks').then((r) => r.text()).then((x) => { window.heldTasks = x; window.held = true; })`)
		b.until("held", "window.held === true")
	}
	open("#2")
	hold()
	task("#2", "--parent", "#4")
	press("document.querySelector('#task-up-rows .unlink')")
	b.until("refused", "!document.getElementById('error').hidden")
	if got := b.eval("document.getElementById('error').textContent"); got != "Could not save: task #2 has changed since it was read" || parentOf("#2") != "#4" {
		t.Errorf("moved meanwhile: %q, %q", got, parentOf("#2"))
	}
	b.until("read again", "document.querySelector('#task-up-rows .kin-row[data-task=\"#4\"]')")
	b.eval("document.getElementById('error').hidden = true")

	// The dialog too: refused, it takes the task as read again, and the next press goes.
	hold()
	b.eval("document.getElementById('task-parent-change').click()")
	kin("#40", "")
	task("#2", "--parent", "#3")
	b.eval("document.getElementById('kin').requestSubmit()")
	b.until("refused", "!document.getElementById('kin-error').hidden")
	if got := b.eval("document.getElementById('kin-note').textContent"); got != "#40 made becomes the parent, in place of #3" {
		t.Errorf("read again in the dialog: %q", got)
	}
	b.eval("document.getElementById('kin').requestSubmit()")
	b.until("saved", "!document.getElementById('kin-dialog').open")
	if parentOf("#2") != "#40" {
		t.Errorf("pressed again: %q", parentOf("#2"))
	}
	b.eval("window.fetch = window.realFetch; document.getElementById('toasts').replaceChildren()")

	// Pressed twice as it goes: taken off once, nothing refused.
	b.until("drawn", "document.querySelector('#task-up-rows .unlink[data-task=\"#40\"]')")
	b.eval("const u = document.querySelector('#task-up-rows .unlink'); u.click(); u.click()")
	b.until("taken off", "document.querySelector('#toasts').textContent.includes('Took')")
	b.still()
	if got := b.eval("document.getElementById('error').hidden + ' ' + document.querySelectorAll('#toasts .toast').length"); got != "true 1" || parentOf("#2") != "" {
		t.Errorf("pressed twice: %q, %q", got, parentOf("#2"))
	}
	b.eval("document.getElementById('toasts').replaceChildren()")

	// A name that may not be the parent is not said to be: itself, one with a parent.
	task("#5", "--parent", "#4")
	open("#1")
	b.eval("document.getElementById('task-parent-change').click()")
	for _, name := range []string{"#1", "#5"} {
		kin(name, "")
		if got := b.eval("document.getElementById('kin-note').hidden + ''"); got != "true" {
			t.Errorf("%s said: %q", name, b.eval("document.getElementById('kin-note').textContent"))
		}
	}
	b.eval("document.getElementById('kin-dialog').close()")

	// A parent by hand that makes no family: named again, nothing is written; changed, said so, with no way back the store would refuse.
	byHand := func(name, parent string) {
		t.Helper()
		file := filepath.Join(s.Env["FLOE_DIR"], "tasks.json")
		text, err := os.ReadFile(file) //nolint:gosec // G304: a path the test names
		if err != nil {
			t.Fatal(err)
		}
		var raw map[string]map[string]map[string]any
		if err = json.Unmarshal(text, &raw); err != nil {
			t.Fatal(err)
		}
		raw[project][name]["parent"] = parent
		if text, err = json.Marshal(raw); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(file, text, 0o600); err != nil { //nolint:gosec // G703: a path the test names
			t.Fatal(err)
		}
	}
	byHand("#7", "#9")
	open("#7")
	if got := parts() + " " + b.eval("document.getElementById('task-up-rows').children.length + ''"); got != "true false true false false 0" {
		t.Errorf("a parent by hand: %q", got)
	}
	b.eval("document.getElementById('task-parent-change').click()")
	kin("#9", "")
	if got := b.eval(said); got != "Change the parent task|Child: #7 by hand||true|Make it the parent" {
		t.Errorf("the parent by hand named again: %q", got)
	}
	saved()
	if parentOf("#7") != "#9" {
		t.Errorf("named again, written: %q", parentOf("#7"))
	}
	b.eval("document.getElementById('task-parent-change').click()")
	kin("#3", "")
	if got := b.eval(said); got != "Change the parent task|Child: #7 by hand|#3 up becomes the parent, in place of #9|true|Make it the parent" {
		t.Errorf("a parent by hand, changed: %q", got)
	}
	saved()
	if got := b.eval(toasts); got != "Changed the parent from #9 to #3" || parentOf("#7") != "#3" {
		t.Errorf("by hand, changed: %q, %q", got, parentOf("#7"))
	}

	// Set the task: the task sent as it was read (rev); one changed meanwhile is refused, and a step alone saves.
	b.navigate("/?project=proj")
	b.until("the rows", "document.querySelector('#list li[data-slug=p7]')")
	setDialog := func() {
		t.Helper()
		b.eval("document.querySelector('#list li[data-slug=p7] .more').open = true; document.querySelector('#list li[data-slug=p7] .menu .task-set').click()")
		b.until("the dialog", "document.getElementById('set-dialog').open")
	}
	setDialog()
	b.eval("document.getElementById('set-parent').value = '#4'")
	task("#7", "--parent", "")
	b.eval("document.getElementById('set').requestSubmit()")
	b.until("refused", "!document.getElementById('set-error').hidden")
	if got := b.eval("document.getElementById('set-error').textContent"); !strings.Contains(got, "task #7 has changed since it was read") || parentOf("#7") != "" {
		t.Errorf("set, moved meanwhile: %q, %q", got, parentOf("#7"))
	}
	b.eval("document.getElementById('set-dialog').close()")
	// The parent as written, spaces round it, left alone: not sent, nor written again.
	byHand("#7", " #9")
	b.until("read", "tasksRead.tasks.proj['#7'].parent === ' #9'")
	setDialog()
	if got := b.eval("document.getElementById('set-parent').value"); got != " #9" {
		t.Errorf("the parent's field: %q", got)
	}
	b.eval("document.getElementById('set-step').value = 'build'")
	b.eval("document.getElementById('set').requestSubmit()")
	b.until("saved", "!document.getElementById('set-dialog').open")
	if got := st.ReadMeta("p7").Work.Step; got != "build" {
		t.Errorf("a step alone: %q", got)
	}
	if got := parentOf("#7"); got != " #9" {
		t.Errorf("the parent left alone: %q", got)
	}
}

// A task's family stands in a card inside the head, apart from the steps of pages, wide and narrow, by mouse and by touch:
// each row headed by its state and ended by its unread dot; on the children's rows, the states as wide as the widest with
// their words inside, the names in line, and a long name cut at a fifth so that its title shows, both whole in the link's
// title and read name first.
func TestUITaskFamilyCard(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	b.navigate("/")
	const long = "#payment-provider-migration-2026q4"
	const longTitle = "move the payments to the new provider, running both until the old contract ends"
	task := func(args ...string) {
		t.Helper()
		s.floe(t, append([]string{"work", "--project", "proj", "--task"}, args...)...)
	}
	task("#1", "--title", "up")
	task("#2", "--title", "two", "--state", "doing", "--parent", "#1")
	task(long, "--title", longTitle, "--state", "waiting", "--parent", "#1")
	task("#4", "--title", "four", "--parent", "#1")
	touch := func(on bool) {
		t.Helper()
		if _, err := chromedp.Call(b.ctx, emulation.SetTouchEmulationEnabled, emulation.SetTouchEmulationEnabledParams{Enabled: on, MaxTouchPoints: 1}); err != nil {
			t.Fatal(err)
		}
	}
	// The card in the head's box, of its own ground and edge; each row headed and ended; the touch screen shows the buttons.
	const card = `(() => {
  const head = document.getElementById('task-head'), card = document.getElementById('task-family');
  const hs = getComputedStyle(head), cs = getComputedStyle(card);
  const h = head.getBoundingClientRect(), c = card.getBoundingClientRect();
  const inside = c.left >= h.left + parseFloat(hs.paddingLeft) - 0.5 && c.right <= h.right - parseFloat(hs.paddingRight) + 0.5;
  const rows = [...card.querySelectorAll('.kin-row')];
  const ends = rows.every((r) => r.firstElementChild.matches('.kid-state') && r.lastElementChild.matches('.dot, .ring'));
  const buttons = matchMedia('(hover: none)').matches ? 'touch ' + [...card.querySelectorAll('.unlink')].every((u) => getComputedStyle(u).opacity === '1') : 'mouse';
  return [inside, cs.backgroundColor !== hs.backgroundColor && parseFloat(cs.borderTopWidth) > 0, ends, buttons].join(' ');
})()`
	// The children's rows: the states of one width, their words inside; the names in line; the long one cut at a fifth of
	// the rows, its title seen and its row in the card; the link's title and name whole.
	longRow := `(() => {
  const rows = [...document.querySelectorAll('#task-family-rows .kin-row')];
  const states = rows.map((r) => r.querySelector('.kid-state'));
  const wide = new Set(states.map((e) => e.getBoundingClientRect().width.toFixed(1))).size === 1 && states.every((e) => e.scrollWidth <= e.clientWidth);
  const names = new Set(rows.map((r) => r.querySelector('.tname').getBoundingClientRect().left.toFixed(1))).size === 1;
  const row = document.querySelector('#task-family .kin-row[data-task="` + long + `"]'), name = row.querySelector('.tname');
  const cut = name.scrollWidth > name.clientWidth && name.getBoundingClientRect().width <= document.getElementById('task-family-rows').getBoundingClientRect().width / 5 + 0.5;
  return [rows.length, wide, names, cut, row.querySelector('.kin-title').getBoundingClientRect().width > 20,
    row.closest('.kin-line').getBoundingClientRect().right <= document.getElementById('task-family').getBoundingClientRect().right,
    row.title === ` + js(long+"\n"+longTitle) + `, row.getAttribute('aria-label') === ` + js(long+" "+longTitle+", Waiting") + `].join(' ')
})()`
	defer b.unsize()
	out := make([]string, 0, 4)
	for _, width := range []int64{1440, 400} {
		b.size(width, 900, width == 400)
		touch(width == 400)
		for _, name := range []string{"%231", "%232"} {
			b.navigate("/tasks?project=proj&task=" + name)
			b.until("the family", "document.querySelector('#task-family .kin-row')")
			b.still()
			got := b.eval(card)
			if name == "%231" {
				got += " " + b.eval(longRow)
			}
			out = append(out, got)
		}
	}
	touch(false)
	check(t, "the card", out, strings.Join([]string{
		"true true true mouse 3 true true true true true true true", "true true true mouse",
		"true true true touch true 3 true true true true true true true", "true true true touch true",
	}, "|"))
}
