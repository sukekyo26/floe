package tmpl

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
)

// An image is taken only under its directory, by a name of its type, holding what the name says and
// no larger than maxImage.
func TestImageURI(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	png, err := os.ReadFile("testdata/sizes/chart.png")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"a.png":         png,
		"sub/b.PNG":     png,
		"dir.png/c.png": png,
		"png.gif":       png,
		"logo.svg":      []byte("\xef\xbb\xbf<!DOCTYPE svg>\n<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 1 1\"/>"),
		"text.svg":      []byte("<html><svg/></html>"),
		"xlink.svg": []byte(`<?xml version="1.0" encoding="ISO-8859-1"?><!DOCTYPE svg [<!ENTITY w "1">]>` +
			`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" xml:space="preserve">` +
			`<rect id="a"/><use xlink:href="#a"/></svg>`),
		"bare.svg":      []byte(`<svg viewBox="0 0 1 1"/>`),
		"two.svg":       []byte(`<svg xmlns="http://www.w3.org/2000/svg"/><svg xmlns="http://www.w3.org/2000/svg"/>`),
		"empty.svg":     []byte("<!-- c -->\n"),
		"open.svg":      []byte("<svg xmlns=\"http://www.w3.org/2000/svg\">\n<rect>"),
		"entity.svg":    []byte(`<!DOCTYPE svg [<!ENTITY w "1">]><svg xmlns="http://www.w3.org/2000/svg" width="&w;"/>`),
		"png.svg":       png,
		"unknown.svg":   []byte(`<?xml version="1.0" encoding="x-unknown"?><svg xmlns="http://www.w3.org/2000/svg"/>`),
		"notes.txt":     []byte("text"),
		"large.png":     append(bytes.Clone(png), bytes.Repeat([]byte{0}, maxImage)...),
		".hidden/c.png": png,
	}
	for name, b := range files {
		if err = os.MkdirAll(filepath.Join(dir, filepath.Dir(name)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	outside := filepath.Join(t.TempDir(), "out.png")
	if err = os.WriteFile(outside, png, 0o600); err != nil { //nolint:gosec // G703: the test's own directory
		t.Fatal(err)
	}
	// A symlink is refused wherever it leads: out of the directory, through a directory, to a name that
	// starts with a dot, or inside by an absolute path.
	for link, to := range map[string]string{
		"link.png": outside, "out": filepath.Dir(outside), "pic.png": ".hidden/c.png",
		"abs.png": filepath.Join(dir, "a.png"),
	} {
		if err = os.Symlink(to, filepath.Join(dir, link)); err != nil {
			t.Fatal(err)
		}
	}
	r, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, tt := range []struct{ name, want string }{
		{"a.png", "data:image/png;base64,iVBOR"},
		{"sub/b.PNG", "data:image/png;base64,"},
		{"logo.svg", "data:image/svg+xml;base64,"},
		{"png.gif", "tmpl_image_content"},
		{"text.svg", "tmpl_image_svg"},
		{"xlink.svg", "data:image/svg+xml;base64,"},
		{"bare.svg", "tmpl_image_svg"},
		{"two.svg", "tmpl_image_svg"},
		{"empty.svg", "tmpl_image_svg"},
		{"open.svg", "tmpl_image_svg"},
		{"entity.svg", "tmpl_image_svg"},
		{"png.svg", "tmpl_image_svg"},
		{"unknown.svg", "tmpl_image_svg"},
		{"notes.txt", "tmpl_image_type"},
		{"large.png", "tmpl_image_large"},
		{"none.png", "tmpl_image_missing"},
		{"sub", "tmpl_image_type"},
		{"dir.png", "tmpl_image_not_file"},
		{"../a.png", "tmpl_image_path"},
		{"/a.png", "tmpl_image_path"},
		{".hidden/c.png", "tmpl_image_path"},
		{"sub/../a.png", "tmpl_image_path"},
		{"link.png", "tmpl_image_symlink"},
		{"out/out.png", "tmpl_image_symlink"},
		{"pic.png", "tmpl_image_symlink"},
		{"abs.png", "tmpl_image_symlink"},
		{"A.png", "tmpl_image_missing"},
		{"SUB/b.PNG", "tmpl_image_missing"},
		{"a.png/x.png", "tmpl_image_missing"},
	} {
		uri, e := imageURI(r.FS(), tt.name, dir)
		got := string(uri)
		if e != nil {
			got = e.Key
		}
		if !strings.HasPrefix(got, tt.want) {
			t.Errorf("imageURI(%s) = %s, want %s", tt.name, got, tt.want)
		}
	}
	// What makes an SVG none is told under it, whole, at the line of the SVG where encoding/xml has one.
	const root = `: its root is not one <svg xmlns="http://www.w3.org/2000/svg">; write the drawing in one such element`
	for name, want := range map[string]string{
		"bare.svg":   filepath.Join(dir, "bare.svg") + root,
		"two.svg":    filepath.Join(dir, "two.svg") + root,
		"text.svg":   filepath.Join(dir, "text.svg") + root,
		"empty.svg":  filepath.Join(dir, "empty.svg") + root,
		"open.svg":   filepath.Join(dir, "open.svg") + ":2: unexpected EOF; write it as XML",
		"entity.svg": filepath.Join(dir, "entity.svg") + ":1: invalid character entity &w;; write it as XML",
		"png.svg":    filepath.Join(dir, "png.svg") + ":3: invalid UTF-8; write it as XML",
		"unknown.svg": filepath.Join(dir, "unknown.svg") + `: xml: opening charset "x-unknown": unsupported charset: "x-unknown"; ` +
			"write an encoding and a version of XML that a reader takes in its <?xml ?> declaration, or leave them out",
	} {
		_, e := imageURI(r.FS(), name, dir)
		if e == nil || e.Key != "tmpl_image_svg" || len(e.Under) != 1 || e.Under[0].Error() != want {
			t.Errorf("the mistake of %s = %v, want under it %s", name, e, want)
		}
	}
	// Just over the limit, an image is told by the byte.
	_, e := imageURI(r.FS(), "large.png", dir)
	if want := fmt.Sprintf(`is %s bytes, over the 1,048,576 bytes (1 MiB)`, english.Sprintf("%d", len(png)+maxImage)); e == nil || !strings.Contains(e.Error(), want) {
		t.Errorf("the mistake of a large image = %v, want %s", e, want)
	}
}

// An image named by many items is reported at each of them, and read once.
func TestCheckImages(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	png, err := os.ReadFile("testdata/sizes/chart.png")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "a.png"), png, 0o600); err != nil { //nolint:gosec // G703: the test's own directory
		t.Fatal(err)
	}
	s := shapeOf(t, `{"properties": {"pics": {"type": "array", "items": {"type": "string", "format": "image"}}}}`)
	d, root := data(t, "pics:\n  - a.png\n  - none.png\n  - a.png\n  - none.png\n")
	err = checkImages(s.drawnWhole(d.Values), d, root, diskImages(dir))
	want := "d.yaml:3:5: pics[1]: there is no image \"none.png\" under " + dir + "\n" +
		"d.yaml:5:5: pics[3]: there is no image \"none.png\" under " + dir
	if err == nil || err.Error() != want {
		t.Errorf("checkImages = %v\nwant %s", err, want)
	}
	if len(d.images) != 1 || !strings.HasPrefix(string(d.images["a.png"]), "data:image/png;base64,") {
		t.Errorf("the images = %v", d.images)
	}
	// An SVG that is none is told at each item, its reason under the first, as told above under the next.
	if err = os.WriteFile(filepath.Join(dir, "b.svg"), []byte("<svg xmlns=\"http://www.w3.org/2000/svg\">\n<rect>"), 0o600); err != nil { //nolint:gosec // G703: the test's own directory
		t.Fatal(err)
	}
	d, root = data(t, "pics:\n  - b.svg\n  - b.svg\n")
	err = checkImages(s.drawnWhole(d.Values), d, root, diskImages(dir))
	want = "d.yaml:2:5: \"b.svg\" is not an SVG document\n" +
		"  " + filepath.Join(dir, "b.svg") + ":2: unexpected EOF; write it as XML\n" +
		"d.yaml:3:5: \"b.svg\" is not an SVG document, as told above"
	if got := usererr.Tell(i18n.EnglishCatalog(), err).Render(""); got != want {
		t.Errorf("checkImages of an SVG that is none = %s\nwant %s", got, want)
	}
}

