// Package usererr carries a mistake the user can fix. It holds a message key and its arguments, and
// where the mistake is, and is worded in the reader's language where it is shown (the CLI's exit, an
// HTTP response): every layer makes, collects and shows its mistakes by it.
package usererr

import (
	"errors"
	"os"
	"path/filepath"
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

// Localize words the refusal in the language of c, as Render lays it out without floe's name: the mistakes
// it stands for told under it, all of them, whatever was told before it. An argument that is itself a
// Localizer (a worded part of the message, such as the name of a diagram part) is worded in c too.
func (e *Error) Localize(c *i18n.Catalog) string {
	return Tell(c, e).Render("")
}

// tell words e in the language of c, and the mistakes it stands for under it, told once of all those that
// told notes: another that stands for them says they are told above. It still tells the Under of one told
// above, into told, which notes nothing new as long as the same Of stands for the same Under.
// LIMIT: same Of, same Under, as every template reads its parts from the same dirs; if not, skip a told Under
func (e *Error) tell(c *i18n.Catalog, told map[string]bool) Told {
	args := make([]any, len(e.Args))
	for i, a := range e.Args {
		if l, ok := a.(i18n.Localizer); ok {
			a = l.Localize(c)
		}
		args[i] = a
	}
	t := Told{
		File: e.File, Line: e.Line, Column: e.Column, At: e.At, Text: c.Msg(e.Key, args...), Conflict: e.Conflict,
		Under: nil, Above: e.Of != "" && told[e.Of],
	}
	if t.Above {
		t.Text += c.Msg("told_above")
	} else if e.Of != "" {
		told[e.Of] = true
	}
	for _, u := range e.Under {
		t.Under = append(t.Under, u.tell(c, told))
	}
	return t
}

// CannotRead is the refusal for a file the user named that cannot be read, with the reason (no such
// file, permission denied) but not the path twice.
func CannotRead(file string, err error) *Error {
	return New("file_cannot_read", file, reason(err))
}

// CannotWrite is the refusal for a file floe cannot write where the user asked, with the reason: told by
// the step that could not be made when err names file or one of the directories above it (MkdirAll stops at
// the first it cannot make), so that "its directory" is the one to check; else by file.
func CannotWrite(file string, err error) *Error {
	var pe *os.PathError
	if errors.As(err, &pe) {
		if at := filepath.Clean(pe.Path); at == filepath.Clean(file) ||
			strings.HasPrefix(filepath.Clean(file), at+string(filepath.Separator)) {
			file = at
		}
	}
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
	return l.Localize(i18n.EnglishCatalog())
}

// Localize words the mistakes in the language of c, one per line, as Render lays them out without floe's
// name: the mistakes of a part or a theme that several stand for told under the first.
func (l List) Localize(c *i18n.Catalog) string {
	return Tell(c, l).Render("")
}
