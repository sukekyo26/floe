package yamlpos_test

import (
	"testing"

	"github.com/sukekyo26/floe/internal/yamlpos"
)

// Source gives a value as written, whatever the parser's lines and columns say of it, and none of what is around.
func TestSource(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name, src string
		at        []string
		want      string
	}{
		{
			"block", "p:\n  - d:\n      # before\n      w: 300   # trailing\n      t: { en: F, ja: \"流れ\" }\n" +
				"      parts:\n        - box: { x: 1 }\n\n        - label: |\n            two\n            lines\n      s: true\n" +
				"      # after\n  - text: after\n",
			[]string{"p", "0", "d"},
			"w: 300   # trailing\nt: { en: F, ja: \"流れ\" }\nparts:\n  - box: { x: 1 }\n\n  - label: |\n" +
				"      two\n      lines\ns: true",
		},
		{"flow", "p:\n  - d: {w: 1, t: \"流れ\"}  # c\n  - text: x\n", []string{"p", "0", "d"}, `{w: 1, t: "流れ"}`},
		{
			"flow over lines", "p:\n  - d: {w: 1,\n      t: x,\n        parts: []}\n  - text: x\n",
			[]string{"p", "0", "d"},
			"{w: 1,\nt: x,\n  parts: []}",
		},
		{"in a flow sequence", "p: [{d: {w: 1}}, {text: x}]\n", []string{"p", "0", "d"}, "{w: 1}"},
		{
			"folded with a blank line", "d:\n  a: >\n    one\n\n    two\n  c: 2\nb: 1\n",
			[]string{"d"},
			"a: >\n  one\n\n  two\nc: 2",
		},
		{
			"folded in an item", "p:\n  - d:\n      t: >\n        one\n\n        two\n  - text: after\n",
			[]string{"p", "0", "d"},
			"t: >\n  one\n\n  two",
		},
		{"tabs", "d:\n  a:\t\t{w: 1}\nb: 2\n", []string{"d"}, "a:\t\t{w: 1}"},
		{"CR", "d:\r  a: 1\r  c: 2\rb: 3\r", []string{"d"}, "a: 1\nc: 2"},
		{"CRLF", "d:\r\n  a: 1\r\n  c: 2\r\nb: 3\r\n", []string{"d"}, "a: 1\nc: 2"},
		{"BOM", "\ufeffd:\n  a: 1\n  c: 2\n", []string{"d"}, "a: 1\nc: 2"},
		{"a comment between", "d:\n  a: 1\n# note\n  b: 2\nc: 3\n", []string{"d"}, "a: 1\n# note\nb: 2"},
		{"literal before a sibling", "p:\n  - x: |2\n      one\n  - x: 2\n", []string{"p", "0"}, "x: |2\n  one"},
		{"a comment ending the last line", "d:\n  t: \"x\"  # last\nb: 1\n", []string{"d"}, `t: "x"`},
		// The LIMIT of a quoted value of a shallow next line: the keys after it seem to move.
		{"quoted over a shallow line", "d:\n  a: \"one\n two\"\n  b: 1\n", []string{"d"}, "a: \"one\ntwo\"\n b: 1"},
	} {
		root, err := yamlpos.Parse("d.yaml", []byte(tt.src))
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		_, v := yamlpos.Find(root, tt.at)
		if got := yamlpos.Source(v); got != tt.want {
			t.Errorf("%s: Source =\n%q\nwant\n%q", tt.name, got, tt.want)
		}
	}
}

// File is a spec file as shown: no BOM, line breaks \n, trimmed.
func TestFile(t *testing.T) {
	t.Parallel()

	if got := yamlpos.File([]byte("\ufeffa: 1\r\nb: 2\r\n\n")); got != "a: 1\nb: 2" {
		t.Errorf("File = %q", got)
	}
}
