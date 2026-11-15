package files

import (
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

// Stat tells of name under the directory without opening it, so that a FIFO does not block.
func (r *Root) Stat(name string) (fs.FileInfo, error) {
	if err := r.ready(); err != nil {
		return nil, err
	}
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
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
// dir is not "": its callers leave out a directory not set. Nothing is held open, so there is nothing to close.
func DirFS(dir string) fs.FS {
	return dirFS(dir)
}

// dirFS is a DirFS. It has no ReadFile nor ReadDir, so that fs.ReadFile, fs.ReadDir and fs.WalkDir open through
// Open.
type dirFS string

func (d dirFS) Open(name string) (fs.File, error) {
	return openRead(d, name)
}

func (d dirFS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}
	return os.Stat(d.join(name)) //nolint:wrapcheck // the system's
}

func (d dirFS) OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error) {
	return os.OpenFile(d.join(name), flag, perm) //nolint:wrapcheck,gosec // the system's; G304: under the caller's dir
}

// join puts name after d as os.DirFS does, by its letters: filepath.Join would fold link/.. in d by its letters,
// where the system follows the link.
func (d dirFS) join(name string) string {
	if os.IsPathSeparator(d[len(d)-1]) {
		return string(d) + name
	}
	return string(d) + string(os.PathSeparator) + name
}

// openRead opens name of o, a slash-separated path under it that fs.ValidPath takes, to read as Open opens a path.
func openRead(o Opener, name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	f, _, err := openWith(o, name, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	return f, nil
}
