package browsertest_test

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/chromedp"

	"github.com/sukekyo26/floe/internal/kit"
)

// checksPage holds of each result another count, with what cannot fold in each place a check holds it: a title and a how
// of no break, a line of code, a table wider than a phone.
var checksPage = `template: report
lang: en
title: Checks
sections:
  - heading: Checks
    parts:
      - checks:
          - { title: Short, result: ok, actual: It held., how: By hand }
          - title: ` + strings.Repeat("unbroken", 30) + `
            result: ng
            actual: Text of ` + strings.Repeat("unbroken", 30) + `
            how: ` + strings.Repeat("unbroken", 30) + `
          - title: A title long enough to fold onto more than one line where it stands beside the pill that says what came of it
            result: ok
            actual: |
              A list:

              - of two
              - items
          - title: Wide code
            result: ng
            actual: It scrolls.
            parts:
              - code: { lang: sh, text: "wiki import ` + strings.Repeat("--a-long-flag ", 30) + `\n" }
          - title: A wide table
            result: ok
            actual: It scrolls.
            parts:
              - table:
                  columns: [` + strings.TrimSuffix(strings.Repeat("a-long-column-name, ", 6), ", ") + `]
                  rows:
                    - [` + strings.TrimSuffix(strings.Repeat("a-long-cell-of-text, ", 6), ", ") + `]
            how: In the browser
          - title: More that cannot fold
            result: ok
            actual: Each in its own way.
            parts:
              - terminal: { shell: bash, steps: [ { run: "wiki import ` + strings.Repeat("--a-long-flag ", 30) + `", out: "` + strings.Repeat("output", 60) + `" } ] }
              - diff: { text: "@@ -1 +1 @@\n-` + strings.Repeat("old", 90) + `\n+` + strings.Repeat("new", 90) + `\n" }
              - image: { file: wide.png, alt: A wide shot }
              - note: { tone: warn, title: A note, text: Held in a check. }
              - diagram: { width: 2000, height: 200, title: A wide diagram, parts: [ { box: { x: 20, y: 40, w: 160, h: 60, rows: [one] } } ] }
              - entries:
                  - { title: ` + strings.Repeat("unbroken", 30) + `, tags: [` + strings.Repeat("tag", 40) + `], text: ` + strings.Repeat("word", 60) + ` }
          - { title: Not yet, result: unchecked, actual: Not from here. }
`

