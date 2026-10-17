package tmpl

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
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

	"github.com/sukekyo26/floe/internal/yamlpos"
)

// partFiles holds a directory per part of floe's (schema.json, part.html, part.css and checks.yaml),
// and the schemas the parts share (tone.json).
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
	return b.ResolveReference(r).String()
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
	page   *template.Template
	css    string
	checks []relation
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
	for _, s := range set.schemas {
		s.resolve(set)
	}
	// A part readies its item with no data file to look up what it drew: it holds no item of a part or
	// a format of floe's.
	for name, p := range set.parts {
		if p.schema.drawsWhole() {
			return nil, fmt.Errorf("%w: %s", errPartWhole, name)
		}
	}
	return set, nil
}

var errPartWhole = errors.New("a part holds an item of a part or a format of floe's")

// drawsWhole reports whether s, or a schema under it, declares an item of a part or a format of
// floe's.
func (s *schema) drawsWhole() bool {
	if s == nil {
		return false
	}
	if s.part != nil || slices.Contains(formats, s.Format) || s.Items.drawsWhole() {
		return true
	}
	for _, p := range s.Properties {
		if p.drawsWhole() {
			return true
		}
	}
	return false
}

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
	page, err := template.New(name).Option("missingkey=error").Funcs(funcs).Parse(string(html))
	if err != nil {
		return nil, fmt.Errorf("parse the part %s: %w", name, err)
	}
	css, err := partFiles.ReadFile(path.Join(dir, "part.css"))
	if err != nil {
		return nil, fmt.Errorf("read the part %s: %w", name, err)
	}
	p := &part{name: name, schema: s, page: page, css: string(css), checks: nil}
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
	// A part, or an item of a format of floe's, is drawn whole: what it holds is not looked into.
	switch {
	case s.part != nil:
		*found = append(*found, partAt{itemAt: itemAt{value: v, at: at}, part: s.part})
		return
	case slices.Contains(formats, s.Format):
		return
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
	if found == nil {
		return nil
	}
	d.parts = make(map[string]template.HTML, len(found))
	var problems yamlpos.Problems
	for _, f := range found {
		more, err := f.part.check(d.File, f.itemAt, root)
		if err != nil {
			return err
		}
		if more != nil {
			problems = append(problems, more...)
			continue
		}
		html, err := f.part.draw(f.value)
		if err != nil {
			return err
		}
		d.parts[pathKey(f.at)] = html
		if !slices.Contains(d.used, f.part.name) {
			d.used = append(d.used, f.part.name)
		}
	}
	if problems != nil {
		return problems.Sorted()
	}
	return nil
}

// check runs the checks of p on the item f of the data file file, under root. An error is p's own
// mistake, and the problems are the data's.
func (p *part) check(file string, f itemAt, root ast.Node) (yamlpos.Problems, error) {
	var problems yamlpos.Problems
	for _, c := range p.checks {
		items := []itemAt{f}
		if c.For != "" {
			items = nil
			if !itemsAt(f.value, strings.Split(c.For, "."), f.at, &items) {
				return nil, yamlpos.New(c.file, c.at["for"], "tmpl_checks_not_list", c.For, file)
			}
		}
		var doc map[string]any
		if m, ok := f.value.(map[string]any); ok {
			doc = m
		}
		more, err := c.run(file, doc, items, root)
		if err != nil {
			return nil, err
		}
		problems = append(problems, more...)
	}
	return problems, nil
}

// draw draws v, an item p checked, as HTML.
func (p *part) draw(v any) (template.HTML, error) {
	var b bytes.Buffer
	if err := p.page.Execute(&b, p.schema.prepare(v, nil, nil)); err != nil {
		return "", fmt.Errorf("draw the part %s: %w", p.name, err)
	}
	return template.HTML(strings.TrimSpace(b.String())), nil //nolint:gosec // G203: drawn by html/template
}

// partStyles returns the style of the parts named, in the order of their names, for the page's <head>.
func partStyles(names []string) (string, error) {
	set, err := builtinParts()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("<style>\n")
	for _, name := range slices.Sorted(slices.Values(names)) {
		b.WriteString(set.parts[name].css)
	}
	b.WriteString("</style>\n")
	return b.String(), nil
}

// withStyles puts style into page before the template's own styles, which then win over the parts'
// where they say otherwise: before the first <style> or <link> of its <head>, else before its </head>.
// A page without a <head> gets it before its first <style> or <link>, else at its end. One in a
// script, a noscript or a comment is not the template's style.
// LIMIT: the tags are found as written, not as an HTML parser reads them (a <style in an attribute's
// value is taken for one). Parse the page if a template comes to write one so.
func withStyles(page []byte, style string) []byte {
	at := len(page)
	if end := headEnd.FindIndex(page); end != nil {
		at = end[0]
	}
	hidden := notStyled.FindAllIndex(page[:at], -1)
	for _, own := range ownStyle.FindAllIndex(page[:at], -1) {
		if !slices.ContainsFunc(hidden, func(h []int) bool { return h[0] <= own[0] && own[0] < h[1] }) {
			at = own[0]
			break
		}
	}
	return slices.Concat(page[:at], []byte(style), page[at:])
}

var (
	headEnd   = regexp.MustCompile(`(?i)</head\s*>`)
	ownStyle  = regexp.MustCompile(`(?i)<(?:style|link)\b`)
	notStyled = regexp.MustCompile(`(?is)<!--.*?-->|<script\b.*?</script\s*>|<noscript\b.*?</noscript\s*>`)
)

// partVariables returns the CSS variables the style of the parts named takes from the theme, once
// each, in the order of their names.
func partVariables(names []string) ([]string, error) {
	set, err := builtinParts()
	if err != nil {
		return nil, err
	}
	var vars []string
	for _, name := range slices.Sorted(slices.Values(names)) {
		for _, m := range fallback.FindAllStringSubmatch(set.parts[name].css, -1) {
			if !slices.Contains(vars, m[1]) {
				vars = append(vars, m[1])
			}
		}
	}
	return vars, nil
}

// fallback finds the variables a style takes with a value of its own for a theme without them.
var fallback = regexp.MustCompile(`var\(\s*(--[\w-]+)\s*,`)
