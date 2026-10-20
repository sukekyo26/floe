package tmpl

import (
	"bytes"
	"cmp"
	"crypto/rand"
	"html/template"
	"maps"
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

// htmlItem is an item of format html of a data file: its path of keys, its text, and where it is written,
// which Read keeps for Render to tell a mistake of it at, as Render has no YAML.
type htmlItem struct {
	at      []string
	text    string
	written map[string]bool // the elements it opens by a tag (writtenIn)
	pos     yamlpos.Pos
	path    string
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
			at: f.at, text: text, written: writtenIn(text), pos: yamlpos.OfValue(key, value), path: path,
		})
	}
	slices.SortStableFunc(items, func(a, b htmlItem) int {
		return cmp.Or(cmp.Compare(a.pos.Line, b.pos.Line), cmp.Compare(a.pos.Column, b.pos.Column),
			cmp.Compare(pathKey(a.at), pathKey(b.at)))
	})
	return items
}

// htmlMarks marks the items of format html of a drawing that checks them: each between the comments
// <!--mark:n--> and <!--/mark:n-->, n its index in the items of the file (pathKey), as written but the
// item empty, -1 for none, or all of them with none.
type htmlMarks struct {
	mark  string
	at    map[string]int
	empty int
	none  bool
}

// marked returns text, the item of format html at the path at, between its marks, as m draws it; as it
// is if it is none of the items, an empty one, whose marks would make it no longer empty to page.html.
func (m *htmlMarks) marked(text string, at []string) string {
	n, ok := m.at[pathKey(at)]
	if !ok {
		return text
	}
	if m.none || n == m.empty {
		text = ""
	}
	i := strconv.Itoa(n)
	return "<!--" + m.mark + ":" + i + "-->" + text + "<!--/" + m.mark + ":" + i + "-->"
}

// checkDrawnHTML refuses the first item of format html of d, in the order of the file, that changes the
// page around it, as a browser reads the page page draws, titled title: one that leaves open what takes
// in the page after it (an <svg>, a comment, a <b> around the next item's text), or closes an element of
// the page. Whether it does depends on where page.html puts it, and on the items around it, so the page
// drawn with them all is compared, without the item, with the page drawn with it alone empty. d is drawn
// as a copy, marked, so that d draws its page as it is.
// LIMIT: page.html gets the items marked; one looking into an item's text (len, eq) may draw them otherwise.
// LIMIT: each item draws the page once more, every time; check once per data and title if pages get slow.
func (d *Data) checkDrawnHTML(page *template.Template, title string) error {
	c := d.marked()
	full, err := c.drawnPage(page, title)
	if err != nil {
		return err
	}
	for k, it := range d.htmlItems {
		c.marks.empty = k
		bare, err := c.drawnPage(page, title)
		if err != nil {
			return err
		}
		if why := changed(full, bare, c.marks.mark, strconv.Itoa(k), it.written); why != nil {
			return itemError(d.File, it.pos, it.path, why)
		}
	}
	return nil
}

// marked returns a copy of d whose items of format html are marked, as written, so that d draws its page
// as it is.
func (d *Data) marked() *Data {
	m := &htmlMarks{mark: rand.Text(), at: map[string]int{}, empty: -1, none: false}
	for i, it := range d.htmlItems {
		m.at[pathKey(it.at)] = i
	}
	c := *d
	c.marks = m
	return &c
}

// hidesHead reports whether d's items of format html hide the </head> of the page page draws, titled
// title, which the page drawn with them empty has: an open <script> of one does.
func (d *Data) hidesHead(page *template.Template, title string) bool {
	c := d.marked()
	c.marks.none = true
	out, err := c.execute(page, title)
	return err == nil && tagsOf(tagsIn(string(out), false)).head >= 0
}

