package tmpl_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/tmpl"
)

// tries is how many times a check runs: jsonschema walks maps in a random order, which moved the place of
// a mistake of propertyNames.
const tries = 20

// A key of patternProperties that is no regular expression, or of $vocabulary that is no URL, is told at
// the schema that holds it, in every draft whose metaschema checks it, whatever items are beside it; two at
// one pointer by their words.
func TestPropertyNamesPlaceInSchema(t *testing.T) {
	t.Parallel()

	const beside = `"a": {"type": "string"}, "b": {"type": "string"}, "y": {"type": "string"}, "z": {"type": "string"}, ` +
		`"risks": {"$ref": "floe:///parts/table.json"}`
	tests := []struct{ name, draft, item, want string }{
		{
			"2020-12", "https://json-schema.org/draft/2020-12/schema", `"t": {"type": "object", "patternProperties": {"([": {"type": "string"}}}`,
			"#/properties/t/patternProperties: JSON Schema does not take this: invalid propertyName '(['",
		},
		{
			"2019-09", "https://json-schema.org/draft/2019-09/schema", `"t": {"patternProperties": {"([": {}}}`,
			"#/properties/t/patternProperties: JSON Schema does not take this: invalid propertyName '(['",
		},
		{
			"draft-07", "http://json-schema.org/draft-07/schema#", `"t": {"patternProperties": {"([": {}}}`,
			"#/properties/t/patternProperties: JSON Schema does not take this: invalid propertyName '(['",
		},
		{
			"$vocabulary", "https://json-schema.org/draft/2020-12/schema", `"t": {"$vocabulary": {"not a uri": true}}`,
			"#/properties/t/$vocabulary: JSON Schema does not take this: invalid propertyName 'not a uri'",
		},
		{
			"an item of / and ~", "https://json-schema.org/draft/2020-12/schema", `"a/b~c": {"patternProperties": {"([": {}}}`,
			"#/properties/a~1b~0c/patternProperties: JSON Schema does not take this: invalid propertyName '(['",
		},
		{
			"two keys", "https://json-schema.org/draft/2020-12/schema", `"t": {"patternProperties": {"a(": {}, "([": {}}}`,
			"#/properties/t/patternProperties: JSON Schema does not take this: invalid propertyName '(['\n" +
				"FILE: #/properties/t/patternProperties: JSON Schema does not take this: invalid propertyName 'a('",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := filepath.Join(memo(t, memoItems), "memo")
			schema := filepath.Join(dir, "schema.json")
			writeFile(t, schema, `{"$schema": "`+tt.draft+`", "type": "object", "properties": {`+beside+`, `+tt.item+`}}`)
			want := schema + ": " + strings.ReplaceAll(tt.want, "FILE", schema)
			for range tries {
				if _, mistakes := check(dir, "", ""); joined(mistakes) != want {
					t.Fatalf("Check = %s, want %s", joined(mistakes), want)
				}
			}
		})
	}
}

// One in a part's own schema is told there.
func TestPropertyNamesPlaceInPart(t *testing.T) {
	t.Parallel()

	parts := ownParts(t, map[string]string{
		"p/schema.json": `{"type": "object", "properties": {"a": {"type": "string"}, ` +
			`"t": {"patternProperties": {"([": {}}}, "z": {"type": "string"}}, "required": ["a"]}`,
		"p/part.html": `<div data-floe-part="p">{{.a}}</div>`,
		"p/part.css":  "",
	})
	want := "the part p cannot be used\n  " + filepath.Join(parts, "p", "schema.json") +
		": #/properties/t/patternProperties: JSON Schema does not take this: invalid propertyName '(['"
	for range tries {
		if _, mistakes := checkOwn(parts, themesOf(parts)); joined(mistakes) != want {
			t.Fatalf("CheckOwnParts = %s, want %s", joined(mistakes), want)
		}
	}
}

// One in what a $ref points at is told in the $ref's words.
func TestPropertyNamesPlaceInRef(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(memo(t, memoItems+`, "x": {"enum": [{"properties": {"a": {"type": "string"}, `+
		`"t": {"patternProperties": {"([": {}}}, "z": {"type": "string"}}}]}, "r": {"$ref": "#/properties/x/enum/0"}`), "memo")
	want := filepath.Join(dir, "schema.json") + ": #/properties/r: a $ref to #/properties/x/enum/0 points at what is no schema " +
		"(/properties/t/patternProperties: invalid propertyName '(['): refer to a schema"
	for range tries {
		if _, mistakes := check(dir, "", ""); joined(mistakes) != want {
			t.Fatalf("Check = %s, want %s", joined(mistakes), want)
		}
	}
}

// One in a data file is told at the item whose key it is, and under items, whose place jsonschema loses,
// at the top: never at another item.
func TestPropertyNamesPlaceInData(t *testing.T) {
	t.Parallel()

	dir := memo(t, memoItems+`, "m": {"type": "object", "propertyNames": {"pattern": "^x"}}, `+
		`"l": {"type": "array", "items": {"type": "object", "propertyNames": {"pattern": "^x"}}}, `+
		`"n": {"type": "object", "properties": {"k": {"type": "string"}}}, "o": {"$ref": "#/properties/m"}`)
	lib := tmpl.NewLibrary(dir, "", "", "")
	tests := []struct{ src, want string }{
		{"m:\n  bad: 1\nn:\n  k: s\n", "data.yaml:4:3: m: invalid propertyName 'bad'"},
		{"o:\n  bad: 1\nn:\n  k: s\n", "data.yaml:4:3: o: invalid propertyName 'bad'"},
		{"l:\n  - x: 1\n  - bad: 1\nn:\n  k: s\n", "data.yaml:1:1: the top level: invalid propertyName 'bad'"},
	}
	for _, tt := range tests {
		for range tries {
			if _, err := draw(lib, "template: memo\nlang: en\n"+tt.src); err == nil || err.Error() != tt.want {
				t.Fatalf("Read of %q = %v, want %s", tt.src, err, tt.want)
			}
		}
	}
}

func joined(errs []error) string {
	words := make([]string, len(errs))
	for i, e := range errs {
		words[i] = e.Error()
	}
	return strings.Join(words, "\n")
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
