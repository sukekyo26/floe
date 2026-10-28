// Package draw draws a page of a data file with its template and the parts its schema names, checks the page
// drawn for what floe puts into it, and puts in the page's styles and its headings' ids. It returns HTML alone:
// where a page is kept, and its CSP, are its caller's.
package draw

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"slices"
	"strings"
	ttemplate "text/template"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/page"
	"github.com/sukekyo26/floe/internal/schema"
	"github.com/sukekyo26/floe/internal/textpos"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// Page draws d, checked, with t of k in the theme named so, as a page titled title, calc the values of t's
// computed.yaml (nil for none), the images it names read from images (nil for none) and named in mistakes under
// imagesDir ("" for the data file's directory), its diagrams' icons
// looked up in icons (nil for none kept): each part and item of a format from the inner ones out, then
// page.html. The mistakes of the items are told together, a part
// holding one that failed left undrawn. It checks the page drawn, its items of format html first (items.check),
// and returns it as the parser read it, with its styles in its head and its headings given ids, written anew.
func Page(
	k *kit.Kit, t *kit.Template, theme string, d *doc.Data, calc map[string]any, title string,
	images fs.FS, imagesDir string, icons diagram.Icons,
) ([]byte, error) {
	s := newItems(images, imagesDir, icons)
	holds := map[string][]string{}
	var found usererr.List
	var failed [][]string
	data, err := t.Schema().Walk(d.Values, func(it schema.Item, ready any) (any, error) {
		if it.Part != "" {
			holds[it.Part] = it.Holds
		}
		// Walk calls fn with the items a part holds before the part, so that what failed in it is known here; at
		// the part's own path are what its root is: a format (a part whose value is an image), or another part.
		if slices.ContainsFunc(failed, func(at []string) bool { return atOrUnder(at, it.At) }) {
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
		return nil, s.pageMistakes(t, err)
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

// readPage reads src, the page drawn from d with the items s, as a browser does, checks its items of format
// html, then, their marks taken out, the page, and returns the page read and what it names. The page an item
// changed is not checked: what it holds would be told by what the item did.
func readPage(k *kit.Kit, d *doc.Data, s *items, src []byte) (*html.Node, named, error) {
	end := s.secret + "end"
	marked := slices.Concat(src, []byte("<!--"+end+"-->"))
	tree, _ := html.Parse(bytes.NewReader(marked)) //nolint:errcheck // a bytes.Reader fails no read, Parse's one error
	// A page cut short, its end mark read as a comment, is checked no further: an item's marks past the cut are
	// gone, and would tell it changed. Else an item that ate the mark (a tag it left open) is told at it, and a
	// page cut with no item to blame is told cut.
	cut := !cutEnd(tree, end)
	if cut && tokenized(marked, end) {
		return nil, named{}, yamlpos.New(d.File, d.Where["template"], "draw_page_cut")
	}
	ns := namesIn(k)
	if err := s.check(k, ns, d, tree); err != nil {
		return nil, named{}, err
	}
	if cut {
		return nil, named{}, yamlpos.New(d.File, d.Where["template"], "draw_page_cut")
	}
	s.unmark(tree)
	n, err := checkPage(k, ns, d, tree)
	return tree, n, err
}

// cutEnd takes end, the text of the comment put after the page drawn, out of tree, and reports whether it was
// there: as the comment, or in a text the page ended in (one of an element html/template does not follow, as
// xmp or plaintext, or svg's CDATA).
// x/net/html v0.61.0 leaves out the rest of a page past a <template> in an <svg> or a <math> (parse.go:672).
// LIMIT: tells neither where the page was cut nor what wrote it; drop when x/net/html reads such a page whole.
func cutEnd(tree *html.Node, end string) bool {
	for n := range tree.Descendants() {
		switch {
		case n.Type == html.CommentNode && n.Data == end:
			n.Parent.RemoveChild(n)
			return true
		case (n.Type == html.CommentNode || n.Type == html.TextNode) && strings.Contains(n.Data, end):
			n.Data = strings.Replace(strings.Replace(n.Data, "<!--"+end+"-->", "", 1), end, "", 1)
			return true
		}
	}
	return false
}

// tokenized reports whether the tokenizer, read alone, takes end, at the end of src, as a comment of its own:
// only to choose whom to tell of a page without the mark, the page or an item that ate it.
// LIMIT: past a cut, a page ending in an open raw text (xmp) blames an item, or the wrong fix; rework if real.
func tokenized(src []byte, end string) bool {
	z := html.NewTokenizer(bytes.NewReader(src))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return false
		case html.CommentToken:
			if string(z.Text()) == end {
				return true
			}
		default:
		}
	}
}

// withStyles puts a <style> of each of sheets at the end of the head of tree, which the parser always makes.
func withStyles(tree *html.Node, sheets []string) {
	head := page.First(tree, atom.Head)
	for _, sheet := range sheets {
		style := node(html.ElementNode, atom.Style, "style")
		style.AppendChild(node(html.TextNode, 0, sheet))
		head.AppendChild(style)
		head.AppendChild(node(html.TextNode, 0, "\n"))
	}
}

// node returns a node of the type typ, of no namespace and with no attributes, to put in a tree.
func node(typ html.NodeType, a atom.Atom, data string) *html.Node {
	return &html.Node{
		Parent: nil, FirstChild: nil, LastChild: nil, PrevSibling: nil, NextSibling: nil,
		Type: typ, DataAtom: a, Data: data, Namespace: "", Attr: nil,
	}
}

// atOrUnder reports whether the path at is the path of, or under it.
func atOrUnder(at, of []string) bool {
	return len(at) >= len(of) && slices.Equal(at[:len(of)], of)
}

// pageMistakes returns the mistakes err of executing the page.html of t: one of a file of its own (an asset
// that cannot be read) told there; one of page.html, or of no file, at its line and column in it; or floe's
// failure.
func (s *items) pageMistakes(t *kit.Template, err error) error {
	mistakes, line, column, failed := executed(err, "page.html", t.PageSource())
	if failed != nil {
		return failed
	}
	for i, e := range mistakes {
		e.Args = s.plainArgs(e.Args)
		if e.File() != "" && e.File() != t.File("page.html") {
			continue
		}
		mistakes[i] = e.PlacedAt(t.File("page.html"), line, column)
	}
	return mistakes.Err()
}

// drawItem draws it, an item of d, ready as Walk made it: a part's by its part.html (drawPart); one of a format
// of floe's from its text, refused at the item for another value: markdown as markdown draws it, html marked for
// s to check in the page drawn, an image as the data URI of the file it names, read by s.
func drawItem(k *kit.Kit, d *doc.Data, it schema.Item, ready any, s *items) (any, error) {
	if it.Part != "" {
		return drawPart(k, d, it, ready, s)
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
		if errors.As(err, &e) && e.File() == "" {
			// What names no file is the item's: one of the file is told at it, once.
			pos := d.PosOf(it.At)
			return nil, e.PlacedAt(d.File, pos.Line, pos.Column)
		}
		return uri, err
	}
	return nil, fmt.Errorf("draw: no format %s of floe's", it.Format) //nolint:err113 // floe's own mistake
}

// drawPart draws it, an item of d of a part, ready as Walk made it, by the part's part.html in d's language, its
// function diagram drawing the part's value as written.
func drawPart(k *kit.Kit, d *doc.Data, it schema.Item, ready any, s *items) (any, error) {
	p, ok := k.Part(it.Part)
	if !ok {
		return nil, fmt.Errorf("draw: no part %s, which the schema was compiled with", it.Part) //nolint:err113 // floe's
	}
	page, err := p.Page(d.Lang)
	if err != nil {
		return nil, err //nolint:wrapcheck // floe's failure, which says what failed
	}
	// Walk's ready holds the keys a schema declares and the value leaves out, as nil: not the spec written.
	page.Funcs(template.FuncMap{"diagram": s.diagramOf(d, it)})
	var b bytes.Buffer
	if err := page.Execute(&b, ready); err != nil {
		mistakes, line, column, failed := executed(err, "part.html", p.PageSource())
		if failed != nil {
			return nil, failed
		}
		for i, e := range mistakes {
			e.Args = s.plainArgs(e.Args)
			switch {
			case e.File() == p.File("part.html") && line > 0:
				// One part.html tells of itself (a word its words.yaml has not): at its line and column, told
				// once whatever uses the part.
				mistakes[i] = e.PlacedAt(e.File(), line, column)
			case e.File() == "" && e.At() == "":
				// At the value drawn: a part of floe's is no file one can fix, and the value tells why. One of
				// the whole output (the icons of a diagram) takes no place.
				pos := d.PosOf(it.At)
				mistakes[i] = e.PlacedAt(d.File, pos.Line, pos.Column)
			}
		}
		// A function's mistakes at a JSON pointer are at one from the root of the part's value.
		return nil, yamlpos.Place(mistakes.Err(), d.File, d.Root, it.At) //nolint:wrapcheck // mistakes placed
	}
	return template.HTML(s.trim(b.String())), nil //nolint:gosec // G203: drawn by html/template
}

// executed returns the mistakes err of executing the template name, whose text is src (nil when where it is
// in the text is not asked): the usererr.Error or the usererr.List it wraps, copies, or text/template's words
// without their head, and where the head says it is in src (textpos.OfTemplate), a line of 0 for none; or
// floe's failure, an error of no execution.
func executed(err error, name string, src []byte) (mistakes usererr.List, line, column int, failed error) {
	var ee ttemplate.ExecError
	if !errors.As(err, &ee) || errors.Is(err, kit.ErrFunc) {
		return nil, 0, 0, fmt.Errorf("draw with %s: %w", name, err)
	}
	line, column, rest := textpos.OfTemplate(src, ee.Error(), name)
	var inner usererr.List
	if inner.Add(ee.Err) != nil || inner == nil {
		return usererr.List{usererr.LibraryAt("", 0, 0, rest)}, line, column, nil
	}
	mistakes = make(usererr.List, len(inner))
	for i, e := range inner {
		c := *e
		mistakes[i] = &c
	}
	return mistakes, line, column, nil
}
