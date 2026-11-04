package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/sukekyo26/floe/internal/files"
	"github.com/sukekyo26/floe/internal/usererr"
)

const (
	readsName = "reads.json"
	// What Listed.News says of a stored page: never seen, or with a version after the one read.
	newsNew     = "new"
	newsVersion = "version"
)

// errReadsBroken is a since in reads.json that is no time.
var errReadsBroken = errors.New("since is not a time")

// pageRead is the version of a page a viewer read: its number and when it was made, a number coming again after a
// removal.
type pageRead struct {
	N  int    `json:"n"`
	At string `json:"at"`
}

// viewerReads is what reads.json keeps of a viewer: since, when floe first served them, and the pages read, by slug.
type viewerReads struct {
	Since string              `json:"since,omitempty"`
	Pages map[string]pageRead `json:"pages,omitempty"`
}

func (s *Store) readsFile() string {
	return filepath.Join(s.root, readsName)
}

// readReads reads reads.json, by viewer: fs.ErrNotExist for none, and a mistake for one that cannot be read as it. It
// takes no lock: a write replaces the file whole, so that a reader has the one before it or the one after.
func (s *Store) readReads() (map[string]viewerReads, error) {
	b, _, err := files.Read(s.readsFile())
	if err != nil {
		return nil, err //nolint:wrapcheck // for the caller, which tells what it does of it
	}
	var all map[string]viewerReads
	if err := json.Unmarshal(b, &all); err != nil {
		return nil, err //nolint:wrapcheck // as above
	}
	for _, v := range all {
		if _, err := time.Parse(time.RFC3339, v.Since); v.Since != "" && err != nil {
			return nil, errReadsBroken
		}
	}
	return all, nil
}

// writeReads replaces reads.json with all. It is called under the lock.
func (s *Store) writeReads(all map[string]viewerReads) error {
	b, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", readsName, err)
	}
	if err := files.WriteAtomic(s.readsFile(), append(b, '\n'), 0o600, nil); err != nil {
		return usererr.CannotWrite(s.readsFile(), err) //nolint:wrapcheck // the system's words, at the file
	}
	return nil
}

// Begin writes when viewer starts, the time now, unless reads.json has it: since never moves after. A reads.json that
// cannot be read goes to reads.json.broken, the one there replaced, and a new one takes its place.
// LIMIT: the pages read of a broken reads.json are lost until it is mended by hand.
func (s *Store) Begin(viewer, now string) error {
	unlock, err := s.lock(true)
	if err != nil {
		return err
	}
	defer unlock()
	all, err := s.readReads()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		broken := s.readsFile() + ".broken"
		if err := os.Rename(s.readsFile(), broken); err != nil {
			return usererr.CannotWrite(broken, err) //nolint:wrapcheck // the system's words, at where it goes
		}
	}
	if all[viewer].Since != "" {
		return nil
	}
	if all == nil {
		all = map[string]viewerReads{}
	}
	v := all[viewer]
	v.Since = now
	all[viewer] = v
	return s.writeReads(all)
}

// reader is what a viewer has read, as news and MarkRead compare it.
type reader struct {
	since time.Time
	pages map[string]pageRead
}

// readerIn is viewer of all: nil without a since, which nothing can be compared to.
func readerIn(all map[string]viewerReads, viewer string) *reader {
	v := all[viewer]
	if v.Since == "" {
		return nil
	}
	return &reader{since: timeOf(v.Since), pages: v.Pages}
}

