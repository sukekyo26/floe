package store

import (
	"cmp"
	"errors"
	"io/fs"
	"path/filepath"

	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/files"
	"github.com/sukekyo26/floe/internal/kit"
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
	if !m.TakesTheme() {
		return usererr.New("store_theme_link")
	}
	picked := *m
	picked.PickedTheme = theme
	got, drawErr := s.drawPicked(dw, slug, &picked)
	if errors.Is(drawErr, ErrNotFound) {
		return drawErr
	}
	unlock, err := s.lock(false)
	if errors.Is(err, fs.ErrNotExist) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	defer unlock()
	// Compared before the drawing's mistakes: a publish meanwhile may have taken the copy drawn from. A link has no copy:
	// linked to another file, its path tells it (published again from the same one, it keeps the pick and is drawn when
	// opened).
	now := s.ReadMeta(slug)
	if now == nil {
		return ErrNotFound
	}
	if now.Source != m.Source || now.Path != m.Path || now.PickedTheme != m.PickedTheme {
		return conflict(usererr.New("store_theme_changed", slug))
	}
	if drawErr != nil {
		return drawErr
	}
	now.PickedTheme = theme
	// Written though the page drawn is the one it has (or a link's, drawn when opened): the pick is kept for later
	// drawings.
	return s.keepDrawn(slug, now, got, nil, true)
}

// drawPicked draws the page slug, whose meta.json is m: a stored page from its copy (what redraw returns), a linked
// data file only to be checked (nil). ErrNotFound for a page whose file or copy is not there.
func (s *Store) drawPicked(dw Drawing, slug string, m *Meta) (*drawn, error) {
	if m.IsLink() {
		file, ok := s.File(slug, m, "")
		if !ok {
			return nil, ErrNotFound
		}
		_, err := DrawLink(dw, slug, m, file, false)
		return nil, err
	}
	dir, ok := s.copyDir(slug, m)
	if !ok {
		return nil, ErrNotFound
	}
	return redraw(dw, slug, m, dir)
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

// OwnTheme is the theme the page slug drawn from a data file is drawn in without the theme picked for it: --theme, else
// its data's theme:, else the default it took when first stored, else default, as the drawing picks it. It reads what
// the page is drawn from, its copy's data file or the linked one. ErrNotFound for a page that takes no theme.
func (s *Store) OwnTheme(slug string) (string, error) {
	m := s.ReadMeta(slug)
	if m == nil || !m.TakesTheme() {
		return "", ErrNotFound
	}
	var file string
	var ok bool
	if m.IsLink() {
		file, ok = s.File(slug, m, "")
	} else {
		var dir string
		dir, ok = s.copyDir(slug, m)
		file = filepath.Join(dir, dataName)
	}
	if !ok {
		return "", ErrNotFound
	}
	src, _, err := files.Read(file)
	if err != nil {
		return "", usererr.CannotRead(file, err) //nolint:wrapcheck // the system's words, at the file
	}
	d, err := doc.Read(file, src, m.Theme)
	if d == nil {
		return "", err //nolint:wrapcheck // its mistakes, at their places
	}
	return cmp.Or(d.Theme, m.CreatedTheme, kit.DefaultTheme), nil
}
