package kit_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"floe/internal/kit"
)

// plainText returns the text of n and what it holds, as the page draws it: a code span's text too, so that a heading
// *What `floe part check` refuses* and the words that name it are one text; whitespace collapsed to single spaces.
func plainText(n ast.Node, src []byte) string {
	var b strings.Builder
	var add func(ast.Node)
	add = func(n ast.Node) {
		switch n := n.(type) {
		case *ast.Text:
			b.Write(n.Segment.Value(src))
			if n.SoftLineBreak() || n.HardLineBreak() {
				b.WriteByte(' ')
			}
			return
		case *ast.String:
			b.Write(n.Value)
			return
		}
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			add(c)
		}
	}
	add(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

// The docs floe prints name a section of a doc in emphasis, (*Tones*): an agent reads the name and looks for the heading.
// So emphasis is for that alone: each emphasis (`*…*`; **…** is not one) of a doc of docs/agents is the text of a heading
// of one of them, read from the Markdown's tree as the page draws it. A heading that goes, or an emphasis that names none,
// is told.
// LIMIT: the heading may be in any doc, not the one the sentence points at (it is told by the command of the doc beside
// it, `floe part guide` (*Tones*), which the test does not read); a section of a doc of the same name as another's passes.
func TestAgentDocEmphasis(t *testing.T) {
	t.Parallel()

	type seen struct{ doc, text string }
	var emphasised []seen
	var headings []string
	for _, rel := range embeddedDocs(t) {
		src, err := os.ReadFile("../../docs/" + rel) //nolint:gosec // G304: a doc of this repository
		if err != nil {
			t.Fatal(err)
		}
		tree := kit.PageMarkdown.Parser().Parse(text.NewReader(src))
		err = ast.Walk(tree, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				return ast.WalkContinue, nil
			}
			switch n := n.(type) {
			case *ast.Heading:
				headings = append(headings, plainText(n, src))
			case *ast.Emphasis:
				if n.Level == 1 {
					emphasised = append(emphasised, seen{rel, plainText(n, src)})
				}
			}
			return ast.WalkContinue, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(emphasised) < 20 || len(headings) < 50 {
		t.Fatalf("%d emphasised texts and %d headings found: the docs name their sections in emphasis", len(emphasised), len(headings))
	}
	for _, e := range emphasised {
		if !slices.Contains(headings, e.text) {
			t.Errorf("docs/%s: *%s* is no heading of a doc of docs/agents: emphasis names a section, so write the text plain, or the heading it names", e.doc, e.text)
		}
	}
}
