// Package draw draws a page of a data file with its template and the parts its schema names, checks the page
// drawn for what floe puts into it, and puts in the page's styles and its headings' ids. It returns HTML alone:
// where a page is kept, and its CSP, are its caller's.
package draw

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"slices"
	"strconv"
	"strings"
	ttemplate "text/template"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/schema"
	"github.com/sukekyo26/floe/internal/textpos"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// Page draws d, checked, with t of k in the theme named so, as a page titled title, calc the values of t's
// computed.yaml (nil for none), the images it names read under imagesDir ("" for none): each part and item of
// a format from the inner ones out, then page.html. The mistakes of the items are told together, a part
// holding one that failed left undrawn. It checks the page drawn, its items of format html first (items.check),
// and returns it as the parser read it, with its styles in its head and its headings given ids, written anew.
func Page(
	k *kit.Kit, t *kit.Template, theme string, d *doc.Data, calc map[string]any, title, imagesDir string,
) ([]byte, error) {
	s := newItems(imagesDir)
	defer s.close()
	holds := map[string][]string{}
	var found usererr.List
	var failed [][]string
	data, err := t.Schema().Walk(d.Values, func(it schema.Item, ready any) (any, error) {
		if it.Part != "" {
			holds[it.Part] = it.Holds
		}
		if slices.ContainsFunc(failed, func(at []string) bool { return under(at, it.At) }) {
			return nil, nil //nolint:nilnil // not drawn: the mistake of what it holds is told
		}
		drawn, err := drawItem(k, d, it, ready, s)
		if err != nil {
			failed = append(failed, it.At)
			return nil, found.Add(err)
		}
		return drawn, nil
	})
	if err != nil {
		return nil, err //nolint:wrapcheck // floe's failure
	}
	if found != nil {
		return nil, found.Sorted().Err()
	}
	page, err := t.Page(d.Lang)
	if err != nil {
		return nil, err //nolint:wrapcheck // floe's failure, which says what failed
	}
	var b bytes.Buffer
	if err = page.Execute(&b, map[string]any{"Title": title, "Lang": d.Lang, "Data": data, "Calc": calc}); err != nil {
		e, at, failed := executed(err, "page.html", t.PageSource())
		if failed != nil {
			return nil, failed
		}
		e.File, e.Line, e.Column = t.File("page.html"), 0, 0
		if at >= 0 {
			e.Line, e.Column = textpos.Of(t.PageSource(), at)
		}
		return nil, e
	}
	tree, named, err := readPage(k, d, s, b.Bytes())
	if err != nil {
		return nil, err
	}
	for name, parts := range holds {
		named.uses[name] = append(named.uses[name], parts...)
	}
	for name, used := range named.uses {
		named.uses[name] = slices.Compact(slices.Sorted(slices.Values(used)))
	}
	sheets, err := pageStyle(k, theme, named, t.CSS())
	if err != nil {
		return nil, err
	}
	withStyles(tree, sheets)
	withHeadingIDs(tree)
	var out bytes.Buffer
	// LIMIT: Render writes no form's pointer: a <form> ended in a <div> or a <table> it opened, read again, owns
	// other inputs, or drops a nested one. Write the forms apart if a template comes to rely on them (the CSP's
	// form-action 'none' sends none anyway).
	if err := html.Render(&out, tree); err != nil {
		return nil, fmt.Errorf("draw: write the page: %w", err)
	}
	return out.Bytes(), nil
}

// readPage reads page, drawn from d with the items s, as a browser does, checks its items of format html, then,
// their marks taken out, the page, and returns the page read and what it names. The page an item changed is not
// checked: what it holds would be told by what the item did.
func readPage(k *kit.Kit, d *doc.Data, s *items, page []byte) (*html.Node, named, error) {
	tree, _ := html.Parse(bytes.NewReader(page)) //nolint:errcheck // a bytes.Reader fails no read, Parse's one error
	ns := namesIn(k)
	if err := s.check(k, ns, d, tree); err != nil {
		return nil, named{}, err
	}
	s.unmark(tree)
	n, err := checkPage(k, ns, d, tree)
	return tree, n, err
}

// withStyles puts a <style> of each of sheets at the end of the head of tree, which the parser always makes.
func withStyles(tree *html.Node, sheets []string) {
	for n := range tree.Descendants() {
		if n.Type == html.ElementNode && n.DataAtom == atom.Head && n.Namespace == "" {
			for _, sheet := range sheets {
				style := node(html.ElementNode, atom.Style, "style")
				style.AppendChild(node(html.TextNode, 0, sheet))
				n.AppendChild(style)
				n.AppendChild(node(html.TextNode, 0, "\n"))
			}
			return
		}
	}
}

// node returns a node of the type typ, of no namespace and with no attributes, to put in a tree.
func node(typ html.NodeType, a atom.Atom, data string) *html.Node {
	return &html.Node{
		Parent: nil, FirstChild: nil, LastChild: nil, PrevSibling: nil, NextSibling: nil,
		Type: typ, DataAtom: a, Data: data, Namespace: "", Attr: nil,
	}
}

