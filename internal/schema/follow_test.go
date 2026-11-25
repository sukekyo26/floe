package schema_test

import (
	"maps"
	"slices"
	"strings"
	"testing"
)

// Rest is the schema of additionalProperties, not true or false, taken through a $ref unless s has its own.
func TestFollowRest(t *testing.T) {
	t.Parallel()

	const rest = `"additionalProperties": {"type": "string"}`
	for _, tt := range []struct {
		name, prop string
		want       bool
	}{{"a schema", "schema", true}, {"true", "yes", false}, {"false", "no", false}, {"none", "none", false}, {"through a $ref", "ref", true}, {"at a part", "part", false}} {
		c, err := compile(t, `"properties": {"schema": {`+rest+`}, "yes": {"additionalProperties": true}, "no": {"additionalProperties": false}, "none": {},
"ref": {"$ref": "#/$defs/r"}, "part": `+N+`}, "$defs": {"r": {`+rest+`}}`)
		if c == nil {
			t.Fatalf("%s: %s", tt.name, told(err))
		}
		if st := c.Follow(c.Root.Properties[tt.prop]); (st.Rest != nil) != tt.want {
			t.Errorf("%s: Rest = %v, want it %v", tt.name, st.Rest, tt.want)
		}
	}
}

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
// additionalProperties: false declares neither in its properties nor its patternProperties, where the types on the way
// let the value be no list, or a list whose items forbid it; and any key of a value the types let be neither keys nor a
// list. A list on the way is gone through, a part's value from its root; where the value may be keys and a list (no
// type too), a key not declared, a list of no items, and a schema met again in a list's items, a key is not known to be
// forbidden. What the last step leads to is not asked.
func TestForbidding(t *testing.T) {
	t.Parallel()

	const closed = `"additionalProperties": false, `
	const objA = `{"type": "object", "properties": {"a": {"type": "array"}}}`
	const shut = `{"type": "object", "additionalProperties": false, "properties": {"n": {}}}`
	for _, tt := range []struct {
		name, body, path, want string
	}{
		{"declared", closed + `"properties": {"items": {"type": "array"}}`, "items", ""},
		{"not declared, closed", closed + `"properties": {"items": {}}`, "itemz", "itemz"},
		{"spaces, closed", closed + `"properties": {"items": {}}`, " ", " "},
		{"open", `"properties": {"items": {}}`, "itemz", ""},
		{"a pattern takes it", closed + `"patternProperties": {"^x-": {}}`, "x-1", ""},
		{"a pattern takes another", closed + `"patternProperties": {"^x-": {}}`, "y", "y"},
		{"through a list", `"properties": {"rows": {"type": "array", "items": {"type": "object", ` + closed + `"properties": {"a": {}}}}}`, "rows.b", "b"},
		// Items of no type may be lists: their keys are not known.
		{"through a list, items of no type", `"properties": {"rows": {"type": "array", "items": {` + closed + `"properties": {"a": {}}}}}`, "rows.b", ""},
		{"a list of lists", `"properties": {"rows": {"type": "array", "items": {"type": "array", "items": {` + closed + `"properties": {"a": {}}}}}}`, "rows.a", ""},
		{"anyOf", `"properties": {"r": {"anyOf": [{"type": "array"}, {"type": "null"}]}}`, "r.x", ""},
		{"a list its own items", `"properties": {"rows": {"$ref": "#/$defs/l"}}, "$defs": {"l": {"type": "array", "items": {"$ref": "#/$defs/l"}}}`, "rows.x", ""},
		{"closed beside a $ref", `"properties": {"s": {"type": "object", "$ref": "#/$defs/b", ` + closed + `"properties": {"p": {}}}}, "$defs": {"b": {"properties": {"q": {}}}}`, "s.q", "q"},
		{"closed beside a $ref, of no type", `"properties": {"s": {"$ref": "#/$defs/b", ` + closed + `"properties": {"p": {}}}}, "$defs": {"b": {"properties": {"q": {}}}}`, "s.q", ""},
		{"into a part", `"properties": {"n": ` + N + `}`, "n.zz", "zz"},
		// Of no type, keys and items, beside a closed $ref: rows may be a list of objects with k.
		{"keys and items, then a step", `"properties": {"rows": {"properties": {"a": {` + closed + `"properties": {"x": {}}}}, "items": {"properties": {"a": {"properties": {"y": {}}}}}}}`, "rows.a.y", ""},
		{"another key, closed items, additionalProperties", `"properties": {"rows": {"items": {` + closed + `"properties": {"a": {}}}, "additionalProperties": {}}}`, "rows.foo", ""},
		{"closed beside a $ref to additionalProperties", `"properties": {"a": {"type": "object", "$ref": "#/$defs/open", ` + closed + `"properties": {"k": {}}}}, "$defs": {"open": {"additionalProperties": {}}}`, "a.zz", "zz"},
		{"closed beside a $ref to additionalProperties, of no type", `"properties": {"a": {"$ref": "#/$defs/open", ` + closed + `"properties": {"k": {}}}}, "$defs": {"open": {"additionalProperties": {}}}`, "a.zz", ""},
		{"keys and items", `"properties": {"rows": {"$ref": "#/$defs/c", "properties": {"k": {}}, "items": {"properties": {"k": {}}}}}, "$defs": {"c": {` + closed + `"properties": {}}}`, "rows.k", ""},
		// Of the items items does not take, those before it by prefixItems, a key is not known.
		{"prefixItems before closed items", `"properties": {"rows": {"prefixItems": [` + objA + `], "items": {` + closed + `"properties": {"b": {}}}}}`, "rows.a", ""},
		{"prefixItems and no items, closed", `"properties": {"rows": {` + closed + `"prefixItems": [` + objA + `]}}`, "rows.a", ""},
		{"prefixItems through a $ref, closed beside", `"properties": {"rows": {"$ref": "#/$defs/l", "additionalProperties": false}}, "$defs": {"l": {"prefixItems": [` + objA + `]}}`, "rows.a", ""},
		{"closed items beside a $ref to prefixItems", `"properties": {"rows": {"type": "array", "$ref": "#/$defs/l", "items": {"type": "object", ` + closed + `"properties": {"b": {}}}}}, "$defs": {"l": {"prefixItems": [` + objA + `]}}`, "rows.a", "a"},
		// Of no type, rows may be an object with a.
		{"closed items beside a $ref to prefixItems, of no type", `"properties": {"rows": {"$ref": "#/$defs/l", "items": {` + closed + `"properties": {"b": {}}}}}, "$defs": {"l": {"prefixItems": [` + objA + `]}}`, "rows.a", ""},
		// LIMIT of Forbidding: a key no item may hold is not refused either.
		{"a key no item holds, prefixItems before", `"properties": {"rows": {"type": "array", "prefixItems": [{"type": "object", ` + closed + `"properties": {"p": {}}}], "items": {"type": "object", ` + closed + `"properties": {"b": {}}}}}`, "rows.zzz", ""},
		{"a list, prefixItems before closed items", `"properties": {"rows": {"type": "array", "prefixItems": [` + shut + `], "items": ` + shut + `}}`, "rows.zz", ""},
		// The value's shape is read from type: keys, a list, or both, which stops it.
		{"items beside additionalProperties true", `"properties": {"rows": {"items": ` + shut + `, "additionalProperties": true}}`, "rows.foo", ""},
		{"items, additionalProperties left out", `"properties": {"rows": {"items": ` + shut + `}}`, "rows.foo", ""},
		{"a list", `"properties": {"rows": {"type": "array", "items": ` + shut + `}}`, "rows.foo", "foo"},
		{"a list or keys", `"properties": {"rows": {"type": ["array", "object"], "items": ` + shut + `, "additionalProperties": true}}`, "rows.foo", ""},
		{"a list or null", `"properties": {"rows": {"type": ["array", "null"], "items": ` + shut + `}}`, "rows.foo", "foo"},
		{"a list by a $ref", `"properties": {"rows": {"$ref": "#/$defs/arr", "items": ` + shut + `}}, "$defs": {"arr": {"type": "array"}}`, "rows.foo", "foo"},
		{"closed, of no type", `"properties": {"x": {"additionalProperties": false}}`, "x.a", ""},
		{"a closed list of no items", `"properties": {"x": {"type": "array", ` + closed + `"properties": {"p": {}}}}`, "x.a", ""},
		{"keys alone, declared beside items", `"properties": {"rows": {"type": "object", "properties": {"a": ` + shut + `}, "items": {"properties": {"a": {}}}}}`, "rows.a.zz", "zz"},
		{"a list alone, declared", `"properties": {"rows": {"type": "array", "properties": {"a": {}}, "items": ` + shut + `}}`, "rows.a", "a"},
		{"keys alone, closed, prefixItems", `"properties": {"x": {"type": "object", ` + closed + `"prefixItems": [{}]}}`, "x.a", "a"},
		{"keys alone, closed, items", `"properties": {"x": {"type": "object", ` + closed + `"items": {}}}`, "x.a", "a"},
		{"keys alone, items", `"properties": {"rows": {"type": "object", "items": ` + shut + `}}`, "rows.foo", ""},
		{"a list alone, additionalProperties", `"properties": {"rows": {"type": "array", "additionalProperties": {}, "items": ` + shut + `}}`, "rows.foo", "foo"},
		{"open, of no type", `"properties": {"o1": {"properties": {"a": ` + shut + `}}}`, "o1.a.zz", ""},
		// Neither keys nor a list: no key.
		{"a number", `"properties": {"n": {"type": "number"}}`, "n.x", "x"},
		{"text or null", `"properties": {"n": {"type": ["string", "null"]}}`, "n.x", "x"},
		{"items of text", `"properties": {"g": {"type": "array", "items": {"type": "string"}}}`, "g.x", "x"},
		{"text by a $ref", `"properties": {"n": {"$ref": "#/$defs/s"}}, "$defs": {"s": {"type": "string"}}`, "n.x", "x"},
		{"text alone by two schemas", `"properties": {"n": {"type": ["string", "array"], "$ref": "#/$defs/s"}}, "$defs": {"s": {"type": ["string", "object"]}}`, "n.x", "x"},
		{"a format", `"properties": {"x": {"type": "string", "format": "markdown"}}`, "x.a", "a"},
		{"a format, of no type", `"properties": {"x": {"format": "markdown"}}`, "x.a", ""},
		{"a part of text", `"properties": {"n": {"$ref": "floe:///parts/prose.json"}}`, "n.x", "x"},
		{"text of a part in a list", `"properties": {"n": {"type": "array", "items": {"$ref": "floe:///parts/any.json"}}}`, "n.note.title.x", "x"},
		// A part's value is read from its root alone, as Follow does.
		{"text beside a $ref to a part", `"properties": {"n": {"type": "string", "$ref": "floe:///parts/note.json"}}`, "n.title", ""},
		{"text or keys, open", `"properties": {"n": {"type": ["string", "object"]}}`, "n.x", ""},
		{"text or a list, a key of its items", `"properties": {"n": {"type": ["string", "array"], "items": ` + shut + `}}`, "n.n", ""},
		{"text or a list, no key of its items", `"properties": {"n": {"type": ["string", "array"], "items": ` + shut + `}}`, "n.zz", "zz"},
		{"a list of items of text, prefixItems before", `"properties": {"n": {"type": "array", "prefixItems": [` + objA + `], "items": {"type": "string"}}}`, "n.a", ""},
		{"ends at text", `"properties": {"n": {"type": "string"}}`, "n", ""},
		{"ends at keys", `"properties": {"n": {"type": "object"}}`, "n", ""},
		// LIMIT of Forbidding: text or a list of no items.
		{"text or a list, no items", `"properties": {"n": {"type": ["string", "array"]}}`, "n.x", ""},
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
	// Whole schemas: of 2019-09, and of no type at the root.
	for _, tt := range []struct {
		name, src, path string
	}{
		{"2019-09, items of an array", `{"$schema": "https://json-schema.org/draft/2019-09/schema", "type": "object", "properties": {"x": {"items": [` + objA + `], "additionalProperties": false}}}`, "x.a"},
		{"2019-09, a list of items of an array", `{"$schema": "https://json-schema.org/draft/2019-09/schema", "type": "object", "properties": {"x": {"type": "array", "items": [` + objA + `], "additionalProperties": false}}}`, "x.a"},
		{"a root of no type", `{"$schema": "https://json-schema.org/draft/2020-12/schema", ` + closed + `"properties": {"items": {"type": "array", "items": ` + shut + `}}}`, "itemz"},
	} {
		c, err := schemaCompile(t, tt.src)
		if c == nil {
			t.Fatalf("%s: %s", tt.name, told(err))
		}
		if key, ok := c.Forbidding(strings.Split(tt.path, ".")); ok {
			t.Errorf("%s: Forbidding(%q) = %q, true; want none", tt.name, tt.path, key)
		}
	}
}
