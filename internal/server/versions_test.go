package server_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/pagediff"
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
	if w, wantBody := e.do(t, http.MethodGet, "/api/artifacts/d/versions", ""), `{"versions":`+string(b)+`,"read":0}`; w.Code != http.StatusOK ||
		w.Body.String() != wantBody || !strings.Contains(w.Body.String(), `{"n":2,"at":"`) {
		t.Errorf("versions %d %s, want %s", w.Code, w.Body, wantBody)
	}
	writeFiles(t, filepath.Join(e.home, "proj"), map[string]string{"h.html": "<title>h</title>"})
	want(t, e.write(t, http.MethodPost, "/api/artifacts", server.Link{Path: filepath.Join(e.home, "proj", "h.html")}), http.StatusCreated, `"slug":"h"`) //nolint:exhaustruct // the rest not given
	if w := e.do(t, http.MethodGet, "/api/artifacts/h/versions", ""); w.Code != http.StatusOK || w.Body.String() != `{"versions":[],"read":0}` {
		t.Errorf("a link's versions %d %s", w.Code, w.Body)
	}
	for _, slug := range []string{"none", "Bad_Slug", "..%2Fd"} {
		want(t, e.do(t, http.MethodGet, "/api/artifacts/"+slug+"/versions", ""), http.StatusNotFound, "not found")
	}
	if w := e.do(t, http.MethodGet, "/api/artifacts", ""); strings.Contains(w.Body.String(), `"versions"`) ||
		!strings.Contains(w.Body.String(), `"versionCount":2`) {
		t.Errorf("the list carries versions, or not how many: %s", w.Body)
	}
}

// The difference of two versions is of the sections by their words, of a stored page alone, and reads nothing as read. from and to
// are numbers of versions, in either order; the same one differs in nothing.
func TestDiffAPI(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.publish(t, "d", noted("one two three"), nil)
	e.publish(t, "d", noted("one two four"), nil)
	reads := filepath.Join(e.root, "reads.json")
	before, err := os.ReadFile(reads)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	type answer struct {
		From, To struct {
			N int
		}
		Counts   pagediff.Counts
		Sections []pagediff.Section
	}
	get := func(query string) answer {
		t.Helper()

		w := e.do(t, http.MethodGet, "/api/artifacts/d/diff?"+query, "")
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", query, w.Code, w.Body)
		}
		var a answer
		if err = json.Unmarshal(w.Body.Bytes(), &a); err != nil {
			t.Fatal(err)
		}
		return a
	}
	ops := func(a answer) string {
		var b strings.Builder
		for _, s := range a.Sections {
			for _, block := range s.Blocks {
				for _, span := range block.Spans {
					if span.Op != "same" {
						b.WriteString(span.Op + ":" + span.Text + " ")
					}
				}
			}
		}
		return b.String()
	}
	a := get("from=1&to=2")
	if a.From.N != 1 || a.To.N != 2 || a.Counts.Changed != 1 || ops(a) != "del:three ins:four " {
		t.Errorf("from 1 to 2: %+v, %s", a, ops(a))
	}
	if a := get("from=2&to=1"); ops(a) != "del:four ins:three " {
		t.Errorf("from 2 to 1: %s", ops(a))
	}
	if a := get("from=2&to=2"); a.Counts.Changed != 0 || a.Counts.Same == 0 {
		t.Errorf("from 2 to 2: %+v", a)
	}
	after, err := os.ReadFile(reads)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("reads.json changed: %s", after)
	}

	for _, query := range []string{"", "from=1", "to=2", "from=abc&to=2", "from=0&to=2", "from=01&to=2", "from=-1&to=2", "from=1&to=2x"} {
		want(t, e.do(t, http.MethodGet, "/api/artifacts/d/diff?"+query, ""), http.StatusBadRequest, "from and to must be numbers of versions")
	}
	want(t, e.do(t, http.MethodGet, "/api/artifacts/d/diff?from=1&to=3", ""), http.StatusNotFound, "not found")
	want(t, e.do(t, http.MethodGet, "/api/artifacts/none/diff?from=1&to=2", ""), http.StatusNotFound, "not found")
	writeFiles(t, filepath.Join(e.home, "proj"), map[string]string{"h.html": "<title>h</title>"})
	want(t, e.write(t, http.MethodPost, "/api/artifacts", server.Link{Path: filepath.Join(e.home, "proj", "h.html")}), http.StatusCreated, `"slug":"h"`)
	want(t, e.do(t, http.MethodGet, "/api/artifacts/h/diff?from=1&to=2", ""), http.StatusNotFound, "not found")
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
	got := make([]string, 0, 11)
	got = append(got, ask("/api/changed/d?since="+one+"&v=1"), ask("/api/changed/d?since="+two+"&v=2"))
	e.publish(t, "d", noted("three"), nil)
	three := since(3)
	got = append(got, ask("/api/changed/d?since="+one+"&v=1"), ask("/api/changed/d?since="+two+"&v=2"))
	writeFiles(t, e.tpl, map[string]string{"pic/page.html": "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head><body>v2</body></html>\n"})
	want(t, e.write(t, http.MethodPost, "/api/rebuild", struct{}{}), http.StatusNoContent, "")
	got = append(got, ask("/api/changed/d?since="+one+"&v=1"), ask("/api/changed/d?since="+three+"&v=3"),
		ask("/api/changed/d/x?since="+one+"&v=1"), ask("/api/changed/nope?since="+one+"&v=1"))
	want(t, e.do(t, http.MethodDelete, "/api/artifacts/d", "", "Origin", origin), http.StatusNoContent, "")
	// Each v1 a version before the last, whose mark has no copy in it.
	e.publish(t, "d", noted("one"), nil)
	e.publish(t, "d", noted("two"), nil)
	got = append(got, ask("/api/changed/d?since="+one+"&v=1"))
	one = since(1)
	want(t, e.do(t, http.MethodDelete, "/api/artifacts/d", "", "Origin", origin), http.StatusNoContent, "")
	e.publish(t, "o", noted("one"), nil)
	e.publish(t, "o", noted("two"), nil)
	want(t, e.write(t, http.MethodPatch, "/api/artifacts/o", map[string]any{"slug": "d"}), http.StatusOK, `"slug":"d"`)
	got = append(got, ask("/api/changed/d?since="+one+"&v=1"))
	one = since(1)
	// d, renamed from o, is o.yaml's: published again from it.
	for i := range 20 {
		up := server.Upload{File: "o.yaml", Source: []byte(noted("n" + strconv.Itoa(i))), Project: "p", Place: "o.yaml"} //nolint:exhaustruct // the rest not given
		want(t, e.write(t, http.MethodPut, "/api/artifacts/d", up), http.StatusOK, `"slug":"d"`)
	}
	got = append(got, ask("/api/changed/d?since="+one+"&v=1"))
	want := "same same same changed same changed changed changed changed changed changed"
	if strings.Join(got, " ") != want {
		t.Errorf("answers %q, want %q", strings.Join(got, " "), want)
	}
}

