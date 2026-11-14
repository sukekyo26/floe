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
	"sync"
	"sync/atomic"

	"floe/internal/bom"
	"floe/internal/css"
	"floe/internal/files"
	"floe/internal/rules"
	"floe/internal/schema"
	"floe/internal/usererr"
)

// Kit is the parts, looks, themes and templates read at stage 1, over those of a base. It is not changed
// once read: reading more makes a new Kit, so that one Kit may serve every run at once.
type Kit struct {
	base        *Kit
	baseCSS     []byte // floe's base.css, of floe's Kit alone
	defaultsCSS []byte // floe's defaults.css, of floe's Kit alone
	parts       map[string]*Part
	looks       map[string]*Look
	themes      map[string]*Theme
	templates   map[string]*Template
	// rebuilt are the base's templates compiled again with its own parts in any, each when first taken: called by
	// Template alone.
	rebuilt map[string]func() (*Template, error)
	// compiled is a count the tests alone read: it and what OnceValues holds are all written after read.
	compiled  atomic.Int32
	shared    map[string]*shared
	resources map[string]schema.Resource // by URL, those of base too
}

// Dir is a directory read: its files, and the path its mistakes name them under.
type Dir struct {
	FS   fs.FS
	Name string
	read map[string][]byte // the files readFile read, by name, when the Dir is recording
}

// recording returns d writing down the files readFile reads of it, as they are, for floe part show and the
// others: a reader of one part, template or theme reads through its own.
func (d Dir) recording() Dir {
	d.read = map[string][]byte{}
	return d
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
		*to = Dir{FS: files, Name: dir, read: nil}
	}
	k, err := read(nil, src)
	if err != nil {
		return nil, err
	}
	for _, f := range []struct {
		name string
		to   *[]byte
	}{{"base.css", &k.baseCSS}, {"defaults.css", &k.defaultsCSS}} {
		src, err := floeFiles.ReadFile(path.Join("floe", f.name))
		if err != nil {
			return nil, fmt.Errorf("kit: floe's %s: %w", f.name, err)
		}
		*f.to = css.Preprocess(bom.Trim(src))
	}
	return k, nil
})

// Floe returns floe's own, read once a process and not checked: the tests check them, as Load does.
func Floe() (*Kit, error) {
	return floe()
}

// Load reads src over base, floe's own or nil, and checks what it read: the Kit of one's own a page is drawn
// with. A mistake of any of one's own stops it, every mistake told together, and no Kit is returned. Load and
// Floe are the only ways a Kit is made outside this package, so that one not checked is never drawn with.
func Load(base *Kit, src Sources) (*Kit, error) {
	k, err := read(base, src)
	if err != nil {
		return nil, err
	}
	if err := check(k); err != nil {
		return nil, err
	}
	return k, nil
}

// root returns floe's Kit, the one k reads over, k itself when it reads over none.
func (k *Kit) root() *Kit {
	for k.base != nil {
		k = k.base
	}
	return k
}

// BaseCSS returns floe's base.css, under every page's own CSS: that of the floe Kit k reads over.
func (k *Kit) BaseCSS() []byte {
	return bytes.Clone(k.root().baseCSS)
}

// DefaultsCSS returns floe's defaults.css, the defaults of the variables floe sets on the page's root, under every
// palette: that of the floe Kit k reads over, as a Kit read over it holds none.
func (k *Kit) DefaultsCSS() []byte {
	return bytes.Clone(k.root().defaultsCSS)
}

// Tones returns the tones a part may take, tone.json's of floe's, in its order, or floe's mistake when it has none.
func (k *Kit) Tones() ([]string, error) {
	k = k.root()
	if s := k.shared[toneSchema]; s != nil && s.schema != nil {
		if tones := s.schema.Enum(); len(tones) != 0 {
			return tones, nil
		}
	}
	return nil, fmt.Errorf("kit: floe's %s.json lists no tones", toneSchema) //nolint:err113 // floe's own mistake
}

// toneSchema is the shared schema of the tones a part may take.
const toneSchema = "tone"

// read reads src over base (nil for none): parts and looks, then themes and templates, a name or a $ref found
// in either. It does what it takes to have the values, and returns every mistake of it Sorted, and no Kit.
func read(base *Kit, src Sources) (*Kit, error) {
	k := &Kit{
		base: base, baseCSS: nil, defaultsCSS: nil, parts: map[string]*Part{}, looks: map[string]*Look{},
		themes: map[string]*Theme{}, templates: map[string]*Template{}, rebuilt: nil, compiled: atomic.Int32{},
		shared: map[string]*shared{}, resources: map[string]schema.Resource{},
	}
	if base != nil {
		maps.Copy(k.resources, base.resources)
	}
	var found usererr.List
	if failed := found.Add(k.readParts(src.Parts)); failed != nil {
		return nil, failed
	}
	// any takes one's own parts before they are compiled, so that a part of one's own holding any holds them too.
	// None is put in any when one could not be read: readPart may hold its resource before it refuses it.
	if found == nil {
		if err := k.ownInAny(); err != nil {
			return nil, err
		}
		if failed := found.Add(k.compileParts()); failed != nil {
			return nil, failed
		}
	}
	// A template's schema is compiled with every part's: one that could not be compiled would be told missing.
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
	k.rebuild()
	return k, nil
}

