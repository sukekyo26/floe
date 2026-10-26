package css

import (
	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
)

// CheckPalette refuses what Check does in src, a theme's palette.css, and when it refuses none, anything but
// :root { } of declarations of --name and color-scheme, and a declaration's important flag: a palette is
// variables alone, which the themes over it set again, and !important would win over every layer. It reads
// the tokens as CSS Syntax does its blocks and declarations, not what the selectors and values mean.
func CheckPalette(file string, src []byte) error {
	s := read(file, src)
	if s.check() != nil {
		// Where the tokens are read otherwise, the shape would be told at the wrong places.
		return s.found.Sorted().Err()
	}
	p := &palette{sheet: s, i: 0}
	p.top()
	return s.found.Sorted().Err()
}

// palette walks the tokens of a palette.css, at i.
type palette struct {
	*sheet
	i int
}

// next returns the next token not a space or a comment, and false at the end.
func (p *palette) next() (token, bool) {
	for p.i < len(p.toks) {
		t := p.toks[p.i]
		p.i++
		if t.tt != css.WhitespaceToken && t.tt != css.CommentToken {
			return t, true
		}
	}
	return token{tt: css.ErrorToken, data: nil, at: 0}, false
}

// top reads the rules at the top: :root { declarations } alone.
func (p *palette) top() {
	for {
		t, ok := p.next()
		if !ok {
			return
		}
		if p.isRoot(t) {
			p.declarations()
			continue
		}
		p.mistake(t, "css_palette_rule")
		p.skip(t, false)
	}
}

// isRoot reports whether t, and the tokens after it, are :root and the { of its block, read up to it.
func (p *palette) isRoot(t token) bool {
	if t.tt != css.ColonToken {
		return false
	}
	at := p.i
	name, ok := p.next()
	brace, more := p.next()
	if ok && more && name.tt == css.IdentToken && parse.EqualFold(name.data, []byte("root")) &&
		brace.tt == css.LeftBraceToken {
		return true
	}
	p.i = at
	return false
}

// declarations reads the declarations of :root's block, up to the } that closes it.
func (p *palette) declarations() {
	for {
		t, ok := p.next()
		switch {
		case !ok || t.tt == css.RightBraceToken:
			return
		case t.tt == css.SemicolonToken:
		case t.tt == css.CustomPropertyNameToken:
			p.value(true)
		case t.tt == css.IdentToken && parse.EqualFold(t.data, []byte("color-scheme")):
			p.value(false)
		default:
			p.mistake(t, "css_palette_rule")
			if p.skip(t, true) {
				return
			}
		}
	}
}

// value reads the : and the value of a declaration, up to the ; or the } of the block, which it leaves to
// be read, refusing its important flag; a { in it is the value's in a custom property, else a rule.
func (p *palette) value(custom bool) {
	colon, ok := p.next()
	if !ok {
		return
	}
	if colon.tt != css.ColonToken {
		p.mistake(colon, "css_palette_rule")
		p.i--
		p.skip(colon, true)
		return
	}
	var top []token // the tokens of the value at its own level, but spaces and comments
	var open nest
	for p.i < len(p.toks) {
		t := p.toks[p.i]
		switch {
		case len(open) == 0 && (t.tt == css.SemicolonToken || t.tt == css.RightBraceToken):
			p.important(top)
			return
		case t.tt == css.LeftBraceToken && len(open) == 0 && !custom:
			p.mistake(t, "css_palette_rule")
			p.skip(t, true)
			return
		case len(open) == 0 && t.tt != css.WhitespaceToken && t.tt != css.CommentToken && !opens(t.tt):
			top = append(top, t)
		}
		open.step(t)
		p.i++
	}
	p.important(top)
}

// important refuses the important flag of a declaration whose value at its own level is top: its last two
// tokens ! and important.
func (p *palette) important(top []token) {
	n := len(top)
	if n >= 2 && top[n-2].tt == css.DelimToken && string(top[n-2].data) == "!" &&
		top[n-1].tt == css.IdentToken && parse.EqualFold(top[n-1].data, []byte("important")) {
		p.mistake(top[n-2], "css_palette_important")
	}
}

// skip reads past what t begins, refused: a rule up to the end of its block, or a statement up to its ;. In
// a block (inBlock), it stops at the } that closes the block, reads it, and reports that it did.
func (p *palette) skip(t token, inBlock bool) bool {
	var open nest
	open.step(t)
	for p.i < len(p.toks) {
		t := p.toks[p.i]
		p.i++
		switch {
		case len(open) == 0 && t.tt == css.SemicolonToken:
			return false
		case len(open) == 0 && t.tt == css.RightBraceToken && inBlock:
			return true
		case open.step(t) && len(open) == 0 && t.tt == css.RightBraceToken:
			return false
		}
	}
	return false
}

func opens(tt css.TokenType) bool {
	return tt == css.LeftBraceToken || tt == css.LeftParenthesisToken || tt == css.LeftBracketToken ||
		tt == css.FunctionToken
}
