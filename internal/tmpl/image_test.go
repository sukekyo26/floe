package tmpl_test

import (
	"bytes"
	"encoding/base64"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/sukekyo26/floe/internal/files"
	"github.com/sukekyo26/floe/internal/tmpl"
)

// imagesIn returns the root of a new directory holding files, by their paths, closed at the end of the test.
func imagesIn(t *testing.T, files map[string][]byte) (string, *files.Root) {
	t.Helper()
	dir := t.TempDir()
	for name, b := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir, rootOf(t, dir)
}

func rootOf(t *testing.T, dir string) *files.Root {
	t.Helper()
	root := files.NewRoot(dir)
	t.Cleanup(func() { _ = root.Close() })
	return root
}

// report's part image draws its file as a data URI, with its alt, in a section, in tabs and in details.
func TestImagePart(t *testing.T) {
	t.Parallel()

	_, images := imagesIn(t, map[string][]byte{"shots/a.png": png, "shots/A.PNG": png})
	src := `lang: en
template: report
sections:
  - parts:
      - image:
          alt: The list
          file: shots/a.png
      - tabs:
          - title: Before
            parts:
              - image: { file: shots/A.PNG, alt: Before }
          - title: After
            parts:
              - text: After
      - details:
          title: Folded
          parts:
            - image: { file: shots/a.png, alt: Folded }
`
	page, _, err := tmpl.Draw(floeKit(t), "d.yaml", []byte(src), tmpl.Options{Theme: "", Untitled: "x", Images: images, ImagesDir: ""})
	if err != nil {
		t.Fatalf("told\n%s", told(err))
	}
	uri := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	for _, alt := range []string{"The list", "Before", "Folded"} {
		if want := `<figure data-floe-part="image"><img src="` + uri + `" alt="` + alt + `"/></figure>`; !bytes.Contains(page, []byte(want)) {
			t.Errorf("no %s in\n%s", want, page)
		}
	}
}

