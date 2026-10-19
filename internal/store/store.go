// Package store keeps the pages under the store directory: <slug>/source-<id>/ (the data file,
// data.yaml, and the images it shows), which meta.json names, for a stored page, meta.json alone for a
// linked file.
package store

import (
	"bytes"
	"cmp"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	"github.com/sukekyo26/floe/internal/link"
	"github.com/sukekyo26/floe/internal/page"
	"github.com/sukekyo26/floe/internal/tmpl"
	"github.com/sukekyo26/floe/internal/usererr"
)

const (
	isoMillis = "2006-01-02T15:04:05.000Z"
	// sourcePrefix names a copy under a stored page: its data file, tmpl.KeptName, and the images it
	// shows, at the paths it names them by.
	sourcePrefix = "source-"
)

var slugRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$`)

// ErrNotFound is returned for a slug with no page.
var ErrNotFound = errors.New("no such page")

// ValidSlug reports whether s can name a page.
func ValidSlug(s string) bool {
	return slugRe.MatchString(s)
}

// Meta is meta.json.
type Meta struct {
	// Path is the real path of a linked file; a stored page has none.
	Path  string `json:"path,omitempty"`
	Title string `json:"title,omitempty"`
	// TitleEdited and ProjectEdited mark values set on the management page, kept across republishes.
	TitleEdited bool   `json:"titleEdited,omitempty"`
	Description string `json:"description"`
	// Theme and PartsTheme are the themes a data file is drawn with over the data's (floe publish --theme
	// and --parts-theme), and DataTitle its title: when published, which clearing the title on the
	// management page gives back.
	Theme      string `json:"theme,omitempty"`
	PartsTheme string `json:"partsTheme,omitempty"`
	DataTitle  string `json:"dataTitle,omitempty"`
	// Source is the directory of a stored page's copy of its data file and images, under its own.
	Source        string `json:"source,omitempty"`
	Project       string `json:"project,omitempty"`
	ProjectEdited bool   `json:"projectEdited,omitempty"`
	// PublishedFrom is the project that owns a stored page; Project is only what the list shows.
	PublishedFrom string `json:"publishedFrom,omitempty"`
	Favorite      bool   `json:"favorite,omitempty"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

// IsLink reports whether the entry is a linked file.
func (m *Meta) IsLink() bool {
	return m.Path != ""
}

// Store reads and writes the store directory.
type Store struct {
	root string
	home string
	// templates draw the pages of data files.
	templates *tmpl.Library

	mu sync.Mutex
	// LIMIT: entries of unlinked paths stay (a few hundred bytes each). Drop them on removal if a
	// server ever sees tens of thousands of paths in its lifetime.
	titles map[string]cachedTitle
	// drawn is what the last drawing of each data file read besides it, by slug and file (see
	// sourceKey).
	// LIMIT: one entry for every slug and file ever drawn, removed, renamed, relinked and republished
	// ones too (a republish's copy is a new file), as titles are. Drop them on removal if a server comes
	// to see thousands of them in its lifetime.
	drawn map[string]drawing
}

type cachedTitle struct {
	key, title string
}

// New returns the store at root. home is the user's home directory, which bounds what may be linked,
// and templates draw the pages of data files.
func New(root, home string, templates *tmpl.Library) *Store {
	return &Store{
		root: root, home: home, templates: templates, mu: sync.Mutex{}, titles: map[string]cachedTitle{},
		drawn: map[string]drawing{},
	}
}

// Root returns the store directory.
func (s *Store) Root() string {
	return s.root
}

// Templates are the templates the store draws data files with: the server's, whoever published them.
func (s *Store) Templates() *tmpl.Library {
	return s.templates
}

func (s *Store) dir(slug string) string {
	return filepath.Join(s.root, slug)
}

// ReadMeta returns the meta.json of slug, or nil when it is missing or unreadable.
func (s *Store) ReadMeta(slug string) *Meta {
	b, err := os.ReadFile(filepath.Join(s.dir(slug), "meta.json"))
	if err != nil {
		return nil
	}
	var m *Meta
	if json.Unmarshal(b, &m) != nil {
		return nil
	}
	return m
}

// WriteMeta replaces the meta.json of slug.
func (s *Store) WriteMeta(slug string, m *Meta) error {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return fmt.Errorf("encode meta.json of %s: %w", slug, err)
	}
	return WriteAtomic(filepath.Join(s.dir(slug), "meta.json"), b.Bytes(), 0o600)
}

// WriteAtomic writes data to path with the mode perm, less the umask, through a file beside it, so that a
// reader sees the old content or the new, never a part. The file beside it is a new one of its own
// (O_EXCL, under a random name), so that one left there, or placed there, cannot keep its mode or lead
// the write elsewhere.
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	f, err := createNew(path+".tmp-", perm)
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// createNew creates a file named prefix and a random suffix, with the mode perm less the umask (which
// os.CreateTemp does not take), trying another name while one is taken.
func createNew(prefix string, perm os.FileMode) (*os.File, error) {
	for {
		name := prefix + rand.Text()
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm) //nolint:gosec // G304: beside the file written
		if !errors.Is(err, fs.ErrExist) {
			return f, err //nolint:wrapcheck // WriteAtomic says what it wrote
		}
	}
}

