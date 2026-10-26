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
		{"after the value", `{} x`, "schema.json:1:4: invalid character after top-level value"},
		{"after the value, line 2", "{}\n  x", "schema.json:2:3: invalid character after top-level value"},
		{"after the value, wide", `{} 日`, "schema.json:1:4: invalid character after top-level value"},
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
