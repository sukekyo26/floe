package kit

import (
	"bytes"
	"errors"
	"html/template"
	"path"
	"slices"
	"strings"

	"github.com/goccy/go-yaml/ast"

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
	unread  []string // the files of its directory it does not read, for the check of what it holds
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

// Page returns the part.html of p, parsed, in the language tag of a page.
func (p *Part) Page(lang string) *template.Template { return p.pages.page(p.words, lang) }

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

// partsURL is where floe holds the schemas of the parts, and those they share: floe:///parts/<name>.json.
const partsURL = "floe:///parts/"

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
	found, err := k.clashes(d, dirs, shared)
	if err != nil {
		return err
	}
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
	return found.Sorted().Err()
}

// clashes refuses each of the parts and looks (dirs) and the shared schemas of d that is of a name another of
// d is, or that one of the base of another kind is, told at it. A look over the base's part, or a part over
// its look, is of another kind too: the base's parts and schemas name the base's by its name.
func (k *Kit) clashes(d Dir, dirs, shared []string) (usererr.List, error) {
	var found usererr.List
	twice := func(at, name, other string) {
		found = append(found, usererr.NewAt(at, 0, 0, "kit_name_twice", name, other, at))
	}
	for _, name := range dirs {
		dir := path.Join(d.Name, name)
		_, isPart, err := readFile(d, name, "schema.json")
		if err != nil {
			return nil, err
		}
		if slices.Contains(shared, name) {
			twice(path.Join(d.Name, name+".json"), name, dir)
		} else if r, ok := k.resources[partsURL+name+".json"]; ok && r.Kind == schema.Shared {
			twice(dir, name, r.File)
		} else if p, ok := k.base.Part(name); ok && !isPart {
			found = append(found, usererr.NewAt(dir, 0, 0, "kit_look_over_part", name, p.dir))
		} else if l, ok := k.base.Look(name); ok && isPart {
			found = append(found, usererr.NewAt(dir, 0, 0, "kit_part_over_look", name, l.dir))
		}
	}
	for _, name := range shared {
		if other, ok := k.baseDir(name); ok && !slices.Contains(dirs, name) {
			twice(path.Join(d.Name, name+".json"), name, other)
		}
	}
	return found, nil
}

// refused reports whether found refuses the entry name of d.
func refused(found usererr.List, d Dir, name string) bool {
	return slices.ContainsFunc(found, func(e *usererr.Error) bool { return e.File == path.Join(d.Name, name) })
}

// readShared reads into k the schema the parts of d share, name.json.
func (k *Kit) readShared(d Dir, name string) error {
	src, _, err := readFile(d, "", name+".json")
	if err != nil {
		return err
	}
	r, err := schema.NewResource(path.Join(d.Name, name+".json"), src, schema.Shared)
	if err != nil {
		return err //nolint:wrapcheck // its mistakes, or floe's failure
	}
	k.resources[partsURL+name+".json"] = r
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
	r, err := schema.NewResource(path.Join(dir, "schema.json"), schemaSrc, schema.Part)
	if failed := found.Add(err); failed != nil {
		return failed
	}
	checks, err := readChecks(d, name)
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
	k.resources[partsURL+name+".json"] = r
	unread, err := unreadIn(d, name, "schema.json", "part.html", "part.css", "part.yaml", "checks.yaml", "words.yaml")
	if err != nil {
		return err
	}
	k.parts[name] = &Part{
		name: name, dir: dir, looks: looks, css: css, hasCSS: hasCSS, checks: checks, words: w, pages: ps,
		pageSrc: pageSrc, unread: unread,
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

// baseDir returns the directory of the part or look of the base of k named so, and false for none.
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
	return "", false
}

// readPages parses the HTML file of the directory dir of d with fns and the words w, and returns it with its
// text; nil without one.
func readPages(d Dir, dir, file string, fns template.FuncMap, w words) (pages, []byte, error) {
	src, ok, err := readFile(d, dir, file)
	if !ok || err != nil {
		return nil, nil, err
	}
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
			looks, more = readLooks(file, k, kv.Value)
			found = append(found, more...)
		case key == "description" && !look:
			found = append(found, yamlpos.New(file, yamlpos.Of(k), "kit_part_yaml_description"))
		case key == "description":
			if about, ok = textOf(kv.Value); !ok {
				found = append(found, yamlpos.New(file, yamlpos.OfValue(k, kv.Value), "kit_part_yaml_about"))
			}
		default:
			found = append(found, yamlpos.New(file, yamlpos.Of(k), "kit_part_yaml_key", key))
		}
	}
	return looks, about, found.Sorted().Err()
}

// readLooks reads the looks part.yaml, file, gives the key k: a list of names, each text as written (YAML
// reads a name of digits, 404, as a number).
func readLooks(file string, k, value ast.Node) ([]taken, usererr.List) {
	seq, ok := value.(*ast.SequenceNode)
	if !ok {
		return nil, usererr.List{yamlpos.New(file, yamlpos.OfValue(k, value), "kit_part_yaml_looks")}
	}
	looks := make([]taken, 0, len(seq.Values))
	var found usererr.List
	for _, v := range seq.Values {
		_, scalar := v.(ast.ScalarNode)
		_, null := v.(*ast.NullNode)
		if !scalar || null || v.GetToken().Value == "" {
			found = append(found, yamlpos.New(file, yamlpos.Of(v), "kit_part_yaml_looks"))
			continue
		}
		looks = append(looks, taken{name: v.GetToken().Value, file: file, at: yamlpos.Of(v)})
	}
	return looks, found
}

// textOf returns the text of n, a string or a | or > block, and false for anything else.
func textOf(n ast.Node) (string, bool) {
	switch text := n.(type) {
	case *ast.StringNode:
		return text.Value, true
	case *ast.LiteralNode:
		return text.Value.Value, true
	}
	return "", false
}
