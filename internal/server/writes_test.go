package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"floe/internal/server"
	"floe/internal/store"
)

// bodyWrite is a write that takes a body, of up to limit bytes, of the page d.
type bodyWrite struct {
	method, path string
	limit        int
}

// bodyWrites are the writes that take a body.
func bodyWrites() []bodyWrite {
	const small = 64 << 10
	return []bodyWrite{
		{http.MethodPost, "/api/pages", small},
		{http.MethodPost, "/api/pages/data", server.MaxUpload},
		{http.MethodPatch, "/api/pages/d", small},
		{http.MethodPost, "/api/pages/d/read", small},
		{http.MethodPut, "/api/pages/d/theme", small},
		{http.MethodPatch, "/api/tasks", small},
		{http.MethodPost, "/api/rebuild", small},
		{http.MethodPut, "/api/settings", small},
		{http.MethodPost, "/api/stop", small},
	}
}

// sentWrite is a write sent with a body of the type typ.
type sentWrite struct{ method, path, body, typ string }

// textWrites are bodyWrites, each with a body of text, which changes nothing.
func textWrites() []sentWrite {
	routes := bodyWrites()
	writes := make([]sentWrite, len(routes))
	for i, to := range routes {
		writes[i] = sentWrite{to.method, to.path, "{}", "text/plain"}
	}
	return writes
}

// Every request but GET and HEAD, of a route or of none, is refused from another origin before anything is looked for,
// and no route refuses the server's own; GET and HEAD are taken from any.
func TestWritesOfOthers(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	want(t, e.publish(t, "d", "lang: en\ntemplate: pic\n", nil), http.StatusCreated, `"slug":"d"`)
	// The ones that would change something from the server's own origin are sent as they change nothing: text for JSON,
	// another pid, a page or a task that is not there.
	for _, c := range append([]sentWrite{
		{http.MethodDelete, "/api/pages/none", "", ""},
		{http.MethodDelete, "/api/pages/none/read", "", ""},
		{http.MethodDelete, "/api/tasks?taskProject=p&task=none", "", ""},
		{http.MethodPost, "/api/stop", `{"pid":1}`, jsonType},
		{http.MethodPatch, "/api/pages/d", "{", jsonType},
		{http.MethodPatch, "/api/pages/d", strings.Repeat("x", 70<<10), jsonType},
		{http.MethodPatch, "/api/pages/Bad%20slug", "{}", jsonType},
		{http.MethodPut, "/api/pages/a%2Fb/theme", "{}", jsonType},
		{http.MethodPost, "/nope", "{}", jsonType},
		{http.MethodPost, "/a/d/", "", ""},
		{http.MethodPatch, "/nope", "{}", jsonType},
		{http.MethodPut, "/nope", "{}", jsonType},
		{http.MethodDelete, "/api/health", "", ""},
		{http.MethodOptions, "/api/pages", "", ""},
		{http.MethodTrace, "/api/pages", "", ""},
		{"FOO", "/api/pages", "", ""},
	}, textWrites()...) {
		for _, from := range []string{
			"", "null", "http://evil.example", "http://127.0.0.1:1", "https://127.0.0.1:7325",
			// Not those that begin as the server's own, such as a server on another port of this machine.
			origin + "0", origin + ".evil.example", "http://localhost:73250",
		} {
			want(t, e.do(t, c.method, c.path, c.body, "Origin", from, "Content-Type", c.typ), http.StatusForbidden, "forbidden origin")
		}
		want(t, e.do(t, c.method, c.path, c.body, "Content-Type", c.typ), http.StatusForbidden, "forbidden origin")
		for _, from := range []string{origin, "http://localhost:7325"} {
			if w := e.do(t, c.method, c.path, c.body, "Origin", from, "Content-Type", c.typ); w.Code == http.StatusForbidden {
				t.Errorf("%s %s from %s = 403, %q", c.method, c.path, from, w.Body)
			}
		}
	}
	for _, path := range []string{"/", "/api/pages", "/a/d/?raw", "/api/changed/d", "/api/pages/d/download"} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			for _, from := range []string{"", "null", "http://evil.example", origin} {
				if w := e.do(t, method, path, "", "Origin", from); w.Code != http.StatusOK {
					t.Errorf("%s %s from %q = %d, want 200", method, path, from, w.Code)
				}
			}
		}
	}
	// The host and the path are looked at first.
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/pages", nil)
	req.Host = "evil.example:7325"
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	want(t, w, http.StatusForbidden, "forbidden host")
	want(t, e.do(t, http.MethodPost, "/api//pages", ""), http.StatusNotFound, "not found")
}

