package browsertest_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/chromedp/chromedp/kb"
)

const (
	diffV1 = "<h2>Stay</h2><p>Two beds.</p><p>The hotel by the station, two nights for 26,000.</p><h2>Keep</h2><p>Words kept all along.</p>"
	diffV2 = "<h2>Stay</h2><p>Two beds.</p><p>The hotel by the station, two nights for 28,000.</p><h2>Keep</h2><p>Words kept all along.</p>"
	diffV3 = "<h2>Stay</h2><p>Two beds.</p><p>The hotel by the station, two nights for 38,000.</p><p>&lt;img src=x onerror=&quot;window.pwned=1&quot;&gt;</p>" +
		"<h2>Keep</h2><p>Words kept all along.</p><h2>Rain</h2><p>The museum instead.</p>"
)

// diffSite is a site with d of three versions, not read, one of a single version, and a link.
func diffSite(t *testing.T) *site {
	t.Helper()

	s := newSite(t)
	s.floe(t, "publish", s.page(t, "diff/one.yaml", "One", "<p>one</p>"), "--slug", "one")
	s.floe(t, "publish", "--link", s.write(t, "diff/link/l.html", "<title>L</title><p>l</p>"), "--slug", "l")
	for _, html := range []string{diffV1, diffV2, diffV3} {
		s.floe(t, "publish", s.page(t, "diff/d.yaml", "D", html), "--slug", "d")
	}
	return s
}

const (
	diffPair  = "[document.getElementById('diff-from').value, document.getElementById('diff-to').value].join(' ')"
	diffPath  = "location.pathname + location.search"
	diffShown = "document.querySelector('#diff-body > *')"
)

func openDiff(b *browser, path string) {
	b.t.Helper()

	b.navigate(path)
	b.until("what changed", diffShown)
}

