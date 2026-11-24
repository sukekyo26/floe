package store_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	floefiles "floe/internal/files"
	"floe/internal/store"
	"floe/internal/testfs"
	"floe/internal/usererr"
)

// A page served (its copy held open) is published again and removed all the same, its old copy gone (Windows: floe
// reads sharing delete).
func TestServeWhilePublish(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := picKit(t, "", "")
	if _, err := put(t, s, dw, "plan.yaml", "plan.yaml", ownSrc, store.Options{}); err != nil { //nolint:exhaustruct // none
		t.Fatal(told(err, root))
	}
	m := s.ReadMeta("plan")
	page, ok := s.File("plan", m, "")
	if !ok {
		t.Fatal("no page to serve")
	}
	served, _, err := floefiles.Open(page)
	if err != nil {
		t.Fatal(err)
	}
	defer served.Close()
	if _, err = put(t, s, dw, "plan.yaml", "plan.yaml", ownSrc+"title: Again\n", store.Options{}); err != nil { //nolint:exhaustruct // none
		t.Fatalf("publish while the page is served = %v", told(err, root))
	}
	if _, err = os.Lstat(filepath.Join(root, "plan", m.Source)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the copy served is left after the publish: %v", err)
	}
	again, ok := s.File("plan", s.ReadMeta("plan"), "")
	if !ok {
		t.Fatal("no page to serve")
	}
	servedAgain, _, err := floefiles.Open(again)
	if err != nil {
		t.Fatal(err)
	}
	defer servedAgain.Close()
	if err := s.Remove("plan"); err != nil {
		t.Errorf("rm while the page is served = %v", told(err, root))
	}
	if _, err := os.Lstat(filepath.Join(root, "plan")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the page's directory is left after rm: %v", err)
	}
}

// A new slug moves the page first: one that cannot be moved leaves it all as it was; one moved whose meta.json cannot
// be written is moved back, all as it was.
func TestEditRenameFirst(t *testing.T) {
	t.Parallel()
	testfs.UnixOnly(t, "permission bits")
	if os.Geteuid() == 0 {
		t.Skip("root writes where the bits say not")
	}

	for _, readOnly := range []string{"the store", "the page"} {
		t.Run(readOnly, func(t *testing.T) {
			t.Parallel()

			s, root := newStore(t)
			if _, err := put(t, s, picKit(t, "", ""), "plan.yaml", "plan.yaml", ownSrc, store.Options{}); err != nil { //nolint:exhaustruct // none
				t.Fatal(told(err, root))
			}
			dir := root
			if readOnly == "the page" {
				dir = filepath.Join(root, "plan")
			}
			if err := os.Chmod(dir, 0o500); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })                               //nolint:errcheck // a temporary directory
			_, err := s.Edit("plan", store.Edit{Slug: new("plan-b"), Title: new("New")}) //nolint:exhaustruct // none
			ue, ok := errors.AsType[*usererr.Error](err)
			if !ok || ue.Key != "cannot_write" {
				t.Fatalf("Edit with %s read only = %v, want cannot_write", readOnly, err)
			}
			// Moved back, it is told at the meta.json there is, under the slug as it was.
			if want := filepath.Join(root, "plan", "meta.json"); readOnly == "the page" && ue.File() != want {
				t.Errorf("Edit with the page read only is told at %s, want %s", ue.File(), want)
			}
			if m := s.ReadMeta("plan"); m == nil || m.Title == "New" {
				t.Errorf("the page after the failed Edit = %+v, want it as it was", m)
			}
			if s.ReadMeta("plan-b") != nil {
				t.Error("the page is at the new slug after the failed Edit")
			}
		})
	}
}

// A page whose directory is a link out of the store is left out of the list (os.Root refuses it), though it is
// there; the others are listed (List's LIMIT).
func TestListLeavesLinkOut(t *testing.T) {
	t.Parallel()

	s, root := newStore(t)
	dw := picKit(t, "", "")
	for _, name := range []string{"plan.yaml", "other.yaml"} {
		if _, err := put(t, s, dw, name, name, ownSrc, store.Options{}); err != nil { //nolint:exhaustruct // none
			t.Fatal(told(err, root))
		}
	}
	out := filepath.Join(t.TempDir(), "plan")
	if err := os.Rename(filepath.Join(root, "plan"), out); err != nil {
		t.Fatal(err)
	}
	testfs.Symlink(t, out, filepath.Join(root, "plan"))
	list, err := s.List("")
	if err != nil {
		t.Fatal(err)
	}
	slugs := make([]string, 0, len(list))
	for _, l := range list {
		slugs = append(slugs, l.Slug)
	}
	if len(slugs) != 1 || slugs[0] != "other" {
		t.Errorf("listed %v, want other alone", slugs)
	}
	if s.ReadMeta("plan") == nil {
		t.Error("the page out of the store is not there by its slug")
	}
	// The store is not held open after the list: it can be removed.
	if err := floefiles.RemoveAll(root); err != nil {
		t.Errorf("RemoveAll of the store after a list = %v", err)
	}
}
