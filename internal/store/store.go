// Package store keeps the pages under the store directory: <slug>/index.html and meta.json for a
// stored page, meta.json alone for a linked file. The layout and meta.json are the JS version's, so
// either can read the other's store.
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
	"strconv"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	"github.com/sukekyo26/floe/internal/link"
	"github.com/sukekyo26/floe/internal/page"
)

const isoMillis = "2006-01-02T15:04:05.000Z"

var slugRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$`)

// ErrNotFound is returned for a slug with no page.
var ErrNotFound = errors.New("no such page")

// ValidSlug reports whether s can name a page.
func ValidSlug(s string) bool {
	return slugRe.MatchString(s)
}

// Meta is meta.json. The field order and the omitted empty keys follow the JS version's output.
type Meta struct {
	// Path is the real path of a linked file; a stored page has none.
	Path  string `json:"path,omitempty"`
	Title string `json:"title,omitempty"`
	// TitleEdited and ProjectEdited mark values set on the management page, kept across republishes.
	TitleEdited bool   `json:"titleEdited,omitempty"`
	Description string `json:"description"`
	// Lang is the language a body-only page is wrapped in (a BCP 47 tag); none is page.DefaultLang.
	Lang          string `json:"lang,omitempty"`
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

	mu sync.Mutex
	// LIMIT: entries of unlinked paths stay (a few hundred bytes each). Drop them on removal if a
	// server ever sees tens of thousands of paths in its lifetime.
	titles map[string]cachedTitle
}

type cachedTitle struct {
	key, title string
}

// New returns the store at root. home is the user's home directory, which bounds what may be linked.
func New(root, home string) *Store {
	return &Store{root: root, home: home, mu: sync.Mutex{}, titles: map[string]cachedTitle{}}
}

// Root returns the store directory.
func (s *Store) Root() string {
	return s.root
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
	return writeAtomic(filepath.Join(s.dir(slug), "meta.json"), b.Bytes())
}

func writeAtomic(path string, data []byte) error {
	tmp := path + ".tmp-" + strconv.Itoa(os.Getpid())
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// File returns the file to serve for rest under slug: a stored page has its index.html only, a link
// its registered file and the files under its directory (see link.Resolve).
func (s *Store) File(slug string, m *Meta, rest string) (string, bool) {
	if m.IsLink() {
		return link.Resolve(m.Path, rest)
	}
	if rest != "" {
		return "", false
	}
	return filepath.Join(s.dir(slug), "index.html"), true
}

// Version is what live reload compares: a stored page's publish time, a link's file mtime ("" when
// the file cannot be read).
func Version(m *Meta, file string) string {
	if !m.IsLink() {
		return m.UpdatedAt
	}
	fi, err := os.Stat(file)
	if err != nil {
		return ""
	}
	return fi.ModTime().UTC().Format(isoMillis)
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
		title, readable = s.linkTitle(file)
	}
	if l.Title == "" {
		l.Title = cmp.Or(title, slug)
	}
	if v := Version(m, file); readable && v != "" {
		l.UpdatedAt = v
	}
	missing := !readable
	l.Missing = &missing
	return l
}

// linkTitle returns the title of a linked file, cached per version of the file: the management page
// and the open pages poll the list every few seconds, and a report can be megabytes.
func (s *Store) linkTitle(file string) (string, bool) {
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
	title := page.Title(string(b))
	s.mu.Lock()
	s.titles[file] = cachedTitle{key: key, title: title}
	s.mu.Unlock()
	return title, true
}

// Now is the time recorded in createdAt and updatedAt.
func Now() string {
	return time.Now().UTC().Format(isoMillis)
}
