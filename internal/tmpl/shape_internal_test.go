package tmpl

import (
	"slices"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/sukekyo26/floe/internal/usererr"
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

	s, _, err := readShape(jsonDoc(t, src), schemaURL("t"), "t/schema.json", "")
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

// A keyword floe reads in another form than it reads is refused, at its pointer.
func TestShapeRefusesWrongTypes(t *testing.T) {
	t.Parallel()

	for src, want := range map[string]string{
		`{"properties": {"a": true}}`:          "a schema floe does not read: floe reads #/properties/a only as an object",
		`{"items": [{"type": "string"}]}`:      "a schema floe does not read: floe reads #/items only as an object",
		`{"properties": {"a": {"format": 1}}}`: "a schema floe does not read: floe reads #/properties/a/format only as text",
		`{"required": ["a", 1]}`:               "a schema floe does not read: floe reads #/required only as a list of text",
		`{"enum": "a"}`:                        "a schema floe does not read: floe reads #/enum only as a list",
	} {
		if _, _, err := readShape(jsonDoc(t, src), schemaURL("t"), "t/schema.json", ""); err == nil || err.Error() != want {
			t.Errorf("readShape of %s = %v, want %s", src, err, want)
		}
	}
}

// A key declared null declares nothing of it: the parts under the shape are found past it.
func TestShapeNullKey(t *testing.T) {
	t.Parallel()

	s := shapeOf(t, `{"properties": {"a": null, "b": {"$ref": "floe:///parts/table.json"}}}`)
	var names []string
	s.partsUnder(&names)
	if !slices.Contains(names, "table") {
		t.Errorf("partsUnder = %v, want table among them", names)
	}
}

// Beside a $ref to a part or a schema the parts share, a schema takes what declares nothing, and the root
// its draft too: what the $ref names declares the item (決定 6).
func TestShapeBesideRef(t *testing.T) {
	t.Parallel()

	for src, refused := range map[string]bool{
		`{"properties": {"a": {"$ref": "floe:///parts/tone.json", "description": "d", "title": "t", "$comment": "c"}}}`:         false,
		`{"$schema": "https://json-schema.org/draft/2020-12/schema", "$ref": "floe:///parts/any.json"}`:                         false,
		`{"properties": {"a": {"$ref": "#/$defs/a", "type": "string"}}, "$defs": {"a": {}}}`:                                    false,
		`{"properties": {"a": {"$ref": "floe:///parts/any.json", "items": {}}}}`:                                                true,
		`{"properties": {"a": {"$ref": "floe:///parts/table.json", "required": ["rows"]}}}`:                                     true,
		`{"properties": {"a": {"$schema": "https://json-schema.org/draft/2020-12/schema", "$ref": "floe:///parts/tone.json"}}}`: true,
	} {
		_, notes, err := readShape(jsonDoc(t, src), schemaURL("t"), "t/schema.json", "")
		if err != nil {
			t.Fatal(err)
		}
		if got := slices.ContainsFunc(notes.mistakes, func(e *usererr.Error) bool { return e.Key == "tmpl_ref_beside" }); got != refused {
			t.Errorf("readShape of %s refuses beside the $ref: %v, want %v", src, got, refused)
		}
	}
}
