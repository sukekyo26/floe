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
// and words.yaml) and per look alone (part.css and part.yaml), and the schemas the parts share (tone.json and
// any.json).
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

// rootPart returns the part of set the $ref at the root of the schema s names, and false for none: a
// schema declares its own items, and names a part under properties or items, which floe draws the item by. A
// look, which draws nothing, and a part that cannot be used are told as they are elsewhere, and a schema the
// parts share may be the root, its items the schema's own.
func (set *partSet) rootPart(s *shape) (string, bool) {
	if s == nil {
		return "", false
	}
	name, ok := partName(s.ref)
	if !ok {
		return "", false
	}
	p := set.parts[name]
	return name, p != nil && !p.isLook()
}

// pointsInto returns the part of floe's, or the schema the parts share that holds what floe draws, whole,
// that ref points into, and false when it does not: a ref into one would be checked as written but drawn as
// plain data.
func (set *partSet) pointsInto(ref string) (whole string, into bool) {
	whole, _, into = strings.Cut(ref, "#")
	name, ok := partName(whole)
	return whole, into && ok && (set.parts[name] != nil && !set.parts[name].isLook() || set.draws(set.sharedSchema(whole)))
}

// first returns the first of s and the shapes under it that ok takes, with its pointer under ptr, or nil.
func (s *shape) first(ptr string, ok func(*shape) bool) (string, *shape) {
	if s == nil {
		return "", nil
	}
	if ok(s) {
		return ptr, s
	}
	in := s.inside()
	for _, name := range slices.Sorted(maps.Keys(in.keys)) {
		if at, found := in.keys[name].first(ptr+"/properties/"+pointerKey.Replace(name), ok); found != nil {
			return at, found
		}
	}
	return in.each.first(ptr+"/items", ok)
}

// formatIn returns the first item of a format of floe's in the schema s, with its pointer under ptr, or
// nil, through a schema the parts share that s refers to too, whose items are drawn as s's own: its
// pointer goes on after the $ref's.
func (set *partSet) formatIn(s *shape, ptr string) (at string, item *shape) {
	at, item = s.first(ptr, func(x *shape) bool {
		_, held := set.formatIn(set.sharedSchema(x.ref), x.ref+"#")
		return slices.Contains(formats, x.format) || held != nil
	})
	if item == nil || slices.Contains(formats, item.format) {
		return at, item
	}
	under, item := set.formatIn(set.sharedSchema(item.ref), item.ref+"#")
	return at + " → " + under, item
}

// sharedSchema returns the shape of the schema the parts share that ref names, or nil.
func (set *partSet) sharedSchema(ref string) *shape {
	if !set.shares[ref] {
		return nil
	}
	return set.shapes[ref]
}

// draws reports whether floe draws an item of s, once set is read: it holds a part or an item of a format
// of floe's, in a schema the parts share that it refers to too.
func (set *partSet) draws(s *shape) bool {
	_, found := s.first("#", func(x *shape) bool {
		return x.part != nil || slices.Contains(formats, x.format) || set.draws(set.sharedSchema(x.ref))
	})
	return found != nil
}

