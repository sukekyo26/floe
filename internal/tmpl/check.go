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
	"strings"
	tparse "text/template/parse"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/sukekyo26/floe/internal/dirs"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// Check checks the template at dir as a template of the directory's name, whatever its example.yaml
// names: its files, then its example.yaml through every stage of a publish, drawing included, with the
// parts, themes and icons of l. The mistakes stop it from being drawn, and the warnings do not. In a run,
// what every template it checks shares is read once, and its mistakes are the same for each. A relative dir
// is under cwd, as Read takes a file, and its mistakes name dir as it is written; a relative dir with cwd ""
// is the caller's bug, which floe's own failure tells.
func (l *Library) Check(dir, cwd string) (warnings usererr.List, err error) {
	// "" is the working directory, as filepath.Abs took it, for a caller that does not go through the CLI,
	// which names it so. LIMIT: what "" names is for floe-usage-issues 62 to decide.
	abs, ok := dirs.Under(cwd, cmp.Or(dir, "."))
	if !ok {
		//nolint:err113 // floe's own mistake: its callers give a cwd, or an absolute dir
		return nil, fmt.Errorf("tmpl: the template %q is relative, with no cwd to read it under", dir)
	}
	name := filepath.Base(abs)
	if !validName(name) {
		return nil, usererr.New("tmpl_new_name", name)
	}
	if _, err = os.Stat(abs); err != nil {
		return nil, usererr.CannotRead(dir, err)
	}
	// Found as a library of the directory above would find it, symlink and all.
	in := &Library{
		dir: filepath.Dir(abs), mine: l.mine, icons: l.icons, run: l.runOf(), floe: l.floe, checked: l.checked,
	}
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
	unusable := slices.Concat(set.unusable(kindPart, nil), set.unusable(kindTheme, nil))
	return slices.Clone(set.strays), usererr.OfWhole(unusable).Err()
}

// checkFiles checks the files of t apart from any data: what makes it unusable, as a data file's check finds it
// (fileMistakes, what Template.unusable tells), and what template check alone checks besides: page.html's <head>
// and doctype, the parts and images it names, the paths of checks.yaml against the schema and the colours of
// page.css. A check of what another made (page.html as parsed, the schema as compiled) is left out while that
// has mistakes.
func (t *Template) checkFiles() (usererr.List, error) {
	if err := t.missingFiles(); err != nil {
		return nil, err
	}
	set, err := t.parts()
	if err != nil {
		return nil, err
	}
	unusable := t.fileMistakes()
	read, _ := t.compileShape() //nolint:errcheck // told by unusable
	var warnings usererr.List
	if read.shape != nil {
		warnings = read.notes.warnings
	}
	list, _ := t.readChecks() //nolint:errcheck // told by unusable
	declared := checksDeclared(list, read.compiled, false)
	checked, assetsErr := t.checkAssets()
	p, _ := t.page() //nolint:errcheck // told by unusable
	var named []namedPart
	var pageErr error
	if p.src != nil {
		named = namedParts(string(p.src))
		pageErr = t.checkPage(set, read.shape, checked, p, named)
	}
	scope, cssErr := t.checkCSS()
	inSchema, err := set.unusableRefs(read.notes)
	if err != nil {
		return nil, err
	}
	// A copy: the schema's warnings are those its run keeps.
	warnings = slices.Concat(warnings, scope, brokenParts(set, named, inSchema))
	return warnings, usererr.Collect(unusable, declared, pageErr, assetsErr, cssErr)
}

// checkPage checks p, the page.html of t as read (Template.page), which names the parts named of set, for
// what template check alone checks: its <head> and doctype, the parts it names, and, parsed, the images it
// names to asset but those checked (checkAssets). s is the schema of t, nil when it cannot be read.
func (t *Template) checkPage(set *partSet, s *shape, checked map[string]bool, p parsedPage, named []namedPart) error {
	var assets error
	if p.page != nil {
		assets = t.checkPageAssets(p.page, checked)
	}
	return usererr.Collect(t.checkHead(s, p), t.checkNamedParts(set, p.src, named), assets)
}

