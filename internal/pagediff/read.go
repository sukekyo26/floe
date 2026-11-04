package pagediff

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/net/html"
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
var (
	// What is no part of what a reader reads: a nav repeats the headings, a button is a page's control, and what is in a
	// progress or a
	// meter is the words for a browser that cannot draw it, which the part says beside it.
	unread = names("head script style template noscript nav button progress meter input")
	// Elements that stand on their own, as HTML draws them by default.
	blockNames = names(`html body address article aside blockquote caption details dd div dl dt fieldset figcaption figure footer
		form header hr label li main menu ol p section summary table tbody tfoot thead ul`)
	// Elements that mark words within a line: other elements between words are apart, as parts lay out their spans side by
	// side.
	wordNames = names(`a abbr b bdi bdo cite code del dfn em i ins kbd mark q rp rt ruby s samp strong sub sup u
		var wbr`)
)

func names(list string) map[string]bool {
	set := map[string]bool{}
	for name := range strings.FieldsSeq(list) {
		set[name] = true
	}
	return set
}

// read reads the page as a browser does into its sections.
func read(page []byte) ([]section, error) {
	root, err := html.Parse(bytes.NewReader(page))
	if err != nil {
		return nil, fmt.Errorf("read a version of the page: %w", err)
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
		r.add("svg", svgText(n), svgKey(n))
	case n.Data == "img":
		r.flush()
		r.add("img", squeeze(attr(n, "alt")), digest(attr(n, "src")))
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

// preLines cuts what is in a pre into lines, its spaces kept: code's are told by its newlines, the lines of diff and
// terminal by
// their elements (the code family writes each line as a child of its pre, or of the one code in it). Words only a
// screen reader
// reads go at the head of the line after them.
// LIMIT: a pre that colours words within a line by elements is cut at them. floe's parts draw none.
func preLines(n *html.Node) []string {
	box := n
	if c := n.FirstChild; c != nil && c == n.LastChild && c.Type == html.ElementNode && c.Data == "code" {
		box = c
	}
	var lines []string
	head := ""
	for c := box.FirstChild; c != nil; c = c.NextSibling {
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
	return lines
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

// svgKey is the digest of the svg as written, its ids and what names them numbered in its own order: a diagram's ids
// hold its number
// in the page, which a diagram added before it changes.
func svgKey(n *html.Node) string {
	ids := []string{}
	note := func(m *html.Node) {
		if id := attr(m, "id"); id != "" && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	note(n)
	for m := range n.Descendants() {
		note(m)
	}
	pairs := make([][2]string, len(ids))
	for i, id := range ids {
		pairs[i] = [2]string{id, "\x00" + strconv.Itoa(i) + "\x00"}
	}
	// The longest first, as one id may hold another.
	slices.SortStableFunc(pairs, func(a, b [2]string) int { return cmp.Compare(len(b[0]), len(a[0])) })
	flat := make([]string, 0, 2*len(pairs))
	for _, p := range pairs {
		flat = append(flat, p[0], p[1])
	}
	var out bytes.Buffer
	_ = html.Render(&out, n) //nolint:errcheck // a buffer takes every write, and the parser makes no node it cannot render
	return digest(strings.NewReplacer(flat...).Replace(out.String()))
}
