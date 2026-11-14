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

	"floe/internal/dirs"
	"floe/internal/doc"
	"floe/internal/files"
	"floe/internal/link"
	"floe/internal/project"
	"floe/internal/tmpl"
	"floe/internal/usererr"
	"floe/internal/yamlpos"
)

var indexRe = regexp.MustCompile(`(?i)^index\.html?$`)

// linkName is the slug a link to file from cwd is named after: index.html does not name anything, so it takes the
// name of its directory instead.
func linkName(file, cwd string) string {
	if !indexRe.MatchString(filepath.Base(file)) {
		return files.SlugFromFile(file)
	}
	if abs, ok := dirs.Under(cwd, file); ok {
		file = abs
	}
	return files.Slug(filepath.Base(filepath.Dir(filepath.Clean(file))))
}

// owns tells the entries of the file a publish is of: a link's of the same path, a stored page's of the same file
// (sameFile). newMeta gives a file to stored pages alone.
func owns(path, repo, place string) func(*Meta) bool {
	if path != "" {
		return func(m *Meta) bool { return m.Path == path }
	}
	return func(m *Meta) bool { return sameFile(m, repo, place) }
}

// sameFile is whether the stored page m is the data file of the key repo and place (project.Place): of the same place,
// and of the same repository unless the place is a real path, which names the file alone (a page published before
// keys were the file's alone holds the project it was published from in PublishedFrom).
func sameFile(m *Meta, repo, place string) bool {
	return m.PublishedFile != "" && m.PublishedFile == place && (filepath.IsAbs(place) || m.PublishedFrom == repo)
}

// ownSlug is the slug of a publish without --slug, of file, whose entries own tells, named base: base when its entry
// is the file's, else the first of the file's entries by slug, else base, which a link takes when free, else base-2,
// base-3 … so that two index.html files never take over each other. base is checked only then: a file whose name
// makes no slug still finds its page.
// LIMIT: unless base is the file's, every meta.json is read, under the store's lock too when Save picks again. Keep the
// slugs of the files apart if publishing slows.
// LIMIT: of two pages of one file (a publish with --slug, or one before publishes found the file's page), base's is
// written and the other is left as it was, to be floe rm'd.
func (s *Store) ownSlug(base, file string, own func(*Meta) bool, link bool) (string, error) {
	if m := s.ReadMeta(base); m != nil && own(m) {
		return base, nil
	}
	slugs, err := s.slugs()
	if err != nil {
		return "", err
	}
	for _, slug := range slugs {
		if m := s.ReadMeta(slug); m != nil && own(m) {
			return slug, nil
		}
	}
	if base == "" {
		return "", usererr.NewAt(file, 0, 0, "store_slug_unnamed")
	}
	slug := base
	for n := 2; link && s.ReadMeta(slug) != nil; n++ {
		suffix := "-" + strconv.Itoa(n)
		slug = strings.TrimRight(base[:min(len(base), files.MaxSlugLen-len(suffix))], "-") + suffix
	}
	if mistake := slugMistake(slug); mistake != nil {
		return "", mistake
	}
	return slug, nil
}

// Options are the choices of a publish. A nil string is not given; an empty one is.
type Options struct {
	Slug        *string
	Title       *string
	Description *string
	Theme       *string
	Force       bool
	// MoveProject puts the page in the project of the place it is published from (Upload's), though it is in another.
	MoveProject bool
}

// Accept checks a publish of file, a link or a data file, for what needs neither the store nor the disk,
// and returns o with each text given trimmed, whoever gave it. A slug not given is the store's to pick (ownSlug).
// A slug is a name, checked as given: trimmed, it would not be the one a URL names.
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
	// OffTask is the task the page was taken off, as its project changed (offTaskOf), "" for none.
	OffTask string
	// Warnings are those of the page drawn, which Save does not keep.
	Warnings []*usererr.Error

	// What settle builds Meta from, with the link's path, the data's title: and the theme of the Meta
	// it replaces: the publish's options, the project published from (a new page's, and what a page with no file is
	// compared by), and the key of a stored page's data file ("" for a link).
	opts    Options
	project string
	repo    string
	place   string
	// named is what a publish without --slug is named after, for Save to pick its slug again ("" for a name that makes
	// none).
	named string
	// file is the file as the publisher named it, where a name that makes no slug is told.
	file string
	// created is the default theme the page was drawn with, which settle keeps for a new page, or one replacing
	// another file's, as its CreatedTheme: read before the drawing, not at Save, so that the two agree.
	created string
}

