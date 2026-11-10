package store

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"floe/internal/doc"
	"floe/internal/files"
	"floe/internal/tmpl"
	"floe/internal/usererr"
)

// Rebuild draws every stored page again, with dw, from its copy, and keeps the page drawn where it changed: in a
// new copy, which its meta.json then names, its updatedAt as it was. A page that no longer draws, or whose copy
// cannot be read or written, keeps the page it had, its mistakes told with store_rebuild_kept, and the next
// page is drawn; one published again, removed or renamed meanwhile is left to that. Its meta.json takes the template
// its data file names, drawn or not. Links are drawn each time they are opened, and are left alone.
// LIMIT: a page whose copy's data file cannot be read keeps the template it had. Publish it again.
// LIMIT: the slugs are listed once, before the first page: one renamed meanwhile to a slug not listed is not
// drawn again. List them again at the end, and draw those not drawn, if that comes to matter.
// LIMIT: a mistake without a place (a theme of meta.json that is none) is told once for every page, by Tell's
// rule, the second page's store_rebuild_kept without it above. Hang each page's mistakes under its
// store_rebuild_kept, with an indent, if that comes to confuse.
// LIMIT: a template floe fails to compile again with one's own parts stops it at that template's first page, the
// pages of other templates before it drawn again and written; floe's own failure alone, which its tests catch.
func (s *Store) Rebuild(dw Drawing) error {
	if dw.Err != nil {
		return dw.Err
	}
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

// drawn is a page drawn again from its copy: the data file, the page (nil for the one it has) and every image of the
// copy, and the template the data file names ("" for none it can tell).
type drawn struct {
	src, page []byte
	images    map[string][]byte
	template  string
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
	if err == nil && got.page == nil && (got.template == "" || got.template == m.Template) {
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
	// Published again, removed or renamed since it was read: what it is now is not what was drawn. A theme picked
	// meanwhile is in a new copy, or the pick alone where the page drawn in it is the one it has, as this one is then.
	now := s.ReadMeta(slug)
	if now == nil || now.Source != m.Source {
		return nil
	}
	err = s.keepDrawn(slug, now, got, err, false)
	if err == nil {
		return nil
	}
	told := make(usererr.List, 0, 2)
	if failed := told.Add(err); failed != nil {
		return failed
	}
	return append(told, usererr.NewAt(filepath.Join(dir, dataName), 0, 0, "store_rebuild_kept", slug))
}

// keepDrawn writes what redraw returned, got (nil for a link, which has no copy) and err, of the page slug, under the
// lock: of now, meta.json as it is, so that an edit made while the page was drawn stays, and changed by the caller when
// changed. It returns err, or what could not be written.
func (s *Store) keepDrawn(slug string, now *Meta, got *drawn, err error, changed bool) error {
	named := got != nil && got.template != "" && got.template != now.Template
	if named {
		now.Template = got.template
	}
	switch {
	case err == nil && got != nil && got.page != nil:
		err = s.keepCopy(slug, now, got.src, got.page, got.images, nil)
	case named || changed:
		err = usererr.Collect(err, s.WriteMeta(slug, now))
	}
	return err
}

// redraw draws slug, whose meta.json is m, from its copy dir: a nil page when the page drawn is the one it has. With a
// mistake, what it could read of the copy, or nil when it could not read the data file.
func redraw(dw Drawing, slug string, m *Meta, dir string) (*drawn, error) {
	data := filepath.Join(dir, dataName)
	src, err := os.ReadFile(data) //nolint:gosec // G304: a copy of the store's own
	if err != nil {
		return nil, usererr.CannotRead(data, err) //nolint:wrapcheck // the system's words, at the copy
	}
	got := &drawn{src: src, page: nil, images: nil, template: ""}
	if d, _ := doc.Read(data, src, m.Theme); d != nil { //nolint:errcheck // its mistakes are told when it is drawn
		got.template = d.Template
	}
	had := filepath.Join(dir, pageName)
	old, err := os.ReadFile(had) //nolint:gosec // G304: as above
	if err != nil {
		return got, usererr.CannotRead(had, err) //nolint:wrapcheck // as above
	}
	// One path for both, so that the images read are those the mistakes name.
	imagesDir := filepath.Join(dir, imagesName)
	images := files.NewRoot(imagesDir)
	defer images.Close() //nolint:errcheck // only read from
	// LIMIT: a page of no title: is drawn under the slug of now, which a rename on the management page changes
	// while the list keeps the title it had. Keep the title drawn under in meta.json if that comes to matter.
	// LIMIT: kit_image_path names the directory of data, not images/, for a name floe no longer reads, which
	// a publish checked by the same rule. Have draw name it by ImagesDir too if the rule comes to change.
	o := tmpl.Options{
		Theme: m.GivenTheme(), Default: m.CreatedTheme, Untitled: slug, Images: images, ImagesDir: imagesDir,
		Icons:  dw.Icons,
		CutOff: false,
	}
	// Its warnings are of the copy, which no one mends: a publish of the data file tells them.
	page, _, err := tmpl.Draw(dw.Kit, data, src, o)
	if err != nil {
		return got, err //nolint:wrapcheck // its mistakes, or what failed
	}
	if bytes.Equal(page, old) {
		return got, nil
	}
	kept, err := everyImage(images, imagesDir)
	if err != nil {
		return got, err
	}
	got.page, got.images = page, kept
	return got, nil
}

// everyImage reads every regular file under images, the directory dir, by its slash-separated path: those
// the drawing did not read too, which a page drawn again may read. A symlink, a FIFO or the like, which no save
// writes, is left behind.
func everyImage(images fs.FS, dir string) (map[string][]byte, error) {
	kept := map[string][]byte{}
	err := fs.WalkDir(images, ".", func(name string, d fs.DirEntry, err error) error {
		at := filepath.Join(dir, filepath.FromSlash(name))
		switch {
		case err != nil:
			return usererr.CannotRead(at, err) //nolint:wrapcheck // the system's words, at what cannot be read
		case !d.Type().IsRegular():
			return nil
		}
		b, err := fs.ReadFile(images, name)
		if err != nil {
			return usererr.CannotRead(at, err) //nolint:wrapcheck // as above
		}
		kept[name] = b
		return nil
	})
	if err != nil {
		return nil, err //nolint:wrapcheck // a usererr, at what cannot be read
	}
	return kept, nil
}
