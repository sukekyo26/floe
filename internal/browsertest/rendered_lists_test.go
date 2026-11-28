package browsertest_test

import (
	"os"
	"strings"
	"testing"
)

// TestRenderedLists draws the lists of a look and of parts in one's own kit (listsKitAll): an element that takes a
// look keeps its list under a theme's rule for every list, and takes its template's own; a look's own room and padding
// stand over floe's; timeline and options keep their own padding and gap; and decision's list in its text keeps its
// marks, the options set aside their crosses.
func TestRenderedLists(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.own(t, listsKitAll(t))
	techSpec, err := os.ReadFile("../kit/floe/templates/tech-spec/example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	decided := `(() => { const c = getComputedStyle(document.querySelector('.decided'));
  return [c.display, c.listStyleType, c.paddingInlineStart, c.rowGap].join(' '); })()`
	techSpecFile := s.write(t, "tech-spec.yaml", string(techSpec))
	s.floe(t, "publish", techSpecFile, "--slug", "lists-tech-spec-plain", "--theme", "plain")
	b.navigate("/a/lists-tech-spec-plain/?raw")
	wantDecided := b.eval(decided)
	s.floe(t, "publish", techSpecFile, "--slug", "lists-tech-spec-theme", "--theme", "lists")
	b.navigate("/a/lists-tech-spec-theme/?raw")
	if got := b.eval(decided); got != wantDecided {
		t.Errorf("a look's list under the theme's rule for lists: %s, want %s, as without it", got, wantDecided)
	}
	techSpecMine := s.write(t, "tech-spec-mine.yaml", strings.Replace(string(techSpec), "template: tech-spec\n", "template: lists-tech-spec\n", 1))
	s.floe(t, "publish", techSpecMine, "--slug", "lists-tech-spec-page", "--theme", "plain")
	b.navigate("/a/lists-tech-spec-page/?raw")
	if got := b.eval(decided); got != "flex upper-roman 32px 16px" {
		t.Errorf("a look's list under its template's rule for lists: %s, want flex upper-roman 32px 16px, the template's", got)
	}

	s.floe(t, "publish", s.write(t, "look.yaml", "lang: en\ntemplate: lists-look\ntitle: Look\n"), "--slug", "lists-look")
	b.navigate("/a/lists-look/?raw")
	look := `(() => { const c = getComputedStyle(document.querySelector('[data-floe-look=lists-gap]')); return c.rowGap + ' ' + c.paddingInlineStart; })()`
	if got := b.eval(look); got != "7px 9px" {
		t.Errorf("a look's own room and padding: %s, want 7px 9px, over floe's", got)
	}

	s.floe(t, "publish", s.write(t, "every.yaml", s.everyPart(t)), "--slug", "lists-plain")
	b.navigate("/a/lists-plain/?raw")
	ownWay := `(() => [getComputedStyle(document.querySelector('[data-floe-part=timeline]')).paddingInlineStart,
  getComputedStyle(document.querySelector('[data-floe-part=options]')).columnGap].join(' '))()`
	if got := b.eval(ownWay); got != "20.8px 12px" {
		t.Errorf("timeline's padding and options' gap: %s, want 20.8px 12px, the parts' own", got)
	}

	s.floe(t, "publish", s.write(t, "decision.yaml", listsDecision), "--slug", "lists-decision")
	b.navigate("/a/lists-decision/?raw")
	marks := `(() => ['[data-floe-part=decision] ul:not(.rejected)', '[data-floe-part=decision] .rejected']
  .map((q) => getComputedStyle(document.querySelector(q)).listStyleType + ' ' +
    getComputedStyle(document.querySelector(q + ' > li'), '::before').content).join(', '))()`
	if got := b.eval(marks); got != `disc none, none "×" / ""` {
		t.Errorf("decision's list in its text and the options set aside: %s, want disc none, none \"×\" / \"\"", got)
	}
}
