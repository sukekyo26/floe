package browsertest_test

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp/kb"

	"floe/internal/store"
)

// choose picks the version n in the select of id (chooseValue), once the list of versions has come: the choice is empty until then.
func choose(b *browser, id string, n int) {
	b.t.Helper()
	b.until("the option", "document.querySelector('#"+id+" option[value=\""+strconv.Itoa(n)+"\"]')")
	chooseValue(b, id, strconv.Itoa(n))
}

// The same, by the screen alone: the words over a page not the latest, what the frame and the page beside the list show, and the page
// beside the list (its slug, version and what it loaded).
const (
	verWords     = `(document.querySelector('[data-testid=ver-note]')?.textContent ?? '').replace(/\(.*?\)/, '(at)')`
	frameTold    = `[location.search + location.hash, document.getElementById('frame').getAttribute('src'), document.getElementById('frame-version')?.hidden ?? true, ` + verWords + `].join(' / ')`
	besideSlug   = "document.getElementById('read-frame').dataset.slug"
	besideV      = "document.getElementById('read-frame').dataset.version"
	besideLoaded = "document.getElementById('read-frame').dataset.loaded"
	readTold     = `[` + besideLoaded + `, document.getElementById('read-version').hidden, document.getElementById('read-version').dataset.old === 'true', ` + verWords + `].join(' / ')`
	pageToasts   = "document.querySelectorAll('[data-testid=toast][data-page]').length"
	// The page toasts come since pageToastsNow: each toast shown is a new element.
	pageToastsNow = "window.pageToastsBefore = new Set(document.querySelectorAll('[data-testid=toast][data-page]'))"
	newPageToasts = "[...document.querySelectorAll('[data-testid=toast][data-page]')].filter((x) => !window.pageToastsBefore.has(x)).length"
)

// newsOf says what the row of slug tells of its news: "" read, "version" a version not read, "new" a page never read.
func newsOf(slug string) string {
	return "((r) => r?.dataset.unread !== 'true' ? '' : r.querySelector('[data-testid=ver]') ? 'version' : 'new')(document.querySelector(" + js("#list li[data-slug="+slug+"]") + "))"
}

// pageNews is a promise of the news of slug as the server tells the list of pages: a page whole has no row to tell it.
func pageNews(slug string) string {
	return "fetch('/api/pages').then((r) => r.json()).then((l) => l.find((a) => a.slug === " + js(slug) + ")?.news ?? '')"
}

// goneReads counts in window.versionsTaken, from 0, the answers to the reads of slug's versions that do not list v2, each once the page
// has taken it: a read answered before v2 went is not counted.
func goneReads(b *browser, slug string) {
	b.t.Helper()
	b.eval(`window.versionsTaken = 0; { const f = window.fetch; window.fetch = (u, i) => f(u, i).then((r) => {
		if (new URL(String(u), location.href).pathname !== ` + js("/api/pages/"+slug+"/versions") + `) return r;
		const json = r.json.bind(r);
		r.json = () => json().then((v) => { if (!v.versions?.some((x) => x.n === 2)) setTimeout(() => { window.versionsTaken++; }); return v; });
		return r;
	}); }`)
}

// pressMenuItem opens the "…" menu in box and presses its item named item (data-testid). The menu is opened once more if it closes
// while waited for: the list's or the bar's first read may draw it anew, closed, just after it opened. Closed twice fails: a read
// that draws the same list anew each time.
func pressMenuItem(b *browser, box, item string) {
	b.t.Helper()
	menuItemShown(b, box, item, "")
	b.moving("document.querySelector(" + js(box+" [data-testid=more] [data-testid="+item+"]") + ").click()")
}

// menuItemShown opens the "…" menu in box by its summary (first, JS run on the summary s before the press, as "s.focus();") and
// waits until item shows in it, opening it once more if a redraw closed it.
func menuItemShown(b *browser, box, item, first string) {
	b.t.Helper()
	menu := "document.querySelector(" + js(box+" [data-testid=more]") + ")"
	got := "document.querySelector(" + js(box+" [data-testid=more] [data-testid="+item+"]") + ")"
	b.eval("window.menuOpened = 0")
	if b.until("the item "+item, "(() => { const m = "+menu+"; if (!m) return false; "+
		"if (!m.open) { if (window.menuOpened++ > 1) return 'closed again'; const s = m.querySelector('summary'); "+first+" s.click(); } "+
		"return "+got+"?.checkVisibility() && 'shown'; })()") != "shown" {
		b.t.Fatalf("the menu in %s closed again as it was opened again: a read draws it anew each time", box)
	}
}

