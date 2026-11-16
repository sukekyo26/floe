// Package files holds what floe does with the files it reads and writes, apart from what they hold: which
// is a data file, the slug a file's name gives, writing a file whole, and the mark that tells a file changed.
package files

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"floe/internal/usererr"
)

var nonSlugRe = regexp.MustCompile(`[^a-z0-9]+`)

// MaxSlugLen is the length a slug may be.
const MaxSlugLen = 64

// IsDataFile reports whether file is a data file (.yaml or .yml) rather than a page.
func IsDataFile(file string) bool {
	ext := strings.ToLower(filepath.Ext(file))
	return ext == ".yaml" || ext == ".yml"
}

// IsPage reports whether file is named as a page (.html).
func IsPage(file string) bool {
	return strings.ToLower(filepath.Ext(file)) == ".html"
}

// IsNotFile reports whether info is of what is neither a regular file nor a directory (a FIFO, a device, a
// socket): what floe does not open.
func IsNotFile(info fs.FileInfo) bool {
	return !info.Mode().IsRegular() && !info.IsDir()
}

// Opener is a directory a file is looked at and opened under: an *os.Root, which keeps them under it (ReadIn).
type Opener interface {
	Stat(name string) (fs.FileInfo, error)
	OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error)
}

// target is one file to look at and open, once: a path allowed checked, or a name under an Opener.
type target interface {
	stat() (fs.FileInfo, error)
	open(flag int, perm fs.FileMode) (*os.File, error)
	name() string
}

// pathTarget is a path allowed checked, told as path, as given.
type pathTarget struct {
	p    checked
	path string
}

func (t pathTarget) stat() (fs.FileInfo, error) {
	fi, err := statAt(t.p)
	return fi, asWritten(err, t.path)
}

// open opens t with flag: to read alone (openFlags aside) sharing delete (openShared), else as os.OpenFile does.
func (t pathTarget) open(flag int, perm fs.FileMode) (*os.File, error) {
	var f *os.File
	var err error
	if flag&^openFlags == os.O_RDONLY {
		f, err = openShared(t.p)
	} else {
		f, err = openAt(t.p, flag, perm)
	}
	return f, asWritten(err, t.path)
}

func (t pathTarget) name() string { return t.path }

// rootTarget is the name n under o, which looks at it (os.Root keeps it under its directory).
type rootTarget struct {
	o Opener
	n string
}

func (t rootTarget) stat() (fs.FileInfo, error) { return t.o.Stat(t.n) } //nolint:wrapcheck // the system's, at n

func (t rootTarget) open(flag int, perm fs.FileMode) (*os.File, error) {
	return t.o.OpenFile(t.n, flag, perm) //nolint:wrapcheck // as above
}

func (t rootTarget) name() string { return t.n }

// Open opens the file path, a link followed, to read it when it is a regular file, and returns what the system
// says of it; else an *fs.PathError of usererr.ErrNotFile, the file unopened: a FIFO would block the open, and a
// device may do more on it. What it opens without blocking (O_NONBLOCK, which a regular file's reads do not
// take) is looked at again, so that one made such since is refused unread; another regular file put in its place
// (a save by rename) is read. A directory is refused once opened, with EISDIR (refuseDir). The system's errors are
// *fs.PathError (fs.ErrNotExist for none); one of opening it comes with what the system said of it before. What
// allowed refuses is not opened.
func Open(path string) (*os.File, fs.FileInfo, error) {
	return refuseDir(path)(openPath(path, os.O_RDONLY, 0))
}

// refuseDir refuses what was opened to read as name when it is a directory, which Windows opens and then reads as
// "Incorrect function.", with what Unix says reading one (EISDIR). The directories fs.ReadDir and fs.WalkDir open
// do not come here: Root's go through openRead, DirFS's through openPath (dirFS.Open).
func refuseDir(name string) func(*os.File, fs.FileInfo, error) (*os.File, fs.FileInfo, error) {
	return func(f *os.File, info fs.FileInfo, err error) (*os.File, fs.FileInfo, error) {
		if err == nil && info.IsDir() {
			_ = f.Close() //nolint:errcheck // not read
			return nil, info, &fs.PathError{Op: "read", Path: name, Err: syscall.EISDIR}
		}
		return f, info, err
	}
}

