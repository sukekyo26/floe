// Package kit reads, at stage 1, the parts, looks, themes and templates pages are drawn with: floe's own,
// and over them one's own. Reading only makes their values (the files parsed, the schemas compiled); the
// rules over the values are checked apart. floe's own are read once a process, and never checked there.
package kit

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
	"sync"

	"github.com/sukekyo26/floe/internal/bom"
	"github.com/sukekyo26/floe/internal/css"
	"github.com/sukekyo26/floe/internal/rules"
	"github.com/sukekyo26/floe/internal/schema"
	"github.com/sukekyo26/floe/internal/usererr"
)

// Kit is the parts, looks, themes and templates read at stage 1, over those of a base. It is not changed
// once read: reading more makes a new Kit, so that one Kit may serve every run at once.
type Kit struct {
	base      *Kit
	baseCSS   []byte // floe's base.css, of floe's Kit alone
	parts     map[string]*Part
	looks     map[string]*Look
	themes    map[string]*Theme
	templates map[string]*Template
	resources map[string]schema.Resource // by URL, those of base too
}

// Dir is a directory read: its files, and the path its mistakes name them under.
type Dir struct {
	FS   fs.FS
	Name string
}

// Sources are the directories of parts (and the schemas they share), themes and templates, each with a nil
// FS for none.
type Sources struct {
	Parts, Themes, Templates Dir
}

//go:embed floe
var floeFiles embed.FS

// floe reads floe's own once, a process.
var floe = sync.OnceValues(func() (*Kit, error) {
	var src Sources
	for dir, to := range map[string]*Dir{"parts": &src.Parts, "themes": &src.Themes, "templates": &src.Templates} {
		files, err := fs.Sub(floeFiles, path.Join("floe", dir))
		if err != nil {
			return nil, fmt.Errorf("kit: floe's %s: %w", dir, err)
		}
		*to = Dir{FS: files, Name: dir}
	}
	k, err := Read(nil, src)
	if err != nil {
		return nil, err
	}
	base, err := floeFiles.ReadFile("floe/base.css")
	if err != nil {
		return nil, fmt.Errorf("kit: floe's base.css: %w", err)
	}
	k.baseCSS = css.Preprocess(bom.Trim(base))
	return k, nil
})

// Floe returns floe's own, read once a process and not checked.
func Floe() (*Kit, error) {
	return floe()
}

// BaseCSS returns floe's base.css, under every page's own CSS: that of the floe Kit k reads over.
func (k *Kit) BaseCSS() []byte {
	for k.base != nil {
		k = k.base
	}
	return bytes.Clone(k.baseCSS)
}

// Read reads src over base (nil for none): parts and looks, then themes and templates, a name or a $ref found
// in either. It does what it takes to have the values, and returns every mistake of it Sorted, and no Kit.
func Read(base *Kit, src Sources) (*Kit, error) {
	k := &Kit{
		base: base, baseCSS: nil, parts: map[string]*Part{}, looks: map[string]*Look{}, themes: map[string]*Theme{},
		templates: map[string]*Template{}, resources: map[string]schema.Resource{},
	}
	if base != nil {
		maps.Copy(k.resources, base.resources)
	}
	var found usererr.List
	if failed := found.Add(k.readParts(src.Parts)); failed != nil {
		return nil, failed
	}
	// A template's schema is compiled with every part's: one that could not be read would be told missing.
	partsRead := found == nil
	if failed := found.Add(k.readThemes(src.Themes)); failed != nil {
		return nil, failed
	}
	if failed := found.Add(k.readTemplates(src.Templates, partsRead)); failed != nil {
		return nil, failed
	}
	if found != nil {
		return nil, found.Sorted().Err()
	}
	return k, nil
}

// Part returns the part of k named so, its own or its base's. Parts and looks share their names: one's own
// look hides its base's part of the name, and its own part the base's look.
func (k *Kit) Part(name string) (*Part, bool) {
	for at := k; at != nil; at = at.base {
		if p, ok := at.parts[name]; ok {
			return p, true
		}
		if _, ok := at.looks[name]; ok {
			return nil, false
		}
	}
	return nil, false
}

// Look returns the look of k named so, its own or its base's, as Part does.
func (k *Kit) Look(name string) (*Look, bool) {
	for at := k; at != nil; at = at.base {
		if l, ok := at.looks[name]; ok {
			return l, true
		}
		if _, ok := at.parts[name]; ok {
			return nil, false
		}
	}
	return nil, false
}

