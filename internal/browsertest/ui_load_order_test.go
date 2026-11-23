package browsertest_test

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"floe/internal/i18n"
	"floe/internal/store"
)

// loadOrderJS is the test's own fetch over the page's, in window.lo. lo.hold(kind, fail, edit) holds the next read of the list
// ('read'), write ('write') or folder's directory ('folder') until lo.h[kind].release(): served once the server answered, fail
// answers 500 (404 a directory) instead, edit changes the list answered. The flags fail or change each request of their kind; the
// marks' bodies go to lo.marks, a test's mark (at 'x') answered 404 as of a page the server does not know. lo.log holds, at each
// read's end, its number, the reads of the list sent so far, the rows, the mistake said and whether lo.folders are open: what a read
// drew, before the read an action makes after its write draws over it. lo.errors holds each mistake said in turn (lo.error(), ” for
// none) from what it is set to. lo.bad names an entry answered with tags the page cannot draw.
const loadOrderJS = `(() => {
	const real = window.fetch, html = document.documentElement;
	const lo = window.lo = { gets: 0, limited: null, filesLimited: null, hang: false, slowMarks: false, answered: 0, heard: 0, next: {}, h: {},
		marks: [], log: [], errors: [], bad: '', listBroken: false, tasksBroken: false, markBroken: false, failWrite: false, folders: [] };
	const error = lo.error = () => (document.getElementById('error').hidden ? '' : document.getElementById('error').textContent);
	lo.hold = (kind, fail = false, edit = null) => {
		const h = lo.h[kind] = lo.next[kind] = { fail, edit, taken: false, served: false };
		h.held = new Promise((go) => { h.release = go; });
	};
	const take = (kind) => {
		const h = lo.next[kind];
		delete lo.next[kind];
		if (h) h.taken = true;
		return h;
	};
	const later = (h, res) => {
		res.then(() => { h.served = true; });
		return res.then((r) => h.held.then(() => r));
	};
	const edited = (res, edit) => res.then(async (r) => (r.ok ? new Response(JSON.stringify(edit(await r.json()))) : r));
	window.fetch = (u, i) => {
		const path = new URL(String(u), location.href).pathname, method = i?.method ?? 'GET';
		if (path === '/api/pages' && method === 'GET') {
			lo.gets++;
			lo.limited = Boolean(i?.signal);
			if (lo.listBroken) return Promise.resolve(new Response('', { status: 500 }));
			if (lo.hang) {
				lo.hang = false;
				return new Promise((_, no) => i?.signal?.addEventListener('abort', () => no(i.signal.reason)));
			}
			let res = real(u, i);
			if (lo.bad) res = edited(res, (list) => list.map((a) => (a.slug === lo.bad ? { ...a, tags: { length: 1 } } : a)));
			const h = take('read');
			if (!h) return res;
			if (h.fail) res = Promise.resolve(new Response('broken', { status: 500 }));
			else if (h.edit) res = edited(res, h.edit);
			return later(h, res);
		}
		if (path === '/api/tasks' && method === 'GET' && lo.tasksBroken) return Promise.resolve(new Response('tasks broken', { status: 500 }));
		if (path.endsWith('/files') && method === 'GET') {
			lo.filesLimited = Boolean(i?.signal);
			const h = take('folder');
			if (h) return later(h, h.fail ? Promise.resolve(new Response('', { status: 404 })) : real(u, i));
		}
		if (method === 'GET') return real(u, i);
		if (path.endsWith('/read')) {
			if (i.body) lo.marks.push(i.body);
			let res = lo.markBroken ? Promise.resolve(new Response('marks broken', { status: 500 }))
				: i.body && JSON.parse(i.body).at === 'x' ? Promise.resolve(new Response('', { status: 404 })) : real(u, i);
			if (lo.slowMarks) res = res.then((r) => new Promise((go) => setTimeout(() => go(r), 20)));
			return res;
		}
		const done = (r) => {
			lo.answered++;
			return r;
		};
		if (lo.failWrite) {
			lo.failWrite = false;
			return Promise.resolve(new Response('refused', { status: 500 })).then(done);
		}
		const h = take('write');
		return (h ? h.held.then(() => real(u, i)) : real(u, i)).then(done);
	};
	new MutationObserver(() => lo.log.push({
		n: Number(html.dataset.loads), gets: lo.gets, error: error(),
		rows: [...document.querySelectorAll('#list li[data-slug]:not([data-d])')].map((li) => li.dataset.slug),
		open: lo.folders.map((slug) => document.querySelector('#list li[data-slug="' + slug + '"]:not([data-d]) [data-testid=title]')?.getAttribute('aria-expanded')),
	})).observe(html, { attributes: true, attributeFilter: ['data-loads'] });
	new MutationObserver(() => { if (lo.errors.at(-1) !== error()) lo.errors.push(error()); })
		.observe(document.getElementById('error'), { attributes: true, childList: true, characterData: true, subtree: true });
	addEventListener('message', (e) => { if (e.data?.pageServed) lo.heard++; });
})()`

