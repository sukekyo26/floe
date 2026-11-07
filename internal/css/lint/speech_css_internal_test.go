package lint

import (
	"slices"
	"strings"

	"github.com/tdewolff/parse/v2/css"
)

// speechCSS keeps what floe's CSS draws with content out of a screen reader's way: the last content of each block that
// holds one right (a rule's, in @media or nested too) is read after its /, by "" (a mark, or a number that words do not
// tell from what is around it, not read) or by all the counters before it alone, in their order (a number, read), and by
// no string, which passes no words.yaml.
// LIMIT: a counter style's prefix, suffix or symbols, a counter or text within var() after / (taken as no counter), and
// ::marker's content, which floe's CSS does not write, are not read.
var speechCSS = Rule{Name: "speech-css", Check: func(in *Input) []Finding {
	var found []Finding
	for _, s := range in.Sheets() {
		in.Saw()
		last := map[int]decl{}
		var order []int
		for _, d := range declarations(s.Src, func(p string) bool { return p == "content" })["content"] {
			if _, ok := last[d.block]; !ok {
				order = append(order, d.block)
			}
			last[d.block] = d
		}
		for _, block := range order {
			d := last[block]
			value := text(s.Src, d.value)
			if value == `""` || value == "none" || value == "normal" {
				continue
			}
			before, after, ok := cutSlash(d.value)
			var why string
			switch {
			case !ok:
				why = "content: " + value + " in " + d.rule + " is read out; write / \"\" after a mark, or the counters before / after a number"
			case len(after) == 1 && after[0].Type == css.StringToken && string(after[0].Data) == `""`:
			case !slices.Equal(joined(after), counters(before)):
				why = "content: " + value + " in " + d.rule + " reads " + text(s.Src, after) + "; read \"\" or the counters before / alone, all of them once"
			}
			if why != "" {
				file, line := s.at(d.at)
				found = append(found, Finding{file, line, "speech-css", why})
			}
		}
	}
	return found
}}

// counters returns the counter() and counters() of a value, in order, as text.
func counters(value []token) []string {
	var out []string
	for _, g := range joined(value) {
		if strings.HasPrefix(g, "counter(") || strings.HasPrefix(g, "counters(") {
			out = append(out, g)
		}
	}
	return out
}
