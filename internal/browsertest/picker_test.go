package browsertest_test

import (
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

// TestUIPagePicker picks a page from the frame's bar with the mouse and the keys: the title opens the pages to pick from,
// searched as the list is; ↑↓ go round, Enter opens the page reached (only closing on the one open now), Escape closes and
// gives the focus back; a press in the page under the bar closes it, the title pressed again too; what is typed and reached
// stays while the list is fetched again; ← and → move nowhere; the title and the field take the one focus ring.
func TestUIPagePicker(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	// Keys that type, and the arrows; Enter is pressed as a keyboard does (pressKey), so that a page's preventDefault stops it
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
	open := "document.getElementById('page-pick').matches(':popover-open')"
	// Opened, with the focus in its field, which it takes as the box shows (its toggle event)
	opened := open + " && document.activeElement.id === 'page-pick-q'"
	// The pages offered, the one reached in [ ], the focus and whether the none-found shows.
	state := `[[...document.querySelectorAll('#page-pick-list [role=option]')].map((o) => o.getAttribute('aria-selected') === 'true' ? '[' + o.textContent + ']' : o.textContent).join(','),
		document.getElementById(document.getElementById('page-pick-q').getAttribute('aria-activedescendant'))?.textContent ?? '-',
		document.activeElement.id, !document.getElementById('page-pick-none').hidden].join(' ')`
	// Leaving the page, as Enter on a page would, is told at once: beforeunload runs as location is set.
	b.navigate("/")
	b.eval("sessionStorage.setItem('artifacts.list', '')")
	b.navigate("about:blank")
	b.stub(colorsList)
	b.navigate("/a/a/")
	b.until("the title", "!document.getElementById('frame-pick').hidden")
	b.eval("addEventListener('beforeunload', () => { window.left = true; })")

	out := make([]string, 0, 24)
	// Pressed, the title opens the pages, the one open now reached and marked, the focus in the field.
	press("#frame-pick")
	b.until("the pages to open", opened)
	out = append(out, b.eval("document.getElementById('frame-pick').getAttribute('aria-expanded')"), b.eval(state),
		b.eval("document.querySelector('#page-pick-list [aria-current=page]').textContent"))
	// Typed, the pages are searched as the list is (a tag, a description), the first found reached; none found, it says so.
	key("plan")
	out = append(out, b.eval(state))
	key("x")
	out = append(out, b.eval(state))
	b.eval("const q = document.getElementById('page-pick-q'); q.value = 'about'; q.dispatchEvent(new Event('input'))")
	out = append(out, b.eval(state))
	b.eval("const q2 = document.getElementById('page-pick-q'); q2.value = 'b'; q2.dispatchEvent(new Event('input'))")
	// ↑ from the first goes round to the last, ↓ from there to the first.
	key(kb.ArrowUp)
	out = append(out, b.eval(state))
	key(kb.ArrowDown)
	out = append(out, b.eval(state))
	// What is typed and reached stays while the list is fetched again, changed (a page given a new title), which draws the
	// frame anew: the pages offered are made as it opens, not as the list comes.
	key(kb.ArrowDown)
	before := b.eval(state + " + ' ' + document.getElementById('page-pick-q').value")
	b.eval("window.list = JSON.stringify(JSON.parse(window.list).map((a) => a.slug === 'p9' ? { ...a, title: 'P9 anew' } : a)); load()")
	b.until("the list to be drawn", "items.some((a) => a.title === 'P9 anew')")
	out = append(out, strings.ReplaceAll(b.eval(state+" + ' ' + document.getElementById('page-pick-q').value"), before, "kept"))
	// The pages are links: a click with a key held (or the middle button) is the browser's, to open one in a new tab; a
	// listener after the page's tells whether the page stopped it, and stops it itself, so that no tab opens.
	out = append(out, b.eval("[...document.querySelectorAll('#page-pick-list [role=option]')].map((o) => o.tagName + ' ' + o.getAttribute('href')).join(',')"))
	b.eval("addEventListener('click', (e) => { window.stopped = e.defaultPrevented; e.preventDefault(); }, { once: true }); " +
		"document.querySelectorAll('#page-pick-list [role=option]')[1].dispatchEvent(new MouseEvent('click', { ctrlKey: true, bubbles: true, cancelable: true }))")
	out = append(out, b.eval("[String(window.stopped), String(window.left), "+open+"].join(' ')"))
	// Pressed on a heading, the field keeps the focus, and so the keys.
	press("#page-pick-list .gh")
	out = append(out, b.eval("document.activeElement.id"))
	// Escape closes it and gives the focus back to the title.
	key(kb.Escape)
	out = append(out, b.eval("["+open+", document.activeElement.id].join(' ')"))
	// Enter at once, on the page open now, only closes it.
	b.pressKey(kb.Enter)
	b.until("the pages to open", opened)
	b.pressKey(kb.Enter)
	out = append(out, b.eval("["+open+", String(window.left), document.getElementById('frame').getAttribute('src'), document.activeElement.id].join(' ')"))
	// The title pressed again closes it, not opening it anew.
	press("#frame-pick")
	b.until("the pages to open", opened)
	press("#frame-pick")
	out = append(out, b.eval(open))
	// ← and → move nowhere.
	b.eval("document.body.tabIndex = -1; document.body.focus(); document.body.removeAttribute('tabindex')")
	key(kb.ArrowRight)
	key(kb.ArrowLeft)
	out = append(out, b.eval("String(window.left) + ' ' + location.pathname"))
	// The title and the field take the one focus ring, reached by keyboard.
	for range 10 {
		key(kb.Tab)
		if b.eval("document.activeElement.id") == "frame-pick" {
			break
		}
	}
	out = append(out, b.eval("document.activeElement.id")+": "+b.eval(ringOf))
	b.pressKey(kb.Enter)
	b.until("the pages to open", opened)
	out = append(out, b.eval("document.activeElement.id")+": "+b.eval(ringOf))
	// ↓ then Enter opens the page reached.
	key(kb.ArrowDown)
	b.pressKey(kb.Enter)
	out = append(out, b.until("the page to open", "location.pathname === '/a/b/' && location.pathname"))
	// A press in the page under the bar, which this page is not told of, closes it too. Last, as the focus then is in that page
	b.until("the title", "!document.getElementById('frame-pick').hidden")
	press("#frame-pick")
	b.until("the pages to open", opened)
	r := b.rect("document.getElementById('frame')")
	// Off the box, which opens over the top left of the page
	b.click(r.X+r.Width-40, r.Y+r.Height-40)
	out = append(out, b.until("the pages to close", "!"+open+" && 'closed'"))
	// The page gone while they are open, they close with the title that opened them.
	b.eval("document.getElementById('frame-pick').click()")
	b.until("the pages to open", open)
	b.eval("window.list = JSON.stringify(JSON.parse(window.list).filter((a) => a.slug !== 'b')); load()")
	out = append(out, b.until("the gone page's pages to close", "!"+open+" && document.getElementById('frame-pick').hidden && 'gone'"))

	pages := "Alpha,Beta,P0,P1,P2,P3,P4,P5,P6,P7,P8,P9"
	ring := "solid 2px rgb(107, 105, 100) 1px seen"
	check(t, "picker", out, strings.Join([]string{
		"true", "[Alpha]," + strings.TrimPrefix(pages, "Alpha,") + " Alpha page-pick-q false", "Alpha",
		"[Alpha],Beta Alpha page-pick-q false", " - page-pick-q true", "[Alpha] Alpha page-pick-q false",
		"Alpha,[Beta] Beta page-pick-q false", "[Alpha],Beta Alpha page-pick-q false",
		"kept",
		"A /a/a/,A /a/b/", "false undefined true", "page-pick-q",
		"false frame-pick",
		"false undefined ?raw frame-pick",
		"false",
		"undefined /a/a/",
		"frame-pick: " + ring, "page-pick-q: " + ring,
		"/a/b/", "closed", "gone",
	}, "|"))
}
