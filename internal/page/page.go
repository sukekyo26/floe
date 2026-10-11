// Package page tells a whole document from a body-only fragment, takes a page's title and wraps a
// fragment in the template.
package page

import (
	"bytes"
	"html"
	"html/template"
	"regexp"
	"strings"

	"github.com/sukekyo26/floe/internal/ui"
)

const (
	// SniffSize is how much of a page StartsAsDocument looks at.
	SniffSize = 1024
	// DefaultLang is the language of a wrapped page unless the publisher names another.
	DefaultLang = "en"
)

var (
	// An SVG <title> describes the figure, so it neither titles the page nor marks a document.
	// LIMIT: a nested <svg> ends at the inner </svg>. Switch to an HTML parser if nested figures misjudge.
	svgRe      = regexp.MustCompile(`(?is)<svg[\s>].*?</svg>`)
	svgStartRe = regexp.MustCompile(`(?i)<svg[\s>]`)
	documentRe = regexp.MustCompile(`(?i)<!doctype|<(html|head|body|title)[\s>]`)
	titleRe    = regexp.MustCompile(`(?is)<title(?:[\s/][^>]*)?>(.*?)</title>`)
	h1Re       = regexp.MustCompile(`(?is)<h1(?:[\s/][^>]*)?>(.*?)</h1>`)
	tagRe      = regexp.MustCompile(`<[^>]*>`)
	// JavaScript's \s: Go's \s is ASCII only, but a decoded &nbsp; or a full-width space must collapse too.
	spaceRe = regexp.MustCompile(`[\s\v\p{Z}\x{FEFF}]+`)
	wrapper = template.Must(template.New("page").Parse(ui.Template))
)

func outsideSvg(doc string) string {
	return svgRe.ReplaceAllString(doc, "")
}

// Title returns the <title>, else the first <h1>, as plain text, or "" when the page has neither.
func Title(doc string) string {
	s := outsideSvg(doc)
	m := titleRe.FindStringSubmatch(s)
	if m == nil {
		m = h1Re.FindStringSubmatch(s)
	}
	if m == nil {
		return ""
	}
	// html/template writes " and + as &#34; and &#43;, so every entity is decoded, not only the named five.
	text := html.UnescapeString(tagRe.ReplaceAllString(m[1], ""))
	return strings.TrimSpace(spaceRe.ReplaceAllString(text, " "))
}

// IsDocument reports whether doc has a document's frame (<!doctype> <html> <head> <body> <title>).
func IsDocument(doc string) bool {
	return documentRe.MatchString(outsideSvg(doc))
}

// StartsAsDocument reports whether the head of a page (its first SniffSize bytes) already marks a
// document. It looks before the first <svg> only, so that an SVG <title> does not count. A document
// marked later is still told apart by Wrap, which reads the whole page.
func StartsAsDocument(head []byte) bool {
	if i := svgStartRe.FindIndex(head); i != nil {
		head = head[:i[0]]
	}
	return documentRe.Match(head)
}

// Wrap returns doc as is when it is a document, and otherwise the template around it, in the language
// lang (a BCP 47 tag; "" is DefaultLang).
func Wrap(doc, title, lang string) ([]byte, error) {
	if IsDocument(doc) {
		return []byte(doc), nil
	}
	if lang == "" {
		lang = DefaultLang
	}
	var b bytes.Buffer
	//nolint:gosec // G203: the fragment is the page itself; it is meant to be inserted verbatim
	data := struct {
		Lang    string
		Title   string
		Content template.HTML
	}{Lang: lang, Title: title, Content: template.HTML(doc)}
	if err := wrapper.Execute(&b, data); err != nil {
		return nil, err //nolint:wrapcheck // the template is embedded; an error here is a floe bug
	}
	return b.Bytes(), nil
}
