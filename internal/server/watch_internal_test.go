package server

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/store"
)

// watched is a server whose own templates are under tpl (a symlink to the directory dir when link), with one
// template mine.
func watched(t *testing.T, link bool) (*Server, string) {
	t.Helper()

	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(tmp, "dir")
	tpl := dir
	if link {
		tpl = filepath.Join(tmp, "tpl")
		if err := os.Symlink(dir, tpl); err != nil {
			t.Fatal(err)
		}
	}
	for name, text := range map[string]string{
		"mine/page.html":    "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head><body>{{.Data.n}}</body></html>\n",
		"mine/schema.json":  `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
		"mine/example.yaml": "lang: en\ntemplate: mine\nn: 1\n",
	} {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	drawing := func() (store.Drawing, error) {
		floe, err := kit.Floe()
		if err != nil {
			return store.Drawing{Kit: nil, Icons: nil, Err: nil}, err
		}
		none := kit.Dir{FS: nil, Name: ""}
		k, err := kit.Load(floe, kit.Sources{Parts: none, Themes: none, Templates: kit.Dir{FS: os.DirFS(tpl), Name: tpl}})
		return store.Drawing{Kit: k, Icons: nil, Err: nil}, err
	}
	s := New(Config{
		Store: store.New(filepath.Join(tmp, "store")), Port: 7325, Build: "test", Drawing: drawing, Templates: tpl,
		Parts: "", Themes: "", Icons: "", Home: tmp,
	})
	return s, dir
}

func write(t *testing.T, path, text string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// The drawing is read again when what it is read from changes, however it is changed, and only then: not for a
// name kit leaves alone. A root that is a symlink is watched inside.
func TestLook(t *testing.T) {
	t.Parallel()

	for _, link := range []bool{false, true} {
		s, dir := watched(t, link)
		gen := func() int { _, g := s.now(); return g }
		page := filepath.Join(dir, "mine", "page.html")
		steps := []struct {
			what    string
			change  func()
			changed bool
		}{
			{"nothing", func() {}, false},
			{"a .swp written", func() { write(t, filepath.Join(dir, "mine", ".page.html.swp"), "x") }, false},
			{"page.html written in place", func() { write(t, page, "<p>{{.Data.n}}</p>\n") }, true},
			{"its mtime set back", func() {
				old := time.Unix(1, 0)
				if err := os.Chtimes(page, old, old); err != nil {
					t.Fatal(err)
				}
			}, true},
			{"page.html replaced by a rename", func() {
				write(t, page+".new", "<p>{{.Data.n}}!</p>\n")
				if err := os.Rename(page+".new", page); err != nil {
					t.Fatal(err)
				}
			}, true},
			{"a file added", func() { write(t, filepath.Join(dir, "mine", "page.css"), "p {}") }, true},
			{"a file removed", func() {
				if err := os.Remove(filepath.Join(dir, "mine", "page.css")); err != nil {
					t.Fatal(err)
				}
			}, true},
			{"a directory made", func() {
				if err := os.Mkdir(filepath.Join(dir, "mine", "assets"), 0o700); err != nil {
					t.Fatal(err)
				}
			}, true},
			{"a file linked", func() {
				write(t, filepath.Join(dir, "a.css"), "a {}")
				write(t, filepath.Join(dir, "b.css"), "b {}")
				if err := os.Symlink(filepath.Join(dir, "a.css"), filepath.Join(dir, "mine", "page.css")); err != nil {
					t.Fatal(err)
				}
			}, true},
			{"the link led elsewhere", func() {
				at := filepath.Join(dir, "mine", "page.css")
				if err := os.Remove(at); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(dir, "b.css"), at); err != nil {
					t.Fatal(err)
				}
			}, true},
			{"the file it leads to written", func() { write(t, filepath.Join(dir, "b.css"), "b { color: red }") }, true},
			{"the directory replaced by a copy", func() {
				mine := filepath.Join(dir, "mine")
				if err := os.Rename(mine, mine+".old"); err != nil {
					t.Fatal(err)
				}
				if err := os.CopyFS(mine, os.DirFS(mine+".old")); err != nil {
					t.Fatal(err)
				}
				if err := os.RemoveAll(mine + ".old"); err != nil {
					t.Fatal(err)
				}
			}, true},
		}
		for _, step := range steps {
			before := gen()
			step.change()
			s.look()
			if got := gen() != before; got != step.changed {
				t.Errorf("link %v, %s: read again %v, want %v", link, step.what, got, step.changed)
			}
		}
	}
}

// A drawing that cannot be read is what every drawing refuses with, until it is mended.
func TestLookBroken(t *testing.T) {
	t.Parallel()

	s, dir := watched(t, false)
	write(t, filepath.Join(dir, "mine", "schema.json"), "{")
	s.look()
	if d, _ := s.now(); d.Err == nil {
		t.Fatal("a broken template is read")
	}
	write(t, filepath.Join(dir, "mine", "schema.json"), `{"type": "object"}`)
	s.look()
	if d, _ := s.now(); d.Err != nil || d.Kit == nil {
		t.Errorf("mended, still %v", d.Err)
	}
}

// A version changes with a publish, a linked data file saved, the drawing read again and a restart; not with
// nothing.
func TestVersion(t *testing.T) {
	t.Parallel()

	s, dir := watched(t, false)
	home := filepath.Dir(dir)
	data := filepath.Join(home, "proj", "d.yaml")
	if err := os.MkdirAll(filepath.Dir(data), 0o700); err != nil {
		t.Fatal(err)
	}
	write(t, data, "lang: en\ntemplate: mine\nn: 1\n")
	d, _ := s.now()
	e, err := s.cfg.Store.PrepareLink(t.Context(), d, data, store.Options{}, "", home) //nolint:exhaustruct // none given
	if err != nil {
		t.Fatal(err)
	}
	if err := s.cfg.Store.Save(e); err != nil {
		t.Fatal(err)
	}
	v := s.version("d", s.cfg.Store.ReadMeta("d"), "")
	if s.version("d", s.cfg.Store.ReadMeta("d"), "") != v {
		t.Error("a version changed with nothing")
	}
	write(t, data, "lang: en\ntemplate: mine\nn: 2\n")
	if s.version("d", s.cfg.Store.ReadMeta("d"), "") == v {
		t.Error("the data file saved, the version is the same")
	}
	v = s.version("d", s.cfg.Store.ReadMeta("d"), "")
	write(t, filepath.Join(dir, "mine", "page.html"), "<p>{{.Data.n}}</p>\n")
	s.look()
	if s.version("d", s.cfg.Store.ReadMeta("d"), "") == v {
		t.Error("the drawing read again, the version is the same")
	}
	again, _ := watched(t, false)
	if again.id == s.id {
		t.Error("two servers of one id")
	}
}

// Serving ends with its context, the watching of the drawing with it.
func TestServeEnds(t *testing.T) {
	t.Parallel()

	s, _ := watched(t, false)
	s.every = time.Millisecond
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx, ln) }()
	time.Sleep(10 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Serve did not end")
	}
}

// A root of one's own that is not there is watched for: made, the drawing is read again.
func TestLookRootMade(t *testing.T) {
	t.Parallel()

	s, dir := watched(t, false)
	s.cfg.Parts = filepath.Join(filepath.Dir(dir), "parts")
	s.look()
	_, before := s.now()
	if err := os.Mkdir(s.cfg.Parts, 0o700); err != nil {
		t.Fatal(err)
	}
	s.look()
	if _, after := s.now(); after == before {
		t.Error("a root made, the drawing is not read again")
	}
}

// A drawing that failed for floe's reasons (no usererr) is read again at the next look, what it reads unchanged,
// even with no directories to mark.
func TestLookFailure(t *testing.T) {
	t.Parallel()

	s, _ := watched(t, false)
	s.cfg.Parts, s.cfg.Themes, s.cfg.Templates, s.cfg.Icons = "", "", "", ""
	calls := 0
	read := s.cfg.Drawing
	s.cfg.Drawing = func() (store.Drawing, error) {
		calls++
		if calls == 1 {
			return store.Drawing{Kit: nil, Icons: nil, Err: nil}, os.ErrDeadlineExceeded
		}
		return read()
	}
	s.mu.Lock()
	s.fresh = false
	s.mu.Unlock()
	s.look()
	if d, _ := s.now(); d.Err == nil {
		t.Fatal("a failure is not kept")
	}
	s.look()
	if d, _ := s.now(); d.Err != nil || calls != 2 {
		t.Errorf("after a failure: %v, %d reads", d.Err, calls)
	}
}

// The version served with a page is of the meta.json its content was read by: of a publish before, it differs
// from the one after, so that the tab reloads to it.
func TestVersionOfContent(t *testing.T) {
	t.Parallel()

	s, _ := watched(t, false)
	d, _ := s.now()
	publish := func() *store.Meta {
		e, err := s.cfg.Store.PrepareData(d, store.Upload{File: "p.yaml", Source: []byte("lang: en\ntemplate: mine\nn: 1\n"), Images: nil, Project: "p"}, store.Options{}) //nolint:exhaustruct // none given
		if err != nil {
			t.Fatal(err)
		}
		if err := s.cfg.Store.Save(e); err != nil {
			t.Fatal(err)
		}
		return s.cfg.Store.ReadMeta("p")
	}
	old := publish()
	v := s.version("p", old, "")
	publish()
	if s.version("p", s.cfg.Store.ReadMeta("p"), "") == v {
		t.Error("a publish after, the version is the same")
	}
}

// The watching of the drawing and requests of every kind run together (go test -race), the drawing changed
// meanwhile.
func TestWatchParallel(t *testing.T) {
	t.Parallel()

	s, dir := watched(t, false)
	s.every = time.Millisecond
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go s.watchDrawing(ctx)
	h := s.Handler()
	home := filepath.Dir(dir)
	data := filepath.Join(home, "proj", "d.yaml")
	if err := os.MkdirAll(filepath.Dir(data), 0o700); err != nil {
		t.Fatal(err)
	}
	write(t, data, "lang: en\ntemplate: mine\nn: 1\n")
	d, _ := s.now()
	e, err := s.cfg.Store.PrepareLink(t.Context(), d, data, store.Options{}, "", home) //nolint:exhaustruct // none given
	if err != nil {
		t.Fatal(err)
	}
	if err := s.cfg.Store.Save(e); err != nil {
		t.Fatal(err)
	}
	// A stored page of the template too, which a rebuild draws again (it leaves links alone).
	if w := publish(t, h, "m", mineData); w.Code != http.StatusCreated {
		t.Fatalf("publish = %d %s", w.Code, w.Body)
	}
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			for j := range 5 {
				mine(t, dir, "{{.Data.n}} "+strconv.Itoa(i*10+j))
				for _, path := range []string{"/a/d/?raw", "/api/artifacts", "/api/artifacts/d/download"} {
					req := httptest.NewRequestWithContext(ctx, http.MethodGet, path, nil)
					req.Host = "127.0.0.1:7325"
					h.ServeHTTP(httptest.NewRecorder(), req)
				}
				// A file read half written is the user's mistake (400), never floe's failure.
				noFailure(t, api(t, h, http.MethodPost, "/api/rebuild", "{}"), http.StatusNoContent)
				noFailure(t, publish(t, h, "m", mineData), http.StatusOK)
			}
		})
	}
	wg.Wait()
}

// meanwhile are icons whose lookup does what the test gives, once: as an editor would save while a page is drawn.
type meanwhile struct{ then func() }

func (m *meanwhile) Lookup(string) ([]byte, error) {
	if m.then != nil {
		then := m.then
		m.then = nil
		then()
	}
	return []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), nil
}

// A page's version is taken before its content: a data file saved while it is drawn leaves the tab a version that
// differs from the one after, so that it reloads, rather than one of the new file over the old page.
func TestVersionBeforeContent(t *testing.T) {
	t.Parallel()

	s, dir := watched(t, false)
	write(t, filepath.Join(dir, "mine", "schema.json"), `{"type": "object", "properties": {"d": {"$ref": "floe:///parts/diagram.json"}}}`)
	write(t, filepath.Join(dir, "mine", "page.html"), "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head><body>{{.Data.d}}</body></html>\n")
	icons := &meanwhile{then: nil}
	read := s.cfg.Drawing
	s.cfg.Drawing = func() (store.Drawing, error) {
		d, err := read()
		d.Icons = icons
		return d, err
	}
	s.look()
	home := filepath.Dir(dir)
	data := filepath.Join(home, "proj", "d.yaml")
	if err := os.MkdirAll(filepath.Dir(data), 0o700); err != nil {
		t.Fatal(err)
	}
	spec := "d: {width: 300, height: 100, title: i, parts: [{icon: {icon: aws/res/aws-lambda, x: 10, y: 10}}]}\n"
	write(t, data, "lang: en\ntemplate: mine\n"+spec)
	d, _ := s.now()
	e, err := s.cfg.Store.PrepareLink(t.Context(), d, data, store.Options{}, "", home) //nolint:exhaustruct // none given
	if err != nil {
		t.Fatal(err)
	}
	if err := s.cfg.Store.Save(e); err != nil {
		t.Fatal(err)
	}
	icons.then = func() { write(t, data, "lang: en\ntemplate: mine\ntitle: saved meanwhile\n"+spec) }
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/a/d/?raw", nil)
	req.Host = "127.0.0.1:7325"
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	now := url.QueryEscape(s.version("d", s.cfg.Store.ReadMeta("d"), ""))
	if strings.Contains(w.Body.String(), "since="+now) {
		t.Error("the page drawn before the save is served with the version after it: its tab would not reload")
	}
}
