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
	"slices"
	"strconv"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// Check checks the template at dir as a template of the directory's name, whatever its example.yaml
// names: its files, then its example.yaml through every stage of a publish, drawing included, with the
// parts of partsDir and the icons kept in iconsDir in its diagrams. The mistakes stop it from being drawn,
// and the warnings do not.
func Check(dir, partsDir, iconsDir string) (warnings []*usererr.Error, mistakes []error) {
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
	t, err := NewLibrary(filepath.Dir(abs), partsDir, iconsDir).resolve(name)
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

// CheckOwnParts checks the parts and the parts themes of one's own in partsDir, "" for none, as a page
// would read them: the mistakes are of those that cannot be used, and the warnings of what goes into no
// page. It draws none of them, which have no data; a template that uses one draws its example with it.
func CheckOwnParts(partsDir string) (warnings []*usererr.Error, mistakes []error) {
	set, err := partsIn(partsDir)
	if err != nil {
		return nil, []error{err}
	}
	for _, why := range slices.Concat(set.unusable(nil), set.unusableThemes(nil)) {
		mistakes = append(mistakes, why)
	}
	return set.strays, mistakes
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
	set, err := t.parts()
	if err != nil {
		return nil, []error{err}
	}
	sch, doc, err := compileSchema(set, t.Name, src)
	if err != nil {
		return nil, []error{err}
	}
	s, err := t.readSchema(src)
	if err != nil {
		return nil, []error{err}
	}
	lint := &schemaLint{
		file: t.file("schema.json"), base: schemaURL(t.Name), warnings: nil, mistakes: nil, set: set,
	}
	lint.walk(doc, "#", true)
	warnings, mistakes = lint.warnings, lint.mistakes
	page, pageSrc, pageErr := t.page()
	named := namedParts(string(pageSrc))
	// Told beside the other mistakes of the files, not instead of them.
	if err = t.refuseShared(s); err != nil {
		mistakes = append(mistakes, err)
	}
	mistakes = append(mistakes, t.checkChecks(sch)...)
	if err = t.checkComputed(s); err != nil {
		mistakes = append(mistakes, err)
	}
	if pageErr != nil {
		return warnings, append(mistakes, pageErr)
	}
	if tagsOf(tagsIn(string(pageSrc), true)).head < 0 {
		mistakes = append(mistakes, usererr.New("tmpl_page_head", t.file("page.html")))
	}
	mistakes = append(mistakes, t.checkAssets(page)...)
	scope, css := t.checkCSS()
	warnings, mistakes = append(warnings, scope...), append(mistakes, css...)
	return append(warnings, brokenParts(set, named)...), append(mistakes, t.checkNamedParts(set, pageSrc, named)...)
}

// checkComputed returns the mistake of t's computed.yaml, of the items of its schema s, if it has one.
func (t *Template) checkComputed(s *schema) error {
	computed, found, err := t.readOptional("computed.yaml")
	if err != nil || !found {
		return err
	}
	_, err = readComputed(t.file("computed.yaml"), computed, slices.Sorted(maps.Keys(s.Properties)))
	return err
}

// brokenParts warns of each part of one's own in set that cannot be used, but those page.html names by an
// attribute, of named, which checkNamedParts refuses: the schema refers to none of them, as it compiled. A
// parts theme that cannot be used is warned of too: a data file picks it.
func brokenParts(set *partSet, named []namedPart) []*usererr.Error {
	refused := map[string]bool{}
	for _, n := range named {
		if !n.text {
			refused[n.name] = true
		}
	}
	// floe's stays in place of one of a name set keeps (diagram), which page.html may name.
	warnings := set.unusable(func(name string) bool { return refused[name] && set.parts[name] == nil })
	warnings = append(warnings, set.unusableThemes(nil)...)
	return append(warnings, set.strays...)
}

// checkCSS returns the mistakes of t's CSS, page.css and its themes, that cssSheet.refuse refuses, and
// the warnings of the colours that change with the mode which it sets on :scope for light mode alone:
// over every palette, they stay in dark mode too, where the others turn dark. Each file is told once,
// of what it sets so with any theme.
func (t *Template) checkCSS() ([]*usererr.Error, []error) {
	themes, err := t.themes()
	if err != nil {
		return nil, []error{err}
	}
	modes, err := floeModes()
	if err != nil {
		return nil, []error{err}
	}
	var mistakes []error
	// Every file is checked, page.css refused or not.
	under := newModeColours()
	if page, err := t.ownSheet("page.css"); err != nil {
		mistakes = append(mistakes, err)
	} else {
		under.readScope(page, "page.css")
	}
	byFile := map[string]map[string]bool{} // the names each file sets for light mode alone
	note := func(c *modeColours) {
		for _, v := range c.lightOnly(modes) {
			if byFile[c.from[v]] == nil {
				byFile[c.from[v]] = map[string]bool{}
			}
			byFile[c.from[v]][v] = true
		}
	}
	read := 0
	for _, theme := range themes {
		name := path.Join("themes", theme+".css")
		sheet, err := t.ownSheet(name)
		if err != nil {
			mistakes = append(mistakes, err)
			continue
		}
		// The theme comes after page.css, and sets over it.
		c := under.clone()
		c.readScope(sheet, name)
		note(c)
		read++
	}
	if read == 0 {
		note(under)
	}
	var warnings []*usererr.Error
	for _, file := range slices.Sorted(maps.Keys(byFile)) {
		names := slices.Sorted(maps.Keys(byFile[file]))
		warnings = append(warnings, usererr.New("tmpl_scope_dark", t.file(file), andList(names)))
	}
	return warnings, mistakes
}

// checkNamedParts returns a mistake for each part src, page.html, names, of named, by data-floe-part or
// data-floe-look, that set does not have, as the page would get no style for it, and for each
// data-floe-part that is not one part's name or names a look.
func (t *Template) checkNamedParts(set *partSet, src []byte, named []namedPart) []error {
	var mistakes []error
	told := map[int]bool{} // the attributes not of one name told, by where each is
	for _, n := range named {
		line := lineAt(src, n.at)
		switch set.misnamed(n) {
		case nameNotOne:
			if !told[n.at] {
				told[n.at] = true
				mistakes = append(mistakes, usererr.New("tmpl_part_several", t.file("page.html"), line, n.value))
			}
		case nameLookBox:
			mistakes = append(mistakes, usererr.New("tmpl_look_box", t.file("page.html"), line, n.name))
		case nameBroken:
			mistakes = append(mistakes, usererr.New("tmpl_part_broken", n.name, set.broken[n.name]))
		case nameUnknown:
			mistakes = append(mistakes, usererr.New("tmpl_page_part", t.file("page.html"), line, n.name, set.candidates(n.look)))
		case nameRight:
		}
	}
	return mistakes
}

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
	if err = t.check(h, diskImages(t.Dir), Themes{Template: "", Parts: ""}, problems); err != nil {
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
	set      *partSet // the parts the template is drawn with
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
	case !knownFormat(l.set, f):
		l.mistakes = append(l.mistakes, usererr.New("tmpl_format_unknown", l.file, ptr, f))
	case slices.Contains(formats, f) && !reached:
		l.mistakes = append(l.mistakes, usererr.New("tmpl_format_unreached", l.file, ptr, f))
	case slices.Contains(formats, f):
		if typ != formatTypes[f] {
			l.mistakes = append(l.mistakes, usererr.New("tmpl_format_type", l.file, ptr, f, formatTypes[f]))
		}
		return true
	}
	return false
}

// part notes the part of floe's ref names, at ptr, and reports whether it names one: one prepare does
// not reach would never be drawn, nor would a schema the parts share that holds what floe draws. A ref
// into either, wherever it is, would be checked as written but drawn as plain data.
func (l *schemaLint) part(ref, ptr string, reached bool) bool {
	ref = absolute(l.base, ref)
	if !strings.HasPrefix(ref, partsURL) {
		return false
	}
	name, ok := partName(ref)
	set := l.set
	switch {
	case !ok:
		if _, into := set.pointsInto(ref); into {
			l.mistakes = append(l.mistakes,
				usererr.New("tmpl_item", l.file, usererr.New("tmpl_item", ptr, unknownPart(set, ref))))
		}
		return false
	case set.parts[name] == nil:
		if !reached && set.draws(set.sharedSchema(ref)) {
			l.mistakes = append(l.mistakes, usererr.New("tmpl_shared_unreached", l.file, ptr, ref))
		}
		return false
	case !reached:
		l.mistakes = append(l.mistakes, usererr.New("tmpl_part_unreached", l.file, ptr, name))
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
