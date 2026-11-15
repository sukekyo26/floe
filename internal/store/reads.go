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

	"floe/internal/files"
	"floe/internal/usererr"
)

const (
	readsName = "reads.json"
	// What Listed.News says of a stored page: never seen, or with a version after the one read.
	newsNew     = "new"
	newsVersion = "version"
)

// errReadsBroken is a reads.json read that holds no reads.
var errReadsBroken = errors.New("no reads")

// pageRead is the version of a page a viewer read: its number and when it was made, a number coming again after a
// removal. N 0 says nothing is read, marked so by hand, whatever since says. Before is what was read until this.
type pageRead struct {
	N      int       `json:"n"`
	At     string    `json:"at"`
	Before *pageRead `json:"before,omitempty"`
}

// viewerReads is what reads.json keeps of a viewer: since, when floe first served them, and the pages read, by their
// meta.json's id, which a rename does not change.
// LIMIT: the record of a page removed by hand, or replaced while its meta.json could not be read, stays, of no page.
// Drop those of no id there is in Begin if the size of reads.json comes to matter.
type viewerReads struct {
	Since string              `json:"since,omitempty"`
	Pages map[string]pageRead `json:"pages,omitempty"`
}

func (s *Store) readsFile() string {
	return filepath.Join(s.root, readsName)
}

