package browsertest_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// The list is read one load at a time, with no write going or sent meanwhile: an answer of the time before a write is never drawn,
// await load() returns with the list of the time after every write sent before it, and a failed read is told all the same.
func TestUILoadOrder(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	for _, slug := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		s.floe(t, "publish", s.page(t, slug+".yaml", "T "+slug, ""), "--slug", slug)
	}
	more := filepath.Dir(s.write(t, "more/m.html", "<title>m</title>"))
	two := filepath.Dir(s.write(t, "two/t.html", "<title>t</title>"))
	b.navigate("/")
	b.until("the rows", "document.querySelectorAll('#list li[data-slug]').length === 8")
	got := b.eval(`(async () => {
		const real = window.fetch;
		// The next read of the list (or of a write) waits for its release: served tells the server answered, fail answers 500
		const marks = [];
		let pagesLimited = null, hang = false, slowMarks = false, limited = false, gets = 0, holdRead = null, holdWrite = null, holdFolder = null;
		const hold = (fail = false) => {
			const h = { fail };
			h.held = new Promise((r) => { h.release = r; });
			h.served = new Promise((r) => { h.serve = r; });
			return h;
		};
		window.fetch = (u, i) => {
			if (String(u) === '/api/artifacts' && !i?.method) {
				gets++;
				limited = Boolean(i?.signal);
				if (hang) {
					hang = false;
					return new Promise((_, no) => i?.signal?.addEventListener('abort', () => no(i.signal.reason)));
				}
				const h = holdRead;
				holdRead = null;
				if (h) {
					const res = h.fail ? Promise.resolve(new Response('broken', { status: 500 })) : real(u, i);
					res.then(h.serve);
					return res.then((r) => h.held.then(() => r));
				}
			}
			if (String(u).includes('/pages?')) pagesLimited = Boolean(i?.signal);
			if (String(u).includes('/pages?') && holdFolder) {
				const h = holdFolder;
				holdFolder = null;
				const res = h.fail ? Promise.resolve(new Response('', { status: 404 })) : real(u, i);
				res.then(h.serve);
				return res.then((r) => h.held.then(() => r));
			}
			if (String(u) === '/api/artifacts/kept/read') marks.push(i.body);
			if (slowMarks && i?.method === 'POST' && String(u).endsWith('/read')) return real(u, i).then((r) => new Promise((go) => setTimeout(() => go(r), 20)));
			if (i?.method === 'PATCH' && holdWrite) {
				const h = holdWrite;
				holdWrite = null;
				return h.held.then(() => real(u, i));
			}
			return real(u, i);
		};
		const archive = { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: '{"archived":true}' };
		const has = (slug) => Boolean(document.querySelector('#list li[data-slug=' + slug + ']'));
		const tick = () => new Promise((r) => setTimeout(r, 100));
		// A read held that never comes (a step before went wrong) fails the test rather than hold it
		const served = (h) => Promise.race([h.served, new Promise((_, no) => setTimeout(() => no(new Error('the read held never came')), 3000))]);
		const out = [];

		// Two loads: the second reads after the first, which cannot draw over it. a changes outside (as by the CLI) between them
		let h = holdRead = hold();
		gets = 0;
		const l1 = load();
		await served(h);
		await real('/api/artifacts/a', archive);
		const l2 = load();
		await tick();
		out.push('reads before the first answered: ' + gets);
		h.release();
		await l1;
		await l2;
		out.push('a there: ' + has('a'));

		// A write going: the load reads after its answer, and returns with what it wrote
		h = holdWrite = hold();
		gets = 0;
		const w = api('/api/artifacts/b', archive);
		const l3 = load(true);
		await tick();
		out.push('reads while the write goes: ' + gets);
		h.release();
		await w;
		await l3;
		out.push('b there: ' + has('b'));

		// A write sent while the list is read: read again
		h = holdRead = hold();
		gets = 0;
		const l4 = load(true);
		await served(h);
		await api('/api/artifacts/c', archive);
		h.release();
		await l4;
		out.push('read again: ' + (gets > 1) + ', c there: ' + has('c'));

		// A read failing while a write is sent: told all the same
		h = holdRead = hold(true);
		const l5 = load();
		await served(h);
		await api('/api/artifacts/d', archive);
		h.release();
		await l5;
		out.push('failure told: ' + !document.getElementById('error').hidden);

		// A folder added and opened while the list is read: the list of the time before, which has no such folder, closes nothing
		h = holdRead = hold();
		const l6 = load(true);
		await served(h);
		const added = await api('/api/artifacts', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ path: ` + js(more) + ` }) });
		const { slug } = await added.json();
		opened.add(slug);
		h.release();
		await l6;
		out.push('folder open: ' + opened.has(slug));

		// A write sent while the folders open are read: the list is read again
		h = holdFolder = hold();
		const l7 = load(true);
		await served(h);
		await api('/api/artifacts/' + slug, archive);
		h.release();
		await l7;
		out.push('folder there: ' + has(slug));

		// A poll held back by a press, and an action's load joined to the one waiting: that one draws all the same. e changes outside
		h = holdRead = hold();
		const l8 = load(true);
		await served(h);
		pressedAt = performance.now();
		await real('/api/artifacts/e', archive);
		const joined = [load(true), load()];
		h.release();
		await l8;
		await Promise.all(joined);
		pressedAt = -Infinity;
		out.push('joined, e there: ' + has('e'));

		// A draw that throws: the error goes to its caller and is said, and the next load draws the same list again. No poll meanwhile, so
		// that the loads drawn are the step's own
		const realLoad = load;
		load = (polled) => (polled ? Promise.resolve() : realLoad(polled));
		await realLoad();
		const realRender = render;
		let renders = 0;
		render = () => {
			renders++;
			if (renders === 1) throw new Error('broken draw');
			realRender();
		};
		drawn = '';
		const thrown = await load().then(() => 'none', (e) => e.message);
		const toldDraw = document.getElementById('error').textContent === t('ui_draw_failed', 'broken draw');
		const after = await Promise.race([load().then(() => 'went on', () => 'stopped'), new Promise((r) => setTimeout(() => r('stuck'), 2000))]);
		render = realRender;
		load = realLoad;
		out.push('thrown: ' + thrown + ', said: ' + toldDraw + ', the next: ' + after + ', drawn again: ' + (renders === 2) + ', cleared: ' + document.getElementById('error').hidden);
		out.push('read with a time limit: ' + limited);

		// A read that never answers: cut off, the load fails and says so, and the next goes on
		const shownBefore = !document.getElementById('error').hidden;
		READ_MS = 300;
		hang = true;
		const cut = await Promise.race([load().then(() => 'returned'), new Promise((r) => setTimeout(() => r('stuck'), 3000))]);
		const said = document.getElementById('error').textContent === t('ui_load_slow', READ_MS / 1000, POLL_MS / 1000);
		READ_MS = 10000;
		const failed = !document.getElementById('error').hidden;
		const next = await Promise.race([load().then(() => 'returned'), new Promise((r) => setTimeout(() => r('stuck'), 3000))]);
		out.push('cut off: ' + cut + ', error: ' + shownBefore + ' then ' + failed + ', the next: ' + next + ', said why: ' + said);

		// A page that keeps telling its read, each mark slow enough that the next is always waiting: the marks wait for a load reading,
		// which draws
		slowMarks = true;
		const flood = setInterval(() => tellRead('gone', { read: { n: 1, at: 'x' }, say: false }), 1);
		await tick();
		const drawnUnder = await Promise.race([load().then(() => 'drawn'), new Promise((r) => setTimeout(() => r('never'), 3000))]);
		clearInterval(flood);
		slowMarks = false;
		out.push('under read marks: ' + drawnUnder);

		// Two marks of a page told while a load reads: only the last is sent. The marks before are done first
		const noMarks = () => new Promise((r) => { const look = () => (readsGoing ? setTimeout(look, 10) : r()); look(); });
		await noMarks();
		h = holdRead = hold();
		const l9 = load(true);
		await served(h);
		tellRead('kept', { read: { n: 1, at: 'x' }, say: false });
		tellRead('kept', { read: { n: 2, at: 'x' }, say: false });
		h.release();
		await l9;
		await noMarks();
		out.push('marks sent: ' + marks.join());

		// A write sent by what runs on another write's answer, before the load that waits for that answer goes on: the load waits for it too
		h = holdWrite = hold();
		const wa = api('/api/artifacts/f', archive);
		const hb = hold();
		let wb;
		noWrites().then(() => {
			holdWrite = hb;
			wb = api('/api/artifacts/g', archive);
		});
		const l11 = load(true);
		h.release();
		await wa;
		await tick();
		hb.release();
		await wb;
		await l11;
		out.push('g there: ' + has('g'));

		// A folder's read in the first round, a write meanwhile, and a press in the second round: nothing of the first round is kept
		const added2 = await api('/api/artifacts', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ path: ` + js(two) + ` }) });
		const { slug: slug2 } = await added2.json();
		opened.add(slug2);
		await load();
		h = holdFolder = hold(true);
		const l12 = load(true);
		await served(h);
		await api('/api/artifacts/h', archive);
		pressedAt = performance.now();
		h.release();
		await l12;
		pressedAt = -Infinity;
		out.push('held in the second round, folder open: ' + opened.has(slug2));

		// A read outside a load (a folder opened by hand, versions, a diff) has no time limit
		// Its fetch is called at once, before a poll's read of the folder can come between
		const outside = readListing(slug2, '');
		const outsideLimited = pagesLimited;
		await outside;
		out.push('a read outside a load limited: ' + outsideLimited);

		window.fetch = real;
		return out.join('|');
	})()`)
	check(t, "load order", strings.Split(got, "|"), strings.Join([]string{
		"reads before the first answered: 1", "a there: false",
		"reads while the write goes: 0", "b there: false",
		"read again: true, c there: false",
		"failure told: true",
		"folder open: true",
		"folder there: false",
		"joined, e there: false",
		"thrown: broken draw, said: true, the next: went on, drawn again: true, cleared: true",
		"read with a time limit: true",
		"cut off: returned, error: false then true, the next: returned, said why: true",
		"under read marks: drawn",
		`marks sent: {"n":2,"at":"x"}`,
		"g there: false",
		"held in the second round, folder open: true",
		"a read outside a load limited: false",
	}, "|"))

	// The load's mistakes are said in one place: a draw's, then the tasks' when the draw comes right, then none.
	b.navigate("/tasks")
	b.until("the task list", "document.getElementById('list') && tasksRead.raw !== undefined")
	got = b.eval(`(async () => {
		const realLoad = load;
		load = (polled) => (polled ? Promise.resolve() : realLoad(polled));
		await realLoad();
		const real = window.fetch;
		let tasksBroken = false;
		window.fetch = (u, i) => (tasksBroken && String(u) === '/api/tasks' ? Promise.resolve(new Response('tasks broken', { status: 500 })) : real(u, i));
		const realRender = render;
		let renders = 0;
		render = () => {
			renders++;
			if (renders === 1) throw new Error('broken draw');
			realRender();
		};
		const error = () => (document.getElementById('error').hidden ? 'none' : document.getElementById('error').textContent);
		drawn = '';
		await load().catch(() => {});
		const draw = error() === t('ui_draw_failed', 'broken draw');
		tasksBroken = true;
		await load();
		const tasks = error() === t('ui_tasks_failed', 'tasks broken');
		tasksBroken = false;
		await load();
		const none = error();
		// Both at once: the draw's goes first
		tasksBroken = true;
		renders = 0;
		drawn = '';
		await load().catch(() => {});
		const first = error() === t('ui_draw_failed', 'broken draw');
		tasksBroken = false;
		await load();
		// Another's mistake (a failed delete) stays through a load that says none
		showError('a delete failed');
		await load();
		const kept = error();
		render = realRender;
		load = realLoad;
		window.fetch = real;
		return 'the draw: ' + draw + ', then the tasks: ' + tasks + ', then: ' + none + ', both: the draw first ' + first + ', kept: ' + kept;
	})()`)
	check(t, "load mistakes", []string{got}, "the draw: true, then the tasks: true, then: none, both: the draw first true, kept: a delete failed")
}
