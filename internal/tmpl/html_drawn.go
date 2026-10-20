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
		pos, path := itemPos(root, f.at, false)
		items = append(items, htmlItem{at: f.at, text: text, pos: pos, path: path})
	}
	slices.SortStableFunc(items, func(a, b htmlItem) int {
		return cmp.Or(cmp.Compare(a.pos.Line, b.pos.Line), cmp.Compare(a.pos.Column, b.pos.Column),
			cmp.Compare(pathKey(a.at), pathKey(b.at)))
	})
	return items
}

// htmlMarks marks the items of format html of a drawing that checks them: one shown is drawn as written,
// one not shown as the comment <!--mark:n-->, n its index in the items of the file (pathKey). markdown
// keeps the HTML of the items of format markdown, which each drawing draws again.
type htmlMarks struct {
	mark     string
	at       map[string]int
	shown    func(n int) bool
	markdown map[string]template.HTML
}

// marked returns text, the item of format html at the path at, as m draws it; as it is if it is none of
// the items, an empty one, which a mark would make no longer empty to page.html.
func (m *htmlMarks) marked(text string, at []string) string {
	if n, ok := m.at[pathKey(at)]; ok && !m.shown(n) {
		return m.comment(n)
	}
	return text
}

// comment returns the mark of the item n.
func (m *htmlMarks) comment(n int) string {
	return "<!--" + m.mark + ":" + strconv.Itoa(n) + "-->"
}

// drawnOrder returns the items whose marks page, drawn with every item a mark, holds as they are, in the
// order page draws them. An item escaped there (in a value, a <title>, a <script>) or not drawn is no
// markup of the page's and changes none.
func (m *htmlMarks) drawnOrder(page []byte) []int {
	where := make([]int, len(m.at))
	order := make([]int, 0, len(m.at))
	for n := range where {
		where[n] = bytes.Index(page, []byte(m.comment(n)))
		if where[n] >= 0 {
			order = append(order, n)
		}
	}
	slices.SortFunc(order, func(a, b int) int { return cmp.Compare(where[a], where[b]) })
	return order
}

// checkDrawnHTML refuses the first item of format html of d, in the order the page draws them, that changes
// the page around it, as a browser reads page, the page page drew, titled title, without its styles: one that
// leaves open what takes in the page after it (an <svg>, a comment, a <b> around the next item's text),
// ends where a tag starts, which the page after it goes on, or closes an element of the page. Whether it
// does depends on where page.html puts it, and on the items around it, so the page is compared with the
// page drawn with the item a mark, there the item read alone, as a fragment of what holds the mark. The
// items before it in the page change none, so that one that does is told, not the one after it. One that
// leaves a <form> open, which the tree does not show, is refused too: the page's inputs after it would be
// sent where the item says. d is drawn as a copy, marked, so that d draws its page as it is. An item is
// checked in the <head> or the <body>: page.html putting one outside them is refused, as is one looking
// into an item (slice, index), whose drawing with a mark fails.
// LIMIT: a branch of page.html on an item's text (eq, len) is checked as the mark takes it; refuse it if seen.
// LIMIT: each item draws and reads the page again, every view (400 items, 0.25s); keep the result if slow.
// LIMIT: an item left open for the next to close (<li>a, <li>b) is refused; read them together if pages need it.
// LIMIT: a comment mark rebuilds no <b> page.html left open (<p><b>x</p>); mark with text if pages need it.
func (d *Data) checkDrawnHTML(page *template.Template, title string, drawn []byte) error {
	want := shape(parseDrawn(drawn))
	c := d.marked(rand.Text())
	c.marks.shown = func(int) bool { return false }
	all, err := c.drawMarked(page, title)
	if err != nil {
		return err
	}
	for _, k := range c.marks.drawnOrder(all) {
		it := d.htmlItems[k]
		c.marks.shown = func(n int) bool { return n != k }
		bare, err := c.drawMarked(page, title)
		if err != nil {
			return err
		}
		doc := parseDrawn(bare)
		formIn, outside := inPlace(doc, c.marks.mark, k, it.text)
		switch {
		case outside:
			return usererr.New("tmpl_html_outside", d.tmpl.file("page.html"))
		case formIn && leavesForm(it.text) || !slices.Equal(shape(doc), want):
			return itemError(d.File, it.pos, it.path, usererr.New("tmpl_html_page"))
		}
	}
	return nil
}

