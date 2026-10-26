package draw

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"strings"
	"testing"

	"golang.org/x/net/html"

	"github.com/sukekyo26/floe/internal/kit"
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
	// floe's own failure in one of its functions, which text/template wraps as it wraps a template's mistake.
	floeFails := template.FuncMap{"fails": func() (string, error) { return "", fmt.Errorf("%w: x", kit.ErrFunc) }}
	page = template.Must(template.New("page.html").Funcs(floeFails).Parse("{{fails}}"))
	if e, _, failed = executed(page.Execute(io.Discard, nil), "page.html", nil); failed == nil || e != nil {
		t.Errorf("floe's failure in a function: %v, %v, want floe's failure", e, failed)
	}
	// A function of text/template, which fails by the template's mistake, is the template's.
	page = template.Must(template.New("page.html").Parse("{{index . 5}}"))
	if e, _, failed = executed(page.Execute(io.Discard, []int{1}), "page.html", nil); failed != nil || e == nil {
		t.Errorf("index out of range: %v, %v, want the template's mistake", e, failed)
	}
	if e, _, failed = executed(errors.New("disk"), "page.html", nil); failed == nil || e != nil {
		t.Errorf("an error of no execution: %v, %v, want floe's failure", e, failed)
	}
}

// The CSP goes first in the head, after the charset, before what the page's head holds; a page without a
// head gets it in the head the parser makes.
func TestCutOffPage(t *testing.T) {
	t.Parallel()

	for _, page := range []string{
		"<!doctype html><html><head><title>t</title></head><body></body></html>", "<!-- c --> <!DOCTYPE html>\n<p>x",
	} {
		got, err := CutOffPage([]byte(page))
		if err != nil {
			t.Fatal(err)
		}
		tree, _ := html.Parse(bytes.NewReader(got)) //nolint:errcheck // a bytes.Reader fails no read
		var head *html.Node
		for n := range tree.Descendants() {
			if n.Type == html.ElementNode && n.Data == "head" {
				head = n
				break
			}
		}
		charset, csp := head.FirstChild, head.FirstChild.NextSibling
		if v, _ := attrOf(charset, "charset"); v != "utf-8" {
			t.Errorf("%q: the head starts %q", page, got)
		}
		if v, _ := attrOf(csp, "content"); v != CutOff("'none'") {
			t.Errorf("%q: no CSP second in the head: %q", page, got)
		}
	}
}

// Where an item's text ends in a tag it does not finish is past the last token read whole, else the last token,
// which the tokenizer ended at the end.
func TestUnfinished(t *testing.T) {
	t.Parallel()

	for text, want := range map[string]string{
		"a</b": "</b", "a</p x=": "</p x=", "<b class='x>y</b>": "<b class='x>y</b>", "<!DOCTYPE x": "<!DOCTYPE x", "<b\nclass='x\ny": "<b class='x y",
		"x" + strings.Repeat("y", 50) + "</b " + strings.Repeat("z", 50): "</b " + strings.Repeat("z", 36) + "…",
	} {
		if got := unfinished(text); got != want {
			t.Errorf("unfinished(%q) = %q, want %q", text, got, want)
		}
	}
}
