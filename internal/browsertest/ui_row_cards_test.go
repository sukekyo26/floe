package browsertest_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"

	"floe/internal/store"
)

// TestUIRowCards lays the list's rows out as cards on the list's ground, on the page list, a task's page and the task list: an edge,
// the menu's corner and the small shadow; apart by a margin that a heading and a folder's rows do not take; the row chosen edged in
// the new colour, with no line inside; a folder open one card with its rows, chosen and pointed at within it; the headings' words
// where a card's mark is, the category's add button ending where a card does; and an empty list of no room.
func TestUIRowCards(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	// The folder first, so that the category's card made after it follows its rows
	s.write(t, "fold/one.html", "<title>one</title><p>one</p>")
	s.write(t, "fold/sub/two.html", "<title>two</title><p>two</p>")
	// A file after the one chosen, so that the one chosen is not the card's last row
	s.write(t, "fold/zz.html", "<title>zz</title><p>zz</p>")
	s.floe(t, "publish", "--link", filepath.Join(s.Env["HOME"], "fold"), "--slug", "fold")
	if _, err := st.Edit("fold", store.Edit{Project: ptrTo("demo")}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ChangeTasks(store.TaskChange{Slugs: []string{"fold"}, Task: ptrTo("S1"), Category: ptrTo("one")}); err != nil {
		t.Fatal(err)
	}
	for _, p := range [][2]string{{"plan", "one"}, {"read", "two"}, {"notes", "two"}} {
		s.floe(t, "publish", s.page(t, p[0]+".yaml", "T "+p[0], "<p>x</p>"), "--slug", p[0])
		if _, err := st.Edit(p[0], store.Edit{Project: ptrTo("demo")}); err != nil {
			t.Fatal(err)
		}
		s.floe(t, "task", "--slug", p[0], "--task", "S1", "--category", p[1])
	}
	s.floe(t, "task", "--project", "demo", "--task", "S2", "--title", "No pages")
	b.size(1400, 900, false)
	defer b.unsize()

	var out []string
	say := func(what, expr string) {
		t.Helper()
		out = append(out, what+": "+b.eval(expr))
	}
	// A value of a :root variable as the browser computes it for the property named
	const as = `((prop, value) => { const e = document.createElement('div'); e.style[prop] = value; document.body.append(e); const got = getComputedStyle(e)[prop]; e.remove(); return got; })`
	const card = `((li) => { const st = getComputedStyle(li); return [st.borderTopWidth, st.borderTopLeftRadius === ` + as + `('borderTopLeftRadius', 'var(--corner-menu)'), st.boxShadow === ` + as + `('boxShadow', 'var(--shadow-sm)')].join(' '); })`
	// Each row's place after the one above: C a card, T a folder's row, G a heading, with the room between them
	const gaps = `(() => { const rows = [...document.querySelectorAll('#list > li')], r = (e) => e.getBoundingClientRect();
		return rows.slice(1).map((li, i) => (li.classList.contains('group') ? 'G' : li.classList.contains('tree') ? 'T' : 'C') + Math.round(r(li).top - r(rows[i]).bottom)).join(' '); })()`
	const ground = `(getComputedStyle(document.querySelector('.list-body')).backgroundColor === ` + as + `('backgroundColor', 'var(--bg)'))`

	// The page list: a card; the row chosen edged in --new, no inset line; the ground.
	b.navigate("/?project=demo")
	b.until("the rows", "document.querySelector('#list li[data-slug=plan]')")
	say("card", card+"(document.querySelector('#list li[data-slug=plan]'))")
	b.eval("document.querySelector('#list li[data-slug=read] .title').click()")
	b.until("chosen", "document.querySelector('#list li[data-slug=read].picked')")
	say("chosen", "(() => { const st = getComputedStyle(document.querySelector('#list li[data-slug=read]')); return [st.borderTopColor === "+as+"('borderTopColor', 'var(--new)'), st.boxShadow.includes('inset')].join(' '); })()")
	say("ground", ground)
	// A page published meanwhile: its card rises in, as a new row does, and those there do not (heard once they have risen in).
	b.still()
	b.eval("window.rose = []; document.getElementById('list').addEventListener('animationstart', (e) => rose.push(e.target.dataset.key))")
	s.floe(t, "publish", s.page(t, "late.yaml", "T late", "<p>x</p>"), "--slug", "late")
	if _, err := st.Edit("late", store.Edit{Project: ptrTo("demo")}); err != nil {
		t.Fatal(err)
	}
	b.until("the new card risen", "rose.includes('entry:late')")
	b.still()
	say("rose", "rose.join(',')")

	// A task's page with its folder open: the margins, the headings' words and add buttons, the folder one card, its row chosen and
	// pointed at within it.
	b.eval("localStorage.setItem('list.open', JSON.stringify(['fold']))")
	b.navigate("/tasks?project=demo&task=S1")
	b.until("the folder's rows", "document.querySelectorAll('#list li.tree').length === 3")
	b.still()
	say("gaps", gaps)
	say("headings", `(() => { const r = (e) => e.getBoundingClientRect(), head = document.querySelector('#list li.group.adds'), first = document.querySelector('#list li[data-slug=plan]');
		return [r(head.querySelector('[role=heading]')).left === r(first.querySelector('.lead')).left, r(head.querySelector('.category-add')).right === r(first).right,
			getComputedStyle(head).borderTopWidth, getComputedStyle(head).boxShadow].join(' '); })()`)
	say("folder", `(() => { const st = (e) => getComputedStyle(e), r = (e) => e.getBoundingClientRect(), f = document.querySelector('#list li[data-slug=fold]'), trees = [...document.querySelectorAll('#list li.tree')];
		const last = trees.at(-1);
		return [st(f).borderBottomWidth, st(f).borderBottomLeftRadius, st(trees[0]).borderLeftWidth, st(trees[0]).borderBottomWidth, st(last).borderBottomWidth,
			st(last).borderBottomLeftRadius === `+as+`('borderTopLeftRadius', 'var(--corner-menu)'), r(trees[0]).top - r(f).bottom, r(trees[0]).left - r(f).left, r(trees[0]).right - r(f).right].join(' '); })()`)
	b.eval("[...document.querySelectorAll('#list li.tree .title')].find((e) => e.textContent.includes('one.html')).click()")
	b.until("the file chosen", "document.querySelector('#list li.tree.picked')")
	r := b.rect("document.querySelector('#list li.tree:not(.picked)')")
	if _, err := chromedp.Call(b.ctx, input.DispatchMouseEvent, input.DispatchMouseEventParams{Type: input.DispatchMouseEventTypeMouseMoved, X: r.X + 60, Y: r.Y + r.Height/2}); err != nil {
		t.Fatal(err)
	}
	b.still()
	say("within", "(() => { const bg = (e) => getComputedStyle(e).backgroundColor; return [bg(document.querySelector('#list li.tree.picked')) === "+as+"('backgroundColor', 'var(--new-soft)'), bg(document.querySelector('#list li.tree:not(.picked)')) === "+as+"('backgroundColor', 'var(--hover)'), getComputedStyle(document.querySelector('#list li[data-slug=fold]')).borderColor === "+as+"('borderColor', 'var(--line)'), getComputedStyle(document.querySelector('#list li.tree.picked')).boxShadow.includes('inset')].join(' '); })()")

	// Folded, the folder is a card as any.
	b.eval("localStorage.setItem('list.open', '[]')")
	b.navigate("/tasks?project=demo&task=S1")
	b.until("the rows", "document.querySelector('#list li[data-slug=fold]') && !document.querySelector('#list li.tree')")
	say("folded", card+"(document.querySelector('#list li[data-slug=fold]')) + ' ' + getComputedStyle(document.querySelector('#list li[data-slug=fold]')).borderBottomWidth")

	// A task of no page: no room left in the empty list; the task list's rows are cards too.
	b.navigate("/tasks?project=demo&task=S2")
	b.until("none", "!document.getElementById('task-none').hidden")
	say("empty", "getComputedStyle(document.getElementById('list')).paddingTop + ' ' + "+ground)
	b.navigate("/tasks?project=demo&view=todo")
	b.until("the task list", "document.querySelectorAll('#list > li').length === 2")
	say("task list", card+"(document.querySelector('#list > li'))")

	check(t, "row cards", out, strings.Join([]string{
		"card: 1px true true",
		"chosen: true false",
		"ground: true",
		"rose: entry:late",
		"gaps: C0 T0 T0 T0 C6 G0 C0 C6",
		"headings: true true 0px none",
		"folder: 0px 0px 1px 0px 1px true 0 0 0",
		"within: true true true false",
		"folded: 1px true true 1px",
		"empty: 0px true",
		"task list: 1px true true",
	}, "|"))
}
