package css

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/tdewolff/parse/v2/css"
)

// textSteps are base.css's steps of parts' small text, smallest first.
var textSteps = []string{"var(--floe-text-small)", "var(--floe-text-aside)"}

// codeText is the text of code, which follows code's size rather than a step: a file under parts, the rule it writes
// the size in, and the size (code's lines, a tree's names, the code in an entry's tags).
var codeText = map[[2]string]string{
	{"code/part.css", ":scope pre"}:           "0.875rem",
	{"tree/part.css", ":scope .name"}:         "0.86rem",
	{"entries/part.css", ":scope .tags code"}: "0.86rem",
}

// TestTextScale keeps the size of parts' and looks' text, in font-size and in font, to base.css's steps of small text
// (--floe-text-small, --floe-text-aside), inherit, or a rem or an em of 1 or more: but the text of code (codeText),
// which follows code's size, and the text in a diagram's SVG (.dg) in px, measured in the SVG's own units. Only
// base.css sets a step.
// LIMIT: the templates' page.css and one's own parts are not read for sizes.
func TestTextScale(t *testing.T) {
	t.Parallel()

	sheets(t, "../kit/floe/parts", func(name string, src []byte) {
		for prop, decls := range declarations(src, func(p string) bool { return p == "font-size" || p == "font" }) {
			for _, d := range decls {
				if why := sizeAmiss(src, name, d.rule, prop, d.value); why != "" {
					t.Errorf("%s: %s: %s in %q %s; write a part's small text by one of base.css's steps %v",
						name, prop, text(src, d.value), d.rule, why, textSteps)
				}
			}
		}
	})
	sheets(t, "../kit/floe", func(name string, src []byte) {
		if name == "base.css" {
			return
		}
		for p := range declarations(src, func(p string) bool { return strings.HasPrefix(p, "--floe-text-") }) {
			t.Errorf("%s: sets %s; only base.css sets the steps of small text, and a theme writes a size of its own as it is", name, p)
		}
	})
}

// sizeAmiss returns why the size a declaration of prop writes in rule of the file name is not one a part may write,
// or "" when it is.
func sizeAmiss(src []byte, name, rule, prop string, value []token) string {
	v, ok := sizeOf(src, prop, value)
	switch {
	case !ok:
		return "writes no size"
	case !sized(name, rule, v):
		return fmt.Sprintf("has the size %q", v)
	default:
		return ""
	}
}

// sized reports whether v is a size a part may write in rule of the file name.
func sized(name, rule, v string) bool {
	return slices.Contains(textSteps, v) || v == "inherit" || codeText[[2]string{name, rule}] == v ||
		name == "diagram/part.css" && strings.HasPrefix(rule, ".dg ") && strings.HasSuffix(v, "px") && !strings.Contains(v, "(") ||
		large(v)
}

// sizeOf returns the size a declaration of prop (font-size or font) writes, and whether it writes one: inherit, in
// any case, as inherit; font-size's value as it is, none when empty; a font shorthand's item before its / outside every
// bracket, or else its first length, percentage or function, past an angle (oblique's), none when there is none (a
// system font, a keyword's size, CSS's own keywords but inherit, a number alone). Every check of a size reads it so,
// and a value that writes no size passes none.
func sizeOf(src []byte, prop string, value []token) (string, bool) {
	if items := groups(value); len(items) == 1 && strings.EqualFold(items[0], "inherit") {
		return "inherit", true
	}
	if prop == "font-size" {
		return text(src, value), len(value) > 0
	}
	if before, _, ok := cutSlash(value); ok {
		items := groups(before)
		if len(items) == 0 {
			return "", false
		}
		return items[len(items)-1], true
	}
	depth := 0
	start := -1
	for i, t := range value {
		switch t.tt {
		case css.FunctionToken, css.LeftParenthesisToken:
			if depth == 0 {
				start = i
			}
			depth++
		case css.RightParenthesisToken:
			depth--
			if depth == 0 && start >= 0 {
				return strings.Join(groups(value[start:i+1]), ""), true
			}
		case css.DimensionToken, css.PercentageToken:
			if depth == 0 && !angle(string(t.data)) {
				return string(t.data), true
			}
		default:
		}
	}
	return "", false
}

