package browsertest_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

// unreadSite is a site of three new pages, a link, and a script that notes what is sent of a read, and answers as a test plans:
// window.plan[method] holds the answers to come (a status, or 'throw'), window.gate holds a request back until window.open().
func unreadSite(t *testing.T) *site {
	t.Helper()

	s := newSite(t)
	for _, slug := range []string{"p1", "p2", "p3"} {
		s.floe(t, "publish", s.page(t, "unread/"+slug+".yaml", "Title "+slug, "<p id=\"p\">"+slug+"</p>"), "--slug", slug)
	}
	s.floe(t, "publish", "--link", s.write(t, "unread/link/l.html", "<title>L</title><p>l</p>"), "--slug", "l")
	s.b.addScript(`(() => { const real = window.fetch; window.sent = []; window.plan = { POST: [], DELETE: [] };
		window.fetch = async (u, i) => {
			if (!String(u).endsWith('/read')) return real(u, i);
			const method = i?.method ?? 'GET', slug = String(u).split('/')[3];
			if (window.gate) await window.gate;
			window.sent.push(method + ' ' + slug);
			const planned = window.plan[method].shift();
			if (planned === 'throw') throw new Error('cut off');
			if (planned) return new Response('no', { status: planned });
			return real(u, i);
		};
		window.shut = () => { window.gate = new Promise((go) => { window.open_ = () => { window.gate = null; go(); }; }); };
	})()`)
	return s
}

const (
	unreadRow    = "document.querySelector('#list li[data-slug=%s]')"
	pageToastSay = "document.querySelector('[data-testid=toast][data-page]')?.firstChild && [...document.querySelector('[data-testid=toast][data-page]').childNodes].filter((n) => n.nodeType === 3).map((n) => n.textContent).join('')"
)

func unreadList(b *browser) {
	b.t.Helper()
	b.size(1440, 800, false)
	b.navigate("/")
	b.until("the rows", fmt.Sprintf(unreadRow, "p1"))
	b.eval(readMarks)
}

// TestUIUnread opens a new page beside the list: read, with a toast to take it back, whose press leaves the page where it is, and
// which opening again reads; a page read, a link and a move on say nothing more.
func TestUIUnread(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := unreadSite(t)
	b := s.b
	unreadList(b)
	b.eval("document.activeElement.blur(); window.loads = 0; document.getElementById('read-frame').addEventListener('load', () => { window.loads++; })")
	out := make([]string, 0, 16)
	openRowOf(b, "p1")
	b.until("the toast", pageToastButton)
	untilUnseen(b, "p1", false)
	out = append(out, b.eval(pageToastSay), b.eval(pageToastButton+".textContent + ' / ' + "+pageToastButton+".getAttribute('aria-label')"),
		b.eval(roleJS+"(document.getElementById('toasts')) + ' ' + document.activeElement.matches('[data-testid=toast] *')"))
	b.until("the page in", "window.loads > 0")
	loads := b.eval("window.loads")
	// Pressed: the row new again, the count up, the page left as it is.
	before := b.eval("document.getElementById('news-n').textContent")
	b.eval(pageToastButton + ".click()")
	b.until("the words", pageToastSay+" === 'Marked as unread. It is marked as read when you open it again.'")
	untilUnseen(b, "p1", true)
	out = append(out, b.eval("window.sent.join(',')"), b.eval("document.querySelectorAll('[data-testid=toast][data-page] button').length"),
		b.eval("document.getElementById('news-n').textContent !== "+js(before)), b.eval("String(window.loads) === "+js(loads)))
	// On to the next: one toast, of that page; back: read, with the toast.
	openRowOf(b, "p2")
	b.until("the next toast", pageToastButton)
	out = append(out, b.eval(pageToasts))
	openRowOf(b, "p1")
	b.until("read again", "window.sent.filter((x) => x === 'POST p1').length === 2")
	b.until("its toast", pageToastButton+"?.getAttribute('aria-label') === 'Mark Title p1 as unread'")
	untilUnseen(b, "p1", false)
	// A page read, and a link: no toast, the one before gone.
	openRowOf(b, "p2")
	b.until("told", "window.sent.filter((x) => x === 'POST p2').length === 2")
	b.marksDone(5) // POST p1, DELETE p1, POST p2, POST p1, POST p2
	out = append(out, b.eval(pageToasts))
	openRowOf(b, "l")
	b.until("the link", besideSlug+" === 'l'")
	b.marksDone(5)
	out = append(out, b.eval(pageToasts+" + ' ' + window.sent.includes('POST l')"))
	// A page of its own: the same, and loaded anew it is read again.
	b.navigate("/a/p3/")
	b.until("the toast whole", pageToastButton)
	b.eval(pageToastButton + ".focus(); " + pageToastButton + ".click()")
	b.until("the words whole", "document.querySelector('[data-testid=toast][data-page]') && !"+pageToastButton)
	out = append(out, b.eval("document.activeElement.matches('#frame-actions summary')"))
	b.navigate("/a/p3/")
	b.until("the toast again", pageToastButton)
	check(t, "unread", out, strings.Join([]string{
		"Marked as read", "Mark as unread / Mark Title p1 as unread", "status false",
		"POST p1,DELETE p1", "0", "true", "true", "1", "0", "0 false", "true",
	}, "|"))
}

