package yamlpos_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/goccy/go-yaml/ast"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

const (
	anchors = "anchors, aliases, merge keys (<<) and tags are not supported; write the value out"
	cutAt   = "a # after a space began a YAML comment, which may have cut the value short, to %s: put the value in quotes if the # is part of it, or write the comment on a line of its own"
	bom     = "\xef\xbb\xbf"
)

func notNumber(written, number string) string {
	return fmt.Sprintf("%[1]s reads as the number %[2]s here, but as text in YAML 1.2: write %[2]s for the number, or put %[1]s in quotes for text", written, number)
}

func notText(written, shown, number string) string {
	return fmt.Sprintf("%s reads as text here, but as the number %s in YAML 1.2: write %s for the number, or put %s in quotes for text", written, shown, number, written)
}

func octal(written, read, want, octal string) string {
	return fmt.Sprintf("%s reads as the octal number %s here, but as %s in YAML 1.2: write %s for the decimal number, or %s for the octal one", written, read, want, want, octal)
}

func outsideFloat(written string) string {
	return written + " is a number in YAML 1.2 that a float64 cannot hold: put it in quotes to keep it as text"
}

func tooBig(written string) string {
	return written + " is a number in YAML 1.2, too large for floe to read: put it in quotes to keep it as text"
}

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
		// A plain value over lines is cut where its last line holds the comment.
		{"title: Plan\n  draft #1\n", "a.yaml:2:9: " + fmt.Sprintf(cutAt, "Plan draft")},
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
		// A byte order mark at the start is left out: the first line's columns are an editor's.
		{bom + "title: Plan #1 draft\n", "a.yaml:1:13: " + fmt.Sprintf(cutAt, "Plan")},
		{bom + "%YAML 1.2\n---\nlang: en\n", ""},
		{bom + "---\n---\n", "a.yaml:2:1: a YAML file holds one document: write it as one, without --- between parts"},
		// LIMIT: text a , or a comment line cut, as the LIMITs of cut tell.
		{"x: [1,500 words]\n", ""},
		// A plain value YAML 1.2 reads otherwise than goccy/go-yaml, told with a way both read alike.
		{"x: 017\n", "a.yaml:1:4: " + octal("017", "15", "17", "0o17")},
		{"x: -017\n", "a.yaml:1:4: " + octal("-017", "-15", "-17", "-15")},
		{"x: 0b101\n", "a.yaml:1:4: " + notNumber("0b101", "5")},
		{"x: 1_000\n", "a.yaml:1:4: " + notNumber("1_000", "1000")},
		{"x: -0x1F\n", "a.yaml:1:4: " + notNumber("-0x1F", "-31")},
		{"x: +-1\n", "a.yaml:1:4: " + notNumber("+-1", "1")},
		{"x: 1_0.0\n", "a.yaml:1:4: " + notNumber("1_0.0", "10.0")},
		{"x: 1_000_000_000_000_000_000_000.0\n", "a.yaml:1:4: " + notNumber("1_000_000_000_000_000_000_000.0", "1.0e+21")},
		{"x: 1e3\n", "a.yaml:1:4: " + notText("1e3", "1000", "1000.0")},
		{"x: 012e3\n", "a.yaml:1:4: " + notText("012e3", "12000", "12000.0")},
		{"x: 08\n", "a.yaml:1:4: " + notText("08", "8", "8")},
		{"x: 99999999999999999999\n", "a.yaml:1:4: " + tooBig("99999999999999999999")},
		{"x: -9223372036854775809\n", "a.yaml:1:4: " + tooBig("-9223372036854775809")},
		{"x: 030000000000000000000\n", "a.yaml:1:4: " + tooBig("030000000000000000000")},
		{"x: +.inf\n", "a.yaml:1:4: +.inf is not a value a YAML file can hold; write a number in digits, or quote it as text"},
		{"x: 1e400\n", "a.yaml:1:4: " + outsideFloat("1e400")},
		{"a: 1e-400\nb: 1.0e-400\nc: -1e-400\n", "a.yaml:1:4: " + outsideFloat("1e-400") + "\na.yaml:2:4: " + outsideFloat("1.0e-400") + "\na.yaml:3:4: " + outsideFloat("-1e-400")},
		// A true 0 is no float64 left behind: goccy/go-yaml reads 0e-400 as text, as it does 1e3.
		{"a: 0e-400\n", "a.yaml:1:4: " + notText("0e-400", "0", "0.0")},
		{"a: 0.0e-400\nb: -0.0e-400\n", ""},
		{"x: [017, 0x1F, 1e3]\n", "a.yaml:1:5: " + octal("017", "15", "17", "0o17") + "\na.yaml:1:16: " + notText("1e3", "1000", "1000.0")},
		{"x: {a: 0b101, b: 12}\n", "a.yaml:1:8: " + notNumber("0b101", "5")},
		{"1e3: a\n08: b\n", "a.yaml:1:1: a key is a name: write 1e3 as text, in quotes\na.yaml:2:1: a key is a name: write 08 as text, in quotes"},
		// A number refused is judged for a comment that cut it short too: quoting it as told would drop the rest.
		{"title: 1e3 #1 draft\n", "a.yaml:1:8: " + notText("1e3", "1000", "1000.0") + "\na.yaml:1:12: " + fmt.Sprintf(cutAt, "1e3")},
		{"x: 017 #1 draft\n", "a.yaml:1:4: " + octal("017", "15", "17", "0o17") + "\na.yaml:1:8: " + fmt.Sprintf(cutAt, "017")},
		// Read alike, or not plain, or not a number at its start, it passes.
		{"a: 0o17\nb: 0x1F\nc: +1\nd: 0\ne: 00\nf: .5\ng: 1.0\nh: 1.\ni: 012.5\nj: 18446744073709551615\n", ""},
		{"a: 1000.0\nb: 12000.0\nc: 1.0e+20\nd: -0.0\ne: 1.5e3\n", ""},
		{"a: \"017\"\nb: '1e3'\nc: Plan 2026\nd: v1.0\ne: 2026-10-26\nf: 1.2.3\ng: 1,000\nh: +.nan\n", ""},
		{"\"1e3\": a\n", ""},
		{"x: 0x1F #1 draft\n", ""},
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
	for _, src := range []string{"", "# just a note\n", bom, bom + "# just a note\n"} {
		if _, err := yamlpos.Parse("a.yaml", []byte(src)); !errors.Is(err, yamlpos.ErrEmpty) {
			t.Errorf("Parse(%q) = %v, want ErrEmpty", src, err)
		}
	}
}

