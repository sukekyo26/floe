package tmpl

import (
	"bytes"
	"io"
	"slices"
	"strings"
)

// cdns are where a page drawn from a data file may load libraries from, with their styles and fonts.
const cdns = "https://cdnjs.cloudflare.com https://cdn.jsdelivr.net https://unpkg.com"

// CutOff is the CSP of a page drawn from a data file: what it shows is in it (images, fonts and media
// as data URIs), so it loads nothing but Google Fonts and the libraries of cdns, and its fetch, XHR and
// EventSource reach only the source given as connect. A template's or the data's script cannot send
// what the page holds out with fetch, XHR, an image or a form, and a <base> points no link elsewhere.
// LIMIT: moving the page, opening a window, WebRTC and DNS prefetch escape CSP; rethink for untrusted templates.
func CutOff(connect string) string {
	return "default-src 'none'; img-src data: blob:; media-src data: blob:; " +
		"style-src 'unsafe-inline' https://fonts.googleapis.com " + cdns + "; " +
		"font-src data: blob: https://fonts.gstatic.com " + cdns + "; " +
		"script-src 'unsafe-inline' " + cdns + "; form-action 'none'; base-uri 'none'; connect-src " + connect
}

// WithCutOff puts CutOff into a page opened without the headers (a download, or a file floe render
// writes): right after the doctype (past a BOM and comments before it), since a <meta> binds only what
// comes after it; the parser moves it into <head> all the same. Nothing connects: the reload script is
// not in such a page.
// A page floe draws is UTF-8: the charset goes first, so that the CSP does not push the page's own out
// of the bytes a browser reads for it.
func WithCutOff(page []byte) io.Reader {
	at, meta := cutOffAt(page)
	return io.MultiReader(bytes.NewReader(page[:at]), strings.NewReader(meta), bytes.NewReader(page[at:]))
}

// CutOffPage is the page WithCutOff reads, whole.
func CutOffPage(page []byte) []byte {
	at, meta := cutOffAt(page)
	return slices.Concat(page[:at], []byte(meta), page[at:])
}

// cutOffAt returns where in page the <meta> of CutOff goes, and the <meta>.
func cutOffAt(page []byte) (int, string) {
	at := len(page) - len(bytes.TrimPrefix(page, []byte("\uFEFF")))
	text := string(page)
	for {
		at = len(page) - len(bytes.TrimLeft(page[at:], " \t\n\f\r"))
		n := comment(text[at:])
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
	return at, meta
}

// comment returns the length of the comment s starts with, ended where an HTML parser ends it, or 0 for
// none or one never closed: <!-- … --> or --!>, the empty <!--> and <!--->, and <? … >.
func comment(s string) int {
	switch {
	case strings.HasPrefix(s, "<?"):
		return strings.IndexByte(s, '>') + 1
	case !strings.HasPrefix(s, "<!--"):
		return 0
	}
	body := s[len("<!--"):]
	switch {
	case strings.HasPrefix(body, ">"):
		return len("<!-->")
	case strings.HasPrefix(body, "->"):
		return len("<!--->")
	}
	// The first -- that a > or a !> follows ends it.
	for at := 0; ; {
		i := strings.Index(body[at:], "--")
		if i < 0 {
			return 0
		}
		switch rest := body[at+i+len("--"):]; {
		case strings.HasPrefix(rest, ">"):
			return len("<!--") + at + i + len("-->")
		case strings.HasPrefix(rest, "!>"):
			return len("<!--") + at + i + len("--!>")
		}
		at += i + 1
	}
}
