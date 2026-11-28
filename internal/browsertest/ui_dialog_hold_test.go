package browsertest_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"

	"floe/internal/store"
)

// A modal's writes, and the read of the list after them, held back or failed by kind (hold.PATCH, fail.POST, hold.GET…); GET is the
// list's and the tasks' reads alone (TASKS the tasks' alone). answered counts the writes answered.
const dialogHoldJS = `(() => {
window.where = (el) => {
  if (!el || el === document.body) return 'body';
  const d = el.closest('dialog');
  if (d && !d.open) return 'IN CLOSED ' + d.id;
  if (el.closest('[data-testid=toast]')) return 'toast ' + el.textContent.trim();
  const at = el.closest('#list') ? 'list ' : el.closest('dialog') ? el.closest('dialog').id + ' ' : '';
  return at + (el.id || el.getAttribute('aria-label') || el.tagName + ' ' + el.textContent.trim());
};
window.sent = []; window.answered = 0; window.hold = {}; window.fail = {}; window.lets = [];
window.realFetch ??= window.fetch;
window.fetch = (u, i) => {
  const method = i?.method ?? 'GET';
  const path = new URL(u, location).pathname;
  if (method === 'GET' && path !== '/api/pages' && path !== '/api/tasks') return window.realFetch(u, i);
  if (method !== 'GET') sent.push(method + ' ' + path);
  const go = method === 'GET' ? window.realFetch : (u2, i2) => (fail[method] ? Promise.resolve(new Response('refused here', { status: 500 })) : window.realFetch(u2, i2));
  const held = Promise.resolve(path === '/api/tasks' && method === 'GET' ? hold.TASKS ?? hold.GET : hold[method]);
  // Held, a read is still cut by its own time limit, as the server's answer would be
  const cut = new Promise((_, no) => i?.signal?.addEventListener('abort', () => no(i.signal.reason)));
  return Promise.race([held.then(() => go(u, i)), cut]).finally(() => { if (method !== 'GET') answered++; });
};
window.holdOn = (...methods) => { for (const m of methods) hold[m] = new Promise((go) => lets.push(go)); return true; };
window.letGo = () => { hold = {}; for (const go of lets.splice(0)) go(); return true; };
// The modal held: open, its close request off, and nothing in it to press
window.heldOf = (id) => { const d = document.getElementById(id); return [d.open, d.getAttribute('closedby'), [...d.querySelectorAll('input, button, select, textarea')].filter((e) => !e.matches(':disabled')).length].join(' '); };
})()`

