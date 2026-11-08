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
// LIMIT: whatever cannot be told right is "-", which floe part show <name> schema.json is read for: a branch, a key a
// pattern or a schema takes, more than one type, a list's items in a list, and the like.
func (c *Compiled) Shape(s *jsonschema.Schema, more ...string) string {
	if c == nil || s == nil {
		return "-"
	}
	return c.shape(s, more, true)
}

func (c *Compiled) shape(s *jsonschema.Schema, more []string, top bool) string {
	st, through := c.step(s, map[*jsonschema.Schema]bool{})
	types := map[string]bool{}
	one := false
	for _, n := range through {
		if !c.plain(n) {
			return "-"
		}
		if n.Types != nil {
			for _, t := range n.Types.ToStrings() {
				types[t] = true
			}
		}
		one = one || n.MaxProperties != nil
	}
	typed := func(t string) bool { return len(types) == 0 || len(types) == 1 && types[t] }
	switch {
	case st.Part != "":
		return PartName(st.Part)
	case st.Format != "":
		return st.Format
	case len(st.Props) > 0 && st.Items == nil && typed("object") && required(through, st.Props):
		keys := strings.Join(slices.Sorted(slices.Values(slices.Concat(slices.Collect(maps.Keys(st.Props)), more))), ",")
		if one {
			return "one of " + keys
		}
		return keys
	case len(st.Props) == 0 && st.Items != nil && typed("array") && top:
		if items := c.shape(st.Items, nil, false); items != "-" {
			return "[] " + items
		}
	}
	return "-"
}

// plain reports whether s keeps to what Shape takes: no field shapeFields refuses or lacks, and its allowed ones
// within their rules. Its required keys are matched against the properties later, as a $ref may hold them.
func (c *Compiled) plain(s *jsonschema.Schema) bool {
	v := reflect.ValueOf(s).Elem()
	for i := range v.NumField() {
		f := v.Type().Field(i)
		if !f.IsExported() || v.Field(i).IsZero() {
			continue
		}
		if use, ok := shapeFields[f.Name]; !ok || use == fieldRefused {
			return false
		}
	}
	if s.Types != nil && len(s.Types.ToStrings()) > 1 {
		return false
	}
	if _, ok := s.AdditionalProperties.(*jsonschema.Schema); ok {
		return false
	}
	if _, ok := s.Items.([]*jsonschema.Schema); ok {
		return false
	}
	if s.MaxItems != nil && *s.MaxItems == 0 {
		return false
	}
	one := func(n *int) bool { return n != nil && *n == 1 }
	if (s.MaxProperties != nil || s.MinProperties != nil) && !(one(s.MaxProperties) && one(s.MinProperties)) {
		return false
	}
	if s.Ref != nil {
		if _, part := c.partRoot(s.Ref); !part && !c.followed(s, s.Ref) {
			return false
		}
	}
	return true
}

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