// Theme returns the theme of k named so, its own or its base's.
func (k *Kit) Theme(name string) (*Theme, bool) {
	return lookUp(k, name, func(k *Kit) map[string]*Theme { return k.themes })
}

// Template returns the template of k named so, its own or its base's.
func (k *Kit) Template(name string) (*Template, bool) {
	return lookUp(k, name, func(k *Kit) map[string]*Template { return k.templates })
}

// Resources returns the schemas of the parts and of what they share by URL, its own and its base's, a copy.
func (k *Kit) Resources() map[string]schema.Resource {
	return maps.Clone(k.resources)
}

// Names returns the names of the parts, looks, themes and templates of k and its base, each sorted, a name
// one's own hides as it does (Part).
func (k *Kit) Names() (parts, looks, themes, templates []string) {
	for at := k; at != nil; at = at.base {
		parts = append(parts, slices.Collect(maps.Keys(at.parts))...)
		looks = append(looks, slices.Collect(maps.Keys(at.looks))...)
		themes = append(themes, slices.Collect(maps.Keys(at.themes))...)
		templates = append(templates, slices.Collect(maps.Keys(at.templates))...)
	}
	sorted := func(s []string) []string { return slices.Compact(slices.Sorted(slices.Values(s))) }
	parts = slices.DeleteFunc(sorted(parts), func(n string) bool { _, ok := k.Part(n); return !ok })
	looks = slices.DeleteFunc(sorted(looks), func(n string) bool { _, ok := k.Look(n); return !ok })
	return parts, looks, sorted(themes), sorted(templates)
}

// lookUp returns the one named so of the things of k that of gives, k's own before its base's.
func lookUp[T any](k *Kit, name string, of func(*Kit) map[string]*T) (*T, bool) {
	for at := k; at != nil; at = at.base {
		if v, ok := of(at)[name]; ok {
			return v, true
		}
	}
	return nil, false
}

// isPartOrLook reports whether name is a part or a look of k or its base.
func (k *Kit) isPartOrLook(name string) bool {
	_, part := k.Part(name)
	_, look := k.Look(name)
	return part || look
}

// entries returns the names of the directories (dirs) or files of the directory dir of d ("." for d
// itself), those starting with a dot left out, sorted; none without an FS.
func entries(d Dir, dir string, dirs bool) ([]string, error) {
	if d.FS == nil {
		return nil, nil
	}
	all, err := fs.ReadDir(d.FS, dir)
	if err != nil {
		return nil, fmt.Errorf("kit: read %s: %w", path.Join(d.Name, dir), err)
	}
	var names []string
	for _, e := range all {
		if e.IsDir() == dirs && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// readFile returns the file name of the directory dir of d, without a byte order mark at its start, and
// false when there is none.
// LIMIT: a file floe cannot read is floe's failure; the reading of one's own directories is to tell it as
// the reader's.
func readFile(d Dir, dir, name string) ([]byte, bool, error) {
	src, err := fs.ReadFile(d.FS, path.Join(dir, name))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, false, nil
	case err != nil:
		return nil, false, fmt.Errorf("kit: read %s: %w", path.Join(d.Name, dir, name), err)
	}
	return bom.Trim(src), true, nil
}

// readCSS returns the CSS file name of the directory dir of d as readFile does, its line breaks made LF
// (css.Preprocess): what is checked is what goes in the page.
func readCSS(d Dir, dir, name string) ([]byte, bool, error) {
	src, ok, err := readFile(d, dir, name)
	if !ok || err != nil {
		return nil, ok, err
	}
	return css.Preprocess(src), true, nil
}

// pageFuncs are the functions of a template's page.html.
func pageFuncs() template.FuncMap {
	return maps.Clone(funcs)
}

// partFuncsAll are the functions of a part's part.html.
func partFuncsAll() template.FuncMap {
	fns := maps.Clone(funcs)
	maps.Copy(fns, partFuncs)
	return fns
}

// readChecks reads the checks.yaml of the directory dir of d, nil without one.
func readChecks(d Dir, dir string) (*rules.Checks, error) {
	src, ok, err := readFile(d, dir, "checks.yaml")
	if !ok || err != nil {
		return nil, err
	}
	return rules.ReadChecks(path.Join(d.Name, dir, "checks.yaml"), src) //nolint:wrapcheck // its mistakes
}

// readWordsIn reads the words.yaml of the directory dir of d, none without one.
func readWordsIn(d Dir, dir string) (words, error) {
	src, ok, err := readFile(d, dir, "words.yaml")
	if !ok || err != nil {
		return words{}, err
	}
	return readWords(path.Join(d.Name, dir, "words.yaml"), src)
}