// checkHead refuses src, page.html, without a <head>, and one without a standard doctype where s, the
// schema of t, nil when it cannot be read, has an item of format html.
func (t *Template) checkHead(s *shape, p parsedPage) error {
	var doctype, head error
	if s != nil {
		doctype = doctypeMistake(t.file("page.html"), s, p.src)
	}
	found := tagsIn(string(p.src), true)
	switch {
	case tagsOf(found).head < 0:
		head = usererr.NewAt(t.file("page.html"), 0, 0, "tmpl_page_head")
	case p.page != nil && !headDrawn(p.page, found):
		head = usererr.NewAt(t.file("page.html"), lineAt(p.src, tagsOf(found).head), 0, "tmpl_page_head_action")
	}
	return usererr.Collect(doctype, head)
}

// headDrawn reports whether page, parsed from the text whose tags are found, draws a </head> whatever the
// data: one in the text of the page itself, outside every action ({{if}}, {{with}}, {{range}}, {{define}}),
// which is drawn as it is.
func headDrawn(page *template.Template, found []tag) bool {
	for _, n := range page.Tree.Root.Nodes {
		text, ok := n.(*tparse.TextNode)
		if !ok {
			continue
		}
		from, to := int(text.Pos), int(text.Pos)+len(text.Text)
		for _, tg := range found {
			if tg.end && tg.name == "head" && tg.start >= from && tg.start < to {
				return true
			}
		}
	}
	return false
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

// brokenParts warns of each part in set that cannot be used, but those page.html names by an attribute, of
// named, which checkNamedParts refuses, and those the schema names, inSchema, which its $refs are refused
// for: the schema compiled with each standing as false, so it reaches the others through floe's schemas
// alone, as any.json, which refuse the data that takes them where it does (schemaCheck.unusablePart). A
// theme that cannot be used is warned of too: a data file picks it.
func brokenParts(set *partSet, named []namedPart, inSchema map[string]bool) usererr.List {
	refused := maps.Clone(inSchema)
	for _, n := range named {
		if !n.text {
			refused[n.name] = true
		}
	}
	// floe's stays in place of one of a name set keeps (diagram), which page.html may name.
	warnings := usererr.OfWhole(slices.Concat(
		set.unusable(kindPart, func(name string) bool { return refused[name] && set.parts[name] == nil }),
		set.unusable(kindTheme, nil),
	))
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
		return nil, nil //nolint:nilerr // told by unusable
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
			mistakes = append(mistakes, set.brokenErr(kindPart, n.name).Placed(page, line, 0, ""))
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
		_, err := t.assets()(name)
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

// checksDeclared returns the mistakes of the paths of list that the compiled schema s, of a part when part
// is true, does not declare (declared): none without s.
func checksDeclared(list []relation, s *jsonschema.Schema, part bool) error {
	if s == nil {
		return nil
	}
	var problems usererr.List
	for _, c := range list {
		if e := c.declared(s, part); e != nil {
			problems = append(problems, e)
		}
	}
	return problems.Err()
}

// declared checks that the compiled schema s, of a part when part is true, else of a template, declares
// the list at c.For and c.Field under its items, or c.Field without for: a path misspelt would find
// nothing in the data to check, and fail no data. A path is refused only where the schema surely refuses
// it, a list only where its types leave array out. A part's value may be one or a list of them, so its
// path is refused only where every kind of value s takes surely refuses it.
func (c relation) declared(s *jsonschema.Schema, part bool) *usererr.Error {
	sides := []*jsonschema.Schema{s}
	if part && takes(s, "array") {
		sides = []*jsonschema.Schema{itemsOf(s)}
		if takes(s, "object") {
			sides = append(sides, s)
		}
	}
	var first *usererr.Error
	for _, side := range sides {
		e := c.declaredIn(side, part)
		if e == nil {
			return nil
		}
		if first == nil {
			first = e
		}
	}
	return first
}

// declaredIn checks the paths of c where the value is of the compiled schema s, or anything without s.
func (c relation) declaredIn(s *jsonschema.Schema, part bool) *usererr.Error {
	if c.For == "" {
		if c.Field == "" {
			return nil
		}
		if field, sure := declaredAt(s, strings.Split(c.Field, "."), false); field != nil || !sure {
			return nil
		}
		if part {
			return yamlpos.New(c.file, c.at["field"], "tmpl_checks_field_part", c.Field)
		}
		return yamlpos.New(c.file, c.at["field"], "tmpl_checks_field_whole", c.Field)
	}
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
