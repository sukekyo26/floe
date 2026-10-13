package tmpl

import (
	"cmp"
	"errors"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// Check checks the template at dir as a template of the directory's name, whatever its example.yaml
// names: its files, then its example.yaml through every stage of a publish, drawing included. The
// mistakes stop it from being drawn, and the warnings do not.
func Check(dir string) (warnings []*usererr.Error, mistakes []error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, []error{usererr.CannotRead(dir, err)}
	}
	name := filepath.Base(abs)
	if !validName(name) {
		return nil, []error{usererr.New("tmpl_new_name", name)}
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, []error{usererr.CannotRead(dir, err)}
	}
	t := &Template{Name: name, Dir: resolved, Path: dir}
	// The floe a template needs comes before its mistakes, which another floe may not make.
	needs, err := t.needs()
	if err != nil {
		return nil, []error{err}
	}
	have, err := thisFloe()
	if err != nil {
		return nil, []error{err}
	}
	if needs == nil {
		warnings = append(warnings, usererr.New("tmpl_no_template_yaml", dir, have.minor()))
	} else if err = fits(name, *needs, have); err != nil {
		return warnings, []error{err}
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
	src, err := t.readFile("schema.json")
	if err != nil {
		return nil, []error{err}
	}
	_, doc, err := compileSchema(t.Name, src)
	if err != nil {
		return nil, []error{err}
	}
	s, err := schemaOf(t)
	if err != nil {
		return nil, []error{err}
	}
	lint := &schemaLint{file: t.file("schema.json"), warnings: nil, mistakes: nil}
	lint.walk(doc, "#", true)
	warnings, mistakes = lint.warnings, lint.mistakes
	mistakes = append(mistakes, t.checkChecks(s)...)
	computed, found, err := t.readOptional("computed.yaml")
	if err == nil && found {
		_, err = readComputed(t.file("computed.yaml"), computed, slices.Sorted(maps.Keys(s.Properties)))
	}
	if err != nil {
		mistakes = append(mistakes, err)
	}
	if _, err = t.page(); err != nil {
		mistakes = append(mistakes, err)
	}
	if _, err = t.readFile("themes/" + DefaultTheme + ".css"); err != nil {
		mistakes = append(mistakes, err)
	}
	return warnings, mistakes
}

// checkChecks returns the mistakes of the checks.yaml of t, if it has one, against its schema s.
func (t *Template) checkChecks(s *schema) []error {
	src, found, err := t.readOptional("checks.yaml")
	if err != nil {
		return []error{err}
	}
	if !found {
		return nil
	}
	list, err := readChecks(t.file("checks.yaml"), src)
	if err != nil {
		return []error{err}
	}
	var mistakes []error
	for _, c := range list {
		if err = c.declared(s); err != nil {
			mistakes = append(mistakes, err)
		}
		if _, err = c.program(); err != nil {
			mistakes = append(mistakes, err)
		}
	}
	return mistakes
}

// checkExample reads the example.yaml of t as a data file of it, and draws it.
func (t *Template) checkExample() error {
	file := t.file("example.yaml")
	src, err := t.readFile("example.yaml")
	if err != nil {
		return err
	}
	d, root, where, problems, err := start(file, src, false)
	if errors.Is(err, yamlpos.ErrEmpty) {
		return yamlpos.New(file, yamlpos.Pos{Line: 1, Column: 1}, "tmpl_no_template", t.Name)
	}
	if err != nil {
		return err
	}
	// A data file started from it names this template, as floe template new writes it.
	switch {
	case d.Template == "" && problems == nil:
		problems = append(problems, yamlpos.New(file, where["template"], "tmpl_no_template", t.Name))
	case d.Template != "" && d.Template != t.Name:
		problems = append(problems, yamlpos.New(file, where["template"], "tmpl_example_name", d.Template, t.Name))
	}
	if err = t.check(d, root, "", where["theme"], problems); err != nil {
		return err
	}
	_, err = d.Render(cmp.Or(d.Title, file), "")
	return err
}

// schemaLint finds what jsonschema compiles but floe would not draw as the schema means.
type schemaLint struct {
	file     string
	warnings []*usererr.Error
	mistakes []error
}

// walk walks the schema v at the JSON pointer ptr. reached tells that prepare reaches it, through
// properties and items alone, so that a format of floe's own there marks an item for drawing.
func (l *schemaLint) walk(v any, ptr string, reached bool) {
	m, ok := v.(map[string]any)
	if !ok {
		return
	}
	if f, ok := m["format"].(string); ok {
		switch {
		case !knownFormat(f):
			l.mistakes = append(l.mistakes, usererr.New("tmpl_format_unknown", l.file, ptr, f))
		case slices.Contains(formats, f) && !reached:
			l.mistakes = append(l.mistakes, usererr.New("tmpl_format_unreached", l.file, ptr, f))
		}
	}
	_, closed := m["additionalProperties"]
	_, unevaluated := m["unevaluatedProperties"]
	if _, ok := m["properties"]; ok && reached && (!closed && !unevaluated || m["additionalProperties"] == true) {
		l.warnings = append(l.warnings, usererr.New("tmpl_schema_open", l.file, ptr))
	}
	for _, kw := range slices.Sorted(maps.Keys(m)) {
		l.keyword(kw, m[kw], ptr+"/"+pointerKey.Replace(kw), reached)
	}
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