// timeOf is the time s writes: zero, before any other, for one written by hand that is none.
func timeOf(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// sameVersion reports whether v is the version n, at, a number alone coming again after a removal.
func sameVersion(n int, at string) func(Version) bool {
	return func(v Version) bool { return v.N == n && v.At == at }
}

// taken is the number of the version of the page slug, whose meta.json is m, the reader takes as read: the one on
// record, else, of a page from before since, the last made by then; 0 for none.
// LIMIT: a record that is of none of m's versions is no record (a page removed and made again, one replaced, a version
// dropped past keepVersions), so that a page whose version read was dropped is new again. Keep a mark of the page if
// that comes to matter.
func (r *reader) taken(slug string, m *Meta) int {
	if rec, ok := r.pages[slug]; ok && slices.ContainsFunc(m.Versions, sameVersion(rec.N, rec.At)) {
		return rec.N
	}
	if timeOf(m.CreatedAt).After(r.since) {
		return 0
	}
	n := 0
	for _, v := range m.Versions {
		if !timeOf(v.At).After(r.since) {
			n = v.N
		}
	}
	return n
}

// news is what the reader has not seen of the stored page slug, whose meta.json is m: "" for nothing, or for a nil
// reader, of a reads.json that cannot be read, when every page counts as read rather than all as new.
func (r *reader) news(slug string, m *Meta) string {
	if r == nil || len(m.Versions) == 0 {
		return ""
	}
	n := r.taken(slug, m)
	switch {
	case n == m.Versions[len(m.Versions)-1].N:
		return ""
	case n > 0 || !timeOf(m.CreatedAt).After(r.since):
		return newsVersion
	default:
		return newsNew
	}
}

// MarkRead keeps that viewer read the version n, at, of the stored page slug, unless the page has no such version or
// viewer has read one as new: ErrNotFound for no stored page, store_reads_unset without a since to compare to. One
// that changes nothing takes no lock, as a page read before is passed over often.
func (s *Store) MarkRead(viewer, slug string, n int, at string) error {
	if _, newer, err := s.newerRead(viewer, slug, n, at); err != nil || !newer {
		return err
	}
	unlock, err := s.lock(false)
	if errors.Is(err, fs.ErrNotExist) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	defer unlock()
	// Again, under the lock: another save may have come since.
	all, newer, err := s.newerRead(viewer, slug, n, at)
	if err != nil || !newer {
		return err
	}
	v := all[viewer]
	if v.Pages == nil {
		v.Pages = map[string]pageRead{}
	}
	v.Pages[slug] = pageRead{N: n, At: at}
	all[viewer] = v
	return s.writeReads(all)
}

// newerRead is reads.json, and whether viewer reading the version n, at, of slug changes what it keeps.
func (s *Store) newerRead(viewer, slug string, n int, at string) (map[string]viewerReads, bool, error) {
	m := s.ReadMeta(slug)
	if m == nil || m.IsLink() {
		return nil, false, ErrNotFound
	}
	if !slices.ContainsFunc(m.Versions, sameVersion(n, at)) {
		return nil, false, nil
	}
	all, err := s.readReads()
	r := readerIn(all, viewer)
	if err != nil || r == nil {
		return nil, false, usererr.NewAt(s.readsFile(), 0, 0, "store_reads_unset")
	}
	return all, r.taken(slug, m) < n, nil
}

// moveRead moves what each viewer read of the page from to the page to, or forgets it ("" for to), and what they read
// of an earlier page to. It is called under the lock, after the rename or removal, which is done whatever comes of it.
// LIMIT: one that cannot be written leaves the page as if read by no one, and a key that no version of a later page of
// that slug matches. Tell it if that comes to matter.
func (s *Store) moveRead(from, to string) {
	all, err := s.readReads()
	if err != nil {
		return
	}
	changed := false
	for _, v := range all {
		rec, had := v.Pages[from]
		_, stale := v.Pages[to]
		if !had && !stale {
			continue
		}
		changed = true
		delete(v.Pages, to)
		delete(v.Pages, from)
		if had && to != "" {
			v.Pages[to] = rec
		}
	}
	if changed {
		_ = s.writeReads(all) //nolint:errcheck // as above
	}
}

// ReadMark is the version n of the stored page slug, whose meta.json is m, as a viewer reads it, 0 for its last: false
// for a version m does not list, and a link.
func (s *Store) ReadMark(slug string, m *Meta, n int) (Version, bool) {
	if n == 0 && len(m.Versions) > 0 {
		n = m.Versions[len(m.Versions)-1].N
	}
	_, at, ok := s.versionFile(slug, m, n)
	if !ok {
		return Version{N: 0, At: "", Mark: ""}, false
	}
	return m.Versions[at], true
}