// markingPage is a page that tells the list it was read, as floe's own script does, on the test's word: one mark ({ mark: { n, at } }),
// a mark every millisecond ({ flood: true }) up to { stop: true }.
const markingPage = `<script>
let flood = null;
const post = (read) => parent.postMessage({ pageServed: read, pagePath: location.pathname }, '*');
addEventListener('message', (e) => {
  if (e.data?.mark) post(e.data.mark);
  if (e.data?.flood) flood = setInterval(() => post({ n: 1, at: 'x' }), 1);
  if (e.data?.stop) clearInterval(flood);
});
</script>`

// The list is read one load at a time, with no write going or sent meanwhile: an answer of the time before a write is never drawn,
// a read waits for every write sent before it and draws the list of the time after them, and a failed read is told all the same.
func TestUILoadOrder(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	st := store.New(s.Env["FLOE_DIR"])
	msg := i18n.New(i18n.English).Msg
	for _, slug := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		s.floe(t, "publish", s.page(t, slug+".yaml", "T "+slug, ""), "--slug", slug)
	}
	s.floe(t, "publish", s.page(t, "kept.yaml", "T kept", markingPage), "--slug", "kept")
	s.floe(t, "publish", "--link", filepath.Dir(s.write(t, "more/m.html", "<title>m</title>")), "--slug", "more")
	two := filepath.Dir(s.write(t, "two/t.html", "<title>t</title>"))
	yes := true
	archive := func(slug string) {
		t.Helper()
		if _, err := st.Edit(slug, store.Edit{Archived: &yes}); err != nil {
			t.Fatal(err)
		}
	}
	// Wide, so that a row picked opens beside the list
	b.size(1400, 900, false)
	defer b.unsize()
	b.navigate("/")
	b.until("the rows", "document.querySelectorAll('#list li[data-slug]').length === 10")
	b.eval(loadOrderJS)
	b.eval(readMarks)
	b.eval("lo.folders = ['more', 'two']; true")

	var out []string
	say := func(what, expr string) {
		t.Helper()
		out = append(out, what+": "+b.eval(expr))
	}
	row := func(slug string) string {
		return "document.querySelector('#list li[data-slug=" + slug + "]:not([data-d])')"
	}
	// The row's own button of testid (archive, star), pressed
	press := func(slug, testid string) string {
		return row(slug) + ".querySelector('[data-testid=actions] > [data-testid=" + testid + "]').click()"
	}
	const poll = "document.dispatchEvent(new Event('visibilitychange'))"
	served := func(kind string) {
		t.Helper()
		b.until("the "+kind+" held to be answered", "lo.h."+kind+"?.served")
	}
	// A write pressed, and its answer waited for
	written := func(js string) {
		t.Helper()
		b.eval("lo.answered = 0; " + js)
		b.until("the write answered", "lo.answered === 1")
	}
	// expr of what the read n drew (e), once it ended
	at := func(n int, expr string) string {
		t.Helper()
		b.until("the read "+strconv.Itoa(n)+" to end", "lo.log.some((e) => e.n === "+strconv.Itoa(n)+")")
		return b.eval("(() => { const e = lo.log.find((x) => x.n === " + strconv.Itoa(n) + "); return " + expr + "; })()")
	}
	// The reads of the list sent by now and a moment later, with nothing let go between
	const getsLater = "new Promise((go) => setTimeout(() => go(lo.gets), 100))"
	toFrame := func(data string) {
		t.Helper()
		b.eval("document.getElementById('read-frame').contentWindow.postMessage(" + data + ", '*')")
	}

	// Two reads: the second reads after the first, which cannot draw over it. a changes outside (as by the CLI) between them
	begun := b.acted("lo.hold('read'); lo.gets = 0; " + poll)
	served("read")
	archive("a")
	b.reread()
	say("reads before the first answered", getsLater)
	b.eval("lo.h.read.release()")
	out = append(out, "a there: "+at(begun+2, "e.rows.includes('a')"))

	// A write going: the read waits for its answer, and draws what it wrote
	begun = b.acted("lo.hold('write'); lo.gets = 0; " + press("b", "archive") + "; " + poll)
	say("reads while the write goes", getsLater)
	b.eval("lo.h.write.release()")
	out = append(out, "b there: "+at(begun+1, "e.rows.includes('b')"))

	// A write sent while the list is read: read again
	begun = b.acted("lo.hold('read'); lo.gets = 0; " + poll)
	served("read")
	written(press("c", "archive"))
	b.eval("lo.h.read.release()")
	out = append(out, "read again: "+at(begun+1, "(e.gets > 1) + ', c there: ' + e.rows.includes('c')"))

	// A read failing while a write is sent: told all the same
	begun = b.acted("lo.hold('read', true); " + poll)
	served("read")
	written(press("d", "archive"))
	b.eval("lo.h.read.release()")
	out = append(out, "failure told: "+at(begun+1, "e.error === "+js(msg("ui_load_failed", "HTTP 500"))))

	// A folder opened, and a write sent while the list is read, its answer of the time before the folder: it closes nothing
	b.eval(row("more") + ".querySelector('[data-testid=title]').click()")
	b.until("the folder open", "document.querySelector('#list li[data-d][data-slug=more]')")
	begun = b.acted("lo.hold('read', false, (list) => list.filter((a) => a.slug !== 'more')); " + poll)
	served("read")
	written(press("h", "star"))
	b.eval("lo.h.read.release()")
	out = append(out, "folder open: "+at(begun+1, "e.open[0] === 'true'"))

	// A write sent while the folders open are read: the list is read again
	begun = b.acted("lo.hold('folder'); " + poll)
	served("folder")
	written(press("more", "archive"))
	b.eval("lo.h.folder.release()")
	out = append(out, "folder there: "+at(begun+1, "e.rows.includes('more')"))

	// A poll held back by a press, and an action's read joined to the one waiting: that one draws all the same. e changes outside.
	// The read held is let go once the star's toast tells its read was asked
	begun = b.acted("lo.hold('read'); " + poll)
	served("read")
	archive("e")
	b.pressList(poll + "; " + press("g", "star") + `;
		(async () => {
			while (![...document.querySelectorAll('[data-testid=toast]')].some((x) => x.textContent.includes('T g'))) await new Promise((go) => setTimeout(go, 5));
			lo.h.read.release();
		})()`)
	out = append(out, "joined, e there: "+at(begun+2, "e.rows.includes('e')"))
	b.letGoList()

	// A draw that throws, as of an entry the page cannot draw: said, not thrown, the next read goes on, and the list drawn again once
	// it can be. The error's words are the browser's: only the page's words around them are compared
	// LIMIT: the same list drawn again after a throw is not told from one that changed: the same data throws again
	// The list is the React screen's, which says a draw that throws as the screen's (S422-P9)
	draw := strings.SplitN(msg("ui_screen_draw_failed", "\x00"), "\x00", 2)
	thrown := b.eval("window.thrown.length")
	begun = b.acted("lo.bad = 'h'; " + poll)
	b.untilRead(begun, "the draw that throws", "true")
	toldDraw := b.eval("(() => { const e = document.getElementById('error'); return !e.hidden && e.textContent.startsWith(" + js(draw[0]) + ") && e.textContent.endsWith(" + js(draw[1]) + "); })()")
	b.untilRead(b.reread(), "the next read", "true")
	begun = b.acted("lo.bad = ''; " + poll)
	b.untilRead(begun, "the list drawn again", "true")
	say("thrown", "(window.thrown.length === "+thrown+" ? 'none' : window.thrown.slice("+thrown+").join()) + ', said: "+toldDraw+", the next: went on, drawn again: ' + Boolean("+row("h")+") + ', cleared: ' + document.getElementById('error').hidden")
	say("read with a time limit", "lo.limited")

	// A read that never answers: cut off at the page's limit, 10 s, the read fails and says so, and the next goes on
	shownBefore := b.eval("!document.getElementById('error').hidden")
	begun = b.acted("lo.hang = true; " + poll)
	b.untilWithin(20*time.Second, "the read cut off", "lo.log.some((e) => e.n === "+strconv.Itoa(begun+1)+")")
	said := at(begun+1, "e.error === "+js(msg("ui_load_slow", 10, 3)))
	failed := at(begun+1, "e.error !== ''")
	b.untilRead(b.reread(), "the next read", "true")
	out = append(out, "cut off: returned, error: "+shownBefore+" then "+failed+", the next: returned, said why: "+said)

	// Two marks of the page beside the list told while the list is read: only the last is sent. The page's own first mark is done first
	b.eval(row("kept") + ".querySelector('[data-testid=title]').click()")
	b.marksDone(1)
	b.eval("lo.marks.length = 0")
	b.acted("lo.hold('read'); " + poll)
	served("read")
	heard := b.eval("lo.heard")
	toFrame("{ mark: { n: 1, at: 'x' } }")
	toFrame("{ mark: { n: 2, at: 'x' } }")
	b.until("the two marks told", "lo.heard === "+heard+" + 2")
	b.eval("lo.h.read.release()")
	b.marksDone(2)
	say("marks sent", "lo.marks.join()")

	// A page that keeps telling its read, each mark slow enough that the next is always waiting: the marks wait for a read of the
	// list, which draws
	b.eval("lo.slowMarks = true")
	heard = b.eval("lo.heard")
	toFrame("{ flood: true }")
	b.until("the marks told", "lo.heard > "+heard+" + 5")
	b.untilWithin(3*time.Second, "a read under read marks", "Number(document.documentElement.dataset.loads ?? 0) > "+strconv.Itoa(b.reread()))
	out = append(out, "under read marks: drawn")
	toFrame("{ stop: true }")
	b.eval("lo.slowMarks = false")
	b.until("the marks done", "window.marks.every((m) => m.done)")
	b.untilRead(b.reread(), "a read after the marks", "true")

	// A write sent by what runs on another write's answer: two archived in turn by one press, the first held, and a read asked
	// meanwhile draws them both gone
	// LIMIT: the read waits while (not if) a write goes after the one it waited for, which ran on that one's answer before the read
	// went on: no press of the screen comes between the two, and review holds it
	b.eval(row("f") + ".querySelector('[data-testid=sel]').click(); " + row("g") + ".querySelector('[data-testid=sel]').click()")
	b.until("the bulk bar", "!document.getElementById('bulk').hidden")
	begun = b.acted("lo.hold('write'); document.getElementById('bulk-archive').click(); " + poll)
	b.until("the first write held", "lo.h.write.taken")
	b.eval("lo.h.write.release()")
	out = append(out, "f and g there: "+at(begun+1, "e.rows.includes('f') + ' ' + e.rows.includes('g')"))

	// A folder's read failing in the first round, a write meanwhile, and a press in the second round: nothing of the first round is kept,
	// the folder open as the next read draws it
	s.floe(t, "publish", "--link", two, "--slug", "two")
	b.untilRead(b.reread(), "the folder two", row("two"))
	b.eval(row("two") + ".querySelector('[data-testid=title]').click()")
	b.until("the folder two open", "document.querySelector('#list li[data-d][data-slug=two]')")
	b.untilRead(b.reread(), "a read with it open", "true")
	begun = b.acted("lo.hold('folder', true); " + poll)
	served("folder")
	written(press("h", "archive"))
	b.pressList("lo.h.folder.release()")
	out = append(out, "held in the second round, folder open: "+at(begun+2, "e.open[1]"))
	b.letGoList()

	// A read outside a load (a folder opened by hand, versions, a diff) has no time limit. Its fetch is called as the folder is pressed,
	// before a poll's read of the folder can come between
	say("a read outside a load limited", "(() => { const title = "+row("two")+".querySelector('[data-testid=title]'); title.click(); lo.filesLimited = null; title.click(); return lo.filesLimited; })()")
	b.until("the folder two open again", "document.querySelector('#list li[data-d][data-slug=two]')")

	check(t, "load order", out, strings.Join([]string{
		"reads before the first answered: 1", "a there: false",
		"reads while the write goes: 0", "b there: false",
		"read again: true, c there: false",
		"failure told: true",
		"folder open: true",
		"folder there: false",
		"joined, e there: false",
		"thrown: none, said: true, the next: went on, drawn again: true, cleared: true",
		"read with a time limit: true",
		"cut off: returned, error: false then true, the next: returned, said why: true",
		`marks sent: {"n":2,"at":"x"}`,
		"under read marks: drawn",
		"f and g there: false false",
		"held in the second round, folder open: true",
		"a read outside a load limited: false",
	}, "|"))

	loadMistakes(t, s, st, msg)
}