// under reports whether the path at is under the path of, longer.
func under(at, of []string) bool {
	return len(at) > len(of) && slices.Equal(at[:len(of)], of)
}

// drawItem draws it, an item of d, ready as Walk made it: a part's by its part.html in d's language; one of a
// format of floe's from its text, refused at the item for another value: markdown as markdown draws it, html
// marked for s to check in the page drawn, an image as the data URI of the file it names, read by s.
func drawItem(k *kit.Kit, d *doc.Data, it schema.Item, ready any, s *items) (any, error) {
	if it.Part != "" {
		p, ok := k.Part(it.Part)
		if !ok {
			//nolint:err113 // floe's own mistake
			return nil, fmt.Errorf("draw: no part %s, which the schema was compiled with", it.Part)
		}
		page, err := p.Page(d.Lang)
		if err != nil {
			return nil, err //nolint:wrapcheck // floe's failure, which says what failed
		}
		var b bytes.Buffer
		if err := page.Execute(&b, ready); err != nil {
			e, at, failed := executed(err, "part.html", p.PageSource())
			if failed != nil {
				return nil, failed
			}
			switch {
			case e.File == p.File("part.html") && at >= 0:
				// One part.html tells of itself (a word its words.yaml has not): at its line and column, told
				// once whatever uses the part.
				e.Line, e.Column = textpos.Of(p.PageSource(), at)
			case e.File == "":
				// At the value drawn: a part of floe's is no file one can fix, and the value tells why.
				pos := d.PosOf(it.At)
				e.File, e.Line, e.Column = d.File, pos.Line, pos.Column
			}
			return nil, e
		}
		return template.HTML(strings.TrimSpace(b.String())), nil //nolint:gosec // G203: drawn by html/template
	}
	text, ok := it.Value.(string)
	if !ok {
		// A schema of one's own may not say "type": "string".
		return nil, yamlpos.New(d.File, d.PosOf(it.At), "draw_not_text", it.Format)
	}
	switch it.Format {
	case "markdown":
		return kit.Markdown(text) //nolint:wrapcheck // goldmark fails only to write, and a buffer does not
	case "html":
		return s.marked(it.At, text), nil
	case "image":
		uri, err := s.image(d, text)
		var e *usererr.Error
		if errors.As(err, &e) && e.File == "" {
			// What names no file is the item's: one of the file is told at it, once.
			return nil, yamlpos.New(d.File, d.PosOf(it.At), e.Key, e.Args...)
		}
		return uri, err
	}
	return nil, fmt.Errorf("draw: no format %s of floe's", it.Format) //nolint:err113 // floe's own mistake
}

// executed returns the mistake err of executing the template name, whose text is src (nil when where it is
// in the text is not asked): the usererr.Error it wraps, a copy, or text/template's words without their head
// (template: name:line:column: ), and where the head says it is in src, -1 for none; or floe's failure, an
// error of no execution. The head's line counts \n alone and its column bytes from 0: they are made a place
// in src, which textpos counts.
// LIMIT: text/template gives where it failed in its words alone; take it from the error once it gives it apart.
func executed(err error, name string, src []byte) (*usererr.Error, int, error) {
	var ee ttemplate.ExecError
	if !errors.As(err, &ee) || errors.Is(err, kit.ErrFunc) {
		return nil, -1, fmt.Errorf("draw with %s: %w", name, err)
	}
	line, column, rest, ok := cutHead(ee.Error(), name)
	at := -1
	if ok {
		at = offset(src, line, column)
	}
	var inner *usererr.Error
	if errors.As(err, &inner) && inner != nil {
		e := *inner
		return &e, at, nil
	}
	return usererr.LibraryAt("", 0, 0, rest), at, nil
}

// cutHead returns the line and the column the head of msg, template: name:line:column: , gives, and msg
// without it; false when msg has no such head.
func cutHead(msg, name string) (line, column int, rest string, ok bool) {
	after, ok := strings.CutPrefix(msg, "template: "+name+":")
	if !ok {
		return 0, 0, msg, false
	}
	lineText, after, ok1 := strings.Cut(after, ":")
	columnText, after, ok2 := strings.Cut(after, ": ")
	line, err1 := strconv.Atoi(lineText)
	column, err2 := strconv.Atoi(columnText)
	if !ok1 || !ok2 || err1 != nil || err2 != nil || line < 1 || column < 0 {
		return 0, 0, msg, false
	}
	return line, column, after, true
}

// offset returns the byte of src at line, counting \n alone, and column, bytes from 0, as text/template
// counts them; -1 when src holds no such byte.
func offset(src []byte, line, column int) int {
	start := 0
	for range line - 1 {
		i := bytes.IndexByte(src[start:], '\n')
		if i < 0 {
			return -1
		}
		start += i + 1
	}
	if at := start + column; at < len(src) {
		return at
	}
	return -1
}
