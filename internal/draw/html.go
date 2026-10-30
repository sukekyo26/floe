package draw

import (
	"cmp"
	"crypto/rand"
	"fmt"
	"html/template"
	"io/fs"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/sukekyo26/floe/internal/diagram"
	"github.com/sukekyo26/floe/internal/dirs"
	"github.com/sukekyo26/floe/internal/doc"
	"github.com/sukekyo26/floe/internal/kit"
	"github.com/sukekyo26/floe/internal/usererr"
	"github.com/sukekyo26/floe/internal/yamlpos"
)

// probe is the white space drawn around the item n of format html, its spaces counting it, so that it tells the
// item where HTML's tags are stripped (an attribute's value). A parser rebuilds the formatting elements left
// open (<b>, <a>) before it puts in text, which comments do not make it do: the probe after an item is rebuilt
// into one the item left open, and the one before, into one the page left open, so that the item's marks share
// it. White space goes in as it is in a table and in a head.
func probe(n int) string {
	return "\t\n" + strings.Repeat(" ", n+1) + "\t"
}

// items are the items of one drawing that floe draws, and what they need: those of format html, each marked so
// that the page drawn tells whether it changes the page around it, the images of the data's directory, each
// read once, and the icons of the diagrams, counted so that the ids of each differ.
type items struct {
	secret    string     // of the marks, in lower case as a parser makes tag and attribute names
	edge      string     // the text of the comment drawn outside each probe: the secret and a letter, no mark
	html      [][]string // the path of each item of format html, by its number
	texts     []string   // its text
	images    fs.FS      // the data's images, nil for none
	imagesDir string     // the directory mistakes name them under, "" for the data file's
	read      map[string]image
	icons     diagram.Icons
	diagrams  int
	warnings  []*usererr.Error // of the items drawn
}

// image is an image read, or its mistake.
type image struct {
	uri template.URL
	err error
}

// newItems returns the items of a drawing whose data's images are images (nil for none), named in mistakes
// under imagesDir ("" for the data file's directory), and whose diagrams show the icons of icons (nil for none
// kept).
func newItems(images fs.FS, imagesDir string, icons diagram.Icons) *items {
	secret := strings.ToLower(rand.Text())
	return &items{
		secret: secret, edge: secret + "p", html: nil, texts: nil, images: images, imagesDir: imagesDir,
		read: map[string]image{}, icons: icons, diagrams: 0, warnings: nil,
	}
}

// marked returns text, the item of format html at the path at, between its marks and the probes.
func (s *items) marked(at []string, text string) template.HTML {
	n := len(s.html)
	s.html = append(s.html, at)
	s.texts = append(s.texts, text)
	start, end := s.around(n)
	return template.HTML(start + text + end) //nolint:gosec // G203: the data's HTML, checked in the page drawn (check)
}

// around returns what is drawn before and after the item n: the probes and the marks, each probe between a mark
// and an edge, a comment of the secret, so that it is a text of its own: text page.html writes beside it is no
// part of it, which the parser would move with it out of a table. A part's output trimmed keeps it too.
func (s *items) around(n int) (before, after string) {
	edge := "<!--" + s.edge + "-->"
	return edge + probe(n) + "<!--" + s.mark(n, "s") + "-->", "<!--" + s.mark(n, "e") + "-->" + probe(n) + edge
}

// plain returns text without the probes, the marks and the edges of the items, as they are drawn around them: the
// text of a mistake page.html or a part's part.html made with an item.
func (s *items) plain(text string) string {
	if len(s.html) == 0 {
		return text
	}
	pairs := make([]string, 0, 4*len(s.html))
	for n := range s.html {
		start, end := s.around(n)
		pairs = append(pairs, start, "", end, "")
	}
	return strings.NewReplacer(pairs...).Replace(text)
}

// plainArgs returns args, of a mistake, each text plain (without the probes and the marks).
func (s *items) plainArgs(args []any) []any {
	plain := slices.Clone(args)
	for i, a := range plain {
		if text, ok := a.(string); ok {
			plain[i] = s.plain(text)
		}
	}
	return plain
}

// image reads the image name of d, once a drawing.
func (s *items) image(d *doc.Data, name string) (template.URL, error) {
	if got, ok := s.read[name]; ok {
		return got.uri, got.err
	}
	if s.images == nil {
		return "", fmt.Errorf("draw: no images to read the image %s from", name) //nolint:err113 // the caller's mistake
	}
	var got image
	got.uri, got.err = kit.ReadImage(s.images, name, cmp.Or(s.imagesDir, dirs.Dir(d.File)), d.File)
	s.read[name] = got
	return got.uri, got.err
}

