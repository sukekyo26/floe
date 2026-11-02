package css

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

// textSteps are base.css's steps of parts' small text, smallest first.
var textSteps = []string{"var(--floe-text-small)", "var(--floe-text-aside)"}

// codeText is the text of code, which follows code's size rather than a step (base.css's pre and a line's code): a
// file under parts, the rule it writes the size in, and the size.
var codeText = map[[2]string]string{
	{"code/part.css", ":scope pre"}:           "0.875rem",
	{"tree/part.css", ":scope .name"}:         "0.86rem",
	{"entries/part.css", ":scope .tags code"}: "0.86rem",
}

// TestTextScale keeps the size of parts' and looks' text to base.css's steps of small text (--floe-text-small,
// --floe-text-aside), inherit, or a rem or an em of 1 or more: but the text of code (codeText), which follows code's
// size, and a diagram's in px, measured in its own units. A theme sets no step; its own sizes are its own.
// LIMIT: the templates' page.css and one's own parts are not read.
func TestTextScale(t *testing.T) {
	t.Parallel()

	sheets(t, "../kit/floe/parts", func(name string, src []byte) {
		for _, d := range declarations(src, func(p string) bool { return p == "font-size" })["font-size"] {
			v := text(src, d.value)
			if slices.Contains(textSteps, v) || v == "inherit" || codeText[[2]string{name, d.rule}] == v ||
				strings.HasPrefix(name, "diagram/") && strings.HasSuffix(v, "px") && !strings.Contains(v, "(") || large(v) {
				continue
			}
			t.Errorf("%s: font-size: %s in %q; write a part's small text by one of base.css's steps %v", name, v, d.rule, textSteps)
		}
	})
	sheets(t, "../kit/floe/themes", func(name string, src []byte) {
		for p := range declarations(src, func(p string) bool { return strings.HasPrefix(p, "--floe-text-") }) {
			t.Errorf("%s: sets %s; a theme keeps base.css's steps of small text, and writes a size of its own as it is", name, p)
		}
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
