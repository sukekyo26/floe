package yamlpos_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

const (
	anchors = "anchors, aliases, merge keys (<<) and tags are not supported; write the value out"
	cutAt   = "a # after a space began a YAML comment, which may have cut the value short, to %s: put the value in quotes if the # is part of it, or write the comment on a line of its own"
)

func told(err error) string {
	if err == nil {
		return ""
	}
	return usererr.Tell(i18n.EnglishCatalog(), err).Render("floe: ")
}

// Parse refuses what floe does not read as written, each where it is written, all of a file at once in the
// order of its lines, walking neither what an anchor, alias or tag holds nor a second document; it passes
// what YAML reads as written.
func TestParse(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ src, want string }{
		// goccy/go-yaml's own words, at its place.
		{"title: a: b\n", "a.yaml:1:8: mapping value is not allowed in this context"},
		{"title: `x`\n", "a.yaml:1:8: '`' is a reserved character"},
		{"lang: en\nlang: ja\n", `a.yaml:2:1: mapping key "lang" already defined at [1:1]`},
		// One document.
		{"lang: en\n---\ntitle: x\n", "a.yaml:2:1: a YAML file holds one document: write it as one, without --- between parts"},
		{"lang: en\n---\nx: &a 1\n", "a.yaml:2:1: a YAML file holds one document: write it as one, without --- between parts"},
		{"%YAML 1.2\n---\nlang: en\n---\nx: 1\n", "a.yaml:4:1: a YAML file holds one document: write it as one, without --- between parts"},
		{"a: 1\n...\n%YAML 1.2\n---\nb: 2\n", "a.yaml:3:1: a YAML file holds one document: write it as one, without --- between parts"},
		{"a: 1\n---\n%YAML 1.2\n---\nb: 2\n", "a.yaml:2:1: a YAML file holds one document: write it as one, without --- between parts"},
		// goccy/go-yaml reads a second --- as an empty document and drops what follows it.
		{"a: 1\n---\n---\nb: 2\n", "a.yaml:3:1: a YAML file holds one document: write it as one, without --- between parts"},
		{"---\n---\nid: d\n", "a.yaml:2:1: a YAML file holds one document: write it as one, without --- between parts"},
		// Anchors, aliases, merge keys and tags, not what they hold.
		{"x: &a 1\ny: *a\n", "a.yaml:1:4: " + anchors + "\na.yaml:2:4: " + anchors},
		{"title: !!str 3\n", "a.yaml:1:8: " + anchors},
		{"&k x: 1\n", "a.yaml:1:1: " + anchors},
		{"k: &a key\n*a : 1\n", "a.yaml:1:4: " + anchors + "\na.yaml:2:1: " + anchors},
		{"!!str 0x10: 1\n", "a.yaml:1:1: " + anchors},
		{"m: &m {a: 1}\nx:\n  <<: *m\n", "a.yaml:1:4: " + anchors + "\na.yaml:3:3: " + anchors},
		{"--- !!map\nlang: en\n", "a.yaml:1:5: " + anchors},
		{"&top\nlang: en\n", "a.yaml:1:1: " + anchors},
		{"x: &a Plan #1 draft\n", "a.yaml:1:4: " + anchors},
		{"x: {a: 1, &b c}\n", "a.yaml:1:11: " + anchors},
		// Keys, as keys.
		{"0x10: a\n~: 1\ntrue: 1\n", "a.yaml:1:1: a key is a name: write 0x10 as text, in quotes\na.yaml:2:1: a key is a name: write ~ as text, in quotes\na.yaml:3:1: a key is a name: write true as text, in quotes"},
		{".inf: 1\n", "a.yaml:1:1: a key is a name: write .inf as text, in quotes"},
		{"x: {a: 1, 5}\n", "a.yaml:1:11: a key is a name: write 5 as text, in quotes"},
		{"? Plan #1\n: x\n", "a.yaml:1:8: " + fmt.Sprintf(cutAt, "Plan")},
		// Values.
		{"x: .inf\n", "a.yaml:1:4: .inf is not a value a YAML file can hold; write a number in digits, or quote it as text"},
		{"x: [.NaN, -.Inf]\n", "a.yaml:1:5: .NaN is not a value a YAML file can hold; write a number in digits, or quote it as text\na.yaml:1:11: -.Inf is not a value a YAML file can hold; write a number in digits, or quote it as text"},
		{"title: Plan #1 draft\n", "a.yaml:1:13: " + fmt.Sprintf(cutAt, "Plan")},
		{"sections:\n  - heading: Step #2\n", "a.yaml:2:19: " + fmt.Sprintf(cutAt, "Step")},
		{"x: {a, b: 1}\n", `a.yaml:1:5: "a" is a key without a value in { }: write its value after a ":", or, if a , in text ended the text before it, put that text in quotes`},
		{"x: {a #c\n , b: 1}\n", "a.yaml:1:7: " + fmt.Sprintf(cutAt, "a")},
		// A long one shows its end, where the cut is, or its head.
		{"lead: " + strings.Repeat("a", 50) + "bcd # c\n", "a.yaml:1:61: " + fmt.Sprintf(cutAt, "…"+strings.Repeat("a", 37)+"bcd")},
		{"a: { " + strings.Repeat("b", 50) + " }\n", "a.yaml:1:6: \"" + strings.Repeat("b", 40) + "…\" is a key without a value in { }: write its value after a \":\", or, if a , in text ended the text before it, put that text in quotes"},
		// All of a file at once, in the order of its lines.
		{"title: Plan #1 draft\nx: &a 1\n0x10: a\n", "a.yaml:1:13: " + fmt.Sprintf(cutAt, "Plan") + "\na.yaml:2:4: " + anchors + "\na.yaml:3:1: a key is a name: write 0x10 as text, in quotes"},
		// What YAML reads as written passes.
		{"title: 'Plan #1 draft'\n", ""},
		{"title: Plan#1\n", ""},
		{"lead: |\n  Plan #1 draft\n", ""},
		{"# a note\nsections: []  # trailing\n", ""},
		{"x: {a: , b: 1}\n", ""},
		{"\"0x10\": a\nx: \".inf\"\n", ""},
		{"---\nlang: en\n...\n", ""},
		{"lang: en\n---\n", ""},
		{"a: 1\n---\n# c\n", ""},
		{"a: 1\r\n---\r\n\r\n", ""},
		{"a: 1\n---\n... # end\n", ""},
		{"%YAML 1.2\n---\nlang: en\n", ""},
		// LIMIT: text a , or a comment line cut, as the LIMITs of cut tell.
		{"x: [1,500 words]\n", ""},
		// A comment after a number, true or ~ cuts no text; text cut to a number is the schema's, by its type.
		{"enabled: true  # turn on\nx: 3 #c\ny: ~ #c\n", ""},
		{"title: 2024 #1\n", ""},
		{"title: Plan\n  #1 draft\n", ""},
	} {
		_, err := yamlpos.Parse("a.yaml", []byte(tt.src))
		if got := told(err); got != tt.want {
			t.Errorf("Parse(%q) told\n%s\nwant\n%s", tt.src, got, tt.want)
		}
	}
	for _, src := range []string{"", "# just a note\n"} {
		if _, err := yamlpos.Parse("a.yaml", []byte(src)); !errors.Is(err, yamlpos.ErrEmpty) {
			t.Errorf("Parse(%q) = %v, want ErrEmpty", src, err)
		}
	}
}

