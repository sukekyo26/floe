package tmpl

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"maps"
	"net/url"
	"os"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/goccy/go-yaml/ast"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// partFiles holds a directory per part of floe's (schema.json, part.html, part.css, part.yaml, checks.yaml
// and words.yaml) and per look alone (part.css and part.yaml), the schemas the parts share (tone.json),
// and the parts themes (themes/).
//
//go:embed parts
var partFiles embed.FS

// partsURL is where a schema refers to floe's parts and the schemas they share: floe:///parts/table.json.
const partsURL = "floe:///parts/"

// partName returns the name of the part of floe's, or of the schema the parts share, that ref names as
// floe:///parts/<name>.json, and false for a ref written otherwise.
func partName(ref string) (string, bool) {
	name, ok := strings.CutPrefix(ref, partsURL)
	if !ok {
		return "", false
	}
	name, ok = strings.CutSuffix(name, ".json")
	return name, ok && name != "" && !strings.ContainsAny(name, "/#")
}

// absolute returns ref read from the schema at base, as jsonschema reads it: ../../parts/table.json
// from a template's schema is floe:///parts/table.json. A ref that is not a URL stays as written.
func absolute(base, ref string) string {
	b, err := url.Parse(base)
	if err != nil {
		return ref
	}
	r, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	// An empty fragment points at the whole: floe:///parts/table.json# is the table.
	return strings.TrimSuffix(b.ResolveReference(r).String(), "#")
}

// pointsInto returns the part of floe's, whole, that ref points into, and false when it does not: a
// ref into a part would be checked as the part but drawn as plain data.
func (set *partSet) pointsInto(ref string) (whole string, into bool) {
	whole, _, into = strings.Cut(ref, "#")
	name, ok := partName(whole)
	return whole, into && ok && set.parts[name] != nil && !set.parts[name].look
}

// first returns the first of s and the schemas under it that ok takes, with its pointer under ptr, or nil.
func (s *schema) first(ptr string, ok func(*schema) bool) (string, *schema) {
	if s == nil {
		return "", nil
	}
	if ok(s) {
		return ptr, s
	}
	for _, name := range slices.Sorted(maps.Keys(s.Properties)) {
		if at, found := s.Properties[name].first(ptr+"/properties/"+pointerKey.Replace(name), ok); found != nil {
			return at, found
		}
	}
	return s.Items.first(ptr+"/items", ok)
}

// absolute makes the $ref of s, and those under it, absolute, read from the schema at base.
func (s *schema) absolute(base string) {
	if s == nil {
		return
	}
	if s.Ref != "" {
		s.Ref = absolute(base, s.Ref)
	}
	for _, p := range s.Properties {
		p.absolute(base)
	}
	s.Items.absolute(base)
}

// part is a part of floe's: a template's schema refers to it, and floe checks it, draws it and puts its
// style into the page. A look alone (panel) is a style other parts and HTML take, which draws nothing.
type part struct {
	name    string
	schema  *schema                       // nil for a look alone
	pages   map[string]*template.Template // by the language of its words; nil for one floe draws in Go (goParts)
	words   words
	html    string // the template of pages, as written
	css     string
	checks  []relation
	looks   []string // the looks it takes, as part.yaml names them, each before it
	looksAt []int    // the line of each in part.yaml
	look    bool     // a look alone: a part.css and a part.yaml, no schema.json nor part.html
	about   string   // what a look alone is, part.yaml's description
	uses    []string // the parts its HTML names, its schema holds and it takes the look of, itself among them
}

// goParts are the parts floe draws in Go, each a description and its style: their schema takes an object,
// whose shape floe checks where it draws it.
var goParts = map[string]struct{ about, css string }{
	"diagram": {
		about: "A diagram: a spec as floe diagram render takes it (id, width, height, title, parts); " +
			"floe diagram example shows one of each kind",
		css: diagram.Style(),
	},
	"script": {
		about: "Something that moves: js, a function (el, data) { … } the page calls with an empty element in " +
			"its place and data, any value. It runs in the page's sandbox, where fetch, XHR, images from " +
			"elsewhere and forms are blocked",
		css: "",
	},
}

// isGoPart reports whether name is a part floe draws in Go.
func isGoPart(name string) bool {
	_, ok := goParts[name]
	return ok
}

