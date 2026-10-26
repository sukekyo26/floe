// Package dirs tells where a path the user gave is: a relative one is under the working directory floe was
// given, never the process's.
package dirs

import "path/filepath"

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
