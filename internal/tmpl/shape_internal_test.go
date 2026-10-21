package tmpl

import (
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// jsonDoc reads src as jsonschema reads a schema.
func jsonDoc(t *testing.T, src string) any {
	t.Helper()

	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

// shapeOf reads the schema src of the template t into its shape, bound to floe's parts.
func shapeOf(t *testing.T, src string) *shape {
	t.Helper()

	s, err := readShape(jsonDoc(t, src), schemaURL("t"))
	if err != nil {
		t.Fatal(err)
	}
	set, err := builtinParts()
	if err != nil {
		t.Fatal(err)
	}
	s.bind(set)
	return s
}

// A place that names a part holds the part's own shape, not a copy of it: the part's items are told and
// walked from there.
func TestShapeNamesPart(t *testing.T) {
	t.Parallel()

	set, err := builtinParts()
	if err != nil {
		t.Fatal(err)
	}
	s := shapeOf(t, `{"properties": {"t": {"$ref": "floe:///parts/table.json", "description": "Rows"}}}`)
	table := s.keys["t"]
	if table.part != set.parts["table"] || table.inside() != set.parts["table"].shape {
		t.Errorf("t is drawn by %v and holds %p, want the table and its shape %p", table.part, table.inside(),
			set.parts["table"].shape)
	}
	if got, want := table.description(), "Rows. "+set.parts["table"].shape.about; got != want {
		t.Errorf("description = %q, want %q", got, want)
	}
}

// A keyword floe reads of another JSON type than JSON Schema's is refused, at its pointer.
func TestShapeRefusesWrongTypes(t *testing.T) {
	t.Parallel()

	for src, want := range map[string]string{
		`{"properties": {"a": true}}`:          "a keyword of the wrong JSON type: #/properties/a must be an object",
		`{"items": [{"type": "string"}]}`:      "a keyword of the wrong JSON type: #/items must be an object",
		`{"properties": {"a": {"format": 1}}}`: "a keyword of the wrong JSON type: #/properties/a/format must be text",
		`{"required": ["a", 1]}`:               "a keyword of the wrong JSON type: #/required must be a list of text",
		`{"enum": "a"}`:                        "a keyword of the wrong JSON type: #/enum must be a list",
	} {
		if _, err := readShape(jsonDoc(t, src), schemaURL("t")); err == nil || err.Error() != want {
			t.Errorf("readShape of %s = %v, want %s", src, err, want)
		}
	}
}
