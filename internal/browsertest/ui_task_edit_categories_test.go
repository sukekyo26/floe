package browsertest_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/chromedp/chromedp/kb"

	"floe/internal/store"
)

// The task edit dialog adds categories by its last row, which turns into a field, and removes those of no page: the button of a category with pages
// is there, not pressed, and says why. Enter adds, not while the input method has it; a name there already, or of spaces, is said under the
// row; words left in the field stop the save. Refused as stale, the categories are as the person left them, without those another removed, with
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
			s.floe(t, "work", p[0], "--task", "S300", "--category", p[1])
		}
	}
	s.floe(t, "work", "--project", "proj", "--task", "S300", "--title", "Edit the task", "--categories", "案,設計,実装,リリース")
	s.floe(t, "work", "none-a", "--task", "N1")
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
	const categories = "[...document.querySelectorAll('#edit-categories li')].map((li) => li.dataset.category + (li.querySelector('.category-drop').getAttribute('aria-disabled') === 'true' ? 'x' : 'd')).join(',')"
	const said = "document.getElementById('edit-category-said').textContent"
	const last = "(document.getElementById('edit-category-plus').hidden ? 'field' : 'plus') + ' / ' + document.activeElement.id"
	open := func(task string) {
		t.Helper()
		b.navigate("/tasks?project=proj&task=" + task)
		b.until("the task's head", "!document.getElementById('task-head').hidden && document.getElementById('task-title').textContent.includes("+js(task)+")")
		b.eval("window.sent = []; window.realFetch ??= window.fetch; window.fetch = (url, init) => { if (String(url).includes('/api/tasks') && init?.method === 'PATCH') sent.push(init.body); return window.realFetch(url, init); }")
		b.eval("document.getElementById('task-edit').click()")
		b.until("the dialog", "document.getElementById('edit-task-dialog').open")
	}
	// Types a name in the last row's field, as a person does (the field, not the button), and a key on it: composing, as the input method
	// has it
	key := func(name, k string, composing bool) {
		t.Helper()
		b.eval(`(() => {
			const input = document.getElementById('edit-category-new');
			if (input.hidden) document.getElementById('edit-category-plus').click();
			input.value = ` + js(name) + `;
			input.dispatchEvent(new Event('input', { bubbles: true }));
			input.dispatchEvent(new KeyboardEvent('keydown', { key: ` + js(k) + `, bubbles: true, cancelable: true, isComposing: ` + strconv.FormatBool(composing) + ` }));
		})()`)
	}
	drop := func(category string) {
		t.Helper()
		b.eval("[...document.querySelectorAll('#edit-categories li')].find((li) => li.dataset.category === " + js(category) + ").querySelector('.category-drop').click()")
	}
	save := func(what string) {
		t.Helper()
		b.eval("document.getElementById('edit-task-error').hidden = true; document.getElementById('edit-task').requestSubmit()")
		b.until(what, "!document.getElementById('edit-task-dialog').open || !document.getElementById('edit-task-error').hidden || document.getElementById('edit-category-new').validationMessage")
	}
	closed := func(what string) {
		t.Helper()
		b.until(what, "!document.getElementById('edit-task-dialog').open")
	}

	// Opened: the last row adds, and a category's remove button is pressed only where it has no page, its name saying why.
	open("S300")
	say("opened", categories+" + ' / ' + "+last)
	say("plus", "document.getElementById('edit-category-plus').getAttribute('aria-label') + ' / ' + document.getElementById('edit-category-plus').title + ' / ' + document.getElementById('edit-categories-head').textContent")
	say("names", "[...document.querySelectorAll('#edit-categories .category-drop')].map((b) => b.getAttribute('aria-label') + '=' + b.title).join(', ')")
	// The remove button of a category with pages is reached by the keys; pressed, it says why under the rows and removes nothing.
	b.eval("document.querySelector('#edit-categories li .category-down').focus()")
	b.pressKey(kb.Tab)
	b.pressKey(kb.Enter)
	say("paged pressed", "document.activeElement.getAttribute('aria-label') + ' / ' + "+categories+" + ' / ' + "+said)
	// The plus turns into the field, which has the focus; Enter adds, and the field stays, empty, for the next.
	b.eval("document.getElementById('edit-category-plus').click()")
	say("pressed", last)
	key("  公開 ", "Enter", false)
	say("added", categories+" + ' / ' + "+last+" + ' / ' + document.getElementById('edit-category-new').value + ' / ' + document.getElementById('edit-categories-said').textContent + ' / ' + document.getElementById('edit-task-dialog').open")
	// Not added: a name there already, or of spaces, said under the row; nor while the input method has the key.
	key("設計", "Enter", false)
	say("there", categories+" + ' / ' + "+said+" + ' / ' + document.getElementById('edit-category-new').getAttribute('aria-describedby')")
	key("   ", "Enter", false)
	say("blank", said)
	key("検証", "Enter", true)
	say("composing", categories)
	// Escape: not while composing; else the field is the plus again, the dialog open, the focus on the plus.
	key("検証", "Escape", true)
	say("escape composing", last)
	key("検証", "Escape", false)
	say("escape", last+" + ' / ' + document.getElementById('edit-task-dialog').open + ' / ' + document.getElementById('edit-category-new').value")
	// Left empty, the field turns back; with words in it, it stays.
	b.eval("document.getElementById('edit-category-plus').click(); document.getElementById('edit-task-title').focus()")
	say("left empty", last)
	key("検証", "x", false)
	b.eval("document.getElementById('edit-task-title').focus()")
	say("left with words", last)
	// Words left stop the save, said on the field, until typed again.
	save("words left")
	say("words left", "document.getElementById('edit-category-new').validationMessage + ' / ' + document.getElementById('edit-task-dialog').open + ' / ' + sent.length")
	b.eval("const f = document.getElementById('edit-category-new'); f.value = ''; f.dispatchEvent(new Event('input', { bubbles: true }))")
	say("typed again", "document.getElementById('edit-category-new').validationMessage === ''")
	// A name there already, left in the field, stops the save saying it is there, not to press Enter.
	b.eval("const f = document.getElementById('edit-category-new'); f.value = '設計'; f.dispatchEvent(new Event('input', { bubbles: true }))")
	save("there left")
	say("there left", "document.getElementById('edit-category-new').validationMessage + ' / ' + sent.length")
	b.eval("const f = document.getElementById('edit-category-new'); f.value = ''; f.dispatchEvent(new Event('input', { bubbles: true }))")
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
	key("公開", "Enter", false)
	drop("公開")
	save("unchanged")
	closed("closed")
	say("unchanged", "sent.length")

	// A category removed that came to have a page meanwhile, which moves no rev: saved removed, it comes back after those written, from its
	// page (LIMIT).
	s.floe(t, "work", "--project", "proj", "--task", "S300", "--categories", "案,設計,実装,リリース")
	open("S300")
	drop("リリース")
	s.floe(t, "work", "memo-a", "--task", "S300", "--category", "リリース")
	save("paged meanwhile")
	closed("closed")
	out = append(out, "paged meanwhile, written: "+row("S300"))
	open("S300")
	say("paged meanwhile, opened", categories)
	b.pressKey(kb.Escape)
	closed("closed")
	// Stale, a category added while another removed one of no page: the one added stays, the other goes.
	s.floe(t, "work", "--project", "proj", "--task", "S300", "--categories", "案,設計,実装,リリース,検証")
	open("S300")
	key("公開", "Enter", false)
	s.floe(t, "work", "--project", "proj", "--task", "S300", "--categories", "案,設計,実装,リリース")
	save("stale, added")
	say("stale, added", categories)
	save("saved again")
	closed("closed")
	out = append(out, "saved again: "+row("S300"))

	// No categories: the last row is there all the same.
	open("N1")
	say("no categories", "document.getElementById('edit-categories').hidden + ' / ' + document.getElementById('edit-categories-none').hidden + ' / ' + "+last)
	key("案", "Enter", false)
	save("saved the first")
	closed("closed")
	out = append(out, "first: "+row("N1"))

	check(t, "task edit categories", out, strings.Join([]string{
		"opened: 案x,設計x,実装x,リリースd,レビューx / plus / edit-task-title",
		"plus: Add a category / Add a category / Categories",
		"names: 案 has pages, so it cannot be deleted=案 has pages, so it cannot be deleted, 設計 has pages, so it cannot be deleted=設計 has pages, so it cannot be deleted, 実装 has pages, so it cannot be deleted=実装 has pages, so it cannot be deleted, Delete リリース=Delete リリース, レビュー has pages, so it cannot be deleted=レビュー has pages, so it cannot be deleted",
		"paged pressed: 案 has pages, so it cannot be deleted / 案x,設計x,実装x,リリースd,レビューx / 案 has pages, so it cannot be deleted",
		"pressed: field / edit-category-new",
		"added: 案x,設計x,実装x,リリースd,レビューx,公開d / field / edit-category-new /  / Added 公開 at 6 of 6 / true",
		"there: 案x,設計x,実装x,リリースd,レビューx,公開d / 設計 is already among the categories / edit-category-said",
		"blank: Write the category's name",
		"composing: 案x,設計x,実装x,リリースd,レビューx,公開d",
		"escape composing: field / edit-category-new",
		"escape: plus / edit-category-plus / true / ",
		"left empty: plus / edit-task-title",
		"left with words: field / edit-task-title",
		"words left: 検証 is not added yet: press Enter to add it, or empty the field / true / 0",
		"typed again: true",
		"there left: 設計 is already among the categories / 0",
		"dropped: 案x,設計x,実装x,レビューx,公開d / Move レビュー up / Deleted リリース",
		"dropped last: 案x,設計x,実装x,レビューx / Move レビュー up",
		"saved: 3 [案,設計,実装,レビュー]",
		`sent: {"new":false,"rev":2,"categories":["案","設計","実装","レビュー"]}`,
		"unchanged: 0",
		"paged meanwhile, written: 5 [案,設計,実装,レビュー]",
		"paged meanwhile, opened: 案x,設計x,実装x,レビューx,リリースx",
		"stale, added: 案x,設計x,実装x,リリースx,レビューx,公開d",
		"saved again: 8 [案,設計,実装,リリース,レビュー,公開]",
		"no categories: true / false / plus / edit-task-title",
		"first: 2 [案]",
	}, "|"))
}