// An image of assets/ is read once a drawing: a second call does not read it again.
func TestAssetsReadOnce(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "sizes")
	if err := os.CopyFS(dir, os.DirFS("testdata/sizes")); err != nil {
		t.Fatal(err)
	}
	asset := NewLibrary("", "", "", "").template("sizes", dir, dir).assets()
	first, err := asset("logo.svg")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(dir, "assets", "logo.svg")); err != nil {
		t.Fatal(err)
	}
	if again, err := asset("logo.svg"); err != nil || again != first {
		t.Errorf("asset again = %.30s, %v; want it as read first", again, err)
	}
}

// The images a drawing watches, and a publish asks for, are those it may read, once each: not a name it
// refuses whatever file is there.
func TestTriedImages(t *testing.T) {
	t.Parallel()

	s := shapeOf(t, `{"properties": {"pics": {"type": "array", "items": {"type": "string", "format": "image"}}}}`)
	d, root := data(t, "pics: [a.png, ../b.png, a.png, .c.png, notes.txt, Data.yaml/d.png]\n")
	_ = checkImages(s.drawnWhole(d.Values), d, root, diskImages(t.TempDir())) //nolint:errcheck // the images are missing; what was tried is the point
	if want := []string{"a.png"}; !slices.Equal(d.tried, want) {
		t.Errorf("tried = %v, want %v", d.tried, want)
	}
}

