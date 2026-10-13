package tmpl

import "testing"

// A copy's example names the copy, its other text as it was.
func TestRenamed(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ src, want string }{
		{"# report\ntemplate: report # the report\ntitle: report\n", "# report\ntemplate: spec # the report\ntitle: report\n"},
		{"title: t\ntemplate: 'report'\n", "title: t\ntemplate: 'spec'\n"},
		{"template: \"report\"\r\n", "template: \"spec\"\r\n"},
		{"title: no template\n", "title: no template\n"},
		{"template: [report]\n", "template: [report]\n"},
		{"sections:\n  - template: report\n", "sections:\n  - template: report\n"},
		{"not: [valid\n", "not: [valid\n"},
		// Blanks after the value, or a value on a line of its own, do not lose it.
		{"template: report   \n", "template: spec   \n"},
		{"template:   report \t# r\n", "template:   spec \t# r\n"},
		{"template:\n  report\n", "template:\n  spec\n"},
		// A column counts characters, not bytes.
		{"{a: 日日日日 report, template: report}\n", "{a: 日日日日 report, template: spec}\n"},
	} {
		if got := string(renamed([]byte(tt.src), "spec")); got != tt.want {
			t.Errorf("renamed(%q) = %q, want %q", tt.src, got, tt.want)
		}
	}
}
