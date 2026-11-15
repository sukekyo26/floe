package pagediff

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"

	"floe/internal/page"
	"floe/internal/usererr"
)

// block is what a page holds in one place: a paragraph, an item, a row, a line of code, a diagram or an image.
// key is what it is compared by; node the element that holds it, and from where each rune of text comes from.
type block struct {
	kind, text, key string
	node            *html.Node
	from            []origin
	// cells are a row's cells, in order.
	cells []cell
}

// cell is a cell of a row: its element, its words, and where each rune of them comes from.
type cell struct {
	node *html.Node
	text string
	from []origin
}

// section is what lies under a heading, up to the next of any level. level 0 is what comes before the first heading,
// whose head is nil.
type section struct {
	heading string
	level   int
	head    *html.Node
	from    []origin
	blocks  []block
}

// origin is where a rune of what is read comes from: the byte at of the text node n, or no text node (n nil) for a
// space set between elements, the bar between a row's cells or an image's alt.
type origin struct {
	n  *html.Node
	at int
}

// words are runes read with where each comes from.
type words struct {
	b    strings.Builder
	from []origin
}

func (w *words) add(s string, n *html.Node) {
	for i := range s {
		w.from = append(w.from, origin{n: n, at: i})
	}
	w.b.WriteString(s)
}

func (w *words) addApart(s string) {
	w.add(s, nil)
}

// addRead adds words read already, from telling where each rune of them comes from.
func (w *words) addRead(s string, from []origin) {
	w.b.WriteString(s)
	w.from = append(w.from, from...)
}

func (w *words) reset() {
	w.b.Reset()
	w.from = w.from[:0]
}

// squeezed is the words as squeeze writes them, with where each rune comes from: a run of spaces is its first.
func (w *words) squeezed() (string, []origin) {
	var b strings.Builder
	var from []origin
	space := -1
	i := 0
	for _, r := range w.b.String() {
		if unicode.IsSpace(r) {
			if space < 0 {
				space = i
			}
			i++
			continue
		}
		if space >= 0 && b.Len() > 0 {
			b.WriteByte(' ')
			from = append(from, w.from[space])
		}
		space = -1
		b.WriteRune(r)
		from = append(from, w.from[i])
		i++
	}
	return b.String(), from
}

// LIMIT: the HTML's elements alone are read, not its CSS: a span drawn as a block runs on in its block's words, and a
// div drawn inline
// is a block of its own. It only tells how the words read, not whether they differ.
// LIMIT: words alone are compared: making words bold, a link's target, a heading's level and CSS tell nothing. The
// steps part says nothing
// of its state doing, so that todo to doing tells nothing either: give it words if that comes to trouble someone.
var (
	// What is no part of what a reader reads: a nav repeats the headings, a button is a page's control, and what is in a
	// progress or a
	// meter is the words for a browser that cannot draw it, which the part says beside it.
	unread = names("head script style template noscript nav button progress meter input")
	// Elements that stand on their own, as HTML draws them by default.
	blockNames = names(`html body address article aside blockquote caption details dd div dl dt fieldset figcaption
		figure footer
		form header hr label li main menu ol p section summary table tbody tfoot thead ul`)
	// Elements that mark words within a line: other elements between words are apart, as parts lay out their spans side by
	// side.
	wordNames = names(`a abbr acronym b bdi bdo big cite code data del dfn em font i ins kbd mark nobr q rb rp rt rtc ruby
		s samp small strike strong sub sup time tt u var wbr`)
)

func names(list string) map[string]bool {
	set := map[string]bool{}
	for name := range strings.FieldsSeq(list) {
		set[name] = true
	}
	return set
}

// readTree reads the page as a browser does into its sections, with the tree it reads, which it leaves as the
// parser made it.
// LIMIT: a kept version past the parser's depth (read again, see draw.CutOffPage) cannot be compared: the writer cannot
// mend a version kept, and the words do not say which. Keep them apart here if such pages come to be written.
func readTree(page []byte) (*html.Node, []section, error) {
	root, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, nil, usererr.Library(err) //nolint:wrapcheck // the library's words
	}
	r := &reader{
		sections: []section{{heading: "", level: 0, head: nil, from: nil, blocks: nil}}, kind: "", node: nil,
		words: words{b: strings.Builder{}, from: nil},
	}
	r.walk(root)
	r.flush()
	if len(r.sections[0].blocks) == 0 {
		return root, r.sections[1:], nil
	}
	return root, r.sections, nil
}

// reader goes through a page in order. words are those of the block being read, in node.
type reader struct {
	sections []section
	kind     string
	node     *html.Node
	words    words
}

