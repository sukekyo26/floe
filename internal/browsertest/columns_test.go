package browsertest_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/chromedp"
)

const columnsPage = `template: report
lang: en
title: Columns
sections:
  - heading: Two
    parts:
      - columns:
          - { title: Before, parts: [ { text: one } ] }
          - { title: After, parts: [ { text: two } ] }
  - heading: Three
    parts:
      - columns:
          - { title: A, parts: [ { text: one } ] }
          - { title: B, parts: [ { text: two } ] }
          - { parts: [ { text: three } ] }
`

// measureColumns returns a line for what stands amiss at this width: two columns fill their box side by side where
// both fit, a third wrapped keeps their width, and one column under another fills the box.
const measureColumns = `(() => {
  const probe = document.body.appendChild(document.createElement('div'));
  probe.style.width = 'var(--floe-room)';
  const room = probe.getBoundingClientRect().width;
  probe.remove();
  const out = [], near = (a, b) => Math.abs(a - b) < 0.5;
  const [two, three] = document.querySelectorAll('[data-floe-part=columns]');
  const cols = (c) => [...c.children].map((e) => e.getBoundingClientRect());
  const [a, b] = cols(two), root = two.getBoundingClientRect();
  if (innerWidth >= 1000) {
    if (!near(a.top, b.top)) out.push('two: tops ' + a.top + ' ' + b.top);
    if (!near(a.width, b.width)) out.push('two: widths ' + a.width + ' ' + b.width);
    if (!near(a.width + room + b.width, root.width)) out.push('two: ' + a.width + ' + ' + b.width + ' fill not ' + root.width);
  } else if (innerWidth >= 500) {
    const [p, , r] = cols(three);
    if (!(r.top > p.bottom - 0.5)) out.push('three: the third is not under the first');
    if (!near(p.width, r.width)) out.push('three: widths ' + p.width + ' ' + r.width);
  } else {
    if (!(b.top > a.bottom - 0.5)) out.push('narrow: not one under another');
    if (!near(b.width, root.width)) out.push('narrow: width ' + b.width + ' of ' + root.width);
  }
  if (document.documentElement.scrollWidth > innerWidth) out.push('the page is ' + document.documentElement.scrollWidth + ' wide on ' + innerWidth);
  return out.join('\n');
})()`

// TestUIColumnsPart measures how the part columns stands: side by side filling the box, of one width when they wrap, on
// screen and on paper's width, one under another on a phone, inside a screen narrower than a column's least width. It
// reads a column with a title as a screen reader is given it, a group named by the title; one without is no group.
func TestUIColumnsPart(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.write(t, "columns.yaml", columnsPage), "--slug", "columns")
	b.navigate("/a/columns/?raw")
	tree, err := chromedp.Call(b.ctx, accessibility.GetFullAXTree, accessibility.GetFullAXTreeParams{})
	if err != nil {
		t.Fatal(err)
	}
	var groups []string
	for _, n := range tree.Nodes {
		if n.Ignored || n.Role == nil || n.Name == nil {
			continue
		}
		var role, name string
		if json.Unmarshal(n.Role.Value, &role) != nil || json.Unmarshal(n.Name.Value, &name) != nil {
			continue
		}
		if role == "group" && name != "" {
			groups = append(groups, name)
		}
	}
	if got, want := strings.Join(groups, "|"), "Before|After|A|B"; got != want {
		t.Errorf("groups: %s; want %s", got, want)
	}
	defer b.unsize()
	for _, w := range []int64{1000, 620, 390, 240} {
		b.size(w, 900, false)
		b.navigate("/a/columns/?raw")
		if got := b.eval(measureColumns); got != "" {
			t.Errorf("at %d px:\n%s", w, got)
		}
	}
	b.size(paperWidth, 1000, false)
	b.media("print")
	b.navigate("/a/columns/?raw")
	if got := b.eval(measureColumns); got != "" {
		t.Errorf("on paper:\n%s", got)
	}
}
