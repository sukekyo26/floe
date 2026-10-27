package draw

import (
	"bytes"
	"fmt"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"

	"github.com/sukekyo26/floe/internal/page"
)

// cdns are where a page drawn from a data file may load libraries from, with their styles and fonts.
const cdns = "https://cdnjs.cloudflare.com https://cdn.jsdelivr.net https://unpkg.com"

// CutOff is the CSP of a page drawn from a data file: what it shows is in it (images, fonts and media as data
// URIs), so it loads nothing but Google Fonts and the libraries of cdns, and its fetch, XHR and EventSource
// reach only the source given as connect. A template's or the data's script cannot send what the page holds
// out with fetch, XHR, an image or a form, and a <base> points no link elsewhere.
// LIMIT: moving the page, opening a window, WebRTC and DNS prefetch escape CSP; rethink for untrusted templates.
func CutOff(connect string) string {
	return "default-src 'none'; img-src data: blob:; media-src data: blob:; " +
		"style-src 'unsafe-inline' https://fonts.googleapis.com " + cdns + "; " +
		"font-src data: blob: https://fonts.gstatic.com " + cdns + "; " +
		"script-src 'unsafe-inline' " + cdns + "; form-action 'none'; base-uri 'none'; connect-src " + connect
}

// CutOffPage puts CutOff into a page opened without the headers (a file floe render writes): first in its
// head, read as a browser reads it, since a <meta> binds only what comes after it. Nothing connects. A page
// floe draws is UTF-8: the charset goes first, so that the CSP does not push the page's own out of the bytes
// a browser reads for it. The parser always makes a head.
func CutOffPage(src []byte) ([]byte, error) {
	tree, _ := html.Parse(bytes.NewReader(src)) //nolint:errcheck // a bytes.Reader fails no read, Parse's one error
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
	var out bytes.Buffer
	if err := html.Render(&out, tree); err != nil {
		return nil, fmt.Errorf("draw: write the page: %w", err)
	}
	return out.Bytes(), nil
}
