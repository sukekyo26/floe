package css

import (
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/tdewolff/parse/v2/css"
)

// faces are the declarations whose strings may hold letters in a theme: the names of typefaces, which no page shows.
// LIMIT: a theme reading one of them where text shows (content: var(--display)) writes a word unseen; read them in
// a typeface's place alone.
var faces = []string{"font-family", "font", "--font", "--display", "--mono"}

// counted are the properties that name a counter or list style.
var counted = []string{"list-style", "list-style-type"}

// listWords are the words a theme may write where a counter or list style is named: digits and shapes, the same in
// every language, the places of a list's marker, CSS's own keywords and !important.
var listWords = []string{
	"decimal", "decimal-leading-zero", "none", "disc", "circle", "square",
	"inside", "outside", "inherit", "initial", "unset", "revert", "revert-layer", "important",
}

// moves are the transform functions a theme may write, which move alone, never turn, skew nor scale.
var moves = []string{"translate", "translatex", "translatey"}

// transforms are the properties of a transform; unturned are those a theme writes not at all (they turn, scale or
// move along a path).
var (
	transforms = []string{"transform", "-webkit-transform", "-moz-transform"}
	unturned   = []string{"rotate", "scale", "zoom", "offset-path", "offset-rotate", "perspective"}
)

// turnings are the functions a variable may not hold, as a transform may read it: they turn, skew, scale or stand
// in for those.
var turnings = []string{"rotate", "skew", "scale", "matrix", "perspective"}

// global are CSS's own keywords, which any property takes.
var global = []string{"inherit", "initial", "unset", "revert", "revert-layer", "none", "important"}

func foldIn(list []string, s string) bool {
	return slices.ContainsFunc(list, func(w string) bool { return strings.EqualFold(w, s) })
}

// declAmiss returns what a theme's declaration of prop writes amiss, by the forms a theme may write alone: a counter
// or list style of digits or shapes and no variable, a transform that moves alone, and no variable that turns.
func declAmiss(prop string, value []token) []string {
	var amiss []string
	switch {
	case slices.Contains(unturned, prop):
		return []string{"turns, scales or moves along a path; a theme writes it not"}
	case slices.Contains(transforms, prop):
		for _, tok := range value {
			f := strings.ToLower(strings.TrimSuffix(string(tok.data), "("))
			if (tok.tt == css.FunctionToken && !slices.Contains(moves, f)) || (tok.tt == css.IdentToken && !foldIn(global, f)) {
				amiss = append(amiss, "turns, skews or scales; a theme moves alone")
			}
		}
		return amiss
	case strings.HasPrefix(prop, "--"):
		for _, tok := range value {
			f := strings.ToLower(string(tok.data))
			if tok.tt == css.FunctionToken && slices.ContainsFunc(turnings, func(t string) bool { return strings.HasPrefix(f, t) }) {
				amiss = append(amiss, "holds a turn for a transform; a theme moves alone")
			}
		}
		return amiss
	default:
	}
	counter, commas := false, 0
	for _, tok := range value {
		data := string(tok.data)
		switch tok.tt {
		case css.FunctionToken:
			f := strings.ToLower(strings.TrimSuffix(data, "("))
			if f == "counter" || f == "counters" {
				counter, commas = true, 0
			}
			if f == "var" && (counter || slices.Contains(counted, prop)) {
				amiss = append(amiss, "counts by a variable; count in digits")
			}
			if f == "attr" {
				amiss = append(amiss, "shows an attribute's text; write a mark")
			}
		case css.CommaToken:
			commas++
		case css.RightParenthesisToken:
			counter = false
		case css.IdentToken:
			style := (counter && commas > 0) || (!counter && slices.Contains(counted, prop))
			if style && !foldIn(listWords, data) {
				amiss = append(amiss, "counts in "+data+"; count in digits")
			}
		default:
		}
	}
	return amiss
}

