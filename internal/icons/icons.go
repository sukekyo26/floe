// Package icons keeps icon sets on the user's machine and finds their icons by name. floe does not ship
// them (that would redistribute them): each user fetches a set's official package once. An icon is
// named by its set and its name in the set, like aws/amazon-ec2.
package icons

import (
	"archive/zip"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/sukekyo26/floe/internal/textpos"
	"github.com/sukekyo26/floe/internal/usererr"
)

// kit is what differs between icon sets: where their package is and which of its files are icons.
type kit struct {
	name       string // the head of the set's icon names, and its directory under the root
	title      string // the set as messages name it
	page       string // the page that links the package
	host, size string // where the package downloads from and about how large it is
	link       *regexp.Regexp
	version    *regexp.Regexp // the package's version in its URL
	iconOf     func(path string) (packed, bool)
}

// packed is an icon in a package: dark marks its variant for dark backgrounds, kept only for an icon
// without a light one.
type packed struct {
	version, name, category string
	dark                    bool
}

var kits = []*kit{&aws}

var (
	// ErrNoSet is a name without a known set in front (aws/…).
	ErrNoSet = errors.New("no icon set in front")
	// ErrNoIcon is a name its set does not hold.
	ErrNoIcon = errors.New("no such icon")
)

var (
	slugRe   = regexp.MustCompile(`[_\s]+`)
	dashesRe = regexp.MustCompile(`-+`)
)

// rename moves the new icons into place; a test makes it fail.
var rename = os.Rename

// Sets are the names of the icon sets floe can fetch.
func Sets() []string {
	names := make([]string, len(kits))
	for i, k := range kits {
		names[i] = k.name
	}
	return names
}

// CheckSet refuses a set that floe cannot fetch, naming the ones it can.
func CheckSet(set string) error {
	if kitOf(set) == nil {
		return usererr.New("icons_unknown_set", set, strings.Join(Sets(), ", "))
	}
	return nil
}

func kitOf(set string) *kit {
	i := slices.IndexFunc(kits, func(k *kit) bool { return k.name == set })
	if i < 0 {
		return nil
	}
	return kits[i]
}

// Store is the icon sets kept under a root, a directory each.
type Store struct {
	root string
	// The indexes of the sets read so far, by set: a diagram looks up many icons.
	mu      sync.Mutex
	indexes map[string]*index
}

// New returns the store under root, an absolute path.
func New(root string) *Store {
	return &Store{root: root, mu: sync.Mutex{}, indexes: map[string]*index{}}
}

func (s *Store) dir(k *kit) string {
	return filepath.Join(s.root, k.name)
}

type index struct {
	Version string           `json:"version"`
	Source  string           `json:"source"`
	Icons   map[string]entry `json:"icons"`
}

type entry struct {
	Category string `json:"category"`
}

// readIndex returns nil when the set has not been fetched. One that cannot be read is told in the system's
// words; one that is no index, as a fetch again replaces it.
func (s *Store) readIndex(k *kit) (*index, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if idx := s.indexes[k.name]; idx != nil {
		return idx, nil
	}
	file := filepath.Join(s.dir(k), "index.json")
	b, err := os.ReadFile(file) //nolint:gosec // G304: the set's own index
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil //nolint:nilnil // not fetched is not an error to fetch
	}
	if err != nil {
		return nil, usererr.CannotRead(file, err)
	}
	var idx index
	if err = json.Unmarshal(b, &idx); err != nil {
		return nil, broken(file, b, err, k)
	}
	// JSON of another shape (null, {}) is no index a fetch wrote: each holds its version.
	if idx.Version == "" {
		return nil, usererr.NewAt(file, 0, 0, "icons_index_no_version", k.name)
	}
	s.indexes[k.name] = &idx
	return &idx, nil
}

// broken is err, of reading b, the index file, as JSON: in encoding/json's words, at the place it names.
func broken(file string, b []byte, err error, k *kit) error {
	var at int64
	var syntax *json.SyntaxError
	var typed *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntax):
		at = syntax.Offset
	case errors.As(err, &typed):
		at = typed.Offset
	default:
		return fmt.Errorf("read %s: %w", file, err)
	}
	line, column := textpos.Ending(b, int(at))
	return libraryAt(file, line, column, "icons_broken", err, k.name)
}