// partSet is floe's parts, every schema under parts/, by URL: the parts' and those they share, and the
// parts themes.
type partSet struct {
	parts   map[string]*part
	docs    map[string]any         // as read, for jsonschema
	schemas map[string]*schema     // as floe reads them
	themes  map[string]*partsTheme // by name
	broken  map[string]error       // the parts of one's own that cannot be used, by name, and why
	// the parts themes of one's own that cannot be used, by name, and why
	brokenThemes map[string]error
	// the directories of one's own it read, the directory of parts first, by their real paths
	dirs   []string
	strays []*usererr.Error // what of one's own goes into no page, to be warned of
}

// builtinParts reads floe's parts once.
var builtinParts = sync.OnceValues(func() (*partSet, error) { return readParts("", nil) })

// readParts reads floe's parts and, over them, those of one's own in dir, "" for none, which holds
// entries: one of the name of one of floe's takes its place whole. One of one's own that cannot be used
// is left out, its mistake kept for a template that uses it.
func readParts(dir string, entries []os.DirEntry) (*partSet, error) {
	set := &partSet{
		parts: map[string]*part{}, docs: map[string]any{}, schemas: map[string]*schema{}, themes: map[string]*partsTheme{},
		broken: map[string]error{}, brokenThemes: map[string]error{}, dirs: nil, strays: nil,
	}
	if err := set.readBuiltin(); err != nil {
		return nil, err
	}
	if dir != "" {
		if err := set.readOwn(dir, entries); err != nil {
			return nil, err
		}
	}
	for _, s := range set.schemas {
		s.resolve(set)
	}
	for _, p := range set.parts {
		p.uses = append(set.partsNamedIn(p.html), p.looks...)
		p.schema.partsUnder(&p.uses)
		slices.Sort(p.uses)
		p.uses = slices.Compact(p.uses)
	}
	return set, nil
}

// readBuiltin reads floe's parts, those it draws in Go too, the schemas they share and the parts themes
// into set.
func (set *partSet) readBuiltin() error {
	builtin, err := fs.Sub(partFiles, "parts")
	if err != nil {
		return fmt.Errorf("read the parts: %w", err)
	}
	entries, err := fs.ReadDir(builtin, ".")
	if err != nil {
		return fmt.Errorf("read the parts: %w", err)
	}
	for _, e := range entries {
		switch {
		case e.Name() == themesDir:
			// Read once the parts are, whose names its files take.
		case e.IsDir():
			var files fs.FS
			if files, err = fs.Sub(builtin, e.Name()); err == nil {
				err = set.readPart(files, e.Name(), path.Join("parts", e.Name()))
			}
		default:
			// A schema the parts share.
			err = set.readSchema(builtin, partsURL+e.Name(), e.Name(), path.Join("parts", e.Name()))
		}
		if err != nil {
			return err
		}
	}
	for name, p := range goParts {
		url := partsURL + name + ".json"
		set.docs[url] = map[string]any{"type": "object", "description": p.about}
		set.schemas[url] = &schema{
			Format: "", Ref: "", Properties: nil, Items: nil, part: nil, Description: p.about, Type: "object", Enum: nil,
			Required: nil,
		}
		set.parts[name] = &part{
			name: name, schema: set.schemas[url], pages: nil, words: nil, html: "", css: p.css, checks: nil, looks: nil,
			looksAt: nil, look: false, about: "", uses: nil,
		}
	}
	themes, err := fs.Sub(builtin, themesDir)
	if err != nil {
		return fmt.Errorf("read the parts themes: %w", err)
	}
	entries, err = fs.ReadDir(themes, ".")
	if err != nil {
		return fmt.Errorf("read the parts themes: %w", err)
	}
	for _, e := range entries {
		t := newPartsTheme()
		files, err := fs.Sub(themes, e.Name())
		if err == nil {
			err = set.readPartsTheme(files, path.Join("parts", themesDir, e.Name()), t)
		}
		if err != nil {
			return err
		}
		set.themes[e.Name()] = t
	}
	return nil
}

// partsUnder adds to names the parts the schemas under s refer to. No part holds itself: readOwn leaves
// out one that would.
func (s *schema) partsUnder(names *[]string) {
	if s == nil {
		return
	}
	for _, p := range s.Properties {
		if p.part != nil {
			*names = append(*names, p.part.name)
		}
		p.partsUnder(names)
	}
	if s.Items != nil && s.Items.part != nil {
		*names = append(*names, s.Items.part.name)
	}
	s.Items.partsUnder(names)
}