// TestUIRowAgainUnread keeps a page marked unread as j finds no row past its own, and opens it whole, read with its toast, as its row
// is pressed again.
func TestUIRowAgainUnread(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := unreadSite(t)
	b := s.b
	unreadList(b)
	last := b.eval("[...document.querySelectorAll('#list li[data-slug]')].at(-1).dataset.slug")
	openRowOf(b, last)
	b.until("the toast", pageToastButton)
	b.eval(pageToastButton + ".click()")
	b.until("the words", "document.querySelector('[data-testid=toast][data-page]') && !"+pageToastButton)
	untilUnseen(b, last, true)
	b.marksDone(2) // POST, DELETE
	b.eval(fmt.Sprintf(unreadRow, last) + ".querySelector('[data-testid=title]').focus()")
	b.pressKey("j")
	begun := b.reread()
	b.untilRead(begun, "a read after j", "true")
	untilUnseen(b, last, true)
	// Kept unread: no read sent after j.
	check(t, "the last row kept", []string{b.eval(besideSlug + " === " + js(last) + " && window.marks.length === 2 && !" + pageToastButton)}, "true")
	openRowOf(b, last)
	b.until("whole", "location.pathname === "+js("/a/"+last+"/"))
	b.until("its toast whole", pageToastButton)
}

// The button of the toast of the page shown.
const pageToastButton = "document.querySelector('[data-testid=toast][data-page] button')"

// openRowOf presses the title of the row of slug.
func openRowOf(b *browser, slug string) {
	b.t.Helper()
	b.moving(fmt.Sprintf(unreadRow, slug) + ".querySelector('[data-testid=title]').click()")
}

// untilUnseen waits until the row of slug is unseen, or not, as want says.
func untilUnseen(b *browser, slug string, want bool) {
	b.t.Helper()
	b.until(fmt.Sprintf("%s unseen %v", slug, want), "("+fmt.Sprintf(unreadRow, slug)+".dataset.unread === 'true') === "+fmt.Sprint(want))
}

// TestUIReadRefused says a read the server refuses as a failure to mark the page read, the server's answer its reason.
func TestUIReadRefused(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := unreadSite(t)
	b := s.b
	unreadList(b)
	b.eval("window.plan.POST = [500]")
	openRowOf(b, "p1")
	b.until("refused", "document.getElementById('error').textContent !== ''")
	check(t, "refused", []string{b.eval("document.getElementById('error').textContent")}, "Failed to mark as read: HTTP 500: no")
}

