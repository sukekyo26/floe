// Package css checks the CSS floe puts in a page for what floe's way of putting it there needs: it reads
// tdewolff's tokens and the nesting of their blocks, as CSS Syntax does, never the rules' selectors and values.
package css

import (
	"bytes"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"

	"github.com/sukekyo26/floe/internal/textpos"
	"github.com/sukekyo26/floe/internal/usererr"
)

// Preprocess makes each line break of src a LF, as CSS Syntax reads them before its tokens: CRLF, CR and FF.
// tdewolff does not, and would split \69<CR><LF>mport into two tokens where a browser reads one. Check reads
// the CSS made so, which is what goes in the page.
// LIMIT: an editor breaks no line at a FF, so the lines told after one are one more than it shows.
func Preprocess(src []byte) []byte {
	if !bytes.ContainsAny(src, "\r\f") {
		return src
	}
	src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	src = bytes.ReplaceAll(src, []byte("\r"), []byte("\n"))
	return bytes.ReplaceAll(src, []byte("\f"), []byte("\n"))
}

// token is a token of tdewolff's lexer, and the byte it starts at.
type token struct {
	tt   css.TokenType
	data []byte
	at   int
}

// lex reads the tokens of src. tdewolff writes a 0 past the end of the bytes it reads, so it is given a copy:
// src may be a part of a token's, or the bytes a Kit holds.
func lex(src []byte) []token {
	l := css.NewLexer(parse.NewInputBytes(bytes.Clone(src)))
	var toks []token
	at := 0
	for {
		tt, data := l.Next()
		if tt == css.ErrorToken {
			return toks
		}
		toks = append(toks, token{tt: tt, data: data, at: at})
		at += len(data)
	}
}

// sheet is a CSS file read into tokens, and the mistakes told of it.
type sheet struct {
	file  string
	src   []byte
	toks  []token
	found usererr.List
}

// read reads src, made by Preprocess.
func read(file string, src []byte) *sheet {
	return &sheet{file: file, src: src, toks: lex(src), found: nil}
}

// mistake adds the mistake key at the token t.
func (s *sheet) mistake(t token, key string) {
	line, column := textpos.Of(s.src, t.at)
	s.found = append(s.found, usererr.NewAt(s.file, line, column, key))
}

// Check refuses in src, the CSS file file made by Preprocess: a } that closes no block, as it would close
// floe's @layer and leave the rules after it out of it; a block, a string, a url or a comment still open at
// the end, which would take the } that closes the layer; an @import, which a browser does not read in a
// layer; an escape (\) outside a string, a url and a comment, which floe does not read; and a token a browser
// reads otherwise than tdewolff (a string or url cut short, a url( holding more than one string).
func Check(file string, src []byte) error {
	return read(file, src).check().Sorted().Err()
}

// nest is the openers of the blocks a token is in, as CSS Syntax reads its simple blocks and functions: {, (,
// a function and [. Only the closer of the innermost closes it: in ( ) and [ ], a } is a token of the value.
type nest []token

var closer = map[css.TokenType]css.TokenType{
	css.RightBraceToken: css.LeftBraceToken, css.RightParenthesisToken: css.LeftParenthesisToken,
	css.RightBracketToken: css.LeftBracketToken,
}

// opener is the token type of the block t opens: a function's is (.
func opener(t token) css.TokenType {
	if t.tt == css.FunctionToken {
		return css.LeftParenthesisToken
	}
	return t.tt
}

// step takes the token t into n: true when t closes the innermost block.
func (n *nest) step(t token) bool {
	switch t.tt {
	case css.LeftBraceToken, css.LeftParenthesisToken, css.LeftBracketToken, css.FunctionToken:
		*n = append(*n, t)
	case css.RightBraceToken, css.RightParenthesisToken, css.RightBracketToken:
		if len(*n) > 0 && opener((*n)[len(*n)-1]) == closer[t.tt] {
			*n = (*n)[:len(*n)-1]
			return true
		}
	default:
	}
	return false
}

func (s *sheet) check() usererr.List {
	var open nest
	misread := false // a token a browser reads otherwise, after which the nesting is not the browser's
	for _, t := range s.toks {
		top := len(open) == 0
		closed := open.step(t)
		switch t.tt {
		case css.RightBraceToken:
			// One that closes nothing at the top would close floe's layer.
			if top && !closed {
				s.mistake(t, "css_brace")
			}
		case css.BadStringToken, css.BadURLToken:
			s.mistake(t, "css_misread")
			misread = true
			continue
		case css.URLToken:
			if !oneString(t.data) {
				s.mistake(t, "css_misread")
				misread = true
			}
			continue
		case css.StringToken, css.CommentToken:
			continue
		case css.AtKeywordToken:
			if parse.EqualFold(t.data[1:], []byte("import")) {
				s.mistake(t, "css_import")
			}
		default:
		}
		if bytes.IndexByte(t.data, '\\') >= 0 {
			s.mistake(t, "css_escape")
			misread = true
		}
	}
	if !misread {
		s.unclosed(open)
	}
	return s.found
}

// unclosed refuses what is still open at the end of the file, open the blocks: the last token, or else the
// innermost block, whose closing is what lets the next be seen.
func (s *sheet) unclosed(open nest) {
	if n := len(s.toks); n > 0 && !closed(s.toks[n-1]) {
		s.mistake(s.toks[n-1], "css_unclosed")
	} else if len(open) > 0 {
		s.mistake(open[len(open)-1], "css_unclosed")
	}
}

// closed reports whether the token t, the last of a file, ends as it begins: a comment with */, a string
// with its quote and a url with ), neither escaped. tdewolff ends each at the end of the file without them.
func closed(t token) bool {
	switch t.tt {
	case css.CommentToken:
		return len(t.data) >= len("/**/") && bytes.HasSuffix(t.data, []byte("*/"))
	case css.StringToken:
		return len(t.data) >= 2 && unescaped(t.data, t.data[0])
	case css.URLToken:
		return unescaped(t.data, ')')
	default:
		return true
	}
}

// unescaped reports whether data ends in end, with no \ escaping it: an even count of \ before it.
func unescaped(data []byte, end byte) bool {
	n := len(data)
	if n == 0 || data[n-1] != end {
		return false
	}
	slashes := 0
	for i := n - 2; i >= 0 && data[i] == '\\'; i-- {
		slashes++
	}
	return slashes%2 == 0
}

// oneString reports whether the url token data holds what a browser reads as tdewolff does: an address, or
// one string, alone between spaces. A browser reads url( and a quote as a function and a string.
// LIMIT: tdewolff v2.8.20 makes every url( of a string with more after it a BadURL already; this guards the
// form against a version that does not.
func oneString(data []byte) bool {
	inside := data[len("url("):]
	inside = bytes.TrimSuffix(inside, []byte(")"))
	inside = bytes.TrimLeft(inside, " \t\n")
	if len(inside) == 0 || inside[0] != '"' && inside[0] != '\'' {
		return true
	}
	strings := 0
	for _, t := range lex(inside) {
		switch t.tt {
		case css.WhitespaceToken:
		case css.StringToken:
			strings++
		default:
			return false
		}
	}
	return strings == 1
}
