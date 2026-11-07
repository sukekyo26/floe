package browsertest_test

import (
	"os"
	"regexp"
	"slices"
	"testing"
)

// look is a rule of floe's CSS that only a browser can tell, measured on the pages floe draws: its name, as its line in
// .claude/rules/css.md names it, and the test that measures it.
type look struct {
	name string
	run  func(t *testing.T)
}

// looks are the rules TestLookRule measures, in the order of their names.
var looks = []look{
	{"band-face", lookBandFace},
	{"box-pad", lookBoxPad},
	{"cards-gap", lookCardsGap},
	{"contrast", lookContrast},
	{"fragment-root", lookFragmentRoot},
	{"layers", lookLayers},
	{"lists-off", lookListsOff},
	{"marks", lookMarks},
	{"nesting", lookNesting},
	{"paper", lookPaper},
	{"room", lookRoom},
	{"row-pad", lookRowPad},
	{"scroll-box", lookScrollBox},
	{"shadows", lookShadows},
	{"speech-read", lookSpeechRead},
	{"template-markdown", lookTemplateMarkdown},
	{"tone", lookTone},
	{"wrapped-alike", lookWrappedAlike},
}

// lookLine matches a line of css.md that ends in `TestLookRule/<name>`, and the name.
var lookLine = regexp.MustCompile("^- \\*\\*.*`TestLookRule/([a-z-]+)`$")

// TestLookRule measures each rule of looks on the pages floe draws, a subtest each. Before Chrome is needed, it holds the
// lines of css.md that end in TestLookRule/<name> and looks to one another, both ways, so that a rule is neither named
// with no test nor tested with no line.
func TestLookRule(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	src, err := os.ReadFile("../../.claude/rules/css.md")
	if err != nil {
		t.Fatal(err)
	}
	var named []string
	for _, line := range regexp.MustCompile("\n").Split(string(src), -1) {
		if m := lookLine.FindStringSubmatch(line); m != nil {
			named = append(named, m[1])
		}
	}
	for _, l := range looks {
		if !slices.Contains(named, l.name) {
			t.Errorf("looks holds %s, which no line of css.md ends in: end its line in `TestLookRule/%s`", l.name, l.name)
		}
	}
	for _, name := range named {
		if !slices.ContainsFunc(looks, func(l look) bool { return l.name == name }) {
			t.Errorf("css.md ends a line in TestLookRule/%s, which looks does not hold: add the rule to looks", name)
		}
	}
	for _, l := range looks { //nolint:paralleltest // as above
		t.Run(l.name, l.run)
	}
}
