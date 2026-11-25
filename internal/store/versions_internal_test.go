package store

import (
	"path/filepath"
	"strconv"
	"testing"
)

// A save that another of the slug comes between, after it gzipped the page before outside the lock, keeps the page of
// the version that is last under the lock: the one in between is kept as its own version, not under the number the
// gzip was made for.
func TestSaveVersionRace(t *testing.T) {
	t.Parallel()

	s := New(filepath.Join(t.TempDir(), "store"))
	slug := "d"
	// package store cannot import publish: the entry is built as publish.PrepareData builds it, its page told apart by n
	// as its data (and so its contentMark) is.
	prepare := func(n int) *Entry {
		src := "lang: en\ntemplate: report\ntitle: T" + strconv.Itoa(n) + "\nsections:\n  - heading: H\n    parts:\n      - text: x\n"
		e, err := s.NewEntry(Seed{Slug: slug, Named: "", File: "d.yaml", Owner: Owner{Path: "", Repo: "p", Place: "d.yaml"}, Folder: false, FolderTitle: "", Data: []byte(src), Options: Options{Slug: &slug}, Project: "p", Created: ""})
		if err != nil {
			t.Fatal(err)
		}
		e.Page = []byte("<p>page " + strconv.Itoa(n) + "</p>")
		e.Images = map[string][]byte{}
		return e
	}
	drawn := map[int]string{}
	for n := 1; n <= 5; n++ {
		e := prepare(n)
		if err := s.Save(e); err != nil {
			t.Fatal(err)
		}
		drawn[n] = string(e.Page)
	}
	late := prepare(7)
	mark := contentMark(late.Source, late.Images)
	pre := s.press(slug, mark)
	if pre == nil || pre.n != 5 {
		t.Fatalf("pressed %+v", pre)
	}
	between := prepare(6)
	if err := s.Save(between); err != nil {
		t.Fatal(err)
	}
	drawn[6] = string(between.Page)
	if err := s.commit(late, mark, pre); err != nil {
		t.Fatal(err)
	}
	m := s.ReadMeta(slug)
	if got := len(m.Versions); got != 7 || m.Versions[6].N != 7 {
		t.Fatalf("versions %+v", m.Versions)
	}
	for n := 5; n <= 6; n++ {
		if b, ok := s.VersionPage(slug, m, n); !ok || string(b) != drawn[n] {
			t.Errorf("version %d is not its page: %v\n%s", n, ok, b)
		}
	}
}
