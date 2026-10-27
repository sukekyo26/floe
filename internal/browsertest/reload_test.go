package browsertest_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"
)

// listening attaches one message listener per document and returns the last message the page sent.
// It keeps the page's own words, not what floe's script in the page tells the frame (artifactHash).
const listening = "(window.listening ||= (addEventListener('message', (e) => { if (typeof e.data === 'string') window.reported = e.data; }), true)) && window.reported"

// TestUIReload checks that an open page reloads itself when republished, from inside the sandbox, and
// that the page URL is a frame which allows clipboard writes and fullscreen, sends links to other sites
// to new tabs out of the sandbox, and moves the whole tab for links to another page. A page not there, or a
// slug that cannot be one, is the frame showing the way back.
func TestUIReload(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	publish := func(name, slug, title, body string) {
		s.floe(t, "publish", s.page(t, "live/"+name, title, body), "--slug", slug)
	}
	shown := func(v string) string { return "document.getElementById('v')?.textContent === '" + v + "'" }

	out := make([]string, 0, 8)
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
	out = append(out, leave(t, s, publish))
	// An unknown page, and a slug that cannot be one, show the way back instead of the frame.
	for _, path := range []string{"/a/no-such/", "/a/Bad_Slug/"} {
		b.navigate(path)
		out = append(out, b.until("the not-found state of "+path,
			"!document.getElementById('notfound').hidden && document.getElementById('frame').hidden && !document.getElementById('frame').hasAttribute('src')"))
	}

	check(t, "reload", out, "1|2|Live · floe|true|true,true|own own none /a/no-such/|true|true")
}

// leave clicks links out of a page in the frame: a target=_blank tab runs in the other site's origin
// out of the sandbox, a link without target opens a new tab too (most sites refuse to be framed), a
// link within the server stays in the frame, and a link to a frame URL moves the whole tab. The other
// site refuses framing. It returns each tab's origin (none when no tab opened) and where the tab moved.
func leave(t *testing.T, s *site, publish func(name, slug, title, body string)) string {
	t.Helper()

	b := s.b
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
	// The inner link moves the page in the frame, and opens no tab.
	inner := s.Base + "/a/leave/inner.html"
	for deadline := time.Now().Add(waitLimit); frameURL(t, b) != inner; {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for the frame to move to %s: at %s", inner, frameURL(t, b))
		}
		time.Sleep(50 * time.Millisecond)
	}
	var origins []string
	for _, url := range []string{other.URL + "/blank", other.URL + "/plain", inner} {
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
	// A tab the inner link opened late would be there by now.
	if tab(inner) != nil {
		t.Error("the inner link opened a tab")
	}
	return strings.Join(origins, " ")
}

// frameURL is the URL of the page in the frame of the tab's page, "" for none.
func frameURL(t *testing.T, b *browser) string {
	t.Helper()

	res, err := chromedp.Call(b.ctx, page.GetFrameTree, cdp.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	if kids := res.FrameTree.ChildFrames; len(kids) == 1 {
		return kids[0].Frame.URL
	}
	return ""
}
