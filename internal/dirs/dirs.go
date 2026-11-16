// Package dirs tells where a path the user gave is: a relative one is under the working directory floe was
// given, never the process's; and how a path is shown to people.
package dirs

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrNoCwd is Under's for a relative path with no working directory to put it under.
	ErrNoCwd = errors.New("a relative path with no working directory")
	// ErrDriveRelative is Under's for a path relative to a drive's own working directory (C:foo, Windows), which is
	// the process's, not the one floe was given.
	ErrDriveRelative = errors.New("a path relative to a drive's working directory")
)

// Under returns path under cwd when it is relative, and path itself when it is absolute. "" stays "". The path is
// joined as written, not cleaned: a .. after a symlink leads where the system says, from where the link leads,
// which folding it by its letters would not. One from the root of the drive (\foo, Windows) is put on cwd's
// volume. Every place that resolves a relative path resolves it here, never under the process's working
// directory.
func Under(cwd, path string) (string, error) {
	switch {
	case path == "" || filepath.IsAbs(path):
		return path, nil
	case filepath.VolumeName(path) != "":
		return "", ErrDriveRelative
	case cwd == "":
		return "", ErrNoCwd
	case os.IsPathSeparator(path[0]):
		return filepath.VolumeName(cwd) + path, nil
	}
	return Join(cwd, path), nil
}

// Dir returns the directory path names its last name in, as written: what is before its last separator ("." for
// none), not cleaned, as Under joins; the root of its volume (/, C:\, \\host\share\) for a name right under it.
func Dir(path string) string {
	vol := filepath.VolumeName(path)
	i := len(path) - 1
	for i >= len(vol) && !os.IsPathSeparator(path[i]) {
		i--
	}
	switch {
	case i < len(vol) && vol == "":
		return "."
	case i < len(vol):
		return vol + "."
	case i == len(vol):
		return path[:i+1]
	}
	return path[:i]
}

// Join returns name in the directory dir as written, not cleaned, with one separator between them; name alone in
// ".".
func Join(dir, name string) string {
	switch {
	case dir == ".":
		return name
	case dir != "" && os.IsPathSeparator(dir[len(dir)-1]):
		return dir + name
	}
	return dir + string(filepath.Separator) + name
}

// Shown is path in the form people write: a path as files.Real gives it on Windows (\\?\C:\…, \\?\UNC\host\share\…)
// without its \\?\, as os.Readlink shows a link's target (os/file_windows.go normaliseLinkPath); any other form,
// \\?\Volume{…}\ among them, as it is. Shown paths are for people: floe never opens one again.
// LIMIT: a name the system reads otherwise without \\?\ (one ending in a dot or a space, NUL, CON) is shown so;
// given back to floe, it names another file.
func Shown(path string) string { return shownWith(path, verbatim) }

// shownWith is Shown with the prefix of verbatim paths given, so that its cases are tried on every system.
func shownWith(path, prefix string) string {
	rest, ok := strings.CutPrefix(path, prefix)
	switch {
	case prefix == "" || !ok:
		return path
	case len(rest) >= 3 && rest[1] == ':' && rest[2] == '\\':
		return rest
	case strings.HasPrefix(rest, `UNC\`):
		return `\\` + rest[len(`UNC\`):]
	}
	return path
}
