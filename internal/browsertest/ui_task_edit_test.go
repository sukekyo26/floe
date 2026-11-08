package browsertest_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/chromedp/chromedp/kb"

	"github.com/sukekyo26/floe/internal/store"
)

// A task's page edits the task's own fields in one dialog: its title, URL and the order of its steps, sending what was changed alone,
// with the rev it showed. The steps are reordered, their names kept, by buttons that keep the focus and say where a step went, or by a
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
			s.floe(t, "work", p[0], "--task", "S300", "--step", p[1])
		}
	}
	s.floe(t, "work", "--project", "proj", "--task", "S300", "--title", "Edit the task", "--url", "https://example.com/489", "--steps", "案,設計,実装,リリース")
	s.floe(t, "work", "gone-a", "--task", "G1", "--step", "x")
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
		return strconv.Itoa(r.Rev) + " " + r.Title + " " + r.URL + " [" + strings.Join(r.Steps, ",") + "]"
	}
	const steps = "[...document.querySelectorAll('#edit-steps li')].map((li) => li.querySelector('.step-name').textContent + (li.querySelector('.step-up').disabled ? '^' : '') + (li.querySelector('.step-down').disabled ? 'v' : '')).join(',')"
	const fields = "[document.getElementById('edit-task-title').value, document.getElementById('edit-task-url').value].join(' / ') + ' / ' + " + steps
	const errorText = "(document.getElementById('edit-task-error').hidden ? '-' : document.getElementById('edit-task-error').textContent)"
	open := func(task string) {
		t.Helper()
		b.navigate("/tasks?project=proj&task=" + task)
		b.until("the task's head", "!document.getElementById('task-head').hidden && document.getElementById('task-title').textContent.includes("+js(task)+")")
		// The bodies of the task's writes, as sent
		b.eval("window.sent = []; window.realFetch ??= window.fetch; window.fetch = (url, init) => { if (String(url).includes('/api/tasks') && init?.method === 'PATCH') sent.push(init.body); return window.realFetch(url, init); }")
		b.eval("document.getElementById('task-edit').focus(); document.getElementById('task-edit').click()")
		b.until("the dialog", "document.getElementById('edit-task-dialog').open")
	}
	field := func(id, value string) {
		t.Helper()
		b.eval("document.getElementById(" + js(id) + ").value = " + js(value) + "; document.getElementById(" + js(id) + ").dispatchEvent(new Event('input', { bubbles: true }))")
	}
	press := func(step, dir string) {
		t.Helper()
		b.eval("[...document.querySelectorAll('#edit-steps li')].find((li) => li.dataset.step === " + js(step) + ").querySelector('.step-" + dir + "').click()")
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
	// A drag of a step's grip onto another step, in its upper half or its lower one, as a browser sends it
	drag := func(from, onto string, after bool) {
		t.Helper()
		b.eval(`(() => {
			const rows = [...document.querySelectorAll('#edit-steps li')];
			const li = rows.find((x) => x.dataset.step === ` + js(from) + `), to = rows.find((x) => x.dataset.step === ` + js(onto) + `);
			const box = to.getBoundingClientRect(), y = ` + strconv.FormatBool(after) + ` ? box.bottom - 2 : box.top + 2;
			const data = new DataTransfer();
			li.querySelector('.step-grip').dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
			li.dispatchEvent(new DragEvent('dragstart', { bubbles: true, dataTransfer: data }));
			to.dispatchEvent(new DragEvent('dragover', { bubbles: true, cancelable: true, dataTransfer: data, clientY: y }));
			to.dispatchEvent(new DragEvent('drop', { bubbles: true, cancelable: true, dataTransfer: data, clientY: y }));
			li.dispatchEvent(new DragEvent('dragend', { bubbles: true, dataTransfer: data }));
		})()`)
	}

	// Opened: the title, URL and steps written in order, those of no page marked, then those of pages alone; the ends move no further.
	open("S300")
	say("button", "document.getElementById('task-edit').getAttribute('aria-label') + ' / ' + document.getElementById('task-edit').textContent.trim()")
	say("opened", fields)
	say("no page", "[...document.querySelectorAll('#edit-steps li')].filter((li) => li.querySelector('.step-name small')).map((li) => li.dataset.step).join(',')")
	say("described", "document.getElementById('edit-steps').getAttribute('aria-describedby') + ' ' + document.getElementById('edit-task-error').getAttribute('role')")
	// Nothing changed, nothing is sent; moved and moved back is no change.
	save("closed unchanged")
	out = append(out, "unchanged: "+row("S300"))
	b.eval("document.getElementById('task-edit').click()")
	b.until("the dialog", "document.getElementById('edit-task-dialog').open")
	press("レビュー", "up")
	say("moved up", steps+" + ' / ' + document.activeElement.getAttribute('aria-label') + ' / ' + document.getElementById('edit-steps-said').textContent")
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

	// A step at the top, its up gone: the focus goes to its down.
	open("S300")
	press("設計", "up")
	say("at the top", steps+" + ' / ' + document.activeElement.getAttribute('aria-label')")
	b.pressKey(kb.Escape)
	closed("closed")

	// The title alone changed is sent alone.
	open("S300")
	field("edit-task-title", "New title")
	save("saved the title")
	closed("closed")
	out = append(out, "title: "+row("S300"))
	say("sent the title", "sent.join(' ; ')")
	// Reordered by buttons: the pages' steps follow, and a step of pages alone is written too.
	open("S300")
	press("レビュー", "up")
	press("レビュー", "up")
	save("saved the order")
	closed("closed")
	out = append(out, "order: "+row("S300"))
	say("sent the order", "sent.join(' ; ')")
	say("groups", "[...document.querySelectorAll('#list li.group')].map((li) => li.textContent.trim()).join(',')")
	// Reordered by a drag: before a step in its upper half, after it in its lower one.
	open("S300")
	drag("リリース", "案", false)
	drag("案", "実装", true)
	say("dragged", steps)
	// A grip pressed and let go without a drag leaves its row no drag from anywhere; a drag of words selected, from a text node, moves nothing
	// and throws nothing.
	b.eval(`(() => {
		const li = document.querySelector('#edit-steps li');
		li.querySelector('.step-grip').dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
		li.dispatchEvent(new PointerEvent('pointerup', { bubbles: true }));
	})()`)
	say("let go", "[...document.querySelectorAll('#edit-steps li')].some((li) => li.draggable)")
	b.eval(`(() => {
		window.thrown = 0;
		addEventListener('error', () => { thrown++; });
		const word = document.querySelectorAll('#edit-steps .step-name')[1].firstChild;
		const data = new DataTransfer();
		word.dispatchEvent(new DragEvent('dragstart', { bubbles: true, dataTransfer: data }));
		const to = document.querySelector('#edit-steps li');
		to.dispatchEvent(new DragEvent('drop', { bubbles: true, cancelable: true, dataTransfer: data, clientY: to.getBoundingClientRect().top + 2 }));
	})()`)
	say("words dragged", "thrown + ' / ' + "+steps)
	save("saved the drag")
	closed("closed")
	out = append(out, "drag: "+row("S300"))
	// A screen without hover shows no grip: the rule is there.
	say("no grip without hover", "[...document.styleSheets].flatMap((sh) => [...sh.cssRules]).some((r) => r.media?.mediaText === '(hover: none)' && [...r.cssRules].some((x) => x.selectorText === '.step-order .step-grip' && x.style.display === 'none'))")

	// Refused by the store: the dialog stays open with its words.
	open("S300")
	field("edit-task-url", "ftp://x")
	save("refused")
	say("refused", errorText+" + ' / ' + document.getElementById('edit-task-dialog').open")
	field("edit-task-url", "")
	field("edit-task-title", "")
	save("saved empty")
	closed("closed")
	out = append(out, "emptied: "+row("S300"))
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
	// Reordered while a step of no page was taken off: the order stays, without it.
	open("S300")
	press("実装", "down")
	s.floe(t, "work", "--project", "proj", "--task", "S300", "--steps", "案,設計")
	save("stale order")
	say("stale order", steps)
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
	say("narrow moved", steps)
	b.pressKey(kb.Escape)

	check(t, "task edit", out, strings.Join([]string{
		"button: Edit S300 / Edit",
		"opened: Edit the task / https://example.com/489 / 案^,設計,実装,リリース no page,レビューv",
		"no page: リリース",
		"described: edit-steps-hint alert",
		"unchanged: 5 Edit the task https://example.com/489 [案,設計,実装,リリース]",
		"moved up: 案^,設計,実装,レビュー,リリース no pagev / Move レビュー up / Moved レビュー to 4 of 5",
		"moved back: 5 Edit the task https://example.com/489 [案,設計,実装,リリース]",
		"sent unchanged: 0",
		"focus back: task-edit",
		"at the top: 設計^,案,実装,リリース no page,レビューv / Move 設計 down",
		"title: 6 New title https://example.com/489 [案,設計,実装,リリース]",
		`sent the title: {"new":false,"rev":5,"title":"New title"}`,
		"order: 7 New title https://example.com/489 [案,設計,レビュー,実装,リリース]",
		`sent the order: {"new":false,"rev":6,"steps":["案","設計","レビュー","実装","リリース"]}`,
		"groups: 案,設計,レビュー,実装",
		"dragged: リリース no page^,設計,レビュー,実装,案v",
		"let go: false",
		"words dragged: 0 / リリース no page^,設計,レビュー,実装,案v",
		"drag: 8 New title https://example.com/489 [リリース,設計,レビュー,実装,案]",
		"no grip without hover: true",
		`refused: Could not save: "ftp://x" is not an http or https URL / true`,
		"emptied: 9   [リリース,設計,レビュー,実装,案]",
		"opened over the drawn task: Later / 10",
		"stale: Typed / https://example.com/new / リリース no page^,設計,レビュー,実装,案v / task S300 has changed since it was read: the tasks are read again, so check and save again",
		"again: 12 Typed https://example.com/new [リリース,設計,レビュー,実装,案]",
		`sent stale and again: {"new":false,"rev":10,"title":"Typed"} ; {"new":false,"rev":11,"title":"Typed"}`,
		"stale order: 設計^,レビュー,案,実装v",
		"order again: 14 Typed https://example.com/new [設計,レビュー,案,実装]",
		"gone: This task was removed elsewhere: close this and look at the task list / true",
		"gone, typed: This task was removed elsewhere: close this and look at the task list",
		"G1: none",
		"narrow: true",
		"narrow moved: レビュー^,設計,案,実装v",
	}, "|"))
}
