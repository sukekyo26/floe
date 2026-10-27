package schema_test

import (
	"testing"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/schema"
)

const empty = `schema.json: the schema is empty: write a JSON Schema object, such as {"type": "object"}`

// A schema.json that is no JSON is told in jsontext's words at the line and column of the character, in the file
// past its byte order mark; one of nothing, and a value after the value, in floe's words.
func TestReadPlaces(t *testing.T) {
	t.Parallel()

	bomTold := "schema.json:1:1: " + i18n.EnglishCatalog().Msg("text_bom")
	after := func(at string) string {
		return "schema.json:" + at + ": " + i18n.EnglishCatalog().Msg("schema_after_value")
	}
	for _, tt := range []struct{ name, src, want string }{
		{"line 1", `{"type": }`, "schema.json:1:10: missing value after object name"},
		{"line 3", "{\n  \"type\": \"object\",\n  \"x\": ,\n}", "schema.json:3:8: invalid character ',' at start of value"},
		{"in a string", `{"a": "\q"}`, "schema.json:1:8: invalid escape sequence `\\q` in string"},
		{"cut in a value", "{\n  \"type\": \"obj", "schema.json:2:15: unexpected EOF"},
		{"cut after a key", "{\n  \"type\":", "schema.json:2:10: unexpected EOF"},
		{"cut, ending in a line break", "{\n  \"type\": \"object\"\n", "schema.json:3:1: unexpected EOF"},
		{"empty", "", empty},
		{"spaces", "  \n\t\n", empty},
		{"a mark alone", mark, empty},
		{"wide", `{"title": "日本語", "x": }`, "schema.json:1:23: missing value after object name"},
		{"wide, line 2", "{\n\"説明\": \"あいう\" ,, \n}", "schema.json:2:14: invalid character ',' at start of value"},
		{"wide, the character", `{"a": 日}`, "schema.json:1:7: invalid character '日' at start of value"},
		{"a mark", mark + `{"type": }`, "schema.json:1:10: missing value after object name"},
		{"two marks", mark + mark + `{}`, bomTold},
		{"CRLF", "{\r\n  \"x\": ,\r\n}", "schema.json:2:8: invalid character ',' at start of value"},
		{"CR", "{\r  \"x\": ,\r}", "schema.json:2:8: invalid character ',' at start of value"},
		{"tab", "{\n\t\"x\": ,\n}", "schema.json:2:7: invalid character ',' at start of value"},
		{"after the value", `{} x`, after("1:4")},
		{"after the value, line 2", "{}\n  x", after("2:3")},
		{"after the value, wide", `{} 日`, after("1:4")},
		{"a second value", `{} {}`, after("1:4")},
		{"a mark after the value", `{}` + mark, bomTold[:len("schema.json:1:")] + "3" + bomTold[len("schema.json:1:1"):]},
		{"inside a character", "{\"a\": \"\\日\"}", "schema.json:1:8: invalid escape sequence \"\\\\\\xe6\" in string"},
		{"a bad \\u", `{"a": "\u12g4"}`, "schema.json:1:8: invalid escape sequence `\\u12g4` in string"},
		{"a short \\u", `{"a": "\u12"}`, "schema.json:1:8: invalid escape sequence `\\u12\"}` in string"},
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
// through Compile and NewResource alike: by its name as decoded, so that an escape is one name, and from the
// second on, those before a syntax mistake too. Keys alike in other objects, and values alike, are no mistake.
func TestKeysTwice(t *testing.T) {
	t.Parallel()

	twice := func(at, name string) string {
		return "schema.json:" + at + ": the key " + name + " is in this object already: only the last would be read; keep one"
	}
	for _, tt := range []struct{ name, src, want string }{
		{"required", `{"required": ["n"], "required": []}`, twice("1:21", `"required"`)},
		{"nested", `{"$defs": {"x": {"minLength": 3, "minLength": 0}}}`, twice("1:34", `"minLength"`)},
		{"escaped", `{"required": ["n"], "\u0072equired": []}`, twice("1:21", `"required"`)},
		{"three", `{"type": "object", "type": "object", "type": "object"}`, lines(twice("1:20", `"type"`), twice("1:38", `"type"`))},
		{"lines", "{\n  \"type\": \"object\",\n\t\"type\": \"object\"\n}", twice("3:2", `"type"`)},
		{"a mark", mark + `{"a": 1, "a": 2}`, twice("1:10", `"a"`)},
		{"a syntax mistake after", `{"a": 1, "a": }`, lines(twice("1:10", `"a"`), "schema.json:1:15: missing value after object name")},
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

// What encoding/json would read without a word, as U+FFFD or as a character of a name, is refused where it is
// written: a lone surrogate and a byte that is no UTF-8 in jsontext's words, a byte order mark past the start in
// floe's, the one and the other together; one jsontext stops at too is told once, as a mark.
func TestMisread(t *testing.T) {
	t.Parallel()

	mark := "\xef\xbb\xbf"
	bomAt := func(at string) string { return "schema.json:" + at + ": " + i18n.EnglishCatalog().Msg("text_bom") }
	surrogate := func(at, quoted string) string {
		return "schema.json:" + at + ": invalid surrogate pair " + quoted + " in string"
	}
	for _, tt := range []struct{ name, src, want string }{
		{"a high one", `{"description": "a\ud800b"}`, surrogate("1:19", "`\\ud800b\"}`")},
		{"a low one", `{"description": "a\udc00b"}`, surrogate("1:19", "`\\udc00b\"}`")},
		{"in a key", `{"properties": {"x\ud800": {}}}`, surrogate("1:19", `"\\ud800\": {}}"`)},
		{"after a written U+FFFD", "{\"a\": \"\xef\xbf\xbd \\ud800\"}", surrogate("1:10", "`\\ud800\"}`")},
		{"after an escaped backslash", `{"a": "\\ufffd \ud800"}`, surrogate("1:16", "`\\ud800\"}`")},
		{"keys of two", `{"properties": {"\ud800": {}, "\udc00": {}}}`, surrogate("1:18", `"\\ud800\": {},"`)},
		{"a key twice, then one", `{"a": 1, "a": 2, "b": "\ud800"}`, lines("schema.json:1:10: the key \"a\" is in this object already: only the last would be read; keep one", surrogate("1:24", "`\\ud800\"}`"))},
		{"no UTF-8", "{\"a\": \"a\xc3b\"}", "schema.json:1:9: invalid UTF-8"},
		{"a mark in a key", "{\"properties\": {\"" + mark + "title\": {}}}", bomAt("1:18")},
		{"a mark in a value and a surrogate", "{\"a\": \"" + mark + "x\", \"b\": \"\\ud800\"}", lines(bomAt("1:8"), surrogate("1:19", "`\\ud800\"}`"))},
		{"a mark between tokens", "{\"a\": 1," + mark + " \"b\": 2}", bomAt("1:9")},
		{"a mark at the start and between", mark + "{\"a\": 1," + mark + " \"b\": 2}", bomAt("1:9")},
		{"a pair", `{"a": "a\ud83d\ude00b"}`, ""},
		{"an escaped backslash", `{"a": "a\\ud800b"}`, ""},
		{"an escaped mark", `{"a": "\ufeffx"}`, ""},
	} {
		_, err := schema.Compile(tmplID, "schema.json", []byte(tt.src), nil)
		if got := told(err); got != tt.want {
			t.Errorf("%s: Compile told\n%s\nwant\n%s", tt.name, got, tt.want)
		}
	}
}
