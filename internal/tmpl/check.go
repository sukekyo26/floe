package tmpl

import (
	"cmp"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// Check checks the template at dir as a template of the directory's name, whatever its example.yaml
// names: its files, then its example.yaml through every stage of a publish, drawing included, with the
// icons kept in iconsDir in its diagrams. The mistakes stop it from being drawn, and the warnings do not.
func Check(dir, iconsDir string) (warnings []*usererr.Error, mistakes []error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, []error{usererr.CannotRead(dir, err)}
	}
	name := filepath.Base(abs)
	if !validName(name) {
		return nil, []error{usererr.New("tmpl_new_name", name)}
	}
	if _, err = os.Stat(abs); err != nil {
		return nil, []error{usererr.CannotRead(dir, err)}
	}
	// Found as a library of the directory above would find it, symlink and all.
	t, err := NewLibrary(filepath.Dir(abs), iconsDir).resolve(name)
	if err != nil {
		return nil, []error{err}
	}
	if t == nil {
		return nil, []error{usererr.New("tmpl_check_not_dir", dir)}
	}
	t.Path = dir
	// The floe a template needs comes before its mistakes, which another floe may not make.
	if err = t.checkVersion(); err != nil {
		return nil, []error{err}
	}
	needs, err := t.needs()
	if err != nil {
		return nil, []error{err}
	}
	if needs == nil {
		var have semver
		if have, err = thisFloe(); err != nil {
			return nil, []error{err}
		}
		warnings = append(warnings, usererr.New("tmpl_no_template_yaml", dir, have.minor()))
	}
	more, mistakes := t.checkFiles()
	warnings = append(warnings, more...)
	if mistakes != nil {
		return warnings, mistakes
	}
	// Each stage runs on what passed the one before: a broken file would only fail the example.
	if err = t.checkExample(); err != nil {
		return warnings, []error{err}
	}
	return warnings, nil
}

// checkFiles checks the files of t apart from any data: its schema, its checks and computed values,
// page.html and the default theme.
func (t *Template) checkFiles() (warnings []*usererr.Error, mistakes []error) {
	needed := []string{"page.html", "schema.json", "example.yaml", "themes/" + DefaultTheme + ".css"}
	for _, name := range needed {
		_, found, err := t.readOptional(name)
		switch {
		case err != nil:
			mistakes = append(mistakes, err)
		case !found:
			mistakes = append(mistakes, usererr.New("tmpl_file_missing", t.file(name), andList(needed)))
		}
	}
	if mistakes != nil {
		return nil, mistakes
	}
	src, err := t.readFile("schema.json")
	if err != nil {
		return nil, []error{err}
	}
	sch, doc, err := compileSchema(t.Name, src)
	if err != nil {
		return nil, []error{err}
	}
	s, err := t.readSchema(src)
	if err != nil {
		return nil, []error{err}
	}
	lint := &schemaLint{
		file: t.file("schema.json"), base: schemaURL(t.Name), warnings: nil, mistakes: nil, parts: nil,
	}
	lint.walk(doc, "#", true)
	warnings, mistakes = lint.warnings, lint.mistakes
	// Told beside the other mistakes of the files, not instead of them.
	if err = t.refuseShared(s); err != nil {
		mistakes = append(mistakes, err)
	}
	more, err := t.checkThemeVariables(lint)
	warnings = append(warnings, more...)
	if err != nil {
		mistakes = append(mistakes, err)
	}
	mistakes = append(mistakes, t.checkChecks(sch)...)
	computed, found, err := t.readOptional("computed.yaml")
	if err == nil && found {
		_, err = readComputed(t.file("computed.yaml"), computed, slices.Sorted(maps.Keys(s.Properties)))
	}
	if err != nil {
		mistakes = append(mistakes, err)
	}
	page, src, err := t.page()
	if err != nil {
		return warnings, append(mistakes, err)
	}
	mistakes = append(mistakes, t.checkAssets(page)...)
	mistakes = append(mistakes, t.checkCSS()...)
	return warnings, append(mistakes, t.checkNamedParts(src)...)
}

// checkCSS returns the mistakes of page.css and of each theme of t that ownCSS refuses.
func (t *Template) checkCSS() []error {
	themes, err := t.themes()
	if err != nil {
		return []error{err}
	}
	names := []string{"page.css"}
	for _, theme := range themes {
		names = append(names, path.Join("themes", theme+".css"))
	}
	var mistakes []error
	for _, name := range names {
		if _, err := t.ownCSS(name); err != nil {
			mistakes = append(mistakes, err)
		}
	}
	return mistakes
}

