// Package usererr carries a mistake the user can fix. It holds a message key and its arguments, and
// where the mistake is, and is worded in the reader's language where it is shown (the CLI's exit): every
// layer makes, collects and shows its mistakes by it.
package usererr

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/sukekyo26/floe/internal/i18n"
)

// Error is a mistake the user can fix: what (a message key and its arguments) and where, each part of
// the place left out when it has none.
type Error struct {
	Key    string
	Args   []any
	File   string
	Line   int // 0: no line
	Column int // 0: no column
}

// New returns an *Error for the catalog message key with args, without a place.
func New(key string, args ...any) *Error {
	return &Error{Key: key, Args: args, File: "", Line: 0, Column: 0}
}

// NewAt returns an *Error for the catalog message key with args, at the line and column of file, 0 for
// none.
func NewAt(file string, line, column int, key string, args ...any) *Error {
	return &Error{Key: key, Args: args, File: file, Line: line, Column: column}
}

// Library returns the mistake a library told in err, in the library's own words.
func Library(err error) *Error {
	return New("library", err.Error())
}

// LibraryAt returns the mistake a library told in text, in its own words, at the line and column of file
// it gave, 0 for none.
func LibraryAt(file string, line, column int, text string) *Error {
	return NewAt(file, line, column, "library", text)
}

// Error words the mistake in English, for logs and for callers that do not localize.
func (e *Error) Error() string {
	return Tell(i18n.EnglishCatalog(), e).Render("")
}

// List is mistakes told together, one per line.
type List []*Error

// Add adds err to l: an Error or a List; a nil *Error, which a function returning *Error gives for none,
// is left out, in a List too. Any other error is a failure, not a mistake, and is returned for the caller to stop at.
func (l *List) Add(err error) error {
	var more List
	var one *Error
	switch {
	case err == nil:
	case errors.As(err, &more):
		for _, e := range more {
			if e != nil {
				*l = append(*l, e)
			}
		}
	case errors.As(err, &one):
		if one != nil {
			*l = append(*l, one)
		}
	default:
		return err
	}
	return nil
}

// Collect returns the mistakes of errs together, as Add takes them, sorted, nil for none, or the first
// failure among them: for the checks of a stage that do not depend on one another.
func Collect(errs ...error) error {
	var l List
	for _, err := range errs {
		if failed := l.Add(err); failed != nil {
			return failed
		}
	}
	return l.Sorted().Err()
}

// Err returns l as it is, or nil when it holds none: a List of none is no error.
func (l List) Err() error {
	if len(l) == 0 {
		return nil
	}
	return l
}

// Sorted returns l with the mistakes of each file in the order of their lines and columns, the files in
// the order they first come in l; those without a file come together where the first of them is.
func (l List) Sorted() List {
	first := map[string]int{}
	for _, e := range l {
		if _, ok := first[e.File]; !ok {
			first[e.File] = len(first)
		}
	}
	slices.SortStableFunc(l, func(x, y *Error) int {
		if d := first[x.File] - first[y.File]; d != 0 {
			return d
		}
		if d := x.Line - y.Line; d != 0 {
			return d
		}
		return x.Column - y.Column
	})
	return l
}

// Error words the mistakes in English, one per line.
func (l List) Error() string {
	return Tell(i18n.EnglishCatalog(), l).Render("")
}

// Told is a mistake as told: worded in the reader's language, and where it is.
type Told struct {
	File         string
	Line, Column int
	Text         string
}

// Report is an error as told: the user's mistakes, or what floe failed at, never both.
type Report struct {
	Mistakes []Told
	Failure  string
}

// Tell words err in the language of c: its mistakes, as List.Add takes them, in the order given, or, for an
// error that is not the user's or holds no mistake, floe's failure. It is the only place a mistake is
// worded. The words of what wraps a mistake are not told: a place or a name goes into the mistake's
// arguments or place, not into fmt.Errorf.
func Tell(c *i18n.Catalog, err error) Report {
	var all List
	if failed := all.Add(err); failed != nil {
		return Report{Mistakes: nil, Failure: cmp.Or(failed.Error(), fmt.Sprintf("%T", failed))}
	}
	if len(all) == 0 {
		return Report{Mistakes: nil, Failure: fmt.Sprintf("%T with no mistake", err)}
	}
	r := Report{Mistakes: make([]Told, 0, len(all)), Failure: ""}
	for _, e := range all {
		r.Mistakes = append(r.Mistakes, Told{File: e.File, Line: e.Line, Column: e.Column, Text: c.Msg(e.Key, e.Args...)})
	}
	return r
}

// Render lays r out, the only place that does: each mistake on a line, after where it is,
// file:line:column, or as much of it as it has; name before each that is not at a place, and before
// floe's failure. The command line names floe so; a page in the browser names nothing ("").
func (r Report) Render(name string) string {
	if r.Failure != "" {
		return name + r.Failure
	}
	lines := make([]string, len(r.Mistakes))
	for i, t := range r.Mistakes {
		lines[i] = t.line()
		if t.File == "" {
			lines[i] = name + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}

// line lays t out after where it is.
func (t Told) line() string {
	switch {
	case t.File == "":
		return t.Text
	case t.Line == 0:
		return t.File + ": " + t.Text
	case t.Column == 0:
		return fmt.Sprintf("%s:%d: %s", t.File, t.Line, t.Text)
	}
	return fmt.Sprintf("%s:%d:%d: %s", t.File, t.Line, t.Column, t.Text)
}
