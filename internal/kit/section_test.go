package kit_test

import (
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"floe/docs"
	"floe/internal/kit"
)

// A level 2 section goes from its heading's line to the next heading of level 1 or 2, read from the Markdown's tree:
// not cut at a level 3 heading, a line of fenced code like a heading, or a heading in a quote or list; a setext heading
// of one or more lines is found by its first line, and CRLF lines are kept as they are. A section of no such heading is
// floe's failure.
func TestWithoutSection(t *testing.T) {
	t.Parallel()

	for _, c := range []struct{ name, md, want string }{
		{"middle", "# D\n\n## A\n\na\n\n## Tasks\n\nt\n\n### Sub\n\ns\n\n```\n## Fake\n```\n\n## B\n\nb\n", "# D\n\n## A\n\na\n\n## B\n\nb\n"},
		{"last", "## A\n\na\n\n## Tasks\n\nt\n", "## A\n\na\n\n"},
		{"to a level 1", "## Tasks\n\nt\n\n# Next\n", "# Next\n"},
		{"setext", "A\n-\n\nTasks\n---\n\nt\n\nB\n-\n", "A\n-\n\nB\n-\n"},
		{"setext ends it", "## Tasks\n\nt\n\nTwo\nlines\n---\n", "Two\nlines\n---\n"},
		{"crlf", "## A\r\n\r\n## Tasks\r\n\r\nt\r\n\r\n## B\r\n", "## A\r\n\r\n## B\r\n"},
		{"not in a quote or list", "## Tasks\n\n> ## Q\n\n- ## L\n\nt\n\n## B\n", "## B\n"},
	} {
		got, err := kit.WithoutSection([]byte(c.md), "Tasks")
		if err != nil || string(got) != c.want {
			t.Errorf("%s: %q, %v; want %q", c.name, got, err, c.want)
		}
	}
	for _, md := range []string{"## A\n", "### Tasks\n", "> ## Tasks\n", "```\n## Tasks\n```\n", "## Tasks of mine\n"} {
		if got, err := kit.WithoutSection([]byte(md), "Tasks"); err == nil {
			t.Errorf("%q: %q, want no section", md, got)
		}
	}
}

// floe guide without the Tasks section, as the task mode off prints it, names that section nowhere: no emphasis
// (*Tasks*) or link to it is left to point at nothing.
func TestGuideWithoutTasks(t *testing.T) {
	t.Parallel()

	off, err := kit.WithoutSection(docs.Guide, "Tasks")
	if err != nil {
		t.Fatal(err)
	}
	if len(off) >= len(docs.Guide) || strings.Contains(string(off), "## Tasks") {
		t.Fatalf("the guide without its Tasks section is %d bytes of %d", len(off), len(docs.Guide))
	}
	err = ast.Walk(kit.PageMarkdown.Parser().Parse(text.NewReader(off)), func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if e, ok := n.(*ast.Emphasis); ok && plainText(e, off) == "Tasks" {
			t.Error("the guide without its Tasks section names *Tasks*")
		}
		if l, ok := n.(*ast.Link); ok && strings.EqualFold(string(l.Destination), "#tasks") {
			t.Error("the guide without its Tasks section links #tasks")
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
