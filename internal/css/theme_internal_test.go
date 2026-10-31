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

// listStyles are the counter and list styles a theme may write: digits and shapes, the same in every language.
var listStyles = []string{"decimal", "decimal-leading-zero", "none", "disc", "circle", "square"}

// texted are the properties whose strings a page shows as text.
var texted = []string{"content", "quotes", "list-style", "list-style-type"}

// turns are the transform functions that turn, skew or scale. matrix() is among them, whatever it holds: reading
// whether one only moves would be reading the value's meaning.
var turns = []string{"rotate", "skew", "scale", "matrix"}

// marksAmiss returns what a theme's declaration of prop writes amiss: a string with a letter (a word of a language
// the page may not be in) or an escape (which only the browser reads), attr() (the markup's text), a counter or list
// style that is no digit or shape, or a transform that turns, skews or scales. A translate only moves.
func marksAmiss(prop string, value []token) []string {
	var amiss []string
	if prop == "rotate" || prop == "scale" {
		if len(value) != 1 || !strings.EqualFold(string(value[0].data), "none") {
			amiss = append(amiss, "turns or scales; a theme draws upright")
		}
		return amiss
	}
	counter, commas := false, 0
	for _, tok := range value {
		data := string(tok.data)
		switch tok.tt {
		case css.StringToken:
			if slices.Contains(texted, prop) && strings.ContainsFunc(data, unicode.IsLetter) {
				amiss = append(amiss, "writes a word; write a mark")
			}
			if slices.Contains(texted, prop) && strings.Contains(data, `\`) {
				amiss = append(amiss, "writes an escape; write the character")
			}
		case css.FunctionToken:
			f := strings.ToLower(strings.TrimSuffix(data, "("))
			counter, commas = f == "counter" || f == "counters", 0
			if f == "attr" && slices.Contains(texted, prop) {
				amiss = append(amiss, "shows an attribute's text; write a mark")
			}
			if slices.ContainsFunc(turns, func(turn string) bool { return strings.HasPrefix(f, turn) }) {
				amiss = append(amiss, "turns, skews or scales; a theme draws upright")
			}
		case css.CommaToken:
			commas++
		case css.RightParenthesisToken:
			counter = false
		case css.IdentToken:
			style := (counter && commas > 0) || (!counter && (prop == "list-style" || prop == "list-style-type"))
			if style && !slices.ContainsFunc(listStyles, func(s string) bool { return strings.EqualFold(s, data) }) {
				amiss = append(amiss, "counts in "+data+"; count in digits")
			}
		default:
		}
	}
	return amiss
}

// TestThemeMarks keeps a theme's decorations out of words and tilts: what a theme's CSS shows as text is marks and
// digits alone, and nothing it draws turns, skews or scales, in a transform, the properties rotate and scale, or a
// variable a transform may read.
func TestThemeMarks(t *testing.T) {
	t.Parallel()

	themes := os.DirFS("../kit/floe/themes")
	found := 0
	watched := func(p string) bool {
		return slices.Contains(texted, p) || strings.HasPrefix(p, "--") ||
			slices.Contains([]string{"transform", "-webkit-transform", "rotate", "scale"}, p)
	}
	err := fs.WalkDir(themes, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || path.Ext(name) != ".css" {
			return err
		}
		src, err := fs.ReadFile(themes, name)
		if err != nil {
			return err
		}
		for prop, decls := range declarations(src, watched) {
			for _, d := range decls {
				found++
				for _, why := range marksAmiss(prop, d.value) {
					t.Errorf("%s: %s: %s %s", name, prop, text(src, d.value), why)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found == 0 {
		t.Fatal("nothing watched found in floe's themes")
	}
}

// TestMarksAmiss runs the check on what floe's themes do not write today, each way it may come.
func TestMarksAmiss(t *testing.T) {
	t.Parallel()

	for src, want := range map[string]bool{
		`content: "“"`: false, `content: "●" / ""`: false, `content: counter(c, Decimal)`: false,
		`content: counters(c, ".", decimal-leading-zero)`: false, `transform: translate(2px, 50%)`: false, `rotate: none`: false,
		`content: "QUOTE"`: true, `content: "\201C"`: true, `content: attr(data-floe-tone)`: true,
		`quotes: "QUOTE" ""`: true, `list-style-type: "Q"`: true, `list-style-type: cjk-decimal`: true,
		`content: counter(c, cjk-decimal)`: true, `content: counters(c, ".", lower-roman)`: true,
		`transform: rotate(-2deg)`: true, `-webkit-transform: skewX(4deg)`: true, `transform: matrix(1, 0, 0, 1, 2, 2)`: true,
		`rotate: 2deg`: true, `scale: 1.1`: true, `--tilt: rotate(-2deg)`: true,
	} {
		decls := declarations([]byte(":scope { "+src+"; }"), func(string) bool { return true })
		amiss := false
		for prop, ds := range decls {
			for _, d := range ds {
				amiss = amiss || len(marksAmiss(prop, d.value)) > 0
			}
		}
		if amiss != want {
			t.Errorf("%s: amiss %v, want %v", src, amiss, want)
		}
	}
}
