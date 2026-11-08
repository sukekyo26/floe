package browsertest_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/kit"
)

// wrapPage holds a path and a URL in options, which have no place to break a line, words beside narrow
// columns (keys' text beside its keycaps, the language of code and the counts of a diff beside a long file
// name), a path in a table, in a tree's note (beside a deep tree) and in a text's Markdown table and code (and a long word in a table's cell), which scroll instead, and entries with a long URL for a title, a
// long path, a long pill and a long line of code in its text. A long block and a table also stand in an item of a
// list and in a quote, in a text, a text in details, a card of options and of a decision, a step and a tab, where an
// item of a grid or a flex row around them takes their width unless the box that scrolls tells nothing of it.
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

// scrollsJS defines scrolls, which tells an element that scrolls sideways, for the measures below.
const scrollsJS = `const scrolls = (e) => ['auto', 'scroll'].includes(getComputedStyle(e).overflowX);`

// measureWrap returns one line for a page wider than the window, one for each part's box, or a box in it, that is
// wider than its place or reaches past the window, out of those that scroll sideways, and one for each
// word that breaks across lines in keys' text, code's language, a diff's counts, a table and a tree's note.
const measureWrap = `(() => {
  ` + scrollsJS + `
  const out = [], w = innerWidth;
  // What is in a box that scrolls sideways may be wider than it.
  const scrolled = (e) => {
    for (let a = e.parentElement; a; a = a.parentElement) {
      if (scrolls(a)) return true;
    }
    return false;
  };
  // A phone's emulated screen widens to the page, so the page is held to the width set, and each part's box too.
  if (document.documentElement.scrollWidth > w) out.push('the page is ' + document.documentElement.scrollWidth + ' px wide');
  for (const e of document.querySelectorAll('[data-floe-part], [data-floe-part] *')) {
    const s = getComputedStyle(e);
    if (scrolled(e) || scrolls(e) || s.display === 'inline') continue;
    if (e.scrollWidth > e.clientWidth + 1 || e.getBoundingClientRect().right > w + 1) out.push('overflows: ' + e.textContent.trim().slice(0, 40));
  }
  const whole = '[data-floe-part=keys] dd, [data-floe-part=code] .lang, [data-floe-part=diff] figcaption > :not(.file), td, [data-floe-part=tree] .note';
  for (const e of document.querySelectorAll(whole)) {
    const walk = document.createTreeWalker(e, NodeFilter.SHOW_TEXT);
    for (let n; (n = walk.nextNode()); ) {
      for (const m of n.data.matchAll(/[\w+−]{2,}/g)) {
        const r = document.createRange();
        r.setStart(n, m.index);
        r.setEnd(n, m.index + m[0].length);
        if (new Set([...r.getClientRects()].map((c) => Math.round(c.top))).size > 1) out.push('breaks: ' + m[0]);
      }
    }
  }
  return out.join('\n');
})()`

// measureContain returns one line for each element that scrolls sideways and tells its content's width outside, with
// no contain of inline-size (size and strict hold it); a form's control scrolls by the browser's own sheet.
const measureContain = `(() => {
  ` + scrollsJS + `
  const out = [];
  for (const e of document.querySelectorAll('*')) {
    if (e.matches('textarea, select, input')) continue;
    if (!scrolls(e)) continue;
    if (!getComputedStyle(e).contain.split(' ').some((v) => ['inline-size', 'size', 'strict'].includes(v))) out.push(e.tagName.toLowerCase() + ' ' + e.textContent.trim().slice(0, 40));
  }
  return out.join('\n');
})()`

