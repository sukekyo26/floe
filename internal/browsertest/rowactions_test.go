package browsertest_test

import (
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/store"
)

// A row's project as a pill in its chip's color, in the list and the search modal, and the row's button that archives or gives back:
// beside the star, not in the menu, the focus going to the same button of the row after as a row leaves, to the last at the end.
func TestUIRowActions(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	for _, p := range [][2]string{{"a", "one"}, {"b", "one"}, {"c", "two"}, {"d", strings.Repeat("a-long-name-", 5)}} {
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
	press := func(selector string) {
		t.Helper()
		b.eval("document.querySelector(" + js(selector) + ").focus(); document.querySelector(" + js(selector) + ").click()")
	}
	archive := func(slug string) string { return "#list li[data-slug=" + slug + "] .actions > .archive" }
	pill := func(slug string) string { return "document.querySelector('#list li[data-slug=" + slug + "] .proj')" }
	dot := func(slug string) string { return "getComputedStyle(" + pill(slug) + ", '::before').backgroundColor" }
	chip := func(project string) string {
		return "getComputedStyle(document.querySelector('.chip[data-project=" + project + "]'), '::before').backgroundColor"
	}
	const focus = "document.activeElement.closest('li').dataset.slug + ' ' + document.activeElement.classList[1]"

	b.navigate("/")
	b.until("the rows", pill("d"))
	say("the rows", "[...document.querySelectorAll('#list li[data-slug]')].map((li) => li.dataset.slug).join('')")
	say("a pill's dot in its chip's color", dot("a")+" === "+chip("one")+" && "+dot("c")+" === "+chip("two"))
	say("each project's own", dot("a")+" !== "+dot("c"))
	say("the ground of the color too", "getComputedStyle("+pill("a")+").backgroundColor !== getComputedStyle("+pill("c")+").backgroundColor")
	say("a long name cut within the pill", "(() => { const e = "+pill("d")+"; return e.scrollWidth > e.clientWidth && e.offsetWidth <= 10 * parseFloat(getComputedStyle(document.documentElement).fontSize); })()")
	say("the row's buttons", "[...document.querySelector('#list li[data-slug=a] .actions').children].map((e) => e.classList[1] ?? e.className).join(' ')")
	b.eval("document.querySelector('#list li[data-slug=a] .more').open = true")
	say("the menu leaves it out", "getComputedStyle(document.querySelector('#list li[data-slug=a] .menu .archive')).display")
	b.eval("document.querySelector('#list li[data-slug=a] .more').open = false")
	say("its words", "document.querySelector("+js(archive("a"))+").title + ' / ' + document.querySelector("+js(archive("a"))+").getAttribute('aria-label')")

	press(archive("c"))
	b.until("c archived", "!document.querySelector('#list li[data-slug=c]')")
	out = append(out, "the row after: "+b.until("the focus", focus))
	press(archive("a"))
	b.until("a archived", "!document.querySelector('#list li[data-slug=a]')")
	out = append(out, "the last row: "+b.until("the focus", focus))

	b.navigate("/?view=archived")
	b.until("the archived", "document.querySelector("+js(archive("c"))+")")
	say("an archived row's button", "document.querySelector("+js(archive("c"))+").classList.contains('on') + ' ' + document.querySelector("+js(archive("c"))+").title")
	press(archive("c"))
	b.until("c given back", "!document.querySelector('#list li[data-slug=c]')")
	say("the last left", "document.querySelector('#list li[data-slug]').dataset.slug + ' ' + document.activeElement.classList[1]")

	// An item of the menu done by keys: the menu closes, the focus going back to its button.
	b.navigate("/")
	b.until("the rows again", "document.querySelector('#list li[data-slug=c]')")
	b.eval("document.querySelector('#list li[data-slug=c] summary').click()")
	press("#list li[data-slug=c] .pin-item")
	out = append(out, "the menu's item: "+b.until("the pin", "!document.querySelector('#list li[data-slug=c] .more')?.open && document.activeElement.tagName === 'SUMMARY' && document.activeElement.closest('li').dataset.slug"))

	// The search modal draws the same pill.
	b.eval("document.activeElement.blur()")
	realKey(b, "k", 2)
	b.until("the modal's rows", "document.querySelector('#find-list .proj')")
	say("the modal's pill", "getComputedStyle(document.querySelector('#find-list .proj'), '::before').backgroundColor !== 'rgba(0, 0, 0, 0)'")

	check(t, "row actions", out, rowActionsWant)
}

const rowActionsWant = "the rows: dcba|a pill's dot in its chip's color: true|each project's own: true|the ground of the color too: true|a long name cut within the pill: true|the row's buttons: archive star more|the menu leaves it out: none|its words: Archive / Archive T a|the row after: b archive|the last row: b archive|an archived row's button: true Restore|the last left: a archive|the menu's item: c|the modal's pill: true"
