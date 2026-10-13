package tmpl

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"math/big"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// formats are the formats floe gives a meaning to, beyond JSON Schema's own. Validating them is a
// no-op: they mark an item for drawing, and what an item holds is checked where it is drawn.
var formats = []string{"html"}

// checkSchema checks the values of d against its template's schema.json and reports every mistake at
// its line and column under root, the data file's top mapping.
func checkSchema(d *Data, root ast.Node) error {
	src, err := builtin.ReadFile(path.Join("builtin", d.Template, "schema.json"))
	if err != nil {
		return fmt.Errorf("read the schema of %s: %w", d.Template, err)
	}
	return validate(src, d, root)
}

// validate checks the values of d against the schema src.
func validate(src []byte, d *Data, root ast.Node) error {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(src))
	if err != nil {
		return fmt.Errorf("read the schema of %s: %w", d.Template, err)
	}
	url := "floe:///templates/" + d.Template + "/schema.json"
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	for _, f := range formats {
		c.RegisterFormat(&jsonschema.Format{Name: f, Validate: func(any) error { return nil }})
	}
	if err = c.AddResource(url, doc); err != nil {
		return fmt.Errorf("read the schema of %s: %w", d.Template, err)
	}
	sch, err := c.Compile(url)
	if err != nil {
		return fmt.Errorf("compile the schema of %s: %w", d.Template, err)
	}
	err = sch.Validate(d.Values)
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return err //nolint:wrapcheck // nil, or not a mistake in the data
	}
	s := &schemaCheck{file: d.File, template: d.Template, root: root, schema: doc, problems: nil}
	s.collect(ve)
	slices.SortStableFunc(s.problems, func(x, y *yamlpos.Error) int {
		return cmp.Or(cmp.Compare(x.Pos.Line, y.Pos.Line), cmp.Compare(x.Pos.Column, y.Pos.Column))
	})
	return s.problems
}

type schemaCheck struct {
	file, template string
	root           ast.Node
	schema         any // schema.json as read, to list the items a mapping may hold
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
	_, value, at := yamlpos.Find(s.root, e.InstanceLocation)
	switch k := e.ErrorKind.(type) {
	case *kind.Required:
		for _, name := range k.Missing {
			s.add(value, "tmpl_schema_required", join(at, name))
		}
	case *kind.AdditionalProperties:
		names := s.properties(e.SchemaURL)
		if len(e.InstanceLocation) == 0 {
			names = append(names, shared...)
			slices.Sort(names)
		}
		allowed := strings.Join(names, ", ")
		for _, name := range k.Properties {
			extra, _, where := yamlpos.Find(s.root, append(slices.Clone(e.InstanceLocation), name))
			s.add(extra, "tmpl_schema_unknown", where, s.template, allowed)
		}
	case *kind.Type:
		want := make(orList, len(k.Want))
		for i, t := range k.Want {
			want[i] = usererr.New("tmpl_type_" + t)
		}
		s.add(value, "tmpl_schema_type", label(at), usererr.New("tmpl_type_"+k.Got), want)
	case *kind.Enum:
		want := make([]string, len(k.Want))
		for i, w := range k.Want {
			want[i] = fmt.Sprint(w)
		}
		s.add(value, "tmpl_schema_enum", label(at), fmt.Sprint(k.Got), strings.Join(want, ", "))
	default:
		if key, args, ok := wording(e.ErrorKind); ok {
			s.add(value, key, append([]any{label(at)}, args...)...)
			return
		}
		// The rest are rare in a template's schema; their wording stays the library's English.
		s.add(value, "tmpl_schema_other", label(at), e.ErrorKind.LocalizedString(english))
	}
}

// wording words a mistake the catalog has a message for, after the path: a bound (how many items or
// keys, how long, how large), a pattern, a duplicate or a number JSON cannot hold.
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
	case *kind.InvalidJsonValue:
		// .inf and .nan: YAML has them, JSON does not.
		return "tmpl_schema_not_finite", []any{fmt.Sprint(k.Value)}, true
	}
	return "", nil, false
}

var english = message.NewPrinter(language.English)

func (s *schemaCheck) add(n ast.Node, key string, args ...any) {
	s.problems = append(s.problems, yamlpos.New(s.file, yamlpos.Of(n), key, args...))
}

// properties lists the items the mapping at the schema location url declares, for a misspelt one.
func (s *schemaCheck) properties(url string) []string {
	_, frag, _ := strings.Cut(url, "#")
	v := s.schema
	for step := range strings.SplitSeq(frag, "/") {
		if step == "" {
			continue
		}
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[strings.NewReplacer("~1", "/", "~0", "~").Replace(step)]
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
