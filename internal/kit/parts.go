package kit

import (
	"errors"
	"html/template"
	"path"
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
func (p *Part) CSS() []byte { return p.css }

// Checks returns the checks of p, nil for none.
func (p *Part) Checks() *rules.Checks { return p.checks }

// Page returns the part.html of p, parsed, in the language tag of a page.
func (p *Part) Page(lang string) *template.Template { return p.pages.page(p.words, lang) }

// Look is a look read: CSS that parts and HTML take without being its box.
type Look struct {
	name    string
	dir     string
	about   string
	looks   []taken
	css     []byte
	hasCSS  bool
	hasHTML bool // a part.html without a schema.json: a part missing its schema
}

// Name returns the name of l.
func (l *Look) Name() string { return l.name }

// About returns what l is, its part.yaml's description.
func (l *Look) About() string { return l.about }

// Looks returns the names of the looks l takes, in order.
func (l *Look) Looks() []string { return names(l.looks) }

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
	css, hasCSS, err := readCSS(d, name, "part.css")
	if err != nil {
		return err
	}
	looks, about, yamlErr := readPartYAML(d, name, !isPart)
	if !isPart {
		if yamlErr == nil {
			_, hasHTML, failed := readFile(d, name, "part.html")
			if failed != nil {
				return failed
			}
			k.looks[name] = &Look{name: name, dir: dir, about: about, looks: looks, css: css, hasCSS: hasCSS, hasHTML: hasHTML}
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
	k.parts[name] = &Part{
		name: name, dir: dir, looks: looks, css: css, hasCSS: hasCSS, checks: checks, words: w, pages: ps,
		pageSrc: pageSrc,
	}
	return nil
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
