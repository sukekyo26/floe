package tmpl

import (
	"bytes"
	"crypto/rand"
	"html"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/goccy/go-yaml/ast"
	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/sukekyo26/floe/internal/usererr"
)

// headingID is the id of a heading whose text is text, near the one GitHub gives: lower case, without what
// is not a letter, a mark, a number, - or _, the spaces between the words left made one -, as a browser
// shows them as one (an ideographic space and a no-break space too).
// LIMIT: Go's Unicode classes and ToLower, not github-slugger's pattern and toLowerCase; they part on rare runes.
func headingID(text string) string {
	kept := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r) || r == '-' || r == '_' || unicode.IsSpace(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, text)
	return strings.Join(strings.Fields(kept), "-")
}

var headings = []atom.Atom{atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6}

// pageAnchors is what the links of a page drawn may go to, its ids and the names of its <a>s, and the
// fragments its links to a part of it go to, # left out, as written.
type pageAnchors struct {
	targets map[string]bool
	links   []string
}

// withHeadingIDs gives each heading of page without an id the id headingID makes of its text, but what a
// part draws in it (data-floe-part), in the order of the page as the parser reads it, with -1, -2 and on
// till one is free of the ids and the names of the page, wherever they are, and of those given before it.
// One whose text makes none gets none. The rest of page is left byte for byte. It returns the anchors of
// the page given its ids. Which start tag each heading the parser makes is of is told by a mark on each,
// which the page returned has not: the parser ends a heading where the tags around it say, and moves one
// out of a table.
func withHeadingIDs(page []byte) ([]byte, pageAnchors) {
	mark := "data-floe-heading-" + strings.ToLower(rand.Text())
	marked := atHeadingTags(page, func(k int) string { return " " + mark + `="` + strconv.Itoa(k) + `"` })
	anchors := pageAnchors{targets: map[string]bool{}, links: nil}
	var found []*xhtml.Node
	for n := range parseDrawn(marked).Descendants() {
		if n.Type != xhtml.ElementNode {
			continue
		}
		if n.Namespace == "" && slices.Contains(headings, n.DataAtom) {
			found = append(found, n)
		}
		anchors.add(n)
	}
	ids := map[string]string{}
	for _, n := range found {
		k, hasMark := attrOf(n, mark)
		// An empty id is none: a link cannot go to it. The parser keeps the id put before it.
		if id, _ := attrOf(n, "id"); !hasMark || id != "" {
			continue
		}
		// A mark in its text was put where the tokenizer read a tag that the parser reads as text.
		if text := textOf(n); !strings.Contains(text, mark) {
			if id := headingID(text); id != "" {
				id = free(anchors.targets, id)
				anchors.targets[id] = true
				ids[k] = id
			}
		}
	}
	if len(ids) == 0 {
		return page, anchors
	}
	return atHeadingTags(page, func(k int) string {
		if id, ok := ids[strconv.Itoa(k)]; ok {
			return ` id="` + xhtml.EscapeString(id) + `"`
		}
		return ""
	}), anchors
}

// add adds what n, an element, gives a's: its id, its name as an <a>, the fragment of its href as an <a>
// or an <area> that links to a part of the page.
func (a *pageAnchors) add(n *xhtml.Node) {
	inHTML := n.Namespace == ""
	for _, at := range n.Attr {
		switch {
		case at.Val == "" && at.Key != "href":
			// An empty id or name is none.
		case at.Key == "id" && at.Namespace == "":
			a.targets[at.Val] = true
		case inHTML && n.DataAtom == atom.A && at.Key == "name":
			a.targets[at.Val] = true
		case inHTML && (n.DataAtom == atom.A || n.DataAtom == atom.Area) && at.Key == "href" &&
			strings.HasPrefix(at.Val, "#"):
			a.links = append(a.links, at.Val[1:])
		}
	}
}

// atHeadingTags returns page with attr(k) put in the k-th start tag of a heading, <h2/> too, which the
// parser reads as a start, after its name. What a tag left open at the end holds is kept.
// LIMIT: in <svg> the tokenizer reads as text what the parser does not (<title/>, <style/>, <textarea/>, CDATA): no id.
func atHeadingTags(page []byte, attr func(k int) string) []byte {
	var out bytes.Buffer
	k := 0
	z := xhtml.NewTokenizer(bytes.NewReader(page))
	for {
		tt := z.Next()
		// Read before the next token, which reuses it; Token and TagName would lower the tag's name in it.
		raw := z.Raw()
		switch {
		case tt == xhtml.ErrorToken:
			out.Write(raw)
			return out.Bytes()
		case (tt == xhtml.StartTagToken || tt == xhtml.SelfClosingTagToken) && headingTag(raw):
			out.Write(raw[:3])
			out.WriteString(attr(k))
			out.Write(raw[3:])
			k++
		default:
			out.Write(raw)
		}
	}
}

// headingTag reports whether raw, a start tag as written, is a heading's: its name, which ends at a space,
// a / or a >, is h1 to h6 in either case.
func headingTag(raw []byte) bool {
	return len(raw) > 3 && (raw[1] == 'h' || raw[1] == 'H') && raw[2] >= '1' && raw[2] <= '6' &&
		bytes.IndexByte([]byte(" \t\n\f\r/>"), raw[3]) >= 0
}