// File returns the file to serve for rest under slug: a stored page has its data file only, a link its
// registered file and the files under its directory (see link.Resolve). A linked data file has its
// own file only: the images of its page are in the page.
func (s *Store) File(slug string, m *Meta, rest string) (string, bool) {
	if m.IsLink() && (rest == "" || !IsDataFile(m.Path)) {
		return link.Resolve(m.Path, rest)
	}
	if m.IsLink() || rest != "" || !copyName(m.Source) {
		return "", false
	}
	return filepath.Join(s.dir(slug), m.Source, tmpl.KeptName), true
}

// copyName reports whether source may name the copy of a stored page: one name a save gives, or none
// (a page from before copies, which has none to read). Another, in a meta.json written by hand, could
// lead out of the page's directory.
func copyName(source string) bool {
	return source == "" || source == filepath.Base(source) && strings.HasPrefix(source, sourcePrefix)
}

// IsDrawn reports whether the page of m is drawn from a data file each time it is opened: a stored
// page, or a linked data file.
func (m *Meta) IsDrawn() bool {
	return !m.IsLink() || IsDataFile(m.Path)
}

// Draw draws the page of the data file file of slug, whose meta.json is m: in its language, with the
// themes given over the data's if any, under the title pinned for it, else its title:, else slug. What
// the drawing read besides file is kept for Version, mistakes or not, and version is the Version the
// page was drawn at.
// LIMIT: draws on every opening, a few ms for a page of 100 KB, its images read and encoded again
// each time. Keep the page drawn while its Version stays if pages of megabytes of images come to be
// opened often.
func (s *Store) Draw(slug string, m *Meta, file string) (page []byte, version string, err error) {
	// Taken before the reading, so that a change in between costs a reload rather than a miss; but
	// only once what the drawing reads is known, or the first drawing would reload at once.
	key := sourceKey(slug, file)
	s.mu.Lock()
	last, ok := s.drawn[key]
	s.mu.Unlock()
	// Published again, it may be drawn with another theme: its version waits for this drawing.
	known := ok && last.at == m.UpdatedAt
	if known {
		version = s.Version(slug, m, file)
	}
	src, err := os.ReadFile(file) //nolint:gosec // G304: file passed File
	if err != nil {
		// What it read before stays watched; that it cannot be read is in its version.
		err = fmt.Errorf("%w: %w", ErrNotFound, usererr.CannotRead(file, err))
	} else {
		var d *tmpl.Data
		var sources tmpl.Sources
		d, sources, err = s.templates.ReadSources(file, src, m.themes(), "")
		s.mu.Lock()
		s.drawn[key] = drawing{at: m.UpdatedAt, sources: sources}
		s.mu.Unlock()
		if err == nil {
			page, err = drawData(d, m, slug)
		}
	}
	if !known {
		version = s.Version(slug, m, file)
	}
	return page, version, err
}

// themes are the themes m draws its data file with over the data's.
func (m *Meta) themes() tmpl.Themes {
	return tmpl.Themes{Template: m.Theme, Parts: m.PartsTheme}
}

// drawing is what a drawing of a data file read besides it, and the publish it drew (its
// updatedAt).
type drawing struct {
	at      string
	sources tmpl.Sources
}

// sourceKey keys what a drawing read: one file linked under two slugs may be drawn with two themes, and a
// slug removed in another process (floe rm) may come to link another file.
func sourceKey(slug, file string) string {
	return slug + "\x00" + file
}

// drawData draws d, the data file of slug, under the title pinned for it, else its title:, else slug.
func drawData(d *tmpl.Data, m *Meta, slug string) ([]byte, error) {
	return d.Render(cmp.Or(m.Title, d.Title, slug)) //nolint:wrapcheck // a usererr for the caller to show
}

// Version is what live reload compares: a linked file's mtime, and for a data file of slug, stored or
// linked, its publish time too (another theme) and the state of what its last drawing
// read besides it (its template's files, its images) and whether it can be read, so that publishing
// it again, saving any of them, or making it readable again reloads the page.
// It is "" when the file is gone.
func (s *Store) Version(slug string, m *Meta, file string) string {
	v := modTime(file)
	if v == "" || !m.IsDrawn() {
		return v
	}
	s.mu.Lock()
	last := s.drawn[sourceKey(slug, file)]
	s.mu.Unlock()
	v += "+" + m.UpdatedAt + "+" + stamp(last.sources)
	if unix.Access(file, unix.R_OK) != nil {
		return v + "+unreadable"
	}
	return v
}

// modTime is the mtime of file, or "" when it cannot be read.
func modTime(file string) string {
	fi, err := os.Stat(file)
	if err != nil {
		return ""
	}
	return fi.ModTime().UTC().Format(isoMillis)
}

