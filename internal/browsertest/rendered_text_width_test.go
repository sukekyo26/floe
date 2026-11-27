package browsertest_test

import "testing"

// longLine is a paragraph longer than a line in a wide window.
const longLine = "A paragraph long enough to fill a wide window, longer than a line of the page in it, so that it " +
	"wraps where the column or the box it is in ends, or short of it if anything holds it narrower, and its width " +
	"tells which, for the text of the page, for text folded in details, and for the text in the box of entries."

// textWidthPage holds a long paragraph as the page's text, folded in details, and in entries' text, and a note in the
// page and in details.
const textWidthPage = `template: report
lang: en
title: Text width
sections:
  - heading: Text width
    parts:
      - text: '` + longLine + `'
      - note: { text: Short }
      - details:
          title: Folded
          open: true
          parts:
            - text: '` + longLine + `'
            - note: { text: Short }
      - entries:
          - { title: Long, text: '` + longLine + `' }
`

// measureWidth returns, for each element the selector picks, the name of the part it is in (or page) and whether it is
// held short of what it is in (narrow) or takes its width (wide).
func measureWidth(selector string) string {
	return `(() => [...document.querySelectorAll('` + selector + `')].map((e) => {
  const box = e.parentElement, s = getComputedStyle(box);
  const inner = box.clientWidth - parseFloat(s.paddingLeft) - parseFloat(s.paddingRight);
  const part = box.closest('[data-floe-part]');
  return (part ? part.dataset.floePart : 'page') + ' ' + (inner - e.getBoundingClientRect().width > 40 ? 'narrow' : 'wide');
}).join(', '))()`
}

// TestRenderedTextWidth draws a page in a wide window (1600 px): text takes the width of what it is in, the page's column for
// the page's own text, folded in details too, and the box for a part's text; so does a note's box, wider than --text-width.
func TestRenderedTextWidth(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.write(t, "width.yaml", textWidthPage), "--slug", "width")
	b.size(1600, 900, false)
	b.navigate("/a/width/?raw")
	if got, want := b.eval(measureWidth("p")), "page wide, note wide, stack wide, note wide, entries wide"; got != want {
		t.Errorf("at 1600 px the paragraphs are %q, want %q", got, want)
	}
	if got, want := b.eval(measureWidth("[data-floe-part=note]")), "page wide, stack wide"; got != want {
		t.Errorf("at 1600 px the notes are %q, want %q", got, want)
	}
}