// partsNamedIn returns the names of the parts the data-floe-part and data-floe-look attributes of html
// name, in order.
func (set *partSet) partsNamedIn(html string) []string {
	return slices.Concat(set.namedBy(html, false), set.namedBy(html, true))
}

// namedBy returns the names of the parts of set the data-floe-look attributes of html name when look,
// else its data-floe-part attributes, in order.
func (set *partSet) namedBy(html string, look bool) []string {
	named := map[string]bool{}
	for _, n := range namedParts(html) {
		if n.look == look {
			named[n.name] = true
		}
	}
	var names []string
	for _, name := range slices.Sorted(maps.Keys(set.parts)) {
		if named[name] {
			names = append(names, name)
		}
	}
	return names
}

// namedPart is a name a data-floe-part or data-floe-look attribute gives, the attribute's whole value, and
// the offset where the value starts.
type namedPart struct {
	name  string
	value string
	look  bool // named by data-floe-look
	at    int
}

// several reports whether n is of a data-floe-part that names more than one part: an element is the box
// of one part, and takes the look of others by data-floe-look.
func (n namedPart) several() bool {
	return !n.look && strings.ContainsFunc(strings.TrimSpace(n.value), unicode.IsSpace)
}

// namedParts returns the names the data-floe-part and data-floe-look attributes of html give, as written:
// an element of a part names it (data-floe-part="note"), and one that takes a part's look without being
// its box names that (data-floe-look="panel"). A name that html puts together ({{.kind}}) is not one.
// LIMIT: the attributes are found as written, not as an HTML parser reads them (one in a script's string
// counts); parse the page if a page comes to write one so.
func namedParts(html string) []namedPart {
	found := partAttr.FindAllStringSubmatchIndex(html, -1)
	names := make([]namedPart, 0, len(found))
	for _, m := range found {
		value := html[m[4]:m[5]]
		if strings.Contains(value, "{{") {
			continue
		}
		value = strings.Trim(value, `"'`)
		look := strings.EqualFold(html[m[2]:m[3]], "look")
		for _, name := range strings.Fields(value) {
			names = append(names, namedPart{name: name, value: value, look: look, at: m[4]})
		}
	}
	return names
}

// partAttr finds the value of a data-floe-part or data-floe-look attribute: data-floe-look="table cards",
// '…' or x.
var partAttr = regexp.MustCompile(`(?i)\sdata-floe-(part|look)\s*=\s*("[^"]*"|'[^']*'|[^\s"'=<>` + "`" + `]+)`)

