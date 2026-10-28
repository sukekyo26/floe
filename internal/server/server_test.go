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

	"golang.org/x/sys/unix"

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
	e.publish(t, "d", "lang: en\ntemplate: pic\n", nil)
	for typ, status := range map[string]int{
		"text/plain": 415, "application/jsonx": 415, "application/json-seq": 415, "application/json; charset": 415,
		"Application/JSON; charset=utf-8": 200, "application/json": 200,
	} {
		w := e.do(t, http.MethodPatch, "/api/artifacts/d", `{"favorite": true}`, "Origin", origin, "Content-Type", typ)
		if w.Code != status {
			t.Errorf("Content-Type %q: %d, want %d", typ, w.Code, status)
		}
	}
	// Another site cannot tell whether a page is there.
	want(t, e.do(t, http.MethodPatch, "/api/artifacts/none", "{}", "Origin", "http://evil.example", "Content-Type", "application/json"),
		http.StatusForbidden, "forbidden origin")
	want(t, e.do(t, http.MethodPatch, "/api/artifacts/none", "{}", "Origin", origin, "Content-Type", "application/json"), http.StatusNotFound, "not found")
	want(t, e.do(t, http.MethodPatch, "/api/artifacts/Bad%20slug", "x", "Origin", origin, "Content-Type", "text/plain"), http.StatusNotFound, "not found")
	want(t, e.do(t, http.MethodGet, "/a/x/%2e%2e/y", ""), http.StatusNotFound, "not found")
	want(t, e.write(t, http.MethodPost, "/api/artifacts", map[string]any{"path": "/x", "bogus": 1}), http.StatusBadRequest,
		`unknown object member name "bogus"`)
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
	w := e.do(t, http.MethodGet, "/a/d/?raw", "")
	if !strings.HasPrefix(w.Body.String(), string(kept)) || !strings.Contains(w.Body.String(), "new window.EventSource") ||
		!strings.Contains(w.Body.String(), "artifactOpen") {
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

// The management page and a page's URL are the frame, in the reader's language, which only the management page and
// VS Code may frame; a page not there is the frame with 404.
func TestManagementPageAndFrame(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.publish(t, "d", "lang: en\ntemplate: pic\n", nil)
	// A stored page whose page.html is gone is not there, as ?raw finds it.
	e.publish(t, "gone", "lang: en\ntemplate: pic\n", nil)
	if err := os.Remove(filepath.Join(e.root, "gone", store.New(e.root).ReadMeta("gone").Source, "page.html")); err != nil {
		t.Fatal(err)
	}
	frame := "frame-ancestors 'self' vscode-webview: vscode-file:"
	for target, status := range map[string]int{
		"/": http.StatusOK, "/a/d/": http.StatusOK, "/a/none/": http.StatusNotFound, "/a/Bad_Slug/": http.StatusNotFound,
		"/a/gone/": http.StatusNotFound,
	} {
		w := e.do(t, http.MethodGet, target, "")
		// The frame is drawn as one from the start, its slug for the script.
		html := `<html lang="en">`
		if slug := strings.Trim(strings.TrimPrefix(target, "/a/"), "/"); target != "/" {
			html = `<html lang="en" class="framed" data-slug="` + slug + `">`
		}
		want(t, w, status, html)
		if got := w.Header().Get("Content-Security-Policy"); got != frame {
			t.Errorf("GET %s: CSP %q", target, got)
		}
		want(t, e.do(t, http.MethodGet, target, "", "Accept-Language", "ja,en;q=0.5"), status, "一覧へ戻る")
	}
	// The slug as the server read it, unescaped, is escaped in the HTML.
	want(t, e.do(t, http.MethodGet, "/a/x%22%3E%3C%3Fy/", ""), http.StatusNotFound, `data-slug="x&#34;&gt;&lt;?y"`)
	// Not the frame, whose messages say "not found" too.
	for _, w := range []*httptest.ResponseRecorder{
		e.do(t, http.MethodPost, "/", ""), e.do(t, http.MethodGet, "/a/none/?raw", ""), e.do(t, http.MethodGet, "/a/gone/?raw", ""),
	} {
		if w.Code != http.StatusNotFound || w.Body.String() != "not found" {
			t.Errorf("answered %d %.40q, want 404 not found", w.Code, w.Body)
		}
	}
	// The query is kept, so that /a/d?raw stays the page, not its frame.
	for target, to := range map[string]string{"/a/d": "/a/d/", "/a/d?raw": "/a/d/?raw"} {
		if w := e.do(t, http.MethodGet, target, ""); w.Code != http.StatusMovedPermanently || w.Header().Get("Location") != to {
			t.Errorf("GET %s = %d to %q, want 301 to %q", target, w.Code, w.Header().Get("Location"), to)
		}
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
	want(t, e.do(t, http.MethodGet, "/a/d/?raw", ""), http.StatusOK, "one")
	writeFiles(t, filepath.Dir(data), map[string]string{"d.yaml": "lang: en\ntemplate: pic\nnote: two\n"})
	want(t, e.do(t, http.MethodGet, "/a/d/?raw", ""), http.StatusOK, "two")
	writeFiles(t, filepath.Dir(data), map[string]string{"d.yaml": "lang: en\ntemplate: nope\n"})
	want(t, e.do(t, http.MethodGet, "/a/d/?raw", "", "Accept-Language", "ja"), http.StatusOK, "データファイルに誤りがあります")
	want(t, e.do(t, http.MethodGet, "/api/artifacts/d/download", ""), http.StatusBadRequest, `unknown template "nope"`)
	want(t, e.write(t, http.MethodPost, "/api/artifacts", server.Link{Path: filepath.Join(e.home, "proj", "h.html")}), http.StatusCreated, `"slug":"h"`) //nolint:exhaustruct // the rest not given
	want(t, e.do(t, http.MethodGet, "/a/h/s.css", ""), http.StatusOK, "p{}")
	want(t, e.do(t, http.MethodGet, "/a/h/nope.css", ""), http.StatusNotFound, "not found")
	// What is not a regular file is not served, so that a FIFO does not block the server.
	if err := unix.Mkfifo(filepath.Join(e.home, "proj", "f.css"), 0o600); err != nil {
		t.Fatal(err)
	}
	want(t, e.do(t, http.MethodGet, "/a/h/f.css", ""), http.StatusNotFound, "not found")
	if err := os.Remove(data); err != nil {
		t.Fatal(err)
	}
	if err := unix.Mkfifo(data, 0o600); err != nil {
		t.Fatal(err)
	}
	want(t, e.do(t, http.MethodGet, "/api/artifacts/d/download", ""), http.StatusNotFound, "not found")
	plain := e.do(t, http.MethodGet, "/api/artifacts/h/download", "")
	if plain.Body.String() != "<title>h</title><p>x</p>" {
		t.Errorf("a linked HTML file downloaded as %q", plain.Body)
	}
}

// An edit archives a page as true or false, which the list then carries; favorite and archived null are not
// changed, as anything else are refused as encoding/json words it, and a page that is not there is not found.
func TestEditArchived(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.publish(t, "d", "lang: en\ntemplate: pic\n", nil)
	want(t, e.write(t, http.MethodPatch, "/api/artifacts/d", map[string]any{"archived": true}), http.StatusOK, `{"slug":"d"}`)
	want(t, e.do(t, http.MethodGet, "/api/artifacts", ""), http.StatusOK, `"archived":true`)
	// encoding/json/v2 says "cannot" or "unable to" as the process falls: what follows the verb is told.
	for body, told := range map[string]string{
		`{"archived":"yes"}`: `unmarshal JSON string into Go bool within "/archived"`,
		`{"archived":1}`:     `unmarshal JSON number into Go bool within "/archived"`,
		`{"favorite":"yes"}`: `unmarshal JSON string into Go bool within "/favorite"`,
	} {
		w := e.write(t, http.MethodPatch, "/api/artifacts/d", json.RawMessage(body))
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), told) {
			t.Errorf("%s: answered %d %q, want %q", body, w.Code, w.Body.String(), told)
		}
	}
	want(t, e.write(t, http.MethodPatch, "/api/artifacts/d", map[string]any{"archived": false}), http.StatusOK, `{"slug":"d"}`)
	if w := e.do(t, http.MethodGet, "/api/artifacts", ""); strings.Contains(w.Body.String(), "archived") {
		t.Errorf("given back, still listed archived: %s", w.Body.String())
	}
	want(t, e.write(t, http.MethodPatch, "/api/artifacts/none", map[string]any{"archived": true}), http.StatusNotFound, "not found")
}