// checkNamedParts returns a mistake for each part src, page.html, takes the look of, by data-floe-part,
// that floe does not have: the page would get no style for it.
func (t *Template) checkNamedParts(src []byte) []error {
	set, err := builtinParts()
	if err != nil {
		return []error{err}
	}
	var mistakes []error
	for _, n := range namedParts(string(src)) {
		if _, ok := set.parts[n.name]; !ok {
			mistakes = append(mistakes, usererr.New("tmpl_page_part", t.file("page.html"), lineAt(src, n.at), n.name,
				strings.Join(slices.Sorted(maps.Keys(set.parts)), ", ")))
		}
	}
	return mistakes
}

// checkThemeVariables warns of each theme of t that leaves out a CSS variable the parts and diagrams
// its schema declares, as lint found them, take their colours and fonts from, which page.css does not
// define either: they would keep light colours in dark mode.
// LIMIT: a variable is looked for as written (--fg:), not as CSS reads it. Parse the CSS if a theme
// comes to define one otherwise (through @import, or in a comment that hides it).
func (t *Template) checkThemeVariables(lint *schemaLint) ([]*usererr.Error, error) {
	vars, err := lint.variables()
	if vars == nil || err != nil {
		return nil, err
	}
	themes, err := t.themes()
	if err != nil {
		return nil, err
	}
	// page.css may define them for every theme.
	page, _, err := t.readOptional("page.css")
	if err != nil {
		return nil, err
	}
	inPage := definedVariables(page)
	var warnings []*usererr.Error
	for _, theme := range themes {
		name := path.Join("themes", theme+".css")
		css, err := t.readFile(name)
		if err != nil {
			return nil, err
		}
		defined := definedVariables(css)
		var colours, others []string
		for _, v := range vars {
			switch {
			case defined[v] || inPage[v]:
			case slices.Contains(sameInDark, v):
				others = append(others, v)
			default:
				colours = append(colours, v)
			}
		}
		if colours != nil {
			warnings = append(warnings, usererr.New("tmpl_theme_variables", t.file(name), andList(colours)))
		}
		if others != nil {
			warnings = append(warnings, usererr.New("tmpl_theme_variables_plain", t.file(name), andList(others)))
		}
	}
	return warnings, nil
}

// sameInDark are the CSS variables of the parts whose default serves dark mode as well: their fonts, the
// width of a note, and the text of code, a terminal's prompt and a diff's lines, colours of a dark ground in light
// mode too.
var sameInDark = []string{"--font", "--mono", "--text-width", "--code-fg", "--code-prompt", "--code-add", "--code-del"}

// definedVariables returns the CSS variables src defines (--fg: #000), by name.
func definedVariables(src []byte) map[string]bool {
	defined := map[string]bool{}
	for _, m := range cssVariable.FindAllSubmatch(src, -1) {
		defined[string(m[1])] = true
	}
	return defined
}

var cssVariable = regexp.MustCompile(`(?:^|[^\w-])(--[\w-]+)\s*:`)

// checkAssets returns the mistakes of the images under the assets/ of t, if it has one, and of those
// page.html names to asset, in every branch.
func (t *Template) checkAssets(page *template.Template) []error {
	var mistakes []error
	var checked map[string]bool
	err := t.files(func(files fs.FS) error {
		fi, err := fs.Stat(files, "assets")
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return nil
		case err != nil:
			return err //nolint:wrapcheck // wrapped below
		case !fi.IsDir():
			return usererr.New("tmpl_assets_not_dir", t.file("assets"))
		}
		sub, err := fs.Sub(files, "assets")
		if err != nil {
			return err //nolint:wrapcheck // wrapped below
		}
		mistakes, checked, err = t.assetImages(sub)
		return err
	})
	var notDir *usererr.Error
	switch {
	case errors.As(err, &notDir):
		return []error{notDir}
	case err != nil:
		return []error{fmt.Errorf("read the assets of %s: %w", t.Name, err)}
	}
	for _, name := range assetsNamed(page) {
		if checked[name] {
			continue // its mistake, if it has one, is told already
		}
		var bad *usererr.Error
		_, err = t.asset(name)
		switch {
		case errors.As(err, &bad):
			mistakes = append(mistakes, t.assetMistake(bad))
		case err != nil:
			mistakes = append(mistakes, err)
		}
	}
	return mistakes
}

