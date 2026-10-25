// Package page takes the title of an HTML page.
package page

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Title returns the text of the page's first <title>, else of its first <h1>, as an HTML parser reads
// them (scripting on, as a browser opens a page), with ASCII whitespace collapsed and trimmed as
// document.title does; "" when the page has neither.
// LIMIT: the tree is walked whole, the content of a <template> and an <svg>'s <title> or <text> in an
// <h1> among it, where a browser's document.title does not look; no special case is made for them.
func Title(doc string) string {
	root, err := html.Parse(strings.NewReader(doc))
	if err != nil {
		return ""
	}
	n := first(root, atom.Title)
	if n == nil {
		n = first(root, atom.H1)
	}
	if n == nil {
		return ""
	}
	return strings.Join(strings.FieldsFunc(text(n), isASCIISpace), " ")
}

// first returns the first element of n, in tree order, of the HTML namespace and the tag a, or nil.
func first(n *html.Node, a atom.Atom) *html.Node {
	for d := range n.Descendants() {
		if d.Type == html.ElementNode && d.Namespace == "" && d.DataAtom == a {
			return d
		}
	}
	return nil
}

// text returns the text n holds, its descendants' in tree order.
func text(n *html.Node) string {
	var b strings.Builder
	for d := range n.Descendants() {
		if d.Type == html.TextNode {
			b.WriteString(d.Data)
		}
	}
	return b.String()
}

// isASCIISpace reports whether r is ASCII whitespace, as HTML has it: tab, newline, form feed, carriage
// return and space.
func isASCIISpace(r rune) bool {
	return r == '\t' || r == '\n' || r == '\f' || r == '\r' || r == ' '
}
