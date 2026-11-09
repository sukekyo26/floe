// Package browsertest_test checks in headless Chrome what only a browser can tell: that the CSP and the frame
// keep a page in its sandbox, that a script part runs under it, that the frame moves a tab for its links and
// reloads a republished page, and that every theme's text can be read and wraps at a phone's width.
//
// LIMIT: the management page's delete and its error text, the connection's dot, and add and its refusals are checked in a browser
// against a stubbed list only (TestUIExtras), not a real server. Check them against one if they come to break apart from the list.
// LIMIT: a frame URL's #part is checked in a browser for a folder's page only (TestUIFolders: written by the frame, and
// followed from the address bar), not kept on reload and Back. Check it on a stored page if that comes to break.
// LIMIT: the diagram's zoom, pan and keys are not checked in a browser; port develop's TestUIZoom in a PR that changes zoom.js.
package browsertest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"

	"github.com/sukekyo26/floe/internal/cli"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/servetest"
)

// errNoChrome is a Chrome that is not there where FLOE_CHROME says, or not found on CI.
var errNoChrome = errors.New("no Chrome")

// findChrome is the Chrome to run: FLOE_CHROME, else the first of google-chrome, chromium and chromium-browser on
// PATH. On CI, where FLOE_CHROME names the Chrome of the pinned version, it is required: one found on PATH would
// be the runner image's, of another version. Without one, the tests are skipped, but not on CI.
func findChrome(getenv func(string) string, lookPath func(string) (string, error)) (path string, skip bool, err error) {
	ci := getenv("CI") != ""
	if path := getenv("FLOE_CHROME"); path != "" {
		info, err := os.Stat(path)
		if err != nil {
			return "", false, fmt.Errorf("%w at FLOE_CHROME: %w", errNoChrome, err)
		}
		if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
			return "", false, fmt.Errorf("%w at FLOE_CHROME: %s is not a file to run", errNoChrome, path)
		}
		return path, false, nil
	}
	if ci {
		return "", false, fmt.Errorf("%w: on CI, set FLOE_CHROME to the Chrome of the pinned version", errNoChrome)
	}
	for _, name := range []string{"google-chrome", "chromium", "chromium-browser"} {
		if path, err := lookPath(name); err == nil {
			return path, false, nil
		}
	}
	return "", true, nil
}

// chrome is the Chrome to run, as findChrome finds it; the test is skipped without one.
func chrome(t *testing.T) string {
	t.Helper()

	path, skip, err := findChrome(os.Getenv, exec.LookPath)
	switch {
	case err != nil:
		t.Fatal(err)
	case skip:
		t.Skip("Chrome is not installed; set FLOE_CHROME or put it on PATH")
	}
	return path
}

type browser struct {
	t    *testing.T
	ctx  context.Context
	base string
	// faked is the script (*browser).stub added to every page this tab opens, "" for none.
	faked page.ScriptIdentifier
}

// The tests share one Chrome and run one after another. Started per test and in parallel, six Chromes on a
// two-core CI runner took more than the 5 s a wait allows to open a tab.
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
		profile, err := os.MkdirTemp("", "floe-browser-")
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
			// A screen with a mouse, as the management page shows a row's actions only while it is pointed at there.
			// Headless Chrome has no pointing device, and Emulation.setEmulatedMedia does not take hover.
			chromedp.Flag("blink-settings", "primaryHoverType=2,availableHoverTypes=2,primaryPointerType=4,availablePointerTypes=4"),
			// Scrollbars drawn, which headless hides, as the management page keeps their room.
			chromedp.Flag("hide-scrollbars", false),
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

// The window of a tab that asks for no other size. It is set on each tab, not with --window-size: that sets the outer window, and what Chrome
// keeps of it above the page differs by version (of 800x600, 400 high on 156 and 457 on 157).
// LIMIT: the tabs newBrowser opens alone: a popup a page opens has Chrome's own window. Set it there too if a test comes to measure one
const (
	windowWidth  = 780
	windowHeight = 380
)

