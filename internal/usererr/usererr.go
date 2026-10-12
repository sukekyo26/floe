// Package usererr carries a mistake the user can fix. It holds a message key and its arguments, and
// is worded in the reader's language where it is shown (the CLI's exit, an HTTP response).
package usererr

import (
	"errors"
	"os"

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
	reason := err.Error()
	var pe *os.PathError
	if errors.As(err, &pe) {
		reason = pe.Err.Error()
	}
	return New("file_cannot_read", file, reason)
}