// part is a part of floe's: a template's schema refers to it, and floe checks it, draws it and puts its
// style into the page. A look alone (panel) is a style other parts and HTML take, which draws nothing.
type part struct {
	name   string
	shape  *shape                        // its schema's; nil for a look alone
	pages  map[string]*template.Template // by the language of its words; nil for one floe draws in Go (goParts)
	words  words
	html   string // the template of pages, as written
	css    string
	checks []relation
	looks  []takenLook // the looks it takes, as part.yaml names them, each before it
	taken  []string    // those and the looks they take in turn, each once, once all are read
	uses   []string    // the parts its HTML names and its schema holds, itself among them
	about  string      // a look's description, from part.yaml; a part's is its schema's
	// where one of one's own was found, as its mistakes name it, "" for floe's, and whether it takes the
	// place of floe's of its name
	dir      string
	replaces bool
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
// themes.
type partSet struct {
	parts  map[string]*part
	docs   map[string]any          // as read, for jsonschema
	shapes map[string]*shape       // as floe reads them
	notes  map[string]*schemaNotes // what floe checks of them once all are read
	shares map[string]bool         // the URLs of the schemas the parts share, floe's alone
	themes map[string]*theme       // by name
	broken map[string]error        // the parts of one's own that cannot be used, by name, and why
	// the themes of one's own that cannot be used, by name, and why
	brokenThemes map[string]error
	// floe's own default palette, under every page's, whatever one's own themes change
	palette string
	// the directories of one's own it read, those of parts and themes among them, by their real paths
	dirs []string
	// what to warn of: what of one's own goes into no page, and the colours of the parts and themes
	// written where the palette is to give them
	strays []*usererr.Error
}

// builtinParts reads floe's parts once.
var builtinParts = sync.OnceValues(func() (*partSet, error) {
	return readParts(ownDirs{parts: "", themes: ""}, nil, staged{name: "", dir: ""})
})

// readParts reads floe's parts and themes and, over them, those of one's own in own, the parts' holding
// entries, and stage: a part of the name of one of floe's takes its place whole. One of one's own that
// cannot be used is left out, its mistake kept for a template that uses it.
func readParts(own ownDirs, entries []os.DirEntry, stage staged) (*partSet, error) {
	set := &partSet{
		parts: map[string]*part{}, docs: map[string]any{}, shapes: map[string]*shape{}, notes: map[string]*schemaNotes{},
		shares: map[string]bool{}, themes: map[string]*theme{},
		broken: map[string]error{}, brokenThemes: map[string]error{}, palette: "", dirs: nil, strays: nil,
	}
	if err := set.readBuiltin(); err != nil {
		return nil, err
	}
	for _, dir := range []string{own.parts, own.themes} {
		set.keepRoot(dir)
	}
	if own.parts != "" {
		set.readOwn(own.parts, entries, stage)
	}
	if own.themes != "" {
		if err := set.readOwnThemes(own.themes); err != nil {
			return nil, err
		}
	}
	for _, s := range set.shapes {
		s.bind(set)
	}
	for _, p := range set.parts {
		p.uses = set.partsNamedIn(p.html)
		p.shape.partsUnder(&p.uses)
		slices.Sort(p.uses)
		p.uses = slices.Compact(p.uses)
	}
	for name, p := range set.parts {
		p.taken = set.looksOf(name)
	}
	return set, nil
}

// readBuiltin reads floe's parts, those it draws in Go too, the schemas they share and the themes into
// set.
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
		case e.IsDir():
			var files fs.FS
			if files, err = fs.Sub(builtin, e.Name()); err == nil {
				err = set.readPart(files, e.Name(), path.Join("parts", e.Name()))
			}
		default:
			// A schema the parts share.
			set.shares[partsURL+e.Name()] = true
			err = set.readSchema(builtin, partsURL+e.Name(), e.Name(), path.Join("parts", e.Name()), "")
		}
		if err != nil {
			return err
		}
	}
	for name, p := range goParts {
		url := partsURL + name + ".json"
		set.docs[url] = map[string]any{"type": "object", "description": p.about}
		set.shapes[url] = &shape{
			keys: nil, each: nil, ref: "", format: "", named: nil, part: nil, about: p.about, typ: "object", enum: nil,
			required: nil,
		}
		set.parts[name] = &part{
			name: name, shape: set.shapes[url], pages: nil, words: nil, html: "", css: p.css, checks: nil,
			looks: nil, taken: nil, uses: nil, about: "", dir: "", replaces: false,
		}
	}
	// Once the parts are read, whose names the themes' files take.
	entries, err = fs.ReadDir(themeFiles, floeThemesDir)
	if err != nil {
		return fmt.Errorf("read the themes: %w", err)
	}
	for _, e := range entries {
		t := newTheme()
		at := path.Join(floeThemesDir, e.Name())
		files, err := fs.Sub(themeFiles, at)
		if err == nil {
			err = set.readTheme(files, at, t)
		}
		if err != nil {
			return err
		}
		set.themes[e.Name()] = t
	}
	set.palette = set.themes[DefaultTheme].palettes[0]
	return nil
}