// measureScrollGround returns one line for each element with a ground (a background colour or image) in a box that scrolls
// sideways, and does, that ends left of what it holds: the right of its elements and text, what an inner box clips
// (one whose overflow-x is not visible, the element itself too) counted to that box's own right, and visually hidden
// text not counted.
// LIMIT: a frame (a border, --floe-inset's shadow), a ground of ::before or ::after and the ground of an element laid in
// a line (inline, inline-block and the like) are not measured; measure them too if a theme comes to paint them inside a box that scrolls.
const measureScrollGround = `(() => {
  ` + scrollsJS + `
  const out = [];
  const reach = (e) => {
    let far = e.getBoundingClientRect().right;
    // A box that clips, as a pre that scrolls in a table, shows nothing past its own right.
    if (getComputedStyle(e).overflowX !== 'visible') return far;
    const walk = document.createTreeWalker(e, NodeFilter.SHOW_ELEMENT | NodeFilter.SHOW_TEXT, (n) => {
      if (n.nodeType === Node.TEXT_NODE) return NodeFilter.FILTER_ACCEPT;
      if (n.matches('.floe-hidden')) return NodeFilter.FILTER_REJECT;
      // An inner box clips what reaches past it: its own right is what shows.
      if (getComputedStyle(n).overflowX !== 'visible') {
        far = Math.max(far, n.getBoundingClientRect().right);
        return NodeFilter.FILTER_REJECT;
      }
      return NodeFilter.FILTER_ACCEPT;
    });
    for (let n; (n = walk.nextNode()); ) {
      if (n.nodeType === Node.TEXT_NODE) {
        const r = document.createRange();
        r.selectNodeContents(n);
        for (const c of r.getClientRects()) far = Math.max(far, c.right);
      } else {
        far = Math.max(far, n.getBoundingClientRect().right);
      }
    }
    return far;
  };
  for (const box of document.querySelectorAll('*')) {
    if (!scrolls(box) || box.scrollWidth <= box.clientWidth) continue;
    for (const e of box.querySelectorAll('*')) {
      const s = getComputedStyle(e);
      if ((s.backgroundColor === 'rgba(0, 0, 0, 0)' && s.backgroundImage === 'none') || s.display.startsWith('inline')) continue;
      const right = e.getBoundingClientRect().right, far = reach(e);
      if (far > right + 1) out.push(e.tagName.toLowerCase() + ' in ' + (box.dataset.floePart || box.tagName.toLowerCase()) + ' ends ' + Math.round(far - right) + ' px short');
    }
  }
  return out.join('\n');
})()`

// lookScrollBox draws a page as wide as a phone (390 px), and as a narrow window (620 px), wider than where parts lay
// out for a phone: a path or a URL breaks rather than overflow its card, and no other word breaks for it. It draws the
// page in every theme too, as wide as a phone, to find a box that scrolls sideways and tells its width outside, and a
// ground in such a box that stops short of what it holds.
func lookScrollBox(t *testing.T) { //nolint:paralleltest,thelper // a subtest of TestLookRule; the browser tests share one Chrome
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.write(t, "wrap.yaml", wrapPage), "--slug", "wrap")
	for _, w := range []int64{390, 620} {
		b.size(w, 844, true)
		b.navigate("/a/wrap/?raw")
		// A phone's scrollbar is drawn over the page, so that the page is laid out as wide as the window, as measureWrap
		// takes it.
		if got, want := b.eval("innerWidth + ' ' + document.documentElement.clientWidth"), fmt.Sprintf("%d %d", w, w); got != want {
			t.Fatalf("at %d px the page is laid out %s wide", w, got)
		}
		if got := b.eval(measureWrap); got != "" {
			t.Errorf("at %d px:\n  %s", w, strings.ReplaceAll(got, "\n", "\n  "))
		}
	}
	// A box that scrolls sideways on screen tells nothing of its content's width outside, and a ground in it reaches to
	// the end of what it holds, in every theme's files too.
	b.size(390, 844, true)
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	for _, theme := range themes {
		s.floe(t, "publish", s.write(t, "wrap-"+theme+".yaml", wrapPage+"theme: "+theme+"\n"), "--slug", "wrap-"+theme)
		b.navigate("/a/wrap-" + theme + "/?raw")
		if got := b.eval(measureContain); got != "" {
			t.Errorf("%s: scrolls sideways with no contain: inline-size:\n  %s", theme, strings.ReplaceAll(got, "\n", "\n  "))
		}
		// The trees are there and scroll, or measureScrollGround finds nothing to measure.
		if got := b.eval("(() => { const trees = [...document.querySelectorAll('[data-floe-part=tree]')]; return trees.length > 0 && trees.every((e) => e.scrollWidth > e.clientWidth); })()"); got != "true" {
			t.Errorf("%s: wrapPage has no tree, or one that does not scroll sideways", theme)
			continue
		}
		if got := b.eval(measureScrollGround); got != "" {
			t.Errorf("%s: a ground stops short of what it holds in a box that scrolls:\n  %s", theme, strings.ReplaceAll(got, "\n", "\n  "))
		}
	}
}
