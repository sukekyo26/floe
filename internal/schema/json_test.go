package schema_test

import (
	"testing"

	"github.com/sukekyo26/floe/internal/schema"
)

const empty = `schema.json: the schema is empty: write a JSON Schema object, such as {"type": "object"}`

// A schema.json that is no JSON is told in encoding/json's words at the line and column of the character, in
// the file past its byte order mark; one of nothing, in floe's words.
func TestReadPlaces(t *testing.T) {
	t.Parallel()

	bomMark := "\\" + "ufeff" // the words encoding/json writes for a byte order mark it meets
	for _, tt := range []struct{ name, src, want string }{
		{"line 1", `{"type": }`, "schema.json:1:10: invalid character '}' looking for beginning of value"},
		{"line 3", "{\n  \"type\": \"object\",\n  \"x\": ,\n}", "schema.json:3:8: invalid character ',' looking for beginning of value"},
		{"in a string", `{"a": "\q"}`, "schema.json:1:9: invalid escape sequence `\\q` in string"},
		{"cut in a value", "{\n  \"type\": \"obj", "schema.json:2:15: unexpected EOF"},
		{"cut after a key", "{\n  \"type\":", "schema.json:2:10: unexpected EOF"},
		{"cut, ending in a line break", "{\n  \"type\": \"object\"\n", "schema.json:2:19: unexpected EOF"},
		{"empty", "", empty},
		{"spaces", "  \n\t\n", empty},
		{"a mark alone", mark, empty},
		{"wide", `{"title": "日本語", "x": }`, "schema.json:1:23: invalid character '}' looking for beginning of value"},
		{"wide, line 2", "{\n\"説明\": \"あいう\" ,, \n}", "schema.json:2:14: invalid character ',' looking for beginning of object key string"},
		{"wide, the character", `{"a": 日}`, "schema.json:1:7: invalid character '日' looking for beginning of value"},
		{"a mark", mark + `{"type": }`, "schema.json:1:10: invalid character '}' looking for beginning of value"},
		{"two marks", mark + mark + `{}`, "schema.json:1:1: invalid character '" + bomMark + "' looking for beginning of value"},
		{"CRLF", "{\r\n  \"x\": ,\r\n}", "schema.json:2:8: invalid character ',' looking for beginning of value"},
		{"CR", "{\r  \"x\": ,\r}", "schema.json:2:8: invalid character ',' looking for beginning of value"},
		{"tab", "{\n\t\"x\": ,\n}", "schema.json:2:7: invalid character ',' looking for beginning of value"},
		{"after the value", `{} x`, "schema.json:1:4: invalid character 'x' after top-level value"},
		{"after the value, line 2", "{}\n  x", "schema.json:2:3: invalid character 'x' after top-level value"},
		{"after the value, wide", `{} 日`, "schema.json:1:4: invalid character '日' after top-level value"},
		{"inside a character", "{\"a\": \"\\日\"}", "schema.json:1:9: invalid escape sequence \"\\\\\\xe6\" in string"},
		// LIMIT: a bad \u escape is placed at its end.
		{"a bad \\u", `{"a": "\u12g4"}`, "schema.json:1:13: invalid escape sequence `\\u12g4` in string"},
		{"a short \\u", `{"a": "\u12"}`, "schema.json:1:13: invalid escape sequence `\\u12\"}` in string"},
		// What reads as JSON is as it was.
		{"null", `null`, "schema.json: #: got null, want boolean or object"},
		{"right", `{"type":"object"}`, ""},
	} {
		_, err := schema.Compile(tmplID, "schema.json", []byte(tt.src), nil)
		if got := told(err); got != tt.want {
			t.Errorf("%s: Compile told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
	}
}

// A key an object of a schema has already, which encoding/json would read over the first, is refused at it,
// through Compile and NewResource alike: by its name as decoded, so that an escape or lone surrogates are one
// name, and from the second on. Keys alike in other objects, and values alike, are no mistake.
func TestKeysTwice(t *testing.T) {
	t.Parallel()

	twice := func(at, name string) string {
		return "schema.json:" + at + ": the key " + name + " is in this object already: only the last would be read; keep one"
	}
	for _, tt := range []struct{ name, src, want string }{
		{"required", `{"required": ["n"], "required": []}`, twice("1:21", `"required"`)},
		{"nested", `{"$defs": {"x": {"minLength": 3, "minLength": 0}}}`, twice("1:34", `"minLength"`)},
		{"escaped", `{"required": ["n"], "\u0072equired": []}`, twice("1:21", `"required"`)},
		{"lone surrogates", `{"properties": {"\ud800": {}, "\udc00": {}}}`, twice("1:31", `"`+"�"+`"`)},
		{"one in a name", `{"properties": {"x\ud800y": {}, "x\udc00y": {}}}`, twice("1:33", `"x`+"�"+`y"`)},
		{"three", `{"type": "object", "type": "object", "type": "object"}`, lines(twice("1:20", `"type"`), twice("1:38", `"type"`))},
		{"lines", "{\n  \"type\": \"object\",\n\t\"type\": \"object\"\n}", twice("3:2", `"type"`)},
		{"a mark", mark + `{"a": 1, "a": 2}`, twice("1:10", `"a"`)},
		{"a syntax mistake first", `{"a": 1, "a": }`, "schema.json:1:15: invalid character '}' looking for beginning of value"},
		{"after an empty object", `{"a": {}, "b": 1}`, ""},
		{"after an empty object in an array", `{"a": [{}, {"x": 1}], "b": 2}`, ""},
		{"one name nested", `{"a": {"a": {"a": 1}}}`, ""},
		{"a number of no float", `{"maximum": 1e400}`, ""},
		{"siblings", `{"a": {"type": "string"}, "b": {"type": "string"}}`, ""},
		{"cases", `{"a": 1, "A": 2}`, ""},
		{"values alike", `{"enum": ["x", "x"]}`, ""},
		{"names of the schema's words", `{"properties": {"properties": {"type": "string"}, "type": {"type": "string"}}}`, ""},
		{"objects in an array alike", `{"properties": {"n": {"anyOf": [{"type": "string"}, {"type": "number"}]}}}`, ""},
	} {
		_, err := schema.Compile(tmplID, "schema.json", []byte(tt.src), nil)
		if got := told(err); got != tt.want {
			t.Errorf("%s: Compile told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
		_, err = schema.NewResource("floe:///parts/mine.json", "schema.json", []byte(tt.src), schema.Part)
		if got := told(err); got != tt.want {
			t.Errorf("%s: NewResource told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
	}
}
