package textpos_test

import (
	"errors"
	"io"
	"testing"
	"text/template"

	"github.com/sukekyo26/floe/internal/textpos"
)

// Of counts LF, CRLF and CR as one line break each, and runes for the column, a byte no UTF-8 as one.
func TestOf(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		src          string
		at           int
		line, column int
	}{
		{"abc", 0, 1, 1},
		{"abc", 3, 1, 4},
		{"a\nb", 2, 2, 1},
		{"a\r\nb", 3, 2, 1},
		{"a\r\nb", 2, 1, 3}, // the LF of a CRLF is still on its line
		{"a\rb", 2, 2, 1},
		{"é x", 3, 1, 3},
		{"a\fb", 2, 1, 3},
		{"日本x", 6, 1, 3},
		{"a\nbc", 3, 2, 2},
		{"a\r\n\rb\nc", 6, 4, 1},
		{"\tx", 1, 1, 2},
		{"\xff\xfex", 2, 1, 3},
		{"ab\r", 3, 2, 1},
	} {
		if line, column := textpos.Of([]byte(tt.src), tt.at); line != tt.line || column != tt.column {
			t.Errorf("Of(%q, %d) = %d:%d, want %d:%d", tt.src, tt.at, line, column, tt.line, tt.column)
		}
	}
}

// Ending places the character that ends at an offset, one inside a character at its start.
func TestEnding(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		src          string
		at           int
		line, column int
	}{
		{"{", 1, 1, 1},
		{"ab", 0, 1, 1},
		{"a\n日x", 3, 2, 1}, // inside 日
		{"a\n日x", 5, 2, 1},
		{"a\n日x", 6, 2, 2},
	} {
		if line, column := textpos.Ending([]byte(tt.src), tt.at); line != tt.line || column != tt.column {
			t.Errorf("Ending(%q, %d) = %d:%d, want %d:%d", tt.src, tt.at, line, column, tt.line, tt.column)
		}
	}
}

// OfTemplate reads text/template's head as its parse and its execution write it: a parse's line as Of counts
// it, without a column, the end of the text a place; an execution's line and column, a byte of the text.
func TestOfTemplate(t *testing.T) {
	t.Parallel()

	failing := template.FuncMap{"bad": func(any) (string, error) { return "", errors.New("x") }}
	const ran = `executing "page.html" at <bad .>: error calling bad: x`
	for _, tt := range []struct {
		src, run     string // run: text that parses, executed instead of src when not ""
		line, column int
		rest         string
	}{
		{"{{}}}", "", 1, 0, "missing value for command"},
		{"a\rb\n{{ bad }}", "", 3, 0, `function "bad" not defined`},
		{"a\r{{ bad }}\n", "", 1, 0, `function "bad" not defined`}, // LIMIT: an editor says 2
		{"{{if .x}}\n", "", 2, 0, "unexpected EOF"},
		{"", "{{bad .}}", 1, 3, ran},
		{"", "a\r{{bad .}}", 2, 3, ran},
		{"", "x\n  {{- bad .}}", 2, 7, ran},
	} {
		src, msg := tt.src, ""
		if tt.run == "" {
			_, err := template.New("page.html").Parse(src) // bad is no function of it
			msg = err.Error()
		} else {
			src = tt.run
			err := template.Must(template.New("page.html").Funcs(failing).Parse(src)).Execute(io.Discard, nil)
			msg = err.Error()
		}
		line, column, rest := textpos.OfTemplate([]byte(src), msg, "page.html")
		if line != tt.line || column != tt.column || rest != tt.rest {
			t.Errorf("OfTemplate(%q, %q) = %d:%d %q, want %d:%d %q", src, msg, line, column, rest, tt.line, tt.column, tt.rest)
		}
	}

	twoLines := []byte("{{if .x}}\n{{.}}\n")
	for _, tt := range []struct {
		src          []byte
		msg          string
		line, column int
		rest         string
	}{
		{twoLines, "template: page.html:5: unexpected EOF", 0, 0, "unexpected EOF"}, // past the text
		{nil, "template: page.html:1: unexpected EOF", 0, 0, "unexpected EOF"},      // no text to place it in
		{nil, "template: page.html:1:2: " + ran, 0, 0, ran},
		{twoLines, "template: page.html:3:0: " + ran, 0, 0, ran}, // the end of the text is no byte of it
		{twoLines, "template: part.html:1: x", 0, 0, "template: part.html:1: x"},
		{twoLines, "template: page.html:x:0: x", 0, 0, "template: page.html:x:0: x"},
		{twoLines, "template: page.html:0: x", 0, 0, "template: page.html:0: x"},
		{twoLines, "template: page.html:1:-1: x", 0, 0, "template: page.html:1:-1: x"},
		{twoLines, "template: page.html:1: 3: x", 1, 0, "3: x"}, // a parse's words start after a space
	} {
		line, column, rest := textpos.OfTemplate(tt.src, tt.msg, "page.html")
		if line != tt.line || column != tt.column || rest != tt.rest {
			t.Errorf("OfTemplate(%q, %q) = %d:%d %q, want %d:%d %q", tt.src, tt.msg, line, column, rest, tt.line, tt.column, tt.rest)
		}
	}
}
