package kit

import (
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// docFunctions returns the code in the first column of the table under the heading "Functions of part.html" of
// docs/agents/parts.md, Markdown as floe draws it. The docs hold the same small reader in each package that tests a table
// (lint's paletteTable is a _test.go of that package, which none else can call, and sharing it would take a package of its
// own for tests).
func docFunctions(t *testing.T) []string {
	t.Helper()

	src, err := os.ReadFile("../../docs/agents/parts.md")
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
	var table *html.Node
	under := false
	for n := range root.Descendants() {
		switch {
		case n.DataAtom == atom.H2 || n.DataAtom == atom.H3:
			under = textOf(n) == "Functions of part.html"
		case under && n.DataAtom == atom.Table && table == nil:
			table = n
		}
	}
	if table == nil {
		t.Fatal("docs/agents/parts.md has no table under the heading Functions of part.html")
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
	got := docFunctions(t)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("the table of functions in docs/agents/parts.md names %v, a part.html can call %v: write a row for each, and no other", got, want)
	}
}

// docCode returns the code spans of the section under the h2 heading of docs/agents/parts.md, down to the next h2,
// Markdown as floe draws it.
func docCode(t *testing.T, heading string) []string {
	t.Helper()

	src, err := os.ReadFile("../../docs/agents/parts.md")
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
	var spans []string
	under := false
	for n := range root.Descendants() {
		switch {
		case n.DataAtom == atom.H2:
			under = textOf(n) == heading
		case under && n.DataAtom == atom.Code:
			spans = append(spans, textOf(n))
		}
	}
	return spans
}

// The class names Code's look of docs/agents/parts.md gives (`.names`, `.lines`) are classes the part code's own CSS
// styles: a part that writes them in its HTML is styled by that look, so each one the section names, alone in a code
// span, is in code's part.css.
func TestPartsDocCodeClasses(t *testing.T) {
	t.Parallel()

	k, err := Floe()
	if err != nil {
		t.Fatal(err)
	}
	p, ok := k.Part("code")
	if !ok {
		t.Fatal("floe has no part code")
	}
	css := string(p.CSS())
	named := 0
	for _, span := range docCode(t, "Code's look") {
		if !regexp.MustCompile(`^\.[a-z][a-z-]*$`).MatchString(span) {
			continue
		}
		named++
		if !regexp.MustCompile(regexp.QuoteMeta(span) + `[^a-z-]`).MatchString(css) {
			t.Errorf("docs/agents/parts.md names the class %s in Code's look, which the part code's part.css does not style: write the classes it does", span)
		}
	}
	if named < 5 {
		t.Errorf("%d classes found in Code's look: the section names the classes of code's heading and lines", named)
	}
}
