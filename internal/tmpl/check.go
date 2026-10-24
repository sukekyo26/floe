package tmpl

import (
	"cmp"
	"errors"
	"html/template"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// Check checks the template at dir as a template of the directory's name, whatever its example.yaml
// names: its files, then its example.yaml through every stage of a publish, drawing included, with the
// parts, themes and icons of l. The mistakes stop it from being drawn, and the warnings do not. In a run,
// what every template it checks shares is read once, and its mistakes are the same for each.
func (l *Library) Check(dir string) (warnings usererr.List, err error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, usererr.CannotRead(dir, err)
	}
	name := filepath.Base(abs)
	if !validName(name) {
		return nil, usererr.New("tmpl_new_name", name)
	}
	if _, err = os.Stat(abs); err != nil {
		return nil, usererr.CannotRead(dir, err)
	}
	// Found as a library of the directory above would find it, symlink and all.
	in := &Library{dir: filepath.Dir(abs), mine: l.mine, icons: l.icons, run: l.runOf()}
	t, err := in.resolve(name)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, usererr.New("tmpl_check_not_dir", dir)
	}
	t.Path = dir
	// The floe a template needs comes before its mistakes, which another floe may not make.
	if err = t.checkVersion(); err != nil {
		return nil, err
	}
	needs, err := t.needs()
	if err != nil {
		return nil, err
	}
	if needs == nil {
		var have semver
		if have, err = thisFloe(); err != nil {
			return nil, err
		}
		warnings = append(warnings, usererr.New("tmpl_no_template_yaml", dir, have.minor()))
	}
	more, err := t.checkFiles()
	warnings = append(warnings, more...)
	if err != nil {
		return warnings, err
	}
	// Each stage runs on what passed the one before: a broken file would only fail the example.
	return warnings, t.checkExample()
}

// CheckOwnParts checks the parts and themes of one's own of l as a page would read them: the mistakes
// are of those that cannot be used, and the warnings of what goes into no page and of the colours written
// where the palette is to give them. It draws none of them, which have no data; a template that uses one
// draws its example with it.
func (l *Library) CheckOwnParts() (warnings usererr.List, err error) {
	set, err := l.partsRead()
	if err != nil {
		return nil, err
	}
	return set.strays, slices.Concat(set.unusable(nil), set.unusableThemes(nil)).Err()
}

// checkFiles checks the files of t apart from any data: its schema, its checks and computed values, its
// words, page.html and page.css. A check of what another made (page.html as parsed, the schema as
// compiled) is left out while that has mistakes.
func (t *Template) checkFiles() (usererr.List, error) {
	if err := t.missingFiles(); err != nil {
		return nil, err
	}
	set, err := t.parts()
	if err != nil {
		return nil, err
	}
	warnings, s, schemaErr := t.checkSchema(set)
	w, wordsErr := t.words()
	checked, assetsErr := t.checkAssets()
	src, pageErr := t.readFile("page.html")
	var named []namedPart
	if pageErr == nil {
		named = namedParts(string(src))
		pageErr = t.checkPage(set, s, w, checked, src, named)
	}
	scope, cssErr := t.checkCSS()
	warnings = append(append(warnings, scope...), brokenParts(set, named)...)
	return warnings, usererr.Collect(schemaErr, wordsErr, pageErr, assetsErr, cssErr)
}

// checkPage checks src, the page.html of t, which names the parts named of set: its tags, as text, and,
// parsed, the words it names of w, its words.yaml, nil when it has mistakes, and the images it names to
// asset but those checked (checkAssets). s is the schema of t, nil when it cannot be read.
func (t *Template) checkPage(
	set *partSet, s *shape, w words, checked map[string]bool, src []byte, named []namedPart,
) error {
	text := usererr.Collect(t.refuseInPage(src), t.checkHead(s, src), t.checkNamedParts(set, src, named))
	page, err := t.parsePage(src, w)
	if err != nil {
		return usererr.Collect(text, err)
	}
	return usererr.Collect(text, t.pageWords(page, w), t.checkPageAssets(page, checked))
}

