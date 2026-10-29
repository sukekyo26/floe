package store

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing/fstest"
	"time"

	"golang.org/x/sys/unix"

	"github.com/sukekyo26/floe/internal/dirs"
	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/files"
	"github.com/sukekyo26/floe/internal/link"
	"github.com/sukekyo26/floe/internal/project"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

var indexRe = regexp.MustCompile(`(?i)^index\.html?$`)

// linkName is the slug a link to file from cwd is named after: index.html does not name anything, so it takes
// the name of its directory instead.
func linkName(file, cwd string) string {
	if !indexRe.MatchString(filepath.Base(file)) {
		return files.SlugFromFile(file)
	}
	if abs, ok := dirs.Under(cwd, file); ok {
		file = abs
	}
	return files.Slug(filepath.Base(filepath.Dir(filepath.Clean(file))))
}

// freeSlug names a link registered without a slug: the entry of the same path whatever its slug,
// else base, else base-2, base-3 … so that two index.html files never take over each other.
func (s *Store) freeSlug(base, path string) string {
	if slugs, err := s.slugs(); err == nil {
		for _, slug := range slugs {
			if m := s.ReadMeta(slug); m != nil && m.Path == path {
				return slug
			}
		}
	}
	for n := 1; ; n++ {
		suffix := ""
		if n > 1 {
			suffix = "-" + strconv.Itoa(n)
		}
		slug := strings.TrimRight(base[:min(len(base), files.MaxSlugLen-len(suffix))], "-") + suffix
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
	Force       bool
}

// Accept checks a publish of file, a link or a data file, for what needs neither the store nor the disk,
// and returns o with each text given trimmed, whoever gave it, and a data file's slug made of its name
// when none is given. A slug is a name, checked as given: trimmed, it would not be the one a URL names.
// A title or a theme given empty is not given: the data's, or the one kept, is taken. Its mistakes: a data
// file empty or that is not one, and a slug that cannot be one. The command line checks with it before it
// sends, and the server again.
func Accept(file string, link bool, o Options) (Options, error) {
	for _, field := range []**string{&o.Title, &o.Description, &o.Theme} {
		if *field != nil {
			v := strings.TrimSpace(**field)
			*field = &v
		}
	}
	for _, field := range []**string{&o.Title, &o.Theme} {
		if *field != nil && **field == "" {
			*field = nil
		}
	}
	var found usererr.List
	switch {
	case link: // an empty link is readLink's, as it reads it
	case file == "":
		found = append(found, usererr.New("store_file_empty"))
	case !files.IsDataFile(file):
		found = append(found, usererr.New("store_not_data", file))
	}
	// Made of the name of a data file only: a stored page is drawn from one.
	if o.Slug == nil && !link && len(found) == 0 {
		slug := files.SlugFromFile(file)
		o.Slug = &slug
	}
	if o.Slug != nil {
		if mistake := slugMistake(*o.Slug); mistake != nil {
			found = append(found, mistake)
		}
	}
	return o, found.Err()
}

// theme is the theme o gives over the data's, "" for none.
func (o Options) theme() string {
	if o.Theme == nil {
		return ""
	}
	return *o.Theme
}

// Entry is a checked publish, ready to be saved.
type Entry struct {
	Slug string
	// Meta is what Save writes as meta.json. Save builds it again on the slug as it is then, so a value
	// set here between PrepareData or PrepareLink and Save does not stay.
	Meta Meta
	// Source is the data file of a stored page, Page the page drawn from it, and Images the images it read, by
	// the path it names them by; nil for a link.
	Source []byte
	Page   []byte
	Images map[string][]byte
	// Prev is the entry this one overwrites, nil for a new slug.
	Prev *Meta
	// Warnings are those of the page drawn, which Save does not keep.
	Warnings []*usererr.Error

	// What settle builds Meta from, with the link's path, the data's title: and the theme of the Meta
	// it replaces: the publish's options and the project published from.
	opts Options
	from string
	// named is what a link without --slug is named after, for Save to pick its slug again: "" else.
	named string
}

// Upload is a data file sent for a publish: what PrepareData checks and builds a stored page of, without
// reading the publisher's disk.
type Upload struct {
	File   string // as the publisher named it, for its slug and its mistakes
	Source []byte
	// Images are the images sent with it, by the path the data names them by; nil for none sent, which asks
	// which images the data reads (NeedImages). An empty one is sent.
	Images  map[string][]byte
	Project string // the project it is published from, which owns the page
}

// NeedImages answers an Upload without images whose data reads some: Names, as it writes them.
type NeedImages struct {
	Names []string
}

func (e *NeedImages) Error() string {
	return "the data names images to be sent with it: " + strings.Join(e.Names, ", ")
}

// recorder notes each name read of fsys, with Stat as with Open: kit.ReadImage stats an image before it opens
// it, and an image that is not there fails the Stat.
type recorder struct {
	fsys  fs.FS
	names []string
	seen  map[string]bool
}

func (r *recorder) note(name string) {
	if !r.seen[name] {
		r.seen[name] = true
		r.names = append(r.names, name)
	}
}

func (r *recorder) Open(name string) (fs.File, error) {
	r.note(name)
	return r.fsys.Open(name) //nolint:wrapcheck // as fsys says
}

func (r *recorder) Stat(name string) (fs.FileInfo, error) {
	r.note(name)
	return fs.Stat(r.fsys, name) //nolint:wrapcheck // as fsys says
}

// PrepareData checks a publish of the data file up as a stored page, drawn with dw, and builds what Save
// writes, without writing anything or reading the publisher's disk. Without up.Images, a data file that reads
// images is answered NeedImages, once nothing that needs no image refuses the publish (the slug, the page's
// owner) and before the mistakes of its drawing, which may be of the images not sent; a mistake before its
// drawing (of the YAML, the schema, the checks) is told without asking.
func (s *Store) PrepareData(dw Drawing, up Upload, opts Options) (*Entry, error) {
	opts, err := Accept(up.File, false, opts)
	if err != nil {
		return nil, err
	}
	if dw.Err != nil {
		return nil, dw.Err
	}
	sent := fstest.MapFS{}
	for name, b := range up.Images {
		sent[name] = &fstest.MapFile{Data: b, Mode: 0, ModTime: time.Time{}, Sys: nil}
	}
	images := &recorder{fsys: sent, names: nil, seen: map[string]bool{}}
	o := tmpl.Options{Theme: opts.theme(), Untitled: *opts.Slug, Images: images, ImagesDir: "", Icons: dw.Icons}
	page, warnings, drawErr := tmpl.Draw(dw.Kit, up.File, up.Source, o)
	var mistakes usererr.List
	if failed := mistakes.Add(drawErr); failed != nil {
		return nil, failed
	}
	// LIMIT: a template changed between the two puts to read another image too is told that image is not there,
	// for this put sends the images the first asked (a map, not nil): publish again. Ask again if that comes to matter.
	if up.Images == nil && len(images.names) > 0 {
		if _, refused := s.storedPage(up.File, nil, opts, up.Project); refused != nil {
			return nil, refused
		}
		return nil, &NeedImages{Names: images.names}
	}
	if drawErr != nil {
		return nil, drawErr //nolint:wrapcheck // mistakes at their places, for the caller to show
	}
	e, err := s.storedPage(up.File, up.Source, opts, up.Project)
	if err != nil {
		return nil, usererr.WithWarnings(err, warnings) //nolint:wrapcheck // its mistakes, with the page's warnings
	}
	e.Page = page
	e.Warnings = warnings
	e.Images = map[string][]byte{}
	for _, name := range images.names {
		if b, ok := up.Images[name]; ok {
			e.Images[name] = b
		}
	}
	if err := imagesApart(up.File, e.Images); err != nil {
		return nil, usererr.WithWarnings(err, warnings) //nolint:wrapcheck // as above
	}
	return e, nil
}

// imagesApart refuses images of the data file file one of which is under another, as a.png and a.png/b.png:
// images sent are read as they are named, which a disk, where the copy goes, cannot hold. Its mistakes are at
// file, by the names.
func imagesApart(file string, images map[string][]byte) error {
	var found usererr.List
	for name := range images {
		for dir := path.Dir(name); dir != "."; dir = path.Dir(dir) {
			if _, ok := images[dir]; ok {
				found = append(found, usererr.NewAt(file, 0, 0, "store_image_under_image", name, dir))
			}
		}
	}
	return found.Sorted().Err()
}

// storedPage builds the entry of the data file file as a stored page of content src (nil checks its owner
// only), published from the project from. opts are those Accept returned without a mistake, whose Slug is set.
func (s *Store) storedPage(file string, src []byte, opts Options, from string) (*Entry, error) {
	e := &Entry{
		Slug: *opts.Slug, Meta: seedMeta("", file, src, opts.theme()), Source: src, Page: nil, Images: nil,
		Prev: nil, Warnings: nil, opts: opts, from: from, named: "",
	}
	if err := e.settle(s.ReadMeta(e.Slug)); err != nil {
		return nil, err
	}
	return e, nil
}

// PrepareLink checks a link to file and builds what Save writes, without writing anything. cwd resolves
// a relative file, "" for none to resolve it in: a relative file is then refused, with the other mistakes
// of the options. home bounds what may be linked (link.Check). The project of a link is that of its
// directory. A data file linked is drawn with dw once, as it is when opened, its images under its real
// directory, to be refused for its mistakes.
func (s *Store) PrepareLink(
	ctx context.Context, dw Drawing, file string, opts Options, cwd, home string,
) (*Entry, error) {
	opts, err := Accept(file, true, opts)
	path, src, linkErr := readLink(file, opts, cwd, home)
	if err = usererr.Collect(err, linkErr); err != nil {
		return nil, err
	}
	slug, err := s.linkSlug(file, path, opts, cwd)
	if err != nil {
		return nil, err
	}
	var data []byte
	var warnings []*usererr.Error
	if files.IsDataFile(path) {
		if dw.Err != nil {
			return nil, dw.Err
		}
		data = src
		images := files.NewRoot(filepath.Dir(path))
		o := tmpl.Options{Theme: opts.theme(), Untitled: slug, Images: images, ImagesDir: "", Icons: dw.Icons}
		_, warnings, err = tmpl.Draw(dw.Kit, path, src, o)
		_ = images.Close() //nolint:errcheck // only read from
		if err != nil {
			return nil, err //nolint:wrapcheck // mistakes at their places, for the caller to show
		}
	}
	named := ""
	if opts.Slug == nil {
		named = linkName(file, cwd)
	}
	e := &Entry{
		Slug: slug, Meta: seedMeta(path, path, data, opts.theme()), Source: nil, Page: nil, Images: nil, Prev: nil,
		Warnings: warnings, opts: opts, from: project.Name(ctx, filepath.Dir(path)), named: named,
	}
	if err := e.settle(s.ReadMeta(e.Slug)); err != nil {
		return nil, usererr.WithWarnings(err, warnings) //nolint:wrapcheck // its mistakes, with the page's warnings
	}
	return e, nil
}

// seedMeta is the seed of the Meta settle builds: the link's path ("" for a stored page), and of a page drawn
// from the data file file of content data (nil for HTML), its title: and the theme it is drawn with.
func seedMeta(path, file string, data []byte, theme string) Meta {
	var m Meta
	m.Path = path
	if data != nil {
		m.Theme = theme
		if d, _ := doc.Read(file, data, theme); d != nil { //nolint:errcheck // drawn without a mistake already
			m.DataTitle = d.Title
		}
	}
	return m
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
	e.Meta.Theme = was.Theme
	e.Prev = prev
	return nil
}

// readLink returns the real path of the linked file and its content. A link is checked and stored by
// its real path: a symlink would otherwise reach into a hidden directory. It is of the kind of its real
// file, whatever the name of a symlink to it says: HTML is shown as it is, in its own look, so it takes
// no theme.
func readLink(file string, opts Options, cwd, home string) (string, []byte, error) {
	if file == "" {
		return "", nil, usererr.New("store_link_empty")
	}
	// The API gives no cwd: a relative link from it is the sender's mistake.
	abs, ok := dirs.Under(cwd, file)
	if !ok {
		return "", nil, usererr.New("store_link_relative", file)
	}
	path, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", nil, usererr.CannotRead(file, err) //nolint:wrapcheck // the system's words, at the file
	}
	if err = link.Check(path, home); err != nil {
		return "", nil, err //nolint:wrapcheck // a usererr for the caller to show
	}
	if !files.IsDataFile(path) && opts.theme() != "" {
		return "", nil, usererr.New("store_theme_link")
	}
	b, _, err := files.Read(path)
	if err != nil {
		return "", nil, usererr.CannotRead(file, err) //nolint:wrapcheck // the system's words, at the file
	}
	return path, b, nil
}

