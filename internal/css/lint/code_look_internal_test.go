package lint

import (
	"slices"
	"strings"

	"github.com/tdewolff/parse/v2/css"
)

// codeLook keeps a part that takes code's look (its looks' looks too) to what code sets: it writes no length of padding
// (code's --floe-code-pad and --floe-code-room hold the room), no share of a colour (--floe-code-faint and
// --floe-code-head-faint hold the faint ones) and does not name the copy button (code sets the room for it).
// LIMIT: a length set in a variable of the part's own and read in padding, and a fade written otherwise than a share
// (rgb(… / .5)), pass; floe's parts write neither.
var codeLook = Rule{Name: "code-look", Check: func(in *Input) []Finding {
	var found []Finding
	for _, s := range in.Sheets() {
		if s.Kind != kindPart || !slices.Contains(in.Taken(s.Owner), "code") {
			continue
		}
		in.Saw()
		for prop, ds := range declarations(s.Src, func(p string) bool { return strings.HasPrefix(p, "padding") || p == "color" }) {
			for _, d := range ds {
				for _, t := range d.value {
					if prop == "color" && t.Type == css.PercentageToken || prop != "color" && t.Type == css.DimensionToken {
						file, line := s.at(d.at)
						found = append(found, Finding{
							file, line, "code-look",
							prop + ": " + text(s.Src, d.value) + " in " + d.rule + "; read code's --floe-code-* rather than copy its numbers",
						})
						break
					}
				}
			}
		}
		toks := tokensOf(s.Src)
		for i, t := range toks {
			if t.Type == css.IdentToken && string(t.Data) == "floe-copy" && i > 0 && toks[i-1].Type == css.DelimToken && string(toks[i-1].Data) == "." {
				file, line := s.at(t.At)
				found = append(found, Finding{file, line, "code-look", "names .floe-copy; code sets the room for the copy button in --floe-code-room"})
			}
		}
	}
	return found
}}
