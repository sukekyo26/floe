package draw

import (
	"bytes"
	"errors"
	"html/template"
	"io"
	"testing"

	"github.com/sukekyo26/floe/internal/textpos"
	"github.com/sukekyo26/floe/internal/usererr"
)

// Where text/template says a template failed, a line counting \n alone and a column of bytes from 0, is made
// the place textpos counts in the template's text; one outside the text, or a head of another name, is none.
func TestExecuted(t *testing.T) {
	t.Parallel()

	bad := template.FuncMap{"bad": func(any) (string, error) { return "", usererr.New("kit_num_digits") }}
	for _, tt := range []struct {
		src          string
		line, column int
	}{
		{"{{bad .}}", 1, 3},
		{"あいう{{bad .}}", 1, 6},
		{"a\r\rb{{bad .}}", 3, 4},
		{"x\n  {{- bad .}}", 2, 7},
	} {
		page := template.Must(template.New("page.html").Funcs(bad).Parse(tt.src))
		e, at, failed := executed(page.Execute(io.Discard, nil), "page.html", []byte(tt.src))
		if failed != nil || e.Key != "kit_num_digits" || at < 0 {
			t.Errorf("%q: executed = %v, %d, %v", tt.src, e, at, failed)
			continue
		}
		if line, column := textpos.Of([]byte(tt.src), at); line != tt.line || column != tt.column {
			t.Errorf("%q: at %d:%d, want %d:%d", tt.src, line, column, tt.line, tt.column)
		}
	}

	// The library's words, without their head.
	page := template.Must(template.New("page.html").Option("missingkey=error").Parse("{{.a.b}}"))
	e, _, failed := executed(page.Execute(io.Discard, map[string]any{}), "page.html", []byte("{{.a.b}}"))
	if failed != nil {
		t.Fatal(failed)
	}
	if want := `executing "page.html" at <.a.b>: map has no entry for key "a"`; e.Args[0] != want {
		t.Errorf("told %q, want %q", e.Args[0], want)
	}

	for _, tt := range []struct{ msg, name string }{
		{"template: page.html:9:0: executing", "page.html"},
		{"template: part.html:1:0: executing", "page.html"},
		{"template: page.html:x:0: executing", "page.html"},
	} {
		if line, column, _, ok := cutHead(tt.msg, tt.name); ok && offset([]byte("{{bad .}}"), line, column) >= 0 {
			t.Errorf("%q: a place in the text", tt.msg)
		}
	}
	if e, _, failed = executed(errors.New("disk"), "page.html", nil); failed == nil || e != nil {
		t.Errorf("an error of no execution: %v, %v, want floe's failure", e, failed)
	}
}

// The CSP goes after the doctype, past a BOM and comments.
func TestCutOffPage(t *testing.T) {
	t.Parallel()

	meta := []byte(`<meta http-equiv="Content-Security-Policy"`)
	for _, page := range []string{"<!doctype html><html>", "\xef\xbb\xbf<!-- c --> <!DOCTYPE html>\n<p>", "<p>x"} {
		got := CutOffPage([]byte(page))
		at := bytes.Index(got, meta)
		if at < 0 || bytes.Contains(got[at:], []byte("<!doctype")) || bytes.Contains(got[at:], []byte("<!DOCTYPE")) {
			t.Errorf("%q: %q", page, got)
		}
	}
}
