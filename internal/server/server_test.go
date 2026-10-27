package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/server"
	"github.com/sukekyo26/floe/internal/store"
)

const (
	port   = 7325
	origin = "http://127.0.0.1:7325"
)

// env is a server on a store in a temporary directory, its own templates under tpl, and home, under which links go.
type env struct {
	srv        *server.Server
	h          http.Handler
	root, home string
	tpl        string
}

// pic is a template of one's own: a page of its title, its note and its images.
const pic = `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"note": {"type": "string"}, "i": {"type": "array", "items": {"type": "string", "format": "image"}}}}`

func newEnv(t *testing.T) env {
	t.Helper()

	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	e := env{root: filepath.Join(tmp, "store"), home: filepath.Join(tmp, "home"), tpl: filepath.Join(tmp, "tpl")}
	writeFiles(t, e.tpl, map[string]string{
		"pic/page.html": "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head><body>" +
			"{{with .Data.note}}{{.}}{{end}}{{range .Data.i}}<img src=\"{{.}}\">{{end}}</body></html>\n",
		"pic/schema.json":  pic,
		"pic/example.yaml": "lang: en\ntemplate: pic\n",
	})
	if err := os.MkdirAll(e.home, 0o700); err != nil {
		t.Fatal(err)
	}
	drawing := func() (store.Drawing, error) {
		floe, err := kit.Floe()
		if err != nil {
			return store.Drawing{}, err //nolint:exhaustruct // none
		}
		none := kit.Dir{FS: nil, Name: ""}
		k, err := kit.Load(floe, kit.Sources{Parts: none, Themes: none, Templates: kit.Dir{FS: os.DirFS(e.tpl), Name: e.tpl}})
		if err != nil {
			return store.Drawing{}, err //nolint:exhaustruct // none
		}
		return store.Drawing{Kit: k, Icons: nil, Err: nil}, nil
	}
	e.srv = server.New(server.Config{
		Store: store.New(e.root), Port: port, Build: "test", Drawing: drawing, Templates: e.tpl, Parts: "", Themes: "",
		Icons: "", Home: e.home,
	})
	e.h = e.srv.Handler()
	return e
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()

	for name, text := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// do sends a request of method to path with body (nil for none) and the headers given as name, value pairs; the
// Host is the server's.
func (e env) do(t *testing.T, method, path, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()

	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequestWithContext(t.Context(), method, path, r)
	req.Host = "127.0.0.1:7325"
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	return w
}

// write sends a write as the CLI does: from the server's origin, as JSON, taking a report as JSON.
func (e env) write(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return e.do(t, method, path, string(b), "Origin", origin, "Content-Type", "application/json", "Accept", "application/json")
}

// publish publishes src as slug, from project p, with the images given.
func (e env) publish(t *testing.T, slug, src string, images map[string][]byte) *httptest.ResponseRecorder {
	t.Helper()

	return e.write(t, http.MethodPut, "/api/artifacts/"+slug,
		server.Upload{File: slug + ".yaml", Source: []byte(src), Images: images, Project: "p"}) //nolint:exhaustruct // the rest not given
}

func want(t *testing.T, w *httptest.ResponseRecorder, status int, body string) {
	t.Helper()

	if w.Code != status || !strings.Contains(w.Body.String(), body) {
		t.Errorf("answered %d %q, want %d with %q", w.Code, w.Body.String(), status, body)
	}
}

// What no request should see is refused as the protocol is, in fixed English: another host, another origin, a
// body that is not JSON as mime reads its type.
func TestProtocol(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/artifacts", nil)
	req.Host = "evil.example:7325"
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	want(t, w, http.StatusForbidden, "forbidden host")
	want(t, e.do(t, http.MethodDelete, "/api/artifacts/x", ""), http.StatusForbidden, "forbidden origin")
	for typ, status := range map[string]int{
		"text/plain": 415, "application/jsonx": 415, "application/json-seq": 415, "application/json; charset": 415,
		"Application/JSON; charset=utf-8": 404, "application/json": 404,
	} {
		w := e.do(t, http.MethodPatch, "/api/artifacts/none", `{"favorite": true}`, "Origin", origin, "Content-Type", typ)
		if w.Code != status {
			t.Errorf("Content-Type %q: %d, want %d", typ, w.Code, status)
		}
	}
	want(t, e.do(t, http.MethodGet, "/a/x/%2e%2e/y", ""), http.StatusNotFound, "not found")
	want(t, e.write(t, http.MethodPost, "/api/artifacts", map[string]any{"path": "/x", "bogus": 1}), http.StatusBadRequest,
		`cannot read the request: json: unknown field "bogus"`)
	want(t, e.write(t, http.MethodPut, "/api/artifacts/x", map[string]any{
		"file": "x.yaml", "source": []byte("a"),
		"images": map[string][]byte{"../e.png": nil},
	}), http.StatusBadRequest, "send each image by a slash-separated path")
}

// A publish is 201, again 200, of another project 409, its mistakes 400; a report as JSON only for an Accept of JSON
// exactly.
func TestPublish(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	want(t, e.publish(t, "d", "lang: en\ntemplate: pic\ntitle: T\n", nil), http.StatusCreated, `"slug":"d"`)
	want(t, e.publish(t, "d", "lang: en\ntemplate: pic\ntitle: T\n", nil), http.StatusOK, `"prev":{"title":"T"`)
	other := e.write(t, http.MethodPut, "/api/artifacts/d", server.Upload{File: "d.yaml", Source: []byte("lang: en\ntemplate: pic\n"), Project: "q"}) //nolint:exhaustruct // the rest not given
	want(t, other, http.StatusConflict, `{"mistakes":[{"text":"slug \"d\" was published from project p; use another --slug, or add --force to replace that page"}]}`)
	want(t, e.publish(t, "e", "lang: en\ntemplate: nope\n", nil), http.StatusBadRequest, `unknown template \"nope\"`)
	asText := e.do(t, http.MethodPost, "/api/artifacts", `{"path": "rel.html"}`, "Origin", origin, "Content-Type", "application/json",
		"Accept", "application/json, text/plain")
	want(t, asText, http.StatusBadRequest, "rel.html is not an absolute path")
	if strings.HasPrefix(asText.Body.String(), "{") {
		t.Errorf("a report as JSON to an Accept that is not JSON exactly: %s", asText.Body)
	}
	need := e.publish(t, "f", "lang: en\ntemplate: pic\ni: [img/a.png, img/b.png]\n", nil)
	want(t, need, http.StatusUnprocessableEntity, `{"images":["img/a.png","img/b.png"]}`)
	png := []byte("\x89PNG\r\n\x1a\n0000")
	want(t, e.publish(t, "f", "lang: en\ntemplate: pic\ni: [img/a.png]\n", map[string][]byte{"img/a.png": png}), http.StatusCreated, `"slug":"f"`)
}

// A stored page is the page drawn when published, its bytes as kept, in the sandbox and cut off, with the script
// that reloads it; a template changed since draws it no other, until a rebuild.
func TestStoredPage(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.publish(t, "d", "lang: en\ntemplate: pic\ntitle: T\nnote: one\n", nil)
	m := store.New(e.root).ReadMeta("d")
	kept, err := os.ReadFile(filepath.Join(e.root, "d", m.Source, "page.html"))
	if err != nil {
		t.Fatal(err)
	}
	w := e.do(t, http.MethodGet, "/a/d/", "")
	if !strings.HasPrefix(w.Body.String(), string(kept)) || !strings.Contains(w.Body.String(), "new window.EventSource") {
		t.Errorf("served\n%s", w.Body)
	}
	csp := w.Header().Get("Content-Security-Policy")
	if !strings.HasPrefix(csp, "sandbox allow-scripts") || !strings.Contains(csp, "connect-src http://127.0.0.1:7325/api/events/") {
		t.Errorf("CSP %q", csp)
	}
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("no nosniff")
	}
	download := e.do(t, http.MethodGet, "/api/artifacts/d/download", "")
	if !strings.Contains(download.Body.String(), `http-equiv="Content-Security-Policy"`) || strings.Contains(download.Body.String(), "EventSource") {
		t.Errorf("downloaded\n%s", download.Body)
	}
}