// newMeta keeps what was set on the management page (favourite, title, project, tags), the description and
// createdAt from base, but not archived: a page published again comes back to the list. --title and
// --description win.
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
		Source:        "",
		Project:       from,
		ProjectEdited: base.ProjectEdited,
		PublishedFrom: "",
		Favorite:      base.Favorite,
		Archived:      false,
		Tags:          base.Tags,
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

// linkSlug is the slug of a link to path, given as file from cwd: the one given, which Accept checked,
// else the slug of the link to path, else one free made of file's name.
func (s *Store) linkSlug(file, path string, opts Options, cwd string) (string, error) {
	if opts.Slug != nil {
		return *opts.Slug, nil
	}
	// Named after the given file, not the symlink's target.
	slug := s.freeSlug(linkName(file, cwd), path)
	if mistake := slugMistake(slug); mistake != nil {
		return "", mistake
	}
	return slug, nil
}

// slugMistake is the mistake of slug if it cannot be one, nil else.
func slugMistake(slug string) *usererr.Error {
	if !ValidSlug(slug) {
		return usererr.New("store_invalid_slug", slug)
	}
	return nil
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
		return false, conflict(usererr.New(key, slug))
	}
	if path != "" {
		if prev.Path == path {
			return false, nil
		}
		if force {
			return true, nil
		}
		return false, conflict(usererr.New("store_slug_linked", slug, prev.Path))
	}
	owner := prev.PublishedFrom
	if owner == "" || owner == from {
		return false, nil
	}
	if force {
		return true, nil
	}
	return false, conflict(usererr.New("store_slug_owned", slug, owner))
}

