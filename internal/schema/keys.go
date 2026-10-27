package schema

import (
	"bytes"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"io"

	"github.com/sukekyo26/floe/internal/bom"
	"github.com/sukekyo26/floe/internal/textpos"
	"github.com/sukekyo26/floe/internal/usererr"
)

// LIMIT: encoding/json/jsontext is in Go 1.27 by default, under goexperiment.jsonv2, outside the Go 1
// compatibility promise. If a release drops or changes it, import github.com/go-json-experiment/json/jsontext,
// the same package, here.

// misread reads src, the JSON of file Trim has left, as jsontext does, the one reading of its syntax: the mistake
// jsontext stops at, in its words at its byte (a syntax mistake, or what encoding/json reads without a word: a
// lone surrogate or a byte that is no UTF-8 in a string), but one at a byte order mark, which bom.Refuse tells;
// each key another before it in its object has, at the key, told as it is read, and those before where jsontext
// stopped too; and what follows the value. An empty src is schema_empty. It reads the tokens alone, not what the
// keys mean.
// LIMIT: jsontext's words are told as they are, and quote src as Go does: a lone surrogate in a key shows as
// \\ud800. Tell how to mend it if jsontext comes to name the kind of its mistakes.
func misread(file string, src []byte) error {
	dec := jsontext.NewDecoder(bytes.NewReader(src), jsontext.AllowDuplicateNames(true))
	w := &keyWalk{file: file, src: src, dec: dec, found: nil}
	err := w.value()
	var syntax *jsontext.SyntacticError
	switch {
	case errors.Is(err, io.EOF): // no value at all: one cut short is a SyntacticError
		return usererr.NewAt(file, 0, 0, "schema_empty")
	case errors.As(err, &syntax):
		if !bom.At(src[syntax.ByteOffset:]) {
			line, column := textpos.Of(src, int(syntax.ByteOffset))
			w.found = append(w.found, usererr.LibraryAt(file, line, column, syntax.Err.Error()))
		}
	case err != nil:
		return fmt.Errorf("schema: read %s: %w", file, err)
	default:
		w.after()
	}
	return w.found.Err()
}

// keyWalk reads the tokens of src, the JSON of file, one value in another.
type keyWalk struct {
	file  string
	src   []byte
	dec   *jsontext.Decoder
	found usererr.List
}

// value reads one value, an object or an array with what it holds.
func (w *keyWalk) value() error {
	tok, err := w.dec.ReadToken()
	if err != nil {
		return err //nolint:wrapcheck // told by misread
	}
	switch tok.Kind() {
	case '{':
		names := map[string]bool{}
		for w.dec.PeekKind() != '}' {
			if err = w.key(names); err != nil {
				return err
			}
			if err = w.value(); err != nil {
				return err
			}
		}
	case '[':
		for w.dec.PeekKind() != ']' {
			if err = w.value(); err != nil {
				return err
			}
		}
	default:
		return nil
	}
	_, err = w.dec.ReadToken() // the } or the ]
	return err                 //nolint:wrapcheck // as above
}

// key reads a key of the object whose keys so far are names, refusing it when it is one of them.
func (w *keyWalk) key(names map[string]bool) error {
	at := keyStart(w.src, int(w.dec.InputOffset()))
	tok, err := w.dec.ReadToken()
	if err != nil {
		return err //nolint:wrapcheck // as above
	}
	name := tok.String()
	if names[name] {
		line, column := textpos.Of(w.src, at)
		w.found = append(w.found, usererr.NewAt(w.file, line, column, "schema_key_twice", name))
	}
	names[name] = true
	return nil
}

// after refuses what follows the value, past the spaces after it: jsontext reads values one after another. A
// byte order mark there is bom.Refuse's.
func (w *keyWalk) after() {
	at := int(w.dec.InputOffset())
	at += len(w.src[at:]) - len(bytes.TrimLeft(w.src[at:], " \t\n\r"))
	if at < len(w.src) && !bom.At(w.src[at:]) {
		line, column := textpos.Of(w.src, at)
		w.found = append(w.found, usererr.NewAt(w.file, line, column, "schema_after_value"))
	}
}

// keyStart is where the key read next from offset in src starts: past the spaces and the comma before it.
func keyStart(src []byte, offset int) int {
	for offset < len(src) && bytes.IndexByte([]byte(" \t\n\r,"), src[offset]) >= 0 {
		offset++
	}
	return offset
}
