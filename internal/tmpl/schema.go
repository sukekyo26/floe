package tmpl

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/goccy/go-yaml/ast"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// formats are the formats floe gives a meaning to, beyond JSON Schema's own: how to read a text, as
// HTML, the path of an image or Markdown. Validating them is a no-op: they mark an item for drawing.
var formats = []string{"html", "image", "markdown"}

// validate checks the values of d against the schema src, its template's schema.json, and reports
// every mistake at its line and column under root, the data file's top mapping.
func validate(set *partSet, doc any, d *Data, root ast.Node) error {
	sch, err := compileSchema(set, d.Template, doc)
	if err != nil {
		return err
	}
	err = sch.Validate(d.Values)
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return err //nolint:wrapcheck // nil, or not a mistake in the data
	}
	s := &schemaCheck{file: d.File, template: d.Template, root: root, schema: doc, set: set, problems: nil}
	s.collect(ve)
	if len(s.problems) == 0 {
		// A failure without a mistake to point at is not one the data can fix.
		return fmt.Errorf("check %s against its schema: %w", d.File, ve)
	}
	return s.problems
}

// compileSchema compiles doc, the schema of the template name as jsonschema reads it, which refers to the
// parts of set.
func compileSchema(set *partSet, name string, doc any) (*jsonschema.Schema, error) {
	if err := checkDraft(doc); err != nil {
		return nil, usererr.New("tmpl_schema_load", name, err)
	}
	url := schemaURL(name)
	c := newCompiler(set)
	if err := c.AddResource(url, doc); err != nil {
		return nil, fmt.Errorf("read the schema of %s: %w", name, err)
	}
	for partURL, partDoc := range set.docs {
		if err := c.AddResource(partURL, partDoc); err != nil {
			return nil, fmt.Errorf("read the schema %s: %w", partURL, err)
		}
	}
	sch, err := c.Compile(url)
	// jsonschema wraps noLoader's mistake in an error of its own, which does not unwrap: floe's own is told,
	// so that it is worded in the reader's language.
	// partSet.mistakes told a $ref's at its pointer before; those it does not see (its LIMIT) name the template.
	var load *jsonschema.LoadURLError
	switch {
	case errors.As(err, &load):
		return nil, usererr.New("tmpl_schema_load", name, load.Err)
	case err != nil:
		return nil, fmt.Errorf("compile the schema of %s: %w", name, err)
	}
	return sch, nil
}

// drafts are the drafts of JSON Schema a $schema may name.
var drafts = []*jsonschema.Draft{
	jsonschema.Draft4, jsonschema.Draft6, jsonschema.Draft7, jsonschema.Draft2019, jsonschema.Draft2020,
}

// checkDraft refuses the schema doc when its $schema names no draft of JSON Schema: jsonschema would take a
// schema of floe's there, a part's, as the rules of the schema, and load any other.
func checkDraft(doc any) error {
	m, ok := doc.(map[string]any)
	if !ok {
		return nil
	}
	named, ok := m["$schema"].(string)
	if !ok || isDraft(named) {
		return nil
	}
	return usererr.New("tmpl_schema_draft", named)
}

// isDraft reports whether url names a draft of JSON Schema, as jsonschema reads it, which it holds itself.
func isDraft(url string) bool {
	return slices.ContainsFunc(drafts, func(d *jsonschema.Draft) bool { return sameDraft(d.String(), url) }) ||
		sameDraft("https://json-schema.org/schema", url)
}

// sameDraft reports whether the $schema named is the draft at url, as jsonschema reads it: by http or https,
// with an empty fragment or none.
func sameDraft(url, named string) bool {
	bare := func(u string) string {
		u = strings.TrimSuffix(u, "#")
		if rest, ok := strings.CutPrefix(u, "http://"); ok {
			return rest
		}
		return strings.TrimPrefix(u, "https://")
	}
	return bare(url) == bare(named)
}

// schemaURL is the URL of the schema of the template name, which a relative $ref is read from.
func schemaURL(name string) string {
	return "floe:///templates/" + name + "/schema.json"
}

// newCompiler returns a compiler of a schema that refers to the parts of set.
func newCompiler(set *partSet) *jsonschema.Compiler {
	c := jsonschema.NewCompiler()
	// A template's schema is one file: a URL it refers to, file:// above all, could read one out of the
	// template.
	c.UseLoader(noLoader{set: set})
	c.AssertFormat()
	for _, f := range formats {
		c.RegisterFormat(&jsonschema.Format{Name: f, Validate: func(any) error { return nil }})
	}
	return c
}

