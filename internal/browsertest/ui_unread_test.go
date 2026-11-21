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
	pageToastSay = "document.querySelector('.toast.page')?.firstChild && [...document.querySelector('.toast.page').childNodes].filter((n) => n.nodeType === 3).map((n) => n.textContent).join('')"
)

func pickUnread(b *browser, slug string) {
	b.t.Helper()
	b.moving(fmt.Sprintf(unreadRow, slug) + ".querySelector('.title').click()")
}

func isUnseen(b *browser, slug string, want bool) {
	b.t.Helper()
	b.until(fmt.Sprintf("%s unseen %v", slug, want), fmt.Sprintf(unreadRow, slug)+".classList.contains('unseen') === "+fmt.Sprint(want))
}

func unreadList(b *browser) {
	b.t.Helper()
	b.size(1440, 800, false)
	b.navigate("/")
	b.until("the rows", fmt.Sprintf(unreadRow, "p1"))
}

// sentAll waits for the line of requests to be done.
func sentAll(b *browser) {
	b.t.Helper()
	b.eval("new Promise((go) => { const look = () => (readsGoing ? setTimeout(look, 10) : go()); look(); })")
}

// TestUIUnread opens a new page beside the list: read, with a toast to take it back, whose press leaves the page where it is, and
// which opening again reads; a page read, a link and a move on say nothing more.
func TestUIUnread(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := unreadSite(t)
	b := s.b
	unreadList(b)
	b.eval("document.activeElement.blur(); window.loads = 0; document.getElementById('read-frame').addEventListener('load', () => { window.loads++; })")
	out := make([]string, 0, 16)
	pickUnread(b, "p1")
	b.until("the toast", "document.querySelector('.toast.page .btn')")
	isUnseen(b, "p1", false)
	out = append(out, b.eval(pageToastSay), b.eval("document.querySelector('.toast.page .btn').textContent + ' / ' + document.querySelector('.toast.page .btn').getAttribute('aria-label')"),
		b.eval("document.getElementById('toasts').getAttribute('role') + ' ' + document.activeElement.matches('.toast *')"))
	b.until("the page in", "window.loads > 0")
	loads := b.eval("window.loads")
	// Pressed: the row new again, the count up, the page left as it is.
	before := b.eval("document.getElementById('news-n').textContent")
	b.eval("document.querySelector('.toast.page .btn').click()")
	b.until("the words", pageToastSay+" === 'Marked as unread. It is marked as read when you open it again.'")
	isUnseen(b, "p1", true)
	out = append(out, b.eval("window.sent.join(',')"), b.eval("document.querySelectorAll('.toast.page .btn').length"),
		b.eval("document.getElementById('news-n').textContent !== "+js(before)), b.eval("String(window.loads) === "+js(loads)))
	// On to the next: one toast, of that page; back: read, with the toast.
	pickUnread(b, "p2")
	b.until("the next toast", "document.querySelector('.toast.page .btn')")
	out = append(out, b.eval("document.querySelectorAll('.toast.page').length"))
	pickUnread(b, "p1")
	b.until("read again", "window.sent.filter((x) => x === 'POST p1').length === 2")
	b.until("its toast", "document.querySelector('.toast.page .btn')?.getAttribute('aria-label') === 'Mark Title p1 as unread'")
	isUnseen(b, "p1", false)
	// A page read, and a link: no toast, the one before gone.
	pickUnread(b, "p2")
	b.until("told", "window.sent.filter((x) => x === 'POST p2').length === 2")
	sentAll(b)
	out = append(out, b.eval("document.querySelectorAll('.toast.page').length"))
	pickUnread(b, "l")
	b.until("the link", "picked?.slug === 'l'")
	sentAll(b)
	out = append(out, b.eval("document.querySelectorAll('.toast.page').length + ' ' + window.sent.includes('POST l')"))
	// A page of its own: the same, and loaded anew it is read again.
	b.navigate("/a/p3/")
	b.until("the toast whole", "document.querySelector('.toast.page .btn')")
	b.eval("document.querySelector('.toast.page .btn').focus(); document.querySelector('.toast.page .btn').click()")
	b.until("the words whole", "document.querySelector('.toast.page') && !document.querySelector('.toast.page .btn')")
	out = append(out, b.eval("document.activeElement.matches('#frame-actions summary')"))
	b.navigate("/a/p3/")
	b.until("the toast again", "document.querySelector('.toast.page .btn')")
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
	pickUnread(b, last)
	b.until("the toast", "document.querySelector('.toast.page .btn')")
	b.eval("document.querySelector('.toast.page .btn').click()")
	b.until("the words", "document.querySelector('.toast.page') && !document.querySelector('.toast.page .btn')")
	isUnseen(b, last, true)
	b.eval(fmt.Sprintf(unreadRow, last) + ".querySelector('.title').focus()")
	b.pressKey("j")
	sentAll(b)
	isUnseen(b, last, true)
	check(t, "the last row kept", []string{b.eval("picked.slug === " + js(last) + " && keptUnread === " + js(last) + " && !document.querySelector('.toast.page .btn')")}, "true")
	pickUnread(b, last)
	b.until("whole", "location.pathname === "+js("/a/"+last+"/"))
	b.until("its toast whole", "document.querySelector('.toast.page .btn')")
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
func TestUIUnreadKept(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := unreadSite(t)
	b := s.b
	unreadList(b)
	anew := func() {
		t.Helper()
		// Heard here after the frame's own listener, which has by then sent it or kept it back.
		b.eval(`window.heard = false; addEventListener('message', (e) => { if (e.data?.pageServed) window.heard = true; }, { once: false });
			document.getElementById('read-frame').contentWindow.location.replace(readLoaded)`)
		b.until("told anew", "window.heard")
		sentAll(b)
	}
	posts := func(slug string) string {
		t.Helper()
		return b.eval("window.sent.filter((x) => x === 'POST " + slug + "').length")
	}
	out := make([]string, 0, 16)
	// Marked unread and drawn anew: unread, told nothing. Another page marked from its row does not take that away, nor its failure.
	pickUnread(b, "p1")
	b.until("the toast", "document.querySelector('.toast.page .btn')")
	b.eval("document.querySelector('.toast.page .btn').click()")
	isUnseen(b, "p1", true)
	anew()
	out = append(out, posts("p1"))
	b.eval("markUnread('p2')")
	sentAll(b)
	anew()
	out = append(out, posts("p1"))
	b.eval("window.plan.DELETE = [500]; markUnread('p3')")
	sentAll(b)
	out = append(out, b.eval("(document.getElementById('error').textContent === 'Failed to mark as unread: HTTP 500: no') + ' ' + keptUnread"))
	anew()
	out = append(out, posts("p1"))
	isUnseen(b, "p1", true)
	// Pressed twice, the first failing: the error goes with the next that goes well, and it is still held.
	b.eval("window.plan.DELETE = [500]; window.shut(); markUnread('p1'); markUnread('p1')")
	b.eval("window.open_()")
	sentAll(b)
	out = append(out, b.eval("keptUnread + ' ' + (document.getElementById('error').textContent === '')"))
	// A failure of its own lets it go.
	b.eval("window.plan.DELETE = ['throw']; markUnread('p1')")
	sentAll(b)
	out = append(out, b.eval("String(keptUnread) + ' ' + document.getElementById('error').textContent.includes('cut off')"))
	// Pressed while the read is on its way: no words of being read.
	unreadList(b)
	b.eval("window.toasted = []; { const f = toast; toast = (m, o) => { window.toasted.push(m); f(m, o); }; }")
	b.eval("window.shut()")
	pickUnread(b, "p3")
	b.until("the frame told", "readsGoing || readsWaiting.size > 0")
	b.eval("markUnread('p3'); window.open_()")
	sentAll(b)
	isUnseen(b, "p3", true)
	out = append(out, b.eval("window.toasted.join(',')"))
	// The answer to a read comes after a move on: no toast of the page left.
	unreadList(b)
	b.eval("window.toasted = []; { const f = toast; toast = (m, o) => { window.toasted.push(m); f(m, o); }; }; window.shut()")
	pickUnread(b, "p1")
	b.until("told", "readsGoing")
	pickUnread(b, "l")
	b.eval("window.open_()")
	sentAll(b)
	out = append(out, b.eval("window.toasted.length + ' ' + window.sent.at(-1)"))
	b.eval("window.toasted.length = 0")
	// A page marked from its row, the mark waiting behind another, then opened: the read takes its place, with nothing said of it.
	const said = "window.toasted.filter((m) => m === 'Marked as read').length + ' ' + (document.getElementById('error').textContent === '')"
	for _, slug := range []string{"p2", "p1"} {
		was := posts(slug)
		pickUnread(b, slug)
		b.until("told of "+slug, "window.sent.filter((x) => x === 'POST "+slug+"').length > "+was)
		sentAll(b)
	}
	b.eval("window.toasted.length = 0; window.shut(); markUnread('p3'); markUnread('p2')")
	pickUnread(b, "p2")
	b.until("the read in its place", "readsWaiting.get('p2')?.read")
	b.eval("window.open_()")
	sentAll(b)
	out = append(out, b.eval("window.sent.slice(-2).join(',')"), b.eval(said))
	// The page shown, the mark waiting, away and back: the same, as it was opened last.
	sent := b.eval("window.sent.length")
	b.eval("window.toasted.length = 0; window.shut(); markUnread('p3'); markUnread('p2')")
	pickUnread(b, "p1")
	pickUnread(b, "p2")
	b.until("the read in its place", "readsWaiting.get('p2')?.read")
	b.eval("window.open_()")
	sentAll(b)
	out = append(out, b.eval("window.sent.slice("+sent+").filter((x) => x.endsWith('p2')).join(',')"), b.eval(said))
	check(t, "kept", out, strings.Join([]string{
		"1", "1", "true p1", "1", "p1 true", "null true", "Marked as unread. It is marked as read when you open it again.",
		"0 POST p1", "DELETE p3,POST p2", "0 true", "POST p2", "0 true",
	}, "|"))
}

// TestUIUnreadPublish reads a page open as a publish draws it anew, saying nothing; marked unread after, the version before counts.
func TestUIUnreadPublish(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := unreadSite(t)
	b := s.b
	unreadList(b)
	pickUnread(b, "p1")
	b.until("the toast", "document.querySelector('.toast.page .btn')")
	b.eval("document.querySelector('.toast.page').remove()")
	s.floe(t, "publish", s.page(t, "unread/p1.yaml", "Title p1", "<p id=\"p\">p1, more</p>"), "--slug", "p1")
	b.until("read as it drew itself", "window.sent.filter((x) => x === 'POST p1').length === 2")
	sentAll(b)
	isUnseen(b, "p1", false)
	out := make([]string, 0, 2)
	out = append(out, b.eval("document.querySelectorAll('.toast.page').length"))
	b.eval("markUnread('p1')")
	isUnseen(b, "p1", true)
	out = append(out, b.eval(fmt.Sprintf(unreadRow, "p1")+".querySelector('.ver').textContent"))
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
		return b.eval("document.querySelector(" + js(box+" [data-testid=mark-unread]") + ").hidden")
	}
	out := make([]string, 0, 10)
	out = append(out, item("#list li[data-slug=p1]"), item("#list li[data-slug=l]"))
	openRowOf(b, "p1")
	untilUnseen(b, "p1", false)
	out = append(out, item("#list li[data-slug=p1]"), item("#read-actions"), b.eval("document.querySelector('#list li[data-slug=p1] [data-testid=mark-unread]').getAttribute('aria-label')"))
	b.pressKey("k", kb.ModifierCtrl)
	b.until("the search", "document.getElementById('find').open && document.querySelector('#find-actions [data-testid=mark-unread]')")
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
	b.until("read in its bar", "document.querySelector('#frame-actions [data-testid=mark-unread]')?.hidden === false")
	pressMenuItem(b, "#frame-actions", "mark-unread")
	b.until("the words", "!"+pageToastButton)
	b.until("hidden in its bar", "document.querySelector('#frame-actions [data-testid=mark-unread]').hidden")
	check(t, "menu", out, strings.Join([]string{
		"true", "true", "false", "false", "Mark Title p1 as unread", "true", "false", "true", "true", "true",
	}, "|"))
}
