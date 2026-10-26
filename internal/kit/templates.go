package kit

import (
	"bytes"
	"html/template"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/sukekyo26/floe/internal/rules"
	"github.com/sukekyo26/floe/internal/schema"
	"github.com/sukekyo26/floe/internal/usererr"
)

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
	for _, name := range dirs {
		t := &Theme{
			name: name, dir: path.Join(d.Name, name), base: nil, palette: nil, partsCSS: map[string][]byte{}, unread: nil,
		}
		all, err := entries(d, name, false)
		if err != nil {
			return err
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
				return failed
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
			return err
		}
		k.themes[name] = t
	}
	return nil
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
}

// Name returns the name of t.
func (t *Template) Name() string { return t.name }

// Schema returns the schema of t compiled, nil without one.
func (t *Template) Schema() *schema.Compiled { return t.schema }

// Page returns the page.html of t, parsed, in the language tag of a page; nil without one.
func (t *Template) Page(lang string) *template.Template { return t.pages.page(t.words, lang) }

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

// templatesURL is where floe holds the schemas of the templates: floe:///templates/<name>/schema.json.
const templatesURL = "floe:///templates/"

// readTemplates reads into k the templates of d, each a directory, their schemas compiled with the resources
// of k when partsRead: one would be told missing when a part could not be read.
func (k *Kit) readTemplates(d Dir, partsRead bool) error {
	dirs, err := entries(d, ".", true)
	if err != nil {
		return err
	}
	var found usererr.List
	for _, name := range dirs {
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
		pageSrc: nil, example: nil, hasSchema: false, unread: nil,
	}
	var found usererr.List
	var err error
	t.words, err = readWordsIn(d, name)
	if failed := found.Add(err); failed != nil {
		return failed
	}
	t.pages, t.pageSrc, err = readPages(d, name, "page.html", pageFuncs(), t.words)
	if failed := found.Add(err); failed != nil {
		return failed
	}
	if t.css, _, err = readCSS(d, name, "page.css"); err != nil {
		return err
	}
	if t.example, _, err = readFile(d, name, "example.yaml"); err != nil {
		return err
	}
	t.checks, err = readChecks(d, name)
	if failed := found.Add(err); failed != nil {
		return failed
	}
	if partsRead {
		if failed := found.Add(k.compileTemplate(d, t)); failed != nil {
			return failed
		}
	}
	if found != nil {
		return found.Sorted().Err()
	}
	read := []string{"page.html", "page.css", "schema.json", "checks.yaml", "words.yaml", "example.yaml"}
	// computed.yaml is read with the schema alone, whose items its expressions see.
	if t.schema != nil {
		read = append(read, "computed.yaml")
	}
	if t.unread, err = unreadIn(d, name, read...); err != nil {
		return err
	}
	k.templates[name] = t
	return nil
}

// compileTemplate compiles the schema.json of t, in d, with the resources of k, and reads its computed.yaml,
// whose expressions see the items the schema declares at its top; none without a schema.json.
func (k *Kit) compileTemplate(d Dir, t *Template) error {
	src, ok, err := readFile(d, t.name, "schema.json")
	if !ok || err != nil {
		return err
	}
	t.hasSchema = true
	c, err := schema.Compile(templatesURL+t.name+"/schema.json", path.Join(t.dir, "schema.json"), src, k.Resources())
	if err != nil {
		return err //nolint:wrapcheck // its mistakes, or floe's failure
	}
	t.schema = c
	src, ok, err = readFile(d, t.name, "computed.yaml")
	if !ok || err != nil {
		return err
	}
	names := slices.Sorted(maps.Keys(c.Follow(c.Root).Props))
	t.computed, err = rules.ReadComputed(path.Join(t.dir, "computed.yaml"), src, names)
	return err //nolint:wrapcheck // its mistakes, or floe's failure
}
