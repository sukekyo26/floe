package store

import (
	"cmp"
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"

	"floe/internal/files"
	"floe/internal/kit"
	"floe/internal/usererr"
)

// MoveThemes writes the meta.json of each page drawn from a data file that was written before a page had one theme, in
// the theme it is drawn in: the one picked on the management page (pickedTheme), else --theme (theme then), else its
// data's theme:, else the default it took when first stored (createdTheme), else default; the one --theme or theme:
// names as GivenTheme. Such a page holds pickedTheme or createdTheme, or no theme; written once, it holds none of them
// but theme, so that a second run writes nothing. A page published with --theme alone holds theme as one of now does
// and is left: drawn in it as before, the next publish naming the same as it would have. It runs under the lock, once,
// when the server starts, before it answers.
// LIMIT: a step of moving the stores of the release before; remove it once the release after it is out.
func (s *Store) MoveThemes() error {
	unlock, err := s.lock(false)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer unlock()
	slugs, err := s.slugs()
	if err != nil {
		return err
	}
	var found usererr.List
	for _, slug := range slugs {
		if failed := found.Add(s.moveTheme(slug)); failed != nil {
			return failed
		}
	}
	return found.Err()
}

// moveTheme is MoveThemes of the page slug.
func (s *Store) moveTheme(slug string) error {
	dir, _ := s.dir(slug)
	b, _, err := files.Read(filepath.Join(dir, "meta.json"))
	if err != nil {
		return nil //nolint:nilerr // no page, as ReadMeta tells
	}
	m := metaOf(b)
	var before struct {
		Picked  string `json:"pickedTheme"`
		Created string `json:"createdTheme"`
	}
	if m == nil || !m.TakesTheme() {
		return nil
	}
	_ = json.Unmarshal(b, &before) //nolint:errcheck // read as metaOf read it
	if before.Picked == "" && before.Created == "" && m.Theme != "" {
		return nil
	}
	given := cmp.Or(m.Theme, s.dataTheme(slug, m))
	m.Theme = cmp.Or(before.Picked, given, before.Created, kit.DefaultTheme)
	m.GivenTheme = given
	return s.WriteMeta(slug, m)
}

// dataTheme is the theme: of the data file the page slug, whose meta.json is m, is drawn from, its copy's or the
// linked one: "" for none, or one that cannot be read.
func (s *Store) dataTheme(slug string, m *Meta) string {
	file, ok := s.File(slug, m, "")
	if !m.IsLink() {
		var dir string
		dir, ok = s.copyDir(slug, m)
		file = filepath.Join(dir, dataName)
	}
	if !ok {
		return ""
	}
	src, _, err := files.Read(file)
	if err != nil {
		return ""
	}
	return writtenTheme(file, src)
}
