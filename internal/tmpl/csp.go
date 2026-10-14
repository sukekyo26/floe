package tmpl

import (
	"bytes"
	"io"
	"strings"
)

// cdns are where a page drawn from a data file may load libraries from, with their styles and fonts.
const cdns = "https://cdnjs.cloudflare.com https://cdn.jsdelivr.net https://unpkg.com"

// CutOff is the CSP of a page drawn from a data file: what it shows is in it (images, fonts and media
// as data URIs), so it loads nothing but Google Fonts and the libraries of cdns, and its fetch, XHR and
// EventSource reach only the source given as connect. A template's or the data's script cannot send
// what the page holds out with fetch, XHR, an image or a form.
// LIMIT: moving the page, opening a window, WebRTC and DNS prefetch escape CSP; rethink for untrusted templates.
func CutOff(connect string) string {
	return "default-src 'none'; img-src data: blob:; media-src data: blob:; " +
		"style-src 'unsafe-inline' https://fonts.googleapis.com " + cdns + "; " +
		"font-src data: blob: https://fonts.gstatic.com " + cdns + "; " +
		"script-src 'unsafe-inline' " + cdns + "; form-action 'none'; connect-src " + connect
}

// WithCutOff puts CutOff into a page opened without the headers (a download, or a file floe render
// writes): right after the doctype (past a BOM and comments before it), since a <meta> binds only what
// comes after it; the parser moves it into <head> all the same. Nothing connects: the reload script is
// not in such a page.
// A page floe draws is UTF-8: the charset goes first, so that the CSP does not push the page's own out
// of the bytes a browser reads for it.
func WithCutOff(page []byte) io.Reader {
	at := len(page) - len(bytes.TrimPrefix(page, []byte("\uFEFF")))
	for {
		at = len(page) - len(bytes.TrimLeft(page[at:], " \t\n\f\r"))
		n := comment(page[at:])
		if n == 0 {
			break
		}
		at += n
	}
	doctype := []byte("<!doctype")
	if rest := page[at:]; len(rest) >= len(doctype) && bytes.EqualFold(rest[:len(doctype)], doctype) {
		if end := bytes.IndexByte(rest, '>'); end >= 0 {
			at += end + 1
		}
	}
	meta := "\n<meta charset=\"utf-8\"><meta http-equiv=\"Content-Security-Policy\" content=\"" + CutOff("'none'") + "\">"
	return io.MultiReader(bytes.NewReader(page[:at]), strings.NewReader(meta), bytes.NewReader(page[at:]))
}

// comment returns the length of the comment b starts with, ended where an HTML parser ends it, or 0 for
// none or one never closed: <!-- … --> or --!>, the empty <!--> and <!--->, and <? … >.
func comment(b []byte) int {
	switch {
	case bytes.HasPrefix(b, []byte("<?")):
		return bytes.IndexByte(b, '>') + 1
	case !bytes.HasPrefix(b, []byte("<!--")):
		return 0
	}
	body := b[len("<!--"):]
	switch {
	case bytes.HasPrefix(body, []byte(">")):
		return len("<!-->")
	case bytes.HasPrefix(body, []byte("->")):
		return len("<!--->")
	}
	end := 0
	for _, closing := range []string{"-->", "--!>"} {
		if i := bytes.Index(body, []byte(closing)); i >= 0 && (end == 0 || len("<!--")+i+len(closing) < end) {
			end = len("<!--") + i + len(closing)
		}
	}
	return end
}
