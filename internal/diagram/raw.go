package diagram

import (
	"fmt"
	"html/template"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/sukekyo26/floe/internal/page"
)

// rawElement is an element raw SVG may not hold, by its name.
type rawElement string

func (e rawElement) Error() string {
	return "raw SVG holds <" + string(e) + ">"
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
	var b strings.Builder
	for _, n := range nodes {
		if bad := refused(n); bad != "" {
			return "", rawElement(bad)
		}
		if err := html.Render(&b, n); err != nil {
			return "", fmt.Errorf("diagram: write raw SVG: %w", err)
		}
	}
	// Read and written by x/net/html, of SVG's elements alone, none a script or a foreignObject.
	return template.HTML(b.String()), nil //nolint:gosec // see above
}

// refused is the name of the first element under n, n itself included, that raw SVG may not hold; "" for none.
func refused(n *html.Node) string {
	if n.Type == html.ElementNode {
		if n.Namespace != "svg" || page.Refused(n) != "" {
			return n.Data
		}
		switch strings.ToLower(n.Data) {
		case "script", "foreignobject":
			return n.Data
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if bad := refused(c); bad != "" {
			return bad
		}
	}
	return ""
}