// stamp sums up the state of sources: the size and mtime of each image, of each index of icons and of
// every file under the template's directory and under the directory of parts of one's own, and the mtime
// of the directory of templates, where one may come.
// LIMIT: walks the template's directory and the parts' on each look (every second for each open tab).
// Watch them with fsnotify if they grow to hundreds of files.
func stamp(sources tmpl.Sources) string {
	h := fnv.New64a()
	add := func(p string) {
		fi, err := os.Stat(p)
		if err != nil {
			_, _ = fmt.Fprintf(h, "%s:-\n", p) //nolint:errcheck // a hash takes every write
			return
		}
		_, _ = fmt.Fprintf(h, "%s:%d:%d\n", p, fi.Size(), fi.ModTime().UnixNano()) //nolint:errcheck // as above
	}
	if sources.Templates != "" {
		add(sources.Templates)
	}
	addTree(sources.Template, add, false)
	addTree(sources.Parts, add, true)
	for _, file := range slices.Concat(sources.Images, sources.Icons) {
		add(file)
	}
	return strconv.FormatUint(h.Sum64(), 36)
}

// addTree adds each file under dir, "" for none, to a stamp with add, but those of a name starting with a
// dot (.git), no file of a template or a part. With links, an entry of dir that is a symlink, a part of
// one's own kept elsewhere as a template may be, is walked through.
func addTree(dir string, add func(string), links bool) {
	if dir == "" {
		return
	}
	//nolint:errcheck // a file that cannot be walked to is left out of the sum
	_ = filepath.WalkDir(dir, func(p string, e fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return nil //nolint:nilerr // as above
		case p != dir && strings.HasPrefix(e.Name(), "."):
			if e.IsDir() {
				return fs.SkipDir
			}
		case links && e.Type()&fs.ModeSymlink != 0 && filepath.Dir(p) == dir:
			if to, err := filepath.EvalSymlinks(p); err == nil {
				addTree(to, add, false)
			} else {
				add(p)
			}
		case !e.IsDir():
			add(p)
		}
		return nil
	})
}

// Listed is an entry of the list. A link takes its title and update time from the file; Missing is
// set for links only, true when the file cannot be served.
type Listed struct {
	Slug string `json:"slug"`
	Meta

	Missing *bool `json:"missing,omitempty"`
}

// List returns the pages, newest first. Entries with a missing or broken meta.json are skipped so that
// one of them does not break the whole list.
func (s *Store) List() ([]Listed, error) {
	dirents, err := os.ReadDir(s.root)
	if errors.Is(err, os.ErrNotExist) {
		return []Listed{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", s.root, err)
	}
	list := []Listed{}
	for _, d := range dirents {
		slug := d.Name()
		m := s.ReadMeta(slug)
		if !ValidSlug(slug) || m == nil || m.UpdatedAt == "" {
			continue
		}
		list = append(list, s.listed(slug, m))
	}
	slices.SortStableFunc(list, func(a, b Listed) int {
		return cmp.Compare(b.UpdatedAt, a.UpdatedAt)
	})
	return list, nil
}

func (s *Store) listed(slug string, m *Meta) Listed {
	l := Listed{Slug: slug, Meta: *m, Missing: nil}
	if !m.IsLink() {
		return l
	}
	// A file that is not served is not read either (the same check as File).
	title, readable := "", false
	file, ok := s.File(slug, m, "")
	if ok {
		title, readable = s.linkTitle(file, m.IsDrawn())
	}
	if l.Title == "" {
		l.Title = cmp.Or(title, slug)
	}
	if v := modTime(file); readable && v != "" {
		l.UpdatedAt = v
	}
	missing := !readable
	l.Missing = &missing
	return l
}

// linkTitle returns the title of a linked file, its <title> or first <h1>, or the title: of a data
// file, cached per version of the file: the management page and the open pages poll the list every few
// seconds, and a report can be megabytes. A data file with mistakes has none, but can be read.
func (s *Store) linkTitle(file string, data bool) (string, bool) {
	var st unix.Stat_t
	if unix.Stat(file, &st) != nil {
		return "", false
	}
	key := fmt.Sprintf("%d:%d:%d:%d", st.Ino, st.Size, st.Mtim.Nano(), st.Ctim.Nano())
	s.mu.Lock()
	hit, ok := s.titles[file]
	s.mu.Unlock()
	if ok && hit.key == key {
		return hit.title, true
	}
	b, err := os.ReadFile(file) //nolint:gosec // G304: file passed link.Resolve
	if err != nil {
		return "", false
	}
	var title string
	if data {
		title = tmpl.TitleOf(b)
	} else {
		title = page.Title(string(b))
	}
	s.mu.Lock()
	s.titles[file] = cachedTitle{key: key, title: title}
	s.mu.Unlock()
	return title, true
}

// Now is the time recorded in createdAt and updatedAt.
func Now() string {
	return time.Now().UTC().Format(isoMillis)
}
