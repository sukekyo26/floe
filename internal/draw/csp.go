package draw

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"floe/internal/page"
)

// cdns are where a page drawn from a data file may load libraries from, with their styles and fonts.
const cdns = "https://cdnjs.cloudflare.com https://cdn.jsdelivr.net https://unpkg.com"

// CutOff is the CSP of a page drawn from a data file: what it shows is in it (images, fonts and media as data
// URIs), so it loads nothing but Google Fonts and the libraries of cdns, and its fetch and XHR reach only the
// source given as connect. A template's or the data's script cannot send what the page holds
// out with fetch, XHR, an image or a form, and a <base> points no link elsewhere.
// LIMIT: moving the page, opening a window, WebRTC and DNS prefetch escape CSP; rethink for untrusted templates.
func CutOff(connect string) string {
	return "default-src 'none'; img-src data: blob:; media-src data: blob:; " +
		"style-src 'unsafe-inline' https://fonts.googleapis.com " + cdns + "; " +
		"font-src data: blob: https://fonts.gstatic.com " + cdns + "; " +
		"script-src 'unsafe-inline' " + cdns + "; form-action 'none'; base-uri 'none'; connect-src " + connect
}

// CutOffPage puts CutOff into src, a stored page, which has no data file to name, for one who downloads it, as
// withCutOff does.
// LIMIT: read again, a page drawn may go past the parser's depth (503 <div> and <form><i><nobr><h2></form><div><li>
// <button><h1> in them), told here and not when it was published; check it when drawn if such pages come to be written.
func CutOffPage(src []byte) ([]byte, error) {
	tree, err := parsePage("", src)
	if err != nil {
		return nil, err
	}
	withCutOff(tree)
	return writePage("", tree)
}

// withCutOff puts CutOff into tree, nothing connecting: first in its head, read as a browser reads it, since a
// <meta> binds only what comes after it. A page floe draws is UTF-8, and a page holds one charset: the page's own
// go, wherever, and floe's goes first, so that the CSP does not push it out of the bytes a browser reads for it. The
// parser always makes a head.
// LIMIT: a page the server answers keeps page.html's charsets, its header's utf-8 overriding; take them in tmpl.Draw
func withCutOff(tree *html.Node) {
	// Taken out after the walk, which stops at a node taken out.
	var charsets []*html.Node
	for n := range tree.Descendants() {
		if _, ok := page.Attr(n, "charset"); ok && n.Type == html.ElementNode && n.Namespace == "" && n.DataAtom == atom.Meta {
			charsets = append(charsets, n)
		}
	}
	for _, n := range charsets {
		// The spaces after one in the head show nothing, and would leave an empty line; an item's marks taken out
		// leave them in more than one node.
		for t := n.NextSibling; n.Parent.DataAtom == atom.Head && t != nil && t.Type == html.TextNode &&
			strings.TrimLeft(t.Data, " \t\n\f\r") == ""; t = n.NextSibling {
			n.Parent.RemoveChild(t)
		}
		n.Parent.RemoveChild(n)
	}
	head := page.First(tree, atom.Head)
	first := head.FirstChild
	for _, attrs := range [][]html.Attribute{
		{{Namespace: "", Key: "charset", Val: "utf-8"}},
		{
			{Namespace: "", Key: "http-equiv", Val: "Content-Security-Policy"},
			{Namespace: "", Key: "content", Val: CutOff("'none'")},
		},
	} {
		meta := node(html.ElementNode, atom.Meta, "meta")
		meta.Attr = attrs
		head.InsertBefore(meta, first)
	}
}
