package tmpl

import (
	"bytes"
	"regexp"
	"strings"

	tparse "github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"

	"github.com/sukekyo26/floe/internal/usererr"
)

// ownCSS returns t's page.css, empty when t has none, refused as cssSheet.refuse refuses one of
// @scope: floe scopes the template's CSS to the page, @scope (:root).
func (t *Template) ownCSS() ([]byte, error) {
	s, err := t.pageSheet()
	if err != nil {
		return nil, err
	}
	return s.src, nil
}

// pageSheet returns t's page.css, read once, refused as ownCSS refuses it.
func (t *Template) pageSheet() (*cssSheet, error) {
	return t.reads.sheet.of(func() (*cssSheet, error) {
		src, _, err := t.readOptional("page.css")
		if err != nil {
			return nil, err
		}
		s := readCSS(src)
		if err = s.refuse(t.file("page.css"), true); err != nil {
			return nil, err
		}
		return s, nil
	})
}

// refuse refuses s, the file shown as file, that floe puts in a layer, where a } closes no block,
// which would close the layer around it and leave the rules after it out of it, over the parts' styles,
// or where it imports a stylesheet, which a browser does not read in a layer. CSS floe puts in @scope,
// scoped, is refused too where a selector names the root as :root or html, where :scope is the root and
// those name nothing.
func (s *cssSheet) refuse(file string, scoped bool) error {
	if s.stray >= 0 {
		return usererr.NewAt(file, lineAt(s.src, s.stray), 0, "tmpl_css_brace")
	}
	for _, r := range s.rules {
		switch {
		case !r.block && strings.HasPrefix(strings.ToLower(r.prelude), "@import"):
			return usererr.NewAt(file, lineAt(s.src, r.at), 0, "tmpl_css_import")
		case scoped && r.block && !strings.HasPrefix(r.prelude, "@"):
			if m := rootSelector.FindStringSubmatchIndex(r.prelude); m != nil {
				return usererr.NewAt(file, lineAt(s.src, r.at+m[2]), 0, "tmpl_css_root", r.prelude[m[2]:m[3]])
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

// cssSheet is CSS read as CSS Syntax reads it: its tokens, spaces and comments among them, its rules in
// order, those in blocks too, the declarations written outside any block, which floe's @scope applies to
// :scope, and the offset of a } that closes no block, -1 for none.
type cssSheet struct {
	src    []byte
	tokens []cssToken
	rules  []cssRule
	decls  []cssDeclaration
	stray  int
}

// cssToken is a token of CSS, as written, and the offset where it starts.
type cssToken struct {
	typ  css.TokenType
	text string
	at   int
}

// cssRule is what comes before a block of CSS, a selector or an at-rule's prelude, or an at-rule written
// as a statement (@import …;), as written, its comments spaces, and the offset where it starts, so that an
// offset in it is one in the file from there. A block's depth is how many blocks hold it, parent the index
// of the rule of the innermost, -1 for none; first, open and end are the indices of the tokens of the
// prelude's start, of its { and of its }, the end of the tokens for one left open, open and end -1 for a
// statement. decls are the declarations of the block, but those of the rules it holds.
type cssRule struct {
	prelude          string
	at               int
	block            bool
	depth, parent    int
	first, open, end int
	decls            []cssDeclaration
}

// readCSS reads src, which floe's lexer of CSS Syntax reads as a browser does: a string, a comment, an
// escaped character or the URL of url(…) holds no brace or ; of the CSS's.
func readCSS(src []byte) *cssSheet {
	s := &cssSheet{src: src, tokens: nil, rules: nil, decls: nil, stray: -1}
	l := css.NewLexer(tparse.NewInputString(string(src)))
	for at := 0; at < len(src); {
		typ, text := l.Next()
		if typ == css.ErrorToken {
			break
		}
		s.tokens = append(s.tokens, cssToken{typ: typ, text: string(src[at : at+len(text)]), at: at})
		at += len(text)
	}
	s.readRules()
	return s
}

// opened is a block or a bracket open in CSS: the token that closes it, and its rule, -1 for a bracket
// or a block of a value (--x: { … }).
type opened struct {
	closer css.TokenType
	rule   int
}

// readRules reads the rules of s from its tokens, and the declarations of their blocks. Within brackets, as
// in a function, a brace or a ; is the value's: a browser reads to the ) that closes them, or to the end.
func (s *cssSheet) readRules() {
	r := &ruleReader{s: s, stack: nil, open: 0, start: 0}
	for i, t := range s.tokens {
		if !r.read(i, t) {
			return
		}
	}
	// What the file ends with: a declaration of the block left open, or a statement without its ;, which
	// the browser reads all the same.
	r.segment(len(s.tokens))
}

// ruleReader reads the rules of s: the blocks and brackets open, innermost last, the rule blocks among
// them, and the token the next rule or declaration starts at.
type ruleReader struct {
	s           *cssSheet
	stack       []opened
	open, start int
}

// read reads the token t, at i, and reports whether to read on: not past a } that closes no block.
func (r *ruleReader) read(i int, t cssToken) bool {
	inValue := len(r.stack) > 0 && r.stack[len(r.stack)-1].rule < 0
	switch t.typ {
	case css.FunctionToken, css.LeftParenthesisToken:
		r.stack = append(r.stack, opened{closer: css.RightParenthesisToken, rule: -1})
	case css.LeftBracketToken:
		r.stack = append(r.stack, opened{closer: css.RightBracketToken, rule: -1})
	case css.LeftBraceToken:
		r.brace(i, inValue)
	case css.RightParenthesisToken, css.RightBracketToken, css.RightBraceToken:
		return r.close(i, t)
	case css.SemicolonToken:
		if !inValue {
			r.segment(i)
			r.start = i + 1
		}
	default:
	}
	return true
}

// innermost returns the index of the rule of the innermost block open, -1 for none.
func (r *ruleReader) innermost() int {
	for j := len(r.stack) - 1; j >= 0; j-- {
		if r.stack[j].rule >= 0 {
			return r.stack[j].rule
		}
	}
	return -1
}

// brace reads the { at i: a rule's block, or one of a value (--x: { … }) or in brackets.
func (r *ruleReader) brace(i int, inValue bool) {
	if inValue || r.s.customProperty(r.start, i) {
		r.stack = append(r.stack, opened{closer: css.RightBraceToken, rule: -1})
		return
	}
	rule := r.s.ruleOf(r.start, i)
	rule.block, rule.depth, rule.parent, rule.open, rule.end = true, r.open, r.innermost(), i, len(r.s.tokens)
	r.stack = append(r.stack, opened{closer: css.RightBraceToken, rule: len(r.s.rules)})
	r.s.rules = append(r.s.rules, rule)
	r.open++
	r.start = i + 1
}

// close reads the closing token t at i, and reports whether to read on: a } that closes no block is
// noted as the stray one of s. One of another kind than what is open is the value's.
func (r *ruleReader) close(i int, t cssToken) bool {
	if len(r.stack) == 0 && t.typ == css.RightBraceToken {
		r.s.stray = t.at
		return false
	}
	if len(r.stack) == 0 || r.stack[len(r.stack)-1].closer != t.typ {
		return true
	}
	top := r.stack[len(r.stack)-1]
	if top.rule >= 0 {
		r.segment(i) // the block's last declaration, without its ;
	}
	r.stack = r.stack[:len(r.stack)-1]
	if top.rule >= 0 {
		r.s.rules[top.rule].end = i
		r.open--
		r.start = i + 1
	}
	return true
}

// segment notes what is written from the start up to end: an at-rule written as a statement, or a
// declaration of the innermost block open, or of none.
func (r *ruleReader) segment(end int) {
	rule := r.s.ruleOf(r.start, end)
	in := r.innermost()
	if strings.HasPrefix(rule.prelude, "@") {
		rule.depth, rule.parent = r.open, in
		r.s.rules = append(r.s.rules, rule)
		return
	}
	d, ok := r.s.declarationOf(r.start, end)
	switch {
	case !ok:
	case in >= 0:
		r.s.rules[in].decls = append(r.s.rules[in].decls, d)
	default:
		r.s.decls = append(r.s.decls, d)
	}
}

// customProperty reports whether the tokens of s from start to end start a declaration of a variable,
// --x:, whose value a { goes on.
func (s *cssSheet) customProperty(start, end int) bool {
	var kinds []css.TokenType
	for _, t := range s.tokens[start:end] {
		if t.typ != css.WhitespaceToken && t.typ != css.CommentToken {
			kinds = append(kinds, t.typ)
		}
	}
	return len(kinds) >= 2 && kinds[0] == css.CustomPropertyNameToken && kinds[1] == css.ColonToken
}

// ruleOf is the statement written in the tokens of s from start to end, from its first token but spaces
// and comments, its comments spaces but their line breaks, and without the spaces after it. One of no
// token is at end, the { of a block of no prelude.
func (s *cssSheet) ruleOf(start, end int) cssRule {
	r := cssRule{
		prelude: "", at: len(s.src), block: false, depth: 0, parent: -1, first: end, open: -1, end: -1, decls: nil,
	}
	if end < len(s.tokens) {
		r.at = s.tokens[end].at
	}
	var text strings.Builder
	for i := start; i < end; i++ {
		t := s.tokens[i]
		blank := t.typ == css.CommentToken || t.typ == css.WhitespaceToken
		switch {
		case text.Len() == 0 && blank:
			continue
		case text.Len() == 0:
			r.at, r.first = t.at, i
		}
		if t.typ == css.CommentToken {
			text.WriteString(strings.Map(func(c rune) rune {
				if c == '\n' {
					return c
				}
				return ' '
			}, t.text))
			continue
		}
		text.WriteString(t.text)
	}
	r.prelude = strings.TrimRight(text.String(), " \t\r\n\f")
	return r
}

// cssDeclaration is a declaration of a block: its name, lower case but for a variable's, its value
// without !important, trimmed, whether it is !important, and the offset it starts at.
type cssDeclaration struct {
	name, value string
	important   bool
	at          int
}

// declarationOf returns the declaration in the tokens of s from start to end, false for none.
func (s *cssSheet) declarationOf(start, end int) (cssDeclaration, bool) {
	var toks []cssToken
	for _, t := range s.tokens[start:end] {
		if t.typ != css.CommentToken {
			toks = append(toks, t)
		}
	}
	toks = trimSpaces(toks)
	if len(toks) == 0 {
		return cssDeclaration{name: "", value: "", important: false, at: 0}, false
	}
	d := cssDeclaration{name: "", value: "", important: false, at: toks[0].at}
	name := toks[0]
	rest := trimSpaces(toks[1:])
	if len(rest) == 0 || rest[0].typ != css.ColonToken {
		d.name = joinTokens(toks)
		return d, true
	}
	d.name = name.text
	if name.typ != css.CustomPropertyNameToken {
		d.name = strings.ToLower(name.text)
	}
	value := trimSpaces(rest[1:])
	if n := len(value); n >= 2 && value[n-1].typ == css.IdentToken && strings.EqualFold(value[n-1].text, "important") {
		if bang := trimSpaces(value[:n-1]); len(bang) > 0 && bang[len(bang)-1].text == "!" {
			d.important, value = true, trimSpaces(bang[:len(bang)-1])
		}
	}
	d.value = joinTokens(value)
	return d, true
}

// trimSpaces returns toks without the spaces at either end.
func trimSpaces(toks []cssToken) []cssToken {
	for len(toks) > 0 && toks[0].typ == css.WhitespaceToken {
		toks = toks[1:]
	}
	for len(toks) > 0 && toks[len(toks)-1].typ == css.WhitespaceToken {
		toks = toks[:len(toks)-1]
	}
	return toks
}

// joinTokens returns the text of toks.
func joinTokens(toks []cssToken) string {
	var b strings.Builder
	for _, t := range toks {
		b.WriteString(t.text)
	}
	return b.String()
}
