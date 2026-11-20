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
// Real, with the names after it as written. A link whose target is not there is followed to it: a directory made
// there later is read through the link. False when a .. is among those names, after a name not there: Unix
// cannot follow it; Windows folds it by its letters before it opens a path, so one there is Real already.
// LIMIT: the names not there are kept as written: a short name, another case, or a directory that cannot be
// opened is compared by its letters (macOS, Windows); so, on Unix, is a link that cannot be followed for another
// reason (a loop, a target that cannot be looked at), which Windows gives false for. Follow them if needed.
// LIMIT: past maxHops links give false, compared with nothing: on Unix the links to nothing followed here, on
// Windows every link walked, as allowed counts them. So do a link to nothing whose target has .. after a name not
// there (Unix) and a link to a drive not there (Windows, which is not opened). Keep their letters if that comes to
// matter.
// LIMIT: a link to nothing is compared where it leads alone, not where it is: one in another place (FLOE_PARTS_DIR
// holding the FLOE_TEMPLATES_DIR it is) overlaps none until it is made, as a link that leads somewhere does; that
// place's read refuses it then (a link to a directory). Compare it at both if that comes to matter.
// LIMIT: on Windows a symlink made to nothing is a file's, which a directory made there later is not read through;
// it is compared where it leads all the same, refusing more than needed. Look at its kind if that comes to matter.
func RealAbove(path string) (string, bool) {
	if !filepath.IsAbs(path) {
		return "", false
	}
	if checksPaths {
		return realAboveWalked(path)
	}
	rest, hops := "", 0
	for at := path; ; {
		got, err := Real(at)
		if err == nil {
			return filepath.Join(got, rest), true
		}
		if to, ok := linkToNothing(at, err); ok {
			if hops++; hops > maxHops {
				return "", false
			}
			at, rest = joinWritten(to, rest), ""
			continue
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

// realAboveWalked is RealAbove where allowed walks the path (Windows): the part there, its links followed (to
// nothing too), Real, with the names not there as written. A target on a drive not there, which is not opened, and
// whatever else allowed refuses (a share, a device, a loop, a name it cannot look at) is false.
func realAboveWalked(path string) (string, bool) {
	w, err := walk(path, true)
	if err != nil || w.noDrive {
		return "", false
	}
	got, err := realOfChecked(w.at, path)
	if err != nil {
		return "", false
	}
	return filepath.Join(append([]string{got}, w.rest...)...), true
}

// linkToNothing is where the link at leads, as written, when Real failed for it with err as not there: a relative
// target under at's directory as written, its .. left for Real, which follows the links before it. False when at is
// no link, or Real failed for another reason (a loop, a target that cannot be looked at).
func linkToNothing(at string, err error) (string, bool) {
	if !errors.Is(err, fs.ErrNotExist) {
		return "", false
	}
	p, err := allowed(at, false)
	if err != nil {
		return "", false
	}
	to, err := readlinkAt(p)
	if err != nil {
		return "", false
	}
	if !filepath.IsAbs(to) {
		to = dirs.Join(dirs.Dir(at), to)
	}
	return to, true
}

// joinWritten is rest after to, as written: filepath.Join would fold lnk/.. by its letters.
func joinWritten(to, rest string) string {
	if rest == "" {
		return to
	}
	return dirs.Join(to, rest)
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