// assetImages returns the mistakes of the images of assets, the assets/ of t, and the names of those
// it checked. A file not named as an image (a README), or whose name starts with a dot, which asset
// cannot name, is left alone.
func (t *Template) assetImages(assets fs.FS) (mistakes []error, checked map[string]bool, err error) {
	checked = map[string]bool{}
	err = fs.WalkDir(assets, ".", func(name string, e fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case name != "." && strings.HasPrefix(e.Name(), ".") && e.IsDir():
			return fs.SkipDir
		case e.IsDir() || strings.HasPrefix(e.Name(), "."):
			return nil
		}
		if _, image := imageTypes[strings.ToLower(path.Ext(name))]; image {
			checked[name] = true
			if _, bad := imageURI(assets, name, t.file("assets")); bad != nil {
				mistakes = append(mistakes, usererr.New("tmpl_item", t.file("assets"), bad))
			}
		}
		return nil
	})
	return mistakes, checked, err //nolint:wrapcheck // wrapped by checkAssets
}

// assetsNamed returns the names page passes to asset as written, {{asset "logo.svg"}} or {{"logo.svg" |
// asset}}, once each. One of the data is left to the drawing.
func assetsNamed(page *template.Template) []string {
	return callsOf(page, "asset").names
}

// checkChecks returns the mistakes of the checks.yaml of t, if it has one, against its schema doc as
// read, in the order of the file.
func (t *Template) checkChecks(s *jsonschema.Schema) []error {
	src, found, err := t.readOptional("checks.yaml")
	if err != nil {
		return []error{err}
	}
	if !found {
		return nil
	}
	list, err := readChecks(t.file("checks.yaml"), src, false)
	var problems yamlpos.Problems
	if err != nil && !errors.As(err, &problems) {
		return []error{err}
	}
	for _, c := range list {
		if e := c.declared(s); e != nil {
			problems = append(problems, e)
		}
		if _, e := c.program(); e != nil {
			problems = append(problems, e)
		}
	}
	if problems == nil {
		return nil
	}
	return []error{problems.Sorted()}
}

// declared checks that the compiled schema s declares the list at c.For and c.Field under its items:
// a path misspelt would find nothing in the data to check, and fail no data. A path is refused only
// where the schema surely refuses it, a list only where its types leave array out.
func (c relation) declared(s *jsonschema.Schema) *yamlpos.Error {
	list, sure := declaredAt(s, strings.Split(c.For, "."), true)
	switch {
	case list == nil && sure:
		return yamlpos.New(c.file, c.at["for"], "tmpl_checks_undeclared", c.For)
	case list == nil:
		return nil
	case !takes(list, "array"):
		return yamlpos.New(c.file, c.at["for"], "tmpl_checks_not_array", c.For)
	case c.Field == "":
		return nil
	}
	items := itemsOf(list)
	if items == nil {
		return nil
	}
	if field, sure := declaredAt(items, strings.Split(c.Field, "."), false); field == nil && sure {
		return yamlpos.New(c.file, c.at["field"], "tmpl_checks_field", c.Field, c.For)
	}
	return nil
}

// declaredAt follows the path of keys steps from the compiled schema s, through properties and $ref,
// and through the items of a list on the way when lists is true. It returns the schema declared
// there, or nil, sure true where the path is surely not declared and false where a schema may take it.
// LIMIT: a key is not matched against patternProperties, nor followed through allOf, anyOf and the
// like: it is taken as declared there. A schema that takes both a list and keys and values is followed
// as keys and values. Follow them if a template comes to declare its lists so.
func declaredAt(s *jsonschema.Schema, steps []string, lists bool) (found *jsonschema.Schema, sure bool) {
	for _, step := range steps {
		if lists {
			s = intoItems(s)
		}
		if s == nil {
			return nil, false
		}
		if s.Bool != nil {
			return nil, !*s.Bool // true takes any value, false none
		}
		if !takes(s, "object") {
			return nil, true
		}
		next := propertyOf(s, step)
		if next == nil {
			// A schema and those its $ref leads to all apply: one that refuses the key is enough.
			for r := s; r != nil; r = r.Ref {
				if refuses(r) {
					return nil, true
				}
			}
			return nil, false
		}
		s = next
	}
	if s.Bool != nil {
		return nil, !*s.Bool
	}
	return s, true
}

// intoItems returns the schema of the items of s while s is surely a list and not keys and values, or
// nil where a list does not declare its items, or declares lists of itself all the way down.
func intoItems(s *jsonschema.Schema) *jsonschema.Schema {
	seen := map[*jsonschema.Schema]bool{}
	for s != nil && s.Bool == nil && !takes(s, "object") && takes(s, "array") {
		if seen[s] {
			return nil
		}
		seen[s] = true
		s = itemsOf(s)
	}
	return s
}