// readSchema reads the schema file of files, shown in its mistakes as shown, at url.
func (set *partSet) readSchema(files fs.FS, url, file, shown string) error {
	b, err := fs.ReadFile(files, file)
	if err != nil {
		return fmt.Errorf("read %s: %w", shown, err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("read %s: %w", shown, err)
	}
	s, err := decodeSchema(b)
	if err != nil {
		return fmt.Errorf("read %s: %w", shown, err)
	}
	// As jsonschema reads a $ref: cell.json is floe:///parts/cell.json.
	s.absolute(url)
	set.docs[url], set.schemas[url] = doc, s
	return nil
}

// readPart reads the part name from files, its directory, shown in its mistakes as dir, into set: a
// directory without schema.json and part.html is a look alone.
func (set *partSet) readPart(files fs.FS, name, dir string) error {
	look, err := isLook(files, name)
	if err != nil {
		return err
	}
	spec, err := readPartYAML(files, path.Join(dir, "part.yaml"), look)
	if err != nil {
		return err
	}
	if look {
		return set.readLook(files, name, dir, spec)
	}
	url := partsURL + name + ".json"
	if err = set.readSchema(files, url, "schema.json", path.Join(dir, "schema.json")); err != nil {
		return err
	}
	html, err := fs.ReadFile(files, "part.html")
	if err != nil {
		return fmt.Errorf("read the part %s: %w", name, err)
	}
	file := path.Join(dir, "words.yaml")
	w, err := readPartWords(files, file)
	if err != nil {
		return err
	}
	page, err := template.New(name).Option("missingkey=error").Funcs(funcs).Funcs(partFuncs).
		Funcs(template.FuncMap{"word": w.word(file, fallbackLang)}).Parse(string(html))
	if err != nil {
		return fmt.Errorf("parse the part %s: %w", name, err)
	}
	if err = w.checkWords(page, path.Join(dir, "part.html"), file); err != nil {
		return err
	}
	// One page per language of its words, word writing each of them: word "likely" is likely in words.yaml.
	pages := map[string]*template.Template{fallbackLang: page}
	for lang := range w {
		if lang == fallbackLang {
			continue
		}
		if pages[lang], err = page.Clone(); err != nil {
			return fmt.Errorf("parse the part %s: %w", name, err)
		}
		pages[lang].Funcs(template.FuncMap{"word": w.word(file, lang)})
	}
	css, err := readPartCSS(files, name, dir)
	if err != nil {
		return err
	}
	p := &part{
		name: name, schema: set.schemas[url], pages: pages, words: w, html: string(html), css: css, checks: nil,
		looks: spec.looks, looksAt: spec.looksAt, look: false, about: "", uses: nil,
	}
	if p.checks, err = readPartChecks(files, name, dir); err != nil {
		return err
	}
	set.parts[name] = p
	return nil
}

// isLook reports whether files, the directory of the part name, is a look alone: it holds neither
// schema.json nor part.html. One of them without the other is the mistake reading it reports.
func isLook(files fs.FS, name string) (bool, error) {
	var missing int
	for _, file := range []string{"schema.json", "part.html"} {
		switch _, err := fs.Stat(files, file); {
		case errors.Is(err, fs.ErrNotExist):
			missing++
		case err != nil:
			return false, fmt.Errorf("read the part %s: %w", name, err)
		}
	}
	return missing == 2, nil
}

// readLook reads the look alone name from files, its directory, shown in its mistakes as dir, into set.
func (set *partSet) readLook(files fs.FS, name, dir string, spec partSpec) error {
	// A look draws nothing: words and checks of one would be read for nothing.
	for _, file := range []string{"words.yaml", "checks.yaml"} {
		if _, err := fs.Stat(files, file); err == nil {
			return usererr.New("tmpl_look_files", path.Join(dir, file), name)
		}
	}
	css, err := readPartCSS(files, name, dir)
	if err != nil {
		return err
	}
	set.parts[name] = &part{
		name: name, schema: nil, pages: nil, words: nil, html: "", css: css, checks: nil, looks: spec.looks,
		looksAt: spec.looksAt, look: true, about: spec.about, uses: nil,
	}
	return nil
}

// readPartCSS reads the part.css of the part name in files, its directory, shown as dir.
func readPartCSS(files fs.FS, name, dir string) (string, error) {
	css, err := fs.ReadFile(files, "part.css")
	if err != nil {
		return "", fmt.Errorf("read the part %s: %w", name, err)
	}
	if err = refuseCSS(path.Join(dir, "part.css"), css, true); err != nil {
		return "", err
	}
	return string(css), nil
}

// readLooks reads the looks part.yaml, file, gives the key k, with the line of each.
func readLooks(file string, k, value ast.Node) ([]string, []int, error) {
	seq, ok := value.(*ast.SequenceNode)
	if !ok {
		return nil, nil, yamlpos.New(file, yamlpos.OfValue(k, value), "tmpl_part_yaml_looks")
	}
	var looks []string
	var lines []int
	for _, v := range seq.Values {
		name, text := v.(*ast.StringNode)
		if !text || !validName(name.Value) {
			return nil, nil, yamlpos.New(file, yamlpos.Of(v), "tmpl_part_yaml_looks")
		}
		looks, lines = append(looks, name.Value), append(lines, yamlpos.Of(v).Line)
	}
	return looks, lines, nil
}

// partSpec is what part.yaml says of a part: the looks it takes, each with its line, and, of a look
// alone, what it is.
type partSpec struct {
	looks   []string
	looksAt []int
	about   string
}

// readPartYAML reads file, the part.yaml of a part in files, its directory, look telling whether the part
// is a look alone; a part without one takes no look.
func readPartYAML(files fs.FS, file string, look bool) (partSpec, error) {
	var spec partSpec
	src, err := fs.ReadFile(files, "part.yaml")
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return spec, nil
	case err != nil:
		return spec, fmt.Errorf("read %s: %w", file, err)
	}
	body, err := yamlpos.Parse(file, src, "tmpl_part_yaml")
	if errors.Is(err, yamlpos.ErrEmpty) {
		return spec, nil
	}
	if err != nil {
		return spec, err //nolint:wrapcheck // a mistake at its line and column
	}
	if err = plain(file, body); err != nil {
		return spec, err
	}
	m, ok := body.(*ast.MappingNode)
	if !ok {
		return spec, yamlpos.New(file, yamlpos.Of(body), "tmpl_part_yaml")
	}
	for _, kv := range m.Values {
		k := yamlpos.KeyOf(kv)
		switch key := k.GetToken().Value; {
		case key == "looks":
			if spec.looks, spec.looksAt, err = readLooks(file, k, kv.Value); err != nil {
				return spec, err
			}
		case key == "description" && look:
			text, ok := kv.Value.(*ast.StringNode)
			if !ok {
				return spec, yamlpos.New(file, yamlpos.OfValue(k, kv.Value), "tmpl_part_yaml_description")
			}
			spec.about = text.Value
		case key == "description":
			// A part's description is its schema's, which floe template show tells beside its items.
			return spec, yamlpos.New(file, yamlpos.Of(k), "tmpl_part_yaml_described")
		default:
			return spec, yamlpos.New(file, yamlpos.Of(k), "tmpl_part_yaml_key", key)
		}
	}
	return spec, nil
}

