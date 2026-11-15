package textpos_test

import (
	"errors"
	"io"
	"strings"
	"testing"
	"text/template"
	"time"
	"unicode/utf8"

	"floe/internal/textpos"
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
		{"é", 1, 1, 2},  // in a character: the byte before is one
		{"日x", 2, 1, 3}, // two bytes of a character cut short, each one
	} {
		if line, column := textpos.Of([]byte(tt.src), tt.at); line != tt.line || column != tt.column {
			t.Errorf("Of(%q, %d) = %d:%d, want %d:%d", tt.src, tt.at, line, column, tt.line, tt.column)
		}
	}
}

// At is Of's inverse, at every character of src and its end, whatever its line breaks; -1 past src's lines or a
// line's end. NextLine is where the line after an offset's starts, as Of counts lines.
func TestAt(t *testing.T) {
	t.Parallel()

	for _, src := range []string{"abc", "a\nb", "a\r\nb", "a\rb", "日本x\ny", "a\r\n\rb\nc", "ab\r", "\xff\xfex\n", "a\n", "", "a\r\n"} {
		for at := range len(src) + 1 {
			if at < len(src) && !utf8.RuneStart(src[at]) {
				continue
			}
			line, column := textpos.Of([]byte(src), at)
			if got := textpos.At([]byte(src), line, column); got != at {
				t.Errorf("At(%q, %d, %d) = %d, want %d", src, line, column, got, at)
			}
		}
	}
	for _, tt := range []struct {
		src          string
		line, column int
	}{{"ab\ncd", 3, 1}, {"ab\ncd", 1, 4}, {"ab\ncd", 0, 1}, {"ab\ncd", 1, 0}, {"日本", 1, 4}, {"a\n", 3, 1}, {"a\n", 5, 1}, {"a\r\n", 3, 1}, {"", 2, 1}} {
		if got := textpos.At([]byte(tt.src), tt.line, tt.column); got != -1 {
			t.Errorf("At(%q, %d, %d) = %d, want -1", tt.src, tt.line, tt.column, got)
		}
	}
	for _, tt := range []struct {
		src      string
		at, next int
	}{{"a\nb", 0, 2}, {"a\r\nb", 0, 3}, {"a\r\nb", 1, 3}, {"a\rb", 0, 2}, {"ab", 0, 2}, {"a\nb", 2, 3}, {"日\nb", 0, 4}} {
		if got := textpos.NextLine([]byte(tt.src), tt.at); got != tt.next {
			t.Errorf("NextLine(%q, %d) = %d, want %d", tt.src, tt.at, got, tt.next)
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

// Each tells, in one reading, each place as Of tells it alone: over LF, CRLF and CR, and a wide character.
func TestEach(t *testing.T) {
	t.Parallel()

	src := []byte("a\nb\r\nc\rd日e\n\nf")
	ats := []int{0, 2, 3, 5, 7, 8, 11, 13, 14}
	got := textpos.Each(src, ats)
	for i, at := range ats {
		line, column := textpos.Of(src, at)
		if got[i] != (textpos.Place{Line: line, Column: column}) {
			t.Errorf("Each at %d = %+v, Of = %d:%d", at, got[i], line, column)
		}
	}
}

// Each, going on from the place before on a line, tells each place as counting from the line's start does: at
// every byte, in a character too, a place twice and the end, among bytes that are no UTF-8 and line breaks.
func TestEachEveryByte(t *testing.T) {
	t.Parallel()

	src := []byte("aé日😀\xc3\xe3\x81x\xff\r\nb日\rc\n\x80é\n😀z")
	ats := make([]int, 0, 2*(len(src)+1))
	for at := range len(src) + 1 {
		ats = append(ats, at, at)
	}
	got := textpos.Each(src, ats)
	for i, at := range ats {
		// Counted afresh from the line's start, as a plain reference.
		line, start := 1, 0
		for j := range at {
			if src[j] == '\n' || src[j] == '\r' && (j+1 >= len(src) || src[j+1] != '\n') {
				line, start = line+1, j+1
			}
		}
		want := textpos.Place{Line: line, Column: utf8.RuneCount(src[start:at]) + 1}
		if got[i] != want {
			t.Errorf("Each at %d = %+v, want %+v", at, got[i], want)
		}
	}
}

// Each reads a long line once, not from its start for each place: a line of 1 MiB with 500,000 places.
func TestEachLinear(t *testing.T) { //nolint:paralleltest // it measures time, alone
	src := []byte(strings.Repeat("1,", 1<<19))
	ats := make([]int, 0, len(src)/2)
	for at := 0; at < len(src); at += 2 {
		ats = append(ats, at)
	}
	done := make(chan []textpos.Place, 1)
	go func() { done <- textpos.Each(src, ats) }()
	// Waited for apart, so that one counting from the line's start (minutes) fails here, not at go test's timeout.
	select {
	case got := <-done:
		if last := got[len(got)-1]; last != (textpos.Place{Line: 1, Column: len(src) - 1}) {
			t.Errorf("last place %+v", last)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Each took over 10s")
	}
}
