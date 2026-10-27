package store

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sukekyo26/floe/internal/files"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

// Rebuild draws every stored page again, with dw, from its copy, and keeps the page drawn where it changed: in a
// new copy, which its meta.json then names, its updatedAt as it was. A page that no longer draws, or whose copy
// cannot be read or written, keeps the page it had, its mistakes told with store_rebuild_kept, and the next
// page is drawn; one published again, removed or renamed meanwhile is left to that. Links are drawn each time
// they are opened, and are left alone.
// LIMIT: the slugs are listed once, before the first page: one renamed meanwhile to a slug not listed is not
// drawn again. List them again at the end, and draw those not drawn, if that comes to matter.
// LIMIT: a mistake without a place (a theme of meta.json that is none) is told once for every page, by Tell's
// rule, the second page's store_rebuild_kept without it above. Hang each page's mistakes under its
// store_rebuild_kept, with an indent, if that comes to confuse.
func (s *Store) Rebuild(dw Drawing) error {
	slugs, err := s.slugs()
	if err != nil {
		return err
	}
	var found usererr.List
	for _, slug := range slugs {
		if failed := found.Add(s.rebuildPage(dw, slug)); failed != nil {
			return failed
		}
	}
	return found.Err()
}

// drawn is a page drawn again from its copy: the data file, the page and every image of the copy.
type drawn struct {
	src, page []byte
	images    map[string][]byte
}

// rebuildPage draws slug again, as Rebuild does.
func (s *Store) rebuildPage(dw Drawing, slug string) error {
	m := s.ReadMeta(slug)
	if m == nil || m.IsLink() {
		return nil
	}
	dir, ok := s.copyDir(slug, m)
	if !ok {
		return nil
	}
	got, err := redraw(dw, slug, m, dir)
	if err == nil && got == nil {
		return nil
	}
	unlock, lockErr := s.lock(false)
	if lockErr != nil {
		if errors.Is(lockErr, fs.ErrNotExist) {
			return nil // the store went, and the page with it
		}
		return lockErr
	}
	defer unlock()
	// Published again, removed or renamed since it was read: what it is now is not what was drawn.
	if now := s.ReadMeta(slug); now == nil || now.Source != m.Source {
		return nil
	}
	if err == nil {
		err = s.keep(slug, m, got)
	}
	if err == nil {
		return nil
	}
	told := make(usererr.List, 0, 2)
	if failed := told.Add(err); failed != nil {
		return failed
	}
	return append(told, usererr.NewAt(filepath.Join(dir, dataName), 0, 0, "store_rebuild_kept", slug))
}

// redraw draws slug, whose meta.json is m, from its copy dir: nil when the page drawn is the one it has.
func redraw(dw Drawing, slug string, m *Meta, dir string) (*drawn, error) {
	data := filepath.Join(dir, dataName)
	src, err := os.ReadFile(data) //nolint:gosec // G304: a copy of the store's own
	if err != nil {
		return nil, usererr.CannotRead(data, err) //nolint:wrapcheck // the system's words, at the copy
	}
	had := filepath.Join(dir, pageName)
	old, err := os.ReadFile(had) //nolint:gosec // G304: as above
	if err != nil {
		return nil, usererr.CannotRead(had, err) //nolint:wrapcheck // as above
	}
	// One path for both, so that the images read are those the mistakes name.
	imagesDir := filepath.Join(dir, imagesName)
	images := files.NewRoot(imagesDir)
	defer images.Close() //nolint:errcheck // only read from
	o := tmpl.Options{Theme: m.Theme, Untitled: slug, Images: images, ImagesDir: imagesDir, Icons: dw.Icons}
	page, err := tmpl.Draw(dw.Kit, data, src, o)
	if err != nil {
		return nil, err //nolint:wrapcheck // its mistakes, or what failed
	}
	if bytes.Equal(page, old) {
		return nil, nil //nolint:nilnil // nothing to keep
	}
	kept, err := everyImage(images, imagesDir)
	if err != nil {
		return nil, err
	}
	return &drawn{src: src, page: page, images: kept}, nil
}

// everyImage reads every regular file under images, the directory dir, by its slash-separated path: those
// the drawing did not read too, which a page drawn again may read. A symlink, a FIFO or the like, which no save
// writes, is left behind.
func everyImage(images fs.FS, dir string) (map[string][]byte, error) {
	kept := map[string][]byte{}
	err := fs.WalkDir(images, ".", func(name string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case !d.Type().IsRegular():
			return nil
		}
		b, err := fs.ReadFile(images, name)
		if err != nil {
			return err //nolint:wrapcheck // told below
		}
		kept[name] = b
		return nil
	})
	if err != nil {
		return nil, usererr.CannotRead(dir, err) //nolint:wrapcheck // the system's words, at the images
	}
	return kept, nil
}

// keep writes got, slug's page drawn again, as a new copy, which meta.json m then names, and removes the one
// before: under the lock, m as it is now.
func (s *Store) keep(slug string, m *Meta, got *drawn) error {
	dir, ok := s.dir(slug)
	if !ok {
		return fmt.Errorf("store: keep %q, which cannot be a slug", slug) //nolint:err113 // floe's own mistake
	}
	source, err := writeSource(dir, got.src, got.page, got.images)
	if err != nil {
		return err
	}
	m.Source = source
	if err := s.WriteMeta(slug, m); err != nil {
		_ = os.RemoveAll(filepath.Join(dir, source)) //nolint:errcheck // the error is the write's
		return err
	}
	removeSources(dir, source)
	return nil
}
