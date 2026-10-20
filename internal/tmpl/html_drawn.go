package tmpl

import (
	"bytes"
	"cmp"
	"crypto/rand"
	"html/template"
	"slices"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// htmlItem is an item of format html of a data file: its path of keys, its text, and where it is written,
// which Read keeps for Render to tell a mistake of it at, as Render has no YAML.
type htmlItem struct {
	at   []string
	text string
	pos  yamlpos.Pos
	path string
}

// htmlItemsOf returns the items of format html of d, text but empty, which draws nothing, in the order of
// the file under root, those at one place (an alias) by their paths; nil for none.
func htmlItemsOf(d *Data, root ast.Node) []htmlItem {
	var found []itemAt
	d.schema.find("html", d.Values, nil, &found)
	var items []htmlItem
	for _, f := range found {
		text, isText := f.value.(string)
		if !isText || text == "" {
			continue
		}
		key, value, path := yamlpos.Find(root, f.at)
		items = append(items, htmlItem{
			at: f.at, text: text, pos: yamlpos.OfValue(key, value), path: path,
		})
	}
	slices.SortStableFunc(items, func(a, b htmlItem) int {
		return cmp.Or(cmp.Compare(a.pos.Line, b.pos.Line), cmp.Compare(a.pos.Column, b.pos.Column),
			cmp.Compare(pathKey(a.at), pathKey(b.at)))
	})
	return items
}

// htmlMarks marks the items of format html of a drawing that checks them: one shown is drawn as written,
// one not shown as nothing, or, with a mark, as the comment <!--mark:n-->, n its index in the items of the
// file (pathKey).
type htmlMarks struct {
	mark  string
	at    map[string]int
	shown func(n int) bool
}

// marked returns text, the item of format html at the path at, as m draws it; as it is if it is none of
// the items, an empty one, which a mark would make no longer empty to page.html.
func (m *htmlMarks) marked(text string, at []string) string {
	n, ok := m.at[pathKey(at)]
	switch {
	case !ok || m.shown(n):
		return text
	case m.mark == "":
		return ""
	default:
		return "<!--" + m.mark + ":" + strconv.Itoa(n) + "-->"
	}
}

// checkDrawnHTML refuses the first item of format html of d, in the order of the file, that changes the
// page around it, as a browser reads page, the page page drew, titled title, without its styles: one that
// leaves open what takes in the page after it (an <svg>, a comment, a <b> around the next item's text),
// ends where a tag starts, which the page after it goes on, or closes an element of the page. Whether it
// does depends on where page.html puts it, and on the items around it, so the page is compared with the
// page drawn with the item a mark, there the item read alone, as a fragment of what holds the mark. d is
// drawn as a copy, marked, so that d draws its page as it is.
// LIMIT: page.html gets the item a mark; one looking into an item's text (len, eq) may draw it otherwise.
// LIMIT: each item draws and reads the page again, every view: 400 items, 0.25s; keep the result if slow.
// LIMIT: an item read alone is put where its mark is, in what the item before left open (<li>a then <li>b).
func (d *Data) checkDrawnHTML(page *template.Template, title string, drawn []byte) error {
	want := shape(parseDrawn(drawn))
	c := d.marked(rand.Text(), nil)
	for k, it := range d.htmlItems {
		c.marks.shown = func(n int) bool { return n != k }
		bare, err := c.execute(page, title)
		if err != nil {
			return err
		}
		doc := parseDrawn(bare)
		inPlace(doc, c.marks.mark, k, it.text)
		if shape(doc) != want {
			return itemError(d.File, it.pos, it.path, usererr.New("tmpl_html_page"))
		}
	}
	return nil
}

// marked returns a copy of d whose items of format html are drawn as m draws them with mark and shown,
// nil for all, so that d draws its page as it is.
func (d *Data) marked(mark string, shown func(n int) bool) *Data {
	if shown == nil {
		shown = func(int) bool { return true }
	}
	m := &htmlMarks{mark: mark, at: map[string]int{}, shown: shown}
	for i, it := range d.htmlItems {
		m.at[pathKey(it.at)] = i
	}
	c := *d
	c.marks = m
	return &c
}

// headHider returns the index of the first item of format html of d, in the order of the file, with which
// those before it hide the </head> of the page page draws, titled title, which the page drawn without them
// has: an open <script> or tag of one does. -1 for none: the page has no </head> without them either.
func (d *Data) headHider(page *template.Template, title string) int {
	for k := -1; k < len(d.htmlItems); k++ {
		out, err := d.marked("", func(n int) bool { return n <= k }).execute(page, title)
		hidden := err != nil || tagsOf(tagsIn(string(out), false)).head < 0
		switch {
		case hidden && k < 0:
			return -1
		case hidden:
			return k
		}
	}
	return -1
}

// parseDrawn reads page as a browser does, with a script after it, as floe puts the diagrams' and a served
// page's there, which an item may take in.
func parseDrawn(page []byte) *html.Node {
	// A bytes.Reader fails no read, the one error Parse returns.
	doc, _ := html.Parse(bytes.NewReader(append(page, "<script></script>"...))) //nolint:errcheck // see above
	return doc
}

// inPlace puts item, read alone, in doc where the marks of the item n of mark are: in place of each, the
// nodes of item read as a fragment of what holds the mark, a <body> for the <html>, where what is not of
// the head starts one; and in text that holds a mark as text (a <noscript>, a <title>), item as text.
func inPlace(doc *html.Node, mark string, n int, item string) {
	want := mark + ":" + strconv.Itoa(n)
	var marks []*html.Node
	for c := range doc.Descendants() {
		switch {
		case c.Type == html.CommentNode && c.Data == want:
			marks = append(marks, c)
		case c.Type == html.TextNode:
			c.Data = strings.ReplaceAll(c.Data, "<!--"+want+"-->", item)
		}
	}
	for _, m := range marks {
		context := m.Parent
		if context.Type != html.ElementNode || context.DataAtom == atom.Html {
			context = &html.Node{
				Parent: nil, FirstChild: nil, LastChild: nil, PrevSibling: nil, NextSibling: nil,
				Type: html.ElementNode, DataAtom: atom.Body, Data: "body", Namespace: "", Attr: nil,
			}
		}
		// A strings.Reader fails no read, the one error ParseFragment returns.
		nodes, _ := html.ParseFragment(strings.NewReader(item), context) //nolint:errcheck // see above
		for _, c := range nodes {
			m.Parent.InsertBefore(c, m)
		}
		m.Parent.RemoveChild(m)
	}
}

// openingOf writes the element n by its name and the names of its attributes; one the parser may make
// itself (implied) by its attributes alone, which one it makes has none.
func openingOf(n *html.Node) string {
	keys := make([]string, 0, len(n.Attr))
	for _, a := range n.Attr {
		keys = append(keys, a.Namespace+":"+a.Key)
	}
	slices.Sort(keys)
	switch {
	case implied(n) && len(keys) == 0:
		return ""
	case implied(n):
		return "@" + n.Data + " " + strings.Join(keys, " ") + "@"
	default:
		return "<" + cmp.Or(n.Namespace, "html") + ":" + n.Data + " " + strings.Join(keys, " ") + ">"
	}
}

// implied reports whether n is an element of HTML the parser may make itself, around what comes in it:
// where it is made, around an item's text or the page's after it, is the parser's, not the page's. The
// head is not: what an item puts in it that ends it (a <div>) changes the page.
func implied(n *html.Node) bool {
	switch n.DataAtom {
	case atom.Html, atom.Body, atom.Tbody, atom.Tr, atom.Colgroup:
		return n.Namespace == ""
	default:
		return false
	}
}

// shape writes the elements, comments and texts of n in order, by their names, the names of the elements'
// attributes, and the texts' words, those next to each other as one, as an item's text and the page's
// after it are in the page drawn. An item that gives the page's <body> an attribute (<body hidden>) changes
// the page, and so does one that takes in the page's text (x </). The attributes' values are left out,
// which an item drawn in a value is in, read or not, and so are the elements the parser may make itself
// (implied), but their attributes.
func shape(n *html.Node) string {
	var b strings.Builder
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			b.WriteString("#text(" + strconv.Quote(text.String()) + ")")
			text.Reset()
		}
	}
	var write func(n *html.Node)
	write = func(n *html.Node) {
		if n.Type != html.TextNode {
			flush()
		}
		switch n.Type {
		case html.ElementNode:
			b.WriteString(openingOf(n))
		case html.TextNode:
			text.WriteString(n.Data)
		case html.CommentNode:
			b.WriteString("#comment")
		default:
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			write(c)
		}
		if n.Type == html.ElementNode && !implied(n) {
			flush()
			b.WriteString("</>")
		}
	}
	write(n)
	flush()
	return b.String()
}