// TestUIDiff compares a page's versions: opening it reads nothing, its words stay words, three ways to show them, and the versions
// chosen in two lists where the keys go on, none of it lost as the list is read again.
func TestUIDiff(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := diffSite(t)
	b := s.b
	b.size(1440, 900, false)
	openDiff(b, "/a/d/?diff")
	out := make([]string, 0, 16)

	// The last two, no page loaded, nothing read.
	const counts = "[...document.querySelectorAll('#diff-count > span')].map((x) => x.textContent).join(', ')"
	const unseen = "fetch('/api/artifacts').then((r) => r.json()).then((l) => l.find((a) => a.slug === 'd').news ?? 'none')"
	out = append(out, b.eval(diffPair), b.eval(counts), b.eval("String(document.getElementById('frame').hasAttribute('src'))"), b.eval(unseen))
	// What is gone, what is added and what is the same, side by side and in one.
	const marks = "[...document.querySelectorAll('#diff-body del, #diff-body ins')].map((x) => x.localName + ':' + x.lastChild.textContent).join(' ')"
	out = append(out, b.eval(marks))
	// Words of a page never become elements, nor run.
	out = append(out, b.eval("String(document.querySelector('#diff-body img') === null && window.pwned === undefined && document.getElementById('diff-body').textContent.includes('<img src=x'))"))
	b.eval("document.querySelector('[data-diff-view=all]').click()")
	out = append(out, b.eval("document.querySelector('.dflow') !== null && document.querySelectorAll('.drow').length === 0 && document.getElementById('diff-body').textContent.includes('Two beds')"), b.eval("document.querySelector('[data-diff-view=all]').getAttribute('aria-pressed')"))
	b.still()
	if low := b.eval(measureContrast); low != "" {
		t.Errorf("all together: text that cannot be read:\n%s", low)
	}
	// Changes only: no section the same, in the words or the contents.
	b.eval("document.querySelector('[data-diff-view=only]').click()")
	out = append(out, b.eval("document.getElementById('diff-body').textContent.includes('Two beds') + ' ' + document.querySelectorAll('.dsame').length + ' ' + [...document.querySelectorAll('#diff-toc button > span:first-child')].map((x) => x.textContent).join('|')"))
	// The way to show it is kept, reloaded.
	openDiff(b, "/a/d/?diff")
	out = append(out, b.eval("document.querySelector('[data-diff-view=only]').getAttribute('aria-pressed')"))
	b.eval("document.querySelector('[data-diff-view=side]').click()")
	b.still()
	if low := b.eval(measureContrast); low != "" {
		t.Errorf("side by side: text that cannot be read:\n%s", low)
	}

	// The list of versions: none not older or newer than the other, and the arrow keys going on from the focus left in it.
	b.eval("document.getElementById('diff-from').focus()")
	out = append(out, b.eval("[...document.getElementById('diff-from').options].map((o) => o.value + (o.disabled ? '-' : '')).join(' ')"),
		b.eval("[...document.getElementById('diff-to').options].map((o) => o.value + (o.disabled ? '-' : '')).join(' ')"))
	b.pressKey(kb.ArrowDown)
	b.until("from v1", "location.search === '?diff&from=1&to=3' && diffAnswer.from.n === 1")
	out = append(out, b.eval("document.activeElement.id + ' ' + document.getElementById('diff-from-open').getAttribute('href')"))

	// n and p from within the list, the focus staying, and through a short window, so that there is somewhere to scroll to.
	b.size(1440, 300, false)
	b.pressKey("n")
	b.pressKey("n")
	b.pressKey("p")
	b.pressKey("n")
	out = append(out, b.eval("diffAt + ' ' + document.activeElement.id + ' ' + document.querySelector('#diff-toc .on > span').textContent"))
	// Drawn once: the list read again leaves what was chosen, the scrolling and n's place.
	const state = "[diffPair.from, diffPair.to, diffAt, document.getElementById('diff-body').scrollTop].join(' ')"
	before := b.eval(state)
	got := b.eval(`(async () => {
		const real = window.fetch, asked = [];
		window.fetch = (u, i) => { asked.push(String(u)); return real(u, i); };
		drawn = '';
		await load(true);
		window.fetch = real;
		return asked.filter((u) => u !== '/api/artifacts').join(' ');
	})()`)
	if after := b.eval(state); got != "" || after != before || after == "1 3 1 0" {
		t.Errorf("the list read again asked %q: %s, was %s", got, after, before)
	}
	check(t, "diff", out, strings.Join([]string{
		"2 3", "1 changed, 1 added, 2 the same", "false", "new",
		"del:2 ins:3 ins:<img src=x onerror=\"window.pwned=1\"> ins:Rain ins:The museum instead.",
		"true", "true", "true",
		"false 0 Stay|Rain",
		"true",
		"3- 2 1", "3 2- 1-",
		"diff-from ?raw&v=1",
		"1 diff-from Rain",
	}, "|"))
	// j from within the list, too: on through the pages, whole.
	b.eval("document.getElementById('diff-from').focus()")
	b.pressKey("j")
	b.until("the next page", "location.pathname === '/a/l/' || location.pathname === '/a/one/'")
}