// TestUIUnreadKept keeps a page marked unread as it draws itself anew, whatever else goes on, until another page is opened; and a
// page read as it draws itself anew says nothing.
// LIMIT: a page marked unread while its read is on its way is not checked here: a person can press it only in a narrow case (a page
// read, a newer version known by the 3 s poll, pressed before that version's mark is answered). The React rebuild's unit test of
// features/reader (P4) checks that the read's answer does not take the mark away.
func TestUIUnreadKept(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := unreadSite(t)
	b := s.b
	unreadList(b)
	// heard records the paths of the pages that told the frame they were served, heard here after the frame's own listener, which
	// has by then sent its read or kept it back, or set it in the line.
	const heard = "window.heard = []; addEventListener('message', (e) => { if (e.data?.pageServed) window.heard.push(e.data.pagePath); })"
	heardOf := func(slug string) {
		t.Helper()
		b.until("told of "+slug, "window.heard.some((p) => p.startsWith("+js("/a/"+slug+"/")+"))")
	}
	// anew has the page beside the list draw itself anew, then waits for marks read marks in all.
	anew := func(marks int) {
		t.Helper()
		b.eval("window.heard = []; document.getElementById('read-frame').contentWindow.location.replace(" + besideLoaded + ")")
		b.until("told anew", "window.heard.length > 0")
		b.marksDone(marks)
	}
	posts := func(slug string) string {
		t.Helper()
		return b.eval("window.sent.filter((x) => x === 'POST " + slug + "').length")
	}
	// open opens the row of slug, read, and waits for marks read marks in all.
	open := func(slug string, marks int) {
		t.Helper()
		openRowOf(b, slug)
		b.until(slug+" beside", besideSlug+" === "+js(slug))
		b.marksDone(marks)
	}
	b.eval(heard)
	out := make([]string, 0, 16)
	// p2 and p3 read first, so that their rows' menus mark them unread.
	open("p2", 1)
	open("p3", 2)
	// Marked unread and drawn anew: unread, told nothing. Another page marked from its row does not take that away, nor its failure.
	openRowOf(b, "p1")
	b.until("the toast", pageToastButton)
	b.eval(pageToastButton + ".click()")
	untilUnseen(b, "p1", true)
	b.marksDone(4)
	anew(4)
	out = append(out, posts("p1"))
	pressMenuItem(b, "#list li[data-slug=p2]", "mark-unread")
	b.marksDone(5)
	anew(5)
	out = append(out, posts("p1"))
	b.eval("window.plan.DELETE = [500]")
	pressMenuItem(b, "#list li[data-slug=p3]", "mark-unread")
	b.marksDone(6)
	out = append(out, b.eval("document.getElementById('error').textContent === 'Failed to mark as unread: HTTP 500: no'"))
	anew(6)
	out = append(out, posts("p1"))
	untilUnseen(b, "p1", true)
	// Read again with its toast, and pressed twice, the first failing: the error goes with the next that goes well, and it is still held.
	open("p2", 7)
	openRowOf(b, "p1")
	b.until("the toast again", pageToastButton+"?.getAttribute('aria-label') === 'Mark Title p1 as unread'")
	b.marksDone(8)
	b.eval("window.plan.DELETE = [500]; window.shut(); " + pageToastButton + ".click(); " + pageToastButton + ".click()")
	b.eval("window.open_()")
	b.marksDone(10)
	untilUnseen(b, "p1", true)
	out = append(out, b.eval("document.getElementById('error').textContent === ''"))
	anew(10)
	out = append(out, posts("p1"))
	// A failure of its own lets it go: drawn anew, it is read.
	open("p2", 11)
	openRowOf(b, "p1")
	b.until("the toast once more", pageToastButton+"?.getAttribute('aria-label') === 'Mark Title p1 as unread'")
	b.marksDone(12)
	b.eval("window.plan.DELETE = ['throw']; " + pageToastButton + ".click()")
	b.marksDone(13)
	out = append(out, b.eval("document.getElementById('error').textContent.includes('cut off')"))
	anew(14)
	out = append(out, posts("p1"))
	// The answer to a read comes after a move on: no toast of the page left.
	unreadList(b)
	b.eval(heard + `; window.toasted = [];
		new MutationObserver((records) => records.forEach((r) => r.addedNodes.forEach((n) => {
			if (n.dataset?.testid === 'toast') window.toasted.push([...n.childNodes].filter((c) => c.nodeType === 3).map((c) => c.textContent).join(''));
		}))).observe(document.getElementById('toasts'), { childList: true });
		window.shut()`)
	openRowOf(b, "p1")
	b.until("the mark held at the gate", "window.marks.length === 1")
	openRowOf(b, "l")
	b.eval("window.open_()")
	b.marksDone(1)
	out = append(out, b.eval("window.toasted.length + ' ' + window.sent.at(-1)"))
	b.eval("window.toasted.length = 0")
	// A page marked from its row, the mark waiting behind another, then opened: the read takes its place, with nothing said of it.
	const said = "window.toasted.filter((m) => m === 'Marked as read').length + ' ' + (document.getElementById('error').textContent === '')"
	for i, slug := range []string{"p3", "p2", "p1"} {
		was := posts(slug)
		openRowOf(b, slug)
		b.until("told of "+slug, "window.sent.filter((x) => x === 'POST "+slug+"').length > "+was)
		b.marksDone(2 + i)
	}
	b.eval("window.toasted.length = 0; window.shut()")
	pressMenuItem(b, "#list li[data-slug=p3]", "mark-unread")
	pressMenuItem(b, "#list li[data-slug=p2]", "mark-unread")
	b.eval("window.heard = []")
	openRowOf(b, "p2")
	heardOf("p2")
	b.eval("window.open_()")
	b.marksDone(6)
	out = append(out, b.eval("window.sent.slice(-2).join(',')"), b.eval(said))
	// The page shown, the mark waiting, away and back: the same, as it was opened last.
	open("p3", 7)
	open("p2", 8)
	sent := b.eval("window.sent.length")
	b.eval("window.toasted.length = 0; window.shut()")
	pressMenuItem(b, "#list li[data-slug=p3]", "mark-unread")
	pressMenuItem(b, "#list li[data-slug=p2]", "mark-unread")
	b.eval("window.heard = []")
	openRowOf(b, "p1")
	heardOf("p1")
	openRowOf(b, "p2")
	heardOf("p2")
	b.eval("window.open_()")
	b.marksDone(11)
	out = append(out, b.eval("window.sent.slice("+sent+").filter((x) => x.endsWith('p2')).join(',')"), b.eval(said))
	check(t, "kept", out, strings.Join([]string{
		"1", "1", "true", "1", "true", "2", "true", "4",
		"0 POST p1", "DELETE p3,POST p2", "0 true", "POST p2", "0 true",
	}, "|"))
}