// checkSchema checks the schema of t, which refers to the parts of set, and its checks.yaml and
// computed.yaml, which name its items. It returns the schema's warnings, and its shape, nil when it
// cannot be read.
func (t *Template) checkSchema(set *partSet) (usererr.List, *shape, error) {
	read, err := t.compileShape(set)
	var warnings usererr.List
	var computed error
	if read.shape != nil {
		warnings, computed = read.notes.warnings, t.checkComputed(read.shape)
	}
	return warnings, read.shape, usererr.Collect(err, t.checkChecks(read.compiled), computed)
}

// checkHead refuses src, page.html, without a <head>, and one without a standard doctype where s, the
// schema of t, nil when it cannot be read, has an item of format html.
func (t *Template) checkHead(s *shape, src []byte) error {
	var doctype, head error
	if s != nil {
		doctype = doctypeMistake(t.file("page.html"), s, src)
	}
	if tagsOf(tagsIn(string(src), true)).head < 0 {
		head = usererr.NewAt(t.file("page.html"), 0, 0, "tmpl_page_head")
	}
	return usererr.Collect(doctype, head)
}

// missingFiles returns the mistake of each file t needs that it does not have, or that cannot be read.
func (t *Template) missingFiles() error {
	needed := []string{"page.html", "schema.json", "example.yaml"}
	errs := make([]error, len(needed))
	for i, name := range needed {
		_, found, err := t.readOptional(name)
		if err == nil && !found {
			err = usererr.New("tmpl_file_missing", t.file(name), andList(needed))
		}
		errs[i] = err
	}
	return usererr.Collect(errs...)
}

// checkComputed returns the mistake of t's computed.yaml, of the items of its schema s, if it has one.
func (t *Template) checkComputed(s *shape) error {
	_, err := t.readComputed(s)
	return err
}

