package yamlpos_test

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"go.yaml.in/yaml/v4"

	"floe/internal/i18n"
	"floe/internal/usererr"
	"floe/internal/yamlpos"
)

const bom = "\xef\xbb\xbf"

func told(err error) string {
	if err == nil {
		return ""
	}
	return usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: ")
}

// placed is each mistake of err as its line, column and catalog key, a line each, in order; a failure as such.
func placed(err error) string {
	var found usererr.List
	if failed := found.Add(err); failed != nil {
		return "failure: " + failed.Error()
	}
	lines := make([]string, len(found))
	for i, e := range found {
		lines[i] = fmt.Sprintf("%d:%d %s", e.Line(), e.Column(), e.Key)
	}
	return strings.Join(lines, "\n")
}

// Parse refuses what floe does not read as written, each where it is written, all of a file at once in the
// order of its lines, walking neither what an anchor, alias or tag holds nor a second document; it passes
// what YAML 1.2 reads as written.
func TestParse(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ src, want string }{
		// go.yaml.in/yaml/v4's own mistakes, at its place.
		{"title: a: b\n", "1:9 library"},
		{"title: `x`\n", "1:8 library"},
		{"t: |\n    a\n  b\n", "3:3 yaml_after_block"},
		{"title: %y\n", "1:8 library"},
		{"%YAML 1.2\n---\nlang: en\n", "1:1 library"},
		{"%FOO bar\n---\nlang: en\n", "1:5 library"},
		{"x: \"" + `\ud83d\ude00` + "\"\n", "1:7 library"},
		{"x: \"abc\ny: 1\n", "3:1 library"},
		{"x: [1, 2\ny: 3\n", "2:2 library"},
		// One document: a second one holding something, at its start.
		{"lang: en\n---\ntitle: x\n", "2:1 yaml_documents"},
		{"lang: en\n---\nx: &a 1\n", "2:1 yaml_documents"},
		{"%YAML 1.1\n---\nlang: en\n---\nx: 1\n", "4:1 yaml_documents"},
		{"a: 1\n---\n---\nb: 2\n", "3:1 yaml_documents"},
		{"a: 1\n---\n~\n", "2:1 yaml_documents"},
		{"a: 1\n--- null\n", "2:1 yaml_documents"},
		{"a: 1\n--- ''\n", "2:1 yaml_documents"},
		{"a: 1\n--- !!null\n", "2:1 yaml_documents"},
		{"a: 1\n--- |\n", "2:1 yaml_documents"},
		// A document with nothing written is none.
		{"---\n---\nid: d\n", ""},
		{"lang: en\n---\n---\n---\n", ""},
		{bom + "---\n---\nlang: en\n", ""},
		{"---\nlang: en\n...\n", ""},
		{"lang: en\n---\n", ""},
		{"a: 1\n---\n# c\n", ""},
		{"a: 1\r\n---\r\n\r\n", ""},
		{"a: 1\n---\n... # end\n", ""},
		{"%YAML 1.1\n---\nlang: en\n", ""},
		// Anchors, aliases, merge keys and tags, not what they hold; a tag of ! alone too.
		{"x: &a 1\ny: *a\n", "1:4 yaml_no_anchors\n2:4 yaml_no_anchors"},
		{"title: !!str 3\n", "1:8 yaml_no_anchors"},
		{"title: ! 3\n", "1:8 yaml_no_anchors"},
		{"&k x: 1\n", "1:1 yaml_no_anchors"},
		{"k: &a key\n*a : 1\n", "1:4 yaml_no_anchors\n2:1 yaml_no_anchors"},
		{"!!str 0x10: 1\n", "1:1 yaml_no_anchors"},
		{"m: {a: 1}\nx:\n  <<: {b: 2}\n", "3:3 yaml_no_anchors"},
		{"--- !!map\nlang: en\n", "1:5 yaml_no_anchors"},
		{"&top\nlang: en\n", "1:1 yaml_no_anchors"},
		{"x: {a: 1, &b c}\n", "1:11 yaml_no_anchors"},
		// Keys, as keys: text as YAML 1.2 reads it, and each once in its mapping.
		{"0x10: a\n~: 1\ntrue: 1\n", "1:1 yaml_key_not_text\n2:1 yaml_key_not_text\n3:1 yaml_key_not_text"},
		{".inf: 1\n1e3: a\n08: b\n", "1:1 yaml_key_not_text\n2:1 yaml_key_not_text\n3:1 yaml_key_not_text"},
		{"x: {a: 1, 5}\n", "1:11 yaml_key_not_text"},
		{"[a]: 1\n", "1:1 yaml_key_not_text"},
		{"lang: en\nlang: ja\n", "2:1 yaml_key_twice"},
		{"x: {a: 1, a: 2}\ny:\n  a: 1\n  b: 2\n  a: 3\n", "1:11 yaml_key_twice\n5:3 yaml_key_twice"},
		{"\"a\": 1\na: 2\n", "2:1 yaml_key_twice"},
		{"a: 1\nb:\n  a: 2\n", ""},
		{"\"0x10\": a\n\"1e3\": a\n", ""},
		// A key in { } with no value written.
		{"x: {a, b: 1}\n", "1:5 yaml_flow_key_alone"},
		{"x: {a: , b: 1}\n", "1:5 yaml_flow_key_alone"},
		{"x: {text: Hello, world}\n", "1:18 yaml_flow_key_alone"},
		{"x: [k: ]\n", "1:5 yaml_flow_key_alone"},
		{"x: {a: ~, b: null, c: ''}\n", ""},
		// Values YAML 1.2 reads as numbers floe does not take.
		{"x: .inf\n", "1:4 yaml_not_finite"},
		{"x: [.NaN, -.Inf]\n", "1:5 yaml_not_finite\n1:11 yaml_not_finite"},
		{"x: 99999999999999999999\n", "1:4 yaml_number_too_big"},
		{"x: -9223372036854775809\n", "1:4 yaml_number_too_big"},
		{"x: 030000000000000000000\n", "1:4 yaml_number_too_big"},
		{"x: 1e400\n", "1:4 yaml_number_outside_float"},
		{"a: 1e-400\nb: 1.0e-400\nc: -1e-400\n", "1:4 yaml_number_outside_float\n2:4 yaml_number_outside_float\n3:4 yaml_number_outside_float"},
		{"a: 0e-400\nb: 0.0e-400\nc: -0.0e-400\n", ""},
		// All of a file at once, in the order of its lines.
		{"x: &a 1\n0x10: a\nx: 2\n", "1:4 yaml_no_anchors\n2:1 yaml_key_not_text\n3:1 yaml_key_twice"},
		// What YAML 1.2 reads as written passes: numbers YAML 1.1 reads otherwise, a tab in a | text.
		{"a: 017\nb: -017\nc: 0b101\nd: 1_000\ne: -0x1F\nf: +-1\ng: 1e3\nh: 012e3\ni: 08\n", ""},
		{"a: 0o17\nb: 0x1F\nc: +1\nd: 0\ne: 00\nf: .5\ng: 1.0\nh: 1.\ni: 012.5\nj: 18446744073709551615\n", ""},
		{"a: \"017\"\nb: '1e3'\nc: Plan 2026\nd: v1.0\ne: 2026-10-26\nf: 1.2.3\ng: 1,000\nh: +.nan\n", ""},
		{"c: |\n  \tx\n  b\n", ""},
		{"x: \"a\tb\n\tc\"\n", ""},
		{"title: 'Plan #1 draft'\n", ""},
		{"title: Plan#1\n", ""},
		{"# a note\nsections: []  # trailing\n", ""},
		{"a:\n-\nb: 1\n", ""},
		// LIMIT: text a , cut, or a , left out over lines, as the LIMIT of node tells.
		{"x: [1,500 words]\n", ""},
		{"x: [a\n  b]\n", ""},
	} {
		_, err := yamlpos.Parse("a.yaml", []byte(tt.src))
		if got := placed(err); got != tt.want {
			t.Errorf("Parse(%q) told\n%s\nwant\n%s", tt.src, got, tt.want)
		}
	}
	for _, src := range []string{"", "# just a note\n", bom, bom + "# just a note\n", "---\n", "---\n...\n", bom + "---\n---\n"} {
		if _, err := yamlpos.Parse("a.yaml", []byte(src)); !errors.Is(err, yamlpos.ErrEmpty) {
			t.Errorf("Parse(%q) = %v, want ErrEmpty", src, err)
		}
	}
}

