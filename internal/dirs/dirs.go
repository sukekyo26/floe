// Package dirs tells where a path the user gave is: a relative one is under the working directory floe was
// given, never the process's.
package dirs

import (
	"path/filepath"
	"strings"
)

// Under returns path under cwd when it is relative, and path itself when it is absolute; ok is false for a
// relative path with no cwd ("") to put it under. "" stays "". The path is joined as written, not cleaned: a ..
// after a symlink leads where the system says, from where the link leads, which folding it by its letters
// would not. Every place that resolves a relative path resolves it here, never under the process's working
// directory.
func Under(cwd, path string) (abs string, ok bool) {
	switch {
	case path == "" || filepath.IsAbs(path):
		return path, true
	case cwd == "":
		return "", false
	}
	return strings.TrimSuffix(cwd, string(filepath.Separator)) + string(filepath.Separator) + path, true
}

// Dir returns the directory path names its last name in, as written: what is before its last separator ("."
// for none), not cleaned, as Under joins.
func Dir(path string) string {
	i := strings.LastIndexByte(path, filepath.Separator)
	switch {
	case i < 0:
		return "."
	case i == 0:
		return string(filepath.Separator)
	}
	return path[:i]
}

// Join returns name in the directory dir as written, not cleaned; name alone in ".".
func Join(dir, name string) string {
	if dir == "." {
		return name
	}
	return strings.TrimSuffix(dir, string(filepath.Separator)) + string(filepath.Separator) + name
}