func (r *reader) add(kind, text, key string, node *html.Node, from []origin) {
	s := &r.sections[len(r.sections)-1]
	s.blocks = append(s.blocks, block{
		kind: kind, text: text, key: kind + "\x00" + key, node: node, from: from, cells: nil,
	})
}

func (r *reader) flush() {
	if text, from := r.words.squeezed(); text != "" {
		r.add(r.kind, text, text, r.node, from)
	}
	r.words.reset()
}

func (r *reader) walk(n *html.Node) {
	switch n.Type { //nolint:exhaustive // the others hold nothing read
	case html.TextNode:
		r.words.add(n.Data, n)
		return
	case html.DocumentNode:
		r.children(n)
		return
	case html.ElementNode:
	default:
		return
	}
	switch {
	case unread[n.Data]:
	case n.Namespace == "svg" && n.Data == "svg":
		r.flush()
		text := svgText(n)
		r.add("svg", text, svgKey(n), n, nil)
	case n.Data == "img":
		r.flush()
		alt, _ := page.Attr(n, "alt")
		src, _ := page.Attr(n, "src")
		alt = squeeze(alt)
		r.add("img", alt, alt+"\x00"+digest(src), n, nil)
	case n.Data == "tr":
		r.flush()
		text, from, cells := row(n)
		r.add("tr", text, text, n, from)
		s := &r.sections[len(r.sections)-1]
		s.blocks[len(s.blocks)-1].cells = cells
	case n.Data == "pre":
		r.flush()
		for _, line := range preLines(n) {
			r.add("pre", line.text, line.text, n, line.from)
		}
	case headingLevel(n) > 0:
		r.flush()
		text, from := flowOf(n)
		r.sections = append(r.sections, section{heading: text, level: headingLevel(n), head: n, from: from, blocks: nil})
	case blockNames[n.Data]:
		r.flush()
		outerKind, outerNode := r.kind, r.node
		r.kind, r.node = n.Data, n
		r.children(n)
		r.flush()
		r.kind, r.node = outerKind, outerNode
	case wordNames[n.Data]:
		r.children(n)
	default:
		r.words.addApart(" ")
		r.children(n)
		r.words.addApart(" ")
	}
}

func (r *reader) children(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		r.walk(c)
	}
}

func headingLevel(n *html.Node) int {
	if len(n.Data) == 2 && n.Data[0] == 'h' && n.Data[1] >= '1' && n.Data[1] <= '6' && n.Namespace == "" {
		return int(n.Data[1] - '0')
	}
	return 0
}

func squeeze(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// flow writes the words under n as they read: an element that is no mark of words stands apart from what is beside it.
func flow(n *html.Node, w *words) {
	if n.Type == html.TextNode {
		w.add(n.Data, n)
		return
	}
	// LIMIT: of an image in a line or a heading, its alt is told, not its src, and of a diagram there, nothing.
	if n.Data == "img" {
		alt, _ := page.Attr(n, "alt")
		w.addApart(" " + alt + " ")
		return
	}
	if unread[n.Data] || n.Namespace == "svg" {
		return
	}
	apart := n.Type == html.ElementNode && !wordNames[n.Data]
	if apart {
		w.addApart(" ")
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		flow(c, w)
	}
	if apart {
		w.addApart(" ")
	}
}

func flowOf(n *html.Node) (string, []origin) {
	var w words
	flow(n, &w)
	return w.squeezed()
}

func row(n *html.Node) (string, []origin, []cell) {
	var w words
	var cells []cell
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		if len(cells) > 0 {
			w.addApart(" | ")
		}
		text, from := flowOf(c)
		cells = append(cells, cell{node: c, text: text, from: from})
		w.addRead(text, from)
	}
	return w.b.String(), w.from, cells
}

func plain(n *html.Node, w *words) {
	if n.Type == html.TextNode {
		w.add(n.Data, n)
		return
	}
	if unread[n.Data] {
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		plain(c, w)
	}
}

// line is a line of a pre, with where each rune comes from.
type line struct {
	text string
	from []origin
}