// go.yaml.in/yaml/v4's mistake is told in its words alone, without the context it puts before them.
func TestParseLibraryWords(t *testing.T) {
	t.Parallel()

	src := []byte("a:\n  - x\n y\n")
	l, err := yaml.NewLoader(bytes.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	var le *yaml.LoadError
	if !errors.As(l.Load(new(yaml.Node)), &le) || le.ContextMsg == "" {
		t.Fatalf("go.yaml.in/yaml/v4 gives no mistake with a context for %q", src)
	}
	_, err = yamlpos.Parse("a.yaml", src)
	if got, want := told(err), fmt.Sprintf("a.yaml:%d:%d: %s", le.Mark.Line, le.Mark.Column, le.Message); got != want {
		t.Errorf("told %q, want %q", got, want)
	}
}

// A mistake go.yaml.in/yaml/v4 tells on a line right after a | or > text is told with how to mend the text after
// its words, whatever the mistake; its words and its place are those of a mistake told without. One elsewhere, one
// on a line past an empty [], {} or alias after the text, one with no line of its own and one where what is before
// its line is not read are told as they were. A text written right passes.
func TestParseAfterBlock(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ src, want string }{
		{"a: |\n   one\n  two\nb: x\n", "3:3 yaml_after_block"},
		{"a: >\n   one\n  two\nb: x\n", "3:3 yaml_after_block"},
		{"l:\n  - |\n     one\n    two\n  - x\n", "4:5 yaml_after_block"},
		{"a: |\n\n   one\n  two\nb: x\n", "4:3 yaml_after_block"},
		{"- |\n   one\n  two\n", "3:3 yaml_after_block"},
		{"a: |\n  ok\nb: |\n   one\n  two\n", "5:3 yaml_after_block"},
		// Whatever the mistake: a table, a diff's head and a tab are the scanner's, an alias of no anchor the
		// composer's.
		{"a: |\n   one\n  | col | col |\n", "3:5 yaml_after_block"},
		{"a: |\n   context\n  @@ -1 +1 @@\n", "3:3 yaml_after_block"},
		{"a: |\n   one\n\ttwo\nb: x\n", "3:1 yaml_after_block"},
		{"a: |\n  one\nb: *x\n", "3:4 yaml_after_block"},
		// Not after the text: an empty [], {} or alias is written last before the line.
		{"- |\n  x\n- []\n  y\n", "4:3 library"},
		{"- |\n  x\n- {}\n  y\n", "4:3 library"},
		{"- &a x\n- |\n  t\n- *a\n  y\n", "5:3 library"},
		// No | or > text before the line.
		{"a:\n  - x\n y\n", "3:2 library"},
		{"a: \"one\n  two\"\n b\n", "3:2 library"},
		// What is before the line is not read: the mistake told is the first one, not that of reading it.
		{"p:\n  a: |\n     one\ntwo\n  b: x\n", "5:4 library"},
		{"a: |\n   one\n  two\n  three: x\n", "4:8 library"},
		// A mistake with no line of its own, placed by its byte inside the text.
		{"a: |\n  one\n  tw\x01o\nb: x\n", "3:5 library"},
		// Right.
		{"a: |2\n   one\n  two\nb: x\n", ""},
		{"a: |\n  one\n   two\nb: x\n", ""},
		{"l:\n  - a: |\n        one\n    two: y\n", ""},
	} {
		_, err := yamlpos.Parse("a.yaml", []byte(tt.src))
		if got := placed(err); got != tt.want {
			t.Errorf("Parse(%q) = %q, want %q", tt.src, got, tt.want)
			continue
		}
		at, key, _ := strings.Cut(tt.want, " ")
		if key == "" {
			continue
		}
		l, lerr := yaml.NewLoader(strings.NewReader(tt.src))
		if lerr != nil {
			t.Fatal(lerr)
		}
		var le *yaml.LoadError
		if !errors.As(l.Load(new(yaml.Node)), &le) {
			t.Fatalf("go.yaml.in/yaml/v4 gives no mistake for %q", tt.src)
		}
		plain := fmt.Sprintf("a.yaml:%s: %s", at, le.Message)
		got := told(err)
		if key == "library" && got != plain || key != "library" && !strings.HasPrefix(got, plain+": ") {
			t.Errorf("Parse(%q) told %q, want it to start with %q", tt.src, got, plain)
		}
	}
}

// A key that is not text is named in its mistake: by its text, or by the line it is written on when it has none.
func TestParseKeyNames(t *testing.T) {
	t.Parallel()

	for src, at := range map[string]string{
		"[a]: 1\n": "1:1", "{a: 1}: 1\n": "1:1", "? [a]\n: 1\n": "1:3", "? a: 1\n: x\n": "1:3", "x:\n  : 1\n": "2:3",
		"0x10: a\n": "1:1",
	} {
		_, err := yamlpos.Parse("a.yaml", []byte(src))
		var found usererr.List
		if failed := found.Add(err); failed != nil || len(found) != 1 || found[0].Key != "yaml_key_not_text" ||
			fmt.Sprintf("%d:%d", found[0].Line(), found[0].Column()) != at || fmt.Sprint(found[0].Args[0]) == "" {
			t.Errorf("Parse(%q) = %v, want yaml_key_not_text at %s with its name", src, err, at)
		}
	}
}

