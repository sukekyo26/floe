package server_test

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"floe/internal/server"
)

const tooDeep = "html: open stack of elements exceeds 512 nodes"

// deepTemplate writes the template deep of the body given, its note shown, among those of e.
func deepTemplate(t *testing.T, e env, body string) {
	t.Helper()
	writeFiles(t, e.tpl, map[string]string{
		"deep/page.html": "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head><body>" +
			"{{.Data.note}}" + body + "</body></html>\n",
		"deep/schema.json":  pic,
		"deep/example.yaml": "lang: en\ntemplate: deep\n",
	})
}

// A page past the parser's depth is refused in words: published, its last version stays; linked, no link is made; once linked, it opens as a page of mistakes and is not downloaded.
func TestTooDeep(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	deepTemplate(t, e, "")
	src := "lang: en\ntemplate: deep\nnote: one\n"
	want(t, e.publish(t, "d", src, nil), http.StatusCreated, "")
	data := filepath.Join(e.home, "proj", "l.yaml")
	writeFiles(t, filepath.Dir(data), map[string]string{"l.yaml": src})
	want(t, e.write(t, http.MethodPost, "/api/pages", server.Link{Path: data}), http.StatusCreated, `"slug":"l"`) //nolint:exhaustruct // the rest not given

	deepTemplate(t, e, strings.Repeat("<div>", 600))
	want(t, e.publish(t, "d", "lang: en\ntemplate: deep\nnote: two\n", nil), http.StatusBadRequest, `{"file":"d.yaml","text":"`+tooDeep+`"}`)
	want(t, e.do(t, http.MethodGet, "/a/d/?raw", ""), http.StatusOK, "one")

	want(t, e.do(t, http.MethodGet, "/a/l/?raw", ""), http.StatusOK, tooDeep)
	want(t, e.do(t, http.MethodGet, "/api/pages/l/download", ""), http.StatusBadRequest, data+": "+tooDeep)

	other := filepath.Join(e.home, "proj", "n.yaml")
	writeFiles(t, filepath.Dir(other), map[string]string{"n.yaml": src})
	want(t, e.write(t, http.MethodPost, "/api/pages", server.Link{Path: other}), http.StatusBadRequest, `{"file":"`+other+`","text":"`+tooDeep+`"}`) //nolint:exhaustruct // the rest not given
	want(t, e.do(t, http.MethodGet, "/a/n/?raw", ""), http.StatusNotFound, "")
}

// A stored page that the parser takes, written and read again, no more: what reads it again says so in words, of no
// file. A linked data file is drawn for its download, and not read again.
func TestKeptNotReadAgain(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	deepTemplate(t, e, strings.Repeat("<div>", 503)+"<form><i><nobr><h2></form><div><li><button><h1>")
	want(t, e.publish(t, "d", "lang: en\ntemplate: deep\nnote: one\n", nil), http.StatusCreated, "")
	want(t, e.publish(t, "d", "lang: en\ntemplate: deep\nnote: two\n", nil), http.StatusOK, "")
	want(t, e.do(t, http.MethodGet, "/a/d/?raw", ""), http.StatusOK, "two")
	for _, path := range []string{"/api/pages/d/download", "/api/pages/d/diff?from=1&to=2"} {
		w := e.do(t, http.MethodGet, path, "")
		if w.Code != http.StatusBadRequest || w.Body.String() != tooDeep {
			t.Errorf("%s = %d, %q", path, w.Code, w.Body)
		}
	}

	data := filepath.Join(e.home, "proj", "l.yaml")
	writeFiles(t, filepath.Dir(data), map[string]string{"l.yaml": "lang: en\ntemplate: deep\nnote: one\n"})
	want(t, e.write(t, http.MethodPost, "/api/pages", server.Link{Path: data}), http.StatusCreated, `"slug":"l"`) //nolint:exhaustruct // the rest not given
	want(t, e.do(t, http.MethodGet, "/api/pages/l/download", ""), http.StatusOK, `<meta http-equiv="Content-Security-Policy"`)
}

// What the library will not write is the page's mistake, not floe's failure.
func TestNotWritten(t *testing.T) {
	t.Parallel()

	e := newEnv(t)
	deepTemplate(t, e, "<svg><col>x</col></svg>")
	want(t, e.publish(t, "d", "lang: en\ntemplate: deep\nnote: one\n", nil), http.StatusBadRequest,
		`{"file":"d.yaml","text":"html: void element \u003ccol\u003e has child nodes"}`)
}