// wantText is want of an answer in plain text, which nothing keeps.
func wantText(t *testing.T, w *httptest.ResponseRecorder, status int, body string) {
	t.Helper()

	want(t, w, status, body)
	if typ := w.Header().Get("Content-Type"); typ != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type %q", typ)
	}
	wantNoStore(t, w)
}

func wantNoStore(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()

	if kept := w.Header().Values("Cache-Control"); len(kept) != 1 || kept[0] != "no-store" {
		t.Errorf("Cache-Control %q", kept)
	}
}

// Every write that takes a body refuses one that is not JSON by its type first, then one over its limit, then one that
// is not a JSON object or holds a key it does not have. A slug that names no page is refused before the body is read.
func TestWriteBodies(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.publish(t, "d", "lang: en\ntemplate: pic\n", nil)
	for _, to := range bodyWrites() {
		t.Run(to.method+" "+to.path, func(t *testing.T) {
			t.Parallel()

			send := func(body, typ string) *httptest.ResponseRecorder {
				return e.do(t, to.method, to.path, body, "Origin", origin, "Content-Type", typ)
			}
			whole := strings.Repeat("x", to.limit)
			wantText(t, send("{}", "text/plain"), http.StatusUnsupportedMediaType, "send application/json")
			wantText(t, send(whole+"x", "text/plain"), http.StatusUnsupportedMediaType, "send application/json")
			wantText(t, send(whole+"x", jsonType), http.StatusRequestEntityTooLarge, "request body too large")
			wantText(t, send(whole, jsonType), http.StatusBadRequest, "cannot read the request: send a JSON object")
			wantText(t, send(`{"no_such_key":1}`, jsonType), http.StatusBadRequest, "cannot read the request: ")
		})
	}
	for path, method := range map[string]string{
		"/api/pages/Bad%20slug": http.MethodPatch, "/api/pages/Bad%20slug/read": http.MethodPost,
		"/api/pages/Bad%20slug/theme": http.MethodPut,
	} {
		wantText(t, e.do(t, method, path, "x", "Origin", origin, "Content-Type", "text/plain"), http.StatusNotFound, "not found")
	}
}

// What is not there is 404 in plain text, whoever asks: the CLI's Accept of JSON gets no report of it.
func TestNotThere(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/pages/none/files", ""},
		{http.MethodGet, "/api/pages/none/theme", ""},
		{http.MethodPatch, "/api/pages/none", "{}"},
		{http.MethodDelete, "/api/pages/none", ""},
		{http.MethodPost, "/api/pages/none/read", `{"n":1,"at":"x"}`},
		{http.MethodDelete, "/api/pages/none/read", ""},
		{http.MethodPut, "/api/pages/none/theme", `{"theme":""}`},
		{http.MethodPatch, "/api/tasks", `{"taskProject":"p","task":"#9","parent":""}`},
		{http.MethodDelete, "/api/tasks?taskProject=p&task=none", ""},
	} {
		for _, accept := range []string{"", jsonType} {
			w := e.do(t, c.method, c.path, c.body, "Origin", origin, "Content-Type", jsonType, "Accept", accept)
			wantText(t, w, http.StatusNotFound, "not found")
		}
	}
}

// A write with nothing to say is 204, with no body and no type; as any answer, it is not kept, nor is one with a body.
func TestNoContent(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	if err := store.New(e.root).Begin(server.Viewer, "2000-01-01T00:00:00.000Z"); err != nil {
		t.Fatal(err)
	}
	e.publish(t, "d", "lang: en\ntemplate: pic\n", nil)
	for _, c := range []struct {
		method, path, body string
		status             int
	}{
		{http.MethodPatch, "/api/tasks", `{"taskProject":"p","task":"#1","title":"a"}`, http.StatusOK},
		{http.MethodDelete, "/api/tasks?taskProject=p&task=%231", "", http.StatusNoContent},
		{http.MethodPost, "/api/pages/d/read", `{"n":9,"at":"x"}`, http.StatusOK},
		{http.MethodDelete, "/api/pages/d/read", "", http.StatusNoContent},
		{http.MethodPost, "/api/rebuild", "{}", http.StatusNoContent},
		{http.MethodDelete, "/api/pages/d", "", http.StatusNoContent},
		// Last: the server it stops is the one the rows before are sent to.
		{http.MethodPost, "/api/stop", `{"pid":` + strconv.Itoa(os.Getpid()) + `}`, http.StatusNoContent},
	} {
		w := e.do(t, c.method, c.path, c.body, "Origin", origin, "Content-Type", jsonType)
		want(t, w, c.status, "")
		wantNoStore(t, w)
		if typ, typed := w.Header()["Content-Type"]; c.status == http.StatusNoContent && (typed || w.Body.Len() > 0) {
			t.Errorf("%s %s: type %q, body %q", c.method, c.path, typ, w.Body)
		}
	}
}
