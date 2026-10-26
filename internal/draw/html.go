package draw

import (
	"crypto/rand"
	"fmt"
	"html/template"
	"os"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"

	"github.com/sukekyo26/floe/internal/dirs"
	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// probe is the white space drawn around an item of format html. A parser rebuilds the formatting elements left
// open (<b>, <a>) before it puts in text, which comments do not make it do: the probe after an item is rebuilt
// into one the item left open, and the one before, into one the page left open, so that the item's marks share
// it. White space goes in as it is in a table and in a head.
const probe = "\t\n \t"

// items are the items of one drawing that floe draws, and what they need: those of format html, each marked so
// that the page drawn tells whether it changes the page around it, and the images of the data's directory, each
// read once.
type items struct {
	mark      string     // the marks' secret, in lower case as a parser makes tag and attribute names
	html      [][]string // the path of each item of format html, by its number
	texts     []string   // its text
	imagesDir string
	root      *os.Root
	rootErr   error
	images    map[string]image
}

// image is an image read, or its mistake.
type image struct {
	uri template.URL
	err error
}

// newItems returns the items of a drawing whose data's images are under imagesDir ("" for none).
func newItems(imagesDir string) *items {
	return &items{
		mark: strings.ToLower(rand.Text()), html: nil, texts: nil, imagesDir: imagesDir, root: nil, rootErr: nil,
		images: map[string]image{},
	}
}

// close lets go of the directory of the images.
func (s *items) close() {
	if s.root != nil {
		_ = s.root.Close() //nolint:errcheck // only read from
	}
}

// marked returns text, the item of format html at the path at, between its marks and the probes.
func (s *items) marked(at []string, text string) template.HTML {
	n := strconv.Itoa(len(s.html))
	s.html = append(s.html, at)
	s.texts = append(s.texts, text)
	//nolint:gosec // G203: the data's HTML, checked in the page drawn (check)
	return template.HTML(probe + "<!--" + s.mark + " " + n + " s-->" + text + "<!--" + s.mark + " " + n + " e-->" + probe)
}

// image reads the image name of d, once a drawing.
func (s *items) image(d *doc.Data, name string) (template.URL, error) {
	if got, ok := s.images[name]; ok {
		return got.uri, got.err
	}
	dir := dirs.Dir(d.File)
	if s.imagesDir == "" {
		return "", fmt.Errorf("draw: no directory to read the image %s under", name) //nolint:err113 // the caller's mistake
	}
	if s.root == nil && s.rootErr == nil {
		s.root, s.rootErr = os.OpenRoot(s.imagesDir)
	}
	var got image
	if s.rootErr != nil {
		got.err = usererr.CannotRead(dir, s.rootErr)
	} else {
		got.uri, got.err = kit.ReadImage(s.root.FS(), name, dir)
	}
	s.images[name] = got
	return got.uri, got.err
}

// occurrence is where the page drawn has a mark of an item: n's start (or end), a comment as drawn when at is
// nil, else in a text or an attribute of the element at.
type occurrence struct {
	n    int
	node *html.Node
	at   *html.Node
}

// check refuses each item of format html of d that changes the page tree, drawn, around it, at the item, and an
// item page.html puts where HTML does not work, at d's template:. They are looked at in the order of the page;
// past the first that changed the page, which the others are read in, none is. It refuses at the item a name of
// a part its elements give that the page would be refused for (checkNames).
// LIMIT: the marks and the probe show the tree and the formatting rebuilt in body text alone: the form an item
// left open, the attributes it adds to body or html, and formatting left open in a table, where the probe of
// white space rebuilds none, are not checked; check them in the browser (B12).
// LIMIT: page.html puts an item of format html where elements go, and of table rows in a tbody; not straight in
// a table followed by text, nor between html and head: there the probe is moved or dropped and a right item is
// refused. A name of a part the parser moves out of a table is told at template:, by checkPage.
func (s *items) check(k *kit.Kit, ns names, d *doc.Data, tree *html.Node) error {
	if len(s.html) == 0 {
		return nil
	}
	var found usererr.List
	for _, o := range s.starts(tree) {
		if o.at != nil {
			found = append(found, yamlpos.New(d.File, d.Where["template"], "draw_html_placed",
				strings.Join(s.html[o.n], "."), "<"+o.at.Data+">"))
			break
		}
		pos := d.PosOf(s.html[o.n])
		refuse := func(key string, args ...any) { found = append(found, yamlpos.New(d.File, pos, key, args...)) }
		end := s.end(tree, o)
		if end == nil {
			refuse("draw_html_whole", s.lost(tree, o))
			break
		}
		if where := changed(o.node, end); where != "" {
			refuse("draw_html_whole", where)
			break
		}
		for n := o.node.NextSibling; n != end; n = n.NextSibling {
			for e := range n.Descendants() {
				checkNames(k, ns, e, map[string]bool{}, map[string]bool{}, refuse)
			}
			checkNames(k, ns, n, map[string]bool{}, map[string]bool{}, refuse)
		}
	}
	return found.Err()
}

// starts returns the starts of the items in the order of tree: each comment that is one, and each text or
// attribute that holds one.
func (s *items) starts(tree *html.Node) []occurrence {
	var found []occurrence
	for n := range tree.Descendants() {
		switch n.Type {
		case html.CommentNode:
			if i, ok := s.number(n.Data, "s"); ok {
				found = append(found, occurrence{n: i, node: n, at: nil})
			}
		case html.TextNode:
			for _, i := range s.within(n.Data, "s") {
				found = append(found, occurrence{n: i, node: n, at: n.Parent})
			}
		case html.ElementNode:
			for _, a := range n.Attr {
				for _, i := range s.within(a.Key+" "+a.Val, "s") {
					found = append(found, occurrence{n: i, node: n, at: n})
				}
			}
		default:
		}
	}
	return found
}

// number returns n of a comment's text that is the mark "mark n end" as drawn.
func (s *items) number(text, end string) (int, bool) {
	rest, ok := strings.CutPrefix(text, s.mark+" ")
	if !ok {
		return 0, false
	}
	digits, ok := strings.CutSuffix(rest, " "+end)
	n, err := strconv.Atoi(digits)
	return n, ok && err == nil && n >= 0 && n < len(s.html) && strconv.Itoa(n) == digits
}

// within returns the numbers of the marks of end ("s" or "e") text holds, whatever its case.
func (s *items) within(text, end string) []int {
	var found []int
	text = strings.ToLower(text)
	for i := range s.html {
		if strings.Contains(text, s.mark+" "+strconv.Itoa(i)+" "+end) {
			found = append(found, i)
		}
	}
	return found
}

// end returns the comment that ends the item of o, the first after o in tree; nil for none.
func (s *items) end(tree *html.Node, o occurrence) *html.Node {
	past := false
	for n := range tree.Descendants() {
		if n == o.node {
			past = true
			continue
		}
		if i, ok := s.number(n.Data, "e"); past && ok && n.Type == html.CommentNode && i == o.n {
			return n
		}
	}
	return nil
}

// lost says what took in the end mark of the item of o: the comment, the element whose text or attribute holds
// it; else, the parser having dropped it in an end tag or a doctype, the tag the item's text ends in.
func (s *items) lost(tree *html.Node, o occurrence) string {
	past := false
	for n := range tree.Descendants() {
		if past {
			if holder := s.holding(n, o.n); holder != "" {
				return holder
			}
		}
		past = past || n == o.node
	}
	return unfinished(s.texts[o.n])
}

// holding says what n is that holds the end mark of the item i: a comment, as <!--, or the element whose text
// or attribute it is in; "" for none.
func (s *items) holding(n *html.Node, i int) string {
	switch n.Type {
	case html.CommentNode:
		if slices.Contains(s.within(n.Data, "e"), i) {
			return "<!--"
		}
	case html.TextNode:
		if slices.Contains(s.within(n.Data, "e"), i) && n.Parent != nil {
			return "<" + n.Parent.Data + ">"
		}
	case html.ElementNode:
		for _, a := range n.Attr {
			if slices.Contains(s.within(a.Key+" "+a.Val, "e"), i) {
				return "<" + n.Data + ">"
			}
		}
	default:
	}
	return ""
}

// unfinished returns where text, read alone, ends in a tag it does not finish: what is past the last token a
// tokenizer reads whole, else the last token, which the tokenizer ended at the end; 40 characters of it at most.
func unfinished(text string) string {
	z := html.NewTokenizer(strings.NewReader(text))
	end, last := 0, ""
	for z.Next() != html.ErrorToken {
		end += len(z.Raw())
		last = string(z.Raw())
	}
	rest := text[end:]
	if rest == "" {
		rest = last
	}
	if utf8.RuneCountInString(rest) > 40 {
		rest = string([]rune(rest)[:40]) + "…"
	}
	return rest
}

// changed says how an item between the marks start and end changed the page around it: the elements it left
// open, outside in, and those of the page it closed, inside out; or the formatting elements the probe after it
// was rebuilt in; "" for none.
func changed(start, end *html.Node) string {
	if start.Parent != end.Parent {
		var open, closed []string
		for p := end.Parent; p != nil && !holds(p, start); p = p.Parent {
			open = append([]string{"<" + p.Data + ">"}, open...)
		}
		for p := start.Parent; p != nil && !holds(p, end); p = p.Parent {
			closed = append(closed, "</"+p.Data+">")
		}
		return strings.Join(append(open, closed...), " ")
	}
	next := end.NextSibling
	if next != nil && next.Type == html.TextNode && strings.HasPrefix(next.Data, probe) {
		return ""
	}
	var rebuilt []string
	for ; next != nil && next.Type == html.ElementNode; next = next.FirstChild {
		rebuilt = append(rebuilt, "<"+next.Data+">")
	}
	if rebuilt == nil {
		// The probe is gone: the page put the item where text does not stay.
		return "<" + end.Parent.Data + ">"
	}
	return strings.Join(rebuilt, " ")
}

// holds reports whether n is p or under it.
func holds(p, n *html.Node) bool {
	for ; n != nil; n = n.Parent {
		if n == p {
			return true
		}
	}
	return false
}

// unmark takes the marks of the items, and their probes, out of tree, checked.
func (s *items) unmark(tree *html.Node) {
	var marks []*html.Node
	for n := range tree.Descendants() {
		if n.Type != html.CommentNode {
			continue
		}
		if _, ok := s.number(n.Data, "s"); ok {
			trim(n.PrevSibling, strings.TrimSuffix)
			marks = append(marks, n)
		}
		if _, ok := s.number(n.Data, "e"); ok {
			trim(n.NextSibling, strings.TrimPrefix)
			marks = append(marks, n)
		}
	}
	for _, n := range marks {
		n.Parent.RemoveChild(n)
	}
}

// trim takes the probe off the text n by cut, and n out of its parent once it is empty.
func trim(n *html.Node, cut func(s, probe string) string) {
	if n == nil || n.Type != html.TextNode {
		return
	}
	if n.Data = cut(n.Data, probe); n.Data == "" {
		n.Parent.RemoveChild(n)
	}
}
