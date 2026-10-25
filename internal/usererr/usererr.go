// Package usererr carries a mistake the user can fix. It holds a message key and its arguments, and is
// worded in the reader's language where it is shown (the CLI's exit): every layer makes and shows its
// mistakes by it.
package usererr

import (
	"cmp"
	"errors"
	"fmt"
	"strings"

	"github.com/sukekyo26/floe/internal/i18n"
)

// Error is a mistake the user can fix: a message key and its arguments.
type Error struct {
	Key  string
	Args []any
}

// New returns an *Error for the catalog message key with args.
func New(key string, args ...any) *Error {
	return &Error{Key: key, Args: args}
}

// Library returns the mistake a library told in err, in the library's own words.
func Library(err error) *Error {
	return New("library", err.Error())
}

// Error words the mistake in English, for logs and for callers that do not localize.
func (e *Error) Error() string {
	return Tell(i18n.EnglishCatalog(), e).Render("")
}

// Told is a mistake as told: worded in the reader's language.
type Told struct {
	Text string
}

// Report is an error as told: the user's mistakes, or what floe failed at, never both.
type Report struct {
	Mistakes []Told
	Failure  string
}

// Tell words err in the language of c: the *Error it is or wraps, or, for an error that is not the user's
// or holds no mistake, floe's failure. It is the only place a mistake is worded.
func Tell(c *i18n.Catalog, err error) Report {
	var one *Error
	isMistake := errors.As(err, &one)
	switch {
	case one != nil:
		return Report{Mistakes: []Told{{Text: c.Msg(one.Key, one.Args...)}}, Failure: ""}
	case isMistake || err == nil:
		return Report{Mistakes: nil, Failure: fmt.Sprintf("%T with no mistake", err)}
	}
	return Report{Mistakes: nil, Failure: cmp.Or(err.Error(), fmt.Sprintf("%T", err))}
}

// Render lays r out, the only place that does: each mistake on a line, name before each and before floe's
// failure. The command line names floe so; a page in the browser names nothing ("").
func (r Report) Render(name string) string {
	if r.Failure != "" {
		return name + r.Failure
	}
	lines := make([]string, len(r.Mistakes))
	for i, t := range r.Mistakes {
		lines[i] = name + t.Text
	}
	return strings.Join(lines, "\n")
}
