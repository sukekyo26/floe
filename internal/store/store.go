// Package store keeps the pages under the store directory: <slug>/meta.json for each, and for a stored page
// <slug>/source-<id>/, which meta.json names: the data file published (data.yaml), the page drawn from it
// (page.html, without its CSP, which goes with it when it is served) and the images it shows (images/). The versions
// of a stored page are in meta.json, its last the copy's page, the ones before gzipped under <slug>/versions/.
package store

import (
	"bytes"
	"cmp"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"floe/internal/diagram"
	"floe/internal/doc"
	"floe/internal/files"
	"floe/internal/kit"
	"floe/internal/link"
	"floe/internal/page"
	"floe/internal/usererr"
)

const (
	isoMillis = "2006-01-02T15:04:05.000Z"
	// sourcePrefix names a copy under a stored page.
	sourcePrefix = "source-"
	dataName     = "data.yaml"
	pageName     = "page.html"
	imagesName   = "images"
	versionsName = "versions"
	// keepVersions is how many versions a stored page keeps, the oldest going first.
	keepVersions = 20
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
	// ID names the page for good, so that what was read of it stays when it is renamed or published again: made when
	// it is first stored, or replaced by another file's (newMeta).
	ID string `json:"id,omitempty"`
	// Path is the real path of a linked file; a stored page has none.
	Path  string `json:"path,omitempty"`
	Title string `json:"title,omitempty"`
	// TitleEdited marks a title set on the management page, kept across republishes.
	TitleEdited bool   `json:"titleEdited,omitempty"`
	Description string `json:"description"`
	// Theme is the theme a data file is drawn with over the data's (floe publish --theme), and DataTitle
	// its title: when published, which clearing the title on the management page gives back; of a folder, where it is
	// in its repository when linked (docs/v1), else its name.
	Theme     string `json:"theme,omitempty"`
	DataTitle string `json:"dataTitle,omitempty"`
	// CreatedTheme is the default theme of settings.json when a page drawn from a data file was first stored, or
	// replaced by another file's: the default it is drawn with for good, under --theme and the data's theme:.
	CreatedTheme string `json:"createdTheme,omitempty"`
	// PickedTheme is the theme picked for the page on the management page (PickTheme), over --theme: kept across
	// republishes and rebuilds, dropped when another file's page replaces it.
	PickedTheme string `json:"pickedTheme,omitempty"`
	// Template is the template a stored page is drawn with, read from its data file at each publish and rebuild.
	Template string `json:"template,omitempty"`
	// Source is the directory of a stored page's copy, under its own; a new one each publish and rebuild.
	Source string `json:"source,omitempty"`
	// Project is the page's project on the lists and the task list: the one a new page is published from, kept when it
	// is published again from anywhere, and set on the management page.
	Project string `json:"project,omitempty"`
	// PublishedFrom and PublishedFile are the key of the data file that owns a stored page (project.Place): the file's
	// repository and its path from the repository's root, else "" and its real path. A page published before keys
	// were the file's alone holds the project it was published from in PublishedFrom.
	PublishedFrom string `json:"publishedFrom,omitempty"`
	PublishedFile string `json:"publishedFile,omitempty"`
	Favorite      bool   `json:"favorite,omitempty"`
	// Folder marks a link of a folder, which serves the files under it and lists its pages a directory at a time.
	Folder bool `json:"folder,omitempty"`
	// Archived keeps the page off the list (floe list and the management page), served and drawn as any.
	Archived bool `json:"archived,omitempty"`
	// Tags are the page's tags, as tagged keeps them.
	Tags []string `json:"tags,omitempty"`
	// Work is its place on the task list; its task's state is in tasks.json.
	Work *Work `json:"work,omitempty"`
	// Versions are a stored page's versions, oldest first: a publish of other content than the last's adds one.
	Versions  []Version `json:"versions,omitempty"`
	CreatedAt string    `json:"createdAt"`
	UpdatedAt string    `json:"updatedAt"`
}

