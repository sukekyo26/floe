package schema_test

import (
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"floe/internal/schema"
)

// Walk draws by additionalProperties the keys its schema does not leave to properties or to a patternProperties beside
// it, one a $ref brings too, and tells the key as written in At.
func TestWalkRest(t *testing.T) {
	t.Parallel()

	const rest = `"patternProperties": {"^x-": {"type": "number"}}, "additionalProperties": ` + N
	c, err := compile(t, `"properties": {"a": {"properties": {"own": {"type": "string"}}, `+rest+`}, "b": {"$ref": "#/$defs/r"}}, "$defs": {"r": {`+rest+`}}`)
	if c == nil {
		t.Fatal(told(err))
	}
	note := map[string]any{"text": "t"}
	values := map[string]any{
		"a": map[string]any{"own": "o", "x-n": 3, "x/y": note},
		"b": map[string]any{"x-n": 3, "y": note},
	}
	var at []string
	ready, err := c.Walk(values, func(it schema.Item, _ any) (any, error) {
		at = append(at, strings.Join(it.At, " ")+"="+it.Part)
		return "drawn", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a x/y=note", "b y=note"}; !slices.Equal(at, want) {
		t.Errorf("items %v, want %v", at, want)
	}
	want := map[string]any{
		"a": map[string]any{"own": "o", "x-n": 3, "x/y": "drawn"},
		"b": map[string]any{"x-n": 3, "y": "drawn"},
	}
	if !reflect.DeepEqual(ready, want) {
		t.Errorf("ready %#v, want %#v", ready, want)
	}
}

// Walk goes through the values a step of Follow at a time, into a part's value from the root of its schema,
// through a $ref of the same file to a part too, and calls fn with each item from the inner ones out; the
// values it returns hold what fn returned, and nil for an item declared and left out.
func TestWalk(t *testing.T) {
	t.Parallel()

	c, err := compile(t, `"properties": {"lead": {"type": "string", "format": "markdown"}, "n": `+N+`, "plain": {},
"s": {"$ref": "#/$defs/x"}}, "$defs": {"x": {"$ref": "floe:///parts/stack.json"}}`)
	if c == nil {
		t.Fatal(told(err))
	}
	values := map[string]any{
		"lead": "**a**", "extra": 1,
		"s": map[string]any{"items": []any{map[string]any{"note": map[string]any{"text": "t"}}, map[string]any{"text": "x"}}},
	}
	var calls []string
	ready, err := c.Walk(values, func(it schema.Item, ready any) (any, error) {
		name := strings.Join(it.At, ".") + "=" + it.Part + it.Format
		calls = append(calls, name)
		return "<" + name + ">", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"lead=markdown", "s.items.0.note=note", "s.items.1.text=markdown", "s=stack"}; !slices.Equal(calls, want) {
		t.Errorf("calls %v, want %v", calls, want)
	}
	want := map[string]any{"extra": 1, "lead": "<lead=markdown>", "n": nil, "plain": nil, "s": "<s=stack>"}
	if !reflect.DeepEqual(ready, want) {
		t.Errorf("ready %#v, want %#v", ready, want)
	}

	// What fn is given of a part: its items made ready, those declared and left out nil.
	_, err = c.Walk(values, func(it schema.Item, ready any) (any, error) {
		if it.Part != "stack" {
			return "<x>", nil
		}
		want := map[string]any{"items": []any{
			map[string]any{"note": "<x>", "text": nil, "stack": nil}, map[string]any{"note": nil, "text": "<x>", "stack": nil},
		}}
		if !reflect.DeepEqual(ready, want) {
			t.Errorf("stack ready %#v, want %#v", ready, want)
		}
		return nil, nil //nolint:nilnil // the value is not read
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Walk tells whether a format is held in a part's value: one of the template's own after a part is not, one in a
// part's value is, and one after a part that value holds still is; a part is not told so.
func TestWalkInPart(t *testing.T) {
	t.Parallel()

	c, err := compile(t, `"properties": {"a": {"$ref": "floe:///parts/stack.json"}, "z": {"type": "string", "format": "markdown"}}`)
	if c == nil {
		t.Fatal(told(err))
	}
	values := map[string]any{
		"a": map[string]any{"items": []any{map[string]any{"note": map[string]any{"text": "t"}}, map[string]any{"text": "x"}}},
		"z": "**z**",
	}
	var calls []string
	if _, err := c.Walk(values, func(it schema.Item, _ any) (any, error) {
		calls = append(calls, strings.Join(it.At, ".")+"="+it.Part+it.Format+" "+strconv.FormatBool(it.InPart))
		return nil, nil //nolint:nilnil // the value is not read
	}); err != nil {
		t.Fatal(err)
	}
	want := []string{"a.items.0.note=note false", "a.items.1.text=markdown true", "a=stack false", "z=markdown false"}
	if !slices.Equal(calls, want) {
		t.Errorf("calls %v, want %v", calls, want)
	}
}

// Walk takes items, as the library does, only for the items after the prefixItems of the schema items is of: those
// before go as written, a key left out not made nil, and none at all are taken under items false.
func TestWalkPrefixItems(t *testing.T) {
	t.Parallel()

	const (
		s2020 = `"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"`
		s2019 = `"$schema": "https://json-schema.org/draft/2019-09/schema", "type": "object"`
		md    = `{"type": "string", "format": "markdown"}`
		str   = `{"type": "string"}`
		p     = `"p": {"type": "object", "properties": {"t": {}, "note": {}, "more": {}}}`
	)
	two := []any{"**h**", "**i**"}
	for _, tt := range []struct {
		name, head, x, defs string
		values              []any
		calls               []string
		ready               []any
	}{
		{
			"items after prefixItems", s2020, `{"prefixItems": [` + str + `], "items": ` + md + `}`, "", two,
			[]string{"x.1=markdown false"},
			[]any{"**h**", "<x.1>"},
		},
		{
			"a key left out of a prefixItems item", s2020,
			`{"prefixItems": [{"$ref": "#/$defs/p", "required": ["note"]}], "items": {"$ref": "#/$defs/p"}}`, p,
			[]any{map[string]any{"t": "head", "note": "n"}, map[string]any{"t": "two"}},
			nil,
			[]any{map[string]any{"t": "head", "note": "n"}, map[string]any{"t": "two", "note": nil, "more": nil}},
		},
		{
			"fewer items than prefixItems", s2020, `{"prefixItems": [` + str + `, ` + str + `], "items": ` + md + `}`, "",
			[]any{"**a**"},
			nil,
			[]any{"**a**"},
		},
		{
			"both through a $ref", s2020, `{"$ref": "#/$defs/l"}`, `"l": {"prefixItems": [` + str + `], "items": ` + md + `}`,
			two,
			[]string{"x.1=markdown false"},
			[]any{"**h**", "<x.1>"},
		},
		{
			"prefixItems beside a $ref to items", s2020, `{"$ref": "#/$defs/l", "prefixItems": [` + str + `]}`,
			`"l": {"items": ` + md + `}`, two,
			[]string{"x.0=markdown false", "x.1=markdown false"},
			[]any{"<x.0>", "<x.1>"},
		},
		{
			"items beside a $ref to prefixItems", s2020, `{"$ref": "#/$defs/l", "items": ` + md + `}`,
			`"l": {"prefixItems": [` + str + `, ` + str + `], "items": ` + str + `}`, two,
			[]string{"x.0=markdown false", "x.1=markdown false"},
			[]any{"<x.0>", "<x.1>"},
		},
		{
			"items false beside a $ref to items", s2020, `{"$ref": "#/$defs/l", "prefixItems": [` + str + `], "items": false}`,
			`"l": {"items": ` + md + `}`, two, nil, two,
		},
		{
			"a part's root", s2020, `{"$ref": "floe:///parts/pre.json"}`, "", two,
			[]string{"x.1=markdown true", "x=pre false"},
			nil,
		},
		// 2019-09 has no prefixItems: the library leaves it out of the tree, and items takes every item.
		{
			"2019-09", s2019, `{"prefixItems": [` + str + `], "items": ` + md + `}`, "", two,
			[]string{"x.0=markdown false", "x.1=markdown false"},
			[]any{"<x.0>", "<x.1>"},
		},
	} {
		res := resources(t)
		pre, err := schema.NewResource("floe:///parts/pre.json", "parts/pre.json", []byte(`{"$schema": "https://json-schema.org/draft/2020-12/schema",
"type": "array", "prefixItems": [`+str+`], "items": `+md+`}`), schema.Part)
		if err != nil {
			t.Fatal(err)
		}
		res["floe:///parts/pre.json"] = pre
		c, err := schema.Compile(template, "schema.json",
			[]byte("{"+tt.head+`, "properties": {"x": `+tt.x+`}, "$defs": {`+tt.defs+"}}"), res)
		if c == nil {
			t.Fatalf("%s: %s", tt.name, told(err))
		}
		var calls []string
		ready, err := c.Walk(map[string]any{"x": tt.values}, func(it schema.Item, _ any) (any, error) {
			calls = append(calls, strings.Join(it.At, ".")+"="+it.Part+it.Format+" "+strconv.FormatBool(it.InPart))
			return "<" + strings.Join(it.At, ".") + ">", nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(calls, tt.calls) {
			t.Errorf("%s: calls %v, want %v", tt.name, calls, tt.calls)
		}
		if tt.ready != nil {
			if got := ready.(map[string]any)["x"]; !reflect.DeepEqual(got, tt.ready) { //nolint:errcheck,forcetypeassert // the root is an object
				t.Errorf("%s: ready %#v, want %#v", tt.name, got, tt.ready)
			}
		}
	}
}
