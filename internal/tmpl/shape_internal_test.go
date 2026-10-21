package tmpl

import (
	"fmt"
	"slices"
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
		if got := slices.ContainsFunc(notes.met, func(x noted) bool { return x.mistake != nil && x.mistake.Key == "tmpl_ref_beside" }); got != refused {
			t.Errorf("readShape of %s refuses beside the $ref: %v, want %v", src, got, refused)
		}
	}
}

// A $ref out of floe is refused at its pointer, but for what jsonschema holds itself, the drafts and their
// vocabularies; a format of floe's in a part's schema is refused as such wherever it is.
func TestShapeMistakes(t *testing.T) {
	t.Parallel()

	set, err := builtinParts()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ src, part, want string }{
		{`{"$ref": "https://json-schema.org/draft/2020-12/meta/core"}`, "", ""},
		{
			`{"properties": {"a": {"anyOf": [{"$ref": "file:///etc/hosts"}]}}}`, "",
			"t/schema.json: #/properties/a/anyOf/0: file:///etc/hosts: a schema refers to no other file or URL",
		},
		{`{"anyOf": [{"type": "string", "format": "markdown"}]}`, "p", `t/schema.json: #/anyOf/0: a part's schema takes no "format": "markdown"`},
	} {
		_, notes, err := readShape(jsonDoc(t, c.src), schemaURL("t"), "t/schema.json", c.part)
		if err != nil {
			t.Fatal(err)
		}
		got := fmt.Sprint(set.mistakes(notes))
		if c.want == "" && got != "[]" || c.want != "" && !strings.Contains(got, c.want) {
			t.Errorf("mistakes of %s = %s, want %q", c.src, got, c.want)
		}
	}
}

// The items floe draws whole are listed once, a part after the parts it holds, which it draws as they were
// drawn, and an item of a format of floe's among them.
func TestDrawnWhole(t *testing.T) {
	t.Parallel()

	s := shapeOf(t, `{"properties": {"t": {"$ref": "floe:///parts/table.json"}, "m": {"type": "string", "format": "markdown"}}}`)
	table := map[string]any{"columns": []any{map[string]any{"title": "a"}}, "rows": []any{[]any{"x", "y"}}}
	found := s.drawnWhole(map[string]any{"t": table, "m": "hi"})
	got := make([]string, 0, len(found))
	for _, f := range found {
		got = append(got, strings.Join(f.at, ".")+":"+drawerOf(f))
	}
	want := []string{"m:markdown", "t.rows.0.0:cell", "t.rows.0.1:cell", "t:table"}
	if sorted := slices.Sorted(slices.Values(got)); !slices.Equal(sorted, want) {
		t.Errorf("drawnWhole = %v, want %v", got, want)
	}
	if at := slices.Index(got, "t:table"); at < slices.Index(got, "t.rows.0.0:cell") || at < slices.Index(got, "t.rows.0.1:cell") {
		t.Errorf("drawnWhole = %v, the table before its cells", got)
	}
}

// drawerOf names what draws f whole: its format, or its part.
func drawerOf(f drawnItem) string {
	if f.part != nil {
		return f.part.name
	}
	return f.format
}

// What an item of a format of floe's holds is not looked into, a part wins over a format, and an item of a
// format in a part is the part's to draw, not listed.
func TestDrawnWholeStops(t *testing.T) {
	t.Parallel()

	set, err := builtinParts()
	if err != nil {
		t.Fatal(err)
	}
	table := set.parts["table"]
	rows := map[string]any{"columns": []any{map[string]any{"title": "a"}}, "rows": []any{[]any{"x"}}}
	named := func(p *part, format string) *shape {
		s := new(shape)
		s.part, s.named, s.format = p, p.shape, format
		return s
	}
	html := new(shape)
	html.format, html.keys = "html", map[string]*shape{"t": named(table, "")}
	marked := new(shape)
	marked.format = "markdown"
	inner := new(shape)
	inner.keys = map[string]*shape{"m": marked}
	holder := &part{
		name: "holder", shape: inner, pages: nil, words: nil, html: "", css: "", checks: nil, looks: nil, taken: nil, uses: nil,
		about: "", dir: "", replaces: false,
	}
	for _, c := range []struct {
		s    *shape
		v    any
		want []string
	}{
		{html, map[string]any{"t": rows}, []string{":html"}},
		{named(table, "markdown"), rows, []string{"rows.0.0:cell", ":table"}},
		{named(holder, ""), map[string]any{"m": "x"}, []string{":holder"}},
	} {
		found := c.s.drawnWhole(c.v)
		got := make([]string, 0, len(found))
		for _, f := range found {
			got = append(got, strings.Join(f.at, ".")+":"+drawerOf(f))
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("drawnWhole = %v, want %v", got, c.want)
		}
	}
}