// readReads reads reads.json, by viewer: fs.ErrNotExist for none, errReadsBroken for what holds no reads. It
// takes no lock: a write replaces the file whole, so that a reader has the one before it or the one after.
func (s *Store) readReads() (map[string]viewerReads, error) {
	b, _, err := files.Read(s.readsFile())
	if err != nil {
		return nil, err //nolint:wrapcheck // for the caller, which tells what it does of it
	}
	var all map[string]viewerReads
	if err := json.Unmarshal(b, &all); err != nil {
		return nil, fmt.Errorf("%w: %w", errReadsBroken, err)
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

// hasVersion reports whether m has the version n, at, a number alone coming again after a removal.
func (m *Meta) hasVersion(n int, at string) bool {
	return slices.ContainsFunc(m.Versions, func(v Version) bool { return v.N == n && v.At == at })
}

// LastNumber is the number of m's last version, 0 for none.
func (m *Meta) LastNumber() int {
	if len(m.Versions) == 0 {
		return 0
	}
	return m.Versions[len(m.Versions)-1].N
}

// taken is the number of the version of the page whose meta.json is m the reader takes as read: the one on record (0
// for one marked unread), else, of a page from before since, the last made by then; 0 for none.
// LIMIT: a record that is of none of m's versions is no record (one dropped past keepVersions, or a last one that could
// not be read, which nextVersion drops), so that a page whose version read was dropped is new again. Keep the number
// alone if that comes to matter.
func (r *reader) taken(m *Meta) int {
	if rec, ok := r.pages[m.ID]; ok && (rec.N == 0 || m.hasVersion(rec.N, rec.At)) {
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

// ReadBefore is the number of the version of the stored page whose meta.json is m viewer read before the last: the one
// taken as read, or as that is the last, the one before reading it, as opening a page reads it at once. 0 for none,
// never the last.
func (s *Store) ReadBefore(viewer string, m *Meta) int {
	all, _ := s.readReads() //nolint:errcheck // what cannot be read is nothing read, as the list has it
	r := readerIn(all, viewer)
	if r == nil || m.IsLink() {
		return 0
	}
	n := r.taken(m)
	if n != m.LastNumber() {
		return n
	}
	rec, ok := r.pages[m.ID]
	if !ok || !m.hasVersion(rec.N, rec.At) || rec.Before == nil || !m.hasVersion(rec.Before.N, rec.Before.At) {
		return 0
	}
	return rec.Before.N
}

// takenRead is the record of what taken says, nil for nothing read.
func (r *reader) takenRead(m *Meta) *pageRead {
	n := r.taken(m)
	at := slices.IndexFunc(m.Versions, func(v Version) bool { return v.N == n })
	if at < 0 {
		return nil
	}
	return &pageRead{N: n, At: m.Versions[at].At, Before: nil}
}

// news is what the reader has not seen of the stored page whose meta.json is m: "" for nothing, or for a nil reader.
// LIMIT: a reads.json that goes bad while the server runs has no reader: every page counts as read, rather than all as
// new, until floe serve starts again and writes it anew.
func (r *reader) news(m *Meta) string {
	if r == nil || len(m.Versions) == 0 {
		return ""
	}
	n := r.taken(m)
	rec, kept := r.pages[m.ID]
	unread := kept && rec.N == 0
	switch {
	case n == m.LastNumber():
		return ""
	case n > 0, !unread && !timeOf(m.CreatedAt).After(r.since):
		return newsVersion
	default:
		return newsNew
	}
}

// MarkRead keeps that viewer read the version n, at, of the stored page slug, unless the page has no such version or
// viewer has read one as new: ErrNotFound for no stored page, store_reads_unset without a since to compare to, and
// what the system says of a reads.json it cannot read. It says whether this read the page up to its last, which it had
// not been. It keeps what viewer had read before, for MarkUnread.
func (s *Store) MarkRead(viewer, slug string, n int, at string) (bool, error) {
	cleared := false
	err := s.changeRead(viewer, slug, func(r *reader, m *Meta) *pageRead {
		cleared = false
		if !m.hasVersion(n, at) || r.taken(m) >= n {
			return nil
		}
		cleared = n == m.LastNumber()
		return &pageRead{N: n, At: at, Before: r.takenRead(m)}
	})
	return cleared, err
}

// MarkUnread takes the stored page slug back to what viewer had read before reading it, or to nothing read: a page
// with news stays as it is, so that pressing twice goes no further back. Its errors are MarkRead's.
// LIMIT: one step back alone: what was read before the before is not kept. Keep more if that comes to matter.
func (s *Store) MarkUnread(viewer, slug string) error {
	return s.changeRead(viewer, slug, func(r *reader, m *Meta) *pageRead {
		if r.news(m) != "" {
			return nil
		}
		rec, ok := r.pages[m.ID]
		if ok && rec.Before != nil && m.hasVersion(rec.N, rec.At) && m.hasVersion(rec.Before.N, rec.Before.At) {
			return rec.Before
		}
		return &pageRead{N: 0, At: "", Before: nil}
	})
}

// nextRecord is what a viewer's record of a page comes to, nil for no change.
type nextRecord func(*reader, *Meta) *pageRead

// changeRead keeps what next says viewer's record of slug comes to. One that changes nothing takes no lock, as a page
// read before is passed over often.
func (s *Store) changeRead(viewer, slug string, next nextRecord) error {
	if _, _, rec, err := s.nextRead(viewer, slug, next); err != nil || rec == nil {
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
	// Again, under the lock: another save may have come since, one replacing the page with another of another id too.
	all, m, rec, err := s.nextRead(viewer, slug, next)
	if err != nil || rec == nil {
		return err
	}
	v := all[viewer]
	if v.Pages == nil {
		v.Pages = map[string]pageRead{}
	}
	v.Pages[m.ID] = *rec
	all[viewer] = v
	return s.writeReads(all)
}

// nextRead is reads.json, the meta.json of the stored page slug, and what next says of it. A page of no id, as
// none floe stores, keeps no record, as a link.
func (s *Store) nextRead(viewer, slug string, next nextRecord) (map[string]viewerReads, *Meta, *pageRead, error) {
	m := s.ReadMeta(slug)
	if m == nil || m.IsLink() || m.ID == "" {
		return nil, nil, nil, ErrNotFound
	}
	all, err := s.readReads()
	if err != nil && !errors.Is(err, fs.ErrNotExist) && !errors.Is(err, errReadsBroken) {
		return nil, nil, nil, usererr.CannotRead(s.readsFile(), err) //nolint:wrapcheck // the system's words, at the file
	}
	r := readerIn(all, viewer)
	if r == nil {
		return nil, nil, nil, usererr.NewAt(s.readsFile(), 0, 0, "store_reads_unset")
	}
	return all, m, next(r, m), nil
}

// forgetRead drops what each viewer read of the page whose meta.json is m, which is going. It is called under the
// lock, before the page goes, so that one that cannot be written stops it. A reads.json missing or broken keeps no
// record to drop; one that cannot be read may, once mended, so it is told.
func (s *Store) forgetRead(m *Meta) error {
	if m.IsLink() || m.ID == "" {
		return nil
	}
	all, err := s.readReads()
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, errReadsBroken) {
		return nil
	}
	if err != nil {
		return usererr.CannotRead(s.readsFile(), err) //nolint:wrapcheck // the system's words, at the file
	}
	changed := false
	for _, v := range all {
		if _, ok := v.Pages[m.ID]; ok {
			delete(v.Pages, m.ID)
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return s.writeReads(all)
}

// ReadMark is the version n of the stored page slug, whose meta.json is m, as a viewer reads it, 0 for its last: false
// for a version m does not list, and a link.
func (s *Store) ReadMark(slug string, m *Meta, n int) (Version, bool) {
	if n == 0 {
		n = m.LastNumber()
	}
	_, at, ok := s.versionFile(slug, m, n)
	if !ok {
		return Version{N: 0, At: "", Mark: ""}, false
	}
	return m.Versions[at], true
}