// libraryAt is the mistake a library told in err, its words the first argument of the catalog message key and
// args after them, at the line and column of file, 0 for none.
func libraryAt(file string, line, column int, key string, err error, args ...any) error {
	e := usererr.LibraryWith(key, err)
	e.Args = append(e.Args, args...)
	e.File, e.Line, e.Column = file, line, column
	return e
}

func (s *Store) loadIndex(k *kit) (*index, error) {
	idx, err := s.readIndex(k)
	if err != nil {
		return nil, err
	}
	if idx == nil {
		return nil, s.notFetched(k)
	}
	return idx, nil
}

func (s *Store) notFetched(k *kit) *usererr.Error {
	return usererr.New("icons_not_fetched", k.title, s.dir(k), k.name, k.size, k.host)
}

// Source is a package to fetch: At, a URL or an absolute path, and Name, as it was written, which its mistakes
// are told at. Without one, the package is the one the set's page links.
type Source struct {
	Name, At string
}

// Fetched is what Fetch did: kept Count icons of Version in Dir, or found Version already there.
type Fetched struct {
	Title   string // the set as messages name it
	Dir     string
	Version string
	Count   int
	Already bool
}

// Fetch keeps the icons of set (one of Sets) from its package zip in its directory, replacing what is there as
// a whole. A version already there is kept unless force; without force, the index there is read to tell it,
// and one that cannot be read stops the fetch, as it stops a search.
func (s *Store) Fetch(ctx context.Context, set string, from Source, force bool) (Fetched, error) {
	if err := CheckSet(set); err != nil {
		return Fetched{}, err //nolint:exhaustruct // nothing fetched
	}
	k := kitOf(set)
	got := Fetched{Title: k.title, Dir: s.dir(k), Version: "", Count: 0, Already: false}
	if from.At == "" {
		at, err := k.packageURL(ctx)
		if err != nil {
			return got, err
		}
		from = Source{Name: at, At: at}
	}
	if !force {
		current, err := s.readIndex(k)
		if err != nil {
			return got, err
		}
		if current != nil {
			got.Version = current.Version
		}
	}
	if k.tells(from.At, got.Version) {
		got.Already = true
		return got, nil
	}
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return got, usererr.CannotWrite(s.root, err)
	}
	s.clearLeftovers(k)
	tmp, err := os.MkdirTemp(s.root, k.name+".tmp-")
	if err != nil {
		return got, usererr.CannotWrite(s.root, err)
	}
	defer os.RemoveAll(tmp) //nolint:errcheck // a leftover is harmless
	idx, err := k.fetch(ctx, from, tmp)
	if err != nil {
		return got, err
	}
	if idx.Version == got.Version {
		got.Already = true
		return got, nil
	}
	if err = replace(tmp, got.Dir); err != nil {
		return got, err
	}
	s.mu.Lock()
	s.indexes[k.name] = idx
	s.mu.Unlock()
	got.Version, got.Count = idx.Version, len(idx.Icons)
	return got, nil
}

// clearLeftovers removes what a fetch of k stopped on its way left: its directories beside the set's.
// LIMIT: a fetch stopped between the two renames of replace leaves the set gone and its previous icons in a
// leftover, which this removes; make replace one rename (a symlink to swap) if that comes to matter.
func (s *Store) clearLeftovers(k *kit) {
	left, _ := filepath.Glob(filepath.Join(s.root, k.name+".tmp-*")) //nolint:errcheck // the pattern is well formed
	for _, dir := range left {
		_ = os.RemoveAll(dir) //nolint:errcheck // a leftover is harmless
	}
}

// tells reports whether at, the package's URL, tells it is of version: a URL may tell its version, which saves
// the download; a path or another URL is opened to tell it.
func (k *kit) tells(at, version string) bool {
	u, err := url.Parse(at)
	if err != nil || !IsURL(at) {
		return false
	}
	m := k.version.FindStringSubmatch(path.Base(u.Path))
	return m != nil && version != "" && m[1] == version
}

