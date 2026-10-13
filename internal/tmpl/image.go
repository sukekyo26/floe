package tmpl

import (
	"bytes"
	"encoding/base64"
	"errors"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// maxImage is the size an image may be.
// LIMIT: 1 MiB an image, and no bound on how many a page holds, as every image goes into the page.
// Raise the one, or bound the other, if pages come to need it.
const maxImage = 1 << 20

// imageTypes are the images floe takes, by the extension of their name.
var imageTypes = map[string]string{
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp",
	".svg": "image/svg+xml",
}

// imageURI reads the image name, a slash-separated path under files, as a data URI, which html/template
// takes for a src as it is. where names files in the mistake: name must be a path under it without
// .. or a name starting with a dot, of an image floe takes, by its name and by what it holds.
func imageURI(files fs.FS, name, where string) (template.URL, *usererr.Error) {
	if !fs.ValidPath(name) || name == "." || strings.HasPrefix(name, ".") || strings.Contains(name, "/.") {
		return "", usererr.New("tmpl_image_path", name, where)
	}
	typ, ok := imageTypes[strings.ToLower(path.Ext(name))]
	if !ok {
		return "", usererr.New("tmpl_image_type", name)
	}
	b, e := readImage(files, name, where)
	if e != nil {
		return "", e
	}
	if holds := http.DetectContentType(b); holds != typ && (typ != "image/svg+xml" || !isSVG(b)) {
		return "", usererr.New("tmpl_image_content", name, typ)
	}
	//nolint:gosec // G203: an image of a type floe takes, checked by what it holds
	return template.URL("data:" + typ + ";base64," + base64.StdEncoding.EncodeToString(b)), nil
}

// readImage reads the file name of files, a regular file up to maxImage. os.Root refuses a symlink
// that leads out of files.
func readImage(files fs.FS, name, where string) ([]byte, *usererr.Error) {
	fi, err := fs.Stat(files, name)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, usererr.New("tmpl_image_missing", name, where)
	case err != nil:
		return nil, usererr.New("tmpl_image_unreadable", name, err.Error())
	case !fi.Mode().IsRegular():
		return nil, usererr.New("tmpl_image_not_file", name)
	case fi.Size() > maxImage:
		return nil, tooLarge(name, fi.Size())
	}
	b, err := readAtMost(files, name, maxImage+1)
	switch {
	case err != nil:
		return nil, usererr.New("tmpl_image_unreadable", name, err.Error())
	case len(b) > maxImage: // grown since it was looked at
		return nil, tooLarge(name, int64(len(b)))
	}
	return b, nil
}

// readAtMost reads up to n bytes of the file name of files.
func readAtMost(files fs.FS, name string, n int64) ([]byte, error) {
	f, err := files.Open(name)
	if err != nil {
		return nil, err //nolint:wrapcheck // worded by the caller
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, n)) //nolint:wrapcheck // worded by the caller
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
		return "", err
	}
	if e != nil {
		return "", e
	}
	return uri, nil
}

// checkImages reads the images of d, its items its schema s declares of format image, under dir, the
// directory of the data file, into d.images, and reports each mistake at its line and column under
// root.
func checkImages(s *schema, d *Data, root ast.Node, dir string) error {
	var found []imageAt
	s.findImages(d.Values, nil, &found)
	if found == nil {
		return nil
	}
	r, err := os.OpenRoot(dir)
	if err != nil {
		return usererr.CannotRead(dir, err)
	}
	defer r.Close()
	d.images = map[string]template.URL{}
	refused := map[string]*usererr.Error{}
	var problems yamlpos.Problems
	for _, f := range found {
		// An image named by many items is read once, and its mistake reported at each of them.
		_, read := d.images[f.name]
		e, seen := refused[f.name]
		if !read && !seen {
			var uri template.URL
			if uri, e = imageURI(r.FS(), f.name, dir); e == nil {
				d.images[f.name] = uri
			}
			refused[f.name] = e
		}
		if e == nil {
			continue
		}
		key, value, at := yamlpos.Find(root, f.at)
		problems = append(problems, &yamlpos.Error{
			File: d.File, Pos: yamlpos.OfValue(key, value), Err: usererr.New("tmpl_item", at, e),
		})
	}
	if problems != nil {
		return problems.Sorted()
	}
	return nil
}

// imageAt is an image a data file names, and the path of keys to where it does.
type imageAt struct {
	name string
	at   []string
}

// findImages adds to found the images v holds where s declares format image, v being at the path at.
func (s *schema) findImages(v any, at []string, found *[]imageAt) {
	if s == nil {
		return
	}
	switch v := v.(type) {
	case string:
		if s.Format == "image" {
			*found = append(*found, imageAt{name: v, at: at})
		}
	case map[string]any:
		for k, item := range v {
			s.Properties[k].findImages(item, append(at[:len(at):len(at)], k), found)
		}
	case []any:
		for i, item := range v {
			s.Items.findImages(item, append(at[:len(at):len(at)], strconv.Itoa(i)), found)
		}
	}
}
