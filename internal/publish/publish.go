// Package publish reads what a publish is of (the data and images sent, or the file or folder linked and its
// repository), draws it, and builds the store.Entry that store.Save writes.
package publish

import (
	"cmp"
	"context"
	"errors"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing/fstest"
	"time"

	"floe/internal/dirs"
	"floe/internal/files"
	"floe/internal/link"
	"floe/internal/project"
	"floe/internal/store"
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
	if abs, err := dirs.Under(cwd, file); err == nil {
		file = abs
	}
	return files.Slug(filepath.Base(filepath.Dir(filepath.Clean(file))))
}

// Accept checks a publish of file, a link or a data file, for what needs neither the store nor the disk,
// and returns o with each text given trimmed, whoever gave it. A slug not given is the store's to pick (store.OwnSlug).
// A slug is a name, checked as given: trimmed, it would not be the one a URL names.
// A title or a theme given empty is not given: the data's, or the one kept, is taken. Its mistakes: a data
// file empty or that is not one, and a slug that cannot be one. The command line checks with it before it
// sends, and the server again.
func Accept(file string, link bool, o store.Options) (store.Options, error) {
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
		if mistake := store.SlugMistake(*o.Slug); mistake != nil {
			found = append(found, mistake)
		}
	}
	return o, found.Err()
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
func acceptData(up Upload, opts store.Options) (store.Options, error) {
	opts, err := Accept(up.File, false, opts)
	if err == nil && opts.MoveProject && up.Project == "" {
		err = usererr.New("store_move_no_project")
	}
	return opts, err
}

