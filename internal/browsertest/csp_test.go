package browsertest_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// A page drawn from a data file sends nothing out with fetch, XHR, an image or a form, reaches no
// management API, and still reloads on the event stream; its download sends nothing out either. Linked
// HTML is not cut off, which shows that a request that got out would be seen.
func TestUICutOff(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	// The drawn page and its download try one server, the linked HTML another: what reached the first, however
	// late, is told at the end.
	outside, got := listen(t)
	linkedOut, gotLinked := listen(t)
	// The page tries each way out to out once loaded, and lists what CSP blocked. Once loaded: a form submitted
	// while the page is parsed stops the parsing, before the reload script. The form goes once the others
	// have ended: if it got out, it would move the page away and could cut them short.
	tries := func(out string) string {
		return "<script>window.blocked = [];" +
			"document.addEventListener('securitypolicyviolation', (e) => blocked.push(e.effectiveDirective));" +
			"addEventListener('load', () => {const out = " + js(out) + ";" +
			"const x = new XMLHttpRequest(); x.open('GET', out + '/xhr'); const img = new Image();" +
			"Promise.allSettled([fetch(out + '/fetch'), fetch('/api/artifacts')," +
			"new Promise((r) => { x.onloadend = r; x.send(); })," +
			"new Promise((r) => { img.onload = img.onerror = r; img.src = out + '/img'; })]).then(() => {" +
			"const f = document.createElement('form'); f.method = 'post'; f.action = out + '/form'; document.body.append(f); f.submit();});});" +
			"</script>"
	}
	all := "window.blocked.length === 5 && window.blocked.sort().join()"
	const want = "connect-src,connect-src,connect-src,form-action,img-src"
	out := make([]string, 0, 6)

	publish := func(version string) {
		s.floe(t, "publish", s.page(t, "cut.yaml", "Cut", `<p id="v">`+version+"</p>"+tries(outside)), "--slug", "cut")
	}
	publish("1")
	b.navigate("/a/cut/?raw")
	out = append(out, b.until("the blocked requests", all))
	// What got out would have come by the time CSP blocked the rest: they start together.
	time.Sleep(300 * time.Millisecond)
	out = append(out, "out:"+got())
	publish("2")
	b.until("the page to reload", "document.getElementById('v')?.textContent === '2'")
	out = append(out, "reloaded")

	// The download is opened from another server, without floe's headers: its <meta> cuts it off.
	download := s.get(t, "/api/artifacts/cut/download")
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(download)) //nolint:errcheck,gosec // the test checks what the page does
	}))
	defer plain.Close()
	b.navigate(plain.URL + "/cut.html")
	out = append(out, b.until("the download's blocked requests", all))
	time.Sleep(300 * time.Millisecond)
	out = append(out, "out:"+got())

	// Linked HTML gets out: the requests would be seen.
	s.floe(t, "publish", "--link", s.write(t, "site/out.html", tries(linkedOut)), "--slug", "out")
	b.navigate("/a/out/?raw")
	const gotOut = "/fetch /form /img /xhr"
	for deadline := time.Now().Add(waitLimit); gotLinked() != gotOut && time.Now().Before(deadline); {
		time.Sleep(50 * time.Millisecond)
	}
	out = append(out, "linked out:"+gotLinked())
	// Nothing of the drawn page or its download reached its server, late or not.
	out = append(out, "out in all:"+got())

	check(t, "cut off", out, want+"|out:|reloaded|"+want+"|out:|linked out:"+gotOut+"|out in all:")
}

// listen starts a server outside floe for the test and returns its URL and the paths that reached it so far,
// sorted. The browser's ask for the icon of a page a form moved to is not counted: the page did not ask.
func listen(t *testing.T) (string, func() string) {
	t.Helper()

	var mu sync.Mutex
	var reached []string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/favicon.ico" {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		reached = append(reached, r.URL.Path)
	}))
	t.Cleanup(srv.Close)
	return srv.URL, func() string {
		mu.Lock()
		defer mu.Unlock()
		slices.Sort(reached)
		return strings.Join(reached, " ")
	}
}

