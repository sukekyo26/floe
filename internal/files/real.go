package files

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"floe/internal/dirs"
	"floe/internal/usererr"
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
	p, err := allowed(path, true)
	if err != nil {
		return "", told(err, "real", path, false)
	}
	return realOfChecked(p, path)
}

// realOfChecked is the real path of p, checked of path, which allowed looked at: one the system names off this
// machine's drives (\\?\UNC\…, which a reparse point allowed reads no target of may lead to) is ErrNotLocal.
func realOfChecked(p checked, path string) (string, error) {
	got, err := realOf(p)
	switch {
	case err != nil:
		return "", asWritten(err, path)
	case !dirs.OnDrive(got):
		return "", &fs.PathError{Op: "real", Path: path, Err: &usererr.NotLocal{At: got}}
	}
	return got, nil
}

// RealAbove is the real path of path, there or not: of a name not there, the nearest directory above it that is,
// Real, with the names after it as written. False when a .. is among those names, after a name not there: Unix
// cannot follow it; Windows folds it by its letters before it opens a path, so one there is Real already. A link
// that leads nowhere is a name not there, and a name not there is matched by its letters, so either is refused once
// it is there.
// LIMIT: the names not there are kept as written: a link to nothing, a short name, another case, or a directory
// that cannot be opened is compared by its letters (macOS, Windows). Follow them if needed.
func RealAbove(path string) (string, bool) {
	probe("realabove", path)
	if !filepath.IsAbs(path) {
		return "", false
	}
	if checksPaths {
		return realAboveWalked(path)
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

// realAboveWalked is RealAbove where allowed walks the path (Windows): the part there, its links followed, Real,
// with the names not there as written; for a link to nothing, the part before the path's own link being followed
// and the names from it on. Whatever else allowed refuses (a share, a device, a loop, a name it cannot look at)
// is false.
func realAboveWalked(path string) (string, bool) {
	probe("walk-realabove", path)
	w, err := walk(path, true)
	if err != nil {
		return "", false
	}
	got, err := realOfChecked(w.at, path)
	if err != nil {
		return "", false
	}
	return filepath.Join(append([]string{got}, w.rest...)...), true
}

// LongName is path with each short name made long (Windows: SECRET~1 is .secret), its links not followed: the
// names as written, for what reads them, as the dot rule of a link does. It fails for a name not there.
func LongName(path string) (string, error) {
	p, err := allowedWritten(path)
	if err != nil {
		return "", told(err, "GetLongPathName", path, false)
	}
	got, err := longNameOf(p)
	return got, asWritten(err, path)
}
