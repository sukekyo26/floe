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
// ends, the corners of the parent)
var radiusShapes = []string{"0", "50%", "999px", "inherit"}

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

// TestRadius keeps the round of every corner of floe's CSS in the palette: a corner reads a variable, or is a shape.
// The palette's own corners are one length each.
func TestRadius(t *testing.T) {
	t.Parallel()

	floe := os.DirFS("../kit/floe")
	checked := 0
	err := fs.WalkDir(floe, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(name) != ".css" {
			return err
		}
		src, err := fs.ReadFile(floe, name)
		if err != nil {
			return err
		}
		corner := func(name string) bool { return strings.HasSuffix(name, "radius") || slices.Contains(paletteRadii, name) }
		for prop, values := range declarations(src, corner) {
			for _, value := range values {
				if strings.HasPrefix(prop, "--") {
					if slices.Contains(paletteRadii, prop) && (path.Base(name) != "palette.css" || len(value) != 1) {
						t.Errorf("%s: %s: %s; the palette sets it to one length", name, prop, text(src, value))
					}
					continue
				}
				checked++
				reads := slices.ContainsFunc(value, func(t token) bool {
					return t.tt == css.FunctionToken && strings.EqualFold(string(t.data), "var(")
				})
				if !reads && !slices.Contains(radiusShapes, text(src, value)) {
					t.Errorf("%s: %s: %s; read --radius, --radius-small or --radius-pill, or write a shape (%s)",
						name, prop, text(src, value), strings.Join(radiusShapes, ", "))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("no corner found in floe's CSS")
	}
}
