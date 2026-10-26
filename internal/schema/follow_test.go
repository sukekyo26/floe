package schema_test

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

// Follow stops at a format before a $ref, at a part, follows a $ref to the same file or to a Shared schema
// with the properties beside it (its own first), and takes Properties and Items, Items of one schema in
// 2019-09; nothing else, and a $ref back to what it followed ends.
func TestFollow(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name, body, prop   string
		part, format       string
		props              []string
		items, propsAreSet bool
	}{
		{"a part", `"properties": {"n": ` + N + `}`, "n", "floe:///parts/note.json", "", nil, false, false},
		{"a format", `"properties": {"t": {"type": "string", "format": "html"}}`, "t", "", "html", nil, false, false},
		{"a format before a $ref", `"properties": {"s": {"$ref": "#/$defs/str", "format": "markdown"}}, "$defs": {"str": {"type": "string"}}`, "s", "", "markdown", nil, false, false},
		{"a $ref to a part through the same file", `"properties": {"s": {"$ref": "#/$defs/a", "properties": {"x": {}}}}, "$defs": {"a": ` + N + `}`, "s", "floe:///parts/note.json", "", nil, false, false},
		{"properties beside a $ref", `"properties": {"s": {"$ref": "#/$defs/base", "properties": {"p": {"type": "string"}}}}, "$defs": {"base": {"properties": {"p": ` + N + `, "q": {}}}}`, "s", "", "", []string{"p", "q"}, false, true},
		{"a Shared $ref", `"properties": {"s": {"$ref": "floe:///parts/any.json"}}`, "s", "", "", []string{"note", "stack", "text"}, false, true},
		{"items", `"properties": {"xs": {"type": "array", "items": {"type": "string"}}}`, "xs", "", "", nil, true, true},
		{"allOf is no step", `"properties": {"s": {"allOf": [{"properties": {"n": ` + N + `}}]}}`, "s", "", "", nil, false, true},
		{"a $ref back", `"properties": {"s": {"$ref": "#/$defs/a"}}, "$defs": {"a": {"$ref": "#/$defs/a"}}`, "s", "", "", nil, false, true},
	} {
		// Some are refused, which Follow does not mind: the check of the part p beside a $ref, for one.
		c, err := compile(t, tt.body)
		if c == nil {
			t.Fatalf("%s: %s", tt.name, told(err))
		}
		st := c.Follow(c.Root.Properties[tt.prop])
		props := slices.Sorted(maps.Keys(st.Props))
		if st.Part != tt.part || st.Format != tt.format || (tt.propsAreSet && !slices.Equal(props, tt.props)) || (st.Items != nil) != tt.items {
			t.Errorf("%s: Follow = %+v (props %v)", tt.name, st, props)
		}
	}
	// Items of one schema, before 2020-12.
	c, err := schemaCompile(t, `{"$schema": "https://json-schema.org/draft/2019-09/schema", "type": "object", "properties": {"xs": {"type": "array", "items": {"type": "string"}}}}`)
	if err != nil {
		t.Fatal(told(err))
	}
	if st := c.Follow(c.Root.Properties["xs"]); st.Items == nil {
		t.Errorf("Follow of items in 2019-09 = %+v", st)
	}
}

// Forbidding tells the first step of a check's for that a schema on the way forbids: a key an object of
// additionalProperties: false declares neither in its properties nor its patternProperties. A list on the way
// is gone through, a part's value from its root; a schema of neither the key nor items, and one met again in a
// list's items, are not known to forbid.
func TestForbidding(t *testing.T) {
	t.Parallel()

	const closed = `"additionalProperties": false, `
	for _, tt := range []struct {
		name, body, path, want string
	}{
		{"declared", closed + `"properties": {"items": {"type": "array"}}`, "items", ""},
		{"not declared, closed", closed + `"properties": {"items": {}}`, "itemz", "itemz"},
		{"spaces, closed", closed + `"properties": {"items": {}}`, " ", " "},
		{"open", `"properties": {"items": {}}`, "itemz", ""},
		{"a pattern takes it", closed + `"patternProperties": {"^x-": {}}`, "x-1", ""},
		{"a pattern takes another", closed + `"patternProperties": {"^x-": {}}`, "y", "y"},
		{"through a list", `"properties": {"rows": {"type": "array", "items": {` + closed + `"properties": {"a": {}}}}}`, "rows.b", "b"},
		{"a list of lists", `"properties": {"rows": {"type": "array", "items": {"type": "array", "items": {` + closed + `"properties": {"a": {}}}}}}`, "rows.a", ""},
		{"not a list", `"properties": {"n": {"type": "number"}}`, "n.x", ""},
		{"anyOf", `"properties": {"r": {"anyOf": [{"type": "array"}, {"type": "null"}]}}`, "r.x", ""},
		{"a list its own items", `"properties": {"rows": {"$ref": "#/$defs/l"}}, "$defs": {"l": {"type": "array", "items": {"$ref": "#/$defs/l"}}}`, "rows.x", ""},
		{"closed beside a $ref", `"properties": {"s": {"$ref": "#/$defs/b", ` + closed + `"properties": {"p": {}}}}, "$defs": {"b": {"properties": {"q": {}}}}`, "s.q", "q"},
		{"into a part", `"properties": {"n": ` + N + `}`, "n.zz", "zz"},
		// Of no type, keys and items, beside a closed $ref: rows may be a list of objects with k.
		{"keys and items", `"properties": {"rows": {"$ref": "#/$defs/c", "properties": {"k": {}}, "items": {"properties": {"k": {}}}}}, "$defs": {"c": {` + closed + `"properties": {}}}`, "rows.k", ""},
	} {
		c, err := compile(t, tt.body)
		if c == nil {
			t.Fatalf("%s: %s", tt.name, told(err))
		}
		key, ok := c.Forbidding(strings.Split(tt.path, "."))
		if key != tt.want || ok != (tt.want != "") {
			t.Errorf("%s: Forbidding(%q) = %q, %v; want %q", tt.name, tt.path, key, ok, tt.want)
		}
	}
}
