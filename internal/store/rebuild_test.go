package store_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/store"
)

// A page drawn again the same keeps its copy; one drawn otherwise gets a new copy, every image of the old one
// in it, read or not, its updatedAt as it was. Drawn again once more, nothing changes. A page of no image draws
// again too.
func TestRebuild(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := picKit(t, "", "")
	publish(t, s, dw, "d.yaml", "lang: en\ntemplate: pic\ni: [a.png]\n", map[string][]byte{"a.png": png}, store.Options{})
	publish(t, s, dw, "plain.yaml", "lang: en\ntemplate: pic\n", nil, store.Options{})
	// An image a template no longer reads, kept by hand under the copy, stays with it.
	if err := os.WriteFile(filepath.Join(copyOf(t, s, root, "d"), "images", "old.png"), png, 0o600); err != nil {
		t.Fatal(err)
	}
	before := *s.ReadMeta("d")
	if err := s.Rebuild(dw); err != nil {
		t.Fatal(told(err, root))
	}
	if m := s.ReadMeta("d"); m.Source != before.Source {
		t.Errorf("drawn the same, its copy changed: %s to %s", before.Source, m.Source)
	}
	v2 := picKit(t, "v2", "")
	if err := s.Rebuild(v2); err != nil {
		t.Fatal(told(err, root))
	}
	after := s.ReadMeta("d")
	if after.Source == before.Source || after.UpdatedAt != before.UpdatedAt {
		t.Errorf("meta.json %+v, before %+v", after, before)
	}
	dir := copyOf(t, s, root, "d")
	if page := read(t, filepath.Join(dir, "page.html")); !strings.Contains(page, "<body>v2") {
		t.Errorf("page.html not drawn again:\n%s", page)
	}
	if got, want := files(t, dir), []string{"data.yaml", "images/a.png", "images/old.png", "page.html"}; !slices.Equal(got, want) {
		t.Errorf("files %v, want %v", got, want)
	}
	if n := entries(t, filepath.Join(root, "d")); n != 2 {
		t.Errorf("%d entries under d, want meta.json and one copy", n)
	}
	if page := read(t, filepath.Join(copyOf(t, s, root, "plain"), "page.html")); !strings.Contains(page, "<body>v2") {
		t.Errorf("a page of no image not drawn again:\n%s", page)
	}
	again := s.ReadMeta("d").Source
	if err := s.Rebuild(v2); err != nil || s.ReadMeta("d").Source != again {
		t.Errorf("a second rebuild changed the copy: %v", err)
	}
}

// A page that no longer draws keeps the page it had, its mistakes told, then store_rebuild_kept at its copy; the
// other pages draw again. A theme that is none is told once, a page's mistakes without a place told once.
func TestRebuildKept(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := picKit(t, "", "")
	publish(t, s, dw, "a.yaml", "lang: en\ntemplate: pic\n", nil, store.Options{})
	publish(t, s, dw, "b.yaml", "lang: en\ntemplate: pic\nneed: 1\ni: []\n", nil, store.Options{})
	needs := picKit(t, "v2", `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "required": ["need"]}`)
	pageA := read(t, filepath.Join(copyOf(t, s, root, "a"), "page.html"))
	err := s.Rebuild(needs)
	copyA := strings.Replace(copyOf(t, s, root, "a"), root, "S", 1)
	want := lines(
		copyA+"/data.yaml:1:1: missing property 'need'",
		copyA+`/data.yaml: page "a" keeps the page it had: fix the data file it was published from and publish it again, or fix the template`,
	)
	if got := told(err, root); got != want {
		t.Errorf("told\n%s\nwant\n%s", got, want)
	}
	if got := read(t, filepath.Join(copyOf(t, s, root, "a"), "page.html")); got != pageA {
		t.Error("the page that does not draw changed")
	}
	if page := read(t, filepath.Join(copyOf(t, s, root, "b"), "page.html")); !strings.Contains(page, "<body>v2") {
		t.Error("the page that draws was not drawn again")
	}

	for _, slug := range []string{"a", "b"} {
		m := s.ReadMeta(slug)
		m.Theme = "gone"
		if err = s.WriteMeta(slug, m); err != nil {
			t.Fatal(err)
		}
	}
	err = s.Rebuild(dw)
	got := told(err, root)
	if strings.Count(got, `there is no theme "gone"`) != 1 || strings.Count(got, "keeps the page it had") != 2 {
		t.Errorf("told\n%s", got)
	}
}

// A copy whose page cannot be read (as a store develop wrote, of no page.html), or whose images cannot be, keeps
// its page, told at what cannot be read.
func TestRebuildUnreadable(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root reads what it may not")
	}
	s, root := newStore(t)
	dw := picKit(t, "", "")
	publish(t, s, dw, "a.yaml", "lang: en\ntemplate: pic\n", nil, store.Options{})
	publish(t, s, dw, "b.yaml", "lang: en\ntemplate: pic\ni: [x.png]\n", map[string][]byte{"x.png": png}, store.Options{})
	if err := os.Remove(filepath.Join(copyOf(t, s, root, "a"), "page.html")); err != nil {
		t.Fatal(err)
	}
	images := filepath.Join(copyOf(t, s, root, "b"), "images")
	if err := os.Chmod(images, 0o311); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(images, 0o700) }) //nolint:errcheck // the test is told already
	copyA := strings.Replace(copyOf(t, s, root, "a"), root, "S", 1)
	copyB := strings.Replace(copyOf(t, s, root, "b"), root, "S", 1)
	want := lines(
		copyA+"/page.html: no such file or directory",
		copyA+`/data.yaml: page "a" keeps the page it had: fix the data file it was published from and publish it again, or fix the template`,
		copyB+"/images: permission denied",
		copyB+`/data.yaml: page "b" keeps the page it had: fix the data file it was published from and publish it again, or fix the template`,
	)
	if got := told(s.Rebuild(picKit(t, "v2", "")), root); got != want {
		t.Errorf("told\n%s\nwant\n%s", got, want)
	}
}

// What cannot be written is told at it: a save stops, a rebuild keeps the page and draws the next.
func TestCannotWrite(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root writes where it may not")
	}
	s, root := newStore(t)
	dw := picKit(t, "", "")
	publish(t, s, dw, "a.yaml", "lang: en\ntemplate: pic\n", nil, store.Options{})
	publish(t, s, dw, "b.yaml", "lang: en\ntemplate: pic\n", nil, store.Options{})
	dir := filepath.Join(root, "a")
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:errcheck // the test is told already
	e, err := s.PrepareData(dw, store.Upload{File: "a.yaml", Source: []byte("lang: en\ntemplate: pic\n"), Images: nil, Project: "p"}, store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	want := "S/a: cannot be written (permission denied): check that its directory exists and can be written to, and that its name is free and not too long"
	if got := told(s.Save(e), root); got != want {
		t.Errorf("Save: told\n%s\nwant\n%s", got, want)
	}
	err = s.Rebuild(picKit(t, "v2", ""))
	if got := told(err, root); !strings.HasPrefix(got, want+"\n") || !strings.Contains(got, `page "a" keeps`) {
		t.Errorf("Rebuild: told\n%s", got)
	}
	if page := read(t, filepath.Join(copyOf(t, s, root, "b"), "page.html")); !strings.Contains(page, "<body>v2") {
		t.Error("the page after was not drawn again")
	}
}

func lines(l ...string) string { return strings.Join(l, "\n") }
