package tmpl

import (
	"strings"

	"github.com/tdewolff/parse/v2/css"

	"github.com/sukekyo26/floe/internal/usererr"
)

// colourWarnings returns the warnings of s, CSS of the parts' layer shown as file (a part's part.css, or a
// theme's base.css or file of a part): a colour it writes as a value, where the palette gives the colours,
// and, for a theme's, where ownVariables is false, a variable it sets, which belongs in its palette.css,
// under the colours a template sets on :scope. A mask's colours are its opacity alone, and pass.
// LIMIT: the text of url() and strings goes unread, a data: image's colours among them. Read them if a
// part or a theme comes to draw an image but in a mask.
func colourWarnings(file string, s *cssSheet, ownVariables bool) []*usererr.Error {
	var warnings []*usererr.Error
	for _, r := range s.rules {
		for _, d := range r.decls {
			line := lineAt(s.src, d.at)
			switch {
			case d.value == "":
			case strings.HasPrefix(d.name, "--") && !ownVariables:
				warnings = append(warnings, usererr.New("tmpl_theme_variable", file, line, d.name))
			case strings.HasPrefix(strings.TrimPrefix(d.name, "-webkit-"), "mask"):
			default:
				if c := colourIn(d.value, takesColour(d.name)); c != "" {
					warnings = append(warnings, usererr.New("tmpl_css_colour", file, line, c))
				}
			}
		}
	}
	return warnings
}

// colourIn returns the first colour value writes, as written, "" for none: a hex colour, a colour function
// with its arguments, or, where named is true, a colour's name. transparent and currentColor take no colour
// of their own, and color-mix() and light-dark() are told by the colours they mix.
func colourIn(value string, named bool) string {
	toks := readCSS([]byte(value)).tokens
	for i, t := range toks {
		switch {
		case t.typ == css.HashToken:
			return t.text
		case t.typ == css.FunctionToken && colourFunctions[strings.ToLower(strings.TrimSuffix(t.text, "("))]:
			return functionText(toks[i:])
		case named && t.typ == css.IdentToken && colourNames[strings.ToLower(t.text)]:
			return t.text
		}
	}
	return ""
}

// functionText returns the text of the function toks starts with, to the ) that closes it.
func functionText(toks []cssToken) string {
	depth := 0
	for i, t := range toks {
		switch t.typ {
		case css.FunctionToken, css.LeftParenthesisToken:
			depth++
		case css.RightParenthesisToken:
			if depth--; depth == 0 {
				return joinTokens(toks[:i+1])
			}
		default:
		}
	}
	return joinTokens(toks)
}

// takesColour reports whether the property name takes a colour, so that a word of its value that is a
// colour's name is one: elsewhere it may be another word (font-family: Arial Black, a variable's value).
func takesColour(name string) bool {
	for _, prefix := range []string{"background", "border", "outline", "column-rule", "text-decoration", "-webkit-text-"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return colourProperties[name]
}

var colourProperties = setOf(`color box-shadow text-shadow fill stroke caret-color accent-color
	text-emphasis-color scrollbar-color stop-color flood-color lighting-color`)

var colourFunctions = setOf("rgb rgba hsl hsla hwb lab lch oklab oklch color")

// colourNames are CSS's named colours and its system colours.
var colourNames = setOf(`aliceblue antiquewhite aqua aquamarine azure beige bisque black blanchedalmond blue
	blueviolet brown burlywood cadetblue chartreuse chocolate coral cornflowerblue cornsilk crimson cyan darkblue
	darkcyan darkgoldenrod darkgray darkgreen darkgrey darkkhaki darkmagenta darkolivegreen darkorange darkorchid
	darkred darksalmon darkseagreen darkslateblue darkslategray darkslategrey darkturquoise darkviolet deeppink
	deepskyblue dimgray dimgrey dodgerblue firebrick floralwhite forestgreen fuchsia gainsboro ghostwhite gold
	goldenrod gray green greenyellow grey honeydew hotpink indianred indigo ivory khaki lavender lavenderblush
	lawngreen lemonchiffon lightblue lightcoral lightcyan lightgoldenrodyellow lightgray lightgreen lightgrey
	lightpink lightsalmon lightseagreen lightskyblue lightslategray lightslategrey lightsteelblue lightyellow lime
	limegreen linen magenta maroon mediumaquamarine mediumblue mediumorchid mediumpurple mediumseagreen
	mediumslateblue mediumspringgreen mediumturquoise mediumvioletred midnightblue mintcream mistyrose moccasin
	navajowhite navy oldlace olive olivedrab orange orangered orchid palegoldenrod palegreen paleturquoise
	palevioletred papayawhip peachpuff peru pink plum powderblue purple rebeccapurple red rosybrown royalblue
	saddlebrown salmon sandybrown seagreen seashell sienna silver skyblue slateblue slategray slategrey snow
	springgreen steelblue tan teal thistle tomato turquoise violet wheat white whitesmoke yellow yellowgreen
	accentcolor accentcolortext activetext buttonborder buttonface buttontext canvas canvastext field fieldtext
	graytext highlight highlighttext linktext mark marktext selecteditem selecteditemtext visitedtext`)

func setOf(words string) map[string]bool {
	set := map[string]bool{}
	for _, w := range strings.Fields(words) {
		set[w] = true
	}
	return set
}
