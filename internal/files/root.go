package files

import (
	"errors"
	"io/fs"
	"os"
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
		if r.root, err = os.OpenRoot(r.dir); err != nil {
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

// ReadFile reads the file name under the directory, as ReadIn reads one.
func (r *Root) ReadFile(name string) ([]byte, error) {
	if err := r.ready(); err != nil {
		return nil, err
	}
	return readFileIn(r.root, name)
}

// Stat tells of name under the directory without opening it: of a FIFO too, which Open refuses.
func (r *Root) Stat(name string) (fs.FileInfo, error) {
	if err := r.ready(); err != nil {
		return nil, err
	}
	if err := checkName("stat", name); err != nil {
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

// dirFS is a DirFS. It has no ReadDir, so that fs.ReadDir and fs.WalkDir open through Open; ReadFile reads as
// ReadIn does, so that a directory read as a file is refused.
type dirFS string

func (d dirFS) Open(name string) (fs.File, error) {
	return openRead(d, name)
}

// ReadFile reads the file name, as ReadIn reads one.
func (d dirFS) ReadFile(name string) ([]byte, error) {
	return readFileIn(d, name)
}

func (d dirFS) Stat(name string) (fs.FileInfo, error) {
	p, err := d.join("stat", name)
	if err != nil {
		return nil, err
	}
	return os.Stat(p) //nolint:wrapcheck // the system's
}

func (d dirFS) OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error) {
	p, err := d.join("open", name)
	if err != nil {
		return nil, err
	}
	return os.OpenFile(p, flag, perm) //nolint:wrapcheck,gosec // the system's; G304: under the caller's dir
}

// errNoDir is the error of every name of DirFS(""), as os.DirFS has one.
var errNoDir = errors.New("DirFS of no directory")

// join puts name after d as os.DirFS does, by its letters: filepath.Join would fold link/.. in d by its letters,
// where the system follows the link.
func (d dirFS) join(op, name string) (string, error) {
	switch err := checkName(op, name); {
	case err != nil:
		return "", err
	case d == "":
		return "", &fs.PathError{Op: op, Path: name, Err: errNoDir}
	case os.IsPathSeparator(d[len(d)-1]):
		return string(d) + name, nil
	}
	return string(d) + string(os.PathSeparator) + name, nil
}

// openRead opens name of o, a slash-separated path under it that fs.ValidPath takes, to read as Open opens a path.
func openRead(o Opener, name string) (fs.File, error) {
	if err := checkName("open", name); err != nil {
		return nil, err
	}
	f, _, err := openWith(o, name, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// readFileIn reads name of o, a slash-separated path under it that fs.ValidPath takes, as ReadIn reads a file.
func readFileIn(o Opener, name string) ([]byte, error) {
	if err := checkName("open", name); err != nil {
		return nil, err
	}
	b, _, err := ReadIn(o, name)
	return b, err
}

// checkName refuses name, of the operation op, unless fs.ValidPath takes it, as an fs.FS does.
func checkName(op, name string) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}
	return nil
}