// OpenAppend opens the file path to append to, a link to a file followed (one to nowhere is not, so that the file is
// not made where it leads), made with the permissions 0600 (less the umask) when there is none, as Open does a file to
// read: a FIFO, a device or a socket is refused unopened (an
// *fs.PathError of usererr.ErrNotFile), which would block the open, and one made so since is refused after. A
// directory is the system's to refuse (EISDIR). The info is of the file opened.
func OpenAppend(path string) (*os.File, fs.FileInfo, error) {
	return openPath(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
}

// openPath opens path with flag as Open and OpenAppend say, once allowed has looked at it.
func openPath(path string, flag int, perm fs.FileMode) (*os.File, fs.FileInfo, error) {
	p, err := allowed(path, true)
	if err != nil {
		return nil, nil, told(err, "open", path, flag&os.O_CREATE != 0)
	}
	return openWith(pathTarget{p: p, path: path}, flag, perm)
}

// openWith opens t with flag as Open and OpenAppend say; a file that is not there is made when flag has O_CREATE.
func openWith(t target, flag int, perm fs.FileMode) (*os.File, fs.FileInfo, error) {
	before, err := t.stat()
	creating := flag&os.O_CREATE != 0 && errors.Is(err, fs.ErrNotExist)
	if err != nil && !creating {
		return nil, nil, err
	}
	if err == nil && IsNotFile(before) {
		return nil, before, &fs.PathError{Op: "open", Path: t.name(), Err: usererr.ErrNotFile}
	}
	if creating {
		// A link to nowhere is not followed to make the file where it leads, out of its directory (ELOOP); a link to a
		// file is followed, as it is to read.
		flag |= noFollow
	}
	f, err := t.open(flag|openFlags, perm)
	if err != nil {
		return nil, before, err
	}
	info, err := f.Stat()
	switch {
	case err != nil:
	case creating && info.Mode()&(fs.ModeSymlink|fs.ModeIrregular) != 0:
		// Windows opens a link to nowhere itself (noFollow), where Unix refuses it: told alike. allowed tells such
		// a link before; this is of one made so after it looked.
		err = &fs.PathError{Op: "open", Path: t.name(), Err: syscall.ELOOP}
	case IsNotFile(info):
		err = &fs.PathError{Op: "open", Path: t.name(), Err: usererr.ErrNotFile}
	}
	if err != nil {
		_ = f.Close() //nolint:errcheck // not read
		return nil, before, err
	}
	return f, info, nil
}

// Read reads the file path as Open opens it, and returns what the system says of it too.
func Read(path string) ([]byte, fs.FileInfo, error) {
	return readOpened(Open(path))
}

// ReadIn reads the file name of o as Read reads a path: under an *os.Root, the system refuses one that leads out
// of it, by .. or by a link.
func ReadIn(o Opener, name string) ([]byte, fs.FileInfo, error) {
	return readOpened(refuseDir(name)(openWith(rootTarget{o: o, n: name}, os.O_RDONLY, 0)))
}

// readOpened reads f, opened with info, or tells err of opening it.
func readOpened(f *os.File, info fs.FileInfo, err error) ([]byte, fs.FileInfo, error) {
	if err != nil {
		return nil, info, err
	}
	defer f.Close() //nolint:errcheck // read only
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, info, err //nolint:wrapcheck // the *fs.PathError of the read
	}
	return b, info, nil
}

// SlugFromFile derives a slug from the name of file, without its extension.
func SlugFromFile(file string) string {
	name := filepath.Base(file)
	return Slug(strings.TrimSuffix(name, filepath.Ext(name)))
}

// Slug derives a slug from name as it is: in lower case, each run of what a slug does not take a hyphen.
func Slug(name string) string {
	slug := nonSlugRe.ReplaceAllString(strings.ToLower(name), "-")
	return strings.Trim(slug[:min(len(slug), MaxSlugLen)], "-")
}

// WriteAtomic writes data to path through a file beside it, so that a reader sees the old content or the new,
// never a part: a new file with the mode perm, less the umask; one in place of old, the file there, with old's
// permissions (not its setuid, setgid and sticky bits, nor its owner). The file beside it is a new one of its
// own (O_EXCL, under a random name), so that one left there, or placed there, cannot keep its mode or lead the
// write elsewhere. Its name is short, whatever the length of path's: a name the directory takes is written.
func WriteAtomic(path string, data []byte, perm os.FileMode, old fs.FileInfo) error {
	p, err := allowed(path, false)
	if err != nil {
		return fmt.Errorf("write %s: %w", path, told(err, "open", path, false))
	}
	f, tmp, err := createNew(p.dir(), perm) // beside it: path is real, resolved
	if err != nil {
		return fmt.Errorf("write %s: %w", path, asWritten(err, path))
	}
	if old != nil {
		// Kept where the file system takes it: one that takes no mode (vfat) is written all the same.
		_ = f.Chmod(old.Mode().Perm()) //nolint:errcheck // see above
	}
	_, err = f.Write(data)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = renameIn(p.dir(), filepath.Base(tmp.s), filepath.Base(p.s))
	}
	if err != nil {
		_ = removeAt(tmp) //nolint:errcheck // the write failed, which is what is told
		return fmt.Errorf("write %s: %w", path, asWritten(err, path))
	}
	return nil
}

// createNew creates a new file in dir under a name of .floe- and a random suffix, with the mode perm less the umask
// (which os.CreateTemp does not take), trying another name while one is taken.
func createNew(dir checked, perm os.FileMode) (*os.File, checked, error) {
	for {
		name := dir.join(".floe-" + rand.Text())
		f, err := openAt(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
		if !errors.Is(err, fs.ErrExist) {
			return f, name, err
		}
	}
}

// Hidden reports whether name, of a file in a directory of floe's or one's own, is one floe leaves alone: one
// starting with a dot (.git, an editor's .part.html.swp, .DS_Store).
func Hidden(name string) bool {
	return strings.HasPrefix(name, ".")
}
