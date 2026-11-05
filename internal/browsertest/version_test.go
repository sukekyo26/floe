package browsertest_test

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/chromedp/chromedp/kb"
)

// What the frame's choice and words say, the date in them left out as it changes.
const (
	noteWords = `(document.querySelector('.dnote.ver-note')?.textContent ?? '').replace(/\(.*?\)/, '(at)')`
	frameSaid = `[location.search + location.hash, document.getElementById('frame').getAttribute('src'), document.getElementById('frame-version').hidden, ` + noteWords + `].join(' / ')`
	readSaid  = `[readLoaded, document.getElementById('read-version').hidden, document.getElementById('read-version').classList.contains('old'), ` + noteWords + `].join(' / ')`
	newsOfD   = "(items.find((a) => a.slug === 'd').news ?? '')"
)

// choose picks the version n in the select of id, as a press on its item does.
func choose(b *browser, id string, n int) {
	b.t.Helper()
	b.eval("(() => { const s = document.getElementById(" + js(id) + "); s.value = " + strconv.Itoa(n) + "; s.dispatchEvent(new Event('change', { bubbles: true })); })()")
}

// TestUIVersionPick chooses a version of a page beside the list: it stays through the list drawn anew and a version published, the latest
// not told read; the bar and the row's ways to open the page whole go with it; another page, and the latest chosen, leave it.
func TestUIVersionPick(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := diffSite(t)
	b := s.b
	b.size(1440, 900, false)
	defer b.unsize()
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=d]')")
	b.eval("document.querySelector('#list li[data-slug=d] .title').click()")
	b.until("the choice", "!document.getElementById('read-version').hidden")
	b.until("read", "!("+newsOfD+")")
	out := make([]string, 0, 30)
	const toast = "document.querySelectorAll('.toast.page').length"
	b.eval("document.querySelector('.toast.page')?.remove()")
	out = append(out, b.eval("document.querySelector('#read-version selectedcontent').innerText.trim()"),
		b.eval(`[...document.querySelectorAll('#read-version option')].map((o) => o.textContent.replace(/ · \d\d\/\d\d \d\d:\d\d/, '')).join(', ')`))

	choose(b, "read-version", 2)
	out = append(out, b.until("v2 shown", "document.querySelector('.dnote.ver-note') && "+readSaid),
		b.eval("document.querySelector('#read-version selectedcontent').innerText.trim()"),
		b.eval("document.activeElement.tagName"),
		b.eval(`[document.getElementById('read-title').getAttribute('href'), document.querySelector('#read-actions > .open').getAttribute('href'),
			document.querySelector('#read-actions > .download').getAttribute('href'), document.querySelector('#read-actions > .download').hasAttribute('download'),
			document.querySelector('#read-actions > .diff-open').getAttribute('href'), document.querySelector('#read-actions .menu .open').getAttribute('href')].join(' ')`))
	// Drawn anew, it stays; a version published, too, the words following, the row unread, no toast.
	b.eval("drawn = ''; load()")
	b.still()
	out = append(out, b.eval("readLoaded"))
	s.floe(t, "publish", s.page(t, "diff/d.yaml", "D", diffV3+"<p>four</p>"), "--slug", "d")
	out = append(out, b.until("v4 told", noteWords+".includes('v4') && "+readSaid), b.eval(newsOfD), b.eval(toast))

	// Pressed again and by Enter, the row opens the version shown; its link stays the latest's.
	out = append(out, b.eval("document.querySelector('#list li[data-slug=d] .title').getAttribute('href')"))
	x, y := rowPoint(t, b, "#list li[data-slug=d]", false)
	b.click(x, y)
	out = append(out, b.until("whole", "location.pathname === '/a/d/' && location.search"))
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=d]')")
	b.eval("document.querySelector('#list li[data-slug=d] .title').click()")
	b.until("the choice", "!document.getElementById('read-version').hidden")
	choose(b, "read-version", 2)
	b.eval("document.querySelector('#list li[data-slug=d] .title').focus()")
	b.pressKey(kb.Enter)
	out = append(out, b.until("whole by the key", "location.pathname === '/a/d/' && location.search"))

	// The keys go on from the choice.
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=d]')")
	b.eval("document.querySelector('#list li[data-slug=d] .title').click()")
	b.until("the choice", "!document.getElementById('read-version').hidden")
	b.until("read", "!("+newsOfD+")")
	b.eval("document.querySelector('.toast.page')?.remove()")
	choose(b, "read-version", 2)
	b.until("v2", "readLoaded.endsWith('v=2')")
	b.eval("document.getElementById('read-version').focus()")
	b.pressKey("j")
	out = append(out, b.until("the next row", "picked.slug !== 'd' && [picked.v, document.querySelector('.dnote.ver-note') === null].join(' ')"))
	b.pressKey("k")
	out = append(out, b.until("back", "picked.slug === 'd' && readLoaded"))
	b.until("read", "!("+newsOfD+")")

	// Marked unread: the versions gone through tell nothing, until another page has been open.
	b.eval(`window.reads = 0; window.fetch = ((f) => (u, i) => { if (i?.method === 'POST' && String(u).endsWith('/read')) window.reads++; return f(u, i); })(window.fetch);
		window.heard = 0; addEventListener('message', (e) => { if (e.data?.artifactRead) window.heard++; }); markUnread('d')`)
	b.until("unread", newsOfD)
	const shelfWord = "[...document.querySelectorAll('#read-version option')].find((o) => o.textContent.includes('v2')).textContent.includes('·')"
	out = append(out, b.eval(shelfWord))
	for _, n := range []int{2, 4} {
		was := b.eval("window.heard")
		choose(b, "read-version", n)
		b.until("told", "window.heard > "+was)
	}
	out = append(out, b.eval("window.reads"), b.eval("Boolean("+newsOfD+")"))
	b.pressKey("j")
	b.until("the next row", "picked.slug !== 'd'")
	b.pressKey("k")
	out = append(out, b.until("read again", "picked.slug === 'd' && !("+newsOfD+") && "+toast))

	// A read leaves the choice open, the focus on it, and its words told anew.
	b.eval("document.querySelector('.toast.page')?.remove(); window.kept = document.getElementById('read-version')")
	r := b.rect("document.getElementById('read-version')")
	b.click(r.X+r.Width/2, r.Y+r.Height/2)
	b.until("open", "document.getElementById('read-version').matches(':open')")
	b.eval("markUnread('d')")
	b.until("unread", newsOfD)
	b.still()
	out = append(out, b.eval("[window.kept === document.getElementById('read-version'), window.kept.contains(document.activeElement), window.kept.matches(':open')].join(' ')"))
	b.pressKey(kb.Escape)

	check(t, "pick", out, strings.Join([]string{
		"v3", "v3 · Latest, v2, v1",
		"/a/d/?raw&v=2 / false / true / Showing v2 (at). The latest is v3.Open the latest versionSee what differs from the latest",
		"v2", "BODY",
		"/a/d/?v=2 /a/d/?raw&v=2 /api/artifacts/d/download?v=2 true /a/d/?diff&from=2&to=3 /a/d/?raw&v=2",
		"/a/d/?raw&v=2",
		"/a/d/?raw&v=2 / false / true / Showing v2 (at). The latest is v4.Open the latest versionSee what differs from the latest", "version", "0",
		"/a/d/",
		"?v=2", "?v=2",
		"0 true", "/a/d/?raw",
		"true", "0", "true", "1",
		"true true true",
	}, "|"))
}

