package tmpl

import (
	"slices"
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
	decls := slices.Clone(s.decls)
	for _, r := range s.rules {
		decls = append(decls, r.decls...)
	}
	slices.SortFunc(decls, func(a, b cssDeclaration) int { return a.at - b.at })
	var warnings []*usererr.Error
	for _, d := range decls {
		switch property := unprefixed(d.name); {
		case d.value == "":
		case strings.HasPrefix(d.name, "--") && !ownVariables:
			warnings = append(warnings, usererr.New("tmpl_theme_variable", file, lineAt(s.src, d.at), d.name))
		case strings.HasPrefix(property, "mask"):
		default:
			if c := colourIn(d.value, takesColour(property)); c != "" {
				warnings = append(warnings, usererr.New("tmpl_css_colour", file, lineAt(s.src, d.at), c))
			}
		}
	}
	return warnings
}

// unprefixed is the property name without a browser's prefix (-webkit-mask is mask).
func unprefixed(name string) string {
	if strings.HasPrefix(name, "--") {
		return name
	}
	for _, prefix := range []string{"-webkit-", "-moz-"} {
		name = strings.TrimPrefix(name, prefix)
	}
	return name
}

// colourIn returns the first colour value writes, as written, "" for none: a hex colour, a colour function
// with its arguments, or, where named is true, a colour's name. transparent and currentColor take no colour
// of their own, and color-mix(), light-dark() and a colour made from another (rgb(from var(--accent) r g b
// / 50%)) are told by the colours they take.
func colourIn(value string, named bool) string {
	toks := readCSS([]byte(value)).tokens
	for i, t := range toks {
		switch {
		case t.typ == css.HashToken:
			return t.text
		case t.typ == css.FunctionToken && colourFunctions[strings.ToLower(strings.TrimSuffix(t.text, "("))] &&
			!madeFrom(toks[i+1:]):
			return functionText(toks[i:])
		case named && t.typ == css.IdentToken && colourNames[strings.ToLower(t.text)]:
			return t.text
		}
	}
	return ""
}

// madeFrom reports whether the arguments toks starts with make a colour from another: from, then it.
func madeFrom(toks []cssToken) bool {
	toks = trimSpaces(toks)
	return len(toks) > 0 && toks[0].typ == css.IdentToken && strings.EqualFold(toks[0].text, "from")
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

// takesColour reports whether the property name, without a browser's prefix, takes a colour, so that a word
// of its value that is a colour's name is one: elsewhere it may be another word (font-family: Arial Black,
// a variable's value).
func takesColour(name string) bool {
	for _, prefix := range colourPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return strings.HasSuffix(name, "-color") || strings.HasSuffix(name, "shadow") || colourProperties[name]
}

var colourPrefixes = []string{
	"background", "border", "outline", "column-rule", "text-decoration", "text-emphasis", "text-stroke",
}

var colourProperties = setOf("color fill stroke filter backdrop-filter caret")

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
