package store

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/link"
	"github.com/sukekyo26/floe/internal/page"
	"github.com/sukekyo26/floe/internal/project"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

var (
	indexRe    = regexp.MustCompile(`(?i)^index\.html?$`)
	nonSlugRe  = regexp.MustCompile(`[^a-z0-9]+`)
	maxSlugLen = 64
)

// SlugFromFile derives a slug from a file name. index.html does not name anything, so it takes the
// name of its directory instead; a relative file is resolved against cwd.
func SlugFromFile(file, cwd string) string {
	name := filepath.Base(file)
	if indexRe.MatchString(name) {
		if !filepath.IsAbs(file) {
			file = filepath.Join(cwd, file)
		}
		name = filepath.Base(filepath.Dir(filepath.Clean(file)))
	} else {
		name = strings.TrimSuffix(name, filepath.Ext(name))
	}
	slug := nonSlugRe.ReplaceAllString(strings.ToLower(name), "-")
	return strings.Trim(slug[:min(len(slug), maxSlugLen)], "-")
}

// freeSlug names a link registered without a slug: the entry of the same path whatever its slug,
// else base, else base-2, base-3 … so that two index.html files never take over each other.
func (s *Store) freeSlug(base, path string) string {
	if dirents, err := os.ReadDir(s.root); err == nil {
		for _, d := range dirents {
			if m := s.ReadMeta(d.Name()); ValidSlug(d.Name()) && m != nil && m.Path == path {
				return d.Name()
			}
		}
	}
	for n := 1; ; n++ {
		suffix := ""
		if n > 1 {
			suffix = "-" + strconv.Itoa(n)
		}
		slug := strings.TrimRight(base[:min(len(base), maxSlugLen-len(suffix))], "-") + suffix
		if s.ReadMeta(slug) == nil {
			return slug
		}
	}
}

// Options are the choices of a publish. A nil string is not given; an empty one is.
type Options struct {
	Link        bool
	Slug        *string
	Title       *string
	Description *string
	Lang        *string
	Theme       *string
	Force       bool
}

// Entry is a checked publish, ready to be saved.
type Entry struct {
	Slug string
	Meta Meta
	// Page is the stored HTML, drawn from the data file; nil for a link.
	Page []byte
	// Prev is the entry this one overwrites, nil for a new slug.
	Prev *Meta
}

// Prepare checks a publish of file and builds what Save writes, without writing anything. cwd
// resolves a relative file and names the project of a stored page; a link's project is that of its
// directory.
func (s *Store) Prepare(ctx context.Context, file string, opts Options, cwd string) (*Entry, error) {
	if err := checkKind(file, opts); err != nil {
		return nil, err
	}
	path, src, err := s.read(file, opts.Link, cwd)
	if err != nil {
		return nil, err
	}
	var data *tmpl.Data
	own := ""
	if path == "" {
		theme := ""
		if opts.Theme != nil {
			theme = *opts.Theme
		}
		if data, err = s.templates.Read(file, []byte(src), theme); err != nil {
			return nil, err //nolint:wrapcheck // mistakes at their lines and columns, for the caller to show
		}
		own = data.Title
	}
	slug, err := s.slugFor(file, path, opts, cwd)
	if err != nil {
		return nil, err
	}
	prev := s.ReadMeta(slug)
	dir := cwd
	if path != "" {
		dir = filepath.Dir(path)
	}
	from := project.Name(ctx, dir)
	replacing, err := checkOwner(slug, prev, path, from, opts.Force)
	if err != nil {
		return nil, err
	}
	// A replaced page or link keeps nothing of the old one.
	base := prev
	if replacing || base == nil {
		var none Meta
		base = &none
	}
	e := &Entry{Slug: slug, Meta: newMeta(opts, path, from, base, own, slug), Page: nil, Prev: prev}
	if data != nil {
		e.Meta.Template, e.Meta.Theme = data.Template, data.Theme
		if e.Page, err = s.templates.Render(data, e.Meta.Title, e.Meta.Lang); err != nil {
			return nil, err //nolint:wrapcheck // a usererr for the caller to show
		}
	}
	return e, nil
}

