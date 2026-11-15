package browsertest_test

import (
	"strings"
	"testing"
)

// The header bar's ends, from the left: the server's state is by the logo, apart from the buttons, and the buttons at the end are the
// ones of the list, ending with the settings; a page whole has neither the help nor the settings, only the page's own buttons.
const (
	barIDs   = "[...document.querySelector('.bar-end').children].map((e) => e.id).join(' ')"
	barShown = "[...document.querySelector('.bar-end').children].filter((e) => e.getClientRects().length > 0).map((e) => e.id)"
	// The state of the connection as the bar shows it: its words, whether it is out of the way, and its dot
	serverState = `(() => { const e = document.getElementById('server');
		return [e.textContent !== '', e.offsetWidth <= 1, getComputedStyle(e, '::before').display, e.getAttribute('role')].join(' '); })()`
	// Where it stands, down: its words shown (the dot alone when narrow, the 4th), right of the logo and left of what follows, and the bar's things within it
	serverDown = `(() => { const e = document.getElementById('server'), r = e.getBoundingClientRect(), logo = document.querySelector('.brand').getBoundingClientRect();
		const next = e.nextElementSibling.matches('[hidden]') ? document.querySelector('.screens') : e.nextElementSibling;
		return [e.textContent !== '', r.left >= logo.right, r.right <= next.getBoundingClientRect().left + 0.5, getComputedStyle(e).fontSize !== '0px',
			document.querySelector('.bar-inner').scrollWidth <= document.querySelector('.bar-inner').clientWidth].join(' '); })()`
)

// TestUIBarEnd says what the header bar holds and where: the connection's state, out of the way while it holds and by the logo when
// it is lost (its words with room, its dot alone without); the buttons of the list in their order; and a page whole without the settings
// and the help.
func TestUIBarEnd(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.page(t, "a.yaml", "T a", ""), "--slug", "a")
	b.size(1440, 800, false)
	defer b.unsize()
	out := make([]string, 0, 12)

	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li')")
	out = append(out, b.eval("document.querySelector('.brand').nextElementSibling.id"), b.eval(barIDs), b.eval(barShown+".slice(-2).join(' ')"))
	out = append(out, b.eval(serverState))

	// The poll tells the connection as it is: hold it, to read the bar as lost.
	b.eval("window.tell = setServer; setServer = () => {}; tell(false)")
	out = append(out, b.eval(serverState), b.eval(serverDown))
	// The search folds with a transition, which moves what the words must fit beside.
	b.size(400, 800, false)
	out = append(out, b.eval("document.getElementById('server').offsetWidth < 40"), b.settled(serverDown))
	b.eval("tell(true)")
	out = append(out, b.eval(serverState))
	b.size(1440, 800, false)

	b.navigate("/a/a/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	out = append(out, b.eval(barShown+".join(' ')"), b.eval("['settings-open', 'keys-open'].map((id) => getComputedStyle(document.getElementById(id)).display).join(' ')"))

	check(t, "bar end", out, strings.Join([]string{
		"server", "frame-actions news find-open keys-open settings-open", "keys-open settings-open",
		"false true none status", "true false block status", "true true true true true",
		"true", "true true true false true", "false true none status",
		"frame-actions", "none none",
	}, "|"))
}
