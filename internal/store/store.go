// Package store keeps the pages under the store directory: <slug>/meta.json for each, and for a stored page
// <slug>/source-<id>/, which meta.json names: the data file published (data.yaml), the page drawn from it
// (page.html, without its CSP, which goes with it when it is served) and the images it shows (images/).
package store

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/files"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/link"
	"github.com/sukekyo26/floe/internal/page"
	"github.com/sukekyo26/floe/internal/usererr"
)

const (
	isoMillis = "2006-01-02T15:04:05.000Z"
	// sourcePrefix names a copy under a stored page.
	sourcePrefix = "source-"
	dataName     = "data.yaml"
	pageName     = "page.html"
	imagesName   = "images"
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
	// Theme is the theme a data file is drawn with over the data's (floe publish --theme), and DataTitle
	// its title: when published, which clearing the title on the management page gives back.
	Theme     string `json:"theme,omitempty"`
	DataTitle string `json:"dataTitle,omitempty"`
	// Source is the directory of a stored page's copy, under its own; a new one each publish and rebuild.
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

// Drawing is what pages are drawn with: the templates, parts and themes, and the icons of the diagrams; or, with
// Err, why they cannot be read (one's own part that is broken), which every drawing refuses with.
type Drawing struct {
	Kit   *kit.Kit
	Icons diagram.Icons
	Err   error
}

// Store reads and writes the store directory.
type Store struct {
	root string

	mu sync.Mutex
	// LIMIT: entries of unlinked paths stay (a few hundred bytes each). Drop them on removal if a
	// server ever sees tens of thousands of paths in its lifetime.
	titles map[string]cachedTitle
}

type cachedTitle struct {
	key, title string
}

// New returns the store at root.
func New(root string) *Store {
	return &Store{root: root, mu: sync.Mutex{}, titles: map[string]cachedTitle{}}
}

// dir is the directory of slug, the one way a slug becomes a path: false for one that cannot be a slug (..,
// a /), which no page has.
func (s *Store) dir(slug string) (string, bool) {
	if !ValidSlug(slug) {
		return "", false
	}
	return filepath.Join(s.root, slug), true
}

// copyDir is the directory of the copy of the stored page slug, whose meta.json is m, the one way a copy
// becomes a path: false for a link, or a page whose meta.json names no copy, or one that is no name a save
// gives (written by hand, it could lead out of the page's directory).
func (s *Store) copyDir(slug string, m *Meta) (string, bool) {
	dir, ok := s.dir(slug)
	source := m.Source
	if !ok || m.IsLink() || source != filepath.Base(source) || !strings.HasPrefix(source, sourcePrefix) {
		return "", false
	}
	return filepath.Join(dir, source), true
}

// ReadMeta returns the meta.json of slug, or nil when it is missing, unreadable or slug cannot be one.
func (s *Store) ReadMeta(slug string) *Meta {
	dir, ok := s.dir(slug)
	if !ok {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(dir, "meta.json"))
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
	dir, ok := s.dir(slug)
	if !ok {
		//nolint:err113 // floe's own mistake
		return fmt.Errorf("store: write the meta.json of %q, which cannot be a slug", slug)
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(m); err != nil {
		return fmt.Errorf("encode meta.json of %s: %w", slug, err)
	}
	file := filepath.Join(dir, "meta.json")
	if err := files.WriteAtomic(file, b.Bytes(), 0o600); err != nil {
		return usererr.CannotWrite(file, err) //nolint:wrapcheck // the system's words, at the file
	}
	return nil
}

// File returns the file to serve for rest under slug: a stored page has its page drawn only, a link its
// registered file and the files under its directory (see link.Resolve). A linked data file has its own file
// only: the images of its page are in the page.
func (s *Store) File(slug string, m *Meta, rest string) (string, bool) {
	if _, ok := s.dir(slug); !ok {
		return "", false
	}
	if m.IsLink() {
		if rest != "" && files.IsDataFile(m.Path) {
			return "", false
		}
		return link.Resolve(m.Path, rest)
	}
	dir, ok := s.copyDir(slug, m)
	if !ok || rest != "" {
		return "", false
	}
	return filepath.Join(dir, pageName), true
}

// Listed is an entry of the list. A link takes its title and update time from the file; Missing is
// set for links only, true when the file cannot be served.
type Listed struct {
	Slug string `json:"slug"`
	Meta

	Missing *bool `json:"missing,omitempty"`
}

// slugs returns the slugs of the store, in their order: the names under it that can be one.
func (s *Store) slugs() ([]string, error) {
	dirents, err := os.ReadDir(s.root)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, usererr.CannotRead(s.root, err) //nolint:wrapcheck // the system's words, at the store
	}
	var slugs []string
	for _, d := range dirents {
		if ValidSlug(d.Name()) {
			slugs = append(slugs, d.Name())
		}
	}
	return slugs, nil
}

// List returns the pages, newest first. Entries with a missing or broken meta.json are skipped so that
// one of them does not break the whole list.
func (s *Store) List() ([]Listed, error) {
	slugs, err := s.slugs()
	if err != nil {
		return nil, err
	}
	list := []Listed{}
	for _, slug := range slugs {
		m := s.ReadMeta(slug)
		if m == nil || m.UpdatedAt == "" {
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
		title, readable = s.linkTitle(file, files.IsDataFile(m.Path))
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

// modTime is the mtime of file, or "" when it cannot be read.
func modTime(file string) string {
	fi, err := os.Stat(file)
	if err != nil {
		return ""
	}
	return fi.ModTime().UTC().Format(isoMillis)
}

// linkTitle returns the title of a linked file, its <title> or first <h1>, or the title: of a data
// file, cached per version of the file: the management page and the open pages poll the list every few
// seconds, and a report can be megabytes. A data file with mistakes may have none, but can be read.
func (s *Store) linkTitle(file string, data bool) (string, bool) {
	key := files.Mark(file)
	if key == "-" {
		return "", false
	}
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
		if d, _ := doc.Read(file, b, ""); d != nil { //nolint:errcheck // its mistakes are told when it is drawn
			title = d.Title
		}
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

// Root returns the store directory.
func (s *Store) Root() string {
	return s.root
}