// A byte order mark past the start and a byte that is no UTF-8 are refused where they are, the one and the
// other together and before go.yaml.in/yaml/v4 reads the file, which would read a mark as a character of a key
// or a value. The mark is refused in quotes and comments too; an escape of it is no mark.
// The first byte that is no UTF-8 is told once, a U+FFFD written in UTF-8 being a character.
func TestParseMarksAndUTF8(t *testing.T) {
	t.Parallel()

	mark := i18n.EnglishCatalog().Msg("text_bom")
	utf8 := i18n.EnglishCatalog().Msg("yaml_not_utf8")
	for _, tt := range []struct{ name, src, want string }{
		{"a second mark", bom + bom + "title: t\n", "a.yaml:1:1: " + mark},
		{"at a line's start", "title: t\n" + bom + "note: x\n", "a.yaml:2:1: " + mark},
		{"in a plain value", "note: " + bom + "Plan\n", "a.yaml:1:7: " + mark},
		{"in a block", "note: |\n  a" + bom + "b\n", "a.yaml:2:4: " + mark},
		{"in a block of lines", "note: |\n  aaa\n  bb" + bom + "b\n  cc\n", "a.yaml:3:5: " + mark},
		{"on a line alone", "a: 1\nb: 2\n" + bom + "\nd: 4\n", "a.yaml:3:1: " + mark},
		{"before an indent", "note:\n" + bom + "  a: b\n", "a.yaml:2:1: " + mark},
		{"in a value over lines, spaces at a line's end", "note: hello \n  world" + bom + "\n", "a.yaml:2:8: " + mark},
		{"in a block of a bad head", "note: |x\n  y" + bom + "z\n", "a.yaml:2:4: " + mark},
		{"with CRLF", "a: 1\r\nnote: a" + bom + "b\r\n", "a.yaml:2:8: " + mark},
		{"in a tag", "note: !!str " + bom + "x\n", "a.yaml:1:13: " + mark},
		{"in an anchor", "note: &" + bom + "a 1\n", "a.yaml:1:8: " + mark},
		{"in a flow sequence", "note: [a, " + bom + "b]\n", "a.yaml:1:11: " + mark},
		{"in a flow mapping", "note: {" + bom + "k: v}\n", "a.yaml:1:8: " + mark},
		{"after a quoted value", "note: \"x\"" + bom + "\n", "a.yaml:1:10: " + mark},
		{"in double quotes", "note: \"" + bom + "Plan\"\n", "a.yaml:1:8: " + mark},
		{"in single quotes", "note: 'x" + bom + "y'\n", "a.yaml:1:9: " + mark},
		{"in a comment", "a: 1\n# a note " + bom + "\n", "a.yaml:2:10: " + mark},
		{"FF FE", "\xff\xfetitle: t\n", "a.yaml:1:1: " + utf8},
		{"in a value", "note: a\xc3b\n", "a.yaml:1:8: " + utf8},
		{"once in a file", "a: \xc3\nb: \xc3\n", "a.yaml:1:4: " + utf8},
		{"after a U+FFFD written", "note: \xef\xbf\xbd\xc3x\n", "a.yaml:1:8: " + utf8},
		{"in a comment, as Shift_JIS", "# \x83\x52\n", "a.yaml:1:3: " + utf8},
		{"UTF-32BE", "\x00\x00\xfe\xff", "a.yaml:1:3: " + utf8},
		{"no UTF-8 and a mark", "a: \xc3\nb: " + bom + "x\n", "a.yaml:1:4: " + utf8 + "\na.yaml:2:4: " + mark},
		{"an escape of a mark", "note: \"\\" + "ufeffPlan\"\n", ""},
		{"spaces at a line's end, no mark", "a: x \n  y\nb: 1\n", ""},
		{"the first mark", bom + "a: 1\n", ""},
	} {
		_, err := yamlpos.Parse("a.yaml", []byte(tt.src))
		if got := told(err); got != tt.want {
			t.Errorf("%s: Parse(%q) told\n%s\nwant\n%s", tt.name, tt.src, got, tt.want)
		}
	}
}

// U+0085, U+2028 and U+2029 written as they are are refused at each place, in quotes, keys and comments too,
// in the order of places with a byte order mark past the start, but not in a file that is no UTF-8:
// go.yaml.in/yaml/v4 reads them as line breaks. An escape of one is no such character.
func TestParseLineSeparators(t *testing.T) {
	t.Parallel()

	sep := func(name, escape string) string {
		return i18n.EnglishCatalog().Msg("yaml_line_separator", name, escape)
	}
	mark := i18n.EnglishCatalog().Msg("text_bom")
	nel, ls, ps := "\u0085", "\u2028", "\u2029"
	for _, tt := range []struct{ name, src, want string }{
		{"NEL in a plain value", "note: a" + nel + "b\n", "a.yaml:1:8: " + sep("U+0085", `\u0085`)},
		{"LS in a plain value", "note: a" + ls + "b\n", "a.yaml:1:8: " + sep("U+2028", `\u2028`)},
		{"PS in a plain value", "note: a" + ps + "b\n", "a.yaml:1:8: " + sep("U+2029", `\u2029`)},
		{"in single quotes", "a: 1\nnote: 'x" + ls + "y'\n", "a.yaml:2:9: " + sep("U+2028", `\u2028`)},
		{"in double quotes", "note: \"x" + nel + "\"\n", "a.yaml:1:9: " + sep("U+0085", `\u0085`)},
		{"in a comment", "a: 1\n# a note " + ps + "b: 2\n", "a.yaml:2:10: " + sep("U+2029", `\u2029`)},
		{"in a block", "note: |\n  aaa\n  b" + ls + "c\n", "a.yaml:3:4: " + sep("U+2028", `\u2028`)},
		{"each place", "a: x" + ps + "\nb: y" + nel + "\n", "a.yaml:1:5: " + sep("U+2029", `\u2029`) + "\na.yaml:2:5: " + sep("U+0085", `\u0085`)},
		{"in a key", "a" + ls + "b: 1\n", "a.yaml:1:2: " + sep("U+2028", `\u2028`)},
		{"with CRLF", "a: 1\r\nb: x" + ls + "\r\n", "a.yaml:2:5: " + sep("U+2028", `\u2028`)},
		{"after the first mark", bom + "note: a" + ls + "\n", "a.yaml:1:8: " + sep("U+2028", `\u2028`)},
		{"with a mark inside", "a: " + bom + "x\nb: " + ls + "\n", "a.yaml:1:4: " + mark + "\na.yaml:2:4: " + sep("U+2028", `\u2028`)},
		{"before a mark inside", "a: " + ls + "\nb: " + bom + "x\n", "a.yaml:1:4: " + sep("U+2028", `\u2028`) + "\na.yaml:2:4: " + mark},
		{"in a file not UTF-8", "a: \xc3\nb: " + ls + "\n", "a.yaml:1:4: " + i18n.EnglishCatalog().Msg("yaml_not_utf8")},
		{"escapes", "note: \"\\u0085 \\u2028 \\u2029 \\N \\L \\P\"\n", ""},
	} {
		_, err := yamlpos.Parse("a.yaml", []byte(tt.src))
		if got := told(err); got != tt.want {
			t.Errorf("%s: Parse(%q) told\n%s\nwant\n%s", tt.name, tt.src, got, tt.want)
		}
	}
}