// TestUIUnreadPublish reads a page open as a publish draws it anew, saying nothing; marked unread after, the version before counts.
func TestUIUnreadPublish(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := unreadSite(t)
	b := s.b
	unreadList(b)
	openRowOf(b, "p1")
	b.until("the toast", pageToastButton)
	b.eval(pageToastsNow)
	s.floe(t, "publish", s.page(t, "unread/p1.yaml", "Title p1", "<p id=\"p\">p1, more</p>"), "--slug", "p1")
	b.until("read as it drew itself", "window.sent.filter((x) => x === 'POST p1').length === 2")
	b.marksDone(2)
	untilUnseen(b, "p1", false)
	out := make([]string, 0, 2)
	out = append(out, b.eval(newPageToasts))
	pressMenuItem(b, "#list li[data-slug=p1]", "mark-unread")
	untilUnseen(b, "p1", true)
	out = append(out, b.eval(fmt.Sprintf(unreadRow, "p1")+".querySelector('[data-testid=ver]').textContent"))
	check(t, "publish", out, "0|v2 · new version")
}

// TestUIUnreadToast holds the toast for the time it says, and while pointed at or focused, by mouse and by finger.
func TestUIUnreadToast(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := unreadSite(t)
	b := s.b
	// Only the times of these toasts are in the test's hands.
	b.addScript(`(() => { const set = window.setTimeout, clear = window.clearTimeout; window.timers = new Map(); let id = -1;
		window.setTimeout = (f, ms, ...rest) => { if (ms !== 8000 && ms !== 5000) return set(f, ms, ...rest); window.timers.set(--id, { f, ms }); return id; };
		window.clearTimeout = (x) => { window.timers.delete(x); clear(x); };
		window.fire = () => { const [k, v] = [...window.timers][0]; window.timers.delete(k); v.f(); };
	})()`)
	unreadList(b)
	const (
		timers = "[...window.timers.values()].map((x) => x.ms).join(',')"
		toast  = "[data-testid=toast][data-page]"
	)
	// p3 read first, a page read and not shown to mark unread below; the time of its toast, gone as p1's comes, is not this test's
	openRowOf(b, "p3")
	b.until("p3's toast", pageToastButton)
	untilUnseen(b, "p3", false)
	b.eval("window.timers.clear()")
	openRowOf(b, "p1")
	b.until("the toast", pageToastButton+"?.getAttribute('aria-label') === 'Mark Title p1 as unread'")
	b.still()
	out := make([]string, 0, 13)
	out = append(out, b.eval(timers), b.eval("getComputedStyle(document.querySelector("+js(toast)+")).animationName"))
	at := func(selector string) (float64, float64) {
		t.Helper()
		var x, y float64
		if _, err := fmt.Sscan(b.eval("(() => { const r = document.querySelector("+js(selector)+").getBoundingClientRect(); return (r.left + 12) + ' ' + (r.top + r.height / 2); })()"), &x, &y); err != nil {
			t.Fatal(err)
		}
		return x, y
	}
	move := func(x, y float64) {
		t.Helper()
		if _, err := chromedp.Call(b.ctx, input.DispatchMouseEvent, input.DispatchMouseEventParams{Type: input.DispatchMouseEventTypeMouseMoved, X: x, Y: y}); err != nil {
			t.Fatal(err)
		}
	}
	x, y := at(toast)
	move(x, y)
	b.until("held by the pointer", "window.timers.size === 0")
	b.eval(pageToastButton + ".focus()")
	move(5, 5)
	b.eval("new Promise((go) => setTimeout(go, 50))")
	out = append(out, b.eval(timers)) // the focus still holds it
	b.eval("document.activeElement.blur()")
	out = append(out, b.eval(timers))
	// The focus alone holds it too.
	b.eval(pageToastButton + ".focus()")
	out = append(out, b.eval(timers))
	b.eval("document.activeElement.blur()")
	// By a finger: held only while it is down, though :hover stays.
	touch := func(on bool) {
		t.Helper()
		if _, err := chromedp.Call(b.ctx, emulation.SetTouchEmulationEnabled, emulation.SetTouchEmulationEnabledParams{Enabled: on, MaxTouchPoints: 1}); err != nil {
			t.Fatal(err)
		}
	}
	touch(true)
	defer touch(false)
	x, y = at(toast)
	finger := func(kind input.DispatchTouchEventType, points ...*input.TouchPoint) {
		t.Helper()
		if _, err := chromedp.Call(b.ctx, input.DispatchTouchEvent, input.DispatchTouchEventParams{Type: kind, TouchPoints: points}); err != nil {
			t.Fatal(err)
		}
	}
	finger(input.DispatchTouchEventTypeTouchStart, &input.TouchPoint{X: x, Y: y})
	finger(input.DispatchTouchEventTypeTouchEnd)
	b.until("going again", "window.timers.size === 1")
	touch(false)
	// When its time is up it goes. The words after a press, for 5 seconds.
	b.eval("window.fire()")
	out = append(out, b.eval("document.querySelectorAll("+js(toast)+").length"))
	pressMenuItem(b, "#read-actions", "mark-unread")
	b.until("the words", "document.querySelector("+js(toast)+")")
	out = append(out, b.eval(timers), b.eval("getComputedStyle(document.querySelector("+js(toast)+")).animationName"))
	// With no button it leaves the pointer to the page under it, which one with a button takes.
	out = append(out, b.eval("getComputedStyle(document.querySelector("+js(toast)+")).pointerEvents"))
	// Of a page not shown, the words stand beside the toast of the one shown.
	openRowOf(b, "p2")
	b.until("its toast", pageToastButton)
	out = append(out, b.eval("getComputedStyle(document.querySelector("+js(toast)+")).pointerEvents"))
	pressMenuItem(b, "#list li[data-slug=p3]", "mark-unread")
	b.until("the words of another", "document.querySelectorAll('[data-testid=toast][data-timed]:not([data-page])').length === 1")
	out = append(out, b.eval("document.querySelectorAll("+js(toast+" button")+").length"))
	// In the search modal, on top and to be pressed; its focus goes back to the field.
	b.pressKey("k", kb.ModifierCtrl)
	b.until("the search", "document.getElementById('find').open")
	b.still()
	x, y = at(toast + " button")
	out = append(out, b.eval(fmt.Sprintf("document.elementFromPoint(%f, %f) === %s", x, y, pageToastButton)))
	b.eval(pageToastButton + ".focus(); " + pageToastButton + ".click()")
	b.until("the words in it", "!"+pageToastButton)
	out = append(out, b.eval("document.activeElement.id"))
	check(t, "toast", out, strings.Join([]string{
		"8000", "toast-in", "", "8000", "", "0", "5000", "toast-in", "none", "auto", "1", "true", "find-q",
	}, "|"))
}

