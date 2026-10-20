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
	"testing/fstest"

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
	Slug        *string
	Title       *string
	Description *string
	Theme       *string
	PartsTheme  *string
	Force       bool
}

// themes are the themes o gives over the data's.
func (o Options) themes() tmpl.Themes {
	var themes tmpl.Themes
	if o.Theme != nil {
		themes.Template = *o.Theme
	}
	if o.PartsTheme != nil {
		themes.Parts = *o.PartsTheme
	}
	return themes
}

// Entry is a checked publish, ready to be saved.
type Entry struct {
	Slug string
	// Meta is what Save writes as meta.json. Save builds it again on the slug as it is then, so a value
	// set here between Prepare and Save does not stay.
	Meta Meta
	// Source is the data file of a stored page, and Images the images it shows, by the path it names
	// them by; nil for a link.
	Source []byte
	Images map[string][]byte
	// Prev is the entry this one overwrites, nil for a new slug.
	Prev *Meta

	// What settle builds Meta from, with the link's path, the data's title: and the themes of the Meta
	// it replaces: the publish's options and the project published from.
	opts Options
	from string
	// named is what a link without --slug is named after, for Save to pick its slug again: "" else.
	named string
}

// Upload is a data file read for a publish: what PrepareData checks and builds a stored page of, without
// reading the publisher's disk.
type Upload struct {
	File   string // as the publisher named it, for its slug and its mistakes
	Dir    string // its directory, as the mistakes name it
	Source []byte
	// Images is the directory of the data file, as an fs.FS that lists its directories; nil asks which
	// images the data names (NeedImages).
	Images  fs.FS
	Project string // the project it is published from, which owns the page
}

// NeedImages answers an Upload without images whose data names some: Names, as it writes them.
type NeedImages struct {
	Names []string
}

func (e *NeedImages) Error() string {
	return "the data names images to be sent with it: " + strings.Join(e.Names, ", ")
}

// Prepare checks a publish of the data file file on disk, as PrepareData does, with its images under
// its directory. cwd resolves a relative file and names the project the page belongs to.
func (s *Store) Prepare(ctx context.Context, file string, opts Options, cwd string) (*Entry, error) {
	// Before reading, as PrepareData would refuse it after.
	if !IsDataFile(file) {
		return nil, usererr.New("store_not_data", file)
	}
	abs := file
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(cwd, abs)
	}
	src, err := os.ReadFile(abs) //nolint:gosec // G304: publishing the user's file is the point
	if err != nil {
		return nil, usererr.CannotRead(file, err)
	}
	up := Upload{File: file, Dir: filepath.Dir(abs), Source: src, Images: nil, Project: project.Name(ctx, cwd)}
	e, err := s.PrepareData(up, opts)
	var need *NeedImages
	if !errors.As(err, &need) {
		return e, err
	}
	// Opened only when the data names an image, as a drawing of a link opens it.
	root, err := os.OpenRoot(up.Dir)
	if err != nil {
		return nil, usererr.CannotRead(up.Dir, err)
	}
	defer root.Close()
	up.Images = root.FS()
	return s.PrepareData(up, opts)
}

// PrepareData checks a publish of the data file up as a stored page and builds what Save writes,
// without writing anything. Without up.Images, a data file that names images is answered NeedImages.
func (s *Store) PrepareData(up Upload, opts Options) (*Entry, error) {
	if !IsDataFile(up.File) {
		return nil, usererr.New("store_not_data", up.File)
	}
	images := up.Images
	if images == nil {
		images = fstest.MapFS{}
	}
	themes := opts.themes()
	data, named, err := s.templates.ReadFS(up.File, up.Source, images, up.Dir, themes)
	// Asked before its mistakes, which may be of the images not sent.
	if up.Images == nil && len(named) > 0 {
		return nil, &NeedImages{Names: named}
	}
	if err != nil {
		return nil, err //nolint:wrapcheck // mistakes at their lines and columns, for the caller to show
	}
	slug, err := s.slugFor(up.File, "", opts, "")
	if err != nil {
		return nil, err
	}
	return s.checked(&Entry{
		Slug: slug, Meta: dataMeta("", data, themes), Source: nil, Images: nil, Prev: nil, opts: opts,
		from: up.Project, named: "",
	}, data, string(up.Source))
}