// PrepareData checks a publish of the data file up as a stored page, drawn with dw, and builds what store.Save
// writes, without writing anything or reading the publisher's disk. Without up.Images, a data file that reads
// images is answered NeedImages, once nothing that needs no image refuses the publish (the slug, the page's
// owner) and before the mistakes of its drawing, which may be of the images not sent; a mistake before its
// drawing (of the YAML, the schema, the checks) is told without asking.
func PrepareData(s *store.Store, dw store.Drawing, up Upload, opts store.Options) (*store.Entry, error) {
	opts, err := acceptData(up, opts)
	if err != nil {
		return nil, err
	}
	if dw.Err != nil {
		return nil, dw.Err
	}
	slug, named, err := dataSlug(s, up, opts)
	if err != nil {
		return nil, err
	}
	sent := fstest.MapFS{}
	for name, b := range up.Images {
		sent[name] = &fstest.MapFile{Data: b, Mode: 0, ModTime: time.Time{}, Sys: nil}
	}
	images := &recorder{fsys: sent, names: nil, seen: map[string]bool{}, places: map[string][]yamlpos.Pos{}}
	picked, created, err := s.Themes(slug, "", up.Project, up.Repo, up.Place, opts.Force)
	if err != nil {
		return nil, err //nolint:wrapcheck // a usererr for the caller to show
	}
	o := tmpl.Options{
		Theme: cmp.Or(picked, opts.GivenTheme()), Default: created, Untitled: slug, Images: images, ImagesDir: "",
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
		if _, refused := storedPage(s, slug, named, up, nil, opts, created); refused != nil {
			return nil, usererr.WithWarnings(refused, warnings) //nolint:wrapcheck // its mistakes, with the page's warnings
		}
		return nil, &NeedImages{Images: images.needed()}
	}
	if drawErr != nil {
		return nil, drawErr //nolint:wrapcheck // mistakes at their places, for the caller to show
	}
	e, err := storedPage(s, slug, named, up, up.Source, opts, created)
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

// dataSlug is the slug a publish of up writes to: the one given, else its file's (store.OwnSlug), with the name it is
// picked by ("" for one given).
func dataSlug(s *store.Store, up Upload, opts store.Options) (slug, named string, err error) {
	if opts.Slug != nil {
		return *opts.Slug, "", nil
	}
	named = files.SlugFromFile(up.File)
	slug, err = s.OwnSlug(named, up.File, "", up.Repo, up.Place)
	return slug, named, err //nolint:wrapcheck // a usererr for the caller to show
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
// named after named when no slug was given ("" else), drawn with the default theme created. opts are those Accept
// returned without a mistake.
// LIMIT: renamed between the drawing and Save, an untitled page keeps the slug it was drawn with as its title in the
// page, not in the list; a link's too. Publish it again if that shows.
func storedPage(
	s *store.Store, slug, named string, up Upload, src []byte, opts store.Options, created string,
) (*store.Entry, error) {
	return s.NewEntry(store.Seed{ //nolint:wrapcheck // a usererr for the caller to show
		Slug: slug, Named: named, File: up.File, Path: "", Folder: false, FolderTitle: "", Data: src, Options: opts,
		Project: up.Project, Repo: up.Repo, Place: up.Place, Created: created,
	})
}

// PrepareLink checks a link to file and builds what store.Save writes, without writing anything. cwd resolves
// a relative file, "" for none to resolve it in: a relative file is then refused, with the other mistakes
// of the options. home bounds what may be linked (link.Check). The project of a link is that of its
// directory, a folder's its own.
// A data file linked is drawn with dw once, as it is when opened, its images under its real
// directory, to be refused for its mistakes.
func PrepareLink(
	ctx context.Context, s *store.Store, dw store.Drawing, file string, opts store.Options, cwd, home string,
) (*store.Entry, error) {
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
	slug, err := linkSlug(s, named, file, path, opts)
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
		if picked, created, err = s.Themes(slug, path, from, "", "", opts.Force); err != nil {
			return nil, err //nolint:wrapcheck // a usererr for the caller to show
		}
		data = src
		images := files.NewRoot(filepath.Dir(path))
		o := tmpl.Options{
			Theme: cmp.Or(picked, opts.GivenTheme()), Default: created, Untitled: slug, Images: images, ImagesDir: "",
			ImageAt: nil, Icons: dw.Icons, CutOff: false,
		}
		_, warnings, err = tmpl.Draw(dw.Kit, path, src, o)
		_ = images.Close() //nolint:errcheck // only read from
		if err != nil {
			return nil, err //nolint:wrapcheck // mistakes at their places, for the caller to show
		}
	}
	// A folder's own title, as a data file's title: is: where it is in its repository, its repository's name at its
	// root, else its name
	e, err := s.NewEntry(store.Seed{
		Slug: slug, Named: named, File: file, Path: path, Folder: folder,
		FolderTitle: cmp.Or(within, from, filepath.Base(path)), Data: data, Options: opts, Project: from, Repo: "",
		Place: "", Created: created,
	})
	if err != nil {
		return nil, usererr.WithWarnings(err, warnings) //nolint:wrapcheck // its mistakes, with the page's warnings
	}
	e.Warnings = warnings
	return e, nil
}

// readLink returns the real path of the linked file and its content, or of the linked folder (true), whose files
// are not read. A link is checked and stored by
// its real path: a symlink would otherwise reach into a hidden directory. It is of the kind of its real
// file, whatever the name of a symlink to it says: HTML is shown as it is, in its own look, so it takes
// no theme.
func readLink(file string, opts store.Options, cwd, home string) (string, []byte, bool, error) {
	if file == "" {
		return "", nil, false, usererr.New("store_link_empty")
	}
	// The API gives no cwd: a relative link from it is the sender's mistake.
	abs, err := dirs.Under(cwd, file)
	if errors.Is(err, dirs.ErrDriveRelative) {
		return "", nil, false, usererr.New("path_drive_relative", file, filepath.VolumeName(file), filepath.VolumeName(file))
	}
	if err != nil {
		return "", nil, false, usererr.New("store_link_relative", file)
	}
	path, err := files.Real(abs)
	if err != nil {
		return "", nil, false, usererr.CannotRead(file, err) //nolint:wrapcheck // the system's words, at the file
	}
	folder, err := link.Check(path, home)
	if err != nil {
		return "", nil, false, err //nolint:wrapcheck // a usererr for the caller to show
	}
	// A folder named config.yml is no data file.
	if (folder || !files.IsDataFile(path)) && opts.GivenTheme() != "" {
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

// linkSlug is the slug of a link to path: the one given, which Accept checked, else the slug of the link to path,
// else one free made of named.
func linkSlug(s *store.Store, named, file, path string, opts store.Options) (string, error) {
	if opts.Slug != nil {
		return *opts.Slug, nil
	}
	// Named after the given file, not the symlink's target.
	return s.OwnSlug(named, file, path, "", "") //nolint:wrapcheck // a usererr for the caller to show
}