// Version is a version of a stored page: its number, never going back within the page; when the publish that made it
// was (its updatedAt); and the mark of what it was published with (contentMark).
type Version struct {
	N    int    `json:"n"`
	At   string `json:"at"`
	Mark string `json:"mark"`
}

// IsLink reports whether the entry is a linked file or folder.
func (m *Meta) IsLink() bool {
	return m.Path != ""
}

// GivenTheme is the theme the page is drawn in over its data's theme:, "" for none: the one picked on the management
// page, else --theme.
func (m *Meta) GivenTheme() string {
	return cmp.Or(m.PickedTheme, m.Theme)
}

// TakesTheme reports whether a theme can be picked for the entry: a page drawn from a data file, stored or linked.
func (m *Meta) TakesTheme() bool {
	return !m.IsLink() || m.IsData()
}

// IsData reports whether the entry is a linked data file, drawn each time it is opened: by what it is, so that a
// folder named config.yml is none.
func (m *Meta) IsData() bool {
	return m.IsLink() && !m.Folder && files.IsDataFile(m.Path)
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
	key, title, template string
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

// versionFile is the file of version n of the stored page slug, whose meta.json is m, and where m lists it: the one
// way a version's number becomes a path, false for a number m does not list. The last is the page of the copy, the
// ones before (lastVersion tells) gzipped under versions/.
func (s *Store) versionFile(slug string, m *Meta, n int) (file string, at int, ok bool) {
	at = slices.IndexFunc(m.Versions, func(v Version) bool { return v.N == n })
	if m.IsLink() || at < 0 {
		return "", -1, false
	}
	if lastVersion(m, at) {
		copied, isCopy := s.copyDir(slug, m)
		return filepath.Join(copied, pageName), at, isCopy
	}
	dir, isDir := s.dir(slug)
	return filepath.Join(dir, versionsName, strconv.Itoa(n)+".html.gz"), at, isDir
}

// lastVersion tells whether the version at of m is its last, its copy's page, not gzipped.
func lastVersion(m *Meta, at int) bool {
	return at == len(m.Versions)-1
}

// VersionMark is what tells version n of the stored page slug, whose meta.json is m, from any other, without reading
// it: its number and when it was made, a number coming again after a removal or a rename; for the last, its copy too,
// which a rebuild draws again. "" for a version m does not list.
func (s *Store) VersionMark(slug string, m *Meta, n int) string {
	_, at, ok := s.versionFile(slug, m, n)
	if !ok {
		return ""
	}
	mark := "v" + strconv.Itoa(n) + "." + m.Versions[at].At
	if lastVersion(m, at) {
		mark += "." + m.Source
	}
	return mark
}

// VersionPage is the page of version n of the stored page slug, whose meta.json is m, as it was drawn: false when m
// lists no such version or it cannot be read whole.
func (s *Store) VersionPage(slug string, m *Meta, n int) ([]byte, bool) {
	file, at, ok := s.versionFile(slug, m, n)
	if !ok {
		return nil, false
	}
	b, _, err := files.Read(file)
	if err != nil {
		return nil, false
	}
	if lastVersion(m, at) {
		return b, true
	}
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, false
	}
	// Read to its end, where gzip checks its sum and length: one cut short or broken is no page.
	page, err := io.ReadAll(r)
	return page, err == nil
}

// Pair is two versions of a stored page, of one meta.json, as a diff compares them: their pages, and the key that
// ties what is told of the diff to its pages (PairKey).
type Pair struct {
	Pages [2][]byte
	Key   string
}

// VersionPair is the versions from and to of the stored page slug, whose meta.json is m: false when m lists one of
// them not, or one cannot be read.
func (s *Store) VersionPair(slug string, m *Meta, from, to int) (Pair, bool) {
	var p, none Pair
	for i, n := range [2]int{from, to} {
		page, read := s.VersionPage(slug, m, n)
		if !read {
			return none, false
		}
		p.Pages[i] = page
	}
	p.Key = s.PairKey(slug, m, from, to)
	return p, true
}

