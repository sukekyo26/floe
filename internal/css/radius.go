package css

import (
	"slices"
	"strings"

	"github.com/tdewolff/parse/v2/css"

	"floe/internal/textpos"
)

// Items splits a value into its items at its own depth: a function or a bracket with what it holds, or a token. Give it
// the value's tokens without whitespace and comments.
func Items(value []Token) [][]Token {
	if len(value) == 0 {
		return nil
	}
	base := value[0].Depth
	var out [][]Token
	start := -1
	for i, t := range value {
		switch {
		case t.Depth > base:
		case start >= 0:
			out = append(out, value[start:i+1])
			start = -1
		case opens(t):
			start = i
		default:
			out = append(out, value[i:i+1])
		}
	}
	if start >= 0 {
		out = append(out, value[start:])
	}
	return out
}

// opens is whether t opens a block or a function.
func opens(t Token) bool {
	return t.Type == css.FunctionToken || t.Type == css.LeftParenthesisToken || t.Type == css.LeftBracketToken ||
		t.Type == css.LeftBraceToken
}

// oneCorner are the palette's corners that floe's parts read as one corner: --radius-small in a bar's rx, which takes
// one length, and --radius-pill in a pill's min(). A value of two items voids them without a word.
var oneCorner = []string{"--radius-small", "--radius-pill"}

// Corner is a declaration of a corner of oneCorner whose value is not one item: the line and column of its value (of
// its name when it has none), its name and its value.
type Corner struct {
	Line, Column int
	Name, Value  string
}

// RadiusLength returns each declaration in src of --radius-small or --radius-pill whose value, !important left out, is
// not one item (Items): a length or one function passes, two lengths or a / do not. It reads the value alone: a name
// after a block's { or a ; (or at the start), a colon, and the tokens up to the ; or } of the name's depth; never a
// selector nor where the declaration is.
// LIMIT: a variable that holds two lengths and is read here (--radius-pill: var(--hand)) passes; the value of the
// variable is not followed.
func RadiusLength(src []byte) []Corner {
	src = Preprocess(src)
	toks := Tokens(src)
	var found []Corner
	begins := true
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		switch t.Type {
		case css.WhitespaceToken, css.CommentToken:
			continue
		case css.LeftBraceToken, css.SemicolonToken, css.RightBraceToken:
			begins = true
			continue
		default:
		}
		starts := begins
		begins = false
		if !starts || t.Type != css.CustomPropertyNameToken || !slices.Contains(oneCorner, string(t.Data)) {
			continue
		}
		value, end, ok := valueOf(toks, i)
		if ok && len(Items(value)) != 1 {
			at := t.At
			if len(value) > 0 {
				at = value[0].At
			}
			line, column := textpos.Of(src, at)
			found = append(found, Corner{Line: line, Column: column, Name: string(t.Data), Value: written(src, value)})
		}
		i = end - 1
	}
	return found
}

// valueOf returns the value of the declaration whose name is toks[i], without whitespace, comments and !important, and
// the place of the token that ends it, the ; or } of the name's depth; not ok when no colon follows the name, or a { of
// its depth comes first (a nested rule's prelude).
func valueOf(toks []Token, i int) ([]Token, int, bool) {
	depth := toks[i].Depth
	j := next(toks, i+1)
	if j == len(toks) || toks[j].Type != css.ColonToken {
		return nil, j, false
	}
	var value []Token
	for j++; j < len(toks); j++ {
		v := toks[j]
		if v.Depth == depth && v.Type == css.SemicolonToken || v.Depth < depth {
			break
		}
		if v.Depth == depth && v.Type == css.LeftBraceToken {
			return nil, j, false
		}
		if v.Type != css.WhitespaceToken && v.Type != css.CommentToken {
			value = append(value, v)
		}
	}
	return unimportant(value), j, true
}

// unimportant returns value without the !important at its end.
func unimportant(value []Token) []Token {
	if n := len(value); n >= 2 && value[n-2].Type == css.DelimToken && string(value[n-2].Data) == "!" &&
		value[n-1].Type == css.IdentToken && strings.EqualFold(string(value[n-1].Data), "important") {
		return value[:n-2]
	}
	return value
}

// written returns value as src writes it, from its first token to its last, "" for none.
func written(src []byte, value []Token) string {
	if len(value) == 0 {
		return ""
	}
	last := value[len(value)-1]
	return string(src[value[0].At : last.At+len(last.Data)])
}

// next returns the place of the first token from i that is no whitespace nor comment, len(toks) for none.
func next(toks []Token, i int) int {
	for i < len(toks) && (toks[i].Type == css.WhitespaceToken || toks[i].Type == css.CommentToken) {
		i++
	}
	return i
}
