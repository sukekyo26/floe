package lint

import (
	"slices"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// rawDoc is the doc that tells how to write raw SVG, and the heading of its section: the table of the colours to take,
// the one whose first column is headed Variable, is held to the default palette.
const (
	rawDoc     = "agents/diagrams.md"
	rawHeading = "raw"
)

// rawInDocs keeps the table of colours in the section of raw SVG of docs/agents/diagrams.md, which floe diagram guide
// prints, to default's palette.css in two directions: each variable the first column names is one the palette sets, and
// each variable the palette sets is named there, but those the rule allows (Allow: a face, a size, a corner, a shadow and
// the colours of one part, which raw SVG is not told to take). A variable the palette gains then has to go in the table,
// or in the exceptions with why not.
// LIMIT: only the first column is held; what the other columns say of a variable is not. The exceptions are named by
// hand: which variable of the palette is a colour is not read from its value.
var rawInDocs = Rule{
	Name: "raw-in-docs",
	Check: func(in *Input) []Finding {
		var set []string
		for _, s := range in.Sheets() {
			if s.Kind == kindTheme && s.Owner == "default" && s.For == "palette" {
				for name := range declarations(s.Src, func(p string) bool { return strings.HasPrefix(p, "--") }) {
					set = append(set, name)
				}
			}
		}
		slices.Sort(set)
		if len(set) == 0 {
			return []Finding{{"themes/default/palette.css", 0, "raw-in-docs", "sets no variable; the default palette sets every one"}}
		}
		in.Saw()
		root, err := docTree(in.Docs(), rawDoc)
		if err != "" {
			return []Finding{{rawDoc, 0, "raw-in-docs", err}}
		}
		names, err := variableTable(root, rawHeading)
		if err != "" {
			return []Finding{{rawDoc, 0, "raw-in-docs", err}}
		}
		var found []Finding
		for _, name := range names {
			switch {
			case !slices.Contains(set, name):
				found = append(found, Finding{rawDoc, 0, "raw-in-docs", "the table of colours names " + name + ", which the default palette does not set: remove the row"})
			case in.Allowed(name):
				found = append(found, Finding{rawDoc, 0, "raw-in-docs", "the table of colours names " + name + ", which is not one to take: remove the row"})
			}
		}
		for _, name := range set {
			if !slices.Contains(names, name) && !in.Allowed(name) {
				found = append(found, Finding{rawDoc, 0, "raw-in-docs", "names no " + name + ", which the default palette sets: write a row for it in the table of colours, or, if raw SVG is not to take it, allow it in the rule with why"})
			}
		}
		return found
	},
	Allow: []Allow{
		{"--font", "a face"},
		{"--display", "a face"},
		{"--mono", "a face"},
		{"--text-width", "a size"},
		{"--page-width", "a size"},
		{"--page-pad", "a size"},
		{"--radius", "a corner"},
		{"--radius-small", "a corner"},
		{"--radius-pill", "a corner"},
		{"--rule", "a border, not a colour"},
		{"--shadow", "the colour of shadows"},
		{"--page-bg", "the page's background pattern"},
		{"--chart-1", "a series of the part chart"},
		{"--chart-2", "a series of the part chart"},
		{"--chart-3", "a series of the part chart"},
		{"--chart-4", "a series of the part chart"},
		{"--code-bg", "the panel of the part code"},
		{"--code-fg", "the panel of the part code"},
		{"--code-prompt", "the panel of the part code"},
		{"--code-add", "the panel of the part code"},
		{"--code-del", "the panel of the part code"},
	},
}

// variableTable returns the code of the first column of the table headed Variable, the first one after the h2 or h3 of the
// text heading in the tree of a doc, or why it cannot.
func variableTable(root *html.Node, heading string) ([]string, string) {
	under := false
	for n := range root.Descendants() {
		switch {
		case n.DataAtom == atom.H2 || n.DataAtom == atom.H3:
			under = textOf(n) == heading
		case under && n.DataAtom == atom.Table:
			first := false
			for th := range n.Descendants() {
				if th.DataAtom == atom.Th {
					first = textOf(th) == "Variable"
					break
				}
			}
			if !first {
				continue
			}
			var names []string
			for tr := range n.Descendants() {
				if tr.DataAtom != atom.Tr || tr.Parent.DataAtom != atom.Tbody {
					continue
				}
				for td := tr.FirstChild; td != nil; td = td.NextSibling {
					if td.DataAtom == atom.Td {
						names = append(names, codeIn(td)...)
						break
					}
				}
			}
			return names, ""
		}
	}
	return nil, "has no table headed Variable under the heading " + heading + "; write the table of colours there"
}