// Images are read from any fs.FS, as a server reads those a publish sent: its mistakes name the
// directory given, and ReadImage refuses what the check refuses.
func TestImagesFromFS(t *testing.T) {
	t.Parallel()

	png, err := os.ReadFile("testdata/sizes/chart.png")
	if err != nil {
		t.Fatal(err)
	}
	files := fstest.MapFS{"sub/a.png": {Data: png}}
	s := shapeOf(t, `{"properties": {"pics": {"type": "array", "items": {"type": "string", "format": "image"}}}}`)
	d, root := data(t, "pics: [sub/a.png, none.png]\n")
	err = checkImages(s.drawnWhole(d.Values), d, root, fsImages(files, "/plans"))
	if want := `d.yaml:1:19: pics[1]: there is no image "none.png" under /plans`; err == nil || err.Error() != want {
		t.Errorf("checkImages = %v\nwant %s", err, want)
	}
	if _, ok := d.images["sub/a.png"]; !ok {
		t.Errorf("the images = %v, want sub/a.png", d.images)
	}
	if b, err := ReadImage(files, "sub/a.png", "/plans"); err != nil || !bytes.Equal(b, png) {
		t.Errorf("ReadImage(sub/a.png) = %d bytes, %v", len(b), err)
	}
	if _, err := ReadImage(files, "data.yaml/../a.png", "/plans"); err == nil || !strings.Contains(err.Error(), "without ..") {
		t.Errorf("ReadImage(data.yaml/../a.png) = %v, want the path refused", err)
	}
	for _, name := range []string{"../a.png", ".a.png", "sub/a.txt", "none.png", "Data.yaml/a.png"} {
		if _, err := ReadImage(files, name, "/plans"); err == nil {
			t.Errorf("ReadImage(%s) = nil error", name)
		}
	}
}
