// Package icons keeps icon sets on the user's machine and finds their icons by name. floe does not ship
// them (that would redistribute them): each user fetches a set's official package once. An icon is
// named by its set and its name in the set, like aws/amazon-ec2.
package icons

import (
	"archive/zip"
	"bytes"
	"cmp"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"unicode"

	"github.com/sukekyo26/floe/internal/usererr"
)

// kit is what differs between icon sets: where their package is and which of its files are icons.
type kit struct {
	name       string // the head of the set's icon names, and its directory under the root
	title      string // the set as messages name it
	page       string // the page that links the package
	host, size string // where the package downloads from and about how large it is
	link       *regexp.Regexp
	version    *regexp.Regexp // the package's version in its URL or file name
	common     []string       // words of names too common to suggest a name by
	iconOf     func(path string) (packed, bool)
}

// packed is an icon to keep from a package: dark marks its variant for dark backgrounds.
type packed struct {
	version, name, category string
	dark                    bool
}

var kits = []*kit{&aws}

// ErrNoSet is a name without a known set in front (aws/…).
var ErrNoSet = errors.New("no icon set in front")

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

// Store is the icon sets kept under a root (FLOE_ICONS_DIR), a directory each.
type Store struct {
	root string
	// The indexes of the sets read so far, by set: a diagram looks up many icons.
	mu      sync.Mutex
	indexes map[string]*index
	icons   map[string]*Icon // by name, as Lookup found them
}

// New returns the store under root. An empty root (no HOME to put it under) is refused when used.
func New(root string) *Store {
	return &Store{root: root, mu: sync.Mutex{}, indexes: map[string]*index{}, icons: map[string]*Icon{}}
}

// Indexes are the index files of every set, fetched or not: a fetch replaces the one of its set.
func (s *Store) Indexes() []string {
	if s.root == "" {
		return nil
	}
	files := make([]string, len(kits))
	for i, k := range kits {
		files[i] = filepath.Join(s.dir(k), "index.json")
	}
	return files
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
	Dark     bool   `json:"dark,omitempty"` // whether name.dark.svg holds a variant for dark backgrounds
}

// readIndex returns nil when the set has not been fetched.
func (s *Store) readIndex(k *kit) (*index, error) {
	if s.root == "" {
		return nil, usererr.New("icons_no_dir")
	}
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
	var idx index
	if err == nil {
		err = json.Unmarshal(b, &idx)
	}
	if err != nil {
		return nil, usererr.New("icons_bad_index", file, err.Error(), k.name)
	}
	s.indexes[k.name] = &idx
	return &idx, nil
}

func (s *Store) loadIndex(k *kit) (*index, error) {
	idx, err := s.readIndex(k)
	if err != nil {
		return nil, err
	}
	if idx == nil {
		return nil, usererr.New("icons_not_fetched", k.title, s.dir(k), k.name, k.size, k.host)
	}
	return idx, nil
}

// Fetched is what Fetch did: kept Count icons of Version in Dir, or found Version already there.
type Fetched struct {
	Title   string // the set as messages name it
	Dir     string
	Version string
	Count   int
	Already bool
}

// Fetch keeps the icons of set (one of Sets) from its package zip at source (a URL or a path; when empty, the one
// its page links) in its directory, replacing what is there as a whole. A version already there is
// kept unless force.
func (s *Store) Fetch(ctx context.Context, set, source string, force bool) (Fetched, error) {
	if err := CheckSet(set); err != nil {
		return Fetched{}, err //nolint:exhaustruct // nothing fetched
	}
	k := kitOf(set)
	got := Fetched{Title: k.title, Dir: s.dir(k), Version: "", Count: 0, Already: false}
	if s.root == "" {
		return got, usererr.New("icons_no_dir")
	}
	from, err := k.packageURL(ctx, source)
	if err != nil {
		return got, err
	}
	// A broken index is replaced, so it counts as none.
	if current, _ := s.readIndex(k); current != nil { //nolint:errcheck // see above
		got.Version = current.Version
	}
	if m := k.version.FindStringSubmatch(from); !force && m != nil && m[1] == got.Version {
		got.Already = true
		return got, nil
	}
	// Before the download: a directory that cannot be written to is no reason to fetch the set.
	if err = canWrite(s.root, k.name); err != nil {
		return got, usererr.CannotWrite(got.Dir, err)
	}
	buf, err := read(ctx, from)
	if err != nil {
		return got, err
	}
	tmp, err := os.MkdirTemp(s.root, k.name+".tmp-")
	if err != nil {
		return got, usererr.CannotWrite(got.Dir, err)
	}
	defer os.RemoveAll(tmp) //nolint:errcheck // a leftover is harmless
	idx, err := k.extract(buf, from, tmp)
	if err != nil {
		return got, err
	}
	if !force && idx.Version == got.Version {
		got.Already = true
		return got, nil
	}
	if err = replace(tmp, got.Dir); err != nil {
		return got, err
	}
	s.mu.Lock()
	s.indexes[k.name] = idx
	clear(s.icons)
	s.mu.Unlock()
	got.Version, got.Count = idx.Version, len(idx.Icons)
	return got, nil
}

