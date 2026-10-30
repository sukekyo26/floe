package browsertest_test

import "testing"

// longLine is a paragraph wider than --text-width in a wide window.
const longLine = "A paragraph long enough to fill a wide window, longer than the width prose is held to on a page, " +
	"so that it wraps where the box it is in ends, or where the text of the page ends, and its width tells which of " +
	"the two held it, for the text of the page, for text folded in details, and for the text in the box of entries."

// textWidthPage holds a long paragraph as the page's text, folded in details, and in entries' text.
const textWidthPage = `template: report
lang: en
title: Text width
sections:
  - heading: Text width
    parts:
      - text: '` + longLine + `'
      - details:
          title: Folded
          open: true
          parts:
            - text: '` + longLine + `'
      - entries:
          - { title: Long, text: '` + longLine + `' }
`

// measureTextWidth returns, for each paragraph, its part's name (or page) and whether it is held to --text-width
// (narrow) or reads to its box's width (wide).
const measureTextWidth = `(() => [...document.querySelectorAll('p')].map((p) => {
  const box = p.parentElement, s = getComputedStyle(box);
  const inner = box.clientWidth - parseFloat(s.paddingLeft) - parseFloat(s.paddingRight);
  const part = p.parentElement.closest('[data-floe-part]');
  return (part ? part.dataset.floePart : 'page') + ' ' + (inner - p.getBoundingClientRect().width > 40 ? 'narrow' : 'wide');
}).join(', '))()`

// TestUITextWidth draws a page in a wide window (1600 px): text in a part's box reads to the box's width, while the
// page's own text, folded in details too, keeps --text-width.
func TestUITextWidth(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.write(t, "width.yaml", textWidthPage), "--slug", "width")
	b.size(1600, 900, false)
	b.navigate("/a/width/?raw")
	if got, want := b.eval(measureTextWidth), "page narrow, stack narrow, entries wide"; got != want {
		t.Errorf("at 1600 px the paragraphs are %q, want %q", got, want)
	}
}