// TestUIDiffURLs goes to each URL that may be written by hand: the pair is read, or gives way to the first with words saying so. What has
// no versions to compare says that, and closes. What is no page of its own is as ever. None of it loads the page.
func TestUIDiffURLs(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := diffSite(t)
	b := s.b
	b.size(1440, 900, false)
	for _, tc := range []struct{ query, pair, note, search string }{
		{"?diff", "2 3", "true", "?diff"},
		{"?diff&from=1&to=2", "1 2", "true", "?diff&from=1&to=2"},
		{"?diff&from=1", "1 3", "true", "?diff&from=1"},
		{"?diff&to=2", "1 2", "true", "?diff&to=2"},
		{"?diff&from=abc", "2 3", "false", "?diff"},
		{"?diff&from=0&to=3", "2 3", "false", "?diff"},
		{"?diff&from=01&to=3", "2 3", "false", "?diff"},
		{"?diff&from=9&to=3", "2 3", "false", "?diff"},
		{"?diff&from=3&to=3", "2 3", "false", "?diff"},
		{"?diff&from=3&to=1", "2 3", "false", "?diff"},
		{"?diff&to=1", "2 3", "false", "?diff"},
	} {
		openDiff(b, "/a/d/"+tc.query)
		got := b.eval(diffPair) + "|" + b.eval("String(document.getElementById('diff-url-note').hidden)") + "|" + b.eval("location.search")
		if want := tc.pair + "|" + tc.note + "|" + tc.search; got != want {
			t.Errorf("%s: got %s, want %s", tc.query, got, want)
		}
	}

	// One version, and a link: nothing to compare, and the button and d close it.
	for _, slug := range []string{"one", "l"} {
		b.navigate("/a/" + slug + "/?diff")
		b.until("no versions", "!document.getElementById('diff-none').hidden")
		const bar = "document.querySelector('#frame-actions .diff-open')"
		if got := b.eval(bar + ".hidden + ' ' + " + bar + ".textContent.includes('Close the diff') + ' ' + document.getElementById('frame').hasAttribute('src') + ' ' + document.getElementById('diff-main').hidden"); got != "false true false true" {
			t.Errorf("%s: %s", slug, got)
		}
		b.pressKey("d")
		b.until("closed", "location.search === '' && location.pathname === "+js("/a/"+slug+"/"))
		b.until("the page", "document.getElementById('frame').hasAttribute('src')")
		// d on its page goes nowhere, no button there.
		b.eval("window.stayed = true")
		b.pressKey("d")
		if got := b.eval("String(document.querySelector('#frame-actions .diff-open').hidden && window.stayed)"); got != "true" {
			t.Errorf("%s: d on a page of nothing to compare: %s", slug, got)
		}
	}
	// A folder goes to the list, and a page of it is that page.
	s.write(t, "diff/folder/index.html", "<title>t</title><p>i</p>")
	s.floe(t, "publish", "--link", filepath.Join(s.Env["HOME"], "diff", "folder"), "--slug", "folder")
	b.navigate("/a/folder/?diff")
	b.until("the list", "location.pathname === '/' && document.querySelector('#list li[data-slug=folder]')")
	b.navigate("/a/folder/?file=index.html&diff")
	b.until("its page", "document.getElementById('frame').hasAttribute('src')")
	if got := b.eval("String(document.getElementById('diff').hidden)"); got != "true" {
		t.Errorf("a folder's page with diff: %s", got)
	}
}