// Upload is a data file sent for a publish: what PrepareData checks and builds a stored page of, without
// reading the publisher's disk.
type Upload struct {
	File   string // as the publisher named it, for its slug and its mistakes
	Source []byte
	// Images are the images sent with it, by the path the data names them by; nil for none sent, which asks
	// which images the data reads (NeedImages). An empty one is sent.
	Images  map[string][]byte
	Project string // the project it is published from, a new page's
	// Repo and Place are the file's key, which owns the page from whatever directory it is published (project.Place):
	// its repository and its path from the repository's root, else "" and its real path.
	Repo  string
	Place string
}

// NeedImages answers an Upload without images whose data reads some: the images, as it writes them.
type NeedImages struct {
	Images []NeededImage
}

// NeededImage is an image the data reads, by its name, with the place of each item naming it in the data file.
type NeededImage struct {
	Name   string
	Places []yamlpos.Pos
}

func (e *NeedImages) Error() string {
	names := make([]string, len(e.Images))
	for i, n := range e.Images {
		names[i] = n.Name
	}
	return "the data names images to be sent with it: " + strings.Join(names, ", ")
}

// recorder notes each name read of fsys, with Stat as with Open: kit.ReadImage stats an image before it opens
// it, and an image that is not there fails the Stat. It notes too where the data names each image (at, as
// tmpl.Options.ImageAt), which names it does not read as well: the publisher reads the images it is asked for
// and tells its mistakes, which are of the names, at these places of its data file.
type recorder struct {
	fsys   fs.FS
	names  []string
	seen   map[string]bool
	places map[string][]yamlpos.Pos
}

func (r *recorder) at(name string, line, column int) {
	r.places[name] = append(r.places[name], yamlpos.Pos{Line: line, Column: column})
}

