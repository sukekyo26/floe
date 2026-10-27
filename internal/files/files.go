// Package files holds what floe does with the files it reads and writes, apart from what they hold: which
// is a data file, the slug a file's name gives, writing a file whole, and the mark that tells a file changed.
package files

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/sys/unix"
)

var nonSlugRe = regexp.MustCompile(`[^a-z0-9]+`)

// MaxSlugLen is the length a slug may be.
const MaxSlugLen = 64

// IsDataFile reports whether file is a data file (.yaml or .yml) rather than a page.
func IsDataFile(file string) bool {
	ext := strings.ToLower(filepath.Ext(file))
	return ext == ".yaml" || ext == ".yml"
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

// WriteAtomic writes data to path with the mode perm, less the umask, through a file beside it, so that a
// reader sees the old content or the new, never a part. The file beside it is a new one of its own (O_EXCL,
// under a random name), so that one left there, or placed there, cannot keep its mode or lead the write
// elsewhere. Its name is short, whatever the length of path's: a name the directory takes is written.
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	f, err := createNew(filepath.Join(filepath.Dir(path), ".floe-"), perm) // beside it: path is real, resolved
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	tmp := f.Name()
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

// createNew creates a file named prefix and a random suffix, with the mode perm less the umask (which
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

// Hidden reports whether name, of a file in a directory of floe's or one's own, is one floe leaves alone: one
// starting with a dot (.git, an editor's .part.html.swp, .DS_Store).
func Hidden(name string) bool {
	return strings.HasPrefix(name, ".")
}

// Mark is what the file at path is now, to tell when it changes: its kind, inode, size, mtime and ctime, as
// lstat tells them, and as stat tells them for a symlink (what it leads to); "-" for none. ctime and the inode
// tell what a mtime set back (touch -r, cp -p) or a file replaced by a rename would hide.
// LIMIT: a filesystem of times in whole seconds (FAT, some SMB or NFS) hides two writes of one size in one second
// once the first was marked. Add a hash of the content if pages come to be kept on one.
func Mark(path string) string {
	var st unix.Stat_t
	if unix.Lstat(path, &st) != nil {
		return "-"
	}
	m := stamp(&st)
	if st.Mode&unix.S_IFMT == unix.S_IFLNK {
		if unix.Stat(path, &st) != nil {
			return m + ">-"
		}
		m += ">" + stamp(&st)
	}
	return m
}

// stamp is st's part of a mark. A directory is its kind and inode alone: what is in it is marked by the walk,
// and its times change with every file made in it, a hidden one (an editor's .swp) too.
func stamp(st *unix.Stat_t) string {
	if st.Mode&unix.S_IFMT == unix.S_IFDIR {
		return fmt.Sprintf("%o:%d", st.Mode, st.Ino)
	}
	return fmt.Sprintf("%o:%d:%d:%d:%d", st.Mode, st.Ino, st.Size, st.Mtim.Nano(), st.Ctim.Nano())
}