// A file over 1 MiB is refused at the file alone, counted as its bytes with the byte order mark at its start, and
// before what else is refused of its bytes.
func TestParseSize(t *testing.T) {
	t.Parallel()

	of := func(size int, head string) string {
		return head + "a: 1\n#" + strings.Repeat("x", size-len(head)-len("a: 1\n#\n")) + "\n"
	}
	tooLarge := func(size int) string {
		return "a.yaml: " + i18n.EnglishCatalog().Msg("yaml_too_large", size, 1<<20)
	}
	for _, tt := range []struct{ name, src, want string }{
		{"1 MiB", of(1<<20, ""), ""},
		{"1 MiB with a mark", of(1<<20, bom), ""},
		{"a byte over", of(1<<20+1, ""), tooLarge(1<<20 + 1)},
		{"a byte over with a mark", of(1<<20+1, bom), tooLarge(1<<20 + 1)},
		{"over, not UTF-8", of(1<<20+1, "b: \xc3\n"), tooLarge(1<<20 + 1)},
	} {
		_, err := yamlpos.Parse("a.yaml", []byte(tt.src))
		if got := told(err); got != tt.want {
			t.Errorf("%s: Parse of %d bytes told\n%s\nwant\n%s", tt.name, len(tt.src), got, tt.want)
		}
	}
}

// A tab is a character of a value as YAML 1.2 reads it: after the indentation of a | or > text (with spaces before
// it or not, on its first line or after), and inside a plain value. One where indentation goes is
// go.yaml.in/yaml/v4's mistake.
func TestParseTabs(t *testing.T) {
	t.Parallel()

	for src, want := range map[string]string{
		"c: |\n  \tx\n  b\n":   "\tx\nb\n",
		"c: |\n  \tx\n  # h\n": "\tx\n# h\n",
		"c: >\n  \tx\n":        "\tx\n",
		"c: |\r\n  \tx\r\n":    "\tx\n",
		"c: |\n  x\n  \ty\n":   "x\n\ty\n",
		"c: |2\n  \tx\n":       "\tx\n",
		"c: a\t\tb\n":          "a\t\tb",
		"c: \"\tx\"\n":         "\tx",
	} {
		body, err := yamlpos.Parse("a.yaml", []byte(src))
		if err != nil {
			t.Fatalf("Parse(%q): %s", src, told(err))
		}
		if got, _ := yamlpos.Text(body.Pairs[0].Value); got != want {
			t.Errorf("Parse(%q) reads %q, want %q", src, got, want)
		}
	}
	if _, err := yamlpos.Parse("a.yaml", []byte("c: |\n\tx\n")); placed(err) != "2:1 yaml_after_block" {
		t.Errorf("a tab as indentation: %q", placed(err))
	}
}

// A # in { } or [ ] that took a flow's end leaves it open, go.yaml.in/yaml/v4's own mistake, at its place; a comment
// after a number or on a line of its own passes.
func TestParseFlowComments(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ name, src, want string }{
		{"a comma's", "x: [a, # c ]\n", "2:1 library"},
		{"one a flow over lines holds right", "ports: [\n  8080,  # admin\n  9090,\n]\n", ""},
		{"one on a line of its own", "x: [\n  # c\n  a,\n]\n", ""},
		{"one that took the end of the innermost, at its start", "items: [\n  {name: Issue #1},\n]\n", "3:1 library"},
		{"one on a later line that took the end", "tags: [\n  a, Issue #1]\ny: 1\n", "3:1 library"},
		{"one that took two ends", "x: [\n  {a: 1,\n   b: Issue #1}]\ny: 1\n", "4:1 library"},
		{"one in a sequence", "x: [a, PR #378, c]\ny: 1\n", "2:1 library"},
		{"one with a # in it", "x: [a, PR #1 and #2, c]\ny: 1\n", "2:1 library"},
		{"each of two flows", "x: [a #1]\ny: [b #2]\nz: 1\n", "2:1 library"},
		{"over CRLF lines and a tab", "# c\r\nx: [a\tb, Issue #1]\r\ny: 1\r\n", "3:1 library"},
		{"one without a space before it", "x: [a,#c]\n", "1:7 library"},
		{"one after an alias", "x: [*a #1]\n", "1:5 library"},
		{"one in a flow closed", "x: {a #c\n , b: 1}\n", "1:5 yaml_flow_key_alone"},
		{"one before an end left out", "x: [a, b #note\ny: 1\n", "2:1 library"},
		{"one of the author's before an end left out", "x: [a #1\n  , b\ny: 1\n", "3:2 library"},
		{"one with flows in it", "x: [a #see {b}\ny: 1\n", "2:1 library"},
		{"one with quotes in it", "x: [a, b # 'quoted]\ny: 1\n", "2:1 library"},
		{"one with ends of the other kind in it", "x: [a, b # see [doc}\ny: 1\n", "2:1 library"},
		{"one past an end too many", "x: {a: b}}\ny: [c #1\n", "1:10 library"},
		{"one far past an end left out", "x: [a\ny: 1\nz: 2  # next\n", "2:2 library"},
		{"one on a line past an end left out", "x: [a\ny: 1 # next\nz: 2\n", "2:2 library"},
		{"one past a flow closed in one left open", "x: [a\n- [b] #c\n", "2:3 library"},
		{"one on a later line of a flow left open", "x: [\n  a, # c\n  b\ny: 1\n", "4:2 library"},
		{"one before ...", "x: [a #1]\n...\ny: [b\n", "2:1 library"},
		{"one in a flow past ---", "--- [a\n--- [b #c]\n", "2:1 library"},
		{"one in a second document, before it is refused", "x: 1\n---\ny: [a #c]\n", "4:1 library"},
		{"one with a mistake of the walk", "title: Plan #1\nx: [a #2]\n", "3:1 library"},
		{"one before an end that closes, and one in the flow after it", "x: [a #1\ny: ]\nz: [b #2]\n", "2:1 library"},
		{"a file with \\x", "x: \"\\x41\"\ny: [a #c]\n", "3:1 library"},
		{"a # in quotes", "x: [\"Issue #1\", b]\n", ""},
		{"a # with no space before it", "x: [Issue#1]\n", ""},
		{"a comment after a flow", "x: [a] # out\n", ""},
		{"a comment on a line of its own out of a flow", "# out\nx: [a]\n", ""},
		{"a [ in quotes", "x: \"[a #1\"\n", ""},
		{"a [ in a | text", "x: |\n  [a #1\n", ""},
		{"past --- that leaves a flow", "--- [a\n--- #c\nx: 1\n", "2:1 library"},
		{"past ... that leaves a flow", "x: [a\n...\n# c\n", "2:1 library"},
		{"an end too many", "x: [a]]\n", "1:7 library"},
		{"an end left out, no comment", "x: [a\ny: 1\n", "2:2 library"},
		{"an end left out over CRLF lines and a tab", "# c\r\nx: [a\tb, c\r\ny: 1\r\n", "3:2 library"},
		{"a comment past an end too many", "x: [a]]\ny: [b, c]\nz: 1 # d\n", "1:7 library"},
		{"a comment past an end with none open", "x: ]\ny: [b]\nz: 1 # d\n", "1:4 library"},
		{"a comment past an end of the other kind", "x: {a: [b}, c #d\n", "1:10 library"},
	} {
		_, err := yamlpos.Parse("a.yaml", []byte(tt.src))
		if got := placed(err); got != tt.want {
			t.Errorf("%s: Parse(%q) = %q, want %q", tt.name, tt.src, got, tt.want)
		}
	}
}

