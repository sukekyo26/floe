package lint

import (
	"io/fs"
	"slices"
	"strings"
	"testing"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"floe/internal/kit"
)

// paletteDocs are the docs that hold a table of the palette, each with the heading over its table: the template guide
// and the theme guide print one each, and both are held to the default palette.
var paletteDocs = []struct{ file, heading string }{
	{"agents/templates.md", "The palette"},
	{"agents/themes.md", "Palette"},
}

// paletteInDocs keeps every variable of the default palette named, whole, in the first column of each doc's table of the
// palette: raw SVG and one's own themes rely on that table, and a variable out of it
// is no promise. Every name in that column is whole (--good-soft, not -soft), so that the table reads as the rule.
// LIMIT: default's palette.css alone is read; a variable only a theme sets is the theme's own, and the docs promise none
// of them but those the table names. The table names more than the default palette (a theme's --<tone>-fill, the
// shadow's, the tone's), so a name of it is not held to the palette.
// A table with a Default column (both guides') is held to the values too: the code in a row's Default cell is the
// value of each of its names, in order, as default's palette.css writes it, whitespace aside.
var paletteInDocs = Rule{Name: "palette-in-docs", Check: func(in *Input) []Finding {
	var set []string
	var palette Sheet
	for _, s := range in.Sheets() {
		if s.Kind == kindTheme && s.Owner == kit.DefaultTheme && s.For == "palette" {
			palette = s
			for name := range declarations(s.Src, func(p string) bool { return strings.HasPrefix(p, "--") }) {
				set = append(set, name)
			}
		}
	}
	slices.Sort(set)
	values := map[string][]held{}
	for name, ds := range declarations(palette.Src, func(p string) bool { return strings.HasPrefix(p, "--") }) {
		if d := ds[len(ds)-1]; len(d.value) > 0 {
			values[name] = []held{{file: palette.File, value: valueOf(palette.Src, d)}}
		}
	}
	if len(set) == 0 {
		return []Finding{{"themes/default/palette.css", 0, "palette-in-docs", "sets no variable; the default palette sets every one"}}
	}
	var found []Finding
	in.Saw()
	for _, doc := range paletteDocs {
		root, err := docTree(in.Docs(), doc.file)
		if err != "" {
			found = append(found, Finding{doc.file, 0, "palette-in-docs", err})
			continue
		}
		rows, err := paletteTable(root, doc.heading, "the palette")
		if err != "" {
			found = append(found, Finding{doc.file, 0, "palette-in-docs", err})
			continue
		}
		var names []string
		for _, row := range rows {
			names = append(names, row.names...)
			if !row.hasDefault {
				continue
			}
			for _, text := range rowDefaults(row, values, func(n string) bool { _, ok := values[n]; return ok }) {
				found = append(found, Finding{doc.file, 0, "palette-in-docs", doc.heading + ": " + text})
			}
		}
		for _, name := range names {
			if !strings.HasPrefix(name, "--") || strings.ContainsAny(name, "<>") {
				found = append(found, Finding{doc.file, 0, "palette-in-docs", "the table of the palette names " + name + ": write each variable's name whole"})
			}
		}
		for _, name := range set {
			if !slices.Contains(names, name) {
				found = append(found, Finding{doc.file, 0, "palette-in-docs", "names no " + name + ", which " + palette.File + " sets: write `" + name + "` in the table of the palette"})
			}
		}
	}
	return found
}}

// paletteRow is a row of the table of the palette: the code in its first column, and, when the table has a Default
// column, the code in that cell, each with the text before it there (labelsIn).
type paletteRow struct {
	names, defaults, labels []string
	hasDefault              bool
}