// An edit changes each field by its own key and no other; a field null is not changed. A key it does not know is
// refused with nothing changed, even beside one it knows and before the page is looked for.
func TestEditKeys(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.publish(t, "d", "lang: en\ntemplate: pic\ntitle: T\n", nil)
	type fields struct {
		title, description, project string
		favorite, archived          bool
		// The tags joined by NUL, which no tag sent here holds: a comma would tell ["a,b"] from ["a","b"] no more.
		tags string
	}
	look := func(slug string) fields {
		t.Helper()

		m := store.New(e.root).ReadMeta(slug)
		if m == nil {
			t.Fatalf("no page %q", slug)
		}
		return fields{m.Title, m.Description, m.Project, m.Favorite, m.Archived, strings.Join(m.Tags, "\x00")}
	}
	now := look("d")
	for _, c := range []struct {
		body   string
		change func(*fields)
	}{
		{`{"title":"N"}`, func(f *fields) { f.title = "N" }},
		{`{"description":"D"}`, func(f *fields) { f.description = "D" }},
		{`{"project":"q"}`, func(f *fields) { f.project = "q" }},
		{`{"favorite":true}`, func(f *fields) { f.favorite = true }},
		{`{"archived":true}`, func(f *fields) { f.archived = true }},
		{`{"archived":null}`, func(*fields) {}},
		{`{"tags":["b"," a ","b",""]}`, func(f *fields) { f.tags = "b\x00a" }},
		{`{"tags":["a,b"]}`, func(f *fields) { f.tags = "a,b" }},
		{`{"tags":null}`, func(*fields) {}},
		{`{"tags":[]}`, func(f *fields) { f.tags = "" }},
		// Cleared, the title is the data file's again and the project is dropped.
		{`{"title":""}`, func(f *fields) { f.title = "T" }},
		{`{"project":""}`, func(f *fields) { f.project = "" }},
	} {
		want(t, e.write(t, http.MethodPatch, "/api/artifacts/d", json.RawMessage(c.body)), http.StatusOK, `{"slug":"d"}`)
		c.change(&now)
		if got := look("d"); got != now {
			t.Errorf("%s: %+v, want %+v", c.body, got, now)
		}
	}
	for _, body := range []string{`{"archivd":true}`, `{"title":"T2","color":"red"}`} {
		want(t, e.write(t, http.MethodPatch, "/api/artifacts/d", json.RawMessage(body)), http.StatusBadRequest,
			"unknown object member name")
		if got := look("d"); got != now {
			t.Errorf("%s changed the page: %+v, want %+v", body, got, now)
		}
	}
	for body, told := range map[string]string{
		`{"tags":"a"}`: `unmarshal JSON string into Go []string within "/tags"`,
		`{"tags":[1]}`: `unmarshal JSON number into Go string within "/tags/0"`,
	} {
		want(t, e.write(t, http.MethodPatch, "/api/artifacts/d", json.RawMessage(body)), http.StatusBadRequest, told)
	}
	want(t, e.write(t, http.MethodPatch, "/api/artifacts/none", json.RawMessage(`{"x":1}`)), http.StatusBadRequest,
		`unknown object member name "x"`)
	want(t, e.write(t, http.MethodPatch, "/api/artifacts/d", json.RawMessage(`{"slug":"e"}`)), http.StatusOK, `{"slug":"e"}`)
	if got := look("e"); got != now {
		t.Errorf("renamed: %+v, want %+v", got, now)
	}
	// floe archive sends archived alone.
	yes := true
	if b, err := json.Marshal(server.Edit{Archived: &yes}); err != nil || string(b) != `{"archived":true}` {
		t.Errorf("floe archive sends %s, %v", b, err)
	}
	// Tags none is sent as such, which clears them.
	if b, err := json.Marshal(server.Edit{Tags: &[]string{}}); err != nil || string(b) != `{"tags":[]}` {
		t.Errorf("no tags sent as %s, %v", b, err)
	}
}

