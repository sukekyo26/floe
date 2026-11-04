package browsertest_test

import (
	"fmt"
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

// TestUIDiffRead keeps what is shown of one pair: an answer that fails, or has not come, leaves the columns' heads with the words they are
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
	const heads = "[...document.querySelectorAll('.dcols > div')].map((x) => x.textContent.split(' ')[0]).join(' ')"
	choose := func(name, value string) {
		t.Helper()
		b.eval("(() => { const e = document.getElementById('diff-" + name + "'); e.value = " + js(value) + "; e.dispatchEvent(new Event('change')); })()")
	}
	const error = "document.getElementById('error')"
	// It fails: back to the pair shown, in the list and the URL, its words said.
	b.eval("window.diffRefuse = true")
	choose("from", "1")
	b.until("the error", "!"+error+".hidden")
	b.eval("document.querySelector('[data-diff-view=all]').click()")
	b.eval("document.querySelector('[data-diff-view=side]').click()")
	out := make([]string, 0, 8)
	out = append(out, b.eval(heads), b.eval(diffPair), b.eval("location.search"))
	// The next answer takes the error away.
	b.eval("window.diffRefuse = false")
	choose("from", "1")
	b.until("the error gone", error+".hidden && diffAnswer.from.n === 1")
	out = append(out, b.eval(heads))
	// While an answer is on its way, what is drawn is what was answered.
	b.eval("window.diffHeld = new Promise((go) => { window.diffGo = go; }); 1")
	choose("from", "2")
	b.eval("document.querySelector('[data-diff-view=all]').click()")
	b.eval("document.querySelector('[data-diff-view=side]').click()")
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

// TestUIDiffPlaces: a missing page's frame shows no diff, the contents leave a heading under the columns' heads, a redraw at a phone's
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
	b.eval("document.querySelector('[data-diff-view=side]').click()")
	b.eval("[...document.querySelectorAll('#diff-toc button')].at(-1).click()")
	out = append(out, b.eval(`(() => {
		const go = document.querySelector('#diff-body [data-sections~="' + [...document.querySelectorAll('#diff-toc button')].at(-1).dataset.section + '"]');
		return String(go.getBoundingClientRect().top >= document.querySelector('.dcols').getBoundingClientRect().bottom - 1);
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
		document.querySelector('[data-diff-view=only]').click();
		return moved + ' ' + box.scrollTop + ' ' + getComputedStyle(box).display;
	})()`))
	check(t, "places", out, strings.Join([]string{
		"true", "true", "Close the diff", "What changed since the version read", "", "", "What changed since the version read", "none", "true 0 block",
	}, "|"))
}

// TestUIDiffScrolls keeps the scrolling to one box: the page itself never scrolls, whatever is done, and what scrolls ends with its words,
// no ground left under them. Words only a screen reader reads, placed absolutely, once widened the page far past them and kept it there.
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
	// The page's scrolling, as it can be pushed there too; and of the box, what is left under its last words at its end.
	const measure = `(() => {
		const page = document.scrollingElement;
		page.scrollTop = 1e6;
		const moved = page.scrollTop;
		const narrow = matchMedia('(max-width: 40rem)').matches;
		const box = document.getElementById(narrow ? 'diff' : 'diff-body');
		box.scrollTop = 1e6;
		const last = document.getElementById('diff-body').lastElementChild.getBoundingClientRect().bottom;
		const left = Math.round(box.getBoundingClientRect().bottom - last);
		const r = [moved, page.scrollHeight - page.clientHeight, left > 40 || left < -1 ? 'left ' + left : 'ends'].join(' ');
		page.scrollTop = 0;
		return r;
	})()`
	const fine = "0 0 ends"
	open := func() {
		t.Helper()
		openDiff(b, "/a/z/?diff")
		b.eval("document.querySelector('[data-diff-view=side]').click()")
	}
	for _, size := range [][2]int64{{1440, 900}, {1100, 700}, {800, 700}, {400, 800}} {
		b.size(size[0], size[1], false)
		open()
		for _, view := range []string{"side", "all", "only"} {
			b.eval("document.querySelector('[data-diff-view=" + view + "]').click()")
			// Changes only may all fit, with nothing to scroll.
			if got := b.eval(measure); got != fine && !(view == "only" && strings.HasPrefix(got, "0 0 ")) {
				t.Errorf("%dx%d %s: %s", size[0], size[1], view, got)
			}
		}
	}
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
	// The contents, the words brought to the top left under the columns' heads, of a section in the middle.
	after("the middle of the contents", "[...document.querySelectorAll('#diff-toc button')].find((x) => x.firstChild.textContent === 'S7').click()")
	if got := b.eval(`(() => {
		const i = [...document.querySelectorAll('#diff-toc button')].find((x) => x.firstChild.textContent === 'S7').dataset.section;
		const go = document.querySelector('#diff-body [data-sections~="' + i + '"]').getBoundingClientRect().top;
		const heads = document.querySelector('.dcols').getBoundingClientRect().bottom;
		return go >= heads - 1 && go < heads + 40 && document.getElementById('diff-body').scrollTop > 0 ? 'true' : go + ' under ' + heads + ' at ' + document.getElementById('diff-body').scrollTop;
	})()`); got != "true" {
		t.Errorf("the heading went under the columns' heads: %s", got)
	}
	after("the contents", "[...document.querySelectorAll('#diff-toc button')].at(-1).click()")
	after("scrolled to the end, changes only", "document.getElementById('diff-body').scrollTop = 1e6; document.querySelector('[data-diff-view=only]').click()")
	after("all together", "document.querySelector('[data-diff-view=all]').click()")
	b.eval("(() => { document.getElementById('diff-body').scrollTop = 1e6; const e = document.getElementById('diff-from'); e.value = '2'; e.dispatchEvent(new Event('change')); })()")
	b.until("the other pair", "diffAnswer.from.n === 2")
	after("chosen again", "1")
	b.size(400, 800, false)
	after("narrowed", "1")
	// At that width all of it scrolls, and is at its top after a redraw.
	if got := b.eval(`(() => {
		const box = document.getElementById('diff');
		box.scrollTop = 1e6;
		const moved = box.scrollTop > 0;
		document.querySelector('[data-diff-view=side]').click();
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