// fetch writes the icons of the package at from to dir.
func (k *kit) fetch(ctx context.Context, from Source, dir string) (*index, error) {
	if IsURL(from.At) {
		b, err := download(ctx, from.At, "icons_download")
		if err != nil {
			return nil, err
		}
		return k.extract(bytes.NewReader(b), int64(len(b)), from, dir)
	}
	f, err := os.Open(from.At)
	if err != nil {
		return nil, usererr.CannotRead(from.Name, err)
	}
	defer f.Close() //nolint:errcheck // read only
	info, err := f.Stat()
	if err != nil {
		return nil, usererr.CannotRead(from.Name, err)
	}
	return k.extract(f, info.Size(), from, dir)
}

// IsURL tells a source that Fetch downloads from one it reads as a path.
func IsURL(source string) bool {
	u, err := url.Parse(source)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http")
}

// replace puts the icons in tmp in place of dir as a whole. When that fails, the previous icons stay.
func replace(tmp, dir string) error {
	old := tmp + "-old"
	hadOld := true
	if err := os.Rename(dir, old); errors.Is(err, fs.ErrNotExist) {
		hadOld = false
	} else if err != nil {
		return usererr.CannotWrite(dir, err)
	}
	if err := rename(tmp, dir); err != nil {
		if hadOld {
			if back := os.Rename(old, dir); back != nil {
				return previousLeft(dir, old, err)
			}
		}
		return usererr.CannotWrite(dir, err)
	}
	// The new icons are in place: a previous one left behind is harmless.
	_ = os.RemoveAll(old) //nolint:errcheck // see above
	return nil
}

// previousLeft is err, of moving the new icons to dir, when the previous ones could not be moved back from old
// either: in the system's words, with where they are.
func previousLeft(dir, old string, err error) error {
	words := err.Error()
	var le *os.LinkError
	if errors.As(err, &le) {
		words = le.Err.Error()
	}
	return usererr.NewAt(dir, 0, 0, "icons_previous_left", words, old)
}

// extract writes the set's icons in the zip r of size bytes, from from, to dir, with their index.json.
func (k *kit) extract(r io.ReaderAt, size int64, from Source, dir string) (*index, error) {
	zr, err := zip.NewReader(r, size)
	if errors.Is(err, zip.ErrFormat) {
		return nil, usererr.NewAt(from.Name, 0, 0, "icons_not_zip", k.page)
	}
	if err != nil {
		return nil, zipMistake(from, err)
	}
	idx := &index{Version: "", Source: from.At, Icons: map[string]entry{}}
	light := k.lightOnes(zr)
	var written int64
	for _, f := range zr.File {
		icon, ok := k.iconOf(f.Name)
		// A service in two categories (only the background differs) keeps the first with a light variant, or
		// the first if it has none.
		if _, kept := idx.Icons[icon.name]; !ok || kept || icon.dark && light[icon.name] {
			continue
		}
		if len(idx.Icons) == maxIcons {
			return nil, usererr.NewAt(from.Name, 0, 0, "icons_too_many", maxIcons)
		}
		b, err := readEntry(f)
		if errors.Is(err, errTooLarge) {
			return nil, usererr.NewAt(from.Name, 0, 0, "icons_icon_too_large", f.Name, maxIcon>>20)
		}
		if err != nil {
			return nil, zipMistake(from, err)
		}
		if written += int64(len(b)); written > maxPackage {
			return nil, usererr.NewAt(from.Name, 0, 0, "icons_written_too_large", maxPackage>>20)
		}
		if err = keep(dir, icon, b); err != nil {
			return nil, err
		}
		idx.Version = cmp.Or(idx.Version, icon.version)
		idx.Icons[icon.name] = entry{Category: icon.category}
	}
	if idx.Version == "" {
		return nil, usererr.NewAt(from.Name, 0, 0, "icons_none_in_zip", k.title, k.page)
	}
	return idx, writeIndex(idx, dir)
}

