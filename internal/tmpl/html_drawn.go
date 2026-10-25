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

// htmlItemsOf returns the items of whole of format html, text but empty, which draws nothing, in the order
// of the file under root; nil for none.
func htmlItemsOf(whole drawnItems, root ast.Node) []htmlItem {
	var items []htmlItem
	for _, f := range whole.ofFormat("html") {
		text, isText := f.value.(string)
		if !isText || text == "" {
			continue
		}
		pos, path := itemPos(root, f.at, false)
		items = append(items, htmlItem{at: f.at, text: text, pos: pos, path: path})
	}
	slices.SortFunc(items, func(a, b htmlItem) int {
		return cmp.Or(cmp.Compare(a.pos.Line, b.pos.Line), cmp.Compare(a.pos.Column, b.pos.Column))
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
// checked in the <head> or after the <body> tag of doc, read in standards mode (withStyles asks it first):
// page.html putting one elsewhere is refused. page.html draws it whole: what it makes of an item's text is
// its own.
// LIMIT: page.html computing on an item's text (slice, len, eq) is checked as the mark makes it; theirs.
// LIMIT: each item draws and reads the page again, every view (400 items, 0.25s); keep the result if slow.
// LIMIT: an item left open for the next to close (<li>a, <li>b) is refused; read them together if pages need it.
// LIMIT: a comment mark rebuilds no <b> page.html left open (<p><b>x</p>); mark with text if pages need it.
// It refuses too each name an item gives that the page would be refused for (misnamings: a part that cannot
// be used or there is none of, a data-floe-part not of one part's name, a look as a box), read where the item
// is (a <title> in an <svg> holds elements), at the item.
func (d *Data) checkDrawnHTML(set *partSet, page *template.Template, title string, doc *html.Node) error {
	c := d.marked(rand.Text())
	c.marks.shown = func(int) bool { return false }
	all, err := c.execute(page, title)
	if err != nil {
		return err
	}
	want := treeShape(doc)
	var found usererr.List
	shaped := true
	for _, k := range c.marks.drawnOrder(all) {
		it := d.htmlItems[k]
		c.marks.shown = func(n int) bool { return n != k }
		bare, err := c.execute(page, title)
		if err != nil {
			if failed := found.Add(err); failed != nil {
				return failed //nolint:wrapcheck // what failed, as it is
			}
			// It is not compared with the page, so that what it does to it, which the items after it are
			// compared with, is not known: theirs is not asked.
			shaped = false
			continue
		}
		doc := parseDrawn(bare)
		formIn, outside, nodes := inPlace(doc, c.marks.mark, k, it.text)
		if outside {
			found = append(found, usererr.NewAt(d.tmpl.file("page.html"), 0, 0, "tmpl_html_outside"))
			shaped = false // as for one not drawn
			continue
		}
		atItem := func(bad *usererr.Error) *usererr.Error { return itemError(d.File, it.pos, it.path, bad) }
		found = append(found, set.placedMisnamings(drawnNames(nodes, it.text), d.File, it.pos, atItem)...)
		if formIn && leavesForm(it.text) || shaped && !slices.Equal(treeShape(doc), want) {
			// The items after it are compared with a page it changed: it is the one told.
			found = append(found, itemError(d.File, it.pos, it.path, usererr.New("tmpl_html_page")))
			break
		}
	}
	return found.Err()
}

// standard reports whether a browser reads doc in standards mode, as html.ParseFragment reads an item: its
// first node but comments is <!doctype html>, or its legacy form. Without it, in quirks mode, a <table>
// ends no <p>.
func standard(doc *html.Node) bool {
	for c := doc.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.CommentNode {
			legacy := []html.Attribute{{Namespace: "", Key: "system", Val: "about:legacy-compat"}}
			return c.Type == html.DoctypeNode && c.Data == "html" &&
				(len(c.Attr) == 0 || slices.EqualFunc(c.Attr, legacy, func(a, b html.Attribute) bool {
					return a.Key == b.Key && strings.EqualFold(a.Val, b.Val)
				}))
		}
	}
	return false
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

// withStyles checks out, the page page drew of d, titled title, set's parts its parts, as a browser reads it,
// and returns it with its styles (pageStyle, own its template's CSS) before its </head> (headAt). What it
// checks is of the page drawn, whoever wrote it (page.html, a part, an item of format html): a mistake of
// d's, at its template: (pageMistake). An item of format html is checked (checkDrawnHTML) in a page read in
// standards mode: the page an item broke, or whose items are not checked, would tell it again as a </head>
// or a name, which are then not asked. A <style> is asked all the same: an item holds none (checkHTML).
// LIMIT: what out holds is not told by who wrote it, nor at its line: a page drawn leads back to neither.
func (d *Data) withStyles(set *partSet, page *template.Template, title string, out, own []byte) ([]byte, error) {
	doc := parseDrawn(out)
	style := d.styleMistake(doc)
	if d.htmlItems != nil {
		var items error
		if !standard(doc) {
			items = d.pageMistake("tmpl_html_doctype")
		} else {
			items = d.checkDrawnHTML(set, page, title, doc)
		}
		if items != nil {
			return nil, usererr.Collect(items, style) //nolint:wrapcheck // mistakes of the page drawn and of its items
		}
	}
	k, _ := headAt(out)
	var head error
	if k < 0 {
		head = d.pageMistake("tmpl_drawn_head")
	}
	named := drawnNames([]*html.Node{doc}, string(out))
	at := func(bad *usererr.Error) *usererr.Error { return yamlpos.Place(d.File, d.templateAt, bad) }
	misnamed := set.placedMisnamings(named, d.File, d.templateAt, at)
	if err := usererr.Collect(head, style, misnamed.Err()); err != nil {
		return nil, err //nolint:wrapcheck // mistakes of the page drawn
	}
	sheets, err := set.pageStyle(named, own, d.Theme)
	if err != nil {
		return nil, err
	}
	end := headEnd.FindAllIndex(out, -1)[k][0]
	return slices.Concat(out[:end], []byte(sheets), out[end:]), nil
}

// pageMistake is the mistake key of the page drawn of d, at its template:, which draws it.
func (d *Data) pageMistake(key string) *usererr.Error {
	return yamlpos.New(d.File, d.templateAt, key)
}

// styleMistake refuses doc, the page drawn of d, holding a <style>, of HTML, SVG or MathML: floe puts a page's
// CSS together.
func (d *Data) styleMistake(doc *html.Node) error {
	for n := range doc.Descendants() {
		if n.Type == html.ElementNode && n.Data == "style" {
			return d.pageMistake("tmpl_drawn_style")
		}
	}
	return nil
}

// probeAttr marks the <style> headAt puts into a page to see where a browser puts the page's styles.
const probeAttr = "data-floe-probe"

// headAt returns which </head> written in page, the k-th, the page's styles go before, and the tree of page
// read with a <style> of its own there; -1 and nil for none. A </head> is one where a <style> put before it
// is a <style> of the page, in no <template>, whose content is not drawn, and in no <svg> or <math>, where
// it styles nothing: in the head, or in the body or a table a <div> or a <table> of the head moved it to,
// where it styles the page all the same. One in a comment, a value or a script's text is none: the <style>
// put there is no element. Each </head> written is tried, so that one the tokenizer would not see (after a
// <style> in an <svg>, which is no text) is one too.
// LIMIT: the page is read once for each </head> tried; read fewer, from the tree, if pages come to write many.
func headAt(page []byte) (int, *html.Node) {
	for k, at := range headEnd.FindAllIndex(page, -1) {
		probe := []byte("<style " + probeAttr + "></style>")
		// A bytes.Reader fails no read, the one error Parse returns.
		doc, _ := html.Parse(bytes.NewReader(slices.Concat(page[:at[0]], probe, page[at[0]:]))) //nolint:errcheck // see above
		if styled(doc) {
			return k, doc
		}
	}
	return -1, nil
}

// styled reports whether doc holds the <style> headAt puts, as it puts it, where it styles the page.
func styled(doc *html.Node) bool {
	for n := range doc.Descendants() {
		// Its attribute alone and no content: one of the page's, which the probe went into as a value, is not it.
		if n.Type != html.ElementNode || n.DataAtom != atom.Style || n.Namespace != "" || n.FirstChild != nil ||
			len(n.Attr) != 1 || n.Attr[0].Key != probeAttr {
			continue
		}
		for a := range n.Ancestors() {
			if a.Type == html.ElementNode && (a.Namespace != "" || a.DataAtom == atom.Template) {
				return false
			}
		}
		return true
	}
	return false
}

// drawnNames returns the names the data-floe-part and data-floe-look attributes of the elements of nodes, as
// a browser reads them, give, in order, then those text writes anywhere (textNames), which bring the
// part's style alone. The elements of a <template> are read too, as a script may copy them; the text of a
// <noscript> is text, as a sandboxed page runs its scripts.
func drawnNames(nodes []*html.Node, text string) []namedPart {
	var names []namedPart
	for _, top := range nodes {
		names = append(names, elementNames(top)...)
		for n := range top.Descendants() {
			names = append(names, elementNames(n)...)
		}
	}
	return append(names, textNames(text, nil)...)
}

// elementNames returns the names n gives by its attributes when it is an element.
func elementNames(n *html.Node) []namedPart {
	if n.Type != html.ElementNode {
		return nil
	}
	var names []namedPart
	for _, a := range n.Attr {
		if look := a.Key == "data-floe-look"; a.Namespace == "" && (look || a.Key == "data-floe-part") {
			names = append(names, namesOf(a.Val, look, false, 0)...)
		}
	}
	return names
}

// itemNames returns the names item, HTML that goes into a page, gives, wherever it goes: those of its start
// tags as the tokenizer reads them, which a fragment read in the <body> would leave out (a <tr>), then
// those it writes anywhere (textNames). Where it goes, an item of format html is read again (checkDrawnHTML).
func itemNames(item string) []namedPart {
	var names []namedPart
	z := html.NewTokenizer(strings.NewReader(item))
	for tt := z.Next(); tt != html.ErrorToken; tt = z.Next() {
		if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
			continue
		}
		for _, a := range z.Token().Attr {
			if look := a.Key == "data-floe-look"; look || a.Key == "data-floe-part" {
				names = append(names, namesOf(a.Val, look, false, 0)...)
			}
		}
	}
	return append(names, textNames(item, nil)...)
}

// attrValue returns value, an attribute's as written, as a browser reads it, its character references read.
func attrValue(value string) string {
	return html.UnescapeString(value)
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
// nodes of item read as a fragment of what holds the mark, and reports whether they hold a <form> out of
// a <template>, whose content the parser keeps apart. What holds it is the nearest element the parser
// may not make itself (implied), which the item may end as the page's would (a <tfoot> ends a <tbody>).
// A mark of the <html> itself, before the <head>, between it and the <body> tag or after the </body>, is
// outside: there the parser puts a space in the <html>, a <meta> in the head and a <p> in the body. A mark
// in what the parser reads as text (a <title>, a <script>) is that text's, which shape leaves out. It
// returns the nodes of item it put in too.
func inPlace(doc *html.Node, mark string, n int, item string) (formIn, outside bool, put []*html.Node) {
	want := mark + ":" + strconv.Itoa(n)
	var marks []*html.Node
	for c := range doc.Descendants() {
		if c.Type == html.CommentNode && c.Data == want {
			marks = append(marks, c)
		}
	}
	for _, m := range marks {
		if m.Parent.Type != html.ElementNode || m.Parent.DataAtom == atom.Html {
			return formIn, true, put
		}
		context := m.Parent
		for implied(context) && context.DataAtom != atom.Body {
			context = context.Parent
		}
		// A strings.Reader fails no read, the one error ParseFragment returns.
		nodes, _ := html.ParseFragment(strings.NewReader(item), context) //nolint:errcheck // see above
		inTemplate := slices.ContainsFunc(slices.Collect(m.Ancestors()), isTemplate)
		for _, c := range nodes {
			formIn = formIn || !inTemplate && holdsForm(c)
			m.Parent.InsertBefore(c, m)
		}
		put = append(put, nodes...)
		m.Parent.RemoveChild(m)
	}
	return formIn, false, put
}

// isTemplate reports whether n is a <template> of HTML.
func isTemplate(n *html.Node) bool {
	return n.Type == html.ElementNode && n.DataAtom == atom.Template && n.Namespace == ""
}

// holdsForm reports whether n is or holds a <form> of HTML, out of a <template>, whose content is no part
// of the page.
func holdsForm(n *html.Node) bool {
	switch {
	case isTemplate(n):
		return false
	case n.Type == html.ElementNode && n.DataAtom == atom.Form && n.Namespace == "":
		return true
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

// treeShape returns the elements, comments and texts of n in order, by their names, the names of the elements'
// attributes, and the texts' words, those next to each other as one, as an item's text and the page's
// after it are in the page drawn. An item that gives the page's <body> an attribute (<body hidden>) changes
// the page, and so does one that takes in the page's text (x </). The attributes' values are left out,
// which an item drawn in a value is in, read or not, and so are the elements the parser may make itself
// (implied), but their attributes, and the text the parser reads an element's content as (readAsText).
func treeShape(n *html.Node) []string {
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