// Find follows keys and indexes to a value and its key, none for an item of a sequence, and stops at the last node it
// reaches; OfValue places a mapping or a sequence at that key, and an item at itself (a { or [ in flow).
func TestFind(t *testing.T) {
	t.Parallel()

	body, err := yamlpos.Parse("a.yaml", []byte("a:\n  b: 1\n  c:\n    - x\n    -\nd: [y, z]\ne: [~, y, null]\n"+
		"f: [{j: x}, [1]]\ng: {h: 1}\ni:\n  - k: 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		at   []string
		want yamlpos.Pos
	}{
		{nil, yamlpos.Pos{Line: 1, Column: 1}},
		{[]string{"a", "b"}, yamlpos.Pos{Line: 2, Column: 6}},
		{[]string{"a", "c", "0"}, yamlpos.Pos{Line: 4, Column: 7}},
		{[]string{"a", "c", "1"}, yamlpos.Pos{Line: 5, Column: 6}},
		{[]string{"d", "1"}, yamlpos.Pos{Line: 6, Column: 8}},
		{[]string{"a"}, yamlpos.Pos{Line: 1, Column: 1}},
		{[]string{"a", "c"}, yamlpos.Pos{Line: 3, Column: 3}},
		{[]string{"a", "nope"}, yamlpos.Pos{Line: 1, Column: 1}},
		{[]string{"d", "9"}, yamlpos.Pos{Line: 6, Column: 1}},
		{[]string{"d", "x"}, yamlpos.Pos{Line: 6, Column: 1}},
		{[]string{"f", "0"}, yamlpos.Pos{Line: 8, Column: 5}},
		{[]string{"f", "0", "j"}, yamlpos.Pos{Line: 8, Column: 9}},
		{[]string{"f", "1"}, yamlpos.Pos{Line: 8, Column: 13}},
		{[]string{"g"}, yamlpos.Pos{Line: 9, Column: 1}},
		{[]string{"i", "0"}, yamlpos.Pos{Line: 11, Column: 5}},
		{[]string{"i", "0", "k"}, yamlpos.Pos{Line: 11, Column: 8}},
		// An empty entry of a flow sequence is where it is written.
		{[]string{"e", "0"}, yamlpos.Pos{Line: 7, Column: 5}},
		{[]string{"e", "2"}, yamlpos.Pos{Line: 7, Column: 11}},
	} {
		if got := yamlpos.OfValue(yamlpos.Find(body, tt.at)); got != tt.want {
			t.Errorf("Find(%v) at %v, want %v", tt.at, got, tt.want)
		}
	}
}

// Locate places a path at the last value Find reaches under the root, and one that reaches none, in a file of
// nothing too, at the file alone, with no node.
func TestLocate(t *testing.T) {
	t.Parallel()

	body, err := yamlpos.Parse("a.yaml", []byte("# c\na:\n  b: 1\n  c:\n    - x\n"))
	if err != nil {
		t.Fatal(err)
	}
	root := body
	for _, tt := range []struct {
		root *yamlpos.Node
		at   []string
		want yamlpos.Pos
	}{
		{root, nil, yamlpos.Pos{}},
		{root, []string{"zz"}, yamlpos.Pos{}},
		{root, []string{"zz", "a"}, yamlpos.Pos{}},
		{nil, []string{"a"}, yamlpos.Pos{}},
		{root, []string{"a"}, yamlpos.Pos{Line: 2, Column: 1}},
		{root, []string{"a", "b"}, yamlpos.Pos{Line: 3, Column: 6}},
		{root, []string{"a", "c", "0"}, yamlpos.Pos{Line: 5, Column: 7}},
		{root, []string{"a", "zz"}, yamlpos.Pos{Line: 2, Column: 1}},
	} {
		got, node := yamlpos.Locate(tt.root, tt.at)
		if got != tt.want || (node == nil) != (tt.want == yamlpos.Pos{}) {
			t.Errorf("Locate(%v) = %v, %v; want %v", tt.at, got, node, tt.want)
		}
	}
}

// OfValue places a null written, ~ or null, where it is written, and one left empty, a comment after it too, at
// its key, as it has no text to point at.
func TestOfValue(t *testing.T) {
	t.Parallel()

	for src, want := range map[string]yamlpos.Pos{
		"x: 1\na: ~\n":      {Line: 2, Column: 4},
		"x: 1\na: null\n":   {Line: 2, Column: 4},
		"x: 1\na:\n":        {Line: 2, Column: 1},
		"x: 1\na: # note\n": {Line: 2, Column: 1},
	} {
		body, err := yamlpos.Parse("a.yaml", []byte(src))
		if err != nil {
			t.Fatal(err)
		}
		if got := yamlpos.OfValue(yamlpos.Find(body, []string{"a"})); got != want {
			t.Errorf("OfValue of %q = %v, want %v", src, got, want)
		}
	}
}

// A value is placed at its first character as textpos counts it: after a tab, with spaces after it at a line's
// end, after a comment or quotes over CRLF lines, after \x; go.yaml.in/yaml/v4's own mistakes too.
func TestParsePlaces(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name, src string
		at        []string
		want      yamlpos.Pos
	}{
		{"spaces after it", "- expr:   zzz < 1   \n", []string{"0", "expr"}, yamlpos.Pos{Line: 1, Column: 11}},
		{"a tab before it", "a:\tx\n", []string{"a"}, yamlpos.Pos{Line: 1, Column: 4}},
		{"two tabs before it", "a:\t\tx\n", []string{"a"}, yamlpos.Pos{Line: 1, Column: 5}},
		{"after a tab in a line", "a: [1,\tb]\n", []string{"a", "1"}, yamlpos.Pos{Line: 1, Column: 8}},
		{"after a comment over CRLF", "# c\r\nb: 1\r\n", []string{"b"}, yamlpos.Pos{Line: 2, Column: 4}},
		{"after quotes over CRLF", "a: \"x\r\n  y\"\r\nb: 1\r\n", []string{"b"}, yamlpos.Pos{Line: 3, Column: 4}},
		{"after spaces kept at a block's line end", "a: |\n  x  \n  y\nb:\t1\n", []string{"b"}, yamlpos.Pos{Line: 4, Column: 4}},
		{"after spaces kept at a comment's end", "a: 'x' # c  \nb:\t2\n", []string{"b"}, yamlpos.Pos{Line: 2, Column: 4}},
		{"after an escaped line break", "a: \"x\\\n  y\"\nb: 1\n", []string{"b"}, yamlpos.Pos{Line: 3, Column: 4}},
		{"CR alone", "a: x\rb: 1\r", []string{"b"}, yamlpos.Pos{Line: 2, Column: 4}},
		{"a block over CR alone", "a: |\r  x\rb: 1\r", []string{"b"}, yamlpos.Pos{Line: 3, Column: 4}},
		{"after an empty block", "a: |\nb: 1\n", []string{"b"}, yamlpos.Pos{Line: 2, Column: 4}},
		{"before \\x", "a:\t\"\\x41\"\nb:\t1\n", []string{"a"}, yamlpos.Pos{Line: 1, Column: 4}},
		{"after \\x", "a:\t\"\\x41\"\nb:\t1\n", []string{"b"}, yamlpos.Pos{Line: 2, Column: 4}},
	} {
		body, err := yamlpos.Parse("a.yaml", []byte(tt.src))
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if got := yamlpos.OfValue(yamlpos.Find(body, tt.at)); got != tt.want {
			t.Errorf("%s: %q at %v, want %v", tt.name, tt.src, got, tt.want)
		}
	}
	// An empty block is placed as the --- after it, not read as text after a second document.
	if _, err := yamlpos.Parse("a.yaml", []byte("# c\r\na: |\r\n---\r\n")); err != nil {
		t.Errorf("an empty block before ---: %v", err)
	}
	// go.yaml.in/yaml/v4's own mistakes, where it tells them; tabs YAML 1.2 takes in quotes and between items pass.
	for src, want := range map[string]string{
		"a: \"x\t\n\ty\"\n":       "",
		"a:\n\t- x\n":             "2:1 library",
		"a: \"\\q\"\n":            "1:5 library",
		"# c\r\na: 1\r\na: 2\r\n": "3:1 yaml_key_twice",
		"{a: 1,\tb:\t2}\n":        "",
	} {
		if _, err := yamlpos.Parse("a.yaml", []byte(src)); placed(err) != want {
			t.Errorf("Parse(%q) = %q, want %q", src, placed(err), want)
		}
	}
}