// The frame says what its ?v= asks, as versionParam reads it: a version of a stored page, 0 for its last, gone for what is
// none; nothing where ?v= is not a version's, or there is no page.
func TestFrameVersion(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.publish(t, "d", noted("one"), nil)
	e.publish(t, "d", noted("two"), nil)
	e.publish(t, "only", noted("one"), nil)
	writeFiles(t, filepath.Join(e.home, "proj"), map[string]string{"h.html": "<title>h</title>", "docs/a.html": "<p>a</p>"})
	want(t, e.write(t, http.MethodPost, "/api/artifacts", server.Link{Path: filepath.Join(e.home, "proj", "h.html")}), http.StatusCreated, `"slug":"h"`)  //nolint:exhaustruct // the rest not given
	want(t, e.write(t, http.MethodPost, "/api/artifacts", server.Link{Path: filepath.Join(e.home, "proj", "docs")}), http.StatusCreated, `"slug":"docs"`) //nolint:exhaustruct // the rest not given
	said := regexp.MustCompile(`data-version="([^"]*)"`)
	for _, c := range []struct {
		target string
		status int
		want   string
	}{
		{"/a/d/", http.StatusOK, ""},
		{"/a/d/?v=1", http.StatusOK, "1"},
		{"/a/d/?v=2", http.StatusOK, "0"},
		{"/a/d/?v=99", http.StatusOK, "gone"},
		{"/a/d/?v=abc", http.StatusOK, "gone"},
		{"/a/d/?v=01", http.StatusOK, "gone"},
		{"/a/d/?v=", http.StatusOK, "gone"},
		{"/a/d/?v=0", http.StatusOK, "gone"},
		{"/a/d/?diff&v=1", http.StatusOK, "1"},
		{"/a/only/?v=1", http.StatusOK, "0"},
		{"/a/only/?v=2", http.StatusOK, "gone"},
		{"/a/h/?v=3", http.StatusOK, ""},
		{"/a/docs/?file=a.html&v=3", http.StatusOK, ""},
		{"/a/none/?v=3", http.StatusNotFound, ""},
	} {
		w := e.do(t, http.MethodGet, c.target, "")
		got := ""
		if m := said.FindStringSubmatch(w.Body.String()); m != nil {
			got = m[1]
		}
		if w.Code != c.status || got != c.want {
			t.Errorf("%s: %d data-version %q; want %d %q", c.target, w.Code, got, c.status, c.want)
		}
	}
}

// A download's ?v= asks a version as the page's does, saved under a name of its own; under a link it is not a version's.
func TestDownloadVersion(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.publish(t, "d", noted("one"), nil)
	e.publish(t, "d", noted("two"), nil)
	writeFiles(t, filepath.Join(e.home, "proj"), map[string]string{"h.html": "<title>h</title>"})
	want(t, e.write(t, http.MethodPost, "/api/artifacts", server.Link{Path: filepath.Join(e.home, "proj", "h.html")}), http.StatusCreated, `"slug":"h"`) //nolint:exhaustruct // the rest not given
	for target, c := range map[string]struct{ name, body string }{
		"/api/artifacts/d/download":     {"d.html", "two"},
		"/api/artifacts/d/download?v=1": {"d-v1.html", "one"},
		"/api/artifacts/d/download?v=2": {"d-v2.html", "two"},
		"/api/artifacts/h/download?v=3": {"h.html", "<title>h</title>"},
	} {
		w := e.do(t, http.MethodGet, target, "")
		want(t, w, http.StatusOK, c.body)
		if got := w.Header().Get("Content-Disposition"); got != `attachment; filename="`+c.name+`"` {
			t.Errorf("%s: %s; want %s", target, got, c.name)
		}
	}
	for _, v := range []string{"99", "abc", "01", "", "0"} {
		want(t, e.do(t, http.MethodGet, "/api/artifacts/d/download?v="+v, ""), http.StatusNotFound, "not found")
	}
}
