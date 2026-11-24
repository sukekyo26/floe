package files

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// The system's calls on a path that the rest of floe makes through files alone (forbidigo refuses the os ones out of
// it), so that the path is looked at in one place before the system is called (allowed): each refuses what allowed
// refuses, and gives the system the path allowed checked. Errors name the path as given.

// Stat tells of the file at path, its links followed, as os.Stat does.
func Stat(path string) (fs.FileInfo, error) {
	p, err := allowed(path, true)
	if err != nil {
		return nil, told(err, "stat", path, false)
	}
	fi, err := statAt(p)
	return fi, asWritten(err, path)
}

// Lstat tells of the file at path itself, a link not followed, as os.Lstat does.
func Lstat(path string) (fs.FileInfo, error) {
	p, err := allowed(path, false)
	if err != nil {
		return nil, told(err, "lstat", path, false)
	}
	fi, err := lstatAt(p)
	return fi, asWritten(err, path)
}

// ReadDir lists the directory at path, sorted by name, as os.ReadDir does.
func ReadDir(path string) ([]fs.DirEntry, error) {
	p, err := allowed(path, true)
	if err != nil {
		return nil, told(err, "open", path, false)
	}
	entries, err := readDirAt(p)
	return entries, asWritten(err, path)
}

// OpenRoot opens the directory at path as an *os.Root, as os.OpenRoot does.
func OpenRoot(path string) (*os.Root, error) {
	p, err := allowed(path, true)
	if err != nil {
		return nil, told(err, "open", path, false)
	}
	root, err := openRootAt(p)
	return root, asWritten(err, path)
}

// MkdirAll makes the directory at path and those above it that are not there, as os.MkdirAll does; not where a link
// to nothing leads (ELOOP).
func MkdirAll(path string, perm fs.FileMode) error {
	p, err := allowed(path, true)
	if err != nil {
		return told(err, "mkdir", path, true)
	}
	return asWritten(mkdirAllAt(p, perm), path)
}

// Mkdir makes the directory at path, as os.Mkdir does; not where a link to nothing on the way leads (ELOOP).
func Mkdir(path string, perm fs.FileMode) error {
	p, err := allowed(path, false)
	if err != nil {
		return told(err, "mkdir", path, true)
	}
	return asWritten(mkdirAt(p, perm), path)
}

// OpenLock opens the lock file at path, made with the permissions 0600 (less the umask) when there is none, for Lock:
// read only, for nothing is written to it; not where a link to nothing leads (ELOOP).
func OpenLock(path string) (*os.File, error) {
	p, err := allowed(path, true)
	if err != nil {
		return nil, told(err, "open", path, true)
	}
	f, err := openAt(p, os.O_RDONLY|os.O_CREATE, 0o600)
	return f, asWritten(err, path)
}

// Remove removes the file or empty directory at path, as os.Remove does: a link itself, not where it leads.
func Remove(path string) error {
	p, err := allowed(path, false)
	if err != nil {
		return told(err, "remove", path, false)
	}
	return asWritten(removeAt(p), path)
}

// RemoveAll removes path and what it holds, as os.RemoveAll does: a link itself, not where it leads, and nil when
// there is nothing.
func RemoveAll(path string) error {
	p, err := allowed(path, false)
	if errors.Is(err, errDangling) || errors.Is(err, fs.ErrNotExist) {
		// Nothing there to remove (a link to nothing, or no drive, on the way), as os.RemoveAll says of none.
		return nil
	}
	if err != nil {
		return told(err, "unlinkat", path, false)
	}
	return asWritten(removeAllAt(p), path)
}

// Rename renames from to to, two names in the directory dir (each a name alone, no separator), replacing what is at
// to as os.Rename does: a link itself is moved, not where it leads. A file another holds open sharing delete (floe's
// reads) is replaced, on Windows too (renameIn).
// LIMIT: on Windows a directory with a file open in it, however opened, cannot be renamed (Access is denied): told as
// the system's error and not tried again, as what holds it is anyone's. Rename no directory that is read while
// it runs if that comes to matter.
func Rename(dir, from, to string) error {
	at := func(err error) error {
		var pe *fs.PathError
		var le *os.LinkError
		switch {
		case errors.As(err, &le):
			err = le.Err
		case errors.As(err, &pe):
			err = pe.Err
		}
		return &os.LinkError{Op: "rename", Old: filepath.Join(dir, from), New: filepath.Join(dir, to), Err: err}
	}
	if !oneName(from) || !oneName(to) {
		return at(fs.ErrInvalid)
	}
	d, err := allowed(dir, true)
	if err != nil {
		return at(told(err, "rename", dir, false))
	}
	if err := renameIn(d, from, to); err != nil {
		return at(err)
	}
	return nil
}

// oneName reports whether name is one name of a directory's: not empty, no separator, no . nor .., local.
func oneName(name string) bool {
	return filepath.IsLocal(name) && name != "." && !strings.ContainsAny(name, `/\`)
}

// MkdirTemp makes a new directory in dir, named pattern with a random string put in place of its last *, as
// os.MkdirTemp does; not where a link to nothing leads (ELOOP). It returns dir as given joined with the name made;
// dir "" (os.TempDir to os.MkdirTemp) is fs.ErrInvalid.
func MkdirTemp(dir, pattern string) (string, error) {
	if dir == "" {
		// os.MkdirTemp would make it in os.TempDir, whose name the one returned would not hold.
		return "", &fs.PathError{Op: "mkdirtemp", Path: dir, Err: fs.ErrInvalid}
	}
	d, err := allowed(dir, true)
	if err != nil {
		return "", told(err, "mkdirtemp", dir, true)
	}
	made, err := mkdirTempAt(d, pattern)
	if err != nil {
		return "", asWritten(err, dir)
	}
	return filepath.Join(dir, filepath.Base(made)), nil
}

// The wrappers below are files' one way to the system's calls on a path: each takes a checked alone.

func statAt(p checked) (fs.FileInfo, error)  { return os.Stat(p.s) }  //nolint:wrapcheck // told by the caller
func lstatAt(p checked) (fs.FileInfo, error) { return os.Lstat(p.s) } //nolint:wrapcheck // as above

func readlinkAt(p checked) (string, error) { return os.Readlink(p.s) } //nolint:wrapcheck // as above

func readDirAt(p checked) ([]fs.DirEntry, error) { return os.ReadDir(p.s) } //nolint:wrapcheck // as above

func openRootAt(p checked) (*os.Root, error) { return os.OpenRoot(p.s) } //nolint:wrapcheck // as above

func mkdirAllAt(p checked, perm fs.FileMode) error { return os.MkdirAll(p.s, perm) } //nolint:wrapcheck // as above
func mkdirAt(p checked, perm fs.FileMode) error    { return os.Mkdir(p.s, perm) }    //nolint:wrapcheck // as above

func openAt(p checked, flag int, perm fs.FileMode) (*os.File, error) {
	return os.OpenFile(p.s, flag, perm) //nolint:wrapcheck,gosec // as above; G304: a path allowed looked at
}

func removeAt(p checked) error    { return os.Remove(p.s) }    //nolint:wrapcheck // as above
func removeAllAt(p checked) error { return os.RemoveAll(p.s) } //nolint:wrapcheck // as above

func mkdirTempAt(dir checked, pattern string) (string, error) {
	return os.MkdirTemp(dir.s, pattern) //nolint:wrapcheck // as above
}
