package css

import (
	"strings"
	"testing"
)

// caseChanges returns the text-transform values in src other than none, alone or !important, by its tokens with
// case set aside (NONE is none).
func caseChanges(src []byte) []declaration {
	var changes []declaration
	for _, d := range declarations(src, func(p string) bool { return p == "text-transform" })["text-transform"] {
		var words []string
		for _, tok := range d.value {
			words = append(words, strings.ToLower(string(tok.data)))
		}
		switch strings.Join(words, " ") {
		case "none", "none ! important":
		default:
			changes = append(changes, d)
		}
	}
	return changes
}

// TestTextCase keeps floe's CSS from changing the case of the text it draws: a browser copies the changed case, so a
// file name or code copied from a page would come out as other text.
// LIMIT: the .css files under internal/kit/floe alone are read: not one's own parts, themes and templates, nor the
// style= attributes of floe's part.html and page.html, nor the CSS internal/draw/style.go builds. Nor is
// font-variant-caps (small-caps), which looks like capitals but copies as written. Nor is the management page's
// <style> in internal/ui/ui.html, whose .side-head still sets uppercase: the management page redesign fixes it.
func TestTextCase(t *testing.T) {
	t.Parallel()

	sheets(t, "../kit/floe", func(name string, src []byte) {
		for _, d := range caseChanges(src) {
			t.Errorf("%s: text-transform: %s in %q; draw the text as written, and write capitals as capitals",
				name, text(src, d.value), d.rule)
		}
	})
}

// TestTextCaseForms runs the check on forms floe's CSS does not write today, one at a time.
func TestTextCaseForms(t *testing.T) {
	t.Parallel()

	for src, want := range map[string]bool{
		`text-transform: none`: false, `text-transform: NONE`: false, `text-transform: None`: false,
		`text-transform: none !important`: false, `--case: uppercase`: false, `font-variant-caps: all-small-caps`: false,
		`text-transform: uppercase`: true, `text-transform: capitalize`: true, `text-transform: lowercase`: true,
		`text-transform: full-width`: true, `TEXT-TRANSFORM: uppercase`: true, `text-transform: var(--case)`: true,
		`text-transform: none uppercase`: true,
	} {
		if changes := caseChanges([]byte(":scope { " + src + "; }")); (len(changes) > 0) != want {
			t.Errorf("%s: %d changes, want changes %v", src, len(changes), want)
		}
	}
}