// ownsParts reports whether k puts parts of its own in any: over a base, with a part of its own.
func (k *Kit) ownsParts() bool { return k.base != nil && len(k.parts) > 0 }

// ownInAny puts in k's resources a copy of the base's shared schema any naming k's own parts too, so that a section
// of floe's templates takes them. Its looks and shared schemas are not put in. stack is left as it is: a part of
// one's own may hold a form or HTML, which its sections would break on (stack's $comment).
// LIMIT: every part of one's own goes in, one made to be held by another alone too; mark the ones to place if that
// misleads.
// LIMIT: a part of one's own holding any holds tabs, columns, html and parts of one's own with a form too, as stack
// refuses; give such a part a list of its own if a page breaks so.
func (k *Kit) ownInAny() error {
	if !k.ownsParts() {
		return nil
	}
	u := partURL("any")
	r, ok := k.resources[u]
	if !ok {
		return errors.New("kit: the base holds no shared schema any") //nolint:err113 // floe's failure
	}
	copied, err := schema.AnyWith(r, slices.Sorted(maps.Keys(k.parts)))
	if err != nil {
		return err //nolint:wrapcheck // floe's failure, which says what failed
	}
	k.resources[u] = copied
	return nil
}

// rebuild puts in k.rebuilt, for each of the base's templates with a schema but those k has its own of, a compile of
// it again with k's resources, its any naming k's own parts, run once when first taken; what else it holds is shared
// with the base's. It runs when every own thing was read without a mistake, so that a mistake left is floe's. The map
// is not written after read, nor is the compile's own (its holes, a copy), so that the goroutines of a server may take
// from one Kit at once.
func (k *Kit) rebuild() {
	if !k.ownsParts() {
		return
	}
	k.rebuilt = map[string]func() (*Template, error){}
	for at := k.base; at != nil; at = at.base {
		for _, name := range slices.Sorted(maps.Keys(at.templates)) {
			t := at.templates[name]
			_, own := k.templates[name]
			_, done := k.rebuilt[name]
			if t.schemaSrc == nil || own || done {
				continue
			}
			k.rebuilt[name] = sync.OnceValues(func() (*Template, error) {
				k.compiled.Add(1)
				c, err := k.compileSchema(t, t.schemaSrc)
				if err != nil {
					return nil, floeFailure(name, err)
				}
				again := *t
				again.schema = c
				return &again, nil
			})
		}
	}
}

// CompileAgain takes every template rebuild put in, so that floe part check finds floe's failure to compile one
// again with one's own parts; it returns the first.
func (k *Kit) CompileAgain() error {
	for _, name := range slices.Sorted(maps.Keys(k.rebuilt)) {
		if _, _, err := k.Template(name); err != nil {
			return err
		}
	}
	return nil
}

// floeFailure is floe's failure to compile its template name again with one's own parts: err's mistakes, at floe's
// files, worded as text, as the user can mend none of them.
func floeFailure(name string, err error) error {
	//nolint:err113 // floe's failure
	return fmt.Errorf("kit: floe's template %s does not compile with your own parts: %s", name, err.Error())
}

// Part returns the part of k named so, its own or its base's. Parts and looks share their names, and read
// refuses one's own of another kind than the base's of its name.
func (k *Kit) Part(name string) (*Part, bool) {
	return lookUp(k, name, func(k *Kit) map[string]*Part { return k.parts })
}

// Look returns the look of k named so, its own or its base's.
func (k *Kit) Look(name string) (*Look, bool) {
	return lookUp(k, name, func(k *Kit) map[string]*Look { return k.looks })
}

// Theme returns the theme of k named so, its own or its base's.
func (k *Kit) Theme(name string) (*Theme, bool) {
	return lookUp(k, name, func(k *Kit) map[string]*Theme { return k.themes })
}

// Template returns the template of k named so: its own, else the base's compiled again with its own parts, the first
// time it is taken, else the base's; and floe's failure to compile it again, to be looked at before ok. It is the only
// place a compile of k.rebuilt is called.
func (k *Kit) Template(name string) (*Template, bool, error) {
	for at := k; at != nil; at = at.base {
		if t, ok := at.templates[name]; ok {
			return t, true, nil
		}
		if again, ok := at.rebuilt[name]; ok {
			t, err := again()
			return t, true, err
		}
	}
	return nil, false, nil
}

// Resources returns the schemas of the parts and of what they share by URL, its own and its base's, a copy.
func (k *Kit) Resources() map[string]schema.Resource {
	return maps.Clone(k.resources)
}