// Find follows keys and indexes to a value and its key, the - of an entry of a block sequence, and stops at the
// last node it reaches.
func TestFind(t *testing.T) {
	t.Parallel()

	body, err := yamlpos.Parse("a.yaml", []byte("a:\n  b: 1\n  c:\n    - x\n    -\nd: [y, z]\ne: [~, y, null]\n"))
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
		// An empty entry of a flow sequence is where it is written: goccy/go-yaml gives a flow sequence entries
		// too, whose token is the , before each, nil for the first, not the entry.
		{[]string{"e", "0"}, yamlpos.Pos{Line: 7, Column: 5}},
		{[]string{"e", "2"}, yamlpos.Pos{Line: 7, Column: 11}},
	} {
		if got := yamlpos.OfValue(yamlpos.Find(body, tt.at)); got != tt.want {
			t.Errorf("Find(%v) at %v, want %v", tt.at, got, tt.want)
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

// QuoteOf tells a quoted scalar as the parser read it: a no-break space or an ideographic space before a quote
// is part of a plain scalar.
func TestQuoteOf(t *testing.T) {
	t.Parallel()

	for src, want := range map[string]yamlpos.Quote{
		"\"x\"": yamlpos.Double, "'x'": yamlpos.Single, "x": yamlpos.Plain, "\u00a0'": yamlpos.Plain,
		"\u3000'x": yamlpos.Plain, "1": yamlpos.Plain, "|\n  x\n": yamlpos.Plain,
	} {
		n, err := yamlpos.Parse("a.yaml", []byte("v: "+src+"\n"))
		if err != nil {
			t.Fatalf("Parse(%q): %s", src, told(err))
		}
		if _, v := yamlpos.Find(n, []string{"v"}); yamlpos.QuoteOf(v) != want {
			t.Errorf("QuoteOf(%q) = %d, want %d", src, yamlpos.QuoteOf(v), want)
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
		m, isMap := body.(*ast.MappingNode)
		if !isMap {
			t.Fatalf("%q: the body is %T", tt.src, body)
		}
		got, ok, err := yamlpos.Text(m.Values[0].Value)
		if err != nil || got != tt.want || ok != tt.ok {
			t.Errorf("Text(%q) = %q, %v, %v; want %q, %v", tt.src, got, ok, err, tt.want, tt.ok)
		}
	}
}