// readPartChecks reads the checks.yaml of the part name in files, its directory, shown as dir, compiled
// once: the checks of a part run on each item it draws. A part without one has none.
func readPartChecks(files fs.FS, name, dir string) ([]relation, error) {
	src, err := fs.ReadFile(files, "checks.yaml")
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil
	case err != nil:
		return nil, fmt.Errorf("read the part %s: %w", name, err)
	}
	checks, err := readChecks(path.Join(dir, "checks.yaml"), src, true)
	if err != nil {
		return nil, err
	}
	for i, c := range checks {
		var e *yamlpos.Error
		if checks[i].prog, e = c.program(); e != nil {
			return nil, e
		}
	}
	return checks, nil
}

// readPartWords reads file, the words.yaml of a part in files, its directory; a part without one has the
// words of en alone, none, so that it has a page all the same.
func readPartWords(files fs.FS, file string) (words, error) {
	src, err := fs.ReadFile(files, "words.yaml")
	var w words
	switch {
	case err == nil:
		if w, err = readWords(file, src); err != nil {
			return nil, err
		}
	case !errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Errorf("read %s: %w", file, err)
	}
	if w == nil {
		w = words{fallbackLang: nil}
	}
	return w, nil
}

// resolve fills what s and the schemas under it refer to among floe's in set: the items, types and
// description of a part or a schema the parts share, after the description written beside the $ref.
func (s *schema) resolve(set *partSet) {
	if s == nil {
		return
	}
	for _, p := range s.Properties {
		p.resolve(set)
	}
	s.Items.resolve(set)
	target, ok := set.schemas[s.Ref]
	if !ok {
		return
	}
	name, _ := partName(s.Ref)
	s.part = set.parts[name]
	s.Description = joinDescriptions(s.Description, target.Description)
	if s.Properties == nil {
		s.Properties, s.Required = target.Properties, target.Required
	}
	if s.Items == nil {
		s.Items = target.Items
	}
	if s.Type == nil {
		s.Type = target.Type
	}
	if s.Enum == nil {
		s.Enum = target.Enum
	}
}

// joinDescriptions writes the description beside a $ref, a sentence ended if it is not, before that
// of what the $ref names.
func joinDescriptions(beside, named string) string {
	beside = strings.TrimSpace(beside)
	if beside == "" || named == "" {
		return beside + named
	}
	switch last, _ := utf8.DecodeLastRuneInString(beside); {
	case unicode.IsPunct(last):
	case unicode.In(last, unicode.Han, unicode.Hiragana, unicode.Katakana):
		beside += "。"
	default:
		beside += "."
	}
	if last, _ := utf8.DecodeLastRuneInString(beside); last > unicode.MaxASCII {
		return beside + named
	}
	return beside + " " + named
}

// partAt is an item of a data file drawn by a part, and the path of keys to it.
type partAt struct {
	itemAt

	part *part
}