// knownFormat reports whether floe knows the format f: one of JSON Schema's that it checks, or one of
// its own. jsonschema passes a value of a format it does not know, so a template that uses a format of
// a later floe would be drawn as if it had none. Each is asked once, as every schema read asks again.
func knownFormat(f string) bool {
	if known, ok := knownFormats.Load(f); ok {
		b, isBool := known.(bool)
		return b && isBool
	}
	s, err := compileAlone(map[string]any{"format": f})
	known := err == nil && s.Format != nil
	knownFormats.Store(f, known)
	return known
}

// knownFormats are the formats knownFormat was asked of, by name, and whether floe knows each.
var knownFormats sync.Map

// heldByJSONSchema reports whether jsonschema holds the schema at url itself, as it holds the drafts and
// their vocabularies, so that a $ref to it reads no other file.
func heldByJSONSchema(url string) bool {
	_, err := compileAlone(map[string]any{"$ref": url})
	return err == nil
}

// compileAlone compiles the schema doc, which refers to no part: none are given, and any other file it
// refers to is refused as it is in a template.
func compileAlone(doc any) (*jsonschema.Schema, error) {
	const url = "floe:///alone.json"
	c := newCompiler(nil)
	if err := c.AddResource(url, doc); err != nil {
		return nil, fmt.Errorf("read a schema alone: %w", err)
	}
	s, err := c.Compile(url)
	if err != nil {
		return nil, fmt.Errorf("compile a schema alone: %w", err)
	}
	return s, nil
}

// unknownPart is the mistake of a $ref, ref, under floe:///parts/ that names no part of set whole.
func unknownPart(set *partSet, ref string) error {
	if whole, into := set.pointsInto(ref); into {
		return usererr.New("tmpl_part_into", ref, whole)
	}
	name, ok := partName(ref)
	switch {
	case ok && set.broken[name] != nil:
		return usererr.New("tmpl_part_broken", name, set.broken[name])
	case ok && set.parts[name] != nil && set.parts[name].isLook():
		return usererr.New("tmpl_part_look_ref", ref, name)
	}
	return usererr.New("tmpl_part_unknown", ref, set.names(drawing))
}

// noLoader loads no schema: those of the parts of set are added to the compiler before.
type noLoader struct{ set *partSet }

func (n noLoader) Load(url string) (any, error) {
	if strings.HasPrefix(url, partsURL) {
		return nil, unknownPart(n.set, url)
	}
	return nil, usererr.New("tmpl_schema_url", url)
}

type schemaCheck struct {
	file, template string
	root           ast.Node
	schema         any      // schema.json as read, to list the items a mapping may hold
	set            *partSet // the parts it refers to
	problems       yamlpos.Problems
}

// collect turns the leaves of e into problems. A branch of anyOf or oneOf would say why each choice
// failed; the item is reported once instead.
func (s *schemaCheck) collect(e *jsonschema.ValidationError) {
	switch e.ErrorKind.(type) {
	case *kind.Schema, *kind.Group, *kind.AllOf, *kind.Reference:
		for _, c := range e.Causes {
			s.collect(c)
		}
		return
	}
	key, value, at := yamlpos.Find(s.root, e.InstanceLocation)
	where := yamlpos.OfValue(key, value)
	switch k := e.ErrorKind.(type) {
	case *kind.Required:
		for _, name := range k.Missing {
			s.add(where, "tmpl_schema_required", join(at, name))
		}
	case *kind.AdditionalProperties:
		s.unknown(e, k.Properties, where)
	case *kind.Type:
		args := []any{label(at), usererr.New("tmpl_type_" + k.Got), Types(k.Want)}
		if k.Got == "object" && slices.Contains(k.Want, "string") && yamlpos.ColonText(value) {
			s.problems = append(s.problems, yamlpos.NewHinted(s.file, where, "yaml_hint_colon", "tmpl_schema_type", args...))
			return
		}
		s.add(where, "tmpl_schema_type", args...)
	case *kind.Enum:
		want := make([]string, len(k.Want))
		for i, w := range k.Want {
			want[i] = jsonText(w)
		}
		if k.Got == nil {
			s.add(where, "tmpl_schema_enum_empty", label(at), strings.Join(want, ", "))
			return
		}
		s.add(where, "tmpl_schema_enum", label(at), jsonText(k.Got), strings.Join(want, ", "))
	default:
		if key, args, ok := wording(e.ErrorKind); ok {
			s.add(where, key, append([]any{label(at)}, args...)...)
			return
		}
		// LIMIT: the rest (anyOf, oneOf, const, dependencies…) keep the library's English. Word one
		// in the catalogs when a template's schema comes to use it.
		s.add(where, "tmpl_item", label(at), e.ErrorKind.LocalizedString(english))
	}
}

