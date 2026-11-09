package store_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/sukekyo26/floe/internal/store"
)

const (
	viewer = "local"
	unset  = "S/reads.json: is missing, broken or has no since: run floe stop, then floe serve, which writes it anew"
)

// hour is a time of a day, by its hour.
func hour(h int) string {
	return time.Date(2026, 11, 4, h, 0, 0, 0, time.UTC).Format("2006-01-02T15:04:05.000Z")
}

// made writes the meta.json of a stored page made at the hour created, of the versions, their hours by number from 1.
func made(t *testing.T, s *store.Store, root, slug string, created int, versions ...int) *store.Meta {
	t.Helper()

	m := &store.Meta{Source: "source-1", CreatedAt: hour(created), UpdatedAt: hour(created)}
	for i, h := range versions {
		m.Versions = append(m.Versions, store.Version{N: i + 1, At: hour(h), Mark: "m"})
	}
	if err := os.MkdirAll(filepath.Join(root, slug), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := s.WriteMeta(slug, m); err != nil {
		t.Fatal(err)
	}
	return m
}

// writeReads writes reads.json as text.
func writeReads(t *testing.T, root, text string) {
	t.Helper()

	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "reads.json"), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// readsOf is reads.json as it is kept.
type readsOf map[string]struct {
	Since string `json:"since"`
	Pages map[string]struct {
		N      int    `json:"n"`
		At     string `json:"at"`
		Before *struct {
			N  int    `json:"n"`
			At string `json:"at"`
		} `json:"before"`
	} `json:"pages"`
}

// markRead reads the version n, at, of the page d.
func markRead(s *store.Store, n int, at string) error {
	_, err := s.MarkRead(viewer, "d", n, at)
	return err //nolint:wrapcheck // for the test to look at
}

func keptReads(t *testing.T, root string) readsOf {
	t.Helper()

	var got readsOf
	if err := json.Unmarshal([]byte(read(t, filepath.Join(root, "reads.json"))), &got); err != nil {
		t.Fatal(err)
	}
	return got
}

// listedOf is the entry of slug in the list.
func listedOf(t *testing.T, s *store.Store, slug string) store.Listed {
	t.Helper()

	list, err := s.List(viewer)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range list {
		if l.Slug == slug {
			return l
		}
	}
	t.Fatalf("%s is not listed: %+v", slug, list)
	return store.Listed{}
}

// begun is a store whose viewer started at since.
func begun(t *testing.T, since string) (*store.Store, string) {
	t.Helper()

	s, root := newStore(t)
	if err := s.Begin(viewer, since); err != nil {
		t.Fatal(err)
	}
	return s, root
}

// What a viewer has not seen of a stored page, its since at 10:00: by the record that is of one of its versions, else
// by whether the page came after since, else by the last version made by then.
func TestNews(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		created  int
		versions []int
		rec      string // the record of reads.json, "" for none
		want     string
	}{
		{"the last read", 11, []int{11, 12}, `{"n":2,"at":"` + hour(12) + `"}`, ""},
		{"an earlier read", 11, []int{11, 12}, `{"n":1,"at":"` + hour(11) + `"}`, "version"},
		{"of no version, after since", 11, []int{11}, `{"n":7,"at":"` + hour(11) + `"}`, "new"},
		{"of its number, another time, as after a removal", 11, []int{11}, `{"n":1,"at":"` + hour(9) + `"}`, "new"},
		{"of no version, before since", 8, []int{8}, `{"n":7,"at":"` + hour(8) + `"}`, ""},
		{"no record, after since", 11, []int{11, 12}, "", "new"},
		{"no record, before since, last by then", 8, []int{8, 9}, "", ""},
		{"no record, made at since", 10, []int{10}, "", ""},
		{"no record, before since, one after", 8, []int{8, 9, 11}, "", "version"},
		{"no record, before since, every version after", 8, []int{11, 12}, "", "version"},
		{"no versions", 11, nil, "", ""},
	} {
		s, root := newStore(t)
		made(t, s, root, "d", tt.created, tt.versions...)
		pages := ""
		if tt.rec != "" {
			pages = `,"pages":{"d":` + tt.rec + `}`
		}
		writeReads(t, root, `{"local":{"since":"`+hour(10)+`"`+pages+`}}`)
		if got := listedOf(t, s, "d"); got.News != tt.want {
			t.Errorf("%s: news %q, want %q", tt.name, got.News, tt.want)
		}
	}
	// As times: 19:00 at +09:00 is 10:00, since itself, later as text.
	s, root := newStore(t)
	m := made(t, s, root, "d", 8)
	m.CreatedAt = "2026-11-04T19:00:00+09:00"
	m.Versions = []store.Version{{N: 1, At: m.CreatedAt, Mark: "m"}}
	if err := s.WriteMeta("d", m); err != nil {
		t.Fatal(err)
	}
	writeReads(t, root, `{"local":{"since":"`+hour(10)+`"}}`)
	if got := listedOf(t, s, "d"); got.News != "" || got.Version != 1 {
		t.Errorf("as times: %+v", got)
	}
}

