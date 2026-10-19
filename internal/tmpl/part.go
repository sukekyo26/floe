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
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// partFiles holds a directory per part of floe's (schema.json, part.html, part.css, checks.yaml and
// words.yaml), the schemas the parts share (tone.json), and the parts themes (themes/).
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
	return whole, into && ok && set.parts[name] != nil
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
// style into the page.
type part struct {
	name   string
	schema *schema
	pages  map[string]*template.Template // by the language of its words; nil for one floe draws in Go (goParts)
	words  words
	html   string // the template of pages, as written
	css    string
	checks []relation
	uses   []string // the parts its HTML names and its schema holds, itself among them
}

// goParts are the parts floe draws in Go, each a description: their schema takes an object, whose
// shape floe checks where it draws it.
var goParts = map[string]string{
	"diagram": "A diagram: a spec as floe diagram render takes it (id, width, height, title, parts); " +
		"floe diagram example shows one of each kind",
	"script": "Something that moves: js, a function (el, data) { … } the page calls with an empty element in its " +
		"place and data, any value. It runs in the page's sandbox, where fetch, XHR, images from elsewhere and " +
		"forms are blocked",
}

// isGoPart reports whether name is a part floe draws in Go.
func isGoPart(name string) bool {
	_, ok := goParts[name]
	return ok
}

// partSet is floe's parts, and every schema under parts/, by URL: the parts' and those they share.
type partSet struct {
	parts   map[string]*part
	docs    map[string]any     // as read, for jsonschema
	schemas map[string]*schema // as floe reads them
}

// builtinParts reads floe's parts once.
var builtinParts = sync.OnceValues(readParts)

func readParts() (*partSet, error) {
	set := &partSet{parts: map[string]*part{}, docs: map[string]any{}, schemas: map[string]*schema{}}
	entries, err := fs.ReadDir(partFiles, "parts")
	if err != nil {
		return nil, fmt.Errorf("read the parts: %w", err)
	}
	for _, e := range entries {
		if e.Name() == themesDir {
			continue
		}
		file := path.Join("parts", e.Name())
		url := partsURL + e.Name()
		if e.IsDir() {
			file = path.Join(file, "schema.json")
			url += ".json"
		}
		if err = set.readSchema(url, file); err != nil {
			return nil, err
		}
		if e.IsDir() {
			p, err := readPart(e.Name(), set.schemas[url])
			if err != nil {
				return nil, err
			}
			set.parts[e.Name()] = p
		}
	}
	for name, about := range goParts {
		url := partsURL + name + ".json"
		set.docs[url] = map[string]any{"type": "object", "description": about}
		set.schemas[url] = &schema{
			Format: "", Ref: "", Properties: nil, Items: nil, part: nil, Description: about, Type: "object", Enum: nil,
			Required: nil,
		}
		set.parts[name] = &part{
			name: name, schema: set.schemas[url], pages: nil, words: nil, html: "", css: "", checks: nil, uses: nil,
		}
	}
	for _, s := range set.schemas {
		s.resolve(set)
	}
	for _, p := range set.parts {
		p.uses = set.partsNamedIn(p.html)
		p.schema.partsUnder(&p.uses)
		slices.Sort(p.uses)
		p.uses = slices.Compact(p.uses)
	}
	return set, nil
}

// partsUnder adds to names the parts the schemas under s refer to.
// LIMIT: a part that held itself, at any depth, would walk forever; keep the parts walked if one comes to.
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

// partsNamedIn returns the names of the parts the data-floe-part attributes of html name, in order.
func (set *partSet) partsNamedIn(html string) []string {
	named := map[string]bool{}
	for _, name := range namedParts(html) {
		named[name] = true
	}
	var names []string
	for _, name := range slices.Sorted(maps.Keys(set.parts)) {
		if named[name] {
			names = append(names, name)
		}
	}
	return names
}

