package lint

import (
	"slices"
	"strings"
	"unicode"

	"github.com/tdewolff/parse/v2/css"
)

// faces are the declarations whose strings may hold letters in a theme: the names of typefaces, which no page shows.
// LIMIT: a theme reading one of them where text shows (content: var(--display)) writes a word unseen; read them in a
// typeface's place alone.
var faces = []string{"font-family", "font", "--font", "--display", "--mono"}

// counted are the properties that name a counter or list style.
var counted = []string{"list-style", "list-style-type"}

// listWords are the words a theme may write where a counter or list style is named: digits and shapes, the same in every
// language, the places of a list's marker, and CSS's own keywords.
var listWords = []string{
	"decimal", "decimal-leading-zero", "none", "disc", "circle", "square",
	"inside", "outside", "inherit", "initial", "unset", "revert", "revert-layer",
}

// moves are the transform functions a theme may write, which move alone, never turn, skew nor scale.
var moves = []string{"translate", "translatex", "translatey"}

// transforms are the properties of a transform; unturned are those a theme writes not at all (they turn, scale or move
// along a path).
var (
	transforms = []string{"transform", "-webkit-transform", "-moz-transform"}
	unturned   = []string{"rotate", "scale", "zoom", "offset-path", "offset-rotate", "perspective"}
)

// turnings are the functions a variable may not hold, as a transform may read it: they turn, skew, scale or stand in for
// those.
var turnings = []string{"rotate", "skew", "scale", "matrix", "perspective"}

// global are CSS's own keywords, which any property takes.
var global = []string{"inherit", "initial", "unset", "revert", "revert-layer", "none"}

func foldIn(list []string, s string) bool {
	return slices.ContainsFunc(list, func(w string) bool { return strings.EqualFold(w, s) })
}

// themeMarks keeps a theme's decorations out of words and tilts, by what a theme's CSS may write alone: strings of marks
// (letters only in a typeface's name), counters of digits, and transforms that move.
// LIMIT: a string in a url() (an SVG of text in data:) and list-style-image are not read; floe's themes write neither.
var themeMarks = Rule{Name: "theme-marks", Check: func(in *Input) []Finding {
	var found []Finding
	for _, s := range in.Sheets() {
		if s.Kind != kindTheme {
			continue
		}
		in.Saw()
		named := map[int]bool{} // where the strings of typefaces' names start
		for prop, ds := range declarations(s.Src, func(string) bool { return true }) {
			for _, d := range ds {
				for _, why := range declAmiss(prop, d.value) {
					file, line := s.at(d.at)
					found = append(found, Finding{file, line, "theme-marks", prop + ": " + text(s.Src, d.value) + " " + why})
				}
				if slices.Contains(faces, prop) {
					for _, t := range d.value {
						named[t.At] = true
					}
				}
			}
		}
		for _, t := range tokensOf(s.Src) {
			var why string
			switch {
			case t.Type == css.AtKeywordToken && strings.EqualFold(string(t.Data), "@counter-style"):
				why = "@counter-style counts in a style of its own; count in digits"
			case t.Type == css.StringToken && strings.Contains(string(t.Data), `\`):
				why = string(t.Data) + " writes an escape; write the character"
			case t.Type == css.StringToken && !named[t.At] && strings.ContainsFunc(string(t.Data), unicode.IsLetter):
				why = string(t.Data) + " writes a word; write a mark"
			}
			if why != "" {
				file, line := s.at(t.At)
				found = append(found, Finding{file, line, "theme-marks", why})
			}
		}
	}
	return found
}}

// declAmiss returns what a theme's declaration of prop writes amiss, by the forms a theme may write alone, a variable's
// value as any other's (it may be read anywhere): a counter or list style of digits or shapes and no variable, no
// attribute's text, a transform that moves alone, and no variable that turns.
func declAmiss(prop string, value []token) []string {
	var amiss []string
	switch {
	case slices.Contains(unturned, prop):
		return []string{"turns, scales or moves along a path; a theme writes it not"}
	case slices.Contains(transforms, prop):
		for _, t := range value {
			f := strings.ToLower(strings.TrimSuffix(string(t.Data), "("))
			if t.Type == css.FunctionToken && !slices.Contains(moves, f) || t.Type == css.IdentToken && !foldIn(global, f) {
				amiss = append(amiss, "turns, skews or scales; a theme moves alone")
			}
		}
		return amiss
	case strings.HasPrefix(prop, "--"):
		for _, t := range value {
			f := strings.ToLower(string(t.Data))
			if t.Type == css.FunctionToken && slices.ContainsFunc(turnings, func(w string) bool { return strings.HasPrefix(f, w) }) {
				amiss = append(amiss, "holds a turn for a transform; a theme moves alone")
			}
		}
	}
	for i, t := range value {
		data := string(t.Data)
		switch t.Type {
		case css.FunctionToken:
			f := strings.ToLower(strings.TrimSuffix(data, "("))
			if f == "var" && (inCounter(value[:i], t.Depth) || slices.Contains(counted, prop)) {
				amiss = append(amiss, "counts by a variable; count in digits")
			}
			if f == "attr" {
				amiss = append(amiss, "shows an attribute's text; write a mark")
			}
		case css.IdentToken:
			style := inCounter(value[:i], t.Depth) && afterComma(value[:i], t.Depth) || !inCounter(value[:i], t.Depth) && slices.Contains(counted, prop)
			if style && !foldIn(listWords, data) {
				amiss = append(amiss, "counts in "+data+"; count in digits")
			}
		default:
		}
	}
	return amiss
}

// inCounter is whether a token of depth after before is right in a counter() or counters().
func inCounter(before []token, depth int) bool {
	f := strings.ToLower(around(before, depth))
	return f == "counter(" || f == "counters("
}

// afterComma is whether a token of depth after before stands after a comma of its function: a counter's style, not its
// name.
func afterComma(before []token, depth int) bool {
	for i := len(before) - 1; i >= 0 && before[i].Depth >= depth; i-- {
		if before[i].Depth == depth && before[i].Type == css.CommaToken {
			return true
		}
	}
	return false
}