// Another site cannot frame the management page (Chrome shows its error page in the frame instead),
// while a page that moves its own frame to the management page shows it nested.
func TestUIFraming(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	out := make([]string, 0, 2)

	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, `<iframe src="`+s.Base+`/"></iframe>`) //nolint:errcheck // the test checks the frame
	}))
	defer other.Close()
	if err := chromedp.Do(b.ctx, chromedp.Navigate(other.URL)); err != nil {
		t.Fatal(err)
	}
	framed := ""
	for deadline := time.Now().Add(waitLimit); framed == "" && time.Now().Before(deadline); {
		framed = framedDocument(t, b)
		time.Sleep(50 * time.Millisecond)
	}
	out = append(out, "framed:"+framed)

	s.floe(t, "publish", "--link", s.write(t, "site/mover.html", "<script>location.href = '/'</script>"), "--slug", "mover")
	b.navigate("/a/mover/")
	out = append(out, b.until("the management page in the frame", "frames[0].document.title"))

	check(t, "framing", out, "framed:chrome-error://chromewebdata/|floe")
}

// A script part runs under the CSP of a page drawn from a data file: its function gets the element in
// its place and its data, a key __proto__ a key of it, the example's slider among them.
func TestUIScript(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	data := s.write(t, "script.yaml", "template: report\nsections:\n  - parts:\n      - text: before\n"+
		"      - script:\n          data: { words: [moves, '</script>'], __proto__: { x: 1 } }\n"+
		"          js: |\n            function (el, data) { el.id = 'moved'; el.textContent = data.words.join(' ') + ' ' + Object.keys(data); }; // drawn\nlang: en\n")
	s.floe(t, "publish", data, "--slug", "script")
	b.navigate("/a/script/?raw")
	got := b.eval("const el = document.getElementById('moved'); el.previousElementSibling.textContent + '|' + el.textContent")

	example, err := os.ReadFile("../kit/floe/templates/report/example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	s.floe(t, "publish", s.write(t, "example.yaml", string(example)), "--slug", "example")
	b.navigate("/a/example/?raw")
	slider := b.eval("const input = document.querySelector('input[type=range]'); const before = input.closest('div').querySelector('output').textContent;" +
		"input.value = 1000; input.dispatchEvent(new Event('input')); before + ' ' + input.closest('div').querySelector('output').textContent")

	check(t, "script", []string{got, slider}, "before|moves </script> __proto__,words|7 hours 2 hours")
}

// framedDocument is the URL of the document in the one frame of the tab's page, "" while there is none but
// about:blank. The page's frame tree holds it when the frame runs in the page's process; Chrome for Testing puts a
// frame's error page in a process of its own, a target of its own, out of that tree.
func framedDocument(t *testing.T, b *browser) string {
	t.Helper()

	res, err := chromedp.Call(b.ctx, page.GetFrameTree, cdp.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	if kids := res.FrameTree.ChildFrames; len(kids) == 1 && kids[0].Frame.URL != "about:blank" {
		return kids[0].Frame.URL
	}
	targets, err := chromedp.Targets(b.ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Chrome is shared: the frames of other tests' tabs are among its targets, in browser contexts of their own.
	own := chromedp.FromContext(b.ctx).BrowserContextID
	for _, info := range targets {
		if info.Type != "iframe" || info.BrowserContextID != own {
			continue
		}
		// Attached until the test ends: cancelling the context closes the target.
		tctx, cancel := chromedp.NewContext(b.ctx, chromedp.WithTargetID(info.TargetID))
		t.Cleanup(cancel)
		wait, stop := context.WithTimeout(tctx, waitLimit)
		href, err := chromedp.Run(wait, chromedp.Evaluate[string]("location.href"))
		stop()
		if err != nil {
			t.Fatal(err)
		}
		return href
	}
	return ""
}
