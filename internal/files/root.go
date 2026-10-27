package files

import (
	"io/fs"
	"os"
	"sync"
)

// Root is the directory dir as an fs.FS that the system keeps its reads under (os.Root), opened when first used,
// so that a page that reads nothing of it draws when it cannot be opened. Its maker closes it.
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

func (r *Root) fs() (fs.FS, error) {
	r.once.Do(func() {
		var err error
		if r.root, err = os.OpenRoot(r.dir); err != nil {
			r.err = &DirError{Dir: r.dir, Err: err}
		}
	})
	if r.err != nil {
		return nil, r.err
	}
	return r.root.FS(), nil
}

// Open opens name under the directory.
func (r *Root) Open(name string) (fs.File, error) {
	f, err := r.fs()
	if err != nil {
		return nil, err
	}
	return f.Open(name) //nolint:wrapcheck // the system's error, at name
}

// Stat tells of name under the directory without opening it, so that a FIFO does not block.
func (r *Root) Stat(name string) (fs.FileInfo, error) {
	f, err := r.fs()
	if err != nil {
		return nil, err
	}
	return fs.Stat(f, name) //nolint:wrapcheck // as Open
}

// ReadDir lists the directory name under the directory.
func (r *Root) ReadDir(name string) ([]fs.DirEntry, error) {
	f, err := r.fs()
	if err != nil {
		return nil, err
	}
	return fs.ReadDir(f, name) //nolint:wrapcheck // as Open
}

// Close lets go of the directory, if it was opened.
func (r *Root) Close() error {
	if r.root == nil {
		return nil
	}
	return r.root.Close() //nolint:wrapcheck // only read from
}
