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
		"false true none status", "true false dot status", "true true true true true",
		"true", "true true true false true", "false true none status",
		"frame-actions", "none none",
	}, "|"))
}

// screenSwitch says each link of the bar's switch: what its ground and shadow are (the thumb), its borders, its weight, its words'
// colour and its height, and of the track its ground (line, not the bar's), its border, shadow and height, and the bar's. Computed, as a class may draw a frame or a shadow of any name.
const screenSwitch = `(() => { const probe = (name) => { const e = document.body.appendChild(document.createElement('i')); e.style.backgroundColor = 'var(' + name + ')'; const c = getComputedStyle(e).backgroundColor; e.remove(); return c; };
	const bar = document.querySelector('[data-testid=bar-inner]').getBoundingClientRect();
	return ['to-list', 'to-tasks'].map((id) => { const e = document.getElementById(id), s = getComputedStyle(e), r = e.getBoundingClientRect();
		const ground = s.backgroundColor === 'rgba(0, 0, 0, 0)' ? 'none' : s.backgroundColor;
		return [id, ground, s.boxShadow === 'none' ? 'none' : s.boxShadow.replace(/rgba\(0, 0, 0, 0\) 0px 0px 0px 0px, /g, '') === 'rgba(34, 33, 31, 0.06) 0px 1px 2px 0px' ? 'raised' : s.boxShadow, s.borderWidth, s.fontWeight, s.color, r.height, e.getAttribute('aria-current')].join(' '); }).join(' | ')
		+ ' # ' + (() => { const e = document.getElementById('screens'), s = getComputedStyle(e);
			return [s.backgroundColor, s.backgroundColor === probe('--color-line'), s.backgroundColor !== getComputedStyle(document.querySelector('[data-testid=bar]')).backgroundColor, s.borderWidth, s.boxShadow, e.getBoundingClientRect().height, bar.height].join(' '); })(); })()`

// TestUIScreenSwitch draws the bar's switch on every screen: the link of the screen shown stands on a white thumb with the one shadow,
// the other on the track alone and muted, with no frame of either; a task's page is of the task list, and a page whole of the list it
// was opened from, by its URL the page list. A switch moves neither link, by place or by size.
func TestUIScreenSwitch(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.page(t, "a.yaml", "T a", ""), "--slug", "a")
	s.floe(t, "task", "--slug", "a", "--task", "#1", "--title", "One", "--new")
	b.size(1440, 800, false)
	defer b.unsize()

	const thumb, flat, dark, muted = "rgb(255, 255, 255) raised", "none none", "rgb(34, 33, 31)", "rgb(97, 95, 90)"
	line := func(id, ground, color, current string) string {
		return strings.Join([]string{id, ground, "0px 600", color, "26", current}, " ")
	}
	const track = " # rgb(227, 225, 220) true true 0px none 34 56"
	onPages := line("to-list", thumb, dark, "page") + " | " + line("to-tasks", flat, muted, "") + track
	onTasks := line("to-list", flat, muted, "") + " | " + line("to-tasks", thumb, dark, "page") + track
	kept := func(back string) string {
		return `sessionStorage.setItem('pages.order', '{"order":["a"],"shown":["a"],"back":"` + back + `"}')`
	}
	sizes, lefts := make([]string, 0, 7), make([]string, 0, 3)
	for _, c := range []struct{ name, path, before, want string }{
		{"the page list", "/", "", onPages},
		{"the task list", "/tasks?project=home", "", onTasks},
		{"a task's page", "/tasks?project=home&task=%231", "", onTasks},
		{"a page whole, opened by its URL", "/a/a/", "sessionStorage.removeItem('pages.order')", onPages},
		{"a page whole, opened from the page list", "/a/a/", kept("/?view=fav"), onPages},
		{"a page whole, opened from the task list", "/a/a/", kept("/tasks?project=home"), onTasks},
		{"a page whole, opened from a task's page", "/a/a/", kept("/tasks?project=home&task=%231"), onTasks},
	} {
		if c.before != "" {
			b.navigate("/")
			b.eval(c.before)
		}
		b.navigate(c.path)
		b.until(c.name, "document.getElementById('to-tasks')")
		b.until(c.name+" drawn", "document.getElementById('to-list').getClientRects().length > 0")
		if got := b.settled(screenSwitch); got != c.want {
			t.Errorf("%s:\ngot  %s\nwant %s", c.name, got, c.want)
		}
		sizes = append(sizes, b.eval("['to-list', 'to-tasks'].map((id) => { const r = document.getElementById(id).getBoundingClientRect(); return [r.width, r.top, r.height].map((n) => n.toFixed(1)).join(','); }).join(' ')"))
		// A page whole's back link stands before the switch, where it has one, so that its left is its own
		if !strings.HasPrefix(c.name, "a page whole") {
			lefts = append(lefts, b.eval("['to-list', 'to-tasks'].map((id) => document.getElementById(id).getBoundingClientRect().left.toFixed(1)).join(' ')"))
		}
	}
	for _, moved := range [][]string{sizes, lefts} {
		for _, at := range moved[1:] {
			if at != moved[0] {
				t.Errorf("the links stand at %s, and at %s on another screen", moved[0], at)
			}
		}
	}
}
