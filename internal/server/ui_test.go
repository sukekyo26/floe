package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/store"
)

// The management page in headless Chrome, ported from the JS skill's test/artifact-ui.mjs. Each
// scenario returns its observations joined by |, compared with the JS version's expectations. The
// scenarios that read the page's wording run in every language with a catalog, expecting the wording
// from it.

func chrome(t *testing.T) string {
	t.Helper()

	for _, name := range []string{"google-chrome", "chromium", "chromium-browser"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	if os.Getenv("CI") != "" {
		t.Fatal("Chrome is not installed; the management page tests need it on CI")
	}
	t.Skip("Chrome is not installed")
	return ""
}

type browser struct {
	t    *testing.T
	ctx  context.Context
	base string
}

// The UI tests share one Chrome and run one after another. Started per scenario and in parallel, six
// Chromes on a two-core CI runner took more than the 5 s a wait allows to open a tab.
var shared struct {
	once sync.Once
	ctx  context.Context // the first tab of the shared Chrome
	err  error
	stop func()
}

// TestMain stops the shared Chrome once every test has run.
func TestMain(m *testing.M) {
	code := m.Run()
	if shared.stop != nil {
		shared.stop()
	}
	os.Exit(code)
}

// startChrome starts the shared Chrome from path, once.
func startChrome(path string) (context.Context, error) {
	shared.once.Do(func() {
		// Chrome's helper processes may still write to the profile after it exits, so the profile is
		// removed with retries and left behind if it still cannot be.
		profile, err := os.MkdirTemp("", "floe-ui-")
		if err != nil {
			shared.err = err
			return
		}
		opts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.ExecPath(path),
			chromedp.NoSandbox,
			// Names other than 127.0.0.1 fail at once: the tests do not reach the network, nor wait on it
			// for anything a page loads from elsewhere.
			chromedp.Flag("host-resolver-rules", "MAP * ~NOTFOUND, EXCLUDE 127.0.0.1"),
			chromedp.UserDataDir(profile),
		)
		actx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
		// chromedp logs CDP events newer than its protocol version; they do not matter here.
		ctx, cancel := chromedp.NewContext(actx, chromedp.WithErrorf(func(string, ...any) {}))
		shared.ctx = ctx
		shared.stop = func() {
			cancel()
			cancelAlloc()
			for range 10 {
				if os.RemoveAll(profile) == nil {
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
		// The first call starts Chrome and ties it to ctx.
		if _, err := chromedp.Call(ctx, network.Enable, network.EnableParams{}); err != nil {
			shared.err = fmt.Errorf("start Chrome: %w", err)
		}
	})
	return shared.ctx, shared.err
}

// newBrowser opens a tab of the shared Chrome in a browser context of its own (its own cookies and
// storage), whose requests ask for lang (Accept-Language) as a reader of that language would.
func newBrowser(t *testing.T, base string, lang i18n.Lang) *browser {
	t.Helper()

	parent, err := startChrome(chrome(t))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := chromedp.NewContext(parent, chromedp.WithNewBrowserContext())
	t.Cleanup(cancel)
	if _, err := chromedp.Call(ctx, network.Enable, network.EnableParams{}); err != nil {
		t.Fatalf("open a tab: %v", err)
	}
	headers := network.SetExtraHTTPHeadersParams{Headers: network.Headers{"Accept-Language": string(lang)}}
	if _, err := chromedp.Call(ctx, network.SetExtraHTTPHeaders, headers); err != nil {
		t.Fatalf("set Accept-Language: %v", err)
	}
	return &browser{t: t, ctx: ctx, base: base}
}

// try evaluates the script expr (its last statement's value, awaited when a promise) and returns
// JavaScript's String() of it.
func (b *browser) try(expr string) (string, error) {
	await := func(p *runtime.EvaluateParams) {
		yes := true
		p.AwaitPromise = &yes
	}
	return chromedp.Run(b.ctx, chromedp.Evaluate[string]("(async () => String(await (0, eval)("+js(expr)+")))()", await))
}

func (b *browser) eval(expr string) string {
	b.t.Helper()

	s, err := b.try(expr)
	if err != nil {
		b.t.Fatalf("%s: %v", expr, err)
	}
	return s
}

func truthy(s string) bool {
	switch s {
	case "", "false", "0", "null", "undefined", "NaN":
		return false
	default:
		return true
	}
}

// waitLimit is how long a test waits for the page to change. Only a wait that fails takes that long.
const waitLimit = 10 * time.Second

// until polls expr every 50 ms, up to waitLimit, until it is truthy; an evaluation error (a document
// being replaced) counts as not yet.
func (b *browser) until(what, expr string) string {
	b.t.Helper()

	for deadline := time.Now().Add(waitLimit); time.Now().Before(deadline); {
		if s, err := b.try(expr); err == nil && truthy(s) {
			return s
		}
		time.Sleep(50 * time.Millisecond)
	}
	b.t.Fatalf("timed out waiting for %s", what)
	return ""
}

// navigate loads url and waits until it is loaded. The old document is marked, because the URL cannot
// tell them apart (the current URL may equal the destination).
func (b *browser) navigate(path string) {
	b.t.Helper()

	b.eval("window.leaving = true")
	if err := chromedp.Do(b.ctx, chromedp.Navigate(b.base+path)); err != nil {
		b.t.Fatalf("navigate %s: %v", path, err)
	}
	b.until(path+" to load", "!window.leaving && document.readyState === 'complete'")
}

func (b *browser) click(x, y float64) {
	b.t.Helper()

	if err := chromedp.Do(b.ctx, chromedp.MouseClickXY(x, y)); err != nil {
		b.t.Fatal(err)
	}
}

type rect struct {
	X, Y, Width, Height float64
}

func (b *browser) rect(expr string) rect {
	b.t.Helper()

	var r rect
	if err := json.Unmarshal([]byte(b.eval("JSON.stringify("+expr+".getBoundingClientRect())")), &r); err != nil {
		b.t.Fatal(err)
	}
	return r
}

// hover moves the pointer onto (x, y), over the page in the frame, until that page reports it (it
// posts "over" on pointerover). Chrome routes input into the frame only once the frame has drawn,
// which can lag behind the page's own scripts on a busy machine, and a click sent before is lost.
func (b *browser) hover(x, y float64) {
	b.t.Helper()

	b.eval("window.over = false; window.hovering ||= (addEventListener('message', (e) => { if (e.data === 'over') window.over = true; }), true)")
	move := func(x, y float64) {
		p := input.DispatchMouseEventParams{Type: input.DispatchMouseEventTypeMouseMoved, X: x, Y: y}
		if _, err := chromedp.Call(b.ctx, input.DispatchMouseEvent, p); err != nil {
			b.t.Fatal(err)
		}
	}
	for deadline := time.Now().Add(waitLimit); time.Now().Before(deadline); {
		move(0, 0) // off the frame first: pointerover fires only on entering
		move(x, y)
		if s, err := b.try("window.over"); err == nil && truthy(s) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	b.t.Fatal("timed out waiting for the page in the frame to get the pointer")
}

func js(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func check(t *testing.T, scenario string, out []string, want string) {
	t.Helper()

	if got := strings.Join(out, "|"); got != want {
		t.Errorf("%s:\n got %s\nwant %s", scenario, got, want)
	}
}

// inEveryLanguage runs scenario once for each language with a catalog, with that language's messages.
func inEveryLanguage(t *testing.T, scenario func(t *testing.T, lang i18n.Lang, msg func(string, ...any) string)) {
	t.Helper()

	for _, lang := range i18n.Supported() {
		t.Run(string(lang), func(t *testing.T) {
			scenario(t, lang, i18n.New(lang).Msg)
		})
	}
}

func TestUIManage(t *testing.T) { //nolint:paralleltest // the UI tests share one Chrome, one at a time
	inEveryLanguage(t, uiManage)
}

func uiManage(t *testing.T, lang i18n.Lang, msg func(string, ...any) string) {
	t.Helper()

	e := newEnv(t)
	e.publish(t, e.page(t, "untitled.yaml", "", "<p>untitled</p>"), store.Options{})
	time.Sleep(2 * time.Millisecond)
	e.publish(t, e.page(t, "My Demo.yaml", "Demo Page", "<p>hi</p>"), store.Options{})
	b := newBrowser(t, e.base, lang)

	const titles = "[...document.querySelectorAll('#list li .title')].map((a) => a.textContent).join(',')"
	const rows = "document.querySelectorAll('#list li').length"
	filter := func(q string) {
		b.eval("(() => { const q = document.getElementById('q'); q.value = " + js(q) + "; q.dispatchEvent(new Event('input')); })()")
	}
	clickDelete := func(title string) string {
		return b.eval(`(() => {
			const li = [...document.querySelectorAll('#list li')].find((l) => l.querySelector('.title').textContent === ` + js(title) + `);
			const b = li.querySelector('.delete');
			b.click();
			return b.querySelector('.label').textContent;
		})()`)
	}
	const down = "document.getElementById('server').classList.contains('down')"

	out := make([]string, 0, 24)
	b.navigate("/")
	out = append(out, b.until("the list to render", titles))
	out = append(out, b.eval("(() => { const d = document.querySelector('#list li .download'); const o = document.querySelector('#list li .open'); "+
		"return `${d.getAttribute('href')} ${d.getAttribute('download')} ${o.getAttribute('href')} ${o.target}`; })()"))
	// A row's project tag filters by it and keeps the choice in the URL; the all-projects chip clears it.
	out = append(out, b.eval(`(() => {
		const tag = document.querySelector('#list li .tag');
		tag.click();
		const pressed = document.querySelector('#chips .chip[aria-pressed="true"]').firstChild.textContent;
		return [document.querySelectorAll('#list li').length, pressed === tag.textContent,
			new URLSearchParams(location.search).get('project') === tag.textContent].join(' ');
	})()`))
	b.eval("document.querySelector('#chips .chip').click()")
	out = append(out, b.eval("location.search === '' && document.querySelector('#chips .chip').getAttribute('aria-pressed')"))
	filter("untitled")
	out = append(out, b.eval(rows))
	// The search is kept in the URL and restored from it.
	out = append(out, b.eval("new URLSearchParams(location.search).get('q')"))
	b.navigate("/?q=untitled")
	out = append(out, b.until("the search restored from the URL",
		"document.getElementById('q').value === 'untitled' && document.querySelectorAll('#list li').length"))
	filter("")
	out = append(out, clickDelete("untitled"), b.eval(rows))
	clickDelete("untitled")
	b.until("the deleted row to disappear", rows+" === 1")
	out = append(out, b.eval(titles))
	out = append(out, b.eval("fetch('/api/artifacts').then((r) => r.json()).then((a) => a.map((x) => x.slug).join(','))"))
	// A DELETE that cannot reach the server shows an error and keeps the row.
	b.eval("window.realFetch = window.fetch; window.fetch = () => Promise.reject(new TypeError('Failed to fetch'))")
	clickDelete("Demo Page")
	clickDelete("Demo Page")
	out = append(out, b.until("the delete error", "(() => { const e = document.getElementById('error'); "+
		"return !e.hidden && e.textContent === "+js(msg("ui_delete_failed", "Demo Page", "Failed to fetch"))+"; })()"))
	out = append(out, b.eval(rows), b.eval(down))
	b.eval("window.fetch = window.realFetch")
	// The button stays armed, so one more click deletes, and reaching the server turns the indicator up.
	clickDelete("Demo Page")
	out = append(out, b.until("the empty state", "!document.getElementById('empty').hidden"), b.eval(down))
	// Polling notices a lost server and turns the indicator back up once it answers again.
	b.eval("window.fetch = () => Promise.reject(new TypeError('Failed to fetch'))")
	out = append(out, b.until("the indicator to go down", down))
	b.eval("window.fetch = window.realFetch")
	out = append(out, b.until("the indicator to come back up", "!"+down))
	// The add form opens in a modal from the links tab only, focused on the path; cancel closes it.
	b.eval("document.querySelector('.tabs [data-tab=links]').click()")
	out = append(out, b.eval(`(() => {
		document.getElementById('add-open').click();
		const shown = document.getElementById('add-dialog').open && document.activeElement.id;
		document.getElementById('add-cancel').click();
		return [shown, document.getElementById('add-dialog').open].join(' ');
	})()`))
	// The server's mistakes show a line each, the status on the first: those of a data file with two.
	two := e.write(t, "two.yaml", "template: report\nlang: en\nleed: x\neybrow: y\n")
	b.eval(`(() => {
		document.getElementById('add-open').click();
		document.getElementById('add-path').value = ` + js(two) + `;
		document.getElementById('add').requestSubmit();
	})()`)
	out = append(out, b.until("the add to be refused", "!document.getElementById('add-error').hidden && "+
		"(() => { const lines = document.getElementById('add-error').innerText.split('\\n'); "+
		"return lines.length + ' ' + lines[0].includes('HTTP 400'); })()"))
	b.eval("document.getElementById('add-cancel').click()")
	b.eval("document.querySelector('.tabs [data-tab=pages]').click()")
	out = append(out, b.eval("document.getElementById('add-open').hidden"))
	// Paging over 25 rows (a stubbed list, so that polling keeps returning it): 10 a page by default,
	// then the next page (kept in the URL), then 20 a page (remembered in the browser, back to page 1).
	const pager = "[document.querySelectorAll('#list li').length, document.getElementById('pageno').textContent, document.getElementById('next').disabled].join(' ')"
	b.eval(`(() => {
		const list = JSON.stringify(Array.from({ length: 25 }, (_, i) => ({ slug: ` + "`p${i}`, title: `P${i}`" + `, description: '', updatedAt: '2026-01-01T00:00:00.000Z' })));
		window.fetch = (url, init) => (url === '/api/artifacts' ? Promise.resolve(new Response(list)) : window.realFetch(url, init));
		return load();
	})()`)
	out = append(out, b.eval(pager))
	b.eval("document.getElementById('next').click()")
	out = append(out, b.eval(pager), b.eval("new URLSearchParams(location.search).get('page')"))
	b.eval("[...document.querySelectorAll('#sizes button')].find((b) => b.textContent === '20').click()")
	out = append(out, b.eval(pager)+" "+b.eval("localStorage.getItem('artifacts.pageSize')")+" "+b.eval("location.search === ''"))
	// A reason under a mistake keeps its indent: a part of one's own that cannot be used, which the data uses.
	// Each line is checked to sit 2 deeper than the one above.
	e.write(t, "parts/note/schema.json", `{"type": 1}`)
	e.write(t, "parts/note/part.html", `<div data-floe-part="note"></div>`)
	e.write(t, "parts/note/part.css", "")
	usesNote := e.write(t, "note.yaml", "template: report\nlang: en\nsections:\n  - parts:\n      - note: { text: hi }\n")
	b.eval("window.fetch = window.realFetch; document.querySelector('.tabs [data-tab=links]').click()")
	b.eval(`(() => {
		document.getElementById('add-open').click();
		document.getElementById('add-path').value = ` + js(usesNote) + `;
		document.getElementById('add').requestSubmit();
	})()`)
	out = append(out, b.until("the unusable part to be refused", "!document.getElementById('add-error').hidden && "+
		"document.getElementById('add-error').innerText.split('\\n').every((l, i, all) => all.length > 1 && l.length - l.trimStart().length === 2 * i)"))

	check(t, "manage", out, "Demo Page,untitled|/api/artifacts/my-demo/download my-demo.html /a/my-demo/?raw _blank|"+
		"2 true true|true|1|untitled|1|"+msg("ui_delete_confirm")+"|2|Demo Page|my-demo|true|1|true|true|false|true|true|"+
		"add-path false|2 true|true|10 1 / 3 false|10 2 / 3 false|2|20 1 / 2 false 20 true|true")
}

// extrasStub answers the list and records the writes in window.calls.
const extrasStub = `(() => {
	window.calls = [];
	const at = (d) => ` + "`2026-01-0${d}T00:00:00.000Z`" + `;
	const list = JSON.stringify([
		{ slug: 'b', title: 'Beta', description: '', createdAt: at(3), updatedAt: at(9) },
		{ slug: 'a', title: 'Alpha', description: 'old', createdAt: at(1), updatedAt: at(8) },
		{ slug: 'c', title: 'Gamma', description: '', createdAt: at(5), updatedAt: at(7), favorite: true },
		{ slug: 'l', title: 'Linked', description: '', path: '/tmp/x/l.html', createdAt: at(1), updatedAt: at(1) },
		{ slug: 'm', title: 'Missing', description: '', path: '/tmp/x/m.html', missing: true, createdAt: at(1), updatedAt: at(1) },
	]);
	window.fetch = (url, init = {}) => {
		if (!init.method) return Promise.resolve(new Response(list));
		calls.push([init.method, url, init.body].filter(Boolean).join(' '));
		return Promise.resolve(new Response(null, { status: 204 }));
	};
	navigator.clipboard.writeText = (text) => {
		calls.push(` + "`copy ${text}`" + `);
		return Promise.resolve();
	};
	return load();
})()`

// TestUIExtras sorts, filters favourites, moves by keyboard, deletes in bulk, edits in a modal, copies
// a path, switches the theme and steps through neighbours, against a stubbed list.
func TestUIExtras(t *testing.T) { //nolint:paralleltest // the UI tests share one Chrome, one at a time
	inEveryLanguage(t, uiExtras)
}

func uiExtras(t *testing.T, lang i18n.Lang, msg func(string, ...any) string) {
	t.Helper()

	e := newEnv(t)
	b := newBrowser(t, e.base, lang)
	b.navigate("/")
	b.eval(extrasStub)
	const titles = "[...document.querySelectorAll('#list .title')].map((a) => a.textContent).join(',')"
	sortBy := func(v string) {
		b.eval("(() => { const s = document.getElementById('sort'); s.value = '" + v + "'; s.dispatchEvent(new Event('change')); })()")
	}
	key := func(k string) {
		b.eval("document.body.dispatchEvent(new KeyboardEvent('keydown', { key: '" + k + "', bubbles: true }))")
	}

	out := make([]string, 0, 21)
	out = append(out, b.eval(titles))
	sortBy("created")
	out = append(out, b.eval(titles)+" "+b.eval("new URLSearchParams(location.search).get('sort')"))
	sortBy("title")
	out = append(out, b.eval(titles))
	// The ★ chip narrows to favourites (kept in the URL); a row's star sends the change.
	b.eval("document.getElementById('fav').click()")
	out = append(out, b.eval(titles)+" "+b.eval("new URLSearchParams(location.search).get('fav')"))
	b.eval("document.getElementById('fav').click()")
	b.eval("document.querySelector('#list li .star').click()")
	out = append(out, b.until("the favourite", "calls.splice(0).join(',')"))
	// The time's tooltip has both the creation and the update.
	out = append(out, b.eval("document.querySelector('#list li time').title.split('\\n').map((l) => l.split(' ')[0]).join(' ')"))
	// ↓ moves through the rows by focusing their titles, x selects the focused row.
	key("ArrowDown")
	key("ArrowDown")
	out = append(out, b.eval("document.activeElement.textContent"))
	key("x")
	out = append(out, b.eval("document.getElementById('bulk-count').textContent"))
	// With Alpha selected too by a real mouse click (nothing may sit on top of the checkbox), the first
	// click arms and the second deletes both.
	r := b.rect("document.querySelector('#list li .sel')")
	b.click(r.X+r.Width/2, r.Y+r.Height/2)
	out = append(out, b.until("the mouse click to select",
		"document.querySelector('#list li .sel').checked && document.getElementById('bulk-count').textContent"))
	b.eval("document.getElementById('bulk-delete').click()")
	out = append(out, b.eval("document.getElementById('bulk-delete').textContent"))
	b.eval("document.getElementById('bulk-delete').click()")
	b.until("the selection to clear", "document.getElementById('bulk').hidden")
	out = append(out, b.eval("calls.splice(0).sort().join(',')"))
	// The pencil opens a modal with the current description; saving sends the new one.
	b.eval("document.querySelector('#list li .edit').click()")
	out = append(out, b.eval("[document.getElementById('edit-dialog').open, document.getElementById('edit-desc').value].join(' ')"))
	b.eval("document.getElementById('edit-desc').value = 'new'; document.getElementById('edit').requestSubmit()")
	b.until("the edit modal to close", "!document.getElementById('edit-dialog').open")
	out = append(out, b.eval("calls.splice(0).join(',')"),
		b.eval("document.getElementById('toasts').textContent.includes("+js(msg("ui_saved"))+")"))
	// Only linked files have a copy button for their path.
	out = append(out, b.eval("document.querySelector('#list li .copy').hidden"))
	b.eval("document.querySelector('.tabs [data-tab=links]').click()")
	b.eval("document.querySelector('#list li .copy').click()")
	out = append(out, b.until("the copy", "calls.join(',')"),
		b.eval("document.getElementById('toasts').textContent.includes("+js(msg("ui_path_copied"))+")"))
	// A link whose file is gone has no href, but ↓ still reaches it.
	b.eval("document.activeElement.blur()")
	key("ArrowDown")
	key("ArrowDown")
	out = append(out, b.eval("document.activeElement.textContent"))
	// The add form sends only the fields filled in, trimmed: a field sent is given, empty or not.
	b.eval("document.querySelector('.tabs [data-tab=links]').click()")
	b.eval(`(() => {
		calls.length = 0;
		document.getElementById('add-open').click();
		document.getElementById('add-path').value = ' /tmp/x/n.html ';
		document.getElementById('add-desc').value = ' a link ';
		document.getElementById('add').requestSubmit();
	})()`)
	out = append(out, b.until("the add to be sent", "calls.join(',')"))
	b.eval("document.getElementById('add-cancel').click()")
	b.eval("document.querySelector('.tabs [data-tab=pages]').click()")
	// The theme switch overrides the OS setting and is remembered; the system choice drops the override.
	b.eval("document.querySelector('[data-theme-choice=dark]').click()")
	out = append(out, b.eval("[document.documentElement.dataset.theme, getComputedStyle(document.body).backgroundColor, localStorage.getItem('artifacts.theme')].join(' ')"))
	b.eval("document.querySelector('[data-theme-choice=system]').click()")
	out = append(out, b.eval("String(document.documentElement.dataset.theme)"))
	// A page opened from the list steps to its neighbours in that list's order, and the logo returns to
	// that list.
	b.eval("sessionStorage.setItem('artifacts.list', '?sort=title')")
	b.navigate("/a/b/")
	b.eval(extrasStub)
	out = append(out, b.until("the neighbours", "document.getElementById('frame-pos').textContent && "+
		"['frame-prev', 'frame-next'].map((id) => document.getElementById(id).getAttribute('href'))"+
		".concat(document.getElementById('frame-pos').textContent, document.querySelector('.brand').getAttribute('href')).join(' ')"))

	// The time's tooltip lines start with the label of their format ("Created %s").
	label := func(key string) string { return strings.Fields(msg(key, ""))[0] }
	check(t, "extras", out, `Beta,Alpha,Gamma|Gamma,Beta,Alpha created|Alpha,Beta,Gamma|Gamma 1|`+
		`PATCH /api/artifacts/a {"favorite":true}|`+label("ui_created_at")+" "+label("ui_updated_at")+`|Beta|`+
		msg("ui_selected", 1)+"|"+msg("ui_selected", 2)+"|"+msg("ui_delete_bulk_confirm", 2)+"|"+
		`DELETE /api/artifacts/a,DELETE /api/artifacts/b|true old|PATCH /api/artifacts/a {"description":"new"}|true|true|`+
		`copy /tmp/x/l.html|true|Missing|POST /api/artifacts {"path":"/tmp/x/n.html","description":"a link"}|dark rgb(23, 25, 31) dark|undefined|/a/a/ /a/c/ 2 / 3 /?sort=title`)
}

// listening attaches one message listener per document and returns the last message the page sent.
// It keeps the page's own words, not what floe's script in the page tells the frame (artifactHash).
const listening = "(window.listening ||= (addEventListener('message', (e) => { if (typeof e.data === 'string') window.reported = e.data; }), true)) && window.reported"

// TestUIReload checks that an open page reloads itself when republished, from inside the sandbox, and
// that the page URL is a frame which allows clipboard writes and fullscreen, sends links to other sites
// to new tabs out of the sandbox, and moves the whole tab for links to another page.
func TestUIReload(t *testing.T) { //nolint:paralleltest // the UI tests share one Chrome, one at a time

	e := newEnv(t)
	// The page's wording plays no part here, so one language is enough.
	b := newBrowser(t, e.base, i18n.English)
	publish := func(name, slug, title, body string) {
		time.Sleep(2 * time.Millisecond) // a republish in the same millisecond would keep the version
		e.publish(t, e.page(t, filepath.Join("live", name), title, body), store.Options{Slug: &slug})
	}
	shown := func(v string) string { return "document.getElementById('v')?.textContent === '" + v + "'" }

	out := make([]string, 0, 7)
	publish("live.yaml", "live", "Live", `<p id="v">1</p>`)
	b.navigate("/a/live/?raw")
	b.until("the first version", shown("1"))
	out = append(out, "1")
	publish("live.yaml", "live", "Live", `<p id="v">2</p>`)
	b.until("the page to reload", shown("2"))
	out = append(out, "2")
	// The page URL itself is the frame: the tab is named after the page and the iframe loads ?raw.
	b.navigate("/a/live/")
	out = append(out, b.until("the frame title", "document.title.startsWith('Live') && document.title"))
	out = append(out, b.eval("new URL(document.getElementById('frame').src).search === '?raw'"))
	// The frame lets the page write to the clipboard and go fullscreen. The page keeps reporting its
	// policy, since it may load before the listener is attached.
	publish("perm.yaml", "perm", "Perm", "<script>const policy = document.permissionsPolicy ?? document.featurePolicy; "+
		"setInterval(() => parent.postMessage(policy ? ['clipboard-write', 'fullscreen'].map((f) => policy.allowsFeature(f)).join() : 'no policy API', '*'), 100)</script>")
	b.navigate("/a/perm/")
	out = append(out, b.until("the page permissions", listening))
	out = append(out, leave(t, e, b, publish))
	// An unknown page shows the way back instead of the frame.
	b.navigate("/a/no-such/")
	out = append(out, b.until("the not-found state",
		"!document.getElementById('notfound').hidden && document.getElementById('frame').hidden && !document.getElementById('frame').hasAttribute('src')"))

	check(t, "reload", out, "1|2|Live · floe|true|true,true|own own none /a/no-such/|true")
}

// TestUIFrameHash checks that the frame opens its page at the part its URL names, coming straight, through
// the redirect or from a link in another page, and that a move to a part inside the frame is kept in the
// frame's URL, so that a reload opens it, and Back undoes it at once.
func TestUIFrameHash(t *testing.T) { //nolint:paralleltest // the UI tests share one Chrome, one at a time
	e := newEnv(t)
	b := newBrowser(t, e.base, i18n.English)
	publish := func(slug, body string) {
		e.publish(t, e.page(t, filepath.Join("hash", slug+".yaml"), slug, body), store.Options{Slug: &slug})
	}
	link := func(href, top string) string {
		return `<a href="` + href + `" style="position: fixed; left: 0; top: ` + top + `; width: 200px; height: 40px">go</a>`
	}
	// The page reports the part it is at, and which load of it it is, as the frame cannot read the sandboxed
	// page. Whether Chrome scrolls there is Chrome's: in the tests' Chrome it now and then does not on a
	// load, which floe cannot mend. Both pages take the name parent, which floe's script must not use.
	publish("hash", link("#s2", "0")+link("/a/hash/#s3", "60px")+
		`<div style="height: 3000px"></div><h2 id="s2">s2</h2><div style="height: 3000px"></div><h2 id="s3">s3</h2><div style="height: 3000px"></div>`+
		"<script>var parent = 1; const load = Math.random(); document.addEventListener('pointerover', () => top.postMessage('over', '*'));"+
		"setInterval(() => top.postMessage('at ' + location.hash + ' ' + load, '*'), 50)</script>")
	publish("from", link("/a/hash/#s2", "0")+
		"<script>var parent = 1; document.addEventListener('pointerover', () => top.postMessage('over', '*'))</script>")
	// The page may report a part before floe's script tells the frame of it: the frame's URL is waited for too.
	at := func(what, want string) string {
		return b.until(what, "(window.watching ||= (addEventListener('message', (e) => { if (String(e.data).startsWith('at ')) window.at = e.data; }), true)) && "+
			"window.at?.startsWith("+js("at "+want+" ")+") && location.hash === "+js(want)+" && location.pathname + location.hash")
	}
	click := func(y float64) {
		box := b.rect("document.getElementById('frame')")
		b.hover(box.X+20, box.Y+y)
		b.click(box.X+20, box.Y+y)
	}

	out := make([]string, 0, 6)
	b.navigate("/a/hash/#s2")
	out = append(out, at("the part, opened straight", "#s2"))
	b.navigate("/a/hash#s2")
	out = append(out, at("the part, through the redirect", "#s2"))
	b.navigate("/a/from/")
	b.until("the page in the frame", "document.getElementById('frame').hasAttribute('src')")
	click(20)
	out = append(out, at("the part, from another page", "#s2"))
	b.navigate("/a/hash/")
	at("the top of the page", "")
	click(20)
	out = append(out, at("the part, moved to in the frame", "#s2"))
	b.eval("window.leaving = true; location.reload()")
	b.until("the reload", "!window.leaving && document.readyState === 'complete'")
	out = append(out, at("the part, after a reload", "#s2"))
	b.eval("history.back()")
	out = append(out, at("the top of the page, back", ""))
	// The page follows the frame's own fragment: a link in it to its frame URL, and the address bar.
	click(80)
	out = append(out, at("the part, by a link to the frame URL", "#s3"))
	b.eval("location.hash = '#s2'")
	out = append(out, at("the part, by the address bar", "#s2"))
	b.eval("history.back()")
	out = append(out, at("the part before it, back", "#s3"))
	// Back to no part moves the page to its top, without loading it again.
	load := b.eval("window.at.split(' ').pop()")
	b.eval("history.back()")
	out = append(out, at("the top of the page, back again", ""))
	out = append(out, b.eval("window.at.split(' ').pop() === "+js(load)))

	check(t, "frame hash", out,
		"/a/hash/#s2|/a/hash/#s2|/a/hash/#s2|/a/hash/#s2|/a/hash/#s2|/a/hash/|/a/hash/#s3|/a/hash/#s2|/a/hash/#s3|/a/hash/|true")
}

// leave clicks links out of a page in the frame: a target=_blank tab runs in the other site's origin
// out of the sandbox, a link without target opens a new tab too (most sites refuse to be framed), a
// link within the server stays in the frame, and a link to a frame URL moves the whole tab. The other
// site refuses framing. It returns each tab's origin (none when no tab opened) and where the tab moved.
func leave(t *testing.T, e *env, b *browser, publish func(name, slug, title, body string)) string {
	t.Helper()

	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		_, _ = fmt.Fprint(w, "<title>Elsewhere</title>") //nolint:errcheck // the test checks what opened
	}))
	defer other.Close()
	at := func(top string) string {
		return `style="position: fixed; left: 0; width: 200px; height: 40px; top: ` + top + `"`
	}
	publish("leave.yaml", "leave", "Leave",
		`<a href="`+other.URL+`/blank" target="_blank" `+at("0")+`>blank</a><a href="`+other.URL+`/plain" `+at("60px")+`>plain</a>`+
			`<a href="inner.html" `+at("120px")+`>inner</a><a href="/a/no-such/" `+at("180px")+`>frame</a>`+
			// The page declares globals named like the ones the appended script uses; it must not pick them up.
			"<script>const open = () => {}; const URL = null; const EventSource = null; const self = null;"+
			"document.addEventListener('pointerover', () => parent.postMessage('over', '*'));"+
			"setInterval(() => parent.postMessage('ready', '*'), 100)</script>")
	b.navigate("/a/leave/")
	b.until("the page in the frame", listening)
	box := b.rect("document.getElementById('frame')")
	b.hover(box.X+20, box.Y+20)
	for _, y := range []float64{20, 80, 140} {
		b.click(box.X+20, box.Y+y)
	}
	tab := func(url string) *target.Info {
		targets, err := chromedp.Targets(b.ctx)
		if err != nil {
			t.Fatal(err)
		}
		for _, info := range targets {
			if info.Type == "page" && info.URL == url {
				return info
			}
		}
		return nil
	}
	for deadline := time.Now().Add(waitLimit); tab(other.URL+"/blank") == nil; {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the target=_blank tab")
		}
		time.Sleep(50 * time.Millisecond)
	}
	// A link opened inside the frame adds no tab; if it did, it would come about when blank's did.
	time.Sleep(500 * time.Millisecond)
	var origins []string
	for _, url := range []string{other.URL + "/blank", other.URL + "/plain", e.base + "/a/leave/inner.html"} {
		info := tab(url)
		if info == nil {
			origins = append(origins, "none")
			continue
		}
		tctx, cancel := chromedp.NewContext(b.ctx, chromedp.WithTargetID(info.TargetID))
		origin, err := chromedp.Run(tctx, chromedp.Evaluate[string]("self.origin"))
		if err != nil {
			t.Fatal(err)
		}
		if origin == other.URL {
			origin = "own"
		}
		origins = append(origins, origin)
		// Cancelling a context attached to a target closes the target.
		cancel()
	}
	// The inner link moved the iframe, so the page is opened again before the last click.
	b.navigate("/a/leave/")
	b.until("the page in the frame again", listening)
	box = b.rect("document.getElementById('frame')")
	b.hover(box.X+20, box.Y+200)
	b.click(box.X+20, box.Y+200)
	// The tab moves to the frame URL itself, and no frame is nested in the iframe.
	origins = append(origins, b.until("the tab to move", "location.pathname === '/a/no-such/' && location.pathname"))
	return strings.Join(origins, " ")
}