// findParts adds to found the items v holds that s draws by a part, v being at the path at.
func (s *schema) findParts(v any, at []string, found *[]partAt) {
	if s == nil {
		return
	}
	// An item of a format of floe's is drawn whole: what it holds is not looked into. A part is drawn
	// after the parts it holds, which it draws as they were drawn.
	if slices.Contains(formats, s.Format) {
		return
	}
	if s.part != nil {
		defer func() { *found = append(*found, partAt{itemAt: itemAt{value: v, at: at}, part: s.part}) }()
	}
	switch v := v.(type) {
	case map[string]any:
		for k, item := range v {
			s.Properties[k].findParts(item, append(at[:len(at):len(at)], k), found)
		}
	case []any:
		for i, item := range v {
			s.Items.findParts(item, append(at[:len(at):len(at)], strconv.Itoa(i)), found)
		}
	}
}

// checkParts checks the items of d its schema s draws by parts against the checks of their parts, and
// draws them into d.parts, by the path of keys to each, reporting every failure at its line and column
// under root.
func checkParts(s *schema, d *Data, root ast.Node) error {
	var found []partAt
	s.findParts(d.Values, nil, &found)
	var problems yamlpos.Problems
	for _, f := range found {
		if f.part.pages == nil {
			continue // drawn in Go, by its own check
		}
		more, err := f.part.check(d.File, f.itemAt, root)
		if err != nil {
			return err
		}
		if more != nil {
			problems = append(problems, more...)
			continue
		}
		html, err := f.part.draw(f.itemAt, d)
		var bad *usererr.Error
		switch {
		case errors.As(err, &bad) && bad.Key == "tmpl_words_unknown":
			// A word the data names (word .level) that the part has not: told at the item, with the rest.
			problems = append(problems, itemMistake(d.File, root, f.at, false, bad))
			continue
		case err != nil:
			return err
		}
		d.drew(f.at, html)
	}
	if problems != nil {
		return problems.Sorted()
	}
	return nil
}

// partsNamed returns the items of d its schema s draws by the part name.
func partsNamed(s *schema, d *Data, name string) []itemAt {
	var found []partAt
	s.findParts(d.Values, nil, &found)
	var named []itemAt
	for _, f := range found {
		if f.part.name == name {
			named = append(named, f.itemAt)
		}
	}
	return named
}

// drew notes html, a part's drawing of the item at the path at.
func (d *Data) drew(at []string, html template.HTML) {
	if d.parts == nil {
		d.parts = map[string]template.HTML{}
	}
	d.parts[pathKey(at)] = html
}

// check runs the checks of p on the item f of the data file file, under root. An error is p's own
// mistake, and the problems are the data's.
func (p *part) check(file string, f itemAt, root ast.Node) (yamlpos.Problems, error) {
	var problems yamlpos.Problems
	var doc map[string]any
	if m, ok := f.value.(map[string]any); ok {
		doc = m
	}
	_, isList := f.value.([]any)
	for _, c := range p.checks {
		items := []itemAt{f}
		// A check without for checks the part itself, or each of its items when it is a list.
		if c.For != "" || isList {
			var steps []string
			if c.For != "" {
				steps = strings.Split(c.For, ".")
			}
			items = nil
			if !itemsAt(f.value, steps, f.at, &items) {
				return nil, yamlpos.New(c.file, c.at["for"], "tmpl_checks_not_list", c.For, file)
			}
		}
		more, err := c.run(file, doc, items, root)
		if err != nil {
			return nil, err
		}
		problems = append(problems, more...)
	}
	return problems, nil
}

// draw draws f, an item of d p checked, as HTML, its words in d's language, and the parts it holds as d
// drew them.
func (p *part) draw(f itemAt, d *Data) (template.HTML, error) {
	var b bytes.Buffer
	if err := p.pages[p.words.lang(d.Lang)].Execute(&b, p.schema.prepare(f.value, f.at, d)); err != nil {
		return "", fmt.Errorf("draw the part %s: %w", p.name, err)
	}
	return template.HTML(strings.TrimSpace(b.String())), nil //nolint:gosec // G203: drawn by html/template
}