// sheetAmiss returns what a theme's CSS writes amiss: a string with a letter (a word of a language the page may not
// be in) but in a typeface's name, one with an escape, which only the browser reads (write the character), a counter
// style of its own, and what declAmiss finds.
func sheetAmiss(src []byte) []string {
	var amiss []string
	named := map[int]bool{} // where the strings of typefaces' names start
	for prop, decls := range declarations(src, func(string) bool { return true }) {
		for _, d := range decls {
			for _, why := range declAmiss(prop, d.value) {
				amiss = append(amiss, prop+": "+text(src, d.value)+" "+why)
			}
			if slices.Contains(faces, prop) {
				for _, tok := range d.value {
					named[tok.at] = true
				}
			}
		}
	}
	for _, tok := range lex(src) {
		switch {
		case tok.tt == css.AtKeywordToken && strings.EqualFold(string(tok.data), "@counter-style"):
			amiss = append(amiss, "@counter-style counts in a style of its own; count in digits")
		case tok.tt == css.StringToken && strings.Contains(string(tok.data), `\`):
			amiss = append(amiss, string(tok.data)+" writes an escape; write the character")
		case tok.tt == css.StringToken && !named[tok.at] && strings.ContainsFunc(string(tok.data), unicode.IsLetter):
			amiss = append(amiss, string(tok.data)+" writes a word; write a mark")
		default:
		}
	}
	return amiss
}

// TestThemeMarks keeps a theme's decorations out of words and tilts, by what a theme's CSS may write alone: strings
// of marks (letters only in a typeface's name), counters of digits, and transforms that move.
func TestThemeMarks(t *testing.T) {
	t.Parallel()

	themes := os.DirFS("../kit/floe/themes")
	read := 0
	err := fs.WalkDir(themes, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(name) != ".css" {
			return err
		}
		src, err := fs.ReadFile(themes, name)
		if err != nil {
			return err
		}
		read++
		for _, why := range sheetAmiss(src) {
			t.Errorf("%s: %s", name, why)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if read == 0 {
		t.Fatal("no CSS found in floe's themes")
	}
}

// TestSheetAmiss runs the check on forms floe's themes do not write today, one at a time.
func TestSheetAmiss(t *testing.T) {
	t.Parallel()

	for src, want := range map[string]bool{
		`content: "“"`: false, `content: "●" / ""`: false, `content: counter(c, Decimal)`: false,
		`content: counters(c, ".", decimal-leading-zero)`: false, `transform: translate(2px, 50%)`: false,
		`transform: none`: false, `list-style: inside square`: false, `list-style: none !important`: false,
		`--font: "Hiragino Sans", sans-serif`: false, `font-family: "Yu Mincho"`: false, `--squares: linear-gradient(red, blue)`: false,
		`content: "QUOTE"`: true, `content: "\201C"`: true, `content: attr(data-floe-tone)`: true,
		`quotes: "QUOTE" ""`: true, `list-style-type: "Q"`: true, `list-style-type: cjk-decimal`: true,
		`text-emphasis: "x"`: true, `hyphenate-character: "x"`: true, `text-overflow: "more"`: true, `--label: "Fig."`: true,
		`content: counter(c, cjk-decimal)`: true, `content: counters(c, ".", lower-roman)`: true,
		`content: counter(c, var(--style))`: true, `list-style-type: var(--style)`: true,
		`transform: rotate(-2deg)`: true, `-webkit-transform: skewX(4deg)`: true, `-moz-transform: scale(1.1)`: true,
		`transform: matrix(1, 0, 0, 1, 2, 2)`: true, `transform: var(--move)`: true, `transform: translateZ(2px)`: true,
		`rotate: 2deg`: true, `rotate: none`: true, `scale: 1.1`: true, `zoom: 1.1`: true, `offset-path: path("M0 0")`: true,
		`offset-rotate: 10deg`: true, `perspective: 100px`: true, `--tilt: rotate(-2deg)`: true, `--flat: perspective(9px)`: true,
	} {
		if amiss := sheetAmiss([]byte(":scope { " + src + "; }")); (len(amiss) > 0) != want {
			t.Errorf("%s: amiss %q, want amiss %v", src, amiss, want)
		}
	}
	if amiss := sheetAmiss([]byte("@counter-style kanji { system: fixed; symbols: '一'; }")); len(amiss) == 0 {
		t.Error("@counter-style passes")
	}
}
