package files

import (
	"io/fs"
	"os"
)

// The system's calls on a path that the rest of floe makes through files alone (forbidigo refuses the os ones out of
// it), so that what is looked at before the system is called is looked at in one place. Each is the os one as it is.

// Stat tells of the file at path, its links followed, as os.Stat does.
func Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path) //nolint:wrapcheck // the system's, for the caller to tell
}

// Lstat tells of the file at path itself, a link not followed, as os.Lstat does.
func Lstat(path string) (fs.FileInfo, error) {
	return os.Lstat(path) //nolint:wrapcheck // as above
}

// ReadDir lists the directory at path, sorted by name, as os.ReadDir does.
func ReadDir(path string) ([]fs.DirEntry, error) {
	return os.ReadDir(path) //nolint:wrapcheck // as above
}

// OpenRoot opens the directory at path as an *os.Root, as os.OpenRoot does.
func OpenRoot(path string) (*os.Root, error) {
	return os.OpenRoot(path) //nolint:wrapcheck // as above
}

// MkdirAll makes the directory at path and those above it that are not there, as os.MkdirAll does.
func MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm) //nolint:wrapcheck // as above
}

// OpenLock opens the lock file at path, made with the permissions 0600 (less the umask) when there is none, for Lock:
// read only, for nothing is written to it.
func OpenLock(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0o600) //nolint:wrapcheck,gosec // as above; G304: the caller's lock
}