// measureChecks returns a line for what stands amiss in the part: the counts, large over their words, each in its result's tone; a thing wider than its row or than the
// page, or a part in a check not as wide as its row; a pill over its title, or off the middle of its first line; the room around the rows; the line, the height of its
// row at its left and of the row's tone, each result's its own, its pill's the same.
const measureChecks = `(() => {
  const out = [], near = (a, b) => Math.abs(a - b) < 0.75;
  const probe = document.body.appendChild(document.createElement('div'));
  probe.style.width = 'var(--floe-room)';
  const room = probe.getBoundingClientRect().width;
  probe.remove();
  const root = document.querySelector('[data-floe-part=checks]'), total = root.querySelector('.total'), list = root.querySelector('ul');
  if (total.textContent !== '4 Pass 2 Fail 1 Not checked') out.push('total: ' + total.textContent);
  if (!near(list.getBoundingClientRect().top - total.getBoundingClientRect().bottom, room)) out.push('room under the total');
  const ls = getComputedStyle(list);
  if (ls.listStyleType !== 'none' || parseFloat(ls.paddingLeft) !== 0) out.push('list: ' + ls.listStyleType + ' ' + ls.paddingLeft);
  const rs = getComputedStyle(root), inside = root.clientWidth - parseFloat(rs.paddingLeft) - parseFloat(rs.paddingRight);
  if (!near(list.getBoundingClientRect().width, inside)) out.push('the list is ' + list.getBoundingClientRect().width + ' wide in ' + inside);
  const rows = [...list.children], tones = {};
  rows.forEach((row, i) => {
    const r = row.getBoundingClientRect(), line = getComputedStyle(row, '::before'), pill = row.querySelector('[data-floe-part=cell]'), title = row.querySelector('.title');
    if (i && !near(r.top - rows[i - 1].getBoundingClientRect().bottom, room)) out.push('row ' + i + ': room above');
    if (!near(r.width, list.getBoundingClientRect().width)) out.push('row ' + i + ': ' + r.width + ' wide in ' + list.getBoundingClientRect().width);
    for (const e of row.querySelectorAll(':scope > *, .title, .how > *')) if (e.getBoundingClientRect().right > r.right + 0.5) out.push('row ' + i + ': ' + e.className + ' wider than the row');
    const inner = r.width - parseFloat(getComputedStyle(row).paddingLeft);
    for (const e of row.querySelectorAll('.parts > [data-floe-part=stack] > *')) {
      const max = parseFloat(getComputedStyle(e).maxWidth);
      if (!near(e.getBoundingClientRect().width, Math.min(inner, max || inner))) out.push('row ' + i + ': ' + (e.dataset.floePart || e.tagName) + ' is ' + e.getBoundingClientRect().width + ' wide in ' + inner);
    }
    if (pill.getBoundingClientRect().right > title.getBoundingClientRect().left + 0.5) out.push('row ' + i + ': the pill stands over the title');
    if (!(parseFloat(line.width) > 0) || pill.getBoundingClientRect().left <= r.left + parseFloat(line.width)) out.push('row ' + i + ': the line ' + line.width + ' wide, the pill ' + (pill.getBoundingClientRect().left - r.left) + ' from the row\'s left');
    const stack = row.querySelector('.parts [data-floe-part=stack]'), how = row.querySelector('.how');
    if (stack && how && !near(how.getBoundingClientRect().top - stack.getBoundingClientRect().bottom, room)) out.push('row ' + i + ': room over how');
    if (stack && !how && !near(r.bottom, stack.getBoundingClientRect().bottom)) out.push('row ' + i + ': room under the parts');
    const line1 = document.createRange(); line1.selectNodeContents(title);
    const first = line1.getClientRects()[0], p = pill.getBoundingClientRect(), off = p.top + p.height / 2 - (first.top + first.height / 2);
    if (Math.abs(off) > 1) out.push('row ' + i + ': the pill off the title by ' + off);
    if (!near(parseFloat(line.height), r.height) || parseFloat(line.top) !== 0 || parseFloat(line.left) !== 0 || line.position !== 'absolute') out.push('row ' + i + ': line ' + line.height + ' of ' + r.height + ' at ' + line.top + ' ' + line.left);
    if (getComputedStyle(row).getPropertyValue('--tone') !== getComputedStyle(pill).getPropertyValue('--tone')) out.push('row ' + i + ': the pill is not of the line\'s tone');
    tones[row.dataset.floeTone] = line.backgroundColor;
  });
  const body = parseFloat(getComputedStyle(rows[0]).fontSize);
  for (const c of total.children) {
    const n = getComputedStyle(c.querySelector('b'));
    if (n.color !== tones[c.dataset.floeTone]) out.push('the count of ' + c.dataset.floeTone + ': ' + n.color + ', its lines ' + tones[c.dataset.floeTone]);
    if (!(parseFloat(n.fontSize) > body)) out.push('the count of ' + c.dataset.floeTone + ' is ' + n.fontSize + ' in ' + body);
    if (c.querySelector('b').getBoundingClientRect().bottom > c.getBoundingClientRect().bottom - 2) out.push('the count of ' + c.dataset.floeTone + ' has no word under it');
  }
  if (Object.keys(tones).length !== 3 || new Set(Object.values(tones)).size !== 3) out.push('the lines: ' + JSON.stringify(tones));
  if (document.documentElement.scrollWidth > innerWidth) out.push('the page is ' + document.documentElement.scrollWidth + ' wide on ' + innerWidth);
  return out.join('\n');
})()`

// TestRenderedChecksPart measures how the part checks stands, in every theme, on a screen and as wide as a phone, and reads its
// rows as a screen reader is given them: one list of as many items as checks.
func TestRenderedChecksPart(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	var shot bytes.Buffer
	if err = png.Encode(&shot, image.NewGray(image.Rect(0, 0, 3000, 20))); err != nil {
		t.Fatal(err)
	}
	s.write(t, "wide.png", shot.String())
	file := s.write(t, "checks.yaml", checksPage)
	defer s.b.unsize()
	for _, theme := range themes {
		s.floe(t, "publish", file, "--slug", "checks-"+theme, "--theme", theme)
		for _, w := range []int64{1000, 390, 240} {
			s.b.size(w, 800, false)
			s.b.navigate("/a/checks-" + theme + "/?raw")
			if got := s.b.eval(measureChecks); got != "" {
				t.Errorf("%s at %d px:\n%s", theme, w, got)
			}
		}
		s.b.size(paperWidth, 1000, false)
		s.b.media("print")
		s.b.navigate("/a/checks-" + theme + "/?raw")
		if got := s.b.eval(measureChecks); got != "" {
			t.Errorf("%s on paper:\n%s", theme, got)
		}
		s.b.media("screen")
	}
	tree, err := chromedp.Call(s.b.ctx, accessibility.GetFullAXTree, accessibility.GetFullAXTreeParams{})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[accessibility.NodeID]*accessibility.Node{}
	for _, n := range tree.Nodes {
		byID[n.NodeID] = n
	}
	role := func(n *accessibility.Node) string {
		if n.Role == nil || n.Ignored {
			return ""
		}
		return strings.Trim(string(n.Role.Value), `"`)
	}
	var lists []int
	for _, n := range tree.Nodes {
		if role(n) != "list" {
			continue
		}
		items := 0
		for _, c := range n.ChildIDs {
			if k, ok := byID[c]; ok && role(k) == "listitem" {
				items++
			}
		}
		lists = append(lists, items)
	}
	if len(lists) != 2 || lists[0] != 7 || lists[1] != 2 {
		t.Errorf("lists of %v items; want the 7 rows, then the 2 of the Markdown in one", lists)
	}
}
