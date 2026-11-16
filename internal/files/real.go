package files

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"floe/internal/dirs"
)

// errNotAbs is Real's for a path that is not absolute: what the system would read it under is no one's choice.
var errNotAbs = errors.New("not an absolute path")

// Real is the real path of the file or directory at path, which must be absolute: its links followed and named as
// the system names it (on Windows its junctions too, short names made long and case as on disk, as \\?\C:\…), for
// comparing two paths both made by Real. It fails for a path not there. Names are joined to it with filepath.Join
// alone: after \\?\, Windows reads no / nor .. of their letters.
func Real(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", &fs.PathError{Op: "real", Path: path, Err: errNotAbs}
	}
	return realOf(path)
}

// RealAbove is the real path of path, there or not: of a name not there, the nearest directory above it that is,
// Real, with the names after it as written. False when a .. is among those names, after a name not there: Unix
// cannot follow it; Windows folds it by its letters before it opens a path, so one there is Real already. A link
// that leads nowhere is a name not there, and a name not there is matched by its letters, so either is refused once
// it is there.
// LIMIT: the names not there are kept as written: a link to nothing, a short name, another case, or a directory
// that cannot be opened is compared by its letters (macOS, Windows). Follow them if needed.
func RealAbove(path string) (string, bool) {
	if !filepath.IsAbs(path) {
		return "", false
	}
	rest := ""
	for at := path; ; {
		got, err := Real(at)
		if err == nil {
			return filepath.Join(got, rest), true
		}
		// dirs.Dir, as written: filepath.Dir would fold sym/.. by its letters, and filepath.Split stays on a/.
		up := dirs.Dir(at)
		// At the root, or at a volume alone (\\host\share, which Dir gives as \\host\share.): nothing above.
		if len(up) >= len(at) {
			return "", false
		}
		name := at[len(up):]
		for name != "" && os.IsPathSeparator(name[0]) {
			name = name[1:]
		}
		if name == ".." {
			return "", false
		}
		rest = filepath.Join(name, rest)
		at = up
	}
}

// LongName is path with each short name made long (Windows: SECRET~1 is .secret), its links not followed: the
// names as written, for what reads them, as the dot rule of a link does. It fails for a name not there.
func LongName(path string) (string, error) {
	return longNameOf(path)
}
