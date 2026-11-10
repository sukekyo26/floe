package browsertest_test

import (
	"fmt"
	"testing"

	"floe/internal/kit"
)

// wrapPage holds a path and a URL in options, which have no place to break a line, words beside narrow
// columns (keys' text beside its keycaps, the language of code and the counts of a diff beside a long file
// name), a path in a table, in a tree's note (beside a deep tree) and in a text's Markdown table and code (and a long word in a table's cell), which scroll instead, and entries with a long URL for a title, a
// long path, a long pill and a long line of code in its text. A long block and a table also stand in an item of a
// list and in a quote, in a text, a text in details, a card of options and of a decision, a step and a tab, where an
// item of a grid or a flex row around them takes their width unless the box that scrolls tells nothing of it.
// A bar of a long name stands in details, and facts, stats, keys, a timeline and asks in an option of options in an
// option, the narrowest place measured, where a part lays out by its place's width, not the window's.
const wrapPage = `template: report
lang: en
title: Wrap
sections:
  - heading: Wrap
    parts:
      - options:
          - { title: path, text: 'See ` + "`internal/kit/floe/templates/report/page.html:216`" + ` here.' }
          - { title: url, text: 'See https://github.com/sukekyo26/floe/pull/117/files here.' }
      - keys:
          - { keys: [Control, Shift, Escape], text: open the task manager }
      - code: { file: internal/kit/floe/parts/findings/part_internal_test.go, lang: typescript, text: let a = 1 }
      - diff: { file: internal/kit/floe/parts/findings/part_internal_test.go, text: "+1\n+2\n+3\n+4\n+5\n+6\n+7\n+8\n+9\n+10\n+11\n+12\n-x\n" }
      - table:
          columns: [{ title: file }, { title: what }]
          rows:
            - ['` + "`internal/kit/floe/parts/findings/part_internal_test.go`" + `', a test of the findings]
      - text: |
          | file | what | where | when | who |
          | --- | --- | --- | --- | --- |
          | internal/kit/floe/parts/findings/part_internal_test.go | a test of the findings | in the kit | on Sunday | the team |

              internal/kit/floe/parts/findings/part_internal_test.go internal/kit/floe/parts/findings/part.css

          | word |
          | --- |
          | internal/kit/floe/parts/findings/part_internal_test.go/and/more/of/it/with/no/place/to/break |
      - text: |
          - An item holding a long block:

            ` + "```" + `
            floe render --out-dir internal/kit/floe/parts/findings/testdata/golden internal/kit/floe/parts/findings/example.yaml
            ` + "```" + `
          - An item holding a table:

            | file | what | where | when | who |
            | --- | --- | --- | --- | --- |
            | internal/kit/floe/parts/findings/part_internal_test.go | a test of the findings | in the kit | on Sunday | the team |
          - A list in a list:
            1. holding a long block:

               ` + "```" + `
               floe render --out-dir internal/kit/floe/parts/findings/testdata/golden internal/kit/floe/parts/findings/example.yaml
               ` + "```" + `

          > A quote holding a long block:
          >
          > ` + "```" + `
          > floe render --out-dir internal/kit/floe/parts/findings/testdata/golden internal/kit/floe/parts/findings/example.yaml
          > ` + "```" + `

          > A quote holding a table:
          >
          > | file | what | where | when | who |
          > | --- | --- | --- | --- | --- |
          > | internal/kit/floe/parts/findings/part_internal_test.go | a test of the findings | in the kit | on Sunday | the team |
      - details:
          title: A details holding a long block
          open: true
          parts:
            - text: |
                > A quote holding a long block:
                >
                > ` + "```" + `
                > floe render --out-dir internal/kit/floe/parts/findings/testdata/golden internal/kit/floe/parts/findings/example.yaml
                > ` + "```" + `

                - an item:

                  ` + "```" + `
                  floe render --out-dir internal/kit/floe/parts/findings/testdata/golden internal/kit/floe/parts/findings/example.yaml
                  ` + "```" + `

                > A quote holding a table:
                >
                > | file | what | where | when | who |
                > | --- | --- | --- | --- | --- |
                > | internal/kit/floe/parts/findings/part_internal_test.go | a test of the findings | in the kit | on Sunday | the team |
      - options:
          - title: An option holding a long block
            text: |
              > A quote holding a long block:
              >
              > ` + "```" + `
              > floe render --out-dir internal/kit/floe/parts/findings/testdata/golden internal/kit/floe/parts/findings/example.yaml
              > ` + "```" + `

              > A quote holding a table:
              >
              > | file | what | where | when | who |
              > | --- | --- | --- | --- | --- |
              > | internal/kit/floe/parts/findings/part_internal_test.go | a test of the findings | in the kit | on Sunday | the team |
      - decision:
          title: A decision holding a long block
          text: |
            > A quote holding a long block:
            >
            > ` + "```" + `
            > floe render --out-dir internal/kit/floe/parts/findings/testdata/golden internal/kit/floe/parts/findings/example.yaml
            > ` + "```" + `

            > A quote holding a table:
            >
            > | file | what | where | when | who |
            > | --- | --- | --- | --- | --- |
            > | internal/kit/floe/parts/findings/part_internal_test.go | a test of the findings | in the kit | on Sunday | the team |
      - steps:
          - title: A step holding a long block
            text: |
              - an item:

                ` + "```" + `
                floe render --out-dir internal/kit/floe/parts/findings/testdata/golden internal/kit/floe/parts/findings/example.yaml
                ` + "```" + `

              - an item holding a table:

                | file | what | where | when | who |
                | --- | --- | --- | --- | --- |
                | internal/kit/floe/parts/findings/part_internal_test.go | a test of the findings | in the kit | on Sunday | the team |
      - tabs:
          - title: A
            parts:
              - text: |
                  > A quote holding a long block:
                  >
                  > ` + "```" + `
                  > floe render --out-dir internal/kit/floe/parts/findings/testdata/golden internal/kit/floe/parts/findings/example.yaml
                  > ` + "```" + `

                  > A quote holding a table:
                  >
                  > | file | what | where | when | who |
                  > | --- | --- | --- | --- | --- |
                  > | internal/kit/floe/parts/findings/part_internal_test.go | a test of the findings | in the kit | on Sunday | the team |
          - title: B
            parts:
              - text: b
      - details:
          title: A bar of a long name
          open: true
          parts:
            - bars:
                - { label: Pages copied to the new host, value: 1200, of: 1840, unit: pages per hour for each team }
      - options:
          - title: An option holding options
            parts:
              - options:
                  - title: An option holding parts
                    parts:
                      - facts:
                          - { name: Owner, value: The platform team }
                      - stats:
                          - { label: Pages, value: '1840', from: '1900', better: lower }
                      - keys:
                          - { keys: [Ctrl, K], text: Search the wiki }
                      - timeline:
                          - { date: 2030-04-05, title: Freeze }
                      - asks:
                          - { question: Copy in one go?, options: [One go, By space], recommend: One go }
                  - title: Another
          - title: Another
      - tree:
          - { path: internal/kit/floe/parts/, note: internal/kit/floe/parts/findings/part_internal_test.go }
      - tree:
          - { path: internal/kit/floe/parts/findings/testdata/golden/a/b/c/d/e/f/g/h/i/a-very-long-file-name-of-a-test.go }
      - entries:
          - title: https://github.com/sukekyo26/floe/pull/313/files/a0a4ebfb0c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8
            tags: ['` + "`internal/kit/floe/parts/findings/part.html`" + `', { pill: blocked until the platform team reviews the export and the import and the backup of every page, tone: warn }]
            text: |
              The whole command:

              ` + "```" + `
              floe render --out-dir internal/kit/floe/parts/findings/testdata/golden internal/kit/floe/parts/findings/example.yaml
              ` + "```" + `

              | name | kind | where | what | state | owner |
              | --- | --- | --- | --- | --- | --- |
              | export | nightly | wiki.conf | writes pages | running | platform |
`

