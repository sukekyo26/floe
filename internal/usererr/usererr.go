// Package usererr carries a mistake the user can fix. It holds a message key and its arguments, and
// is worded in the reader's language where it is shown (the CLI's exit, an HTTP response).
package usererr

import (
	"errors"
	"os"
	"strings"

	"github.com/sukekyo26/floe/internal/i18n"
)

// Error is a refusal. Conflict marks a name already in use (HTTP 409 rather than 400).
type Error struct {
	Key      string
	Args     []any
	Conflict bool
}

// New returns an *Error for the catalog message key with args.
func New(key string, args ...any) *Error {
	return &Error{Key: key, Args: args, Conflict: false}
}

// Error words the refusal in English, for logs and for callers that do not localize.
func (e *Error) Error() string {
	return e.Localize(i18n.EnglishCatalog())
}

// Localize words the refusal in the language of c. An argument that is itself a Localizer (a
// worded part of the message, such as the name of a diagram part) is worded in c too.
func (e *Error) Localize(c *i18n.Catalog) string {
	args := make([]any, len(e.Args))
	for i, a := range e.Args {
		if l, ok := a.(i18n.Localizer); ok {
			a = l.Localize(c)
		}
		args[i] = a
	}
	return c.Msg(e.Key, args...)
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

// List is mistakes told together, one per line, each in the reader's language where it can be.
type List []error

// Error words the mistakes in English, one per line.
func (l List) Error() string {
	return l.Localize(i18n.EnglishCatalog())
}

// Localize words the mistakes in the language of c, one per line.
func (l List) Localize(c *i18n.Catalog) string {
	lines := make([]string, len(l))
	for i, e := range l {
		lines[i] = e.Error()
		var loc i18n.Localizer
		if errors.As(e, &loc) {
			lines[i] = loc.Localize(c)
		}
	}
	return strings.Join(lines, "\n")
}

// Join returns the mistakes of errs that are not nil: nil for none, the one alone, else a List of them.
func Join(errs ...error) error {
	var l List
	for _, err := range errs {
		if err != nil {
			l = append(l, err)
		}
	}
	switch len(l) {
	case 0:
		return nil
	case 1:
		return l[0]
	}
	return l
}
