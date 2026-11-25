package browsertest_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"

	"floe/internal/store"
)

// The focus as each close() of a modal returns, read in a microtask: in the same task, before Chrome's close event a frame later. Every
// modal closed, by every way it opens and closes, has it out of the closed modal, where it goes back to or on the body.
const dialogCloseJS = `(() => {
window.closings = [];
window.where = (el) => {
  if (!el || el === document.body) return 'body';
  const d = el.closest('dialog');
  if (d && !d.open) return 'IN CLOSED ' + d.id;
  if (el.closest('[data-testid=toast]')) return 'toast ' + el.textContent.trim();
  const at = el.closest('#list') ? 'list ' : el.closest('#read-actions') ? 'bar ' : el.closest('dialog') ? el.closest('dialog').id + ' ' : '';
  return at + (el.id || el.getAttribute('aria-label') || el.tagName + ' ' + el.textContent.trim());
};
const real = HTMLDialogElement.prototype.close;
HTMLDialogElement.prototype.close = function () {
  const d = this;
  real.call(d);
  queueMicrotask(() => closings.push(d.id + ' > ' + where(document.activeElement)));
};
window.sent = [];
window.realFetch ??= window.fetch;
window.fetch = (u, i) => {
  if (i?.method !== 'PATCH') return window.realFetch(u, i);
  sent.push(new URL(u, location).pathname);
  if (window.failSlug && new URL(u, location).pathname.endsWith('/' + window.failSlug)) return Promise.resolve(new Response('no', { status: 500 }));
  return (window.hold ?? Promise.resolve()).then(() => window.realFetch(u, i));
};
})()`

// pressMenuItemFocused is pressMenuItem with the focus where a person's presses leave it, on the summary and then the item, for the
// tests that read where the focus goes back to.
func pressMenuItemFocused(b *browser, box, item string) {
	b.t.Helper()

	menuItemShown(b, box, item, "s.focus();")
	got := "document.querySelector(" + js(box+" [data-testid=more] [data-testid="+item+"]") + ")"
	b.eval("(() => { const i = " + got + "; i.focus(); i.click(); })()")
}

