package store_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/store"
	"github.com/sukekyo26/floe/internal/usererr"
)

func ptr[T any](v T) *T {
	return &v
}

// told words err as the command line does, the store's path as "S".
func told(err error, root string) string {
	return strings.ReplaceAll(usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: "), root, "S")
}

const picSchema = `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"i": {"type": "array", "items": {"type": "string", "format": "image"}}, "need": {},
"d": {"$ref": "floe:///parts/diagram.json"}}}`

// picKit is floe's own with a template pic of its own, which shows the images of i, its page.html opened by head
// and its schema.json schema (picSchema for "").
func picKit(t *testing.T, head, schema string) store.Drawing {
	t.Helper()

	floe, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	page := "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><title>{{.Title}}</title></head>\n<body>" + head +
		"{{range .Data.i}}<img src=\"{{.}}\">{{end}}{{with .Data.d}}{{.}}{{end}}</body></html>\n"
	files := fstest.MapFS{
		"pic/page.html":    {Data: []byte(page)},
		"pic/schema.json":  {Data: []byte(cmpOr(schema, picSchema))},
		"pic/example.yaml": {Data: []byte("lang: en\ntemplate: pic\n")},
	}
	none := kit.Dir{FS: nil, Name: ""}
	k, err := kit.Load(floe, kit.Sources{Parts: none, Themes: none, Templates: kit.Dir{FS: files, Name: "templates"}})
	if err != nil {
		t.Fatal(err)
	}
	return store.Drawing{Kit: k, Icons: nil, Err: nil}
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func newStore(t *testing.T) (*store.Store, string) {
	t.Helper()

	tmp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(tmp, "store")
	return store.New(root), root
}

var png = []byte("\x89PNG\r\n\x1a\n0000")

// publish prepares and saves the data src as file, the images sent with it, from project p.
func publish(t *testing.T, s *store.Store, dw store.Drawing, file, src string, images map[string][]byte, o store.Options) *store.Entry {
	t.Helper()

	e, err := s.PrepareData(dw, store.Upload{File: file, Source: []byte(src), Images: images, Project: "p"}, o)
	if err != nil {
		t.Fatalf("PrepareData: %v", err)
	}
	if err := s.Save(e); err != nil {
		t.Fatalf("Save: %v", err)
	}
	return e
}

// entries counts the entries of dir.
func entries(t *testing.T, dir string) int {
	t.Helper()

	got, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return len(got)
}

func read(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// copyOf is the directory of the copy of the stored page slug.
func copyOf(t *testing.T, s *store.Store, root, slug string) string {
	t.Helper()

	m := s.ReadMeta(slug)
	if m == nil {
		t.Fatalf("no meta.json of %s", slug)
	}
	return filepath.Join(root, slug, m.Source)
}

// files lists the files under dir, by their slash-separated paths.
func files(t *testing.T, dir string) []string {
	t.Helper()

	var got []string
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, relErr := filepath.Rel(dir, p)
			got = append(got, filepath.ToSlash(rel))
			return relErr
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// A stored page keeps the data file, the page drawn, without its CSP, and the images it read, under images/ at
// the names the data gives them: data.yaml/ among them. An image sent that it did not read is not kept. A
// publish again replaces the copy.
func TestPublishCopy(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := picKit(t, "", "")
	src := "lang: en\ntemplate: pic\ntitle: P\ni: [a.png, data.yaml/b.png]\n"
	images := map[string][]byte{"a.png": png, "data.yaml/b.png": png, "unread.png": png, "../evil.png": png}
	e := publish(t, s, dw, "d.yaml", src, images, store.Options{})
	dir := copyOf(t, s, root, "d")
	if got, want := files(t, dir), []string{"data.yaml", "images/a.png", "images/data.yaml/b.png", "page.html"}; !slices.Equal(got, want) {
		t.Errorf("files %v, want %v", got, want)
	}
	if page := read(t, filepath.Join(dir, "page.html")); page != string(e.Page) || strings.Contains(page, "Content-Security-Policy") {
		t.Errorf("page.html is not the page drawn without its CSP:\n%s", page)
	}
	if got := read(t, filepath.Join(dir, "data.yaml")); got != src {
		t.Errorf("data.yaml %q", got)
	}
	if file, ok := s.File("d", s.ReadMeta("d"), ""); !ok || file != filepath.Join(dir, "page.html") {
		t.Errorf("File = %s, %v", file, ok)
	}
	publish(t, s, dw, "d.yaml", "lang: en\ntemplate: pic\n", nil, store.Options{})
	if n := entries(t, filepath.Join(root, "d")); n != 2 {
		t.Errorf("%d entries under d, want meta.json and one copy", n)
	}
	if got := files(t, copyOf(t, s, root, "d")); !slices.Equal(got, []string{"data.yaml", "page.html"}) {
		t.Errorf("files %v, want no image", got)
	}
	if fi, err := os.Stat(filepath.Join(copyOf(t, s, root, "d"), "images")); err != nil || !fi.IsDir() {
		t.Errorf("images/ of a page of no image: %v", err)
	}
}

// With no images sent, the images the data reads are asked for, after the slug's owner; a mistake before the
// drawing is told without asking. With some sent, or none in an empty map, one not sent is a mistake.
func TestNeedImages(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := picKit(t, "", "")
	two := "lang: en\ntemplate: pic\ni: [img/a.png, img/b.png, img/a.png]\n"
	_, err := s.PrepareData(dw, store.Upload{File: "d.yaml", Source: []byte(two), Images: nil, Project: "p"}, store.Options{})
	if need, ok := errors.AsType[*store.NeedImages](err); !ok || !slices.Equal(need.Names, []string{"img/a.png", "img/b.png"}) {
		t.Errorf("PrepareData = %v, want NeedImages of img/a.png and img/b.png", err)
	}
	schema := "lang: en\ntemplate: pic\ni: [img/a.png]\nx: 1\nj: 2\n"
	_, err = s.PrepareData(picKit(t, "", `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"i": {"type": "array", "items": {"type": "string", "format": "image"}}}, "additionalProperties": false}`),
		store.Upload{File: "d.yaml", Source: []byte(schema), Images: nil, Project: "p"}, store.Options{})
	if _, ok := errors.AsType[*store.NeedImages](err); ok || err == nil {
		t.Errorf("a mistake of the schema: PrepareData = %v", err)
	}
	for _, sent := range []map[string][]byte{{"img/a.png": png}, {}} {
		_, err = s.PrepareData(dw, store.Upload{File: "d.yaml", Source: []byte(two), Images: sent, Project: "p"}, store.Options{})
		if got := told(err, root); !strings.Contains(got, "img/b.png: file does not exist") {
			t.Errorf("sent %v: told\n%s", sent, got)
		}
	}
	publish(t, s, dw, "d.yaml", "lang: en\ntemplate: pic\n", nil, store.Options{})
	_, err = s.PrepareData(dw, store.Upload{File: "d.yaml", Source: []byte(two), Images: nil, Project: "q"}, store.Options{})
	if got, want := told(err, root), `floe: slug "d" was published from project p; use another --slug, or add --force to replace that page`; got != want {
		t.Errorf("told\n%s\nwant\n%s", got, want)
	}
}

// A title or a theme given empty is not given; a slug is checked as given.
func TestAccept(t *testing.T) {
	t.Parallel()

	o, err := store.Accept("a.yaml", false, store.Options{Title: ptr(" "), Theme: ptr(""), Description: ptr(" ")})
	if err != nil || o.Title != nil || o.Theme != nil || o.Description == nil || *o.Description != "" || *o.Slug != "a" {
		t.Errorf("Accept = %+v, %v", o, err)
	}
	for _, slug := range []string{"Foo", "foo ", "-a", strings.Repeat("a", 65)} {
		_, err := store.Accept("a.yaml", false, store.Options{Slug: ptr(slug)})
		want := `floe: invalid slug "` + slug + `": use 1 to 64 lowercase letters, digits and hyphens, starting and ending with a letter or digit`
		if got := told(err, "\x00"); got != want {
			t.Errorf("told\n%s\nwant\n%s", got, want)
		}
	}
	if _, err := store.Accept("a.yaml", false, store.Options{Slug: ptr("a--b")}); err != nil {
		t.Errorf("a--b: %v", err)
	}
}

// A title kept on the management page stays over a publish without --title, and an empty one too.
func TestPublishKeepsTitle(t *testing.T) {
	t.Parallel()

	s, _ := newStore(t)
	dw := picKit(t, "", "")
	publish(t, s, dw, "d.yaml", "lang: en\ntemplate: pic\ntitle: Data\n", nil, store.Options{})
	if _, err := s.Edit("d", store.Edit{Title: ptr("Kept")}); err != nil {
		t.Fatal(err)
	}
	e := publish(t, s, dw, "d.yaml", "lang: en\ntemplate: pic\ntitle: Data\n", nil, store.Options{Title: ptr("")})
	if m := s.ReadMeta("d"); m.Title != "Kept" || !strings.Contains(string(e.Page), "<title>Data</title>") {
		t.Errorf("title %q, page %s", m.Title, e.Page)
	}
}

// A slug that cannot be one is no page, whatever reads or writes it: none leads out of the store.
func TestSlugIsNoPath(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "..", "meta.json"), []byte(`{"updatedAt": "x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"..", ".", "", "a/b", "/", "A"} {
		if s.ReadMeta(slug) != nil {
			t.Errorf("ReadMeta(%q) read a meta.json", slug)
		}
		if err := s.WriteMeta(slug, &store.Meta{}); err == nil {
			t.Errorf("WriteMeta(%q) wrote", slug)
		}
		if _, ok := s.File(slug, &store.Meta{Source: "source-1"}, ""); ok {
			t.Errorf("File(%q) gave a file", slug)
		}
		if _, err := s.Edit(slug, store.Edit{}); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("Edit(%q) = %v", slug, err)
		}
		if err := s.Remove(slug); !errors.Is(err, store.ErrNotFound) {
			t.Errorf("Remove(%q) = %v", slug, err)
		}
	}
	if list, err := s.List(); err != nil || len(list) != 0 {
		t.Errorf("List = %v, %v", list, err)
	}
}

// A copy a meta.json names is one a save gives, or none: one that leads out of the page's directory is no page,
// to serve or to draw again.
func TestCopyIsUnderItsPage(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := picKit(t, "", "")
	publish(t, s, dw, "d.yaml", "lang: en\ntemplate: pic\n", nil, store.Options{})
	m := s.ReadMeta("d")
	for _, source := range []string{"../x", "/x", "source-a/../..", "", "x"} {
		m.Source = source
		if err := s.WriteMeta("d", m); err != nil {
			t.Fatal(err)
		}
		if _, ok := s.File("d", m, ""); ok {
			t.Errorf("File with source %q gave a file", source)
		}
		if err := s.Rebuild(picKit(t, "v2", "")); err != nil {
			t.Errorf("Rebuild with source %q: %s", source, told(err, root))
		}
	}
}

// A link to an index.html is named after its directory, on Save too; HTML takes no theme; a data file linked
// is drawn once, its mistakes refusing it.
func TestPrepareLink(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	home := filepath.Dir(root)
	dir := filepath.Join(home, "proj", "sub")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"index.html": "<title>i</title>", "bad.yaml": "lang: en\ntemplate: nope\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	dw := picKit(t, "", "")
	e, err := s.PrepareLink(t.Context(), dw, "sub/index.html", store.Options{}, filepath.Join(home, "proj"), home)
	if err != nil {
		t.Fatal(told(err, root))
	}
	if err = s.Save(e); err != nil || e.Slug != "sub" {
		t.Errorf("Save: %v, slug %s", err, e.Slug)
	}
	_, err = s.PrepareLink(t.Context(), dw, filepath.Join(dir, "index.html"), store.Options{Theme: ptr("ink")}, "", home)
	if got := told(err, root); got != "floe: --theme picks the theme of a page drawn from a data file: a linked file is shown as it is" {
		t.Errorf("told %s", got)
	}
	_, err = s.PrepareLink(t.Context(), dw, filepath.Join(dir, "bad.yaml"), store.Options{}, "", home)
	if got := told(err, root); !strings.Contains(got, `unknown template "nope"`) {
		t.Errorf("told %s", got)
	}
	if list, err := s.List(); err != nil || len(list) != 1 || list[0].Title != "i" {
		t.Errorf("List = %+v, %v", list, err)
	}
}

// Owners: a page of another project, or a link of another file, is refused without --force; a page and a link
// do not take each other's slug.
func TestOwners(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := picKit(t, "", "")
	publish(t, s, dw, "same.yaml", "lang: en\ntemplate: pic\n", nil, store.Options{})
	_, err := s.PrepareData(dw, store.Upload{File: "same.yaml", Source: []byte("lang: en\ntemplate: pic\n"), Images: map[string][]byte{}, Project: "q"}, store.Options{})
	if got := told(err, root); got != `floe: slug "same" was published from project p; use another --slug, or add --force to replace that page` {
		t.Errorf("told %s", got)
	}
	up := store.Upload{File: "same.yaml", Source: []byte("lang: en\ntemplate: pic\n"), Images: map[string][]byte{}, Project: "q"}
	if _, err = s.PrepareData(dw, up, store.Options{Force: true}); err != nil {
		t.Errorf("--force: %v", err)
	}
	home := filepath.Dir(root)
	html := filepath.Join(home, "proj", "z.html")
	if err = os.MkdirAll(filepath.Dir(html), 0o700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(html, []byte("<title>z</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = s.PrepareLink(t.Context(), dw, html, store.Options{Slug: ptr("same")}, "", home)
	if got := told(err, root); got != `floe: slug "same" is used by a stored page; choose another slug, or remove that page first (floe rm same)` {
		t.Errorf("told %s", got)
	}
}

// Images sent one under another, which a disk cannot hold, are refused before they are saved, at the data file.
func TestImageUnderImage(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	src := "lang: en\ntemplate: pic\ni: [a.png, a.png/b.png]\n"
	_, err := s.PrepareData(picKit(t, "", ""), store.Upload{File: "d.yaml", Source: []byte(src), Images: map[string][]byte{"a.png": png, "a.png/b.png": png}, Project: "p"}, store.Options{})
	want := `d.yaml: the image "a.png/b.png" is under "a.png", which is an image too, and a stored page cannot keep both: rename one of them`
	if got := told(err, root); got != want {
		t.Errorf("told\n%s\nwant\n%s", got, want)
	}
}

// A failure of floe's in drawing is told before the images are asked for.
func TestFailureBeforeNeedImages(t *testing.T) {
	t.Parallel()

	s, _ := newStore(t)
	dw := picKit(t, "", "")
	dw.Icons = failing{}
	src := "lang: en\ntemplate: pic\ni: [a.png]\nd: {width: 300, height: 100, title: i, parts: [{icon: {icon: aws/res/aws-lambda, x: 10, y: 10}}]}\n"
	_, err := s.PrepareData(dw, store.Upload{File: "d.yaml", Source: []byte(src), Images: nil, Project: "p"}, store.Options{})
	var found usererr.List
	if _, ok := errors.AsType[*store.NeedImages](err); ok || found.Add(err) == nil {
		t.Errorf("PrepareData = %v, want floe's failure", err)
	}
}

type failing struct{}

func (failing) Lookup(string) ([]byte, error) { return nil, errors.New("boom") } //nolint:err113 // the test's

// A store that cannot be looked into is told at it.
func TestStoreUnreadable(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root reads what it may not")
	}
	s, root := newStore(t)
	parent := filepath.Dir(root)
	if err := os.Chmod(parent, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) }) //nolint:errcheck // the test is told already
	if got, want := told(s.Remove("a"), "\x00"), root+": permission denied"; got != want {
		t.Errorf("told\n%s\nwant\n%s", got, want)
	}
}

// A stored page is listed under its data's title:, kept as its own to give back when a title pinned is cleared;
// one of none under its slug.
func TestPublishTitle(t *testing.T) {
	t.Parallel()

	s, _ := newStore(t)
	dw := picKit(t, "", "")
	publish(t, s, dw, "d.yaml", "lang: en\ntemplate: pic\ntitle: Data\n", nil, store.Options{})
	if m := s.ReadMeta("d"); m.Title != "Data" || m.DataTitle != "Data" {
		t.Errorf("title %q, data's %q", m.Title, m.DataTitle)
	}
	if _, err := s.Edit("d", store.Edit{Title: ptr("X")}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Edit("d", store.Edit{Title: ptr("")}); err != nil || s.ReadMeta("d").Title != "Data" {
		t.Errorf("cleared: %q, %v", s.ReadMeta("d").Title, err)
	}
	publish(t, s, dw, "e.yaml", "lang: en\ntemplate: pic\n", nil, store.Options{})
	if m := s.ReadMeta("e"); m.Title != "e" || m.DataTitle != "" {
		t.Errorf("title %q, data's %q", m.Title, m.DataTitle)
	}
}

// A name in use is a Conflict, alone, its mistake told as it is; no page to remove is ErrNotFound.
func TestConflict(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := picKit(t, "", "")
	publish(t, s, dw, "same.yaml", "lang: en\ntemplate: pic\n", nil, store.Options{})
	_, err := s.PrepareData(dw, store.Upload{File: "same.yaml", Source: []byte("lang: en\ntemplate: pic\n"), Images: nil, Project: "q"}, store.Options{})
	if _, ok := errors.AsType[*store.Conflict](err); !ok {
		t.Errorf("PrepareData = %v, want a Conflict", err)
	}
	if got := told(err, root); got != `floe: slug "same" was published from project p; use another --slug, or add --force to replace that page` {
		t.Errorf("told %s", got)
	}
	_, err = s.PrepareData(dw, store.Upload{File: "same.yaml", Source: []byte("lang: en\ntemplate: nope\n"), Images: nil, Project: "p"}, store.Options{})
	if _, ok := errors.AsType[*store.Conflict](err); ok {
		t.Errorf("a mistake of the data is a Conflict: %v", err)
	}
	if err := s.Remove("none"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Remove of no page = %v", err)
	}
}