// packageURL is source, or when it is empty the package that the set's page links.
func (k *kit) packageURL(ctx context.Context, source string) (string, error) {
	if source != "" {
		return source, nil
	}
	page, err := download(ctx, k.page)
	if err != nil {
		return "", err
	}
	if from := k.link.FindString(string(page)); from != "" {
		return from, nil
	}
	return "", usererr.New("icons_no_package_link", k.page, k.name)
}

// IsURL tells a source that Fetch downloads from one it reads as a path.
func IsURL(source string) bool {
	return strings.HasPrefix(source, "https://") || strings.HasPrefix(source, "http://")
}

// canWrite makes root, and a directory in it it removes at once, to know before a download that the set
// can be put there.
func canWrite(root, name string) error {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err //nolint:wrapcheck // told by the caller
	}
	tmp, err := os.MkdirTemp(root, name+".tmp-")
	if err != nil {
		return err //nolint:wrapcheck // as above
	}
	return os.Remove(tmp) //nolint:wrapcheck // as above
}

// replace puts the icons in tmp in place of dir as a whole. When that fails, the previous icons stay.
func replace(tmp, dir string) error {
	old := tmp + "-old"
	hadOld := true
	if err := os.Rename(dir, old); errors.Is(err, fs.ErrNotExist) {
		hadOld = false
	} else if err != nil {
		return fmt.Errorf("move the previous icons aside: %w", err)
	}
	if err := rename(tmp, dir); err != nil {
		if hadOld {
			if back := os.Rename(old, dir); back != nil {
				return fmt.Errorf("move the new icons into place: %w (and the previous ones back: %w)", err, back)
			}
		}
		return fmt.Errorf("move the new icons into place: %w", err)
	}
	// The new icons are in place: a previous one left behind is harmless.
	_ = os.RemoveAll(old) //nolint:errcheck // see above
	return nil
}

// extract writes the set's icons in the zip in buf to dir, with their index.json.
func (k *kit) extract(buf []byte, from, dir string) (*index, error) {
	zr, err := zip.NewReader(bytes.NewReader(buf), int64(len(buf)))
	if err != nil {
		return nil, usererr.New("icons_not_zip", from, k.page)
	}
	idx := &index{Version: "", Source: from, Icons: map[string]entry{}}
	light := map[string]bool{}
	// What is written is capped like the package: many entries each under maxIcon could fill the disk.
	var written int64
	for _, f := range zr.File {
		icon, ok := k.iconOf(f.Name)
		// A service in two categories (only the background differs) keeps the first.
		if !ok || (icon.dark && idx.Icons[icon.name].Dark) || (!icon.dark && light[icon.name]) {
			continue
		}
		var b []byte
		if b, err = readEntry(f); err != nil {
			return nil, usererr.New("icons_bad_zip", f.Name, from, err.Error())
		}
		if written += int64(len(b)); written > maxPackage {
			return nil, usererr.New("icons_too_large", from, maxPackage>>20)
		}
		if err = keep(dir, icon, b); err != nil {
			return nil, err
		}
		idx.add(icon)
		light[icon.name] = light[icon.name] || !icon.dark
	}
	if idx.Version == "" {
		return nil, usererr.New("icons_none_in_zip", k.title, from, k.page)
	}
	if err = darkAlone(idx, light, dir); err != nil {
		return nil, err
	}
	return idx, writeIndex(idx, dir)
}

// add records icon, kept from the package, in the index.
func (idx *index) add(icon packed) {
	idx.Version = cmp.Or(idx.Version, icon.version)
	e, seen := idx.Icons[icon.name]
	if !seen {
		e.Category = icon.category
	}
	e.Dark = e.Dark || icon.dark
	idx.Icons[icon.name] = e
}