// TestUIDialogHold holds each modal's write, and the read after it: no key, press or close request closes the modal or sends again, the
// browser's own keys going on; answered, it closes as it did, and failed, it is given back as it was, its mistake said in it.
func TestUIDialogHold(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
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
	onTask := func(slug, task, category string, more ...string) {
		t.Helper()
		s.floe(t, append([]string{"task", "--slug", slug, "--task", task, "--category", category}, more...)...)
	}
	onTask("plan-a", "S300", "案", "--new")
	onTask("check-a", "S301", "x", "--new")
	s.floe(t, "task", "--project", "proj", "--task", "S300", "--categories", "案", "--categories", "実装")
	link := s.write(t, "links/l.html", "<!doctype html><title>L</title><p>l</p>")
	b.size(1400, 900, false)
	defer b.unsize()

	var out []string
	say := func(what, expr string) {
		t.Helper()
		out = append(out, what+": "+b.eval(expr))
	}
	open := func(path string) {
		t.Helper()
		b.navigate(path)
		b.until("the rows", "document.querySelector('#list li[data-slug]')")
		b.eval(dialogHoldJS)
	}
	until := func(id string, opened bool) {
		t.Helper()
		not := "!"
		if opened {
			not = ""
		}
		b.until(id, not+"document.getElementById("+js(id)+").open")
	}
	press := func(expr string) {
		t.Helper()
		r := b.rect(expr)
		b.click(r.X+r.Width/2, r.Y+r.Height/2)
	}
	// A press down or up alone, as a person's: the browser makes the click of the two
	mouse := func(typ input.DispatchMouseEventType, x, y float64) {
		t.Helper()
		if typ == input.DispatchMouseEventTypeMousePressed {
			b.painted()
		}
		if _, err := chromedp.Call(b.ctx, input.DispatchMouseEvent, input.DispatchMouseEventParams{Type: typ, X: x, Y: y, Button: input.MouseButtonLeft, ClickCount: 1}); err != nil {
			t.Fatal(err)
		}
	}
	// A key with modifiers (Ctrl 2), pressed as the browser's own: F5 or Ctrl+R
	key := func(k, code string, vk, mods int64) {
		t.Helper()
		for _, typ := range []input.DispatchKeyEventType{input.DispatchKeyEventTypeRawKeyDown, input.DispatchKeyEventTypeKeyUp} {
			if _, err := chromedp.Call(b.ctx, input.DispatchKeyEvent, input.DispatchKeyEventParams{Type: typ, Key: k, Code: code, WindowsVirtualKeyCode: vk, NativeVirtualKeyCode: vk, Modifiers: mods}); err != nil {
				t.Fatal(err)
			}
		}
	}
	rowMenu := func(slug, item string) func() {
		return func() {
			t.Helper()
			pressMenuItemFocused(b, "#list li[data-slug="+slug+"]", item)
		}
	}
	run := func(expr string) func() {
		return func() {
			t.Helper()
			b.eval(expr)
		}
	}
	// Words typed in field, then what sends them pressed
	typed := func(field, words, then string) func() {
		return func() {
			t.Helper()
			b.fill(field, words)
			b.eval(then)
		}
	}
	// Every way to close or send again, while held: what is left of the modal, and the writes sent
	tryAll := func(name, id string) {
		t.Helper()
		b.pressKey(kb.Escape)
		b.pressKey(kb.Enter)
		b.pressKey(kb.Tab)
		b.pressKey(" ")
		b.eval("document.querySelector('#" + id + " [data-close]')?.click(); document.getElementById(" + js(id) + ").dispatchEvent(new PointerEvent('pointerdown', { bubbles: true })); document.getElementById(" + js(id) + ").dispatchEvent(new MouseEvent('click', { bubbles: true })); true")
		say(name+", held", "heldOf("+js(id)+") + ' / sent ' + sent.length")
	}
	type write struct {
		name, id, page string
		opens, sends   func()
		closes         bool // closes once answered; else stays open on its list (the settings' own)
	}
	// Words not written before, so that each save changes something
	n := 0
	fresh := func() string {
		n++
		return strconv.Itoa(n)
	}
	writes := []write{
		{"tags", "tag-dialog", "/", rowMenu("plan-a", "tag-item"), func() { b.fill("#tag-find", "n1"); applyNewTag(b) }, true},
		{"edit", "edit-dialog", "/", rowMenu("plan-a", "edit"), func() { typed("#edit-desc", "d"+fresh(), "document.getElementById('edit-submit').click()")() }, true},
		{"set", "set-dialog", "/", rowMenu("plan-a", "task-set"), typed("#set-category", "実装", "document.getElementById('set-submit').click()"), true},
		{"off", "set-dialog", "/", rowMenu("check-a", "task-set"), run("document.getElementById('set-out').click()"), true},
		{"add page", "add-page-dialog", "/tasks?project=proj&task=S300", run("const a = document.getElementById('task-add-page'); a.focus(); a.click()"), func() {
			chooseValue(b, "add-page-category", "実装")
			typed("#add-page-q", "memo", "document.querySelector('#add-page-list li').click()")()
		}, true},
		{"kin", "kin-dialog", "/tasks?project=proj&task=S300", run("document.getElementById('task-child').focus(); document.getElementById('task-child').click()"), typed("#kin-task", "S301", "document.getElementById('kin-submit').click()"), true},
		{"task edit", "edit-task-dialog", "/tasks?project=proj&task=S300", run("document.getElementById('task-edit').focus(); document.getElementById('task-edit').click()"), func() {
			typed("#edit-task-title", "Held "+fresh(), "document.getElementById('edit-task-submit').click()")()
		}, true},
		{"add link", "add-dialog", "/", run("document.getElementById('add-link').focus(); document.getElementById('add-link').click()"), typed("#add-path", link, "document.getElementById('add-submit').click()"), true},
		{"page theme", "settings", "/", rowMenu("plan-a", "theme-item"), run("(() => { const c = [...document.querySelectorAll('#theme-grid input')].find((i) => !i.checked && i.value); c.click(); document.getElementById('theme-choose').click(); })()"), true},
		{"mode", "settings", "/", run("document.getElementById('settings-open').focus(); document.getElementById('settings-open').click()"), run("document.querySelector('#settings [data-panel=mode]').click(); if (!document.getElementById('task-mode').checked) document.getElementById('task-mode').click(); document.getElementById('mode-save').click()"), false},
	}
	ready := map[string]string{
		"settings":   "document.querySelectorAll('#theme-grid input').length > 1 && document.getElementById('settings-mode-now').textContent",
		"kin-dialog": "document.getElementById('kin-dialog').open",
	}
	start := func(w write) {
		t.Helper()
		w.opens()
		until(w.id, true)
		if r, ok := ready[w.id]; ok {
			b.until("ready", r)
		}
		w.sends()
		b.until("sent", "sent.length === 1")
	}
	ends := func(w write) {
		t.Helper()
		if w.closes {
			until(w.id, false)
		} else {
			b.until("let go", "document.getElementById("+js(w.id)+").getAttribute('closedby') === 'closerequest'")
		}
	}

	again := func() {
		t.Helper()
		s.floe(t, "task", "--slug", "memo-a", "--task", "")
		onTask("plan-a", "S300", "案")
		onTask("check-a", "S301", "x")
		s.floe(t, "task", "--project", "proj", "--task", "S301", "--parent", "")
	}
	// T1: the write held
	for _, w := range writes {
		open(w.page)
		b.eval("holdOn('PATCH', 'POST', 'PUT')")
		start(w)
		tryAll(w.name, w.id)
		b.eval("letGo()")
		ends(w)
		say(w.name+", answered", "heldOf("+js(w.id)+") + ' / ' + where(document.activeElement) + ' / sent ' + sent.length")
	}
	again()

	// T1b: the read of the list after the write held: still held
	for _, w := range []write{writes[1], writes[2], writes[4], writes[5], writes[6]} {
		open(w.page)
		b.eval("holdOn('GET')")
		start(w)
		b.until("answered", "answered === 1")
		tryAll(w.name+", read", w.id)
		b.eval("letGo()")
		ends(w)
	}
	again()

	// T2: the write failed: given back as it was, its mistake said in it, and it closes on Escape
	errorOf := map[string]string{"edit-dialog": "edit-error", "set-dialog": "set-error", "add-page-dialog": "add-page-error", "kin-dialog": "kin-error", "edit-task-dialog": "edit-task-error", "add-dialog": "add-error"}
	// On a task's page (React), the category typed one of the task's: given back there, its suggestions stay shut (S444)
	setOnTask := write{"set on a task's page", "set-dialog", "/tasks?project=proj&task=S300", rowMenu("plan-a", "task-set"), typed("#set-category", "実装", "document.getElementById('set-submit').click()"), true}
	for _, w := range []write{writes[1], writes[2], writes[3], setOnTask, writes[4], writes[5], writes[6], writes[7]} {
		open(w.page)
		b.eval("holdOn('PATCH', 'POST'); fail.PATCH = true; fail.POST = true; true")
		start(w)
		b.eval("letGo()")
		b.until("let go", "document.getElementById("+js(w.id)+").getAttribute('closedby') === 'closerequest'")
		say(w.name+", failed", "heldOf("+js(w.id)+").split(' ').slice(0, 2).join(' ') + ' / said ' + !document.getElementById("+js(errorOf[w.id])+").hidden + ' / ' + where(document.activeElement)")
		b.pressKey(kb.Escape)
		until(w.id, false)
	}
	again()

	// T3 of the keys and presses
	open("/")
	rowMenu("plan-a", "task-set")()
	until("set-dialog", true)
	b.fill("#set-task", "")
	b.until("the suggestions", "document.getElementById('set-task-names').matches(':popover-open') && document.querySelectorAll('#set-task-names [role=option]').length > 1")
	// The words, the suggestion picked, and whether the modal takes the name for a task known (the new task's title not asked) or new
	const suggested = "[document.getElementById('set-task').value, document.getElementById('set-task').getAttribute('aria-activedescendant'), document.getElementById('set-task-title').checkVisibility() ? 'new' : 'known'].join(' ')"
	say("suggestions, before", suggested)
	// Held by the write itself, which holds the modal as it is pressed: in that task, before Chrome moves the focus off the disabled field a
	// frame later, a key there reaches no listener of it. Then held, the focus is on the modal itself: the field left, its suggestions
	// close; no key reaches the page's shortcuts nor leaves it by Tab; let go after a failure, the focus is back on the field
	say("suggestions, a key as it is held", "(() => { holdOn('PATCH'); fail.PATCH = true; document.getElementById('set-out').click(); document.getElementById('set-task').dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true })); return "+suggested+"; })()")
	b.until("sent", "sent.length === 1")
	say("suggestions, held", "document.getElementById('set-task-names').matches(':popover-open') + ' ' + where(document.activeElement)")
	b.pressKey(kb.Tab)
	b.pressKey(kb.ArrowDown)
	say("suggestions, keys while held", suggested+" + ' ' + where(document.activeElement)")
	b.eval("letGo()")
	b.until("let go", "document.getElementById('set-dialog').getAttribute('closedby') === 'closerequest'")
	say("the focus given back", "where(document.activeElement)")
	b.pressKey(kb.Escape)
	until("set-dialog", false)

	// Saved by Enter in the category's field and failed: the focus given back there opens no suggestions over the mistake said
	rowMenu("plan-a", "task-set")()
	until("set-dialog", true)
	b.fill("#set-category", "")
	b.until("its suggestions", "document.getElementById('set-category-names').matches(':popover-open')")
	b.pressKey(kb.Escape)
	b.eval("sent.length = 0; holdOn('PATCH'); fail.PATCH = true")
	b.pressKey(kb.Enter)
	b.until("sent", "sent.length === 1")
	b.eval("letGo()")
	b.until("let go", "document.getElementById('set-dialog').getAttribute('closedby') === 'closerequest'")
	say("category, failed", "where(document.activeElement) + ' / suggestions ' + document.getElementById('set-category-names').matches(':popover-open') + ' / said ' + !document.getElementById('set-error').hidden")
	b.pressKey(kb.Escape)
	until("set-dialog", false)

	// On a task's page (React), held with the suggestions open: the blur of the field disabled as it is held closes them,
	// and after a failure one Escape closes the modal (S446)
	open("/tasks?project=proj&task=S300")
	rowMenu("plan-a", "task-set")()
	until("set-dialog", true)
	b.fill("#set-task", "")
	b.until("the suggestions", "document.getElementById('set-task-names').matches(':popover-open')")
	b.eval("sent.length = 0; holdOn('PATCH'); fail.PATCH = true; document.getElementById('set-out').click(); true")
	b.until("sent", "sent.length === 1")
	say("on a task's page, held", "document.getElementById('set-task-names').matches(':popover-open') + ' ' + where(document.activeElement)")
	b.eval("letGo()")
	b.until("let go", "document.getElementById('set-dialog').getAttribute('closedby') === 'closerequest'")
	say("on a task's page, given back", "document.getElementById('set-task-names').matches(':popover-open') + ' ' + where(document.activeElement)")
	b.pressKey(kb.Escape)
	until("set-dialog", false)

	// The page's shortcuts while held: a row picked beside the list stays picked, nothing else is written
	open("/")
	const picked = "document.getElementById('read-frame').dataset.slug"
	b.eval("document.querySelector('#list li[data-slug=check-a] [data-testid=title]').click(); true")
	b.until("picked", picked+" === 'check-a' && answered === sent.length")
	b.eval("holdOn('PATCH')")
	writes[1].opens()
	until("edit-dialog", true)
	writes[1].sends()
	b.until("sent", "sent.some((x) => x.startsWith('PATCH'))")
	for _, k := range []string{"j", "k", "d", "f", kb.ArrowDown} {
		b.pressKey(k)
	}
	say("shortcuts while held", picked+" + ' ' + sent.filter((x) => x.startsWith('PATCH')).length + ' ' + document.getElementById('edit-dialog').open")
	// Its cancel button leaves a held modal open, as do Escape and the backdrop (tryAll)
	say("cancel while held", "(document.querySelector('#edit-dialog [data-close]').click(), document.getElementById('edit-dialog').open)")
	b.eval("letGo()")
	until("edit-dialog", false)
	// Any modal open, held or not, the focus off it (on the body): still no shortcut
	writes[1].opens()
	until("edit-dialog", true)
	b.eval("document.activeElement.blur(); true")
	b.pressKey("j")
	say("shortcuts, a modal open, the focus on the body", picked)
	b.pressKey(kb.Escape)
	until("edit-dialog", false)

	// A press down while held and up once let go is no press on the backdrop, nor one down in a field and up outside
	open("/")
	b.eval("holdOn('PATCH'); fail.PATCH = true")
	start(writes[1])
	name := b.rect("document.getElementById('edit-name')")
	mouse(input.DispatchMouseEventTypeMousePressed, name.X+10, name.Y+name.Height/2)
	b.eval("letGo()")
	b.until("let go", "document.getElementById('edit-dialog').getAttribute('closedby') === 'closerequest'")
	mouse(input.DispatchMouseEventTypeMouseReleased, name.X+10, name.Y+name.Height/2)
	say("a press down held, up let go", "document.getElementById('edit-dialog').open")
	mouse(input.DispatchMouseEventTypeMousePressed, name.X+10, name.Y+name.Height/2)
	mouse(input.DispatchMouseEventTypeMouseMoved, 5, 5)
	mouse(input.DispatchMouseEventTypeMouseReleased, 5, 5)
	say("a press down in a field, up outside", "document.getElementById('edit-dialog').open")
	mouse(input.DispatchMouseEventTypeMousePressed, 5, 5)
	mouse(input.DispatchMouseEventTypeMouseReleased, 5, 5)
	say("a press on the backdrop", "document.getElementById('edit-dialog').open")

	// What is neither a field nor a button stays focusable and pressable while held: no key or press reaches it, the edit's fold staying
	// shut
	open("/")
	b.eval("holdOn('PATCH')")
	start(writes[1])
	b.eval("document.querySelector('#edit-more > summary').focus(); true")
	b.pressKey(kb.Enter)
	b.pressKey(" ")
	say("fold, keys while held", "document.getElementById('edit-more').open + ' ' + where(document.activeElement)")
	press("document.querySelector('#edit-more > summary')")
	say("fold, pressed while held", "document.getElementById('edit-more').open")
	b.eval("letGo()")
	until("edit-dialog", false)

	// A page whole reads the tasks after a write: a read that never answers is cut at the list's time limit, and the modal let go
	b.navigate("/a/plan-a/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	b.eval(dialogHoldJS)
	b.untilRead(0, "the bar's actions", "!document.getElementById('frame-actions').hidden")
	pressMenuItem(b, "#frame-actions", "task-set")
	until("set-dialog", true)
	b.eval("holdOn('TASKS')")
	b.eval("document.getElementById('set-out').click()")
	// The page's own time limit, 10 s
	b.untilWithin(20*time.Second, "the tasks' read cut", "!document.getElementById('set-dialog').open")
	say("page whole, the tasks unread", "document.getElementById('set-dialog').getAttribute('closedby') + ' ' + sent.filter((x) => x.startsWith('PATCH')).length")
	b.eval("letGo()")
	onTask("plan-a", "S300", "案")

	// The browser's own keys go on while held: F5 and Ctrl+R load the page again, the way out of a write that never answers
	for _, k := range []struct {
		name, code string
		vk, mods   int64
	}{{"F5", "F5", 116, 0}, {"r", "KeyR", 82, 2}} {
		open("/")
		b.eval("holdOn('PATCH')")
		start(writes[1])
		b.eval("window.before = true")
		key(k.name, k.code, k.vk, k.mods)
		b.until("loaded again", "!window.before && document.querySelector('#list li[data-slug]')")
		out = append(out, "loaded again by "+k.name)
	}

	check(t, "held", out, strings.Join(dialogHoldWant, "|"))
}

var dialogHoldWant = []string{
	"tags, held: true none 0 / sent 1",
	"tags, answered: false closerequest 5 / list More for T plan-a / sent 1",
	"edit, held: true none 0 / sent 1",
	"edit, answered: false closerequest 6 / list More for T plan-a / sent 1",
	"set, held: true none 0 / sent 1",
	"set, answered: false closerequest 6 / list More for T plan-a / sent 1",
	"off, held: true none 0 / sent 1",
	"off, answered: false closerequest 6 / toast Undo / sent 1",
	"add page, held: true none 0 / sent 1",
	"add page, answered: false closerequest 4 / task-add-page / sent 1",
	"kin, held: true none 0 / sent 1",
	"kin, answered: false closerequest 5 / task-child / sent 1",
	"task edit, held: true none 0 / sent 1",
	"task edit, answered: false closerequest 10 / task-edit / sent 1",
	"add link, held: true none 0 / sent 1",
	"add link, answered: false closerequest 5 / add-link / sent 1",
	"page theme, held: true none 0 / sent 1",
	"page theme, answered: false closerequest 22 / list More for T plan-a / sent 1",
	"mode, held: true none 0 / sent 1",
	"mode, answered: true closerequest 21 / settings BUTTON Task mode: OnAgents work with the task list, putting pages on tasks. / sent 1",
	"edit, read, held: true none 0 / sent 1",
	"set, read, held: true none 0 / sent 1",
	"add page, read, held: true none 0 / sent 1",
	"kin, read, held: true none 0 / sent 1",
	"task edit, read, held: true none 0 / sent 1",
	"edit, failed: true closerequest / said true / edit-dialog edit-desc",
	"set, failed: true closerequest / said true / set-dialog set-category",
	"off, failed: true closerequest / said true / set-dialog set-task",
	"set on a task's page, failed: true closerequest / said true / set-dialog set-category",
	"add page, failed: true closerequest / said true / add-page-dialog add-page-q",
	"kin, failed: true closerequest / said true / kin-dialog kin-task",
	"task edit, failed: true closerequest / said true / edit-task-dialog edit-task-title",
	"add link, failed: true closerequest / said true / add-dialog add-path",
	"suggestions, before:   new",
	"suggestions, a key as it is held:   new",
	"suggestions, held: false set-dialog set-dialog",
	"suggestions, keys while held:   new set-dialog set-dialog",
	"the focus given back: set-dialog set-task",
	"category, failed: set-dialog set-category / suggestions false / said true",
	"on a task's page, held: false set-dialog set-dialog",
	"on a task's page, given back: false set-dialog set-task",
	"shortcuts while held: check-a 1 true",
	"cancel while held: true",
	"shortcuts, a modal open, the focus on the body: check-a",
	"a press down held, up let go: true",
	"a press down in a field, up outside: true",
	"a press on the backdrop: false",
	"fold, keys while held: false edit-dialog SUMMARY Slug and projectplan-a · proj",
	"fold, pressed while held: false",
	"page whole, the tasks unread: closerequest 1",
	"loaded again by F5",
	"loaded again by r",
}
