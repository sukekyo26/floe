package tmpl

import (
	"bytes"
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
// is not a letter, a mark, a number, - or _, each space made -.
// LIMIT: Go's Unicode classes and ToLower, not github-slugger's pattern and toLowerCase; they part on rare runes.
func headingID(text string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(text) {
		switch {
		case unicode.IsSpace(r):
			b.WriteByte('-')
		case unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsNumber(r) || r == '-' || r == '_':
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

var headings = []atom.Atom{atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6}

// withHeadingIDs gives each heading of page without an id the id headingID makes of its text, in the order
// of the page, with -1, -2 and on till one is free of the ids and names of the page, wherever they are, and
// of those given before it. One whose text makes none gets none. The rest of page is left byte for byte.
func withHeadingIDs(page []byte) []byte {
	taken := idsIn(page)
	var out, heading bytes.Buffer
	var text strings.Builder
	in := false // reading a heading without an id
	end := func() {
		start := heading.Bytes()
		if id := headingID(text.String()); id != "" {
			id = free(taken, id)
			taken[id] = true
			// After "<h2", whatever the case of its letters.
			start = slices.Concat(start[:3], []byte(` id="`+xhtml.EscapeString(id)+`"`), start[3:])
		}
		out.Write(start)
		in = false
	}
	z := xhtml.NewTokenizer(bytes.NewReader(page))
	for tt := z.Next(); tt != xhtml.ErrorToken; tt = z.Next() {
		// Token lowers the tag's name in the tokenizer's buffer, which Raw returns.
		raw := slices.Clone(z.Raw())
		tok := z.Token()
		isHeading := slices.Contains(headings, tok.DataAtom)
		// As the HTML parser reads them: a heading ends at the end tag of any heading, or where another starts.
		if in && isHeading && tt != xhtml.TextToken {
			end()
		}
		_, hasID := idOf(tok)
		switch {
		// <h2/> starts a heading too: the parser leaves the / of an element that is not void.
		case isHeading && (tt == xhtml.StartTagToken || tt == xhtml.SelfClosingTagToken) && !hasID:
			in = true
			text.Reset()
			heading.Reset()
			heading.Write(raw)
		case in:
			if tt == xhtml.TextToken {
				text.WriteString(tok.Data)
			}
			heading.Write(raw)
		default:
			out.Write(raw)
		}
	}
	if in {
		out.Write(heading.Bytes())
	}
	return out.Bytes()
}

// idsIn returns the ids of the tags of page and the names of its <a>s, which a link goes to as it does to
// an id (anchorsIn).
func idsIn(page []byte) map[string]bool {
	ids := map[string]bool{}
	z := xhtml.NewTokenizer(bytes.NewReader(page))
	for tt := z.Next(); tt != xhtml.ErrorToken; tt = z.Next() {
		tok := z.Token()
		if id, ok := idOf(tok); ok {
			ids[id] = true
		}
		if tok.DataAtom == atom.A && tt != xhtml.EndTagToken {
			for _, a := range tok.Attr {
				if a.Key == "name" {
					ids[a.Val] = true
				}
			}
		}
	}
	return ids
}

// free returns id, or the first of id-1, id-2 and on, that taken has not.
func free(taken map[string]bool, id string) string {
	for n, base := 1, id; taken[id]; n++ {
		id = base + "-" + strconv.Itoa(n)
	}
	return id
}

// idOf returns the id of t, a start tag.
func idOf(t xhtml.Token) (string, bool) {
	if t.Type != xhtml.StartTagToken && t.Type != xhtml.SelfClosingTagToken {
		return "", false
	}
	for _, a := range t.Attr {
		if a.Key == "id" {
			return a.Val, true
		}
	}
	return "", false
}

// checkAnchors refuses each link of page, the page drawn with src, its template's page.html, to a part of
// it that no element is: an <a> or <area> whose href is # and a fragment that is no id nor name of an <a>,
// as written or percent-decoded, as a browser looks for it, without a text fragment (:~: and after it). #
// alone and #top go to the top of the page.
// LIMIT: ids a script makes after drawing (a script part's, the data HTML's) are not seen; their links are refused.
func (d *Data) checkAnchors(page, src []byte) error {
	targets, links := anchorsIn(page)
	var problems usererr.List
	told := map[string]bool{}
	for _, written := range links {
		frag, _, _ := strings.Cut(written, ":~:")
		// The parser read the href's character references already.
		decoded := percentDecoded(frag)
		if frag == "" || targets[frag] || targets[decoded] || strings.EqualFold(decoded, "top") || told[decoded] {
			continue
		}
		told[decoded] = true
		problems = append(problems, d.linkMistakes(written, decoded, src)...)
	}
	return problems.Sorted().Err()
}

// anchorsIn returns what a link in page may go to, its ids and the names of its <a>s, and the fragments
// its links to a part of it go to, # left out, as written.
func anchorsIn(page []byte) (targets map[string]bool, links []string) {
	targets = map[string]bool{}
	for n := range parseDrawn(page).Descendants() {
		if n.Type != xhtml.ElementNode {
			continue
		}
		inHTML := n.Namespace == ""
		for _, a := range n.Attr {
			switch {
			case a.Key == "id" && a.Namespace == "":
				targets[a.Val] = true
			case inHTML && n.DataAtom == atom.A && a.Key == "name":
				targets[a.Val] = true
			case inHTML && (n.DataAtom == atom.A || n.DataAtom == atom.Area) && a.Key == "href" &&
				strings.HasPrefix(a.Val, "#"):
				links = append(links, a.Val[1:])
			}
		}
	}
	return targets, links
}

// linkMistakes returns the mistakes of a link to #written, which goes to decoded and reaches no part of the
// page: one at each item of d that writes it, else one at the template's page.html if src, its text,
// writes it, else one at d's file, which a link the search misses is the data's all the same.
func (d *Data) linkMistakes(written, decoded string, src []byte) []*usererr.Error {
	var mistakes []*usererr.Error
	for _, it := range d.linkItems {
		if writes(it.text, written) {
			mistakes = append(mistakes, itemError(d.File, it.pos, it.path, usererr.New("tmpl_anchor_none", "#"+decoded)))
		}
	}
	switch {
	case mistakes != nil:
		return mistakes
	case writes(string(src), written):
		return []*usererr.Error{usererr.NewAt(d.tmpl.file("page.html"), 0, 0, "tmpl_anchor_none", "#"+decoded)}
	}
	return []*usererr.Error{usererr.NewAt(d.File, 0, 0, "tmpl_anchor_none", "#"+decoded)}
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