// title is --title, else the title pinned on the management page, else (for a stored page) the data's
// title:, the previous one, or the slug. A link records no title of its own: it is read from the file each
// time. It names the page in the list only: the page drawn takes its data's title: (see setTitle).
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

// Save writes a prepared entry. It checks the entry again against the slug as it is then, as an edit,
// another publish or a rename may have changed it since Prepare: what was set on the management page
// stays, and a page of another project is still refused. A stored page's copy goes to a new directory,
// which meta.json then names, so that a save that fails leaves the old copy; the copies it no longer names
// go after.
// LIMIT: a reader that read meta.json just before a save may find its copy gone, and shows that it cannot be
// read until its next look. Keep the copy before until the next save if that shows.
func (s *Store) Save(e *Entry) error {
	unlock, err := s.lock(true)
	if err != nil {
		return err
	}
	defer unlock()
	// Picked again, as another link may have taken the slug, or the file's own been renamed, since.
	if e.named != "" {
		e.Slug = s.freeSlug(e.named, e.Meta.Path)
	}
	if err = e.settle(s.ReadMeta(e.Slug)); err != nil {
		return err
	}
	dir, ok := s.dir(e.Slug)
	if !ok {
		return fmt.Errorf("store: save %q, which cannot be a slug", e.Slug) //nolint:err113 // floe's own mistake
	}
	made := os.Mkdir(dir, 0o700) == nil
	if e.Meta.IsLink() {
		err = s.WriteMeta(e.Slug, &e.Meta)
	} else {
		err = s.keepCopy(e.Slug, &e.Meta, e.Source, e.Page, e.Images)
	}
	// The directory this save made, left, would keep a rename off the slug from the management page, which
	// does not list it. One that was there (its meta.json unreadable) stays.
	if err != nil && made {
		_ = os.RemoveAll(dir) //nolint:errcheck // the error is the save's
	}
	return err
}

