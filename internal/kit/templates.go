package kit

import (
	"bytes"
	"errors"
	"html/template"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/sukekyo26/floe/internal/rules"
	"github.com/sukekyo26/floe/internal/schema"
	"github.com/sukekyo26/floe/internal/usererr"
)

// DefaultTheme is the theme of a page whose data names none, whose palette defines every variable, under any
// other theme's.
const DefaultTheme = "default"

// Theme is a theme read: its CSS by file, without the .css.
type Theme struct {
	name     string
	dir      string
	base     []byte            // base.css, nil for none
	palette  []byte            // palette.css, nil for none
	partsCSS map[string][]byte // <part or look>.css by its name
	unread   []string          // the files of its directory it does not read: of no part or look of the Kit
}

// Name returns the name of t.
func (t *Theme) Name() string { return t.name }

// Base returns the base.css of t, nil for none.
func (t *Theme) Base() []byte { return bytes.Clone(t.base) }

// Palette returns the palette.css of t, nil for none.
func (t *Theme) Palette() []byte { return bytes.Clone(t.palette) }

// CSS returns the CSS t gives the part or look named so, nil for none.
func (t *Theme) CSS(name string) []byte { return bytes.Clone(t.partsCSS[name]) }

// Unread returns the files of the directory of t it does not read.
func (t *Theme) Unread() []string { return slices.Clone(t.unread) }

// readThemes reads into k the themes of d, each a directory: base.css, palette.css, and a file of CSS for a
// part or a look of k, named so. A file of another name is not read.
func (k *Kit) readThemes(d Dir) error {
	dirs, err := entries(d, ".", true)
	if err != nil {
		return err
	}
	var found usererr.List
	for _, name := range dirs {
		if e := k.refusedName(d.Name, KindTheme, name, false); e != nil {
			found = append(found, e)
			continue
		}
		t, err := k.readTheme(d, name)
		if err != nil {
			return err
		}
		k.themes[name] = t
	}
	return found.Err()
}

// readTheme reads the theme of d named name, its directory.
func (k *Kit) readTheme(d Dir, name string) (*Theme, error) {
	t := &Theme{
		name: name, dir: path.Join(d.Name, name), base: nil, palette: nil, partsCSS: map[string][]byte{}, unread: nil,
	}
	all, err := entries(d, name, false)
	if err != nil {
		return nil, err
	}
	var read []string
	for _, file := range all {
		stem, ok := strings.CutSuffix(file, ".css")
		if !ok || stem != "base" && stem != "palette" && !k.isPartOrLook(stem) {
			continue
		}
		read = append(read, file)
		src, _, failed := readCSS(d, name, file)
		if failed != nil {
			return nil, failed
		}
		switch stem {
		case "base":
			t.base = src
		case "palette":
			t.palette = src
		default:
			t.partsCSS[stem] = src
		}
	}
	if t.unread, err = unreadIn(d, name, read...); err != nil {
		return nil, err
	}
	return t, nil
}

// Template is a template read: page.html and the schema of its data, and what goes with them.
type Template struct {
	name      string
	dir       string
	schema    *schema.Compiled // nil without a schema.json
	css       []byte
	checks    *rules.Checks   // nil for none
	computed  *rules.Computed // nil for none
	words     words
	pages     pages // nil without a page.html
	pageSrc   []byte
	example   []byte
	hasSchema bool
	unread    []string
	assets    map[string]func() (template.URL, error) // by the path under assets/, read once when named
	files     []Source                                // schema.json, checks.yaml and example.yaml as read
}

// Name returns the name of t.
func (t *Template) Name() string { return t.name }

// Schema returns the schema of t compiled, nil without one.
func (t *Template) Schema() *schema.Compiled { return t.schema }

// Page returns a copy of the page.html of t, parsed, in the language tag of a page (as i18n.Tag reads it), to
// draw: t's is never drawn, as drawing changes a template (its escaping), which then cannot be copied.
func (t *Template) Page(lang string) (*template.Template, error) {
	return t.pages.copyOf(t.words, lang)
}