// A linked data file is drawn each time it is opened; with mistakes it is the page of them in the reader's
// language, which a download refuses.
func TestLinkedData(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	data := filepath.Join(e.home, "proj", "d.yaml")
	writeFiles(t, filepath.Dir(data), map[string]string{"d.yaml": "lang: en\ntemplate: pic\nnote: one\n", "h.html": "<title>h</title><p>x</p>", "s.css": "p{}"})
	want(t, e.write(t, http.MethodPost, "/api/artifacts", server.Link{Path: data}), http.StatusCreated, `"slug":"d"`) //nolint:exhaustruct // the rest not given
	want(t, e.do(t, http.MethodGet, "/a/d/", ""), http.StatusOK, "one")
	writeFiles(t, filepath.Dir(data), map[string]string{"d.yaml": "lang: en\ntemplate: pic\nnote: two\n"})
	want(t, e.do(t, http.MethodGet, "/a/d/", ""), http.StatusOK, "two")
	writeFiles(t, filepath.Dir(data), map[string]string{"d.yaml": "lang: en\ntemplate: nope\n"})
	want(t, e.do(t, http.MethodGet, "/a/d/", "", "Accept-Language", "ja"), http.StatusOK, "データファイルに誤りがあります")
	want(t, e.do(t, http.MethodGet, "/api/artifacts/d/download", ""), http.StatusBadRequest, `unknown template "nope"`)
	want(t, e.write(t, http.MethodPost, "/api/artifacts", server.Link{Path: filepath.Join(e.home, "proj", "h.html")}), http.StatusCreated, `"slug":"h"`) //nolint:exhaustruct // the rest not given
	want(t, e.do(t, http.MethodGet, "/a/h/s.css", ""), http.StatusOK, "p{}")
	want(t, e.do(t, http.MethodGet, "/a/h/nope.css", ""), http.StatusNotFound, "not found")
	plain := e.do(t, http.MethodGet, "/api/artifacts/h/download", "")
	if plain.Body.String() != "<title>h</title><p>x</p>" {
		t.Errorf("a linked HTML file downloaded as %q", plain.Body)
	}
}

