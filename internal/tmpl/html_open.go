package tmpl

import (
	"crypto/rand"
	"errors"
	"io"
	"slices"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/sukekyo26/floe/internal/usererr"
)

// openIn returns the mistake of item, HTML of the data, that closes an element it does not open, one of
// the page's around it, or that leaves open what would take in the page after it, as a browser reads it:
// an <svg> whose foreign content keeps the page's elements, a comment, the text of a <textarea>, a <table>
// that moves them, a <b> they would be in, or a tag or a quote it ends in. nil for none.
func openIn(item string) *usererr.Error {
	if closesOuter(item) {
		return usererr.New("tmpl_html_closes")
	}
	return leftOpen(item)
}

// voids are the elements of HTML that hold nothing and have no end tag.
var voids = []string{
	"area", "base", "br", "col", "embed", "hr", "img", "input", "keygen", "link", "meta", "param", "source",
	"track", "wbr",
}

// closesOuter reports whether item holds an end tag of no element it opens: the page's parser would close
// one of the page's with it, whichever the page has open around the item, or drop it.
func closesOuter(item string) bool {
	var open []string
	z := html.NewTokenizer(strings.NewReader(item))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return !errors.Is(z.Err(), io.EOF)
		case html.StartTagToken:
			if name, _ := z.TagName(); !slices.Contains(voids, string(name)) {
				open = append(open, string(name))
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			at := slices.Index(open, string(name))
			for i := len(open) - 1; i > at; i-- {
				if open[i] == string(name) {
					at = i // the innermost
					break
				}
			}
			if at < 0 {
				return true
			}
			open = open[:at]
		default:
		}
	}
}

// leftOpen returns the mistake of item that leaves open what would take in the page after it, nil for none.
// It is read as a browser's parser reads it in a <div> of the body, with a mark after it, a <section> that
// holds a letter: a <section> closes a <p> left open, as the page's markup after the item may, and foreign
// content keeps it; the letter is taken into a <b> or an <a> left open in that <p>, which the parser opens
// again. The item leaves nothing open where the mark is the last child of the <div>, of HTML, and holds the
// letter alone.
func leftOpen(item string) *usererr.Error {
	mark := rand.Text() // no item names it, so that none stands in for it
	body := &html.Node{
		Parent: nil, FirstChild: nil, LastChild: nil, PrevSibling: nil, NextSibling: nil,
		Type: html.ElementNode, DataAtom: atom.Body, Data: "body", Namespace: "", Attr: nil,
	}
	src := strings.NewReader("<div>" + item + `<section id="` + mark + `">z</section>`)
	nodes, _ := html.ParseFragment(src, body) //nolint:errcheck // a strings.Reader fails no read, its one error
	if len(nodes) != 1 {
		return usererr.New("tmpl_html_closes")
	}
	div := nodes[0]
	found := markIn(div, mark)
	switch {
	case found == nil:
		return swallowed(div, mark)
	case found.Data != "section" || len(found.Attr) != 1:
		return usererr.New("tmpl_html_tag") // the item's tag took in the mark: <b<section, <section id=
	case found.Parent != div:
		return usererr.New("tmpl_html_open", "<"+holder(found, div).Data+">")
	case found != div.LastChild:
		return usererr.New("tmpl_html_open", "<"+div.LastChild.Data+">") // a <table> it was moved before
	case found.FirstChild != found.LastChild || found.FirstChild.Type != html.TextNode:
		return usererr.New("tmpl_html_open", "<"+found.FirstChild.Data+">") // opened again around the letter
	}
	return nil
}

// holder returns what holds the mark under div: the outermost <svg>, <math>, <table> or <select> around it,
// whose end the rest of the item would close, else the element it is in.
func holder(mark, div *html.Node) *html.Node {
	held := mark.Parent
	for n := mark.Parent; n != div; n = n.Parent {
		if n.Namespace == "" && (n.DataAtom == atom.Table || n.DataAtom == atom.Select) || n.Namespace != "" &&
			n.Parent.Namespace == "" {
			held = n
		}
	}
	return held
}

// markIn returns the element of n, or under it, whose id is mark, nil for none.
func markIn(n *html.Node, mark string) *html.Node {
	for c := range n.Descendants() {
		if c.Type == html.ElementNode && slices.ContainsFunc(c.Attr, func(a html.Attribute) bool {
			return a.Key == "id" && a.Val == mark
		}) {
			return c
		}
	}
	return nil
}

// swallowed returns the mistake of an item after which the mark, under div, is no element: the text of a
// comment, of an element whose text runs to its end tag (<textarea>), or of a tag or a quote.
func swallowed(div *html.Node, mark string) *usererr.Error {
	for c := range div.Descendants() {
		switch {
		case c.Type == html.CommentNode && strings.Contains(c.Data, mark):
			return usererr.New("tmpl_html_open", "<!--")
		case c.Type == html.TextNode && strings.Contains(c.Data, mark) && c.Parent != div:
			return usererr.New("tmpl_html_open", "<"+c.Parent.Data+">")
		}
	}
	return usererr.New("tmpl_html_tag")
}