// TestUIDiffWays goes in by the button and d from beside the list, keeping the list's order for j, and from a page whole, keeping the
// one it had; and out, which opens the page and so reads it.
func TestUIDiffWays(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := diffSite(t)
	b := s.b
	s.floe(t, "publish", s.page(t, "diff/e.yaml", "E", diffV1), "--slug", "e")
	s.floe(t, "publish", s.page(t, "diff/e.yaml", "E", diffV2), "--slug", "e")
	b.size(1440, 900, false)
	b.navigate("/")
	const row = "document.querySelector('#list li[data-slug=%s] .title')"
	pick := func(slug string) {
		t.Helper()
		b.eval(strings.ReplaceAll(row, "%s", slug) + ".click()")
		b.until("picked", "picked?.slug === "+js(slug))
	}
	b.until("the rows", strings.ReplaceAll(row, "%s", "d"))
	// Which rows may be compared: their bar and d.
	const button = "document.querySelector('#read-actions .diff-open')"
	out := make([]string, 0, 8)
	for _, slug := range []string{"one", "l", "e"} {
		pick(slug)
		out = append(out, b.eval(button+".hidden + ''"))
	}
	// Beside the list: the order kept.
	out = append(out, b.eval(button+".getAttribute('href') + ' ' + "+button+".textContent.includes('Diff')"))
	b.eval(strings.ReplaceAll(row, "%s", "e") + ".focus()")
	b.pressKey("d")
	b.until("to the diff", "location.pathname === '/a/e/' && location.search === '?diff'")
	b.until("the order", "sessionStorage.getItem('artifacts.order')?.includes('e')")
	// Closed by the bar's button.
	b.until("the bar", "!document.querySelector('#frame-actions .diff-open').hidden")
	b.eval("document.querySelector('#frame-actions .diff-open').click()")
	b.until("closed", "location.pathname === '/a/e/' && location.search === ''")
	// From a page whole, the order stays what it was.
	b.eval("sessionStorage.setItem('artifacts.order', '[\"e\",\"one\"]')")
	b.navigate("/a/e/")
	b.until("its bar", "!document.querySelector('#frame-actions .diff-open').hidden")
	b.eval("document.querySelector('#frame-actions .diff-open').click()")
	b.until("from whole", "location.search === '?diff'")
	out = append(out, b.eval("sessionStorage.getItem('artifacts.order')"))
	b.until("what changed", diffShown)
	b.pressKey("j")
	b.until("the order's next", "location.pathname === '/a/one/'")

	// d passed on from within the page, as other keys are: to the diff; closing it reads the page, not read so far.
	b.navigate("/a/d/?diff")
	b.until("what changed", diffShown)
	b.pressKey("d")
	b.until("the page d", "location.pathname === '/a/d/' && location.search === ''")
	b.until("read, with its toast", "document.querySelector('.toast.page .btn')")
	b.until("read", "fetch('/api/artifacts').then((r) => r.json()).then((l) => !l.find((a) => a.slug === 'd').news)")
	// Marked unread, the diff and back: read again.
	b.eval("document.querySelector('.toast.page .btn').click()")
	b.until("unread", "fetch('/api/artifacts').then((r) => r.json()).then((l) => l.find((a) => a.slug === 'd').news)")
	b.navigate("/a/d/?diff")
	b.until("what changed again", diffShown)
	if got := b.eval("fetch('/api/artifacts').then((r) => r.json()).then((l) => l.find((a) => a.slug === 'd').news ?? 'none')"); got == "none" {
		t.Error("the diff read the page")
	}
	b.pressKey("d")
	b.until("read again", "fetch('/api/artifacts').then((r) => r.json()).then((l) => !l.find((a) => a.slug === 'd').news)")

	// At a phone's width: all together, no contents, the button short, and shown on a page whole.
	b.size(400, 800, false)
	b.navigate("/a/d/?diff")
	b.until("what changed narrow", diffShown)
	out = append(out, b.eval("document.querySelector('[data-diff-view=all]').getAttribute('aria-pressed') + ' ' + getComputedStyle(document.querySelector('.dtoc')).display + ' ' + document.querySelector('#frame-actions .diff-open').innerText"))
	b.navigate("/a/d/")
	b.until("its bar narrow", "document.querySelector('#frame-actions .diff-open').offsetWidth > 0")
	check(t, "ways", out, strings.Join([]string{"true", "true", "false", "/a/e/?diff true", `["e","one"]`, "true none Close"}, "|"))
}