// PairKey is what tells the pair of versions from and to of the stored page slug, whose meta.json is m, from any
// other, without reading them: of their marks (VersionMark), which a rebuild of the last, or a number coming again
// after a removal, changes. "" for a version m does not list.
func (s *Store) PairKey(slug string, m *Meta, from, to int) string {
	a, b := s.VersionMark(slug, m, from), s.VersionMark(slug, m, to)
	if a == "" || b == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(a + "\x00" + b))
	return hex.EncodeToString(sum[:8])
}

// gzipped is page gzipped, as a version before the last is kept.
func gzipped(page []byte) []byte {
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	_, _ = w.Write(page) //nolint:errcheck // a buffer takes every write
	_ = w.Close()        //nolint:errcheck // as above
	return b.Bytes()
}

// contentMark is the mark of what a stored page is published with: the data file src and the images it read, by
// name. Each part goes with its length before it, so that no two sets come to the same bytes.
func contentMark(src []byte, images map[string][]byte) string {
	h := sha256.New()
	put := func(b []byte) {
		h.Write(binary.BigEndian.AppendUint64(nil, uint64(len(b))))
		h.Write(b)
	}
	put(src)
	for _, name := range slices.Sorted(maps.Keys(images)) {
		put([]byte(name))
		put(images[name])
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
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
	if err := files.WriteAtomic(file, b.Bytes(), 0o600, nil); err != nil {
		return usererr.CannotWrite(file, err) //nolint:wrapcheck // the system's words, at the file
	}
	return nil
}

// File returns the file to serve for rest under slug, when it is there as a regular file: a stored page has its
// page drawn only, a link its registered file and the files under its directory, a folder the files under it (see
// link.Resolve). A linked data file has its own file only: the images of its page are in the page.
func (s *Store) File(slug string, m *Meta, rest string) (string, bool) {
	if _, ok := s.dir(slug); !ok {
		return "", false
	}
	if m.IsLink() {
		if rest != "" && m.IsData() {
			return "", false
		}
		return link.Resolve(m.Path, m.Folder, rest)
	}
	dir, ok := s.copyDir(slug, m)
	if !ok || rest != "" {
		return "", false
	}
	page := filepath.Join(dir, pageName)
	if info, err := os.Stat(page); err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return page, true
}

// Listed is an entry of the list. A link takes its title and update time from the file, a linked data file its template
// too; Missing is set for links only, true when the file cannot be served. News (newsNew, newsVersion, "" for nothing
// unseen), Version, the number of the last, and VersionCount, how many are kept, are of a stored page only.
type Listed struct {
	Slug string `json:"slug"`
	Meta

	// TaskProject is the project of a task of this page, as tasks.json has it (ProjectKey).
	TaskProject string `json:"taskProject"`
	News        string `json:"news,omitempty"`
	Version     int    `json:"version,omitempty"`
	// VersionCount tells whether there are versions to compare, the versions themselves left out of the list.
	VersionCount int   `json:"versionCount,omitempty"`
	Missing      *bool `json:"missing,omitempty"`
	// Themed tells a page a theme can be picked for (Meta.TakesTheme).
	Themed bool `json:"themed,omitempty"`
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

// List returns the pages, newest first, with what viewer has not seen of them. Entries with a missing or broken
// meta.json are skipped so that one of them does not break the whole list.
func (s *Store) List(viewer string) ([]Listed, error) {
	slugs, err := s.slugs()
	if err != nil {
		return nil, err
	}
	// Whatever cannot be read leaves no reader: every page counts as read.
	all, _ := s.readReads() //nolint:errcheck // as above
	reader := readerIn(all, viewer)
	list := []Listed{}
	for _, slug := range slugs {
		m := s.ReadMeta(slug)
		if m == nil || m.UpdatedAt == "" {
			continue
		}
		list = append(list, s.listed(slug, m, reader))
	}
	slices.SortStableFunc(list, func(a, b Listed) int {
		return cmp.Compare(b.UpdatedAt, a.UpdatedAt)
	})
	return list, nil
}

func (s *Store) listed(slug string, m *Meta, r *reader) Listed {
	l := Listed{
		Slug: slug, Meta: *m, TaskProject: ProjectKey(m.Project), News: "", Version: 0, VersionCount: 0, Missing: nil,
		Themed: m.TakesTheme(),
	}
	// The list is read every few seconds: versions are asked for a page at a time.
	l.Versions = nil
	if !m.IsLink() {
		l.News = r.news(m)
		l.Version = m.LastNumber()
		l.VersionCount = len(m.Versions)
		return l
	}
	// What is in a folder is listed when it is opened (Pages): the list is read every few seconds.
	if m.Folder {
		// Where it is in its repository (docs, docs/v1), else its name: its project heads its group
		l.Title = cmp.Or(l.Title, m.DataTitle, filepath.Base(m.Path))
		missing := !link.IsFolder(m.Path)
		l.Missing = &missing
		return l
	}
	// A file that is not served is not read either (the same check as File).
	title, template, readable := "", "", false
	file, ok := s.File(slug, m, "")
	if ok {
		title, template, readable = s.linkTitle(file, m.IsData())
	}
	l.Template = template
	if l.Title == "" {
		l.Title = cmp.Or(title, slug)
	}
	// The later of the two: a file written, or linked again, comes up the list.
	if v := modTime(file); readable && v != "" {
		l.UpdatedAt = max(v, l.UpdatedAt)
	}
	missing := !readable
	l.Missing = &missing
	return l
}

// Files lists the directory dir ("" for the top) of the folder slug links, as link.Dir does: ErrNotFound for a slug
// that is no folder's link or a directory that is none of it.
func (s *Store) Files(slug, dir string) (link.Listing, error) {
	m := s.ReadMeta(slug)
	if m == nil || !m.Folder {
		return link.Listing{}, ErrNotFound
	}
	l, err := link.Dir(m.Path, dir)
	if errors.Is(err, link.ErrNoDir) {
		return l, ErrNotFound
	}
	if err != nil {
		return l, usererr.CannotRead(m.Path, err) //nolint:wrapcheck // the system's words, at the folder
	}
	return l, nil
}

// modTime is the mtime of file, or "" when it cannot be read.
func modTime(file string) string {
	fi, err := os.Stat(file)
	if err != nil {
		return ""
	}
	return fi.ModTime().UTC().Format(isoMillis)
}

// linkTitle returns the title of a linked file, its <title> or first <h1>, or the title: and template: of a data
// file, cached per version of the file: the management page and the open pages poll the list every few
// seconds, and a report can be megabytes. A data file with mistakes may have none, but can be read.
func (s *Store) linkTitle(file string, data bool) (title, template string, readable bool) {
	key := files.Mark(file)
	if key == "-" {
		return "", "", false
	}
	s.mu.Lock()
	hit, ok := s.titles[file]
	s.mu.Unlock()
	if ok && hit.key == key {
		return hit.title, hit.template, true
	}
	b, _, err := files.Read(file)
	if err != nil {
		return "", "", false
	}
	if data {
		if d, _ := doc.Read(file, b, ""); d != nil { //nolint:errcheck // its mistakes are told when it is drawn
			title, template = d.Title, d.Template
		}
	} else {
		title = page.Title(string(b))
	}
	s.mu.Lock()
	s.titles[file] = cachedTitle{key: key, title: title, template: template}
	s.mu.Unlock()
	return title, template, true
}

// Now is the time recorded in createdAt and updatedAt.
func Now() string {
	return time.Now().UTC().Format(isoMillis)
}

// Root returns the store directory.
func (s *Store) Root() string {
	return s.root
}
