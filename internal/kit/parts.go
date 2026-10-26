package kit

import (
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
	name   string
	dir    string // as its mistakes name it
	looks  []string
	css    []byte
	checks *rules.Checks // nil for none
	words  words
	pages  pages
}

// Name returns the name of p.
func (p *Part) Name() string { return p.name }

// Looks returns the names of the looks p takes, in order.
func (p *Part) Looks() []string { return slices.Clone(p.looks) }

// CSS returns the part.css of p, none for one left out.
func (p *Part) CSS() []byte { return p.css }

// Checks returns the checks of p, nil for none.
func (p *Part) Checks() *rules.Checks { return p.checks }

// Page returns the part.html of p, parsed, in the language tag of a page.
func (p *Part) Page(lang string) *template.Template { return p.pages.page(p.words, lang) }

// Look is a look read: CSS that parts and HTML take without being its box.
type Look struct {
	name  string
	dir   string
	about string
	looks []string
	css   []byte
}

// Name returns the name of l.
func (l *Look) Name() string { return l.name }

// About returns what l is, its part.yaml's description.
func (l *Look) About() string { return l.about }

// Looks returns the names of the looks l takes, in order.
func (l *Look) Looks() []string { return slices.Clone(l.looks) }

// CSS returns the part.css of l.
func (l *Look) CSS() []byte { return l.css }

// partsURL is where floe holds the schemas of the parts, and those they share: floe:///parts/<name>.json.
const partsURL = "floe:///parts/"

// readParts reads into k the parts and looks of d, each a directory, a part one with a schema.json, and the
// schemas the parts share, <name>.json beside them.
func (k *Kit) readParts(d Dir) error {
	dirs, err := entries(d, ".", true)
	if err != nil {
		return err
	}
	var found usererr.List
	for _, name := range dirs {
		if failed := found.Add(k.readPart(d, name)); failed != nil {
			return failed
		}
	}
	files, err := entries(d, ".", false)
	if err != nil {
		return err
	}
	for _, file := range files {
		name, ok := strings.CutSuffix(file, ".json")
		if !ok {
			continue
		}
		src, _, err := readFile(d, "", file)
		if err != nil {
			return err
		}
		r, err := schema.NewResource(path.Join(d.Name, file), src, schema.Shared)
		if failed := found.Add(err); failed != nil {
			return failed
		}
		if err == nil {
			k.resources[partsURL+name+".json"] = r
		}
	}
	return found.Sorted().Err()
}

// readPart reads into k the part or look of d named name, its directory.
func (k *Kit) readPart(d Dir, name string) error {
	dir := path.Join(d.Name, name)
	schemaSrc, isPart, err := readFile(d, name, "schema.json")
	if err != nil {
		return err
	}
	css, _, err := readFile(d, name, "part.css")
	if err != nil {
		return err
	}
	looks, about, yamlErr := readPartYAML(d, name, !isPart)
	if !isPart {
		if yamlErr == nil {
			k.looks[name] = &Look{name: name, dir: dir, about: about, looks: looks, css: css}
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
	ps, err := readPages(d, name, "part.html", partFuncsAll(), w)
	if failed := found.Add(err); failed != nil {
		return failed
	}
	if found != nil {
		return found.Sorted().Err()
	}
	k.resources[partsURL+name+".json"] = r
	k.parts[name] = &Part{
		name: name, dir: dir, looks: looks, css: css, checks: checks, words: w, pages: ps,
	}
	return nil
}

// readPages parses the HTML file of the directory dir of d with fns and the words w; nil without one.
func readPages(d Dir, dir, file string, fns template.FuncMap, w words) (pages, error) {
	src, ok, err := readFile(d, dir, file)
	if !ok || err != nil {
		return nil, err
	}
	return parsePages(path.Join(d.Name, dir, file), src, fns, w)
}

// readPartYAML reads the part.yaml of the directory dir of d, of a look alone when look: the looks it takes,
// and a look's description; none without one. A part's description is its schema's. Its mistakes are told
// all together.
func readPartYAML(d Dir, dir string, look bool) ([]string, string, error) {
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
	var looks []string
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
func readLooks(file string, k, value ast.Node) ([]string, usererr.List) {
	seq, ok := value.(*ast.SequenceNode)
	if !ok {
		return nil, usererr.List{yamlpos.New(file, yamlpos.OfValue(k, value), "kit_part_yaml_looks")}
	}
	looks := make([]string, 0, len(seq.Values))
	var found usererr.List
	for _, v := range seq.Values {
		_, scalar := v.(ast.ScalarNode)
		_, null := v.(*ast.NullNode)
		if !scalar || null || v.GetToken().Value == "" {
			found = append(found, yamlpos.New(file, yamlpos.Of(v), "kit_part_yaml_looks"))
			continue
		}
		looks = append(looks, v.GetToken().Value)
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