// drawMarked draws d, a copy marked, as execute does. As the page drawn as it is did not fail, a mistake is
// page.html's looking into an item, which a mark stands for.
func (d *Data) drawMarked(page *template.Template, title string) ([]byte, error) {
	out, err := d.run(page, title)
	if err != nil {
		return nil, usererr.New("tmpl_html_looked", d.tmpl.file("page.html"), err.Error())
	}
	return out, nil
}

// marked returns a copy of d whose items of format html are drawn with mark, all shown till its marks'
// shown says otherwise, so that d draws its page as it is.
func (d *Data) marked(mark string) *Data {
	m := &htmlMarks{
		mark: mark, at: map[string]int{}, shown: func(int) bool { return true }, markdown: map[string]template.HTML{},
	}
	for i, it := range d.htmlItems {
		m.at[pathKey(it.at)] = i
	}
	c := *d
	c.marks = m
	return &c
}

// headHider returns the index of the first item of format html of d, in the order the page draws them,
// with which those before it hide the </head> of the page page draws, titled title, which the page drawn
// without them has: an open <script> or tag of one does. -1 for none: the page has no </head> without them
// either. They are drawn as marks, so that page.html takes the branches it takes with them ({{with}}).
func (d *Data) headHider(page *template.Template, title string) (int, error) {
	c := d.marked(rand.Text())
	on := make([]bool, len(d.htmlItems))
	c.marks.shown = func(n int) bool { return on[n] }
	all, err := c.drawMarked(page, title)
	if err != nil || headless(all) {
		return -1, err
	}
	for _, k := range c.marks.drawnOrder(all) {
		on[k] = true
		out, err := c.drawMarked(page, title)
		if err != nil {
			return -1, err
		}
		if headless(out) {
			return k, nil
		}
	}
	return -1, nil
}

// headless reports whether page has no </head>.
func headless(page []byte) bool {
	return tagsOf(tagsIn(string(page), false)).head < 0
}

// markdownHTML returns the HTML of text, an item of format markdown, kept by the drawings that check the
// items of format html.
func (d *Data) markdownHTML(text string) template.HTML {
	if d != nil && d.marks != nil {
		if out, ok := d.marks.markdown[text]; ok {
			return out
		}
	}
	out, _ := markdownHTML(text) //nolint:errcheck // goldmark fails only to write, and a buffer does not
	if d != nil && d.marks != nil {
		d.marks.markdown[text] = out
	}
	return out
}

// parseDrawn reads page as a browser does, with a script after it, as floe puts the diagrams' and a served
// page's there, which an item may take in.
func parseDrawn(page []byte) *html.Node {
	// A bytes.Reader fails no read, the one error Parse returns.
	doc, _ := html.Parse(bytes.NewReader(slices.Concat(page, []byte("<script></script>")))) //nolint:errcheck // see above
	return doc
}

// inPlace puts item, read alone, in doc where the marks of the item n of mark are: in place of each, the
// nodes of item read as a fragment of what holds the mark, and reports whether they hold a <form>. What
// holds it is the nearest element the parser may not make itself (implied), which the item may end as
// the page's would (a <tfoot> ends a <tbody>). A mark between the <head> and the <body>, where what is
// not of the head starts the body, is put at the body's start; one elsewhere outside them is outside. A
// mark in what the parser reads as text (a <title>, a <script>) is that text's, which shape leaves out.
func inPlace(doc *html.Node, mark string, n int, item string) (formIn, outside bool) {
	want := mark + ":" + strconv.Itoa(n)
	var marks []*html.Node
	for c := range doc.Descendants() {
		if c.Type == html.CommentNode && c.Data == want {
			marks = append(marks, c)
		}
	}
	for _, m := range marks {
		context, parent, before := m.Parent, m.Parent, m
		for implied(context) && context.DataAtom != atom.Body && context.DataAtom != atom.Html {
			context = context.Parent
		}
		if context.Type != html.ElementNode || context.DataAtom == atom.Html {
			body := bodyAfter(m)
			if body == nil {
				return formIn, true
			}
			context, parent, before = body, body, body.FirstChild
		}
		// A strings.Reader fails no read, the one error ParseFragment returns.
		nodes, _ := html.ParseFragment(strings.NewReader(item), context) //nolint:errcheck // see above
		for _, c := range nodes {
			formIn = formIn || holdsForm(c)
			parent.InsertBefore(c, before)
		}
		m.Parent.RemoveChild(m)
	}
	return formIn, false
}

