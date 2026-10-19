package tmpl

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// dirAt returns the directory at, a symlink followed, as its real path, or "" for a file. A symlink that
// leads nowhere, or one that cannot be read, is its error.
func dirAt(at string) (string, error) {
	dir, err := filepath.EvalSymlinks(at)
	if err != nil {
		return "", err //nolint:wrapcheck // the callers say what it is
	}
	fi, err := os.Stat(dir)
	if err != nil {
		return "", err //nolint:wrapcheck // as above
	}
	if !fi.IsDir() {
		return "", nil
	}
	return dir, nil
}

// entriesOf returns what the directory dir holds: none without one, or with a file in its way.
func entriesOf(dir string) ([]fs.DirEntry, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return nil, nil
	}
	return entries, err //nolint:wrapcheck // the callers say what it is
}

// readIn calls read with the files of dir, as a file system rooted at it: a path, a symlink included,
// that leads out of it is refused.
func readIn(dir string, read func(fs.FS) error) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("open %s: %w", dir, err)
	}
	defer root.Close()
	return read(root.FS())
}
