package css

import (
	"maps"
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
	"calc(var(--corner) - var(--menu-pad))", "calc(var(--corner-menu) - var(--menu-pad))",
	"999px", "50%", "0",
}

// manageSteps are the steps :root sets, which no other rule sets again.
var manageSteps = []string{"--corner", "--corner-menu", "--corner-small", "--menu-pad", "--text-label", "--text-small", "--text"}

// manageText are the management page's steps of text; a heading of 1rem or more and an em of 1 or more are written as
// numbers.
var manageText = []string{"var(--text-label)", "var(--text-small)", "var(--text)", "inherit", "0"}

// manageEm is the one size the management page writes in em under 1, by the rule that writes it: a line's code, sized by
// the text round it.
var manageEm = map[string]string{".state code": "0.8em"}

// colorFunctions are the functions that make a color.
var colorFunctions = []string{
	"rgb(", "rgba(", "hsl(", "hsla(", "hwb(", "lab(", "lch(", "oklab(", "oklch(", "color(", "color-mix(", "light-dark(",
}

// svgColors are the SVG attributes that take a color.
var svgColors = []string{"fill", "stroke", "stop-color", "flood-color", "lighting-color", "color"}

// TestManageSheet keeps the management page's (ui.html's) corners and text to the steps in its :root, set nowhere else,
// and its colors in :root alone: a value outside :root writes no color of its own (a # color or a color function), in the
// style and in an element's style attribute, nor does an SVG attribute of its markup.
// LIMIT: a color's name (white) is not read, nor the HTML the script builds in its strings.
// LIMIT: corners are read in border-…-radius alone, not in -webkit-border-radius nor clip-path's inset(… round …).
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
		if n.Type != html.ElementNode {
			continue
		}
		if n.Data == "style" && n.FirstChild != nil {
			style += n.FirstChild.Data
		}
		for _, a := range n.Attr {
			if slices.Contains(svgColors, a.Key) && colored(lex([]byte(a.Val))) {
				t.Errorf("<%s %s=%q>: give the color a variable in :root and set it from the style", n.Data, a.Key, a.Val)
			}
			if a.Key == "style" {
				manageCheck(t, "<"+n.Data+" style>", []byte("style {"+a.Val+"}"))
			}
		}
	}
	if style == "" {
		t.Fatal("ui.html has no style")
	}
	manageCheck(t, "ui.html's style", []byte(style))
}

// manageCheck checks the declarations of sheet, told as in where.
func manageCheck(t *testing.T, where string, sheet []byte) {
	t.Helper()

	all := declarations(sheet, func(string) bool { return true })
	names := slices.Sorted(maps.Keys(all))
	for _, name := range names {
		for _, d := range all[name] {
			value := text(sheet, d.value)
			preludes := strings.Split(d.rule, " { ")
			inRoot := preludes[len(preludes)-1] == ":root"
			switch {
			case slices.Contains(manageSteps, name) && !inRoot:
				t.Errorf("%s: sets %s in %q; only :root sets the steps", where, name, d.rule)
			case radiusProperty.MatchString(name):
				for _, c := range items(d.value) {
					if c := text(sheet, c); c != "/" && !slices.Contains(manageCorners, c) {
						t.Errorf("%s: %s: %s in %q; read one of :root's corners %v", where, name, value, d.rule, manageCorners)
					}
				}
			case name == "font-size" || name == "font":
				size, ok := sizeOf(name, d.value)
				if !ok || !slices.Contains(manageText, size) && !large(size) && manageEm[d.rule] != size {
					t.Errorf("%s: %s: %s in %q; read one of :root's steps of text %v", where, name, value, d.rule, manageText)
				}
			default:
			}
			if !inRoot && colored(d.value) {
				t.Errorf("%s: %s: %s in %q; give the color a variable in :root", where, name, value, d.rule)
			}
		}
	}
}

// items splits a value into its items: a function with what it holds, or a token.
func items(value []token) [][]token {
	var out [][]token
	depth, start := 0, 0
	for i, t := range value {
		switch t.tt {
		case css.FunctionToken, css.LeftParenthesisToken:
			depth++
		case css.RightParenthesisToken:
			depth--
		default:
		}
		if depth == 0 {
			out = append(out, value[start:i+1])
			start = i + 1
		}
	}
	return out
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
