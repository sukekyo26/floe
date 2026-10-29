package browsertest_test

import (
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

// TestUISortMenu picks the list's order from the sort's menu with the mouse and the keys: the order now reached and marked
// as it opens, ↓ End Home moving, Enter picking it (the list and the URL sorted as before) and Escape not, the focus back on
// the sort either way; a press outside or on the sort again closes it; ↓ moves in it, not to a row of the list, and on the
// sort neither; an order in the URL is the one the sort names.
func TestUISortMenu(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	key := func(k string) {
		t.Helper()

		if err := chromedp.Do(b.ctx, chromedp.KeyEvent(k)); err != nil {
			t.Fatal(err)
		}
	}
	press := func(sel string) {
		t.Helper()

		r := b.rect("document.querySelector(" + js(sel) + ")")
		b.click(r.X+r.Width/2, r.Y+r.Height/2)
	}
	open := "document.getElementById('sort-menu').matches(':popover-open')"
	opened := open + " && document.activeElement.id === 'sort-list'"
	// The sort's name and its state, the orders (the one reached in [ ], the one selected, the order now, *), the focus, the
	// URL's order.
	state := `[document.getElementById('sort').textContent, document.getElementById('sort').getAttribute('aria-expanded'),
		[...document.querySelectorAll('#sort-list [role=option]')].map((o) => (o.classList.contains('reached') ? '[' + o.textContent + ']' : o.textContent) + (o.getAttribute('aria-selected') === 'true' ? '*' : '')).join(','),
		document.activeElement.id, String(new URLSearchParams(location.search).get('sort'))].join(' / ')`
	titles := "[...document.querySelectorAll('#list li:not(.group) a.title')].map((a) => a.textContent).join(',')"
	b.stub(extrasList)
	b.navigate("/")
	b.until("the rows", "document.querySelectorAll('#list li:not(.group)').length > 0")

	out := make([]string, 0, 16)
	press("#sort")
	b.until("the orders to open", opened)
	out = append(out, b.eval(state))
	// ↓ End Home move; the first ↓ from the order now.
	key(kb.ArrowDown)
	out = append(out, b.eval(state))
	key(kb.End)
	out = append(out, b.eval(state))
	key(kb.Home)
	out = append(out, b.eval(state))
	// ↓ in it moves in it, not to a row of the list.
	key(kb.ArrowDown)
	out = append(out, b.eval("document.activeElement.id"))
	// Enter picks it: the list sorted and the URL kept, as the select did, the focus back on the sort.
	b.pressKey(kb.Enter)
	out = append(out, b.until("the list to be sorted", "!"+open+" && "+state), b.eval(titles))
	// Escape closes it, the order as it was.
	b.pressKey(kb.Enter)
	b.until("the orders to open", opened)
	key(kb.ArrowDown)
	key(kb.Escape)
	out = append(out, b.until("the orders to close", "!"+open+" && "+state))
	// Pressed again, the sort closes them; a press outside too.
	press("#sort")
	b.until("the orders to open", opened)
	press("#sort")
	out = append(out, b.until("the orders to close", "!"+open+" && 'closed'"))
	press("#sort")
	b.until("the orders to open", opened)
	press("#count")
	out = append(out, b.until("the orders to close", "!"+open+" && 'closed'"))
	// ↓ on the sort does not move to a row of the list (it scrolls the page, as the browser does, which would close orders
	// opened after it; so last).
	b.eval("document.getElementById('sort').focus()")
	key(kb.ArrowDown)
	out = append(out, b.eval("document.activeElement.id"))
	// An order in the URL is the one the sort names and marks.
	b.navigate("/?sort=title")
	b.until("the rows", "document.querySelectorAll('#list li:not(.group)').length > 0")
	out = append(out, b.eval(state))

	// Opened while the page still scrolls (the tail of a scroll before it), the sort stuck under the header bar, it stays open;
	// the page scrolled after so that the sort moves, it closes, as it would be left behind.
	b.stub(manyPages(60))
	b.until("the rows", "document.querySelectorAll('#list li:not(.group)').length === 50")
	b.eval("scrollTo(0, 600)")
	b.until("the sort to be stuck", "scrollY === 600 && 'stuck'")
	b.eval("scrollBy({ top: 300, behavior: 'smooth' }); document.getElementById('sort').click()")
	b.until("the scroll to end", "scrollY === 900 && 'ended'")
	out = append(out, b.eval("String("+open+")"))
	b.eval("scrollTo(0, 0)")
	out = append(out, b.until("the orders to close", "!"+open+" && 'closed'"))
	// An opening stopped on the way (beforetoggle cancelled) leaves the sort saying it is shut.
	b.eval("document.getElementById('sort-menu').addEventListener('beforetoggle', (e) => e.preventDefault(), { once: true }); document.getElementById('sort').click()")
	out = append(out, b.eval("["+open+", document.getElementById('sort').getAttribute('aria-expanded')].join(' ')"))
	// On the sort, the list's other keys still work: → turns the page.
	b.eval("document.getElementById('sort').focus()")
	key(kb.ArrowRight)
	out = append(out, b.until("the next page", "new URLSearchParams(location.search).get('page')"))

	check(t, "sort", out, strings.Join([]string{
		"Recently updated / true / [Recently updated]*,Recently created,Title / sort-list / null",
		"Recently updated / true / Recently updated*,[Recently created],Title / sort-list / null",
		"Recently updated / true / Recently updated*,Recently created,[Title] / sort-list / null",
		"Recently updated / true / [Recently updated]*,Recently created,Title / sort-list / null",
		"sort-list",
		"Recently created / false / Recently updated,[Recently created]*,Title / sort / created", "Gamma,Beta,Alpha",
		// Escaped, the order stays; the one reached last is reached again only as it opens anew
		"Recently created / false / Recently updated,Recently created*,[Title] / sort / created",
		"closed", "closed",
		"sort",
		"Title / false / Recently updated,Recently created,Title* /  / title",
		"true", "closed", "false false", "2",
	}, "|"))
}