// propertyOf returns the schema s declares for the key in its properties or those of its $ref, or nil.
func propertyOf(s *jsonschema.Schema, key string) *jsonschema.Schema {
	for ; s != nil; s = s.Ref {
		if p, ok := s.Properties[key]; ok {
			return p
		}
	}
	return nil
}

// refuses reports whether s surely refuses a key it does not declare: by additionalProperties: false,
// or by unevaluatedProperties: false where nothing else may take the key.
func refuses(s *jsonschema.Schema) bool {
	if len(s.PatternProperties) > 0 {
		return false
	}
	if closed, ok := s.AdditionalProperties.(bool); ok {
		return !closed
	}
	u := s.UnevaluatedProperties
	if u == nil || u.Bool == nil || *u.Bool {
		return false
	}
	applies := s.Ref != nil || s.RecursiveRef != nil || s.DynamicRef != nil || s.If != nil ||
		len(s.AllOf)+len(s.AnyOf)+len(s.OneOf)+len(s.DependentSchemas)+len(s.Dependencies) > 0
	return s.AdditionalProperties == nil && !applies
}

// takes reports whether s, and the schemas its $ref leads to, may take a value of the JSON type t.
func takes(s *jsonschema.Schema, t string) bool {
	for ; s != nil; s = s.Ref {
		if s.Types != nil && !s.Types.IsEmpty() && !slices.Contains(s.Types.ToStrings(), t) {
			return false
		}
	}
	return true
}

// itemsOf returns the schema of every item of the list s, or nil where none is declared, or where the
// items are declared by their position (prefixItems), not one schema for all.
func itemsOf(s *jsonschema.Schema) *jsonschema.Schema {
	for ; s != nil; s = s.Ref {
		if len(s.PrefixItems) > 0 {
			return nil
		}
		if s.Items2020 != nil {
			return s.Items2020
		}
		switch items := s.Items.(type) {
		case *jsonschema.Schema:
			return items
		case []*jsonschema.Schema:
			return nil
		}
	}
	return nil
}

// checkExample reads the example.yaml of t as a data file of it, and draws it.
func (t *Template) checkExample() error {
	file := t.file("example.yaml")
	src, err := t.readFile("example.yaml")
	if err != nil {
		return err
	}
	h, err := start(file, src, Themes{Template: "", Parts: ""})
	if err != nil {
		return err
	}
	// A data file started from it names this template, as floe template new writes it.
	d, problems := h.d, h.problems
	switch {
	case h.unnamed:
		problems = append(problems, yamlpos.New(file, h.where["template"], "tmpl_no_template", t.Name))
	case d.Template != "" && d.Template != t.Name:
		problems = append(problems, yamlpos.New(file, h.where["template"], "tmpl_example_name", d.Template, t.Name))
	}
	// The later stages check it against t, and name t in their mistakes.
	d.Template = t.Name
	if err = t.check(h, t.Dir, Themes{Template: "", Parts: ""}, problems); err != nil {
		return err
	}
	_, err = d.Render(cmp.Or(d.Title, file))
	return err
}

// formatTypes are the JSON types of the items of floe's formats: the text of HTML, of an image's path,
// or of Markdown.
var formatTypes = map[string]string{"html": "string", "image": "string", "markdown": "string"}

// schemaLint finds what jsonschema compiles but floe would not draw as the schema means.
type schemaLint struct {
	file     string
	warnings []*usererr.Error
	mistakes []error
	base     string   // the schema's URL, which a relative $ref is read from
	parts    []string // the parts of floe's that draw the items it declares, once each
}

// walk walks the schema v at the JSON pointer ptr. reached tells that prepare reaches it, through
// properties and items alone, so that a format of floe's own there marks an item for drawing.
func (l *schemaLint) walk(v any, ptr string, reached bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return
	}
	if ref, ok := m["$ref"].(string); ok && l.part(ref, ptr, reached) {
		// prepare draws it whole, and goes no further in.
		reached = false
	}
	if f, ok := m["format"].(string); ok && l.format(f, m["type"], ptr, reached) {
		// prepare draws it whole, and goes no further in: a format below is not reached.
		reached = false
	}
	closed := m["additionalProperties"] == false || m["unevaluatedProperties"] == false
	if _, ok := m["properties"]; ok && reached && !closed {
		l.warnings = append(l.warnings, usererr.New("tmpl_schema_open", l.file, ptr))
	}
	for _, kw := range slices.Sorted(maps.Keys(m)) {
		l.keyword(kw, m[kw], ptr+"/"+pointerKey.Replace(kw), reached)
	}
}

