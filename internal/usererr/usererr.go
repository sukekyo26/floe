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
// the place left out when it has none. The place is given where the mistake is made, or by PlacedAt and
// PlacedIn, and never to one of the whole output (NewWhole).
type Error struct {
	Key    string
	Args   []any
	file   string
	line   int    // 0: no line
	column int    // 0: no column
	at     string // a JSON pointer in file, "#/…"; "" for none
	whole  bool
}

// New returns an *Error for the catalog message key with args, without a place.
func New(key string, args ...any) *Error {
	return &Error{Key: key, Args: args, file: "", line: 0, column: 0, at: "", whole: false}
}

// NewWhole returns an *Error for the catalog message key with args of the whole output, which mending no
// input puts right (no icons fetched, a setting): it never takes a place, so that Tell tells it once.
func NewWhole(key string, args ...any) *Error {
	return &Error{Key: key, Args: args, file: "", line: 0, column: 0, at: "", whole: true}
}

// NewAt returns an *Error for the catalog message key with args, at the line and column of file, 0 for
// none.
func NewAt(file string, line, column int, key string, args ...any) *Error {
	return &Error{Key: key, Args: args, file: file, line: line, column: column, at: "", whole: false}
}

// NewIn returns an *Error for the catalog message key with args, at at, a JSON pointer, in file.
func NewIn(file, at, key string, args ...any) *Error {
	return &Error{Key: key, Args: args, file: file, line: 0, column: 0, at: at, whole: false}
}

// File is the file the mistake is in, "" for none.
func (e *Error) File() string { return e.file }

// Line is the mistake's line in File, 0 for none.
func (e *Error) Line() int { return e.line }

// Column is the mistake's column in Line, 0 for none.
func (e *Error) Column() int { return e.column }

// At is the mistake's JSON pointer in File, "" for none.
func (e *Error) At() string { return e.at }

// PlacedAt returns a copy of e at the line and column of file, 0 for none, and no JSON pointer, as NewAt
// places one; e itself when it is of the whole output.
func (e *Error) PlacedAt(file string, line, column int) *Error {
	if e.whole {
		return e
	}
	c := *e
	c.file, c.line, c.column, c.at = file, line, column, ""
	return &c
}

// PlacedIn returns a copy of e at at, a JSON pointer in the value whoever holds it places; e itself when it is
// of the whole output, or has a file of its own, a place that goes before one in the value.
func (e *Error) PlacedIn(at string) *Error {
	if e.whole || e.file != "" {
		return e
	}
	c := *e
	c.at = at
	return &c
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
		if _, ok := first[e.file]; !ok {
			first[e.file] = len(first)
		}
	}
	slices.SortStableFunc(l, func(x, y *Error) int {
		if d := first[x.file] - first[y.file]; d != 0 {
			return d
		}
		return cmp.Or(x.line-y.line, x.column-y.column, comparePointers(x.at, y.at), strings.Compare(x.Key, y.Key),
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
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
	At     string `json:"at,omitempty"`
	Text   string `json:"text"`
}

// Report is an error as told: the user's mistakes, or what floe failed at, never both; and the warnings of what
// was drawn, with the mistakes or alone. Render tells a failure alone, its warnings left out.
type Report struct {
	Mistakes []Told `json:"mistakes,omitempty"`
	Failure  string `json:"failure,omitempty"`
	Warnings []Told `json:"warnings,omitempty"`
}

// Tell words err in the language of c: its mistakes, as List.Add takes them, in the order given, each told
// once (one worded at the same place as one before it is left out, wherever it was made), or, for an error
// that is not the user's or holds no mistake, floe's failure. It is the only place a mistake is worded. The
// words of what wraps a mistake are not told: a place or a name goes into the mistake's arguments or place,
// not into fmt.Errorf. An error that carries a report told already (the server's, to the command line) is told as
// it is.
func Tell(c *i18n.Catalog, err error) Report {
	if w, ok := errors.AsType[*warned](err); ok {
		r := Tell(c, w.err)
		if r.Failure == "" {
			r.Warnings = append(r.Warnings, Warn(c, w.warnings)...)
		}
		return r
	}
	if reported, ok := errors.AsType[reporter](err); ok {
		return reported.Report()
	}
	var all List
	if failed := all.Add(err); failed != nil {
		return Report{Mistakes: nil, Failure: cmp.Or(failed.Error(), fmt.Sprintf("%T", failed)), Warnings: nil}
	}
	if len(all) == 0 {
		return Report{Mistakes: nil, Failure: fmt.Sprintf("%T with no mistake", err), Warnings: nil}
	}
	return Report{Mistakes: worded(c, all), Failure: "", Warnings: nil}
}

// Warn words warnings, advice on what was drawn, in the language of c as Tell words mistakes: in the order Sorted
// gives a copy of them, each told once. A warning is an *Error carried apart from the error, never in it: not in
// a List, which is one.
func Warn(c *i18n.Catalog, warnings []*Error) []Told {
	return worded(c, List(slices.Clone(warnings)).Sorted())
}

// worded words l in the language of c, in order, each told once: one worded at the same place as one before it is
// left out, wherever it was made.
func worded(c *i18n.Catalog, l List) []Told {
	all := make([]Told, 0, len(l))
	told := map[Told]bool{}
	for _, e := range l {
		t := Told{File: e.file, Line: e.line, Column: e.column, At: e.at, Text: c.Msg(e.Key, e.Args...)}
		if !told[t] {
			told[t] = true
			all = append(all, t)
		}
	}
	return all
}

// warned is an error made after a page was drawn, with the warnings of that page.
type warned struct {
	err      error
	warnings []*Error
}

func (w *warned) Error() string { return w.err.Error() }

func (w *warned) Unwrap() error { return w.err }

// WithWarnings returns err, made after a page was drawn, with the warnings of that page, which Tell tells after its
// mistakes, so that a refusal after the drawing does not lose them; err itself without warnings, nil for none.
func WithWarnings(err error, warnings []*Error) error {
	if err == nil || len(warnings) == 0 {
		return err
	}
	return &warned{err: err, warnings: warnings}
}

// reporter is an error that carries a report told already.
type reporter interface {
	error
	Report() Report
}

// Render lays r out, the only place that does: each mistake on a line, after where it is,
// file:line:column, or as much of it as it has, then each warning so, its text after "warning: "; name before
// each that is not at a place, and before floe's failure, which is told alone. The command line names floe so;
// a page in the browser names nothing ("").
func (r Report) Render(name string) string {
	if r.Failure != "" {
		return name + r.Failure
	}
	lines := make([]string, 0, len(r.Mistakes)+len(r.Warnings))
	for _, t := range r.Mistakes {
		lines = append(lines, t.line(name))
	}
	for _, t := range r.Warnings {
		// LIMIT: the word is English, as only the command line lays warnings out; take it from the catalog if a
		// page comes to show them.
		t.Text = "warning: " + t.Text
		lines = append(lines, t.line(name))
	}
	return strings.Join(lines, "\n")
}

// line lays t out after where it is: file:line:column, or as much of it as it has, then a JSON pointer; after
// name when it is at no file.
func (t Told) line(name string) string {
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
	line := strings.Join(append(where, t.Text), ": ")
	if t.File == "" {
		return name + line
	}
	return line
}