// darkAlone makes the dark variant of an icon without a light one its usual one.
func darkAlone(idx *index, light map[string]bool, dir string) error {
	for name, e := range idx.Icons {
		if light[name] {
			continue
		}
		if err := os.Rename(filepath.Join(dir, name+".dark.svg"), filepath.Join(dir, name+".svg")); err != nil {
			return fmt.Errorf("keep the icon %s: %w", name, err)
		}
		e.Dark = false
		idx.Icons[name] = e
	}
	return nil
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

// keep writes the icon b to dir as name.svg, or name.dark.svg for a dark variant.
func keep(dir string, icon packed, b []byte) error {
	path := filepath.Join(dir, icon.name+".svg")
	if icon.dark {
		path = filepath.Join(dir, icon.name+".dark.svg")
	}
	// G703: a name holds no slash but those its kit puts in (TestIconOfStaysInside).
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil { //nolint:gosec // see above
		return fmt.Errorf("keep the icon %s: %w", icon.name, err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil { //nolint:gosec // see above
		return fmt.Errorf("keep the icon %s: %w", icon.name, err)
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
	if err := os.WriteFile(filepath.Join(dir, "index.json"), out.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write the icons' index: %w", err)
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

// Search returns the icons of the fetched sets whose name (set/name) contains every word, by name.
func (s *Store) Search(words []string) ([]Hit, error) {
	terms := make([]string, len(words))
	for i, w := range words {
		terms[i] = strings.ToLower(w)
	}
	fetched := false
	var hits []Hit
	for _, k := range kits {
		idx, err := s.readIndex(k)
		if err != nil {
			return nil, err
		}
		if idx == nil {
			continue
		}
		fetched = true
		for name, e := range idx.Icons {
			// Matched as printed, set and all, so that a name copied from a spec finds itself.
			full := k.name + "/" + name
			if !slices.ContainsFunc(terms, func(t string) bool { return !strings.Contains(full, t) }) {
				hits = append(hits, Hit{Name: full, Category: e.Category})
			}
		}
	}
	if !fetched {
		return nil, usererr.New("icons_none_fetched", s.root, strings.Join(Sets(), ", "))
	}
	if len(hits) == 0 {
		return nil, usererr.New("icons_no_match", `"`+strings.Join(terms, `" "`)+`"`)
	}
	slices.SortFunc(hits, func(a, b Hit) int { return strings.Compare(a.Name, b.Name) })
	return hits, nil
}

// Icon is an icon as data URIs. Dark is empty when the icon has no variant for dark backgrounds.
type Icon struct {
	Light, Dark string
}

// Lookup returns the icon of name, its set in front (aws/amazon-ec2). For a name not in its set it
// returns nil and up to five close names, the ones sharing the most words first. A name without a
// known set in front is ErrNoSet.
func (s *Store) Lookup(name string) (*Icon, []string, error) {
	set, rest, _ := strings.Cut(name, "/")
	k := kitOf(set)
	if k == nil {
		return nil, nil, ErrNoSet
	}
	idx, err := s.loadIndex(k)
	if err != nil {
		return nil, nil, err
	}
	e, ok := idx.Icons[rest]
	if !ok {
		return nil, k.closeNames(idx, rest), nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if icon := s.icons[name]; icon != nil {
		return icon, nil, nil
	}
	dir := s.dir(k)
	uri := func(file string) (string, error) {
		b, rerr := os.ReadFile(filepath.Join(dir, file)) //nolint:gosec // G304: a name in the set's index
		if rerr != nil {
			return "", usererr.New("icons_bad_index", filepath.Join(dir, "index.json"), rerr.Error(), k.name)
		}
		return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(b), nil
	}
	icon := &Icon{Light: "", Dark: ""}
	if icon.Light, err = uri(rest + ".svg"); err != nil {
		return nil, nil, err
	}
	if e.Dark {
		if icon.Dark, err = uri(rest + ".dark.svg"); err != nil {
			return nil, nil, err
		}
	}
	s.icons[name] = icon
	return icon, nil, nil
}

// closeNames ranks the names by how many words of name they hold (near), words as slug splits them.
// LIMIT: a word is matched within 2 edits at most, so an abbreviation (ddb for dynamodb) suggests
// nothing. If agents write those often, keep a table of aliases.
func (k *kit) closeNames(idx *index, name string) []string {
	var words []string
	for w := range strings.FieldsFuncSeq(strings.ToLower(name), isSep) {
		// A common word, or a misspelling of one, would find most names.
		if len(w) > 1 && !slices.ContainsFunc(k.common, func(c string) bool { return match(c, w) > 0 }) {
			words = append(words, w)
		}
	}
	type scored struct {
		name  string
		score int
	}
	var found []scored
	for n := range idx.Icons {
		score := 0
		for _, w := range words {
			score += match(n, w)
		}
		if score > 0 {
			found = append(found, scored{n, score})
		}
	}
	slices.SortFunc(found, func(a, b scored) int {
		return cmp.Or(cmp.Compare(b.score, a.score), cmp.Compare(len(a.name), len(b.name)), strings.Compare(a.name, b.name))
	})
	out := make([]string, 0, 5)
	for _, f := range found[:min(5, len(found))] {
		out = append(out, k.name+"/"+f.name)
	}
	return out
}

func isSep(r rune) bool {
	return r == '/' || r == '-' || r == '_' || unicode.IsSpace(r)
}

// match scores how name holds word: 2 when it holds it, 1 when a word of it is within 1 edit (2 for
// a word of 8 letters or more), so that a misspelt word still finds the name, and 0 otherwise. A
// word of 3 letters or fewer must be in the name.
func match(name, word string) int {
	if strings.Contains(name, word) {
		return 2
	}
	edits := 0
	switch {
	case len(word) >= 8:
		edits = 2
	case len(word) >= 4:
		edits = 1
	default:
		return 0
	}
	for w := range strings.FieldsFuncSeq(name, isSep) {
		if distance(w, word) <= edits {
			return 1
		}
	}
	return 0
}

// distance is the Levenshtein distance between a and b, by bytes: icon names are ASCII.
func distance(a, b string) int {
	row := make([]int, len(b)+1)
	for j := range row {
		row[j] = j
	}
	for i := 1; i <= len(a); i++ {
		diag := row[0]
		row[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			diag, row[j] = row[j], min(row[j]+1, row[j-1]+1, diag+cost)
		}
	}
	return row[len(b)]
}
