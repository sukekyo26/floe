// Package files holds what floe does with the files it reads and writes, apart from what they hold: which
// is a data file, the slug a file's name gives, and writing a file whole.
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

	"github.com/sukekyo26/floe/internal/dirs"
)

var (
	indexRe    = regexp.MustCompile(`(?i)^index\.html?$`)
	nonSlugRe  = regexp.MustCompile(`[^a-z0-9]+`)
	maxSlugLen = 64
)

// IsDataFile reports whether file is a data file (.yaml or .yml) rather than a page.
func IsDataFile(file string) bool {
	ext := strings.ToLower(filepath.Ext(file))
	return ext == ".yaml" || ext == ".yml"
}

// SlugFromFile derives a slug from a file name. index.html does not name anything, so it takes the name of
// its directory instead; a relative file is resolved against cwd, and with no cwd ("") named by its path as
// it is written.
func SlugFromFile(file, cwd string) string {
	name := filepath.Base(file)
	if indexRe.MatchString(name) {
		if abs, ok := dirs.Under(cwd, file); ok {
			file = abs
		}
		name = filepath.Base(filepath.Dir(filepath.Clean(file)))
	} else {
		name = strings.TrimSuffix(name, filepath.Ext(name))
	}
	slug := nonSlugRe.ReplaceAllString(strings.ToLower(name), "-")
	return strings.Trim(slug[:min(len(slug), maxSlugLen)], "-")
}

// WriteAtomic writes data to path with the mode perm, less the umask, through a file beside it, so that a
// reader sees the old content or the new, never a part. The file beside it is a new one of its own (O_EXCL,
// under a random name), so that one left there, or placed there, cannot keep its mode or lead the write
// elsewhere.
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	f, err := createNew(path+".tmp-", perm)
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
