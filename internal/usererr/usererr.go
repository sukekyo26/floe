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
	"unicode/utf8"

	"github.com/sukekyo26/floe/internal/i18n"
)

// Error is a mistake the user can fix: what (a message key and its arguments) and where, each part of
// the place left out when it has none.
type Error struct {
	Key    string
	Args   []any
	File   string
	Line   int    // 0: no line
	Column int    // 0: no column
	At     string // a JSON pointer in File, "#/…"; "" for none
}

// New returns an *Error for the catalog message key with args, without a place.
func New(key string, args ...any) *Error {
	return &Error{Key: key, Args: args, File: "", Line: 0, Column: 0, At: ""}
}

// NewAt returns an *Error for the catalog message key with args, at the line and column of file, 0 for
// none.
func NewAt(file string, line, column int, key string, args ...any) *Error {
	return &Error{Key: key, Args: args, File: file, Line: line, Column: column, At: ""}
}

// NewIn returns an *Error for the catalog message key with args, at at, a JSON pointer, in file.
func NewIn(file, at, key string, args ...any) *Error {
	return &Error{Key: key, Args: args, File: file, Line: 0, Column: 0, At: at}
}

// Library returns the mistake a library told in err, in the library's own words.
func Library(err error) *Error {
	return LibraryWith("library", err)
}

// LibraryWith returns the mistake a library told in err, its words the argument of the catalog message key, which
// tells what to do besides: the one way a library's words go into a message of floe's.
func LibraryWith(key string, err error) *Error {
	return New(key, valid(err.Error()))
}

// LibraryAt returns the mistake a library told in text, in its own words, at the line and column of file
// it gave, 0 for none.
func LibraryAt(file string, line, column int, text string) *Error {
	return NewAt(file, line, column, "library", valid(text))
}

// LibraryIn returns the mistake a library told in text, in its own words, at at, a JSON pointer, in file.
func LibraryIn(file, at, text string) *Error {
	return NewIn(file, at, "library", valid(text))
}

// valid is a library's text with each run of bytes that are no UTF-8 written as U+FFFD, so that it shows as
// written wherever it is told.
func valid(text string) string {
	return strings.ToValidUTF8(text, string(utf8.RuneError))
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

// Sorted returns l with the mistakes of each file in the order of their lines, columns and JSON pointers,
// the files in the order they first come in l; those without a file come together where the first of them
// is. Mistakes at one place are in the order of their keys and arguments, so that the order does not follow
// how they were found (a library's map order).
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
		return cmp.Or(x.Line-y.Line, x.Column-y.Column, comparePointers(x.At, y.At), strings.Compare(x.Key, y.Key),
			slices.CompareFunc(x.Args, y.Args, func(a, b any) int { return strings.Compare(fmt.Sprint(a), fmt.Sprint(b)) }))
	})
	return l
}

// comparePointers compares two JSON pointers step by step, in one order of all steps: an index (0, or digits
// not beginning with 0) before any other step, indexes by their number (#/allOf/2 before #/allOf/10), the
// others as text.
func comparePointers(x, y string) int {
	return slices.CompareFunc(strings.Split(x, "/"), strings.Split(y, "/"), func(a, b string) int {
		ia, ib := isIndex(a), isIndex(b)
		switch {
		case ia && ib:
			return cmp.Or(cmp.Compare(len(a), len(b)), strings.Compare(a, b))
		case ia:
			return -1
		case ib:
			return 1
		}
		return strings.Compare(a, b)
	})
}

// isIndex reports whether step is an index of an array as a JSON pointer writes it: 0, or digits not
// beginning with 0.
func isIndex(step string) bool {
	if step == "" || step != "0" && step[0] == '0' {
		return false
	}
	return strings.Trim(step, "0123456789") == ""
}

// Error words the mistakes in English, one per line.
func (l List) Error() string {
	return Tell(i18n.EnglishCatalog(), l).Render("")
}

// Told is a mistake as told: worded in the reader's language, and where it is.
type Told struct {
	File         string
	Line, Column int
	At           string
	Text         string
}

// Report is an error as told: the user's mistakes, or what floe failed at, never both.
type Report struct {
	Mistakes []Told
	Failure  string
}

// Tell words err in the language of c: its mistakes, as List.Add takes them, in the order given, each told
// once (one worded at the same place as one before it is left out, wherever it was made), or, for an error
// that is not the user's or holds no mistake, floe's failure. It is the only place a mistake is worded. The
// words of what wraps a mistake are not told: a place or a name goes into the mistake's arguments or place,
// not into fmt.Errorf.
func Tell(c *i18n.Catalog, err error) Report {
	var all List
	if failed := all.Add(err); failed != nil {
		return Report{Mistakes: nil, Failure: cmp.Or(failed.Error(), fmt.Sprintf("%T", failed))}
	}
	if len(all) == 0 {
		return Report{Mistakes: nil, Failure: fmt.Sprintf("%T with no mistake", err)}
	}
	r := Report{Mistakes: make([]Told, 0, len(all)), Failure: ""}
	told := map[Told]bool{}
	for _, e := range all {
		t := Told{File: e.File, Line: e.Line, Column: e.Column, At: e.At, Text: c.Msg(e.Key, e.Args...)}
		if !told[t] {
			told[t] = true
			r.Mistakes = append(r.Mistakes, t)
		}
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

// line lays t out after where it is: file:line:column, or as much of it as it has, then a JSON pointer.
func (t Told) line() string {
	var where []string
	switch {
	case t.File == "":
	case t.Line == 0:
		where = append(where, t.File)
	case t.Column == 0:
		where = append(where, fmt.Sprintf("%s:%d", t.File, t.Line))
	default:
		where = append(where, fmt.Sprintf("%s:%d:%d", t.File, t.Line, t.Column))
	}
	if t.At != "" {
		where = append(where, t.At)
	}
	return strings.Join(append(where, t.Text), ": ")
}
