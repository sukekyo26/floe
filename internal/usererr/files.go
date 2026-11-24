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

// ErrNotLocal is what is not on a local drive under an ordinary name (Windows): a share, a network drive, a volume
// without a drive letter, a device, a pipe, a stream, or a link to one of them or of a form floe does not follow.
// Opening one may reach another machine or a server. A *NotLocal is it, with where it was found.
var ErrNotLocal = errors.New("not on a local drive under an ordinary name")

// NotLocal is ErrNotLocal at At: the volume, the name or the link's target that is not local.
type NotLocal struct {
	At string
}

// Error is short: path_not_local tells what it may be and what to do.
func (e *NotLocal) Error() string { return ErrNotLocal.Error() + ": " + e.At }

// Is tells it is ErrNotLocal.
func (*NotLocal) Is(target error) bool { return target == ErrNotLocal }

// CannotRead is err, of reading the file named so, at it: the system's words, without the paths it was given,
// which may be absolute, and floe's for ErrNotFile and ErrNotLocal; any other error is floe's failure.
func CannotRead(name string, err error) error {
	if errors.Is(err, ErrNotFile) {
		return NewAt(name, 0, 0, "kit_not_file")
	}
	if nl, ok := errors.AsType[*NotLocal](err); ok {
		return NewAt(name, 0, 0, "path_not_local", nl.At)
	}
	words, ok := systemWords(err)
	if !ok {
		return fmt.Errorf("%s: %w", name, err)
	}
	return LibraryAt(name, 0, 0, words)
}

// CannotWrite is err, of writing the file or the directory named so, at it: floe's words that it cannot be
// written, the system's words of why in them, and what to look at; ErrNotLocal as CannotRead tells it; any other
// error is floe's failure.
func CannotWrite(name string, err error) error {
	if errors.Is(err, ErrNotLocal) {
		return CannotRead(name, err)
	}
	return InSystemWords("cannot_write", name, err)
}

// SettingDir is err, of reading or writing dir: for the setting key that names dir, the system's words of why
// (ErrNotFile's too) with what to do, any other error floe's failure at dir; otherwise, made of err as the place
// is told without a setting, when key is "" (the default).
func SettingDir(key, dir string, err, otherwise error) error {
	if key == "" {
		return otherwise
	}
	words, ok := systemWords(err)
	switch {
	case errors.Is(err, ErrNotFile):
		words = ErrNotFile.Error()
	case !ok:
		return fmt.Errorf("%s: %w", dir, err)
	}
	return NewWhole("dir_setting", words, key, dir)
}

// InSystemWords is err, of reading or writing the file or the directory named so, at it, in the words of the
// catalog message key: the system's words of err (ErrNotFile's too, unlike CannotRead) its first argument and
// args after them, for what more there is to say; any other error is floe's failure.
func InSystemWords(key, name string, err error, args ...any) error {
	words, failed := wordsOf(name, err)
	if failed != nil {
		return failed
	}
	return NewAt(name, 0, 0, key, append([]any{words}, args...)...)
}

// InSystemWordsOf is err, of reading the file name names, in the words of the catalog message key, without a
// place, for the caller to put it where the name was written: the system's words of err its first argument,
// name its second and args after them; any other error is floe's failure.
func InSystemWordsOf(key, name string, err error, args ...any) error {
	words, failed := wordsOf(name, err)
	if failed != nil {
		return failed
	}
	return New(key, append([]any{words, name}, args...)...)
}

// wordsOf returns the system's words of err, of the file or the directory named so, or floe's failure for an
// error that has none.
func wordsOf(name string, err error) (string, error) {
	words, ok := systemWords(err)
	if !ok {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return words, nil
}

// systemWords returns the system's words of err, without the paths it names: those of an *fs.PathError, and of
// an *os.LinkError, which a rename gives; false for another error.
func systemWords(err error) (string, bool) {
	if pe, ok := errors.AsType[*fs.PathError](err); ok {
		return pe.Err.Error(), true
	}
	if le, ok := errors.AsType[*os.LinkError](err); ok {
		return le.Err.Error(), true
	}
	return "", false
}
