// Package css checks the CSS floe puts in a page for what floe's way of putting it there needs: it reads
// tdewolff's tokens and the nesting of their blocks, as CSS Syntax does, never the rules' selectors and values.
package css

import (
	"bytes"
	"unicode/utf8"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"

	"github.com/sukekyo26/floe/internal/usererr"
)

// Preprocess makes each line break of src a LF, as CSS Syntax reads them before its tokens: CRLF, CR and FF.
// tdewolff does not, and would split \69<CR><LF>mport into two tokens where a browser reads one.
func Preprocess(src []byte) []byte {
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

func lex(src []byte) []token {
	l := css.NewLexer(parse.NewInputBytes(src))
	var toks []token
	at := 0
	for {
		tt, data := l.Next()
		if tt == css.ErrorToken {
			return toks
		}
		toks = append(toks, token{tt: tt, data: bytes.Clone(data), at: at})
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

func read(file string, src []byte) *sheet {
	src = Preprocess(src)
	return &sheet{file: file, src: src, toks: lex(src), found: nil}
}

// mistake adds the mistake key at the token t.
// LIMIT: the column counts the runes of the line; a tab is one.
func (s *sheet) mistake(t token, key string) {
	line := 1 + bytes.Count(s.src[:t.at], []byte("\n"))
	column := 1 + utf8.RuneCount(s.src[bytes.LastIndexByte(s.src[:t.at], '\n')+1:t.at])
	s.found = append(s.found, usererr.NewAt(s.file, line, column, key))
}

// Check refuses in src, the CSS file file: a } that closes no block, as it would close floe's @layer and
// leave the rules after it out of it; an @import, which a browser does not read in a layer; an escape (\)
// outside a string, a url and a comment, which floe does not read; and a token a browser reads otherwise
// than tdewolff (a string or url cut short, a url( holding more than one string).
func Check(file string, src []byte) error {
	return read(file, src).check().Sorted().Err()
}

// nest is the openers of the blocks a token is in, as CSS Syntax reads its simple blocks and functions: {, (
// (a function's too) and [. Only the closer of the innermost closes it: in ( ) and [ ], a } is a token of the
// value.
type nest []css.TokenType

var closer = map[css.TokenType]css.TokenType{
	css.RightBraceToken: css.LeftBraceToken, css.RightParenthesisToken: css.LeftParenthesisToken,
	css.RightBracketToken: css.LeftBracketToken,
}

// step takes the token t into n: true when t closes the innermost block.
func (n *nest) step(t token) bool {
	switch t.tt {
	case css.LeftBraceToken, css.LeftParenthesisToken, css.LeftBracketToken:
		*n = append(*n, t.tt)
	case css.FunctionToken:
		*n = append(*n, css.LeftParenthesisToken)
	case css.RightBraceToken, css.RightParenthesisToken, css.RightBracketToken:
		if len(*n) > 0 && (*n)[len(*n)-1] == closer[t.tt] {
			*n = (*n)[:len(*n)-1]
			return true
		}
	default:
	}
	return false
}

func (s *sheet) check() usererr.List {
	var open nest
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
			continue
		case css.URLToken:
			if !oneString(t.data) {
				s.mistake(t, "css_misread")
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
		}
	}
	return s.found
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
