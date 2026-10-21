package tmpl

import (
	"reflect"
	"testing"
)

// An item tells what it takes, a list what its items take, and a null in schema.json tells nothing.
func TestItems(t *testing.T) {
	t.Parallel()

	src := `{
  "required": ["tags"],
  "properties": {
    "tags": { "type": "array", "items": { "type": "string", "enum": ["a", "b"], "description": "A tag" } },
    "size": { "type": ["integer", "null"], "enum": [1000000, 2000000, null] },
    "kind": { "type": "strng" },
    "notes": { "type": "array", "items": { "type": "string", "format": "html" } },
    "gone": null
  }
}`
	s := shapeOf(t, src)
	want := []Item{
		{Name: "gone", Types: nil, Of: nil, Format: "", Enum: nil, Required: false, Description: "", Items: nil},
		{Name: "kind", Types: []string{"strng"}, Of: nil, Format: "", Enum: nil, Required: false, Description: "", Items: nil},
		{Name: "notes", Types: []string{"array"}, Of: []string{"string"}, Format: "html", Enum: nil, Required: false, Description: "", Items: nil},
		{Name: "size", Types: []string{"integer", "null"}, Of: nil, Format: "", Enum: []string{"1000000", "2000000", "null"}, Required: false, Description: "", Items: nil},
		{Name: "tags", Types: []string{"array"}, Of: []string{"string"}, Format: "", Enum: []string{"a", "b"}, Required: true, Description: "A tag", Items: nil},
	}
	if got := s.items(); !reflect.DeepEqual(got, want) {
		t.Errorf("items =\n%+v\nwant\n%+v", got, want)
	}
}