// read returns the real path of a link ("" for a stored page) and the file's content. A link is
// checked and stored by its real path: a symlink would otherwise reach into a hidden directory.
func (s *Store) read(file string, isLink bool, cwd string) (path, content string, err error) {
	abs := file
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(cwd, abs)
	}
	if isLink {
		if path, err = filepath.EvalSymlinks(abs); err != nil {
			return "", "", usererr.CannotRead(file, err)
		}
		if err = link.Check(path, s.home); err != nil {
			return "", "", err //nolint:wrapcheck // a usererr for the caller to show
		}
		abs = path
	}
	b, err := os.ReadFile(abs) //nolint:gosec // G304: publishing the user's file is the point
	if err != nil {
		return "", "", usererr.CannotRead(file, err)
	}
	return path, string(b), nil
}

// checkKind checks that file, opts.Lang and opts.Theme fit the kind of publish, before file is read. A
// stored page is drawn from a data file in opts.Lang and opts.Theme; HTML is only linked, and shown as
// it is, in its own language and look.
func checkKind(file string, opts Options) error {
	switch {
	case !opts.Link && !IsDataFile(file):
		return usererr.New("store_not_data", file)
	case opts.Link && opts.Lang != nil:
		return usererr.New("store_lang_link")
	case opts.Link && opts.Theme != nil:
		return usererr.New("store_theme_link")
	case opts.Lang != nil && !i18n.IsTag(*opts.Lang):
		return usererr.New("lang_invalid", *opts.Lang)
	}
	return nil
}

// newMeta keeps what was set on the management page (favourite, title, project), the description, the
// language and createdAt from base; --title, --description and --lang win. A link has no language.
func newMeta(opts Options, path, from string, base *Meta, own, slug string) Meta {
	keepTitle := opts.Title == nil && base.TitleEdited
	// One reading of the clock, so that a new page's createdAt equals its updatedAt.
	now := Now()
	m := Meta{
		Path:          path,
		Title:         title(opts, path, keepTitle, base, own, slug),
		TitleEdited:   keepTitle,
		Description:   base.Description,
		Lang:          base.Lang,
		Template:      "",
		Theme:         "",
		DataTitle:     own,
		Project:       from,
		ProjectEdited: base.ProjectEdited,
		PublishedFrom: "",
		Favorite:      base.Favorite,
		CreatedAt:     cmp.Or(base.CreatedAt, now),
		UpdatedAt:     now,
	}
	if opts.Description != nil {
		m.Description = *opts.Description
	}
	if opts.Lang != nil {
		m.Lang = *opts.Lang
	}
	if base.ProjectEdited {
		m.Project = base.Project
	}
	if path == "" {
		m.PublishedFrom = from
	} else {
		m.Lang = ""
	}
	return m
}

func (s *Store) slugFor(file, path string, opts Options, cwd string) (string, error) {
	slug := ""
	switch {
	case opts.Slug != nil:
		slug = *opts.Slug
	case path != "":
		// Named after the given file, not the symlink's target.
		slug = s.freeSlug(SlugFromFile(file, cwd), path)
	default:
		slug = SlugFromFile(file, cwd)
	}
	if !ValidSlug(slug) {
		return "", usererr.New("store_invalid_slug", slug)
	}
	return slug, nil
}

// checkOwner refuses to overwrite another's entry without force, and reports whether the publish
// replaces one. A stored page belongs to the project it was published from (pages from before
// publishedFrom fall back to project, unless that was edited and so names no place). A link belongs
// to its path: the same slug for another path certainly means another file.
func checkOwner(slug string, prev *Meta, path, from string, force bool) (bool, error) {
	if prev == nil {
		return false, nil
	}
	if prev.IsLink() != (path != "") {
		key := "store_slug_is_page"
		if prev.IsLink() {
			key = "store_slug_is_link"
		}
		return false, usererr.New(key, slug)
	}
	if path != "" {
		if prev.Path == path {
			return false, nil
		}
		if force {
			return true, nil
		}
		err := usererr.New("store_slug_linked", slug, prev.Path)
		err.Conflict = true
		return false, err
	}
	owner := prev.PublishedFrom
	if owner == "" && !prev.ProjectEdited {
		owner = prev.Project
	}
	if owner == "" || owner == from {
		return false, nil
	}
	if force {
		return true, nil
	}
	return false, usererr.New("store_slug_owned", slug, owner)
}

// title is --title, else the title pinned on the management page, else (for a stored page) the data's
// title:, the previous one, or the slug. A link records no title of its
// own: it is read from the file each time.
func title(opts Options, path string, keepTitle bool, base *Meta, own, slug string) string {
	switch {
	case opts.Title != nil:
		return *opts.Title
	case path != "" || keepTitle:
		return base.Title
	default:
		// As clearing the title on the management page does: the data file's title:, else the slug.
		return cmp.Or(own, slug)
	}
}

