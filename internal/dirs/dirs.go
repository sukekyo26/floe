// Package dirs tells what floe needs to know of a path the user gave: where a relative one is, and of a
// directory the user set, whether floe can write there.
package dirs

import (
	"os"
	"path/filepath"
)

// Under returns path under cwd when it is relative, cleaned as filepath.Join cleans it, and path cleaned when
// it is absolute; ok is false for a relative path with no cwd ("") to put it under. "" stays "". Every place
// that resolves a relative path resolves it here, never under the process's working directory.
func Under(cwd, path string) (abs string, ok bool) {
	switch {
	case path == "":
		return "", true
	case filepath.IsAbs(path):
		return filepath.Clean(path), true
	case cwd == "":
		return "", false
	}
	return filepath.Join(cwd, path), true
}

// CheckWritable makes dir, and a directory in it named after prefix, which it removes at once: a directory there
// already that floe cannot write in passes MkdirAll, so making one in it is what tells.
func CheckWritable(dir, prefix string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err //nolint:wrapcheck // the caller tells where it meant to write
	}
	tmp, err := os.MkdirTemp(dir, prefix)
	if err != nil {
		return err //nolint:wrapcheck // as above
	}
	_ = os.Remove(tmp) //nolint:errcheck // written, so it can be; a leftover is harmless
	return nil
}