// mark is the text of the comment that starts (end "s") or ends (end "e") the item n: the secret, then n, of
// letters and digits alone, which escaping in a URL or a script leaves as they are.
func (s *items) mark(n int, end string) string {
	return s.secret + "n" + strconv.Itoa(n) + end
}

// occurrence is a mark the page drawn holds, in the order of the page: of the item n, its start or its end; the
// comment drawn when exact, else in what holder says (<!-- for a comment, an element's start tag for its text
// or an attribute).
type occurrence struct {
	n      int
	end    bool
	node   *html.Node
	exact  bool
	holder string
}

// marksIn returns the marks tree holds, in its order, walking it once: each comment that is one, and each in a
// text, a comment or an attribute, however escaped there.
func (s *items) marksIn(tree *html.Node) []occurrence {
	var found []occurrence
	// The items open there: between the comments of an item, its own attributes are no page.html's.
	open := map[int]bool{}
	for n := range tree.Descendants() {
		switch n.Type {
		case html.CommentNode:
			if i, end, ok := s.exact(n.Data); ok {
				found = append(found, occurrence{n: i, end: end, node: n, exact: true, holder: ""})
				if end {
					delete(open, i)
				} else {
					open[i] = true
				}
				continue
			}
			found = append(found, s.within(n, n.Data, "<!--")...)
		case html.TextNode:
			if n.Parent != nil {
				found = append(found, s.within(n, n.Data, "<"+n.Parent.Data+">")...)
			}
		case html.ElementNode:
			found = append(found, s.inAttrs(n, len(open) == 0)...)
		default:
		}
	}
	return found
}

// inAttrs returns the marks the attributes of n, an element, hold; and, when it is page.html's (probed), the
// probes.
func (s *items) inAttrs(n *html.Node, probed bool) []occurrence {
	var found []occurrence
	for _, a := range n.Attr {
		found = append(found, s.within(n, a.Key+" "+a.Val, "<"+n.Data+">")...)
		if probed {
			found = append(found, s.probed(n, a.Val)...)
		}
	}
	return found
}

// probed returns the items whose probe value, an attribute of n, holds, read once through it: html/template
// strips an item's tags, its marks among them, from an attribute's value, and leaves the probes.
// LIMIT: an attribute page.html or a part's part.html writes that holds the white space of a probe (\t\n, spaces,
// \t) is told as an item put there; one an item writes is not looked at, as it is between the item's marks.
func (s *items) probed(n *html.Node, value string) []occurrence {
	var found []occurrence
	for {
		at := strings.Index(value, "\t\n")
		if at < 0 {
			return found
		}
		value = value[at+2:]
		spaces := len(value) - len(strings.TrimLeft(value, " "))
		if i := spaces - 1; i >= 0 && i < len(s.html) && strings.HasPrefix(value[spaces:], "\t") {
			found = append(found, occurrence{n: i, end: false, node: n, exact: false, holder: "<" + n.Data + ">"})
		}
	}
}

// exact reads a comment's text that is a mark as drawn: its item, and whether it is the end.
func (s *items) exact(text string) (n int, end, ok bool) {
	n, end, rest, ok := s.next(text)
	return n, end, ok && rest == "" && strings.HasPrefix(text, s.secret)
}

// within returns the marks text, of the node n, holds, whatever its case, holder said of each.
func (s *items) within(n *html.Node, text, holder string) []occurrence {
	var found []occurrence
	text = strings.ToLower(text)
	for {
		at := strings.Index(text, s.secret)
		if at < 0 {
			return found
		}
		i, end, rest, ok := s.next(text[at:])
		if ok {
			found = append(found, occurrence{n: i, end: end, node: n, exact: false, holder: holder})
		}
		text = rest
	}
}

// next reads the mark text starts with, the secret first: its item and whether it is the end, and the text after
// it; ok is false for no mark of an item drawn, and rest is then past the secret.
func (s *items) next(text string) (n int, end bool, rest string, ok bool) {
	rest, _ = strings.CutPrefix(text, s.secret)
	digits, ok := strings.CutPrefix(rest, "n")
	if !ok {
		return 0, false, rest, false
	}
	k := strings.IndexFunc(digits, func(r rune) bool { return r < '0' || r > '9' })
	if k <= 0 || k >= len(digits) || digits[k] != 's' && digits[k] != 'e' {
		return 0, false, rest, false
	}
	n, err := strconv.Atoi(digits[:k])
	if err != nil || n >= len(s.html) || strconv.Itoa(n) != digits[:k] {
		return 0, false, rest, false
	}
	return n, digits[k] == 'e', digits[k+1:], true
}