// Style tells how a scalar is written, as the parser read it: a no-break space or an ideographic space before a
// quote is part of a plain scalar.
func TestStyle(t *testing.T) {
	t.Parallel()

	for src, want := range map[string]yamlpos.Style{
		"\"x\"": yamlpos.Double, "'x'": yamlpos.Single, "x": yamlpos.Plain, "\u00a0'": yamlpos.Plain,
		"\u3000'x": yamlpos.Plain, "1": yamlpos.Plain, "|\n  x\n": yamlpos.Literal, "|-\n  x\n": yamlpos.Literal,
		">\n  x\n": yamlpos.Folded,
	} {
		n, err := yamlpos.Parse("a.yaml", []byte("v: "+src+"\n"))
		if err != nil {
			t.Fatalf("Parse(%q): %s", src, told(err))
		}
		if _, v := yamlpos.Find(n, []string{"v"}); v.Style != want {
			t.Errorf("Style of %q = %d, want %d", src, v.Style, want)
		}
	}
}

// Text reads a text scalar as YAML does, a block keeping the line breaks its end keeps; a number, true, false
// and null are not text.
func TestText(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		src, want string
		ok        bool
	}{
		{"plain", "plain", true},
		{`"a\tb"`, "a\tb", true},
		{"'it''s'", "it's", true},
		{"|\n  x\n", "x\n", true},
		{"|+\n  x\n", "x\n\n", true}, // the line break Parse is given after it is kept too
		{"|-\n  x\n", "x", true},
		{">\n  x\n  y\n", "x y\n", true},
		{"404", "", false},
		{"1.10", "", false},
		{"true", "", false},
		{"false", "", false},
		{"~", "", false},
		{"[x]", "", false},
	} {
		body, err := yamlpos.Parse("f.yaml", []byte("k: "+tt.src+"\n"))
		if err != nil {
			t.Fatal(told(err))
		}
		if got, ok := yamlpos.Text(body.Pairs[0].Value); got != tt.want || ok != tt.ok {
			t.Errorf("Text(%q) = %q, %v; want %q, %v", tt.src, got, ok, tt.want, tt.ok)
		}
	}
}

// Written finds a value's characters where they are written as read, and nothing where they are not.
func TestWritten(t *testing.T) {
	t.Parallel()

	none := yamlpos.Pos{}
	for _, tt := range []struct {
		src  string
		want yamlpos.Pos
	}{
		{"v: abc\n", yamlpos.Pos{Line: 1, Column: 4}},
		{"v: 1.10\n", yamlpos.Pos{Line: 1, Column: 4}},
		{"\ufeffv: abc\n", yamlpos.Pos{Line: 1, Column: 4}},
		{"v: 'abc'\n", yamlpos.Pos{Line: 1, Column: 5}},
		{"v: \"abc\"\n", yamlpos.Pos{Line: 1, Column: 5}},
		{"v: \"\\x41\"\n", none},
		{"v: 'a''b'\n", none},
		{"v: 'a'''\n", none},
		{"v: \"a\\\\\"\n", none},
		{"v: a\n  b\n", none},
		{"v: 'a\n  b'\n", none},
		{"v: |\n  x\n  y\n", yamlpos.Pos{Line: 2, Column: 3}},
		{"v: |-\n    x\n", yamlpos.Pos{Line: 2, Column: 5}},
		{"v: |\n\n  x\n", yamlpos.Pos{Line: 2, Column: 3}},
		{"v: |\r\n  x\r\n", yamlpos.Pos{Line: 2, Column: 3}},
		{"v: |2\n   x\n", none},
		{"v: >\n  x\n", none},
		{"v: [a]\n", none},
	} {
		body, err := yamlpos.Parse("a.yaml", []byte(tt.src))
		if err != nil {
			t.Fatalf("Parse(%q): %s", tt.src, told(err))
		}
		_, v := yamlpos.Find(body, []string{"v"})
		got, ok := yamlpos.Written([]byte(tt.src), v)
		if got != tt.want || ok != (tt.want != none) {
			t.Errorf("Written(%q) = %v, %v; want %v", tt.src, got, ok, tt.want)
		}
	}
}

// An item of a sequence is placed at itself, not at its -: a mapping at its first key, a sequence at its first -,
// one in flow at its { or [, and one left out where the parser puts it, after the -.
func TestItemPlace(t *testing.T) {
	t.Parallel()

	for src, want := range map[string]yamlpos.Pos{
		"l:\n  - x: 1\n":          {Line: 2, Column: 5},
		"l:\n  -   text: a\n":     {Line: 2, Column: 7},
		"l:\n  -\n    text: a\n":  {Line: 3, Column: 5},
		"l:\n  - # c\n    k: v\n": {Line: 3, Column: 5},
		"l:\n  - - x\n":           {Line: 2, Column: 5},
		"l:\n  - [x]\n":           {Line: 2, Column: 5},
		"l:\n  -\n  - x\n":        {Line: 2, Column: 4},
	} {
		body, err := yamlpos.Parse("a.yaml", []byte(src))
		if err != nil {
			t.Fatalf("Parse(%q): %s", src, told(err))
		}
		if got, _ := yamlpos.Locate(body, []string{"l", "0"}); got != want {
			t.Errorf("%q: the item at %v, want %v", src, got, want)
		}
	}
}

