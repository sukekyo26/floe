package tmpl

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// Each kind of mistake a template's schema can name is worded in English and Japanese, and one the
// catalog does not word keeps the library's English.
func TestSchemaWording(t *testing.T) {
	t.Parallel()

	schema := `{
  "type": "object",
  "required": ["name"],
  "properties": {
    "name": { "type": "string" },
    "op": { "enum": ["GetItem", "Query"] },
    "tags": { "type": "array", "minItems": 2, "maxItems": 3, "uniqueItems": true },
    "id": { "type": "string", "pattern": "^AP-[0-9]+$", "minLength": 4, "maxLength": 5 },
    "size": { "type": "integer", "minimum": 1, "maximum": 9 },
    "rate": { "type": "number", "exclusiveMinimum": 0, "exclusiveMaximum": 1.5 },
    "keys": { "type": "object", "maxProperties": 1 },
    "kind": { "const": "x" },
    "huge": { "type": "number" },
    "note": { "type": "string" },
    "pick": { "enum": ["a", "b"] },
    "open": { "type": "object", "patternProperties": { "^x-": {} }, "additionalProperties": false }
  }
}`
	src := `op: Scan
tags: [a, a, b, c]
id: AP-x12345
size: 10
rate: 1.5
keys: { a: 1, b: 2 }
kind: y
huge: x
note:
pick:
open: { y: 1 }
`
	tests := []struct{ en, ja string }{
		{`d.yaml:1:1: name is required but missing`, `d.yaml:1:1: name は必須の項目ですが、ありません`},
		{`d.yaml:1:5: op: "Scan" is not allowed; use GetItem, Query`, `d.yaml:1:5: op: 「Scan」は使えません。使えるのは GetItem, Query です`},
		{`d.yaml:2:7: tags: the number of items must be at most 3`, `d.yaml:2:7: tags: 3 個までにしてください`},
		{`d.yaml:2:7: tags: items 1 and 2 are the same; write each once`, `d.yaml:2:7: tags: 1 番目と 2 番目が同じです。1 つずつにしてください`},
		{`d.yaml:3:5: id: "AP-x12345" does not match ^AP-[0-9]+$`, `d.yaml:3:5: id: 「AP-x12345」は ^AP-[0-9]+$ の形に合いません`},
		{`d.yaml:3:5: id: the length in characters must be at most 5`, `d.yaml:3:5: id: 5 文字までにしてください`},
		{`d.yaml:4:7: size: write 9 or less`, `d.yaml:4:7: size: 9 以下にしてください`},
		{`d.yaml:5:7: rate: write less than 1.5`, `d.yaml:5:7: rate: 1.5 より小さくしてください`},
		{`d.yaml:6:9: keys: the number of keys must be at most 1`, `d.yaml:6:9: keys: キーは 1 個までにしてください`},
		{`d.yaml:7:7: kind: value must be 'x'`, `d.yaml:7:7: kind: value must be 'x'`},
		{`d.yaml:8:7: huge: write a number, not text`, `d.yaml:8:7: huge: 文字列ではなく数で書いてください`},
		{`d.yaml:9:1: note: write text, not nothing`, `d.yaml:9:1: note: 空ではなく文字列で書いてください`},
		{`d.yaml:10:1: pick: write one of a, b`, `d.yaml:10:1: pick: a, b のどれかを書いてください`},
		{`d.yaml:11:9: open.y is not an item of the template report (a typo?)`, `d.yaml:11:9: open.y はテンプレート report に無い項目です（打ち間違い？）`},
	}
	got := schemaProblems(t, schema, src)
	for _, tt := range tests {
		if !strings.Contains(got.Error(), tt.en) {
			t.Errorf("the mistakes lack %q:\n%s", tt.en, got.Error())
		}
		if ja := got.Localize(i18n.New("ja")); !strings.Contains(ja, tt.ja) {
			t.Errorf("the mistakes lack %q:\n%s", tt.ja, ja)
		}
	}

	// A mapping allowed by pattern has no list of items to suggest.
	if strings.Contains(got.Error(), "open.y is not an item of the template report (a typo?); use") {
		t.Errorf("an item allowed by pattern lists the items: %s", got.Error())
	}

	// The lower bounds, and a mistake at the top level.
	low := schemaProblems(t, schema, "name: x\ntags: [a]\nid: AP-\nsize: 0\nrate: 0\n").Error()
	for _, want := range []string{
		"tags: the number of items must be at least 2", "id: the length in characters must be at least 4",
		"size: write 1 or more", "rate: write more than 0",
	} {
		if !strings.Contains(low, want) {
			t.Errorf("the mistakes lack %q:\n%s", want, low)
		}
	}
	if top := schemaProblems(t, `{"minProperties": 3}`, "a: 1\n").Error(); top != "d.yaml:1:1: the top level: the number of keys must be at least 3" {
		t.Errorf("a mistake at the top level = %s", top)
	}
}

// A schema refers to no other file: a file:// URL would read one out of the template.
func TestSchemaRefersToNoFile(t *testing.T) {
	t.Parallel()

	outside := filepath.Join(t.TempDir(), "defs.json")
	if err := os.WriteFile(outside, []byte(`{"type": "string"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	d, root := data(t, "a: x\n")
	set, err := builtinParts()
	if err != nil {
		t.Fatal(err)
	}
	err = validate(set, jsonDoc(t, `{"properties": {"a": {"$ref": "file://`+outside+`"}}}`), d, root)
	if err == nil || !strings.HasPrefix(err.Error(), "the schema of the template t cannot be read: file://"+outside+": a schema refers to no other file or URL") {
		t.Errorf("validate of a $ref to a file = %v", err)
	}
	// Its $schema names a draft of JSON Schema, not a schema of its own nor a part's, which jsonschema would
	// take as the rules of the schema.
	for _, named := range []string{"https://example.com/meta", "floe:///parts/table.json", "floe:///templates/t/schema.json"} {
		err = validate(set, jsonDoc(t, `{"$schema": "`+named+`"}`), d, root)
		if err == nil || err.Error() != "the schema of the template t cannot be read: $schema "+named+
			" names no draft of JSON Schema; name one, such as https://json-schema.org/draft/2020-12/schema, or leave it out" {
			t.Errorf("validate of the $schema %s = %v", named, err)
		}
	}
	for _, named := range []string{"https://json-schema.org/draft/2020-12/schema", "http://json-schema.org/draft-07/schema#", "https://json-schema.org/schema"} {
		if err = validate(set, jsonDoc(t, `{"$schema": "`+named+`"}`), d, root); err != nil {
			t.Errorf("validate of the $schema %s = %v", named, err)
		}
	}
}

func schemaProblems(t *testing.T, schema, src string) yamlpos.Problems {
	t.Helper()

	d, root := data(t, src)
	d.Template = "report"
	set, err := builtinParts()
	if err != nil {
		t.Fatal(err)
	}
	var p yamlpos.Problems
	if err := validate(set, jsonDoc(t, schema), d, root); !errors.As(err, &p) {
		t.Fatalf("validate = %v, want problems", err)
	}
	return p
}
