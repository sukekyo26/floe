package store_test

import (
	"bytes"
	"cmp"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/store"
)

// numbers are the numbers of the versions of the page d.
func numbers(t *testing.T, s *store.Store) []int {
	t.Helper()

	m := s.ReadMeta("d")
	if m == nil {
		t.Fatal("no meta.json of d")
	}
	got := make([]int, 0, len(m.Versions))
	for _, v := range m.Versions {
		got = append(got, v.N)
	}
	return got
}

// gzips are the names under versions/ of the page d.
func gzips(t *testing.T, root string) []string {
	t.Helper()

	got := []string{}
	entries, err := os.ReadDir(filepath.Join(root, "d", "versions"))
	if os.IsNotExist(err) {
		return got
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range entries {
		got = append(got, d.Name())
	}
	// By number: 2.html.gz before 10.html.gz.
	slices.SortFunc(got, func(a, b string) int { return cmp.Or(len(a)-len(b), strings.Compare(a, b)) })
	return got
}

// page is the page of version n of slug, which must be there.
func page(t *testing.T, s *store.Store, slug string, n int) string {
	t.Helper()

	b, ok := s.VersionPage(slug, s.ReadMeta(slug), n)
	if !ok {
		t.Fatalf("no version %d of %s", n, slug)
	}
	return string(b)
}

func titled(title string) string {
	return "lang: en\ntemplate: pic\ntitle: " + title + "\ni: [a.png]\n"
}

// A publish adds a version when what it is published with, the data file and the images it reads, is not the last
// version's: the same again, a theme only, or an image sent but not read adds none. The version before is the page
// of the copy as it was when it was last, gzipped; the last is the copy's page.
func TestSaveVersions(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := picKit(t, "", "")
	images := map[string][]byte{"a.png": png, "unread.png": png}
	other := []byte("\x89PNG\r\n\x1a\n1111")
	steps := []struct {
		name   string
		src    string
		images map[string][]byte
		theme  string
		want   []int
		gzips  []string
	}{
		{"first", titled("A"), images, "", []int{1}, []string{}},
		{"the same", titled("A"), images, "", []int{1}, []string{}},
		{"a theme only", titled("A"), images, "swiss", []int{1}, []string{}},
		{"another title", titled("B"), images, "", []int{1, 2}, []string{"1.html.gz"}},
		{"an image read", titled("B"), map[string][]byte{"a.png": other, "unread.png": png}, "", []int{1, 2, 3}, []string{"1.html.gz", "2.html.gz"}},
		{"an image not read", titled("B"), map[string][]byte{"a.png": other, "unread.png": other}, "", []int{1, 2, 3}, []string{"1.html.gz", "2.html.gz"}},
	}
	drawn := make([]string, len(steps))
	for i, step := range steps {
		o := store.Options{}
		if step.theme != "" {
			o.Theme = ptr(step.theme)
		}
		drawn[i] = string(publish(t, s, dw, "d.yaml", step.src, step.images, o).Page)
		if got := numbers(t, s); !slices.Equal(got, step.want) {
			t.Errorf("%s: versions %v, want %v", step.name, got, step.want)
		}
		if got := gzips(t, root); !slices.Equal(got, step.gzips) {
			t.Errorf("%s: versions/ %v, want %v", step.name, got, step.gzips)
		}
	}
	if drawn[2] == drawn[1] {
		t.Fatal("a theme drew the same page")
	}
	// v1 was last drawn in the theme, v2 by the fourth publish; v3 is the copy, the last publish's page.
	for n, want := range map[int]string{1: drawn[2], 2: drawn[3], 3: drawn[5]} {
		if got := page(t, s, "d", n); got != want {
			t.Errorf("version %d is not the page it was last:\n%s", n, got)
		}
	}
	gz := filepath.Join(root, "d", "versions", "1.html.gz")
	b, err := os.ReadFile(gz)
	if err != nil || !bytes.HasPrefix(b, []byte{0x1f, 0x8b}) {
		t.Fatalf("1.html.gz is no gzip: %v", err)
	}
	for _, broken := range [][]byte{b[:2], b[:len(b)/2], b[:len(b)-1]} {
		if err := os.WriteFile(gz, broken, 0o600); err != nil { //nolint:gosec // G703: the test's own temporary file
			t.Fatal(err)
		}
		if got, ok := s.VersionPage("d", s.ReadMeta("d"), 1); ok {
			t.Errorf("a broken gzip of %d bytes read as %q", len(broken), got)
		}
	}
}

// Past 20 versions, the oldest go, from the list and the disk.
func TestVersionsKept(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := picKit(t, "", "")
	for i := range 21 {
		publish(t, s, dw, "d.yaml", titled(strings.Repeat("x", i+1)), map[string][]byte{"a.png": png}, store.Options{})
	}
	var want []int
	var names []string
	for n := 2; n <= 21; n++ {
		want = append(want, n)
		if n < 21 {
			names = append(names, strconv.Itoa(n)+".html.gz")
		}
	}
	if got := numbers(t, s); !slices.Equal(got, want) {
		t.Errorf("versions %v, want %v", got, want)
	}
	if got := gzips(t, root); !slices.Equal(got, names) {
		t.Errorf("versions/ %v, want %v", got, names)
	}
}

// A rebuild adds no version and writes no gzip: it draws the last one, the copy, again. What versions/ holds that
// meta.json does not name goes when the page is drawn otherwise, and stays when it is drawn the same.
func TestRebuildKeepsVersions(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := picKit(t, "", "")
	publish(t, s, dw, "d.yaml", titled("A"), map[string][]byte{"a.png": png}, store.Options{})
	publish(t, s, dw, "d.yaml", titled("B"), map[string][]byte{"a.png": png}, store.Options{})
	stray := filepath.Join(root, "d", "versions", "9.html.gz")
	if err := os.WriteFile(stray, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, first := s.ReadMeta("d").Versions, page(t, s, "d", 1)
	if err := s.Rebuild(dw); err != nil {
		t.Fatal(told(err, root))
	}
	if got := gzips(t, root); !slices.Equal(got, []string{"1.html.gz", "9.html.gz"}) {
		t.Errorf("drawn the same, versions/ %v", got)
	}
	if err := s.Rebuild(picKit(t, "v2", "")); err != nil {
		t.Fatal(told(err, root))
	}
	if after := s.ReadMeta("d").Versions; !slices.Equal(after, before) {
		t.Errorf("versions %v, before %v", after, before)
	}
	if got := gzips(t, root); !slices.Equal(got, []string{"1.html.gz"}) {
		t.Errorf("versions/ %v, want 1.html.gz", got)
	}
	if !strings.Contains(page(t, s, "d", 2), "<body>v2") || page(t, s, "d", 1) != first {
		t.Errorf("not the last drawn again only:\n%s\n%s", page(t, s, "d", 1), page(t, s, "d", 2))
	}
}

// A page replaced goes on from the numbers of the one before, whatever its content, keeping none of its versions; one
// removed starts again at 1. A page of no versions, as published before pages had them, gets one whatever its
// content, of the time it was published before; one replaced, or of another publish after, of now.
func TestVersionsNumbers(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := picKit(t, "", "")
	publish(t, s, dw, "d.yaml", titled("A"), map[string][]byte{"a.png": png}, store.Options{})
	publish(t, s, dw, "d.yaml", titled("B"), map[string][]byte{"a.png": png}, store.Options{})
	up := store.Upload{File: "d.yaml", Source: []byte(titled("B")), Images: map[string][]byte{"a.png": png}, Project: "q"}
	e, err := s.PrepareData(dw, up, store.Options{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Save(e); err != nil {
		t.Fatal(err)
	}
	if got := numbers(t, s); !slices.Equal(got, []int{3}) {
		t.Errorf("replaced: versions %v, want [3]", got)
	}
	if got := gzips(t, root); len(got) != 0 {
		t.Errorf("replaced: versions/ %v", got)
	}
	if err = s.Remove("d"); err != nil {
		t.Fatal(err)
	}
	publish(t, s, dw, "d.yaml", titled("B"), map[string][]byte{"a.png": png}, store.Options{})
	if got := numbers(t, s); !slices.Equal(got, []int{1}) {
		t.Errorf("removed: versions %v, want [1]", got)
	}
	const long = "2026-01-02T03:04:05.000Z"
	unversioned := func(slug string) *store.Meta {
		m := s.ReadMeta(slug)
		m.Versions, m.UpdatedAt = nil, long
		if err = s.WriteMeta(slug, m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	was := unversioned("d")
	publish(t, s, dw, "d.yaml", titled("B"), map[string][]byte{"a.png": png}, store.Options{})
	m := s.ReadMeta("d")
	if len(m.Versions) != 1 || m.Versions[0].N != 1 || m.Versions[0].At != long || m.UpdatedAt != long || m.CreatedAt != was.CreatedAt {
		t.Errorf("of no versions: %+v", m)
	}
	publish(t, s, dw, "d.yaml", titled("C"), map[string][]byte{"a.png": png}, store.Options{})
	if m := s.ReadMeta("d"); m.UpdatedAt == long || m.Versions[1].At != m.UpdatedAt {
		t.Errorf("a publish after: %+v", m)
	}
	unversioned("d")
	e, err = s.PrepareData(dw, up, store.Options{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Save(e); err != nil {
		t.Fatal(err)
	}
	if m := s.ReadMeta("d"); m.UpdatedAt == long || m.Versions[0].At != m.UpdatedAt {
		t.Errorf("replaced, of no versions: %+v", m)
	}
}

// names lists every name under dir, by its slash-separated path.
func names(t *testing.T, dir string) []string {
	t.Helper()

	var got []string
	err := filepath.WalkDir(dir, func(p string, _ os.DirEntry, err error) error {
		rel, _ := filepath.Rel(dir, p) //nolint:errcheck // under dir
		got = append(got, filepath.ToSlash(rel))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// A version before that cannot be written leaves the page as it was: no new copy, its meta.json the same.
func TestSaveVersionFails(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := picKit(t, "", "")
	publish(t, s, dw, "d.yaml", titled("A"), map[string][]byte{"a.png": png}, store.Options{})
	dir := filepath.Join(root, "d")
	if err := os.WriteFile(filepath.Join(dir, "versions"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	try := func(name string) {
		t.Helper()

		before, meta := names(t, dir), read(t, filepath.Join(dir, "meta.json"))
		up := store.Upload{File: "d.yaml", Source: []byte(titled(name)), Images: map[string][]byte{"a.png": png}, Project: "p"}
		e, err := s.PrepareData(dw, up, store.Options{})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Save(e); err == nil {
			t.Errorf("%s: saved", name)
		}
		if after := names(t, dir); !slices.Equal(after, before) {
			t.Errorf("%s: %v, before %v", name, after, before)
		}
		if got := read(t, filepath.Join(dir, "meta.json")); got != meta {
			t.Errorf("%s: meta.json %s", name, got)
		}
	}
	try("versions is a file")
	if os.Geteuid() == 0 {
		t.Skip("root writes where it may not")
	}
	if err := os.Remove(filepath.Join(dir, "versions")); err != nil {
		t.Fatal(err)
	}
	publish(t, s, dw, "d.yaml", titled("B"), map[string][]byte{"a.png": png}, store.Options{})
	if err := os.Chmod(filepath.Join(dir, "versions"), 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "versions"), 0o700) }) //nolint:errcheck // told already
	try("versions cannot be written")
}

// An edit keeps the versions, and a rename takes them with the page.
func TestEditKeepsVersions(t *testing.T) {
	t.Parallel()

	s, _ := newStore(t)
	dw := picKit(t, "", "")
	publish(t, s, dw, "d.yaml", titled("A"), map[string][]byte{"a.png": png}, store.Options{})
	publish(t, s, dw, "d.yaml", titled("B"), map[string][]byte{"a.png": png}, store.Options{})
	before, first, last := s.ReadMeta("d").Versions, page(t, s, "d", 1), page(t, s, "d", 2)
	if _, err := s.Edit("d", store.Edit{Title: ptr("T"), Archived: ptr(true)}); err != nil {
		t.Fatal(err)
	}
	if got := s.ReadMeta("d").Versions; !slices.Equal(got, before) {
		t.Errorf("edited: versions %v, before %v", got, before)
	}
	if _, err := s.Edit("d", store.Edit{Slug: ptr("e")}); err != nil {
		t.Fatal(err)
	}
	if got := s.ReadMeta("e").Versions; !slices.Equal(got, before) || page(t, s, "e", 1) != first || page(t, s, "e", 2) != last {
		t.Errorf("renamed: versions %v, before %v", got, before)
	}
}

// Only a number meta.json lists is a version, whatever versions/ holds; the last is the copy's page. The list carries
// no versions.
func TestVersionPage(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := picKit(t, "", "")
	publish(t, s, dw, "d.yaml", titled("A"), map[string][]byte{"a.png": png}, store.Options{})
	publish(t, s, dw, "d.yaml", titled("B"), map[string][]byte{"a.png": png}, store.Options{})
	for _, name := range []string{"7.html.gz", "2.html.gz"} {
		if err := os.WriteFile(filepath.Join(root, "d", "versions", name), []byte("\x1f\x8bnot"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	m := s.ReadMeta("d")
	for _, n := range []int{0, -1, 3, 7} {
		if b, ok := s.VersionPage("d", m, n); ok || s.VersionMark("d", m, n) != "" {
			t.Errorf("version %d: %q", n, b)
		}
	}
	if got := page(t, s, "d", 2); got != read(t, filepath.Join(copyOf(t, s, root, "d"), "page.html")) {
		t.Errorf("the last is not the copy's page:\n%s", got)
	}
	if got, want := s.VersionMark("d", m, 1), "v1."+m.Versions[0].At; got != want {
		t.Errorf("mark %q, want %q", got, want)
	}
	if got, want := s.VersionMark("d", m, 2), "v2."+m.Versions[1].At+"."+m.Source; got != want {
		t.Errorf("mark %q, want %q", got, want)
	}
	home := filepath.Dir(root)
	html := filepath.Join(home, "proj", "z.html")
	if err := os.MkdirAll(filepath.Dir(html), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(html, []byte("<title>z</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	e, err := s.PrepareLink(t.Context(), dw, html, store.Options{}, "", home)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Save(e); err != nil {
		t.Fatal(err)
	}
	link := s.ReadMeta("z")
	link.Versions = m.Versions
	if b, ok := s.VersionPage("z", link, 1); ok {
		t.Errorf("a link's version: %q", b)
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range list {
		if l.Versions != nil {
			t.Errorf("%s listed with versions %v", l.Slug, l.Versions)
		}
	}
}

// A publish over files of floe's own that cannot be read loses only what floe can no longer serve: a last version
// whose page is gone, or, without meta.json, every version.
func TestSaveUnreadable(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := picKit(t, "", "")
	publish(t, s, dw, "d.yaml", titled("A"), map[string][]byte{"a.png": png}, store.Options{})
	publish(t, s, dw, "d.yaml", titled("B"), map[string][]byte{"a.png": png}, store.Options{})
	first := page(t, s, "d", 1)
	if err := os.Remove(filepath.Join(copyOf(t, s, root, "d"), "page.html")); err != nil {
		t.Fatal(err)
	}
	publish(t, s, dw, "d.yaml", titled("C"), map[string][]byte{"a.png": png}, store.Options{})
	if got := numbers(t, s); !slices.Equal(got, []int{1, 3}) || page(t, s, "d", 1) != first {
		t.Errorf("of a page gone: versions %v", got)
	}
	if got := gzips(t, root); !slices.Equal(got, []string{"1.html.gz"}) {
		t.Errorf("of a page gone: versions/ %v", got)
	}
	if err := os.WriteFile(filepath.Join(root, "d", "meta.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	publish(t, s, dw, "d.yaml", titled("D"), map[string][]byte{"a.png": png}, store.Options{})
	if got := numbers(t, s); !slices.Equal(got, []int{1}) {
		t.Errorf("of meta.json broken: versions %v", got)
	}
	if got := gzips(t, root); len(got) != 0 {
		t.Errorf("of meta.json broken: versions/ %v", got)
	}
}