// wording words a mistake the catalog has a message for, after the path: a bound (how many items or
// keys, how long, how large), a pattern or a duplicate.
func wording(k jsonschema.ErrorKind) (key string, args []any, ok bool) {
	switch k := k.(type) {
	case *kind.MinItems:
		return "tmpl_schema_min_items", []any{k.Want}, true
	case *kind.MaxItems:
		return "tmpl_schema_max_items", []any{k.Want}, true
	case *kind.MinProperties:
		return "tmpl_schema_min_keys", []any{k.Want}, true
	case *kind.MaxProperties:
		return "tmpl_schema_max_keys", []any{k.Want}, true
	case *kind.MinLength:
		return "tmpl_schema_min_length", []any{k.Want}, true
	case *kind.MaxLength:
		return "tmpl_schema_max_length", []any{k.Want}, true
	case *kind.Minimum:
		return "tmpl_schema_minimum", []any{number(k.Want)}, true
	case *kind.Maximum:
		return "tmpl_schema_maximum", []any{number(k.Want)}, true
	case *kind.ExclusiveMinimum:
		return "tmpl_schema_above", []any{number(k.Want)}, true
	case *kind.ExclusiveMaximum:
		return "tmpl_schema_below", []any{number(k.Want)}, true
	case *kind.Pattern:
		return "tmpl_schema_pattern", []any{k.Got, k.Want}, true
	case *kind.UniqueItems:
		// The library counts from 0; people count items from 1.
		return "tmpl_schema_unique", []any{k.Duplicates[0] + 1, k.Duplicates[1] + 1}, true
	}
	return "", nil, false
}

var english = message.NewPrinter(language.English)

// unknown reports the items of the mapping at e that the template does not have, each at its key
// (or at the mapping, where), with the items it has.
func (s *schemaCheck) unknown(e *jsonschema.ValidationError, extras []string, where yamlpos.Pos) {
	names := s.properties(e.SchemaURL)
	if len(e.InstanceLocation) == 0 {
		names = append(names, shared...)
		slices.Sort(names)
	}
	for _, name := range extras {
		extra, _, path := yamlpos.Find(s.root, append(slices.Clone(e.InstanceLocation), name))
		at := where
		if extra != nil {
			at = yamlpos.Of(extra)
		}
		if len(names) == 0 {
			// Items the schema allows by pattern or through $ref have no list to give.
			s.add(at, "tmpl_schema_unknown_bare", path, s.template)
			continue
		}
		s.add(at, "tmpl_schema_unknown", path, s.template, strings.Join(names, ", "))
	}
}

func (s *schemaCheck) add(at yamlpos.Pos, key string, args ...any) {
	s.problems = append(s.problems, yamlpos.New(s.file, at, key, args...))
}

// properties lists the items the mapping at the schema location url declares, for a misspelt one: in
// the template's schema, or in a part's of floe's.
func (s *schemaCheck) properties(url string) []string {
	base, frag, _ := strings.Cut(url, "#")
	v := s.schema
	if strings.HasPrefix(base, partsURL) {
		v = s.set.docs[base]
	}
	for step := range strings.SplitSeq(frag, "/") {
		if step == "" {
			continue
		}
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[pointerStep.Replace(step)]
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	if parent, ok := m["properties"].(map[string]any); ok {
		names := make([]string, 0, len(parent))
		for n := range parent {
			names = append(names, n)
		}
		slices.Sort(names)
		return names
	}
	return nil
}

// pointerStep unescapes a step of a JSON pointer.
var pointerStep = strings.NewReplacer("~1", "/", "~0", "~")

// join adds name to the path at.
func join(at, name string) string {
	if at == "" {
		return name
	}
	return at + "." + name
}

// label names the item at in a message: its path, or the top level.
func label(at string) any {
	if at == "" {
		return usererr.New("tmpl_top_level")
	}
	return at
}

func number(r *big.Rat) string {
	if r.IsInt() {
		return r.Num().String()
	}
	f, _ := r.Float64()
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// Types words JSON types (string, array…) as alternatives in a message: text or a number. A name
// JSON does not have stays as written.
func Types(types []string) i18n.Localizer {
	words := make(orList, len(types))
	for i, t := range types {
		words[i] = t
		if slices.Contains(jsonTypes, t) {
			words[i] = usererr.New("tmpl_type_" + t)
		}
	}
	return words
}

var jsonTypes = []string{"array", "boolean", "integer", "null", "number", "object", "string"}

// jsonText writes a value of JSON as JSON does, text as it is: null rather than Go's <nil>.
func jsonText(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

// andList words its items as a list: page.html, schema.json.
type andList []string

func (l andList) Localize(c *i18n.Catalog) string {
	return strings.Join(l, c.Msg("tmpl_and"))
}

// orList words its items as alternatives: text or a number.
type orList []any

func (l orList) Localize(c *i18n.Catalog) string {
	words := make([]string, len(l))
	for i, w := range l {
		if loc, ok := w.(i18n.Localizer); ok {
			w = loc.Localize(c)
		}
		words[i] = fmt.Sprint(w)
	}
	return strings.Join(words, c.Msg("tmpl_or"))
}
