package tmpl

import (
	"bytes"
	"cmp"
	"crypto/rand"
	"html/template"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// htmlItem is an item of format html of a data file: its path of keys, where it is written, and the
// mistake of it at its line and column, which Read keeps for Render, which has no YAML.
type htmlItem struct {
	at      []string
	pos     yamlpos.Pos
	mistake func(e *usererr.Error) *yamlpos.Error
}

// htmlItemsOf returns the items of format html of d, text, in the order of the file under root; nil for
// none.
func htmlItemsOf(d *Data, root ast.Node) []htmlItem {
	var found []itemAt
	d.schema.find("html", d.Values, nil, &found)
	var items []htmlItem
	for _, f := range found {
		if _, isText := f.value.(string); !isText {
			continue
		}
		key, value, _ := yamlpos.Find(root, f.at)
		mistake := func(e *usererr.Error) *yamlpos.Error { return itemMistake(d.File, root, f.at, false, e) }
		items = append(items, htmlItem{at: f.at, pos: yamlpos.OfValue(key, value), mistake: mistake})
	}
	slices.SortFunc(items, func(a, b htmlItem) int {
		return cmp.Or(cmp.Compare(a.pos.Line, b.pos.Line), cmp.Compare(a.pos.Column, b.pos.Column))
	})
	return items
}

// htmlMarks marks the items of format html of a drawing that checks them: each between the comments
// <!--mark:n--> and <!--/mark:n-->, n its index in the items of the file (pathKey), its text left out
// when empty, or when n is past upTo.
type htmlMarks struct {
	mark  string
	at    map[string]int
	empty bool
	upTo  int
}

// marked returns text, the item of format html at the path at, between its marks, as m draws it.
func (m *htmlMarks) marked(text string, at []string) string {
	n := m.at[pathKey(at)]
	if m.empty || n > m.upTo {
		text = ""
	}
	i := strconv.Itoa(n)
	return "<!--" + m.mark + ":" + i + "-->" + text + "<!--/" + m.mark + ":" + i + "-->"
}

// checkDrawnHTML refuses the first item of format html of d that changes the page around it, as a
// browser reads the page page draws, titled title: one that leaves open what takes in the page after it
// (an <svg>, a comment, a <b>), or closes an element of the page. Whether it does depends on where
// page.html puts it, so the page is drawn with its items marked and with them empty, and the two are
// compared. When they change it, the item told is the first, in the order of the file, with which the
// items before it do: two may do so only together, one closing what the other leaves open, which the
// page with all of them shows, as the page between them is in an element of the first.
// LIMIT: page.html is given the items marked, so one that looks into an item's text (len, eq) may draw
// them otherwise than the page it checks. Finding the item draws the page once more per item, but only
// when they change it; halve the items drawn if pages of many come to be slow.
func (d *Data) checkDrawnHTML(page *template.Template, title string) error {
	m := &htmlMarks{mark: rand.Text(), at: map[string]int{}, empty: true, upTo: len(d.htmlItems) - 1}
	for i, it := range d.htmlItems {
		m.at[pathKey(it.at)] = i
	}
	d.marks = m
	defer func() { d.marks = nil }()
	bare, err := d.drawnTree(page, title)
	if err != nil {
		return err
	}
	m.empty = false
	full, err := d.drawnTree(page, title)
	if err != nil || changed(full, bare, m.mark) == nil {
		return err
	}
	for k := range d.htmlItems {
		m.upTo = k
		if full, err = d.drawnTree(page, title); err != nil {
			return err
		}
		if why := changed(full, bare, m.mark); why != nil {
			return d.htmlItems[k].mistake(why)
		}
	}
	return nil
}

// drawnTree draws d, as it is marked, with page, and reads it as a browser does: without its styles, and
// with a script after it, as floe puts the diagrams' and a served page's there, which an item may take in.
func (d *Data) drawnTree(page *template.Template, title string) (*html.Node, error) {
	out, err := d.execute(page, title)
	if err != nil {
		return nil, err
	}
	// A bytes.Reader fails no read, the one error Parse returns.
	doc, _ := html.Parse(bytes.NewReader(append(out, "<script></script>"...))) //nolint:errcheck // see above
	return doc, nil
}

// changed returns the mistake of the items marked in full, the page drawn with them, against bare, the
// page drawn with them empty, nil when they leave it as it is: an item whose end mark is gone, taken in by
// what it leaves open, one of whose elements holds the page after it, or the page outside the items not
// that of bare, by shape. An item drawn as no markup, in a <title> or a value, has no marks, and is left
// alone; one whose start mark is text of the page's that it ends (a <noscript>) leaves its end mark in the
// page, unlike bare.
func changed(full, bare *html.Node, mark string) *usererr.Error {
	begins, ends := marksIn(full, mark)
	for n, b := range begins {
		e := ends[n]
		switch {
		case len(e) > len(b):
			return usererr.New("tmpl_html_page") // a start mark is text of the page's that it ends
		case len(e) < len(b):
			return swallowed(full, mark, n)
		}
		for i := range b {
			if why := cut(full, b[i], e[i]); why != nil {
				return why
			}
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
		case c.Type == html.CommentNode && c.Data != end && holdsMark(c.Data, end):
			return usererr.New("tmpl_html_open", "<!--")
		case c.Type == html.TextNode && holdsMark(c.Data, end) && c.Parent != nil &&
			c.Parent.Type == html.ElementNode:
			return usererr.New("tmpl_html_open", "<"+c.Parent.Data+">")
		}
	}
	return usererr.New("tmpl_html_tag")
}

// holdsMark reports whether text holds the mark end, its index whole: /M:1 is not in /M:10.
func holdsMark(text, end string) bool {
	for at := strings.Index(text, end); at >= 0; at = strings.Index(text, end) {
		text = text[at+len(end):]
		if text == "" || text[0] < '0' || text[0] > '9' {
			return true
		}
	}
	return false
}

// shape writes the elements, comments and texts of n in order, by their names, and the names of the
// elements' attributes: an item that gives the page's <body> one (<body hidden>) changes the page. The
// texts' words and the attributes' values are left out, which an item drawn as text or in a value changes.
func shape(n *html.Node) string {
	var b strings.Builder
	var write func(n *html.Node)
	write = func(n *html.Node) {
		switch n.Type {
		case html.ElementNode:
			keys := make([]string, 0, len(n.Attr))
			for _, a := range n.Attr {
				keys = append(keys, a.Namespace+":"+a.Key)
			}
			slices.Sort(keys)
			b.WriteString("<" + cmp.Or(n.Namespace, "html") + ":" + n.Data + " " + strings.Join(keys, " ") + ">")
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
