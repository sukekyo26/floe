package schema

import (
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// fieldUse is what Shape makes of a field of jsonschema.Schema.
type fieldUse int

const (
	fieldSkipped fieldUse = iota // not a keyword, or a note that leaves the shape as it is
	fieldAllowed                 // taken when it keeps to allowed's rules
	fieldRefused                 // a value of it makes the shape "-"
)

// shapeFields sorts every exported field of jsonschema.Schema (v6.0.3). A field it lacks is refused, so that one a
// newer library adds makes the shape "-" until it is sorted here (TestShapeFields).
var shapeFields = map[string]fieldUse{
	"DraftVersion": fieldSkipped, "Location": fieldSkipped, "ID": fieldSkipped, "Anchor": fieldSkipped,
	"DynamicAnchor": fieldSkipped, "RecursiveAnchor": fieldSkipped, "Title": fieldSkipped,
	"Description": fieldSkipped, "Default": fieldSkipped, "Comment": fieldSkipped, "ReadOnly": fieldSkipped,
	"WriteOnly": fieldSkipped, "Examples": fieldSkipped, "Deprecated": fieldSkipped,

	"Ref": fieldAllowed, "Types": fieldAllowed, "Format": fieldAllowed, "Properties": fieldAllowed,
	"Required": fieldAllowed, "AdditionalProperties": fieldAllowed, "MaxProperties": fieldAllowed,
	"MinProperties": fieldAllowed, "Items": fieldAllowed, "Items2020": fieldAllowed, "MinItems": fieldAllowed,
	"MaxItems": fieldAllowed, "UniqueItems": fieldAllowed, "MinLength": fieldAllowed, "MaxLength": fieldAllowed,
	"Pattern": fieldAllowed,

	"Bool": fieldRefused, "RecursiveRef": fieldRefused, "DynamicRef": fieldRefused, "Enum": fieldRefused,
	"Const": fieldRefused, "Not": fieldRefused, "AllOf": fieldRefused, "AnyOf": fieldRefused, "OneOf": fieldRefused,
	"If": fieldRefused, "Then": fieldRefused, "Else": fieldRefused, "PropertyNames": fieldRefused,
	"PatternProperties": fieldRefused, "Dependencies": fieldRefused, "DependentRequired": fieldRefused,
	"DependentSchemas": fieldRefused, "UnevaluatedProperties": fieldRefused, "Contains": fieldRefused,
	"MinContains": fieldRefused, "MaxContains": fieldRefused, "AdditionalItems": fieldRefused,
	"PrefixItems": fieldRefused, "UnevaluatedItems": fieldRefused, "ContentEncoding": fieldRefused,
	"ContentMediaType": fieldRefused, "ContentSchema": fieldRefused, "Maximum": fieldRefused,
	"Minimum": fieldRefused, "ExclusiveMaximum": fieldRefused, "ExclusiveMinimum": fieldRefused,
	"MultipleOf": fieldRefused, "Extensions": fieldRefused,
}

// Shape tells what a value of s is written as, for the lists of floe part list and the like: its keys by name
// (with more, the keys floe reads itself, for a template), "one of " before them when exactly one is written, "[] "
// before the shape of a list's items, the name of the format or of the part it is; or "-" when the list cannot tell
// it right: a field shapeFields refuses, or an allowed one beyond its rules, on any schema Follow goes through.
// LIMIT: whatever cannot be told right is "-", for which the files floe part show <name> names are read: a branch, a
// key a pattern or a schema takes, more than one type, a list's items in a list, and the like.
// LIMIT: of a schema of one's own, Shape tells right as far as Follow and shapeFields see; a rarer form may not be
// "-". TestShapes keeps floe's parts and templates right, every one of them.
func (c *Compiled) Shape(s *jsonschema.Schema, more ...string) string {
	if c == nil || s == nil {
		return "-"
	}
	return c.shapeAt(s, more, true)
}

func (c *Compiled) shapeAt(s *jsonschema.Schema, more []string, top bool) string {
	st, through := c.stepClosed(s)
	if slices.ContainsFunc(through, func(n *jsonschema.Schema) bool { return !c.plain(n) }) {
		return "-"
	}
	switch {
	case st.Part != "":
		return PartName(st.Part)
	case st.Format != "":
		return st.Format
	case st.Items == nil && (len(st.Props) > 0 || len(more) > 0) && typed(through, "object"):
		return c.keysShape(st, through, more)
	case len(st.Props) == 0 && st.Items != nil && typed(through, "array") && top:
		if items := c.shapeAt(st.Items, nil, false); items != "-" {
			return "[] " + items
		}
	}
	return "-"
}

// plain reports whether s keeps to what Shape takes: no field shapeFields refuses or lacks, and its allowed ones
// within their rules. Its required keys are matched against the properties later, as a $ref may hold them.
func (c *Compiled) plain(s *jsonschema.Schema) bool {
	return !refused(s) && withinRules(s) && (s.Ref == nil || c.reached(s))
}

// refused reports whether s has a field shapeFields refuses or does not sort.
func refused(s *jsonschema.Schema) bool {
	v := reflect.ValueOf(s).Elem()
	for i := range v.NumField() {
		f := v.Type().Field(i)
		if !f.IsExported() || v.Field(i).IsZero() {
			continue
		}
		if use, ok := shapeFields[f.Name]; !ok || use == fieldRefused {
			return true
		}
	}
	return false
}

// withinRules reports whether the allowed fields of s keep to what Shape can tell: one type, additionalProperties
// true or false, items of one schema, a list that may hold an item, and exactly one key or no bound on the keys.
func withinRules(s *jsonschema.Schema) bool {
	_, rest := s.AdditionalProperties.(*jsonschema.Schema)
	_, tuple := s.Items.([]*jsonschema.Schema)
	one := func(n *int) bool { return n != nil && *n == 1 }
	bounded := s.MaxProperties != nil || s.MinProperties != nil
	return (s.Types == nil || len(s.Types.ToStrings()) <= 1) && !rest && !tuple &&
		(s.MaxItems == nil || *s.MaxItems != 0) && (!bounded || one(s.MaxProperties) && one(s.MinProperties))
}

// reached reports whether the $ref of s names a part's root or what Follow goes on to.
func (c *Compiled) reached(s *jsonschema.Schema) bool {
	_, part := c.partRoot(s.Ref)
	return part || c.followed(s, s.Ref)
}

// typed reports whether the schemas gone through name no type, or t alone.
func typed(through []*jsonschema.Schema, t string) bool {
	types := map[string]bool{}
	for _, s := range through {
		if s.Types != nil {
			for _, name := range s.Types.ToStrings() {
				types[name] = true
			}
		}
	}
	return len(types) == 0 || len(types) == 1 && types[t]
}

// keysShape is the shape of an object of the step st, with more keys floe reads itself: its keys by name, "one of "
// before them when exactly one is written; "-" when a key would be read wrong or not be written as listed: a name the
// list would read as something else, a key whose schema Shape does not take (one Follow leads to a field shapeFields
// refuses), a key a closed schema forbids (Step.Forbids), a key required that is not there, or exactly one written
// where other keys are not forbidden, or beside required keys or more.
func (c *Compiled) keysShape(st Step, through []*jsonschema.Schema, more []string) string {
	for name, p := range st.Props {
		if listMisreads(name) || st.Forbids(name) || c.unwritable(p) {
			return "-"
		}
	}
	if !required(through, st.Props) {
		return "-"
	}
	names := slices.Sorted(slices.Values(slices.Concat(slices.Collect(maps.Keys(st.Props)), more)))
	if len(names) == 1 && (slices.Contains(Formats, names[0]) || c.held("floe:///parts/"+names[0]+".json", Part)) {
		return "-"
	}
	keys := strings.Join(names, ",")
	if !slices.ContainsFunc(through, func(s *jsonschema.Schema) bool { return s.MaxProperties != nil }) {
		return keys
	}
	if len(more) > 0 || len(st.closed) == 0 ||
		slices.ContainsFunc(through, func(s *jsonschema.Schema) bool { return len(s.Required) > 0 }) {
		return "-"
	}
	return "one of " + keys
}

// listMisreads reports whether the list would read a key of name as something else: empty, holding what parts the
// columns or the keys, or written as another shape starts. A key alone named as a format or a part is the caller's.
func listMisreads(name string) bool {
	return name == "" || name == "-" || strings.ContainsAny(name, "\t\n\r,") || strings.HasPrefix(name, "[] ") ||
		strings.HasPrefix(name, "one of ")
}

// unwritable reports whether a key of schema p may be one no value is written for: a schema Follow goes through from
// p is false, holds an empty enum, a field of keyRefused, or a $ref Follow does not take. What narrows the value
// (its type, a bound, a pattern, an enum of values) leaves the key written; the shape of the value is not listed.
// LIMIT: a key whose narrowing takes no value (minimum over maximum, a type its enum is not) is listed; read show.
func (c *Compiled) unwritable(p *jsonschema.Schema) bool {
	_, through := c.stepClosed(p)
	return slices.ContainsFunc(through, func(n *jsonschema.Schema) bool {
		v := reflect.ValueOf(n).Elem()
		return n.Bool != nil && !*n.Bool || n.Enum != nil && len(n.Enum.Values) == 0 ||
			slices.ContainsFunc(keyRefused, func(f string) bool { return !v.FieldByName(f).IsZero() }) ||
			n.Ref != nil && !c.reached(n)
	})
}

// keyRefused are the fields of jsonschema.Schema that join or turn over schemas, by which a key's schema may take no
// value: unwritable makes the shape "-" for them.
var keyRefused = []string{"Not", "AllOf", "AnyOf", "OneOf", "If", "Then", "Else", "RecursiveRef", "DynamicRef"}

// required reports whether every key required on the schemas gone through is one of props.
func required(through []*jsonschema.Schema, props map[string]*jsonschema.Schema) bool {
	for _, s := range through {
		for _, key := range s.Required {
			if _, ok := props[key]; !ok {
				return false
			}
		}
	}
	return true
}
