package tmpl

import (
	"bytes"
	"encoding/base64"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/usererr"
)

// KeptName is the name a published page keeps its data file by, beside the images it shows: an image
// may not be under a directory of that name, in any case, as macOS ignores it.
const KeptName = "data.yaml"

// maxImage is the size an image may be.
// LIMIT: 1 MiB an image, and no bound on how many a page holds, as every image goes into the page.
// Raise the one, or bound the other, if pages come to need it.
const maxImage = 1 << 20

// imageTypes are the images floe takes, by the extension of their name.
var imageTypes = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp",
	".svg": "image/svg+xml",
}

// imageDir is the directory a data file's images are read under: open gives it, and what lets it go,
// when the data names an image, and its mistakes name it dir.
type imageDir struct {
	dir  string
	open func() (fs.FS, func(), error)
}

// diskImages is the directory dir on disk.
func diskImages(dir string) imageDir {
	return imageDir{dir: dir, open: func() (fs.FS, func(), error) {
		r, err := os.OpenRoot(dir)
		if err != nil {
			return nil, nil, usererr.CannotRead(dir, err)
		}
		return r.FS(), func() { _ = r.Close() }, nil //nolint:errcheck // only read from
	}}
}

// fsImages is the directory dir as files holds it, which needs no opening.
func fsImages(files fs.FS, dir string) imageDir {
	return imageDir{dir: dir, open: func() (fs.FS, func(), error) { return files, func() {}, nil }}
}

// ReadImage reads the image name under files as a data file's check reads it, which where names in its
// mistakes: see dataImageName and imageOf.
func ReadImage(files fs.FS, name, where string) ([]byte, error) {
	typ, e := dataImageName(name, where)
	if e != nil {
		return nil, e
	}
	b, e := imageOf(files, name, where, typ)
	if e != nil {
		return nil, e
	}
	return b, nil
}

// dataImageName checks the name a data file gives an image, as imageName does, but not under KeptName,
// in any case, where a stored page keeps its data file.
func dataImageName(name, where string) (string, *usererr.Error) {
	// A name that is no path is told so first, under KeptName or not.
	if first, _, under := strings.Cut(name, "/"); imagePath(name) && under && strings.EqualFold(first, KeptName) {
		return "", usererr.New("tmpl_image_kept_name", name, KeptName)
	}
	return imageName(name, where)
}

// imageURI reads the image name as a data URI, which html/template takes for a src as it is.
func imageURI(files fs.FS, name, where string) (template.URL, *usererr.Error) {
	typ, e := imageName(name, where)
	if e != nil {
		return "", e
	}
	b, e := imageOf(files, name, where, typ)
	if e != nil {
		return "", e
	}
	return dataURI(b, typ), nil
}

// dataURI is the image b, of the type typ, as a data URI.
func dataURI(b []byte, typ string) template.URL {
	//nolint:gosec // G203: an image of a type floe takes, checked by what it holds
	return template.URL("data:" + typ + ";base64," + base64.StdEncoding.EncodeToString(b))
}

// imageName checks name, which names an image, whatever file is there, and returns its type: a
// slash-separated path under the directory where names, without .. or a name starting with a dot, of
// an image floe takes by its name.
func imageName(name, where string) (string, *usererr.Error) {
	if !imagePath(name) {
		return "", usererr.New("tmpl_image_path", name, where)
	}
	typ, ok := imageTypes[strings.ToLower(path.Ext(name))]
	if !ok {
		return "", usererr.New("tmpl_image_type", name)
	}
	return typ, nil
}

// imageOf reads the image name under files, of the type typ its name gives, holding what that says.
func imageOf(files fs.FS, name, where, typ string) ([]byte, *usererr.Error) {
	b, e := readImageFile(files, name, where)
	if e != nil {
		return nil, e
	}
	if holds := http.DetectContentType(b); holds != typ && (typ != "image/svg+xml" || !isSVG(b)) {
		return nil, usererr.New("tmpl_image_content", name, typ)
	}
	return b, nil
}

// imagePath reports whether name may name an image: a slash-separated path under its directory,
// without .. or a name starting with a dot.
func imagePath(name string) bool {
	return fs.ValidPath(name) && name != "." && !strings.HasPrefix(name, ".") && !strings.Contains(name, "/.")
}

// readImageFile reads the file name of files, a regular file up to maxImage. Each name on its path must be
// in its directory as written, case and all, so that a page drawn on macOS, which ignores case, draws
// on linux too; and none may be a symlink, which could lead out of files or to a name starting with a
// dot.
func readImageFile(files fs.FS, name, where string) ([]byte, *usererr.Error) {
	steps := strings.Split(name, "/")
	for i := range steps {
		entries, err := fs.ReadDir(files, path.Join(append([]string{"."}, steps[:i]...)...))
		if err != nil {
			return nil, usererr.New("tmpl_image_unreadable", name, err.Error())
		}
		k := slices.IndexFunc(entries, func(e fs.DirEntry) bool { return e.Name() == steps[i] })
		last := i == len(steps)-1
		switch {
		case k < 0:
			return nil, usererr.New("tmpl_image_missing", name, where)
		case entries[k].Type()&fs.ModeSymlink != 0:
			return nil, usererr.New("tmpl_image_symlink", name, where)
		case !last && !entries[k].IsDir():
			return nil, usererr.New("tmpl_image_missing", name, where)
		case last && !entries[k].Type().IsRegular():
			return nil, usererr.New("tmpl_image_not_file", name)
		}
	}
	f, err := files.Open(name)
	if err != nil {
		return nil, usererr.New("tmpl_image_unreadable", name, err.Error())
	}
	defer f.Close()
	// The file opened is told by itself: its name may lead elsewhere by now.
	fi, err := f.Stat()
	switch {
	case err != nil:
		return nil, usererr.New("tmpl_image_unreadable", name, err.Error())
	case !fi.Mode().IsRegular():
		return nil, usererr.New("tmpl_image_not_file", name)
	case fi.Size() > maxImage:
		return nil, tooLarge(name, fi.Size())
	}
	b, err := io.ReadAll(io.LimitReader(f, maxImage+1))
	switch {
	case err != nil:
		return nil, usererr.New("tmpl_image_unreadable", name, err.Error())
	case len(b) > maxImage:
		return nil, usererr.New("tmpl_image_changed", name)
	}
	return b, nil
}

