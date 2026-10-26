package draw

import (
	"bytes"
	"slices"

	"golang.org/x/net/html"
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

// CutOffPage puts CutOff into a page opened without the headers (a file floe render writes): right after the
// doctype (past a BOM and comments before it), since a <meta> binds only what comes after it; the parser moves
// it into <head> all the same. Nothing connects. A page floe draws is UTF-8: the charset goes first, so that
// the CSP does not push the page's own out of the bytes a browser reads for it.
func CutOffPage(page []byte) []byte {
	at, meta := cutOffAt(page)
	return slices.Concat(page[:at], []byte(meta), page[at:])
}

// cutOffAt returns where in page the <meta> of CutOff goes, and the <meta>: past a BOM, the ASCII whitespace,
// the comments and the doctype an HTML tokenizer reads at its start, and before anything else. A comment or
// a doctype that runs to the end of page is taken as one left open, and the <meta> goes before it, not in
// it: whether one is closed is not read apart.
// LIMIT: a page that ends with its doctype gets the <meta> before it, which a browser draws in quirks mode;
// a page floe draws has a <head> after its doctype.
func cutOffAt(page []byte) (int, string) {
	meta := "\n<meta charset=\"utf-8\"><meta http-equiv=\"Content-Security-Policy\" content=\"" + CutOff("'none'") + "\">"
	at := len(page) - len(bytes.TrimPrefix(page, []byte("\xef\xbb\xbf")))
	z := html.NewTokenizer(bytes.NewReader(page[at:]))
	for {
		tt := z.Next()
		raw := len(z.Raw())
		toEnd := at+raw == len(page)
		switch tt {
		case html.TextToken:
			if rest := bytes.TrimLeft(z.Raw(), " \t\n\f\r"); len(rest) > 0 {
				return at + raw - len(rest), meta
			}
			at += raw
		case html.CommentToken:
			if toEnd {
				return at, meta
			}
			at += raw
		case html.DoctypeToken:
			if toEnd {
				return at, meta
			}
			return at + raw, meta
		default:
			return at, meta
		}
	}
}