// needed is the images read of fsys, as NeedImages answers them.
func (r *recorder) needed() []NeededImage {
	images := make([]NeededImage, len(r.names))
	for i, name := range r.names {
		images[i] = NeededImage{Name: name, Places: r.places[name]}
	}
	return images
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

// acceptData is Accept of the data file of up, which --move-project refuses to put in the project of a place with
// none: it would empty the page's, which the next publish from another place would fill again.
func acceptData(up Upload, opts Options) (Options, error) {
	opts, err := Accept(up.File, false, opts)
	if err == nil && opts.MoveProject && up.Project == "" {
		err = usererr.New("store_move_no_project")
	}
	return opts, err
}

// PrepareData checks a publish of the data file up as a stored page, drawn with dw, and builds what Save
// writes, without writing anything or reading the publisher's disk. Without up.Images, a data file that reads
// images is answered NeedImages, once nothing that needs no image refuses the publish (the slug, the page's
// owner) and before the mistakes of its drawing, which may be of the images not sent; a mistake before its
// drawing (of the YAML, the schema, the checks) is told without asking.
func (s *Store) PrepareData(dw Drawing, up Upload, opts Options) (*Entry, error) {
	opts, err := acceptData(up, opts)
	if err != nil {
		return nil, err
	}
	if dw.Err != nil {
		return nil, dw.Err
	}
	slug, named, err := s.dataSlug(up, opts)
	if err != nil {
		return nil, err
	}
	sent := fstest.MapFS{}
	for name, b := range up.Images {
		sent[name] = &fstest.MapFile{Data: b, Mode: 0, ModTime: time.Time{}, Sys: nil}
	}
	images := &recorder{fsys: sent, names: nil, seen: map[string]bool{}, places: map[string][]yamlpos.Pos{}}
	picked, created, err := s.themesFor(slug, "", up.Project, up.Repo, up.Place, opts.Force)
	if err != nil {
		return nil, err
	}
	o := tmpl.Options{
		Theme: cmp.Or(picked, opts.theme()), Default: created, Untitled: slug, Images: images, ImagesDir: "",
		ImageAt: images.at, Icons: dw.Icons, CutOff: false,
	}
	page, warnings, drawErr := tmpl.Draw(dw.Kit, up.File, up.Source, o)
	var mistakes usererr.List
	if failed := mistakes.Add(drawErr); failed != nil {
		return nil, failed
	}
	// LIMIT: a template changed between the two puts to read another image too is told that image is not there,
	// for this put sends the images the first asked (a map, not nil): publish again. Ask again if that comes to matter.
	if up.Images == nil && len(images.names) > 0 {
		if _, refused := s.storedPage(slug, named, up, nil, opts); refused != nil {
			return nil, usererr.WithWarnings(refused, warnings) //nolint:wrapcheck // its mistakes, with the page's warnings
		}
		return nil, &NeedImages{Images: images.needed()}
	}
	if drawErr != nil {
		return nil, drawErr //nolint:wrapcheck // mistakes at their places, for the caller to show
	}
	e, err := s.storedPage(slug, named, up, up.Source, opts)
	if err != nil {
		return nil, usererr.WithWarnings(err, warnings) //nolint:wrapcheck // its mistakes, with the page's warnings
	}
	e.Page = page
	e.Warnings = warnings
	e.created = created
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

// dataSlug is the slug a publish of up writes to: the one given, else its file's (ownSlug), with the name it is
// picked by ("" for one given).
func (s *Store) dataSlug(up Upload, opts Options) (slug, named string, err error) {
	if opts.Slug != nil {
		return *opts.Slug, "", nil
	}
	named = files.SlugFromFile(up.File)
	slug, err = s.ownSlug(named, up.File, owns("", up.Repo, up.Place), false)
	return slug, named, err
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

// storedPage builds the entry of the data file up as the stored page slug of content src (nil checks its owner only),
// named after named when no slug was given ("" else). opts are those Accept returned without a mistake.
// LIMIT: renamed between the drawing and Save, an untitled page keeps the slug it was drawn with as its title in the
// page, not in the list; a link's too. Publish it again if that shows.
func (s *Store) storedPage(slug, named string, up Upload, src []byte, opts Options) (*Entry, error) {
	e := &Entry{
		Slug: slug, Meta: seedMeta("", up.File, src, opts.theme()), Source: src, Page: nil, Images: nil, Prev: nil,
		OffTask: "", Warnings: nil, opts: opts, project: up.Project, repo: up.Repo, place: up.Place, named: named,
		file: up.File, created: "",
	}
	if err := e.settle(s.ReadMeta(e.Slug)); err != nil {
		return nil, err
	}
	return e, nil
}

// PrepareLink checks a link to file and builds what Save writes, without writing anything. cwd resolves
// a relative file, "" for none to resolve it in: a relative file is then refused, with the other mistakes
// of the options. home bounds what may be linked (link.Check). The project of a link is that of its
// directory, a folder's its own.
// A data file linked is drawn with dw once, as it is when opened, its images under its real
// directory, to be refused for its mistakes.
func (s *Store) PrepareLink(
	ctx context.Context, dw Drawing, file string, opts Options, cwd, home string,
) (*Entry, error) {
	opts, err := Accept(file, true, opts)
	path, src, folder, linkErr := readLink(file, opts, cwd, home)
	if err = usererr.Collect(err, linkErr); err != nil {
		return nil, err
	}
	dir := filepath.Dir(path)
	if folder {
		dir = path
	}
	from := project.Name(ctx, dir)
	// A folder is named by its project and where it is in it (api-docs/docs is api-docs-docs, its title docs), as
	// folders of one name (docs) in several repositories are common; outside a repository its project is its name.
	within, named := "", ""
	if folder {
		within = project.Within(ctx, path)
		named = files.Slug(from + "-" + within)
		if within == "" {
			named = files.Slug(cmp.Or(from, filepath.Base(path)))
		}
	} else if opts.Slug == nil {
		named = linkName(file, cwd)
	}
	slug, err := s.linkSlug(named, file, path, opts)
	if err != nil {
		return nil, err
	}
	var data []byte
	var warnings []*usererr.Error
	picked, created := "", ""
	if !folder && files.IsDataFile(path) {
		if dw.Err != nil {
			return nil, dw.Err
		}
		if picked, created, err = s.themesFor(slug, path, from, "", "", opts.Force); err != nil {
			return nil, err
		}
		data = src
		images := files.NewRoot(filepath.Dir(path))
		o := tmpl.Options{
			Theme: cmp.Or(picked, opts.theme()), Default: created, Untitled: slug, Images: images, ImagesDir: "", ImageAt: nil,
			Icons: dw.Icons, CutOff: false,
		}
		_, warnings, err = tmpl.Draw(dw.Kit, path, src, o)
		_ = images.Close() //nolint:errcheck // only read from
		if err != nil {
			return nil, err //nolint:wrapcheck // mistakes at their places, for the caller to show
		}
	}
	if opts.Slug != nil {
		named = ""
	}
	e := &Entry{
		Slug: slug, Meta: seedMeta(path, path, data, opts.theme()), Source: nil, Page: nil, Images: nil, Prev: nil,
		OffTask: "", Warnings: warnings, opts: opts, project: from, repo: "", place: "", named: named, file: file,
		created: created,
	}
	e.Meta.Folder = folder
	// A folder's own title, as a data file's title: is: where it is in its repository, its repository's name at its
	// root, else its name
	if folder {
		e.Meta.DataTitle = cmp.Or(within, from, filepath.Base(path))
	}
	if err := e.settle(s.ReadMeta(e.Slug)); err != nil {
		return nil, usererr.WithWarnings(err, warnings) //nolint:wrapcheck // its mistakes, with the page's warnings
	}
	return e, nil
}

// seedMeta is the seed of the Meta settle builds: the link's path ("" for a stored page), and of a page drawn
// from the data file file of content data (nil for HTML), its title: and the theme it is drawn with, and of a stored
// page its template: (a link's is read from its file for the list).
func seedMeta(path, file string, data []byte, theme string) Meta {
	var m Meta
	m.Path = path
	if data != nil {
		m.Theme = theme
		if d, _ := doc.Read(file, data, theme); d != nil { //nolint:errcheck // drawn without a mistake already
			m.DataTitle = d.Title
			if path == "" {
				m.Template = d.Template
			}
		}
	}
	return m
}

// settle checks e against prev, the entry its slug holds (nil for none), and builds its meta.json on
// it. A replaced page or link keeps nothing of the old one.
func (e *Entry) settle(prev *Meta) error {
	was := e.Meta
	replacing, err := checkOwner(e.Slug, prev, was.Path, e.project, e.repo, e.place, e.opts.Force)
	if err != nil {
		return err
	}
	base := prev
	if replacing || base == nil {
		var none Meta
		base = &none
	}
	e.Meta = newMeta(e.opts, was.Path, e.project, e.repo, e.place, base, was.DataTitle, e.Slug)
	e.OffTask = offTaskOf(&e.Meta, ProjectKey(base.Project))
	if replacing || prev == nil {
		e.Meta.CreatedTheme = e.created
	}
	e.Meta.Theme = was.Theme
	e.Meta.Template = was.Template
	// A folder's link lost its folder would serve its parent's files.
	e.Meta.Folder = was.Folder
	e.Prev = prev
	return nil
}

// readLink returns the real path of the linked file and its content, or of the linked folder (true), whose files
// are not read. A link is checked and stored by
// its real path: a symlink would otherwise reach into a hidden directory. It is of the kind of its real
// file, whatever the name of a symlink to it says: HTML is shown as it is, in its own look, so it takes
// no theme.
func readLink(file string, opts Options, cwd, home string) (string, []byte, bool, error) {
	if file == "" {
		return "", nil, false, usererr.New("store_link_empty")
	}
	// The API gives no cwd: a relative link from it is the sender's mistake.
	abs, ok := dirs.Under(cwd, file)
	if !ok {
		return "", nil, false, usererr.New("store_link_relative", file)
	}
	path, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", nil, false, usererr.CannotRead(file, err) //nolint:wrapcheck // the system's words, at the file
	}
	folder, err := link.Check(path, home)
	if err != nil {
		return "", nil, false, err //nolint:wrapcheck // a usererr for the caller to show
	}
	// A folder named config.yml is no data file.
	if (folder || !files.IsDataFile(path)) && opts.theme() != "" {
		return "", nil, false, usererr.New("store_theme_link")
	}
	if folder {
		return path, nil, true, nil
	}
	b, _, err := files.Read(path)
	if err != nil {
		return "", nil, false, usererr.CannotRead(file, err) //nolint:wrapcheck // the system's words, at the file
	}
	return path, b, false, nil
}

// themesFor returns the themes a page drawn from a data file at slug is drawn with: the one picked for it and the
// default, none picked and the one settings.json chooses for a new page, or one replacing another file's (checkOwner),
// else the page's own. It looks at meta.json without the lock, before the drawing: settle keeps the default returned
// (Entry.created), so that the page drawn and its meta.json agree. A mistake of the owner is told by settle, after the
// drawing, as without it.
// LIMIT: a page there when it is read, but replaced by another publish before Save, is kept as replacing with the
// default it had, the one it was drawn with; refuse it at Save if that comes to matter.
// LIMIT: a theme picked between the look and Save is kept, the page drawn in the one before it: the next publish or
// pick mends it. Refuse it at Save as PickTheme does if that comes to matter.
// LIMIT: a slug Save picks again (pickAgain: the file's page renamed, or the slug taken, meanwhile) is drawn in the
// themes of the one looked at here, its meta.json taking those of the page picked, as the title in storedPage's LIMIT.
// Publish again if that shows.
func (s *Store) themesFor(slug, path, project, repo, place string, force bool) (picked, created string, err error) {
	prev := s.ReadMeta(slug)
	replacing, _ := checkOwner(slug, prev, path, project, repo, place, force) //nolint:errcheck // told by settle, as above
	if prev != nil && !replacing {
		return prev.PickedTheme, prev.CreatedTheme, nil
	}
	set, err := s.ReadSettings()
	return "", set.Theme, err
}

// projectOf is the project of a page published from project: the one it was in, unless it is new, replaces another
// file's or was emptied of its project, or opts moves it.
func projectOf(opts Options, base *Meta, project string) string {
	if opts.MoveProject {
		return project
	}
	return cmp.Or(base.Project, project)
}

// newMeta keeps what was set on the management page (favourite, title, tags), the description, createdAt and the
// project from base, but not archived: a page published again comes back to the list. --title and --description
// win. A new page, or one replacing another file's, is in project, and a page emptied of its project comes back to
// it. A stored page is owned by the file of the key repo and place; an empty place keeps base's.
func newMeta(opts Options, path, project, repo, place string, base *Meta, own, slug string) Meta {
	keepTitle := opts.Title == nil && base.TitleEdited
	// One reading of the clock, so that a new page's createdAt equals its updatedAt.
	now := Now()
	m := Meta{
		Path:          path,
		Title:         title(opts, path, keepTitle, base, own, slug),
		TitleEdited:   keepTitle,
		Description:   base.Description,
		Theme:         "",
		CreatedTheme:  base.CreatedTheme,
		PickedTheme:   base.PickedTheme,
		DataTitle:     own,
		Template:      "",
		Source:        "",
		Project:       projectOf(opts, base, project),
		PublishedFrom: "",
		PublishedFile: "",
		Favorite:      base.Favorite,
		Folder:        false,
		Archived:      false,
		Tags:          base.Tags,
		Task:          base.Task,
		Versions:      base.Versions,
		CreatedAt:     cmp.Or(base.CreatedAt, now),
		UpdatedAt:     now,
	}
	if opts.Description != nil {
		m.Description = *opts.Description
	}
	if path == "" {
		m.PublishedFrom = repo
		m.PublishedFile = cmp.Or(place, base.PublishedFile)
	}
	return m
}

// linkSlug is the slug of a link to path: the one given, which Accept checked, else the slug of the link to path,
// else one free made of named.
func (s *Store) linkSlug(named, file, path string, opts Options) (string, error) {
	if opts.Slug != nil {
		return *opts.Slug, nil
	}
	// Named after the given file, not the symlink's target.
	return s.ownSlug(named, file, owns(path, "", ""), true)
}

// slugMistake is the mistake of slug if it cannot be one, nil else.
func slugMistake(slug string) *usererr.Error {
	if !ValidSlug(slug) {
		return usererr.New("store_invalid_slug", slug)
	}
	return nil
}

// checkOwner refuses to overwrite another's entry without force, and reports whether the publish
// replaces one. A stored page belongs to its data file, of the key repo and place (sameFile), from whatever project
// it is published, as a link belongs to its path: the same slug for another file certainly means another page.
// LIMIT: a page whose file is in a repository but was published from outside it, before keys were the file's alone,
// holds its real path: published again, its key is the repository's path, which differs, so it is refused as another
// file's, or, renamed, its slug is left and a second page written at the file's name. None is in the store of now;
// publish it with --force if one comes.
func checkOwner(slug string, prev *Meta, path, project, repo, place string, force bool) (bool, error) {
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
	// LIMIT: a page published before pages kept their file has none, and is compared by the project it was published
	// from: it takes the file of the next publish from that project, another file too, once, which keys it from then
	// on; remove this before floe is released, or once every page was published again.
	if prev.PublishedFile == "" {
		if owner := prev.PublishedFrom; owner != "" && owner != project {
			if force {
				return true, nil
			}
			return false, conflict(usererr.New("store_slug_owned", slug, owner))
		}
		return false, nil
	}
	if sameFile(prev, repo, place) {
		return false, nil
	}
	if force {
		return true, nil
	}
	// A path in a repository is told with the repository it is from, the page's file's and not the list's project.
	if !filepath.IsAbs(prev.PublishedFile) && prev.PublishedFrom != "" {
		return false, conflict(usererr.New("store_slug_placed", slug, prev.PublishedFile, prev.PublishedFrom))
	}
	return false, conflict(usererr.New("store_slug_placed_alone", slug, prev.PublishedFile))
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
// go after. A stored page published with other content than its last version's gets a version (nextVersion), the
// page of the copy before kept, gzipped outside the lock, as the version before.
// LIMIT: a reader that read meta.json just before a save may find its copy gone, and shows that it cannot be
// read until its next look. Keep the copy before until the next save if that shows.
func (s *Store) Save(e *Entry) error {
	var mark string
	var pre *pressed
	if !e.Meta.IsLink() {
		mark = contentMark(e.Source, e.Images)
		pre = s.press(e.Slug, mark)
	}
	return s.commit(e, mark, pre)
}

// pressed is the page of the copy source of a stored page, gzipped, to keep as its version n.
type pressed struct {
	source string
	n      int
	gz     []byte
}

// press gzips the page of the stored page slug as it is now, when a save of content of mark makes a version after
// its last: nil else, or when it cannot be read, for commit to look again under the lock.
func (s *Store) press(slug, mark string) *pressed {
	m := s.ReadMeta(slug)
	if m == nil || m.IsLink() || len(m.Versions) == 0 {
		return nil
	}
	last := m.Versions[len(m.Versions)-1]
	if last.Mark == mark {
		return nil
	}
	page, ok := s.VersionPage(slug, m, last.N)
	if !ok {
		return nil
	}
	return &pressed{source: m.Source, n: last.N, gz: gzipped(page)}
}

// commit is Save, the content of a stored page of mark, its page before pressed as pre (nil for none).
func (s *Store) commit(e *Entry, mark string, pre *pressed) error {
	unlock, err := s.lock(true)
	if err != nil {
		return err
	}
	defer unlock()
	if err = s.pickAgain(e); err != nil {
		return err
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
		// What it replaces is a link, or a page whose meta.json could not be read, whose copy and versions go.
		if err = s.WriteMeta(e.Slug, &e.Meta); err == nil {
			s.tidy(e.Slug, &e.Meta)
		}
	} else {
		err = s.keepCopy(e.Slug, &e.Meta, e.Source, e.Page, e.Images, s.nextVersion(e, mark, pre))
	}
	// The directory this save made, left, would keep a rename off the slug from the management page, which
	// does not list it. One that was there (its meta.json unreadable) stays.
	if err != nil && made {
		_ = os.RemoveAll(dir) //nolint:errcheck // the error is the save's
	}
	return err
}

// pickAgain picks the slug of e, a publish without --slug, again under the lock, unless it holds what it held at
// Prepare: nothing, or the file's entry. Another publish may have taken it, or the file's own been renamed, since. A
// save picking another stored page gets its version before gzipped under the lock (nextVersion).
// LIMIT: a slug free at Prepare and free still is kept: when another publish of the file made it and the management
// page renamed it meanwhile, a second page is made at it. Look for the file's page under the lock if that shows.
func (s *Store) pickAgain(e *Entry) error {
	if e.opts.Slug != nil {
		return nil
	}
	own := owns(e.Meta.Path, e.repo, e.place)
	m := s.ReadMeta(e.Slug)
	if (e.Prev == nil && m == nil) || (e.Prev != nil && own(e.Prev) && m != nil && own(m)) {
		return nil
	}
	slug, err := s.ownSlug(e.named, e.file, own, e.Meta.IsLink())
	if err != nil {
		return err
	}
	e.Slug = slug
	return nil
}

// nextVersion adds to e.Meta, settled under the lock, the version the save of a stored page of content mark makes,
// and returns the page of the version before to keep (nil for none), pre when it is of the copy that is still last.
// A page replaced, or of no version, gets one whatever its content, its page before being none of its versions; one
// whose last version's content is mark gets none, and keeps its updatedAt, which moves with the versions alone. Numbers
// go on from the last of the page replaced.
// A last version that cannot be read, which nothing serves, is dropped.
// LIMIT: a publish of the same content (another theme, or after a template or part was mended) does not come up the
// list and tells no one, as the list is by what changed; one archived comes back at the place it had. The user chose
// so: publish other content to bring it up.
func (s *Store) nextVersion(e *Entry, mark string, pre *pressed) *pressed {
	now := e.Prev
	n := 1
	if now != nil && len(now.Versions) > 0 {
		n = now.Versions[len(now.Versions)-1].N + 1
	}
	versions := slices.Clone(e.Meta.Versions)
	var kept *pressed
	if len(versions) > 0 {
		last := versions[len(versions)-1]
		if last.Mark == mark {
			e.Meta.UpdatedAt = now.UpdatedAt
			return nil
		}
		if pre != nil && pre.source == now.Source && pre.n == last.N {
			kept = pre
		} else if page, ok := s.VersionPage(e.Slug, now, last.N); ok {
			// LIMIT: gzipped under the lock, when another save of the slug came between press and the lock.
			kept = &pressed{source: now.Source, n: last.N, gz: gzipped(page)}
		} else {
			versions = versions[:len(versions)-1]
		}
	}
	versions = append(versions, Version{N: n, At: e.Meta.UpdatedAt, Mark: mark})
	e.Meta.Versions = versions[max(0, len(versions)-keepVersions):]
	return kept
}

// keepCopy writes a new copy of the stored page slug, the data file src, the page drawn from it and its
// images, then the page of the version before, kept (nil for none), where m names it, then m naming the copy as
// its meta.json, then removes the copies and versions m does not name: what a save and a rebuild keep. What
// meta.json could not come to name goes. It is called under the lock.
func (s *Store) keepCopy(slug string, m *Meta, src, page []byte, images map[string][]byte, kept *pressed) error {
	dir, ok := s.dir(slug)
	if !ok {
		return fmt.Errorf("store: keep a copy of %q, which cannot be a slug", slug) //nolint:err113 // floe's own mistake
	}
	source, err := writeSource(dir, src, page, images)
	if err != nil {
		return err
	}
	undo := func() {
		_ = os.RemoveAll(filepath.Join(dir, source)) //nolint:errcheck // the error is the write's
	}
	if kept != nil {
		unkeep, err := s.writeVersion(slug, m, kept)
		if err != nil {
			undo()
			return err
		}
		undo = func() {
			_ = os.RemoveAll(filepath.Join(dir, source)) //nolint:errcheck // as above
			unkeep()
		}
	}
	m.Source = source
	if err := s.WriteMeta(slug, m); err != nil {
		// meta.json is the one before, whose last version is kept's, served from its copy: none names the file.
		undo()
		return err
	}
	s.tidy(slug, m)
	return nil
}

// tidy removes what is under the page slug that m, its meta.json written, does not name: the copies and versions
// that a save replaced or dropped, or left when it stopped halfway; of a link, which names none, every one.
func (s *Store) tidy(slug string, m *Meta) {
	dir, _ := s.dir(slug)
	removeSources(dir, m.Source)
	s.removeVersions(slug, m)
}

// writeVersion writes the page kept, gzipped, as the version m names it, and returns what removes it, with
// versions/ when it made it.
func (s *Store) writeVersion(slug string, m *Meta, kept *pressed) (func(), error) {
	file, at, ok := s.versionFile(slug, m, kept.n)
	if !ok || lastVersion(m, at) {
		//nolint:err113 // floe's own mistake
		return nil, fmt.Errorf("store: keep version %d of %q, which meta.json names as none before its last", kept.n, slug)
	}
	dir := filepath.Dir(file)
	made := os.Mkdir(dir, 0o700) == nil
	unkeep := func() {
		_ = os.Remove(file) //nolint:errcheck // the error is the write's
		if made {
			_ = os.Remove(dir) //nolint:errcheck // as above
		}
	}
	if err := files.WriteAtomic(file, kept.gz, 0o600, nil); err != nil {
		unkeep()
		return nil, usererr.CannotWrite(file, err) //nolint:wrapcheck // the system's words, at the version
	}
	return unkeep, nil
}

// removeVersions removes what is under versions/ of slug but the versions before the last m names: those dropped
// past keepVersions, or left by a save that stopped halfway. The save is done by then: what cannot go is left for
// the next.
func (s *Store) removeVersions(slug string, m *Meta) {
	dir, _ := s.dir(slug)
	dir = filepath.Join(dir, versionsName)
	named := map[string]bool{}
	for _, v := range m.Versions {
		if file, at, ok := s.versionFile(slug, m, v.N); ok && !lastVersion(m, at) {
			named[filepath.Base(file)] = true
		}
	}
	if len(named) == 0 {
		_ = os.RemoveAll(dir) //nolint:errcheck // as below
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, d := range entries {
		if !named[d.Name()] {
			_ = os.RemoveAll(filepath.Join(dir, d.Name())) //nolint:errcheck // as above
		}
	}
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
	if err := removePage(dir); err != nil {
		return err
	}
	s.moveRead(slug, "")
	return nil
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

// Edit is a change from the management page or the command line. A nil field is left as it is.
type Edit struct {
	Title       *string
	Description *string
	Project     *string
	Slug        *string
	Favorite    *bool
	Archived    *bool
	Tags        *TagsEdit
}

// TagsEdit changes a page's tags: the names in Remove are taken off, then those in Add put on, so that a name in both
// stays on. Only the names a writer changed are sent, so that a tag another wrote since it read the page stays.
type TagsEdit struct {
	Add    []string `json:"add,omitzero"`
	Remove []string `json:"remove,omitzero"`
}

// Edited is a page as an edit wrote it: its slug afterwards, and its project (ProjectKey) and task, under the lock.
type Edited struct {
	Slug        string    `json:"slug"`
	TaskProject string    `json:"taskProject"`
	Task        *PageTask `json:"task,omitzero"`
	// OffTask is the task the page was on and left: its project changed (Edit), or ChangeTasks put it on another task
	// or none; "" for none.
	OffTask string `json:"offTask,omitempty"`
	// OffCategory is the page's category on OffTask as ChangeTasks found it, "" for none.
	OffCategory string `json:"offCategory,omitempty"`
}

// Edit applies ed to slug and returns the page as it left it. updatedAt stays, so neither the order of
// the list nor the open tabs move. Clearing the title hands it back to the file (a link) or to the
// data file's title: when published (a stored page), else to the slug; clearing the project drops it.
// Tags are changed under the lock, so that writers who send only their change keep each other's.
// The page drawn stays as it is, its title and slug as when it was drawn.
func (s *Store) Edit(slug string, ed Edit) (Edited, error) {
	from, ok := s.dir(slug)
	if !ok {
		return Edited{}, ErrNotFound
	}
	unlock, err := s.lock(false)
	if errors.Is(err, fs.ErrNotExist) {
		return Edited{}, ErrNotFound
	}
	if err != nil {
		return Edited{}, err
	}
	defer unlock()
	m := s.ReadMeta(slug)
	if m == nil {
		return Edited{}, ErrNotFound
	}
	setMarks(m, ed)
	off, err := setProject(m, ed)
	if err != nil {
		return Edited{}, err
	}
	to, err := s.renameTo(slug, ed.Slug)
	if err != nil {
		return Edited{}, err
	}
	editTitle(slug, to, m, ed.Title)
	if err := s.WriteMeta(slug, m); err != nil {
		return Edited{}, err
	}
	// Renaming moves the directory only; tabs open on the old URL show "not found".
	if to != slug {
		dest, _ := s.dir(to)
		if err := os.Rename(from, dest); err != nil {
			return Edited{}, usererr.CannotWrite(dest, err) //nolint:wrapcheck // the system's words, at the new name
		}
		s.moveRead(slug, to)
	}
	return Edited{Slug: to, TaskProject: ProjectKey(m.Project), Task: m.Task, OffTask: off, OffCategory: ""}, nil
}

// setProject sets the project ed names of m, taking m off its task when it moves it: it returns the task taken off,
// "" for none.
func setProject(m *Meta, ed Edit) (string, error) {
	if ed.Project == nil {
		return "", nil
	}
	// "/" is the management page's value for "no project".
	if strings.TrimSpace(*ed.Project) == "/" {
		return "", usererr.New("store_project_slash")
	}
	was := ProjectKey(m.Project)
	m.Project = strings.TrimSpace(*ed.Project)
	return offTaskOf(m, was), nil
}

// offTaskOf takes m off its task when its project is no longer was (ProjectKey): a task is of its project, and one of
// the same name in another is another task. It returns the task taken off, "" for none. The pin stays.
func offTaskOf(m *Meta, was string) string {
	if ProjectKey(m.Project) == was || m.Task == nil || m.Task.Name == "" {
		return ""
	}
	task, off := m.Task.Name, ""
	setPageTask(m, &PageTaskEdit{Name: &off, Category: nil, Pin: nil})
	return task
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
		m.Tags = tagged(append(without(m.Tags, ed.Tags.Remove), ed.Tags.Add...))
	}
}

// without returns tags but those named in names, each name as tagged keeps it.
func without(tags, names []string) []string {
	off := tagged(names)
	return slices.DeleteFunc(slices.Clone(tags), func(tag string) bool { return slices.Contains(off, tag) })
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

// editTitle sets the title of m, renamed from slug to to, as t says (nil for none given). A title made of the slug
// follows it, as publishing the page again makes it: else that publish would tell the title changed. A --title equal to
// the old slug, of a page without title:, is followed too.
func editTitle(slug, to string, m *Meta, t *string) {
	switch {
	case t != nil:
		setTitle(to, m, *t)
	case to != slug && !m.TitleEdited && m.DataTitle == "" && m.Title == slug:
		m.Title = to
	}
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