// loadMistakes: the load's mistakes are said in one place, the read's before the draw's before the tasks', each taken back when it
// comes right; a load's never covers another's, nor clears it. On a task's page, which reads the tasks, then the marks on the list.
func loadMistakes(t *testing.T, s *site, st *store.Store, msg func(string, ...any) string) {
	t.Helper()

	b := s.b
	project := "proj"
	if _, err := st.Edit("kept", store.Edit{Project: &project}); err != nil {
		t.Fatal(err)
	}
	s.floe(t, "task", "--slug", "kept", "--task", "T1", "--category", "c")
	b.navigate("/tasks?project=proj&task=T1")
	b.until("the task's rows", "document.querySelector('#list li[data-slug=kept]')")
	b.eval(loadOrderJS)

	var out []string
	const shown = "(document.getElementById('error').hidden ? 'none' : document.getElementById('error').textContent)"
	say := func(what, want string) {
		t.Helper()
		got := b.eval(shown)
		if got == want {
			got = "right"
		}
		out = append(out, what+": "+got)
	}
	const poll = "document.dispatchEvent(new Event('visibilitychange'))"
	// A poll's read after set, ended
	step := func(set string) {
		t.Helper()
		b.untilRead(b.acted(set+"; "+poll), "the read", "true")
	}
	// The task page is the React screen's, which says a draw that throws as the screen's (S422-P8)
	parts := strings.SplitN(msg("ui_screen_draw_failed", "\x00"), "\x00", 2)
	sayDraw := func(what string) {
		t.Helper()
		got := b.eval(shown)
		if strings.HasPrefix(got, parts[0]) && strings.HasSuffix(got, parts[1]) {
			got = "right"
		}
		out = append(out, what+": "+got)
	}
	read := msg("ui_load_failed", "HTTP 500")
	tasks := msg("ui_tasks_failed", "tasks broken")
	failedStar := msg("ui_favorite_failed", msg("ui_http_error", "refused", 500))
	const star = "document.querySelector('#list li[data-slug=kept] [data-testid=actions] > [data-testid=star]').click()"
	failStar := func() {
		t.Helper()
		b.eval("lo.failWrite = true; " + star)
		b.until("the star's mistake", shown+" === "+js(failedStar))
	}

	// kept beside the list, whose tag line is drawn with the tasks unread too, where its row is not
	b.eval("document.querySelector('#list li[data-slug=kept] [data-testid=title]').click()")
	b.until("kept beside the list", "document.getElementById('read-frame').dataset.slug === 'kept'")
	// The draw's, all three at once
	step("lo.bad = 'kept'; lo.tasksBroken = true")
	sayDraw("the draw")
	// The bar is in the screen the draw took, rows and all: drawn again by a good answer
	const drawn = "Boolean(document.getElementById('find-open')) + ' ' + Boolean(document.querySelector('#list li[data-slug=kept]'))"
	out = append(out, "the draw, bar and rows: "+b.eval(drawn))
	// The read's over it; when the read comes right, the draw's again, then the tasks', then none
	step("lo.listBroken = true")
	say("the read", read)
	step("lo.listBroken = false")
	sayDraw("the draw after the read")
	step("lo.bad = ''")
	say("the tasks after the draw", tasks)
	step("lo.tasksBroken = false")
	out = append(out, "drawn again, bar and rows: "+b.until("drawn again", "document.getElementById('find-open') && "+drawn))
	say("then", "none")

	// A read held back by a press takes back the read's mistake
	step("lo.listBroken = true")
	b.eval("lo.listBroken = false")
	b.pressList(poll)
	say("held back", "none")
	b.letGoList()

	// An action's mistake stays through a load that says none, and a load's (the read's: with the tasks unread, the task's page has no
	// rows to press) does not cover it
	failStar()
	step("")
	say("kept", failedStar)
	step("lo.listBroken = true")
	say("not covered", failedStar)
	// An action that comes right clears it, and the load's is said
	b.eval(star)
	b.until("the star's toast", "[...document.querySelectorAll('[data-testid=toast]')].some((x) => x.textContent === "+js(msg("ui_favorited", "T kept"))+")")
	b.untilRead(b.reread(), "the read after the star", "true")
	say("said once clear", read)
	// An action that comes right clears its own only: the load's stays, through the action and the read after it
	b.eval("lo.errors = [lo.error()]; lo.answered = 0; " + star)
	b.until("the star's write answered", "lo.answered === 1")
	b.untilRead(b.reread(), "the read after the star", "true")
	out = append(out, "an action clears its own only: "+b.eval("lo.errors.length === 1 && lo.errors[0] === "+js(read)+" ? 'right' : lo.errors.join(' > ')"))
	step("lo.listBroken = false")

	// A read mark's mistake covers the load's (the read's, on the list, of the page beside it), and the load's is said again by the read
	// after the mark. Over none, it stays through a load that says none, and is taken back when a mark comes right (one of a page the
	// server does not know, 404)
	b.navigate("/")
	b.until("the rows", "document.querySelector('#list li[data-slug=kept]')")
	b.eval(loadOrderJS)
	b.eval(readMarks)
	b.eval("document.querySelector('#list li[data-slug=kept] [data-testid=title]').click()")
	b.marksDone(1)
	mark := func(n int) {
		t.Helper()
		b.eval("document.getElementById('read-frame').contentWindow.postMessage({ mark: { n: 1, at: 'x' } }, '*')")
		b.marksDone(n)
	}
	marked := msg("ui_read_failed", msg("ui_http_error", "marks broken", 500))
	// The mistakes said in turn since lo.errors was set
	sayTurns := func(what string, want ...string) {
		t.Helper()
		got := b.eval("lo.errors.join(' > ')")
		if got == strings.Join(want, " > ") {
			got = "right"
		}
		out = append(out, what+": "+got)
	}
	step("lo.listBroken = true")
	b.eval("lo.errors = [lo.error()]; lo.markBroken = true")
	mark(2)
	sayTurns("a mark over the load, the load again", read, marked, read)
	step("lo.listBroken = false")
	b.eval("lo.errors = [lo.error()]")
	mark(3)
	b.untilRead(b.reread(), "a read that says none", "true")
	b.eval("lo.markBroken = false")
	mark(4)
	sayTurns("a mark through a load, the mark taken back", "", marked, "")

	// A mark neither covers an action's mistake nor clears it
	failStar()
	b.eval("lo.markBroken = true")
	mark(5)
	say("a mark under an action", failedStar)
	b.eval("lo.markBroken = false")
	mark(6)
	say("a mark clears not an action", failedStar)

	check(t, "load mistakes", []string{strings.Join(out, ", ")}, "the draw: right, the draw, bar and rows: false false, the read: right, the draw after the read: right, the tasks after the draw: right, drawn again, bar and rows: true true, then: right, held back: right, kept: right, not covered: right, said once clear: right, an action clears its own only: right, a mark over the load, the load again: right, a mark through a load, the mark taken back: right, a mark under an action: right, a mark clears not an action: right")
}
