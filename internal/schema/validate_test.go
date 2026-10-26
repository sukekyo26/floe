package schema_test

import (
	"testing"

	"github.com/sukekyo26/floe/internal/doc"
)

// validate compiles schema and validates data (a data file of template t), telling what Validate says.
func validate(t *testing.T, body, data string) string {
	t.Helper()

	c, err := compile(t, body)
	if err != nil {
		t.Fatal(told(err))
	}
	d, err := doc.Read("d.yaml", []byte("template: t\nlang: en\n"+data))
	if err != nil {
		t.Fatal(told(err))
	}
	return told(c.Validate(d))
}

// Validate tells each mistake in the library's words, at the line and column of the value it is of, in the
// order of the file, the same each time (run with -count=20): the output is compared as it comes.
func TestValidate(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ name, body, data, want string }{
		{
			"required, type, additional properties",
			`"properties": {"n": {"type":"integer"}, "s": ` + N + `}, "required":["n"], "additionalProperties": false`,
			"s:\n  text: 3\n  bad: 1\nzz: 1\nyy: 2\n",
			lines("d.yaml:1:1: additional properties 'yy', 'zz' not allowed", "d.yaml:1:1: missing property 'n'",
				"d.yaml:4:3: additional properties 'bad' not allowed", "d.yaml:4:9: got number, want string"),
		},
		{
			"enum, minLength, oneOf",
			`"properties": {"k": {"enum":["a","b"]}, "m": {"type":"string","minLength":3}, "o": {"oneOf":[{"type":"string"},{"type":"integer"}]}}`,
			"k: c\nm: ab\no: [1]\n",
			lines("d.yaml:3:4: value must be one of 'a', 'b'", "d.yaml:4:4: minLength: got 2, want 3", "d.yaml:5:4: 'oneOf' failed, none matched"),
		},
		{
			"numbers doc.Read made ints",
			`"properties": {"n": {"type":"integer","minimum":5}, "f": {"type":"integer"}, "big": {"maximum":10}}`,
			"n: 3\nf: 3.5\nbig: 18446744073709551615\n",
			lines("d.yaml:3:4: minimum: got 3, want 5", "d.yaml:4:4: got number, want integer",
				"d.yaml:5:6: maximum: got 1.8446744073709552×10¹⁹, want 10"),
		},
		{"none", `"properties": {"n": {"type":"integer"}}`, "n: 1\n", ""},
	} {
		if got := validate(t, tt.body, tt.data); got != tt.want {
			t.Errorf("%s: Validate told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
	}
}

// A property name the schema refuses is told at the key of the one object at its depth that has it, else at
// the top of the file, the same each time (run with -count=20).
func TestPropertyNames(t *testing.T) {
	t.Parallel()

	const lower = `{"pattern": "^[a-z]+$"}`
	const bad = "invalid propertyName 'Bad'"
	for _, tt := range []struct{ name, body, data, want string }{
		{
			"one object", `"properties": {"title2": {"type":"string"}, "tags": {"type":"object","propertyNames":` + lower + `}}`,
			"title2: t\ntags:\n  ok: 1\n  Bad: b\n", "d.yaml:6:3: " + bad,
		},
		{
			"two items", `"properties": {"items": {"type":"array","items":{"type":"object","propertyNames":` + lower + `}}}`,
			"items:\n  - Bad: 1\n    a: 1\n  - Bad: 2\n    b: 2\n", lines("d.yaml:1:1: "+bad, "d.yaml:1:1: "+bad),
		},
		{
			"one $defs twice", `"$defs": {"pn": {"propertyNames": ` + lower + `}}, "properties": {"tags": {"allOf": [{"$ref": "#/$defs/pn"}, {"$ref": "#/$defs/pn"}]}}`,
			"tags:\n  ok: 1\n  Bad: b\n", lines("d.yaml:5:3: "+bad, "d.yaml:5:3: "+bad),
		},
		{
			"the key elsewhere", `"$defs": {"pn": {"propertyNames": ` + lower + `}}, "properties": {"tags": {"allOf": [{"$ref": "#/$defs/pn"}, {"$ref": "#/$defs/pn"}]}, "more": {}}`,
			"tags:\n  ok: 1\n  Bad: b\nmore:\n  Bad: 1\n", lines("d.yaml:1:1: "+bad, "d.yaml:1:1: "+bad),
		},
		{
			"two depths", `"properties": {"tags": {"propertyNames":` + lower + `}, "n2": {"type":"object","properties":{"m":{"propertyNames":` + lower + `}}}}`,
			"tags:\n  Bad: 1\nn2:\n  m:\n    Bad: 2\n", lines("d.yaml:4:3: "+bad, "d.yaml:7:5: "+bad),
		},
		{
			"the top", `"propertyNames": ` + lower + `, "additionalProperties": false`,
			"Bad: 1\n", lines("d.yaml:1:1: additional properties 'Bad' not allowed", "d.yaml:3:1: "+bad),
		},
		{
			"names in two objects", `"properties": {"tags": {"propertyNames":` + lower + `}, "more": {}, "x": {"propertyNames": {"pattern": "^[a-y]+$"}}, "y": {}}`,
			"tags:\n  Bad: 1\nmore:\n  Bad: 1\nx:\n  Zed: 1\ny:\n  Zed: 1\n",
			lines("d.yaml:1:1: "+bad, "d.yaml:1:1: invalid propertyName 'Zed'"),
		},
	} {
		if got := validate(t, tt.body, tt.data); got != tt.want {
			t.Errorf("%s: Validate told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
	}
}

// A Data without a node, as no file doc reads gives, is told at the top of the file.
func TestValidateNoNode(t *testing.T) {
	t.Parallel()

	c, err := compile(t, `"required": ["n"]`)
	if err != nil {
		t.Fatal(told(err))
	}
	d := &doc.Data{File: "d.yaml", Lang: "", Template: "", Theme: "", Title: "", Values: map[string]any{}, Root: nil, Where: nil}
	if got := told(c.Validate(d)); got != "d.yaml:1:1: missing property 'n'" {
		t.Errorf("Validate told %q", got)
	}
}
