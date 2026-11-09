package lint

import (
	"fmt"
	"slices"
	"strings"

	"github.com/tdewolff/parse/v2/css"
)

// paletteRadii are the palette's corners, which floe's CSS reads within calc() and as one corner of several: each is one
// length.
var paletteRadii = []string{"--radius", "--radius-small", "--radius-pill"}

// radiusRule keeps the round of every corner of floe's CSS in the palette: a corner (rx, or a property ending in radius)
// is a shape (cornerShapes), or takes all its lengths from variables set in a palette, the root of a theme's base.css or
// the root of floe's base.css alone. The palette's three corners are set in a palette alone, one length each.
// LIMIT: clip-path's inset(… round …), corner-shape, SVG's rx= attribute and style attributes are not read; floe's CSS
// writes none.
var radiusRule = Rule{Name: "radius", Check: func(in *Input) []Finding {
	type corner struct {
		file  string
		line  int
		text  string
		reads []string
	}
	var found []Finding
	setIn := map[string][]string{} // a variable: where it is set where corners may not read it
	setOK := map[string]bool{}     // a variable: set where corners may read it
	var corners []corner
	for _, s := range in.Sheets() {
		for name, ds := range declarations(s.Src, func(p string) bool { return strings.HasPrefix(p, "--") }) {
			for _, d := range ds {
				if setsCorners(s, d.rule) {
					setOK[name] = true
				} else {
					setIn[name] = append(setIn[name], s.File+" "+d.rule)
				}
				if slices.Contains(paletteRadii, name) && (s.For != "palette" || len(d.value) != 1) {
					file, line := s.at(d.at)
					found = append(found, Finding{file, line, "radius", name + ": " + text(s.Src, d.value) + "; the palette sets it to one length"})
				}
			}
		}
		for prop, ds := range declarations(s.Src, func(p string) bool {
			return p == "rx" || strings.HasSuffix(p, "radius") && !strings.HasPrefix(p, "--")
		}) {
			for _, d := range ds {
				in.Saw()
				v := text(s.Src, d.value)
				if slices.Contains(cornerShapes, v) {
					continue
				}
				file, line := s.at(d.at)
				if !lengthless(d.value) {
					found = append(found, Finding{file, line, "radius", fmt.Sprintf(
						"%s: %s; write var() of a variable, calc() or min() of those, with no length nor number but 0 (a gap in calc() too), or a shape (%s)", prop, v, strings.Join(cornerShapes, ", "))})
					continue
				}
				c := corner{file: file, line: line, text: prop + ": " + v}
				for _, t := range d.value {
					if t.Type == css.CustomPropertyNameToken {
						c.reads = append(c.reads, string(t.Data))
					}
				}
				corners = append(corners, c)
			}
		}
	}
	for _, c := range corners {
		for _, v := range c.reads {
			switch {
			case !setOK[v]:
				found = append(found, Finding{c.file, c.line, "radius", fmt.Sprintf(
					"%s; %s is set in no palette nor the root of a base: set it there", c.text, v)})
			case len(setIn[v]) > 0:
				found = append(found, Finding{c.file, c.line, "radius", fmt.Sprintf(
					"%s; %s is set in %s too, and a corner reads a variable set in a palette or the root of a base alone",
					c.text, v, strings.Join(setIn[v], ", "))})
			}
		}
	}
	return found
}}

// setsCorners is whether a variable a corner reads may be set in rule of s: a palette, where a theme's corners are, the
// root of a theme's base.css, which holds what is the theme's own alone (default's palette lies under every theme's), or
// the :root of floe's base.css, which holds floe's own (--floe-room).
func setsCorners(s Sheet, rule string) bool {
	switch {
	case s.For == "palette":
		return true
	case s.Kind == kindBase:
		return rootOf(rule, ":root") == onTheRoot
	default:
		return s.For == "base" && rootOf(rule, ":scope") == onTheRoot
	}
}

// lengthless is whether a corner's value takes every length from a variable: var() of one name, calc(), min() of such
// values (a comma right in min() alone), their operators and the corner 0, with no length nor fallback of its own.
func lengthless(value []token) bool {
	for i, t := range value {
		switch t.Type {
		case css.FunctionToken:
			if f := strings.ToLower(string(t.Data)); f != "var(" && f != "calc(" && f != "min(" {
				return false
			}
		case css.CommaToken:
			if f := around(value[:i], t.Depth); !strings.EqualFold(f, "min(") {
				return false
			}
		case css.RightParenthesisToken, css.CustomPropertyNameToken, css.DelimToken:
		case css.NumberToken:
			if string(t.Data) != "0" {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// around returns the function a token of depth after before is right in, "" for none.
func around(before []token, depth int) string {
	for i := len(before) - 1; i >= 0; i-- {
		if before[i].Depth < depth {
			if before[i].Type == css.FunctionToken {
				return string(before[i].Data)
			}
			return ""
		}
	}
	return ""
}