// A page not there is 404 to remove; one that cannot be removed is told why.
func TestRemove(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root removes what it may not")
	}
	e := newEnv(t)
	want(t, e.do(t, http.MethodDelete, "/api/artifacts/none", "", "Origin", origin), http.StatusNotFound, "not found")
	e.publish(t, "d", "lang: en\ntemplate: pic\n", nil)
	dir := filepath.Join(e.root, "d")
	m := store.New(e.root).ReadMeta("d")
	if err := os.Chmod(filepath.Join(dir, m.Source), 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, m.Source), 0o700) }) //nolint:errcheck // told already
	want(t, e.do(t, http.MethodDelete, "/api/artifacts/d", "", "Origin", origin), http.StatusBadRequest, "cannot be written (permission denied)")
	want(t, e.do(t, http.MethodGet, "/api/artifacts", ""), http.StatusOK, `"slug":"d"`)
	if err := os.Chmod(filepath.Join(dir, m.Source), 0o700); err != nil {
		t.Fatal(err)
	}
	want(t, e.do(t, http.MethodDelete, "/api/artifacts/d", "", "Origin", origin), http.StatusNoContent, "")
	want(t, e.do(t, http.MethodGet, "/api/artifacts", ""), http.StatusOK, "[]")
}

// Requests in parallel, of every kind, share the drawing (go test -race).
func TestParallel(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	data := filepath.Join(e.home, "proj", "d.yaml")
	writeFiles(t, filepath.Dir(data), map[string]string{"d.yaml": "lang: en\ntemplate: pic\nnote: one\n"})
	e.write(t, http.MethodPost, "/api/artifacts", server.Link{Path: data}) //nolint:exhaustruct // the rest not given
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			e.do(t, http.MethodGet, "/a/d/", "")
			e.publish(t, "s", "lang: en\ntemplate: pic\n", nil)
			e.do(t, http.MethodGet, "/api/artifacts", "")
		})
	}
	wg.Wait()
}
