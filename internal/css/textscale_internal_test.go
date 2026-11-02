package css

import (
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
		sizes := declarations(src, func(p string) bool { return p == "font-size" || p == "font" })
		for _, d := range sizes["font-size"] {
			if v := text(src, d.value); !sized(name, d.rule, v) {
				t.Errorf("%s: font-size: %s in %q; write a part's small text by one of base.css's steps %v", name, v, d.rule, textSteps)
			}
		}
		for _, d := range sizes["font"] {
			if v := fontSize(d.value); !sized(name, d.rule, v) {
				t.Errorf("%s: font: %s in %q has the size %q; write a part's small text by one of base.css's steps %v",
					name, text(src, d.value), d.rule, v, textSteps)
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

// sized reports whether v is a size a part may write in rule of the file name.
func sized(name, rule, v string) bool {
	return slices.Contains(textSteps, v) || v == "inherit" || codeText[[2]string{name, rule}] == v ||
		name == "diagram/part.css" && strings.HasPrefix(rule, ".dg ") && strings.HasSuffix(v, "px") && !strings.Contains(v, "(") ||
		large(v)
}

// fontSize returns the size a font shorthand writes: the item before its / outside every bracket, or else its first
// length, percentage or function; inherit is inherit, and "" when there is none.
func fontSize(value []token) string {
	if before, _, ok := cutSlash(value); ok {
		items := groups(before)
		if len(items) == 0 {
			return ""
		}
		return items[len(items)-1]
	}
	if items := groups(value); len(items) == 1 && strings.EqualFold(items[0], "inherit") {
		return "inherit"
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
				return strings.Join(groups(value[start:i+1]), "")
			}
		case css.DimensionToken, css.PercentageToken:
			if depth == 0 {
				return string(t.data)
			}
		default:
		}
	}
	return ""
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
