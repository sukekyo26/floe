package files

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// Root is the directory dir as an fs.FS that the system keeps its reads under (os.Root), opened when first used,
// so that a page that reads nothing of it draws when it cannot be opened. Its files are opened as Open opens a
// path: a FIFO, a device or a socket is refused unopened. Its maker closes it.
type Root struct {
	dir  string
	once sync.Once
	root *os.Root
	err  error
}

// NewRoot returns the directory dir to read under, not opened yet.
func NewRoot(dir string) *Root {
	return &Root{dir: dir, once: sync.Once{}, root: nil, err: nil}
}

// DirError is why a Root's directory cannot be opened, whatever is read in it.
type DirError struct {
	Dir string
	Err error // the system's, an *fs.PathError
}

func (e *DirError) Error() string { return e.Dir + ": " + e.Err.Error() }

// Unwrap gives the system's error, for its words.
func (e *DirError) Unwrap() error { return e.Err }

func (r *Root) ready() error {
	r.once.Do(func() {
		var err error
		if r.root, err = OpenRoot(r.dir); err != nil {
			r.err = &DirError{Dir: r.dir, Err: err}
		}
	})
	return r.err
}

// Open opens name under the directory, as Open opens a path.
func (r *Root) Open(name string) (fs.File, error) {
	if err := r.ready(); err != nil {
		return nil, err
	}
	return openRead(r.root, name)
}

// Stat tells of name under the directory without opening it: of a FIFO too, which Open refuses.
func (r *Root) Stat(name string) (fs.FileInfo, error) {
	if err := r.ready(); err != nil {
		return nil, err
	}
	if _, err := checkName("stat", name); err != nil {
		return nil, err
	}
	return r.root.Stat(name) //nolint:wrapcheck // the system's error, at name
}

// Close lets go of the directory, if it was opened.
func (r *Root) Close() error {
	if r.root == nil {
		return nil
	}
	return r.root.Close() //nolint:wrapcheck // only read from
}

// DirFS returns the directory dir as an fs.FS whose files are opened as Open opens a path, links followed as
// os.DirFS follows them, out of it too: the name is put after dir by its letters, for the system to resolve.
// Of dir "", as of os.DirFS(""), every name is an error. Nothing is held open, so there is nothing to close.
func DirFS(dir string) fs.FS {
	return dirFS(dir)
}

// dirFS is a DirFS. It has no ReadFile nor ReadDir, so that fs.ReadFile, fs.ReadDir and fs.WalkDir open through
// Open.
type dirFS string

// Open opens name under d as Open opens a path: d and name joined, then looked at by allowed once.
func (d dirFS) Open(name string) (fs.File, error) {
	p, err := d.join("open", name)
	if err != nil {
		return nil, err
	}
	f, _, err := Open(p)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (d dirFS) Stat(name string) (fs.FileInfo, error) {
	p, err := d.join("stat", name)
	if err != nil {
		return nil, err
	}
	return Stat(p)
}

// errNoDir is the error of every name of DirFS(""), as os.DirFS has one.
var errNoDir = errors.New("DirFS of no directory")

// join puts name, as the system's names (Localize), after d as os.DirFS does, by its letters: filepath.Join would
// fold link/.. in d by its letters, where the system follows the link. After \\?\ in d, Windows reads no / as a
// separator, so name's are made the system's.
func (d dirFS) join(op, name string) (string, error) {
	local, err := checkName(op, name)
	switch {
	case err != nil:
		return "", err
	case d == "":
		return "", &fs.PathError{Op: op, Path: name, Err: errNoDir}
	case local == ".":
		// d itself: after \\?\, Windows reads no . as the directory it is in.
		return string(d), nil
	case os.IsPathSeparator(d[len(d)-1]):
		return string(d) + local, nil
	}
	return string(d) + string(os.PathSeparator) + local, nil
}

// openRead opens name of o (an *os.Root), a slash-separated path under it that ValidName takes, to read as Open
// opens a path.
func openRead(o Opener, name string) (fs.File, error) {
	if _, err := checkName("open", name); err != nil {
		return nil, err
	}
	f, _, err := openWith(rootTarget{o: o, n: name}, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// checkName refuses name, of the operation op, unless ValidName takes it, as an fs.FS does; it gives name as the
// system's names.
func checkName(op, name string) (string, error) {
	local, err := filepath.Localize(name)
	if err != nil {
		return "", &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	return local, nil
}

// ValidName reports whether name, slash-separated, is one an fs.FS of floe's (Root, DirFS) reads: fs.ValidPath takes
// it, and the system reads it as names under the directory (filepath.Localize). On Windows that refuses a \, a :
// (a stream, a drive) and a device's name (NUL, CON); everywhere, a NUL byte.
func ValidName(name string) bool {
	_, err := checkName("", name)
	return err == nil
}
