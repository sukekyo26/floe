package browsertest_test

import (
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
  if (el.closest('.toast')) return 'toast ' + el.textContent.trim();
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
	rowMenu := func(slug, item string) string {
		return "(() => { const m = document.querySelector('#list li[data-slug=" + slug + "] .more'); m.querySelector('summary').focus(); m.open = true; const i = m.querySelector('.menu ." + item + "'); i.focus(); i.click(); })()"
	}
	// The search modal opened from the body (or a row's title, which it gives the focus back to as it closes), a row found and its menu's
	// item pressed
	fromFindAt := func(item, focus string) string {
		return "(() => { document.activeElement.blur(); " + focus + "; document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'k', ctrlKey: true })); const q = document.getElementById('find-q'); q.value = 'plan'; q.dispatchEvent(new Event('input')); document.querySelector('#find-actions .more summary').click(); const i = document.querySelector('#find-actions .more .menu ." + item + "'); i.focus(); i.click(); })()"
	}
	fromFind := func(item string) string { return fromFindAt(item, "") }
	const rowTitle = "document.querySelector('#list li[data-slug=memo-a] .title').focus()"
	press := func(expr string) {
		t.Helper()
		r := b.rect(expr)
		b.click(r.X+r.Width/2, r.Y+r.Height/2)
	}
	cancel := func(id string) string {
		return "document.querySelector('#" + id + " [data-close]').click()"
	}
	backdrop := func(id string) string {
		return "document.getElementById(" + js(id) + ").dispatchEvent(new MouseEvent('click', { bubbles: true }))"
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
	newTag := "(() => { const f = document.getElementById('tag-find'); f.value = 'n' + Math.random().toString(36).slice(2, 7); f.dispatchEvent(new Event('input')); document.getElementById('tag-new').click(); document.getElementById('tag-apply').click(); })()"

	// Tags: a row's menu, by each way of closing
	open("/")
	for _, c := range []struct{ name, close string }{
		{"apply", newTag},
		{"cancel", cancel("tag-dialog")},
		{"backdrop", backdrop("tag-dialog")},
		{"escape", ""},
	} {
		b.eval(rowMenu("plan-a", "tag-item"))
		until("tag-dialog", true)
		if c.close == "" {
			b.pressKey(kb.Escape)
		} else {
			b.eval(c.close)
		}
		until("tag-dialog", false)
		say("tag, row, " + c.name)
	}
	// The tag line's button; the selection's, all applied (the bar goes with it), one failed (it stays) and Escape; from the search modal
	b.eval("document.querySelector('#list li[data-slug=plan-a] .title').click()")
	b.until("picked", "!document.getElementById('read-tags').hidden")
	for _, close := range []string{newTag, ""} {
		b.eval("document.querySelector('#read-tags .tag-edit').focus(); document.querySelector('#read-tags .tag-edit').click()")
		until("tag-dialog", true)
		if close == "" {
			b.pressKey(kb.Escape)
		} else {
			b.eval(close)
		}
		until("tag-dialog", false)
		say("tag, tag line")
	}
	bulk := func(slugs ...string) {
		t.Helper()
		for _, slug := range slugs {
			b.eval("document.querySelector('#list li[data-slug=" + slug + "] .sel').click()")
		}
		b.eval("document.getElementById('bulk-tag').focus(); document.getElementById('bulk-tag').click()")
		until("tag-dialog", true)
	}
	bulk("memo-a")
	b.pressKey(kb.Escape)
	until("tag-dialog", false)
	say("tag, bulk, escape")
	bulk()
	b.eval(newTag)
	until("tag-dialog", false)
	say("tag, bulk, all applied")
	b.eval("window.failSlug = 'check-a'")
	bulk("memo-a", "check-a")
	b.eval(newTag)
	until("tag-dialog", false)
	b.eval("window.failSlug = null; document.getElementById('bulk-clear').click()")
	say("tag, bulk, one failed")
	b.eval(fromFind("tag-item"))
	until("tag-dialog", true)
	b.pressKey(kb.Escape)
	until("tag-dialog", false)
	say("tag, search, escape")
	// Held open: while the tags are sent (closedby none), and while Escape clears the find field's words
	b.eval("window.hold = new Promise((go) => { window.letGo = go; }); true")
	b.eval(rowMenu("plan-a", "tag-item"))
	until("tag-dialog", true)
	b.eval(newTag)
	b.until("sending", "document.getElementById('tag-dialog').getAttribute('closedby') === 'none'")
	b.pressKey(kb.Escape)
	out = append(out, "tag, escape while sent: "+b.eval("document.getElementById('tag-dialog').open + ' ' + closings.length"))
	b.eval("window.hold = null; window.letGo()")
	until("tag-dialog", false)
	say("tag, sent")
	b.eval(rowMenu("plan-a", "tag-item"))
	until("tag-dialog", true)
	b.eval("const f = document.getElementById('tag-find'); f.focus(); f.value = 'abc'; f.dispatchEvent(new Event('input'))")
	b.pressKey(kb.Escape)
	out = append(out, "tag, escape with words: "+b.eval("document.getElementById('tag-dialog').open + ' ' + JSON.stringify(document.getElementById('tag-find').value) + ' ' + closings.length"))
	b.pressKey(kb.Escape)
	until("tag-dialog", false)
	say("tag, escape again")

	// Edit: a row's menu, the bar's, the heading pressed, the search modal, a row gone
	for _, c := range []struct{ name, close string }{
		{"save", "document.getElementById('edit-name').value = 'T plan-a 2'; document.getElementById('edit-submit').click()"},
		{"cancel", cancel("edit-dialog")},
		{"backdrop", backdrop("edit-dialog")},
		{"escape", ""},
	} {
		b.eval(rowMenu("plan-a", "edit"))
		until("edit-dialog", true)
		if c.close == "" {
			b.pressKey(kb.Escape)
		} else {
			b.eval(c.close)
		}
		until("edit-dialog", false)
		say("edit, row, " + c.name)
	}
	b.eval("(() => { const m = document.querySelector('#read-actions .more'); m.querySelector('summary').focus(); m.open = true; const i = m.querySelector('.menu .edit'); i.focus(); i.click(); })()")
	until("edit-dialog", true)
	b.pressKey(kb.Escape)
	until("edit-dialog", false)
	say("edit, bar, escape")
	b.eval(rowMenu("plan-a", "edit"))
	until("edit-dialog", true)
	press("document.getElementById('edit-title')")
	b.pressKey(kb.Escape)
	until("edit-dialog", false)
	say("edit, heading pressed, escape")
	for _, c := range []struct{ name, close string }{
		{"save", "document.getElementById('edit-name').value = 'T plan-a 3'; document.getElementById('edit-submit').click()"},
		{"escape", ""},
	} {
		b.eval(fromFind("edit"))
		until("edit-dialog", true)
		if c.close == "" {
			b.pressKey(kb.Escape)
		} else {
			b.eval(c.close)
		}
		until("edit-dialog", false)
		say("edit, search, " + c.name)
		b.eval(fromFindAt("edit", rowTitle))
		until("edit-dialog", true)
		if c.close == "" {
			b.pressKey(kb.Escape)
		} else {
			b.eval(strings.Replace(c.close, "plan-a 3", "plan-a 4", 1))
		}
		until("edit-dialog", false)
		say("edit, search from a row, " + c.name)
	}
	b.eval(rowMenu("plan-a", "edit"))
	until("edit-dialog", true)
	yes, no := true, false
	if _, err := st.Edit("plan-a", store.Edit{Archived: &yes}); err != nil {
		t.Fatal(err)
	}
	b.eval("load()")
	b.until("the row gone", "!document.querySelector('#list li[data-slug=plan-a]')")
	b.pressKey(kb.Escape)
	until("edit-dialog", false)
	say("edit, row gone, escape")
	if _, err := st.Edit("plan-a", store.Edit{Archived: &no}); err != nil {
		t.Fatal(err)
	}
	// Escape and Enter at once: the Enter goes where the focus went back to, and saves nothing
	b.eval("load()")
	b.until("the row back", "document.querySelector('#list li[data-slug=plan-a]')")
	for _, from := range []string{rowMenu("plan-a", "edit"), fromFind("edit")} {
		b.eval(from)
		until("edit-dialog", true)
		b.eval("document.getElementById('edit-name').value = 'Escaped'")
		if _, err := chromedp.Call(b.ctx, input.DispatchKeyEvent, input.DispatchKeyEventParams{Type: input.DispatchKeyEventTypeKeyDown, Key: "Escape", Code: "Escape", WindowsVirtualKeyCode: 27, NativeVirtualKeyCode: 27}); err != nil {
			t.Fatal(err)
		}
		enter()
		if _, err := chromedp.Call(b.ctx, input.DispatchKeyEvent, input.DispatchKeyEventParams{Type: input.DispatchKeyEventTypeKeyUp, Key: "Escape", Code: "Escape", WindowsVirtualKeyCode: 27, NativeVirtualKeyCode: 27}); err != nil {
			t.Fatal(err)
		}
		until("edit-dialog", false)
		b.eval("document.querySelector('.more[open]')?.removeAttribute('open')")
		say("edit, escape and enter")
	}
	out = append(out, "edit, title kept: "+st.ReadMeta("plan-a").Title)

	// Set the task: a row's menu by each way of closing, off its task from the keys and the pointer, the search modal
	open("/")
	for _, c := range []struct{ name, close string }{
		{"save", "document.getElementById('set-category').value = '実装'; document.getElementById('set-submit').click()"},
		{"cancel", cancel("set-dialog")},
		{"backdrop", backdrop("set-dialog")},
		{"escape", ""},
	} {
		b.eval(rowMenu("plan-a", "task-set"))
		until("set-dialog", true)
		if c.close == "" {
			b.pressKey(kb.Escape)
		} else {
			b.eval(c.close)
		}
		until("set-dialog", false)
		say("set, row, " + c.name)
	}
	for _, detail := range []string{"0", "1"} {
		b.eval(rowMenu("plan-a", "task-set"))
		until("set-dialog", true)
		b.eval("document.getElementById('set-out').dispatchEvent(new MouseEvent('click', { bubbles: true, detail: " + detail + " }))")
		until("set-dialog", false)
		say("set, off, detail " + detail)
		onTask("plan-a", "S300", "案")
		b.eval("load()")
		b.until("on again", "items.find((a) => a.slug === 'plan-a').task?.name === 'S300'")
	}
	b.eval(fromFind("task-set"))
	until("set-dialog", true)
	b.pressKey(kb.Escape)
	until("set-dialog", false)
	say("set, search, escape")

	// The ask to move a page off another task, over Set the task: saved by a press as a person does, as Chrome closes on one Escape the
	// modals opened with no press between. Yes answers in its task, the save going on at once
	toMove := func() {
		t.Helper()
		b.eval(rowMenu("plan-a", "task-set"))
		until("set-dialog", true)
		b.eval("const f = document.getElementById('set-task'); f.value = 'S301'; f.dispatchEvent(new Event('input')); document.getElementById('set-category').value = 'x'")
		b.still()
		press("document.getElementById('set-submit')")
		until("move-dialog", true)
	}
	toMove()
	out = append(out, "move, yes, at once: "+b.eval("(async () => { document.getElementById('move-yes').click(); for (let i = 0; i < 5; i++) await null; return document.getElementById('set-submit').disabled + ' ' + moveAnswer; })()"))
	until("set-dialog", false)
	say("move, yes")
	onTask("plan-a", "S300", "案")
	b.eval("load()")
	b.until("on again", "items.find((a) => a.slug === 'plan-a').task?.name === 'S300'")
	toMove()
	b.eval("document.getElementById('move-yes').click()")
	enter()
	until("set-dialog", false)
	say("move, yes and enter")
	out = append(out, "move, written: "+st.ReadMeta("plan-a").Task.Name)
	onTask("plan-a", "S300", "案")
	b.eval("load()")
	b.until("on again", "items.find((a) => a.slug === 'plan-a').task?.name === 'S300'")
	for _, c := range []struct{ name, close string }{
		{"no", cancel("move-dialog")},
		{"escape", ""},
	} {
		toMove()
		if c.close == "" {
			b.pressKey(kb.Escape)
		} else {
			b.eval(c.close)
		}
		until("move-dialog", false)
		say("move, " + c.name)
		out = append(out, "move, "+c.name+", set open: "+b.eval("document.getElementById('set-dialog').open"))
		b.eval(cancel("set-dialog"))
		until("set-dialog", false)
		b.eval("closings.length = 0")
	}

	// Add a page, on a task's page: picked by Enter or a press, cancel, backdrop, Escape in its field; and a page of another task, asked
	open("/tasks?project=proj&task=S300")
	addTo := func(category string) {
		t.Helper()
		b.eval("const a = document.querySelector('#list li[data-key=\"group:" + category + "\"] .category-add'); a.focus(); a.click()")
		until("add-page-dialog", true)
	}
	find := func(words string) {
		t.Helper()
		b.eval("const q = document.getElementById('add-page-q'); q.value = " + js(words) + "; q.dispatchEvent(new Event('input', { bubbles: true }))")
	}
	addTo("実装")
	find("memo")
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.Enter)
	until("add-page-dialog", false)
	say("add page, enter")
	addTo("実装")
	find("check")
	b.eval("document.querySelector('#add-page-list li').click()")
	until("move-dialog", true)
	b.eval("document.getElementById('move-yes').click()")
	enter()
	until("add-page-dialog", false)
	say("add page, moved, yes and enter")
	for _, c := range []struct{ name, close string }{
		{"cancel", cancel("add-page-dialog")},
		{"backdrop", backdrop("add-page-dialog")},
		{"escape", ""},
	} {
		addTo("案")
		if c.close == "" {
			b.pressKey(kb.Escape)
		} else {
			b.eval(c.close)
		}
		until("add-page-dialog", false)
		say("add page, " + c.name)
	}
	// The task page's own modals, which the browser gives the focus back from
	b.eval("document.getElementById('task-edit').focus(); document.getElementById('task-edit').click()")
	until("edit-task-dialog", true)
	b.pressKey(kb.Escape)
	until("edit-task-dialog", false)
	say("edit task, escape")
	b.eval("document.getElementById('task-child').focus(); document.getElementById('task-child').click()")
	until("kin-dialog", true)
	b.pressKey(kb.Escape)
	until("kin-dialog", false)
	say("kin, escape")

	// Settings: a page's theme from its menu, by each way of closing; the bar's button
	open("/")
	for _, c := range []struct{ name, close string }{
		{"save", "(() => { const card = [...document.querySelectorAll('#theme-grid input')].find((i) => !i.checked && i.value); card.click(); document.getElementById('theme-choose').click(); })()"},
		{"cancel", cancel("settings")},
		{"backdrop", backdrop("settings")},
		{"escape", ""},
	} {
		b.eval(rowMenu("plan-a", "theme-item"))
		until("settings", true)
		b.until("the cards", "document.querySelectorAll('#theme-grid input').length > 1")
		if c.close == "" {
			b.pressKey(kb.Escape)
		} else {
			b.eval(c.close)
		}
		until("settings", false)
		say("settings, page, " + c.name)
	}
	b.eval("document.getElementById('settings-open').focus(); document.getElementById('settings-open').click()")
	until("settings", true)
	b.pressKey(kb.Escape)
	until("settings", false)
	say("settings, bar, escape")

	// The search modal opened from the body, a row reached: Escape leaves no field to take the Enter after it
	b.eval("document.activeElement.blur(); document.body.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'k', ctrlKey: true }))")
	until("find", true)
	b.eval("const q = document.getElementById('find-q'); q.value = 'plan'; q.dispatchEvent(new Event('input'))")
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
	"edit, bar, escape: edit-dialog > bar More for T plan-a 2",
	"edit, heading pressed, escape: edit-dialog > list More for T plan-a 2",
	"edit, search, save: find > edit-dialog edit-name, edit-dialog > body / sent 1",
	"edit, search from a row, save: find > edit-dialog edit-name, edit-dialog > body / sent 1",
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
	"set, search, escape: find > set-dialog set-task, set-dialog > body",
	"move, yes, at once: true null",
	"move, yes: move-dialog > set-dialog set-submit, set-dialog > list More for T plan-a 4 / sent 1",
	"move, yes and enter: move-dialog > set-dialog set-submit, set-dialog > list More for T plan-a 4 / sent 1",
	"move, written: S301",
	"move, no: move-dialog > set-dialog set-submit",
	"move, no, set open: true",
	"move, escape: move-dialog > set-dialog set-submit",
	"move, escape, set open: true",
	"add page, enter: add-page-dialog > list Add a page to 実装 / sent 1",
	"add page, moved, yes and enter: move-dialog > add-page-dialog add-page-q, add-page-dialog > list Add a page to 実装 / sent 1",
	"add page, cancel: add-page-dialog > list Add a page to 案",
	"add page, backdrop: add-page-dialog > list Add a page to 案",
	"add page, escape: add-page-dialog > list Add a page to 案",
	"edit task, escape: edit-task-dialog > task-edit",
	"kin, escape: kin-dialog > task-child",
	"settings, page, save: settings > list More for T plan-a 4",
	"settings, page, cancel: settings > list More for T plan-a 4",
	"settings, page, backdrop: settings > list More for T plan-a 4",
	"settings, page, escape: settings > list More for T plan-a 4",
	"settings, bar, escape: settings > settings-open",
	"search, escape: find > body",
	"search, still: /",
	"keys, escape: keys > keys-open",
	"add-dialog, escape: add-dialog > add-link",
	"filter, escape: filter-dialog > narrow-filter",
}