// docTree returns the doc file of docs, Markdown, as floe draws it, as an HTML tree, or why it cannot.
func docTree(docs fs.FS, file string) (*html.Node, string) {
	src, err := fs.ReadFile(docs, file)
	if err != nil {
		return nil, err.Error()
	}
	drawn, err := kit.Markdown(string(src))
	if err != nil {
		return nil, err.Error()
	}
	root, err := html.Parse(strings.NewReader(string(drawn)))
	if err != nil {
		return nil, err.Error()
	}
	return root, ""
}

// paletteTable returns the rows of the first table after the h2 or h3 of the text heading in the tree of a doc, or why it
// cannot. what names the table in the message that asks for it, as "the palette" in "the table of the palette".
func paletteTable(root *html.Node, heading, what string) ([]paletteRow, string) {
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
		return nil, "has no table under the heading " + heading + "; write the table of " + what + " there"
	}
	defaultAt := -1
	for n := range table.Descendants() {
		if n.DataAtom == atom.Th && textOf(n) == "Default" {
			defaultAt = 0
			for p := n.PrevSibling; p != nil; p = p.PrevSibling {
				if p.DataAtom == atom.Th {
					defaultAt++
				}
			}
		}
	}
	var rows []paletteRow
	for n := range table.Descendants() {
		if n.DataAtom != atom.Tr || n.Parent.DataAtom != atom.Tbody {
			continue
		}
		var cells []*html.Node
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.DataAtom == atom.Td {
				cells = append(cells, c)
			}
		}
		if len(cells) == 0 {
			continue
		}
		row := paletteRow{names: codeIn(cells[0])}
		if defaultAt >= 0 && defaultAt < len(cells) {
			row.hasDefault = true
			row.defaults, row.labels = labelsIn(cells[defaultAt])
		}
		rows = append(rows, row)
	}
	return rows, ""
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

// labelsIn returns the text of each code element in n, as codeIn does, and the text before each, from the code before
// it or n's start, its spaces and commas at its ends cut: a part's name in "note `1.6rem`, cards `1.75rem` (a card's
// number)". The text after the last code is read by neither.
func labelsIn(n *html.Node) (codes, labels []string) {
	var before strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			switch {
			case c.DataAtom == atom.Code:
				codes = append(codes, strings.Join(strings.Fields(textOf(c)), " "))
				labels = append(labels, strings.TrimFunc(before.String(), func(r rune) bool { return unicode.IsSpace(r) || r == ',' }))
				before.Reset()
			case c.Type == html.TextNode:
				before.WriteString(c.Data)
			default:
				walk(c)
			}
		}
	}
	walk(n)
	return codes, labels
}

// codeIn returns the text of each code element in n, whitespace collapsed to single spaces.
func codeIn(n *html.Node) []string {
	var out []string
	for c := range n.Descendants() {
		if c.DataAtom == atom.Code {
			out = append(out, strings.Join(strings.Fields(textOf(c)), " "))
		}
	}
	return out
}

// TestLabelsIn reads the name before each code of a cell, from the code before it, its spaces and commas at its ends cut,
// and leaves the text after the last code out.
func TestLabelsIn(t *testing.T) {
	t.Parallel()

	for cell, want := range map[string]string{
		"note `1rem`, cards `2rem` (on x)":        "note=1rem cards=2rem",
		"`2rem`, note `1rem`":                     "=2rem note=1rem",
		"(on x) cards `2rem`":                     "(on x) cards=2rem",
		"a card's number `1rem`,\n  links `2rem`": "a card's number=1rem links=2rem",
		"**note** `1rem`":                         "note=1rem",
	} {
		drawn, err := kit.Markdown(cell)
		if err != nil {
			t.Fatal(err)
		}
		root, err := html.Parse(strings.NewReader(string(drawn)))
		if err != nil {
			t.Fatal(err)
		}
		codes, labels := labelsIn(root)
		var got []string
		for i := range codes {
			got = append(got, labels[i]+"="+codes[i])
		}
		if strings.Join(got, " ") != want {
			t.Errorf("%q: %q, want %q", cell, got, want)
		}
	}
}
