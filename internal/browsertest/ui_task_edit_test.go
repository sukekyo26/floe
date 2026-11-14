package browsertest_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/chromedp/chromedp/kb"

	"floe/internal/store"
)

// A task's page edits the task's own fields in one dialog: its title, URLs and the order of its categories, sending what was changed alone,
// with the rev it showed: of the URLs, those added and those taken off. The categories are reordered, their names kept, by buttons that keep the focus and say where a category went, or by a
// drag of a grip that a screen without hover does not show. Refused as stale, what was changed stays and the rest is read again; a task
// removed meanwhile is not made again.
func TestUITaskEdit(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	project := "proj"
	for _, p := range [][2]string{{"plan-a", "案"}, {"design-a", "設計"}, {"build-a", "実装"}, {"review-a", "レビュー"}, {"gone-a", ""}} {
		s.floe(t, "publish", s.page(t, p[0]+".yaml", "T "+p[0], ""), "--slug", p[0])
		if _, err := st.Edit(p[0], store.Edit{Project: &project}); err != nil {
			t.Fatal(err)
		}
		if p[1] != "" {
			s.floe(t, "work", p[0], "--task", "S300", "--category", p[1])
		}
	}
	s.floe(t, "work", "--project", "proj", "--task", "S300", "--title", "Edit the task", "--url", "https://example.com/489", "--categories", "案,設計,実装,リリース,レビュー")
	s.floe(t, "work", "gone-a", "--task", "G1", "--category", "x")
	b.size(1400, 900, false)
	defer b.unsize()

	var out []string
	say := func(what, expr string) {
		t.Helper()
		out = append(out, what+": "+b.eval(expr))
	}
	row := func(name string) string {
		t.Helper()
		all, err := st.ReadTasks()
		if err != nil {
			t.Fatal(err)
		}
		r, ok := all["proj"][name]
		if !ok {
			return "none"
		}
		return strconv.Itoa(r.Rev) + " " + r.Title + " " + strings.Join(r.URLs, ",") + " [" + strings.Join(r.Categories, ",") + "]"
	}
	const categories = "[...document.querySelectorAll('#edit-categories li')].map((li) => li.querySelector('.category-name').textContent + (li.querySelector('.category-up').disabled ? '^' : '') + (li.querySelector('.category-down').disabled ? 'v' : '')).join(',')"
	const urls = "[...document.querySelectorAll('#edit-urls li')].map((li) => li.dataset.url).join(',')"
	const fields = "[document.getElementById('edit-task-title').value, " + urls + "].join(' / ') + ' / ' + " + categories
	const errorText = "(document.getElementById('edit-task-error').hidden ? '-' : document.getElementById('edit-task-error').textContent)"
	open := func(task string) {
		t.Helper()
		b.navigate("/tasks?project=proj&task=" + task)
		b.until("the task's head", "!document.getElementById('task-head').hidden && document.getElementById('task-title').textContent.includes("+js(task)+")")
		// The bodies of the task's writes, as sent
		b.eval("window.sent = []; window.realFetch ??= window.fetch; window.fetch = (url, init) => { if (String(url).includes('/api/work') && init?.method === 'PATCH') sent.push(init.body); return window.realFetch(url, init); }")
		b.eval("document.getElementById('task-edit').focus(); document.getElementById('task-edit').click()")
		b.until("the dialog", "document.getElementById('edit-task-dialog').open")
	}
	field := func(id, value string) {
		t.Helper()
		b.eval("document.getElementById(" + js(id) + ").value = " + js(value) + "; document.getElementById(" + js(id) + ").dispatchEvent(new Event('input', { bubbles: true }))")
	}
	// Adds a URL as typed: the button, then the field and Enter
	addURL := func(url string) {
		t.Helper()
		b.eval("document.getElementById('edit-url-plus').click()")
		field("edit-url-new", url)
		b.eval("document.getElementById('edit-url-new').dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))")
	}
	dropURL := func(url string) {
		t.Helper()
		b.eval("[...document.querySelectorAll('#edit-urls li')].find((li) => li.dataset.url === " + js(url) + ").querySelector('.url-drop').click()")
	}
	press := func(category, dir string) {
		t.Helper()
		b.eval("[...document.querySelectorAll('#edit-categories li')].find((li) => li.dataset.category === " + js(category) + ").querySelector('.category-" + dir + "').click()")
	}
	// Saves, until the dialog closes or says why it did not
	save := func(what string) {
		t.Helper()
		b.eval("document.getElementById('edit-task-error').hidden = true; document.getElementById('edit-task').requestSubmit()")
		b.until(what, "!document.getElementById('edit-task-dialog').open || !document.getElementById('edit-task-error').hidden")
	}
	closed := func(what string) {
		t.Helper()
		b.until(what, "!document.getElementById('edit-task-dialog').open")
	}
	// A drag of a category's grip onto another category, in its upper half or its lower one, as a browser sends it
	drag := func(from, onto string, after bool) {
		t.Helper()
		b.eval(`(() => {
			const rows = [...document.querySelectorAll('#edit-categories li')];
			const li = rows.find((x) => x.dataset.category === ` + js(from) + `), to = rows.find((x) => x.dataset.category === ` + js(onto) + `);
			const box = to.getBoundingClientRect(), y = ` + strconv.FormatBool(after) + ` ? box.bottom - 2 : box.top + 2;
			const data = new DataTransfer();
			li.querySelector('.category-grip').dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
			li.dispatchEvent(new DragEvent('dragstart', { bubbles: true, dataTransfer: data }));
			to.dispatchEvent(new DragEvent('dragover', { bubbles: true, cancelable: true, dataTransfer: data, clientY: y }));
			to.dispatchEvent(new DragEvent('drop', { bubbles: true, cancelable: true, dataTransfer: data, clientY: y }));
			li.dispatchEvent(new DragEvent('dragend', { bubbles: true, dataTransfer: data }));
		})()`)
	}

	// Opened: the title, URL and categories written in order, those of no page marked, then those of pages alone; the ends move no further.
	open("S300")
	say("button", "document.getElementById('task-edit').getAttribute('aria-label') + ' / ' + document.getElementById('task-edit').textContent.trim()")
	say("opened", fields)
	say("no page", "[...document.querySelectorAll('#edit-categories li')].filter((li) => li.querySelector('.category-name small')).map((li) => li.dataset.category).join(',')")
	say("described", "document.getElementById('edit-categories').getAttribute('aria-describedby') + ' ' + document.getElementById('edit-task-error').getAttribute('role')")
	// Nothing changed, nothing is sent; moved and moved back is no change.
	save("closed unchanged")
	out = append(out, "unchanged: "+row("S300"))
	b.eval("document.getElementById('task-edit').click()")
	b.until("the dialog", "document.getElementById('edit-task-dialog').open")
	press("レビュー", "up")
	say("moved up", categories+" + ' / ' + document.activeElement.getAttribute('aria-label') + ' / ' + document.getElementById('edit-categories-said').textContent")
	press("レビュー", "down")
	save("closed moved back")
	out = append(out, "moved back: "+row("S300"))
	say("sent unchanged", "sent.length")
	// Escape closes it, the focus back on its button.
	b.eval("document.getElementById('task-edit').click()")
	b.until("the dialog", "document.getElementById('edit-task-dialog').open")
	b.pressKey(kb.Escape)
	closed("closed by Escape")
	say("focus back", "document.activeElement.id")

	// A category at the top, its up gone: the focus goes to its down.
	open("S300")
	press("設計", "up")
	say("at the top", categories+" + ' / ' + document.activeElement.getAttribute('aria-label')")
	b.pressKey(kb.Escape)
	closed("closed")

	// The title alone changed is sent alone.
	open("S300")
	field("edit-task-title", "New title")
	save("saved the title")
	closed("closed")
	out = append(out, "title: "+row("S300"))
	say("sent the title", "sent.join(' ; ')")
	// Reordered by buttons: the pages' categories follow, and a category of pages alone is written too.
	open("S300")
	press("レビュー", "up")
	press("レビュー", "up")
	save("saved the order")
	closed("closed")
	out = append(out, "order: "+row("S300"))
	say("sent the order", "sent.join(' ; ')")
	say("groups", "[...document.querySelectorAll('#list li.group')].map((li) => li.textContent.trim()).join(',')")
	// Reordered by a drag: before a category in its upper half, after it in its lower one.
	open("S300")
	drag("リリース", "案", false)
	drag("案", "実装", true)
	say("dragged", categories)
	// A grip pressed and let go without a drag leaves its row no drag from anywhere; a drag of words selected, from a text node, moves nothing
	// and throws nothing.
	b.eval(`(() => {
		const li = document.querySelector('#edit-categories li');
		li.querySelector('.category-grip').dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
		li.dispatchEvent(new PointerEvent('pointerup', { bubbles: true }));
	})()`)
	say("let go", "[...document.querySelectorAll('#edit-categories li')].some((li) => li.draggable)")
	b.eval(`(() => {
		window.thrown = 0;
		addEventListener('error', () => { thrown++; });
		const word = document.querySelectorAll('#edit-categories .category-name')[1].firstChild;
		const data = new DataTransfer();
		word.dispatchEvent(new DragEvent('dragstart', { bubbles: true, dataTransfer: data }));
		const to = document.querySelector('#edit-categories li');
		to.dispatchEvent(new DragEvent('drop', { bubbles: true, cancelable: true, dataTransfer: data, clientY: to.getBoundingClientRect().top + 2 }));
	})()`)
	say("words dragged", "thrown + ' / ' + "+categories)
	save("saved the drag")
	closed("closed")
	out = append(out, "drag: "+row("S300"))
	// A screen without hover shows no grip: the rule is there.
	say("no grip without hover", "[...document.styleSheets].flatMap((sh) => [...sh.cssRules]).some((r) => r.media?.mediaText === '(hover: none)' && [...r.cssRules].some((x) => x.selectorText === '.category-order .category-grip' && x.style.display === 'none'))")

	// URLs: one added and one taken off are sent as such, and the page shows a link of each. A URL there already, or typed and not added, is
	// said, not sent.
	open("S300")
	addURL("https://example.com/pull/9")
	addURL("https://example.com/pull/9")
	say("added twice", urls+" + ' / ' + document.getElementById('edit-url-said').textContent")
	// A URL's row and a category's are taken off by one icon, drawn alike
	say("one bin", `(() => {
		const d = (on) => [...document.querySelectorAll(on + ' svg path')].map((p) => p.getAttribute('d')).join(' ');
		const url = document.querySelector('#edit-urls .url-drop svg'), category = document.querySelector('#edit-categories .category-drop svg');
		return [d('#edit-urls li:first-child .url-drop') === d('#edit-categories li:first-child .category-drop'), url.getAttribute('stroke-width') === category.getAttribute('stroke-width')].join(' ');
	})()`)
	field("edit-url-new", "https://example.com/left")
	b.eval("document.getElementById('edit-task').requestSubmit()")
	say("typed, not added", "document.getElementById('edit-url-new').validationMessage + ' / ' + document.getElementById('edit-task-dialog').open")
	field("edit-url-new", "")
	dropURL("https://example.com/489")
	say("dropped", urls+" + ' / ' + document.activeElement.getAttribute('aria-label') + ' / ' + document.getElementById('edit-urls-said').textContent")
	addURL("https://example.com/489")
	save("saved the URLs")
	closed("closed")
	out = append(out, "urls: "+row("S300"))
	say("sent the URLs", "sent.join(' ; ')")
	b.until("the links", "document.querySelectorAll('#task-link a').length === 2")
	say("links", "[...document.querySelectorAll('#task-link a')].map((a) => a.href + ' ' + a.textContent).join(',')")
	// Under the state's line, one a line, at one left edge
	say("links stacked", `(() => {
		const r = [...document.querySelectorAll('#task-link a')].map((a) => a.getBoundingClientRect()), state = document.querySelector('#task-head .tstate').getBoundingClientRect();
		return [r[0].top >= state.bottom, r[1].top >= r[0].bottom, r[0].left === r[1].left, document.getElementById('task-link').hidden].join(' ');
	})()`)
	// One longer than the head wraps in its line, all its words there: none cut, none past the head's edge
	say("a long link wraps", `(() => {
		const head = document.getElementById('task-head');
		head.style.width = '9rem';
		const a = document.querySelector('#task-link a'), r = a.getBoundingClientRect(), edge = document.getElementById('task-link').getBoundingClientRect().right;
		const said = [r.height > 2 * parseFloat(getComputedStyle(a).fontSize), r.right <= edge, a.scrollWidth <= a.clientWidth, a.textContent === a.href].join(' ');
		head.style.width = '';
		return said;
	})()`)
	// At 10, one more is said and not added
	open("S300")
	for i := 3; i <= 11; i++ {
		addURL("https://example.com/more/" + strconv.Itoa(i))
	}
	say("an 11th", "document.querySelectorAll('#edit-urls li').length + ' / ' + document.getElementById('edit-url-said').textContent + ' / ' + document.getElementById('edit-urls-hint').textContent")
	b.pressKey(kb.Escape)
	b.pressKey(kb.Escape)
	closed("closed")

	// Refused by the store: the dialog stays open with its words.
	open("S300")
	addURL("ftp://x")
	save("refused")
	say("refused", errorText+" + ' / ' + document.getElementById('edit-task-dialog').open")
	// Not reached: said as a save that failed, not as the browser's words alone.
	b.eval("window.cutFetch = window.fetch; window.fetch = (url, init) => (String(url).startsWith('/api/work') && init?.method === 'PATCH' ? Promise.reject(new TypeError('Failed to fetch')) : window.cutFetch(url, init))")
	save("not reached")
	say("not reached", errorText+" + ' / ' + document.getElementById('edit-task-dialog').open")
	b.eval("window.fetch = window.cutFetch")
	dropURL("ftp://x")
	dropURL("https://example.com/pull/9")
	dropURL("https://example.com/489")
	field("edit-task-title", "")
	save("saved empty")
	closed("closed")
	out = append(out, "emptied: "+row("S300"))
	b.until("no links", "document.getElementById('task-link').hidden && document.getElementById('task-link').getClientRects().length === 0")
	// Opened over the task drawn before the tasks were read again: its fields and rev are of the one read now, not of what was drawn.
	b.eval("window.drawnTask = tasksOf().find((x) => x.name === 'S300')")
	s.floe(t, "work", "--project", "proj", "--task", "S300", "--title", "Later")
	b.eval("window.reread = false; readTasks().then((r) => { tasksRead = r; window.reread = true; })")
	b.until("the tasks read again", "window.reread")
	b.eval("openTaskEdit(drawnTask)")
	b.until("the dialog", "document.getElementById('edit-task-dialog').open")
	say("opened over the drawn task", "document.getElementById('edit-task-title').value + ' / ' + taskEditing.rev")
	b.pressKey(kb.Escape)
	closed("closed")

	// Stale: what was changed stays, the rest is read again, and the second save sends the change alone.
	open("S300")
	s.floe(t, "work", "--project", "proj", "--task", "S300", "--url", "https://example.com/new")
	field("edit-task-title", "Typed")
	save("stale")
	say("stale", fields+" + ' / ' + "+errorText)
	save("saved again")
	closed("closed")
	out = append(out, "again: "+row("S300"))
	say("sent stale and again", "sent.join(' ; ')")
	// URLs changed elsewhere meanwhile: the one added there is kept, the one taken off there stays off, and only the person's own is sent.
	open("S300")
	addURL("https://example.com/mine")
	s.floe(t, "work", "--project", "proj", "--task", "S300", "--url", "https://example.com/theirs", "--remove-url", "https://example.com/new")
	save("stale URLs")
	say("stale URLs", urls)
	save("saved the URLs again")
	closed("closed")
	out = append(out, "urls again: "+row("S300"))
	say("sent the URLs again", "sent.join(' ; ')")
	// Reordered while a category of no page was taken off: the order stays, without it.
	open("S300")
	press("実装", "down")
	s.floe(t, "work", "--project", "proj", "--task", "S300", "--categories", "案,設計,実装,レビュー")
	save("stale order")
	say("stale order", categories)
	save("saved the order again")
	closed("closed")
	out = append(out, "order again: "+row("S300"))

	// Removed meanwhile: not made again.
	open("G1")
	s.floe(t, "work", "gone-a", "--clear")
	s.floe(t, "work", "--project", "proj", "--task", "G1", "--forget")
	field("edit-task-title", "Again")
	save("gone")
	say("gone", errorText+" + ' / ' + document.getElementById('edit-task-submit').disabled")
	field("edit-task-title", "Again and again")
	say("gone, typed", errorText)
	out = append(out, "G1: "+row("G1"))

	// A narrow screen: the dialog fits, and its buttons reorder.
	b.size(390, 844, true)
	open("S300")
	say("narrow", "document.getElementById('edit-task').scrollWidth <= document.getElementById('edit-task').clientWidth")
	press("設計", "down")
	say("narrow moved", categories)
	b.pressKey(kb.Escape)

	check(t, "task edit", out, strings.Join([]string{
		"button: Edit S300 / Edit",
		"opened: Edit the task / https://example.com/489 / 案^,設計,実装,リリース no page,レビューv",
		"no page: リリース",
		"described: edit-categories-hint alert",
		"unchanged: 5 Edit the task https://example.com/489 [案,設計,実装,リリース,レビュー]",
		"moved up: 案^,設計,実装,レビュー,リリース no pagev / Move レビュー up / Moved レビュー to 4 of 5",
		"moved back: 5 Edit the task https://example.com/489 [案,設計,実装,リリース,レビュー]",
		"sent unchanged: 0",
		"focus back: task-edit",
		"at the top: 設計^,案,実装,リリース no page,レビューv / Move 設計 down",
		"title: 6 New title https://example.com/489 [案,設計,実装,リリース,レビュー]",
		`sent the title: {"taskProject":"proj","task":"S300","new":false,"rev":5,"title":"New title"}`,
		"order: 7 New title https://example.com/489 [案,設計,レビュー,実装,リリース]",
		`sent the order: {"taskProject":"proj","task":"S300","new":false,"rev":6,"categories":["案","設計","レビュー","実装","リリース"]}`,
		"groups: 案Add a page,設計Add a page,レビューAdd a page,実装Add a page,リリースAdd a pageNo pages yet",
		"dragged: リリース no page^,設計,レビュー,実装,案v",
		"let go: false",
		"words dragged: 0 / リリース no page^,設計,レビュー,実装,案v",
		"drag: 8 New title https://example.com/489 [リリース,設計,レビュー,実装,案]",
		"no grip without hover: true",
		"added twice: https://example.com/489,https://example.com/pull/9 / https://example.com/pull/9 is already among the URLs",
		"one bin: true true",
		"typed, not added: https://example.com/left is not added yet: press Enter to add it, or empty the field / true",
		"dropped: https://example.com/pull/9 / Take off https://example.com/pull/9 / Took off https://example.com/489",
		"urls: 9 New title https://example.com/489,https://example.com/pull/9 [リリース,設計,レビュー,実装,案]",
		`sent the URLs: {"taskProject":"proj","task":"S300","new":false,"rev":8,"urls":{"add":["https://example.com/pull/9"]}}`,
		"links: https://example.com/489 https://example.com/489,https://example.com/pull/9 https://example.com/pull/9",
		"links stacked: true true true false",
		"a long link wraps: true true true true",
		"an 11th: 10 / A task holds 10 URLs at most: take one off first / http or https URLs, 10 at most",
		`refused: Failed to save: "ftp://x" is not an http or https URL / true`,
		"not reached: Failed to save: Failed to fetch / true",
		"emptied: 10   [リリース,設計,レビュー,実装,案]",
		"opened over the drawn task: Later / 11",
		"stale: Typed / https://example.com/new / リリース no page^,設計,レビュー,実装,案v / Failed to save: task S300 has changed since it was read; the tasks are read again, so check and save again",
		"again: 13 Typed https://example.com/new [リリース,設計,レビュー,実装,案]",
		`sent stale and again: {"taskProject":"proj","task":"S300","new":false,"rev":11,"title":"Typed"} ; {"taskProject":"proj","task":"S300","new":false,"rev":12,"title":"Typed"}`,
		"stale URLs: https://example.com/mine,https://example.com/theirs",
		"urls again: 15 Typed https://example.com/theirs,https://example.com/mine [リリース,設計,レビュー,実装,案]",
		`sent the URLs again: {"taskProject":"proj","task":"S300","new":false,"rev":13,"urls":{"add":["https://example.com/mine"]}} ; {"taskProject":"proj","task":"S300","new":false,"rev":14,"urls":{"add":["https://example.com/mine"]}}`,
		"stale order: 設計^,レビュー,案,実装v",
		"order again: 17 Typed https://example.com/theirs,https://example.com/mine [設計,レビュー,案,実装]",
		"gone: This task was deleted elsewhere: close this and look at the task list / true",
		"gone, typed: This task was deleted elsewhere: close this and look at the task list",
		"G1: none",
		"narrow: true",
		"narrow moved: レビュー^,設計,案,実装v",
	}, "|"))
}