// tooLarge refuses the image name of size bytes, saying by how much: one just over the limit would be
// 1 MiB as bytes writes it.
func tooLarge(name string, size int64) *usererr.Error {
	return usererr.New("tmpl_image_large", name, english.Sprintf("%d", size), english.Sprintf("%d", maxImage),
		byteSize(maxImage))
}

// byteSize writes n bytes as bytes in page.html does.
func byteSize(n int64) string {
	s, err := formatBytes(n)
	if err != nil { // only for a value that is not a number
		return strconv.FormatInt(n, 10) + " B"
	}
	return s
}

// isSVG reports whether b is an SVG document: <svg after blanks, an XML declaration, comments and a
// DOCTYPE. http.DetectContentType tells SVG only as text or XML.
// LIMIT: a DOCTYPE with an internal subset holding > is not skipped whole. Parse it if one comes to.
func isSVG(b []byte) bool {
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	for {
		b = bytes.TrimLeft(b, " \t\r\n")
		end := ""
		switch {
		case bytes.HasPrefix(b, []byte("<?")):
			end = "?>"
		case bytes.HasPrefix(b, []byte("<!--")):
			end = "-->"
		case bytes.HasPrefix(b, []byte("<!")):
			end = ">"
		default:
			rest, ok := bytes.CutPrefix(b, []byte("<svg"))
			return ok && len(rest) > 0 && strings.ContainsRune(" \t\r\n/>", rune(rest[0]))
		}
		i := bytes.Index(b, []byte(end))
		if i < 0 {
			return false
		}
		b = b[i+len(end):]
	}
}

// asset returns the file name under the assets/ of t as a data URI, for page.html to show.
func (t *Template) asset(name string) (template.URL, error) {
	var uri template.URL
	var e *usererr.Error
	err := t.files(func(files fs.FS) error {
		sub, err := fs.Sub(files, "assets")
		if err != nil {
			return err //nolint:wrapcheck // wrapped below
		}
		uri, e = imageURI(sub, name, t.file("assets"))
		return nil
	})
	if err != nil {
		return "", usererr.CannotRead(t.file("assets"), err)
	}
	if e != nil {
		return "", t.assetMistake(e)
	}
	return uri, nil
}

// assetMistake places the mistake e of the image of assets/ page.html asks for: a name that finds no
// image floe takes is a mistake of page.html, and a file that is not right one of assets/.
func (t *Template) assetMistake(e *usererr.Error) *usererr.Error {
	where := "assets"
	switch e.Key {
	case "tmpl_image_path", "tmpl_image_type", "tmpl_image_missing":
		where = "page.html"
	}
	return e.Placed(t.file(where), 0, 0, "")
}

// assets returns asset, each image read once for t, its mistake too: every data file drawn with t tells
// the same one.
func (t *Template) assets() func(name string) (template.URL, error) {
	return func(name string) (template.URL, error) {
		return onceIn(t.reads.assets, name).of(func() (template.URL, error) { return t.asset(name) })
	}
}

// checkImages reads the images of d, its items of whole of format image, under images, the directory of
// the data file, into d.images, and reports each mistake at its line and column under root.
func checkImages(whole drawnItems, d *Data, root ast.Node, images imageDir) error {
	found := whole.ofFormat("image")
	if found == nil {
		return nil
	}
	files, release, err := images.open()
	if err != nil {
		return err
	}
	defer release()
	d.images = map[string]template.URL{}
	refused := map[string]*usererr.Error{}
	var problems usererr.List
	for _, f := range found {
		// Not text where the schema gives no type: drawn as written.
		name, ok := f.value.(string)
		if !ok {
			continue
		}
		// An image named by many items is read once, and its mistake reported at each of them: refused
		// holds nil for one read.
		e, seen := refused[name]
		if !seen {
			// Only a name that a file can make right is tried, and watched.
			var typ string
			if typ, e = dataImageName(name, images.dir); e == nil {
				d.tried = append(d.tried, name)
				var b []byte
				if b, e = imageOf(files, name, images.dir, typ); e == nil {
					d.images[name] = dataURI(b, typ)
				}
			}
			refused[name] = e
		}
		if e == nil {
			continue
		}
		problems = append(problems, itemMistake(d.File, root, f.at, false, e))
	}
	return problems.Err()
}
