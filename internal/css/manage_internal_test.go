package css

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/tdewolff/parse/v2/css"
	"golang.org/x/net/html"
)

// manageCorners are the management page's corners: its three steps, a box within a pop-up's or a menu's box (the outer
// corner less the room between them), and the shapes.
var manageCorners = []string{
	"var(--corner)", "var(--corner-menu)", "var(--corner-small)",
	"calc(var(--corner)-var(--menu-pad))", "calc(var(--corner-menu)-var(--menu-pad))",
	"999px", "50%", "0",
}

// manageText are the management page's steps of text; a heading of 1rem or more and an em of 1 or more are written as
// numbers.
var manageText = []string{"var(--text-label)", "var(--text-small)", "var(--text)", "inherit", "0"}

// manageEm is the one size the management page writes in em under 1, by the rule that writes it: a line's code, sized by
// the text round it.
var manageEm = map[string]string{".state code": "0.8em"}

// manageMixed are the colors mixed outside :root, by the rule and the property that write them: a project's pill mixes
// its item's color (--c, set on each item), which a variable in :root would read in :root.
var manageMixed = map[[2]string]string{
	{".proj", "color"}:      "color-mix(in srgb, var(--c) 55%, var(--fg))",
	{".proj", "background"}: "color-mix(in srgb, var(--c) 14%, var(--panel))",
}

// colorFunctions are the functions that make a color.
var colorFunctions = []string{
	"rgb(", "rgba(", "hsl(", "hsla(", "hwb(", "lab(", "lch(", "oklab(", "oklch(", "color(", "color-mix(", "light-dark(",
}

// svgColors are the SVG attributes that take a color.
var svgColors = []string{"fill", "stroke", "stop-color", "flood-color", "lighting-color", "color"}

// TestManageSheet keeps the management page's (ui.html's) corners and text to the steps in its :root, and its colors in
// :root alone: a value outside :root writes no color of its own (a # color or a color function), nor does an SVG
// attribute of its markup; the icon's data URI, a string, writes them out.
// LIMIT: a color's name (white) is not read.
func TestManageSheet(t *testing.T) {
	t.Parallel()

	src, err := os.ReadFile("../ui/ui.html")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := html.Parse(strings.NewReader(string(src)))
	if err != nil {
		t.Fatal(err)
	}
	var style string
	for n := range doc.Descendants() {
		if n.Type == html.ElementNode && n.Data == "style" && n.FirstChild != nil {
			style += n.FirstChild.Data
		}
		if n.Type != html.ElementNode {
			continue
		}
		for _, a := range n.Attr {
			if slices.Contains(svgColors, a.Key) && colored(lex([]byte(a.Val))) {
				t.Errorf("<%s %s=%q>: give the color a variable in :root and set it from the style", n.Data, a.Key, a.Val)
			}
		}
	}
	if style == "" {
		t.Fatal("ui.html has no style")
	}
	sheet := []byte(style)
	for _, d := range values(sheet) {
		name, v := d.name, groups(d.value)
		switch {
		case radiusProperty.MatchString(name):
			for _, c := range v {
				if c != "/" && !slices.Contains(manageCorners, c) {
					t.Errorf("%s: %s in %q; read one of :root's corners %v", name, text(sheet, d.value), d.rule, manageCorners)
				}
			}
		case name == "font-size" || name == "font":
			size := text(sheet, d.value)
			if name == "font" {
				size = fontSize(d.value)
			}
			if size == "" || !slices.Contains(manageText, size) && !large(size) && manageEm[d.rule] != size {
				t.Errorf("%s: %s in %q; read one of :root's steps of text %v", name, text(sheet, d.value), d.rule, manageText)
			}
		}
		preludes := strings.Split(d.rule, " { ")
		if preludes[len(preludes)-1] != ":root" && manageMixed[[2]string{d.rule, name}] != text(sheet, d.value) && colored(d.value) {
			t.Errorf("%s: %s in %q; give the color a variable in :root", name, text(sheet, d.value), d.rule)
		}
	}
}

// radiusProperty names the properties of corners.
var radiusProperty = regexp.MustCompile(`^border(-[a-z]+)*-radius$`)

// colored reports whether a value writes a color of its own: a # color or a color function.
func colored(value []token) bool {
	for _, t := range value {
		if t.tt == css.HashToken || t.tt == css.FunctionToken && slices.Contains(colorFunctions, strings.ToLower(string(t.data))) {
			return true
		}
	}
	return false
}

// namedDeclaration is a declaration and its property's name.
type namedDeclaration struct {
	declaration
	name string
}

// values returns every declaration of src, a property's name at the start of a block or after a ;, followed by a colon,
// and its value up to the ; or } that ends it; what reaches a { is a rule's prelude (li:has(…) {), not a declaration.
func values(src []byte) []namedDeclaration {
	toks := lex(src)
	in, blocks := rules(toks)
	var found []namedDeclaration
	start := true
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		switch t.tt {
		case css.WhitespaceToken, css.CommentToken:
			continue
		case css.LeftBraceToken, css.SemicolonToken, css.RightBraceToken:
			start = true
			continue
		default:
		}
		begins := start && (t.tt == css.IdentToken || t.tt == css.CustomPropertyNameToken)
		start = false
		if !begins {
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
		depth, prelude := 0, false
		for j++; j < len(toks); j++ {
			tt := toks[j].tt
			if depth == 0 && (tt == css.SemicolonToken || tt == css.RightBraceToken) {
				break
			}
			if depth == 0 && tt == css.LeftBraceToken {
				prelude = true
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
		if prelude {
			continue
		}
		found = append(found, namedDeclaration{
			declaration: declaration{value: value, rule: in[i], block: blocks[i]},
			name:        strings.ToLower(string(t.data)),
		})
		i = j - 1
	}
	return found
}