// bodyAfter returns the <body> after m, a node of the <html> after its <head>; nil for none.
func bodyAfter(m *html.Node) *html.Node {
	if m.Parent.Type != html.ElementNode {
		return nil
	}
	for c := m.Parent.FirstChild; c != nil; c = c.NextSibling {
		switch {
		case c == m:
			return nil
		case c.Type == html.ElementNode && c.DataAtom == atom.Head:
			for b := m.NextSibling; b != nil; b = b.NextSibling {
				if b.Type == html.ElementNode && b.DataAtom == atom.Body {
					return b
				}
			}
			return nil
		}
	}
	return nil
}

// holdsForm reports whether n is or holds a <form> of HTML, out of a <template>, whose content is no part
// of the page.
func holdsForm(n *html.Node) bool {
	if n.Type == html.ElementNode && n.Namespace == "" {
		switch n.DataAtom {
		case atom.Template:
			return false
		case atom.Form:
			return true
		default:
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if holdsForm(c) {
			return true
		}
	}
	return false
}

// leavesForm reports whether the last tag of a form item writes out of a <template>, read as the parser
// does, is a start tag: the parser keeps that form, out of the tree, as the form of the inputs after it,
// till a </form>.
func leavesForm(item string) bool {
	open, templates := false, 0
	z := html.NewTokenizer(strings.NewReader(item))
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			return open
		case html.StartTagToken, html.SelfClosingTagToken, html.EndTagToken:
		default:
			continue
		}
		switch name, _ := z.TagName(); {
		case string(name) == "template" && tt == html.EndTagToken:
			templates = max(templates-1, 0)
		case string(name) == "template":
			templates++
		case string(name) == "form" && templates == 0:
			open = tt != html.EndTagToken
		}
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
// head is not: what an item puts in it that ends it (a <div>) changes the page. Nor is a <tr>: an item
// that ends the page's row and starts another (</tr><tr>) moves the page's cells after it.
// LIMIT: an item splitting the page's <tbody> or <colgroup> is not seen (rows keep order); see it if it matters.
func implied(n *html.Node) bool {
	switch n.DataAtom {
	case atom.Html, atom.Body, atom.Tbody, atom.Colgroup:
		return n.Namespace == ""
	default:
		return false
	}
}

// readAsText reports whether the parser reads what is in n as its text: an item drawn there is text,
// which html/template escapes in a <script>, a <style>, a <title> and a <textarea>, and one that ends n
// changes what comes after it.
func readAsText(n *html.Node) bool {
	switch n.DataAtom {
	case atom.Script, atom.Style, atom.Title, atom.Textarea, atom.Noscript, atom.Xmp, atom.Iframe,
		atom.Noembed, atom.Noframes, atom.Plaintext:
		return n.Namespace == ""
	default:
		return false
	}
}

// newlineDropped reports whether the parser drops a newline that starts n's text, right after its tag,
// which it keeps in an item read alone.
func newlineDropped(n *html.Node) bool {
	return (n.DataAtom == atom.Pre || n.DataAtom == atom.Listing) && n.Namespace == ""
}

// shape returns the elements, comments and texts of n in order, by their names, the names of the elements'
// attributes, and the texts' words, those next to each other as one, as an item's text and the page's
// after it are in the page drawn. An item that gives the page's <body> an attribute (<body hidden>) changes
// the page, and so does one that takes in the page's text (x </). The attributes' values are left out,
// which an item drawn in a value is in, read or not, and so are the elements the parser may make itself
// (implied), but their attributes, and the text the parser reads an element's content as (readAsText).
func shape(n *html.Node) []string {
	var out []string
	var text strings.Builder
	dropNewline := false
	flush := func() {
		s := text.String()
		if dropNewline {
			s = strings.TrimLeft(s, "\n")
		}
		if s != "" {
			out = append(out, "#text"+s)
		}
		text.Reset()
		dropNewline = false
	}
	var write func(n *html.Node)
	write = func(n *html.Node) {
		if n.Type != html.TextNode {
			flush()
		}
		switch n.Type {
		case html.ElementNode:
			if o := openingOf(n); o != "" {
				out = append(out, o)
			}
			dropNewline = newlineDropped(n)
		case html.TextNode:
			text.WriteString(n.Data)
		case html.CommentNode:
			out = append(out, "#comment")
		default:
		}
		if !readAsText(n) {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				write(c)
			}
		}
		if n.Type == html.ElementNode && !implied(n) {
			flush()
			out = append(out, "</>")
		}
	}
	write(n)
	flush()
	return out
}
