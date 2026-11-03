package store

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/sukekyo26/floe/internal/kit"
)

// A save that another of the slug comes between, after it gzipped the page before outside the lock, keeps the page of
// the version that is last under the lock: the one in between is kept as its own version, not under the number the
// gzip was made for.
func TestSaveVersionRace(t *testing.T) {
	t.Parallel()

	floe, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	dw := Drawing{Kit: floe, Icons: nil, Err: nil}
	s := New(filepath.Join(t.TempDir(), "store"))
	slug := "d"
	prepare := func(n int) *Entry {
		src := "lang: en\ntemplate: report\ntitle: T" + strconv.Itoa(n) + "\nsections:\n  - heading: H\n    parts:\n      - text: x\n"
		e, err := s.PrepareData(dw, Upload{File: "d.yaml", Source: []byte(src), Images: map[string][]byte{}, Project: "p"}, Options{Slug: &slug})
		if err != nil {
			t.Fatal(err)
		}
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
	if err := s.save(late, mark, pre); err != nil {
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
