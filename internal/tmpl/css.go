package tmpl

import (
	"bytes"
	"regexp"
	"strings"

	tparse "github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"

	"github.com/sukekyo26/floe/internal/usererr"
)

// ownCSS returns name, a file of t's CSS, empty when t has none, refused as refuseCSS refuses one of
// @scope: floe scopes the template's CSS to the page, @scope (:root).
func (t *Template) ownCSS(name string) ([]byte, error) {
	s, err := t.ownSheet(name)
	if err != nil {
		return nil, err
	}
	return s.src, nil
}

// ownSheet returns name, a file of t's CSS, read, refused as ownCSS refuses it.
func (t *Template) ownSheet(name string) (*cssSheet, error) {
	src, _, err := t.readOptional(name)
	if err != nil {
		return nil, err
	}
	s := readCSS(src)
	if err = s.refuse(t.file(name), true); err != nil {
		return nil, err
	}
	return s, nil
}

// refuseCSS refuses css, the file shown as file, as cssSheet.refuse does.
func refuseCSS(file string, src []byte, scoped bool) error {
	return readCSS(src).refuse(file, scoped)
}

// refuse refuses s, the file shown as file, that floe puts in a layer, where a } closes no block,
// which would close the layer around it and leave the rules after it out of it, over the parts' styles,
// or where it imports a stylesheet, which a browser does not read in a layer. CSS floe puts in @scope,
// scoped, is refused too where a selector names the root as :root or html, where :scope is the root and
// those name nothing.
func (s *cssSheet) refuse(file string, scoped bool) error {
	if s.stray >= 0 {
		return usererr.New("tmpl_css_brace", file, lineAt(s.src, s.stray))
	}
	for _, r := range s.rules {
		switch {
		case !r.block && strings.HasPrefix(strings.ToLower(r.prelude), "@import"):
			return usererr.New("tmpl_css_import", file, lineAt(s.src, r.at))
		case scoped && r.block && !strings.HasPrefix(r.prelude, "@"):
			if m := rootSelector.FindStringSubmatchIndex(r.prelude); m != nil {
				return usererr.New("tmpl_css_root", file, lineAt(s.src, r.at+m[2]), r.prelude[m[2]:m[3]])
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
// order, those in blocks too, and the offset of a } that closes no block, -1 for none.
type cssSheet struct {
	src    []byte
	tokens []cssToken
	rules  []cssRule
	stray  int
}

// cssToken is a token of CSS, as written, and the offset where it starts.
type cssToken struct {
	typ  css.TokenType
	text string
	at   int
}

// cssRule is what comes before a block of CSS, a selector or an at-rule's prelude, or an at-rule written
// as a statement (@import …;), as written without its comments, and the offset where it starts. A block's
// depth is how many blocks hold it; first, open and end are the indices of the tokens of the prelude's
// start, of its { and of its }, the end of the tokens for one left open, open and end -1 for a statement.
type cssRule struct {
	prelude          string
	at               int
	block            bool
	depth            int
	first, open, end int
}

// readCSS reads src, which floe's lexer of CSS Syntax reads as a browser does: a string, a comment, an
// escaped character or the URL of url(…) holds no brace or ; of the CSS's.
func readCSS(src []byte) *cssSheet {
	s := &cssSheet{src: src, tokens: nil, rules: nil, stray: -1}
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

// readRules reads the rules of s from its tokens. Within brackets, as in a function, a brace or a ; is the
// value's: a browser reads to the ) that closes them, or to the end.
func (s *cssSheet) readRules() {
	r := &ruleReader{s: s, stack: nil, open: 0, start: 0}
	for i, t := range s.tokens {
		if !r.read(i, t) {
			return
		}
	}
	// A statement the file ends without its ;, which the browser reads all the same.
	r.statement(len(s.tokens))
}

// ruleReader reads the rules of s: the blocks and brackets open, innermost last, the rule blocks among
// them, and the token the next rule starts at.
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
			r.statement(i)
			r.start = i + 1
		}
	default:
	}
	return true
}

// brace reads the { at i: a rule's block, or one of a value (--x: { … }) or in brackets.
func (r *ruleReader) brace(i int, inValue bool) {
	if inValue || r.s.customProperty(r.start, i) {
		r.stack = append(r.stack, opened{closer: css.RightBraceToken, rule: -1})
		return
	}
	rule := r.s.ruleOf(r.start, i)
	rule.block, rule.depth, rule.open, rule.end = true, r.open, i, len(r.s.tokens)
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
	r.stack = r.stack[:len(r.stack)-1]
	if top.rule >= 0 {
		r.s.rules[top.rule].end = i
		r.open--
		r.start = i + 1
	}
	return true
}

// statement notes the at-rule written as a statement from the start up to end, if it is one.
func (r *ruleReader) statement(end int) {
	if rule := r.s.ruleOf(r.start, end); strings.HasPrefix(rule.prelude, "@") {
		rule.depth = r.open
		r.s.rules = append(r.s.rules, rule)
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

// ruleOf is the statement written in the tokens of s from start to end, without its comments and the
// spaces around it.
func (s *cssSheet) ruleOf(start, end int) cssRule {
	r := cssRule{prelude: "", at: len(s.src), block: false, depth: 0, first: end, open: -1, end: -1}
	var text strings.Builder
	for i := start; i < end; i++ {
		t := s.tokens[i]
		if t.typ == css.CommentToken || text.Len() == 0 && t.typ == css.WhitespaceToken {
			continue
		}
		if text.Len() == 0 {
			r.at, r.first = t.at, i
		}
		text.WriteString(t.text)
	}
	r.prelude = strings.TrimSpace(text.String())
	return r
}

// cssDeclaration is a declaration of a block: its name, lower case but for a variable's, its value
// without !important, trimmed, whether it is !important, and the offset it starts at.
type cssDeclaration struct {
	name, value string
	important   bool
	at          int
}

// declarations returns the declarations of the block r of s, split at the ; outside brackets and the
// blocks of values, and at a rule it holds (&.x { … }), which it leaves out.
func (s *cssSheet) declarations(r *cssRule) []cssDeclaration {
	var decls []cssDeclaration
	depth, start := 0, r.open+1
	for i := start; i <= r.end; i++ {
		if i < r.end && i < len(s.tokens) {
			switch s.tokens[i].typ {
			case css.FunctionToken, css.LeftParenthesisToken, css.LeftBracketToken:
				depth++
				continue
			case css.RightParenthesisToken, css.RightBracketToken, css.RightBraceToken:
				depth = max(depth-1, 0)
				continue
			case css.LeftBraceToken:
				if depth == 0 && !s.customProperty(start, i) {
					i = s.ruleOpenAt(i).end
					start = i + 1
					continue
				}
				depth++
				continue
			case css.SemicolonToken:
				if depth > 0 {
					continue
				}
			default:
				continue
			}
		}
		if d, ok := s.declarationOf(start, min(i, len(s.tokens))); ok {
			decls = append(decls, d)
		}
		start = i + 1
	}
	return decls
}

// ruleOpenAt returns the rule of s whose { is the token at i.
func (s *cssSheet) ruleOpenAt(i int) *cssRule {
	for j := range s.rules {
		if s.rules[j].open == i {
			return &s.rules[j]
		}
	}
	return &cssRule{prelude: "", at: 0, block: true, depth: 0, first: i, open: i, end: len(s.tokens)}
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
