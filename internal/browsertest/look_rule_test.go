package browsertest_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// amiss is what a look finds on a page it measures: which page (a theme, a width, a template), where on it (the part,
// the element, one name), and what is amiss.
type amiss struct{ Page, At, Text string }

// measured is what a look measured: how many things it looked at, and what is amiss.
type measured struct {
	seen  int
	amiss []amiss
}

// add takes o into m.
func (m *measured) add(o measured) {
	m.seen += o.seen
	m.amiss = append(m.amiss, o.amiss...)
}

// measure runs expr, a measure that returns {seen, amiss: [{at, text}]} as JSON, on the page open, told as page.
func (b *browser) measure(page, expr string) measured {
	b.t.Helper()

	var got struct {
		Seen  int `json:"seen"`
		Amiss []struct {
			At   string `json:"at"`
			Text string `json:"text"`
		} `json:"amiss"`
	}
	if err := json.Unmarshal([]byte(b.eval(expr)), &got); err != nil {
		b.t.Fatalf("%s: the measure returns no {seen, amiss}: %v", page, err)
	}
	m := measured{seen: got.Seen, amiss: make([]amiss, 0, len(got.Amiss))}
	for _, a := range got.Amiss {
		m.amiss = append(m.amiss, amiss{Page: page, At: a.At, Text: a.Text})
	}
	return m
}

// look is a rule of floe's CSS that only a browser can tell: its name, as its line in .claude/rules names it, and how
// it is measured.
type look struct {
	name string
	// fix says how to keep the rule, told with every amiss.
	fix string
	// floe draws floe's pages the rule holds for and measures them.
	floe func(t *testing.T, s *site) measured
	// broken draws a page that breaks this rule alone, at one place, measures it as floe does, and names the place (at).
	broken func(t *testing.T, s *site) (at string, m measured)
	// floeOnly says why a writer cannot break the rule (floe's own way keeps it): such a rule has no broken.
	floeOnly string
	// old is the test the rule had, run as it was till the rule is remade into floe and broken.
	old func(t *testing.T)
}

// looks are the rules TestLookRule measures, in the order of their names.
var looks = []look{
	{name: "band-face", old: lookBandFace},
	{name: "box-pad", old: lookBoxPad},
	{name: "cards-gap", old: lookCardsGap},
	{name: "contrast", old: lookContrast},
	{name: "fragment-root", old: lookFragmentRoot},
	{name: "layers", old: lookLayers},
	{name: "lists-marked", fix: lookListsMarkedFix, floe: lookListsMarked, broken: brokenListsMarked},
	{name: "lists-off", old: lookListsOff},
	{name: "marks", old: lookMarks},
	{name: "nesting", old: lookNesting},
	{name: "paper", old: lookPaper},
	{name: "room", fix: lookRoomFix, floe: lookRoom, broken: brokenRoom},
	{name: "row-pad", old: lookRowPad},
	{name: "scroll-box", old: lookScrollBox},
	{name: "shadows", old: lookShadows},
	{name: "speech-read", old: lookSpeechRead},
	{name: "template-markdown", old: lookTemplateMarkdown},
	{name: "tone", fix: lookToneFix, floe: lookTone, broken: brokenTone},
	{name: "wrapped-alike", fix: lookWrappedAlikeFix, floe: lookWrappedAlike, broken: brokenWrappedAlike},
}

// lookNamed matches the rules a line of a rules file names to TestLookRule.
var lookNamed = regexp.MustCompile("`TestLookRule/([a-z-]+)`")

// lookRuleLines returns the rules the lines of .claude/rules name to TestLookRule.
func lookRuleLines(t *testing.T) []string {
	t.Helper()

	files, err := filepath.Glob("../../.claude/rules/*.md")
	if err != nil || len(files) == 0 {
		t.Fatalf("no rules files: %v", err)
	}
	var named []string
	for _, file := range files {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for line := range strings.Lines(string(src)) {
			if strings.HasPrefix(line, "- **") {
				for _, m := range lookNamed.FindAllStringSubmatch(line, -1) {
					named = append(named, m[1])
				}
			}
		}
	}
	return named
}

