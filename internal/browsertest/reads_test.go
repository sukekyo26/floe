package browsertest_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/sukekyo26/floe/internal/store"
)

// A page opened in its frame is read: the frame sends the version the page told it. Not in a tab of its own, nor one
// the frame's iframe moved to, even one naming the frame's path; not told to a site that frames it. A frame that is
// framed itself, as in VS Code's Simple Browser, is told as any.
func TestUIRead(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	for _, slug := range []string{"pv", "other", "own", "nested"} {
		s.floe(t, "publish", s.page(t, "read/"+slug+".yaml", slug, "<p id=\"p\">"+slug+"</p>"), "--slug", slug)
	}
	news := func(slug string) string {
		t.Helper()

		res, err := http.Get(s.Base + "/api/artifacts") //nolint:noctx,gosec // the test's own server
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		var list []store.Listed
		if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
			t.Fatal(err)
		}
		for _, l := range list {
			if l.Slug == slug {
				return l.News
			}
		}
		t.Fatalf("%s is not listed", slug)
		return ""
	}
	waitRead := func(what, slug string) {
		t.Helper()

		// Asked from the page, as until waits on what it can read there.
		b.until(what, "window.reads ??= {}, fetch('/api/artifacts').then((r) => r.json()).then((l) => { window.reads["+js(slug)+"] = l.find((a) => a.slug === "+js(slug)+").news ?? ''; }), window.reads["+js(slug)+"] === ''")
	}
	// In a tab of its own: no frame to tell.
	b.navigate("/a/pv/?raw")
	b.until("the page", "document.getElementById('p') !== null")
	if got := news("pv"); got != "new" {
		t.Errorf("in a tab of its own: news %q", got)
	}
	// In its frame.
	b.navigate("/a/pv/")
	waitRead("the page to be read in its frame", "pv")

	// The iframe moved to another page: what that page tells reaches the frame, which sends none. Its messages are
	// counted, its requests noted, so that nothing here waits on a time.
	moved := func(what, src string) {
		t.Helper()

		b.navigate("/a/own/")
		waitRead("the frame's own page to be read", "own")
		b.eval(`window.heard = 0; addEventListener('message', (e) => { if (e.data?.artifactRead) window.heard++; });
			window.sent = []; { const f = window.fetch; window.fetch = (u, i) => { if (i?.method === 'POST') window.sent.push(String(u)); return f(u, i); }; }
			document.getElementById('frame').src = ` + js(src))
		b.until(what, "window.heard === 1")
		if got := b.eval("window.sent.join()"); got != "" {
			t.Errorf("%s: sent %s", what, got)
		}
	}
	moved("the page moved to to tell", "/a/other/?raw")
	if got := news("other"); got != "new" {
		t.Errorf("moved to: news %q", got)
	}

	// A site that frames ?raw is told nothing: its message goes to this origin alone, and would come before the load.
	site := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		//nolint:errcheck // the test's own
		_, _ = w.Write([]byte(`<!doctype html><title>site</title><script>window.heard = 0;
			addEventListener('message', (e) => { if (e.data?.artifactRead) window.heard++; });</script>
			<iframe id="raw" src="` + s.Base + `/a/other/?raw" onload="window.loaded = true"></iframe>`))
	}))
	t.Cleanup(site.Close)
	b.navigate(site.URL)
	b.until("the framed page to load", "window.loaded === true")
	if got := b.eval("window.heard"); got != "0" {
		t.Errorf("a site framing it heard %s", got)
	}
	if got := news("other"); got != "new" {
		t.Errorf("framed by a site: news %q", got)
	}

	// Cut off while the store's lock is held, the server still counts as reached: the time limit is the request's own.
	s.floe(t, "publish", s.page(t, "read/held.yaml", "held", "<p id=\"p\">held</p>"), "--slug", "held")
	lock, err := os.OpenFile(filepath.Join(s.Dir, "store", ".lock"), os.O_RDONLY|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	b.navigate("/a/held/")
	b.until("the mark to go out", "readsGoing === true")
	b.eval(`new MutationObserver(() => { window.wentDown ||= document.getElementById('server').classList.contains('down'); })
		.observe(document.getElementById('server'), { attributes: true })`)
	b.until("the request to be cut off", "readsGoing === false")
	if got := b.eval("Boolean(window.wentDown) || document.getElementById('server').classList.contains('down')"); got != "false" {
		t.Error("a request cut off by its time limit showed the server as not reached")
	}
	if err = unix.Flock(int(lock.Fd()), unix.LOCK_UN); err != nil {
		t.Fatal(err)
	}

	// Refused, the words show as the page's mistakes do, in no toast of success; and go when one is kept.
	s.floe(t, "publish", s.page(t, "read/bad.yaml", "bad", "<p id=\"p\">bad</p>"), "--slug", "bad")
	reads := filepath.Join(s.Dir, "store", "reads.json")
	kept, err := os.ReadFile(reads)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(reads); err != nil {
		t.Fatal(err)
	}
	b.navigate("/a/bad/")
	shown := "document.getElementById('error').hidden ? '' : document.getElementById('error').textContent"
	got := b.until("the refusal", shown)
	if want := "HTTP 400: " + reads + ": is missing, broken or has no since: run floe stop, then floe serve, which writes it anew"; got != want {
		t.Errorf("shown %q, want %q", got, want)
	}
	if got := b.eval("document.querySelectorAll('.toast').length"); got != "0" {
		t.Errorf("%s toasts of success", got)
	}
	if err = os.WriteFile(reads, kept, 0o600); err != nil {
		t.Fatal(err)
	}
	b.eval("document.getElementById('frame').contentWindow.location.replace('?raw')")
	b.until("the mistake to go", "document.getElementById('error').hidden")

	// The frame framed itself: the list's own page holding it.
	b.navigate("/")
	b.eval(`{ const f = document.createElement('iframe'); f.src = '/a/nested/'; document.body.append(f); }`)
	waitRead("the page to be read in a frame framed", "nested")
}
