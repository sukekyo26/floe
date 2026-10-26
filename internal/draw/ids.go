package draw

import (
	"bytes"
	"crypto/rand"
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

// withHeadingIDs gives each heading of page without an id the id headingID makes of its text, but what a
// part draws in it (data-floe-part), in the order of the page as the parser reads it, with -1, -2 and on
// till one is free of the ids and the names of the page, wherever they are, and of those given before it.
// One whose text makes none gets none. The rest of page is left byte for byte. Which start tag each heading
// the parser makes is of is told by a mark on each, which the page returned has not: the parser ends a
// heading where the tags around it say, and moves one out of a table.
func withHeadingIDs(page []byte) []byte {
	mark := "data-floe-heading-" + strings.ToLower(rand.Text())
	marked := atHeadingTags(page, func(k int) string { return " " + mark + `="` + strconv.Itoa(k) + `"` })
	tree, _ := html.Parse(bytes.NewReader(marked)) //nolint:errcheck // a bytes.Reader fails no read, Parse's one error
	found, taken := headingsOf(tree)
	ids := map[string]string{}
	for _, n := range found {
		k, hasMark := attrOf(n, mark)
		if _, hasID := attrOf(n, "id"); !hasMark || hasID {
			continue
		}
		// A mark in its text was put where the tokenizer read a tag that the parser reads as text.
		if text := textOf(n); !strings.Contains(text, mark) {
			if id := headingID(text); id != "" {
				id = free(taken, id)
				taken[id] = true
				ids[k] = id
			}
		}
	}
	if len(ids) == 0 {
		return page
	}
	return atHeadingTags(page, func(k int) string {
		if id, ok := ids[strconv.Itoa(k)]; ok {
			return ` id="` + html.EscapeString(id) + `"`
		}
		return ""
	})
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

// atHeadingTags returns page with attr(k) put in the k-th start tag of a heading, <h2/> too, which the
// parser reads as a start, after its name. What a tag left open at the end holds is kept.
// LIMIT: in <svg> the tokenizer reads as text what the parser does not (<title/>, <style/>, <textarea/>, CDATA): no id.
func atHeadingTags(page []byte, attr func(k int) string) []byte {
	var out bytes.Buffer
	k := 0
	z := html.NewTokenizer(bytes.NewReader(page))
	for {
		tt := z.Next()
		// Read before the next token, which reuses it; Token and TagName would lower the tag's name in it.
		raw := z.Raw()
		switch {
		case tt == html.ErrorToken:
			out.Write(raw)
			return out.Bytes()
		case (tt == html.StartTagToken || tt == html.SelfClosingTagToken) && headingTag(raw):
			out.Write(raw[:3])
			out.WriteString(attr(k))
			out.Write(raw[3:])
			k++
		default:
			out.Write(raw)
		}
	}
}

// headingTag reports whether raw, a start tag as written, is a heading's: its name, which ends at a space,
// a / or a >, is h1 to h6 in either case.
func headingTag(raw []byte) bool {
	return len(raw) > 3 && (raw[1] == 'h' || raw[1] == 'H') && raw[2] >= '1' && raw[2] <= '6' &&
		bytes.IndexByte([]byte(" \t\n\f\r/>"), raw[3]) >= 0
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
