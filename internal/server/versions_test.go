package server_test

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/server"
	"github.com/sukekyo26/floe/internal/store"
)

func noted(note string) string {
	return "lang: en\ntemplate: pic\ntitle: T\nnote: " + note + "\n"
}

// The versions of a page are its meta.json's, oldest first; a link and a page of none have none; a slug of no page
// is not found. The list carries no versions.
func TestVersionsAPI(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.publish(t, "d", noted("one"), nil)
	e.publish(t, "d", noted("two"), nil)
	b, err := json.Marshal(store.New(e.root).ReadMeta("d").Versions)
	if err != nil {
		t.Fatal(err)
	}
	if w := e.do(t, http.MethodGet, "/api/artifacts/d/versions", ""); w.Code != http.StatusOK || w.Body.String() != string(b) ||
		!strings.Contains(w.Body.String(), `{"n":2,"at":"`) {
		t.Errorf("versions %d %s, want %s", w.Code, w.Body, b)
	}
	writeFiles(t, filepath.Join(e.home, "proj"), map[string]string{"h.html": "<title>h</title>"})
	want(t, e.write(t, http.MethodPost, "/api/artifacts", server.Link{Path: filepath.Join(e.home, "proj", "h.html")}), http.StatusCreated, `"slug":"h"`) //nolint:exhaustruct // the rest not given
	if w := e.do(t, http.MethodGet, "/api/artifacts/h/versions", ""); w.Code != http.StatusOK || w.Body.String() != "[]" {
		t.Errorf("a link's versions %d %s", w.Code, w.Body)
	}
	for _, slug := range []string{"none", "Bad_Slug", "..%2Fd"} {
		want(t, e.do(t, http.MethodGet, "/api/artifacts/"+slug+"/versions", ""), http.StatusNotFound, "not found")
	}
	if w := e.do(t, http.MethodGet, "/api/artifacts", ""); strings.Contains(w.Body.String(), `"versions"`) {
		t.Errorf("the list carries versions: %s", w.Body)
	}
}

// ?raw&v=<n> of a stored page is its version n, served as the page is; any other number is not found. Elsewhere, ?v=
// is a file's own (against a cache), as it was.
func TestServeVersion(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.publish(t, "d", noted("one"), nil)
	e.publish(t, "d", noted("two"), nil)
	s := store.New(e.root)
	page := e.do(t, http.MethodGet, "/a/d/?raw", "")
	for n, note := range map[int]string{1: "one", 2: "two"} {
		w := e.do(t, http.MethodGet, "/a/d/?raw&v="+strconv.Itoa(n), "")
		kept, _ := s.VersionPage("d", s.ReadMeta("d"), n)
		if w.Code != http.StatusOK || !strings.HasPrefix(w.Body.String(), string(kept)) || !strings.Contains(string(kept), note) ||
			!strings.Contains(w.Body.String(), `\u0026v=`+strconv.Itoa(n)+`"`) {
			t.Errorf("v=%d: %d\n%s", n, w.Code, w.Body)
		}
		for _, h := range []string{"Content-Security-Policy", "X-Content-Type-Options", "Cache-Control", "Content-Type"} {
			if w.Header().Get(h) != page.Header().Get(h) {
				t.Errorf("v=%d: %s %q, the page's %q", n, h, w.Header().Get(h), page.Header().Get(h))
			}
		}
	}
	for _, v := range []string{"01", "%2B1", "0", "-1", "1.0", "x", "", "99"} {
		want(t, e.do(t, http.MethodGet, "/a/d/?raw&v="+v, ""), http.StatusNotFound, "not found")
	}
	writeFiles(t, filepath.Join(e.home, "proj"), map[string]string{"h.html": "<title>h</title>", "s.css": "p{}", "docs/a.html": "<p>a</p>"})
	want(t, e.write(t, http.MethodPost, "/api/artifacts", server.Link{Path: filepath.Join(e.home, "proj", "h.html")}), http.StatusCreated, `"slug":"h"`)  //nolint:exhaustruct // the rest not given
	want(t, e.write(t, http.MethodPost, "/api/artifacts", server.Link{Path: filepath.Join(e.home, "proj", "docs")}), http.StatusCreated, `"slug":"docs"`) //nolint:exhaustruct // the rest not given
	for target, body := range map[string]string{
		"/a/h/?raw&v=1": "<title>h</title>", "/a/h/s.css?v=3": "p{}", "/a/h/s.css?v=1.2.0": "p{}", "/a/docs/a.html?v=2": "<p>a</p>",
	} {
		want(t, e.do(t, http.MethodGet, target, ""), http.StatusOK, body)
	}
}

// A version's page asks of that version, which stays "same" until it is no longer what it was served as: a version
// before stays, the last changes with its copy; one of another page under the slug, after a removal or a rename, is
// another. A slug of no page, or a file under it, is "changed".
func TestVersionTabMark(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	since := func(n int) string {
		v := strconv.Itoa(n)
		m := regexp.MustCompile(`/api/changed/d\?since=([^"\\]*)\\u0026v=` + v + `"`).FindStringSubmatch(e.do(t, http.MethodGet, "/a/d/?raw&v="+v, "").Body.String())
		if m == nil {
			t.Fatalf("no since in version %s", v)
		}
		return m[1]
	}
	ask := func(path string) string {
		return e.do(t, http.MethodGet, path, "").Body.String()
	}
	e.publish(t, "d", noted("one"), nil)
	e.publish(t, "d", noted("two"), nil)
	one, two := since(1), since(2)
	got := []string{ask("/api/changed/d?since=" + one + "&v=1"), ask("/api/changed/d?since=" + two + "&v=2")}
	e.publish(t, "d", noted("three"), nil)
	three := since(3)
	got = append(got, ask("/api/changed/d?since="+one+"&v=1"), ask("/api/changed/d?since="+two+"&v=2"))
	writeFiles(t, e.tpl, map[string]string{"pic/page.html": "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head><body>v2</body></html>\n"})
	want(t, e.write(t, http.MethodPost, "/api/rebuild", struct{}{}), http.StatusNoContent, "")
	got = append(got, ask("/api/changed/d?since="+one+"&v=1"), ask("/api/changed/d?since="+three+"&v=3"),
		ask("/api/changed/d/x?since="+one+"&v=1"), ask("/api/changed/nope?since="+one+"&v=1"))
	want(t, e.do(t, http.MethodDelete, "/api/artifacts/d", "", "Origin", origin), http.StatusNoContent, "")
	e.publish(t, "d", noted("one"), nil)
	got = append(got, ask("/api/changed/d?since="+one+"&v=1"))
	one = since(1)
	want(t, e.do(t, http.MethodDelete, "/api/artifacts/d", "", "Origin", origin), http.StatusNoContent, "")
	e.publish(t, "o", noted("one"), nil)
	want(t, e.write(t, http.MethodPatch, "/api/artifacts/o", map[string]any{"slug": "d"}), http.StatusOK, `"slug":"d"`)
	got = append(got, ask("/api/changed/d?since="+one+"&v=1"))
	one = since(1)
	for i := range 20 {
		want(t, e.publish(t, "d", noted("n"+strconv.Itoa(i)), nil), http.StatusOK, `"slug":"d"`)
	}
	got = append(got, ask("/api/changed/d?since="+one+"&v=1"))
	want := "same same same changed same changed changed changed changed changed changed"
	if strings.Join(got, " ") != want {
		t.Errorf("answers %q, want %q", strings.Join(got, " "), want)
	}
}
