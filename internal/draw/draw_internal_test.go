package draw

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"strings"
	"testing"
	ttemplate "text/template"

	"golang.org/x/net/html"

	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/page"
	"github.com/sukekyo26/floe/internal/schema"
	"github.com/sukekyo26/floe/internal/usererr"
)

// Where text/template says a template failed is the place textpos counts in the template's text.
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
		e, line, column, failed := executed(page.Execute(io.Discard, nil), "page.html", "page.html", []byte(tt.src))
		if failed != nil || len(e) != 1 || e[0].Key != "kit_num_digits" {
			t.Errorf("%q: executed = %v, %v", tt.src, e, failed)
			continue
		}
		if line != tt.line || column != tt.column {
			t.Errorf("%q: at %d:%d, want %d:%d", tt.src, line, column, tt.line, tt.column)
		}
	}

	// The library's words, without their head.
	page := template.Must(template.New("page.html").Option("missingkey=error").Parse("{{.a.b}}"))
	e, _, _, failed := executed(page.Execute(io.Discard, map[string]any{}), "page.html", "t/page.html", []byte("{{.a.b}}"))
	if failed != nil {
		t.Fatal(failed)
	}
	if want := `executing "page.html" at <.a.b>: map has no entry for key "a"`; len(e) != 1 || e[0].Args[0] != want ||
		e[0].File() != "t/page.html" {
		t.Errorf("told %v, want %q in t/page.html", e, want)
	}
	// Without a head saying where, of no file: told where a mistake of no file is.
	e, _, _, failed = executed(ttemplate.ExecError{Name: "page.html", Err: errors.New("x")}, "page.html", "t/page.html", nil)
	if failed != nil || len(e) != 1 || e[0].File() != "" {
		t.Errorf("no head: %v, %v, want a mistake of no file", e, failed)
	}

	// floe's own failure in one of its functions, which text/template wraps as it wraps a template's mistake.
	floeFails := template.FuncMap{"fails": func() (string, error) { return "", fmt.Errorf("%w: x", kit.ErrFunc) }}
	page = template.Must(template.New("page.html").Funcs(floeFails).Parse("{{fails}}"))
	if e, _, _, failed = executed(page.Execute(io.Discard, nil), "page.html", "page.html", nil); failed == nil || e != nil {
		t.Errorf("floe's failure in a function: %v, %v, want floe's failure", e, failed)
	}
	// A function of text/template, which fails by the template's mistake, is the template's.
	page = template.Must(template.New("page.html").Parse("{{index . 5}}"))
	if e, _, _, failed = executed(page.Execute(io.Discard, []int{1}), "page.html", "page.html", nil); failed != nil || e == nil {
		t.Errorf("index out of range: %v, %v, want the template's mistake", e, failed)
	}
	if e, _, _, failed = executed(errors.New("disk"), "page.html", "page.html", nil); failed == nil || e != nil {
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

// Each element page.Refused refuses has its words in refusal, which the English catalog holds.
func TestRefusalWords(t *testing.T) {
	t.Parallel()

	words := i18n.EnglishCatalog().Formats("draw_")
	for _, name := range page.Refusable {
		if key, ok := refusal[name]; !ok || words[key] == "" {
			t.Errorf("<%s>: refusal has %q, which the English catalog holds as %q", name, key, words[key])
		}
	}
}

// trim keeps the probe of an item whose mark a part's output starts or ends with, and no other: a comment like a
// mark without the secret is no mark.
func TestTrim(t *testing.T) {
	t.Parallel()

	s := newItems(nil, "", nil)
	h := string(s.marked([]string{"h"}, "<b>ok</b>"))
	for _, tt := range []struct{ name, out, want string }{
		{"the item alone", "\n" + h + "\n", h},
		{"text around", "\n a" + h + "b \n", "a" + h + "b"},
		{"a mark without the secret first", "\n<!--n0s-->x\n", "<!--n0s-->x"},
		{"a mark without the secret last", "\nx<!--n0e-->\n", "x<!--n0e-->"},
		{"a mark with text after it in its comment", "\n<!--" + s.mark(0, "s") + "x-->y\n", "<!--" + s.mark(0, "s") + "x-->y"},
	} {
		if got := s.trim(tt.out); got != tt.want {
			t.Errorf("%s: trim(%q) = %q, want %q", tt.name, tt.out, got, tt.want)
		}
	}
}

// text/template's mistake of running a part.html of floe's is told there too, as one of a part of one's own:
// drawPart parts them not. The value is one the part's schema would refuse, drawn unchecked.
func TestFloePartExecPlaced(t *testing.T) {
	t.Parallel()

	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	d, err := doc.Read("d.yaml", []byte("lang: en\nb: x\n"), "")
	if err != nil {
		t.Fatal(err)
	}
	it := schema.Item{At: []string{"b"}, Part: "code", Holds: nil, Format: "", Value: "x"}
	_, err = drawPart(k, d, it, "x", newItems(nil, "", nil))
	want := `parts/code/part.html:1:39: executing "part.html" at <.file>: can't evaluate field file in type string`
	if got := usererr.Tell(i18n.EnglishCatalog(), err).Render(""); got != want {
		t.Errorf("told %q, want %q", got, want)
	}
}