// preLines cuts what is in a pre into lines, its spaces kept: those of code, diff and terminal by their elements (the
// code family writes each line as a child of its pre, or of the one code in it), a plain pre's by its newlines. Words
// only a screen reader reads go at the head of the line after them.
// LIMIT: a pre that colours words within a line by elements is cut at them. floe's parts draw none.
// LIMIT: blank lines at the end of a line's element or of a pre are not lines, and adding them tells nothing; code's
// are lines, each its own element.
// LIMIT: what CSS draws is no text here: a page whose code's start alone changes, its numbers a counter, tells nothing.
func preLines(n *html.Node) []line {
	box := n
	if c := n.FirstChild; c != nil && c == n.LastChild && c.Type == html.ElementNode && c.Data == "code" {
		box = c
	}
	var lines []line
	head := ""
	for c := box.FirstChild; c != nil; c = c.NextSibling {
		// Between elements each a line, the newline that sets them apart in the HTML is no line.
		if c.Type == html.TextNode && strings.TrimSpace(c.Data) == "" && hasElement(box) {
			continue
		}
		var w words
		plain(c, &w)
		all := w.b.String()
		if c.Type == html.ElementNode && page.HasClass(c, "floe-hidden") {
			head += strings.TrimSpace(all) + " "
			continue
		}
		// The words the screen reader alone reads come from no place of the line.
		at := 0
		for text := range strings.SplitSeq(strings.TrimRight(all, "\n"), "\n") {
			n := utf8.RuneCountInString(text)
			from := make([]origin, utf8.RuneCountInString(head), utf8.RuneCountInString(head)+n)
			from = append(from, w.from[at:at+n]...)
			at += n + 1
			lines = append(lines, line{text: head + text, from: from})
			head = ""
		}
	}
	if head != "" {
		text := strings.TrimSpace(head)
		lines = append(lines, line{text: text, from: make([]origin, utf8.RuneCountInString(text))})
	}
	return lines
}

func hasElement(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			return true
		}
	}
	return false
}

func svgText(n *html.Node) string {
	var texts []string
	for m := range n.Descendants() {
		if m.Type == html.ElementNode && m.Data == "text" {
			var w words
			plain(m, &w)
			texts = append(texts, squeeze(w.b.String()))
		}
	}
	return strings.Join(texts, " / ")
}

func digest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// svgKey is the digest of the svg as written, its ids numbered in its own order and what names them with them: a
// diagram's ids hold its number in the page, which a diagram added before it changes. It numbers a copy, the page's
// tree left as it was drawn.
func svgKey(n *html.Node) string {
	n, _ = copyTree(n)
	renumber(n, func(int, string) string { return "" })
	var out bytes.Buffer
	// LIMIT: Render stops at a void element holding something, so that two versions differing only past it have one
	// digest, and a change to a diagram leaves the diff unsaid. Only a page read again can hold one, as draw refuses
	// it when writing; compare the trees if that comes to be seen.
	_ = html.Render(&out, n) //nolint:errcheck // see above
	return digest(out.String())
}

// renumber gives each id under n, n's own too, the name named makes of its place in n's order and the id, and all that
// names one of them by it (aria-labelledby and aria-describedby, for, #id in other values) the new name too. A name
// attribute is named alike, so that a copy's radios and details make no group with the page's.
func renumber(n *html.Node, named func(i int, id string) string) {
	ids := map[string]string{}
	note := func(m *html.Node) {
		if id, _ := page.Attr(m, "id"); id != "" && ids[id] == "" {
			ids[id] = cmp.Or(named(len(ids), id), "id"+strconv.Itoa(len(ids)))
		}
	}
	note(n)
	for m := range n.Descendants() {
		note(m)
	}
	number := func(m *html.Node) {
		for i, a := range m.Attr {
			switch a.Key {
			case "id":
				m.Attr[i].Val = ids[a.Val]
			case "aria-labelledby", "aria-describedby", "for":
				words := strings.Fields(a.Val)
				for j, word := range words {
					words[j] = cmp.Or(ids[word], word)
				}
				m.Attr[i].Val = strings.Join(words, " ")
			case "name":
				if a.Val != "" {
					m.Attr[i].Val = cmp.Or(named(-1, a.Val), a.Val)
				}
			default:
				m.Attr[i].Val = numberedRefs(a.Val, ids)
			}
		}
	}
	number(n)
	for m := range n.Descendants() {
		number(m)
	}
}

// numberedRefs is the value with each #id in it, as in href="#id" and url(#id), an id of ids, numbered.
func numberedRefs(value string, ids map[string]string) string {
	var b strings.Builder
	for {
		before, after, found := strings.Cut(value, "#")
		b.WriteString(before)
		if !found {
			return b.String()
		}
		b.WriteByte('#')
		end := strings.IndexAny(after, " \t\n\f\r)\"',;")
		if end < 0 {
			end = len(after)
		}
		b.WriteString(cmp.Or(ids[after[:end]], after[:end]))
		value = after[end:]
	}
}
