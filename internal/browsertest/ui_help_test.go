package browsertest_test

import (
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

// What the help shows: the button pressed, the panels in sight, and whether the focus is on the close button.
const (
	helpPressed = "[...document.querySelectorAll('.help-switch .btn')].filter((b) => b.getAttribute('aria-pressed') === 'true').map((b) => b.dataset.help).join('')"
	helpPanels  = "['help-about', 'help-keys'].filter((id) => document.getElementById(id).offsetParent !== null).join(' ')"
	helpOpen    = "document.getElementById('keys').open + ' ' + " + helpPressed + " + ' ' + " + helpPanels +
		" + ' ' + (document.activeElement === document.querySelector('#keys [data-close]'))"
	// Where the dialog stands and each button, to a tenth: the buttons must not move, nor change their width, as one panel goes for the other.
	helpTops = `(() => { const r = (e) => { const b = e.getBoundingClientRect(); return [b.left, b.top, b.width].map((v) => Math.round(v * 10) / 10).join(','); };
		return r(document.getElementById('keys')) + ' ' + [...document.querySelectorAll('.help-switch .btn')].map(r).join(' '); })()`
	// The box of the panels, as the window leaves it.
	helpBox = `(() => { const d = document.getElementById('keys'), p = document.querySelector('.help-panels'), r = d.getBoundingClientRect(),
		c = document.querySelector('#keys [data-close]').getBoundingClientRect();
		return [d.scrollHeight <= d.clientHeight, p.scrollHeight > p.clientHeight, r.top >= 0 && r.bottom <= innerHeight, c.bottom <= innerHeight].join(' '); })()`
)

// TestUIHelp says how the help opens and what it shows: the bar's button opens what floe is, ? the shortcuts, each with the focus on
// the close button; the buttons stay where they are as a panel goes for the other, and the panels scroll from their top; the words
// of the two screens stand in their sections wherever it opens; the button is there from 641px.
func TestUIHelp(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.page(t, "a.yaml", "T a", ""), "--slug", "a")
	b.size(1440, 900, false)
	defer b.unsize()
	out := make([]string, 0, 32)
	// Enter on the close button shuts it; a click from a script puts no focus on the button that opened it, to go back to, so the
	// focus is let go of, as a press of ? after it takes the key from the page.
	enter := func() {
		t.Helper()
		if err := chromedp.Do(b.ctx, chromedp.KeyEvent(kb.Enter)); err != nil {
			t.Fatal(err)
		}
		b.until("the help shut", "!document.getElementById('keys').open")
		b.eval("document.activeElement.blur()")
	}
	open := func(path string) {
		t.Helper()
		b.navigate(path)
		b.until("the page's script", "typeof openKeys === 'function' && document.readyState === 'complete'")
	}

	// The button opens what floe is, ? the shortcuts, the focus on the close button for both; Enter shuts it.
	open("/")
	b.eval("document.getElementById('keys-open').click()")
	out = append(out, b.settled(helpOpen))
	enter()
	out = append(out, b.eval("document.getElementById('keys').open"))
	keyDown(b, "key: '?', shiftKey: true")
	out = append(out, b.settled(helpOpen))
	enter()
	out = append(out, b.eval("document.getElementById('keys').open"))
	// It opens as its way says, not as it was left: shut on the shortcuts, the button opens what floe is.
	b.eval("document.getElementById('keys-open').click()")
	out = append(out, b.eval(helpOpen))
	// A ? with the help open does nothing, as the focus in a dialog is not a key's (plainKey); nor does a call of openKeys, which
	// the open dialog stops.
	keyDown(b, "key: '?', shiftKey: true")
	b.eval("openKeys('keys')")
	out = append(out, b.eval(helpOpen))
	// With another window open, openKeys leaves the help as it was: shut on the shortcuts, it stays on them, and its d row as it was.
	b.eval("document.getElementById('keys').close(); openKeys(); document.getElementById('keys').close(); openFind()")
	b.eval("for (const e of document.querySelectorAll('#keys .diff-go')) e.hidden = false")
	b.eval("openKeys('about')")
	out = append(out, b.eval("document.getElementById('keys').open + ' ' + "+helpPressed+" + ' ' + [...document.querySelectorAll('#keys .diff-go')].every((e) => !e.hidden)"))
	b.eval("document.getElementById('find').close()")

	// The buttons stay as the panel changes, on the list and on the task list.
	for _, path := range []string{"/", "/tasks"} {
		open(path)
		b.eval("document.getElementById('keys-open').click()")
		before := b.settled(helpTops)
		b.eval("document.querySelector('[data-help=keys]').click()")
		after := b.settled(helpTops)
		out = append(out, path+" "+sameOrMoved(before, after)+" "+b.eval(helpPressed+" + ' ' + "+helpPanels))
		b.eval("document.getElementById('keys').close()")
	}

	// What floe is is the same wherever it opens: the two screens' sections in sight, the words in the section of their screen, and
	// two buttons of the one form (.btn), pressed alone, with no key in the words.
	for _, path := range []string{"/", "/tasks", "/a/a/"} {
		open(path)
		if path == "/a/a/" {
			b.until("the bar", "document.getElementById('frame-title').textContent")
		}
		b.eval("openKeys('about')")
		out = append(out, b.settled("[...document.querySelectorAll('#help-about h3')].map((h) => h.offsetParent !== null).join(' ') + ' ' + "+
			"[...document.querySelectorAll('#help-about dl')].map((d) => d.querySelectorAll('dt').length).join(',') + ' ' + "+
			"document.querySelectorAll('.help-switch .btn').length + ' ' + document.querySelectorAll('.help-switch [aria-controls]').length + ' ' + "+
			"document.querySelectorAll('#help-about kbd').length"))
		b.eval("document.getElementById('keys').close()")
	}

	// The panels' box scrolls alone, from its top: the help in a window of 660 by 600 is longer than the window.
	b.size(660, 600, false)
	open("/")
	b.eval("openKeys('about')")
	out = append(out, b.settled(helpBox))
	b.eval("document.querySelector('.help-panels').scrollTop = 1000")
	scrolled := b.eval("document.querySelector('.help-panels').scrollTop > 100")
	b.eval("document.querySelector('[data-help=keys]').click()")
	out = append(out, scrolled, b.eval("document.querySelector('.help-panels').scrollTop"),
		b.eval("document.querySelector('#help-keys > :not([hidden])').getBoundingClientRect().top >= document.querySelector('.help-panels').getBoundingClientRect().top"))
	for _, way := range []string{"openKeys()", "openKeys('about')"} {
		b.eval("document.querySelector('.help-panels').scrollTop = 20; document.getElementById('keys').close()")
		b.eval(way)
		out = append(out, b.eval("document.querySelector('.help-panels').scrollTop"))
	}
	b.eval("document.getElementById('keys').close()")

	// The bar's button is there from 641px, not at 640px.
	const button = "getComputedStyle(document.getElementById('keys-open')).display !== 'none'"
	for _, width := range []int64{641, 640} {
		b.size(width, 800, false)
		out = append(out, b.eval(button))
	}

	check(t, "help", out, strings.Join([]string{
		// the bar's button, ? and Enter; opened again by the button after the shortcuts; ? and openKeys while it is open; another window open
		"true about help-about true", "false", "true keys help-keys true", "false", "true about help-about true", "true about help-about true", "false keys true",
		// the buttons stay where they are as the panel goes
		"/ same keys help-keys", "/tasks same keys help-keys",
		// the two screens' sections, the words in them, the buttons, no key
		"true true 1,3,4 2 0 0", "true true 1,3,4 2 0 0", "true true 1,3,4 2 0 0",
		// a window of 660 by 600: the box scrolls alone, and goes back to its top on a switch and on opening
		"true true true true", "true", "0", "true", "0", "0",
		// the button from 641px
		"true", "false",
	}, "|"))
}

// sameOrMoved says whether the two readings are the same, and the two if they are not.
func sameOrMoved(before, after string) string {
	if before == after {
		return "same"
	}
	return before + " then " + after
}
