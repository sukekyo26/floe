package files

import (
	"cmp"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"floe/internal/dirs"
	"floe/internal/usererr"
)

// checked is a path allowed looked at, as the system is to be given it: on Windows \\?\X:\…, its links followed
// (but the last name's, unless asked), on Unix the path as given. The system is given a checked alone: what is
// looked at is what is opened. Only allowed, allowedWritten and walk (allowed.go) make one, and its dir and join.
type checked struct{ s string }

// dir is p's directory, checked as p is.
func (p checked) dir() checked { return checked{s: filepath.Dir(p.s)} }

// join is name, one name files makes (a temporary file's), in p.
func (p checked) join(name string) checked { return checked{s: filepath.Join(p.s, name)} }

// maxHops is how many links allowed follows on one path: as many as Windows follows (a 64th fails).
const maxHops = 63

// The kinds of drive GetDriveType tells that allowed refuses: a network drive (a share mapped, or substed), and
// one the system cannot tell; and one not there, which allowed tells is not there without opening it (a network
// drive remembered but not connected may answer so, and opening it would connect it again).
const (
	driveUnknown = 0
	driveNoRoot  = 1
	driveRemote  = 4
)

// errDangling is allowed's for a link whose target is not there: each entry tells it as the system would of a
// link to nothing (making: ELOOP; else not there), never as it is.
var errDangling = errors.New("a link to nothing")

// allowed returns path as the system is to be given it, after looking, before the system is called, that it is a
// file on a local drive under an ordinary name (Windows): its volume a drive letter, which is no network drive,
// each name an ordinary one (no device's, no stream), and each link on the way (the last name's too when last)
// to such a path, followed by allowed itself, the path given with it followed. A share's path would reach
// another machine, sending the user's credentials, a pipe's would connect to its server: by a stat already. On
// Unix, which has neither by name, it returns path as it is: a FIFO or a device is told by its kind.
// LIMIT: a name made a link after it is looked at, before the system opens it, is followed by the system. Open
// each name in turn (as os.Root does on Windows) if that comes to matter.
// LIMIT: a reparse point that is no link (a cloud file, an AppExecLink, an LX symlink) is passed as a name: Go's
// Readlink tells none of where it leads. Real refuses what such a one leads off the drives to; the other entries
// do not look.
// LIMIT: a drive substed to a directory that holds a link, and a drive letter DefineDosDevice made, are taken as
// the user's: allowed walks from the drive letter on.
// LIMIT: each call Lstats every name from the root (a read 12 deep: 0.25ms, was 0.03ms), so the watch of one's own
// directories (Mark) and each Read pay it; the store's list reads under one os.Root instead. Do the same where a
// read of many files turns slow.
func allowed(path string, last bool) (checked, error) {
	if !checksPaths {
		return checked{s: path}, nil
	}
	w, err := walk(path, last)
	switch {
	case err != nil:
		return checked{}, err
	case w.dangling:
		return checked{}, errDangling
	}
	return w.full(), nil
}

// allowedWritten is path as allowed takes it before it follows its links (\\?\X:\ and the names as written), once
// allowed has looked at all of it: for LongName, which makes the names as written long and follows no link.
// LIMIT: the system follows path's links here, where allowed looked: both read a link's target by its letters.
func allowedWritten(path string) (checked, error) {
	if !checksPaths {
		return checked{s: path}, nil
	}
	w, err := walk(path, false)
	switch {
	case err != nil:
		return checked{}, err
	case w.dangling:
		return checked{}, errDangling
	}
	return checked{s: w.written}, nil
}

// walked is what walk found of a path: the part that is there, its links followed, and the names after it as
// written, not there; dangling when the first of those came of a link's target, noDrive when that target's drive
// is not there (at is then its root, which is not to be opened).
type walked struct {
	at       checked
	rest     []string
	dangling bool
	noDrive  bool
	written  string
}

// full is w's path, its names not there put after the part that is.
func (w walked) full() checked {
	return checked{s: filepath.Join(append([]string{w.at.s}, w.rest...)...)}
}

