package kit

import (
	"bytes"
	"errors"
	"html/template"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/bom"
	"github.com/sukekyo26/floe/internal/rules"
	"github.com/sukekyo26/floe/internal/schema"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// Part is a part read: what draws an item of its schema.
type Part struct {
	name    string
	dir     string // as its mistakes name it
	looks   []taken
	css     []byte
	hasCSS  bool
	checks  *rules.Checks // nil for none
	words   words
	pages   pages // nil without a part.html
	pageSrc []byte
	unread  []string         // the files of its directory it does not read, for the check of what it holds
	schema  *schema.Compiled // its schema as a root
	files   []Source         // schema.json and checks.yaml as read, for floe part show
}

// taken is a look a part or a look takes, and where its part.yaml names it.
type taken struct {
	name string
	file string
	at   yamlpos.Pos
}

func names(looks []taken) []string {
	out := make([]string, len(looks))
	for i, l := range looks {
		out[i] = l.name
	}
	return out
}

// Name returns the name of p.
func (p *Part) Name() string { return p.name }

// Looks returns the names of the looks p takes, in order.
func (p *Part) Looks() []string { return names(p.looks) }

// CSS returns the part.css of p, none for one left out.
func (p *Part) CSS() []byte { return bytes.Clone(p.css) }

// Unread returns the files of the directory of p it does not read.
func (p *Part) Unread() []string { return slices.Clone(p.unread) }

// Checks returns the checks of p, nil for none.
func (p *Part) Checks() *rules.Checks { return p.checks }

// Page returns a copy of the part.html of p, parsed, in the language tag of a page, to draw, as Template.Page
// does, its copyButton in that language.
func (p *Part) Page(lang string) (*template.Template, error) {
	page, err := p.pages.copyOf(p.words, lang)
	if err != nil {
		return nil, err
	}
	return page.Funcs(template.FuncMap{"copyButton": copyButton(lang)}), nil
}

// File returns the file name of p as its mistakes name it.
func (p *Part) File(name string) string { return path.Join(p.dir, name) }

// PageSource returns the part.html of p as read: where a mistake of drawing it is.
func (p *Part) PageSource() []byte { return bytes.Clone(p.pageSrc) }

// Look is a look read: CSS that parts and HTML take without being its box.
type Look struct {
	name   string
	dir    string
	about  string
	looks  []taken
	css    []byte
	hasCSS bool
	unread []string // the files of its directory it does not read, part.html, checks.yaml and words.yaml too
}

// Name returns the name of l.
func (l *Look) Name() string { return l.name }

// About returns what l is, its part.yaml's description.
func (l *Look) About() string { return l.about }

// Looks returns the names of the looks l takes, in order.
func (l *Look) Looks() []string { return names(l.looks) }

// CSS returns the part.css of l.
func (l *Look) CSS() []byte { return bytes.Clone(l.css) }

// Unread returns the files of the directory of l it does not read.
func (l *Look) Unread() []string { return slices.Clone(l.unread) }

// partURL is where floe holds the schema of the part, or the shared schema, named so.
func partURL(name string) string { return "floe:///parts/" + name + ".json" }

// readParts reads into k the parts and looks of d, each a directory, a part one with a schema.json, and the
// schemas the parts share, <name>.json beside them. A part, a look and a shared schema of one name, here or in
// the base but of the same kind, are refused (clashes).
func (k *Kit) readParts(d Dir) error {
	dirs, err := entries(d, ".", true)
	if err != nil {
		return err
	}
	files, err := entries(d, ".", false)
	if err != nil {
		return err
	}
	var shared []string
	for _, file := range files {
		if name, ok := strings.CutSuffix(file, ".json"); ok {
			shared = append(shared, name)
		}
	}
	found := k.clashes(d, dirs, shared)
	for _, name := range dirs {
		if !refused(found, d, name) {
			if failed := found.Add(k.readPart(d, name)); failed != nil {
				return failed
			}
		}
	}
	for _, name := range shared {
		if !refused(found, d, name+".json") {
			if failed := found.Add(k.readShared(d, name)); failed != nil {
				return failed
			}
		}
	}
	if found != nil {
		return found.Sorted().Err()
	}
	return k.compileParts(dirs, shared)
}

// compileParts compiles each part and shared schema of k read from the names dirs and shared, all read without
// a mistake (a $ref to one not read would be told missing), at its own file: a template compiled with it would
// tell a mistake of it at the template. Each keeps its schema compiled, for check.
func (k *Kit) compileParts(dirs, shared []string) error {
	var found usererr.List
	for _, name := range slices.Concat(dirs, shared) {
		p, part := k.parts[name]
		if !part && !slices.Contains(shared, name) {
			continue // a look, which has no schema
		}
		c, err := schema.CompilePart(partURL(name), k.resources)
		if failed := found.Add(err); failed != nil {
			return failed
		}
		switch {
		case err != nil:
		case part:
			p.schema = c
		default:
			k.shared[name].schema = c
		}
	}
	return found.Sorted().Err()
}

// clashes refuses each of the parts and looks (dirs) and the shared schemas of d whose name refusedName
// refuses. Each is told at it.
func (k *Kit) clashes(d Dir, dirs, shared []string) usererr.List {
	var found usererr.List
	for _, name := range dirs {
		if e := k.refusedName(d.Name, KindPart, name, false); e != nil {
			found = append(found, e)
		}
	}
	for _, name := range shared {
		if e := k.refusedName(d.Name, KindShared, name, slices.Contains(dirs, name)); e != nil {
			found = append(found, e)
		}
	}
	return found
}

// Kind is a kind of the things of one's own, each a directory or a file in the directory of its kind.
type Kind int

// The kinds of one's own: a part or a look (a directory of the parts), a schema the parts share (<name>.json
// beside them), a theme and a template.
const (
	KindPart Kind = iota
	KindShared
	KindTheme
	KindTemplate
)

// NameMistake returns the mistake Load would tell of a part or a look (KindPart), or a template, of one's own
// named name made in dir, the directory of its kind as it is named, over k: nil for none.
func (k *Kit) NameMistake(dir string, kind Kind, name string) error {
	// A part beside a shared schema of its name, which k holds only of its own.
	r, have := k.resources[partURL(name)]
	_, floes := k.baseDir(name)
	twice := kind == KindPart && have && r.Kind == schema.Shared && !floes
	if e := k.refusedName(dir, kind, name, twice); e != nil {
		return e
	}
	return nil
}

// refusedName returns the mistake of the name of one's own of the kind in dir, nil for none: a name floe does
// not take (validName); a part's or a look's that is a theme's file (base, palette); one a part or a look and
// a shared schema of dir both have (twice); and a part's, a look's, a shared schema's or a theme's that the
// base has: floe's templates and parts would go on reading the base's, compiled with it, and a theme of one's
// own would hide floe's whole. It is told at the entry.
func (k *Kit) refusedName(dir string, kind Kind, name string, twice bool) *usererr.Error {
	at := path.Join(dir, name)
	if kind == KindShared {
		at += ".json"
	}
	switch {
	case !validName(name):
		return usererr.NewAt(at, 0, 0, "kit_bad_name")
	case kind == KindPart && (name == "base" || name == "palette"):
		return usererr.NewAt(at, 0, 0, "kit_name_theme_file", name)
	case twice:
		return usererr.NewAt(at, 0, 0, "kit_name_twice", name, path.Join(dir, name), path.Join(dir, name+".json"))
	}
	var other string
	var ok bool
	switch kind {
	case KindPart, KindShared:
		other, ok = k.baseDir(name)
	case KindTheme:
		if k.base != nil {
			var t *Theme
			if t, ok = k.base.Theme(name); ok {
				other = t.dir
			}
		}
	case KindTemplate:
	}
	if ok {
		return usererr.NewAt(at, 0, 0, "kit_name_floe", name, other)
	}
	return nil
}

// validName reports whether name is one floe takes for a part, a look, a shared schema, a theme or a template:
// it goes in a URL (floe:///parts/<name>.json, floe://templates/<name>/) and a CSS selector.
func validName(name string) bool {
	return nameRe.MatchString(name)
}

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// refused reports whether found refuses the entry name of d.
func refused(found usererr.List, d Dir, name string) bool {
	return slices.ContainsFunc(found, func(e *usererr.Error) bool { return e.File() == path.Join(d.Name, name) })
}

// readShared reads into k the schema the parts of d share, name.json.
func (k *Kit) readShared(d Dir, name string) error {
	src, _, err := readFile(d, "", name+".json")
	if err != nil {
		return err
	}
	r, err := schema.NewResource(partURL(name), path.Join(d.Name, name+".json"), src, schema.Shared)
	if err != nil {
		return err //nolint:wrapcheck // its mistakes, or floe's failure
	}
	k.resources[r.URL] = r
	k.shared[name] = &shared{name: name, file: r.File, schema: nil, files: sources(Source{name + ".json", src})}
	return nil
}

// readPart reads into k the part or look of d named name, its directory.
func (k *Kit) readPart(d Dir, name string) error {
	dir := path.Join(d.Name, name)
	schemaSrc, isPart, err := readFile(d, name, "schema.json")
	if err != nil {
		return err
	}
	css, hasCSS, err := readCSS(d, name, "part.css")
	if err != nil {
		return err
	}
	looks, about, yamlErr := readPartYAML(d, name, !isPart)
	if !isPart {
		if yamlErr == nil {
			unread, failed := unreadIn(d, name, "part.css", "part.yaml")
			if failed != nil {
				return failed
			}
			k.looks[name] = &Look{name: name, dir: dir, about: about, looks: looks, css: css, hasCSS: hasCSS, unread: unread}
		}
		return yamlErr
	}
	var found usererr.List
	if failed := found.Add(yamlErr); failed != nil {
		return failed
	}
	r, err := schema.NewResource(partURL(name), path.Join(dir, "schema.json"), schemaSrc, schema.Part)
	if failed := found.Add(err); failed != nil {
		return failed
	}
	checks, checksSrc, err := readChecks(d, name)
	if failed := found.Add(err); failed != nil {
		return failed
	}
	w, err := readWordsIn(d, name)
	if failed := found.Add(err); failed != nil {
		return failed
	}
	ps, pageSrc, err := readPages(d, name, "part.html", partFuncsAll(), w)
	if failed := found.Add(err); failed != nil {
		return failed
	}
	if found != nil {
		return found.Sorted().Err()
	}
	k.resources[r.URL] = r
	unread, err := unreadIn(d, name, "schema.json", "part.html", "part.css", "part.yaml", "checks.yaml", "words.yaml")
	if err != nil {
		return err
	}
	k.parts[name] = &Part{
		name: name, dir: dir, looks: looks, css: css, hasCSS: hasCSS, checks: checks, words: w, pages: ps,
		pageSrc: pageSrc, unread: unread, schema: nil,
		files: sources(Source{"schema.json", schemaSrc}, Source{"checks.yaml", checksSrc}),
	}
	return nil
}

// unreadIn returns the files and directories (with a / after) of the directory dir of d but read, sorted.
func unreadIn(d Dir, dir string, read ...string) ([]string, error) {
	files, err := entries(d, dir, false)
	if err != nil {
		return nil, err
	}
	dirs, err := entries(d, dir, true)
	if err != nil {
		return nil, err
	}
	for _, sub := range dirs {
		files = append(files, sub+"/")
	}
	files = slices.DeleteFunc(files, func(f string) bool { return slices.Contains(read, f) })
	slices.Sort(files)
	return files, nil
}

// baseDir returns the file of the part, look or shared schema of the base of k named so, and false for none.
func (k *Kit) baseDir(name string) (string, bool) {
	if k.base == nil {
		return "", false
	}
	if p, ok := k.base.Part(name); ok {
		return p.dir, true
	}
	if l, ok := k.base.Look(name); ok {
		return l.dir, true
	}
	if r, ok := k.base.resources[partURL(name)]; ok {
		return r.File, true
	}
	return "", false
}

// readPages parses the HTML file of the directory dir of d with fns and the words w, and returns it with its
// text, both without a byte order mark at its start, so that places told in the text agree; nil without one.
func readPages(d Dir, dir, file string, fns template.FuncMap, w words) (pages, []byte, error) {
	src, ok, err := readFile(d, dir, file)
	if !ok || err != nil {
		return nil, nil, err
	}
	src = bom.Trim(src)
	ps, err := parsePages(path.Join(d.Name, dir, file), src, fns, w)
	return ps, src, err
}

// readPartYAML reads the part.yaml of the directory dir of d, of a look alone when look: the looks it takes,
// and a look's description; none without one. A part's description is its schema's. Its mistakes are told
// all together.
func readPartYAML(d Dir, dir string, look bool) ([]taken, string, error) {
	file := path.Join(d.Name, dir, "part.yaml")
	src, ok, err := readFile(d, dir, "part.yaml")
	if !ok || err != nil {
		return nil, "", err
	}
	body, err := yamlpos.Parse(file, src)
	switch {
	case errors.Is(err, yamlpos.ErrEmpty):
		return nil, "", nil
	case err != nil:
		return nil, "", err //nolint:wrapcheck // mistakes at their lines and columns, or floe's failure
	}
	m, ok := body.(*ast.MappingNode)
	if !ok {
		return nil, "", yamlpos.New(file, yamlpos.Of(body), "kit_part_yaml")
	}
	var looks []taken
	var about string
	var found usererr.List
	for _, kv := range m.Values {
		k := yamlpos.KeyOf(kv)
		switch key := k.GetToken().Value; {
		case key == "looks":
			var more usererr.List
			if looks, more, err = readLooks(file, k, kv.Value); err != nil {
				return nil, "", err
			}
			found = append(found, more...)
		case key == "description" && !look:
			found = append(found, yamlpos.New(file, yamlpos.Of(k), "kit_part_yaml_description"))
		case key == "description":
			if about, ok, err = yamlpos.Text(kv.Value); err != nil {
				return nil, "", err //nolint:wrapcheck // floe's failure, which says what failed
			} else if !ok {
				found = append(found, yamlpos.New(file, yamlpos.OfValue(k, kv.Value), "kit_part_yaml_about"))
			}
		default:
			found = append(found, yamlpos.New(file, yamlpos.Of(k), "kit_part_yaml_key", key))
		}
	}
	return looks, about, found.Sorted().Err()
}

// readLooks reads the looks part.yaml, file, gives the key k: a list of names, each text, as a key is (YAML
// would read a name of digits, 1.10, as the number 1.1); or floe's failure.
func readLooks(file string, k, value ast.Node) ([]taken, usererr.List, error) {
	seq, ok := value.(*ast.SequenceNode)
	if !ok {
		return nil, usererr.List{yamlpos.New(file, yamlpos.OfValue(k, value), "kit_part_yaml_looks")}, nil
	}
	looks := make([]taken, 0, len(seq.Values))
	var found usererr.List
	for _, v := range seq.Values {
		name, ok, err := yamlpos.Text(v)
		switch {
		case err != nil:
			return nil, nil, err //nolint:wrapcheck // floe's failure, which says what failed
		case !ok || name == "":
			found = append(found, yamlpos.New(file, yamlpos.Of(v), "kit_part_yaml_looks"))
		default:
			looks = append(looks, taken{name: name, file: file, at: yamlpos.Of(v)})
		}
	}
	return looks, found, nil
}
