package store

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/sukekyo26/floe/internal/link"
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
	Theme       *string
	Force       bool
}

// Entry is a checked publish, ready to be saved.
type Entry struct {
	Slug string
	Meta Meta
	// Source is the data file of a stored page, and Images the images it shows, by the path it names
	// them by; nil for a link.
	Source []byte
	Images map[string][]byte
	// Prev is the entry this one overwrites, nil for a new slug.
	Prev *Meta

	// What settle builds Meta from, with the link's path, the data's title:, the theme and the
	// templates of the Meta it replaces: the publish's options and the project published from.
	opts Options
	from string
}

// Prepare checks a publish of file and builds what Save writes, without writing anything. cwd
// resolves a relative file and names the project of a stored page; a link's project is that of its
// directory.
func (s *Store) Prepare(ctx context.Context, file string, opts Options, cwd string) (*Entry, error) {
	// A link is of the kind of its real file, whatever the name of a symlink to it says (see read).
	if !opts.Link {
		if err := checkKind(file, opts); err != nil {
			return nil, err
		}
	}
	path, src, err := s.read(file, opts, cwd)
	if err != nil {
		return nil, err
	}
	theme := ""
	if opts.Theme != nil {
		theme = *opts.Theme
	}
	data, err := s.readData(file, path, src, theme, cwd)
	if err != nil {
		return nil, err
	}
	own := ""
	if data != nil {
		own = data.Title
	}
	slug, err := s.slugFor(file, path, opts, cwd)
	if err != nil {
		return nil, err
	}
	dir := cwd
	if path != "" {
		dir = filepath.Dir(path)
	}
	var seed Meta
	seed.Path, seed.DataTitle = path, own
	if data != nil {
		seed.Theme, seed.Templates = theme, s.templates.Dir()
	}
	e := &Entry{Slug: slug, Meta: seed, Source: nil, Images: nil, Prev: nil, opts: opts, from: project.Name(ctx, dir)}
	// Checked against the slug as it is now, to fail before drawing; Save checks again.
	if err = e.settle(s.ReadMeta(slug)); err != nil {
		return nil, err
	}
	if data != nil {
		if err = e.draw(data, src); err != nil {
			return nil, err
		}
	}
	return e, nil
}

// readData reads and checks the data file file, of content src, if it is one: nil otherwise. A linked
// one, of the real path path, is checked as a stored page is, its images under its real directory.
func (s *Store) readData(file, path, src, theme, cwd string) (*tmpl.Data, error) {
	// A link is of the kind of its real file, whatever the name of a symlink to it says.
	name := cmp.Or(path, file)
	if !IsDataFile(name) {
		return nil, nil //nolint:nilnil // a page, which is linked as it is
	}
	data, err := s.templates.Read(name, []byte(src), theme, cwd)
	if err != nil {
		return nil, err //nolint:wrapcheck // mistakes at their lines and columns, for the caller to show
	}
	return data, nil
}

// settle checks e against prev, the entry its slug holds (nil for none), and builds its meta.json on
// it. A replaced page or link keeps nothing of the old one.
func (e *Entry) settle(prev *Meta) error {
	was := e.Meta
	replacing, err := checkOwner(e.Slug, prev, was.Path, e.from, e.opts.Force)
	if err != nil {
		return err
	}
	base := prev
	if replacing || base == nil {
		var none Meta
		base = &none
	}
	e.Meta = newMeta(e.opts, was.Path, e.from, base, was.DataTitle, e.Slug)
	e.Meta.Theme, e.Meta.Templates = was.Theme, was.Templates
	e.Prev = prev
	return nil
}

// draw checks that data, of content src, draws, and keeps for a stored page src and its images.
func (e *Entry) draw(data *tmpl.Data, src string) error {
	if !e.Meta.IsLink() {
		e.Source = []byte(src)
		var err error
		if e.Images, err = data.Images(); err != nil {
			return err //nolint:wrapcheck // floe's own encoding, not the user's
		}
	}
	_, err := drawData(data, &e.Meta, e.Slug)
	return err
}

