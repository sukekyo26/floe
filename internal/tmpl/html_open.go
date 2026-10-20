package tmpl

import (
	"crypto/rand"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/sukekyo26/floe/internal/usererr"
)

// openIn returns the mistake of item, HTML of the data, that leaves open what would take in the page after
// it, as a browser reads it: an <svg> whose foreign content keeps the page's elements, a comment, the text
// of a <textarea>, a <table> that moves them, a <b> they would be in, or a tag or a quote it ends in; or that
// closes an element it does not open, one of the page's. nil for none.
//
// It is read as a browser's parser reads it in a <div> of the body, with a mark after it, a <section>: it
// closes a <p> left open, as the page's markup after the item may, and foreign content keeps it. The item
// leaves nothing open where the mark is the last child of the <div>, and of HTML.
func openIn(item string) *usererr.Error {
	mark := rand.Text() // no item names it, so that none stands in for it
	body := &html.Node{
		Parent: nil, FirstChild: nil, LastChild: nil, PrevSibling: nil, NextSibling: nil,
		Type: html.ElementNode, DataAtom: atom.Body, Data: "body", Namespace: "", Attr: nil,
	}
	src := strings.NewReader("<div>" + item + `<section id="` + mark + `"></section>`)
	nodes, _ := html.ParseFragment(src, body) //nolint:errcheck // a strings.Reader fails no read, its one error
	if len(nodes) != 1 {
		return usererr.New("tmpl_html_closes")
	}
	div := nodes[0]
	found := markIn(div, mark)
	switch {
	case found == nil:
		return openOf(div.LastChild)
	case found.Data != "section":
		return usererr.New("tmpl_html_tag") // the item's tag took in the mark's name: <b<section
	case found.Parent == div && found == div.LastChild && found.Namespace == "":
		return nil
	case found.Parent == div:
		return openOf(div.LastChild) // a <table> it was moved before
	}
	for found.Parent != div {
		found = found.Parent
	}
	return openOf(found)
}

// markIn returns the element of n, or under it, whose id is mark, nil for none.
func markIn(n *html.Node, mark string) *html.Node {
	for c := range n.Descendants() {
		if c.Type == html.ElementNode && attrOf(c, "id") == mark {
			return c
		}
	}
	return nil
}

// attrOf returns the value of the attribute key of n, "" for none.
func attrOf(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Namespace == "" && a.Key == key {
			return a.Val
		}
	}
	return ""
}

// openOf returns the mistake of n, what an item leaves open: an element, a comment, or nil for a tag or a
// quote, which takes in the rest as its own. A quote left open in a tag tells the element of the tag.
func openOf(n *html.Node) *usererr.Error {
	switch {
	case n != nil && n.Type == html.ElementNode:
		return usererr.New("tmpl_html_open", "<"+n.Data+">")
	case n != nil && n.Type == html.CommentNode:
		return usererr.New("tmpl_html_open", "<!--")
	default:
		return usererr.New("tmpl_html_tag")
	}
}