// newBrowser opens a tab of the shared Chrome in a browser context of its own (its own cookies and storage),
// whose requests ask for lang (Accept-Language) as a reader of that language would.
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
	b := &browser{t: t, ctx: ctx, base: base}
	b.unsize()
	b.addScript("window.thrown = []; for (const type of ['error', 'unhandledrejection']) addEventListener(type, (e) => window.thrown.push(String(e.error ?? e.reason ?? e.message)));")
	return b
}

// try evaluates the script expr (its last statement's value, awaited when a promise) and returns
// JavaScript's String() of it.
func (b *browser) try(expr string) (string, error) {
	await := func(p *runtime.EvaluateParams) {
		yes := true
		p.AwaitPromise = &yes
	}
	return chromedp.Run(b.ctx, chromedp.Evaluate[string]("(async () => String(await (0, eval)("+js(expr)+")))()", await)) //nolint:wrapcheck // the caller's to tell
}

// moving runs expr, which moves to another page and may leave before the answer comes. Nothing is answered: what the test waits for
// next tells whether it went.
func (b *browser) moving(expr string) {
	b.t.Helper()

	if _, err := b.try(expr); err != nil && !strings.Contains(err.Error(), "Inspected target navigated") {
		b.t.Fatalf("%s: %v", expr, err)
	}
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

	var last string
	var err error
	for deadline := time.Now().Add(waitLimit); time.Now().Before(deadline); {
		if last, err = b.try(expr); err == nil && truthy(last) {
			return last
		}
		time.Sleep(50 * time.Millisecond)
	}
	// What the page threw, and what it drew: a draw that threw stays as it is until the data changes.
	seen, seenErr := b.try("[location.pathname + location.search, document.activeElement?.tagName + '#' + document.activeElement?.id, [...document.querySelectorAll(':is(:open, :popover-open)')].map((e) => e.tagName + '#' + e.id).join(), (window.thrown ?? []).join(' / '), document.getElementById('list')?.innerText?.replaceAll('\\n', ' | ')].join(' ## ')")
	b.t.Fatalf("timed out waiting for %s: last %q, %v; the page: %s, %v", what, last, err, seen, seenErr)
	return ""
}

// still waits until none of the page's transitions and animations runs: a style read while one runs is between two
// states, as a slow machine reads it. It waits for the whole document's, as an inherited style (an icon's fill of
// currentColor) moves with its ancestor's motion, not its own. One that ended and holds its last frame (fill: both, as a
// row's rise) and one without end (the server's pulse) do not count.
// LIMIT: a motion that starts between the wait and the read (the list redrawn, a toast) is not waited for; change nothing between them.
// LIMIT: a frame's document is not waited for; wait in it when a test comes to read styles inside the frame.
func (b *browser) still() {
	b.t.Helper()

	b.until("the page's motions to end", "!document.getAnimations().some((a) => a.playState === 'running' && "+
		"a.effect.getComputedTiming().endTime !== Infinity)")
}

// settled returns JavaScript's String() of expr once the page is still: a style that moves, read without knowing the
// value to expect. expr is evaluated once, after the wait, so that a missing element or a mistake in it fails there at
// once.
func (b *browser) settled(expr string) string {
	b.t.Helper()

	b.still()
	return b.eval(expr)
}

// navigate loads the path, or a URL, and waits until it is loaded. The old document is marked, because the URL
// cannot tell them apart (the current URL may equal the destination).
func (b *browser) navigate(path string) {
	b.t.Helper()

	url := path
	if strings.HasPrefix(path, "/") {
		url = b.base + path
	}
	b.eval("window.leaving = true")
	if err := chromedp.Do(b.ctx, chromedp.Navigate(url)); err != nil {
		b.t.Fatalf("navigate %s: %v", path, err)
	}
	b.until(path+" to load", "!window.leaving && document.readyState === 'complete'")
	if b.faked != "" && b.eval("window.faked === true") == "true" {
		b.loadStubbed()
	}
}