// TestUIUnreadMenu has the item in every menu of a page read, and none of a page with news, a link or a folder.
func TestUIUnreadMenu(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := unreadSite(t)
	b := s.b
	unreadList(b)
	item := func(box string) string {
		t.Helper()
		return b.eval(inMenuItem(box, "mark-unread", "i.hidden"))
	}
	out := make([]string, 0, 10)
	out = append(out, item("#list li[data-slug=p1]"), item("#list li[data-slug=l]"))
	openRowOf(b, "p1")
	untilUnseen(b, "p1", false)
	out = append(out, item("#list li[data-slug=p1]"), item("#read-actions"), b.eval(inMenuItem("#list li[data-slug=p1]", "mark-unread", "i.getAttribute('aria-label')")))
	b.pressKey("k", kb.ModifierCtrl)
	b.until("the search", "document.getElementById('find').open && document.querySelector('#find-actions [data-testid=more]')")
	out = append(out, item("#find-actions")) // the first row, new
	b.fill("#find-q", "Title p1")
	out = append(out, item("#find-actions"))
	b.pressKey(kb.Escape)
	b.until("the search closed", "!document.getElementById('find').open")
	// Pressed in a row's menu, the row drawn anew has the focus on its menu.
	b.eval("(() => { const s = document.querySelector('#list li[data-slug=p1] summary'); s.focus(); s.click(); })()")
	b.until("the item", "document.querySelector('#list li[data-slug=p1] [data-testid=mark-unread]').checkVisibility()")
	b.eval("document.querySelector('#list li[data-slug=p1] [data-testid=mark-unread]').click()")
	untilUnseen(b, "p1", true)
	b.still()
	out = append(out, b.eval("document.activeElement.matches('#list li[data-slug=p1] summary')"), item("#list li[data-slug=p1]"), item("#read-actions"))
	// The bar of a page open whole.
	b.navigate("/a/p1/")
	b.until("the toast", pageToastButton)
	b.until("read in its bar", inMenuItem("#frame-actions", "mark-unread", "i.hidden")+" === false")
	pressMenuItem(b, "#frame-actions", "mark-unread")
	b.until("the words", "!"+pageToastButton)
	b.until("hidden in its bar", inMenuItem("#frame-actions", "mark-unread", "i.hidden"))
	check(t, "menu", out, strings.Join([]string{
		"true", "true", "false", "false", "Mark Title p1 as unread", "true", "false", "true", "true", "true",
	}, "|"))
}
