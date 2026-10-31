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

// declaration is a declaration's value, read by the tokens alone, and the preludes of the rules it is in, outermost
// first, joined by " { ".
type declaration struct {
	value []token
	rule  string
}

// rules returns, for each token, the preludes of the rules it is in, outermost first and joined by " { " ("" outside
// every rule), comments left out.
func rules(toks []token) []string {
	in := make([]string, len(toks))
	var open []string
	var prelude strings.Builder
	for k, t := range toks {
		switch t.tt {
		case css.LeftBraceToken:
			open = append(open, strings.TrimSpace(prelude.String()))
			prelude.Reset()
		case css.RightBraceToken:
			if len(open) > 0 {
				open = open[:len(open)-1]
			}
			prelude.Reset()
		case css.SemicolonToken:
			prelude.Reset()
		case css.CommentToken:
		default:
			prelude.Write(t.data)
		}
		if len(open) > 0 {
			in[k] = strings.Join(open, " { ")
		}
	}
	return in
}

// declarations returns each declaration in src of a property that want takes, by name.
func declarations(src []byte, want func(name string) bool) map[string][]declaration {
	found := map[string][]declaration{}
	toks := lex(src)
	in := rules(toks)
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
		found[name] = append(found[name], declaration{value: value, rule: in[i]})
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

// place is where a variable is set: a file of floe's, and the prelude of the rule.
type place struct{ file, rule string }

// setsCorners is whether a variable a corner reads may be set at p: a palette, where a theme's corners are, the
// :scope of a theme's base.css, which holds what is the theme's own alone (default's palette lies under every
// theme's), or the :root of floe's base, which holds floe's own (--floe-room).
func setsCorners(p place) bool {
	switch {
	case path.Base(p.file) == "palette.css":
		return true
	case p.file == "base.css":
		return isRoot(p.rule, ":root")
	default:
		return path.Base(p.file) == "base.css" && isRoot(p.rule, ":scope")
	}
}

// isRoot is whether rule, the preludes of a declaration's rules, is root alone, within @media rules or none: within
// @scope or another rule, :scope is another element.
func isRoot(rule, root string) bool {
	preludes := strings.Split(rule, " { ")
	for _, p := range preludes[:len(preludes)-1] {
		if !strings.HasPrefix(p, "@media") {
			return false
		}
	}
	return preludes[len(preludes)-1] == root
}

// TestRadius keeps the round of every corner of floe's CSS in the palette: a corner is a shape, or takes all its
// lengths from variables set in a palette or the root of a base alone. The palette's three corners are one length each.
func TestRadius(t *testing.T) {
	t.Parallel()

	floe := os.DirFS("../kit/floe")
	setIn := map[string][]place{} // a variable: where it is set
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
			for _, d := range values {
				setIn[prop] = append(setIn[prop], place{file: name, rule: d.rule})
				if slices.Contains(paletteRadii, prop) && (path.Base(name) != "palette.css" || len(d.value) != 1) {
					t.Errorf("%s: %s: %s; the palette sets it to one length", name, prop, text(src, d.value))
				}
			}
		}
		for prop, values := range declarations(src, func(p string) bool { return strings.HasSuffix(p, "radius") && !strings.HasPrefix(p, "--") }) {
			for _, d := range values {
				value := d.value
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
				slices.ContainsFunc(setIn[v], func(p place) bool { return !setsCorners(p) }) {
				t.Errorf("%s: %s: %s; %s is set in %v, and a corner reads a variable set in a palette or the root of a base alone",
					c.file, c.prop, c.value, v, setIn[v])
			}
		}
	}
}

// TestCornerRoot takes the root of a base, in @media alone, as where a corner's variable may be set.
func TestCornerRoot(t *testing.T) {
	t.Parallel()

	for src, want := range map[string]bool{
		":scope { --c: 1px; }":                           true,
		"@media print { :scope { --c: 1px; } }":          true,
		"@scope (.a) { :scope { --c: 1px; } }":           false,
		".a { :scope { --c: 1px; } }":                    false,
		":scope > div { --c: 1px; }":                     false,
		":scope, .a { --c: 1px; }":                       false,
		"/* the root */ :scope /* here */ { --c: 1px; }": true,
		"blockquote { --c: 1px; }":                       false,
	} {
		d := declarations([]byte(src), func(p string) bool { return p == "--c" })["--c"]
		if len(d) != 1 {
			t.Fatalf("%s: %d declarations of --c", src, len(d))
		}
		if got := setsCorners(place{file: "themes/x/base.css", rule: d[0].rule}); got != want {
			t.Errorf("%s: sets corners %v, want %v", src, got, want)
		}
	}
}
