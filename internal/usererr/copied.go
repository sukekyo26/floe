package usererr

import (
	"errors"
	"fmt"
	"reflect"
)

// Copied is an *Error as one process hands it to another (the process that draws a page, to the server), in JSON:
// its key, its arguments and its place, the mark of one of the whole output kept.
type Copied struct {
	Key    string `json:"key"`
	Args   []Arg  `json:"args,omitzero"`
	File   string `json:"file,omitzero"`
	Line   int    `json:"line,omitzero"`
	Column int    `json:"column,omitzero"`
	At     string `json:"at,omitzero"`
	Whole  bool   `json:"whole,omitzero"`
}

// Arg is an argument of a Copied: a text, a whole number or a number, the kinds the catalog's verbs format.
type Arg struct {
	Text   *string  `json:"text,omitzero"`
	Int    *int64   `json:"int,omitzero"`
	Number *float64 `json:"number,omitzero"`
}

// errArgKind is the failure of copying an argument of a kind the catalog's verbs do not take.
var errArgKind = errors.New("a kind of argument floe does not copy")

// Copy is e as Copied, or floe's failure for an argument of another kind, which the copy would word otherwise.
func Copy(e *Error) (Copied, error) {
	c := Copied{
		Key: e.Key, Args: make([]Arg, len(e.Args)), File: e.file, Line: e.line, Column: e.column, At: e.at, Whole: e.whole,
	}
	for i, a := range e.Args {
		v := reflect.ValueOf(a)
		switch {
		case v.Kind() == reflect.String:
			s := v.String()
			c.Args[i].Text = &s
		case v.CanInt():
			n := v.Int()
			c.Args[i].Int = &n
		case v.CanUint() && v.Uint() <= 1<<63-1:
			n := int64(v.Uint()) //nolint:gosec // G115: checked above
			c.Args[i].Int = &n
		case v.CanFloat():
			f := v.Float()
			c.Args[i].Number = &f
		default:
			return Copied{}, fmt.Errorf("copy the mistake %s: its argument %d is a %T: %w", e.Key, i, a, errArgKind)
		}
	}
	return c, nil
}

// Error is the *Error c copies.
func (c Copied) Error() *Error {
	args := make([]any, len(c.Args))
	for i, a := range c.Args {
		switch {
		case a.Text != nil:
			args[i] = *a.Text
		case a.Int != nil:
			args[i] = *a.Int
		case a.Number != nil:
			args[i] = *a.Number
		}
	}
	return &Error{Key: c.Key, Args: args, file: c.File, line: c.Line, column: c.Column, at: c.At, whole: c.Whole}
}
