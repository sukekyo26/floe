package store

import (
	"os"
	"path/filepath"
	"testing"
)

// The record goes to the id of the page as read under the lock: a page replaced, of another id, after the read before
// the lock has its record at its own id, none at the one before.
func TestChangeReadReplaced(t *testing.T) {
	t.Parallel()

	s := New(filepath.Join(t.TempDir(), "store"))
	if err := os.MkdirAll(filepath.Join(s.root, "d"), 0o700); err != nil {
		t.Fatal(err)
	}
	m := &Meta{ID: "before", Source: "source-1", CreatedAt: "2026-11-04T11:00:00.000Z", UpdatedAt: "2026-11-04T11:00:00.000Z"}
	if err := s.WriteMeta("d", m); err != nil {
		t.Fatal(err)
	}
	if err := s.Begin("local", "2026-11-04T10:00:00.000Z"); err != nil {
		t.Fatal(err)
	}
	calls := 0
	err := s.changeRead("local", "d", func(*reader, *Meta) *pageRead {
		calls++
		if calls == 1 {
			m.ID = "after"
			if err := s.WriteMeta("d", m); err != nil {
				t.Fatal(err)
			}
		}
		return &pageRead{N: 0, At: "", Before: nil}
	})
	if err != nil {
		t.Fatal(err)
	}
	all, err := s.readReads()
	if err != nil {
		t.Fatal(err)
	}
	pages := all["local"].Pages
	if _, ok := pages["after"]; !ok || len(pages) != 1 || calls != 2 {
		t.Errorf("kept %v, of %d reads", pages, calls)
	}
}