// keepCopy writes a new copy of the stored page slug, the data file src, the page drawn from it and its
// images, then m naming it as its meta.json, then removes the copies before: what a save and a rebuild keep. A
// copy that meta.json could not come to name goes. It is called under the lock.
func (s *Store) keepCopy(slug string, m *Meta, src, page []byte, images map[string][]byte) error {
	dir, ok := s.dir(slug)
	if !ok {
		return fmt.Errorf("store: keep a copy of %q, which cannot be a slug", slug) //nolint:err113 // floe's own mistake
	}
	source, err := writeSource(dir, src, page, images)
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

// lock waits to hold the store alone among saves, edits, removals and rebuilds, in this process or another,
// and returns what lets it go. One lock for the store holds a slug not made yet, and the slug a rename
// takes, as well. The lock file is made when missing; the store too with create, else a store not made yet
// fails with fs.ErrNotExist, as it holds no page. What cannot be written is told at it.
// LIMIT: saves of different pages wait for each other too, while the copy of a page and its images
// is written (a few ms for the plan pages here), and all wait while a process holding it is stopped
// (Ctrl-Z). Lock each slug, and each name a rename takes, if a store
// comes to see many saves at once.
func (s *Store) lock(create bool) (func(), error) {
	if create {
		if err := os.MkdirAll(s.root, 0o700); err != nil {
			return nil, usererr.CannotWrite(s.root, err) //nolint:wrapcheck // the system's words, at the store
		}
	} else if _, err := os.Stat(s.root); errors.Is(err, fs.ErrNotExist) {
		return nil, err //nolint:wrapcheck // for the caller to tell as it may
	} else if err != nil {
		return nil, usererr.CannotRead(s.root, err) //nolint:wrapcheck // the system's words, at the store
	}
	file := filepath.Join(s.root, ".lock")
	f, err := os.OpenFile(file, os.O_RDONLY|os.O_CREATE, 0o600) //nolint:gosec // G304: the store's own
	if err != nil {
		return nil, usererr.CannotWrite(file, err) //nolint:wrapcheck // the system's words, at the lock
	}
	if err = unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil { //nolint:gosec // G115: a descriptor fits an int
		_ = f.Close()                              //nolint:errcheck // nothing written
		return nil, usererr.CannotWrite(file, err) //nolint:wrapcheck // the system's words, at the lock
	}
	// Closing it lets the lock go.
	return func() { _ = f.Close() }, nil //nolint:errcheck // as above
}

// writeSource writes the copy of a stored page, the data file src, the page drawn from it and its images at
// the paths it names them by under images/ (made when there are none, for a rebuild to read), to a new
// directory under dir, and returns its name. One left by a write that fails goes.
func writeSource(dir string, src, page []byte, images map[string][]byte) (string, error) {
	tmp, err := os.MkdirTemp(dir, sourcePrefix)
	if err != nil {
		return "", usererr.CannotWrite(dir, err) //nolint:wrapcheck // the system's words, at the page's directory
	}
	if err = writeCopy(tmp, src, page, images); err != nil {
		_ = os.RemoveAll(tmp) //nolint:errcheck // the next save removes it otherwise
		return "", err
	}
	return filepath.Base(tmp), nil
}

// writeCopy writes the copy under dir, through os.Root so that nothing leads the writes out of it.
func writeCopy(dir string, src, page []byte, images map[string][]byte) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return usererr.CannotWrite(dir, err) //nolint:wrapcheck // the system's words, at the copy
	}
	defer root.Close()
	at := func(name string, err error) error {
		return usererr.CannotWrite(filepath.Join(dir, filepath.FromSlash(name)), err) //nolint:wrapcheck // as above
	}
	for _, f := range []struct {
		name string
		b    []byte
	}{{dataName, src}, {pageName, page}} {
		if err := root.WriteFile(f.name, f.b, 0o600); err != nil {
			return at(f.name, err)
		}
	}
	if err := root.Mkdir(imagesName, 0o700); err != nil {
		return at(imagesName, err)
	}
	for name, b := range images {
		name = path.Join(imagesName, name)
		if err := root.MkdirAll(path.Dir(name), 0o700); err != nil {
			return at(path.Dir(name), err)
		}
		if err := root.WriteFile(name, b, 0o600); err != nil {
			return at(name, err)
		}
	}
	return nil
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

