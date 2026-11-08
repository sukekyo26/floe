package browsertest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chromedp/chromedp/kb"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/store"
)

// TestUITagLine: the tags of a page open, under its bar beside the list, in the frame and on a task's page, and the one tag
// dialog its button, the row's menu and the selection open: what each sends, where the focus goes back, and where the line is
// not drawn.
func TestUITagLine(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	msg := i18n.New(i18n.English).Msg
	st := store.New(s.Env["FLOE_DIR"])
	s.floe(t, "publish", s.page(t, "tl/none.yaml", "None", ""), "--slug", "none")
	s.floe(t, "publish", s.page(t, "tl/one.yaml", "One", "<p>1</p>"), "--slug", "one")
	s.floe(t, "publish", s.page(t, "tl/one.yaml", "One", "<p>2</p>"), "--slug", "one")
	s.floe(t, "publish", s.page(t, "tl/many.yaml", "Many", ""), "--slug", "many")
	gone := s.write(t, "tl/g/gone.html", "<title>G</title><p>g</p>")
	s.floe(t, "publish", "--link", gone, "--slug", "gone")
	if err := os.Remove(gone); err != nil {
		t.Fatal(err)
	}
	s.write(t, "tl/docs/index.html", "<!doctype html><title>t</title><p>page</p>")
	s.floe(t, "publish", "--link", filepath.Join(s.Env["HOME"], "tl/docs"), "--slug", "docs")
	tag := func(slug string, tags ...string) {
		t.Helper()
		if _, err := st.Edit(slug, store.Edit{Tags: &tags}); err != nil {
			t.Fatal(err)
		}
	}
	tag("one", "plan")
	tag("many", "a1", "a2", "a3", "a4", "a5", "a6", "a7", "a long name that the line cuts at its end")
	tags := func(slug string) string {
		t.Helper()
		return strings.Join(st.ReadMeta(slug).Tags, ",")
	}
	b.size(1440, 900, false)
	defer b.unsize()

	var out []string
	say := func(what, expr string) {
		t.Helper()
		out = append(out, what+": "+b.eval(expr))
	}
	// The line of id: shown, its names, the button's look and words, and that it is no part of a bar.
	line := func(id string) string {
		return `(() => { const l = document.getElementById(` + js(id) + `); const e = l.querySelector('.tag-edit');
			if (l.hidden || !l.getClientRects().length) return 'hidden';
			return [[...l.querySelectorAll('.tagline-names .tagpill')].map((p) => p.textContent).join(','), e.classList.contains('some'),
				e.title, e.getAttribute('aria-label') === e.title, !e.closest('.frame-actions, .rhead, .bar')].join(' / '); })()`
	}
	pick := func(slug, file string) {
		t.Helper()
		b.eval("pickRow({ slug: " + js(slug) + ", file: " + js(file) + " })")
		b.until(slug+" beside the list", "picked?.slug === "+js(slug))
	}
	const boxes = "[...document.querySelectorAll('#tag-boxes .tag-box')].map((l) => { const c = l.querySelector('input'); " +
		"return l.querySelector('.name').textContent + ':' + (c.indeterminate ? 'some' : c.checked ? 'all' : 'none'); }).join(',')"
	box := func(name string) string {
		return "[...document.querySelectorAll('#tag-boxes .tag-box')].find((l) => l.querySelector('.name').textContent === " + js(name) + ").querySelector('input')"
	}
	toast := func(words string) string {
		return "document.getElementById('toasts').textContent.includes(" + js(words) + ")"
	}
	apply := func(what string) {
		t.Helper()
		b.eval("document.getElementById('toasts').replaceChildren(); document.getElementById('tag-apply').click()")
		b.until(what, "!document.getElementById('tag-dialog').open")
	}

	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=one]')")
	// None, one and many tags; none where the page is not its own or is gone.
	pick("none", "")
	say("none", line("read-tags"))
	pick("one", "")
	say("one", line("read-tags"))
	pick("many", "")
	say("many", line("read-tags"))
	pick("gone", "")
	say("a link whose file is gone", line("read-tags"))
	pick("docs", "index.html")
	say("a folder's page", line("read-tags"))

	// The line's button opens the dialog of that page alone: a page selected stays so, and the focus goes back to the button.
	b.eval("document.querySelector('#list li[data-slug=none] .sel').click()")
	pick("one", "")
	b.eval("document.querySelector('#read-tags .tag-edit').focus(); document.querySelector('#read-tags .tag-edit').click()")
	b.until("the dialog", "document.getElementById('tag-dialog').open")
	say("its dialog", "document.getElementById('tag-title').textContent + ' / ' + "+boxes)
	b.eval(box("a1") + ".click()")
	apply("applied from the line")
	out = append(out, "sent: "+tags("one")+" / "+tags("none"))
	say("after", line("read-tags")+" + ' / ' + "+toast(msg("ui_tag_done", "One"))+" + ' / ' + selected.has('none') + ' / ' + (document.activeElement === document.querySelector('#read-tags .tag-edit'))")
	// Escape gives the focus back to the button as well; the selection's button then has its own back.
	b.eval("document.querySelector('#read-tags .tag-edit').click()")
	b.until("the dialog", "document.getElementById('tag-dialog').open")
	b.pressKey(kb.Escape)
	b.until("closed", "!document.getElementById('tag-dialog').open")
	say("escape", "document.activeElement === document.querySelector('#read-tags .tag-edit')")
	b.eval("document.getElementById('bulk-tag').focus(); document.getElementById('bulk-tag').click()")
	b.until("the selection's dialog", "document.getElementById('tag-dialog').open")
	say("the selection's", "document.getElementById('tag-title').textContent")
	b.pressKey(kb.Escape)
	b.until("closed", "!document.getElementById('tag-dialog').open")
	say("its focus back", "document.activeElement.id")
	b.eval("document.getElementById('bulk-clear').click()")

	// Changed elsewhere to what the dialog asks, nothing is sent; nor of a page gone since it opened.
	b.eval("document.querySelector('#read-tags .tag-edit').click()")
	b.until("the dialog", "document.getElementById('tag-dialog').open")
	b.eval(box("a1") + ".click()")
	tag("one", "plan")
	apply("applied with nothing to change")
	say("nothing to change", toast(msg("ui_tag_bulk_none"))+" + ' / ' + "+toast(msg("ui_tag_done", "One")))
	pick("none", "")
	b.eval("document.querySelector('#read-tags .tag-edit').click()")
	b.until("the dialog", "document.getElementById('tag-dialog').open")
	b.eval(box("plan") + ".click()")
	s.floe(t, "rm", "none")
	apply("applied to a page gone")
	say("a page gone", toast(msg("ui_tag_bulk_none"))+" + ' / ' + document.getElementById('error').hidden")

	// A row's menu: the item named, the dialog of its page, and the focus back on the menu's button of the row drawn anew.
	b.eval("document.querySelector('#list li[data-slug=one] .more').open = true")
	say("the menu's item", "document.querySelector('#list li[data-slug=one] .tag-item').getAttribute('aria-label')")
	b.eval("document.querySelector('#list li[data-slug=one] .tag-item').click()")
	b.until("the dialog", "document.getElementById('tag-dialog').open")
	b.eval(box("a2") + ".click()")
	apply("applied from the menu")
	out = append(out, "sent from the menu: "+tags("one"))
	say("the focus", "document.activeElement === document.querySelector('#list li[data-slug=one] .more > summary')")

	// Enter in the find field takes the one tag it leaves, and makes a name that leaves none or several.
	pick("one", "")
	b.eval("document.querySelector('#read-tags .tag-edit').click()")
	b.until("the dialog", "document.getElementById('tag-dialog').open")
	find := func(text string) {
		t.Helper()
		b.eval("(() => { const f = document.getElementById('tag-find'); f.value = " + js(text) + "; f.dispatchEvent(new Event('input')); f.focus(); })()")
		b.pressKey(kb.Enter)
	}
	find("long")
	say("the one left", boxes+" + ' / ' + document.getElementById('tag-find').value")
	find("zz")
	find("a")
	say("none or several left", "["+box("zz")+".checked, "+box("a")+".checked].join(' ')")
	b.pressKey(kb.Escape)

	// On a task's page, the page beside its list has the line too.
	s.floe(t, "work", "one", "--task", "#1")
	b.until("the page on its task", "items.find((a) => a.slug === 'one')?.work?.task === '#1'")
	b.eval("location.href = taskHref({ project: items.find((a) => a.slug === 'one').taskProject, name: '#1' })")
	b.until("the task's rows", "document.querySelector('#list li[data-slug=one]')")
	pick("one", "")
	say("a task's page", line("read-tags"))

	// In the frame: the line under the bar, its dialog, the menu's item, and none in print or in the diff.
	b.navigate("/a/one/")
	b.until("the frame", "!document.getElementById('frame-tags').hidden")
	say("the frame", line("frame-tags"))
	say("the frame's menu", "!!document.querySelector('#frame-actions .tag-item')")
	b.eval("document.querySelector('#frame-tags .tag-edit').click()")
	b.until("the dialog", "document.getElementById('tag-dialog').open")
	b.eval(box("a3") + ".click()")
	apply("applied in the frame")
	out = append(out, "sent from the frame: "+tags("one"))
	say("redrawn", line("frame-tags"))
	b.media("print")
	say("in print", "getComputedStyle(document.getElementById('frame-tags')).display")
	b.media("")
	b.eval("location.href = document.querySelector('#frame-actions .diff-open').href")
	b.until("the diff", "!document.getElementById('diff').hidden")
	say("the diff", line("frame-tags"))
	b.size(390, 844, true)
	b.navigate("/a/many/")
	b.until("the frame", "!document.getElementById('frame-tags').hidden")
	// The names wrap, the line taller than the bar, and the frame starts under it.
	say("narrow", "document.getElementById('frame-tags').getBoundingClientRect().height > document.querySelector('.bar').getBoundingClientRect().height && "+
		"document.getElementById('frame').getBoundingClientRect().top >= document.getElementById('frame-tags').getBoundingClientRect().bottom")

	added, changed := func(n string) string { return msg("ui_tag_add_named", n) }, func(n string) string { return msg("ui_tag_change_named", n) }
	check(t, "tag line", out, strings.Join([]string{
		"none:  / false / " + added("None") + " / true / true",
		"one: plan / true / " + changed("One") + " / true / true",
		"many: a1,a2,a3,a4,a5,a6,a7,a long name that the line cuts at its end / true / " + changed("Many") + " / true / true",
		"a link whose file is gone: hidden",
		"a folder's page: hidden",
		"its dialog: " + msg("ui_tag_one_title", "One") + " / plan:all,a long name that the line cuts at its end:none,a1:none,a2:none,a3:none,a4:none,a5:none,a6:none,a7:none",
		"sent: plan,a1 / ",
		"after: plan,a1 / true / " + changed("One") + " / true / true / true / true / true",
		"escape: true",
		"the selection's: " + msg("ui_tag_bulk_title", 1),
		"its focus back: bulk-tag",
		"nothing to change: true / false",
		"a page gone: true / true",
		"the menu's item: " + changed("One"),
		"sent from the menu: plan,a2",
		"the focus: true",
		"the one left: a2:all,plan:all,a long name that the line cuts at its end:all,a1:none,a3:none,a4:none,a5:none,a6:none,a7:none / ",
		"none or several left: true true",
		"a task's page: plan,a2 / true / " + changed("One") + " / true / true",
		"the frame: plan,a2 / true / " + changed("One") + " / true / true",
		"the frame's menu: true",
		"sent from the frame: plan,a2,a3",
		"redrawn: plan,a2,a3 / true / " + changed("One") + " / true / true",
		"in print: none",
		"the diff: hidden",
		"narrow: true",
	}, "|"))
}
