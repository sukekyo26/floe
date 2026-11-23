package browsertest_test

import (
	"encoding/json"
	"testing"

	"floe/internal/i18n"
	"floe/internal/store"
)

// Every button of words is of one form, as drawn: 2rem high, 2.25rem at a modal's foot, its corners --corner-small. They
// are every button and link with words shown, in each place, every modal of the management page among them, but the rows the
// rule names and the links of a row or a mark. Their words keep to one line, within what clips them, the row of buttons going
// onto another where they do not fit; #tag-new alone, of words a person typed, goes over lines. One on the screen has an icon,
// and one in a modal, the bar of the selected or a toast has none. The task's way back, #task-up, is measured for its words alone.

// notButtons are not buttons of words by the rule (.claude/rules/ui.md): the shelf's rows and the add buttons under them, in a
// modal too on a narrow screen; the settings' rows and their way back; the "…" menu's items. Nor are the links of a row or a mark
// (the brand, a row's title in the list and the search, a row's task and tags, a task's parent and children, the diff's contents,
// the bar's count of the unread, a link of words: the user's choice, S422-P8), nor tabs (role=tab: the help's switches, as the
// React screen makes them). The React screens' bar's links to the screens (#screens a, links of a nav, the screen shown darker:
// the user's choice, S448) are left out there alone: ui.html's frame does not show its own
const notButtons = "#shelf *, #filter-dialog #shelf-more *, [data-testid=settings-item], #settings-back, [data-testid=menu] *, " +
	"[data-testid=brand], #list li [data-testid=title], #find-list li [data-testid=title], [data-testid=task-tag], #list li [data-testid=tagpill], [data-testid=kin-row], #diff-toc button, [role=tab], #news"

// reactOnly are not buttons of words on the React screens alone (#boot tells their build)
const reactOnly = "#screens a"

// wordsOnly is a button of words in no form of .btn, measured for its words alone: the task's way back.
const wordsOnly = "#task-up"

// LIMIT: buttons of words in another form than .btn, neither a row nor a mark, on ui.html's screens. The user's decisions (S425,
// S422-P8): they take the form of .btn. ui.html is not mended while it is rebuilt, so they pass on its screens; the React screens
// are measured by the rule (#boot tells their build), and each row goes as its last ui.html screen does (one not found there fails
// to tell so).
var otherForm = map[string]string{
	"back": "the bar of a page whole: its round way back",
}