// Parse gives each value as a Node: its kind, its value as written, how it is written and whether it is null;
// empty and nested flow ones, and a key in quotes, by its name.
func TestNode(t *testing.T) {
	t.Parallel()

	body, err := yamlpos.Parse("a.yaml", []byte("a: ~\nb:\nc: null\nd: 1.10\ne: 'q'\nf: {}\ng: []\n"+
		"h: {i: [1, {j: x}]}\n'1': x\nk: |\n  t\n"))
	if err != nil {
		t.Fatal(told(err))
	}
	names := make([]string, 0, len(body.Pairs))
	for _, p := range body.Pairs {
		names = append(names, p.Name())
	}
	if got := strings.Join(names, " "); got != "a b c d e f g h 1 k" {
		t.Errorf("the keys are %q", got)
	}
	value := func(at ...string) *yamlpos.Node {
		_, v := yamlpos.Find(body, at)
		return v
	}
	for at, want := range map[string]struct {
		value string
		null  bool
	}{"a": {"~", true}, "b": {"", true}, "c": {"null", true}, "d": {"1.10", false}} {
		if v := value(at); v.Kind != yamlpos.Scalar || v.Value != want.value || v.Null != want.null {
			t.Errorf("%s: %d %q null %v, want %q null %v", at, v.Kind, v.Value, v.Null, want.value, want.null)
		}
		if _, ok := yamlpos.Text(value(at)); ok {
			t.Errorf("%s is text", at)
		}
	}
	if v := value("e"); v.Style != yamlpos.Single || v.Value != "q" {
		t.Errorf("e: %d %q", v.Style, v.Value)
	}
	if f, g := value("f"), value("g"); f.Kind != yamlpos.Mapping || len(f.Pairs) != 0 || g.Kind != yamlpos.Sequence || len(g.Items) != 0 {
		t.Errorf("f: %d %d, g: %d %d", f.Kind, len(f.Pairs), g.Kind, len(g.Items))
	}
	if i := value("h", "i"); i.Kind != yamlpos.Sequence || len(i.Items) != 2 || i.Items[1].Kind != yamlpos.Mapping ||
		i.Items[1].Pairs[0].Name() != "j" {
		t.Errorf("h.i: %+v", i)
	}
	if text, ok := yamlpos.Text(value("k")); !ok || text != "t\n" || value("k").Style != yamlpos.Literal {
		t.Errorf("k: %q %v", text, ok)
	}
}

// Decode reads a plain scalar by YAML 1.2's core schema, and a quoted one or a | text as text; a whole number is
// an int, or uint64 past int64.
func TestDecode(t *testing.T) {
	t.Parallel()

	for src, want := range map[string]any{
		"~": nil, "null": nil, "Null": nil, "NULL": nil, "": nil, "'~'": "~", "nul": "nul",
		"true": true, "True": true, "TRUE": true, "false": false, "FALSE": false, "yes": "yes", "tRue": "tRue",
		"0": 0, "-3": -3, "+3": 3, "0o17": 15, "0x1F": 31, "9223372036854775807": 9223372036854775807,
		"9223372036854775808": uint64(9223372036854775808), "1.5": 1.5, "-.5": -0.5, "1.": 1.0,
		"2030-04-12": "2030-04-12", "'1'": "1", "\"true\"": "true", "|\n  1\n": "1\n", "a b": "a b",
		"[1, x]": []any{1, "x"}, "[]": []any{}, "{}": map[string]any{}, "{a: ~}": map[string]any{"a": nil},
	} {
		body, err := yamlpos.Parse("a.yaml", []byte("v: "+src+"\n"))
		if err != nil {
			t.Fatalf("Parse(%q): %s", src, told(err))
		}
		got, _ := yamlpos.Decode(body).(map[string]any) //nolint:errcheck // a nil map holds no v, told below
		if !reflect.DeepEqual(got["v"], want) {
			t.Errorf("Decode(%q) = %#v, want %#v", src, got["v"], want)
		}
	}
}

// A control character is go.yaml.in/yaml/v4's mistake, at its byte, a byte order mark before it or not; UTF-16 with
// a mark, or of letters beyond ASCII, is no UTF-8 (yaml_not_utf8), and of ASCII alone without a mark it is UTF-8
// with NUL bytes, refused at the first.
func TestParseControlAndUTF16(t *testing.T) {
	t.Parallel()

	for src, want := range map[string]string{
		"heading: x\x01y\n":                  "1:11 library",
		bom + "heading: x\x01y\n":            "1:11 library",
		"a: 1\nheading: x\x00y\n":            "2:11 library",
		"heading: x\x1by\n":                  "1:11 library",
		"heading: x\xef\xbf\xbey\n":          "1:11 library",
		"heading: x\xef\xbf\xbfy\n":          "1:11 library",
		"\xff\xfea\x00:\x00 \x001\x00\n\x00": "1:1 yaml_not_utf8",
		"\xfe\xff\x00a\x00:\x00 \x001\x00\n": "1:1 yaml_not_utf8",
		"a\x00:\x00 \x001\x00\n\x00":         "1:2 library",
		"\x00a\x00:\x00 \x001\x00\n":         "1:1 library",
		"a\x00:\x00 \x00\xe5\x65\n\x00":      "1:7 yaml_not_utf8",
	} {
		_, err := yamlpos.Parse("a.yaml", []byte(src))
		if got := placed(err); got != want {
			t.Errorf("Parse(%q) = %q, want %q", src, got, want)
		}
	}
}

// A nesting past go.yaml.in/yaml/v4's depth is its mistake, at its line and column, read in memory as the input
// grows.
func TestParseDeep(t *testing.T) {
	t.Parallel()

	src := "x: " + strings.Repeat("[", 12000) + strings.Repeat("]", 12000) + "\n"
	if got := placed(func() error { _, err := yamlpos.Parse("a.yaml", []byte(src)); return err }()); got != "1:10004 library" {
		t.Errorf("12000 [ = %q", got)
	}
}

