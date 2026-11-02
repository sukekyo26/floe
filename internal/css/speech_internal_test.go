package css

import (
	"slices"
	"strings"
	"testing"

	"github.com/tdewolff/parse/v2/css"
)

// speechAmiss returns what is amiss in the readings src gives what it draws with content: the last content of each
// block that holds one right (a rule's, in @media or nested too) is read after its /, by "" (a mark, not read) or by
// the counters before it alone (a number, read), and by no string, which passes no words.yaml.
// LIMIT: one's own parts, themes and templates are not read, nor the management page (ui.html), a counter style's
// prefix, suffix or symbols (handoff's Q, which takes no reading), a counter or text within var() after / (taken as
// no counter), nor ::marker's content, which floe's CSS does not write.
func speechAmiss(src []byte) []string {
	last := map[int]declaration{}
	var order []int
	for _, d := range declarations(src, func(p string) bool { return p == "content" })["content"] {
		if _, ok := last[d.block]; !ok {
			order = append(order, d.block)
		}
		last[d.block] = d
	}
	var amiss []string
	for _, block := range order {
		d := last[block]
		value := text(src, d.value)
		if value == `""` || value == "none" || value == "normal" {
			continue
		}
		before, after, ok := cutSlash(d.value)
		switch {
		case !ok:
			amiss = append(amiss, "content: "+value+" in "+d.rule+" is read out; write / \"\" after a mark, or the counters before / after a number")
		case len(after) == 1 && after[0].tt == css.StringToken && string(after[0].data) == `""`:
		default:
			counters := groups(before)
			for _, g := range groups(after) {
				if !strings.HasPrefix(g, "counter(") && !strings.HasPrefix(g, "counters(") || !slices.Contains(counters, g) {
					amiss = append(amiss, "content: "+value+" in "+d.rule+" reads "+g+"; read \"\" or the counters before / alone")
					break
				}
			}
		}
	}
	return amiss
}

// cutSlash splits a value at its / outside every bracket.
func cutSlash(value []token) (before, after []token, ok bool) {
	depth := 0
	for i, t := range value {
		switch t.tt {
		case css.FunctionToken, css.LeftParenthesisToken:
			depth++
		case css.RightParenthesisToken:
			depth--
		case css.DelimToken:
			if depth == 0 && string(t.data) == "/" {
				return value[:i], value[i+1:], true
			}
		default:
		}
	}
	return value, nil, false
}

// groups returns the items of a value as text: a function with what it holds, or a token.
func groups(value []token) []string {
	var out []string
	var b strings.Builder
	depth := 0
	for _, t := range value {
		b.Write(t.data)
		switch t.tt {
		case css.FunctionToken, css.LeftParenthesisToken:
			depth++
		case css.RightParenthesisToken:
			depth--
		default:
		}
		if depth == 0 {
			out = append(out, b.String())
			b.Reset()
		}
	}
	return out
}

// TestMarkSpeech keeps what floe's CSS draws with content out of a screen reader's way: a mark is not read, a number
// is, and no word of CSS is; what a mark alone means is told in the HTML by hidden words.
func TestMarkSpeech(t *testing.T) {
	t.Parallel()

	sheets(t, "../kit/floe", func(name string, src []byte) {
		for _, why := range speechAmiss(src) {
			t.Errorf("%s: %s", name, why)
		}
	})
}

// TestSpeechAmiss runs the check on forms floe's CSS does not write today.
func TestSpeechAmiss(t *testing.T) {
	t.Parallel()

	for src, want := range map[string]int{
		`li::before { content: "✓"; content: "✓" / ""; }`:                                         0,
		`li::before { content: counter(a); content: counter(a) / counter(a); }`:                   0,
		`li::after { content: counter(a) " / " counter(b) / counter(a); }`:                        0,
		`li::after { content: counter(a) " / " counter(b) / ""; }`:                                0,
		`li::before { content: ""; } li::after { content: none; }`:                                0,
		`li::before { content: "✓"; }`:                                                            1,
		`li::before { content: "✓" / "✓"; }`:                                                      1,
		`li::before { content: "✓" / "done"; }`:                                                   1,
		`li::before { content: "Step " counter(x) / "Step " counter(x); }`:                        1,
		`li::after { content: counter(a) " / " counter(b) / counter(a) " / " counter(b); }`:       1,
		`li::before { content: counter(a) / counter(b); }`:                                        1,
		`li::before { content: "✓" / ""; } li::before { content: "✓"; }`:                          1,
		`@media print { li::before { content: "✓"; } li.x::after { content: "+" / ""; } }`:        1,
		`.a { content: "✓"; &::after { content: "+" / ""; } }`:                                    1,
		`li::before { content: "✓" / ""; content: "✓"; }`:                                         1,
	} {
		if got := speechAmiss([]byte(src)); len(got) != want {
			t.Errorf("%s: %d amiss %q, want %d", src, len(got), got, want)
		}
	}
}
