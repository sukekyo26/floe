package browsertest_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/chromedp/chromedp/kb"

	"floe/internal/store"
)

// The task edit dialog adds categories by the add modal its button under the box opens over it, and removes those of no page: the button of a
// category with pages is there, not pressed, and says why. Enter adds and closes the add modal; a name there already, or of spaces, is said
// under its field, the modal open; Escape closes the add modal only. Refused as stale, the categories are as the person left them, without those another removed, with
// those another added, and with one the person removed back if it has come to have pages.
func TestUITaskEditCategories(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	project := "proj"
	for _, p := range [][2]string{{"plan-a", "案"}, {"design-a", "設計"}, {"build-a", "実装"}, {"review-a", "レビュー"}, {"memo-a", ""}, {"none-a", ""}} {
		s.floe(t, "publish", s.page(t, p[0]+".yaml", "T "+p[0], ""), "--slug", p[0])
		if _, err := st.Edit(p[0], store.Edit{Project: &project}); err != nil {
			t.Fatal(err)
		}
		if p[1] != "" {
			s.floe(t, "task", "--slug", p[0], "--task", "S300", "--category", p[1])
		}
	}
	s.floe(t, "task", "--project", "proj", "--task", "S300", "--title", "Edit the task", "--categories", "案", "--categories", "設計", "--categories", "実装", "--categories", "リリース", "--categories", "レビュー")
	s.floe(t, "task", "--slug", "none-a", "--task", "N1")
	// 実装's one page archived: a page all the same, so 実装 cannot be deleted
	yes := true
	if _, err := st.Edit("build-a", store.Edit{Archived: &yes}); err != nil {
		t.Fatal(err)
	}
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
		return strconv.Itoa(r.Rev) + " [" + strings.Join(r.Categories, ",") + "]"
	}
	// Each category, d when its remove button may be pressed, x when not
	const categories = "[...document.querySelectorAll('#edit-categories li')].map((li) => li.dataset.category + (li.querySelector('[data-testid=category-drop]').getAttribute('aria-disabled') === 'true' ? 'x' : 'd')).join(',')"
	const said = "document.getElementById('edit-category-said').textContent"
	const addSaid = "document.getElementById('edit-add-said').textContent"
	const last = "(document.getElementById('edit-add-dialog').open ? 'adding' : 'shut') + ' / ' + document.activeElement.id"
	open := func(task string) {
		t.Helper()
		b.navigate("/tasks?project=proj&task=" + task)
		b.until("the task's head", "!document.getElementById('task-head').hidden && document.getElementById('task-title').textContent.includes("+js(task)+")")
		// The test's script: every write of the task kept as sent and counted answered; revTo, when set, is the rev each sends
		b.eval(`window.sent = []; window.answered = 0; window.revTo = null; window.realFetch ??= window.fetch;
			window.fetch = (url, init) => {
				if (!String(url).includes('/api/tasks') || init?.method !== 'PATCH') return window.realFetch(url, init);
				if (window.revTo !== null) init = { ...init, body: JSON.stringify({ ...JSON.parse(init.body), rev: window.revTo }) };
				window.sent.push(init.body);
				return window.realFetch(url, init).finally(() => { window.answered++; });
			}`)
		b.eval("document.getElementById('task-edit').click()")
		b.until("the dialog", "document.getElementById('edit-task-dialog').open")
	}
	// Types a name in the add modal's field, opening it by the button under the box if it is not open, and presses Enter
	enter := func(name string) {
		t.Helper()
		b.eval("if (!document.getElementById('edit-add-dialog').open) document.getElementById('edit-category-plus').click()")
		b.until("the add modal", "document.getElementById('edit-add-dialog').open && document.activeElement.id === 'edit-add-new'")
		b.fill("#edit-add-new", name)
		b.pressKey(kb.Enter)
	}
	// enter, and the add modal closed as the name is added
	add := func(name string) {
		t.Helper()
		enter(name)
		b.until("added "+name, "!document.getElementById('edit-add-dialog').open")
	}
	// enter, and the refusal said under the add modal's field
	refused := func(name string) {
		t.Helper()
		enter(name)
		b.until("refused "+name, addSaid+" !== ''")
	}
	drop := func(category string) {
		t.Helper()
		b.eval("[...document.querySelectorAll('#edit-categories li')].find((li) => li.dataset.category === " + js(category) + ").querySelector('[data-testid=category-drop]').click()")
	}
	save := func(what string) {
		t.Helper()
		n := b.eval("window.answered")
		b.eval("document.getElementById('edit-task-submit').click()")
		b.until(what, "!document.getElementById('edit-task-dialog').open || (window.answered > "+n+" && !document.getElementById('edit-task-title').matches(':disabled'))")
	}
	closed := func(what string) {
		t.Helper()
		b.until(what, "!document.getElementById('edit-task-dialog').open")
	}

	// Opened: the last row adds, and a category's remove button is pressed only where it has no page, its name saying why.
	open("S300")
	say("opened", categories+" + ' / ' + "+last)
	say("plus", "document.getElementById('edit-category-plus').getAttribute('aria-label') + ' / ' + document.getElementById('edit-category-plus').title + ' / ' + document.getElementById('edit-categories-head').textContent")
	say("names", "[...document.querySelectorAll('#edit-categories [data-testid=category-drop]')].map((b) => b.getAttribute('aria-label') + '=' + b.title).join(', ')")
	// The remove button of a category with pages is reached by the keys; pressed, it says why under the rows and removes nothing.
	b.eval("document.querySelector('#edit-categories li [data-testid=category-down]').focus()")
	b.pressKey(kb.Tab)
	b.pressKey(kb.Enter)
	say("paged pressed", "document.activeElement.getAttribute('aria-label') + ' / ' + "+categories+" + ' / ' + "+said)
	// The button opens the add modal over the dialog, its field focused; Enter adds, closes it, and the focus is back on the button.
	b.eval("document.getElementById('edit-category-plus').click()")
	b.until("the add modal", "document.activeElement.id === 'edit-add-new'")
	say("pressed", last+" + ' / ' + "+said)
	add("  公開 ")
	b.until("the focus back", "document.activeElement.id === 'edit-category-plus'")
	say("added", categories+" + ' / ' + "+last+" + ' / ' + document.getElementById('edit-categories-said').textContent + ' / ' + document.getElementById('edit-task-dialog').open")
	// Not added: a name there already, or of spaces, said under the add modal's field, the modal open.
	refused("設計")
	say("there", categories+" + ' / ' + "+addSaid+" + ' / ' + document.getElementById('edit-add-new').getAttribute('aria-describedby') + ' / ' + "+last)
	b.fill("#edit-add-new", "")
	refused("   ")
	say("blank", addSaid)
	// Escape closes the add modal only, the focus back on the button.
	b.pressKey(kb.Escape)
	b.until("the add modal closed", "!document.getElementById('edit-add-dialog').open")
	b.until("the focus back", "document.activeElement.id === 'edit-category-plus'")
	say("escape", last+" + ' / ' + document.getElementById('edit-task-dialog').open")
	// Removed: the focus on a button of the row after that can be pressed, or of the row before; said.
	drop("リリース")
	say("dropped", categories+" + ' / ' + document.activeElement.getAttribute('aria-label') + ' / ' + document.getElementById('edit-categories-said').textContent")
	drop("公開")
	say("dropped last", categories+" + ' / ' + document.activeElement.getAttribute('aria-label')")
	save("saved")
	closed("closed")
	out = append(out, "saved: "+row("S300"))
	say("sent", "sent.join(' ; ')")
	// Added and removed is no change.
	open("S300")
	add("公開")
	drop("公開")
	save("unchanged")
	closed("closed")
	say("unchanged", "sent.length")

	// A category removed that came to have a page meanwhile: the page's coming wrote the task, so the save is refused as stale
	// and said; nothing written.
	s.floe(t, "task", "--project", "proj", "--task", "S300", "--categories", "案", "--categories", "設計", "--categories", "実装", "--categories", "リリース", "--categories", "レビュー")
	open("S300")
	drop("リリース")
	s.floe(t, "task", "--slug", "memo-a", "--task", "S300", "--category", "リリース")
	save("paged meanwhile")
	say("paged meanwhile", "document.getElementById('edit-task-error').textContent")
	b.pressKey(kb.Escape)
	closed("closed")
	out = append(out, "paged meanwhile, written: "+row("S300"))
	// The same of the rev written since, as a modal that read the task after the page came would send: refused, not as stale but
	// as the category has pages, and said; nothing written.
	s.floe(t, "task", "--slug", "memo-a", "--task", "")
	s.floe(t, "task", "--project", "proj", "--task", "S300", "--categories", "案", "--categories", "設計", "--categories", "実装", "--categories", "リリース", "--categories", "レビュー")
	open("S300")
	drop("リリース")
	s.floe(t, "task", "--slug", "memo-a", "--task", "S300", "--category", "リリース")
	b.eval("window.revTo = " + strings.Fields(row("S300"))[0])
	save("paged since read")
	say("paged since read", "document.getElementById('edit-task-error').textContent")
	b.pressKey(kb.Escape)
	closed("closed")
	out = append(out, "paged since read, written: "+row("S300"))
	// Stale, a category added while another removed one of no page: the one added stays, the other goes.
	s.floe(t, "task", "--project", "proj", "--task", "S300", "--categories", "案", "--categories", "設計", "--categories", "実装", "--categories", "リリース", "--categories", "検証", "--categories", "レビュー")
	open("S300")
	add("公開")
	s.floe(t, "task", "--project", "proj", "--task", "S300", "--categories", "案", "--categories", "設計", "--categories", "実装", "--categories", "リリース", "--categories", "レビュー")
	save("stale, added")
	say("stale, added", categories)
	save("saved again")
	closed("closed")
	out = append(out, "saved again: "+row("S300"))

	// No categories: the last row is there all the same.
	open("N1")
	say("no categories", "document.getElementById('edit-categories').hidden + ' / ' + document.getElementById('edit-categories-none').hidden + ' / ' + "+last)
	add("案")
	save("saved the first")
	closed("closed")
	out = append(out, "first: "+row("N1"))

	check(t, "task edit categories", out, strings.Join([]string{
		"opened: 案x,設計x,実装x,リリースd,レビューx / shut / edit-task-title",
		"plus: Add a category / Add a category / Categories",
		"names: 案 has pages, so it cannot be deleted=案 has pages, so it cannot be deleted, 設計 has pages, so it cannot be deleted=設計 has pages, so it cannot be deleted, 実装 has pages, so it cannot be deleted=実装 has pages, so it cannot be deleted, Delete リリース=Delete リリース, レビュー has pages, so it cannot be deleted=レビュー has pages, so it cannot be deleted",
		"paged pressed: 案 has pages, so it cannot be deleted / 案x,設計x,実装x,リリースd,レビューx / 案 has pages, so it cannot be deleted",
		"pressed: adding / edit-add-new / ",
		"added: 案x,設計x,実装x,リリースd,レビューx,公開d / shut / edit-category-plus / Added 公開 at 6 of 6 / true",
		"there: 案x,設計x,実装x,リリースd,レビューx,公開d / 設計 is already among the categories / edit-add-said / adding / edit-add-new",
		"blank: Write the category's name",
		"escape: shut / edit-category-plus / true",
		"dropped: 案x,設計x,実装x,レビューx,公開d / Move レビュー up / Deleted リリース",
		"dropped last: 案x,設計x,実装x,レビューx / Move レビュー up",
		"saved: 6 [案,設計,実装,レビュー]",
		`sent: {"taskProject":"proj","task":"S300","new":false,"rev":5,"categories":["案","設計","実装","レビュー"]}`,
		"unchanged: 0",
		"paged meanwhile: Failed to save: task S300 has changed since it was read; the tasks are read again, so check and save again",
		"paged meanwhile, written: 8 [案,設計,実装,リリース,レビュー]",
		"paged since read: Failed to save: category \"リリース\" of task S300 in project proj has pages (memo-a): take them off it first, or keep it in the order",
		"paged since read, written: 9 [案,設計,実装,リリース,レビュー]",
		"stale, added: 案x,設計x,実装x,リリースx,レビューx,公開d",
		"saved again: 12 [案,設計,実装,リリース,レビュー,公開]",
		"no categories: true / false / shut / edit-task-title",
		"first: 2 [案]",
	}, "|"))
}