// TestLookRule measures each rule of looks, a subtest each. Before Chrome is needed, it holds the lines of the rules
// files that name TestLookRule/<name> and looks to one another, both ways, and each rule to one way of being measured:
// the test it had (old), or floe's pages (floe) with a fix and either a page that breaks it (broken) or why a writer
// cannot (floeOnly). A rule remade has two subtests: floe, which fails on what is amiss and on measuring nothing, and
// broken, which fails unless every amiss names the place it broke, at least one.
func TestLookRule(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	named := lookRuleLines(t)
	for _, l := range looks {
		if !slices.Contains(named, l.name) {
			t.Errorf("looks holds %s, which no line of .claude/rules names: end its line in `TestLookRule/%s`", l.name, l.name)
		}
		switch {
		case (l.old == nil) == (l.floe == nil):
			t.Errorf("%s has both a test as it was (old) and floe, or neither: keep one", l.name)
		case l.floe != nil && l.fix == "":
			t.Errorf("%s says not how to keep it: write its fix", l.name)
		case l.floe != nil && (l.broken == nil) == (l.floeOnly == ""):
			t.Errorf("%s has both a broken page and why none (floeOnly), or neither: keep one", l.name)
		default:
		}
	}
	for _, name := range named {
		if !slices.ContainsFunc(looks, func(l look) bool { return l.name == name }) {
			t.Errorf("a line of .claude/rules names TestLookRule/%s, which looks does not hold: add the rule to looks", name)
		}
	}
	if t.Failed() {
		return
	}
	for _, l := range looks { //nolint:paralleltest // as above
		if l.old != nil {
			t.Run(l.name, l.old)
			continue
		}
		t.Run(l.name, func(t *testing.T) {
			t.Run("floe", func(t *testing.T) {
				m := l.floe(t, newSite(t))
				if m.seen == 0 {
					t.Fatalf("%s measured nothing on floe's pages: what it measures is renamed or gone", l.name)
				}
				for _, a := range m.amiss {
					t.Errorf("%s %s: %s: %s; %s", a.Page, a.At, l.name, a.Text, l.fix)
				}
			})
			if l.broken == nil {
				return
			}
			t.Run("broken", func(t *testing.T) {
				at, m := l.broken(t, newSite(t))
				if at == "" {
					t.Fatalf("%s's broken page names no place it broke", l.name)
				}
				if len(m.amiss) == 0 {
					t.Errorf("%s tells nothing of the page that breaks it at %s: the measure no longer sees the rule", l.name, at)
				}
				for _, a := range m.amiss {
					if a.At != at {
						t.Errorf("%s tells of %s on the page broken at %s alone: %s", l.name, a.At, at, a.Text)
					}
				}
			})
		})
	}
}

// brokenPage publishes data over the files own (parts and themes of one's own), with args to publish, and opens it.
func (s *site) brokenPage(t *testing.T, own map[string]string, data string, args ...string) {
	t.Helper()

	s.own(t, own)
	s.floe(t, append([]string{"publish", s.write(t, "broken.yaml", data), "--slug", "broken"}, args...)...)
	s.b.navigate("/a/broken/?raw")
}

// ownPart is the files of a part of one's own, name, that draws html and takes css. report places it by its html in
// the part html (report takes no part of one's own by name).
func ownPart(name, html, css string) map[string]string {
	return map[string]string{
		"parts/" + name + "/schema.json": `{"type": "object"}`,
		"parts/" + name + "/part.html":   html,
		"parts/" + name + "/part.css":    css,
	}
}

// htmlPage is a page of report that holds html in the part html, between two texts.
func htmlPage(html string) string {
	return "template: report\nlang: en\ntitle: Broken\nsections:\n  - heading: Broken\n    parts:\n      - text: one\n" +
		"      - html: |\n          " + html + "\n      - text: two\n"
}
