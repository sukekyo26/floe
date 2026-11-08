package schema_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/sukekyo26/floe/internal/schema"
)

// Shape tells the keys, the items, a format or a part only when it can tell them right, else "-".
func TestShape(t *testing.T) {
	t.Parallel()

	const a = `"properties": {"a": {"type": "string"}}`
	for _, tt := range []struct{ name, body, prop, want string }{
		{"keys by name", `"type": "object", "properties": {"b": {"type": "string"}, "a": {"type": "string"}}`, "", "a,b"},
		{"a list of Markdown", `"properties": {"x": {"type": "array", "items": {"type": "string", "format": "markdown"}}}`, "x", "[] markdown"},
		{"a part", `"$ref": "floe:///parts/note.json"`, "", "note"},
		{"keys through a $ref", `"$ref": "#/$defs/x", "$defs": {"x": {"type": "object", ` + a + `}}`, "", "a"},
		{"a list through a $ref", `"$ref": "#/$defs/l", "$defs": {"l": {"type": "array", "items": {"type": "object", ` + a + `}}}`, "", "[] a"},
		{"a list of itself", `"$ref": "#/$defs/n", "$defs": {"n": {"type": "array", "items": {"$ref": "#/$defs/n"}}}`, "", "-"},
		{"no keys", `"type": "object", "properties": {}`, "", "-"},
		{"two types", `"type": ["object", "array"], ` + a + `, "items": {"type": "string"}`, "", "-"},
		{"a branch through a $ref", `"$ref": "#/$defs/x", "$defs": {"x": {"type": "object", ` + a + `, "oneOf": [{"required": ["a"]}, {}]}}`, "", "-"},
		{"a branch", `"oneOf": [{"type": "object", ` + a + `}, {"type": "string"}]`, "", "-"},
		{"a schema of the rest", `"type": "object", ` + a + `, "additionalProperties": {"type": "string"}`, "", "-"},
		{"prefixItems", `"type": "array", "prefixItems": [{"type": "string"}], "items": {"type": "object", ` + a + `}`, "", "-"},
		{"an enum of objects", `"type": "object", ` + a + `, "enum": [{"a": "x"}]`, "", "-"},
		{"exactly one", `"type": "object", "maxProperties": 1, "minProperties": 1, "properties": {"a": {}, "b": {}}`, "", "one of a,b"},
		{"at most one", `"type": "object", "maxProperties": 1, "properties": {"a": {}, "b": {}}`, "", "-"},
		{"at least one", `"type": "object", "minProperties": 1, "properties": {"a": {}, "b": {}}`, "", "-"},
		{"at most two", `"type": "object", "maxProperties": 2, "properties": {"a": {}, "b": {}}`, "", "-"},
		{"sizes of a list", `"type": "array", "minItems": 1, "maxItems": 3, "uniqueItems": true, "items": {"type": "object", ` + a + `}`, "", "[] a"},
		{"an empty list", `"type": "array", "maxItems": 0, "items": {"type": "object", ` + a + `}`, "", "-"},
		{"a key required but not there", `"type": "object", "required": ["z"], ` + a, "", "-"},
		{"keys of a string", `"type": "string", ` + a, "", "-"},
		{"items of an object", `"type": "object", "items": {"type": "object", ` + a + `}`, "", "-"},
	} {
		c, err := schemaCompile(t, `{"$schema": "https://json-schema.org/draft/2020-12/schema", `+tt.body+`}`)
		if c == nil {
			t.Fatalf("%s: %s", tt.name, told(err))
		}
		s := c.Root
		if tt.prop != "" {
			s = c.Root.Properties[tt.prop]
		}
		if got := c.Shape(s); got != tt.want {
			t.Errorf("%s: Shape = %q, want %q", tt.name, got, tt.want)
		}
	}

	c, err := compile(t, a)
	if c == nil {
		t.Fatal(told(err))
	}
	if got := c.Shape(c.Root, "title", "lang"); got != "a,lang,title" {
		t.Errorf("Shape with more = %q, want the keys floe reads too, by name", got)
	}
	if got := (*schema.Compiled)(nil).Shape(nil); got != "-" {
		t.Errorf("Shape of no schema = %q, want -", got)
	}
}

// Every exported field of jsonschema.Schema is sorted for Shape: one the library adds or renames fails here.
func TestShapeFields(t *testing.T) {
	t.Parallel()

	want := strings.Fields(`DraftVersion Location Bool ID Ref Anchor RecursiveRef RecursiveAnchor DynamicRef DynamicAnchor
Types Enum Const Not AllOf AnyOf OneOf If Then Else Format MaxProperties MinProperties Required PropertyNames Properties
PatternProperties AdditionalProperties Dependencies DependentRequired DependentSchemas UnevaluatedProperties MinItems
MaxItems UniqueItems Contains MinContains MaxContains Items AdditionalItems PrefixItems Items2020 UnevaluatedItems
MinLength MaxLength Pattern ContentEncoding ContentMediaType ContentSchema Maximum Minimum ExclusiveMaximum
ExclusiveMinimum MultipleOf Extensions Title Description Default Comment ReadOnly WriteOnly Examples Deprecated`)
	var got []string
	for f := range slices.Values(reflect.VisibleFields(reflect.TypeFor[jsonschema.Schema]())) {
		if f.IsExported() {
			got = append(got, f.Name)
		}
	}
	if !slices.Equal(got, want) {
		t.Errorf("the fields of jsonschema's Schema changed: sort them in shapeFields for Shape\n%v\nwas\n%v", got, want)
	}
	if missing := schema.ShapeFieldsMissing(want); len(missing) > 0 {
		t.Errorf("shapeFields sorts not %v", missing)
	}
}
