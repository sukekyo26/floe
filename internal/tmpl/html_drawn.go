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

// htmlItem is an item of format html of a data file: its path of keys, and where it is written, which
// Read keeps for Render to tell a mistake of it at, as Render has no YAML.
type htmlItem struct {
	at   []string
	pos  yamlpos.Pos
	path string
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
		key, value, path := yamlpos.Find(root, f.at)
		items = append(items, htmlItem{at: f.at, pos: yamlpos.OfValue(key, value), path: path})
	}
	slices.SortFunc(items, func(a, b htmlItem) int {
		return cmp.Or(cmp.Compare(a.pos.Line, b.pos.Line), cmp.Compare(a.pos.Column, b.pos.Column))
	})
	return items
}

// htmlMarks marks the items of format html of a drawing that checks them: each between the comments
// <!--mark:n--> and <!--/mark:n-->, n its index in the items of the file (pathKey). The item only is drawn
// as written and the item empty as nothing, -1 for none, and the others as a letter, which a <b> one leaves
// open would be around.
type htmlMarks struct {
	mark        string
	at          map[string]int
	only, empty int
}

// marked returns text, the item of format html at the path at, between its marks, as m draws it.
func (m *htmlMarks) marked(text string, at []string) string {
	n := m.at[pathKey(at)]
	switch n {
	case m.only:
	case m.empty:
		text = ""
	default:
		text = "z"
	}
	i := strconv.Itoa(n)
	return "<!--" + m.mark + ":" + i + "-->" + text + "<!--/" + m.mark + ":" + i + "-->"
}

// checkDrawnHTML refuses the first item of format html of d, in the order of the file, that changes the
// page around it, as a browser reads the page page draws, titled title: one that leaves open what takes
// in the page after it (an <svg>, a comment, a <b> around the next item's text), or closes an element of
// the page. Whether it does depends on where page.html puts it, so each item is drawn as written, the
// others as a letter, and the page, without the item, is compared with the page drawn with it empty: a
// letter would end the page's <head> itself. d is drawn as a copy, marked, so that d draws its page as it
// is.
// LIMIT: page.html is given the items marked, so one that looks into an item's text (len, eq) may draw
// them otherwise than the page it checks. The page is drawn twice more per item, every time; draw them
// all at once first if pages of many items come to be slow.
func (d *Data) checkDrawnHTML(page *template.Template, title string) error {
	c := d.marked()
	m := c.marks
	for k, it := range d.htmlItems {
		m.only, m.empty = -1, k
		bare, err := c.drawnPage(page, title)
		if err != nil {
			return err
		}
		m.only, m.empty = k, -1
		full, err := c.drawnPage(page, title)
		if err != nil {
			return err
		}
		if why := changed(full, bare, m.mark, strconv.Itoa(k)); why != nil {
			return itemError(d.File, it.pos, it.path, why)
		}
	}
	return nil
}

// marked returns a copy of d whose items of format html are marked, all of them as letters, so that d
// draws its page as it is.
func (d *Data) marked() *Data {
	m := &htmlMarks{mark: rand.Text(), at: map[string]int{}, only: -1, empty: -1}
	for i, it := range d.htmlItems {
		m.at[pathKey(it.at)] = i
	}
	c := *d
	c.marks = m
	return &c
}

// hidesHead reports whether d's items of format html hide the </head> of the page page draws, titled
// title, which the page drawn with them as letters has: an open <script> of one does.
func (d *Data) hidesHead(page *template.Template, title string) bool {
	out, err := d.marked().execute(page, title)
	return err == nil && tagsOf(tagsIn(string(out), false)).head >= 0
}

// drawnPage draws d, as it is marked, with page: without its styles, and with a script after it, as floe
// puts the diagrams' and a served page's there, which an item may take in.
func (d *Data) drawnPage(page *template.Template, title string) ([]byte, error) {
	out, err := d.execute(page, title)
	if err != nil {
		return nil, err
	}
	return append(out, "<script></script>"...), nil
}

// changed returns the mistake of the item n of full, a page drawn with it as written, against bare, the
// page drawn with it a letter, read as a browser reads them, nil when it leaves the page as it is: its end
// mark gone, taken in by what it leaves open; one of its elements holding the page after it; or the page
// without it not that of bare without it, by shape. An item drawn as no markup, in a <title> or a value,
// has no marks, and is left alone; one whose start mark is text of the page's that it ends (a <noscript>)
// leaves its end mark in the page, unlike bare.
func changed(full, bare []byte, mark, n string) *usererr.Error {
	// A bytes.Reader fails no read, the one error Parse returns.
	doc, _ := html.Parse(bytes.NewReader(full))     //nolint:errcheck // see above
	without, _ := html.Parse(bytes.NewReader(bare)) //nolint:errcheck // see above
	begins, ends := marksIn(doc, mark, n)
	switch {
	case len(ends) > len(begins):
		return usererr.New("tmpl_html_page") // a start mark is text of the page's that it ends
	case len(ends) < len(begins):
		return swallowed(doc, mark, n)
	}
	for i := range begins {
		if why := cut(doc, begins[i], ends[i]); why != nil {
			return why
		}
	}
	begins, ends = marksIn(without, mark, n)
	for i := range min(len(begins), len(ends)) {
		cut(without, begins[i], ends[i]) //nolint:errcheck // a letter holds no page
	}
	if shape(doc) != shape(without) {
		return usererr.New("tmpl_html_page")
	}
	return nil
}

// markRe reads a mark of an item: / for its end, and its index.
var markRe = regexp.MustCompile(`^(/?)([A-Z0-9]+):(\d+)$`)

// marksIn returns the comments of doc that mark the item n, its starts and its ends, in order.
func marksIn(doc *html.Node, mark, n string) (begins, ends []*html.Node) {
	for c := range doc.Descendants() {
		if c.Type != html.CommentNode {
			continue
		}
		if m := markRe.FindStringSubmatch(c.Data); m != nil && m[2] == mark && m[3] == n {
			if m[1] == "" {
				begins = append(begins, c)
			} else {
				ends = append(ends, c)
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