// angleUnits are CSS Values' units of an angle, which a font shorthand writes after oblique.
var angleUnits = []string{"deg", "grad", "rad", "turn"}

// angle reports whether the dimension v is in a unit of an angle.
func angle(v string) bool {
	return slices.ContainsFunc(angleUnits, func(u string) bool {
		return len(v) > len(u) && strings.EqualFold(v[len(v)-len(u):], u)
	})
}

// large reports whether v is a rem or an em of 1 or more, written as a number alone.
func large(v string) bool {
	for _, unit := range []string{"rem", "em"} {
		if n, ok := strings.CutSuffix(v, unit); ok {
			f, err := strconv.ParseFloat(n, 64)
			return err == nil && f >= 1
		}
	}
	return false
}

// TestSizeOf reads the size of forms floe's CSS does not write today: none from a value that writes no size, an
// angle passed over, inherit in any case.
func TestSizeOf(t *testing.T) {
	t.Parallel()

	for decl, want := range map[string]string{
		"font-size: ": "", "font-size: /**/": "", "font: caption": "", "font: bold small sans-serif": "",
		"font: initial": "", "font: unset": "", "font: revert": "", "font: revert-layer": "", "font: 0 serif": "",
		"font: 700 var(--floe-text-small)/1.4 sans-serif": "var(--floe-text-small)", "font: oblique 10deg 0.8rem serif": "0.8rem",
		"font: oblique 1.5rad var(--floe-text-aside) serif": "var(--floe-text-aside)", "font: INHERIT": "inherit",
		"font-size: INHERIT": "inherit", "font: 1.1rem serif": "1.1rem", "font-size: small": "small",
		"font-size: initial": "initial", "font: bold small/1.4 serif": "small",
	} {
		src := []byte(":scope { " + decl + "; }")
		prop, _, _ := strings.Cut(decl, ":")
		ds := declarations(src, func(p string) bool { return p == prop })[prop]
		if len(ds) != 1 {
			t.Fatalf("%s: %d declarations", decl, len(ds))
		}
		got, ok := sizeOf(src, prop, ds[0].value)
		if got != want || ok != (want != "") {
			t.Errorf("sizeOf(%s) = %q, %v; want %q, %v", decl, got, ok, want, want != "")
		}
	}
}

// TestSized checks what TestTextScale says of forms floe's parts do not write today, in a rule codeText names (code's
// :scope pre) and in one it does not.
func TestSized(t *testing.T) {
	t.Parallel()

	refused := []string{
		"font-size: ", "font-size: /**/", "font: caption", "font: bold small sans-serif", "font: initial", "font: unset",
		"font: revert", "font: revert-layer", "font: 0 serif", "font: 0.7rem sans-serif", "font-size: small",
		"font-size: initial", "font: bold small/1.4 serif",
	}
	passed := []string{
		"font: 700 var(--floe-text-small)/1.4 sans-serif", "font: inherit", "font: 1.1rem serif",
		"font: oblique 10deg var(--floe-text-small) serif", "font-size: INHERIT",
	}
	check := func(rule, decl string, want bool) {
		t.Helper()

		src := []byte(rule + " { " + decl + "; }")
		prop, _, _ := strings.Cut(decl, ":")
		for _, d := range declarations(src, func(p string) bool { return p == prop })[prop] {
			if why := sizeAmiss(src, "code/part.css", d.rule, prop, d.value); (why == "") != want {
				t.Errorf("%s { %s }: %q, want passed %v", rule, decl, why, want)
			}
		}
	}
	for _, rule := range []string{":scope pre", ":scope .lines"} {
		for _, decl := range refused {
			check(rule, decl, false)
		}
		for _, decl := range passed {
			check(rule, decl, true)
		}
	}
	check(":scope pre", "font: 0.875rem monospace", true)
	check(":scope .lines", "font: 0.875rem monospace", false)
}
