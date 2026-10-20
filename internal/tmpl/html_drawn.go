package tmpl

import (
	"bytes"
	"cmp"
	"crypto/rand"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/sukekyo26/floe/internal/usererr"
)

// htmlMarks marks the items of format html of a drawing that checks them: each between the comments
// <!--mark:n--> and <!--/mark:n-->, n its index by its path of keys (pathKey), its text left out when
// empty, or when only is another's, -1 for none.
type htmlMarks struct {
	mark  string
	at    map[string]int
	empty bool
	only  int
}

// marked returns text, the item of format html at the path at, between its marks, as m draws it.
func (m *htmlMarks) marked(text string, at []string) string {
	n := m.at[pathKey(at)]
	if m.empty || m.only >= 0 && m.only != n {
		text = ""
	}
	i := strconv.Itoa(n)
	return "<!--" + m.mark + ":" + i + "-->" + text + "<!--/" + m.mark + ":" + i + "-->"
}

// checkDrawnHTML refuses the first item of format html of d that changes the page around it, at its line
// and column under root, as a browser reads the page drawn: one that leaves open what takes in the page
// after it (an <svg>, a comment, a <b>), or closes an element of the page. Whether it does depends on
// where page.html puts it, so d is drawn, with its items marked and with them empty, and the two are
// compared. A drawing that fails is left to Render to tell.
// LIMIT: finding the item draws d once more per item, but only when one changes the page; halve the items
// left in each drawing if pages of many items come to be slow.
func checkDrawnHTML(d *Data, root ast.Node) error {
	var found []itemAt
	d.schema.find("html", d.Values, nil, &found)
	found = slices.DeleteFunc(found, func(f itemAt) bool {
		_, isText := f.value.(string)
		return !isText
	})
	if found == nil {
		return nil
	}
	m := &htmlMarks{mark: rand.Text(), at: map[string]int{}, empty: true, only: -1}
	for i, f := range found {
		m.at[pathKey(f.at)] = i
	}
	d.marks = m
	defer func() { d.marks = nil }()
	bare, ok := d.drawnTree()
	if !ok {
		return nil
	}
	m.empty = false
	full, ok := d.drawnTree()
	if !ok {
		return nil
	}
	why := changed(full, bare, m.mark)
	if why == nil {
		return nil
	}
	for i := range found {
		m.only = i
		page, ok := d.drawnTree()
		if !ok {
			return nil
		}
		if one := changed(page, bare, m.mark); one != nil {
			return itemMistake(d.File, root, found[i].at, false, one)
		}
	}
	// Items that change it only together, one closing what another leaves open: told at the first.
	return itemMistake(d.File, root, found[0].at, false, why)
}

// drawnTree draws d as it is marked, without its styles, and reads the page as a browser does, false for
// a drawing that fails. A script ends it, as the one a served page gets: an item that takes it in changes
// the page too.
func (d *Data) drawnTree() (*html.Node, bool) {
	page, err := d.Render(d.Title)
	if err != nil {
		return nil, false
	}
	// A bytes.Reader fails no read, the one error Parse returns.
	doc, _ := html.Parse(bytes.NewReader(append(page, "<script></script>"...))) //nolint:errcheck // see above
	return doc, true
}

// changed returns the mistake of the items marked in full, the page drawn with them, against bare, the
// page drawn with them empty, nil when they leave it as it is: an item whose end mark is gone, taken in by
// what it leaves open, one of whose elements holds the page after it, or the page outside the items not
// that of bare, whose text and attributes, which an item drawn as text or in a value changes, are not
// compared. An item drawn as no markup, in a <title> or a value, has no marks, and is left alone.
func changed(full, bare *html.Node, mark string) *usererr.Error {
	begins, ends := marksIn(full, mark)
	for n, b := range begins {
		e := ends[n]
		if len(e) != len(b) {
			return swallowed(full, mark, n)
		}
		for i := range b {
			if why := cut(full, b[i], e[i]); why != nil {
				return why
			}
		}
	}
	for n, e := range ends {
		if len(begins[n]) != len(e) {
			return swallowed(full, mark, n)
		}
	}
	if shape(full) != shape(bare) {
		return usererr.New("tmpl_html_page")
	}
	return nil
}

// markRe reads a mark of an item: / for its end, and its index.
var markRe = regexp.MustCompile(`^(/?)([A-Z0-9]+):(\d+)$`)

// marksIn returns the comments of doc that mark the items, the starts and the ends by index, in order.
func marksIn(doc *html.Node, mark string) (begins, ends map[string][]*html.Node) {
	begins, ends = map[string][]*html.Node{}, map[string][]*html.Node{}
	for n := range doc.Descendants() {
		if n.Type != html.CommentNode {
			continue
		}
		if m := markRe.FindStringSubmatch(n.Data); m != nil && m[2] == mark {
			if m[1] == "" {
				begins[m[3]] = append(begins[m[3]], n)
			} else {
				ends[m[3]] = append(ends[m[3]], n)
			}
		}
	}
	return begins, ends
}

// cut takes the item between begin and end out of doc, its end mark put after begin: the nodes that start
// between them. It returns the mistake of an item one of whose elements holds a node of the page after it.
func cut(doc, begin, end *html.Node) *usererr.Error {
	var item []*html.Node
	in := map[*html.Node]bool{}
	started := false
	for n := range doc.Descendants() {
		switch {
		case n == begin:
			started = true
		case n == end:
			started = false
		case started:
			item = append(item, n)
			in[n] = true
		}
		if n == end {
			break
		}
	}
	// The outermost first: the order of the page.
	for _, n := range item {
		for c := range n.Descendants() {
			if in[c] || c == end {
				continue
			}
			if n.DataAtom == atom.Body {
				return usererr.New("tmpl_html_page") // what it draws in the page's <head> ends it
			}
			return usererr.New("tmpl_html_open", "<"+n.Data+">")
		}
	}
	end.Parent.RemoveChild(end)
	begin.Parent.InsertBefore(end, begin.NextSibling)
	for _, n := range item {
		if n.Parent != nil && !in[n.Parent] {
			n.Parent.RemoveChild(n)
		}
	}
	return nil
}

// swallowed returns the mistake of the item n whose end mark is no comment: taken in by a comment, by an
// element whose text runs to its end tag (<textarea>), or by a tag or a quote it ends in.
func swallowed(doc *html.Node, mark, n string) *usererr.Error {
	end := "/" + mark + ":" + n
	for c := range doc.Descendants() {
		switch {
		case c.Type == html.CommentNode && strings.Contains(c.Data, end):
			return usererr.New("tmpl_html_open", "<!--")
		case c.Type == html.TextNode && strings.Contains(c.Data, end) && c.Parent != nil &&
			c.Parent.Type == html.ElementNode:
			return usererr.New("tmpl_html_open", "<"+c.Parent.Data+">")
		}
	}
	return usererr.New("tmpl_html_tag")
}

// shape writes the elements, comments and texts of n in order, by their names, without the texts'
// words or the elements' attributes.
func shape(n *html.Node) string {
	var b strings.Builder
	var write func(n *html.Node)
	write = func(n *html.Node) {
		switch n.Type {
		case html.ElementNode:
			b.WriteString("<" + cmp.Or(n.Namespace, "html") + ":" + n.Data + ">")
		case html.TextNode:
			b.WriteString("#text")
		case html.CommentNode:
			b.WriteString("#comment")
		default:
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			write(c)
		}
		if n.Type == html.ElementNode {
			b.WriteString("</>")
		}
	}
	write(n)
	return b.String()
}
