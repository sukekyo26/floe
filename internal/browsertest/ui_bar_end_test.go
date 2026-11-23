package browsertest_test

import (
	"strings"
	"testing"
)

// The header bar's ends, from the left: the server's state is by the logo, apart from the buttons, and the buttons at the end are the
// ones of the list, ending with the settings; a page whole has neither the help nor the settings, only the page's own buttons.
const (
	barIDs   = "[...document.querySelector('[data-testid=bar-end]').children].map((e) => e.id).join(' ')"
	barShown = "[...document.querySelector('[data-testid=bar-end]').children].filter((e) => e.getClientRects().length > 0).map((e) => e.id)"
	// The state of the connection as the bar shows it: its words, whether it is out of the way, and its dot
	serverState = `(() => { const e = document.getElementById('server');
		const dot = getComputedStyle(e, '::before');
		return [e.textContent !== '', e.offsetWidth <= 1, dot.display !== 'none' && !['none', 'normal'].includes(dot.content) ? 'dot' : 'none', ` + roleJS + `(e)].join(' '); })()`
	// Where it stands, down: its words shown (the dot alone when narrow, the 4th), right of the logo and left of what follows, and the bar's things within it
	serverDown = `(() => { const e = document.getElementById('server'), r = e.getBoundingClientRect(), logo = document.querySelector('[data-testid=brand]').getBoundingClientRect();
		const next = e.nextElementSibling.matches('[hidden]') ? document.getElementById('screens') : e.nextElementSibling;
		return [e.textContent !== '', r.left >= logo.right, r.right <= next.getBoundingClientRect().left + 0.5, getComputedStyle(e).fontSize !== '0px',
			document.querySelector('[data-testid=bar-inner]').scrollWidth <= document.querySelector('[data-testid=bar-inner]').clientWidth].join(' '); })()`
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
	out = append(out, b.eval("document.querySelector('[data-testid=brand]').nextElementSibling.id"), b.eval(barIDs), b.eval(barShown+".slice(-2).join(' ')"))
	out = append(out, b.eval(serverState))
	// The links to the screens: one weight, heavier than the bar's words, the screen shown darker, no line under them (S448)
	out = append(out, b.eval(`(() => { const [pages, tasks] = ['to-list', 'to-tasks'].map((id) => getComputedStyle(document.getElementById(id)));
		return [pages.fontWeight, tasks.fontWeight, pages.color !== tasks.color, pages.color === getComputedStyle(document.body).color, pages.borderBottomWidth, tasks.borderBottomWidth].join(' '); })()`))

	// Every request fails while window.down, the polls too, so that the bar holds as lost.
	b.eval(`(() => { const f = window.fetch; window.down = true;
		window.fetch = (url, init) => (window.down ? Promise.reject(new TypeError('Failed to fetch')) : f(url, init)); })()`)
	b.reread()
	b.until("the bar to show the connection lost", "document.getElementById('server').textContent !== ''")
	out = append(out, b.eval(serverState), b.eval(serverDown))
	// The search folds with a transition, which moves what the words must fit beside.
	b.size(400, 800, false)
	out = append(out, b.eval("document.getElementById('server').offsetWidth < 40"), b.settled(serverDown))
	b.eval("window.down = false")
	b.reread()
	b.until("the bar to show the connection back", "document.getElementById('server').textContent === ''")
	out = append(out, b.eval(serverState))
	b.size(1440, 800, false)

	b.navigate("/a/a/")
	b.until("the bar", "document.getElementById('frame-title').textContent")
	out = append(out, b.eval(barShown+".join(' ')"), b.eval("['settings-open', 'keys-open'].map((id) => getComputedStyle(document.getElementById(id)).display).join(' ')"))

	check(t, "bar end", out, strings.Join([]string{
		// The list's bar has no frame's buttons and no way to the search modal beside its search field (S422-P9)
		"server", "news keys-open settings-open", "keys-open settings-open",
		"false true none status", "600 600 true true 0px 0px", "true false dot status", "true true true true true",
		"true", "true true true false true", "false true none status",
		"frame-actions", "none none",
	}, "|"))
}
