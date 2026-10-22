// Package usererr carries a mistake the user can fix. It holds a message key and its arguments, and
// where the mistake is, and is worded in the reader's language where it is shown (the CLI's exit, an
// HTTP response): every layer makes, collects and shows its mistakes by it.
package usererr

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/sukekyo26/floe/internal/i18n"
)

// Error is a mistake the user can fix: what (a message key and its arguments) and where, each part of
// the place left out when it has none. Conflict marks a name already in use (HTTP 409 rather than 400).
type Error struct {
	Key      string
	Args     []any
	Conflict bool
	File     string
	Line     int    // 0: no line
	Column   int    // 0: no column
	At       string // a JSON pointer in a schema, "" for none
	Under    List   // the mistakes it stands for: those of a part that cannot be used
	Of       string // what Under is the mistakes of, such as "part:quote", told once in a list
}

// New returns an *Error for the catalog message key with args, without a place.
func New(key string, args ...any) *Error {
	return &Error{Key: key, Args: args, Conflict: false, File: "", Line: 0, Column: 0, At: "", Under: nil, Of: ""}
}

// NewAt returns an *Error for the catalog message key with args, at the line and column of file, 0 for
// none.
func NewAt(file string, line, column int, key string, args ...any) *Error {
	return &Error{
		Key: key, Args: args, Conflict: false, File: file, Line: line, Column: column, At: "", Under: nil, Of: "",
	}
}

// NewIn returns an *Error for the catalog message key with args, at at, a JSON pointer, in file.
func NewIn(file, at, key string, args ...any) *Error {
	return &Error{Key: key, Args: args, Conflict: false, File: file, Line: 0, Column: 0, At: at, Under: nil, Of: ""}
}

// Over returns an *Error for the catalog message key with args that stands for the mistakes under, those
// of of, such as "part:quote": told under it, once in a list, where another that stands for them says
// they are told above.
func Over(of string, under List, key string, args ...any) *Error {
	return &Error{Key: key, Args: args, Conflict: false, File: "", Line: 0, Column: 0, At: "", Under: under, Of: of}
}

// Placed returns e at the line and column of file, 0 for none, and at, "" for none, in place of any
// place it had.
func (e *Error) Placed(file string, line, column int, at string) *Error {
	placed := *e
	placed.File, placed.Line, placed.Column, placed.At = file, line, column, at
	return &placed
}

// Error words the refusal in English, for logs and for callers that do not localize.
func (e *Error) Error() string {
	return e.Localize(i18n.EnglishCatalog())
}

// Localize words the refusal in the language of c, after where it is, file:line:column: at, or as much of
// it as it has, and the mistakes it stands for under it, indented. An argument that is itself a Localizer
// (a worded part of the message, such as the name of a diagram part) is worded in c too.
func (e *Error) Localize(c *i18n.Catalog) string {
	return e.told(c, map[string]bool{})
}

// told words e as Localize does, the mistakes it stands for told once of all those that told notes.
func (e *Error) told(c *i18n.Catalog, told map[string]bool) string {
	args := make([]any, len(e.Args))
	for i, a := range e.Args {
		if l, ok := a.(i18n.Localizer); ok {
			a = l.Localize(c)
		}
		args[i] = a
	}
	var where []string
	switch {
	case e.File == "":
	case e.Line == 0:
		where = append(where, e.File)
	case e.Column == 0:
		where = append(where, fmt.Sprintf("%s:%d", e.File, e.Line))
	default:
		where = append(where, fmt.Sprintf("%s:%d:%d", e.File, e.Line, e.Column))
	}
	if e.At != "" {
		where = append(where, e.At)
	}
	text := strings.Join(append(where, c.Msg(e.Key, args...)), ": ")
	switch {
	case e.Of != "" && told[e.Of]:
		return text + c.Msg("told_above")
	case e.Of != "":
		told[e.Of] = true
	}
	for _, u := range e.Under {
		text += "\n  " + strings.ReplaceAll(u.told(c, told), "\n", "\n  ")
	}
	return text
}

// CannotRead is the refusal for a file the user named that cannot be read, with the reason (no such
// file, permission denied) but not the path twice.
func CannotRead(file string, err error) *Error {
	return New("file_cannot_read", file, reason(err))
}

// CannotWrite is the refusal for a file floe cannot write where the user asked, with the reason.
func CannotWrite(file string, err error) *Error {
	return New("file_cannot_write", file, reason(err))
}

// reason is why a file could not be read or written, without its paths.
func reason(err error) string {
	var pe *os.PathError
	var le *os.LinkError
	switch {
	case errors.As(err, &pe):
		return pe.Err.Error()
	case errors.As(err, &le):
		return le.Err.Error()
	}
	return err.Error()
}

// List is mistakes told together, one per line.
type List []*Error

// Add adds err to l: an Error or a List; a nil *Error, which a function returning *Error gives for none,
// is left out. Any other error is a failure, not a mistake, and is returned for the caller to stop at.
func (l *List) Add(err error) error {
	var more List
	var one *Error
	switch {
	case err == nil:
	case errors.As(err, &more):
		*l = append(*l, more...)
	case errors.As(err, &one):
		if one != nil {
			*l = append(*l, one)
		}
	default:
		return err
	}
	return nil
}

// AddPlaced adds err to l as Add does, but returns a mistake without a place, as it returns a failure:
// for a stage that stops at a mistake about the whole of it (icons not fetched), told alone.
func (l *List) AddPlaced(err error) error {
	var one *Error
	if errors.As(err, &one) && one != nil && one.File == "" {
		return err
	}
	return l.Add(err)
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
	return l.Localize(i18n.EnglishCatalog())
}

// Localize words the mistakes in the language of c, one per line, the mistakes of a part or a theme that
// several stand for told under the first.
func (l List) Localize(c *i18n.Catalog) string {
	told := map[string]bool{}
	lines := make([]string, len(l))
	for i, e := range l {
		lines[i] = e.told(c, told)
	}
	return strings.Join(lines, "\n")
}

// Shown is how an error is told: its text in the reader's language; whether the user can fix it (a
// refusal, 400, not floe's failure, 500); whether it is placed mistakes, told as they are, one per line,
// for editors and CI to read, without floe's name before them; and whether it is a name in use (409).
type Shown struct {
	Text     string
	Fixable  bool
	Placed   bool
	Conflict bool
}

// Show tells how err is shown in the language of c: a List is placed, and so is an Error with a file.
func Show(c *i18n.Catalog, err error) Shown {
	s := Shown{Text: err.Error(), Fixable: false, Placed: false, Conflict: false}
	var l i18n.Localizer
	if errors.As(err, &l) {
		s.Text, s.Fixable = l.Localize(c), true
	}
	var all List
	var one *Error
	switch {
	case errors.As(err, &all):
		s.Placed = true
	case errors.As(err, &one):
		s.Placed, s.Conflict = one.File != "", one.Conflict
	}
	return s
}
