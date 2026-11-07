package css_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/sukekyo26/floe/internal/css"
	"github.com/sukekyo26/floe/internal/kit"
)

// TestPaletteInDocs keeps every variable of floe's default palette named, whole, in the first column of the docs'
// table of the palette, English and Japanese: raw SVG and one's own themes rely on that table, and a variable out of
// it is no promise. Every name in that column is whole (--good-soft, not -soft), so that the table reads as the test.
// LIMIT: default's palette.css alone is read; a variable only a theme sets is the theme's own, and the docs promise
// none of them but those the table names.
func TestPaletteInDocs(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("../kit/floe/themes/default/palette.css")
	if err != nil {
		t.Fatal(err)
	}
	set := css.Variables(src)
	if len(set) == 0 {
		t.Fatal("default's palette.css sets no variable")
	}
	for doc, heading := range map[string]string{
		"../../docs/templates.md": "The palette", "../../docs/templates.ja.md": "パレット",
	} {
		names := paletteTable(t, doc, heading)
		for _, name := range names {
			if !strings.HasPrefix(name, "--") || strings.ContainsAny(name, "<>") {
				t.Errorf("%s: the table of the palette names %q: write each variable's name whole", doc, name)
			}
		}
		for _, name := range set {
			if !slices.Contains(names, name) {
				t.Errorf("%s does not name %s, which default's palette.css sets: write `%s` in the table of the palette",
					doc, name, name)
			}
		}
	}
}

// paletteTable returns the code in the first column of the first table after the heading of the text heading in doc,
// Markdown, as floe draws it.
func paletteTable(t *testing.T, doc, heading string) []string {
	t.Helper()

	text, err := os.ReadFile(doc)
	if err != nil {
		t.Fatal(err)
	}
	drawn, err := kit.Markdown(string(text))
	if err != nil {
		t.Fatal(err)
	}
	root, err := html.Parse(strings.NewReader(string(drawn)))
	if err != nil {
		t.Fatal(err)
	}
	var table *html.Node
	under := false
	for n := range root.Descendants() {
		switch {
		case n.DataAtom == atom.H2:
			under = textOf(n) == heading
		case under && n.DataAtom == atom.Table && table == nil:
			table = n
		}
	}
	if table == nil {
		t.Fatalf("%s has no table under the heading %s", doc, heading)
	}
	var names []string
	for n := range table.Descendants() {
		if n.DataAtom != atom.Tr || n.Parent.DataAtom != atom.Tbody {
			continue
		}
		cell := n.FirstChild
		for cell != nil && cell.DataAtom != atom.Td {
			cell = cell.NextSibling
		}
		if cell == nil {
			continue
		}
		for c := range cell.Descendants() {
			if c.DataAtom == atom.Code {
				names = append(names, textOf(c))
			}
		}
	}
	return names
}

func textOf(n *html.Node) string {
	var b strings.Builder
	for c := range n.Descendants() {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
		}
	}
	return b.String()
}