// brokenParts warns of each part of one's own in set that cannot be used, but those page.html names by an
// attribute, of named, which checkNamedParts refuses: the schema refers to none of them, as it compiled. A
// theme that cannot be used is warned of too: a data file picks it.
func brokenParts(set *partSet, named []namedPart) usererr.List {
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

// checkCSS returns the mistake of t's CSS, page.css, that cssSheet.refuse refuses, and the warning of the
// colours that change with the mode which it sets on :scope for light mode alone: over every palette, they
// stay in dark mode too, where the others turn dark.
func (t *Template) checkCSS() (usererr.List, error) {
	modes, err := floeModes()
	if err != nil {
		return nil, err
	}
	page, err := t.pageSheet()
	if err != nil {
		return nil, err
	}
	c := newModeColours()
	c.readScope(page)
	if names := c.lightOnly(modes); names != nil {
		return usererr.List{usererr.NewAt(t.file("page.css"), 0, 0, "tmpl_scope_dark", andList(names))}, nil
	}
	return nil, nil
}

// checkNamedParts returns a mistake for each part src, page.html, names, of named, by data-floe-part or
// data-floe-look, that set does not have, as the page would get no style for it, and for each
// data-floe-part that is not one part's name or names a look.
func (t *Template) checkNamedParts(set *partSet, src []byte, named []namedPart) error {
	var mistakes usererr.List
	told := map[int]bool{} // the attributes not of one name told, by where each is
	page := t.file("page.html")
	for _, n := range named {
		line := lineAt(src, n.at)
		switch set.misnamed(n) {
		case nameNotOne:
			if !told[n.at] {
				told[n.at] = true
				mistakes = append(mistakes, usererr.NewAt(page, line, 0, "tmpl_part_several", n.value))
			}
		case nameLookBox:
			mistakes = append(mistakes, usererr.NewAt(page, line, 0, "tmpl_look_box", n.name))
		case nameBroken:
			mistakes = append(mistakes, set.brokenPart(n.name).Placed(page, line, 0, ""))
		case nameUnknown:
			mistakes = append(mistakes, usererr.NewAt(page, line, 0, "tmpl_page_part", n.name, set.candidates(n.look)))
		case nameRight:
		}
	}
	return mistakes.Err()
}

// checkAssets returns the mistakes of the images under the assets/ of t, if it has one, and the names of
// those it checked; nil for none when assets/ cannot be read, which is its mistake.
func (t *Template) checkAssets() (map[string]bool, error) {
	var mistakes usererr.List
	checked := map[string]bool{}
	err := t.files(func(files fs.FS) error {
		fi, err := fs.Stat(files, "assets")
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return nil
		case err != nil:
			return usererr.CannotRead(t.file("assets"), err)
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
	var mistake *usererr.Error
	switch {
	case errors.As(err, &mistake):
		return nil, mistake
	case err != nil:
		return nil, usererr.CannotRead(t.file("assets"), err)
	}
	return checked, mistakes.Err()
}

// checkPageAssets returns the mistakes of the images page, page.html, names to asset, in every branch,
// but those checked, told already; none while checked is nil, assets/ unread, which is told already.
func (t *Template) checkPageAssets(page *template.Template, checked map[string]bool) error {
	if checked == nil {
		return nil
	}
	var mistakes usererr.List
	for _, name := range assetsNamed(page) {
		if checked[name] {
			continue
		}
		_, err := t.asset(name)
		if failed := mistakes.Add(err); failed != nil {
			return failed //nolint:wrapcheck // already says what failed
		}
	}
	return mistakes.Err()
}

// assetImages returns the mistakes of the images of assets, the assets/ of t, and the names of those
// it checked. A file not named as an image (a README), or whose name starts with a dot, which asset
// cannot name, is left alone.
func (t *Template) assetImages(assets fs.FS) (mistakes usererr.List, checked map[string]bool, err error) {
	checked = map[string]bool{}
	err = fs.WalkDir(assets, ".", func(name string, e fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return usererr.CannotRead(filepath.Join(t.file("assets"), filepath.FromSlash(name)), err)
		case name != "." && strings.HasPrefix(e.Name(), ".") && e.IsDir():
			return fs.SkipDir
		case e.IsDir() || strings.HasPrefix(e.Name(), "."):
			return nil
		}
		if _, image := imageTypes[strings.ToLower(path.Ext(name))]; image {
			checked[name] = true
			if _, bad := imageURI(assets, name, t.file("assets")); bad != nil {
				mistakes = append(mistakes, bad.Placed(t.file("assets"), 0, 0, ""))
			}
		}
		return nil
	})
	return mistakes, checked, err //nolint:wrapcheck // a mistake at its path
}

// assetsNamed returns the names page passes to asset as written, {{asset "logo.svg"}} or {{"logo.svg" |
// asset}}, once each. One of the data is left to the drawing.
func assetsNamed(page *template.Template) []string {
	return callsOf(page, "asset").names
}

// checkChecks returns the mistakes of the checks.yaml of t, if it has one: of its YAML and its expressions,
// and, against its schema s as compiled, nil while the schema has mistakes of its own, of the lists it names.
func (t *Template) checkChecks(s *jsonschema.Schema) error {
	list, err := t.readChecks()
	var problems usererr.List
	if failed := problems.Add(err); failed != nil {
		return failed //nolint:wrapcheck // already says what failed
	}
	for _, c := range list {
		if s != nil {
			if e := c.declared(s); e != nil {
				problems = append(problems, e)
			}
		}
	}
	return problems.Err()
}

// declared checks that the compiled schema s declares the list at c.For and c.Field under its items:
// a path misspelt would find nothing in the data to check, and fail no data. A path is refused only
// where the schema surely refuses it, a list only where its types leave array out.
func (c relation) declared(s *jsonschema.Schema) *usererr.Error {
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
	h, err := start(file, src, "")
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
	if err = t.check(h, diskImages(t.Dir), "", problems); err != nil {
		return err
	}
	_, err = d.Render(cmp.Or(d.Title, file))
	return err
}

// pointerKey escapes a key as a step of a JSON pointer.
var pointerKey = strings.NewReplacer("~", "~0", "/", "~1")