// textOf returns the text in n, but that of a part (data-floe-part) and that of an element whose content
// the parser reads as text (a <script>, a <style>).
func textOf(n *xhtml.Node) string {
	var b strings.Builder
	var walk func(n *xhtml.Node)
	walk = func(n *xhtml.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			_, isPart := attrOf(c, "data-floe-part")
			switch {
			case c.Type == xhtml.TextNode:
				b.WriteString(c.Data)
			case c.Type == xhtml.ElementNode && !isPart && !readAsText(c):
				walk(c)
			}
		}
	}
	walk(n)
	return b.String()
}

// attrOf returns the value of n's attribute key, of no namespace.
func attrOf(n *xhtml.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Namespace == "" && a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

// free returns id, or the first of id-1, id-2 and on, that taken has not.
func free(taken map[string]bool, id string) string {
	for n, base := 1, id; taken[id]; n++ {
		id = base + "-" + strconv.Itoa(n)
	}
	return id
}

// checkAnchors refuses each link of a page, whose anchors are a, drawn with src, its template's page.html,
// to a part of it that no element is: a fragment that is no id nor name of an <a>, as written or
// percent-decoded, as a browser looks for it, without a text fragment (:~: and after it). # alone and #top
// go to the top of the page.
// LIMIT: ids a script makes after drawing (a script part's, the data HTML's) are not seen; their links are refused.
func (d *Data) checkAnchors(a pageAnchors, src []byte) error {
	var problems usererr.List
	told := map[string]bool{}
	for _, written := range a.links {
		frag, _, _ := strings.Cut(written, ":~:")
		// The parser read the href's character references already.
		decoded := percentDecoded(frag)
		if frag == "" || a.targets[frag] || a.targets[decoded] || strings.EqualFold(decoded, "top") || told[decoded] {
			continue
		}
		told[decoded] = true
		problems = append(problems, d.linkMistakes(written, decoded, src)...)
	}
	return problems.Sorted().Err()
}

// linkMistakes returns the mistakes of a link to #written, which goes to decoded and reaches no part of the
// page: one at each item of d that writes it, else one at the page.html of d's own template if src, its
// text, writes it, else one at d's file, which a link the search misses is the data's all the same.
func (d *Data) linkMistakes(written, decoded string, src []byte) []*usererr.Error {
	key, args := "tmpl_anchor_none", []any{"#" + decoded}
	if decoded == "ZgotmplZ" {
		// html/template's word for a URL it found unsafe (javascript: and the like), drawn in its place.
		key, args = "tmpl_anchor_unsafe", nil
	}
	var mistakes []*usererr.Error
	for _, it := range d.linkItems {
		if writes(it.text, written) {
			mistakes = append(mistakes, itemError(d.File, it.pos, it.path, usererr.New(key, args...)))
		}
	}
	switch {
	case mistakes != nil:
		return mistakes
	case d.tmpl.isOwn() && writes(string(src), written):
		return []*usererr.Error{usererr.NewAt(d.tmpl.file("page.html"), 0, 0, key, args...)}
	}
	return []*usererr.Error{usererr.NewAt(d.File, 0, 0, key, args...)}
}

// writes reports whether text writes a link to the fragment frag: # and frag, as written or percent-decoded,
// in text as written or with its character references read and percent-decoded, ended by text, a space or
// one of ) " ' > ] <.
// LIMIT: a search of the text, not of the link as drawn; a link it misses is told at the file without a line.
func writes(text, frag string) bool {
	for _, t := range []string{text, unescapeAll(text)} {
		for _, f := range []string{frag, percentDecoded(frag)} {
			for rest := t; ; {
				i := strings.Index(rest, "#"+f)
				if i < 0 {
					break
				}
				rest = rest[i+1+len(f):]
				if rest == "" || strings.ContainsAny(rest[:1], " \t\n)\"'>]<") {
					return true
				}
			}
		}
	}
	return false
}

// unescapeAll reads the character references of s and percent-decodes it.
func unescapeAll(s string) string {
	return percentDecoded(html.UnescapeString(s))
}

// percentDecoded percent-decodes s, as it is where a % starts no escape.
func percentDecoded(s string) string {
	if u, err := url.PathUnescape(s); err == nil {
		return u
	}
	return s
}

// linkItemsOf returns the texts of values that hold a #, which may write a link in the page, where root
// writes them, in the order of the file.
func linkItemsOf(values map[string]any, root ast.Node) []placedText {
	var items []placedText
	var walk func(v any, at []string)
	walk = func(v any, at []string) {
		switch v := v.(type) {
		case string:
			if strings.Contains(v, "#") {
				pos, path := itemPos(root, at, false)
				items = append(items, placedText{at: at, text: v, pos: pos, path: path})
			}
		case map[string]any:
			for k, x := range v {
				walk(x, append(at[:len(at):len(at)], k))
			}
		case []any:
			for i, x := range v {
				walk(x, append(at[:len(at):len(at)], strconv.Itoa(i)))
			}
		}
	}
	walk(values, nil)
	slices.SortFunc(items, byPlace)
	return items
}