// The mistakes of each image of part image are told together: one that names no file floe reads at its file:,
// once at each; one that cannot be read at its path, once.
func TestImagePartMistakes(t *testing.T) {
	t.Parallel()

	dir, images := imagesIn(t, map[string][]byte{"shots/a.png": png, "shots/a.bmp": png, "shots/big.png": bytes.Repeat([]byte{0}, 1<<20+1)})
	if err := os.Mkdir(filepath.Join(dir, "dir.png"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc/hostname", filepath.Join(dir, "shots/out.png")); err != nil {
		t.Fatal(err)
	}
	src := `lang: en
template: report
sections:
  - parts:
      - image:
          alt: one
          file: ./shots/a.png
      - image: { alt: two, file: shots/a.bmp }
      - image: { alt: three, file: shots/a.bmp }
      - image: { alt: four, file: shots/none.png }
      - image: { alt: five, file: shots/none.png }
      - image: { alt: six, file: dir.png }
      - image: { alt: seven, file: shots/big.png }
      - image: { alt: eight, file: shots/out.png }
      - image: { alt: nine, file: ../a.png }
`
	path := func(at, name string) string {
		return "d.yaml:" + at + `: "` + name + `" is not a path floe reads under the directory of d.yaml: write it relative to that directory, with one / between names, without ./ (write pic.png for ./pic.png), .., or a / at either end`
	}
	bmp := func(at string) string {
		return "d.yaml:" + at + `: "shots/a.bmp" is not an image floe takes: use png, jpeg, gif, webp or svg`
	}
	want := lines(
		path("7:17", "./shots/a.png"),
		bmp("8:34"),
		bmp("9:36"),
		`d.yaml:12:34: "dir.png" is not a file of an image, but a directory or the like: name a regular file of the image`,
		`d.yaml:13:36: the image "shots/big.png" is 1,048,577 bytes, over the 1,048,576 bytes (1 MiB) an image may be; make it smaller`,
		path("15:35", "../a.png"),
		"shots/none.png: no such file or directory",
		"shots/out.png: path escapes from parent",
	)
	if got := drawn(t, floeKit(t), src, tmpl.Options{Theme: "", Untitled: "x", Images: images, ImagesDir: ""}); got != want {
		t.Errorf("told\n%s\nwant\n%s", got, want)
	}
}

// A part of one's own takes images under properties and items, each told of apart; one whose value is an image
// is not drawn when the image cannot be read, so that its part.html tells nothing of a value it never had.
func TestOwnPartImages(t *testing.T) {
	t.Parallel()

	k := ownKit(t, map[string]string{
		"shots/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"one": {"type": "string", "format": "image"}, "many": {"type": "array", "items": {"type": "string", "format": "image"}}}}`,
		"shots/part.html":  `<div data-floe-part="shots">{{with .one}}<img src="{{.}}" alt="">{{end}}{{range .many}}<img src="{{.}}" alt="">{{end}}</div>`,
		"shots/part.css":   "",
		"shot/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "string", "format": "image"}`,
		"shot/part.html":   `<div data-floe-part="shot"><img src="{{.}}" alt="" data-n="{{len .}}"></div>`,
		"shot/part.css":    "",
	}, mine(
		"<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head>\n<body>\n"+
			"{{.Data.shots}}{{.Data.shot}}\n</body></html>\n",
		`{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"shots": {"$ref": "floe:///parts/shots.json"}, "shot": {"$ref": "floe:///parts/shot.json"}}}`, nil))
	_, images := imagesIn(t, map[string][]byte{"a.png": png})
	o := tmpl.Options{Theme: "", Untitled: "x", Images: images, ImagesDir: ""}

	page, _, err := tmpl.Draw(k, "d.yaml", []byte("lang: en\ntemplate: mine\nshots: { one: a.png, many: [a.png, a.png] }\nshot: a.png\n"), o)
	if err != nil {
		t.Fatalf("told\n%s", told(err))
	}
	if n := strings.Count(string(page), `<img src="data:image/png;base64,`); n != 4 {
		t.Errorf("%d images drawn, want 4, in\n%s", n, page)
	}

	src := "lang: en\ntemplate: mine\nshots:\n  one: a.bmp\n  many: [gone.png, b.bmp]\nshot: none.png\n"
	want := lines(
		"none.png: no such file or directory",
		"gone.png: no such file or directory",
		`d.yaml:4:8: "a.bmp" is not an image floe takes: use png, jpeg, gif, webp or svg`,
		`d.yaml:5:20: "b.bmp" is not an image floe takes: use png, jpeg, gif, webp or svg`,
	)
	if got := drawn(t, k, src, o); got != want {
		t.Errorf("told\n%s\nwant\n%s", got, want)
	}
}

// counted is an fs.FS that counts the Open and the Stat of each name.
type counted struct {
	fs.StatFS
	mu    sync.Mutex
	opens map[string]int
	stats map[string]int
}

func (c *counted) Open(name string) (fs.File, error) {
	c.mu.Lock()
	c.opens[name]++
	c.mu.Unlock()
	return c.StatFS.Open(name) //nolint:wrapcheck // the files' own
}

func (c *counted) Stat(name string) (fs.FileInfo, error) {
	c.mu.Lock()
	c.stats[name]++
	c.mu.Unlock()
	return c.StatFS.Stat(name) //nolint:wrapcheck // the files' own
}

// An image named twice is read once a drawing.
func TestImageReadOnce(t *testing.T) {
	t.Parallel()

	images := &counted{StatFS: fstest.MapFS{"a.png": &fstest.MapFile{Data: png}}, opens: map[string]int{}, stats: map[string]int{}}
	src := "lang: en\ntemplate: report\nsections:\n  - parts:\n      - image: { file: a.png, alt: a }\n      - image: { file: a.png, alt: b }\n"
	if _, _, err := tmpl.Draw(floeKit(t), "d.yaml", []byte(src), tmpl.Options{Theme: "", Untitled: "x", Images: images, ImagesDir: ""}); err != nil {
		t.Fatalf("told\n%s", told(err))
	}
	if images.opens["a.png"] != 1 || images.stats["a.png"] != 1 {
		t.Errorf("a.png opened %d and stated %d times, want once each", images.opens["a.png"], images.stats["a.png"])
	}
}
