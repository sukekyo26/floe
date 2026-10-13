package yamlpos_test

import (
	"testing"

	"github.com/goccy/go-yaml/parser"

	"github.com/sukekyo26/floe/internal/i18n"
	"github.com/sukekyo26/floe/internal/usererr"
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
	clean, err := parser.ParseBytes([]byte("a: 1\n"), 0)
	if err != nil || yamlpos.FirstAnchor(clean.Docs[0].Body) != nil {
		t.Error("FirstAnchor found one in plain YAML")
	}
}

// Find follows keys and indexes, writes the path as it goes, and stops where the path leaves the file.
func TestFind(t *testing.T) {
	t.Parallel()

	doc, err := parser.ParseBytes([]byte("list:\n  - a: 1\n    \"b c\": 2\n"), 0)
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
		{[]string{"list", "0"}, "list[0]", false, yamlpos.Pos{Line: 2, Column: 5}},
		{[]string{"list", "5", "a"}, "list", true, yamlpos.Pos{Line: 2, Column: 3}},
		{[]string{"nope"}, "", false, yamlpos.Pos{Line: 1, Column: 1}},
	} {
		key, value, path := yamlpos.Find(root, tt.at)
		if path != tt.path || (key != nil) != tt.key || yamlpos.Of(value) != tt.pos {
			t.Errorf("Find(%q) = %v, %v at %+v, %q", tt.at, key, value, yamlpos.Of(value), path)
		}
	}
}

func TestProblems(t *testing.T) {
	t.Parallel()

	p := yamlpos.Problems{
		{File: "a.yaml", Pos: yamlpos.Pos{Line: 1, Column: 2}, Err: usererr.New("diagram_empty")},
		{File: "a.yaml", Pos: yamlpos.Pos{Line: 3, Column: 4}, Err: usererr.New("yaml_no_anchors")},
	}
	want := "a.yaml:1:2: spec が空です。id・width・height・title・parts を持つ図を書いてください\n" +
		"a.yaml:3:4: アンカー・エイリアス・タグは使えません。値をそのまま書いてください"
	if got := p.Localize(i18n.New("ja")); got != want {
		t.Errorf("Localize = %q", got)
	}
	if p.Error() == p.Localize(i18n.New("ja")) {
		t.Error("Error is not in English")
	}
}
