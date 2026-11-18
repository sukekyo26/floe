package browsertest_test

import (
	"strings"
	"testing"

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
  if (el.closest('.toast')) return 'toast ' + el.textContent.trim();
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
window.heldOf = (id) => { const d = document.getElementById(id); return [d.open, d.getAttribute('closedby'), [...d.querySelectorAll('input, button, select, textarea')].filter((e) => !e.disabled).length].join(' '); };
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
	onTask := func(slug, task, category string) {
		t.Helper()
		s.floe(t, "task", "--slug", slug, "--task", task, "--category", category)
	}
	onTask("plan-a", "S300", "案")
	onTask("check-a", "S301", "x")
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
	rowMenu := func(slug, item string) string {
		return "(() => { const m = document.querySelector('#list li[data-slug=" + slug + "] .more'); m.querySelector('summary').focus(); m.open = true; const i = m.querySelector('.menu ." + item + "'); i.focus(); i.click(); })()"
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
		name, id, page, opens, sends string
		closes                       bool // closes once answered; else stays open on its list (the settings' own)
	}
	setAt := "const f = document.getElementById('set-category'); f.value = '実装';"
	writes := []write{
		{"tags", "tag-dialog", "/", rowMenu("plan-a", "tag-item"), "const f = document.getElementById('tag-find'); f.value = 'n1'; f.dispatchEvent(new Event('input')); document.getElementById('tag-new').click(); document.getElementById('tag-apply').click()", true},
		{"edit", "edit-dialog", "/", rowMenu("plan-a", "edit"), "document.getElementById('edit-desc').value = String(Math.random()); document.getElementById('edit-submit').click()", true},
		{"set", "set-dialog", "/", rowMenu("plan-a", "task-set"), setAt + " document.getElementById('set-submit').click()", true},
		{"off", "set-dialog", "/", rowMenu("check-a", "task-set"), "document.getElementById('set-out').click()", true},
		{"add page", "add-page-dialog", "/tasks?project=proj&task=S300", "const a = document.querySelector('#list li[data-key=\"group:実装\"] .category-add'); a.focus(); a.click()", "const q = document.getElementById('add-page-q'); q.value = 'memo'; q.dispatchEvent(new Event('input', { bubbles: true })); document.querySelector('#add-page-list li').click()", true},
		{"kin", "kin-dialog", "/tasks?project=proj&task=S300", "document.getElementById('task-child').focus(); document.getElementById('task-child').click()", "const f = document.getElementById('kin-task'); f.value = 'S301'; f.dispatchEvent(new Event('input')); document.getElementById('kin').requestSubmit()", true},
		{"task edit", "edit-task-dialog", "/tasks?project=proj&task=S300", "document.getElementById('task-edit').focus(); document.getElementById('task-edit').click()", "document.getElementById('edit-task-title').value = 'Held ' + Math.random(); document.getElementById('edit-task-submit').click()", true},
		{"add link", "add-dialog", "/", "document.getElementById('add-link').focus(); document.getElementById('add-link').click()", "document.getElementById('add-path').value = " + js(link) + "; document.getElementById('add-submit').click()", true},
		{"page theme", "settings", "/", rowMenu("plan-a", "theme-item"), "(() => { const c = [...document.querySelectorAll('#theme-grid input')].find((i) => !i.checked && i.value); c.click(); document.getElementById('theme-choose').click(); })()", true},
		{"mode", "settings", "/", "document.getElementById('settings-open').focus(); document.getElementById('settings-open').click()", "document.querySelector('.settings-item[data-panel=mode]').click(); document.getElementById('task-mode').checked = true; document.getElementById('mode-save').click()", false},
	}
	ready := map[string]string{
		"settings":   "document.querySelectorAll('#theme-grid input').length > 1 && document.getElementById('settings-mode-now').textContent",
		"kin-dialog": "document.getElementById('kin-dialog').open",
	}
	start := func(w write) {
		t.Helper()
		b.eval(w.opens)
		until(w.id, true)
		if r, ok := ready[w.id]; ok {
			b.until("ready", r)
		}
		b.eval(w.sends)
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
		if w.closes {
			b.eval("closeDialog(document.getElementById(" + js(w.id) + ")); true")
		}
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
	for _, w := range []write{writes[1], writes[2], writes[3], writes[4], writes[5], writes[6], writes[7]} {
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
	b.eval(rowMenu("plan-a", "task-set"))
	until("set-dialog", true)
	b.eval("document.getElementById('set-task').value = ''; document.getElementById('set-task').focus(); true")
	b.pressKey(kb.ArrowDown)
	b.until("the suggestions", "document.getElementById('set-task-names').matches(':popover-open') && document.querySelectorAll('#set-task-names [role=option]').length > 1")
	const suggested = "[document.getElementById('set-task').value, document.getElementById('set-task').getAttribute('aria-activedescendant'), setShown?.name ?? '-'].join(' ')"
	say("suggestions, before", suggested)
	// In the task it is held in, before Chrome moves the focus off the disabled field a frame later, a key there reaches no listener of it
	say("suggestions, a key as it is held", "(() => { const d = document.getElementById('set-dialog'); holdDialog(d, true); document.getElementById('set-task').dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true })); const now = "+suggested+"; holdDialog(d, false); return now; })()")
	// Held, the focus is on the modal itself: the field left, its suggestions close; no key reaches the page's shortcuts nor leaves it by
	// Tab; let go after a failure, the focus is back on the field
	b.eval("holdOn('PATCH'); fail.PATCH = true; document.getElementById('set-out').click(); true")
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

	// The page's shortcuts while held: a row picked beside the list stays picked, nothing else is written
	open("/")
	b.eval("document.querySelector('#list li[data-slug=check-a] .title').click(); true")
	b.until("picked", "picked?.slug === 'check-a' && answered === sent.length")
	b.eval("holdOn('PATCH')")
	b.eval(writes[1].opens)
	until("edit-dialog", true)
	b.eval(writes[1].sends)
	b.until("sent", "sent.some((x) => x.startsWith('PATCH'))")
	for _, k := range []string{"j", "k", "d", "f", kb.ArrowDown} {
		b.pressKey(k)
	}
	say("shortcuts while held", "picked.slug + ' ' + sent.filter((x) => x.startsWith('PATCH')).length + ' ' + document.getElementById('edit-dialog').open")
	b.eval("letGo()")
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
	b.eval("(() => { const m = document.querySelector('#frame-actions .more'); m.open = true; m.querySelector('.menu .task-set').click(); })()")
	until("set-dialog", true)
	b.eval("READ_MS = 300; holdOn('TASKS')")
	b.eval("document.getElementById('set-out').click()")
	until("set-dialog", false)
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
	"add page, answered: false closerequest 2 / list Add a page to 実装 / sent 1",
	"kin, held: true none 0 / sent 1",
	"kin, answered: false closerequest 5 / task-child / sent 1",
	"task edit, held: true none 0 / sent 1",
	"task edit, answered: false closerequest 11 / task-edit / sent 1",
	"add link, held: true none 0 / sent 1",
	"add link, answered: false closerequest 5 / add-link / sent 1",
	"page theme, held: true none 0 / sent 1",
	"page theme, answered: false closerequest 20 / list More for T plan-a / sent 1",
	"mode, held: true none 0 / sent 1",
	"mode, answered: true closerequest 19 / settings BUTTON Task mode: OnAgents work with the task list, putting pages on tasks. / sent 1",
	"edit, read, held: true none 0 / sent 1",
	"set, read, held: true none 0 / sent 1",
	"add page, read, held: true none 0 / sent 1",
	"kin, read, held: true none 0 / sent 1",
	"task edit, read, held: true none 0 / sent 1",
	"edit, failed: true closerequest / said true / edit-dialog edit-name",
	"set, failed: true closerequest / said true / set-dialog set-task",
	"off, failed: true closerequest / said true / set-dialog set-task",
	"add page, failed: true closerequest / said true / add-page-dialog add-page-q",
	"kin, failed: true closerequest / said true / kin-dialog kin-task",
	"task edit, failed: true closerequest / said true / edit-task-dialog edit-task-title",
	"add link, failed: true closerequest / said true / add-dialog add-path",
	"suggestions, before:   S300",
	"suggestions, a key as it is held:   S300",
	"suggestions, held: false set-dialog set-dialog",
	"suggestions, keys while held:   S300 set-dialog set-dialog",
	"the focus given back: set-dialog set-task",
	"shortcuts while held: check-a 1 true",
	"a press down held, up let go: true",
	"a press down in a field, up outside: true",
	"a press on the backdrop: false",
	"fold, keys while held: false edit-dialog SUMMARY Slug and projectplan-a · proj",
	"fold, pressed while held: false",
	"page whole, the tasks unread: closerequest 1",
	"loaded again by F5",
	"loaded again by r",
}
