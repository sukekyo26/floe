package schema

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sukekyo26/floe/internal/textpos"
	"github.com/sukekyo26/floe/internal/usererr"
)

// keysTwice refuses each key of src, the JSON of file the library has read, that another before it in its object
// has, at the key: the library reads the last of them, the others left out without a word. Names are compared
// as encoding/json decodes them, as the library keys its maps, so that lone surrogates one U+FFFD are one name.
// It reads src's tokens alone, not what the keys mean.
func keysTwice(file string, src []byte) (usererr.List, error) {
	w := &keyWalk{file: file, src: src, dec: json.NewDecoder(bytes.NewReader(src)), found: nil}
	w.dec.UseNumber() // as the library reads numbers: 1e400 is one
	if err := w.value(); err != nil {
		return nil, fmt.Errorf("schema: read the keys of %s, which its library read: %w", file, err)
	}
	return w.found, nil
}

// errKeyType is a key encoding/json reads as no string, which it never does: floe's failure.
var errKeyType = errors.New("a key read as no string")

// keyWalk reads the tokens of src, the JSON of file, one value in another.
type keyWalk struct {
	file  string
	src   []byte
	dec   *json.Decoder
	found usererr.List
}

// value reads one value, an object or an array with what it holds.
func (w *keyWalk) value() error {
	tok, err := w.dec.Token()
	if err != nil {
		return err //nolint:wrapcheck // floe's failure, told by keysTwice
	}
	switch tok {
	case json.Delim('{'):
		names := map[string]bool{}
		for w.dec.More() {
			if err = w.key(names); err != nil {
				return err
			}
			if err = w.value(); err != nil {
				return err
			}
		}
	case json.Delim('['):
		for w.dec.More() {
			if err = w.value(); err != nil {
				return err
			}
		}
	default:
		return nil
	}
	_, err = w.dec.Token() // the } or the ]
	return err             //nolint:wrapcheck // as above
}

// key reads a key of the object whose keys so far are names, refusing it when it is one of them.
func (w *keyWalk) key(names map[string]bool) error {
	at := keyStart(w.src, int(w.dec.InputOffset()))
	tok, err := w.dec.Token()
	if err != nil {
		return err //nolint:wrapcheck // as above
	}
	name, ok := tok.(string)
	if !ok {
		return fmt.Errorf("%w: %v", errKeyType, tok)
	}
	if names[name] {
		line, column := textpos.Of(w.src, at)
		w.found = append(w.found, usererr.NewAt(w.file, line, column, "schema_key_twice", name))
	}
	names[name] = true
	return nil
}

// keyStart is where the key read next from offset in src starts: past the spaces and the comma before it.
func keyStart(src []byte, offset int) int {
	for offset < len(src) && bytes.IndexByte([]byte(" \t\n\r,"), src[offset]) >= 0 {
		offset++
	}
	return offset
}
