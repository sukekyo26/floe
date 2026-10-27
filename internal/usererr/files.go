package usererr

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// ErrNotFile is what is neither a regular file nor a directory (a FIFO, a device, a socket): floe may never finish
// reading it, and opening one may do more than open it.
var ErrNotFile = errors.New("neither a regular file nor a directory")

// CannotRead is err, of reading the file named so, at it: the system's words, without the paths it was given,
// which may be absolute, and floe's for ErrNotFile; any other error is floe's failure.
func CannotRead(name string, err error) error {
	if errors.Is(err, ErrNotFile) {
		return NewAt(name, 0, 0, "kit_not_file")
	}
	words, ok := systemWords(err)
	if !ok {
		return fmt.Errorf("%s: %w", name, err)
	}
	return LibraryAt(name, 0, 0, words)
}

// CannotWrite is err, of writing the file or the directory named so, at it: floe's words that it cannot be
// written, the system's words of why in them, and what to look at; any other error is floe's failure.
func CannotWrite(name string, err error) error {
	return InSystemWords("cannot_write", name, err)
}

// SettingDir is err, of reading or writing dir, which the setting key names: the system's words of why, with what
// to do; any other error is floe's failure, at dir.
func SettingDir(key, dir string, err error) error {
	words, ok := systemWords(err)
	if !ok {
		return fmt.Errorf("%s: %w", dir, err)
	}
	return New("dir_setting", words, key, dir)
}

// InSystemWords is err, of reading or writing the file or the directory named so, at it, in the words of the
// catalog message key: the system's words of err (ErrNotFile's too, unlike CannotRead) its first argument and
// args after them, for what more there is to say; any other error is floe's failure.
func InSystemWords(key, name string, err error, args ...any) error {
	words, ok := systemWords(err)
	if !ok {
		return fmt.Errorf("%s: %w", name, err)
	}
	return NewAt(name, 0, 0, key, append([]any{words}, args...)...)
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
