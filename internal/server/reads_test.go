package server_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"floe/internal/server"
	"floe/internal/store"
)

const jsonType = "application/json"

// newsOf is what the list says of slug.
func (e env) newsOf(t *testing.T, slug string) string {
	t.Helper()

	var list []store.Listed
	if err := json.Unmarshal(e.do(t, http.MethodGet, "/api/artifacts", "").Body.Bytes(), &list); err != nil {
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

// A viewer's reading a version is written by POST read alone; a mark of no version asks nothing; a link, a folder or a
// slug of no page is not found; a write from elsewhere, or of what cannot be read, is refused as any write is.
func TestReadAPI(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	s := store.New(e.root)
	if err := s.Begin(server.Viewer, "2000-01-01T00:00:00.000Z"); err != nil {
		t.Fatal(err)
	}
	e.publish(t, "d", noted("one"), nil)
	v := s.ReadMeta("d").Versions[0]
	if got := e.newsOf(t, "d"); got != "new" {
		t.Errorf("news %q", got)
	}
	read := func(slug string, body any) *struct {
		code int
		text string
	} {
		t.Helper()

		w := e.write(t, http.MethodPost, "/api/artifacts/"+slug+"/read", body)
		return &struct {
			code int
			text string
		}{w.Code, w.Body.String()}
	}
	if got := read("d", server.Read{N: 0, At: ""}); got.code != http.StatusOK || got.text != `{"cleared":false}` || e.newsOf(t, "d") != "new" {
		t.Errorf("of no version: %+v, news %q", got, e.newsOf(t, "d"))
	}
	if got := read("d", server.Read{N: v.N, At: v.At}); got.code != http.StatusOK || got.text != `{"cleared":true}` || e.newsOf(t, "d") != "" {
		t.Errorf("read: %+v, news %q", got, e.newsOf(t, "d"))
	}
	if got := read("d", server.Read{N: v.N, At: v.At}); got.code != http.StatusOK || got.text != `{"cleared":false}` {
		t.Errorf("read again: %+v", got)
	}
	// Marked unread by the management page alone: no body, no other origin, no page in the sandbox. Twice is once.
	unread := "/api/artifacts/d/read"
	want(t, e.do(t, http.MethodDelete, unread, ""), http.StatusForbidden, "forbidden origin")
	want(t, e.do(t, http.MethodDelete, unread, "", "Origin", "null"), http.StatusForbidden, "forbidden origin")
	want(t, e.do(t, http.MethodDelete, unread, "", "Origin", "https://example.com"), http.StatusForbidden, "forbidden origin")
	if got := e.newsOf(t, "d"); got != "" {
		t.Errorf("refused, news %q", got)
	}
	for range 2 {
		if w := e.do(t, http.MethodDelete, unread, "", "Origin", origin); w.Code != http.StatusNoContent || w.Body.Len() != 0 {
			t.Errorf("marked unread: %d %q", w.Code, w.Body.String())
		}
		if got := e.newsOf(t, "d"); got != "new" {
			t.Errorf("marked unread, news %q", got)
		}
	}
	if got := read("d", server.Read{N: v.N, At: v.At}); got.text != `{"cleared":true}` {
		t.Errorf("read after the mark: %+v", got)
	}
	if w := e.do(t, http.MethodDelete, "/api/artifacts/no-such/read", "", "Origin", origin); w.Code != http.StatusNotFound {
		t.Errorf("no page: %d", w.Code)
	}

	writeFiles(t, filepath.Join(e.home, "proj"), map[string]string{"h.html": "<title>h</title>", "docs/a.html": "<title>a</title>"})
	for path, slug := range map[string]string{"h.html": "h", "docs": "docs"} {
		link := server.Link{Path: filepath.Join(e.home, "proj", path), Slug: &slug} //nolint:exhaustruct // the rest not given
		want(t, e.write(t, http.MethodPost, "/api/artifacts", link), http.StatusCreated, `"slug":"`+slug+`"`)
	}
	for _, slug := range []string{"h", "docs", "none", "Bad_Slug"} {
		if got := read(slug, server.Read{N: 1, At: "x"}); got.code != http.StatusNotFound || got.text != "not found" {
			t.Errorf("%s: %+v", slug, got)
		}
	}
	const path = "/api/artifacts/d/read"
	const body = `{"n":1,"at":"x"}`
	strict := e.do(t, http.MethodPost, path, `{"n":1,"at":"x","mark":"y"}`, "Origin", origin, "Content-Type", jsonType)
	want(t, strict, http.StatusBadRequest, `cannot read the request: `)
	want(t, strict, http.StatusBadRequest, `"mark"`)
	want(t, e.do(t, http.MethodPost, path, body, "Content-Type", jsonType), http.StatusForbidden, "forbidden origin")
	want(t, e.do(t, http.MethodPost, path, body, "Origin", "null", "Content-Type", jsonType), http.StatusForbidden, "forbidden origin")
	want(t, e.do(t, http.MethodPost, path, body, "Origin", origin), http.StatusUnsupportedMediaType, "send application/json")

	// Without a since, a mark of a version is refused, the words saying what to do; the list counts every page as read.
	e.publish(t, "f", noted("one"), nil)
	if err := os.Remove(filepath.Join(e.root, "reads.json")); err != nil {
		t.Fatal(err)
	}
	f := s.ReadMeta("f").Versions[0]
	w := e.do(t, http.MethodPost, "/api/artifacts/f/read", `{"n":`+strconv.Itoa(f.N)+`,"at":"`+f.At+`"}`, "Origin", origin, "Content-Type", jsonType)
	if text := strings.ReplaceAll(w.Body.String(), e.root, "S"); w.Code != http.StatusBadRequest ||
		text != "S/reads.json: is missing, broken or has no since: run floe stop, then floe serve, which writes it anew" {
		t.Errorf("no since: %d %q", w.Code, text)
	}
	if w := e.do(t, http.MethodDelete, "/api/artifacts/f/read", "", "Origin", origin); w.Code != http.StatusBadRequest {
		t.Errorf("marked unread with no since: %d", w.Code)
	}
	if got := e.newsOf(t, "f"); got != "" {
		t.Errorf("no since: news %q", got)
	}
}

// A stored page tells its frame the version it served, of the meta.json it was served by, to this origin alone: ?raw its
// last, ?raw&v=<n> that one. A linked file, a file of a folder and a linked data file tell none.
func TestServedRead(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.publish(t, "d", noted("one"), nil)
	e.publish(t, "d", noted("two"), nil)
	versions := store.New(e.root).ReadMeta("d").Versions
	told := regexp.MustCompile(`if \(window\.parent !== window\) window\.parent\.postMessage\(\{ artifactRead: \{"n":(\d+),"at":"([^"]+)"\}, ` +
		`artifactPath: location\.pathname \}, location\.protocol \+ '//' \+ location\.host\);`)
	for path, v := range map[string]store.Version{"/a/d/?raw": versions[1], "/a/d/?raw&v=1": versions[0], "/a/d/?raw&v=2": versions[1]} {
		got := told.FindStringSubmatch(e.do(t, http.MethodGet, path, "").Body.String())
		if len(got) != 3 || got[1] != strconv.Itoa(v.N) || got[2] != v.At {
			t.Errorf("%s told %v, want %+v", path, got, v)
		}
	}
	writeFiles(t, filepath.Join(e.home, "proj"), map[string]string{
		"h.html": "<title>h</title>", "l.yaml": noted("linked"), "bad.yaml": noted("linked"), "docs/a.html": "<title>a</title>",
	})
	for _, name := range []string{"h.html", "l.yaml", "bad.yaml", "docs"} {
		link := server.Link{Path: filepath.Join(e.home, "proj", name)} //nolint:exhaustruct // the rest not given
		want(t, e.write(t, http.MethodPost, "/api/artifacts", link), http.StatusCreated, `"slug":"`)
	}
	writeFiles(t, filepath.Join(e.home, "proj"), map[string]string{"bad.yaml": "lang: en\ntemplate: nope\n"})
	for _, path := range []string{"/a/h/?raw", "/a/l/?raw", "/a/bad/?raw", "/a/docs/a.html"} {
		w := e.do(t, http.MethodGet, path, "")
		if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "artifactRead") {
			t.Errorf("%s: %d, telling a version: %q", path, w.Code, w.Body)
		}
	}
}

// TestServedKeys gives every HTML served the script that hands keys to the frame: of a page floe drew, the letters too; of any
// other, Ctrl+K alone. What is not HTML has none.
func TestServedKeys(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	e.publish(t, "d", noted("one"), nil)
	writeFiles(t, filepath.Join(e.home, "proj"), map[string]string{"h.html": "<title>h</title>", "l.yaml": noted("linked"), "docs/a.html": "<title>a</title>", "docs/b.txt": "b"})
	for _, name := range []string{"h.html", "l.yaml", "docs"} {
		link := server.Link{Path: filepath.Join(e.home, "proj", name)} //nolint:exhaustruct // the rest not given
		want(t, e.write(t, http.MethodPost, "/api/artifacts", link), http.StatusCreated, `"slug":"`)
	}
	const letters = "['j', 'k', 'h', 'd', '?', '/']"
	for path, drawn := range map[string]bool{"/a/d/?raw": true, "/a/l/?raw": true, "/a/h/?raw": false, "/a/docs/a.html": false} {
		body := e.do(t, http.MethodGet, path, "").Body.String()
		if !strings.Contains(body, "artifactKey: key") || !strings.Contains(body, "artifactKeysWanted") {
			t.Errorf("%s has no script for the keys: %q", path, body)
		}
		if strings.Contains(body, letters) != drawn {
			t.Errorf("%s: the letters handed over, want %v", path, drawn)
		}
		if strings.Count(body, "</script>") != strings.Count(body, "<script") {
			t.Errorf("%s: a script cut: %q", path, body)
		}
	}
	if body := e.do(t, http.MethodGet, "/a/docs/b.txt", "").Body.String(); strings.Contains(body, "artifactKey") {
		t.Errorf("a text file has the script: %q", body)
	}
}