// namedParts returns the names the data-floe-part attributes of html give, as written: an element of a
// part names it, after the parts it is drawn in the look of (data-floe-part="table compare").
// LIMIT: the attributes are found as written, not as an HTML parser reads them (one in a script's string
// counts); parse the page if a page comes to write one so.
func namedParts(html string) []string {
	found := partAttr.FindAllStringSubmatch(html, -1)
	names := make([]string, 0, len(found))
	for _, m := range found {
		names = append(names, strings.Fields(strings.Trim(m[1], `"'`))...)
	}
	return names
}

// partAttr finds the value of a data-floe-part attribute: data-floe-part="table compare", '…' or x.
var partAttr = regexp.MustCompile(`(?i)\sdata-floe-part\s*=\s*("[^"]*"|'[^']*'|[^\s"'=<>` + "`" + `]+)`)

// readSchema reads the schema file of floe's at url.
func (set *partSet) readSchema(url, file string) error {
	b, err := partFiles.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read %s: %w", file, err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("read %s: %w", file, err)
	}
	s, err := decodeSchema(b)
	if err != nil {
		return fmt.Errorf("read %s: %w", file, err)
	}
	set.docs[url], set.schemas[url] = doc, s
	return nil
}

// readPart reads the part name, whose schema is s.
func readPart(name string, s *schema) (*part, error) {
	dir := path.Join("parts", name)
	html, err := partFiles.ReadFile(path.Join(dir, "part.html"))
	if err != nil {
		return nil, fmt.Errorf("read the part %s: %w", name, err)
	}
	file := path.Join(dir, "words.yaml")
	w, err := readPartWords(file)
	if err != nil {
		return nil, err
	}
	page, err := template.New(name).Option("missingkey=error").Funcs(funcs).Funcs(partFuncs).
		Funcs(template.FuncMap{"word": w.word(file, fallbackLang)}).Parse(string(html))
	if err != nil {
		return nil, fmt.Errorf("parse the part %s: %w", name, err)
	}
	if err = w.checkWords(page, path.Join(dir, "part.html"), file); err != nil {
		return nil, err
	}
	// One page per language of its words, word writing each of them: word "likely" is likely in words.yaml.
	pages := map[string]*template.Template{fallbackLang: page}
	for lang := range w {
		if lang == fallbackLang {
			continue
		}
		if pages[lang], err = page.Clone(); err != nil {
			return nil, fmt.Errorf("parse the part %s: %w", name, err)
		}
		pages[lang].Funcs(template.FuncMap{"word": w.word(file, lang)})
	}
	css, err := partFiles.ReadFile(path.Join(dir, "part.css"))
	if err != nil {
		return nil, fmt.Errorf("read the part %s: %w", name, err)
	}
	p := &part{
		name: name, schema: s, pages: pages, words: w, html: string(html), css: string(css), checks: nil, uses: nil,
	}
	src, err := partFiles.ReadFile(path.Join(dir, "checks.yaml"))
	switch {
	case err == nil:
		if p.checks, err = readChecks(path.Join(dir, "checks.yaml"), src, true); err != nil {
			return nil, err
		}
		// Compiled once: the checks of a part run on each item it draws.
		for i, c := range p.checks {
			var e *yamlpos.Error
			if p.checks[i].prog, e = c.program(); e != nil {
				return nil, e
			}
		}
	case !errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Errorf("read the part %s: %w", name, err)
	}
	return p, nil
}

