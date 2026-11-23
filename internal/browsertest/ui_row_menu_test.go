package browsertest_test

import "testing"

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
