package pagediff

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"floe/internal/usererr"
)

// block is what a page holds in one place: a paragraph, an item, a row, a line of code, a diagram or an image.
// key is what it is compared by.
type block struct {
	kind, text, key string
}

// section is what lies under a heading, up to the next of any level. level 0 is what comes before the first heading.
type section struct {
	heading string
	level   int
	blocks  []block
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

// read reads the page as a browser does into its sections.
// LIMIT: a kept version past the parser's depth (read again, see draw.CutOffPage) cannot be compared: the writer cannot
// mend a version kept, and the words do not say which. Keep them apart here if such pages come to be written.
func read(page []byte) ([]section, error) {
	root, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, usererr.Library(err) //nolint:wrapcheck // the library's words
	}
	r := &reader{sections: []section{{heading: "", level: 0, blocks: nil}}, kind: "", words: strings.Builder{}}
	r.walk(root)
	r.flush()
	if len(r.sections[0].blocks) == 0 {
		return r.sections[1:], nil
	}
	return r.sections, nil
}

// reader goes through a page in order. words are those of the block being read.
type reader struct {
	sections []section
	kind     string
	words    strings.Builder
}

func (r *reader) add(kind, text, key string) {
	s := &r.sections[len(r.sections)-1]
	s.blocks = append(s.blocks, block{kind: kind, text: text, key: kind + "\x00" + key})
}

func (r *reader) flush() {
	if text := squeeze(r.words.String()); text != "" {
		r.add(r.kind, text, text)
	}
	r.words.Reset()
}

func (r *reader) walk(n *html.Node) {
	switch n.Type { //nolint:exhaustive // the others hold nothing read
	case html.TextNode:
		r.words.WriteString(n.Data)
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
		r.add("svg", text, svgKey(n))
	case n.Data == "img":
		r.flush()
		alt := squeeze(attr(n, "alt"))
		r.add("img", alt, alt+"\x00"+digest(attr(n, "src")))
	case n.Data == "tr":
		r.flush()
		text := row(n)
		r.add("tr", text, text)
	case n.Data == "pre":
		r.flush()
		for _, line := range preLines(n) {
			r.add("pre", line, line)
		}
	case headingLevel(n) > 0:
		r.flush()
		r.sections = append(r.sections, section{heading: flowOf(n), level: headingLevel(n), blocks: nil})
	case blockNames[n.Data]:
		r.flush()
		outer := r.kind
		r.kind = n.Data
		r.children(n)
		r.flush()
		r.kind = outer
	case wordNames[n.Data]:
		r.children(n)
	default:
		r.words.WriteByte(' ')
		r.children(n)
		r.words.WriteByte(' ')
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

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// flow writes the words under n as they read: an element that is no mark of words stands apart from what is beside it.
func flow(n *html.Node, b *strings.Builder) {
	if n.Type == html.TextNode {
		b.WriteString(n.Data)
		return
	}
	// LIMIT: of an image in a line or a heading, its alt is told, not its src, and of a diagram there, nothing.
	if n.Data == "img" {
		b.WriteString(" " + attr(n, "alt") + " ")
		return
	}
	if unread[n.Data] || n.Namespace == "svg" {
		return
	}
	apart := n.Type == html.ElementNode && !wordNames[n.Data]
	if apart {
		b.WriteByte(' ')
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		flow(c, b)
	}
	if apart {
		b.WriteByte(' ')
	}
}

func flowOf(n *html.Node) string {
	var b strings.Builder
	flow(n, &b)
	return squeeze(b.String())
}

func row(n *html.Node) string {
	var cells []string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode {
			cells = append(cells, flowOf(c))
		}
	}
	return strings.Join(cells, " | ")
}

func plain(n *html.Node, b *strings.Builder) {
	if n.Type == html.TextNode {
		b.WriteString(n.Data)
		return
	}
	if unread[n.Data] {
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		plain(c, b)
	}
}

// preLines cuts what is in a pre into lines, its spaces kept: those of code, diff and terminal by their elements (the
// code family writes each line as a child of its pre, or of the one code in it), a plain pre's by its newlines. Words
// only a screen reader reads go at the head of the line after them.
// LIMIT: a pre that colours words within a line by elements is cut at them. floe's parts draw none.
// LIMIT: blank lines at the end of a line's element or of a pre are not lines, and adding them tells nothing; code's
// are lines, each its own element.
// LIMIT: what CSS draws is no text here: a page whose code's start alone changes, its numbers a counter, tells nothing.
func preLines(n *html.Node) []string {
	box := n
	if c := n.FirstChild; c != nil && c == n.LastChild && c.Type == html.ElementNode && c.Data == "code" {
		box = c
	}
	var lines []string
	head := ""
	for c := box.FirstChild; c != nil; c = c.NextSibling {
		// Between elements each a line, the newline that sets them apart in the HTML is no line.
		if c.Type == html.TextNode && strings.TrimSpace(c.Data) == "" && hasElement(box) {
			continue
		}
		var b strings.Builder
		plain(c, &b)
		if c.Type == html.ElementNode && slices.Contains(strings.Fields(attr(c, "class")), "floe-hidden") {
			head += strings.TrimSpace(b.String()) + " "
			continue
		}
		for line := range strings.SplitSeq(strings.TrimRight(b.String(), "\n"), "\n") {
			lines = append(lines, head+line)
			head = ""
		}
	}
	if head != "" {
		lines = append(lines, strings.TrimSpace(head))
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
			var b strings.Builder
			plain(m, &b)
			texts = append(texts, squeeze(b.String()))
		}
	}
	return strings.Join(texts, " / ")
}

func digest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// svgKey is the digest of the svg as written, its ids numbered in its own order and what names them with them: a
// diagram's ids hold its number in the page, which a diagram added before it changes. It writes on the tree, read no
// further.
func svgKey(n *html.Node) string {
	ids := map[string]string{}
	note := func(m *html.Node) {
		if id := attr(m, "id"); id != "" && ids[id] == "" {
			ids[id] = "id" + strconv.Itoa(len(ids))
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
			case "aria-labelledby", "aria-describedby":
				words := strings.Fields(a.Val)
				for j, word := range words {
					words[j] = cmp.Or(ids[word], word)
				}
				m.Attr[i].Val = strings.Join(words, " ")
			default:
				m.Attr[i].Val = numberedRefs(a.Val, ids)
			}
		}
	}
	number(n)
	for m := range n.Descendants() {
		number(m)
	}
	var out bytes.Buffer
	// LIMIT: Render stops at a void element holding something, so that two versions differing only past it have one
	// digest, and a change to a diagram leaves the diff unsaid. Only a page read again can hold one, as draw refuses
	// it when writing; compare the trees if that comes to be seen.
	_ = html.Render(&out, n) //nolint:errcheck // see above
	return digest(out.String())
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