// An item with nothing written is placed where go.yaml.in/yaml/v4 puts it, after its -, at the end of a file too,
// and after quotes over CRLF lines.
func TestParseNullItems(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		src  string
		at   []string
		want yamlpos.Pos
	}{
		{"a:\n  -\n", []string{"a", "0"}, yamlpos.Pos{Line: 2, Column: 4}},
		{"a:\n  -", []string{"a", "0"}, yamlpos.Pos{Line: 2, Column: 4}},
		{"a:\n-\n", []string{"a", "0"}, yamlpos.Pos{Line: 2, Column: 2}},
		{"a:\n  - -\n", []string{"a", "0", "0"}, yamlpos.Pos{Line: 2, Column: 6}},
		{
			"template: report\r\nlang: en\r\ntitle: 'x\r\n\r\n  y'\r\nsections:\r\n  - heading: h\r\n    parts:\r\n      -\r\n      - text: a\r\n",
			[]string{"sections", "0", "parts", "0"},
			yamlpos.Pos{Line: 9, Column: 8},
		},
	} {
		body, err := yamlpos.Parse("a.yaml", []byte(tt.src))
		if err != nil {
			t.Fatalf("Parse(%q): %s", tt.src, told(err))
		}
		if got, _ := yamlpos.Locate(body, tt.at); got != tt.want {
			t.Errorf("%q: %v at %v, want %v", tt.src, tt.at, got, tt.want)
		}
	}
}

// An escape of U+2028 in double quotes passes, and the value holds the character.
func TestParseEscapedLineSeparator(t *testing.T) {
	t.Parallel()

	body, err := yamlpos.Parse("a.yaml", []byte("x: \"a\\"+"u2028b\"\n"))
	if err != nil {
		t.Fatal(told(err))
	}
	if got, _ := yamlpos.Text(body.Pairs[0].Value); got != "a"+string(rune(0x2028))+"b" {
		t.Errorf("read %q", got)
	}
}

// A comment that may have cut a value short is refused at the value's start, once a value: on the line of a plain
// value read as text, and right after a | or > text (the forms of its right place too, which v4's tree cannot tell
// apart). A key's comment, one in a | text, after a number or a quoted value, and on a line of its own after a
// plain value pass.
func TestParseComments(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ src, want string }{
		// On the line of a plain text value, in a flow too, at the end of a value over lines.
		{"title: Plan #1 draft\n", "1:8 yaml_comment_cut"},
		{"lead: PR #100\n", "1:7 yaml_comment_cut"},
		{"a: b\t#c\n", "1:4 yaml_comment_cut"},
		{"title: Plan\n  draft #1\n", "1:8 yaml_comment_cut"},
		{"x: [a, # c\n  b]\n", "1:5 yaml_comment_cut"},
		{"sections:\n  - heading: Step #2\n", "2:14 yaml_comment_cut"},
		{"- uses: actions/checkout@abc # v4\n", "1:9 yaml_comment_cut"},
		// Right after a | or > text: the next key's head, a key's foot, an item, the document's foot.
		{"t: |\n    a\n  # h\nb: 1\n", "1:4 yaml_comment_after_block"},
		{"t: |\n  a\n # h\nb: 1\n", "1:4 yaml_comment_after_block"},
		{"t: >\n    a\n  # h\n", "1:4 yaml_comment_after_block"},
		{"t: |-\n    a\n  # h\nb: 1\n", "1:4 yaml_comment_after_block"},
		{"t: |+\n    a\n\n  # h\nb: 1\n", "1:4 yaml_comment_after_block"},
		{"- |\n    a\n  # h\n", "1:3 yaml_comment_after_block"},
		{"m:\n  t: |\n      a\n    # h\n    # i\n  b: 1\n", "2:6 yaml_comment_after_block"},
		{"a: |\n\n# c\nb: 1\n", "1:4 yaml_comment_after_block"},
		// LIMIT: the right place of a comment after a text, the next item's -, and after --- (the user decided).
		{"a: |\n  text\n# c\nb: 1\n", "1:4 yaml_comment_after_block"},
		{"- t: |\n    x\n- # c\n  t: y\n", "1:6 yaml_comment_after_block"},
		{"a: |\n  t\n---\n# c\n", "1:4 yaml_comment_after_block"},
		{"a: |\n  t\n...\n# c\n", "1:4 yaml_comment_after_block"},
		{"k:\n- |\n  x\n- []\n# c\n- y\n", "2:3 yaml_comment_after_block"},
		// An anchor, alias, tag or merge key is refused as such, not for its comment.
		{"a: !!str b # c\nb: &x c # c\n", "1:4 yaml_no_anchors\n2:4 yaml_no_anchors"},
		{"x:\n  <<: b # c\n", "2:3 yaml_no_anchors"},
		// Once a value, at two places too.
		{"a:\n  t: |\n    x\n  # c1\n# c2\nb: 1\n", "2:6 yaml_comment_after_block"},
		{"a: Plan #1\nb: |\n  x\n# c\n", "1:4 yaml_comment_cut\n2:4 yaml_comment_after_block"},
		// They pass.
		{"a: # c\n  k: v\n", ""},
		{"parts: # note\n  - text: x\n", ""},
		{"? a # c\n: x\n", ""},
		{"t: | # note\n  body\n", ""},
		{"t: |\n  para\n  # Heading in the text\n  more\nb: 1\n", ""},
		{"a: b\n# section\nc: d\n", ""},
		{"a: b\n\n# section\nc: d\n", ""},
		{"l:\n  - a\n  # between\n  - b\n", ""},
		{"a: b\n# end\n", ""},
		{"a: 1 # c\nb: 'q' # c\nc: true # c\nd: ~ # c\ne: \"x\" # c\n", ""},
		{"# top\na: |\n  t\nb: 1\n", ""},
		{"lang: en\n---\n# c\n", ""},
		{"a: |\n  t\nb: x\n# after a plain value\nc: 1\n", ""},
		{"x:\n  t: |\n    a\n  u: b\n  # c\ny: 1\n", ""},
		{"ports: [\n  8080,  # admin\n  9090,\n]\n", ""},
		{"- text: |\n    one\n- text: | # note on two\n    two\n", ""},
		{"title: 'Plan #1 draft'\n", ""},
		// LIMIT: on a line of its own after a plain value, deeper than its key, and right after a text where v4 gives it to
		// the next item, as the LIMITs of comments tell.
		{"a: b\n   # c\nb: 1\n", ""},
		{"p:\n  - text: |\n      one\n  # Next\n\n  - text: more\n", ""},
		{"x:\n- |\n  x\n # c\n-\n", ""},
		{"a: Plan\n  draft\n  #1 of series\nb: 1\n", ""},
	} {
		_, err := yamlpos.Parse("a.yaml", []byte(tt.src))
		if got := placed(err); got != tt.want {
			t.Errorf("Parse(%q) told\n%s\nwant\n%s", tt.src, got, tt.want)
		}
	}
}

// The end of an empty | or > text is shown as its mark.
func TestParseCommentEmptyBlock(t *testing.T) {
	t.Parallel()

	_, err := yamlpos.Parse("a.yaml", []byte("a: |\n\n# c\nb: >\n\n# d\nc: 1\n"))
	var found usererr.List
	if failed := found.Add(err); failed != nil {
		t.Fatal(failed)
	}
	if len(found) != 2 || fmt.Sprint(found[0].Args) != "[|]" || fmt.Sprint(found[1].Args) != "[>]" {
		t.Errorf("told %s", told(err))
	}
}
