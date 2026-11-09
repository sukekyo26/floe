package store

import (
	"errors"
	"io/fs"
	"path/filepath"

	"github.com/sukekyo26/floe/internal/files"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

// PickTheme picks theme, one of dw's, for the page slug drawn from a data file ("" takes the pick off), as the
// management page does: it draws the page in it first, and writes nothing when it cannot be drawn. A stored page is
// drawn from its copy, its page and the pick then written together; a linked data file, drawn each time it is opened,
// only to be checked. A page published, picked or rebuilt while it was drawn is refused (Conflict), nothing written.
// The page keeps the pick across publishes and rebuilds (GivenTheme).
func (s *Store) PickTheme(dw Drawing, slug, theme string) error {
	if dw.Err != nil {
		return dw.Err
	}
	m := s.ReadMeta(slug)
	if m == nil {
		return ErrNotFound
	}
	if !m.Themed() {
		return usererr.New("store_theme_link")
	}
	picked := *m
	picked.PickedTheme = theme
	var got *drawn
	var drawErr error
	if m.IsLink() {
		file, ok := s.File(slug, m, "")
		if !ok {
			return ErrNotFound
		}
		_, drawErr = DrawLink(dw, slug, &picked, file, false)
	} else {
		dir, ok := s.copyDir(slug, m)
		if !ok {
			return ErrNotFound
		}
		got, drawErr = redraw(dw, slug, &picked, dir)
	}
	unlock, err := s.lock(false)
	if errors.Is(err, fs.ErrNotExist) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	defer unlock()
	// Compared before the drawing's mistakes: a publish meanwhile may have taken the copy drawn from.
	now := s.ReadMeta(slug)
	if now == nil {
		return ErrNotFound
	}
	if now.Source != m.Source || now.PickedTheme != m.PickedTheme {
		return conflict(usererr.New("store_theme_changed", slug))
	}
	if drawErr != nil {
		return drawErr
	}
	now.PickedTheme = theme
	// The page drawn the same as the one it has (nil): the pick alone is written, as it is kept for later drawings.
	if got == nil || got.page == nil {
		return s.WriteMeta(slug, now)
	}
	if got.template != "" {
		now.Template = got.template
	}
	return s.keepCopy(slug, now, got.src, got.page, got.images, nil)
}

// DrawLink draws the linked data file file of slug, whose meta.json is m, with dw: in its theme, under slug for a page
// of no title, its images under its real directory, cut off (tmpl.Options.CutOff) when cut.
func DrawLink(dw Drawing, slug string, m *Meta, file string, cut bool) ([]byte, error) {
	if dw.Err != nil {
		return nil, dw.Err
	}
	src, _, err := files.Read(file)
	if err != nil {
		return nil, usererr.CannotRead(file, err) //nolint:wrapcheck // the system's words, at the file
	}
	images := files.NewRoot(filepath.Dir(file))
	defer images.Close() //nolint:errcheck // only read from
	o := tmpl.Options{
		Theme: m.GivenTheme(), Default: m.CreatedTheme, Untitled: slug, Images: images, ImagesDir: "", Icons: dw.Icons,
		CutOff: cut,
	}
	// Its warnings are the writer's, whom publish and check tell; one who opens the page cannot mend them.
	page, _, err := tmpl.Draw(dw.Kit, file, src, o)
	return page, err //nolint:wrapcheck // its mistakes, or what failed
}