// step is a name still to walk, and whether it came of a link's target rather than of the path.
type step struct {
	name string
	link bool
}

// walker follows a path name by name from its drive's root.
type walker struct {
	path    string   // as given, for errors
	root    string   // the drive's root walked on (\\?\X:\)
	at      []string // the names walked under root, links followed
	pending []step   // the names still to walk
	hops    int
	ended   bool // the walk stopped at a link's target not there
	noDrive bool // and that target's drive is not there
	written string
}

// here is the path walked so far.
func (w *walker) here() string { return filepath.Join(append([]string{w.root}, w.at...)...) }

// walk follows path name by name from its drive's root, as allowed says, and tells what it found.
func walk(path string, last bool) (walked, error) {
	if path == "" {
		// As it is: the system's error of no path, not the working directory filepath.Abs would make of it.
		return walked{at: checked{s: ""}, rest: nil, dangling: false, noDrive: false, written: ""}, nil
	}
	root, names, last, err := shape(path, last)
	if err != nil {
		return walked{}, err
	}
	if missing, err := localDrive(path, root); missing || err != nil {
		if missing {
			err = &fs.PathError{Op: "open", Path: path, Err: fs.ErrNotExist}
		}
		return walked{}, err
	}
	w := &walker{
		path: path, root: root, at: nil, pending: steps(names, false), hops: 0, ended: false, noDrive: false,
		written: filepath.Join(append([]string{root}, names...)...),
	}
	for len(w.pending) > 0 {
		if len(w.pending) == 1 && !last {
			w.at = append(w.at, w.pending[0].name)
			break
		}
		if done, err := w.next(); done || err != nil {
			return w.result(), err
		}
	}
	return walked{at: checked{s: w.here()}, rest: nil, dangling: false, noDrive: false, written: w.written}, nil
}

// result is what w found when it stopped at a name not there: where it stood, its links followed, and the names
// after it as written, dangling when that name came of a link's target.
func (w *walker) result() walked {
	return walked{
		at: checked{s: w.here()}, rest: namesOf(w.pending), dangling: w.ended, noDrive: w.noDrive, written: w.written,
	}
}

// next walks the next name: into it, or to where it leads when it is a link. done when it is not there.
func (w *walker) next() (done bool, err error) {
	n := w.pending[0]
	at := filepath.Join(w.here(), n.name)
	fi, err := os.Lstat(at)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		w.ended = n.link
		return true, nil
	case err != nil:
		return false, errAt(w.path, "lstat", err)
	case fi.Mode()&(fs.ModeSymlink|fs.ModeIrregular) == 0:
		w.at, w.pending = append(w.at, n.name), w.pending[1:]
		return false, nil
	}
	to, err := os.Readlink(at)
	switch {
	case err != nil && fi.Mode()&fs.ModeIrregular != 0 && errors.Is(err, syscall.ENOENT):
		// A reparse point Go reads no target of: no link (a cloud file, an AppExecLink), a name as it is.
		w.at, w.pending = append(w.at, n.name), w.pending[1:]
		return false, nil
	case err != nil:
		return false, errAt(w.path, "readlink", err)
	}
	return w.follow(to)
}

// follow puts the target to of the link next to walk in its place: its names, marked, before the names after the
// link. done when the target's drive is not there.
func (w *walker) follow(to string) (done bool, err error) {
	if w.hops++; w.hops > maxHops {
		return false, &fs.PathError{Op: "open", Path: w.path, Err: syscall.ELOOP}
	}
	root, names, err := linkTarget(to)
	switch {
	case err != nil:
		return false, &fs.PathError{Op: "open", Path: w.path, Err: err}
	case root != "":
		missing, refused := localDrive(w.path, root)
		if refused != nil {
			return false, refused
		}
		w.ended, w.noDrive = missing, missing
		w.root, w.at = root, nil
	default:
		if names, w.at, err = fold(names, w.at, to); err != nil {
			return false, &fs.PathError{Op: "open", Path: w.path, Err: err}
		}
	}
	w.pending = append(steps(names, true), w.pending[1:]...)
	return w.ended, nil
}