// TestUIVersionURL opens the frame at each ?v=: the page of the version, not told as the latest; what is none gives way to the latest,
// which it says; what is not a version's stays. Chosen in the frame, a version is a move that Back undoes, the focus left on the choice.
func TestUIVersionURL(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := diffSite(t)
	b := s.b
	s.write(t, "docs/index.html", "<!doctype html><title>t</title><p>page</p>")
	s.floe(t, "publish", "--link", filepath.Join(s.Env["HOME"], "docs"), "--slug", "docs")
	b.size(1440, 900, false)
	defer b.unsize()
	open := func(path string) string {
		t.Helper()

		b.navigate(path)
		b.until("the bar", "!document.getElementById('frame-actions').hidden && document.querySelector('#frame-actions .diff-open')")
		b.still()
		return b.eval(frameSaid)
	}
	out := make([]string, 0, 24)
	// A page never read: at a version, that version is told read, and the latest is not.
	out = append(out, open("/a/d/?v=2"))
	out = append(out, b.until("v2 told read", newsOfD+" === 'version' && "+newsOfD), b.eval("document.querySelectorAll('.toast').length"), b.eval("document.getElementById('print-raw').getAttribute('href').replace(location.origin, '')"))
	for _, path := range []string{
		"/a/d/?v=3", "/a/d/?v=99", "/a/d/?v=abc", "/a/d/?v=01", "/a/d/?v=", "/a/d/?v=0", "/a/one/?v=1", "/a/one/?v=2", "/a/d/?v=2#part",
		"/a/l/?v=3", "/a/docs/?file=index.html&v=3",
	} {
		got := open(path)
		out = append(out, path+": "+got)
	}
	b.navigate("/a/d/?diff&v=2")
	b.until("the diff", "document.querySelector('#diff-body > *')")
	out = append(out, b.eval(frameSaid))

	// Chosen: a move that Back undoes, the focus on the choice again.
	open("/a/d/?v=2")
	b.eval("document.getElementById('frame-version').focus()")
	choose(b, "frame-version", 1)
	b.until("v1", "location.search === '?v=1' && !document.getElementById('frame-version').hidden && document.activeElement.id === 'frame-version'")
	out = append(out, b.eval(frameSaid))
	b.eval("history.back()")
	b.until("back", "location.search === '?v=2' && !document.getElementById('frame-version').hidden")
	out = append(out, b.eval(frameSaid))
	// Yellow, its words read and it takes the ring.
	b.still()
	if low := b.eval(measureContrast); low != "" {
		t.Errorf("an earlier version: text that cannot be read:\n%s", low)
	}
	b.eval("document.activeElement.blur(); document.getElementById('frame-title').tabIndex = 0; document.getElementById('frame-title').focus()")
	b.pressKey(kb.Tab)
	out = append(out, b.eval("document.activeElement.id")+": "+b.settled(ringOf))
	b.eval("document.getElementById('frame-title').removeAttribute('tabindex')")
	// The words' way to the latest; marked unread, moving among the versions tells nothing.
	b.eval(`markUnread('d')`)
	b.until("unread", "items.find((a) => a.slug === 'd').news")
	choose(b, "frame-version", 1)
	b.until("v1", "location.search === '?v=1' && document.querySelector('.dnote.ver-note')")
	b.eval(`window.reads = 0; window.fetch = ((f) => (u, i) => { if (i?.method === 'POST' && String(u).endsWith('/read')) window.reads++; return f(u, i); })(window.fetch)`)
	b.eval("[...document.querySelectorAll('.dnote.ver-note a')].find((a) => a.textContent.includes('latest')).click()")
	b.until("the latest", "location.search === '' && !document.getElementById('frame-version').hidden")
	b.still()
	out = append(out, b.eval("Boolean("+newsOfD+")"), b.eval("document.querySelectorAll('.toast').length"))

	// A slug changed keeps the version.
	open("/a/d/?v=2")
	b.eval("document.querySelector('#frame-actions .edit').click(); document.getElementById('edit-slug').value = 'd3'; document.getElementById('edit').requestSubmit()")
	out = append(out, b.until("the new name", "location.pathname !== '/a/d/' && location.pathname + location.search"))

	check(t, "url", out, strings.Join([]string{
		"?v=2 / /a/d/?raw&v=2 / false / Showing v2 (at). The latest is v3.Open the latest versionSee what differs from the latest",
		"version", "0", "/a/d/?raw&v=2",
		"/a/d/?v=3:  / /a/d/?raw / false / ",
		"/a/d/?v=99:  / /a/d/?raw / false / That version cannot be opened, so this shows the latest.",
		"/a/d/?v=abc:  / /a/d/?raw / false / That version cannot be opened, so this shows the latest.",
		"/a/d/?v=01:  / /a/d/?raw / false / That version cannot be opened, so this shows the latest.",
		"/a/d/?v=:  / /a/d/?raw / false / That version cannot be opened, so this shows the latest.",
		"/a/d/?v=0:  / /a/d/?raw / false / That version cannot be opened, so this shows the latest.",
		"/a/one/?v=1:  / /a/one/?raw / true / ",
		"/a/one/?v=2:  / /a/one/?raw / true / That version cannot be opened, so this shows the latest.",
		"/a/d/?v=2#part: ?v=2#part / /a/d/?raw&v=2#part / false / Showing v2 (at). The latest is v3.Open the latest versionSee what differs from the latest",
		"/a/l/?v=3: ?v=3 / /a/l/?raw / true / ",
		"/a/docs/?file=index.html&v=3: ?file=index.html&v=3 / /a/docs/index.html / true / ",
		"?diff&v=2 /  / true / ",
		"?v=1 / /a/d/?raw&v=1 / false / Showing v1 (at). The latest is v3.Open the latest versionSee what differs from the latest",
		"?v=2 / /a/d/?raw&v=2 / false / Showing v2 (at). The latest is v3.Open the latest versionSee what differs from the latest",
		"frame-version: solid 2px rgb(107, 105, 100) 1px seen",
		"true", "0",
		"/a/d3/?v=2",
	}, "|"))
}