// check refuses each item of format html of d that changes the page tree, drawn, around it, at the item, and an
// item page.html or a part's part.html puts where HTML does not work (a mark of it is not a comment of its own),
// at d's template:. They are looked at in the order of the page; past the first that changed the page, which the
// others are read in, none is. It refuses at the item a name of a part its elements give that the page would be
// refused for (checkNames).
// LIMIT: the marks and the probe show the tree and the formatting rebuilt in body text alone: the form an item
// left open and the attributes it adds to body or html are not checked; check them in the browser (B12).
// page.html puts an item where elements go; between html and head, the probe is dropped and a right item is
// refused.
// LIMIT: in a table, tbody, thead, tfoot, tr or colgroup, the parser's foster parenting decides. Refused: what is
// not of that place (rows straight in a table, read into a tbody of their own; cells in a tbody; rows in a tr;
// text in a colgroup), and formatting left open at the item's end. Drawn moved, unchecked by the marks and the
// probe but for what checkPage refuses in the whole page: the item's text and elements that are no part of a
// table, and text page.html writes beside the item, put before the table, and formatting left open before its
// rows or misnested (<b><i>x</b>), rebuilt in what page.html writes after the table. A name of a part moved out
// of a table is told at template:, by checkPage. Probe with text the parser moves too if pages come to put items
// there.
func (s *items) check(k *kit.Kit, ns names, d *doc.Data, tree *html.Node) error {
	if len(s.html) == 0 {
		return nil
	}
	marks := s.marksIn(tree)
	var found usererr.List
	for i, o := range marks {
		if o.end && o.exact {
			continue
		}
		pos := d.PosOf(s.html[o.n])
		refuse := func(key string, args ...any) { found = append(found, yamlpos.New(d.File, pos, key, args...)) }
		if !o.exact {
			// The end of an item started as a comment is read with its start, below.
			// LIMIT: told at template:, whether page.html or a part's part.html put the item there: which of them did
			// is not known without marks around a part's output too, which would refuse part.html left open as an
			// item's is. Mark a part's output if one comes to need it told at part.html.
			found = append(found, yamlpos.New(d.File, d.Where["template"], "draw_html_placed",
				strings.Join(s.html[o.n], "."), o.holder))
			break
		}
		end := slices.IndexFunc(marks[i+1:], func(p occurrence) bool { return p.n == o.n && p.end })
		if end < 0 {
			refuse("draw_html_whole", unfinished(s.texts[o.n]))
			break
		}
		if e := marks[i+1+end]; !e.exact {
			refuse("draw_html_whole", e.holder)
			break
		} else if where := changed(o.n, o.node, e.node); where != "" {
			refuse("draw_html_whole", where)
			break
		}
		for n := o.node.NextSibling; n != marks[i+1+end].node; n = n.NextSibling {
			checkNames(k, ns, n, map[string]bool{}, map[string]bool{}, refuse)
			for e := range n.Descendants() {
				checkNames(k, ns, e, map[string]bool{}, map[string]bool{}, refuse)
			}
		}
	}
	return found.Err()
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
	return yamlpos.Head(rest)
}

// changed says how an item between the marks start and end changed the page around it: the elements it left
// open, outside in, and those of the page it closed, inside out; or the formatting elements the probe after it
// was rebuilt in; "" for none.
func changed(n int, start, end *html.Node) string {
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
	if next != nil && next.Type == html.TextNode && strings.HasPrefix(next.Data, probe(n)) {
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

// unmark takes the marks of the items, their probes and the edges, out of tree, checked.
func (s *items) unmark(tree *html.Node) {
	var marks []*html.Node
	for n := range tree.Descendants() {
		if n.Type != html.CommentNode {
			continue
		}
		i, end, ok := s.exact(n.Data)
		switch {
		case n.Data == s.edge:
		case !ok:
			continue
		case end:
			trim(n.NextSibling, probe(i), strings.TrimPrefix)
		default:
			trim(n.PrevSibling, probe(i), strings.TrimSuffix)
		}
		marks = append(marks, n)
	}
	for _, n := range marks {
		n.Parent.RemoveChild(n)
	}
}

// trim takes probe off the text n by cut, and n out of its parent once it is empty.
func trim(n *html.Node, probe string, cut func(s, probe string) string) {
	if n == nil || n.Type != html.TextNode {
		return
	}
	if n.Data = cut(n.Data, probe); n.Data == "" {
		n.Parent.RemoveChild(n)
	}
}