// format checks the format f of the schema at ptr, of the type typ, and reports whether prepare draws
// the item whole.
func (l *schemaLint) format(f string, typ any, ptr string, reached bool) bool {
	switch {
	case isGoPart(f):
		l.mistakes = append(l.mistakes, usererr.New("tmpl_format_part", l.file, ptr, f, f))
		return true
	case !knownFormat(f):
		l.mistakes = append(l.mistakes, usererr.New("tmpl_format_unknown", l.file, ptr, f))
	case slices.Contains(formats, f) && !reached:
		l.mistakes = append(l.mistakes, usererr.New("tmpl_format_unreached", l.file, ptr, f))
	case slices.Contains(formats, f):
		if typ != formatTypes[f] {
			l.mistakes = append(l.mistakes, usererr.New("tmpl_format_type", l.file, ptr, f, formatTypes[f]))
		}
		// Markdown draws its tables in the table part's look.
		if f == "markdown" && !slices.Contains(l.parts, "table") {
			l.parts = append(l.parts, "table")
		}
		return true
	}
	return false
}

// variables returns the CSS variables floe's base, the parts and diagrams l found, and the parts themes
// take from the theme, once each.
func (l *schemaLint) variables() ([]string, error) {
	vars, err := partVariables(l.parts)
	if err != nil {
		return nil, err
	}
	// floe's base styles every page, the template's own text, lists, links, code and quotes, and those in
	// the parts.
	more := appendFound(nil, fallback, baseCSS)
	if slices.Contains(l.parts, "diagram") {
		more = append(more, diagram.Variables()...)
	}
	// Whichever parts theme a data file picks styles the parts, and the quotes of Markdown, which counts
	// as the table part.
	// LIMIT: every variable of every parts theme, not those of the rules for the parts used; read the
	// themes' rules by their classes if a template of few parts comes to be warned of others' colours.
	if len(l.parts) > 0 {
		themes, err := partsThemeVariables()
		if err != nil {
			return nil, err
		}
		more = append(more, themes...)
	}
	for _, v := range more {
		if !slices.Contains(vars, v) {
			vars = append(vars, v)
		}
	}
	return vars, nil
}

// part notes the part of floe's ref names, at ptr, and reports whether it names one: one prepare does
// not reach would never be drawn. A ref into a part, wherever it is, would be checked as the part but
// drawn as plain data.
func (l *schemaLint) part(ref, ptr string, reached bool) bool {
	ref = absolute(l.base, ref)
	if !strings.HasPrefix(ref, partsURL) {
		return false
	}
	name, ok := partName(ref)
	set, err := builtinParts()
	if err != nil {
		l.mistakes = append(l.mistakes, err)
		return false
	}
	switch {
	case !ok:
		if _, into := set.pointsInto(ref); into {
			l.mistakes = append(l.mistakes, usererr.New("tmpl_item", l.file, usererr.New("tmpl_item", ptr, unknownPart(ref))))
		}
		return false
	case set.parts[name] == nil:
		return false
	case !reached:
		l.mistakes = append(l.mistakes, usererr.New("tmpl_part_unreached", l.file, ptr, name))
	case !slices.Contains(l.parts, name):
		l.parts = append(l.parts, name)
	}
	return true
}

// keyword walks the schemas the keyword kw of a schema holds in v, at ptr.
func (l *schemaLint) keyword(kw string, v any, ptr string, reached bool) {
	switch sub := v.(type) {
	case map[string]any:
		switch kw {
		case "properties":
			l.each(sub, ptr, reached)
		case "items":
			l.walk(sub, ptr, reached)
		case "$defs", "definitions", "patternProperties", "dependentSchemas":
			l.each(sub, ptr, false)
		case "additionalProperties", "unevaluatedProperties", "propertyNames", "additionalItems",
			"unevaluatedItems", "contains", "not", "if", "then", "else", "contentSchema":
			l.walk(sub, ptr, false)
		}
	case []any:
		switch kw {
		case "allOf", "anyOf", "oneOf", "prefixItems", "items":
			for i, s := range sub {
				l.walk(s, ptr+"/"+strconv.Itoa(i), false)
			}
		}
	}
}

// each walks the schemas of a keyword that maps names to them, at ptr.
func (l *schemaLint) each(m map[string]any, ptr string, reached bool) {
	for _, k := range slices.Sorted(maps.Keys(m)) {
		l.walk(m[k], ptr+"/"+pointerKey.Replace(k), reached)
	}
}

// pointerKey escapes a key as a step of a JSON pointer.
var pointerKey = strings.NewReplacer("~", "~0", "/", "~1")