// read returns the real path of a link ("" for a stored page) and the file's content. A link is
// checked and stored by its real path: a symlink would otherwise reach into a hidden directory. It is
// of the kind of its real file, whatever the name of a symlink to it says.
func (s *Store) read(file string, opts Options, cwd string) (path, content string, err error) {
	abs := file
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(cwd, abs)
	}
	if opts.Link {
		if path, err = filepath.EvalSymlinks(abs); err != nil {
			return "", "", usererr.CannotRead(file, err)
		}
		if err = link.Check(path, s.home); err != nil {
			return "", "", err //nolint:wrapcheck // a usererr for the caller to show
		}
		if err = checkKind(path, opts); err != nil {
			return "", "", err
		}
		abs = path
	}
	b, err := os.ReadFile(abs) //nolint:gosec // G304: publishing the user's file is the point
	if err != nil {
		return "", "", usererr.CannotRead(file, err)
	}
	return path, string(b), nil
}

// checkKind checks that opts.Theme fits the kind of file: a page is drawn from a data file in
// opts.Theme, stored or linked; HTML is only linked, and shown as it is, in its own look. A link's file
// is its real path.
func checkKind(file string, opts Options) error {
	switch {
	case !opts.Link && !IsDataFile(file):
		return usererr.New("store_not_data", file)
	case opts.Link && !IsDataFile(file) && opts.Theme != nil:
		return usererr.New("store_theme_link")
	}
	return nil
}

