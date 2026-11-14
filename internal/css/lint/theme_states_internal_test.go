package lint

import (
	"strings"

	"github.com/tdewolff/parse/v2/css"
)

// themeStates keeps a theme's CSS out of states and print: a part paints its states by the variables a theme sets
// (--tabs-picked-bg), on screen alone, and every tab shows on paper.
var themeStates = Rule{Name: "theme-states", Check: func(in *Input) []Finding {
	var found []Finding
	for _, s := range in.Sheets() {
		if s.Kind != kindTheme {
			continue
		}
		in.Saw()
		toks := tokensOf(s.Src)
		for i, t := range toks {
			var why string
			switch {
			case t.Type == css.ColonToken && i+1 < len(toks) && toks[i+1].Type == css.IdentToken &&
				strings.EqualFold(string(toks[i+1].Data), "checked") && inPrelude(toks[i+1:], t.Depth):
				why = ":checked paints a state; set the part's variable for it (--tabs-picked-bg)"
			case t.Type == css.AtKeywordToken && strings.EqualFold(string(t.Data), "@media") && printing(toks[i+1:]):
				why = "@media print styles paper; a theme styles the screen and paper alike"
			}
			if why != "" {
				file, line := s.at(t.At)
				found = append(found, Finding{file, line, "theme-states", why})
			}
		}
	}
	return found
}}

// inPrelude is whether the tokens after a token of depth stand in a rule's prelude, a selector: of the tokens no deeper
// than it (out of :has( too), a { comes before a ; or } that ends a declaration's value (--x:checked;).
func inPrelude(toks []token, depth int) bool {
	for _, t := range toks {
		switch {
		case t.Depth > depth:
		case t.Type == css.LeftBraceToken:
			return true
		case t.Type == css.SemicolonToken || t.Type == css.RightBraceToken:
			return false
		}
	}
	return false
}

// printing is whether the media query that toks start with, up to its {, names print.
func printing(toks []token) bool {
	for _, t := range toks {
		if t.Type == css.LeftBraceToken || t.Type == css.SemicolonToken {
			return false
		}
		if t.Type == css.IdentToken && strings.EqualFold(string(t.Data), "print") {
			return true
		}
	}
	return false
}