// Remove deletes slug, ErrNotFound for no page. A linked file itself is left alone.
func (s *Store) Remove(slug string) error {
	dir, ok := s.dir(slug)
	if !ok {
		return ErrNotFound
	}
	unlock, err := s.lock(false)
	if errors.Is(err, fs.ErrNotExist) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	defer unlock()
	// A removal that waited for another of the same slug finds nothing, as one that came after.
	if s.ReadMeta(slug) == nil {
		return ErrNotFound
	}
	return removePage(dir)
}

// removePage removes dir, a page's directory, its meta.json last: one that cannot all be removed keeps it, listed
// still, for floe rm to remove the rest. What cannot be removed is told at it.
func removePage(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return usererr.CannotWrite(dir, err) //nolint:wrapcheck // the system's words, at the page
	}
	for _, e := range entries {
		if e.Name() == "meta.json" {
			continue
		}
		at := filepath.Join(dir, e.Name())
		if err := os.RemoveAll(at); err != nil {
			return usererr.CannotWrite(at, err) //nolint:wrapcheck // as above
		}
	}
	for _, at := range []string{filepath.Join(dir, "meta.json"), dir} {
		if err := os.Remove(at); err != nil {
			return usererr.CannotWrite(at, err) //nolint:wrapcheck // as above
		}
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
	Archived    *bool
	Tags        *[]string
}

