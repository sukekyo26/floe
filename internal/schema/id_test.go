package schema_test

import (
	"fmt"
	"testing"

	"github.com/sukekyo26/floe/internal/schema"
)

const (
	idFloe     = "this $id names %s, a floe: URL, which are floe's own: give the schema an $id of your own, like https://example.com/item.json, or leave $id out"
	idRelative = "this $id, %s, has no scheme, which a part or a schema the parts share may not have, as it may land under floe:///parts/ and take the place of another: write an absolute URL of your own, like https://example.com/item.json, or leave $id out"
	idRoot     = "this $id, %s, is not this file's URL as it is: write %s as it is, or leave $id out"
)

// A template's "$id" of the floe: scheme is refused where it is, however its URL is written, with the
// mistakes of compiling it; a relative one stays under floe://templates/ and takes no part's place.
func TestCompileIDs(t *testing.T) {
	t.Parallel()

	note := `"x": {"$ref": "floe:///parts/note.json"}`
	fake := func(id string) string {
		return `{` + head + `, "properties": {` + note + `}, "$defs": {"fake": {"$id": "` + id + `", "type": "string"}}}`
	}
	at := func(ptr, id string) string { return "schema.json: " + ptr + ": " + fmt.Sprintf(idFloe, id) }
	for _, tt := range []struct{ name, src, want string }{
		{"a part of floe's", fake("floe:///parts/note.json"), at("#/$defs/fake", "floe:///parts/note.json")},
		{"no part of floe's", fake("floe:///parts/q.json"), at("#/$defs/fake", "floe:///parts/q.json")},
		// What a match on the start of the text would let through.
		{"upper case", fake("FLOE:///parts/note.json"), at("#/$defs/fake", "FLOE:///parts/note.json")},
		{"a way round", fake("floe:///x/../parts/note.json"), at("#/$defs/fake", "floe:///x/../parts/note.json")},
		{"the scheme alone", fake("floe:parts/note.json"), at("#/$defs/fake", "floe:parts/note.json")},
		{
			"under an $id of floe's", `{` + head + `, "properties": {` + note + `}, "$defs": {"o": {"$id": "floe:///parts/", "$defs": {"fake": {"$id": "note.json"}}}}}`,
			at("#/$defs/o", "floe:///parts/"),
		},
		{
			"with a $ref compiling does not resolve", `{` + head + `, "properties": {` + note + `, "z": {"$ref": "#/$defs/missing"}}, "$defs": {"fake": {"$id": "floe:///parts/note.json"}}}`,
			lines(`schema.json: json-pointer in "floe://templates/t/schema.json#/$defs/missing" not found`, at("#/$defs/fake", "floe:///parts/note.json")),
		},
		{
			"with a $schema floe does not take", `{` + head + `, "properties": {"x": {"$schema": "http://json-schema.org/draft-07/schema#"}}, "$defs": {"fake": {"$id": "floe:///parts/note.json"}}}`,
			lines(at("#/$defs/fake", "floe:///parts/note.json"), "schema.json: #/properties/x: $schema http://json-schema.org/draft-07/schema# names no draft floe takes; name https://json-schema.org/draft/2020-12/schema or https://json-schema.org/draft/2019-09/schema, or leave it out"),
		},
		{"the root, not its URL as it is", `{` + head + `, "$id": "floe://templates/t/schema.json#"}`, "schema.json: #: " + fmt.Sprintf(idRoot, "floe://templates/t/schema.json#", template)},
		{"the root, its URL", `{` + head + `, "$id": "` + template + `"}`, ""},
		{"an $id of one's own", `{` + head + `, "$id": "https://example.com/t.json", "properties": {"x": {"$ref": "item.json"}}, "$defs": {"i": {"$id": "item.json", "type": "string"}}}`, ""},
		{"a relative $id", `{` + head + `, "properties": {"x": {"$ref": "item.json"}}, "$defs": {"i": {"$id": "item.json", "type": "string"}}}`, ""},
		// The host keeps a relative $ref under the templates, where floe holds no file.
		{"a relative $ref", `{` + head + `, "properties": {"x": {"$ref": "/parts/note.json"}}}`, "schema.json: floe://templates/parts/note.json is no file floe holds: a template refers to a part as floe:///parts/<name>.json, and inside its own schema.json as #/…"},
	} {
		_, err := schema.Compile(template, "schema.json", []byte(tt.src), resources(t))
		if got := told(err); got != tt.want {
			t.Errorf("%s: Compile told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
	}
	for _, id := range []string{"../../parts/note.json", "/parts/note.json", "../../../../parts/note.json"} {
		c, err := schema.Compile(template, "schema.json", []byte(fake(id)), resources(t))
		if err != nil {
			t.Fatalf("%s: %s", id, told(err))
		}
		if got := c.Follow(c.Root.Properties["x"]).Part; got != "floe:///parts/note.json" {
			t.Errorf("a relative $id %s takes the part's place: x is drawn by %q", id, got)
		}
	}
}

// A part's or a shared schema's "$id" is refused as it is read, a relative one too; the root may name the
// file's URL as it is.
func TestNewResourceIDs(t *testing.T) {
	t.Parallel()

	const u, file = "floe:///parts/mine.json", "own/parts/mine/schema.json"
	for _, tt := range []struct {
		src  string
		kind schema.Kind
		want string
	}{
		{`{"type": "object", "$defs": {"f": {"$id": "r.json"}}}`, schema.Part, file + ": #/$defs/f: " + fmt.Sprintf(idRelative, "r.json")},
		{`{"properties": {"y": {"$ref": "note.json"}}, "$defs": {"n": {"$id": "note.json"}}}`, schema.Shared, file + ": #/$defs/n: " + fmt.Sprintf(idRelative, "note.json")},
		{`{"$defs": {"f": {"$id": "floe:///parts/note.json"}}}`, schema.Shared, file + ": #/$defs/f: " + fmt.Sprintf(idFloe, "floe:///parts/note.json")},
		{`{"$id": "floe:///parts/mine.json#"}`, schema.Part, file + ": #: " + fmt.Sprintf(idRoot, "floe:///parts/mine.json#", u)},
		{`{"$id": "FLOE:///parts/mine.json"}`, schema.Part, file + ": #: " + fmt.Sprintf(idRoot, "FLOE:///parts/mine.json", u)},
		{`{"$id": "mine.json"}`, schema.Part, file + ": #: " + fmt.Sprintf(idRoot, "mine.json", u)},
		{`{"$schema": "https://example.com/s"}`, schema.Shared, file + ": #: $schema https://example.com/s names no draft floe takes; name https://json-schema.org/draft/2020-12/schema or https://json-schema.org/draft/2019-09/schema, or leave it out"},
		// Under an absolute $id too, as the rule reads the text alone.
		{`{"$defs": {"o": {"$id": "https://example.com/x.json", "$defs": {"f": {"$id": "../floe.json"}}}}}`, schema.Part, file + ": #/$defs/o/$defs/f: " + fmt.Sprintf(idRelative, "../floe.json")},
		{`{"$id": "floe:///parts/mine.json", "$defs": {"f": {"$id": "https://example.com/f.json"}}, "properties": {"x": {"$ref": "note.json"}}}`, schema.Part, ""},
		// The library reads "" and "#" as no $id.
		{`{"$id": "", "$defs": {"f": {"$id": "#"}, "g": {"$id": ""}}}`, schema.Shared, ""},
	} {
		_, err := schema.NewResource(u, file, []byte(tt.src), tt.kind)
		if got := told(err); got != tt.want {
			t.Errorf("NewResource(%s) told\n%s\nwant\n%s", tt.src, got, tt.want)
		}
	}
}

// A part's relative $ref, under the part's URL, still reaches a part of floe's.
func TestCheckPartRelativeRef(t *testing.T) {
	t.Parallel()

	const u = "floe:///parts/mine.json"
	res := resources(t)
	r, err := schema.NewResource(u, "own/parts/mine/schema.json", []byte(`{`+head+`, "properties": {"x": {"$ref": "note.json"}}}`), schema.Part)
	if err != nil {
		t.Fatal(told(err))
	}
	res[r.URL] = r
	if failed := schema.CheckPart(u, res); failed != nil {
		t.Fatal(told(failed))
	}
	c, err := schema.Compile(template, "schema.json", []byte(`{`+head+`, "properties": {"m": {"$ref": "`+u+`"}}}`), res)
	if err != nil {
		t.Fatal(told(err))
	}
	if got := c.Follow(c.Root.Properties["m"].Ref.Properties["x"]).Part; got != "floe:///parts/note.json" {
		t.Errorf("the part's x is drawn by %q, want floe:///parts/note.json", got)
	}
}