// TestUIDiffNotice says over a page newly at another version what changed since the one read, not over a page read the first time or
// again. It goes as another loads, as the page is marked unread and as it is drawn at another version, and an answer come after any of
// them says nothing. It holds up no read of another page.
func TestUIDiffNotice(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	publish := func(html string) {
		t.Helper()
		s.floe(t, "publish", s.page(t, "diff/n.yaml", "N", html), "--slug", "n")
	}
	// Of the same number as n's when the words show: what clears them is not its number.
	s.floe(t, "publish", s.page(t, "diff/p.yaml", "P", "<p>p</p>"), "--slug", "p")
	s.floe(t, "publish", s.page(t, "diff/p.yaml", "P", "<p>p2</p>"), "--slug", "p")
	publish(diffV1)
	s.b.addScript(`(() => { const real = window.fetch;
		window.diffHeld = null;
		window.fetch = async (u, i) => {
			if (window.diffHeld && String(u).includes('/diff?')) await window.diffHeld;
			const res = await real(u, i);
			if (String(u).includes('/diff?')) window.diffAnswered = (window.diffAnswered ?? 0) + 1;
			return res;
		};
	})()`)
	b.size(1440, 900, false)
	b.navigate("/")
	const note = "document.querySelector('.dnote.page-note')"
	pick := func(slug string) {
		t.Helper()
		b.eval("document.querySelector('#list li[data-slug=" + slug + "] .title').click()")
		b.until(slug+" read", "picked?.slug === "+js(slug)+" && !readsGoing && document.getElementById('read-frame').contentWindow && !document.querySelector('#list li[data-slug="+slug+"]').classList.contains('unseen')")
	}
	settled := func() {
		t.Helper()
		b.eval("new Promise((go) => { const look = () => (readsGoing ? setTimeout(look, 10) : go()); look(); })")
	}
	b.until("the rows", "document.querySelector('#list li[data-slug=n]') && document.querySelector('#list li[data-slug=p]')")
	pick("p")
	// The first time read: no words.
	pick("n")
	b.until("its toast", "document.querySelector('.toast.page .btn')")
	settled()
	out := make([]string, 0, 8)
	out = append(out, b.eval("String("+note+" === null)"))

	// A new version, opened: the words, and where they lead.
	pick("p")
	publish(diffV2)
	b.until("the new version", "document.querySelector('#list li[data-slug=n]').classList.contains('unseen')")
	b.eval("document.querySelector('#list li[data-slug=n] .title').click()")
	b.until("the words", note)
	out = append(out, b.eval(note+".firstChild.textContent + ' ' + "+note+".querySelector('a').getAttribute('href')"))
	// Another page loaded: gone. And back: read, no words.
	pick("p")
	b.until("gone", note+" === null")
	b.eval("document.querySelector('#list li[data-slug=n] .title').click()")
	b.until("n again", "picked?.slug === 'n'")
	settled()
	out = append(out, b.eval("String("+note+" === null)"))

	// Marked unread: gone.
	pick("p")
	publish(diffV3)
	b.until("v3", "document.querySelector('#list li[data-slug=n]').classList.contains('unseen')")
	b.eval("document.querySelector('#list li[data-slug=n] .title').click()")
	b.until("the words of v3", note)
	b.until("the toast of v3", "document.querySelector('.toast.page .btn')")
	b.eval("document.querySelector('.toast.page .btn').click()")
	b.until("unread: gone", note+" === null")
	// Read again, the words shown, and drawn at another version as it is open: gone.
	b.eval("document.querySelector('#list li[data-slug=p] .title').click()")
	b.until("p", "picked?.slug === 'p'")
	b.eval("document.querySelector('#list li[data-slug=n] .title').click()")
	b.until("the words again", note)
	publish(diffV1)
	b.until("gone as drawn anew", note+" === null")

	// An answer that comes after a move to another page says nothing; and the other's read goes on while it is waited for.
	pick("p")
	publish(diffV2)
	b.until("v5", "document.querySelector('#list li[data-slug=n]').classList.contains('unseen')")
	b.eval("window.diffAnswered = 0; window.diffHeld = new Promise((go) => { window.diffGo = go; }); 1")
	b.eval("document.querySelector('#list li[data-slug=n] .title').click()")
	b.until("n's read told", "picked?.slug === 'n' && !readsGoing && !document.querySelector('#list li[data-slug=n]').classList.contains('unseen')")
	b.eval("document.querySelector('#list li[data-slug=p] .title').click()")
	b.until("p again", "picked?.slug === 'p'")
	b.eval("window.diffGo(); window.diffHeld = null")
	b.until("answered", "window.diffAnswered > 0")
	settled()
	b.eval("new Promise((go) => setTimeout(go, 50))")
	out = append(out, b.eval("String("+note+" === null)"))

	// So too after marked unread, its answer come late.
	publish(diffV3)
	b.until("v6", "document.querySelector('#list li[data-slug=n]').classList.contains('unseen')")
	b.eval("window.diffAnswered = 0; window.diffHeld = new Promise((go) => { window.diffGo = go; }); 1")
	b.eval("document.querySelector('#list li[data-slug=n] .title').click()")
	b.until("the toast of v6", "document.querySelector('.toast.page .btn')")
	b.eval("document.querySelector('.toast.page .btn').click()")
	b.until("unread", "document.querySelector('#list li[data-slug=n]').classList.contains('unseen') && !readsGoing")
	b.eval("window.diffGo(); window.diffHeld = null")
	b.until("answered late", "window.diffAnswered > 0")
	b.eval("new Promise((go) => setTimeout(go, 50))")
	out = append(out, b.eval("String("+note+" === null)"))

	check(t, "notice", out, strings.Join([]string{
		"true", "Sections changed since v1, the version last read: 1 /a/n/?diff", "true", "true", "true",
	}, "|"))
}