// readPartWords reads file, the words.yaml of a part; a part without one has the words of en alone, none,
// so that it has a page all the same.
func readPartWords(file string) (words, error) {
	src, err := partFiles.ReadFile(file)
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
		if err != nil {
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

// pageStyle returns the one <style> of the page, in cascade layers that decide which wins whatever the
// selectors: floe's base, under the template's own CSS (own: its page.css, then its theme), under the
// parts'. The parts' are the style of the parts the page names, in the order of their names, the
// diagrams' when it has diagrams, then the parts theme theme, which every page gets, as it styles
// Markdown's quotes too. A part is named by its own element, a table of Markdown, or HTML that takes a
// part's look (<span class="floe-pill good" data-floe-part="cell">).
// The template's style reaches the boxes of the parts, so that it places them, but not what they hold.
// LIMIT: the parts' style and the parts theme are not scoped: a part's rules reach the parts it holds,
// and its theme's rule for a part drawn in another's look loses to the part's more specific one
// (.floe-links > li::before over .floe-card::before), as that part needs. A scoped rule would win over
// an unscoped one of the same specificity, so scope and layer the parts and their themes together, once
// a theme is a file of each part.
func pageStyle(page, own []byte, theme string, diagrams bool) (string, error) {
	var parts strings.Builder
	if bytes.Contains(page, []byte("data-floe-part")) {
		set, err := builtinParts()
		if err != nil {
			return "", err
		}
		for _, name := range set.partsNamedIn(string(page)) {
			parts.WriteString(set.parts[name].css)
		}
	}
	if diagrams {
		parts.WriteString(diagram.Style())
	}
	css, err := readPartsTheme(theme)
	if err != nil {
		return "", err
	}
	return "<style>\n@layer floe.base, floe.page, floe.parts;\n" +
		"@layer floe.base {\n" + baseCSS + "}\n" +
		"@layer floe.page {\n@scope (:root) to ([data-floe-part] > *) {\n" + string(own) + "}\n}\n" +
		"@layer floe.parts {\n" + parts.String() + string(css) + "}\n</style>\n", nil
}

// baseCSS is what every page starts from: text, lists, links, code and quotes, for the template's own
// elements and, where its CSS does not reach, those of the parts.
//
//go:embed base.css
var baseCSS string

// themesDir is the directory of parts/ that holds the parts themes, and so is no part.
const themesDir = "themes"

// PartsThemes returns the themes of floe's parts, by name.
func PartsThemes() ([]string, error) {
	themes, err := themesIn(partFiles, path.Join("parts", themesDir))
	if err != nil {
		return nil, fmt.Errorf("read the parts themes: %w", err)
	}
	return themes, nil
}

func readPartsTheme(name string) ([]byte, error) {
	css, err := partFiles.ReadFile(path.Join("parts", themesDir, name+".css"))
	if err != nil {
		return nil, fmt.Errorf("read the parts theme %s: %w", name, err)
	}
	return css, nil
}

// partsThemeVariables returns the CSS variables the parts themes take from the page, those a theme uses
// and does not define itself, once each, in the order of the themes' names.
func partsThemeVariables() ([]string, error) {
	themes, err := PartsThemes()
	if err != nil {
		return nil, err
	}
	var vars []string
	for _, name := range themes {
		css, err := readPartsTheme(name)
		if err != nil {
			return nil, err
		}
		defined := definedVariables(css)
		for _, v := range appendFound(nil, usedVariable, string(css)) {
			if !defined[v] && !slices.Contains(vars, v) {
				vars = append(vars, v)
			}
		}
	}
	return vars, nil
}

var usedVariable = regexp.MustCompile(`var\(\s*(--[\w-]+)`)

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
	tagOrText = regexp.MustCompile(`(?i)(<!--)|<(script|noscript)\b[^>]*>|<(style|link)\b|</head\s*>`)
	endTag    = map[string]*regexp.Regexp{
		"script":   regexp.MustCompile(`(?i)</script\s*>`),
		"noscript": regexp.MustCompile(`(?i)</noscript\s*>`),
	}
)

// partVariables returns the CSS variables the style of the parts named, and of those they are drawn in
// the look of, takes from the theme, once each, in the order of the parts' names.
func partVariables(names []string) ([]string, error) {
	set, err := builtinParts()
	if err != nil {
		return nil, err
	}
	// The parts used are followed to those they use in turn: tabs holds a stack, which holds a diff, drawn
	// in the look of code.
	used := map[string]bool{}
	for next := names; len(next) > 0; {
		name := next[0]
		next = next[1:]
		if !used[name] {
			used[name] = true
			next = append(next, set.parts[name].uses...)
		}
	}
	// floe's base styles the elements of every part floe draws from HTML.
	var vars []string
	if slices.ContainsFunc(names, func(name string) bool { return !isGoPart(name) }) {
		vars = appendFound(nil, fallback, baseCSS)
	}
	for _, name := range slices.Sorted(maps.Keys(used)) {
		vars = appendFound(vars, fallback, set.parts[name].css)
	}
	return vars, nil
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
