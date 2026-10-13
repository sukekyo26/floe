package tmpl

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		"logo.svg":      []byte("\xef\xbb\xbf<!DOCTYPE svg>\n<svg viewBox=\"0 0 1 1\"/>"),
		"text.svg":      []byte("<html><svg/></html>"),
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
	if err = os.Symlink(outside, filepath.Join(dir, "link.png")); err != nil {
		t.Fatal(err)
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
		{"text.svg", "tmpl_image_content"},
		{"notes.txt", "tmpl_image_type"},
		{"large.png", "tmpl_image_large"},
		{"none.png", "tmpl_image_missing"},
		{"sub", "tmpl_image_type"},
		{"dir.png", "tmpl_image_not_file"},
		{"../a.png", "tmpl_image_path"},
		{"/a.png", "tmpl_image_path"},
		{".hidden/c.png", "tmpl_image_path"},
		{"sub/../a.png", "tmpl_image_path"},
		{"link.png", "tmpl_image_unreadable"},
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
	s, err := decodeSchema([]byte(`{"properties": {"pics": {"type": "array", "items": {"type": "string", "format": "image"}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	d, root := data(t, "pics:\n  - a.png\n  - none.png\n  - a.png\n  - none.png\n")
	err = checkImages(s, d, root, dir)
	want := "d.yaml:3:5: pics[1]: there is no image \"none.png\" under " + dir + "\n" +
		"d.yaml:5:5: pics[3]: there is no image \"none.png\" under " + dir
	if err == nil || err.Error() != want {
		t.Errorf("checkImages = %v\nwant %s", err, want)
	}
	if len(d.images) != 1 || !strings.HasPrefix(string(d.images["a.png"]), "data:image/png;base64,") {
		t.Errorf("the images = %v", d.images)
	}
}