// Names returns the names of the parts, looks, themes and templates of k and its base, each sorted.
func (k *Kit) Names() (parts, looks, themes, templates []string) {
	for at := k; at != nil; at = at.base {
		parts = append(parts, slices.Collect(maps.Keys(at.parts))...)
		looks = append(looks, slices.Collect(maps.Keys(at.looks))...)
		themes = append(themes, slices.Collect(maps.Keys(at.themes))...)
		templates = append(templates, slices.Collect(maps.Keys(at.templates))...)
	}
	sorted := func(s []string) []string { return slices.Compact(slices.Sorted(slices.Values(s))) }
	return sorted(parts), sorted(looks), sorted(themes), sorted(templates)
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

// Taken returns the looks the part or look name takes, and those they take in turn, each once, in the order
// first reached. No look takes itself: check refuses one that would.
func (k *Kit) Taken(name string) []string {
	var found []string
	var follow func(at string)
	follow = func(at string) {
		for _, t := range k.looksOf(at) {
			if !slices.Contains(found, t.name) {
				found = append(found, t.name)
				follow(t.name)
			}
		}
	}
	follow(name)
	return found
}

// Marks returns which elements of the part or look name are rows with a mark at their left, as its part.yaml's
// marks: says: "box", "items", "joined", or none.
func (k *Kit) Marks(name string) string { return k.marksOf(name).name }

func (k *Kit) looksOf(name string) []taken {
	if p, ok := k.Part(name); ok {
		return p.looks
	}
	if l, ok := k.Look(name); ok {
		return l.looks
	}
	return nil
}

func (k *Kit) marksOf(name string) taken {
	if p, ok := k.Part(name); ok {
		return p.marks
	}
	var none taken
	if l, ok := k.Look(name); ok {
		none = l.marks
	}
	return none
}

// isPartOrLook reports whether name is a part or a look of k or its base.
func (k *Kit) isPartOrLook(name string) bool {
	_, part := k.Part(name)
	_, look := k.Look(name)
	return part || look
}

// entries returns the names of the directories (dirs) or files of the directory dir of d ("." for d
// itself), those starting with a dot left out, sorted; none without an FS. A symlink to a file is that file,
// as the system follows it (dotfiles link files into a directory of their own); one to a directory is
// refused, as floe reads the directories it holds as they are; one that leads nowhere cannot be read.
func entries(d Dir, dir string, dirs bool) ([]string, error) {
	if d.FS == nil {
		return nil, nil
	}
	all, err := fs.ReadDir(d.FS, dir)
	if err != nil {
		return nil, usererr.CannotRead(path.Join(d.Name, dir), err)
	}
	var names []string
	var found usererr.List
	for _, e := range all {
		if files.Hidden(e.Name()) {
			continue
		}
		at := path.Join(d.Name, dir, e.Name())
		isDir := e.IsDir()
		if e.Type()&fs.ModeSymlink != 0 {
			fi, err := fs.Stat(d.FS, path.Join(dir, e.Name()))
			switch {
			case err != nil:
				if failed := found.Add(usererr.CannotRead(at, err)); failed != nil {
					return nil, failed
				}
				continue
			case fi.IsDir():
				found = append(found, usererr.NewAt(at, 0, 0, "kit_dir_link"))
				continue
			}
		}
		if isDir == dirs {
			names = append(names, e.Name())
		}
	}
	return names, found.Err()
}

// readFile returns the file name of the directory dir of d as it is, and false when there is none: the byte
// order mark at its start is left to the reader of its kind. One that cannot be read is told in the system's
// words, at it. What is neither a regular file nor a directory (a FIFO, a link to a device) is refused unread,
// as reading it may block or never end.
func readFile(d Dir, dir, name string) ([]byte, bool, error) {
	at := path.Join(d.Name, dir, name)
	// LIMIT: a file made a FIFO between the Stat and the read blocks it; open it without blocking if it comes to.
	if fi, err := fs.Stat(d.FS, path.Join(dir, name)); err == nil && files.IsNotFile(fi) {
		return nil, false, usererr.CannotRead(at, usererr.ErrNotFile)
	}
	src, err := fs.ReadFile(d.FS, path.Join(dir, name))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, false, nil
	case err != nil:
		return nil, false, usererr.CannotRead(at, err)
	}
	if d.read != nil {
		d.read[name] = src
	}
	return src, true, nil
}

// readCSS returns the CSS file name of the directory dir of d as readFile does, without a byte order mark at its
// start and its line breaks made LF (css.Preprocess): what is checked is what goes in the page.
func readCSS(d Dir, dir, name string) ([]byte, bool, error) {
	src, ok, err := readFile(d, dir, name)
	if !ok || err != nil {
		return nil, ok, err
	}
	return css.Preprocess(bom.Trim(src)), true, nil
}

// has reports whether the directory dir of d holds the file name.
func has(d Dir, dir, name string) (bool, error) {
	_, err := fs.Stat(d.FS, path.Join(dir, name))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	case err != nil:
		return false, usererr.CannotRead(path.Join(d.Name, dir, name), err)
	}
	return true, nil
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
		return nil, err
	}
	return readWords(path.Join(d.Name, dir, "words.yaml"), src)
}

// OwnTemplates returns the names of the templates k read itself, not its base's, sorted.
func (k *Kit) OwnTemplates() []string {
	return slices.Sorted(maps.Keys(k.templates))
}
