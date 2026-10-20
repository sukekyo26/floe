package tmpl

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/sukekyo26/floe/internal/usererr"
)

// ownCSS returns name, a file of t's CSS, empty when t has none, refused as refuseCSS refuses one of
// @scope: floe scopes the template's CSS to the page, @scope (:root).
func (t *Template) ownCSS(name string) ([]byte, error) {
	css, _, err := t.readOptional(name)
	if err != nil {
		return nil, err
	}
	if err = refuseCSS(t.file(name), css, true); err != nil {
		return nil, err
	}
	return css, nil
}

// refuseCSS refuses css, the file shown as file, that floe puts in a layer, where a } closes no block,
// which would close the layer around it and leave the rules after it out of it, over the parts' styles,
// or where it imports a stylesheet, which a browser does not read in a layer. CSS floe puts in @scope,
// scoped, is refused too where a selector names the root as :root or html, where :scope is the root and
// those name nothing.
func refuseCSS(file string, css []byte, scoped bool) error {
	rules, stray := cssRules(css)
	if stray >= 0 {
		return usererr.New("tmpl_css_brace", file, lineAt(css, stray))
	}
	for _, r := range rules {
		switch {
		case !r.block && strings.HasPrefix(strings.ToLower(r.prelude), "@import"):
			return usererr.New("tmpl_css_import", file, lineAt(css, r.at))
		case scoped && r.block && !strings.HasPrefix(r.prelude, "@"):
			if m := rootSelector.FindStringSubmatchIndex(r.prelude); m != nil {
				return usererr.New("tmpl_css_root", file, lineAt(css, r.at+m[2]), r.prelude[m[2]:m[3]])
			}
		}
	}
	return nil
}

// lineAt is the line of src, from 1, that the offset at is on.
func lineAt(src []byte, at int) int {
	return bytes.Count(src[:at], []byte("\n")) + 1
}

// rootSelector finds :root or html in a selector.
var rootSelector = regexp.MustCompile(`(?i)(?:^|[\s,>+~(])(:root|html)(?:[\s,.:\[#>+~)]|$)`)

// cssRule is what comes before a block of CSS, a selector or an at-rule's prelude, or an at-rule
// written as a statement (@import …;), as written, and the offset where it starts. A block's depth is
// how many blocks hold it, and open and end the offsets of its { and of its }, the end of the file for
// one left open.
type cssRule struct {
	prelude   string
	at        int
	block     bool
	depth     int
	open, end int
}

// cssRules returns the rules of css in order, those in blocks too, past its comments and strings, and
// the offset of a } that closes no block, -1 for none.
// LIMIT: CSS read as far as its blocks, statements, comments and strings, enough to find a rule but not
// to check it. Use a CSS parser if a template comes to need more.
func cssRules(css []byte) (rules []cssRule, stray int) {
	blank := withoutComments(css)
	var open []int // the rules of the blocks open, innermost last
	start := 0
	for i := 0; i < len(blank); i++ {
		switch blank[i] {
		case '"', '\'':
			i = stringEnd(blank, i)
		case '{':
			r := ruleOf(blank, start, i)
			r.block, r.depth, r.open, r.end = true, len(open), i, len(blank)
			open = append(open, len(rules))
			rules = append(rules, r)
			start = i + 1
		case ';':
			if r := ruleOf(blank, start, i); strings.HasPrefix(r.prelude, "@") {
				r.depth = len(open)
				rules = append(rules, r)
			}
			start = i + 1
		case '}':
			if len(open) == 0 {
				return rules, i
			}
			rules[open[len(open)-1]].end = i
			open = open[:len(open)-1]
			start = i + 1
		}
	}
	// A statement the file ends without its ;, which the browser reads all the same.
	if r := ruleOf(blank, start, len(blank)); strings.HasPrefix(r.prelude, "@") {
		r.depth = len(open)
		rules = append(rules, r)
	}
	return rules, -1
}

// ruleOf is the statement written in css from start to end, its spaces trimmed.
func ruleOf(css []byte, start, end int) cssRule {
	text := string(css[start:end])
	trimmed := strings.TrimLeft(text, " \t\r\n\f")
	return cssRule{
		prelude: strings.TrimSpace(trimmed), at: start + len(text) - len(trimmed), block: false, depth: 0, open: -1, end: -1,
	}
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
// offset in it is one in css. A /* in a string starts no comment; a comment never closed runs to the end.
func withoutComments(css []byte) []byte {
	blank := bytes.Clone(css)
	for i := 0; i < len(blank); i++ {
		switch {
		case blank[i] == '"' || blank[i] == '\'':
			i = stringEnd(blank, i)
		case bytes.HasPrefix(blank[i:], []byte("/*")):
			end := len(blank)
			if n := bytes.Index(blank[i+2:], []byte("*/")); n >= 0 {
				end = i + 2 + n + 2
			}
			for ; i < end; i++ {
				if blank[i] != '\n' {
					blank[i] = ' '
				}
			}
			i--
		}
	}
	return blank
}