// CSS returns the page.css of t, nil for none.
func (t *Template) CSS() []byte { return bytes.Clone(t.css) }

// Checks returns the checks of t, nil for none.
func (t *Template) Checks() *rules.Checks { return t.checks }

// Computed returns the computed values of t, nil for none.
func (t *Template) Computed() *rules.Computed { return t.computed }

// Example returns the example.yaml of t, nil for none.
func (t *Template) Example() []byte { return bytes.Clone(t.example) }

// Unread returns the files of the directory of t it does not read.
func (t *Template) Unread() []string { return slices.Clone(t.unread) }

// File returns the file name of t as its mistakes name it.
func (t *Template) File(name string) string { return path.Join(t.dir, name) }

// PageSource returns the page.html of t as read, nil without one: where a mistake of drawing it is.
func (t *Template) PageSource() []byte { return bytes.Clone(t.pageSrc) }

// templatesURL is where floe holds the schemas of the templates: floe://templates/<name>/schema.json. Its
// host keeps a template's relative $id and $ref under it, off floe:///parts/.
const templatesURL = "floe://templates/"

// readTemplates reads into k the templates of d, each a directory, their schemas compiled with the resources
// of k when partsRead: one would be told missing when a part could not be read.
func (k *Kit) readTemplates(d Dir, partsRead bool) error {
	dirs, err := entries(d, ".", true)
	if err != nil {
		return err
	}
	var found usererr.List
	for _, name := range dirs {
		if e := k.refusedName(d.Name, KindTemplate, name, false); e != nil {
			found = append(found, e)
			continue
		}
		if failed := found.Add(k.readTemplate(d, name, partsRead)); failed != nil {
			return failed
		}
	}
	return found.Sorted().Err()
}

// readTemplate reads into k the template of d named name, its directory, its schema compiled when partsRead.
func (k *Kit) readTemplate(d Dir, name string, partsRead bool) error {
	t := &Template{
		name: name, dir: path.Join(d.Name, name), schema: nil, css: nil, checks: nil, computed: nil, words: nil, pages: nil,
		pageSrc: nil, example: nil, hasSchema: false, unread: nil, assets: nil, files: nil,
	}
	var found usererr.List
	var err error
	t.words, err = readWordsIn(d, name)
	if failed := found.Add(err); failed != nil {
		return failed
	}
	t.assets, err = readAssets(d, name)
	if failed := found.Add(err); failed != nil {
		return failed
	}
	fns := pageFuncs()
	fns["asset"] = t.asset
	t.pages, t.pageSrc, err = readPages(d, name, "page.html", fns, t.words)
	if failed := found.Add(err); failed != nil {
		return failed
	}
	if t.css, _, err = readCSS(d, name, "page.css"); err != nil {
		return err
	}
	if t.example, _, err = readFile(d, name, "example.yaml"); err != nil {
		return err
	}
	var checksSrc, schemaSrc []byte
	t.checks, checksSrc, err = readChecks(d, name)
	if failed := found.Add(err); failed != nil {
		return failed
	}
	if partsRead {
		schemaSrc, err = k.compileTemplate(d, t)
		if failed := found.Add(err); failed != nil {
			return failed
		}
	}
	t.files = sources(
		Source{"schema.json", schemaSrc}, Source{"checks.yaml", checksSrc}, Source{"example.yaml", t.example},
	)
	if found != nil {
		return found.Sorted().Err()
	}
	read := []string{
		"page.html", "page.css", "schema.json", "computed.yaml", "checks.yaml", "words.yaml", "example.yaml", "assets/",
	}
	if t.unread, err = unreadIn(d, name, read...); err != nil {
		return err
	}
	k.templates[name] = t
	return nil
}

