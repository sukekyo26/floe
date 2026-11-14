package browsertest_test

import (
	"strconv"
	"strings"
	"testing"

	"floe/internal/i18n"
	"floe/internal/store"
)

// Every button of words is of one form, as drawn: 2rem high, 2.25rem at a modal's foot, its corners --corner-small. They
// are gathered by their class, .btn, in each place they show, every modal of the management page among them. Their words
// keep to one line, within what clips them, the row of buttons going onto another where they do not fit; #tag-new alone, of
// words a person typed, goes over lines. A .btn on the screen has an icon, and one in a modal, the bar of the selected or a
// toast has none. The task's way back, .up, is no .btn, and is measured for its words alone.
// LIMIT: a button of words that does not have .btn is not found: the rule in .claude/rules/ui.md tells its writer. The
// shelf's rows (its views and picks, and the add buttons under them) are such buttons, in the shape of a row, not .btn: the
// page list and the task list have no .btn to measure, but in their modals.
func TestUIButtons(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	b.navigate("/")
	project := "proj"
	for _, slug := range []string{"a", "b"} {
		s.floe(t, "publish", s.page(t, slug+".yaml", "T "+slug, ""), "--slug", slug)
		if _, err := st.Edit(slug, store.Edit{Project: &project}); err != nil {
			t.Fatal(err)
		}
	}
	// A title that takes a toast's words over lines, its button wider than the room left beside them
	s.floe(t, "publish", s.page(t, "c.yaml", "トーストの字が三行より長くなるほどに長いページの題をここに書いておく", ""), "--slug", "c")
	if _, err := st.Edit("c", store.Edit{Project: &project}); err != nil {
		t.Fatal(err)
	}
	s.floe(t, "work", "a", "--task", "#1", "--title", "one", "--state", "done", "--category", "design")
	s.floe(t, "work", "--project", project, "--task", "#2", "--title", "no page")
	s.floe(t, "work", "--project", project, "--task", "#3", "--title", "a child", "--parent", "#1")

	// The box that clips a button is its nearest ancestor that does not show what overflows it, or the screen; what it shows of
	// itself is measured, not all it scrolls over
	const measure = `(() => {
		const small = getComputedStyle(document.documentElement).getPropertyValue('--corner-small').trim();
		const shown = [...document.querySelectorAll('.btn, .up')].filter((e) => e.getClientRects().length && !e.closest('dialog:not([open])'));
		const clip = (e) => {
			for (let box = e.parentElement; box && box !== document.body && box !== document.documentElement; box = box.parentElement) {
				const st = getComputedStyle(box);
				if (st.overflowX !== 'visible' || st.overflowY !== 'visible') return box.getBoundingClientRect();
			}
			return { left: 0, top: 0, right: innerWidth, bottom: innerHeight };
		};
		const bad = shown.flatMap((e) => {
			const r = e.getBoundingClientRect();
			const c = clip(e);
			const why = [];
			if (e.matches('.btn')) {
				const want = e.closest('.dialog-actions') ? 36 : 32;
				if (e.id === 'tag-new' ? r.height < want - 0.5 : Math.abs(r.height - want) > 0.5) why.push('height ' + Math.round(r.height));
				if (getComputedStyle(e).borderTopLeftRadius !== small) why.push('corner ' + getComputedStyle(e).borderTopLeftRadius);
				if (Boolean(e.querySelector('svg')) === Boolean(e.closest('dialog, .bulk, .toast'))) why.push(e.querySelector('svg') ? 'an icon' : 'no icon');
			}
			if (e.scrollHeight > e.clientHeight + 1) why.push('words over lines');
			if (e.scrollWidth > e.clientWidth + 1) why.push('words cut off');
			if (r.left < c.left - 0.5 || r.right > c.right + 0.5 || r.top < c.top - 0.5 || r.bottom > c.bottom + 0.5) why.push('out of its box');
			return why.length ? [(e.id || e.className) + ': ' + why.join(', ')] : [];
		});
		return shown.length + '|' + bad.join('; ');
	})()`
	check := func(scene string) {
		t.Helper()
		b.still()
		n, bad, _ := strings.Cut(b.eval(measure), "|")
		if count, err := strconv.Atoi(n); err != nil || count == 0 || bad != "" {
			t.Errorf("%s: %s buttons, of another form: %s", scene, n, bad)
		}
	}
	opened := map[string]bool{}
	modal := func(id, open string) {
		t.Helper()
		b.eval(open)
		b.until(id, "document.getElementById("+js(id)+").open")
		check(id)
		b.eval("document.getElementById(" + js(id) + ").close()")
		opened[id] = true
	}
	menu := func(slug, item string) string {
		return "document.querySelector('#list li[data-slug=" + slug + "] .more').open = true; document.querySelector('#list li[data-slug=" + slug + "] .menu ." + item + "').click()"
	}
	const archive = "document.getElementById('task-archive')"

	b.size(1400, 800, false)
	defer b.unsize()
	b.navigate("/?project=proj")
	b.until("the rows", "document.querySelector('#list li[data-slug=b]')")
	modal("add-dialog", "document.getElementById('add-link').click()")
	modal("edit-dialog", menu("b", "edit"))
	modal("set-dialog", menu("b", "task-set"))
	modal("find", "openFind()")
	modal("keys", "openKeys()")
	modal("settings", "document.getElementById('settings-open').click()")
	b.eval("for (const s of [...document.querySelectorAll('#list li:not(.group) .sel')].slice(0, 2)) s.click()")
	b.until("the bar", "!document.getElementById('bulk').hidden")
	check("the bar of the selected")
	modal("tag-dialog", "document.getElementById('bulk-tag').click()")
	b.eval("document.getElementById('bulk-delete').click()")
	check("the delete asked again")
	b.eval("document.getElementById('bulk-clear').click()")
	b.eval("document.querySelector('#list li[data-slug=b] .actions > .archive').click()")
	b.until("the toast", "document.querySelector('.toast .btn')")
	// The toast comes before the list is read again, which draws the rows anew
	b.untilActed("the archive done", "true")
	check("the toast")

	b.navigate("/tasks?project=proj&task=%231")
	b.until("the head", "!"+archive+".hidden")
	check("a task done")
	modal("kin-dialog", "document.getElementById('task-child').click()")
	modal("edit-task-dialog", "document.getElementById('task-edit').click()")
	modal("add-page-dialog", "document.querySelector('#list .category-add').click()")
	b.navigate("/tasks?project=proj&task=%232")
	b.until("no pages", "!document.getElementById('task-none').hidden")
	check("a task of no page")
	b.eval("document.getElementById('task-delete').click()")
	b.until("asked again", "document.getElementById('task-delete').classList.contains('armed')")
	check("the delete of a task asked again")
	b.eval("document.getElementById('task-delete').focus(); document.getElementById('task-delete').blur()")
	b.until("let go", "!document.getElementById('task-delete').classList.contains('armed')")
	check("the delete of a task let go")

	b.size(400, 800, true)
	b.navigate("/?project=proj")
	b.until("the rows", "document.querySelector('#list li[data-slug=a]')")
	modal("filter-dialog", "document.getElementById('narrow-filter').click()")

	// Narrow, in Japanese, whose words break between any two characters: a row of buttons goes onto another line, and a
	// button wider than its line keeps its words to one
	b = newBrowser(t, s.Base, i18n.Lang("ja"))
	ja := i18n.New(i18n.Lang("ja"))
	b.size(280, 800, false)
	b.navigate("/?project=proj")
	b.until("the rows", "document.querySelector('#list li[data-slug=c]')")
	modal("keys", "openKeys()")
	modal("set-dialog", menu("a", "task-set"))
	if got := b.eval("String(document.getElementById('set-out').hidden)"); got != "false" {
		t.Errorf("the set modal of a page on a task: take off hidden %s", got)
	}
	b.eval("document.querySelector('#list li[data-slug=a] .sel').click()")
	b.until("the bar", "!document.getElementById('bulk').hidden")
	b.eval("document.getElementById('bulk-tag').click()")
	b.until("tag-dialog", "document.getElementById('tag-dialog').open")
	b.eval("const q = document.getElementById('tag-find'); q.value = 'a-tag-name-long-enough-to-go-over-the-width-of-a-narrow-modal'; q.dispatchEvent(new Event('input', { bubbles: true }))")
	b.until("the new tag", "!document.getElementById('tag-new').hidden")
	check("a long new tag")
	b.eval("document.getElementById('tag-dialog').close(); document.getElementById('bulk-clear').click()")
	b.eval("document.querySelector('#list li[data-slug=c] .actions > .archive').click()")
	b.until("the toast", "document.querySelector('.toast .btn')")
	b.untilActed("the archive done", "true")
	check("the toast of a long title")

	b.navigate("/tasks?project=proj&task=%233")
	b.until("the parent", "!document.getElementById('task-parent-change').hidden")
	check("a narrow task with a parent")
	b.navigate("/tasks?project=proj&task=%231")
	b.until("the head", "!"+archive+".hidden")
	check("a narrow task done")
	pressTaskArchive(b, ja.Msg("ui_unarchive"))
	check("a narrow task archived")
	// A card's heading longer than its line, as some language might have it: the heading goes over lines, its button whole
	b.navigate("/tasks?project=proj&task=%231")
	b.until("the children", "!document.getElementById('task-child').hidden")
	b.eval("document.getElementById('task-family-word').textContent = 'The child tasks of this task, a heading long enough to take more than its line'")
	check("a long heading of a card")

	// The diff and a page not found, last: publishing a again may take it off its task
	b.size(1400, 800, false)
	s.floe(t, "publish", s.page(t, "a.yaml", "T a", "<p>two</p>"), "--slug", "a")
	b.navigate("/a/a/?diff")
	b.until("the diff", "!document.getElementById('diff-main').hidden")
	check("a diff")
	b.navigate("/a/c/?diff")
	b.until("no versions", "!document.getElementById('diff-none').hidden")
	check("a diff of one version")
	b.navigate("/a/none/")
	b.until("the missing page", "!document.getElementById('notfound').hidden")
	check("a page not found")

	// A modal with no way here is a modal not measured.
	if got := b.eval("document.querySelectorAll('dialog').length + ''"); got != strconv.Itoa(len(opened)) {
		t.Errorf("%s modals, %d opened", got, len(opened))
	}
}
