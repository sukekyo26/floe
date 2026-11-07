package lint

import (
	"io/fs"
	"slices"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/sukekyo26/floe/internal/kit"
)

// paletteDocs are the docs that hold the table of the palette, and the heading over it.
var paletteDocs = []struct{ file, heading string }{
	{"templates.md", "The palette"}, {"templates.ja.md", "パレット"},
}

// paletteInDocs keeps every variable of the default palette named, whole, in the first column of the docs' table of the
// palette, English and Japanese: raw SVG and one's own themes rely on that table, and a variable out of it
// is no promise. Every name in that column is whole (--good-soft, not -soft), so that the table reads as the rule.
// LIMIT: default's palette.css alone is read; a variable only a theme sets is the theme's own, and the docs promise none
// of them but those the table names. The table names more than the default palette (a theme's --<tone>-fill, the
// shadow's, the tone's), so a name of it is not held to the palette.
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
	if len(set) == 0 {
		return []Finding{{"themes/default/palette.css", 0, "palette-in-docs", "sets no variable; the default palette sets every one"}}
	}
	var found []Finding
	for _, doc := range paletteDocs {
		names, err := paletteTable(in.Docs(), doc.file, doc.heading)
		if err != "" {
			found = append(found, Finding{doc.file, 0, "palette-in-docs", err})
			continue
		}
		in.Saw()
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

// paletteTable returns the code in the first column of the first table after the heading of the text heading in the doc
// file of docs, Markdown, as floe draws it, or why it cannot.
func paletteTable(docs fs.FS, file, heading string) ([]string, string) {
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
		return nil, "has no table under the heading " + heading + "; write the table of the palette there"
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
	return names, ""
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