// pressKey presses k as a keyboard does: its text on the key down, which a page's preventDefault then stops too
// (chromedp.KeyEvent sends the text as an event of its own, which nothing stops). For Enter, space and the keys with no text.
func (b *browser) pressKey(k string) {
	b.t.Helper()

	for _, e := range kb.Encode([]rune(k)[0]) {
		if e.Type == kb.KeyChar {
			continue
		}
		if e.Type == kb.KeyDown {
			e.Text = map[string]string{kb.Enter: "\r", " ": " "}[k]
		}
		if _, err := chromedp.Call(b.ctx, input.DispatchKeyEvent, *e); err != nil {
			b.t.Fatal(err)
		}
	}
}

// painted waits for what the page holds now to be on the screen, by having Chrome take a picture of it, of the whole window: one with
// a clip resizes the page for it, and a press in an iframe is lost. Chrome sends a press and its release by what it last put on the
// screen, which on a busy machine lags behind the page: over a list of choices just opened, either reaches the iframe under it, though
// the pointer moving there reaches the list, and two animation frames are not enough. Every press goes through here. A page leaving
// cannot be pictured and has nothing to wait for, so that the error tells nothing.
func (b *browser) painted() {
	b.t.Helper()

	_, _ = chromedp.Call(b.ctx, page.CaptureScreenshot, page.CaptureScreenshotParams{}) //nolint:errcheck // see above
}

func (b *browser) click(x, y float64) {
	b.t.Helper()

	b.painted()
	if err := chromedp.Do(b.ctx, chromedp.MouseClickXY(x, y)); err != nil {
		b.t.Fatal(err)
	}
}

// media draws this test's tab for the medium ("print", or "" for the screen) with the features (prefers-color-scheme
// and the like), as a browser would; a call takes the place of the one before.
func (b *browser) media(medium string, features ...*emulation.MediaFeature) {
	b.t.Helper()

	if _, err := chromedp.Call(b.ctx, emulation.SetEmulatedMedia, emulation.SetEmulatedMediaParams{Media: medium, Features: features}); err != nil {
		b.t.Fatal(err)
	}
}

// size gives this test's tab a window of width by height, a phone's (touch, its own viewport) when mobile; unsize gives the
// tab the window it opened with. Either holds for this test's tab alone.
func (b *browser) size(width, height int64, mobile bool) {
	b.t.Helper()

	metrics := emulation.SetDeviceMetricsOverrideParams{Width: width, Height: height, DeviceScaleFactor: 1, Mobile: mobile}
	if _, err := chromedp.Call(b.ctx, emulation.SetDeviceMetricsOverride, metrics); err != nil {
		b.t.Fatal(err)
	}
}

// scrollbars hides the scrollbars of this test's tab, or shows them again, so that a page is laid out as wide as the
// window, as on paper.
func (b *browser) scrollbars(hidden bool) {
	b.t.Helper()

	if _, err := chromedp.Call(b.ctx, emulation.SetScrollbarsHidden, emulation.SetScrollbarsHiddenParams{Hidden: hidden}); err != nil {
		b.t.Fatal(err)
	}
}

