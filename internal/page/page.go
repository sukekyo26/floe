// Package page reads an HTML page as a parser does: its title, its first element of a tag, and the elements an
// input (a page.html, a part's part.html, an item of format html, a diagram's raw SVG) may not write.
package page

import (
	"slices"
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
	n := First(root, atom.Title)
	if n == nil {
		n = First(root, atom.H1)
	}
	if n == nil {
		return ""
	}
	return strings.Join(strings.FieldsFunc(text(n), isASCIISpace), " ")
}

// First returns the first element of n, in tree order, of the HTML namespace and the tag a, or nil.
func First(n *html.Node, a atom.Atom) *html.Node {
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

// Refusable are the names Refused returns: the elements no input may write, of HTML, SVG or MathML. floe puts a
// page's CSS together, in layers, and an SVG's <style> styles the whole page as an HTML one does; the page's CSP
// ignores a <base>; a <noscript> is read as text with scripts on, and its elements work, unchecked, with them off;
// a page floe draws is static, and a <meta> but those shown does not show it.
var Refusable = []string{"base", "meta", "noscript", "style"}

// Refused returns the name of n when it is an element of Refusable that an input may not write, of any namespace,
// "" when it may. The <style> and the CSP's <meta> floe puts in after the check are not the input's.
func Refused(n *html.Node) string {
	if n.Type != html.ElementNode || !slices.Contains(Refusable, n.Data) || n.Data == "meta" && shown(n) {
		return ""
	}
	return n.Data
}

// shown reports whether n, a <meta>, is one the page drawn may have: its charset alone, or a viewport's name and
// content. A page floe draws is static: what does not show it (a refresh, a referrer policy, a generator) is
// none of its.
func shown(n *html.Node) bool {
	if len(n.Attr) == 1 && n.Attr[0].Key == "charset" {
		return true
	}
	return slices.ContainsFunc(n.Attr, func(a html.Attribute) bool {
		return a.Namespace == "" && a.Key == "name" && strings.EqualFold(a.Val, "viewport")
	}) && !slices.ContainsFunc(n.Attr, func(a html.Attribute) bool {
		return a.Namespace != "" || a.Key != "name" && a.Key != "content"
	})
}