func TestUIButtons(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	b.navigate("/")
	project := "proj"
	for _, slug := range []string{"a", "b"} {
		s.floe(t, "publish", s.page(t, slug+".yaml", "T "+slug, ""), "--slug", slug)
		// A tag on a row: a mark of the row, as its task is
		if _, err := st.Edit(slug, store.Edit{Project: &project, Tags: &store.TagsEdit{Add: []string{"plan"}}}); err != nil {
			t.Fatal(err)
		}
	}
	// A title that takes a toast's words over lines, its button wider than the room left beside them
	s.floe(t, "publish", s.page(t, "c.yaml", "トーストの字が三行より長くなるほどに長いページの題をここに書いておく", ""), "--slug", "c")
	if _, err := st.Edit("c", store.Edit{Project: &project}); err != nil {
		t.Fatal(err)
	}
	s.floe(t, "task", "--slug", "a", "--task", "#1", "--title", "one", "--state", "done", "--category", "design")
	s.floe(t, "task", "--project", project, "--task", "#2", "--title", "no page")
	s.floe(t, "task", "--project", project, "--task", "#3", "--title", "a child", "--parent", "#1")

	// The box that clips a button is its nearest ancestor that does not show what overflows it, or the screen; what it shows of
	// itself is measured, not all it scrolls over. A button's words are its text, but in an icon or a part not shown
	other := make([]string, 0, len(otherForm))
	for id := range otherForm {
		other = append(other, id)
	}
	otherJSON, err := json.Marshal(other)
	if err != nil {
		t.Fatal(err)
	}
	measure := `(() => {
		// Each build names the small corner its own way; the same 6px
		const react = document.getElementById('boot') !== null;
		const small = getComputedStyle(document.documentElement).getPropertyValue(react ? '--radius-small' : '--corner-small').trim();
		const other = ` + string(otherJSON) + `;
		const words = (e) => {
			let said = '';
			const walk = (n) => {
				for (const c of n.childNodes) {
					if (c.nodeType === Node.TEXT_NODE) said += c.textContent;
					else if (c.nodeType === Node.ELEMENT_NODE && c.localName !== 'svg' && c.checkVisibility()) walk(c);
				}
			};
			walk(e);
			return said.trim();
		};
		const shown = [...document.querySelectorAll('button, a')].filter((e) => e.getClientRects().length && !e.closest('dialog:not([open])') && words(e) && !e.matches(` + js(notButtons) + `) && !(react && e.matches(` + js(reactOnly) + `)));
		const clip = (e) => {
			for (let box = e.parentElement; box && box !== document.body && box !== document.documentElement; box = box.parentElement) {
				const st = getComputedStyle(box);
				if (st.overflowX !== 'visible' || st.overflowY !== 'visible') return box.getBoundingClientRect();
			}
			return { left: 0, top: 0, right: innerWidth, bottom: innerHeight };
		};
		const name = (e) => e.id || e.dataset.testid || e.localName + ' ' + words(e);
		const seen = [];
		const bad = shown.flatMap((e) => {
			const r = e.getBoundingClientRect();
			const c = clip(e);
			const why = [];
			if (!e.matches(` + js(wordsOnly) + `)) {
				const form = [];
				const want = e.closest('[data-testid=dialog-actions], [data-testid=find-foot]') ? 36 : 32;
				if (e.id === 'tag-new' ? r.height < want - 0.5 : Math.abs(r.height - want) > 0.5) form.push('height ' + Math.round(r.height));
				if (getComputedStyle(e).borderTopLeftRadius !== small) form.push('corner ' + getComputedStyle(e).borderTopLeftRadius);
				if (Boolean(e.querySelector('svg')) === Boolean(e.closest('dialog, #bulk, [data-testid=toast]'))) form.push(e.querySelector('svg') ? 'an icon' : 'no icon');
				if (!react && other.includes(e.id)) {
					seen.push(e.id);
					if (form.length === 0) why.push('of the form now: take it off the table');
				} else why.push(...form);
			}
			if (e.scrollHeight > e.clientHeight + 1) why.push('words over lines');
			if (e.scrollWidth > e.clientWidth + 1) why.push('words cut off');
			if (r.left < c.left - 0.5 || r.right > c.right + 0.5 || r.top < c.top - 0.5 || r.bottom > c.bottom + 0.5) why.push('out of its box');
			return why.length ? [name(e) + ': ' + why.join(', ')] : [];
		});
		const keys = ['#keys [data-help=about]', '#keys [data-help=keys]', '#keys [data-close]'].filter((sel) => shown.includes(document.querySelector(sel))).length;
		// A small corner of no value would be met by none: said apart from the buttons
		const dialogs = [...document.querySelectorAll('dialog')].map((d) => (react ? 'react ' : 'ui.html ') + d.id);
		return JSON.stringify({ n: shown.length, bad: (small === '' ? ['no small corner; '] : []).concat(bad).join('; '), keys, keysWant: react ? 1 : 3, seen, dialogs, react });
	})()`
	seen := map[string]bool{}
	// The dialogs each build holds, and the build of the screen measured last: a modal is measured on a screen of its build
	present := map[string]bool{}
	build := ""
	check := func(scene string) int {
		t.Helper()
		b.still()
		var got struct {
			N        int
			Bad      string
			Keys     int
			KeysWant int
			Seen     []string
			Dialogs  []string
			React    bool
		}
		if err := json.Unmarshal([]byte(b.eval(measure)), &got); err != nil {
			t.Fatal(err)
		}
		if got.N == 0 || got.Bad != "" {
			t.Errorf("%s: %d buttons, of another form: %s", scene, got.N, got.Bad)
		}
		for _, id := range got.Seen {
			seen[id] = true
		}
		for _, d := range got.Dialogs {
			present[d] = true
		}
		build = map[bool]string{true: "react ", false: "ui.html "}[got.React]
		return got.Keys - got.KeysWant
	}
	opened := map[string]bool{}
	closeModal := func(id string) {
		t.Helper()
		b.eval("document.querySelector(" + js("#"+id+" [data-close]") + ").click()")
		b.until(id+" closed", "!document.getElementById("+js(id)+").open")
	}
	modal := func(id string, open func()) {
		t.Helper()
		open()
		b.until(id, "document.getElementById("+js(id)+").open")
		check(id)
		closeModal(id)
		opened[build+id] = true
	}
	press := func(sel string) func() {
		return func() { b.eval("document.querySelector(" + js(sel) + ").click()") }
	}
	menu := func(slug, item string) func() {
		return func() { pressMenuItem(b, "#list li[data-slug="+slug+"]", item) }
	}
	// Help's three buttons of words, on either part: they are among those measured
	help := func(scene string) {
		t.Helper()
		modal("keys", press("#keys-open"))
		b.eval("document.getElementById('keys-open').click()")
		b.until("the help", "document.getElementById('keys').open && "+helpOn+"(document.querySelector('#keys [data-help=about]'))")
		for _, part := range []string{"about", "keys"} {
			b.eval("document.querySelector('#keys [data-help=" + part + "]').click()")
			b.until("the part "+part, helpOn+"(document.querySelector('#keys [data-help="+part+"]'))")
			// Its two switches and its close button on ui.html; the React screens' switches are tabs, its close button alone
			if n := check(scene + ", help's " + part); n != 0 {
				t.Errorf("%s, help's %s: %d buttons of words measured past or short of its own", scene, part, n)
			}
		}
		closeModal("keys")
	}
	const archive = "document.getElementById('task-archive')"

	b.size(1400, 800, false)
	defer b.unsize()
	b.navigate("/?project=proj")
	b.until("the rows", "document.querySelector('#list li[data-slug=b]')")
	modal("add-dialog", press("#add-link"))
	modal("edit-dialog", menu("b", "edit"))
	modal("set-dialog", menu("b", "task-set"))
	// Asked of a page on another task, as Set the task asks it
	modal("move-dialog", func() {
		menu("a", "task-set")()
		b.until("the set modal", "document.getElementById('set-dialog').open")
		b.fill("#set-task", "#2")
		b.eval("document.getElementById('set-submit').click()")
	})
	closeModal("set-dialog")
	// The list's search field stands for the bar's way to the search modal, which Ctrl+K opens
	modal("find", func() { keyDown(b, "key: 'k', ctrlKey: true") })
	modal("keys", func() { b.pressKey("?") })
	help("the page list")
	modal("settings", press("#settings-open"))
	// The settings' rows and their way back are rows, not .btn: the rule's exceptions, named
	const rows = "[...document.querySelectorAll('#settings button')].filter((e) => e.getClientRects().length && e.matches(" + "'[data-testid=settings-item], #settings-back'" + ")).map((e) => e.id || e.dataset.testid).join(',')"
	b.eval("document.getElementById('settings-open').click()")
	b.until("the settings", "document.getElementById('settings').open")
	if got := b.eval(rows); got != "settings-item,settings-item" {
		t.Errorf("the settings' list: its rows %q, want its two", got)
	}
	b.eval("document.querySelector('#settings [data-panel=mode]').click()")
	b.until("the mode's part", "document.getElementById('settings-back').checkVisibility()")
	if got := b.eval(rows); got != "settings-back" {
		t.Errorf("the mode's part: its rows %q, want the way back", got)
	}
	check("the mode's part")
	closeModal("settings")
	b.eval("for (const s of [...document.querySelectorAll('#list li[data-slug] [data-testid=sel]')].slice(0, 2)) s.click()")
	b.until("the bar", "!document.getElementById('bulk').hidden")
	check("the bar of the selected")
	modal("tag-dialog", press("#bulk-tag"))
	b.eval("document.getElementById('bulk-delete').click()")
	check("the delete asked again")
	b.eval("document.getElementById('bulk-clear').click()")
	begun := b.acted("document.querySelector('#list li[data-slug=b] [data-testid=actions] > [data-testid=archive]').click()")
	b.until("the toast", "document.querySelector('[data-testid=toast] button')")
	// The toast comes before the list is read again, which draws the rows anew
	b.untilRead(begun, "the archive done", "true")
	check("the toast")

	b.navigate("/tasks?project=proj&task=%231")
	b.until("the head", "!"+archive+".hidden")
	check("a task done")
	help("a task")
	modal("kin-dialog", press("#task-child"))
	modal("edit-task-dialog", press("#task-edit"))
	modal("add-page-dialog", press("#task-add-page"))
	// The React screen's own of the rest: each modal it holds is measured there too
	modal("edit-add-dialog", func() {
		press("#task-edit")()
		b.until("the edit", "document.getElementById('edit-task-dialog').open")
		press("#edit-url-plus")()
	})
	closeModal("edit-task-dialog")
	modal("find", press("#find-open"))
	modal("settings", press("#settings-open"))
	modal("tag-dialog", menu("a", "tag-item"))
	modal("edit-dialog", menu("a", "edit"))
	modal("set-dialog", menu("a", "task-set"))
	modal("move-dialog", func() {
		menu("a", "task-set")()
		b.until("the set modal", "document.getElementById('set-dialog').open")
		b.fill("#set-task", "#2")
		b.eval("document.getElementById('set-submit').click()")
	})
	closeModal("set-dialog")
	b.navigate("/tasks?project=proj&task=%232")
	b.until("no pages", "!document.getElementById('task-none').hidden")
	check("a task of no page")
	b.eval("document.getElementById('task-delete').click()")
	b.until("asked again", "document.getElementById('task-delete').dataset.armed === 'true'")
	check("the delete of a task asked again")
	b.eval("document.getElementById('task-delete').focus(); document.getElementById('task-delete').blur()")
	b.until("let go", "document.getElementById('task-delete').dataset.armed !== 'true'")
	check("the delete of a task let go")

	b.size(400, 800, true)
	b.navigate("/?project=proj")
	b.until("the rows", "document.querySelector('#list li[data-slug=a]')")
	modal("filter-dialog", press("#narrow-filter"))

	// Narrow, in Japanese, whose words break between any two characters: a row of buttons goes onto another line, and a
	// button wider than its line keeps its words to one
	b = newBrowser(t, s.Base, i18n.Lang("ja"))
	ja := i18n.New(i18n.Lang("ja"))
	b.size(280, 800, false)
	b.navigate("/?project=proj")
	b.until("the rows", "document.querySelector('#list li[data-slug=c]')")
	modal("keys", func() { b.pressKey("?") })
	help("narrow")
	// Read while open: closed, a build may draw the modal without its page
	modal("set-dialog", func() {
		menu("a", "task-set")()
		b.until("set-dialog", "document.getElementById('set-dialog').open")
		if got := b.eval("String(document.getElementById('set-out').hidden)"); got != "false" {
			t.Errorf("the set modal of a page on a task: take off hidden %s", got)
		}
	})
	b.eval("document.querySelector('#list li[data-slug=a] [data-testid=sel]').click()")
	b.until("the bar", "!document.getElementById('bulk').hidden")
	b.eval("document.getElementById('bulk-tag').click()")
	b.until("tag-dialog", "document.getElementById('tag-dialog').open")
	b.fill("#tag-find", "a-tag-name-long-enough-to-go-over-the-width-of-a-narrow-modal")
	b.until("the new tag", "!document.getElementById('tag-new').hidden")
	check("a long new tag")
	closeModal("tag-dialog")
	b.eval("document.getElementById('bulk-clear').click()")
	begun = b.acted("document.querySelector('#list li[data-slug=c] [data-testid=actions] > [data-testid=archive]').click()")
	b.until("the toast", "document.querySelector('[data-testid=toast] button')")
	b.untilRead(begun, "the archive done", "true")
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

	// ui.html's modals, on the one screen it serves: a page whole. Opened again, as read: the toast of the first opening moves
	// into a modal opened, its button out of the modal's box (S445)
	b.size(1400, 800, false)
	b.navigate("/a/a/")
	b.until("read", "document.querySelector('[data-testid=toast]')")
	b.navigate("/a/a/")
	b.until("the bar", "document.querySelector('#frame-actions [data-testid=task-set]')")
	frameMenu := func(item string) func() {
		return func() { pressMenuItem(b, "#frame-actions", item) }
	}
	modal("find", func() { keyDown(b, "key: 'k', ctrlKey: true") })
	modal("keys", func() { b.pressKey("?") })
	modal("settings", frameMenu("theme-item"))
	modal("tag-dialog", frameMenu("tag-item"))
	modal("edit-dialog", frameMenu("edit"))
	modal("set-dialog", frameMenu("task-set"))
	modal("move-dialog", func() {
		frameMenu("task-set")()
		b.until("the set modal", "document.getElementById('set-dialog').open")
		b.fill("#set-task", "#2")
		b.eval("document.getElementById('set-submit').click()")
	})
	closeModal("set-dialog")

	// The diff and a page not found, last: publishing a again may take it off its task
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

	for id := range otherForm {
		if !seen[id] {
			t.Errorf("#%s of another form is not found: take it off the table", id)
		}
	}

	// A modal with no way here is a modal not measured: each a screen of each build holds, opened on one of that build
	// LIMIT: ui.html still holds the modals of the task list and of the page list, which only /tasks and / open, both served by
	// React. Drop this as the frame leaves ui.html (S422-P10) and ui.html goes (S422-P11)
	notOpened := map[string]bool{
		"ui.html edit-task-dialog": true, "ui.html kin-dialog": true, "ui.html add-page-dialog": true,
		"ui.html add-dialog": true, "ui.html filter-dialog": true,
	}
	for d := range present {
		if !opened[d] && !notOpened[d] {
			t.Errorf("the %s modal is not measured on a screen of its build", d)
		}
	}
}