// Every write refuses a body that is not one object, null too, in one sentence.
func TestNotObject(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.publish(t, "d", "lang: en\ntemplate: pic\n", nil)
	for _, to := range []struct{ method, path string }{
		{http.MethodPost, "/api/artifacts"},
		{http.MethodPut, "/api/artifacts/d"},
		{http.MethodPatch, "/api/artifacts/d"},
		{http.MethodPost, "/api/rebuild"},
	} {
		for _, body := range []string{"null", "1", `"s"`, "[1]", "x", "", "   "} {
			w := e.do(t, to.method, to.path, body, "Origin", origin, "Content-Type", "application/json")
			if w.Code != http.StatusBadRequest || w.Body.String() != "cannot read the request: send a JSON object" {
				t.Errorf("%s %s %s: answered %d %q", to.method, to.path, body, w.Code, w.Body.String())
			}
		}
	}
	for body, told := range map[string]string{
		"{}{}": "jsontext: invalid character '{' after top-level value", "{": "jsontext: unexpected EOF",
	} {
		want(t, e.do(t, http.MethodPatch, "/api/artifacts/d", body, "Origin", origin, "Content-Type", "application/json"),
			http.StatusBadRequest, "cannot read the request: "+told)
	}
}

// Every write reads keys by their case and refuses a key sent twice, in a map in it too; an edit refuses a string
// that is no UTF-8 or holds a lone surrogate, and a publish base64 with a newline. Nothing refused is changed.
func TestStrictKeys(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.publish(t, "d", "lang: en\ntemplate: pic\ntitle: T\n", nil)
	src := `"bGFuZzogZW4KdGVtcGxhdGU6IHBpYwo="` // lang: en, template: pic
	for _, c := range []struct{ method, path, body, told string }{
		{http.MethodPost, "/api/artifacts", `{"Path":"/x"}`, `unknown object member name "Path"`},
		{http.MethodPut, "/api/artifacts/e", `{"File":"e.yaml","source":` + src + `}`, `unknown object member name "File"`},
		{http.MethodPatch, "/api/artifacts/d", `{"Archived":true}`, `unknown object member name "Archived"`},
		{http.MethodPost, "/api/artifacts", `{"path":"/a","path":"/b"}`, `duplicate object member name "path"`},
		{
			http.MethodPut, "/api/artifacts/e", `{"file":"e.yaml","file":"f.yaml","source":` + src + `}`,
			`duplicate object member name "file"`,
		},
		{
			http.MethodPut, "/api/artifacts/e", `{"file":"e.yaml","source":` + src + `,"images":{"x.png":"YQ==","x.png":"Yg=="}}`,
			`duplicate object member name "x.png" within "/images"`,
		},
		{http.MethodPatch, "/api/artifacts/d", `{"title":"a","title":"b"}`, `duplicate object member name "title"`},
		// rebuild has no field: the first x is unknown before the second is seen.
		{http.MethodPost, "/api/rebuild", `{"x":1,"x":1}`, `unknown object member name "x"`},
		{http.MethodPost, "/api/artifacts", `{"path":"/a\ud800"}`, "invalid surrogate pair"},
		{http.MethodPut, "/api/artifacts/e", `{"file":"e\ud800.yaml","source":` + src + `}`, "invalid surrogate pair"},
		{http.MethodPatch, "/api/artifacts/d", `{"title":"\ud800"}`, "invalid surrogate pair"},
		{http.MethodPost, "/api/rebuild", `{"\ud800":1}`, "invalid surrogate pair"},
		{http.MethodPost, "/api/artifacts", "{\"path\":\"/a\xff\"}", "invalid UTF-8"},
		{http.MethodPut, "/api/artifacts/e", "{\"file\":\"e\xff.yaml\",\"source\":" + src + "}", "invalid UTF-8"},
		{
			http.MethodPut, "/api/artifacts/e", "{\"file\":\"e.yaml\",\"source\":" + src + ",\"images\":{\"a\xff.png\":\"YQ==\"}}",
			"invalid UTF-8",
		},
		{http.MethodPatch, "/api/artifacts/d", "{\"title\":\"a\xffb\"}", "invalid UTF-8"},
		{http.MethodPost, "/api/rebuild", "{\"a\xff\":1}", "invalid UTF-8"},
		{http.MethodPut, "/api/artifacts/e", `{"file":"e.yaml","source":"bGFuZzogZW4KdGVtcGxhdGU6IHBpYwo=\n"}`, `within "/source"`},
	} {
		w := e.do(t, c.method, c.path, c.body, "Origin", origin, "Content-Type", "application/json")
		if w.Code != http.StatusBadRequest || !strings.HasPrefix(w.Body.String(), "cannot read the request: ") ||
			!strings.Contains(w.Body.String(), c.told) {
			t.Errorf("%s %s %s: answered %d %q, want %q", c.method, c.path, c.body, w.Code, w.Body.String(), c.told)
		}
	}
	m := store.New(e.root)
	if d := m.ReadMeta("d"); d.Title != "T" || d.Archived {
		t.Errorf("refused edits changed the page: %+v", d)
	}
	if m.ReadMeta("e") != nil {
		t.Error("a refused publish stored e")
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
			e.do(t, http.MethodGet, "/a/d/?raw", "")
			e.publish(t, "s", "lang: en\ntemplate: pic\n", nil)
			e.do(t, http.MethodGet, "/api/artifacts", "")
		})
	}
	wg.Wait()
}