func TestUIDialogClose(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	for _, p := range []string{"plan-a", "check-a", "memo-a"} {
		s.floe(t, "publish", s.page(t, p+".yaml", "T "+p, ""), "--slug", p)
		project := "proj"
		if _, err := st.Edit(p, store.Edit{Project: &project}); err != nil {
			t.Fatal(err)
		}
	}
	onTask := func(slug, task, category string) {
		t.Helper()
		s.floe(t, "task", "--slug", slug, "--task", task, "--category", category)
	}
	// plan-a put back on S300 outside, and drawn so
	onAgain := func() {
		t.Helper()
		onTask("plan-a", "S300", "案")
		b.reread()
		b.until("on again", "document.querySelector('#list li[data-slug=plan-a] [data-testid=task-tag]')?.getAttribute('href')?.includes('task=S300')")
	}
	onTask("plan-a", "S300", "案")
	onTask("check-a", "S301", "x")
	s.floe(t, "task", "--project", "proj", "--task", "S300", "--categories", "案", "--categories", "実装")
	b.size(1400, 900, false)
	defer b.unsize()

	var out []string
	open := func(path string) {
		t.Helper()
		b.navigate(path)
		b.until("the rows", "document.querySelector('#list li[data-slug]')")
		b.eval(dialogCloseJS)
	}
	// What the closes of one case left, and the writes it sent
	say := func(name string) {
		t.Helper()
		out = append(out, name+": "+b.eval("closings.join(', ') + (sent.length ? ' / sent ' + sent.length : '')"))
		b.eval("closings.length = 0; sent.length = 0")
	}
	until := func(id string, opened bool) {
		t.Helper()
		not := "!"
		if opened {
			not = ""
		}
		b.until(id, not+"document.getElementById("+js(id)+").open")
	}
	menuItem := func(box, item string) {
		t.Helper()
		pressMenuItemFocused(b, box, item)
	}
	rowMenu := func(slug, item string) func() {
		return func() {
			t.Helper()
			menuItem("#list li[data-slug="+slug+"]", item)
		}
	}
	// The search modal opened from the body (or a row's title, which it gives the focus back to as it closes), a row found and its menu's
	// item pressed
	fromFindAt := func(item, focus string) func() {
		return func() {
			t.Helper()
			b.eval("(() => { document.activeElement.blur(); " + focus + "; document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'k', ctrlKey: true })); })()")
			until("find", true)
			b.fill("#find-q", "plan")
			b.until("the found row's actions", "!document.getElementById('find-acts').hidden")
			menuItem("#find-actions", item)
		}
	}
	fromFind := func(item string) func() { return fromFindAt(item, "") }
	const rowTitle = "document.querySelector('#list li[data-slug=memo-a] [data-testid=title]').focus()"
	press := func(expr string) {
		t.Helper()
		r := b.rect(expr)
		b.click(r.X+r.Width/2, r.Y+r.Height/2)
	}
	run := func(expr string) func() {
		return func() {
			t.Helper()
			b.eval(expr)
		}
	}
	cancel := func(id string) func() {
		return run("document.querySelector('#" + id + " [data-close]').click()")
	}
	backdrop := func(id string) func() {
		return run("(() => { const d = document.getElementById(" + js(id) + "); d.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true })); d.dispatchEvent(new MouseEvent('click', { bubbles: true })); })()")
	}
	escape := func() {
		t.Helper()
		b.pressKey(kb.Escape)
	}
	// Words typed in field, then the button submit pressed
	save := func(field, words, submit string) func() {
		return func() {
			t.Helper()
			b.fill(field, words)
			b.eval("document.getElementById(" + js(submit) + ").click()")
		}
	}
	enter := func() {
		t.Helper()
		for _, typ := range []input.DispatchKeyEventType{input.DispatchKeyEventTypeKeyDown, input.DispatchKeyEventTypeKeyUp} {
			text := ""
			if typ == input.DispatchKeyEventTypeKeyDown {
				text = "\r"
			}
			if _, err := chromedp.Call(b.ctx, input.DispatchKeyEvent, input.DispatchKeyEventParams{Type: typ, Key: "Enter", Code: "Enter", Text: text, WindowsVirtualKeyCode: 13, NativeVirtualKeyCode: 13}); err != nil {
				t.Fatal(err)
			}
		}
	}
	tags := 0
	newTag := func() {
		t.Helper()
		tags++
		b.fill("#tag-find", "n"+strconv.Itoa(tags))
		applyNewTag(b)
	}

	// Tags: a row's menu, by each way of closing
	open("/")
	for _, c := range []struct {
		name  string
		close func()
	}{
		{"apply", newTag},
		{"cancel", cancel("tag-dialog")},
		{"backdrop", backdrop("tag-dialog")},
		{"escape", escape},
	} {
		rowMenu("plan-a", "tag-item")()
		until("tag-dialog", true)
		c.close()
		until("tag-dialog", false)
		say("tag, row, " + c.name)
	}
	// The tag line's button; the selection's, all applied (the bar goes with it), one failed (it stays) and Escape; from the search modal
	b.eval("document.querySelector('#list li[data-slug=plan-a] [data-testid=title]').click()")
	b.until("picked", "!document.getElementById('read-tags').hidden")
	for _, how := range []func(){newTag, escape} {
		b.eval("document.querySelector('#read-tags [data-testid=tag-edit]').focus(); document.querySelector('#read-tags [data-testid=tag-edit]').click()")
		until("tag-dialog", true)
		how()
		until("tag-dialog", false)
		say("tag, tag line")
	}
	bulk := func(slugs ...string) {
		t.Helper()
		for _, slug := range slugs {
			b.eval("document.querySelector('#list li[data-slug=" + slug + "] [data-testid=sel]').click()")
		}
		b.eval("document.getElementById('bulk-tag').focus(); document.getElementById('bulk-tag').click()")
		until("tag-dialog", true)
	}
	bulk("memo-a")
	b.pressKey(kb.Escape)
	until("tag-dialog", false)
	say("tag, bulk, escape")
	bulk()
	newTag()
	until("tag-dialog", false)
	say("tag, bulk, all applied")
	b.eval("window.failSlug = 'check-a'")
	bulk("memo-a", "check-a")
	newTag()
	until("tag-dialog", false)
	b.eval("window.failSlug = null; document.getElementById('bulk-clear').click()")
	say("tag, bulk, one failed")
	fromFind("tag-item")()
	until("tag-dialog", true)
	b.pressKey(kb.Escape)
	until("tag-dialog", false)
	say("tag, search, escape")
	// Held open: while the tags are sent (closedby none), and while Escape clears the find field's words
	b.eval("window.hold = new Promise((go) => { window.letGo = go; }); true")
	rowMenu("plan-a", "tag-item")()
	until("tag-dialog", true)
	newTag()
	b.until("sending", "document.getElementById('tag-dialog').getAttribute('closedby') === 'none'")
	b.pressKey(kb.Escape)
	out = append(out, "tag, escape while sent: "+b.eval("document.getElementById('tag-dialog').open + ' ' + closings.length"))
	b.eval("window.hold = null; window.letGo()")
	until("tag-dialog", false)
	say("tag, sent")
	rowMenu("plan-a", "tag-item")()
	until("tag-dialog", true)
	b.fill("#tag-find", "abc")
	b.pressKey(kb.Escape)
	out = append(out, "tag, escape with words: "+b.eval("document.getElementById('tag-dialog').open + ' ' + JSON.stringify(document.getElementById('tag-find').value) + ' ' + closings.length"))
	b.pressKey(kb.Escape)
	until("tag-dialog", false)
	say("tag, escape again")

	// Edit: a row's menu, the bar's, the heading pressed, the search modal, a row gone
	for _, c := range []struct {
		name  string
		close func()
	}{
		{"save", save("#edit-name", "T plan-a 2", "edit-submit")},
		{"cancel", cancel("edit-dialog")},
		{"backdrop", backdrop("edit-dialog")},
		{"escape", escape},
	} {
		rowMenu("plan-a", "edit")()
		until("edit-dialog", true)
		c.close()
		until("edit-dialog", false)
		say("edit, row, " + c.name)
	}
	for _, c := range []struct {
		name  string
		close func()
	}{
		{"save", save("#edit-desc", "From the bar", "edit-submit")},
		{"escape", escape},
	} {
		menuItem("#read-actions", "edit")
		until("edit-dialog", true)
		c.close()
		until("edit-dialog", false)
		say("edit, bar, " + c.name)
	}
	rowMenu("plan-a", "edit")()
	until("edit-dialog", true)
	press("document.getElementById('edit-title')")
	b.pressKey(kb.Escape)
	until("edit-dialog", false)
	say("edit, heading pressed, escape")
	for _, c := range []struct {
		name         string
		close, again func()
	}{
		{"save", save("#edit-name", "T plan-a 3", "edit-submit"), save("#edit-name", "T plan-a 4", "edit-submit")},
		{"escape", escape, escape},
	} {
		fromFind("edit")()
		until("edit-dialog", true)
		c.close()
		until("edit-dialog", false)
		say("edit, search, " + c.name)
		fromFindAt("edit", rowTitle)()
		until("edit-dialog", true)
		c.again()
		until("edit-dialog", false)
		say("edit, search from a row, " + c.name)
	}
	rowMenu("plan-a", "edit")()
	until("edit-dialog", true)
	yes, no := true, false
	if _, err := st.Edit("plan-a", store.Edit{Archived: &yes}); err != nil {
		t.Fatal(err)
	}
	b.reread()
	b.until("the row gone", "!document.querySelector('#list li[data-slug=plan-a]')")
	b.pressKey(kb.Escape)
	until("edit-dialog", false)
	say("edit, row gone, escape")
	if _, err := st.Edit("plan-a", store.Edit{Archived: &no}); err != nil {
		t.Fatal(err)
	}
	// Escape and Enter at once: the Enter goes where the focus went back to, and saves nothing
	b.reread()
	b.until("the row back", "document.querySelector('#list li[data-slug=plan-a]')")
	for _, from := range []func(){rowMenu("plan-a", "edit"), fromFind("edit")} {
		from()
		until("edit-dialog", true)
		b.fill("#edit-name", "Escaped")
		if _, err := chromedp.Call(b.ctx, input.DispatchKeyEvent, input.DispatchKeyEventParams{Type: input.DispatchKeyEventTypeKeyDown, Key: "Escape", Code: "Escape", WindowsVirtualKeyCode: 27, NativeVirtualKeyCode: 27}); err != nil {
			t.Fatal(err)
		}
		enter()
		if _, err := chromedp.Call(b.ctx, input.DispatchKeyEvent, input.DispatchKeyEventParams{Type: input.DispatchKeyEventTypeKeyUp, Key: "Escape", Code: "Escape", WindowsVirtualKeyCode: 27, NativeVirtualKeyCode: 27}); err != nil {
			t.Fatal(err)
		}
		until("edit-dialog", false)
		b.eval("document.querySelector('[data-testid=more][open] > summary')?.click()")
		say("edit, escape and enter")
	}
	out = append(out, "edit, title kept: "+st.ReadMeta("plan-a").Title)

	// Set the task: a row's menu by each way of closing, off its task from the keys and the pointer, the search modal
	open("/")
	for _, c := range []struct {
		name  string
		close func()
	}{
		{"save", save("#set-category", "実装", "set-submit")},
		{"cancel", cancel("set-dialog")},
		{"backdrop", backdrop("set-dialog")},
		{"escape", escape},
	} {
		rowMenu("plan-a", "task-set")()
		until("set-dialog", true)
		c.close()
		until("set-dialog", false)
		say("set, row, " + c.name)
	}
	// Off from the keys (a click of detail 0) and the pointer (detail 1)
	for _, detail := range []string{"0", "1"} {
		rowMenu("plan-a", "task-set")()
		until("set-dialog", true)
		if detail == "0" {
			b.eval("document.getElementById('set-out').focus()")
			b.pressKey(kb.Enter)
		} else {
			press("document.getElementById('set-out')")
		}
		until("set-dialog", false)
		say("set, off, detail " + detail)
		onAgain()
	}
	// Off from the keys, Escape while it goes: held, it stays open; answered, the focus is on the way back
	b.eval("window.hold = new Promise((go) => { window.letGo = go; }); true")
	rowMenu("plan-a", "task-set")()
	until("set-dialog", true)
	b.eval("document.getElementById('set-out').focus()")
	b.pressKey(kb.Enter)
	b.until("sent", "sent.length === 1")
	b.pressKey(kb.Escape)
	out = append(out, "set, off, escape while held: "+b.eval("document.getElementById('set-dialog').open"))
	b.eval("window.hold = null; window.letGo(); true")
	until("set-dialog", false)
	out = append(out, "set, off, answered: "+b.eval("where(document.activeElement)"))
	say("set, off, held")
	onAgain()
	fromFind("task-set")()
	until("set-dialog", true)
	b.pressKey(kb.Escape)
	until("set-dialog", false)
	say("set, search, escape")

	// The ask to move a page off another task, over Set the task: saved by a press as a person does, as Chrome closes on one Escape the
	// modals opened with no press between. Yes answers in its task, the save going on at once
	toMove := func() {
		t.Helper()
		rowMenu("plan-a", "task-set")()
		until("set-dialog", true)
		b.fill("#set-task", "S301")
		b.fill("#set-category", "x")
		b.still()
		press("document.getElementById('set-submit')")
		until("move-dialog", true)
	}
	toMove()
	out = append(out, "move, yes, at once: "+b.eval("(async () => { document.getElementById('move-yes').click(); for (let i = 0; i < 5; i++) await null; return document.getElementById('set-submit').matches(':disabled') + ' ' + sent.length; })()"))
	until("set-dialog", false)
	say("move, yes")
	onAgain()
	toMove()
	b.eval("document.getElementById('move-yes').click()")
	enter()
	until("set-dialog", false)
	say("move, yes and enter")
	out = append(out, "move, written: "+st.ReadMeta("plan-a").Task.Name)
	onAgain()
	for _, c := range []struct {
		name  string
		close func()
	}{
		{"no", cancel("move-dialog")},
		{"escape", escape},
	} {
		toMove()
		c.close()
		until("move-dialog", false)
		say("move, " + c.name)
		out = append(out, "move, "+c.name+", set open: "+b.eval("document.getElementById('set-dialog').open"))
		cancel("set-dialog")()
		until("set-dialog", false)
		b.eval("closings.length = 0")
	}

	// Add a page, on a task's page: picked by Enter or a press, cancel, backdrop, Escape in its field; and a page of another task, asked
	open("/tasks?project=proj&task=S300")
	addTo := func(category string) {
		t.Helper()
		b.eval("const a = document.getElementById('task-add-page'); a.focus(); a.click()")
		until("add-page-dialog", true)
		chooseValue(b, "add-page-category", category)
	}
	find := func(words string) {
		t.Helper()
		b.fill("#add-page-q", words)
	}
	addTo("実装")
	find("memo")
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.Enter)
	until("add-page-dialog", false)
	say("add page, enter")
	addTo("実装")
	find("check")
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.Enter)
	until("move-dialog", true)
	b.eval("document.getElementById('move-yes').click()")
	enter()
	until("add-page-dialog", false)
	say("add page, moved, yes and enter")
	for _, c := range []struct {
		name  string
		close func()
	}{
		{"cancel", cancel("add-page-dialog")},
		{"backdrop", backdrop("add-page-dialog")},
		{"escape", escape},
	} {
		addTo("案")
		c.close()
		until("add-page-dialog", false)
		say("add page, " + c.name)
	}
	// The modal over it that adds a category: closed, it alone, the focus on the pages once one is added, else on its button
	for _, c := range []struct {
		name  string
		close func()
	}{
		{"cancel", cancel("add-category-dialog")},
		{"backdrop", backdrop("add-category-dialog")},
		{"escape", escape},
		{"enter", func() {
			t.Helper()
			b.fill("#add-category-new", "新")
			b.pressKey(kb.Enter)
		}},
	} {
		addTo("案")
		b.eval("document.getElementById('add-category-plus').focus()")
		b.pressKey(kb.Enter)
		until("add-category-dialog", true)
		c.close()
		until("add-category-dialog", false)
		out = append(out, "add category, "+c.name+", add page open: "+b.eval("document.getElementById('add-page-dialog').open"))
		say("add category, " + c.name)
		cancel("add-page-dialog")()
		until("add-page-dialog", false)
		b.eval("closings.length = 0")
	}
	// Closed from outside, the modal under it: both, and opened again without it
	addTo("案")
	b.eval("document.getElementById('add-category-plus').focus()")
	b.pressKey(kb.Enter)
	until("add-category-dialog", true)
	b.eval("document.getElementById('add-page-dialog').close()")
	until("add-category-dialog", false)
	addTo("案")
	out = append(out, "add category, closed from outside: "+b.eval("document.getElementById('add-category-dialog').open"))
	cancel("add-page-dialog")()
	until("add-page-dialog", false)
	b.eval("closings.length = 0")
	// The task page's own modals, which the browser gives the focus back from
	b.eval("document.getElementById('task-edit').focus(); document.getElementById('task-edit').click()")
	until("edit-task-dialog", true)
	b.pressKey(kb.Escape)
	until("edit-task-dialog", false)
	say("edit task, escape")
	// The add modal over the edit modal: Escape and the backdrop close it alone, the focus back on the button that opened it
	b.eval("document.getElementById('task-edit').click()")
	until("edit-task-dialog", true)
	for _, c := range []struct {
		name  string
		close func()
	}{
		{"escape", escape},
		{"backdrop", backdrop("edit-add-dialog")},
	} {
		b.eval("document.getElementById('edit-url-plus').focus(); document.getElementById('edit-url-plus').click()")
		until("edit-add-dialog", true)
		c.close()
		until("edit-add-dialog", false)
		out = append(out, "edit add, "+c.name+", edit open: "+b.eval("document.getElementById('edit-task-dialog').open"))
		say("edit add, " + c.name)
	}
	b.pressKey(kb.Escape)
	until("edit-task-dialog", false)
	b.eval("closings.length = 0")
	b.eval("document.getElementById('task-child').focus(); document.getElementById('task-child').click()")
	until("kin-dialog", true)
	b.pressKey(kb.Escape)
	until("kin-dialog", false)
	say("kin, escape")
	b.eval("document.getElementById('task-child').focus(); document.getElementById('task-child').click()")
	until("kin-dialog", true)
	save("#kin-task", "S301", "kin-submit")()
	until("kin-dialog", false)
	say("kin, save")

	// Settings: a page's theme from its menu, by each way of closing; the bar's button
	open("/")
	for _, c := range []struct {
		name  string
		close func()
	}{
		{"save", run("(() => { const card = [...document.querySelectorAll('#theme-grid input')].find((i) => !i.checked && i.value); card.click(); document.getElementById('theme-choose').click(); })()")},
		{"cancel", cancel("settings")},
		{"backdrop", backdrop("settings")},
		{"escape", escape},
	} {
		rowMenu("plan-a", "theme-item")()
		until("settings", true)
		b.until("the cards", "document.querySelectorAll('#theme-grid input').length > 1")
		c.close()
		until("settings", false)
		say("settings, page, " + c.name)
	}
	b.eval("document.getElementById('settings-open').focus(); document.getElementById('settings-open').click()")
	until("settings", true)
	b.pressKey(kb.Escape)
	until("settings", false)
	say("settings, bar, escape")
	// From the search's menu, on the list and on a task's page: the search closed first, so that the settings open
	for _, at := range []string{"/", "/tasks?project=proj&task=S300"} {
		open(at)
		fromFind("theme-item")()
		until("settings", true)
		b.pressKey(kb.Escape)
		until("settings", false)
		say("settings, find, escape, " + at)
	}
	// Opened on a task's page, by the bar's button, a row's "…" and the search's "…": the focus on the settings' title, not
	// on a way back drawn for a moment. Each page's themes after the bar's, which opens on no page
	bar := run("document.getElementById('settings-open').click()")
	for _, c := range []struct {
		name   string
		opener func()
	}{
		{"bar", bar},
		{"row", rowMenu("plan-a", "theme-item")},
		{"bar", bar},
		{"find", fromFind("theme-item")},
	} {
		c.opener()
		until("settings", true)
		b.still()
		out = append(out, "settings opened, "+c.name+": "+b.eval("document.activeElement.id"))
		b.pressKey(kb.Escape)
		until("settings", false)
		b.eval("closings.length = 0; sent.length = 0")
	}
	// A task's page opens and closes as the list does: Ctrl+K the search and again shut, ? the help, and a row's "…" shut
	// by a press outside it and by Escape
	keyDown := func(key string, ctrl bool) {
		t.Helper()
		b.eval(fmt.Sprintf("document.body.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: %s, ctrlKey: %t }))", js(key), ctrl))
	}
	keysMenus := make([]string, 0, 4)
	b.eval("document.activeElement.blur()")
	keyDown("k", true)
	until("find", true)
	keyDown("k", true)
	until("find", false)
	keysMenus = append(keysMenus, "ctrl+k")
	b.eval("document.activeElement.blur()")
	keyDown("?", false)
	until("keys", true)
	escape()
	until("keys", false)
	keysMenus = append(keysMenus, "?")
	const more = "document.querySelector('#list li[data-slug=plan-a] [data-testid=more]')"
	for _, c := range []struct {
		name  string
		close func()
	}{
		{"a press outside", func() { press("document.querySelector('[data-testid=bar-inner]')") }},
		{"escape", escape},
	} {
		// The menu is the screen's open one as its toggle, which comes after the press, is told
		b.eval("window.menuToggled = false; " + more + ".addEventListener('toggle', () => { window.menuToggled = true; }, { once: true }); " +
			more + ".querySelector('summary').focus(); " + more + ".querySelector('summary').click()")
		b.until("the menu open", more+".open && window.menuToggled")
		c.close()
		b.until("the menu shut by "+c.name, "!"+more+".open")
		keysMenus = append(keysMenus, "menu, "+c.name)
	}
	b.eval("closings.length = 0; sent.length = 0")
	check(t, "a task's page", keysMenus, "ctrl+k|?|menu, a press outside|menu, escape")
	open("/")

	// The search modal opened from the body, a row reached: Escape leaves no field to take the Enter after it
	b.eval("document.activeElement.blur(); document.body.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'k', ctrlKey: true }))")
	until("find", true)
	b.fill("#find-q", "plan")
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.Escape)
	until("find", false)
	enter()
	say("search, escape")
	out = append(out, "search, still: "+b.eval("location.pathname"))
	// The rest, from their buttons
	for _, c := range [][2]string{{"keys", "keys-open"}, {"add-dialog", "add-link"}} {
		b.eval("document.getElementById(" + js(c[1]) + ").focus(); document.getElementById(" + js(c[1]) + ").click()")
		until(c[0], true)
		b.pressKey(kb.Escape)
		until(c[0], false)
		say(c[0] + ", escape")
	}
	b.size(400, 800, false)
	b.eval("document.getElementById('narrow-filter').focus(); document.getElementById('narrow-filter').click()")
	until("filter-dialog", true)
	b.pressKey(kb.Escape)
	until("filter-dialog", false)
	say("filter, escape")
	b.eval("document.getElementById('narrow-filter').focus(); document.getElementById('narrow-filter').click()")
	until("filter-dialog", true)
	b.eval("document.getElementById('add-link').focus(); document.getElementById('add-link').click()")
	until("add-dialog", true)
	// The add modal opened has the focus; it is drawn open in a microtask after the press, so the close of the filters reads the
	// focus still on the filters' button there, as no person can see
	b.until("the focus in the add modal", "document.activeElement?.id === 'add-path'")
	b.pressKey(kb.Escape)
	until("add-dialog", false)
	say("filter, a button of the shelf, then escape")

	check(t, "closed", out, strings.Join(dialogCloseWant, "|"))
}

