package draw

import (
	"slices"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// headingID is the id of a heading whose text is text, near the one GitHub gives: lower case, without what
// is not a letter, a mark, a number, - or _, the spaces between the words left made one -, as a browser
// shows them as one (an ideographic space and a no-break space too).
// LIMIT: Go's Unicode classes and ToLower, not github-slugger's pattern and toLowerCase; they part on rare runes.
func headingID(text string) string {
	kept := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r) || r == '-' || r == '_' || unicode.IsSpace(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, text)
	return strings.Join(strings.Fields(kept), "-")
}

var headings = []atom.Atom{atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6}

// withHeadingIDs gives each heading of HTML in tree without an id, or with an empty one, which names nothing,
// the id headingID makes of its text, but what a part draws in it (data-floe-part), in the order of the page,
// with -1, -2 and on till one is free of the ids and the names of the page, wherever they are, and of those
// given before it. One whose text makes none gets none.
func withHeadingIDs(tree *html.Node) {
	found, taken := headingsOf(tree)
	for _, n := range found {
		if id, _ := attrOf(n, "id"); id != "" {
			continue
		}
		if id := headingID(textOf(n)); id != "" {
			id = free(taken, id)
			taken[id] = true
			n.Attr = slices.DeleteFunc(n.Attr, func(a html.Attribute) bool { return a.Namespace == "" && a.Key == "id" })
			n.Attr = append(n.Attr, html.Attribute{Namespace: "", Key: "id", Val: id})
		}
	}
}

// headingsOf returns the headings of HTML in tree, in order, and what a link may go to: the ids and the names
// of its <a>s.
func headingsOf(tree *html.Node) ([]*html.Node, map[string]bool) {
	taken := map[string]bool{}
	var found []*html.Node
	for n := range tree.Descendants() {
		if n.Type != html.ElementNode {
			continue
		}
		if n.Namespace == "" && slices.Contains(headings, n.DataAtom) {
			found = append(found, n)
		}
		named := n.Namespace == "" && n.DataAtom == atom.A
		for _, a := range n.Attr {
			if a.Namespace == "" && a.Val != "" && (a.Key == "id" || named && a.Key == "name") {
				taken[a.Val] = true
			}
		}
	}
	return found, taken
}

// textOf returns the text in n, but that of a part (data-floe-part), of an element whose content the
// parser reads as text (a <script>, a <style>) and of an element not of HTML (an icon's <svg>).
func textOf(n *html.Node) string {
	var b strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			_, isPart := attrOf(c, "data-floe-part")
			switch {
			case c.Type == html.TextNode:
				b.WriteString(c.Data)
			case c.Type == html.ElementNode && c.Namespace == "" && !isPart && !readAsText(c):
				walk(c)
			}
		}
	}
	walk(n)
	return b.String()
}

// readAsText reports whether the parser reads what is in n as its text.
func readAsText(n *html.Node) bool {
	switch n.DataAtom {
	case atom.Script, atom.Style, atom.Title, atom.Textarea, atom.Noscript, atom.Xmp, atom.Iframe,
		atom.Noembed, atom.Noframes, atom.Plaintext:
		return n.Namespace == ""
	default:
		return false
	}
}

// free returns id, or the first of id-1, id-2 and on, that taken has not.
func free(taken map[string]bool, id string) string {
	for n, base := 1, id; taken[id]; n++ {
		id = base + "-" + strconv.Itoa(n)
	}
	return id
}
