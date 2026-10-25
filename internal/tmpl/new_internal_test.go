package tmpl

import "testing"

// A copy's example names the copy, its other text as it was.
func TestRenamed(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ src, want string }{
		{"# report\ntemplate: report\n# the report\ntitle: report\n", "# report\ntemplate: spec\n# the report\ntitle: report\n"},
		// A comment after the value, which its check refuses, does not stop the copy naming itself.
		{"template: report # the report\n", "template: spec # the report\n"},
		{"template:   report \t# r\n", "template:   spec \t# r\n"},
		{"title: t\ntemplate: 'report'\n", "title: t\ntemplate: 'spec'\n"},
		{"template: \"report\"\r\n", "template: \"spec\"\r\n"},
		{"title: no template\n", "title: no template\n"},
		{"template: [report]\n", "template: [report]\n"},
		{"sections:\n  - template: report\n", "sections:\n  - template: report\n"},
		{"not: [valid\n", "not: [valid\n"},
		// Blanks after the value, or a value on a line of its own, do not lose it.
		{"template: report   \n", "template: spec   \n"},
		{"template:   report \t\n# r\n", "template:   spec \t\n# r\n"},
		{"template:\n  report\n", "template:\n  spec\n"},
		// A key in quotes is not written over.
		{"\"template\": report\n", "\"template\": spec\n"},
		{"{'template':report}\n", "{'template':spec}\n"},
		// A column counts characters, not bytes.
		{"{a: 日日日日 report, template: report}\n", "{a: 日日日日 report, template: spec}\n"},
	} {
		if got := string(renamed([]byte(tt.src), "spec")); got != tt.want {
			t.Errorf("renamed(%q) = %q, want %q", tt.src, got, tt.want)
		}
	}
}

// A copy of a part names itself as its box, where the box of the original named it as a browser reads it
// (a character reference read), and nowhere else.
func TestRenamedPart(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ src, want string }{
		{`<div data-floe-part="table"><span data-floe-part="cell">x</span></div>`, `<div data-floe-part="grid"><span data-floe-part="cell">x</span></div>`},
		{`<div DATA-FLOE-PART='table' class="t">`, `<div DATA-FLOE-PART='grid' class="t">`},
		{`<div data-floe-part = table>`, `<div data-floe-part = grid>`},
		{`<div data-floe-part="&#116;able">`, `<div data-floe-part="grid">`},
		// A look of the original, a name among others, one put together, and text are kept.
		{`<p data-floe-look="table">`, `<p data-floe-look="table">`},
		{`<p data-floe-part="table compare">`, `<p data-floe-part="table compare">`},
		{`<p data-floe-part="{{.k}}table">`, `<p data-floe-part="{{.k}}table">`},
		{`<!-- data-floe-part="table" --><p>data-floe-part="table"</p>`, `<!-- data-floe-part="table" --><p>data-floe-part="table"</p>`},
		// An attribute a template writes as it likes is the box all the same.
		{`<p title='data-floe-part="table"' {{if .x}}data-floe-part="table"{{end}}>`, `<p title='data-floe-part="table"' {{if .x}}data-floe-part="grid"{{end}}>`},
		{`<a data-floe-part="table"></a><b data-floe-part="table"></b>`, `<a data-floe-part="grid"></a><b data-floe-part="grid"></b>`},
	} {
		if got := renamedPart(tt.src, "table", "grid"); got != tt.want {
			t.Errorf("renamedPart(%q) = %q, want %q", tt.src, got, tt.want)
		}
	}
}