// PrepareLink checks a link to file and builds what Save writes, without writing anything. cwd resolves
// a relative file; the project of a link is that of its directory. A data file linked is checked as a
// stored page is, its images under its real directory.
func (s *Store) PrepareLink(ctx context.Context, file string, opts Options, cwd string) (*Entry, error) {
	path, src, err := s.readLink(file, opts, cwd)
	if err != nil {
		return nil, err
	}
	themes := opts.themes()
	var data *tmpl.Data
	if IsDataFile(path) {
		if data, err = s.templates.Read(path, src, themes, cwd); err != nil {
			return nil, err //nolint:wrapcheck // mistakes at their lines and columns, for the caller to show
		}
	}
	slug, err := s.slugFor(file, path, opts, cwd)
	if err != nil {
		return nil, err
	}
	named := ""
	if opts.Slug == nil {
		named = SlugFromFile(file, cwd)
	}
	return s.checked(&Entry{
		Slug: slug, Meta: dataMeta(path, data, themes), Source: nil, Images: nil, Prev: nil, opts: opts,
		from: project.Name(ctx, filepath.Dir(path)), named: named,
	}, data, string(src))
}

// dataMeta is the seed of the Meta settle builds: the link's path ("" for a stored page), and the title:
// and themes of data, nil for HTML.
func dataMeta(path string, data *tmpl.Data, themes tmpl.Themes) Meta {
	var m Meta
	m.Path = path
	if data != nil {
		m.DataTitle = data.Title
		m.Theme, m.PartsTheme = themes.Template, themes.Parts
	}
	return m
}

// checked checks e against its slug as it is now, to fail before drawing (Save checks again), then that
// data, of content src, draws, nil for HTML.
func (s *Store) checked(e *Entry, data *tmpl.Data, src string) (*Entry, error) {
	if err := e.settle(s.ReadMeta(e.Slug)); err != nil {
		return nil, err
	}
	if data != nil {
		if err := e.draw(data, src); err != nil {
			return nil, err
		}
	}
	return e, nil
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
	e.Meta.Theme, e.Meta.PartsTheme = was.Theme, was.PartsTheme
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

// readLink returns the real path of the linked file and its content. A link is checked and stored by
// its real path: a symlink would otherwise reach into a hidden directory. It is of the kind of its real
// file, whatever the name of a symlink to it says: HTML is shown as it is, in its own look, so it takes
// no theme.
func (s *Store) readLink(file string, opts Options, cwd string) (string, []byte, error) {
	abs := file
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(cwd, abs)
	}
	path, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", nil, usererr.CannotRead(file, err)
	}
	if err = link.Check(path, s.home); err != nil {
		return "", nil, err //nolint:wrapcheck // a usererr for the caller to show
	}
	if !IsDataFile(path) && (opts.Theme != nil || opts.PartsTheme != nil) {
		return "", nil, usererr.New("store_theme_link")
	}
	b, err := os.ReadFile(path) //nolint:gosec // G304: publishing the user's file is the point
	if err != nil {
		return "", nil, usererr.CannotRead(file, err)
	}
	return path, b, nil
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
		PartsTheme:    "",
		DataTitle:     own,
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
	// Picked again, as another link may have taken the slug, or the file's own been renamed, since.
	if e.named != "" {
		e.Slug = s.freeSlug(e.named, e.Meta.Path)
	}
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
// LIMIT: saves of different pages wait for each other too, while the copy of a page and its images
// is written (a few ms for the plan pages here), and all wait while a process holding it is stopped
// (Ctrl-Z). Lock each slug, and each name a rename takes, if a store
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
