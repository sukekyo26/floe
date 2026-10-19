package tmpl

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/sukekyo26/floe/internal/usererr"
)

// ownCSS refuses css, the file name of t, where a } closes no block, which would close floe's layer
// around the template's CSS and leave the rules after it out of it, over the parts' styles, or where it
// imports a stylesheet, which a browser does not read in a layer.
func (t *Template) ownCSS(name string, css []byte) error {
	rules, stray := cssRules(css)
	if stray >= 0 {
		return usererr.New("tmpl_css_brace", t.file(name), bytes.Count(css[:stray], []byte("\n"))+1)
	}
	for _, r := range rules {
		if !r.block && strings.HasPrefix(strings.ToLower(r.prelude), "@import") {
			return usererr.New("tmpl_css_import", t.file(name), bytes.Count(css[:r.at], []byte("\n"))+1)
		}
	}
	return nil
}

// cssRule is what comes before a block of CSS, a selector or an at-rule's prelude, or an at-rule
// written as a statement (@import …;), as written, and the offset where it starts.
type cssRule struct {
	prelude string
	at      int
	block   bool
}

// cssRules returns the rules of css in order, those in blocks too, past its comments and strings, and
// the offset of a } that closes no block, -1 for none.
// LIMIT: CSS read as far as its blocks, statements, comments and strings, enough to find a rule but not
// to check it. Use a CSS parser if a template comes to need more.
func cssRules(css []byte) (rules []cssRule, stray int) {
	blank := withoutComments(css)
	depth, start := 0, 0
	for i := 0; i < len(blank); i++ {
		switch blank[i] {
		case '"', '\'':
			i = stringEnd(blank, i)
		case '{':
			rules = append(rules, ruleOf(blank, start, i, true))
			depth++
			start = i + 1
		case ';':
			if r := ruleOf(blank, start, i, false); strings.HasPrefix(r.prelude, "@") {
				rules = append(rules, r)
			}
			start = i + 1
		case '}':
			if depth == 0 {
				return rules, i
			}
			depth--
			start = i + 1
		}
	}
	return rules, -1
}

// ruleOf is the rule written in css from start to end, its spaces trimmed.
func ruleOf(css []byte, start, end int, block bool) cssRule {
	text := string(css[start:end])
	trimmed := strings.TrimLeft(text, " \t\r\n\f")
	return cssRule{prelude: strings.TrimSpace(trimmed), at: start + len(text) - len(trimmed), block: block}
}

// stringEnd returns the offset of the end of the string of CSS that starts at the quote at i: its
// closing quote, the line break that ends it unclosed, else the last byte.
func stringEnd(css []byte, i int) int {
	for j := i + 1; j < len(css); j++ {
		switch css[j] {
		case '\\':
			j++
		case css[i], '\n':
			return j
		}
	}
	return len(css) - 1
}

// withoutComments returns css with each byte of its comments a space, but a line break, so that an
// offset in it is one in css.
func withoutComments(css []byte) []byte {
	return cssComment.ReplaceAllFunc(css, func(c []byte) []byte {
		blank := bytes.Clone(c)
		for i, b := range blank {
			if b != '\n' {
				blank[i] = ' '
			}
		}
		return blank
	})
}

// cssComment finds a comment of CSS; one never closed runs to the end.
var cssComment = regexp.MustCompile(`(?s)/\*.*?(?:\*/|$)`)