// Read once, a page goes on being read in the versions kept, and new when the one read is dropped; the page replaced by
// another project is new; a link and a folder have no news, nor a number.
func TestNewsPublished(t *testing.T) {
	t.Parallel()

	s, root := begun(t, hour(0))
	dw := picKit(t, "", "")
	images := map[string][]byte{"a.png": png}
	publish(t, s, dw, "d.yaml", titled("t0"), images, store.Options{})
	if got := listedOf(t, s, "d"); got.News != "new" || got.Version != 1 {
		t.Errorf("published: %+v", got)
	}
	first := s.ReadMeta("d").Versions[0]
	if _, err := s.MarkRead(viewer, "d", 1, first.At); err != nil {
		t.Fatal(err)
	}
	if got := listedOf(t, s, "d"); got.News != "" {
		t.Errorf("read: %+v", got)
	}
	publish(t, s, dw, "d.yaml", titled("t0"), images, store.Options{})
	if got := listedOf(t, s, "d"); got.News != "" {
		t.Errorf("the same content: %+v", got)
	}
	for i := 1; i <= 19; i++ {
		publish(t, s, dw, "d.yaml", titled("t"+strconv.Itoa(i)), images, store.Options{})
	}
	if got := listedOf(t, s, "d"); got.News != "version" || got.Version != 20 {
		t.Errorf("19 later: %+v", got)
	}
	publish(t, s, dw, "d.yaml", titled("t20"), images, store.Options{})
	if got := listedOf(t, s, "d"); got.News != "new" || got.Version != 21 {
		t.Errorf("the one read dropped: %+v", got)
	}

	publish(t, s, dw, "r.yaml", titled("r"), images, store.Options{})
	rm := s.ReadMeta("r")
	if _, err := s.MarkRead(viewer, "r", 1, rm.Versions[0].At); err != nil {
		t.Fatal(err)
	}
	up := store.Upload{File: "r.yaml", Source: []byte(titled("r")), Images: images, Project: "q", Repo: "q", Place: "r.yaml"}
	e, err := s.PrepareData(dw, up, store.Options{Force: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Save(e); err != nil {
		t.Fatal(err)
	}
	if got := listedOf(t, s, "r"); got.News != "new" {
		t.Errorf("replaced: %+v", got)
	}

	home := filepath.Dir(root)
	for name, text := range map[string]string{"proj/h.html": "<title>h</title>", "proj/dir/a.html": "<title>a</title>"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(home, name)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, link := range []string{"h.html", "dir"} {
		e, err := s.PrepareLink(t.Context(), dw, filepath.Join(home, "proj", link), store.Options{Slug: ptr("l-" + strings.TrimSuffix(link, ".html"))}, "", home)
		if err != nil {
			t.Fatal(told(err, root))
		}
		if err = s.Save(e); err != nil {
			t.Fatal(err)
		}
		if got := listedOf(t, s, e.Slug); got.News != "" || got.Version != 0 {
			t.Errorf("%s: %+v", link, got)
		}
	}
}

// Begin writes since when it is not there, and only then; a reads.json that cannot be read goes aside. While the
// server runs, nothing else writes since: without one, every page counts as read and MarkRead refuses.
func TestReadsBegin(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	if err := s.Begin(viewer, hour(1)); err != nil {
		t.Fatal("a store not made yet: ", err)
	}
	if got := keptReads(t, root)[viewer].Since; got != hour(1) {
		t.Errorf("since %s", got)
	}
	m := made(t, s, root, "d", 2, 2)
	if _, err := s.List(viewer); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MarkRead(viewer, "d", 1, m.Versions[0].At); err != nil {
		t.Fatal(err)
	}
	if err := s.Begin(viewer, hour(3)); err != nil {
		t.Fatal(err)
	}
	if got := keptReads(t, root)[viewer]; got.Since != hour(1) || got.Pages["d"].N != 1 {
		t.Errorf("after the list, a mark and a second start: %+v", got)
	}

	// No since of the viewer: written beside what is there.
	for _, text := range []string{
		`{}`, `null`, `{"local":{}}`,
		`{"other":{"since":"` + hour(5) + `"},"local":{"pages":{"d":{"n":1,"at":"x"}}}}`,
	} {
		fresh, dir := newStore(t)
		writeReads(t, dir, text)
		if err := fresh.Begin(viewer, hour(1)); err != nil {
			t.Fatal(err)
		}
		got := keptReads(t, dir)
		if got[viewer].Since != hour(1) || strings.Contains(text, "other") && (got["other"].Since != hour(5) || got[viewer].Pages["d"].N != 1) {
			t.Errorf("%s: %+v", text, got)
		}
		if _, err := os.Stat(filepath.Join(dir, "reads.json.broken")); err == nil {
			t.Errorf("%s went aside", text)
		}
	}

	// Broken: aside, the one there replaced, a new one written.
	for _, text := range []string{`{`, `[]`, `{"local":{"since":"soon"}}`, `{"local":{"since":3}}`} {
		fresh, dir := newStore(t)
		writeReads(t, dir, text)
		if err := os.WriteFile(filepath.Join(dir, "reads.json.broken"), []byte("older"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := fresh.Begin(viewer, hour(1)); err != nil {
			t.Fatal(err)
		}
		if got := read(t, filepath.Join(dir, "reads.json.broken")); got != text {
			t.Errorf("aside %q, want %q", got, text)
		}
		if got := keptReads(t, dir)[viewer]; got.Since != hour(1) || got.Pages != nil {
			t.Errorf("%s: %+v", text, got)
		}
	}

	// A directory that cannot go aside is told at where it goes.
	s, root = newStore(t)
	if err := os.MkdirAll(filepath.Join(root, "reads.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "reads.json.broken", "x"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := told(s.Begin(viewer, hour(1)), root); !strings.HasPrefix(got, "S/reads.json.broken: cannot be written (") {
		t.Errorf("told %s", got)
	}

	// While it runs: gone, broken, or no since.
	for _, text := range []string{"", `{`, `{}`, `{"local":{"pages":{}}}`, `{"other":{"since":"` + hour(1) + `"}}`} {
		s, root := begun(t, hour(1))
		m := made(t, s, root, "d", 2, 2)
		if text == "" {
			if err := os.Remove(filepath.Join(root, "reads.json")); err != nil {
				t.Fatal(err)
			}
		} else {
			writeReads(t, root, text)
		}
		if got := listedOf(t, s, "d"); got.News != "" {
			t.Errorf("%q: news %q", text, got.News)
		}
		if got := told(markRead(s, 1, m.Versions[0].At), root); got != unset {
			t.Errorf("%q: told %s", text, got)
		}
		_, err := os.Stat(filepath.Join(root, "reads.json"))
		if text == "" && err == nil {
			t.Error("gone: written again")
		}
		if text != "" && read(t, filepath.Join(root, "reads.json")) != text {
			t.Errorf("%q: written", text)
		}
	}
}

// A reads.json that cannot be read is told in the system's words, not as one missing; the list counts every page as read.
func TestReadsUnreadable(t *testing.T) {
	t.Parallel()

	s, root := begun(t, hour(1))
	m := made(t, s, root, "d", 2, 2)
	file := filepath.Join(root, "reads.json")
	if os.Geteuid() != 0 { // root reads what a mode keeps
		if err := os.Chmod(file, 0); err != nil {
			t.Fatal(err)
		}
		if got := told(markRead(s, 1, m.Versions[0].At), root); got != "S/reads.json: permission denied" {
			t.Errorf("told %s", got)
		}
		if got := listedOf(t, s, "d"); got.News != "" {
			t.Errorf("news %q", got.News)
		}
		if err := os.Chmod(file, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(file, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := told(markRead(s, 1, m.Versions[0].At), root); !strings.HasPrefix(got, "S/reads.json: ") || got == unset {
		t.Errorf("a directory: told %s", got)
	}
}

// holdLock holds the lock of the store until the test ends, as another process would.
func holdLock(t *testing.T, root string) {
	t.Helper()

	f, err := os.OpenFile(filepath.Join(root, ".lock"), os.O_RDONLY|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		t.Fatal(err)
	}
}

// A mark is kept only when it is of a version of the page after the one taken as read, and takes no lock when it is
// not.
func TestMarkReadNewer(t *testing.T) {
	t.Parallel()

	s, root := begun(t, hour(10))
	m := made(t, s, root, "d", 11, 11, 12, 13)
	v := func(n int) string { return m.Versions[n-1].At }
	mark := func(n int, at string) {
		t.Helper()

		if _, err := s.MarkRead(viewer, "d", n, at); err != nil {
			t.Fatal(err)
		}
	}
	// Not of a version, its number or its time another's.
	mark(0, "")
	mark(1, v(2))
	mark(7, v(1))
	if got := keptReads(t, root)[viewer].Pages; len(got) != 0 {
		t.Errorf("of no version: %+v", got)
	}
	// An earlier version of a page never read is kept: the later are still new to the reader.
	mark(2, v(2))
	if got := listedOf(t, s, "d"); got.News != "version" {
		t.Errorf("an earlier version: %+v", got)
	}
	mark(3, v(3))
	mark(2, v(2))
	if got := keptReads(t, root)[viewer].Pages["d"]; got.N != 3 {
		t.Errorf("an earlier one after the last: %+v", got)
	}

	// Of a page from before since, the last by then taken as read: nothing earlier is written.
	old := made(t, s, root, "old", 8, 8, 9, 9)
	if _, err := s.MarkRead(viewer, "old", 1, old.Versions[0].At); err != nil {
		t.Fatal(err)
	}
	if _, kept := keptReads(t, root)[viewer].Pages["old"]; kept || listedOf(t, s, "old").News != "" {
		t.Errorf("before since: %+v", keptReads(t, root)[viewer].Pages)
	}

	// A number again, of a page made again: written.
	if err := s.Remove("d"); err != nil {
		t.Fatal(err)
	}
	again := made(t, s, root, "d", 14, 14)
	if _, err := s.MarkRead(viewer, "d", 1, again.Versions[0].At); err != nil {
		t.Fatal(err)
	}
	if got := keptReads(t, root)[viewer].Pages["d"]; got.N != 1 || got.At != again.Versions[0].At {
		t.Errorf("made again: %+v", got)
	}

	if _, err := s.MarkRead(viewer, "none", 1, "x"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("no page: %v", err)
	}

	// None to write: no lock, which another holds.
	holdLock(t, root)
	done := make(chan error, 1)
	go func() {
		err := markRead(s, 1, again.Versions[0].At)
		if err == nil {
			err = markRead(s, 9, "x")
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(10 * time.Second):
		t.Error("a mark that changes nothing waited for the lock")
	}
}

// What was read follows the slug in a rename and goes with the page, and the key of a page gone goes when another takes
// its slug. A removal that stops halfway keeps it. A slug that starts the same is another.
func TestReadsFollow(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	writeReads(t, root, `{"local":{"since":"`+hour(1)+`","pages":{"pv":{"n":1,"at":"a"},"pv-old":{"n":2,"at":"b"},"gone":{"n":3,"at":"c"},"left":{"n":4,"at":"d"}}}}`)
	made(t, s, root, "pv", 2, 2)
	made(t, s, root, "pv-old", 2, 2)
	pages := func() []string {
		t.Helper()

		kept := keptReads(t, root)[viewer].Pages
		got := make([]string, 0, len(kept))
		for slug, r := range kept {
			got = append(got, slug+"="+strconv.Itoa(r.N))
		}
		slices.Sort(got)
		return got
	}
	check := func(what string, want ...string) {
		t.Helper()

		if got := pages(); !slices.Equal(got, want) {
			t.Errorf("%s: %v, want %v", what, got, want)
		}
	}
	if _, err := s.Edit("pv", store.Edit{Title: ptr("t")}); err != nil {
		t.Fatal(err)
	}
	check("not renamed", "gone=3", "left=4", "pv-old=2", "pv=1")
	if _, err := s.Edit("pv", store.Edit{Slug: ptr("pv2")}); err != nil {
		t.Fatal(err)
	}
	check("renamed", "gone=3", "left=4", "pv-old=2", "pv2=1")
	if _, err := s.Edit("pv2", store.Edit{Slug: ptr("gone")}); err != nil {
		t.Fatal(err)
	}
	check("onto the key of a page gone", "gone=1", "left=4", "pv-old=2")

	made(t, s, root, "fresh", 2, 2)
	if _, err := s.Edit("fresh", store.Edit{Slug: ptr("left")}); err != nil {
		t.Fatal(err)
	}
	check("none to move, onto the key of a page gone", "gone=1", "pv-old=2")

	if os.Geteuid() != 0 { // root removes what a mode keeps
		dir := filepath.Join(root, "gone")
		if err := os.MkdirAll(filepath.Join(dir, "x"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, 0o500); err != nil {
			t.Fatal(err)
		}
		if err := s.Remove("gone"); err == nil {
			t.Error("removed all the same")
		}
		check("a removal that stopped", "gone=1", "pv-old=2")
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Remove("gone"); err != nil {
		t.Fatal(err)
	}
	check("removed", "pv-old=2")

	// reads.json that cannot be read: the rename and the removal are done all the same.
	writeReads(t, root, `{`)
	if to, err := s.Edit("pv-old", store.Edit{Slug: ptr("pv3")}); err != nil || to.Slug != "pv3" {
		t.Errorf("renamed to %s: %v", to.Slug, err)
	}
	if err := s.Remove("pv3"); err != nil {
		t.Error(err)
	}
}

// A stored page's updatedAt moves when a version is made alone: not for the same content, nor for a theme.
func TestUpdatedAtMovesWithVersions(t *testing.T) {
	t.Parallel()

	s, _ := newStore(t)
	dw := picKit(t, "", "")
	images := map[string][]byte{"a.png": png}
	publish(t, s, dw, "d.yaml", titled("A"), images, store.Options{})
	const long = "2026-01-02T03:04:05.000Z"
	m := s.ReadMeta("d")
	m.UpdatedAt = long
	if err := s.WriteMeta("d", m); err != nil {
		t.Fatal(err)
	}
	publish(t, s, dw, "d.yaml", titled("A"), images, store.Options{})
	if got := s.ReadMeta("d").UpdatedAt; got != long {
		t.Errorf("the same content: %s", got)
	}
	publish(t, s, dw, "d.yaml", titled("A"), images, store.Options{Theme: ptr("ink")})
	if got := s.ReadMeta("d"); got.UpdatedAt != long || got.Theme != "ink" {
		t.Errorf("a theme: %+v", got)
	}
	publish(t, s, dw, "d.yaml", titled("B"), images, store.Options{})
	if got := s.ReadMeta("d"); got.UpdatedAt == long || got.Versions[1].At != got.UpdatedAt {
		t.Errorf("other content: %+v", got)
	}
}

// A file's link is listed at the later of the file's time and when it was linked; a folder's at when it was linked.
func TestListTimes(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	home := filepath.Dir(root)
	folder := filepath.Join(home, "proj", "dir")
	if err := os.MkdirAll(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(home, "proj", "h.html")
	if err := os.WriteFile(file, []byte("<title>h</title>"), 0o600); err != nil {
		t.Fatal(err)
	}
	dw := picKit(t, "", "")
	for _, at := range []string{file, folder} {
		e, err := s.PrepareLink(t.Context(), dw, at, store.Options{}, "", home)
		if err != nil {
			t.Fatal(told(err, root))
		}
		if err = s.Save(e); err != nil {
			t.Fatal(err)
		}
	}
	timed := func(slug, linked string) {
		t.Helper()

		m := s.ReadMeta(slug)
		m.UpdatedAt = linked
		if err := s.WriteMeta(slug, m); err != nil {
			t.Fatal(err)
		}
	}
	touch := func(h int) {
		t.Helper()

		when := time.Date(2026, 11, 4, h, 0, 0, 0, time.UTC)
		if err := os.Chtimes(file, when, when); err != nil {
			t.Fatal(err)
		}
	}
	timed("h", hour(5))
	touch(3)
	if got := listedOf(t, s, "h").UpdatedAt; got != hour(5) {
		t.Errorf("linked after the file was written: %s", got)
	}
	touch(7)
	if got := listedOf(t, s, "h").UpdatedAt; got != hour(7) {
		t.Errorf("written after it was linked: %s", got)
	}
	timed("dir", hour(5))
	if got := listedOf(t, s, "dir").UpdatedAt; got != hour(5) {
		t.Errorf("a folder: %s", got)
	}
}

// The template is in a stored page's meta.json, by publish and rebuild, drawn or not; a linked data file's is read for
// the list and not kept; an HTML file and a folder have none.
func TestListTemplate(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	home := filepath.Dir(root)
	dw := picKit(t, "", "")
	images := map[string][]byte{"a.png": png}
	publish(t, s, dw, "d.yaml", titled("A"), images, store.Options{})
	if got := s.ReadMeta("d").Template; got != "pic" {
		t.Errorf("published: %q", got)
	}
	// Published again, of the same content: settle builds meta.json anew.
	publish(t, s, dw, "d.yaml", titled("A"), images, store.Options{})
	if got := listedOf(t, s, "d").Template; got != "pic" {
		t.Errorf("published again: %q", got)
	}

	cleared := func() *store.Meta {
		t.Helper()

		m := s.ReadMeta("d")
		m.Template = ""
		if err := s.WriteMeta("d", m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	// Rebuilt to the same page: meta.json alone, the copy, the versions and the time as they were.
	was := cleared()
	if err := s.Rebuild(dw); err != nil {
		t.Fatal(err)
	}
	got := s.ReadMeta("d")
	if got.Template != "pic" || got.Source != was.Source || got.UpdatedAt != was.UpdatedAt || len(got.Versions) != len(was.Versions) {
		t.Errorf("the same page: %+v, was %+v", got, was)
	}
	// Rebuilt to another page.
	was = cleared()
	if err := s.Rebuild(picKit(t, "v2", "")); err != nil {
		t.Fatal(err)
	}
	got = s.ReadMeta("d")
	if got.Template != "pic" || got.Source == was.Source || got.UpdatedAt != was.UpdatedAt {
		t.Errorf("another page: %+v, was %+v", got, was)
	}
	// Not drawn: its mistakes told, the template kept all the same.
	was = cleared()
	needs := picKit(t, "", strings.Replace(picSchema, `"type": "object",`, `"type": "object", "required": ["need"],`, 1))
	if err := s.Rebuild(needs); err == nil {
		t.Error("a page that does not draw is not told")
	}
	got = s.ReadMeta("d")
	if got.Template != "pic" || got.Source != was.Source || got.UpdatedAt != was.UpdatedAt {
		t.Errorf("not drawn: %+v, was %+v", got, was)
	}

	write := func(name, text string) string {
		t.Helper()

		at := filepath.Join(home, "proj", name)
		if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		return at
	}
	folder := filepath.Join(home, "proj", "dir")
	if err := os.MkdirAll(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	data := write("l.yaml", "lang: en\ntemplate: pic\n")
	html := write("h.html", "<title>h</title>")
	for _, at := range []string{data, html, folder} {
		e, err := s.PrepareLink(t.Context(), dw, at, store.Options{}, "", home)
		if err != nil {
			t.Fatal(told(err, root))
		}
		if err = s.Save(e); err != nil {
			t.Fatal(err)
		}
	}
	if got := listedOf(t, s, "l").Template; got != "pic" {
		t.Errorf("a linked data file: %q", got)
	}
	if got := s.ReadMeta("l").Template; got != "" {
		t.Errorf("a linked data file's meta.json: %q", got)
	}
	write("l.yaml", "lang: en\ntemplate: report\ntitle: longer than it was\n")
	if got := listedOf(t, s, "l").Template; got != "report" {
		t.Errorf("a linked data file changed: %q", got)
	}
	for _, slug := range []string{"h", "dir"} {
		if got := listedOf(t, s, slug).Template; got != "" {
			t.Errorf("%s: %q", slug, got)
		}
	}
}

// What reading a page and marking it unread come to, by what was read before: the same look as before it was opened, whatever
// since says of the page, and a second mark goes no further.
func TestMarkUnread(t *testing.T) {
	t.Parallel()

	const since = 10
	const before = 12 // a version after since
	cases := []struct {
		name     string
		created  int
		versions []int
		reads    string // pages of reads.json, as it was
		read     int    // the version read, 0 for none
		cleared  bool
		kept     int // what is kept after the mark
		news     string
	}{
		{name: "new, read once: the dot alone", created: before, versions: []int{before}, read: 1, cleared: true, kept: 0, news: "new"},
		{
			name: "a later version read: back to the one before, with its mark", created: before, versions: []int{before, before, before},
			reads: `"p":{"n":1,"at":"` + hour(before) + `"}`, read: 3, cleared: true, kept: 1, news: "version",
		},
		{name: "before since, a version after: back to the one taken as read", created: 9, versions: []int{9, 9, before}, read: 3, cleared: true, kept: 2, news: "version"},
		{name: "before since, all taken as read: the dot alone", created: 9, versions: []int{9, 9}, kept: 0, news: "new"},
		{
			name: "kept from before this was: the dot alone", created: before, versions: []int{before},
			reads: `"p":{"n":1,"at":"` + hour(before) + `"}`, kept: 0, news: "new",
		},
		{
			name: "the one before gone from the versions: the dot alone", created: before, versions: []int{before, before},
			reads: `"p":{"n":2,"at":"` + hour(before) + `","before":{"n":9,"at":"gone"}}`, kept: 0, news: "new",
		},
		{
			name: "a record of no version, its before still one, on a page taken as read: the dot alone", created: 9, versions: []int{9},
			reads: `"p":{"n":7,"at":"gone","before":{"n":1,"at":"` + hour(9) + `"}}`, kept: 0, news: "new",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			s, root := newStore(t)
			writeReads(t, root, `{"local":{"since":"`+hour(since)+`","pages":{`+c.reads+`}}}`)
			m := made(t, s, root, "p", c.created, c.versions...)
			if c.read > 0 {
				cleared, err := s.MarkRead(viewer, "p", c.read, m.Versions[c.read-1].At)
				if err != nil || cleared != c.cleared {
					t.Fatalf("read: %v, %v", cleared, err)
				}
			}
			if got := listedOf(t, s, "p").News; got != "" {
				t.Fatalf("read, news %q", got)
			}
			for range 2 {
				if err := s.MarkUnread(viewer, "p"); err != nil {
					t.Fatal(err)
				}
				rec := keptReads(t, root)[viewer].Pages["p"]
				if got := listedOf(t, s, "p").News; rec.N != c.kept || rec.Before != nil || got != c.news {
					t.Errorf("kept %+v, news %q, want %d %q", rec, got, c.kept, c.news)
				}
			}
			// Opened again, read, and the mark again comes to the same.
			last := m.Versions[len(m.Versions)-1]
			cleared, err := s.MarkRead(viewer, "p", last.N, last.At)
			if err != nil || !cleared || listedOf(t, s, "p").News != "" {
				t.Errorf("read again: %v, %v, news %q", cleared, err, listedOf(t, s, "p").News)
			}
			if err := s.MarkUnread(viewer, "p"); err != nil {
				t.Fatal(err)
			}
			if rec := keptReads(t, root)[viewer].Pages["p"]; rec.N != c.kept {
				t.Errorf("marked again, kept %+v, want %d", rec, c.kept)
			}
		})
	}

	// The errors are those of a read.
	s, root := newStore(t)
	if err := s.MarkUnread(viewer, "none"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("no page: %v", err)
	}
	made(t, s, root, "p", 1, 1)
	if err := s.MarkUnread(viewer, "p"); told(err, root) != "S/reads.json: is missing, broken or has no since: run floe stop, then floe serve, which writes it anew" {
		t.Errorf("no since: %v", told(err, root))
	}
}

// Of a read: whether it took the page up to its last. An earlier version, a read again and no such version are not that.
func TestMarkReadCleared(t *testing.T) {
	t.Parallel()

	s, root := begun(t, hour(1))
	m := made(t, s, root, "p", 2, 2, 3)
	for _, c := range []struct {
		name string
		n    int
		at   string
		want bool
	}{
		{"no such version", 9, "x", false},
		{"marked unread is no version", 0, "", false},
		{"an earlier version", 1, m.Versions[0].At, false},
		{"the last", 2, m.Versions[1].At, true},
		{"the last again", 2, m.Versions[1].At, false},
	} {
		if got, err := s.MarkRead(viewer, "p", c.n, c.at); err != nil || got != c.want {
			t.Errorf("%s: %v, %v", c.name, got, err)
		}
	}
	if rec := keptReads(t, root)[viewer].Pages["p"]; rec.N != 2 || rec.Before == nil || rec.Before.N != 1 {
		t.Errorf("kept %+v", rec)
	}
}

// A page marked unread stays so through a publish of the same content and a rebuild, a new version and one that replaces it, and
// follows the page to its new slug, with what it held; a record left by a replacing publish is of no version.
func TestMarkUnreadKept(t *testing.T) {
	t.Parallel()

	s, root := begun(t, hour(0))
	dw := picKit(t, "", "")
	images := map[string][]byte{"a.png": png}
	publish(t, s, dw, "d.yaml", titled("t0"), images, store.Options{})
	first := s.ReadMeta("d").Versions[0]
	if _, err := s.MarkRead(viewer, "d", 1, first.At); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkUnread(viewer, "d"); err != nil {
		t.Fatal(err)
	}
	still := func(what string) {
		t.Helper()

		if rec := keptReads(t, root)[viewer].Pages["d"]; rec.N != 0 || listedOf(t, s, "d").News != "new" {
			t.Errorf("%s: kept %+v, news %q", what, rec, listedOf(t, s, "d").News)
		}
	}
	publish(t, s, dw, "d.yaml", titled("t0"), images, store.Options{})
	still("the same content")
	if err := s.Rebuild(dw); err != nil {
		t.Fatal(err)
	}
	still("a rebuild")
	publish(t, s, dw, "d.yaml", titled("t1"), images, store.Options{})
	still("a new version")
	publish(t, s, dw, "other.yaml", titled("t2"), images, store.Options{Slug: ptr("d"), Force: true})
	still("replaced")
	if _, err := s.Edit("d", store.Edit{Slug: ptr("d2")}); err != nil {
		t.Fatal(err)
	}
	if rec, ok := keptReads(t, root)[viewer].Pages["d2"]; !ok || rec.N != 0 || listedOf(t, s, "d2").News != "new" {
		t.Errorf("moved: kept %+v %v, news %q", rec, ok, listedOf(t, s, "d2").News)
	}
	if _, err := s.Edit("d2", store.Edit{Slug: ptr("d")}); err != nil {
		t.Fatal(err)
	}

	last := s.ReadMeta("d")
	if _, err := s.MarkRead(viewer, "d", last.Versions[len(last.Versions)-1].N, last.Versions[len(last.Versions)-1].At); err != nil {
		t.Fatal(err)
	}
	publish(t, s, dw, "third.yaml", titled("t3"), images, store.Options{Slug: ptr("d"), Force: true})
	if got := listedOf(t, s, "d").News; got == "" {
		t.Errorf("read before a replacing publish: no news")
	}
	if err := s.MarkUnread(viewer, "d"); err != nil {
		t.Fatal(err)
	}
	if rec := keptReads(t, root)[viewer].Pages["d"]; rec.N == 0 {
		t.Errorf("with news already, a mark changes nothing: kept %+v", rec)
	}
}

// ReadBefore is the version read before the last, opening a page reading it at once: the one taken as read, or the one before reading it
// when that is the last; 0 for none, and never the last, so that a comparison from it compares two.
func TestReadBefore(t *testing.T) {
	t.Parallel()

	const since = 12
	for _, tc := range []struct {
		name     string
		versions []int
		record   string
		want     int
	}{
		{"read v2, reading v3", []int{13, 14, 15}, `{"n":3,"at":"H15","before":{"n":2,"at":"H14"}}`, 2},
		{"read v2, v3 not opened", []int{13, 14, 15}, `{"n":2,"at":"H14"}`, 2},
		{"v3 the first read", []int{13, 14, 15}, `{"n":3,"at":"H15"}`, 0},
		{"marked unread", []int{13, 14, 15}, `{"n":0,"at":""}`, 0},
		{"no record, v2 by since", []int{8, 9, 15}, ``, 2},
		{"no record, the last by since", []int{7, 8, 9}, ``, 0},
		{"no record, made after since", []int{13, 14}, ``, 0},
		{"a record of no version, the last by since", []int{7, 8, 9}, `{"n":9,"at":"H9","before":{"n":2,"at":"H8"}}`, 0},
		{"its before dropped", []int{13, 14, 15}, `{"n":3,"at":"H15","before":{"n":1,"at":"H9"}}`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s, root := begun(t, hour(since))
			m := made(t, s, root, "d", tc.versions[0], tc.versions...)
			if tc.record != "" {
				for h := 23; h >= 0; h-- {
					tc.record = strings.ReplaceAll(tc.record, "H"+strconv.Itoa(h), hour(h))
				}
				writeReads(t, root, `{"local":{"since":"`+hour(since)+`","pages":{"d":`+tc.record+`}}}`)
			}
			if got := s.ReadBefore(viewer, "d", m); got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
	// No reader, and a link.
	s, root := begun(t, hour(since))
	m := made(t, s, root, "d", 13, 13, 14)
	if got := s.ReadBefore("nobody", "d", m); got != 0 {
		t.Errorf("no reader: %d", got)
	}
	if got := s.ReadBefore(viewer, "d", &store.Meta{Path: "/x/y.html"}); got != 0 {
		t.Errorf("a link: %d", got)
	}
}