// writtenIn returns the names of the elements item opens by a tag, lower case: the others an element of it
// is the parser's, as a <tbody> around a <tr>.
func writtenIn(item string) map[string]bool {
	written := map[string]bool{}
	z := html.NewTokenizer(strings.NewReader(item))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return written
		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := z.TagName()
			written[string(name)] = true
		default:
		}
	}
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
// page drawn with it empty, read as a browser reads them, written the elements it opens by a tag, nil when
// it leaves the page as it is: its end mark gone, taken in by what it leaves open; one of its elements
// holding the page after it; or the page without it not bare, by shape. An item drawn as no markup, in a
// <title> or a value, has no marks, and is left alone; one whose start mark is text of the page's that it
// ends (a <noscript>) leaves its end mark in the page, unlike bare.
func changed(full, bare []byte, mark, n string, written map[string]bool) *usererr.Error {
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
	others := othersIn(doc, mark, n)
	for i := range begins {
		if why := cut(doc, begins[i], ends[i], written, others); why != nil {
			return why
		}
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
// between them, which are all where begin is, or out of what other items, of others, left open around it:
// one elsewhere closed an element of the page around the item (</td><td>). An element the parser made for
// it, not of written, gives back to the page what it holds of the page, as a <tbody> around a <tr> does
// the page's spaces, and so does one of its own what it holds of comments and spaces, which draw nothing:
// the marks of the next item before a <li> of it closes this one's. It returns the mistake of an item one
// of whose own elements holds any other node of the page.
func cut(doc, begin, end *html.Node, written map[string]bool, others map[*html.Node]bool) *usererr.Error {
	item, in := between(doc, begin, end)
	if why := outOfPlace(item, in, others, begin); why != nil {
		return why
	}
	for _, n := range item {
		if n.Type == html.ElementNode && written[strings.ToLower(n.Data)] && holdsPage(n, in, end) {
			return usererr.New("tmpl_html_open", "<"+n.Data+">")
		}
	}
	end.Parent.RemoveChild(end)
	begin.Parent.InsertBefore(end, begin.NextSibling)
	// The innermost first, so that what an element gives back is given back again by the one around it.
	for _, n := range slices.Backward(item) {
		giveBack(n, in)
	}
	for _, n := range item {
		if n.Parent != nil && !in[n.Parent] {
			n.Parent.RemoveChild(n)
		}
	}
	return nil
}

// outOfPlace returns the mistake of the nodes of an item, item and as the set in, of which one is not where
// begin is, nor out of what other items, of others, left open around it, which a <li> of it closes: it
// closed the page's element around it. nil for none.
func outOfPlace(item []*html.Node, in, others map[*html.Node]bool, begin *html.Node) *usererr.Error {
	place := map[*html.Node]bool{}
	for p := begin.Parent; p != nil; p = p.Parent {
		place[p] = true
		if !others[p] {
			break
		}
	}
	for _, n := range item {
		if !in[n.Parent] && !place[n.Parent] {
			return usererr.New("tmpl_html_page")
		}
	}
	return nil
}

// holdsPage reports whether n, an element of an item of the set in, holds a node of the page that draws,
// not end, the item's end mark, nor a comment or spaces.
func holdsPage(n *html.Node, in map[*html.Node]bool, end *html.Node) bool {
	for c := range n.Descendants() {
		if !in[c] && c != end && !blank(c) {
			return true
		}
	}
	return false
}

// othersIn returns the nodes of doc of the items marked but the item n.
func othersIn(doc *html.Node, mark, n string) map[*html.Node]bool {
	others := map[*html.Node]bool{}
	begins, ends := map[string][]*html.Node{}, map[string][]*html.Node{}
	for c := range doc.Descendants() {
		if c.Type != html.CommentNode {
			continue
		}
		if m := markRe.FindStringSubmatch(c.Data); m != nil && m[2] == mark && m[3] != n {
			if m[1] == "" {
				begins[m[3]] = append(begins[m[3]], c)
			} else {
				ends[m[3]] = append(ends[m[3]], c)
			}
		}
	}
	for j, b := range begins {
		for i := range min(len(b), len(ends[j])) {
			_, in := between(doc, b[i], ends[j][i])
			maps.Copy(others, in)
		}
	}
	return others
}

// between returns the nodes of doc that start between begin and end, in order, and as a set.
func between(doc, begin, end *html.Node) ([]*html.Node, map[*html.Node]bool) {
	var item []*html.Node
	in := map[*html.Node]bool{}
	started := false
	for n := range doc.Descendants() {
		if n == end {
			break
		}
		if started {
			item = append(item, n)
			in[n] = true
		}
		started = started || n == begin
	}
	return item, in
}

// blank reports whether n draws nothing: a comment, or a text of spaces.
func blank(n *html.Node) bool {
	return n.Type == html.CommentNode || n.Type == html.TextNode && strings.TrimSpace(n.Data) == ""
}

// giveBack puts the children of n, an element of an item, that are not of the item, of in, before it.
func giveBack(n *html.Node, in map[*html.Node]bool) {
	if n.Type != html.ElementNode {
		return
	}
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		if !in[c] {
			n.RemoveChild(c)
			n.Parent.InsertBefore(c, n)
		}
		c = next
	}
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

// implied reports whether n is an element of HTML the parser may make itself, around what comes in it:
// where it is made, around an item's text or the page's after it, is the parser's, not the page's.
func implied(n *html.Node) bool {
	switch n.DataAtom {
	case atom.Html, atom.Head, atom.Body, atom.Tbody, atom.Tr, atom.Colgroup:
		return n.Namespace == ""
	default:
		return false
	}
}

// shape writes the elements, comments and texts of n in order, by their names, and the names of the
// elements' attributes: an item that gives the page's <body> one (<body hidden>) changes the page. The
// texts' words and the attributes' values are left out, which an item drawn as text or in a value changes,
// and so are the elements the parser may make itself (implied), but their attributes: cut keeps an item in
// its place, so that where one starts is the parser's.
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
			if implied(n) {
				if len(keys) > 0 { // one the parser makes has none
					b.WriteString("@" + n.Data + " " + strings.Join(keys, " ") + "@")
				}
				break
			}
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
		if n.Type == html.ElementNode && !implied(n) {
			b.WriteString("</>")
		}
	}
	write(n)
	return b.String()
}
