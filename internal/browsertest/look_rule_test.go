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

	return b.measuredOf(page, b.eval(expr))
}

// measuredOf reads what a measure returned, raw, on the page told as page.
func (b *browser) measuredOf(page, raw string) measured {
	b.t.Helper()

	var got struct {
		Seen  int `json:"seen"`
		Amiss []struct {
			At   string `json:"at"`
			Text string `json:"text"`
		} `json:"amiss"`
	}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
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
	{name: "band-face", fix: lookBandFaceFix, floe: lookBandFace, broken: brokenBandFace},
	{name: "box-pad", fix: lookBoxPadFix, floe: lookBoxPad, broken: brokenBoxPad},
	{name: "cards-gap", fix: lookCardsGapFix, floe: lookCardsGap, broken: brokenCardsGap},
	{name: "cards-gap-var", fix: lookCardsGapVarFix, floe: lookCardsGapVar, broken: brokenCardsGapVar},
	{name: "contrast-ring", fix: lookContrastRingFix, floe: lookContrastRing, broken: brokenContrastRing},
	{name: "contrast-text", fix: lookContrastTextFix, floe: lookContrastText, broken: brokenContrastText},
	{name: "fragment-alone", fix: lookFragmentAloneFix, floe: lookFragmentAlone, broken: brokenFragmentAlone},
	{name: "fragment-outside", fix: lookFragmentOutsideFix, floe: lookFragmentOutside, broken: brokenFragmentOutside},
	{name: "layers", fix: lookLayersFix, floe: lookLayers, floeOnly: lookLayersFloeOnly},
	{name: "lists-marked", fix: lookListsMarkedFix, floe: lookListsMarked, broken: brokenListsMarked},
	{name: "lists-off", fix: lookListsOffFix, floe: lookListsOff, floeOnly: lookListsOffFloeOnly},
	{name: "marks", fix: lookMarksFix, floe: lookMarks, broken: brokenMarks},
	{name: "nesting", fix: lookNestingFix, floe: lookNesting, floeOnly: lookNestingFloeOnly},
	{name: "paper-ground", fix: lookPaperGroundFix, floe: lookPaperGround, broken: brokenPaperGround},
	{name: "paper-wrap", fix: lookPaperWrapFix, floe: lookPaperWrap, broken: brokenPaperWrap},
	{name: "print-tabs", fix: lookPrintTabsFix, floe: lookPrintTabs, broken: brokenPrintTabs},
	{name: "room", fix: lookRoomFix, floe: lookRoom, broken: brokenRoom},
	{name: "row-pad", fix: lookRowPadFix, floe: lookRowPad, broken: brokenRowPad},
	{name: "scroll-contain", fix: lookScrollContainFix, floe: lookScrollContain, broken: brokenScrollContain},
	{name: "scroll-ground", fix: lookScrollGroundFix, floe: lookScrollGround, broken: brokenScrollGround},
	{name: "scroll-wrap", fix: lookScrollWrapFix, floe: lookScrollWrap, broken: brokenScrollWrap},
	{name: "shadow-cast", fix: lookShadowCastFix, floe: lookShadowCast, broken: brokenShadowCast},
	{name: "shadows", fix: lookShadowsFix, floe: lookShadows, broken: brokenShadows},
	{name: "speech-read", fix: lookSpeechReadFix, floe: lookSpeechRead, broken: brokenSpeechRead},
	{name: "template-markdown", fix: lookTemplateMarkdownFix, floe: lookTemplateMarkdown, broken: brokenTemplateMarkdown},
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
	for i, l := range looks {
		if slices.ContainsFunc(looks[:i], func(o look) bool { return o.name == l.name }) {
			t.Errorf("looks holds %s twice: keep one", l.name)
		}
		if !slices.Contains(named, l.name) {
			t.Errorf("looks holds %s, which no line of .claude/rules names: end its line in `TestLookRule/%s`", l.name, l.name)
		}
		switch {
		case (l.old == nil) == (l.floe == nil):
			t.Errorf("%s has both a test as it was (old) and floe, or neither: keep one", l.name)
		case l.old != nil && (l.fix != "" || l.broken != nil || l.floeOnly != ""):
			t.Errorf("%s runs the test as it was (old), which takes no fix, broken page or floeOnly: drop them, or old", l.name)
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
			// floe runs first, so that the files broken adds to the site draw none of floe's pages.
			s := newSite(t)
			t.Run("floe", func(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome
				m := l.floe(t, s)
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
			t.Run("broken", func(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome
				at, m := l.broken(t, s)
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

// brokenPage publishes data over the files own (parts and themes of one's own) and opens it.
func (s *site) brokenPage(t *testing.T, own map[string]string, data string) {
	t.Helper()

	s.own(t, own)
	s.floe(t, "publish", s.write(t, "broken.yaml", data), "--slug", "broken")
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

// partOfJS defines partOf, which names the part an element is in (its own part, if a part's box), else its tag.
const partOfJS = `const partOf = (e) => { const p = e.closest('[data-floe-part]'); return p ? p.dataset.floePart : e.tagName.toLowerCase(); };`

// scrollsJS defines scrolls, which tells an element that scrolls sideways.
const scrollsJS = `const scrolls = (e) => ['auto', 'scroll'].includes(getComputedStyle(e).overflowX);`

// textRightJS defines textRight, the farthest right a text node's lines reach (-Infinity for none laid out).
const textRightJS = `const textRight = (n) => { const r = document.createRange(); r.selectNodeContents(n);
    return Math.max(-Infinity, ...[...r.getClientRects()].map((c) => c.right)); };`

// groundedJS defines grounded, which tells a computed style that paints a ground: a colour or an image.
const groundedJS = `const grounded = (s) => s.backgroundColor !== 'rgba(0, 0, 0, 0)' || s.backgroundImage !== 'none';`

// pxJS defines px, the width in px of the length v (a variable too) read in the element e: a probe in a box of its own,
// so that no rule for e's children (a list's rows) reaches it.
const pxJS = `const px = (e, v) => { const b = document.createElement('div'), d = b.appendChild(document.createElement('i'));
    b.style.cssText = 'position:absolute'; d.style.cssText = 'display:block;width:' + v; e.append(b);
    const w = parseFloat(getComputedStyle(d).width); b.remove(); return w; };`

// pads are the lengths of the variables of a row's and a box's padding, in px: --floe-row-pad, --floe-pad-block and
// --floe-pad-inline.
type pads struct{ Row, Block, Inline float64 }

// padsJS defines pads, the variables of padding read in the element e, as pads.
const padsJS = pxJS + `
  const pads = (e) => ({ Row: px(e, 'var(--floe-row-pad)'), Block: px(e, 'var(--floe-pad-block)'), Inline: px(e, 'var(--floe-pad-inline)') });`

// padsOn returns the variables of padding the body of the page open reads.
func padsOn(t *testing.T, s *site) pads {
	t.Helper()

	var p pads
	if err := json.Unmarshal([]byte(s.b.eval(`(() => { `+padsJS+` return JSON.stringify(pads(document.body)); })()`)), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

// basePads returns base.css's variables of padding, read on a page in a theme of one's own that sets nothing.
func basePads(t *testing.T, s *site) pads {
	t.Helper()

	s.own(t, map[string]string{"themes/plain/base.css": "/* no rules */\n"})
	s.floe(t, "publish", s.write(t, "plain.yaml", themePage("plain", "- text: x\n")), "--slug", "plain")
	s.b.navigate("/a/plain/?raw")
	return padsOn(t, s)
}

// themePage is a page of report in theme that holds parts, a YAML list.
func themePage(theme, parts string) string {
	return "template: report\nlang: en\ntitle: Broken\ntheme: " + theme + "\nsections:\n  - heading: Broken\n    parts:\n" + indent(parts, "      ")
}

// longWord is a word too long for a phone's line, or for paper's.
const longWord = "a_very_long_word_that_goes_on_and_on_and_on_and_on_and_on_and_on_and_on_and_on_and_on_and_on_and_on_and_on_and_on_and_on_and_on_and_on_and_on_and_on"

// htmlPage is a page of report that holds html in the part html, between two texts.
func htmlPage(html string) string {
	return "template: report\nlang: en\ntitle: Broken\nsections:\n  - heading: Broken\n    parts:\n      - text: one\n" +
		"      - html: |\n          " + html + "\n      - text: two\n"
}
