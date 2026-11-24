package browsertest_test

import (
	"testing"

	"github.com/chromedp/chromedp/kb"
)

// TestUIRowMenuStays opens a row's "…" menu and has the page read the same list again: the menu stays open, its row the same element.
// A read that changes the list may draw the menu away; one of the same list draws nothing, so that the menu a person
// opened is not closed under them, and pressMenuItem need not open it again more than once.
func TestUIRowMenuStays(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.page(t, "a.yaml", "T a", ""), "--slug", "a")
	b.navigate("/")
	const menu = "document.querySelector('#list li[data-slug=a] [data-testid=more]')"
	b.until("the row", menu)
	b.eval("window.rowBefore = document.querySelector('#list li[data-slug=a]'); " + menu + ".querySelector('summary').click()")
	b.until("the menu open", menu+".open")
	begun := b.reread()
	b.untilRead(begun, "the same list read again", "true")
	b.still()
	if got := b.eval(menu + ".open + ' ' + (document.querySelector('#list li[data-slug=a]') === window.rowBefore)"); got != "true true" {
		t.Errorf("the menu after the same list read again: open, same row %q, want \"true true\"", got)
	}
}

// A row's menu is the store's from its summary's press: an item pressed before the browser tells the menu opened (its toggle event is a
// task later) closes it, and the next press opens it. The keys open and close it too.
func TestUIRowMenuPressed(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.page(t, "a.yaml", "T a", ""), "--slug", "a")
	b.navigate("/")
	const menu = "document.querySelector('#list li[data-slug=a] [data-testid=more]')"
	b.until("the row", menu)
	b.eval(menu + ".querySelector('summary').click(); " + menu + ".querySelector('[data-testid=edit]').click()")
	b.until("the edit modal", "document.getElementById('edit-dialog').open")
	b.until("the menu closed", "!"+menu+".open")
	b.pressKey(kb.Escape)
	b.until("the modal closed", "!document.getElementById('edit-dialog').open")
	b.eval(menu + ".querySelector('summary').click()")
	b.until("the menu opened by the next press", menu+".open")
	b.eval(menu + ".querySelector('summary').click()")
	b.until("the menu closed by the press after", "!"+menu+".open")

	b.eval(menu + ".querySelector('summary').focus()")
	for _, k := range []string{kb.Enter, " "} {
		b.pressKey(k)
		b.until("the menu opened by a key", menu+".open")
		b.pressKey(k)
		b.until("the menu closed by a key", "!"+menu+".open")
	}
}