// lightOnes are the names of the icons of zr with a light variant: the package's order of the variants is not
// fixed, so they are found before any is kept.
func (k *kit) lightOnes(zr *zip.Reader) map[string]bool {
	light := map[string]bool{}
	for _, f := range zr.File {
		if icon, ok := k.iconOf(f.Name); ok && !icon.dark {
			light[icon.name] = true
		}
	}
	return light
}

// zipMistake is err, of reading the zip from past its start: one that cannot be read in the system's words, or
// one archive/zip cannot read (an entry's header, its size or its checksum) in its words.
func zipMistake(from Source, err error) error {
	if errors.As(err, new(*fs.PathError)) {
		return usererr.CannotRead(from.Name, err)
	}
	return libraryAt(from.Name, 0, 0, "icons_bad_zip", err)
}

func readEntry(f *zip.File) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err //nolint:wrapcheck // worded by the caller
	}
	defer r.Close() //nolint:errcheck // read only
	// An entry may inflate far beyond its size in the zip.
	return readAtMost(r, maxIcon)
}

// keep writes the icon b to dir as name.svg.
func keep(dir string, icon packed, b []byte) error {
	name := icon.name + ".svg"
	// A name holds no slash but those its kit puts in (TestIconOfStaysInside); this keeps it so.
	if !filepath.IsLocal(name) {
		return fmt.Errorf("keep the icon %s: not a name inside the set", icon.name) //nolint:err113 // floe's own mistake
	}
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return usererr.CannotWrite(filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return usererr.CannotWrite(path, err)
	}
	return nil
}

func writeIndex(idx *index, dir string) error {
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", " ")
	if err := enc.Encode(idx); err != nil {
		return fmt.Errorf("write the icons' index: %w", err)
	}
	file := filepath.Join(dir, "index.json")
	if err := os.WriteFile(file, out.Bytes(), 0o600); err != nil {
		return usererr.CannotWrite(file, err)
	}
	return nil
}

func slug(s string) string {
	return dashesRe.ReplaceAllString(slugRe.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

// Hit is an icon found by Search.
type Hit struct {
	Name     string // with its set in front
	Category string
}

// Search returns the icons of the fetched sets whose name (set/name) contains every word, by name. With no set
// fetched, it tells each not fetched.
func (s *Store) Search(words []string) ([]Hit, error) {
	terms := make([]string, len(words))
	for i, w := range words {
		terms[i] = strings.ToLower(w)
	}
	var missing usererr.List
	var hits []Hit
	for _, k := range kits {
		idx, err := s.readIndex(k)
		if err != nil {
			return nil, err
		}
		if idx == nil {
			missing = append(missing, s.notFetched(k))
			continue
		}
		for name, e := range idx.Icons {
			// Matched as printed, set and all, so that a name copied from a spec finds itself.
			full := k.name + "/" + name
			if !slices.ContainsFunc(terms, func(t string) bool { return !strings.Contains(full, t) }) {
				hits = append(hits, Hit{Name: full, Category: e.Category})
			}
		}
	}
	if len(missing) == len(kits) {
		return nil, missing.Err()
	}
	if len(hits) == 0 {
		return nil, usererr.New("icons_no_match", `"`+strings.Join(terms, `" "`)+`"`)
	}
	slices.SortFunc(hits, func(a, b Hit) int { return strings.Compare(a.Name, b.Name) })
	return hits, nil
}

// Lookup returns the SVG of the icon of name, its set in front (aws/amazon-ec2). A name without a known set in
// front is ErrNoSet, and one its set does not hold ErrNoIcon, for the caller to tell where it was written.
func (s *Store) Lookup(name string) ([]byte, error) {
	set, rest, _ := strings.Cut(name, "/")
	k := kitOf(set)
	if k == nil {
		return nil, ErrNoSet
	}
	idx, err := s.loadIndex(k)
	if err != nil {
		return nil, err
	}
	// Only a name in the index is read: it is a file the fetch wrote, inside the set's directory.
	if _, ok := idx.Icons[rest]; !ok {
		return nil, ErrNoIcon
	}
	file := filepath.Join(s.dir(k), rest+".svg")
	b, err := os.ReadFile(file) //nolint:gosec // G304: a name in the set's index
	if err != nil {
		return nil, usererr.CannotRead(file, err)
	}
	return b, nil
}
