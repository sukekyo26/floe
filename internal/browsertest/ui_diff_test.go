package browsertest_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chromedp/chromedp"
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
	diffPair = "[document.getElementById('diff-from').value, document.getElementById('diff-to').value].join(' ')"
	diffPath = "location.pathname + location.search"
	// The marked page of the answer shown, in the frame
	diffShown = "typeof diffAnswer !== 'undefined' && diffAnswer !== null && document.getElementById('diff-frame').src.includes('key=' + diffAnswer.key)"
	// The words marked on the page in the frame, read from the server as the frame reads it
	diffMarks = "fetch(document.getElementById('diff-frame').src).then((r) => r.text()).then((h) => [...new DOMParser().parseFromString(h, 'text/html').querySelectorAll('ins.floe-diff-ins, del.floe-diff-del')].map((x) => x.localName + ':' + x.textContent).join(' '))"
)

func openDiff(b *browser, path string) {
	b.t.Helper()

	b.navigate(path)
	b.until("what changed", diffShown)
}

// TestUIDiff compares a page's versions: opening it reads nothing, the later version's page is shown marked in the
// sandbox, its words never elements of the management page, and the versions chosen in two lists where the keys go
// on, none of it lost as the list is read again.
func TestUIDiff(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := diffSite(t)
	b := s.b
	b.size(1440, 900, false)
	openDiff(b, "/a/d/?diff")
	out := make([]string, 0, 16)

	// The last two, no page loaded in the page's frame, nothing read.
	const counts = "[...document.querySelectorAll('#diff-count > span')].map((x) => x.textContent).join(', ')"
	const unseen = "fetch('/api/pages').then((r) => r.json()).then((l) => l.find((a) => a.slug === 'd').news ?? 'none')"
	out = append(out, b.eval(diffPair), b.eval(counts), b.eval("String(document.getElementById('frame').hasAttribute('src'))"), b.eval(unseen))
	// What is gone and what is added, on the page in the frame.
	out = append(out, b.eval(diffMarks))
	// Words of a page that look like HTML stay words there, and nothing of the page runs in the management page.
	out = append(out, b.eval("String(window.pwned === undefined && document.getElementById('diff').querySelector('img') === null)"))
	b.still()
	if low := b.eval(measureContrast); low != "" {
		t.Errorf("text that cannot be read:\n%s", low)
	}

	// The list of versions: none not older or newer than the other, the keys going on from the focus left in it.
	b.eval("document.getElementById('diff-from').focus()")
	out = append(out, b.eval("[...document.getElementById('diff-from').options].map((o) => o.value + (o.disabled ? '-' : '')).join(' ')"),
		b.eval("[...document.getElementById('diff-to').options].map((o) => o.value + (o.disabled ? '-' : '')).join(' ')"))
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.Enter)
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
	const state = "[diffPair.from, diffPair.to, diffAt, document.getElementById('diff-frame').src].join(' ')"
	before := b.eval(state)
	got := b.eval(`(async () => {
		const real = window.fetch, asked = [];
		window.fetch = (u, i) => { asked.push(String(u)); return real(u, i); };
		drawn = '';
		await load(true);
		window.fetch = real;
		return asked.filter((u) => u !== '/api/pages').join(' ');
	})()`)
	if after := b.eval(state); got != "" || after != before || !strings.HasSuffix(after, "#floe-diff.1") {
		t.Errorf("the list read again asked %q: %s, was %s", got, after, before)
	}
	check(t, "diff", out, strings.Join([]string{
		"2 3", "1 changed, 1 added, 2 the same", "false", "new",
		"del:2 ins:3 ins:<img src=x onerror=\"window.pwned=1\"> ins:Rain ins:The museum instead.",
		"true",
		"3- 2 1", "3 2- 1-",
		"diff-from /a/d/?raw&v=1",
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
		b.still()
		if low := b.eval(measureContrast); low != "" {
			t.Errorf("%s: text that cannot be read:\n%s", tc.query, low)
		}
		got := b.eval(diffPair) + "|" + b.eval("String(document.getElementById('diff-url-note').hidden)") + "|" + b.eval("location.search")
		if want := tc.pair + "|" + tc.note + "|" + tc.search; got != want {
			t.Errorf("%s: got %s, want %s", tc.query, got, want)
		}
	}

	// One version, and a link: nothing to compare, and the button and d close it.
	for _, slug := range []string{"one", "l"} {
		b.navigate("/a/" + slug + "/?diff")
		b.until("no versions", "!document.getElementById('diff-none').hidden")
		b.still()
		if low := b.eval(measureContrast); low != "" {
			t.Errorf("%s: text that cannot be read:\n%s", slug, low)
		}
		const bar = "document.querySelector('#frame-actions .diff-open')"
		if got := b.eval(bar + ".hidden + ' ' + " + bar + ".getAttribute('aria-label').includes('Close the diff') + ' ' + document.getElementById('frame').hasAttribute('src') + ' ' + document.getElementById('diff-main').hidden"); got != "false true false true" {
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
	out = append(out, b.eval(button+".getAttribute('href') + ' ' + "+button+".getAttribute('aria-label')"))
	b.eval(strings.ReplaceAll(row, "%s", "e") + ".focus()")
	b.pressKey("d")
	b.until("to the diff", "location.pathname === '/a/e/' && location.search === '?diff'")
	b.until("the order", "sessionStorage.getItem(ORDER_KEY)?.includes('e')")
	// Closed by the bar's button.
	b.until("the bar", "!document.querySelector('#frame-actions .diff-open').hidden")
	b.moving("document.querySelector('#frame-actions .diff-open').click()")
	b.until("closed", "location.pathname === '/a/e/' && location.search === ''")
	// From a page whole, the order stays what it was.
	b.eval(`sessionStorage.setItem(ORDER_KEY, '{"order":["e","one"],"shown":["e","one"],"back":"/"}')`)
	b.navigate("/a/e/")
	b.until("its bar", "!document.querySelector('#frame-actions .diff-open').hidden")
	b.moving("document.querySelector('#frame-actions .diff-open').click()")
	b.until("from whole", "location.search === '?diff'")
	out = append(out, b.eval("sessionStorage.getItem(ORDER_KEY)"))
	b.until("what changed", diffShown)
	b.pressKey("j")
	b.until("the order's next", "location.pathname === '/a/one/'")

	// d passed on from within the page, as other keys are: to the diff; closing it reads the page, not read so far.
	b.navigate("/a/d/?diff")
	b.until("what changed", diffShown)
	b.pressKey("d")
	b.until("the page d", "location.pathname === '/a/d/' && location.search === ''")
	b.until("read, with its toast", "document.querySelector('.toast.page .btn')")
	b.until("read", "fetch('/api/pages').then((r) => r.json()).then((l) => !l.find((a) => a.slug === 'd').news)")
	// Marked unread, the diff and back: read again.
	b.eval("document.querySelector('.toast.page .btn').click()")
	b.until("unread", "fetch('/api/pages').then((r) => r.json()).then((l) => l.find((a) => a.slug === 'd').news)")
	b.navigate("/a/d/?diff")
	b.until("what changed again", diffShown)
	if got := b.eval("fetch('/api/pages').then((r) => r.json()).then((l) => l.find((a) => a.slug === 'd').news ?? 'none')"); got == "none" {
		t.Error("the diff read the page")
	}
	b.pressKey("d")
	b.until("read again", "fetch('/api/pages').then((r) => r.json()).then((l) => !l.find((a) => a.slug === 'd').news)")

	// At a phone's width: the page's frame, no contents, the button short, and shown on a page whole.
	b.size(400, 800, false)
	b.navigate("/a/d/?diff")
	b.until("what changed narrow", diffShown)
	out = append(out, b.eval("String(document.getElementById('diff-frame').getBoundingClientRect().width > 300) + ' ' + getComputedStyle(document.querySelector('.dtoc')).display + ' ' + document.querySelector('#frame-actions .diff-open').getAttribute('aria-label') + ' ' + document.querySelector('#frame-actions .diff-open').matches('.closing')"))
	b.navigate("/a/d/")
	b.until("its bar narrow", "document.querySelector('#frame-actions .diff-open').offsetWidth > 0")
	check(t, "ways", out, strings.Join([]string{"true", "true", "false", "/a/e/?diff See what changed (d)", `{"order":["e","one"],"shown":["e","one"],"back":"/","at":{"slug":"e","file":"","v":0}}`, "true none Close the diff (d) true"}, "|"))
}

// TestUIDiffPick has the versions chosen from the browser's own list, drawn by the page: the arrows open it and move in it, asking for nothing
// until Enter, the page's keys wait until it is closed, and a browser that does not read base-select has the list it has always had.
func TestUIDiffPick(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := diffSite(t)
	b := s.b
	b.size(1440, 300, false)
	openDiff(b, "/a/d/?diff")
	const state = `(() => {
		const e = document.getElementById('diff-from');
		return [e.matches(':open'), e.value, diffAnswer.from.n, diffAt, document.activeElement.localName, window.asked ?? 0].join(' ');
	})()`
	b.eval(`(() => { const real = window.fetch; window.fetch = (u, i) => { if (String(u).includes('/diff?')) window.asked = (window.asked ?? 0) + 1; return real(u, i); }; })()`)
	b.eval("document.getElementById('diff-from').focus()")
	out := make([]string, 0, 16)
	out = append(out, b.eval("getComputedStyle(document.getElementById('diff-from')).appearance"), b.eval(state))
	// Closed, the arrows open it, the version as it was; the versions that cannot be chosen are passed over.
	b.pressKey(kb.ArrowDown)
	out = append(out, b.eval(state), b.eval("document.activeElement.value"))
	b.still()
	if low := b.eval(measureContrast); low != "" {
		t.Errorf("open: text that cannot be read:\n%s", low)
	}
	// Open, the page has no key, until Escape closes it with nothing chosen.
	for _, key := range []string{"n", "p", "d", "j", "k"} {
		b.pressKey(key)
	}
	out = append(out, b.eval(state), b.eval("location.pathname + location.search"))
	b.pressKey(kb.Escape)
	out = append(out, b.eval(state))
	// Closed, the page's keys are its own.
	b.pressKey("n")
	out = append(out, b.eval(state))
	b.pressKey("n")
	b.pressKey("p")
	out = append(out, b.eval(state))
	// Opened by the mouse, no key of the page's either.
	r := b.rect("document.getElementById('diff-from')")
	b.mouse(chromedp.MousePressed, r.X+r.Width/2, r.Y+r.Height/2, 1)
	b.mouse(chromedp.MouseReleased, r.X+r.Width/2, r.Y+r.Height/2, 1)
	b.until("opened", "document.getElementById('diff-from').matches(':open')")
	b.pressKey("n")
	out = append(out, b.eval(state))
	b.pressKey(kb.Escape)
	// Open, moved and chosen: asked once, and the place goes back to the top with the answer.
	b.pressKey("n")
	b.pressKey(kb.ArrowDown)
	b.pressKey(kb.ArrowDown)
	out = append(out, b.eval(state))
	b.pressKey(kb.Enter)
	out = append(out, b.until("from v1", "diffAnswer.from.n === 1 && "+state))
	b.pressKey("n")
	out = append(out, b.eval(state))
	b.pressKey("d")
	b.until("closed", "location.search === ''")
	check(t, "pick", out, strings.Join([]string{
		"base-select", "false 2 2 -1 select 0",
		"true 2 2 -1 option 0", "2",
		"true 2 2 -1 option 0", "/a/d/?diff",
		"false 2 2 -1 select 0",
		"false 2 2 0 select 0",
		"false 2 2 0 select 0",
		"true 2 2 0 option 0",
		"true 2 2 1 option 0",
		"false 1 1 -1 select 1",
		"false 1 1 0 select 1",
	}, "|"))

	// At a phone's width the list stays in the window.
	b.size(400, 800, false)
	openDiff(b, "/a/d/?diff")
	b.eval("document.getElementById('diff-from').focus()")
	b.pressKey(kb.ArrowDown)
	if got := b.eval(`(() => {
		const o = document.activeElement.getBoundingClientRect(), e = document.getElementById('diff-from').getBoundingClientRect();
		return o.left >= 0 && o.right <= innerWidth && o.bottom <= innerHeight && e.right <= innerWidth;
	})()`); got != "true" {
		t.Errorf("at a phone's width the list leaves the window")
	}
	b.pressKey(kb.Escape)

	// Without the rules for base-select, as a browser that does not read them: the same height and words, the list its own, chosen the same.
	b.size(1440, 900, false)
	openDiff(b, "/a/d/?diff")
	b.eval(`(() => {
		for (const sheet of document.styleSheets) {
			for (let i = sheet.cssRules.length - 1; i >= 0; i--) {
				if (sheet.cssRules[i] instanceof CSSSupportsRule) sheet.deleteRule(i);
			}
		}
		document.getElementById('diff-from').focus();
	})()`)
	if got := b.eval("getComputedStyle(document.getElementById('diff-from')).appearance + ' ' + getComputedStyle(document.getElementById('diff-from')).height + ' ' + document.getElementById('diff-from').offsetWidth"); strings.HasPrefix(got, "base-select") || !strings.Contains(got, " 32px ") {
		t.Errorf("without base-select: %s", got)
	}
	b.still()
	if low := b.eval(measureContrast); low != "" {
		t.Errorf("without base-select: text that cannot be read:\n%s", low)
	}
	b.pressKey(kb.ArrowDown)
	b.until("from v1 at once", "location.search === '?diff&from=1&to=3' && diffAnswer.from.n === 1")
}

// TestUIDiffClose closes from the button: within the window at every width, what is on top where it is pressed, and drawn as
// a button of the bar. What has nothing to compare has its own way to the page.
func TestUIDiffClose(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := diffSite(t)
	b := s.b
	const place = `(() => {
		const r = document.getElementById('diff-close').getBoundingClientRect();
		const top = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2);
		const page = document.scrollingElement;
		return [r.width > 0, r.left >= 0 && r.right <= innerWidth, top?.id, page.scrollWidth <= innerWidth].join(' ');
	})()`
	for _, width := range []int64{1440, 1300, 1250, 1200, 1150, 1100, 1050, 1000, 800, 400} {
		b.size(width, 800, false)
		openDiff(b, "/a/d/?diff")
		if got := b.eval(place); got != "true true diff-close true" {
			t.Errorf("at %d: %s", width, got)
		}
	}
	// A link drawn as the bar's buttons are, not as a link: as a button of the bar put beside it.
	const like = `(() => {
		const close = document.getElementById('diff-close');
		const other = document.createElement('button');
		other.className = 'btn';
		other.textContent = 'x';
		close.before(other);
		const a = getComputedStyle(close), v = getComputedStyle(other);
		const got = a.textDecorationLine + ' ' + (a.color === v.color && a.height === v.height && a.borderRadius === v.borderRadius && a.backgroundColor === v.backgroundColor);
		other.remove();
		return got;
	})()`
	if got := b.eval(like); got != "none true" {
		t.Errorf("not like a button of the bar: %s", got)
	}
	b.moving("document.getElementById('diff-close').click()")
	b.until("the page", "location.pathname + location.search === '/a/d/' && document.getElementById('frame').hasAttribute('src')")

	b.size(1200, 800, false)
	b.navigate("/a/one/?diff")
	b.until("nothing to compare", "!document.getElementById('diff-none').hidden")
	b.moving("document.getElementById('diff-page').click()")
	b.until("its page", "location.pathname + location.search === '/a/one/' && document.getElementById('frame').hasAttribute('src')")
}

// TestUIDiffContents has a section the same as words only, at the place and level of the others: nothing to press, nor to stop at with
// Tab. A section changed is pressed and moves the page's frame to its first place.
func TestUIDiffContents(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	for _, last := range []string{"old", "new"} {
		s.floe(t, "publish", s.page(t, "diff/c.yaml", "C", "<h2>Top</h2><p>top</p><h3>Same</h3><p>same</p><h3>Early</h3>"+strings.Repeat("<p>"+last+"</p>", 30)+"<h3>Moved</h3><p>"+last+"</p>"), "--slug", "c")
	}
	b.size(1440, 300, false)
	openDiff(b, "/a/c/?diff")
	const toc = `[...document.querySelectorAll('#diff-toc > div')].map((x) => x.firstChild.localName + ':' + x.firstChild.firstChild.textContent).join(' ')`
	const stops = `[...document.querySelectorAll('#diff-toc *')].filter((x) => x.tabIndex >= 0 || x.onclick).map((x) => x.firstChild.textContent).join(' ')`
	if got := b.eval(toc) + "|" + b.eval(stops); got != "div:C div:Top div:Same button:Early button:Moved|Early Moved" {
		t.Errorf("the contents: %s", got)
	}
	left := func(name string) string {
		return "[...document.querySelectorAll('#diff-toc span')].find((x) => x.textContent === " + js(name) + ").getBoundingClientRect().left"
	}
	if got := b.eval(left("Same") + " === " + left("Moved") + " && " + left("Top") + " < " + left("Same")); got != "true" {
		t.Errorf("the levels are not kept: %s", b.eval(left("Top")+" + ' ' + "+left("Same")+" + ' ' + "+left("Moved")))
	}

	const at = "new URL(document.getElementById('diff-frame').src).hash + ' ' + document.querySelectorAll('#diff-toc .on').length"
	b.eval("[...document.querySelectorAll('#diff-toc div.dtoc-item')].at(-1).click()")
	if got := b.eval(at); got != " 0" {
		t.Errorf("a section the same was pressed: %s", got)
	}
	b.eval("[...document.querySelectorAll('#diff-toc button')].at(-1).click()")
	if got := b.eval("new URL(document.getElementById('diff-frame').src).hash + ' ' + document.querySelector('#diff-toc .on > span').textContent"); got != "#floe-diff.1 Moved" {
		t.Errorf("a section changed was pressed: %s", got)
	}
	b.still()
	if low := b.eval(measureContrast); low != "" {
		t.Errorf("text that cannot be read:\n%s", low)
	}
}

// TestUIDiffToc gives the contents the tree's edge and buttons: the width dragged or moved by keys and closed are kept, without the tree's,
// and each is reached by Tab. Closed, the button to open them stands clear of the words. Where there are no contents, there is neither.
func TestUIDiffToc(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := diffSite(t)
	b := s.b
	b.size(1440, 900, false)
	b.navigate("/")
	b.eval("localStorage.removeItem('diff.toc'); localStorage.removeItem('frame.side')")
	openDiff(b, "/a/d/?diff")

	const grip = "document.getElementById('diff-toc-grip')"
	const state = "[" + grip + ".getAttribute('aria-valuenow'), " + grip + ".getAttribute('aria-valuemin'), " + grip + ".getAttribute('aria-valuemax'), document.querySelector('.dtoc').offsetWidth, Math.round(" + grip + ".getBoundingClientRect().left + 3), Math.round(document.getElementById('diff-body').getBoundingClientRect().left)].join(' ')"
	const kept = "localStorage.getItem('diff.toc')"
	out := make([]string, 0, 20)
	out = append(out, b.eval(state), b.eval(kept+" ?? 'none kept'"))

	drag := func(to float64) {
		t.Helper()
		r := b.rect(grip)
		x, y := r.X+r.Width/2, r.Y+r.Height/2
		b.mouse(chromedp.MousePressed, x, y, 1)
		b.mouse(chromedp.MouseMoved, to, y, 0)
		b.mouse(chromedp.MouseReleased, to, y, 1)
	}
	// Dragged past its bounds it stops at them, the edge in the column it moves and under the head.
	drag(1000)
	out = append(out, b.until("at most", state+".startsWith('480') && "+state), b.eval(kept), b.eval(grip+".getBoundingClientRect().top === document.getElementById('diff-main').getBoundingClientRect().top"))
	drag(300)
	out = append(out, b.until("dragged", state+".startsWith('300') && "+state), b.eval(kept))
	b.eval(grip + ".focus()")
	b.pressKey(kb.ArrowLeft)
	out = append(out, b.eval(state), b.eval(kept))
	// As read again, and its edge stays within the column it moves.
	openDiff(b, "/a/d/?diff")
	out = append(out, b.eval(state))

	// Closed and opened, the focus on the button that takes its place, the words where they were left; the tree's own is not touched.
	b.eval("document.getElementById('diff-toc-hide').click()")
	const closed = `[document.getElementById('diff-toc-nav').hidden, document.getElementById('diff-toc-grip').hidden, document.getElementById('diff-toc-show').hidden, document.activeElement.id,
		parseFloat(getComputedStyle(document.getElementById('diff-body')).paddingBottom) === 44,
		document.getElementById('diff-toc-show').getBoundingClientRect().top > document.getElementById('diff-main').getBoundingClientRect().top,
		document.elementFromPoint(document.getElementById('diff-toc-show').getBoundingClientRect().left + 8, document.getElementById('diff-toc-show').getBoundingClientRect().top + 8)?.closest('button')?.id].join(' ')`
	out = append(out, b.eval(closed), b.eval(kept), b.eval("localStorage.getItem('frame.side') ?? 'none kept'"))
	openDiff(b, "/a/d/?diff")
	out = append(out, b.eval(closed))
	b.pressKey("n")
	b.eval("document.getElementById('diff-toc-show').click()")
	out = append(out, b.eval("[document.getElementById('diff-toc-nav').hidden, document.activeElement.id].join(' ')"), b.eval(kept))

	for _, id := range []string{"diff-toc-hide", "diff-toc-grip"} {
		b.eval("document.activeElement.blur()")
		for range 40 {
			b.pressKey(kb.Tab)
			if b.eval("document.activeElement.id") == id {
				break
			}
		}
		out = append(out, id+": "+b.eval("document.activeElement.id + ' ' + getComputedStyle(document.activeElement).outlineStyle"))
	}
	b.eval("document.activeElement.blur(); document.getElementById('diff-toc-hide').click()")
	b.eval("document.activeElement.blur()")
	for range 40 {
		b.pressKey(kb.Tab)
		if b.eval("document.activeElement.id") == "diff-toc-show" {
			break
		}
	}
	out = append(out, b.eval("document.activeElement.id"))
	b.still()
	if low := b.eval(measureContrast); low != "" {
		t.Errorf("closed: text that cannot be read:\n%s", low)
	}
	b.eval("document.getElementById('diff-toc-show').click()")

	// The window narrows it for now, the width kept as it was; with no contents at that width, neither the edge nor the buttons.
	b.size(800, 700, false)
	b.until("narrowed", grip+".getAttribute('aria-valuemax') === '400'")
	out = append(out, b.eval(kept))
	for _, width := range []int64{800, 400} {
		b.size(width, 700, false)
		for _, open := range []string{"true", "false"} {
			b.eval("localStorage.setItem('diff.toc', '{\"width\":284,\"open\":" + open + "}')")
			openDiff(b, "/a/d/?diff")
			if got := b.eval("['.dtoc', '#diff-toc-grip', '#diff-toc-show'].map((q) => document.querySelector(q).getClientRects().length).join(' ') + ' ' + document.getElementById('diff-body').getBoundingClientRect().left"); got != "0 0 0 0" {
				t.Errorf("at %d, open %s: %s", width, open, got)
			}
		}
	}
	check(t, "toc", out, strings.Join([]string{
		"240 160 480 240 240 240", "none kept",
		"480 160 480 480 480 480", `{"width":480,"open":true}`, "true",
		"300 160 480 300 300 300", `{"width":300,"open":true}`,
		"284 160 480 284 284 284", `{"width":284,"open":true}`,
		"284 160 480 284 284 284",
		"true true false diff-toc-show true true diff-toc-show", `{"width":284,"open":false}`, "none kept",
		"true true false  true true diff-toc-show",
		"false diff-toc-hide", `{"width":284,"open":true}`,
		"diff-toc-hide: diff-toc-hide solid", "diff-toc-grip: diff-toc-grip solid",
		"diff-toc-show",
		`{"width":284,"open":true}`,
	}, "|"))
}

// TestUIDiffRead keeps what is shown of one pair: an answer that fails, or has not come, leaves the page's frame with the versions it is
// of, an error goes with the next answer, and the last of many choices alone is asked to the end, nothing said of the server.
func TestUIDiffRead(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := diffSite(t)
	b := s.b
	b.size(1440, 900, false)
	openDiff(b, "/a/d/?diff")
	b.eval(`(() => { const real = window.fetch, told = setServer;
		setServer = (up) => { if (!up) window.saidDown = true; told(up); };
		window.diffCalls = [];
		window.fetch = async (u, i) => {
			if (!String(u).includes('/diff?')) return real(u, i);
			window.diffCalls.push(i?.signal);
			if (window.diffRefuse) return new Response('no', { status: 500 });
			if (window.diffHeld) await window.diffHeld;
			return real(u, i);
		};
	})()`)
	// The versions of the page in the frame
	const heads = "(() => { const u = new URL(document.getElementById('diff-frame').src); return 'v' + u.searchParams.get('from') + ' v' + u.searchParams.get('v'); })()"
	choose := func(name, value string) {
		t.Helper()
		b.eval("(() => { const e = document.getElementById('diff-" + name + "'); e.value = " + js(value) + "; e.dispatchEvent(new Event('change')); })()")
	}
	const failure = "document.getElementById('error')"
	// It fails: back to the pair shown, in the list and the URL, its words said.
	b.eval("window.diffRefuse = true")
	choose("from", "1")
	b.until("the error", "!"+failure+".hidden")
	out := make([]string, 0, 8)
	out = append(out, b.eval(heads), b.eval(diffPair), b.eval("location.search"))
	// The next answer takes the error away.
	b.eval("window.diffRefuse = false")
	choose("from", "1")
	b.until("the error gone", failure+".hidden && diffAnswer.from.n === 1")
	out = append(out, b.eval(heads))
	// While an answer is on its way, what is drawn is what was answered.
	b.eval("window.diffHeld = new Promise((go) => { window.diffGo = go; }); 1")
	choose("from", "2")
	out = append(out, b.eval(heads))
	// Chosen again meanwhile, the one before is called off, the server not told down.
	b.eval("window.diffCalls = []")
	choose("to", "2")
	choose("to", "3")
	out = append(out, b.eval("window.diffCalls.length + ' ' + window.diffCalls[0].aborted + ' ' + window.diffCalls[1].aborted"))
	b.eval("window.diffGo(); window.diffHeld = null; 1")
	b.until("the last answer", "diffAnswer.from.n === 2 && diffAnswer.to.n === 3")
	out = append(out, b.eval(heads), b.eval("String(window.saidDown === undefined)"))
	check(t, "read", out, strings.Join([]string{"v2 v3", "2 3", "?diff&from=2&to=3", "v1 v3", "v1 v3", "2 true false", "v2 v3", "true"}, "|"))
}

// TestUIDiffPlaces: a missing page's frame shows no diff, the contents move the page's frame to a section's place, a redraw at a phone's
// width goes back to the top of what scrolls there, and ? tells d only where it works. Printed, nothing of it shows.
func TestUIDiffPlaces(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := diffSite(t)
	b := s.b
	b.size(1440, 900, false)
	b.navigate("/a/none/?diff")
	b.until("not found", "!document.getElementById('notfound').hidden")
	out := make([]string, 0, 12)
	out = append(out, b.eval("String(document.getElementById('diff').hidden)"))

	// The contents, in a window too short for all of it.
	b.size(1440, 420, false)
	openDiff(b, "/a/d/?diff")
	b.eval("[...document.querySelectorAll('#diff-toc button')].at(-1).click()")
	out = append(out, b.eval(`(() => {
		const i = [...document.querySelectorAll('#diff-toc button')].at(-1).dataset.section;
		return String(new URL(document.getElementById('diff-frame').src).hash === '#floe-diff.' + diffAnswer.sections[i].place);
	})()`))

	// The keys told.
	keys := func() string {
		t.Helper()
		b.eval("openKeys()")
		got := b.eval("[...document.querySelectorAll('.keys-part:not([hidden]) kbd')].filter((k) => k.offsetParent && k.textContent === 'd').map((k) => k.parentElement.nextElementSibling.textContent).join('|')")
		b.eval("document.getElementById('keys').close()")
		return got
	}
	out = append(out, keys())
	b.navigate("/a/d/")
	b.until("the bar", "!document.querySelector('#frame-actions .diff-open').hidden")
	out = append(out, keys())
	b.navigate("/a/one/")
	b.until("the bar of one", "document.getElementById('frame').hasAttribute('src')")
	out = append(out, keys())
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=one]')")
	out = append(out, keys())
	b.eval("document.querySelector('#list li[data-slug=d] .title').click()")
	b.until("picked", "picked?.slug === 'd'")
	out = append(out, keys())

	// Printed: neither the diff nor the words over a page.
	openDiff(b, "/a/d/?diff")
	b.media("print")
	out = append(out, b.eval("getComputedStyle(document.getElementById('diff')).display"))
	b.media("")

	// At a phone's width the whole of it scrolls, and is at its top after a redraw.
	b.size(400, 500, false)
	openDiff(b, "/a/d/?diff")
	out = append(out, b.eval(`(() => {
		const box = document.getElementById('diff');
		box.scrollTop = 200;
		const moved = box.scrollTop > 0;
		drawDiff();
		return moved + ' ' + box.scrollTop + ' ' + getComputedStyle(box).display;
	})()`))
	check(t, "places", out, strings.Join([]string{
		"true", "true", "Close the diff", "What changed since the version read", "", "", "What changed since the version read", "none", "true 0 block",
	}, "|"))
}

// TestUIDiffScrolls keeps the scrolling to one box: the page itself never scrolls, whatever is done, and the page's frame ends with the
// box, no ground left under it. Words only a screen reader reads, placed absolutely, once widened the page far past them and kept it there.
func TestUIDiffScrolls(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	var kept, mended strings.Builder
	for i := range 60 {
		fmt.Fprintf(&kept, "<h2>S%d</h2><p>Words of the section %d, kept as they are in both.</p>", i, i)
		word := "are"
		if i%7 == 0 {
			word = "were"
		}
		fmt.Fprintf(&mended, "<h2>S%d</h2><p>Words of the section %d, kept as they %s in both.</p>", i, i, word)
	}
	s.floe(t, "publish", s.page(t, "z.yaml", "Z", kept.String()), "--slug", "z")
	s.floe(t, "publish", s.page(t, "z.yaml", "Z", kept.String()+"<p>one more</p>"), "--slug", "z")
	s.floe(t, "publish", s.page(t, "z.yaml", "Z", mended.String()), "--slug", "z")
	// The page's scrolling, as it can be pushed there too; and of the box, what is left under its last words at its end, past the
	// room kept for what opens the contents, folded.
	const measure = `(() => {
		const page = document.scrollingElement;
		page.scrollTop = 1e6;
		const moved = page.scrollTop;
		const narrow = matchMedia('(max-width: 40rem)').matches;
		const box = document.getElementById(narrow ? 'diff' : 'diff-body');
		box.scrollTop = 1e6;
		const last = document.getElementById('diff-body').lastElementChild.getBoundingClientRect().bottom;
		const left = Math.round(box.getBoundingClientRect().bottom - last - parseFloat(getComputedStyle(document.getElementById('diff-body')).paddingBottom));
		const r = [moved, page.scrollHeight - page.clientHeight, left > 40 || left < -1 ? 'left ' + left : 'ends'].join(' ');
		page.scrollTop = 0;
		return r;
	})()`
	const fine = "0 0 ends"
	open := func() {
		t.Helper()
		openDiff(b, "/a/z/?diff")
	}
	b.navigate("/")
	for _, kept := range []string{"true", "false"} {
		b.eval("localStorage.setItem('diff.toc', '{\"open\":" + kept + "}')")
		for _, size := range [][2]int64{{1440, 900}, {1100, 700}, {800, 700}, {400, 800}} {
			b.size(size[0], size[1], false)
			open()
			if got := b.eval(measure); got != fine {
				t.Errorf("%dx%d, contents shown %s: %s", size[0], size[1], kept, got)
			}
		}
	}
	b.eval("localStorage.removeItem('diff.toc')")
	// What moves the box, and what shortens what it scrolls, leave the page where it is.
	b.size(1440, 900, false)
	open()
	after := func(what, do string) {
		t.Helper()
		b.eval(do + "; 1")
		if got := b.eval("document.scrollingElement.scrollTop + ' ' + (document.scrollingElement.scrollHeight - document.scrollingElement.clientHeight) + ' ' + document.body.scrollTop + ' ' + document.getElementById('diff').scrollTop + ' ' + document.getElementById('diff-main').scrollTop"); got != "0 0 0 0 0" {
			t.Errorf("%s: %s", what, got)
		}
	}
	after("n", "moveChange(1); moveChange(1); moveChange(1)")
	after("p", "moveChange(-1)")
	after("the middle of the contents", "[...document.querySelectorAll('#diff-toc button')].find((x) => x.firstChild.textContent === 'S7').click()")
	after("the contents", "[...document.querySelectorAll('#diff-toc button')].at(-1).click()")
	b.eval("(() => { const e = document.getElementById('diff-from'); e.value = '2'; e.dispatchEvent(new Event('change')); })()")
	b.until("the other pair", "diffAnswer.from.n === 2")
	after("chosen again", "1")
	b.size(400, 800, false)
	after("narrowed", "1")
	// At that width all of it scrolls, and is at its top after a redraw.
	if got := b.eval(`(() => {
		const box = document.getElementById('diff');
		box.scrollTop = 1e6;
		const moved = box.scrollTop > 0;
		drawDiff();
		return moved + ' ' + box.scrollTop;
	})()`); got != "true 0" {
		t.Errorf("redrawn at a phone's width: %s", got)
	}
	b.size(1440, 900, false)
	after("widened", "1")
	after("the list read again", "drawn = ''; load(true)")
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
	b.still()
	if low := b.eval(measureContrast); low != "" {
		t.Errorf("the words over a page: text that cannot be read:\n%s", low)
	}
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
