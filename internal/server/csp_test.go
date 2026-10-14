package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/store"
)

// A page drawn from a data file sends nothing out with fetch, XHR, an image or a form, reaches no
// management API, and still reloads on the event stream; its download sends nothing out either. Linked
// HTML is not cut off, which shows that a request that got out would be seen.
func TestUICutOff(t *testing.T) { //nolint:paralleltest // the UI tests share one Chrome, one at a time
	e := newEnv(t)
	b := newBrowser(t, e.base, i18n.English)
	var mu sync.Mutex
	var reached []string
	outside := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		// The browser asks for the icon of the page the form moved to; the page did not.
		if r.URL.Path == "/favicon.ico" {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		reached = append(reached, r.URL.Path)
	}))
	defer outside.Close()
	got := func() string {
		mu.Lock()
		defer mu.Unlock()
		slices.Sort(reached)
		return strings.Join(reached, " ")
	}
	// The page tries each way out once loaded, and lists what CSP blocked. Once loaded: a form submitted
	// while the page is parsed stops the parsing, before the reload script. The form goes once the others
	// have ended: if it got out, it would move the page away and could cut them short.
	tries := "<script>window.blocked = [];" +
		"document.addEventListener('securitypolicyviolation', (e) => blocked.push(e.effectiveDirective));" +
		"addEventListener('load', () => {const out = " + js(outside.URL) + ";" +
		"const x = new XMLHttpRequest(); x.open('GET', out + '/xhr'); const img = new Image();" +
		"Promise.allSettled([fetch(out + '/fetch'), fetch('/api/artifacts')," +
		"new Promise((r) => { x.onloadend = r; x.send(); })," +
		"new Promise((r) => { img.onload = img.onerror = r; img.src = out + '/img'; })]).then(() => {" +
		"const f = document.createElement('form'); f.method = 'post'; f.action = out + '/form'; document.body.append(f); f.submit();});});" +
		"</script>"
	all := "window.blocked.length === 5 && window.blocked.sort().join()"
	const want = "connect-src,connect-src,connect-src,form-action,img-src"
	out := make([]string, 0, 6)

	publish := func(version string) {
		time.Sleep(2 * time.Millisecond) // a republish in the same millisecond would keep the version
		e.publish(t, e.page(t, "cut.yaml", "Cut", `<p id="v">`+version+"</p>"+tries), store.Options{Slug: ptr("cut")})
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
	page := e.get(t, "/api/artifacts/cut/download").body
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page)) //nolint:errcheck,gosec // the test checks what the page does
	}))
	defer plain.Close()
	b.eval("window.leaving = true")
	if err := chromedp.Do(b.ctx, chromedp.Navigate(plain.URL+"/cut.html")); err != nil {
		t.Fatal(err)
	}
	out = append(out, b.until("the download's blocked requests", "!window.leaving && "+all))
	time.Sleep(300 * time.Millisecond)
	out = append(out, "out:"+got())

	// Linked HTML gets out: the requests would be seen.
	e.publish(t, e.write(t, filepath.Join("site", "out.html"), tries), store.Options{Link: true, Slug: ptr("out")})
	b.navigate("/a/out/?raw")
	const gotOut = "/fetch /form /img /xhr"
	for deadline := time.Now().Add(waitLimit); got() != gotOut && time.Now().Before(deadline); {
		time.Sleep(50 * time.Millisecond)
	}
	out = append(out, "out:"+got())

	check(t, "cut off", out, want+"|out:|reloaded|"+want+"|out:|out:"+gotOut)
}

// A script part runs under the CSP of a page drawn from a data file: its function gets the element in
// its place and its data, the example's slider among them.
func TestUIScript(t *testing.T) { //nolint:paralleltest // the UI tests share one Chrome, one at a time
	e := newEnv(t)
	b := newBrowser(t, e.base, i18n.English)
	data := e.write(t, "script.yaml", "template: report\nsections:\n  - parts:\n      - text: before\n"+
		"      - script:\n          data: { words: [moves, '</script>'] }\n"+
		"          js: |\n            function (el, data) { el.id = 'moved'; el.textContent = data.words.join(' '); }\n")
	e.publish(t, data, store.Options{Slug: ptr("script")})
	b.navigate("/a/script/?raw")
	got := b.eval("const el = document.getElementById('moved'); el.previousElementSibling.textContent + '|' + el.textContent")

	example, err := os.ReadFile("../tmpl/builtin/report/example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	e.publish(t, e.write(t, "example.yaml", string(example)), store.Options{Slug: ptr("example")})
	b.navigate("/a/example/?raw")
	slider := b.eval("const input = document.querySelector('input[type=range]'); const before = input.closest('div').querySelector('output').textContent;" +
		"input.value = 1000; input.dispatchEvent(new Event('input')); before + ' ' + input.closest('div').querySelector('output').textContent")

	check(t, "script", []string{got, slider}, "before|moves </script>|7 hours 2 hours")
}
