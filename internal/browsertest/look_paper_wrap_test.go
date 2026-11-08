package browsertest_test

import (
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/kit"
)

// measurePaper finds, read with the page drawn for print, each element that scrolls sideways (paper shows no scrolled
// part), that clips what it holds sideways (overflow hidden or clip, as code's box rounds its corners) wider than
// itself, or whose own content (its text, or itself if an image, a drawing or a control) reaches past the paper. It
// tells every element that scrolls or clips, the cause, and of those whose content reaches past the paper, the
// innermost alone and none in an element told to scroll or clip (not the boxes that what reaches past widens around
// it), and the page wider than the paper only when no element is told.
// LIMIT: a box with no text of its own (a bar, a frame, an hr) and the text a display: contents element holds are told by
// the page's line alone, with no place; measure their boxes too if a part comes to draw one past the paper.
// A box's scrollWidth is not read where nothing clips: a decoration drawn out of a box (washi's seal on a card) is
// within --floe-room by the rules, and reaches no further than the paper. A word hidden to the eye (base.css's
// .floe-hidden) is clipped to a pixel by design.
// It returns {seen, amiss}: the elements it measured, and for each told its part and what is amiss.
const measurePaper = `(() => {
  ` + partOfJS + textRightJS + `
  let seen = 0;
  const w = innerWidth, cause = [], past = [];
  const name = (e) => e.tagName.toLowerCase() + ' ' + e.textContent.trim().slice(0, 40);
  const reach = (e) => {
    if (e.matches('img, svg, video, canvas, iframe, input, textarea, select')) return e.getBoundingClientRect().right;
    return Math.max(-Infinity, ...[...e.childNodes].filter((n) => n.nodeType === Node.TEXT_NODE && n.data.trim()).map(textRight));
  };
  for (const e of document.querySelectorAll('body *')) {
    if (e.matches('.floe-hidden, .floe-copy-status') || !e.getClientRects().length) continue;
    seen++;
    const x = getComputedStyle(e).overflowX;
    if (['auto', 'scroll'].includes(x)) cause.push({ e, text: 'scrolls: ' + name(e) });
    else if (['hidden', 'clip'].includes(x) && e.scrollWidth > e.clientWidth + 1) cause.push({ e, text: 'clips: ' + name(e) });
    else if (reach(e) > w + 1) past.push({ e, text: 'past the paper: ' + name(e) });
  }
  const inner = past.filter((a) => !cause.some((c) => c.e.contains(a.e)) && !past.some((b) => b !== a && a.e.contains(b.e)));
  const amiss = [...cause, ...inner].map((a) => ({ at: partOf(a.e), text: a.text }));
  const page = document.documentElement.scrollWidth;
  if (!amiss.length && page > w) amiss.push({ at: 'page', text: 'the page is ' + page + ' px wide on ' + w });
  return JSON.stringify({ seen, amiss });
})()`

// printPage holds what scrolls sideways on screen, to be folded on paper: long lines in code, a diff, a terminal,
// Markdown's code in a text and in a card, a deep tree with a long name, an ordinary table with a path in code, and
// a table of six short columns, which fits on paper but for a column's least width on screen (7em).
const printPage = `template: report
lang: en
title: Print
sections:
  - heading: Print
    parts:
      - code: { file: run.sh, lang: sh, text: "floe render --out-dir internal/kit/floe/parts/findings/testdata/golden internal/kit/floe/parts/findings/example.yaml --end\necho done\n" }
      - diff:
          file: run.sh
          text: |
            @@ -1 +1 @@
            -floe render --out-dir internal/kit/floe/parts/findings/testdata/golden internal/kit/floe/parts/findings/example.yaml
            +floe render --out-dir internal/kit/floe/parts/findings/testdata/golden internal/kit/floe/parts/findings/example.yaml --end
      - terminal:
          shell: bash
          steps:
            - run: floe render --out-dir internal/kit/floe/parts/findings/testdata/golden internal/kit/floe/parts/findings/example.yaml
              out: wrote internal/kit/floe/parts/findings/testdata/golden/example.html internal/kit/floe/parts/findings/testdata/golden/second.html
      - tree:
          - { path: internal/kit/floe/parts/findings/testdata/golden/a/b/c/d/e/f/g/h/a-very-long-file-name-of-a-test-on-paper.go }
      - table:
          columns: [File, Change, Lines]
          rows:
            - ['` + "`internal/kit/floe/parts/findings/part_internal_test.go`" + `', adds the tests of the findings part, '120']
      - table:
          columns: [One, Two, Three, Four, Five, Six]
          rows:
            - [red, green, blue, cyan, pink, gray]
      - text: |
          ` + "```" + `
          floe render --out-dir internal/kit/floe/parts/findings/testdata/golden internal/kit/floe/parts/findings/example.yaml --end
          ` + "```" + `
      - options:
          - title: An option
            text: |
              ` + "```" + `
              floe render --out-dir internal/kit/floe/parts/findings/testdata/golden internal/kit/floe/parts/findings/example.yaml --end
              ` + "```" + `
      - links:
          - { title: A guide, url: 'https://example.com/unbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbrokenunbroken' }
`

// paperWidth is an A4 sheet's width less Chrome's margins of 1cm, in CSS px: 190mm. The window draws no scrollbar,
// so that the page is laid out that wide, as on paper.
const paperWidth = 718

// slidesWidth is an A4 sheet's length, in CSS px: slides prints on a landscape sheet with no margin (its @page).
const slidesWidth = 1122

// onPaper draws s's pages for print, as wide as an A4 sheet's text, till the test ends.
func onPaper(t *testing.T, s *site) {
	t.Helper()

	s.b.size(paperWidth, 1000, false)
	s.b.scrollbars(true)
	s.b.media("print")
	t.Cleanup(func() {
		s.b.media("")
		s.b.scrollbars(false)
		s.b.unsize()
	})
}

const lookPaperWrapFix = "fold what a box holds on paper (print), where nothing scrolls sideways, within the paper's width"

// lookPaperWrap measures, on paper (measurePaper), printPage in every theme and every page of floePages (slides as
// wide as its landscape sheet).
// LIMIT: what folds no word there (a table's heads and cells, a column of numbers or of pills, a tree's note and its
// depth, a field's name, a finding's level) may still reach past the paper, or past a card it is in, which nothing
// measures.
func lookPaperWrap(t *testing.T, s *site) measured {
	t.Helper()

	onPaper(t, s)
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	var m measured
	page := s.write(t, "print.yaml", printPage)
	for _, theme := range themes {
		s.floe(t, "publish", page, "--slug", "print-"+theme, "--theme", theme)
		s.b.navigate("/a/print-" + theme + "/?raw")
		m.add(s.b.measure("printPage in "+theme, measurePaper))
	}
	s.floePages(t, func(what string) {
		if strings.HasPrefix(what, "slides in ") {
			s.b.size(slidesWidth, 1000, false)
			defer s.b.size(paperWidth, 1000, false)
		}
		m.add(s.b.measure(what, measurePaper))
	})
	return m
}

// brokenPaperWrap draws a part of one's own whose pre keeps a long line whole on paper.
func brokenPaperWrap(t *testing.T, s *site) (string, measured) {
	t.Helper()

	html := `<pre data-floe-part="wide">` + longWord + `</pre>`
	onPaper(t, s)
	s.brokenPage(t, ownPart("wide", html, ":scope { white-space: pre; margin: 0; }\n"), htmlPage(html))
	return "wide", s.b.measure("broken", measurePaper)
}