var dialogCloseWant = []string{
	"tag, row, apply: tag-dialog > list More for T plan-a / sent 1",
	"tag, row, cancel: tag-dialog > list More for T plan-a",
	"tag, row, backdrop: tag-dialog > list More for T plan-a",
	"tag, row, escape: tag-dialog > list More for T plan-a",
	"tag, tag line: tag-dialog > Change the tags of T plan-a / sent 1",
	"tag, tag line: tag-dialog > Change the tags of T plan-a",
	"tag, bulk, escape: tag-dialog > bulk-tag",
	"tag, bulk, all applied: tag-dialog > body / sent 1",
	"tag, bulk, one failed: tag-dialog > bulk-tag / sent 2",
	"tag, search, escape: find > tag-dialog tag-find, tag-dialog > body",
	"tag, escape while sent: true 0",
	"tag, sent: tag-dialog > list More for T plan-a / sent 1",
	"tag, escape with words: true \"\" 0",
	"tag, escape again: tag-dialog > list More for T plan-a",
	"edit, row, save: edit-dialog > list More for T plan-a 2 / sent 1",
	"edit, row, cancel: edit-dialog > list More for T plan-a 2",
	"edit, row, backdrop: edit-dialog > list More for T plan-a 2",
	"edit, row, escape: edit-dialog > list More for T plan-a 2",
	"edit, bar, save: edit-dialog > bar More for T plan-a 2 / sent 1",
	"edit, bar, escape: edit-dialog > bar More for T plan-a 2",
	"edit, heading pressed, escape: edit-dialog > list More for T plan-a 2",
	"edit, search, save: find > edit-dialog edit-name, edit-dialog > body / sent 1",
	"edit, search from a row, save: find > edit-dialog edit-name, edit-dialog > list A T memo-a / sent 1",
	"edit, search, escape: find > edit-dialog edit-name, edit-dialog > body",
	"edit, search from a row, escape: find > edit-dialog edit-name, edit-dialog > list A T memo-a",
	"edit, row gone, escape: edit-dialog > body",
	"edit, escape and enter: edit-dialog > list More for T plan-a 4",
	"edit, escape and enter: find > edit-dialog edit-name, edit-dialog > body",
	"edit, title kept: T plan-a 4",
	"set, row, save: set-dialog > list More for T plan-a 4 / sent 1",
	"set, row, cancel: set-dialog > list More for T plan-a 4",
	"set, row, backdrop: set-dialog > list More for T plan-a 4",
	"set, row, escape: set-dialog > list More for T plan-a 4",
	"set, off, detail 0: set-dialog > toast Undo / sent 1",
	"set, off, detail 1: set-dialog > list More for T plan-a 4 / sent 1",
	"set, off, escape while held: true",
	"set, off, answered: toast Undo",
	"set, off, held: set-dialog > toast Undo / sent 1",
	"set, search, escape: find > set-dialog set-task, set-dialog > body",
	"move, yes, at once: true 1",
	"move, yes: move-dialog > set-dialog set-submit, set-dialog > list More for T plan-a 4 / sent 1",
	"move, yes and enter: move-dialog > set-dialog set-submit, set-dialog > list More for T plan-a 4 / sent 1",
	"move, written: S301",
	"move, no: move-dialog > set-dialog set-submit",
	"move, no, set open: true",
	"move, escape: move-dialog > set-dialog set-submit",
	"move, escape, set open: true",
	"add page, enter: add-page-dialog > task-add-page / sent 1",
	"add page, moved, yes and enter: move-dialog > add-page-dialog add-page-q, add-page-dialog > task-add-page / sent 1",
	"add page, cancel: add-page-dialog > task-add-page",
	"add page, backdrop: add-page-dialog > task-add-page",
	"add page, escape: add-page-dialog > task-add-page",
	"add category, cancel, add page open: true",
	"add category, cancel: add-category-dialog > add-page-dialog add-category-plus",
	"add category, backdrop, add page open: true",
	"add category, backdrop: add-category-dialog > add-page-dialog add-category-plus",
	"add category, escape, add page open: true",
	"add category, escape: add-category-dialog > add-page-dialog add-category-plus",
	"add category, enter, add page open: true",
	"add category, enter: add-category-dialog > add-page-dialog add-page-q",
	"add category, closed from outside: false",
	"edit task, escape: edit-task-dialog > task-edit",
	"edit add, escape, edit open: true",
	"edit add, escape: edit-add-dialog > edit-task-dialog edit-url-plus",
	"edit add, backdrop, edit open: true",
	"edit add, backdrop: edit-add-dialog > edit-task-dialog edit-url-plus",
	"kin, escape: kin-dialog > task-child",
	"kin, save: kin-dialog > task-child / sent 1",
	"settings, page, save: settings > list More for T plan-a 4",
	"settings, page, cancel: settings > list More for T plan-a 4",
	"settings, page, backdrop: settings > list More for T plan-a 4",
	"settings, page, escape: settings > list More for T plan-a 4",
	"settings, bar, escape: settings > settings-open",
	"settings, find, escape, /: find > settings settings-title, settings > body",
	"settings, find, escape, /tasks?project=proj&task=S300: find > settings settings-title, settings > body",
	"settings opened, bar: settings-title",
	"settings opened, row: settings-title",
	"settings opened, bar: settings-title",
	"settings opened, find: settings-title",
	"search, escape: find > body",
	"search, still: /",
	"keys, escape: keys > keys-open",
	"add-dialog, escape: add-dialog > add-link",
	"filter, escape: filter-dialog > narrow-filter",
	"filter, a button of the shelf, then escape: filter-dialog > narrow-filter, add-dialog > narrow-filter",
}

// applyNewTag marks the tag typed in #tag-find as new and sends it.
func applyNewTag(b *browser) {
	b.t.Helper()

	b.pressed("document.getElementById('tag-new').click()", "the new tag marked", "!document.getElementById('tag-apply').disabled")
	b.eval("document.getElementById('tag-apply').click()")
}
