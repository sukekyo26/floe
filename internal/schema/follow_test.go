package schema_test

import (
	"maps"
	"slices"
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
