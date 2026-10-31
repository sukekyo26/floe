package css

import (
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/tdewolff/parse/v2/css"
)

// radiusShapes are the corners floe's CSS may write as they are: shapes, not sizes (a square, a circle, a pill's
// ends)
var radiusShapes = []string{"0", "50%", "999px"}

// paletteRadii are the palette's corners, which floe's CSS reads within calc() and as one corner of several: each
// is one length
var paletteRadii = []string{"--radius", "--radius-small", "--radius-pill"}

// declarations returns each declaration in src of a property that want takes, by name, with the tokens of its
// value, read by the tokens alone.
func declarations(src []byte, want func(name string) bool) map[string][][]token {
	found := map[string][][]token{}
	toks := lex(src)
	for i := 0; i < len(toks); i++ {
		name := strings.ToLower(string(toks[i].data))
		if (toks[i].tt != css.IdentToken && toks[i].tt != css.CustomPropertyNameToken) || !want(name) {
			continue
		}
		j := i + 1
		for j < len(toks) && toks[j].tt == css.WhitespaceToken {
			j++
		}
		if j == len(toks) || toks[j].tt != css.ColonToken {
			continue
		}
		var value []token
		depth := 0
		for j++; j < len(toks); j++ {
			tt := toks[j].tt
			if depth == 0 && (tt == css.SemicolonToken || tt == css.RightBraceToken) {
				break
			}
			switch tt {
			case css.FunctionToken, css.LeftParenthesisToken:
				depth++
			case css.RightParenthesisToken:
				depth--
			case css.WhitespaceToken, css.CommentToken:
				continue
			default:
			}
			value = append(value, toks[j])
		}
		found[name] = append(found[name], value)
		i = j
	}
	return found
}

// text returns the value as src writes it, from its first token to its last.
func text(src []byte, value []token) string {
	if len(value) == 0 {
		return ""
	}
	last := value[len(value)-1]
	return string(src[value[0].at : last.at+len(last.data)])
}

// lengthless is whether a corner's value takes every length from a variable: var() of one name, calc(), their
// operators and the corner 0, with no length nor fallback of its own.
func lengthless(value []token) bool {
	for _, t := range value {
		switch t.tt {
		case css.FunctionToken:
			if f := strings.ToLower(string(t.data)); f != "var(" && f != "calc(" {
				return false
			}
		case css.CustomPropertyNameToken, css.RightParenthesisToken, css.DelimToken:
		case css.NumberToken:
			if string(t.data) != "0" {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// setsCorners is whether a file may set a variable a corner reads: a palette, where a theme's corners are, a
// theme's base.css, whose root holds what is the theme's own alone (default's palette lies under every theme's), or
// floe's base, which holds floe's own (--floe-room).
func setsCorners(name string) bool {
	return path.Base(name) == "palette.css" || path.Base(name) == "base.css"
}

// TestRadius keeps the round of every corner of floe's CSS in the palette: a corner is a shape, or takes all its
// lengths from variables set in a palette or a base alone. The palette's three corners are one length each.
func TestRadius(t *testing.T) {
	t.Parallel()

	floe := os.DirFS("../kit/floe")
	setIn := map[string][]string{} // a variable: the files that set it
	type corner struct {
		file, prop, value string
		reads             []string
	}
	var corners []corner
	err := fs.WalkDir(floe, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(name) != ".css" {
			return err
		}
		src, err := fs.ReadFile(floe, name)
		if err != nil {
			return err
		}
		for prop, values := range declarations(src, func(p string) bool { return strings.HasPrefix(p, "--") }) {
			setIn[prop] = append(setIn[prop], name)
			for _, value := range values {
				if slices.Contains(paletteRadii, prop) && (path.Base(name) != "palette.css" || len(value) != 1) {
					t.Errorf("%s: %s: %s; the palette sets it to one length", name, prop, text(src, value))
				}
			}
		}
		for prop, values := range declarations(src, func(p string) bool { return strings.HasSuffix(p, "radius") && !strings.HasPrefix(p, "--") }) {
			for _, value := range values {
				if slices.Contains(radiusShapes, text(src, value)) {
					continue
				}
				if !lengthless(value) {
					t.Errorf("%s: %s: %s; read --radius, --radius-small or --radius-pill, or write a shape (%s)",
						name, prop, text(src, value), strings.Join(radiusShapes, ", "))
					continue
				}
				c := corner{file: name, prop: prop, value: text(src, value)}
				for _, tok := range value {
					if tok.tt == css.CustomPropertyNameToken {
						c.reads = append(c.reads, string(tok.data))
					}
				}
				corners = append(corners, c)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(corners) == 0 {
		t.Fatal("no corner found in floe's CSS")
	}
	for _, c := range corners {
		for _, v := range c.reads {
			if len(setIn[v]) == 0 || !slices.ContainsFunc(setIn[v], setsCorners) ||
				slices.ContainsFunc(setIn[v], func(f string) bool { return !setsCorners(f) }) {
				t.Errorf("%s: %s: %s; %s is set in %v, and a corner reads a variable set in a palette or a base alone",
					c.file, c.prop, c.value, v, setIn[v])
			}
		}
	}
}
