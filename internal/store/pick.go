package store

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"

	"floe/internal/drawproc"
	"floe/internal/files"
	"floe/internal/usererr"
)

// PickTheme makes theme, one of dw's, the theme of the page slug drawn from a data file, as the management page does:
// it draws the page in it first, and writes nothing when it cannot be drawn. A stored page is drawn from its copy, its
// last version's page and the theme then written together, the versions before kept as they were drawn; a linked data
// file, drawn each time it is opened, only to be checked, the theme: of its file as it is now taken as the one named
// (GivenTheme), so that the file, once it names another, is drawn in that. A page published, picked or rebuilt while it
// was drawn is refused (Conflict), nothing written. The page keeps the theme until a publish names another (PageTheme).
// LIMIT: a version before the last keeps the theme it was drawn in, as it keeps no data file to be drawn again from: it
// opened, or a diff of two such, shows that one. The user chose so over a store that keeps each version's data and
// images; keep them if a theme of its own for each version comes to matter.
func (s *Store) PickTheme(ctx context.Context, dw Drawing, slug, theme string) error {
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
	picked.Theme = theme
	got, drawErr := s.drawPicked(ctx, dw, slug, &picked)
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
	if now.Source != m.Source || now.Path != m.Path || now.Theme != m.Theme || now.GivenTheme != m.GivenTheme {
		return conflict(usererr.New("store_theme_changed", slug))
	}
	if drawErr != nil {
		return drawErr
	}
	now.Theme, now.GivenTheme = theme, picked.GivenTheme
	// Written though the page drawn is the one it has (or a link's, drawn when opened): the theme is kept for later
	// drawings.
	return s.keepDrawn(slug, now, got, nil, true)
}

// drawPicked draws the page slug, whose meta.json is m: a stored page from its copy (what redraw returns), a linked
// data file only to be checked (nil), m then taking the theme: of its file as the one named. ErrNotFound for a page
// whose file or copy is not there.
func (s *Store) drawPicked(ctx context.Context, dw Drawing, slug string, m *Meta) (*drawn, error) {
	if m.IsLink() {
		file, ok := s.File(slug, m, "")
		if !ok {
			return nil, ErrNotFound
		}
		src, _, err := files.Read(file)
		if err != nil {
			return nil, usererr.CannotRead(file, err) //nolint:wrapcheck // the system's words, at the file
		}
		m.GivenTheme = writtenTheme(file, src)
		_, err = DrawLink(ctx, dw, slug, m, file, false)
		return nil, err
	}
	dir, ok := s.copyDir(slug, m)
	if !ok {
		return nil, ErrNotFound
	}
	session := dw.Draw.Start(ctx)
	defer session.Close()
	return redraw(session, slug, m, dir)
}

// DrawLink draws the linked data file file of slug, whose meta.json is m, with dw under ctx: in the theme its theme:
// names when that is not the one named before, else in the page's (PageTheme), under slug for a page of no title, its
// images under its real directory, cut off (tmpl.Options.CutOff) when cut.
// LIMIT: a data file linked with --theme whose file names another theme: is drawn in the file's, as GivenTheme does
// not keep --theme apart from the theme: it names. Keep --theme apart if links come to take it.
func DrawLink(ctx context.Context, dw Drawing, slug string, m *Meta, file string, cut bool) ([]byte, error) {
	if dw.Err != nil {
		return nil, dw.Err
	}
	src, _, err := files.Read(file)
	if err != nil {
		return nil, usererr.CannotRead(file, err) //nolint:wrapcheck // the system's words, at the file
	}
	// Its warnings are the writer's, whom publish and check tell; one who opens the page cannot mend them.
	got := drawproc.Draw(ctx, dw.Draw, drawproc.Job{
		File: file, Source: src, Theme: drawTheme(m, "", writtenTheme(file, src)), Default: "", Untitled: slug, CutOff: cut,
		ImagesRoot: filepath.Dir(file), Sent: nil, ImagesDir: "",
	})
	return got.Page, got.Err
}