// IsDataFile reports whether file is a data file (.yaml or .yml) rather than a page.
func IsDataFile(file string) bool {
	ext := strings.ToLower(filepath.Ext(file))
	return ext == ".yaml" || ext == ".yml"
}

// Save writes a prepared entry.
func (s *Store) Save(e *Entry) error {
	if err := os.MkdirAll(s.dir(e.Slug), 0o700); err != nil {
		return fmt.Errorf("save %s: %w", e.Slug, err)
	}
	if e.Page != nil {
		if err := writeAtomic(filepath.Join(s.dir(e.Slug), "index.html"), e.Page); err != nil {
			return err
		}
	}
	return s.WriteMeta(e.Slug, &e.Meta)
}

// Remove deletes slug. A linked file itself is left alone.
func (s *Store) Remove(slug string) error {
	if !ValidSlug(slug) || s.ReadMeta(slug) == nil {
		return usererr.New("store_no_page", slug)
	}
	if err := os.RemoveAll(s.dir(slug)); err != nil {
		return fmt.Errorf("remove %s: %w", slug, err)
	}
	return nil
}

// Edit is a change from the management page. A nil field is left as it is.
type Edit struct {
	Title       *string
	Description *string
	Project     *string
	Slug        *string
	Favorite    *bool
}

// Edit applies ed to slug and returns its slug afterwards. updatedAt stays, so neither the order of
// the list nor the open tabs move. Clearing the title hands it back to the file (a link), to the
// data file's title: (a page drawn from one) or to the <title> or first <h1> of a page stored as HTML,
// else to the slug; clearing the project drops it.
func (s *Store) Edit(slug string, ed Edit) (string, error) {
	m := s.ReadMeta(slug)
	if m == nil {
		return "", ErrNotFound
	}
	if ed.Description != nil {
		m.Description = strings.TrimSpace(*ed.Description)
	}
	if ed.Favorite != nil {
		m.Favorite = *ed.Favorite
	}
	if ed.Project != nil {
		// "/" is the management page's value for "no project".
		if strings.TrimSpace(*ed.Project) == "/" {
			return "", usererr.New("store_project_slash")
		}
		m.Project = strings.TrimSpace(*ed.Project)
		m.ProjectEdited = m.Project != ""
	}
	to, err := s.renameTo(slug, ed.Slug)
	if err != nil {
		return "", err
	}
	if ed.Title != nil {
		s.setTitle(slug, to, m, *ed.Title)
	}
	if err := s.WriteMeta(slug, m); err != nil {
		return "", err
	}
	// Renaming moves the directory only; tabs open on the old URL show "not found".
	if to != slug {
		if err := os.Rename(s.dir(slug), s.dir(to)); err != nil {
			return "", fmt.Errorf("rename %s to %s: %w", slug, to, err)
		}
	}
	return to, nil
}

// setTitle sets the title of slug, which the edit renames to: an empty one gives back the page's own,
// or to.
func (s *Store) setTitle(slug, to string, m *Meta, t string) {
	m.Title = strings.TrimSpace(t)
	m.TitleEdited = m.Title != ""
	if m.Title != "" || m.IsLink() {
		return
	}
	if m.Template != "" {
		// The drawn page's <title> is the title it was published under, pinned ones included.
		m.Title = cmp.Or(m.DataTitle, to)
		return
	}
	// A page stored as HTML before pages were drawn from data files.
	own, _ := os.ReadFile(filepath.Join(s.dir(slug), "index.html")) //nolint:errcheck // unreadable means untitled
	m.Title = cmp.Or(page.Title(string(own)), to)
}

// renameTo checks the new slug of an edit: nil keeps slug.
func (s *Store) renameTo(slug string, to *string) (string, error) {
	if to == nil {
		return slug, nil
	}
	name := strings.TrimSpace(*to)
	if !ValidSlug(name) {
		return "", usererr.New("store_invalid_slug", name)
	}
	if _, err := os.Lstat(s.dir(name)); name != slug && !errors.Is(err, os.ErrNotExist) {
		err := usererr.New("store_slug_taken", name)
		err.Conflict = true
		return "", err
	}
	return name, nil
}
