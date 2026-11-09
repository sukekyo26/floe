package schema

import (
	"reflect"
	"testing"
)

// AnyWith names each part in a copy of any's properties by its $ref, leaves the doc it copied as it was, and is
// floe's failure for an any whose properties are not an object.
func TestAnyWith(t *testing.T) {
	t.Parallel()

	const src = `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "properties": {"text": {"$ref": "floe:///parts/text.json"}}}`
	r, err := NewResource("floe:///parts/any.json", "any.json", []byte(src), Shared)
	if err != nil {
		t.Fatal(err)
	}
	before, err := read("any.json", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got, err := AnyWith(r, []string{"quote", "box"})
	if err != nil {
		t.Fatal(err)
	}
	root, _ := got.doc.(map[string]any)             //nolint:errcheck // checked below by its keys
	props, _ := root["properties"].(map[string]any) //nolint:errcheck // checked below by its keys
	for _, name := range []string{"quote", "box"} {
		if want := map[string]any{"$ref": "floe:///parts/" + name + ".json"}; !reflect.DeepEqual(props[name], want) {
			t.Errorf("%s is %v, want %v", name, props[name], want)
		}
	}
	if !reflect.DeepEqual(r.doc, before) {
		t.Errorf("AnyWith changed the doc it copied: %v", r.doc)
	}
	for _, bad := range []string{
		`{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object"}`,
		`{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "properties": true}`,
	} {
		r, err := NewResource("floe:///parts/any.json", "any.json", []byte(bad), Shared)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := AnyWith(r, []string{"quote"}); err == nil || err.Error() != "schema: any.json has no properties of an object" {
			t.Errorf("AnyWith of %s = %v, want floe's failure", bad, err)
		}
	}
}
