package browsertest_test

import (
	"strings"
	"testing"

	"github.com/chromedp/chromedp/kb"

	"github.com/sukekyo26/floe/internal/store"
)

// A row's project as a pill in its chip's color, in the list, the search modal and the task screens, and the row's button that archives
// or gives back: beside the star, not in the menu. A row that leaves leaves the focus on the next row's title; pressed by the keys, the
// focus goes to the toast's way back, which gives the row its focus again.
func TestUIRowActions(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	for _, p := range [][2]string{{"a", "one"}, {"b", "one"}, {"c", "two"}, {"d", strings.Repeat("a-long-name-", 5)}, {"e", "one"}, {"f", "one"}} {
		s.floe(t, "publish", s.page(t, p[0]+".yaml", "T "+p[0], ""), "--slug", p[0])
		if _, err := st.Edit(p[0], store.Edit{Project: &p[1]}); err != nil {
			t.Fatal(err)
		}
	}
	b.size(1400, 800, false)
	defer b.unsize()
	var out []string
	say := func(what, expr string) {
		t.Helper()
		out = append(out, what+": "+b.eval(expr))
	}
	row := func(slug string) string { return "document.querySelector('#list li[data-slug=" + slug + "]')" }
	there := func(slug string) string { return "Boolean(" + row(slug) + ")" }
	archive := func(slug string) string { return row(slug) + ".querySelector('.actions > .archive')" }
	pill := func(slug string) string { return row(slug) + ".querySelector('.proj')" }
	dot := func(slug string) string { return "getComputedStyle(" + pill(slug) + ", '::before').backgroundColor" }
	chip := func(project string) string {
		return "getComputedStyle(document.querySelector('.chip[data-project=" + project + "]'), '::before').backgroundColor"
	}
	mouse := func(expr string) {
		t.Helper()
		r := b.rect(expr)
		b.click(r.X+r.Width/2, r.Y+r.Height/2)
	}
	keys := func(expr string) {
		t.Helper()
		b.eval(expr + ".focus()")
		b.pressKey(kb.Enter)
	}
	undo := func(slug string) string { return "document.querySelector('.toast .undo[data-slug=" + slug + "]')" }
	// Where the focus is: the row and the part of it, or the toast's way back
	const focus = "(() => { const e = document.activeElement; return e.matches('.undo') ? 'undo' : e.closest('li')?.dataset.slug + ' ' + (e.matches('.title') ? 'title' : e.className); })()"

	b.navigate("/")
	b.until("the rows", pill("d"))
	say("the rows", "[...document.querySelectorAll('#list li[data-slug]')].map((li) => li.dataset.slug).join('')")
	say("a pill's dot in its chip's color", dot("a")+" === "+chip("one")+" && "+dot("c")+" === "+chip("two"))
	say("each project's own", dot("a")+" !== "+dot("c"))
	say("the ground of the color too", "getComputedStyle("+pill("a")+").backgroundColor !== getComputedStyle("+pill("c")+").backgroundColor")
	say("a long name cut within the pill", "(() => { const e = "+pill("d")+", name = e.querySelector('.proj-name'); return name.scrollWidth > name.clientWidth && e.offsetWidth <= 10 * parseFloat(getComputedStyle(document.documentElement).fontSize); })()")
	say("the row's buttons", "[..."+row("a")+".querySelector('.actions').children].map((e) => e.classList[1] ?? e.className).join(' ')")
	b.eval(row("a") + ".querySelector('.more').open = true")
	say("the menu leaves it out", "getComputedStyle("+row("a")+".querySelector('.menu .archive')).display")
	b.eval(row("a") + ".querySelector('.more').open = false")
	say("its words", archive("a")+".title + ' / ' + "+archive("a")+".getAttribute('aria-label')")

	// By the mouse: the next row's title has the focus, not its button, and the last row takes it at the end.
	mouse(archive("e"))
	b.until("e archived", "!"+there("e"))
	out = append(out, "pressed: "+b.until("the focus", focus+" !== 'undefined body' && "+focus))
	mouse(archive("a"))
	b.until("a archived", "!"+there("a"))
	out = append(out, "the last: "+b.until("the focus", focus+" !== 'undefined body' && "+focus))

	// By the keys: the way back has the focus, stays while it does, and gives the row its focus again.
	keys(archive("c"))
	out = append(out, "by the keys: "+b.until("the toast", "!"+there("c")+" && "+undo("c")+" && "+focus))
	say("its words", undo("c")+".textContent + ' / ' + "+undo("c")+".getAttribute('aria-label')")
	b.pressKey(kb.Enter)
	out = append(out, "given back: "+b.until("c back", there("c")+" && "+focus+" === 'c title' && "+focus))
	say("no way back then", undo("c")+" === null")
	// Escape leaves it, for the row after.
	keys(archive("c"))
	b.until("c archived", "!"+there("c")+" && document.activeElement.matches('.undo')")
	b.pressKey(kb.Escape)
	out = append(out, "left: "+b.until("the toast gone", undo("c")+" === null && "+focus))
	b.eval(`fetch('/api/artifacts/c', { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: '{"archived":false}' }).then(() => load())`)
	b.untilActed("c back again", there("c"))

	// The last row has none after it: the row that is last now takes the focus.
	keys(archive("b"))
	b.until("b archived", "!"+there("b")+" && document.activeElement.matches('.undo')")
	b.pressKey(kb.Escape)
	out = append(out, "left, the last: "+b.until("the toast gone", undo("b")+" === null && "+focus))
	b.eval(`fetch('/api/artifacts/b', { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: '{"archived":false}' }).then(() => load())`)
	b.untilActed("b back", there("b"))

	// The way back fails: it stays to press again. The list slow to come: it can be pressed all the same.
	mouse(archive("c"))
	b.until("c archived again", "!"+there("c")+" && "+undo("c"))
	b.eval(`window.realFetch = window.fetch; window.fetch = (u, i) => (i?.method === 'PATCH' ? Promise.reject(new TypeError('Failed to fetch')) : window.realFetch(u, i));
		` + undo("c") + `.click()`)
	b.until("the error", "!document.getElementById('error').hidden")
	say("failed, it stays", undo("c")+" !== null && !"+there("c"))
	b.eval(`window.patches = []; window.hold = new Promise((r) => { window.release = r; });
		window.fetch = (u, i) => { if (i?.method === 'PATCH') { window.patches.push(i.body); return window.realFetch(u, i); } return String(u).startsWith('/api/artifacts') ? window.hold.then(() => window.realFetch(u, i)) : window.realFetch(u, i); };
		document.querySelector('#list li[data-slug=b] .actions > .archive').click()`)
	b.until("b's way back, the list not read", undo("b"))
	b.eval(undo("b") + ".click()")
	out = append(out, "the list held: "+b.until("the way back asked", "window.patches.length === 2 && window.patches.join()"))
	b.eval("window.release(); window.fetch = window.realFetch; " + undo("c") + ".click()")
	b.untilActed("all back", there("b")+" && "+there("c"))

	// The archived.
	mouse(archive("b"))
	b.until("b gone", "!"+there("b"))
	b.navigate("/?view=archived")
	b.until("the archived", there("b"))
	say("an archived row's button", archive("b")+".classList.contains('on') + ' ' + "+archive("b")+".title")
	mouse(archive("b"))
	b.until("b given back", "!"+there("b"))
	say("given back, no way back", undo("b")+" === null")

	// The star on the view of favorites leaves as the button does.
	b.navigate("/")
	b.until("the rows after that", pill("d"))
	for _, slug := range []string{"b", "c"} {
		b.eval(row(slug) + ".querySelector('.star').click()")
		b.until("a favorite", row(slug)+".querySelector('.star').getAttribute('aria-pressed') === 'true'")
	}
	b.navigate("/?view=fav")
	b.until("the favorites", there("b")+" && "+there("c"))
	first := b.eval("document.querySelector('#list li[data-slug]').dataset.slug")
	mouse(row(first) + ".querySelector('.star')")
	b.until("off the view", "!"+there(first))
	out = append(out, "the star: "+b.until("the focus", "document.activeElement.matches('.title')"))
	b.eval(row(map[string]string{"b": "c", "c": "b"}[first]) + ".querySelector('.star').click()")
	b.until("none left", "document.querySelectorAll('#list li[data-slug]').length === 0")

	// In the modal: the way back there, and the search has the focus.
	b.navigate("/")
	b.until("the rows once more", pill("d"))
	realKey(b, "k", 2)
	b.until("the modal's rows", "document.querySelector('#find-list .proj') && document.querySelector('#find-actions > .archive[title]')")
	say("the modal's pill", "getComputedStyle(document.querySelector('#find-list .proj'), '::before').backgroundColor !== 'rgba(0, 0, 0, 0)'")
	b.eval("document.querySelector('#find-actions > .archive').click()")
	b.until("the way back in the modal", "document.querySelector('#find .undo')")
	b.eval("document.querySelector('#find .undo').focus(); document.querySelector('#find .undo').click()")
	out = append(out, "the modal: "+b.until("the search", "document.querySelector('.undo') === null && document.activeElement.id"))
	b.pressKey(kb.Escape)

	// A page open whole: its bar's button has the focus again.
	b.navigate("/a/d/")
	b.until("the bar", "document.querySelector('#frame-actions > .archive[title]')")
	b.eval("document.querySelector('#frame-actions > .archive').click()")
	b.until("d's way back", undo("d"))
	b.eval(undo("d") + ".focus(); " + undo("d") + ".click()")
	out = append(out, "whole: "+b.until("the bar's button", undo("d")+" === null && document.activeElement.matches('#frame-actions > .archive') && document.activeElement.title"))

	// A task's page: the row stays under its step, dimmed, keeping its key, and the focus on its button.
	for _, slug := range []string{"a", "e"} {
		b.eval(`fetch('/api/artifacts/` + slug + `', { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: '{"archived":false}' })`)
	}
	s.floe(t, "work", "a", "--task", "#1", "--step", "one")
	s.floe(t, "work", "b", "--task", "#1", "--step", "two")
	b.navigate("/tasks?project=one&task=%231")
	b.until("the task's page", there("a")+" && "+there("b")+" && document.querySelector('#task-project .proj-name')")
	say("the task's head", "document.querySelector('#task-project').textContent + ' ' + (getComputedStyle(document.querySelector('#task-project')).backgroundColor !== 'rgba(0, 0, 0, 0)')")
	mouse(archive("a"))
	out = append(out, "a task's row: "+b.until("archived", row("a")+".querySelector('.actions > .archive').classList.contains('on') && "+focus))

	// The task list: the filter button and the head with the column folded; the page list's stay words.
	b.navigate("/tasks?project=one")
	b.until("the task list", "document.querySelector('#narrow-filter.proj-tint')")
	say("the filter button", "document.getElementById('narrow-filter-name').textContent + ' ' + document.getElementById('narrow-filter').style.getPropertyValue('--c').startsWith('var(--pc')")
	say("the folded head", "document.getElementById('shelf-what').textContent + ' ' + document.getElementById('shelf-what').classList.contains('proj-tint')")
	// No project known yet, as while the tasks cannot be read: the head has no words, whatever the view.
	b.eval("view = 'doing'; project = ''; renderShelf()")
	say("no project, no head", "document.getElementById('shelf-what').textContent === '' && !document.getElementById('narrow-filter').classList.contains('proj-tint')")
	b.navigate("/")
	b.until("the list's button", "document.getElementById('narrow-filter')")
	say("the page list's are words", "document.getElementById('narrow-filter').classList.contains('proj-tint') + ' ' + document.getElementById('shelf-what').classList.contains('proj-tint')")

	check(t, "row actions", out, rowActionsWant)
}

const rowActionsWant = "the rows: fedcba|a pill's dot in its chip's color: true|each project's own: true|the ground of the color too: true|a long name cut within the pill: true|" +
	"the row's buttons: archive star more|the menu leaves it out: none|its words: Archive / Archive T a|pressed: d title|the last: b title|by the keys: undo|" +
	"its words: Restore / Restore T c to the list|given back: c title|no way back then: true|left: b title|left, the last: c title|failed, it stays: true|" +
	`the list held: {"archived":true},{"archived":false}|an archived row's button: true Restore|given back, no way back: true|the star: true|` +
	"the modal's pill: true|the modal: find-q|whole: Archive|the task's head: one true|a task's row: a icon-btn archive on|the filter button: one true|the folded head: one true|no project, no head: true|the page list's are words: false false"