// measureWrap finds each part's box, or a box in it, out of those that scroll sideways and the words only a screen reader
// reads (.floe-hidden, a 1px box their text overflows), that is wider than its place
// or reaches past the window, and tells the innermost of them alone, as what overflows widens the boxes around it; the
// page wider than the window only when no box is told. It finds each word that breaks across lines in keys' text,
// code's language, a diff's counts, a table and a tree's note.
// It returns {seen, amiss}: the boxes and words it measured, and for each told its part and what is amiss.
const measureWrap = `(() => {
  ` + scrollsJS + partOfJS + `
  let seen = 0;
  const w = document.documentElement.clientWidth, wide = [], amiss = [];
  // What is in a box that scrolls sideways may be wider than it.
  const scrolled = (e) => {
    for (let a = e.parentElement; a; a = a.parentElement) {
      if (scrolls(a)) return true;
    }
    return false;
  };
  for (const e of document.querySelectorAll('[data-floe-part], [data-floe-part] *')) {
    const s = getComputedStyle(e);
    if (scrolled(e) || scrolls(e) || s.display === 'inline' || e.closest('.floe-hidden, .floe-copy-status')) continue;
    seen++;
    if (e.scrollWidth > e.clientWidth + 1 || e.getBoundingClientRect().right > w + 1) wide.push(e);
  }
  for (const e of wide.filter((e) => !wide.some((d) => d !== e && e.contains(d)))) amiss.push({ at: partOf(e), text: 'overflows: ' + e.textContent.trim().slice(0, 40) });
  // A phone's emulated screen widens to the page, so the page is held to the width set, and each part's box too.
  const page = document.documentElement.scrollWidth;
  if (!wide.length && page > w) amiss.push({ at: 'page', text: 'the page is ' + page + ' px wide' });
  const whole = '[data-floe-part=keys] dd, [data-floe-part=code] .lang, [data-floe-part=diff] figcaption > :not(.file), td, [data-floe-part=tree] .note';
  for (const e of document.querySelectorAll(whole)) {
    const walk = document.createTreeWalker(e, NodeFilter.SHOW_TEXT);
    for (let n; (n = walk.nextNode()); ) {
      for (const m of n.data.matchAll(/[\w+−]{2,}/g)) {
        seen++;
        const r = document.createRange();
        r.setStart(n, m.index);
        r.setEnd(n, m.index + m[0].length);
        if (new Set([...r.getClientRects()].map((c) => Math.round(c.top))).size > 1) amiss.push({ at: partOf(e), text: 'breaks: ' + m[0] });
      }
    }
  }
  return JSON.stringify({ seen, amiss });
})()`

