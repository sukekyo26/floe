package kit

import (
	"encoding/base64"
	"html/template"
	"io"
	"io/fs"
	"path"
	"strconv"
	"strings"

	"github.com/sukekyo26/floe/internal/dirs"
	"github.com/sukekyo26/floe/internal/usererr"
)

// maxImage is the size an image may be.
// LIMIT: 1 MiB an image, and no bound on how many a page holds, as every image goes into the page. Raise the
// one, or bound the other, if pages come to need it.
const maxImage = 1 << 20

// imageTypes are the images floe takes, by the extension of their name: the type of their data URI.
var imageTypes = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp",
	".svg": "image/svg+xml",
}

// ReadImage reads the image name of files, the directory dir as its mistakes name it, that of the file of, as a
// data URI. A name floe does not read (told relative to the directory of of), an extension of no image floe
// takes, a file that is not a regular one, and one over maxImage are mistakes of the name, without a place; one
// that cannot be read is told at its path under dir, in the system's words (usererr.CannotRead), which says what
// is wrong of the file.
// LIMIT: a file is taken by its name's type, what it holds unchecked: one that holds another shows broken.
// LIMIT: a file made a FIFO between the Stat and the Open blocks the read; open it without blocking if it comes to.
func ReadImage(files fs.FS, name, dir, of string) (template.URL, error) {
	// os.Root's FS says only "invalid argument" of such a name.
	if !fs.ValidPath(name) || name == "." {
		return "", usererr.New("kit_image_path", name, of)
	}
	typ, ok := imageTypes[strings.ToLower(path.Ext(name))]
	if !ok {
		return "", usererr.New("kit_image_type", name)
	}
	at := dirs.Join(dir, name)
	if fi, err := fs.Stat(files, name); err != nil {
		return "", usererr.CannotRead(at, err) //nolint:wrapcheck // the system's words, at the file
	} else if !fi.Mode().IsRegular() {
		// A FIFO would block the read.
		return "", usererr.New("kit_image_not_file", name)
	}
	f, err := files.Open(name)
	if err != nil {
		return "", usererr.CannotRead(at, err) //nolint:wrapcheck // the system's words, at the file
	}
	defer f.Close()
	fi, err := f.Stat()
	switch {
	case err != nil:
		return "", usererr.CannotRead(at, err) //nolint:wrapcheck // the system's words, at the file
	case !fi.Mode().IsRegular():
		return "", usererr.New("kit_image_not_file", name)
	case fi.Size() > maxImage:
		return "", tooLarge(name, fi.Size())
	}
	b, err := io.ReadAll(io.LimitReader(f, maxImage+1))
	switch {
	case err != nil:
		return "", usererr.CannotRead(at, err) //nolint:wrapcheck // the system's words, at the file
	case len(b) > maxImage:
		return "", tooLarge(name, int64(len(b)))
	}
	//nolint:gosec // G203: an image of a type floe takes, as a data URI
	return template.URL("data:" + typ + ";base64," + base64.StdEncoding.EncodeToString(b)), nil
}

// tooLarge refuses the image name of size bytes, saying by how much: one just over the limit would be 1 MiB as
// bytes writes it.
func tooLarge(name string, size int64) *usererr.Error {
	grouped := func(n int64) string {
		s, err := decimal("bytes", n, 0)
		if err != nil {
			return strconv.FormatInt(n, 10)
		}
		return s
	}
	limit, err := formatBytes(int64(maxImage))
	if err != nil {
		limit = strconv.Itoa(maxImage) + " B"
	}
	return usererr.New("kit_image_large", name, grouped(size), grouped(maxImage), limit)
}