// Edit applies ed to slug and returns its slug afterwards. updatedAt stays, so neither the order of
// the list nor the open tabs move. Clearing the title hands it back to the file (a link) or to the
// data file's title: when published (a stored page), else to the slug; clearing the project drops it.
// Tags are kept as tagged makes them.
// The page drawn stays as it is, its title and slug as when it was drawn.
func (s *Store) Edit(slug string, ed Edit) (string, error) {
	from, ok := s.dir(slug)
	if !ok {
		return "", ErrNotFound
	}
	unlock, err := s.lock(false)
	if errors.Is(err, fs.ErrNotExist) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	defer unlock()
	m := s.ReadMeta(slug)
	if m == nil {
		return "", ErrNotFound
	}
	setMarks(m, ed)
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
		dest, _ := s.dir(to)
		if err := os.Rename(from, dest); err != nil {
			return "", usererr.CannotWrite(dest, err) //nolint:wrapcheck // the system's words, at the new name
		}
	}
	return to, nil
}

// setMarks sets what ed changes of m that nothing refuses: its description, favourite, archived and tags.
func setMarks(m *Meta, ed Edit) {
	if ed.Description != nil {
		m.Description = strings.TrimSpace(*ed.Description)
	}
	if ed.Favorite != nil {
		m.Favorite = *ed.Favorite
	}
	if ed.Archived != nil {
		m.Archived = *ed.Archived
	}
	if ed.Tags != nil {
		m.Tags = tagged(*ed.Tags)
	}
}

// tagged is tags as a page keeps them: each trimmed, without the empty and a name the second time, in the
// order given; nil for none.
func tagged(tags []string) []string {
	var kept []string
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" && !slices.Contains(kept, tag) {
			kept = append(kept, tag)
		}
	}
	return kept
}

// setTitle sets the title of m, whose slug the edit renames to: an empty one gives back the page's
// own, or to.
func setTitle(to string, m *Meta, t string) {
	m.Title = strings.TrimSpace(t)
	m.TitleEdited = m.Title != ""
	if m.Title == "" && !m.IsLink() {
		// The drawn page's <title> is the data's title:, else the slug it was drawn under at its publish or
		// rebuild: the list's title, which this is, is of the list alone.
		m.Title = cmp.Or(m.DataTitle, to)
	}
}

// renameTo checks the new slug of an edit: nil keeps slug.
func (s *Store) renameTo(slug string, to *string) (string, error) {
	if to == nil {
		return slug, nil
	}
	// Checked as given, as a publish's slug is.
	name := *to
	if mistake := slugMistake(name); mistake != nil {
		return "", mistake
	}
	dir, _ := s.dir(name)
	if _, err := os.Lstat(dir); name != slug && !errors.Is(err, os.ErrNotExist) {
		return "", conflict(usererr.New("store_slug_taken", name))
	}
	return name, nil
}

// Conflict is a mistake of a name in use (a slug of another page, project or file), which a server answers with
// 409. It is returned alone, never collected with other mistakes: usererr.List.Add keeps the mistake it wraps, not
// the mark.
type Conflict struct {
	Err *usererr.Error
}

func conflict(e *usererr.Error) *Conflict { return &Conflict{Err: e} }

func (c *Conflict) Error() string { return c.Err.Error() }

// Unwrap gives the mistake, which usererr.Tell tells.
func (c *Conflict) Unwrap() error { return c.Err }