// compileTemplate compiles the schema.json of t, in d, with the resources of k, and reads its computed.yaml,
// whose expressions see the items the schema declares at its top; none without a schema.json. It returns the
// text of schema.json.
func (k *Kit) compileTemplate(d Dir, t *Template) ([]byte, error) {
	src, ok, err := readFile(d, t.name, "schema.json")
	if !ok || err != nil {
		return nil, err
	}
	t.hasSchema = true
	c, err := schema.Compile(templatesURL+t.name+"/schema.json", path.Join(t.dir, "schema.json"), src, k.Resources())
	if err != nil {
		return src, err //nolint:wrapcheck // its mistakes, or floe's failure
	}
	t.schema = c
	computed, ok, err := readFile(d, t.name, "computed.yaml")
	if !ok || err != nil {
		return src, err
	}
	names := slices.Sorted(maps.Keys(c.Follow(c.Root).Props))
	t.computed, err = rules.ReadComputed(path.Join(t.dir, "computed.yaml"), computed, names)
	return src, err //nolint:wrapcheck // its mistakes, or floe's failure
}

// asset returns the image of the assets of t at the path name under assets/, as a data URI, read the first
// time it is named.
// LIMIT: a name from the data is told at page.html (asset gets text alone); place it at the data if such get common.
func (t *Template) asset(name string) (template.URL, error) {
	read, ok := t.assets[name]
	if !ok {
		return "", usererr.New("kit_asset_unknown", name)
	}
	return read()
}

// readAssets checks the files under the assets/ of the template of d named name, every directory down, as
// images floe takes (imageStat), without reading them; and returns a read of each, once, by its path under
// assets/. None without assets/.
func readAssets(d Dir, name string) (map[string]func() (template.URL, error), error) {
	there, err := has(d, name, "assets")
	if !there || err != nil {
		return nil, err
	}
	a := assetsOf{
		files: sub{FS: d.FS, dir: path.Join(name, "assets")}, dir: path.Join(d.Name, name, "assets"),
		reads: map[string]func() (template.URL, error){}, found: nil,
	}
	if err := a.walk("."); err != nil {
		return nil, err
	}
	return a.reads, a.found.Sorted().Err()
}

// assetsOf is the assets of a template being read: their files, the directory their mistakes name, a read of
// each file checked, and the mistakes found.
type assetsOf struct {
	files fs.FS
	dir   string
	reads map[string]func() (template.URL, error)
	found usererr.List
}

// walk checks the files of the directory at under the assets, then those of its directories.
func (a *assetsOf) walk(at string) error {
	d := Dir{FS: a.files, Name: a.dir}
	names, err := entries(d, at, false)
	if failed := a.found.Add(err); failed != nil {
		return failed
	}
	for _, file := range names {
		if failed := a.found.Add(a.check(path.Join(at, file))); failed != nil {
			return failed
		}
	}
	subs, err := entries(d, at, true)
	if failed := a.found.Add(err); failed != nil {
		return failed
	}
	for _, s := range subs {
		if err := a.walk(path.Join(at, s)); err != nil {
			return err
		}
	}
	return nil
}

// check checks the file p of the assets as an image floe takes, its mistake at it, and keeps a read of it.
func (a *assetsOf) check(p string) error {
	if _, err := imageStat(a.files, p, a.dir, "assets/"); err != nil {
		var e *usererr.Error
		if errors.As(err, &e) && e.File() == "" {
			return e.PlacedAt(path.Join(a.dir, p), 0, 0)
		}
		return err
	}
	a.reads[p] = sync.OnceValues(func() (template.URL, error) { return ReadImage(a.files, p, a.dir, "assets/") })
	return nil
}

// sub is the directory dir of an FS as an FS of its own, its Stat the FS's: fs.Sub's opens a file to stat it,
// which blocks on a FIFO and fails on a file that cannot be read.
type sub struct {
	fs.FS
	dir string
}

func (s sub) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	return s.FS.Open(path.Join(s.dir, name)) //nolint:wrapcheck // the FS's error, of the file named
}

func (s sub) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrInvalid}
	}
	return fs.Stat(s.FS, path.Join(s.dir, name)) //nolint:wrapcheck // as Open
}

func (s sub) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrInvalid}
	}
	return fs.ReadDir(s.FS, path.Join(s.dir, name)) //nolint:wrapcheck // as Open
}