// Library places goccy/go-yaml's words at its token, without a place when it has none, and returns any other
// error as it is.
func TestLibrary(t *testing.T) {
	t.Parallel()

	var m map[string]any
	err := yaml.Unmarshal([]byte("- a\n"), &m)
	if got := told(yamlpos.Library("a.yaml", err)); got != "a.yaml:1:1: sequence was used where mapping is expected" {
		t.Errorf("Library of a mistake at a token = %q", got)
	}
	if got := told(yamlpos.Library("a.yaml", &yaml.SyntaxError{Message: "bad", Token: nil})); got != "a.yaml: bad" {
		t.Errorf("Library of a mistake without a token = %q", got)
	}
	failure := errors.New("disk on fire")
	if got := yamlpos.Library("a.yaml", failure); !errors.Is(got, failure) {
		t.Errorf("Library of a failure = %v", got)
	}
}

// Find follows keys and indexes to a value and its key, the - of an entry of a block sequence, and stops at the
// last node it reaches.
func TestFind(t *testing.T) {
	t.Parallel()

	body, err := yamlpos.Parse("a.yaml", []byte("a:\n  b: 1\n  c:\n    - x\n    -\nd: [y, z]\n"))
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
		{[]string{"a", "c", "1"}, yamlpos.Pos{Line: 5, Column: 5}},
		{[]string{"d", "1"}, yamlpos.Pos{Line: 6, Column: 8}},
		{[]string{"a", "nope"}, yamlpos.Pos{Line: 2, Column: 3}},
		{[]string{"d", "9"}, yamlpos.Pos{Line: 6, Column: 4}},
		{[]string{"d", "x"}, yamlpos.Pos{Line: 6, Column: 4}},
	} {
		if got := yamlpos.OfValue(yamlpos.Find(body, tt.at)); got != tt.want {
			t.Errorf("Find(%v) at %v, want %v", tt.at, got, tt.want)
		}
	}
}
