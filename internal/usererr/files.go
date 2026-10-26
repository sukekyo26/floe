package usererr

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// CannotRead is err, of reading the file named so, at it: the system's words, without the paths it was given,
// which may be absolute; any other error is floe's failure.
func CannotRead(name string, err error) error {
	words, ok := systemWords(err)
	if !ok {
		return fmt.Errorf("%s: %w", name, err)
	}
	return LibraryAt(name, 0, 0, words)
}

// CannotWrite is err, of writing the file or the directory named so, at it: floe's words that it cannot be
// written, the system's words of why in them, and what to look at; any other error is floe's failure.
func CannotWrite(name string, err error) error {
	words, ok := systemWords(err)
	if !ok {
		return fmt.Errorf("%s: %w", name, err)
	}
	return NewAt(name, 0, 0, "cannot_write", words)
}

// systemWords returns the system's words of err, without the paths it names: those of an *fs.PathError, and of
// an *os.LinkError, which a rename gives; false for another error.
func systemWords(err error) (string, bool) {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error(), true
	}
	var le *os.LinkError
	if errors.As(err, &le) {
		return le.Err.Error(), true
	}
	return "", false
}