// shape is path made the form the system takes as written: its drive's root (\\?\X:\) and its names, each an
// ordinary one, and whether its last name is to be followed (last, or path ending in a separator, which os.Lstat
// follows). A path \\?\X:\… is taken as it is, whose names must hold no . nor .. (nothing reads them there); any
// other is made absolute once, as Win32 reads it (filepath.Abs, GetFullPathName: a last name's trailing dots
// and spaces dropped, .. folded, NUL made \\.\NUL). A volume of no drive letter is refused.
func shape(path string, last bool) (root string, names []string, follow bool, err error) {
	if strings.HasPrefix(path, `\\?\`) {
		root, names, follow, err = verbatim(path)
	} else {
		root, names, follow, err = absolute(path)
	}
	if err != nil {
		return "", nil, false, err
	}
	for _, n := range names {
		if !filepath.IsLocal(n) {
			return "", nil, false, notLocal(path, n)
		}
	}
	return root, names, last || follow, nil
}

// verbatim is the root and names of path, \\?\X:\…, by its letters, and whether it ends in a separator.
func verbatim(path string) (root string, names []string, follow bool, err error) {
	if !dirs.OnDrive(path) {
		return "", nil, false, notLocal(path, filepath.VolumeName(path))
	}
	rest := path[len(`\\?\C:\`):]
	if strings.HasSuffix(rest, `\`) {
		rest, follow = rest[:len(rest)-1], true
	}
	names = split(rest)
	if !plain(names) {
		return "", nil, false, &fs.PathError{Op: "open", Path: path, Err: fs.ErrInvalid}
	}
	return path[:len(`\\?\C:\`)], names, follow, nil
}

// absolute is the root and names of path made absolute as Win32 reads it, and whether it ends in a separator.
func absolute(path string) (root string, names []string, follow bool, err error) {
	if vol := filepath.VolumeName(path); vol != "" && (len(vol) != 2 || vol[1] != ':') {
		return "", nil, false, notLocal(path, vol)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", nil, false, err //nolint:wrapcheck // the system's, at path
	}
	if !dirs.OnDrive(`\\?\` + abs) {
		return "", nil, false, notLocal(path, cmp.Or(filepath.VolumeName(abs), abs))
	}
	return `\\?\` + abs[:len(`C:\`)], split(abs[len(`C:\`):]), os.IsPathSeparator(path[len(path)-1]), nil
}

// linkTarget is a link's target as Readlink tells it, by its letters as the system reads it: an absolute one is its
// drive's root (\\?\X:\) and its names, a relative one its names alone (root ""), to fold onto the link's
// directory. A root-relative target (\x, \Device\Mup\…), one of no drive letter, or a name that is no ordinary one
// or that the system does not read in it (. and .. in an absolute target, an empty one, a /) is refused.
func linkTarget(to string) (string, []string, error) {
	vol := filepath.VolumeName(to)
	switch {
	case strings.HasPrefix(to, `\\?\`) || len(vol) == 2 && vol[1] == ':':
		return absoluteTarget(to)
	case vol != "" || to == "" || os.IsPathSeparator(to[0]):
		return "", nil, &usererr.NotLocal{At: to}
	}
	names := strings.Split(to, `\`)
	for _, n := range names {
		if n == "" || strings.Contains(n, "/") {
			return "", nil, &usererr.NotLocal{At: to}
		}
	}
	return "", names, ordinary(names)
}

// absoluteTarget is the root and names of an absolute target to (X:\…, \\?\X:\…), its one trailing separator (a
// junction's) dropped.
func absoluteTarget(to string) (string, []string, error) {
	v := to
	if !strings.HasPrefix(v, `\\?\`) {
		v = `\\?\` + v
	}
	if !dirs.OnDrive(v) {
		return "", nil, &usererr.NotLocal{At: to}
	}
	names := split(strings.TrimSuffix(v[len(`\\?\C:\`):], `\`))
	if !plain(names) {
		return "", nil, &usererr.NotLocal{At: to}
	}
	return v[:len(`\\?\C:\`)], names, ordinary(names)
}

// plain reports whether names hold none the system does not read after \\?\: an empty one, . or .., or a /.
func plain(names []string) bool {
	for _, n := range names {
		if n == "" || n == "." || n == ".." || strings.Contains(n, "/") {
			return false
		}
	}
	return true
}

// fold puts the relative target names after at, the link's directory under its root, by their letters, as the
// system reads a relative target: . is nothing, .. the directory above; above the root is refused.
func fold(names, at []string, to string) (kept, under []string, err error) {
	for _, n := range names {
		switch {
		case n == ".":
		case n != "..":
			kept = append(kept, n)
		case len(kept) > 0:
			kept = kept[:len(kept)-1]
		case len(at) > 0:
			at = at[:len(at)-1]
		default:
			return nil, nil, &usererr.NotLocal{At: to}
		}
	}
	return kept, at, nil
}

// ordinary refuses the first of names that is no ordinary one (a device's, a stream's): . and .. are folded after.
func ordinary(names []string) error {
	for _, n := range names {
		if n != "." && n != ".." && !filepath.IsLocal(n) {
			return &usererr.NotLocal{At: n}
		}
	}
	return nil
}

// localDrive refuses the drive of root (\\?\X:\) when it is a network drive or one the system cannot tell;
// missing when it is not there.
func localDrive(path, root string) (missing bool, err error) {
	drive := root[len(`\\?\`) : len(`\\?\`)+2]
	kind := driveType(drive + `\`)
	if refusedDrive(kind) {
		return false, notLocal(path, drive)
	}
	return kind == driveNoRoot, nil
}

// refusedDrive reports whether a drive of the kind GetDriveType tells is refused.
func refusedDrive(kind uint32) bool { return kind == driveUnknown || kind == driveRemote }

// notLocal is ErrNotLocal at where, of path.
func notLocal(path, where string) error {
	return &fs.PathError{Op: "open", Path: path, Err: &usererr.NotLocal{At: where}}
}

// errAt is the system's err of op, at path as written: the path walked is not the caller's.
func errAt(path, op string, err error) error {
	if pe := (*fs.PathError)(nil); errors.As(err, &pe) {
		err = pe.Err
	}
	return &fs.PathError{Op: op, Path: path, Err: err}
}

// told is err of allowed as op on path says it: a link to nothing as the system tells one, ELOOP when making
// (nothing is made where it leads), else not there.
func told(err error, op, path string, making bool) error {
	if !errors.Is(err, errDangling) {
		return err
	}
	if making {
		return &fs.PathError{Op: op, Path: path, Err: syscall.ELOOP}
	}
	return &fs.PathError{Op: op, Path: path, Err: fs.ErrNotExist}
}

// asWritten is the system's err of a checked path, at path as written; nil for none.
func asWritten(err error, path string) error {
	var pe *fs.PathError
	var le *os.LinkError
	switch {
	case errors.As(err, &pe):
		return &fs.PathError{Op: pe.Op, Path: path, Err: pe.Err}
	case errors.As(err, &le):
		return &fs.PathError{Op: le.Op, Path: path, Err: le.Err}
	}
	return err
}

// split is the names of rest, separated by \, none for "".
func split(rest string) []string {
	if rest == "" {
		return nil
	}
	return strings.Split(rest, `\`)
}

// steps are names to walk, of a link's target when link.
func steps(names []string, link bool) []step {
	s := make([]step, len(names))
	for i, n := range names {
		s[i] = step{name: n, link: link}
	}
	return s
}

// namesOf is the names of s.
func namesOf(s []step) []string {
	n := make([]string, len(s))
	for i, x := range s {
		n[i] = x.name
	}
	return n
}
