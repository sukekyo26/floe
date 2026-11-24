package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Every request but GET and HEAD, of a route or of none, is refused from another origin before anything is looked for,
// and no route refuses the server's own; GET and HEAD are taken from any.
func TestWritesOfOthers(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	want(t, e.publish(t, "d", "lang: en\ntemplate: pic\n", nil), http.StatusCreated, `"slug":"d"`)
	// The ones that would change something from the server's own origin are sent as they change nothing: text for JSON,
	// another pid, a page or a task that is not there.
	for _, c := range []struct{ method, path, body, typ string }{
		{http.MethodPost, "/api/pages", "{}", "text/plain"},
		{http.MethodPost, "/api/pages/data", "{}", "text/plain"},
		{http.MethodPatch, "/api/pages/d", "{}", "text/plain"},
		{http.MethodDelete, "/api/pages/none", "", ""},
		{http.MethodPost, "/api/pages/d/read", "{}", "text/plain"},
		{http.MethodDelete, "/api/pages/none/read", "", ""},
		{http.MethodPatch, "/api/tasks", "{}", "text/plain"},
		{http.MethodDelete, "/api/tasks?taskProject=p&task=none", "", ""},
		{http.MethodPost, "/api/rebuild", "{}", "text/plain"},
		{http.MethodPut, "/api/pages/d/theme", "{}", "text/plain"},
		{http.MethodPut, "/api/settings", "{}", "text/plain"},
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
	} {
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