func (b *browser) unsize() {
	b.t.Helper()

	b.size(windowWidth, windowHeight, false)
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
	var last string
	var err error
	for deadline := time.Now().Add(waitLimit); time.Now().Before(deadline); {
		// The toast of a page just read lies over it, and takes the pointer: not what these tests are of.
		b.eval("document.querySelector('.toast.page')?.remove()")
		move(0, 0) // off the frame first: pointerover fires only on entering
		move(x, y)
		if last, err = b.try("window.over"); err == nil && truthy(last) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	b.t.Fatalf("timed out waiting for the page in the frame to get the pointer: last %q, %v", last, err)
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

// site is a server with a browser on it.
type site struct {
	*servetest.Site

	b *browser
}

func newSite(t *testing.T) *site {
	t.Helper()

	s := servetest.Start(t, nil)
	return &site{Site: s, b: newBrowser(t, s.Base, i18n.English)}
}

// write writes text to rel under home and returns its path.
func (s *site) write(t *testing.T, rel, text string) string {
	t.Helper()

	file := filepath.Join(s.Env["HOME"], filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(text), 0o600); err != nil { //nolint:gosec // G703: a path the test names
		t.Fatal(err)
	}
	return file
}

// own writes files (paths under the server's directory, slash-separated), parts, themes and templates of one's own.
func (s *site) own(t *testing.T, files map[string]string) {
	t.Helper()

	for name, text := range files {
		at := filepath.Join(s.Dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(text), 0o600); err != nil { //nolint:gosec // G703: a path the test names
			t.Fatal(err)
		}
	}
}

// page writes a data file for the report template under home titled title ("" for none), with html as its one
// part when there is some, and returns its path.
func (s *site) page(t *testing.T, rel, title, html string) string {
	t.Helper()

	var b strings.Builder
	b.WriteString("template: report\nlang: en\n")
	if title != "" {
		b.WriteString("title: " + js(title) + "\n") // a JSON string is a YAML one too
	}
	if html != "" {
		b.WriteString("sections:\n  - parts:\n      - html: |\n")
		for line := range strings.Lines(html) {
			b.WriteString("          " + line)
		}
		b.WriteString("\n")
	}
	return s.write(t, rel, b.String())
}

// floe runs floe as an agent would, from home, against this server, and returns what it printed.
func (s *site) floe(t *testing.T, args ...string) string {
	t.Helper()

	var stdout, stderr bytes.Buffer
	getenv := func(key string) string { return s.Env[key] }
	if code := cli.Run(args, cli.Env{Stdout: &stdout, Stderr: &stderr, Dir: s.Env["HOME"], Getenv: getenv}); code != 0 {
		t.Fatalf("floe %s = %d: %s", strings.Join(args, " "), code, stderr.String())
	}
	return stdout.String()
}

// get answers path of the server, read whole.
func (s *site) get(t *testing.T, path string) string {
	t.Helper()

	res, err := httpGet(t.Context(), s.Base+path)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// everyPart returns the corpus's every-part, the sample of every part, and writes the image its part image shows
// beside where it is written.
func (s *site) everyPart(t *testing.T) string {
	t.Helper()

	text, err := os.ReadFile("../corpus/testdata/every-part/every-part.yaml")
	if err != nil {
		t.Fatal(err)
	}
	shot, err := os.ReadFile("../corpus/testdata/every-part/shots/moved.png")
	if err != nil {
		t.Fatal(err)
	}
	s.write(t, "shots/moved.png", string(shot))
	return string(text)
}

// floePages draws the sample of every part (the corpus's every-part) and the example of every template of floe's in
// every theme of floe's, and calls visit on each, open in the browser, with what tells it.
func (s *site) floePages(t *testing.T, visit func(what string)) {
	t.Helper()

	b := s.b
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, templates := k.Names()
	file := s.write(t, "every-part.yaml", s.everyPart(t))
	for _, name := range themes {
		slug := "parts-" + name
		s.floe(t, "publish", file, "--slug", slug, "--theme", name)
		b.navigate("/a/" + slug + "/?raw")
		b.until("the parts", "!!document.querySelector('[data-floe-part=bars]')")
		visit(name)
	}
	for _, tmpl := range templates {
		tpl, _, err := k.Template(tmpl)
		if err != nil {
			t.Fatal(err)
		}
		example := s.write(t, tmpl+".yaml", string(tpl.Example()))
		for _, name := range themes {
			slug := "template-" + tmpl + "-" + name
			s.floe(t, "publish", example, "--slug", slug, "--theme", name)
			b.navigate("/a/" + slug + "/?raw")
			b.until("the page", "!!document.querySelector('h1')")
			visit(tmpl + " in " + name)
		}
	}
}
