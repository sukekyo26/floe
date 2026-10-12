// Package page takes the title of an HTML page.
package page

import (
	"html"
	"regexp"
	"strings"
)

var (
	// An SVG <title> describes the figure, so it does not title the page.
	// LIMIT: a nested <svg> ends at the inner </svg>. Switch to an HTML parser if nested figures misjudge.
	svgRe   = regexp.MustCompile(`(?is)<svg[\s>].*?</svg>`)
	titleRe = regexp.MustCompile(`(?is)<title(?:[\s/][^>]*)?>(.*?)</title>`)
	h1Re    = regexp.MustCompile(`(?is)<h1(?:[\s/][^>]*)?>(.*?)</h1>`)
	tagRe   = regexp.MustCompile(`<[^>]*>`)
	// JavaScript's \s: Go's \s is ASCII only, but a decoded &nbsp; or a full-width space must collapse too.
	spaceRe = regexp.MustCompile(`[\s\v\p{Z}\x{FEFF}]+`)
)

// Title returns the <title>, else the first <h1>, as plain text, or "" when the page has neither.
func Title(doc string) string {
	s := svgRe.ReplaceAllString(doc, "")
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
