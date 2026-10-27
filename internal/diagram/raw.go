package diagram

import (
	"fmt"
	"html/template"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/sukekyo26/floe/internal/page"
)

// rawElements are the elements raw SVG may not hold, by their names, in the order they are written.
type rawElements []string

func (e rawElements) Error() string {
	return "raw SVG holds <" + strings.Join(e, ">, <") + ">"
}

// checkRaw reads svg as HTML reads it inside an <svg>, and returns the tree it read, written again: every element
// must be one of SVG's, and none a script or a foreignObject, whose content a page reads as script or HTML, or
// one page.Refused refuses, which no input may write. HTML's elements that leave SVG's content (as a <p> does) are
// read as HTML's, and refused so. What is not closed is written closed. Attributes are kept as written: an onload
// or a javascript: URL runs as a script part's script does, under the page's CSP in the sandbox it is shown in.
func checkRaw(svg string) (template.HTML, error) {
	// A parent alone, to read as an <svg>'s content.
	context := &html.Node{ //nolint:exhaustruct_v5 // see above
		Type: html.ElementNode, DataAtom: atom.Svg, Data: "svg", Namespace: "svg",
	}
	nodes, err := html.ParseFragment(strings.NewReader(svg), context)
	if err != nil {
		return "", fmt.Errorf("diagram: read raw SVG: %w", err)
	}
	var bad rawElements
	for _, n := range nodes {
		bad = append(bad, refused(n)...)
	}
	if bad != nil {
		return "", bad
	}
	var b strings.Builder
	for _, n := range nodes {
		if err := html.Render(&b, n); err != nil {
			return "", fmt.Errorf("diagram: write raw SVG: %w", err)
		}
	}
	// Read and written by x/net/html, of SVG's elements alone, none a script or a foreignObject.
	return template.HTML(b.String()), nil //nolint:gosec // see above
}

// refused are the names of the elements under n, n itself included, that raw SVG may not hold, in tree order;
// what one of them holds is not looked into, as it goes with it.
func refused(n *html.Node) []string {
	if n.Type == html.ElementNode {
		if n.Namespace != "svg" || page.Refused(n) != "" {
			return []string{n.Data}
		}
		switch strings.ToLower(n.Data) {
		case "script", "foreignobject":
			return []string{n.Data}
		}
	}
	var bad []string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		bad = append(bad, refused(c)...)
	}
	return bad
}
