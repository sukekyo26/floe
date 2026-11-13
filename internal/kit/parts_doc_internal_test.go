package kit

import (
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// agentDoc returns docs/agents/<file>, Markdown as floe draws it, as an HTML tree. The docs hold the same small reader
// in each package that tests a table (lint's paletteTable is a _test.go of that package, which none else can call, and
// sharing it would take a package of its own for tests).
func agentDoc(t *testing.T, file string) *html.Node {
	t.Helper()

	src, err := os.ReadFile("../../docs/agents/" + file)
	if err != nil {
		t.Fatal(err)
	}
	drawn, err := Markdown(string(src))
	if err != nil {
		t.Fatal(err)
	}
	root, err := html.Parse(strings.NewReader(string(drawn)))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// docFirstColumn returns the code in the first column of the first table under the heading of docs/agents/<file>.
func docFirstColumn(t *testing.T, file, heading string) []string {
	t.Helper()

	var table *html.Node
	under := false
	for n := range agentDoc(t, file).Descendants() {
		switch {
		case n.DataAtom == atom.H2 || n.DataAtom == atom.H3:
			under = textOf(n) == heading
		case under && n.DataAtom == atom.Table && table == nil:
			table = n
		}
	}
	if table == nil {
		t.Fatalf("docs/agents/%s has no table under the heading %s", file, heading)
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

// textOf returns the text n holds.
func textOf(n *html.Node) string {
	var b strings.Builder
	for c := range n.Descendants() {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
		}
	}
	return b.String()
}

// The table of functions in docs/agents/parts.md, which floe part guide prints, has a row for every function a part.html
// can call that floe adds to html/template's: those of page.html (but asset, which a part has not) and a part's own, and
// word, which a part's words.yaml gives, and no other.
func TestPartsDocFunctions(t *testing.T) {
	t.Parallel()

	want := append(slices.Collect(maps.Keys(partFuncsAll())), "word")
	slices.Sort(want)
	got := docFirstColumn(t, "parts.md", "Functions of part.html")
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("the table of functions in docs/agents/parts.md names %v, a part.html can call %v: write a row for each, and no other", got, want)
	}
}