// pageStyle returns the <style>s of the page, in cascade layers that decide which wins whatever the
// selectors: floe's base, under the template's own CSS (own: its page.css, then its theme), under the
// parts'. The parts' are the style of each part the page names, by its own element, a table of Markdown,
// or HTML that takes a part's look (<span class="good" data-floe-look="cell">), and of each look those
// take, before the parts that take it, which adjust it; then the parts theme theme: its base.css, which
// every page gets, as it holds its variables and the look of Markdown's quotes, and its style of each
// of those parts in the same order. Within the layer the more specific rule wins, else the later.
// An element that takes a look without being a part's box (a card of cards) has it a layer under the
// parts', floe.looks: the part it is in adjusts it whatever the selectors, as a part does the looks it
// takes, where the nearer scope would win otherwise.
// A part's style and its theme's reach the part's elements, and the box of a part it holds but not what
// that part holds; the template's reaches the boxes of the parts, so that it places them, but not what
// they hold.
func (set *partSet) pageStyle(page []byte, own [][]byte, theme string) (string, error) {
	themed := set.themes[theme]
	if themed == nil {
		return "", fmt.Errorf("draw with the parts theme %s: %w", theme, fs.ErrNotExist)
	}
	// The first sheet names the layers, which puts them in that order. Each file after it is a sheet of
	// its own, so that a mistake in it, a comment or a block left open, ends with it.
	var style strings.Builder
	style.WriteString("<style>\n@layer floe.base, floe.page, floe.looks, floe.parts;\n@layer floe.base {\n" + baseCSS +
		"}\n</style>\n")
	sheet := func(layer, css string) {
		if css != "" {
			style.WriteString("<style>\n@layer " + layer + " {\n" + css + "}\n</style>\n")
		}
	}
	var names, boxes, looks []string
	if bytes.Contains(page, []byte("data-floe-")) {
		if err := set.refuseBroken(page); err != nil {
			return "", err
		}
		boxes, looks = set.namedBy(string(page), false), set.namedBy(string(page), true)
		names = set.inOrder(set.withLooks(slices.Concat(boxes, looks)))
	}
	// Each style goes to the boxes that have its look, and to the other elements that take it, a layer under.
	styled := func(name, css string) {
		sheet("floe.parts", set.scoped(name, boxes, false, css))
		sheet("floe.looks", set.scoped(name, looks, true, css))
	}
	for _, name := range names {
		styled(name, set.parts[name].css)
	}
	sheet("floe.parts", themed.base)
	for _, name := range names {
		styled(name, themed.parts[name])
	}
	for _, css := range own {
		if len(css) != 0 {
			sheet("floe.page", "@scope (:root) to ([data-floe-part] > *) {\n"+string(css)+"}\n")
		}
	}
	return style.String(), nil
}

// refuseBroken refuses page when it names a part that cannot be used, by a table of Markdown or HTML of the
// data among others, which would leave it without its style.
func (set *partSet) refuseBroken(page []byte) error {
	for _, n := range namedParts(string(page)) {
		if set.parts[n.name] == nil && set.broken[n.name] != nil {
			return usererr.New("tmpl_part_broken", n.name, set.broken[n.name])
		}
	}
	return nil
}

// inOrder returns names, parts of set, each after the parts it uses that names holds, else in order.
func (set *partSet) inOrder(names []string) []string {
	given := map[string]bool{}
	for _, name := range names {
		given[name] = true
	}
	done := map[string]bool{}
	ordered := make([]string, 0, len(names))
	var visit func(name string)
	visit = func(name string) {
		done[name] = true
		for _, used := range set.parts[name].uses {
			if given[used] && !done[used] {
				visit(used)
			}
		}
		ordered = append(ordered, name)
	}
	for _, name := range names {
		if !done[name] {
			visit(name)
		}
	}
	return ordered
}

// scoped is css, the style of the part name or its theme's for it, reaching the elements that have its
// look, from their own on, short of what the parts in them hold: of named, the parts a page names, each
// that is name or takes its look, the boxes (data-floe-part="table"), or the elements that take it
// without being a box when look (data-floe-look="table"). It is "" when none of them has the look, or
// css is "".
func (set *partSet) scoped(name string, named []string, look bool, css string) string {
	var roots []string
	for _, n := range named {
		switch {
		case n != name && !slices.Contains(set.looksOf(n), name):
		case look:
			roots = append(roots, `[data-floe-look~="`+n+`"]`)
		default:
			roots = append(roots, `[data-floe-part="`+n+`"]`)
		}
	}
	if roots == nil || css == "" {
		return ""
	}
	return "@scope (" + strings.Join(roots, ", ") + ") to (:scope [data-floe-part] > *) {\n" + css + "}\n"
}

