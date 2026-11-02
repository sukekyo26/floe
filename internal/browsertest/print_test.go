package browsertest_test

import (
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/kit"
)

// measurePrint returns a line for each element with a ground (a colour or an image) that print would leave out: one
// whose print-color-adjust is not exact, read with the page drawn for print.
const measurePrint = `(() => {
  const out = new Set();
  for (const e of [document.documentElement, ...document.querySelectorAll('*')]) {
    const s = getComputedStyle(e);
    if (s.backgroundColor === 'rgba(0, 0, 0, 0)' && s.backgroundImage === 'none') continue;
    if (s.printColorAdjust !== 'exact') out.add(e.tagName.toLowerCase() + [...e.classList].map((c) => '.' + c).join('') + ': ' + s.printColorAdjust);
  }
  return [...out].join('\n');
})()`

// measureTabs returns a line for each tabs whose titles are not drawn alike, read with the page drawn for print: on
// paper every tab shows under its title, so that the one picked on the screen is no more than the others.
const measureTabs = `(() => {
  const out = [];
  const look = (e) => { const s = getComputedStyle(e);
    return [s.backgroundColor, s.backgroundImage, s.color, s.borderTop, s.borderBottom, s.boxShadow, s.borderRadius].join(' / '); };
  for (const tabs of document.querySelectorAll('[data-floe-part="tabs"]')) {
    const titles = [...tabs.querySelectorAll(':scope > label')].map(look);
    if (new Set(titles).size > 1) out.push('tabs: ' + titles.join(' | '));
  }
  return out.join('\n');
})()`

// measurePaper returns, read with the page drawn for print, a line for a page wider than the paper, one for each
// element that reaches past it, one for each that scrolls sideways (paper shows no scrolled part), and one for each
// that clips what it holds sideways (overflow hidden or clip, as code's box rounds its corners) wider than itself.
// A box's scrollWidth is not read where nothing clips: a decoration drawn out of a box (washi's seal on a card) is
// within --floe-room by the rules, and reaches no further than the paper. A word hidden to the eye (base.css's
// .floe-hidden) is clipped to a pixel by design, and what ends in an ellipsis (links' URL) is cut on purpose.
const measurePaper = `(() => {
  const out = [], w = innerWidth;
  if (document.documentElement.scrollWidth > w) out.push('the page is ' + document.documentElement.scrollWidth + ' px wide on ' + w);
  const name = (e) => e.tagName.toLowerCase() + ' ' + e.textContent.trim().slice(0, 40);
  for (const e of document.querySelectorAll('body *')) {
    if (e.matches('.floe-hidden, .floe-copy-status') || !e.getClientRects().length) continue;
    const x = getComputedStyle(e).overflowX;
    if (['auto', 'scroll'].includes(x)) out.push('scrolls: ' + name(e));
    else if (['hidden', 'clip'].includes(x) && getComputedStyle(e).textOverflow !== 'ellipsis' && e.scrollWidth > e.clientWidth + 1) out.push('clips: ' + name(e));
    else if (e.getBoundingClientRect().right > w + 1) out.push('past the paper: ' + name(e));
  }
  return out.join('\n');
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
`

// paperWidth is an A4 sheet's width less Chrome's margins of 1cm, in CSS px: 190mm. The window draws no scrollbar,
// so that the page is laid out that wide, as on paper.
const paperWidth = 718

// TestUIPrint draws every page of floePages, and printPage, for print as wide as an A4 sheet's text, and finds
// each on paper (measurePaper): nothing scrolls sideways there, as what scrolls is not read on paper, and folds onto
// lines within it, but what folds no word there (LIMIT: a table's heads and cells, a column of numbers or of pills, a
// tree's note and its depth) may still reach past it. It finds every ground kept on paper: print leaves a ground
// out unless asked, as a browser's print does by default, and a light text on a fill would be lost on white paper.
// It finds tabs whose picked title is drawn apart from the others, as tabs paints the picked one on the screen alone.
// LIMIT: tabs is the only part whose state print draws otherwise; check a part's that comes to do so too.
// LIMIT: it tells the picked title drawn apart alone, not a ground a theme gives every title alike outside @media screen.
func TestUIPrint(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	s.b.size(paperWidth, 1000, false)
	s.b.scrollbars(true)
	defer s.b.unsize()
	defer s.b.scrollbars(false)
	s.b.media("print")
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	page := s.write(t, "print.yaml", printPage)
	for _, theme := range themes {
		s.floe(t, "publish", page, "--slug", "print-"+theme, "--theme", theme)
		s.b.navigate("/a/print-" + theme + "/?raw")
		if wide := s.b.eval(measurePaper); wide != "" {
			t.Errorf("printPage in %s on paper:\n  %s", theme, strings.ReplaceAll(wide, "\n", "\n  "))
		}
	}
	s.floePages(t, func(what string) {
		if wide := s.b.eval(measurePaper); wide != "" {
			t.Errorf("%s on paper:\n  %s", what, strings.ReplaceAll(wide, "\n", "\n  "))
		}
		if lost := s.b.eval(measurePrint); lost != "" {
			t.Errorf("%s draws a ground print leaves out:\n  %s", what, strings.ReplaceAll(lost, "\n", "\n  "))
		}
		if apart := s.b.eval(measureTabs); apart != "" {
			t.Errorf("%s draws a tab's title apart on paper:\n  %s", what, strings.ReplaceAll(apart, "\n", "\n  "))
		}
	})
}
