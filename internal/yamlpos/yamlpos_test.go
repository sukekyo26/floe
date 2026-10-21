package yamlpos_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// A mapping is at its first key, not at the indentation before it, and an anchor is found anywhere.
func TestOfAndFirstAnchor(t *testing.T) {
	t.Parallel()

	doc, err := parser.ParseBytes([]byte("list:\n  - a: 1\n    b: &x 2\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	body := doc.Docs[0].Body
	if got := yamlpos.Of(body); got != (yamlpos.Pos{Line: 1, Column: 1}) {
		t.Errorf("Of(the top mapping) = %+v", got)
	}
	a := yamlpos.FirstAnchor(body)
	if a == nil || yamlpos.Of(a) != (yamlpos.Pos{Line: 3, Column: 8}) {
		t.Errorf("FirstAnchor = %v", a)
	}
	merge, err := parser.ParseBytes([]byte("a: 1\n<<: {b: 2}\n"), 0)
	if err != nil || yamlpos.FirstAnchor(merge.Docs[0].Body) == nil {
		t.Error("FirstAnchor missed a merge key")
	}
	clean, err := parser.ParseBytes([]byte("a: 1\n"), 0)
	if err != nil || yamlpos.FirstAnchor(clean.Docs[0].Body) != nil {
		t.Error("FirstAnchor found one in plain YAML")
	}
}

// Find follows keys and indexes, writes the path as it goes, and stops where the path leaves the file.
func TestFind(t *testing.T) {
	t.Parallel()

	doc, err := parser.ParseBytes([]byte("list:\n  - a: 1\n    \"b c\": 2\nflow: [x, ~]\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	root := doc.Docs[0].Body
	for _, tt := range []struct {
		at   []string
		path string
		key  bool
		pos  yamlpos.Pos
	}{
		{nil, "", false, yamlpos.Pos{Line: 1, Column: 1}},
		{[]string{"list", "0", "b c"}, "list[0].b c", true, yamlpos.Pos{Line: 3, Column: 12}},
		{[]string{"list", "0"}, "list[0]", true, yamlpos.Pos{Line: 2, Column: 5}},
		{[]string{"list", "5", "a"}, "list", true, yamlpos.Pos{Line: 2, Column: 3}},
		{[]string{"nope"}, "", false, yamlpos.Pos{Line: 1, Column: 1}},
		// The first entry of a flow sequence has no token, the others a comma: neither is a key.
		{[]string{"flow", "0"}, "flow[0]", false, yamlpos.Pos{Line: 4, Column: 8}},
		{[]string{"flow", "1"}, "flow[1]", false, yamlpos.Pos{Line: 4, Column: 11}},
	} {
		key, value, path := yamlpos.Find(root, tt.at)
		if path != tt.path || (key != nil) != tt.key || yamlpos.OfValue(key, value) != tt.pos {
			t.Errorf("Find(%q) = %v, %v at %+v, %q", tt.at, key, value, yamlpos.OfValue(key, value), path)
		}
	}
}

// Parse takes one document, a directive and an empty document after --- included, and points at a
// syntax error or a second document.
func TestParse(t *testing.T) {
	t.Parallel()

	for _, src := range []string{
		"a: 1\n", "%YAML 1.2\n---\na: 1\n", "---\na: 1\n", "a: 1\n---\n", "a: 1\n---\n# c\n", "a: 1\r\n---\r\n\r\n",
		"a: 1\n---\n... # end\n",
	} {
		if body, err := yamlpos.Parse("a.yaml", []byte(src), "diagram_documents"); err != nil || body == nil {
			t.Errorf("Parse(%q) = %v, %v", src, body, err)
		}
	}
	for _, src := range []string{"", "# only a comment\n", "---\n...\n"} {
		if body, err := yamlpos.Parse("a.yaml", []byte(src), "diagram_documents"); !errors.Is(err, yamlpos.ErrEmpty) || body != nil {
			t.Errorf("Parse(%q) = %v, %v; want no document", src, body, err)
		}
	}
	for _, tt := range []struct{ src, want string }{
		{"a: 1\n---\nb: 2\n", "a.yaml:2:1: a spec holds one diagram"},
		{"a: 1\n...\nb: 2\n", "a.yaml:3:1: a spec holds one diagram"},
		{"a: 1\n...\n%YAML 1.2\n---\nb: 2\n", "a.yaml:3:1: a spec holds one diagram"},
		{"a: 1\n---\n%YAML 1.2\n---\nb: 2\n", "a.yaml:2:1: a spec holds one diagram"},
		// goccy/go-yaml reads a second --- as an empty document and drops what follows it.
		{"a: 1\n---\n---\nb: 2\n", "a.yaml:3:1: a spec holds one diagram"},
		{"---\n---\nid: d\n", "a.yaml:2:1: a spec holds one diagram"},
		{"a: 1\n---\n\n---\nb: 2\n", "a.yaml:4:1: a spec holds one diagram"},
		{"a: 'x\n", "a.yaml:1:4: this is not valid YAML: "},
		{"a: 1\na: 2\n", "a.yaml:2:1: this is not valid YAML: mapping key \"a\" already defined"},
	} {
		_, err := yamlpos.Parse("a.yaml", []byte(tt.src), "diagram_documents")
		var e *yamlpos.Error
		if !errors.As(err, &e) || !strings.HasPrefix(err.Error(), tt.want) {
			t.Errorf("Parse(%q) = %v, want %s", tt.src, err, tt.want)
		}
	}
}

// A syntax error says to quote text when its tokens show YAML read the text as something else, and
// only then.
func TestSyntaxHints(t *testing.T) {
	t.Parallel()

	const (
		bad      = "this is not valid YAML: "
		colon    = `text with ": " in it reads as keys and values: put the text in quotes`
		comment  = "a # after a space begins a comment, which cut { } or [ ] short: put text with # in it in quotes"
		reserved = "a value cannot begin with ` or @ unless it is in quotes"
	)
	for _, tt := range []struct{ src, want string }{
		{"lead: parts-shape: drop\n", "a.yaml:1:7: " + bad + colon + " (mapping value is not allowed in this context)"},
		{"a:\n  - { title: layer #73, from: 2 }\n  - b\n", "a.yaml:2:20: " + bad + comment + " (',' or '}' must be specified)"},
		{"a: [x #y, z]\n", "a.yaml:1:7: " + bad + comment + " (sequence end token ']' not found)"},
		// A ` after the ": " of text is the ": "'s doing, which the hint names, at that ":".
		{"rows:\n  - [a, the `: ` case, c]\n", "a.yaml:2:14: " + bad + colon + " ('`' is a reserved character)"},
		{"a: `b`\n", "a.yaml:1:4: " + bad + reserved + " ('`' is a reserved character)"},
		{"a: { k: `v` }\n", "a.yaml:1:9: " + bad + reserved + " ('`' is a reserved character)"},
		{"a: b: `c`\n", "a.yaml:1:5: " + bad + colon + " ('`' is a reserved character)"},
		// A comment inside a { } or [ ] that closes is the author's, and another mistake gets no hint.
		{"a: [x, # y\n  z]\nb: 'c\n", "a.yaml:3:4: " + bad + "could not find end character of single-quoted text"},
		{"a: 1\na: 2\n", "a.yaml:2:1: " + bad + `mapping key "a" already defined at [1:1]`},
	} {
		_, err := yamlpos.Parse("a.yaml", []byte(tt.src), "diagram_documents")
		if err == nil || err.Error() != tt.want {
			t.Errorf("Parse(%q) = %v, want %s", tt.src, err, tt.want)
		}
	}
	var e *yamlpos.Error
	if _, err := yamlpos.Parse("a.yaml", []byte("lead: parts-shape: drop\n"), "diagram_documents"); !errors.As(err, &e) ||
		e.Localize(i18n.New("ja")) != "a.yaml:1:7: YAML として読めません: 「: 」を含む文はキーと値として読まれます。"+
			"文を引用符で囲んでください（mapping value is not allowed in this context）" {
		t.Errorf("Parse in Japanese = %v", e.Localize(i18n.New("ja")))
	}
}

// ColonText is a plain key and its value on one line; CommentAfter, the text of a comment on the line
// of a value.
func TestColonTextAndCommentAfter(t *testing.T) {
	t.Parallel()

	doc, err := parser.ParseBytes([]byte("- a: b\n- a: b\n  c: d\n- {a: b}\n- \"a\": b\n- a:\n    b\n- x # y z\n- x\n"+
		"- 2024: done\n- a: {b: c}\n- a: [b]\n- a: |\n    x\n- Note:\n- x\n  y # z\n"), 0)
	if err != nil {
		t.Fatal(err)
	}
	seq, ok := doc.Docs[0].Body.(*ast.SequenceNode)
	if !ok {
		t.Fatalf("the body is %T", doc.Docs[0].Body)
	}
	items := seq.Values
	for i, want := range []bool{true, false, false, false, false, false, false, true, false, false, false, false, false} {
		if got := yamlpos.ColonText(items[i]); got != want {
			t.Errorf("ColonText(item %d) = %v, want %v", i, got, want)
		}
	}
	if got := yamlpos.CommentAfter(items[5]); got != " y z" {
		t.Errorf("CommentAfter(x # y z) = %q", got)
	}
	if got := yamlpos.CommentAfter(items[6]); got != "" {
		t.Errorf("CommentAfter(x) = %q", got)
	}
	// On the last line of a value written over lines.
	if got := yamlpos.CommentAfter(items[12]); got != " z" {
		t.Errorf("CommentAfter(x y # z) = %q", got)
	}
}

func TestProblems(t *testing.T) {
	t.Parallel()

	p := yamlpos.Problems{
		yamlpos.New("a.yaml", yamlpos.Pos{Line: 1, Column: 2}, "yaml_syntax", "x"),
		yamlpos.New("a.yaml", yamlpos.Pos{Line: 3, Column: 4}, "yaml_no_anchors"),
	}
	if got, want := p.Error(), "a.yaml:1:2: this is not valid YAML: x\n"+
		"a.yaml:3:4: anchors, aliases, merge keys (<<) and tags are not supported; write the value out"; got != want {
		t.Errorf("Error = %q, want %q", got, want)
	}
	if got, want := p.Localize(i18n.New("ja")), "a.yaml:1:2: YAML として読めません: x\n"+
		"a.yaml:3:4: アンカー・エイリアス・マージキー（<<）・タグは使えません。値をそのまま書いてください"; got != want {
		t.Errorf("Localize = %q, want %q", got, want)
	}
}