// onPhone lays s's pages out as wide as a phone of width px, till the test ends.
func onPhone(t *testing.T, s *site, width int64) {
	t.Helper()

	s.b.size(width, 844, true)
	t.Cleanup(s.b.unsize)
}

// laidOut fails t unless the page open is laid out width px wide, as a phone lays out a page whose viewport is the
// device's: with none, it lays the page out 980 px wide and finds nothing overflow. A page wider than the phone widens
// innerWidth alone, and the measures tell where.
func laidOut(t *testing.T, s *site, width int64) {
	t.Helper()

	if got := s.b.eval("String(document.documentElement.clientWidth)"); got != fmt.Sprint(width) {
		t.Fatalf("at %d px the page is laid out %s wide: its viewport is not the device's", width, got)
	}
}

// wrapThemes publishes wrapPage in every theme, lays each out as wide as a phone (390 px), and visits it.
func wrapThemes(t *testing.T, s *site, visit func(theme string)) {
	t.Helper()

	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	onPhone(t, s, 390)
	for _, theme := range themes {
		s.floe(t, "publish", s.write(t, "wrap-"+theme+".yaml", wrapPage+"theme: "+theme+"\n"), "--slug", "wrap-"+theme)
		s.b.navigate("/a/wrap-" + theme + "/?raw")
		laidOut(t, s, 390)
		visit(theme)
	}
}

const lookScrollWrapFix = "keep a part within its place: let a long path or URL break, or put what is wide in a box that scrolls " +
	"sideways where its place sets its width, not its content; keep a word whole in a narrow column"

// lookScrollWrap measures wrapPage (measureWrap) as wide as a phone (390 px), and as a narrow window (620 px), wider
// than where parts lay out for a phone: a path or a URL breaks rather than overflow its card, and no other word breaks
// for it.
func lookScrollWrap(t *testing.T, s *site) measured {
	t.Helper()

	s.floe(t, "publish", s.write(t, "wrap.yaml", wrapPage), "--slug", "wrap")
	var m measured
	for _, w := range []int64{390, 620} {
		onPhone(t, s, w)
		s.b.navigate("/a/wrap/?raw")
		laidOut(t, s, w)
		m.add(s.b.measure(fmt.Sprintf("wrapPage at %dpx", w), measureWrap))
	}
	return m
}

// brokenScrollWrap draws a part of one's own that keeps a long word on one line, wider than a phone.
func brokenScrollWrap(t *testing.T, s *site) (string, measured) {
	t.Helper()

	html := `<p data-floe-part="nowrap">` + longWord + `</p>`
	onPhone(t, s, 390)
	s.brokenPage(t, ownPart("nowrap", html, "@media screen { :scope { white-space: nowrap; } }\n"), htmlPage(html))
	laidOut(t, s, 390)
	return "nowrap", s.b.measure("broken", measureWrap)
}
