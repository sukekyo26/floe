package draw

import (
	"os"
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"floe/internal/kit"
)

// firstColumn returns the code in the first column of the first table under the heading of a doc, Markdown as floe draws
// it. The docs hold the same small reader in each package that tests a table (lint's paletteTable is a _test.go of that
// package, which none else can call, and sharing it would take a package of its own for tests).
func firstColumn(t *testing.T, file, heading string) []string {
	t.Helper()

	src, err := os.ReadFile(file) //nolint:gosec // G304: a doc of this repository
	if err != nil {
		t.Fatal(err)
	}
	drawn, err := kit.Markdown(string(src))
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
		case n.DataAtom == atom.H2 || n.DataAtom == atom.H3:
			under = textOf(n) == heading
		case under && n.DataAtom == atom.Table && table == nil:
			table = n
		}
	}
	if table == nil {
		t.Fatalf("%s has no table under the heading %q", file, heading)
	}
	var names []string
	for tr := range table.Descendants() {
		if tr.DataAtom != atom.Tr || tr.Parent.DataAtom != atom.Tbody {
			continue
		}
		for td := tr.FirstChild; td != nil; td = td.NextSibling {
			if td.DataAtom != atom.Td {
				continue
			}
			for c := range td.Descendants() {
				if c.DataAtom == atom.Code {
					names = append(names, textOf(c))
				}
			}
			break
		}
	}
	return names
}

// The table of layers in docs/agents/parts.md, which floe part guide prints, names the layers floe puts a page's CSS
// in, in their order: pageLayers, which pageStyle declares first.
func TestPartsDocLayers(t *testing.T) {
	t.Parallel()

	if got := firstColumn(t, "../../docs/agents/parts.md", "The CSS layers and @scope"); !slices.Equal(got, pageLayers) {
		t.Errorf("the table of layers in docs/agents/parts.md names %v, floe puts the CSS in %v, in that order: write the table as the layers are", got, pageLayers)
	}
}
