package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/store"
	"github.com/sukekyo26/floe/internal/usererr"
)

const (
	mineData = "lang: en\ntemplate: mine\nn: 1\n"
	minePage = "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head><body>%s</body></html>\n"
)

// api sends a request to h as the CLI does: from the server's origin, JSON in and out.
func api(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	req.Host = "127.0.0.1:7325"
	req.Header.Set("Origin", "http://127.0.0.1:7325")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// publish sends the data file source as the page slug, of the project p.
func publish(t *testing.T, h http.Handler, slug, source string) *httptest.ResponseRecorder {
	t.Helper()

	b, err := json.Marshal(Upload{File: "m.yaml", Source: []byte(source), Project: "p"}) //nolint:exhaustruct // none else
	if err != nil {
		t.Fatal(err)
	}
	return api(t, h, http.MethodPut, "/api/artifacts/"+slug, string(b))
}

// told is the report of w, laid out.
func told(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()

	var r usererr.Report
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatalf("%d %s: %v", w.Code, w.Body, err)
	}
	return r.Render("floe: ")
}

// mine writes the template mine's page.html with body in it, each of another length than the one before, so that
// a filesystem of times in whole seconds marks it changed too.
func mine(t *testing.T, dir, body string) {
	t.Helper()

	write(t, filepath.Join(dir, "mine", "page.html"), strings.Replace(minePage, "%s", body, 1))
}

// A publish right after the template is saved draws with it, before the watch would look; one with nothing
// changed reads nothing again.
func TestLatestPublish(t *testing.T) {
	t.Parallel()

	s, dir := watched(t, false)
	h := s.Handler()
	if w := publish(t, h, "m", mineData); w.Code != http.StatusCreated {
		t.Fatalf("publish = %d %s", w.Code, w.Body)
	}
	_, gen := s.now()
	if w := publish(t, h, "m", mineData); w.Code != http.StatusOK {
		t.Fatalf("publish again = %d %s", w.Code, w.Body)
	}
	if _, again := s.now(); again != gen {
		t.Errorf("a publish with nothing changed read the drawing again: %d, then %d", gen, again)
	}
	mine(t, dir, "{{.Data.n}} saved just now")
	if w := publish(t, h, "m", mineData); w.Code != http.StatusOK {
		t.Fatalf("publish after the save = %d %s", w.Code, w.Body)
	}
	if _, after := s.now(); after != gen+1 {
		t.Errorf("a publish after the save read the drawing %d times, want once", after-gen)
	}
	page, ok := s.cfg.Store.File("m", s.cfg.Store.ReadMeta("m"), "")
	if !ok {
		t.Fatal("no page stored")
	}
	if b, err := os.ReadFile(page); err != nil || !strings.Contains(string(b), "1 saved just now") {
		t.Errorf("stored page = %q, %v; want the template saved just now", b, err)
	}
}

// A rebuild right after the template is broken is refused, and the page keeps the page it had.
func TestLatestRebuild(t *testing.T) {
	t.Parallel()

	s, dir := watched(t, false)
	h := s.Handler()
	if w := publish(t, h, "m", mineData); w.Code != http.StatusCreated {
		t.Fatalf("publish = %d %s", w.Code, w.Body)
	}
	mine(t, dir, "{{.Data.nope.x}}")
	w := api(t, h, http.MethodPost, "/api/rebuild", "{}")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("rebuild = %d %s, want 400", w.Code, w.Body)
	}
	root := filepath.Dir(dir)
	got := strings.ReplaceAll(told(t, w), root, "<tmp>")
	got = strings.ReplaceAll(got, s.cfg.Store.ReadMeta("m").Source, "<source>")
	want := `<tmp>/dir/mine/page.html:2:75: executing "page.html" at <.Data.nope.x>: map has no entry for key "nope"` + "\n" +
		`<tmp>/store/m/<source>/data.yaml: page "m" keeps the page it had: fix the data file it was published from ` +
		"and publish it again, or fix the template"
	if got != want {
		t.Errorf("rebuild told\n%s\nwant\n%s", got, want)
	}
}

// A link added right after the template is broken is refused.
func TestLatestLink(t *testing.T) {
	t.Parallel()

	s, dir := watched(t, false)
	data := filepath.Join(filepath.Dir(dir), "proj", "l.yaml")
	if err := os.MkdirAll(filepath.Dir(data), 0o700); err != nil {
		t.Fatal(err)
	}
	write(t, data, mineData)
	mine(t, dir, "{{.Data.nope.x}}")
	w := api(t, s.Handler(), http.MethodPost, "/api/artifacts", `{"path": "`+data+`"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("link = %d %s, want 400", w.Code, w.Body)
	}
	got := strings.ReplaceAll(told(t, w), filepath.Dir(dir), "<tmp>")
	want := `<tmp>/dir/mine/page.html:2:75: executing "page.html" at <.Data.nope.x>: map has no entry for key "nope"`
	if got != want {
		t.Errorf("link told\n%s\nwant\n%s", got, want)
	}
}

// A request's look waits for the watch's running one, so that the drawing it takes is of what it saw: the watch's
// look of the mark before a save does not write its drawing over the request's.
func TestLatestWaitsForLook(t *testing.T) {
	t.Parallel()

	s, dir := watched(t, false)
	read := s.cfg.Drawing
	var mu sync.Mutex
	var kits []*kit.Kit
	stopped, release, second := make(chan struct{}), make(chan struct{}), make(chan struct{}, 1)
	var once sync.Once
	let := func() { once.Do(func() { close(release) }) }
	t.Cleanup(let)
	s.cfg.Drawing = func() (store.Drawing, error) {
		d, err := read()
		mu.Lock()
		kits = append(kits, d.Kit)
		n := len(kits)
		mu.Unlock()
		if n == 1 {
			close(stopped)
			<-release
		} else {
			select {
			case second <- struct{}{}:
			default:
			}
		}
		return d, err
	}
	mine(t, dir, "v1, of another length")
	watch := make(chan struct{})
	go func() { s.look(); close(watch) }()
	<-stopped
	mine(t, dir, "v2, of yet another length than v1")
	got := make(chan store.Drawing)
	go func() { got <- s.latest() }()
	// LIMIT: a lock taken away passes when latest() does not reach its read within 50ms; the lock in place never
	// waits more. Make it a hook of look's if that comes to flake.
	select {
	case <-second:
		t.Fatal("latest read before the running look ended")
	case <-time.After(50 * time.Millisecond):
	}
	let()
	d := <-got
	<-watch
	now, _ := s.now()
	mu.Lock()
	defer mu.Unlock()
	if len(kits) != 2 || d.Kit != kits[1] || now.Kit != kits[1] {
		t.Errorf("latest = %p, now = %p; read %v, want both the second", d.Kit, now.Kit, kits)
	}
}

// The report of w, a rebuild or a publish among others, has no failure of floe's.
func noFailure(t *testing.T, w *httptest.ResponseRecorder, ok int) {
	t.Helper()

	switch w.Code {
	case ok:
	case http.StatusBadRequest:
		var r usererr.Report
		if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil || r.Failure != "" || len(r.Mistakes) == 0 {
			t.Errorf("%d %s: %v", w.Code, w.Body, err)
		}
	default:
		t.Errorf("%d %s", w.Code, w.Body)
	}
}