// looksOf returns the looks the part name takes, and those they take in turn, each once. No look takes
// itself: readOwn leaves out one that would.
func (set *partSet) looksOf(name string) []string {
	var found []string
	var follow func(at string)
	follow = func(at string) {
		p := set.parts[at]
		if p == nil {
			return
		}
		for _, look := range p.looks {
			if !slices.Contains(found, look) {
				found = append(found, look)
				follow(look)
			}
		}
	}
	follow(name)
	return found
}

// withLooks returns names, parts of set, with the looks they take, in the order of their names.
func (set *partSet) withLooks(names []string) []string {
	all := slices.Clone(names)
	for _, name := range names {
		all = append(all, set.looksOf(name)...)
	}
	slices.Sort(all)
	return slices.Compact(all)
}

// baseCSS is what every page starts from: text, lists, links, code and quotes, for the template's own
// elements and, where its CSS does not reach, those of the parts.
//
//go:embed base.css
var baseCSS string

// withStyles puts style into page before its </head>, else at its end. Where it goes decides nothing
// else: the <link>s of page.html (fonts, libraries) are out of the layers, and win over them all.
func withStyles(page []byte, style string) []byte {
	at := tagsOf(page).head
	if at < 0 {
		at = len(page)
	}
	return slices.Concat(page[:at], []byte(style), page[at:])
}

// tags are the offsets of the first </head>, <style> and <link> of a page, -1 for one it lacks.
type tags struct{ head, style, link int }

// tagsOf finds the tags of page. A tag in a comment, a script or a noscript is not one: a sandboxed page
// runs its scripts, so a noscript's text is never read as HTML.
// LIMIT: the tags are found as written, not as an HTML parser reads them (a <style in an attribute's
// value is taken for one). Parse the page if a template comes to write one so.
func tagsOf(page []byte) tags {
	found := tags{head: -1, style: -1, link: -1}
	for i := 0; i < len(page); {
		m := tagOrText.FindSubmatchIndex(page[i:])
		if m == nil {
			break
		}
		start, next := i+m[0], i+m[1]
		switch {
		case m[2] >= 0:
			// A comment, ended as an HTML parser ends it; one never closed runs to the end.
			next = len(page)
			if n := comment(page[start:]); n > 0 {
				next = start + n
			}
		case m[4] >= 0:
			// A script's or a noscript's text runs to its end tag.
			next = len(page)
			if closing := endTag[strings.ToLower(string(page[i+m[4]:i+m[5]]))].FindIndex(page[i+m[1]:]); closing != nil {
				next = i + m[1] + closing[1]
			}
		case m[6] >= 0:
			at := &found.link
			if strings.EqualFold(string(page[i+m[6]:i+m[7]]), "style") {
				at = &found.style
			}
			if *at < 0 {
				*at = start
			}
		case found.head < 0:
			found.head = start
		}
		i = next
	}
	return found
}

var (
	// tagOrText finds, in order, a comment, a script or a noscript, a style or a link, and </head>.
	tagOrText = regexp.MustCompile(`(?i)(<!--)|<(script|noscript)(?:[\s/][^>]*)?>|<(style|link)[\s/>]|</head\s*>`)
	endTag    = map[string]*regexp.Regexp{
		"script":   regexp.MustCompile(`(?i)</script\s*>`),
		"noscript": regexp.MustCompile(`(?i)</noscript\s*>`),
	}
)

// variables returns the CSS variables the style of the parts of set named, and of those they are drawn
// in the look of, takes from the theme, once each, in the order of the parts' names.
func (set *partSet) variables(names []string) []string {
	// The parts used are followed to those they use in turn: tabs holds a stack, which holds a diff, drawn
	// in the look of code.
	// One left out (a table of one's own that cannot be used, for Markdown) takes none.
	used := map[string]bool{}
	for next := names; len(next) > 0; {
		name := next[0]
		next = next[1:]
		if p := set.parts[name]; p != nil && !used[name] {
			used[name] = true
			next = append(next, p.uses...)
		}
	}
	var vars []string
	for _, name := range slices.Sorted(maps.Keys(used)) {
		vars = appendFound(vars, fallback, set.parts[name].css)
	}
	return vars
}

// appendFound appends to found what re's first group finds in s, each once.
func appendFound(found []string, re *regexp.Regexp, s string) []string {
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		if !slices.Contains(found, m[1]) {
			found = append(found, m[1])
		}
	}
	return found
}

// fallback finds the variables a style takes with a value of its own for a theme without them.
var fallback = regexp.MustCompile(`var\(\s*(--[\w-]+)\s*,`)
