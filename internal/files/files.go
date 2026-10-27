// Package files holds what floe does with the files it reads and writes, apart from what they hold: which
// is a data file, the slug a file's name gives, and writing a file whole.
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

	"github.com/sukekyo26/floe/internal/usererr"
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

// openFile opens a file Open has looked at; a test puts another file in its place first.
var openFile = os.OpenFile

// Open opens the file path, a link followed, when it is a regular file or a directory, and returns what the system
// says of it; else an *fs.PathError of usererr.ErrNotFile, the file unopened: a FIFO would block the open, and a
// device may do more on it. What it opens without blocking (O_NONBLOCK, which a regular file's reads do not
// take) is looked at again, so that one made such since is refused unread; another regular file put in its place
// (a save by rename) is read. The system's errors are
// *fs.PathError (fs.ErrNotExist for none); one of opening it comes with what the system said of it before.
func Open(path string) (*os.File, fs.FileInfo, error) {
	before, err := os.Stat(path)
	if err != nil {
		return nil, nil, err //nolint:wrapcheck // the system's *fs.PathError, for the caller to tell
	}
	if IsNotFile(before) {
		return nil, before, &fs.PathError{Op: "open", Path: path, Err: usererr.ErrNotFile}
	}
	f, err := openFile(path, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, before, err //nolint:wrapcheck // as above
	}
	info, err := f.Stat()
	if err == nil && IsNotFile(info) {
		err = &fs.PathError{Op: "open", Path: path, Err: usererr.ErrNotFile}
	}
	if err != nil {
		_ = f.Close() //nolint:errcheck // not read
		return nil, before, err
	}
	return f, info, nil
}

// Read reads the file path as Open opens it, and returns what the system says of it too.
func Read(path string) ([]byte, fs.FileInfo, error) {
	f, info, err := Open(path)
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
	f, err := createNew(filepath.Join(filepath.Dir(path), ".floe-"), perm) // beside it: path is real, resolved
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	tmp := f.Name()
	if old != nil {
		// Kept where the file system takes it: one that takes no mode (vfat) is written all the same.
		_ = f.Chmod(old.Mode().Perm()) //nolint:errcheck // see above
	}
	_, err = f.Write(data)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		_ = os.Remove(tmp) //nolint:errcheck // the write failed, which is what is told
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// createNew creates a new file named prefix and a random suffix, with the mode perm less the umask (which
// os.CreateTemp does not take), trying another name while one is taken.
func createNew(prefix string, perm os.FileMode) (*os.File, error) {
	for {
		name := prefix + rand.Text()
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm) //nolint:gosec // G304: beside the file written
		if !errors.Is(err, fs.ErrExist) {
			return f, err //nolint:wrapcheck // WriteAtomic says what it wrote
		}
	}
}