// openSlugField unfolds the part of the edit modal that holds the URL name, once the modal is open.
func openSlugField(b *browser) {
	b.t.Helper()
	b.until("the edit modal", "document.getElementById('edit-dialog').open")
	b.eval("if (!document.getElementById('edit-more').open) document.querySelector('#edit-more > summary').click()")
}

// TestUIVersionPick chooses a version of a page beside the list: it stays through the list drawn anew and a version published, the latest
// not told read; the bar and the row's ways to open the page whole go with it; another page, and the latest chosen, leave it.
func TestUIVersionPick(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := diffSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	b.size(1440, 900, false)
	defer b.unsize()
	const title = "document.querySelector('#list li[data-slug=d] [data-testid=title]')"
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=d]')")
	b.eval(title + ".click()")
	b.until("the choice", "!document.getElementById('read-version').hidden")
	b.until("read", "!("+newsOf("d")+")")
	out := make([]string, 0, 30)
	out = append(out, b.eval("document.querySelector('#read-version selectedcontent').innerText.trim()"),
		b.eval(`[...document.querySelectorAll('#read-version option')].map((o) => o.textContent.replace(/ · \d\d\/\d\d \d\d:\d\d/, '')).join(', ')`))

	choose(b, "read-version", 2)
	out = append(out, b.until("v2 shown", "document.querySelector('[data-testid=ver-note]') && "+readTold),
		b.eval("document.querySelector('#read-version selectedcontent').innerText.trim()"),
		b.eval("document.activeElement.tagName"),
		b.eval(`[document.getElementById('read-title').getAttribute('href'), document.querySelector('#read-actions > [data-testid=open]').getAttribute('href'),
			document.querySelector('#read-actions > [data-testid=download]').getAttribute('href'), document.querySelector('#read-actions > [data-testid=download]').hasAttribute('download'),
			document.querySelector('#read-actions > [data-testid=diff-open]').getAttribute('href'), document.querySelector('#read-actions [data-testid=menu] [data-testid=open]').getAttribute('href')].join(' ')`))
	// The words' way to the latest leaves the focus on the choice, the words gone.
	b.eval("[...document.querySelectorAll('[data-testid=ver-note] a')].find((a) => a.textContent.includes('latest')).click()")
	out = append(out, b.until("the latest", besideLoaded+" === '/a/d/?raw' && document.querySelector('[data-testid=ver-note]') === null && document.activeElement.id"))
	choose(b, "read-version", 2)
	b.until("v2 again", besideLoaded+".endsWith('v=2') && document.querySelector('[data-testid=ver-note]')")
	b.eval(pageToastsNow)
	// Drawn anew, it stays; a version published, too, the words following, the row unread, no toast.
	other := "One again"
	if _, err := st.Edit("one", store.Edit{Title: &other}); err != nil {
		t.Fatal(err)
	}
	b.reread()
	b.until("drawn anew", "document.querySelector('#list li[data-slug=one] [data-testid=title]').textContent.includes('One again')")
	b.still()
	out = append(out, b.eval(besideLoaded))
	s.floe(t, "publish", s.page(t, "diff/d.yaml", "D", diffV3+"<p>four</p>"), "--slug", "d")
	out = append(out, b.until("v4 told", verWords+".includes('v4') && "+readTold), b.eval(newsOf("d")), b.eval(newPageToasts))

	// Pressed again and by Enter, the row opens the version shown; its link stays the latest's.
	out = append(out, b.eval(title+".getAttribute('href')"))
	r := b.rect(title)
	b.click(r.X+12, r.Y+6)
	out = append(out, b.until("whole", "location.pathname === '/a/d/' && location.search"))
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=d]')")
	b.eval(title + ".click()")
	b.until("the choice", "!document.getElementById('read-version').hidden")
	choose(b, "read-version", 2)
	b.eval(title + ".focus()")
	b.pressKey(kb.Enter)
	out = append(out, b.until("whole by the key", "location.pathname === '/a/d/' && location.search"))

	// The keys go on from the choice.
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=d]')")
	b.eval(title + ".click()")
	b.until("the choice", "!document.getElementById('read-version').hidden")
	b.until("read", "!("+newsOf("d")+")")
	b.eval(pageToastsNow)
	choose(b, "read-version", 2)
	b.until("v2", besideLoaded+".endsWith('v=2')")
	b.eval("document.getElementById('read-version').focus()")
	b.pressKey("j")
	out = append(out, b.until("the next row", "("+besideSlug+" ?? 'd') !== 'd' && ["+besideV+", document.querySelector('[data-testid=ver-note]') === null].join(' ')"))
	// The reads told, counted from here: the page come back to is told read, before it is marked unread
	b.eval(`window.reads = 0; window.fetch = ((f) => (u, i) => { if (i?.method === 'POST' && String(u).endsWith('/read')) window.reads++; return f(u, i); })(window.fetch);
		window.heard = 0; addEventListener('message', (e) => { if (e.data?.pageServed) window.heard++; })`)
	b.pressKey("k")
	out = append(out, b.until("back", besideSlug+" === 'd' && "+besideLoaded))
	b.until("read", "window.reads > 0 && !("+newsOf("d")+")")

	// Marked unread: the versions gone through tell nothing, until another page has been open.
	pressMenuItem(b, "#read-actions", "mark-unread")
	b.until("unread", newsOf("d"))
	// Counted once it is unread: a read told late of the page come back to is not of what is checked here
	b.eval("window.reads = 0; window.heard = 0")
	const shelfWord = "[...document.querySelectorAll('#read-version option')].find((o) => o.textContent.includes('v2')).textContent.includes('·')"
	out = append(out, b.eval(shelfWord))
	for _, n := range []int{2, 4} {
		was := b.eval("window.heard")
		choose(b, "read-version", n)
		b.until("told", "window.heard > "+was)
	}
	out = append(out, b.eval("window.reads"), b.eval("Boolean("+newsOf("d")+")"))
	b.pressKey("j")
	b.until("the next row", "("+besideSlug+" ?? 'd') !== 'd'")
	b.pressKey("k")
	out = append(out, b.until("read again", besideSlug+" === 'd' && !("+newsOf("d")+") && "+newPageToasts))

	// A read leaves the choice open, the focus on it, and its words told anew: the page marked unread by its toast's button, pressed with
	// the choice open.
	b.eval("window.kept = document.getElementById('read-version')")
	r = b.rect("document.getElementById('read-version')")
	b.click(r.X+r.Width/2, r.Y+r.Height/2)
	b.until("open", "document.getElementById('read-version').matches(':open')")
	b.eval("document.querySelector('[data-testid=toast][data-page] button').click()")
	b.until("unread", newsOf("d"))
	b.still()
	out = append(out, b.eval("[window.kept === document.getElementById('read-version'), window.kept.contains(document.activeElement), window.kept.matches(':open')].join(' ')"))
	b.pressKey(kb.Escape)

	check(t, "pick", out, strings.Join([]string{
		"v3", "v3 · Latest, v2, v1",
		"/a/d/?raw&v=2 / false / true / Showing v2 (at). The latest is v3.Open the latest versionSee what differs from the latest",
		"v2", "BODY",
		"/a/d/?v=2 /a/d/?raw&v=2 /api/pages/d/download?v=2 true /a/d/?diff&from=2&to=3 /a/d/?raw&v=2",
		"read-version",
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
	// The version each iframe was served, as it tells its frame, and the reads told on, kept across the moves in the tab.
	b.addScript(`if (window.parent === window) {
		const f = window.fetch;
		window.fetch = (u, i) => {
			if (i?.method === 'POST' && String(u).endsWith('/read')) sessionStorage.setItem('reads', Number(sessionStorage.getItem('reads')) + 1);
			return f(u, i);
		};
		addEventListener('message', (e) => { if (e.data?.pageServed) sessionStorage.setItem('served', (sessionStorage.getItem('served') ?? '') + ' ' + e.data.pageServed.n); });
	}`)
	const (
		diffOpen = "document.querySelector('#frame-actions [data-testid=diff-open]')"
		unread   = "document.querySelector('#frame-actions [data-testid=mark-unread]')"
	)
	open := func(path string) string {
		t.Helper()

		b.navigate(path)
		b.until("the bar", "!document.getElementById('frame-actions').hidden && "+diffOpen)
		// The choice is drawn as the list of versions comes, after the bar; a page with no versions to choose has no diff either
		b.until("the versions", diffOpen+".hidden || document.getElementById('frame-version').dataset.numbers")
		b.still()
		return b.eval(frameTold)
	}
	out := make([]string, 0, 24)
	// A page never read: at a version, that version is told read, and the latest is not.
	out = append(out, open("/a/d/?v=2"))
	out = append(out, b.until("v2 told read", pageNews("d")+".then((n) => n === 'version' && n)"), b.eval("document.querySelectorAll('[data-testid=toast]').length"),
		b.eval("document.getElementById('print-raw').getAttribute('href').replace(location.origin, '')"), b.eval("sessionStorage.getItem('served')"))
	// The version last read, told as it is read.
	const words = "[...document.querySelectorAll('#frame-version option')].map((o) => o.textContent.replace(/ · \\d\\d\\/\\d\\d \\d\\d:\\d\\d/, '')).join(', ')"
	out = append(out, b.until("last read", words+".includes('Last read') && "+words))
	for _, path := range []string{
		"/a/d/?v=3", "/a/d/?v=99", "/a/d/?v=abc", "/a/d/?v=01", "/a/d/?v=", "/a/d/?v=0", "/a/one/?v=1", "/a/one/?v=2", "/a/d/?v=2#part",
		"/a/l/?v=3", "/a/docs/?file=index.html&v=3",
	} {
		got := open(path)
		out = append(out, path+": "+got)
	}
	b.navigate("/a/d/?diff&v=2")
	b.until("the diff", "document.querySelector('#diff-body > *')")
	out = append(out, b.eval(frameTold))

	// Chosen with the mouse and the keys: a move that Back undoes, the focus on the choice again.
	open("/a/d/?v=2")
	r := b.rect("document.getElementById('frame-version')")
	b.click(r.X+r.Width/2, r.Y+r.Height/2)
	b.until("open", "document.getElementById('frame-version').matches(':open')")
	o := b.rect("[...document.querySelectorAll('#frame-version option')].find((o) => o.value === '1')")
	b.click(o.X+o.Width/2, o.Y+o.Height/2)
	b.until("v1", "location.search === '?v=1' && !document.getElementById('frame-version').hidden && document.activeElement.id === 'frame-version'")
	out = append(out, b.eval(frameTold))
	b.pressKey(kb.Enter)
	b.until("open by the key", "document.getElementById('frame-version').matches(':open')")
	b.pressKey(kb.ArrowUp)
	b.pressKey(kb.Enter)
	b.until("v2 by the key", "location.search === '?v=2' && !document.getElementById('frame-version').hidden && document.activeElement.id === 'frame-version'")
	b.moving("history.back()")
	b.until("v1 back", "location.search === '?v=1' && !document.getElementById('frame-version').hidden")
	out = append(out, b.eval("[document.getElementById('frame-version').value, document.getElementById('frame').dataset.version].join(' ')"))
	choose(b, "frame-version", 2)
	b.until("v2", "location.search === '?v=2' && !document.getElementById('frame-version').hidden")
	choose(b, "frame-version", 1)
	b.until("v1", "location.search === '?v=1' && !document.getElementById('frame-version').hidden")
	b.moving("history.back()")
	b.until("back", "location.search === '?v=2' && !document.getElementById('frame-version').hidden")
	out = append(out, b.eval(frameTold))
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
	open("/a/d/")
	b.until("read", unread+"?.hidden === false")
	pressMenuItem(b, "#frame-actions", "mark-unread")
	b.eval("sessionStorage.removeItem('reads')")
	b.until("unread", unread+".hidden")
	choose(b, "frame-version", 2)
	b.until("v2", "location.search === '?v=2' && document.querySelector('[data-testid=ver-note]')")
	b.moving("[...document.querySelectorAll('[data-testid=ver-note] a')].find((a) => a.textContent.includes('latest')).click()")
	b.until("the latest", "location.search === '' && !document.getElementById('frame-version').hidden && document.activeElement.id === 'frame-version'")
	b.still()
	out = append(out, b.eval(unread+".hidden"), b.eval("document.querySelectorAll('[data-testid=toast]').length"), b.eval("sessionStorage.getItem('reads') ?? '0'"))
	// What a move kept of another page does not hold this one back.
	s.floe(t, "publish", s.page(t, "diff/n.yaml", "N", "<p>n</p>"), "--slug", "n")
	b.eval(`sessionStorage.setItem('version.move', JSON.stringify({ slug: 'other', focus: true, kept: true }))`)
	// The toast goes after 3 s, which a slow machine may take to open the page: noted as it comes, by a script run before the page's
	toastSeen := b.addScript(`addEventListener('DOMContentLoaded', () => new MutationObserver(() => {
		if (document.querySelector('[data-testid=toast][data-page]')) window.pageToastSeen = true;
	}).observe(document.body, { childList: true, subtree: true }))`)
	b.navigate("/a/n/")
	// Timed out, what was told says which way it went: the versions the frame said it served, the reads sent, the menu's way back
	// (two served, unreadHidden false and toastSeen false was S428: a quick second pageServed dropped the toast)
	told := func(what, expr string) string {
		t.Helper()
		if got, err := b.try(expr); err == nil && truthy(got) {
			return got
		}
		for deadline := time.Now().Add(waitLimit); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
			if got, err := b.try(expr); err == nil && truthy(got) {
				return got
			}
		}
		t.Fatalf("timed out waiting for %s: %s", what, b.eval("JSON.stringify({ served: sessionStorage.getItem('served'), reads: sessionStorage.getItem('reads'), "+
			"unreadHidden: "+unread+"?.hidden, toastSeen: window.pageToastSeen ?? false })"))
		return ""
	}
	out = append(out, told("the page's toast", "window.pageToastSeen === true"), told("read as ever", unread+"?.hidden === false"))
	b.removeScript(toastSeen)

	// A slug changed keeps the version.
	open("/a/d/?v=2")
	pressMenuItem(b, "#frame-actions", "edit")
	openSlugField(b)
	b.fill("#edit-slug", "d3")
	b.moving("document.getElementById('edit-submit').click()")
	out = append(out, b.until("the new name", "location.pathname !== '/a/d/' && location.pathname + location.search"))

	check(t, "url", out, strings.Join([]string{
		"?v=2 / /a/d/?raw&v=2 / false / Showing v2 (at). The latest is v3.Open the latest versionSee what differs from the latest",
		"version", "0", "/a/d/?raw&v=2", " 2",
		"v3 · Latest, v2 · Last read, v1",
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
		"1 1",
		"?v=2 / /a/d/?raw&v=2 / false / Showing v2 (at). The latest is v3.Open the latest versionSee what differs from the latest",
		"frame-version: solid 2px rgb(107, 105, 100) 1px seen",
		"true", "0", "0",
		"true", "true",
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
	out = append(out, b.until("v4 told", verWords+".includes('v4') && "+frameTold))

	// Pushed out by the versions after it, in the frame.
	for i := range 20 {
		s.floe(t, "publish", s.page(t, "diff/d.yaml", "D", diffV3+"<p>many "+strconv.Itoa(i)+"</p>"), "--slug", "d")
	}
	out = append(out, b.until("the latest", "location.search === '' && !document.getElementById('frame-version').hidden && "+verWords+" === "+js(gone)+" && "+frameTold),
		b.eval("document.getElementById('print-raw').getAttribute('href').replace(location.origin, '')"), b.eval("document.querySelectorAll('[data-testid=toast]').length"))

	// One version left: no choice; of a page never read since, unread, marked as such.
	s.floe(t, "publish", s.page(t, "diff/e.yaml", "E", "<p>one</p>"), "--slug", "e")
	s.floe(t, "publish", s.page(t, "diff/e.yaml", "E", "<p>two</p>"), "--slug", "e")
	s.floe(t, "publish", s.page(t, "diff/e.yaml", "E", "<p>three</p>"), "--slug", "e")
	b.navigate("/a/e/?v=2")
	b.until("the choice", "!document.getElementById('frame-version').hidden")
	s.floe(t, "rm", "e")
	s.floe(t, "publish", s.page(t, "diff/e.yaml", "E", "<p>again</p>"), "--slug", "e")
	out = append(out, b.until("the latest", verWords+" === "+js(gone)+" && "+frameTold))
	b.still()
	out = append(out, b.eval(pageNews("e")), b.eval("document.querySelectorAll('[data-testid=toast]').length"))

	// What was asked of a page gone goes with it.
	b.navigate("/a/e/?v=99")
	b.until("the words", verWords+" === "+js(gone))
	s.floe(t, "rm", "e")
	out = append(out, b.until("no page", "!document.getElementById('notfound').hidden && document.querySelector('[data-testid=ver-note]') === null"))

	// The number shown come to be the latest's, after a removal: the latest in its place, with no words.
	s.floe(t, "publish", s.page(t, "diff/h.yaml", "H", "<p>one</p>"), "--slug", "h")
	s.floe(t, "publish", s.page(t, "diff/h.yaml", "H", "<p>two</p>"), "--slug", "h")
	b.navigate("/a/h/?v=1")
	b.until("the choice", "!document.getElementById('frame-version').hidden")
	s.floe(t, "rm", "h")
	s.floe(t, "publish", s.page(t, "diff/h.yaml", "H", "<p>again</p>"), "--slug", "h")
	out = append(out, b.until("the latest", "location.search === '' && document.getElementById('frame').hasAttribute('src') && "+frameTold))

	// What is typed in a modal stays: the page goes to the latest once it closes.
	s.floe(t, "publish", s.page(t, "diff/m.yaml", "M", "<p>one</p>"), "--slug", "m")
	s.floe(t, "publish", s.page(t, "diff/m.yaml", "M", "<p>two</p>"), "--slug", "m")
	s.floe(t, "publish", s.page(t, "diff/m.yaml", "M", "<p>three</p>"), "--slug", "m")
	b.navigate("/a/m/?v=2")
	b.until("the choice", "!document.getElementById('frame-version').hidden")
	pressMenuItem(b, "#frame-actions", "edit")
	b.until("the edit modal", "document.getElementById('edit-dialog').open")
	b.fill("#edit-name", "typed")
	// The answers to the reads of m's versions that find v2 gone: such a read keeps the page while the modal is open, which nothing on
	// screen tells.
	goneReads(b, "m")
	s.floe(t, "rm", "m")
	s.floe(t, "publish", s.page(t, "diff/m.yaml", "M", "<p>again</p>"), "--slug", "m")
	b.reread()
	b.until("the versions read", "window.versionsTaken > 0")
	out = append(out, b.eval("[document.getElementById('edit-dialog').open, document.getElementById('edit-name').value, location.search].join(' ')"))
	b.moving("document.querySelector('#edit-dialog [data-close]').click()")
	out = append(out, b.until("the latest", "location.search === '' && "+verWords+" === "+js(gone)+" && "+frameTold))

	// Beside the list, with the list's look held back across the removal, whose answer in between would let the row go.
	s.floe(t, "publish", s.page(t, "diff/f.yaml", "F", "<p>one</p>"), "--slug", "f")
	s.floe(t, "publish", s.page(t, "diff/f.yaml", "F", "<p>two</p>"), "--slug", "f")
	s.floe(t, "publish", s.page(t, "diff/f.yaml", "F", "<p>three</p>"), "--slug", "f")
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=f]')")
	b.eval("document.querySelector('#list li[data-slug=f] [data-testid=title]').click()")
	b.until("the choice", "!document.getElementById('read-version').hidden && "+newsOf("f")+" === ''")
	choose(b, "read-version", 2)
	b.until("v2", besideLoaded+".endsWith('v=2')")
	// The words come before the versions read that finds v2 gone hides the choice; counted from before the removal, as that read may
	// come at any time after it (the versions are not held)
	goneReads(b, "f")
	release := b.holdReads(unsent, "/api/pages")
	s.floe(t, "rm", "f")
	s.floe(t, "publish", s.page(t, "diff/f.yaml", "F", "<p>again</p>"), "--slug", "f")
	release()
	b.eval(pageToastsNow)
	b.reread()
	b.until("the versions read", "window.versionsTaken > 0")
	out = append(out, b.until("the latest", verWords+" === "+js(gone)+" && "+readTold))
	b.still()
	out = append(out, b.eval(newsOf("f")), b.eval(newPageToasts))
	// Pushed out by the versions after it, others left: the choice stays, and a version chosen takes the place of the words.
	s.floe(t, "publish", s.page(t, "diff/p.yaml", "P", "<p>one</p>"), "--slug", "p")
	s.floe(t, "publish", s.page(t, "diff/p.yaml", "P", "<p>two</p>"), "--slug", "p")
	b.until("the row", "document.querySelector('#list li[data-slug=p]')")
	b.eval("document.querySelector('#list li[data-slug=p] [data-testid=title]').click()")
	b.until("the choice", besideSlug+" === 'p' && !document.getElementById('read-version').hidden && "+newsOf("p")+" === ''")
	b.eval(pageToastsNow)
	choose(b, "read-version", 1)
	b.until("v1", besideLoaded+".endsWith('v=1')")
	for i := range 20 {
		s.floe(t, "publish", s.page(t, "diff/p.yaml", "P", "<p>many "+strconv.Itoa(i)+"</p>"), "--slug", "p")
	}
	out = append(out, b.until("the latest", verWords+" === "+js(gone)+" && "+readTold))
	b.still()
	out = append(out, b.eval(newsOf("p")+" !== ''"), b.eval(newPageToasts))
	choose(b, "read-version", 10)
	out = append(out, b.until("v10", besideLoaded+".endsWith('v=10') && "+verWords+".includes('v10') && "+verWords))
	// Another page, and back: read, the words gone.
	b.eval("document.querySelector('#list li[data-slug=d] [data-testid=title]').click()")
	out = append(out, b.until("another", besideSlug+" === 'd' && !document.querySelector('[data-testid=ver-note]') && 'no words'"))
	b.eval("document.querySelector('#list li[data-slug=f] [data-testid=title]').click()")
	out = append(out, b.until("read", besideSlug+" === 'f' && "+newsOf("f")+" === '' && 'read'"))

	check(t, "gone", out, strings.Join([]string{
		"?v=2 / /a/d/?raw&v=2 / false / Showing v2 (at). The latest is v4.Open the latest versionSee what differs from the latest",
		" / /a/d/?raw / false / " + gone, "/a/d/?raw", "0",
		" / /a/e/?raw / true / " + gone, "new", "0",
		"true",
		" / /a/h/?raw / true / ",
		"true typed ?v=2",
		" / /a/m/?raw / true / " + gone,
		"/a/f/?raw / true / false / " + gone, "new", "0",
		"/a/p/?raw / false / false / " + gone, "true", "0",
		"Showing v10 (at). The latest is v22.Open the latest versionSee what differs from the latest",
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
	b.until("v1 told read", pageNews("d")+".then((n) => n === 'version')")
	b.navigate("/a/d/?v=99")
	b.until("both", "document.querySelector('[data-testid=ver-note]') && document.querySelector('[data-testid=page-note]')")
	order := b.eval(`[...document.querySelectorAll('[data-testid=ver-note], [data-testid=page-note]')].filter((e) => e.getClientRects().length)
		.map((e) => e.dataset.testid === 'ver-note' ? 'version' : 'page').join(' ')`)
	check(t, "notes", []string{order, b.eval(verWords)}, "version page|That version cannot be opened, so this shows the latest.")
}

// TestUIVersionPickAhead chooses the version the list of pages still takes for the latest, that list read before one was published and the
// choice after: it opens that version, not the latest, and does not tell the latest read. The list is held as it was read by the test's own
// answer to its reads.
func TestUIVersionPickAhead(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := diffSite(t)
	b := s.b
	b.size(1440, 900, false)
	defer b.unsize()
	b.addScript(`if (window.parent === window) {
		const f = window.fetch;
		window.fetch = (u, i) => (String(u) === '/api/pages' && window.heldList ? Promise.resolve(new Response(window.heldList, { headers: { 'Content-Type': 'application/json' } })) : f(u, i));
	}`)
	b.navigate("/")
	b.until("the row", "document.querySelector('#list li[data-slug=d]')")
	b.eval("document.querySelector('#list li[data-slug=d] [data-testid=title]').click()")
	b.until("the choice", "!document.getElementById('read-version').hidden")
	b.until("read", "!("+newsOf("d")+")")
	b.eval("fetch('/api/pages').then((r) => r.text()).then((l) => { window.heldList = l; })")
	s.floe(t, "publish", s.page(t, "diff/d.yaml", "D", diffV3+"<p>four</p>"), "--slug", "d")
	// A version read has the versions read again, v4 among them, the list still at v3
	choose(b, "read-version", 2)
	b.until("v4 in the choice", "document.querySelector('#read-version option[value=\"4\"]')")
	choose(b, "read-version", 3)
	b.until("v3", besideLoaded+".endsWith('v=3')")
	check(t, "picked", []string{b.eval(besideV)}, "3")
}

// TestUIVersionAnswers: moving to another page takes the choice and words of the one before away at once, and an answer come late, of a page
// since left, draws nothing.
func TestUIVersionAnswers(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := diffSite(t)
	b := s.b
	s.floe(t, "publish", s.page(t, "diff/e.yaml", "E", "<p>one</p>"), "--slug", "e")
	s.floe(t, "publish", s.page(t, "diff/e.yaml", "E", "<p>two</p>"), "--slug", "e")
	b.size(1440, 900, false)
	defer b.unsize()
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=d]') && document.querySelector('#list li[data-slug=e]')")
	row := func(slug string) string {
		return "document.querySelector('#list li[data-slug=" + slug + "] [data-testid=title]').click()"
	}
	b.eval(row("d"))
	b.until("the choice", "!document.getElementById('read-version').hidden")
	choose(b, "read-version", 2)
	b.until("v2", besideLoaded+".endsWith('v=2') && document.querySelector('[data-testid=ver-note]')")
	b.eval(`window.hold = new Promise((r) => { window.release = r; });
		window.fetch = ((f) => async (u, i) => { if (String(u).endsWith('/e/versions')) { await window.hold; const r = await f(u, i); window.answeredAt = performance.now(); return r; } return f(u, i); })(window.fetch)`)
	b.eval(row("e"))
	const state = `[document.getElementById('read-version').hidden, document.getElementById('read-version').dataset.old === 'true', document.querySelector('[data-testid=ver-note]') === null, [...document.querySelectorAll('#read-version option')].map((o) => o.value).join()].join(' ')`
	out := make([]string, 0, 5)
	out = append(out, b.eval(state))
	b.eval(row("d"))
	b.until("d again", besideSlug+" === 'd' && !document.getElementById('read-version').hidden")
	b.eval("window.release()")
	b.until("the late answer, and time to draw it", "window.answeredAt && performance.now() - window.answeredAt > 200")
	out = append(out, b.eval(state))
	b.eval(row("e"))
	out = append(out, b.until("e's", "!document.getElementById('read-version').hidden && "+state))
	// A read of a version between the last read and the latest moves the words, the list of pages saying nothing of it.
	b.eval(row("d"))
	b.until("d read", besideSlug+" === 'd' && !("+newsOf("d")+")")
	choose(b, "read-version", 2)
	b.until("v2", besideLoaded+".endsWith('v=2')")
	s.floe(t, "publish", s.page(t, "diff/d.yaml", "D", diffV3+"<p>four</p>"), "--slug", "d")
	s.floe(t, "publish", s.page(t, "diff/d.yaml", "D", diffV3+"<p>five</p>"), "--slug", "d")
	const lastRead = "[...document.querySelectorAll('#read-version option')].filter((o) => o.textContent.includes('Last read')).map((o) => o.value).join()"
	out = append(out, b.until("v3 last read", "[...document.querySelectorAll('#read-version option')].length === 5 && "+lastRead))
	choose(b, "read-version", 4)
	out = append(out, b.until("v4 last read", lastRead+" === '4' && "+lastRead))
	check(t, "answers", out, "true false true |false false true 3,2,1|false false true 2,1|3|4")
}
