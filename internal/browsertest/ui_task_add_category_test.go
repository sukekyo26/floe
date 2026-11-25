package browsertest_test

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp/kb"

	"floe/internal/store"
)

// TestUITaskAddCategory names a new category in the dialog that adds a page: it writes nothing, is chosen, and is made at the end of the
// task's categories by the one write that puts the page; closed without one, it is not made.
func TestUITaskAddCategory(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	publish := func(slug, task string) {
		t.Helper()
		s.floe(t, "publish", s.page(t, slug+".yaml", "T "+slug, ""), "--slug", slug)
		project := "proj"
		if _, err := st.Edit(slug, store.Edit{Project: &project}); err != nil {
			t.Fatal(err)
		}
		if task != "" {
			s.floe(t, "task", "--slug", slug, "--task", task)
		}
	}
	publish("own", "S300")
	for _, slug := range []string{"memo-a", "memo-b", "memo-c", "memo-d", "memo-e", "memo-f", "memo-g"} {
		publish(slug, "")
	}
	publish("other", "S301")
	s.floe(t, "task", "--project", "proj", "--task", "S300", "--title", "Title", "--categories", "設計", "--categories", "レビュー")
	b.size(1280, 900, false)
	defer b.unsize()

	var out []string
	say := func(what, expr string) {
		t.Helper()
		out = append(out, what+": "+b.eval(expr))
	}
	// The written row's rev, categories
	row := func() string {
		t.Helper()
		all, err := st.ReadTasks()
		if err != nil {
			t.Fatal(err)
		}
		r := all["proj"]["S300"]
		return strconv.Itoa(r.Rev) + " " + strings.Join(r.Categories, ",")
	}
	const chosen = "[...document.getElementById('add-page-category').options].map((o) => (o.selected ? '*' : '') + o.textContent).join(',')"
	const fresh = "document.getElementById('add-page-fresh').textContent"
	const at = "document.activeElement.id"
	const reached = "document.getElementById('add-page-q').getAttribute('aria-activedescendant')"
	const isOpen = "[...document.querySelectorAll('dialog:open')].map((d) => d.id).join(' over ')"
	open := func() {
		t.Helper()
		b.navigate("/tasks?project=proj&task=S300")
		b.until("the rows", "document.querySelector('#list li[data-slug]')")
		b.eval("window.sent = []; window.realFetch ??= window.fetch; window.fetch = (u, i) => { if (i?.method === 'PATCH') sent.push(i.body); return realFetch(u, i); }")
	}
	until := func(id string, opened bool) {
		t.Helper()
		not := "!"
		if opened {
			not = ""
		}
		b.until(id, not+"document.getElementById("+js(id)+").open")
	}
	focus := func(sel string) {
		t.Helper()
		b.eval("document.querySelector(" + js(sel) + ").focus()")
	}
	openPage := func() {
		t.Helper()
		focus("#task-add-page")
		b.pressKey(kb.Enter)
		until("add-page-dialog", true)
	}
	closePage := func() {
		t.Helper()
		b.eval("document.querySelector('#add-page-dialog [data-close]').click()")
		until("add-page-dialog", false)
	}
	openAdd := func() {
		t.Helper()
		focus("#add-category-plus")
		b.pressKey(kb.Enter)
		until("add-category-dialog", true)
		b.until("the field", at+" === 'add-category-new'")
	}
	add := func(name string) {
		t.Helper()
		openAdd()
		b.fill("#add-category-new", name)
		b.pressKey(kb.Enter)
		until("add-category-dialog", false)
	}

	// By the keys alone, put
	open()
	openPage()
	say("opened", chosen+" + ' / ' + "+fresh+" + ' / ' + "+at)
	b.pressKey(kb.Tab, kb.ModifierShift)
	say("back by Tab", at)
	b.pressKey(kb.Enter)
	until("add-category-dialog", true)
	b.until("the field", at+" === 'add-category-new'")
	b.fill("#add-category-new", "デザイン")
	b.pressKey(kb.Enter)
	until("add-category-dialog", false)
	say("added", chosen+" + ' / ' + "+fresh+" + ' / ' + "+at+" + ' / sent ' + sent.length")
	b.fill("#add-page-q", "memo-a")
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.Enter)
	until("add-page-dialog", false)
	b.until("drawn", "document.querySelector('#list li[data-slug^=memo]')")
	say("put", "sent.map((x) => x.replace(/\"rev\":\\d+/, '\"rev\":n').replace(/memo-./, 'memo')).join(' ; ') + ' / ' + "+at)
	out = append(out, "written: "+strings.SplitN(row(), " ", 2)[1])
	say("headed last", "[...document.querySelectorAll('#list [data-testid=group] [role=heading]')].at(-1).textContent")

	// Another takes its place, one chosen instead keeps it among the choices, and closed none is made
	openPage()
	add("一")
	add("二")
	say("another", chosen+" + ' / ' + "+fresh)
	chooseValue(b, "add-page-category", "設計")
	b.until("the line gone", fresh+" === ''")
	say("chosen instead", chosen)
	b.eval("sent.length = 0")
	closePage()
	openPage()
	say("opened again", chosen+" + ' / ' + "+fresh+" + ' / sent ' + sent.length")
	out = append(out, "not made: "+strings.SplitN(row(), " ", 2)[1])

	// Refused, the modal open: none, spaces, one there (with spaces around too), the one added, but not one apart by its case
	add("Add")
	openAdd()
	for _, name := range []string{"", "  ", "設計", "  設計 ", "Add"} {
		b.fill("#add-category-new", name)
		b.pressKey(kb.Enter)
		b.until("refused "+name, "document.getElementById('add-category-said').textContent !== ''")
		say("refused "+strconv.Quote(name), "document.getElementById('add-category-said').textContent + ' / ' + "+isOpen)
	}
	b.fill("#add-category-new", "ADD")
	b.pressKey(kb.Enter)
	until("add-category-dialog", false)
	say("a case apart", chosen+" + ' / sent ' + sent.length")
	closePage()

	// Enter twice, as it is added: the row reached before is put no more; reached anew, it is
	openPage()
	b.fill("#add-page-q", "memo-b")
	b.pressKey(kb.ArrowDown)
	say("reached", reached)
	openAdd()
	b.fill("#add-category-new", "速い")
	b.pressKey(kb.Enter)
	b.pressKey(kb.Enter)
	say("Enter twice", chosen+" + ' / ' + "+reached+" + ' / ' + "+isOpen+" + ' / sent ' + sent.length")
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.Enter)
	until("add-page-dialog", false)
	say("then put", "sent.length")
	b.eval("sent.length = 0")

	// Written elsewhere meanwhile: refused, the one added still chosen; a key held does not send again before it is read; then put once
	openPage()
	b.fill("#add-page-q", "memo-c")
	add("後")
	s.floe(t, "task", "--project", "proj", "--task", "S300", "--title", "Other")
	b.pressKey(kb.ArrowDown)
	b.keyDown(kb.Enter, false)
	b.until("refused", "!document.getElementById('add-page-error').hidden && "+at+" === 'add-page-q'")
	say("stale", chosen+" + ' / ' + "+isOpen+" + ' / sent ' + sent.length")
	for range 3 {
		b.keyDown(kb.Enter, true)
		b.painted()
	}
	b.keyUp(kb.Enter)
	say("held on", isOpen+" + ' / sent ' + sent.length")
	b.pressKey(kb.Enter)
	until("add-page-dialog", false)
	say("then", "sent.length")
	out = append(out, "written: "+strings.SplitN(row(), " ", 2)[1])
	b.eval("sent.length = 0")
	// Made by another meanwhile: one category of the name
	openPage()
	b.fill("#add-page-q", "memo-d")
	add("同じ")
	s.floe(t, "task", "--project", "proj", "--task", "S300", "--categories", "設計", "--categories", "レビュー", "--categories", "デザイン", "--categories", "速い", "--categories", "後", "--categories", "同じ")
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.Enter)
	b.until("refused", "!document.getElementById('add-page-error').hidden")
	b.pressKey(kb.Enter)
	until("add-page-dialog", false)
	out = append(out, "written once: "+strings.SplitN(row(), " ", 2)[1])
	b.eval("sent.length = 0")

	// Presses double: nothing put, and nothing under the modal pressed
	press := func(sel string, counts ...int) {
		t.Helper()
		b.still()
		r := b.rect("document.querySelector(" + js(sel) + ")")
		for _, count := range counts {
			b.mouse(input.DispatchMouseEventTypeMousePressed, r.X+r.Width/2, r.Y+r.Height/2, count)
			b.mouse(input.DispatchMouseEventTypeMouseReleased, r.X+r.Width/2, r.Y+r.Height/2, count)
		}
		b.painted()
	}
	openPage()
	press("#add-category-plus", 1, 2)
	say("plus double", isOpen+" + ' / ' + "+at+" + ' / sent ' + sent.length")
	press("#add-category-dialog [data-close]", 1, 2)
	say("cancel double", isOpen+" + ' / ' + "+at+" + ' / sent ' + sent.length")
	openAdd()
	b.fill("#add-category-new", "二度")
	press("#add-category-submit", 1, 2)
	say("add double", isOpen+" + ' / ' + "+at+" + ' / sent ' + sent.length")
	openAdd()
	b.still()
	// The backdrop over a row under it
	x := b.eval(`(() => { for (const li of document.querySelectorAll('#add-page-list > li')) { const r = li.getBoundingClientRect(), x = r.left + r.width / 2, y = r.top + r.height / 2; if (document.elementFromPoint(x, y)?.id === 'add-category-dialog' && document.elementFromPoint(x, y).tagName === 'DIALOG') return JSON.stringify({ X: x, Y: y }); } return ''; })()`)
	var over struct{ X, Y float64 }
	if err := json.Unmarshal([]byte(x), &over); err != nil {
		t.Fatalf("no row under the backdrop: %q", x)
	}
	for _, count := range []int{1, 2} {
		b.mouse(input.DispatchMouseEventTypeMousePressed, over.X, over.Y, count)
		b.mouse(input.DispatchMouseEventTypeMouseReleased, over.X, over.Y, count)
	}
	b.painted()
	say("backdrop double", isOpen+" + ' / ' + "+at+" + ' / sent ' + sent.length")
	closePage()

	// Edit's own modal and this one's stand apart: what one adds, the other does not show
	b.eval("document.getElementById('task-edit').click()")
	until("edit-task-dialog", true)
	focus("#edit-category-plus")
	b.pressKey(kb.Enter)
	until("edit-add-dialog", true)
	b.fill("#edit-add-new", "編集の")
	b.pressKey(kb.Enter)
	until("edit-add-dialog", false)
	b.eval("document.querySelector('#edit-task-dialog [data-close]').click()")
	until("edit-task-dialog", false)
	openPage()
	add("ページの")
	say("apart", chosen+" + ' / ' + document.getElementById('edit-add-dialog').open")
	closePage()

	// In a window narrower: in the modal, the button beside its choices or under them
	for _, width := range []int64{1280, 384, 320} {
		b.size(width, 900, false)
		open()
		openPage()
		add("長い名前の区分けのための言葉を並べた長い一つ")
		b.still()
		say(fmt.Sprintf("width %d", width), `(() => { const d = document.getElementById('add-page-dialog'), s = document.getElementById('add-page-category').getBoundingClientRect(), p = document.getElementById('add-category-plus').getBoundingClientRect(), box = d.getBoundingClientRect();
			const inside = [s, p].every((r) => r.left >= box.left && r.right <= box.right);
			return (d.scrollWidth <= d.clientWidth ? 'fits' : 'SCROLLS') + ' ' + (inside ? 'in' : 'OUT') + ' ' + (p.top >= s.bottom ? 'under' : 'beside'); })()`)
		closePage()
	}

	check(t, "task add category", out, strings.Join([]string{
		"opened: *No category,設計,レビュー /  / add-page-q",
		"back by Tab: add-category-plus",
		"added: No category,設計,レビュー,*デザイン / デザイン is created when you add a page / add-page-q / sent 0",
		`put: {"slugs":["memo"],"task":"S300","category":"デザイン","new":false,"rev":n} / task-add-page`,
		"written: 設計,レビュー,デザイン",
		"headed last: デザイン",
		"another: No category,設計,レビュー,デザイン,*二 / 二 is created when you add a page",
		"chosen instead: No category,*設計,レビュー,デザイン,二",
		"opened again: *No category,設計,レビュー,デザイン /  / sent 0",
		"not made: 設計,レビュー,デザイン",
		`refused "": Write the category's name / add-category-dialog over add-page-dialog`,
		`refused "  ": Write the category's name / add-category-dialog over add-page-dialog`,
		`refused "設計": The category 設計 is already there / add-category-dialog over add-page-dialog`,
		`refused "  設計 ": The category 設計 is already there / add-category-dialog over add-page-dialog`,
		`refused "Add": The category Add is already there / add-category-dialog over add-page-dialog`,
		"a case apart: No category,設計,レビュー,デザイン,*ADD / sent 0",
		"reached: add-page-0",
		"Enter twice: No category,設計,レビュー,デザイン,*速い / null / add-page-dialog / sent 0",
		"then put: 1",
		"stale: No category,設計,レビュー,デザイン,速い,*後 / add-page-dialog / sent 1",
		"held on: add-page-dialog / sent 1",
		"then: 2",
		"written: 設計,レビュー,デザイン,速い,後",
		"written once: 設計,レビュー,デザイン,速い,後,同じ",
		"plus double: add-category-dialog over add-page-dialog / add-category-new / sent 0",
		"cancel double: add-page-dialog / add-category-plus / sent 0",
		"add double: add-page-dialog / add-page-q / sent 0",
		"backdrop double: add-page-dialog / add-category-plus / sent 0",
		"apart: No category,設計,レビュー,デザイン,速い,後,同じ,*ページの / false",
		"width 1280: fits in beside",
		"width 384: fits in beside",
		"width 320: fits in under",
	}, "|"))
}
