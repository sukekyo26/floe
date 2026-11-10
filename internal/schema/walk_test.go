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