// newMeta keeps what was set on the management page (favourite, title, project), the description and
// createdAt from base; --title and --description win.
func newMeta(opts Options, path, from string, base *Meta, own, slug string) Meta {
	keepTitle := opts.Title == nil && base.TitleEdited
	// One reading of the clock, so that a new page's createdAt equals its updatedAt.
	now := Now()
	m := Meta{
		Path:          path,
		Title:         title(opts, path, keepTitle, base, own, slug),
		TitleEdited:   keepTitle,
		Description:   base.Description,
		Theme:         "",
		DataTitle:     own,
		Templates:     "",
		Source:        "",
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
	if base.ProjectEdited {
		m.Project = base.Project
	}
	if path == "" {
		m.PublishedFrom = from
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
// replaces one. A stored page belongs to the project it was published from. A link belongs to its
// path: the same slug for another path certainly means another file.
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

// Save writes a prepared entry. It checks the entry again against the slug as it is then, as an edit,
// another publish or a rename may have changed it since Prepare: what was set on the management page
// stays, and a page of another project is still refused. A stored page's data file and images go to
// a new directory, which meta.json then names, so that a save that fails leaves the old copy; the
// copies it no longer names go after.
// LIMIT: a drawing that read meta.json just before a save may find its copy gone, and shows that it
// cannot be read until the tab's next look. Keep the copy before until the next save if that shows.
func (s *Store) Save(e *Entry) error {
	unlock, err := s.lock(true)
	if err != nil {
		return fmt.Errorf("save %s: %w", e.Slug, err)
	}
	defer unlock()
	if err = e.settle(s.ReadMeta(e.Slug)); err != nil {
		return err
	}
	dir := s.dir(e.Slug)
	made := os.Mkdir(dir, 0o700) == nil
	if err = s.write(e, dir); err != nil {
		// The directory this save made, left, would keep a rename off the slug from the management
		// page, which does not list it. One that was there (its meta.json unreadable) stays.
		if made {
			_ = os.RemoveAll(dir) //nolint:errcheck // the error is the save's
		}
		return err
	}
	removeSources(dir, e.Meta.Source)
	return nil
}

// write writes e to dir, the directory of its slug: the copy of a stored page, then meta.json.
func (s *Store) write(e *Entry, dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("save %s: %w", e.Slug, err)
	}
	if !e.Meta.IsLink() {
		var err error
		if e.Meta.Source, err = writeSource(dir, e.Source, e.Images); err != nil {
			return fmt.Errorf("save %s: %w", e.Slug, err)
		}
	}
	return s.WriteMeta(e.Slug, &e.Meta)
}

// lock waits to hold the store alone among saves, edits and removals, in this process or another, and
// returns what lets it go. One lock for the store holds a slug not made yet, and the slug a rename
// takes, as well. The lock file is made when missing; the store too with create, else a store not
// made yet fails with fs.ErrNotExist, as it holds no page.
// LIMIT: saves of different pages wait for each other too, a few ms each, and all wait while a
// process holding it is stopped (Ctrl-Z). Lock each slug, and each name a rename takes, if a store
// comes to see many saves at once.
func (s *Store) lock(create bool) (func(), error) {
	if create {
		if err := os.MkdirAll(s.root, 0o700); err != nil {
			return nil, err //nolint:wrapcheck // the caller says what it locked for
		}
	} else if _, err := os.Stat(s.root); err != nil {
		return nil, err //nolint:wrapcheck // as above
	}
	f, err := os.OpenFile(filepath.Join(s.root, ".lock"), os.O_RDONLY|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err //nolint:wrapcheck // as above
	}
	if err = unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil { //nolint:gosec // G115: a descriptor fits an int
		_ = f.Close()   //nolint:errcheck // nothing written
		return nil, err //nolint:wrapcheck // as above
	}
	// Closing it lets the lock go.
	return func() { _ = f.Close() }, nil //nolint:errcheck // as above
}

// writeSource writes the data file src and its images, at the paths it names them by, to a new
// directory under dir, and returns its name. One left by a write that fails goes.
func writeSource(dir string, src []byte, images map[string][]byte) (string, error) {
	tmp, err := os.MkdirTemp(dir, sourcePrefix)
	if err != nil {
		return "", err //nolint:wrapcheck // Save says what it saved
	}
	if err = writeCopy(tmp, src, images); err != nil {
		_ = os.RemoveAll(tmp) //nolint:errcheck // the next save removes it otherwise
		return "", err
	}
	return filepath.Base(tmp), nil
}

// writeCopy writes the data file src and its images under dir.
func writeCopy(dir string, src []byte, images map[string][]byte) error {
	if err := writeUnder(dir, tmpl.KeptName, src); err != nil {
		return err
	}
	for name, b := range images {
		if err := writeUnder(dir, name, b); err != nil {
			return err
		}
	}
	return nil
}

// writeUnder writes b to name, a slash-separated path, under dir, making the directories on its way.
func writeUnder(dir, name string, b []byte) error {
	p := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err //nolint:wrapcheck // Save says what it saved
	}
	return os.WriteFile(p, b, 0o600) //nolint:wrapcheck // as above
}

// removeSources removes the copies under dir but keep ("" for none): the ones a save replaced, or
// left when it stopped halfway. The save is done by then: one that cannot go is left for the next.
func removeSources(dir, keep string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, d := range entries {
		if name := d.Name(); strings.HasPrefix(name, sourcePrefix) && name != keep {
			_ = os.RemoveAll(filepath.Join(dir, name)) //nolint:errcheck // as above
		}
	}
}

// Remove deletes slug. A linked file itself is left alone.
func (s *Store) Remove(slug string) error {
	if !ValidSlug(slug) {
		return usererr.New("store_no_page", slug)
	}
	unlock, err := s.lock(false)
	if errors.Is(err, fs.ErrNotExist) {
		return usererr.New("store_no_page", slug)
	}
	if err != nil {
		return fmt.Errorf("remove %s: %w", slug, err)
	}
	defer unlock()
	// A removal that waited for another of the same slug finds nothing, as one that came after.
	if s.ReadMeta(slug) == nil {
		return usererr.New("store_no_page", slug)
	}
	if err = os.RemoveAll(s.dir(slug)); err != nil {
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
// the list nor the open tabs move. Clearing the title hands it back to the file (a link) or to the
// data file's title: when published (a stored page), else to the slug; clearing the project drops it.
func (s *Store) Edit(slug string, ed Edit) (string, error) {
	unlock, err := s.lock(false)
	if errors.Is(err, fs.ErrNotExist) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("edit %s: %w", slug, err)
	}
	defer unlock()
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
		setTitle(to, m, *ed.Title)
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

// setTitle sets the title of m, whose slug the edit renames to: an empty one gives back the page's
// own, or to.
func setTitle(to string, m *Meta, t string) {
	m.Title = strings.TrimSpace(t)
	m.TitleEdited = m.Title != ""
	if m.Title == "" && !m.IsLink() {
		// The drawn page's <title> is the title it was published under, pinned ones included.
		m.Title = cmp.Or(m.DataTitle, to)
	}
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
