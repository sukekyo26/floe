package browsertest_test

import (
	"testing"

	"github.com/chromedp/chromedp/kb"

	"floe/internal/store"
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

// TestUIRowMenuItems: a row's menu has its items only while it is open, all of them; one pressed by the keys does what it says, shuts the
// menu and leaves the focus on the menu's button.
func TestUIRowMenuItems(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.page(t, "a.yaml", "T a", ""), "--slug", "a")
	s.floe(t, "task", "--slug", "a", "--task", "#1")
	b.navigate("/")
	const row = "document.querySelector('#list li[data-slug=a]')"
	const menu = row + ".querySelector('[data-testid=more]')"
	const items = "document.querySelectorAll('#list [data-menu-item]').length"
	b.until("the row", row)
	if got := b.eval(items); got != "0" {
		t.Errorf("a menu shut holds %s items, want none", got)
	}
	b.eval(menu + ".querySelector('summary').focus()")
	b.pressKey(kb.Enter)
	b.until("the menu open", menu+".open")
	const all = "fav-item mark-unread select tag-item theme-item pin-item task-set task-out task-open edit copy open download archive delete"
	if got := b.eval("[..." + row + ".querySelectorAll('[data-menu-item]')].map((e) => e.firstElementChild.dataset.testid).join(' ')"); got != all {
		t.Errorf("the menu open holds %q, want %q", got, all)
	}
	b.eval(menu + ".querySelector('[data-testid=pin-item]').focus()")
	b.pressKey(kb.Enter)
	b.until("the menu shut, its items gone", "!"+menu+".open && "+items+" === 0")
	b.until("the focus on its button", "document.activeElement === "+menu+".querySelector('summary')")
	b.until("the pin", "document.querySelector('[data-testid=toast]')")
	if !store.New(s.Env["FLOE_DIR"]).ReadMeta("a").Task.Pin {
		t.Error("pinned from the menu: no pin")
	}
}

// TestUIRowMenuDisarms: a delete pressed once is let go as its menu shuts, whatever shuts it: opened again, one press deletes nothing.
func TestUIRowMenuDisarms(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	for _, slug := range []string{"a", "b"} {
		s.floe(t, "publish", s.page(t, slug+".yaml", "T "+slug, ""), "--slug", slug)
	}
	b.size(1440, 900, false)
	defer b.unsize()
	b.navigate("/")
	summary := func(slug string) string { return "document.querySelector('#list li[data-slug=" + slug + "] summary')" }
	const del = "document.querySelector('#list li[data-slug=a] [data-testid=delete]')"
	b.until("the rows", summary("a")+" && "+summary("b"))
	pointed := func(expr string) {
		t.Helper()
		r := b.rect(expr)
		b.click(r.X+r.Width/2, r.Y+r.Height/2)
	}
	ways := []struct {
		name string
		shut func()
	}{
		{"the script's press of its button", func() { b.eval(summary("a") + ".click()") }},
		{"the script's press of another menu's button", func() { b.eval(summary("b") + ".click()") }},
		{"the pointer's press of another menu's button", func() { pointed(summary("b")) }},
		{"Escape", func() { b.pressKey(kb.Escape) }},
	}
	for _, w := range ways {
		b.eval(summary("a") + ".click()")
		b.until(w.name+": the menu open", del)
		pointed(del)
		b.until(w.name+": armed", del+".dataset.armed")
		w.shut()
		b.until(w.name+": the menu shut", "!"+del)
		b.eval(summary("a") + ".click()")
		b.until(w.name+": the menu open again", del)
		if got := b.eval(del + ".hasAttribute('data-armed')"); got != "false" {
			t.Errorf("%s, opened again: the delete is armed", w.name)
		}
		b.eval(summary("a") + ".click()")
		b.until(w.name+": the menu shut again", "!"+del)
	}
}