// partsUnder adds to names the parts the shapes under s are drawn by, and those the parts hold in turn. No
// part holds itself: readOwn leaves out one that would.
func (s *shape) partsUnder(names *[]string) {
	if s == nil {
		return
	}
	in := s.inside()
	for _, k := range in.keys {
		if k != nil && k.part != nil {
			*names = append(*names, k.part.name)
		}
		k.partsUnder(names)
	}
	if in.each != nil && in.each.part != nil {
		*names = append(*names, in.each.part.name)
	}
	in.each.partsUnder(names)
}

// partsNamedIn returns the names of the parts the data-floe-part and data-floe-look attributes of html
// name, in order.
func (set *partSet) partsNamedIn(html string) []string {
	return set.namedBy(namedParts(html), nil)
}

// namedBy returns the names of the parts of set that named gives, of those keep keeps, every one for nil,
// in order.
func (set *partSet) namedBy(named []namedPart, keep func(namedPart) bool) []string {
	by := map[string]bool{}
	for _, n := range named {
		if keep == nil || keep(n) {
			by[n.name] = true
		}
	}
	var names []string
	for _, name := range slices.Sorted(maps.Keys(set.parts)) {
		if by[name] {
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
	text  bool // no attribute of a tag: in text, a comment or a script's text, where it brings the style alone
	at    int
}

// notOne reports whether n is of a data-floe-part that is not the name of one part as written, which the
// scope of the part's style, [data-floe-part="name"], would not reach: an element is the box of one
// part, and takes the look of others by data-floe-look.
func (n namedPart) notOne() bool {
	return !n.look && n.value != n.name
}

// namedParts returns the names the data-floe-part and data-floe-look attributes of html give, as written:
// an element of a part names it (data-floe-part="note"), and one that takes a part's look without being
// its box names that (data-floe-look="panel"). A name that html puts together ({{.kind}}) is not one. A
// name written so elsewhere than in a tag's attribute, in text, a comment or a script that writes one, is
// text: it brings the part's style alone.
func namedParts(html string) []namedPart {
	if !floeAttr.MatchString(html) {
		return nil
	}
	return namedIn(html, tagsIn(html, true))
}

// namedIn returns the names namedParts returns of html, whose tags are found, in order.
func namedIn(html string, found []tag) []namedPart {
	var names []namedPart
	values := map[int]bool{} // where the values of the attributes start
	for _, t := range found {
		for _, a := range t.attrs {
			look := a.name == "data-floe-look"
			if t.end || a.valueAt < 0 || !look && a.name != "data-floe-part" {
				continue
			}
			values[a.valueAt] = true
			names = append(names, namesOf(a.value, look, false, a.valueAt)...)
		}
	}
	for _, m := range partAttr.FindAllStringSubmatchIndex(html, -1) {
		if !values[m[4]] {
			look := strings.EqualFold(html[m[2]:m[3]], "look")
			names = append(names, namesOf(strings.Trim(html[m[4]:m[5]], `"'`), look, true, m[4])...)
		}
	}
	slices.SortStableFunc(names, func(x, y namedPart) int { return x.at - y.at })
	return names
}

// namesOf returns the names value, of a data-floe-look when look, else of a data-floe-part, at at, gives,
// in text when text; none when a template puts it together ({{.kind}}).
func namesOf(value string, look, text bool, at int) []namedPart {
	if strings.Contains(value, "{{") {
		return nil
	}
	fields := strings.Fields(value)
	names := make([]namedPart, 0, len(fields))
	for _, name := range fields {
		names = append(names, namedPart{name: name, value: value, look: look, text: text, at: at})
	}
	return names
}

// floeAttr finds a data-floe-part or a data-floe-look, as an attribute or in text.
var floeAttr = regexp.MustCompile(`(?i)data-floe-(part|look)`)

// partAttr finds the value of a data-floe-part or data-floe-look written as an attribute, in text too:
// data-floe-look="table cards", '…' or x.
var partAttr = regexp.MustCompile(`(?i)\sdata-floe-(part|look)\s*=\s*("[^"]*"|'[^']*'|[^\s"'=<>` + "`" + `]+)`)

// readSchema reads the schema file of files, shown in its mistakes as shown, at url, of the part part, ""
// for a schema the parts share, and refuses the first mistake it notes: those of its $refs wait for every
// part to be read (checkHeld).
func (set *partSet) readSchema(files fs.FS, url, file, shown, part string) error {
	b, err := fs.ReadFile(files, file)
	if err != nil {
		return fmt.Errorf("read %s: %w", shown, err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("read %s: %w", shown, err)
	}
	// As jsonschema reads a $ref: cell.json is floe:///parts/cell.json.
	s, notes, err := readShape(doc, url, shown, part)
	if err != nil {
		return fmt.Errorf("read %s: %w", shown, err)
	}
	if err = checkDraft(doc); err != nil {
		return usererr.New("tmpl_item", shown, err)
	}
	if m := notes.first(); m != nil {
		return m
	}
	set.docs[url], set.shapes[url], set.notes[url] = doc, s, notes
	return nil
}

// readPart reads the part name from files, its directory, shown in its mistakes as dir, into set: a
// directory without schema.json and part.html is a look alone.
func (set *partSet) readPart(files fs.FS, name, dir string) error {
	look, err := isLookDir(files, name, dir)
	if err != nil {
		return err
	}
	yaml, err := readPartYAML(files, path.Join(dir, "part.yaml"), look)
	if err != nil {
		return err
	}
	if look {
		return set.readLook(files, name, dir, yaml)
	}
	url := partsURL + name + ".json"
	if err = set.readSchema(files, url, "schema.json", path.Join(dir, "schema.json"), name); err != nil {
		return err
	}
	html, err := fs.ReadFile(files, "part.html")
	if err != nil {
		return fmt.Errorf("read the part %s: %w", name, err)
	}
	file := path.Join(dir, "words.yaml")
	w, err := readWordsIn(files, file)
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
	css, warnings, err := readPartCSS(files, name, dir)
	if err != nil {
		return err
	}
	p := &part{
		name: name, shape: set.shapes[url], pages: pages, words: w, html: string(html), css: css, checks: nil,
		looks: yaml.looks, taken: nil, uses: nil, about: "", dir: "", replaces: false,
	}
	if p.checks, err = readPartChecks(files, name, dir); err != nil {
		return err
	}
	set.parts[name] = p
	set.strays = append(set.strays, warnings...)
	return nil
}

// isLookDir reports whether files, the directory of the part name, shown as dir, is a look alone: it holds
// part.css alone of schema.json, part.html and part.css. A part holds the three.
func isLookDir(files fs.FS, name, dir string) (bool, error) {
	var missing []string
	for _, file := range []string{"schema.json", "part.html", "part.css"} {
		there, err := exists(files, name, file)
		if err != nil {
			return false, err
		}
		if !there {
			missing = append(missing, file)
		}
	}
	switch {
	case missing == nil:
		return false, nil
	case slices.Equal(missing, []string{"schema.json", "part.html"}):
		return true, nil
	}
	return false, usererr.New("tmpl_part_files", dir)
}

// exists reports whether files, the directory of the part name, holds file.
func exists(files fs.FS, name, file string) (bool, error) {
	switch _, err := fs.Stat(files, file); {
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("read the part %s: %w", name, err)
	}
	return true, nil
}

// readLook reads the look alone name from files, its directory, shown in its mistakes as dir, into set.
func (set *partSet) readLook(files fs.FS, name, dir string, yaml partYAML) error {
	// A look draws nothing: words and checks of one would be read for nothing.
	for _, file := range []string{"words.yaml", "checks.yaml"} {
		there, err := exists(files, name, file)
		if err != nil {
			return err
		}
		if there {
			return usererr.New("tmpl_look_files", path.Join(dir, file), name)
		}
	}
	css, warnings, err := readPartCSS(files, name, dir)
	if err != nil {
		return err
	}
	set.strays = append(set.strays, warnings...)
	set.parts[name] = &part{
		name: name, shape: nil, pages: nil, words: nil, html: "", css: css, checks: nil, looks: yaml.looks,
		taken: nil, uses: nil, about: yaml.about, dir: "", replaces: false,
	}
	return nil
}

// readPartCSS reads the part.css of the part name in files, its directory, shown as dir, and returns the
// warnings of the colours it writes as values, to be told once the part is read whole.
func readPartCSS(files fs.FS, name, dir string) (string, []*usererr.Error, error) {
	css, err := fs.ReadFile(files, "part.css")
	if err != nil {
		return "", nil, fmt.Errorf("read the part %s: %w", name, err)
	}
	file, sheet := path.Join(dir, "part.css"), readCSS(css)
	if err = sheet.refuse(file, true); err != nil {
		return "", nil, err
	}
	return string(css), colourWarnings(file, sheet, true), nil
}

// readLooks reads the looks part.yaml, file, gives the key k.
func readLooks(file string, k, value ast.Node) ([]takenLook, error) {
	seq, ok := value.(*ast.SequenceNode)
	if !ok {
		return nil, yamlpos.New(file, yamlpos.OfValue(k, value), "tmpl_part_yaml_looks")
	}
	looks := make([]takenLook, 0, len(seq.Values))
	for _, v := range seq.Values {
		// As written: YAML reads a name of digits (404) as a number.
		_, scalar := v.(ast.ScalarNode)
		name := v.GetToken().Value
		if !scalar || !validName(name) {
			return nil, yamlpos.New(file, yamlpos.Of(v), "tmpl_part_yaml_looks")
		}
		looks = append(looks, takenLook{name: name, line: yamlpos.Of(v).Line})
	}
	return looks, nil
}

// isLook reports whether p is a look alone: a part.css and a part.yaml, no schema.json nor part.html.
func (p *part) isLook() bool {
	return p.shape == nil
}

// lookNames returns the names of the looks p takes, in order.
func (p *part) lookNames() []string {
	names := make([]string, len(p.looks))
	for i, look := range p.looks {
		names[i] = look.name
	}
	return names
}

// takenLook is a look a part takes, and the line of part.yaml that names it.
type takenLook struct {
	name string
	line int
}

// partYAML is what a part's part.yaml gives: the looks it takes, and a look's description.
type partYAML struct {
	looks []takenLook
	about string
}

// readPartYAML reads file, the part.yaml of a part in files, its directory, a look alone when look; a part
// without one takes no look. A part's description is its schema's: part.yaml gives a look's alone.
func readPartYAML(files fs.FS, file string, look bool) (partYAML, error) {
	var yaml partYAML
	src, err := fs.ReadFile(files, "part.yaml")
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return yaml, nil
	case err != nil:
		return yaml, fmt.Errorf("read %s: %w", file, err)
	}
	body, err := yamlpos.Parse(file, src, "tmpl_part_yaml_documents")
	if errors.Is(err, yamlpos.ErrEmpty) {
		return yaml, nil
	}
	if err != nil {
		return yaml, err //nolint:wrapcheck // a mistake at its line and column
	}
	if err = plain(file, body); err != nil {
		return yaml, err
	}
	m, ok := body.(*ast.MappingNode)
	if !ok {
		return yaml, yamlpos.New(file, yamlpos.Of(body), "tmpl_part_yaml")
	}
	for _, kv := range m.Values {
		k := yamlpos.KeyOf(kv)
		switch key := k.GetToken().Value; {
		case key == "looks":
			yaml.looks, err = readLooks(file, k, kv.Value)
		case key == "description" && !look:
			return yaml, yamlpos.New(file, yamlpos.Of(k), "tmpl_part_yaml_description")
		case key == "description":
			yaml.about, err = readAbout(file, k, kv.Value)
		default:
			return yaml, yamlpos.New(file, yamlpos.Of(k), "tmpl_part_yaml_key", key)
		}
		if err != nil {
			return yaml, err
		}
	}
	return yaml, nil
}

// readAbout reads the description of a look part.yaml, file, gives the key k.
func readAbout(file string, k, value ast.Node) (string, error) {
	switch text := value.(type) {
	case *ast.StringNode:
		return text.Value, nil
	case *ast.LiteralNode: // | or >: told on a line, as a part's description is
		return text.Value.Value, nil
	}
	return "", yamlpos.New(file, yamlpos.OfValue(k, value), "tmpl_part_yaml_about")
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

// readWordsIn reads file, the words.yaml of a part or a template in files, its directory; one without it
// has the words of en alone, none, so that it has a page all the same.
func readWordsIn(files fs.FS, file string) (words, error) {
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

// checkParts checks the items of d of whole floe draws by parts against the checks of their parts, and
// draws them into d.parts, by the path of keys to each, those a part holds before it, reporting every failure at its
// line and column under root.
func checkParts(whole drawnItems, d *Data, root ast.Node) error {
	var problems yamlpos.Problems
	for _, f := range whole {
		p := f.part
		if p == nil || p.pages == nil {
			continue // a format of floe's, or a part drawn in Go, by its own check
		}
		more, err := p.check(d.File, f.itemAt, root)
		if err != nil {
			return err
		}
		if more != nil {
			problems = append(problems, more...)
			continue
		}
		html, err := p.draw(f.itemAt, d)
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
	if err := p.pages[p.words.lang(d.Lang)].Execute(&b, p.shape.prepare(f.value, f.at, d)); err != nil {
		return "", fmt.Errorf("draw the part %s: %w", p.name, err)
	}
	return template.HTML(strings.TrimSpace(b.String())), nil //nolint:gosec // G203: drawn by html/template
}

// pageStyle returns the <style>s of the page, in cascade layers that decide which wins whatever the
// selectors: the palette, floe's default one then the theme theme's and one's own of its name, under
// floe's base, under the template's own CSS (own, its page.css), which may set the palette's variables over
// it, under the parts', in the theme's shape. The parts' are the style of each part the page names, by its own
// element, a table of Markdown, or HTML that names a part (<span class="good" data-floe-part="cell">) or
// takes its look without being its box (data-floe-look="panel"), and of each look those take, before the
// parts that take it, which adjust it; then the theme's: its base.css, which every page gets, as it
// holds the look of Markdown's quotes, and its style of each of those parts in the same order. Within the
// layer the more specific rule wins, else the later.
// An element that takes a look without being a part's box (a card of cards) has it a layer under the
// template's and the parts', floe.looks: the part it is in, or the template it is the template's own of,
// adjusts it whatever the selectors, as a part does the looks it takes, where the nearer scope would win
// otherwise.
// LIMIT: such elements share the layer, so the look of one held in another wins over the outer's rules for
// it by the nearer scope. Make the outer a part's box (data-floe-part) if HTML comes to nest them so.
// A part's style and its theme's reach the part's elements, and the box of a part it holds but not what
// that part holds; the template's reaches the boxes of the parts, so that it places them, but not what
// they hold.
func (set *partSet) pageStyle(page string, found []tag, own []byte, theme string) (string, error) {
	themed := set.themes[theme]
	if themed == nil {
		return "", fmt.Errorf("draw with the theme %s: %w", theme, fs.ErrNotExist)
	}
	// The first sheet names the layers, which puts them in that order. Each file after it is a sheet of
	// its own, so that a mistake in it, a comment or a block left open, ends with it.
	var style strings.Builder
	// No </style in a CSS, in a comment or a string too, closes its <style> and leaves what follows to be
	// read as the page's HTML.
	write := func(body string) {
		style.WriteString("<style>\n" + styleEnd.ReplaceAllString(body, `<\/$1`) + "</style>\n")
	}
	write("@layer floe.palette, floe.base, floe.looks, floe.page, floe.parts;\n@layer floe.base {\n" + baseCSS + "}\n")
	sheet := func(layer, css string) {
		if css != "" {
			write("@layer " + layer + " {\n" + css + "}\n")
		}
	}
	// floe's own default palette defines every variable; the theme's after it, then one's own of its name,
	// default of one's own too, change those they write.
	sheet("floe.palette", set.palette)
	for i, palette := range themed.palettes {
		if theme != DefaultTheme || i > 0 {
			sheet("floe.palette", palette)
		}
	}
	named := namedIn(page, found)
	if bad := set.misnamedIn(named); bad != nil {
		return "", bad
	}
	boxes := set.namedBy(named, func(n namedPart) bool { return !n.look })
	looks := set.namedBy(named, func(n namedPart) bool { return n.look })
	names := set.inOrder(slices.Concat(boxes, looks))
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
	if len(own) != 0 {
		sheet("floe.page", "@scope (:root) to ([data-floe-part] > *) {\n"+string(own)+"}\n")
	}
	return style.String(), nil
}

// styleEnd finds </style, of either case, which HTML reads as the end of a <style>. Written <\/style it is
// the same CSS in a string, a comment or a url(): the / is escaped, and no comment starts at a / that a
// letter follows.
// LIMIT: elsewhere, as a variable's value (--x: </style>), the value keeps the \; written so, it closed the
// <style> anyway.
var styleEnd = regexp.MustCompile(`(?i)</(style)`)

// withStyles puts style into page before its </head>, false for a page without one: there, in the head,
// no element the page leaves open, an <svg> of the data's HTML among them, takes the styles in. Where it
// goes decides nothing else: the <link>s of page.html (fonts, libraries) are out of the layers, and win
// over them all.
func withStyles(page []byte, found []tag, style string) ([]byte, bool) {
	at := tagsOf(found).head
	if at < 0 {
		return nil, false
	}
	return slices.Concat(page[:at], []byte(style), page[at:]), true
}

// misnamedIn returns the mistake of the first name of named, the names of a page or of HTML of the data,
// that the page is refused for, or nil: a part that cannot be used, by a table of Markdown or HTML of the
// data among others, a data-floe-part not of one part's name, or one that names a look alone, which has
// no box. The elements would be left without their style.
func (set *partSet) misnamedIn(named []namedPart) *usererr.Error {
	for _, n := range named {
		switch set.misnamed(n) {
		case nameBroken:
			return usererr.New("tmpl_part_broken", n.name, set.broken[n.name])
		case nameNotOne:
			return usererr.New("tmpl_page_several", n.value)
		case nameLookBox:
			return usererr.New("tmpl_page_look_box", n.name)
		case nameRight, nameUnknown:
			// A name of no part draws nothing; the check of a template's page.html tells it.
		}
	}
	return nil
}

// inOrder returns names, parts of set, with the looks they take, each after its looks and the parts it
// uses that names holds, else in order.
func (set *partSet) inOrder(names []string) []string {
	names = slices.Compact(slices.Sorted(slices.Values(names)))
	given := map[string]bool{}
	for _, name := range names {
		given[name] = true
	}
	done := map[string]bool{}
	ordered := make([]string, 0, len(names))
	var visit func(name string)
	visit = func(name string) {
		done[name] = true
		p := set.parts[name]
		for _, look := range p.lookNames() {
			if !done[look] {
				visit(look)
			}
		}
		for _, used := range p.uses {
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
		case n != name && !slices.Contains(set.parts[n].taken, name):
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
		for _, look := range p.lookNames() {
			if !slices.Contains(found, look) {
				found = append(found, look)
				follow(look)
			}
		}
	}
	follow(name)
	return found
}

// baseCSS is what every page starts from: its ground, colour and face, and text, lists, links, code and
// quotes, for the template's own elements and, where its CSS does not reach, those of the parts.
//
//go:embed base.css
var baseCSS string