// TestUIVersionGone: the version shown goes, by what is done elsewhere: the latest takes its place with no one told it was read, in the
// frame and beside the list, with the choice or, as one version is left, without; the page read as ever once another has been open.
func TestUIVersionGone(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := diffSite(t)
	b := s.b
	b.size(1440, 900, false)
	defer b.unsize()
	const gone = "That version cannot be opened, so this shows the latest."
	out := make([]string, 0, 12)

	// One published only: nothing goes, the words follow.
	b.navigate("/a/d/?v=2")
	b.until("the choice", "!document.getElementById('frame-version').hidden")
	s.floe(t, "publish", s.page(t, "diff/d.yaml", "D", diffV3+"<p>four</p>"), "--slug", "d")
	out = append(out, b.until("v4 told", noteWords+".includes('v4') && "+frameSaid))

	// Pushed out by the versions after it, in the frame.
	for i := range 20 {
		s.floe(t, "publish", s.page(t, "diff/d.yaml", "D", diffV3+"<p>many "+strconv.Itoa(i)+"</p>"), "--slug", "d")
	}
	out = append(out, b.until("the latest", "location.search === '' && "+noteWords+" === "+js(gone)+" && "+frameSaid),
		b.eval("document.getElementById('print-raw').getAttribute('href').replace(location.origin, '')"), b.eval("document.querySelectorAll('.toast').length"))

	// One version left: no choice; of a page never read since, unread, marked as such.
	s.floe(t, "publish", s.page(t, "diff/e.yaml", "E", "<p>one</p>"), "--slug", "e")
	s.floe(t, "publish", s.page(t, "diff/e.yaml", "E", "<p>two</p>"), "--slug", "e")
	s.floe(t, "publish", s.page(t, "diff/e.yaml", "E", "<p>three</p>"), "--slug", "e")
	b.navigate("/a/e/?v=2")
	b.until("the choice", "!document.getElementById('frame-version').hidden")
	s.floe(t, "rm", "e")
	s.floe(t, "publish", s.page(t, "diff/e.yaml", "E", "<p>again</p>"), "--slug", "e")
	out = append(out, b.until("the latest", noteWords+" === "+js(gone)+" && "+frameSaid))
	b.still()
	out = append(out, b.eval("items.find((a) => a.slug === 'e').news ?? ''"), b.eval("document.querySelectorAll('.toast').length"))

	// Beside the list, with the list's look held back across the removal, whose answer in between would let the row go.
	s.floe(t, "publish", s.page(t, "diff/f.yaml", "F", "<p>one</p>"), "--slug", "f")
	s.floe(t, "publish", s.page(t, "diff/f.yaml", "F", "<p>two</p>"), "--slug", "f")
	s.floe(t, "publish", s.page(t, "diff/f.yaml", "F", "<p>three</p>"), "--slug", "f")
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=f]')")
	b.eval("document.querySelector('#list li[data-slug=f] .title').click()")
	b.until("the choice", "!document.getElementById('read-version').hidden && !items.find((a) => a.slug === 'f').news")
	choose(b, "read-version", 2)
	b.until("v2", "readLoaded.endsWith('v=2')")
	b.eval("window.look = load; load = () => {}")
	s.floe(t, "rm", "f")
	s.floe(t, "publish", s.page(t, "diff/f.yaml", "F", "<p>again</p>"), "--slug", "f")
	b.eval("load = window.look; document.querySelector('.toast.page')?.remove(); load()")
	out = append(out, b.until("the latest", noteWords+" === "+js(gone)+" && "+readSaid))
	b.still()
	out = append(out, b.eval("items.find((a) => a.slug === 'f').news ?? ''"), b.eval("document.querySelectorAll('.toast.page').length"))
	// Another page, and back: read, the words gone.
	b.eval("document.querySelector('#list li[data-slug=d] .title').click()")
	out = append(out, b.until("another", "picked.slug === 'd' && !document.querySelector('.dnote.ver-note') && 'no words'"))
	b.eval("document.querySelector('#list li[data-slug=f] .title').click()")
	out = append(out, b.until("read", "picked.slug === 'f' && !items.find((a) => a.slug === 'f').news && 'read'"))

	check(t, "gone", out, strings.Join([]string{
		"?v=2 / /a/d/?raw&v=2 / false / Showing v2 (at). The latest is v4.Open the latest versionSee what differs from the latest",
		" / /a/d/?raw / false / " + gone, "/a/d/?raw", "0",
		" / /a/e/?raw / true / " + gone, "new", "0",
		"/a/f/?raw / true / false / " + gone, "new", "0",
		"no words", "read",
	}, "|"))
}

// TestUIVersionNotes: at a version gone, the words of it stay over those of a page newly read past the version last read, each going on its own turns.
func TestUIVersionNotes(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := diffSite(t)
	b := s.b
	b.size(1440, 900, false)
	defer b.unsize()
	b.navigate("/a/d/?v=1")
	b.until("v1 told read", "items.find((a) => a.slug === 'd')?.news === 'version'")
	b.navigate("/a/d/?v=99")
	b.until("both", "document.querySelector('.dnote.ver-note') && document.querySelector('.dnote.page-note')")
	order := b.eval(`[...document.querySelectorAll('.dnote')].filter((e) => e.getClientRects().length).map((e) => e.classList.contains('ver-note') ? 'version' : 'page').join(' ')`)
	check(t, "notes", []string{order, b.eval(noteWords)}, "version page|That version cannot be opened, so this shows the latest.")
}
