package browsertest_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/chromedp"

	"github.com/sukekyo26/floe/internal/kit"
)

// TestUIAsksHead reads the count of questions at the head of a handoff: of those still to decide, in the warning's colour
// only while there is one, with the link to the section as long as the section stands.
func TestUIAsksHead(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	for _, c := range []struct{ asks, want string }{
		{"asks:\n  - { question: a, decided: x }\n  - { question: b, options: [x, y], decided: y }\n", "0 link"},
		{"asks:\n  - { question: a, decided: x }\n  - { question: b }\n", "1 link hot"},
		{"", "0"},
	} {
		page := "template: handoff\nlang: en\ntitle: Head\ndate: 2030-01-01\n" + c.asks
		s.floe(t, "publish", s.write(t, "head.yaml", page), "--slug", "head", "--force")
		s.b.navigate("/a/head/?raw")
		got := s.b.eval(`(() => {
  const li = document.querySelectorAll('.counts li')[2];
  return li.querySelector('b').textContent + (li.querySelector('a[href="#asks"]') ? ' link' : '') + (li.matches('.hot[data-floe-tone=warn]') ? ' hot' : '');
})()`)
		if got != c.want {
			t.Errorf("asks %q: %q; want %q", c.asks, got, c.want)
		}
	}
}

// TestUIAsksMark reads a question decided as a screen reader is given it: its check by the word for it, in a question with no
// option to bear the word too.
func TestUIAsksMark(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	const page = "template: report\nlang: en\ntitle: Mark\nsections:\n  - heading: Mark\n    parts:\n      - asks:\n" +
		"          - { question: q, decided: done }\n          - { question: r }\n"
	s.floe(t, "publish", s.write(t, "mark.yaml", page), "--slug", "mark")
	s.b.navigate("/a/mark/?raw")
	tree, err := chromedp.Call(s.b.ctx, accessibility.GetFullAXTree, accessibility.GetFullAXTreeParams{})
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, n := range tree.Nodes {
		if n.Ignored || n.Name == nil {
			continue
		}
		var name string
		if json.Unmarshal(n.Name.Value, &name) == nil {
			counts[strings.TrimSpace(name)]++
		}
	}
	if counts["Decided"] == 0 {
		t.Errorf("read: Decided %d times; want the word", counts["Decided"])
	}
}

// TestUIAsksPills measures the pills beside an option's words, in every theme and as wide as a phone: their middles level, but for a pill folded under the words; a question's check at the right, clear of a long title, its sign in the middle of its circle.
func TestUIAsksPills(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	asks := "template: report\nlang: en\ntitle: Pills\nsections:\n  - heading: Pills\n    parts:\n      - asks:\n" +
		"          - { question: q, options: [a, 'b `c`'], recommend: 'b `c`', decided: 'b `c`' }\n" +
		"          - { question: " + strings.Repeat("a question long enough to fold ", 12) + ", decided: done }\n"
	defer s.b.unsize()
	for _, theme := range themes {
		s.floe(t, "publish", s.write(t, "pills.yaml", asks), "--slug", "pills-"+theme, "--theme", theme)
		for _, w := range []int64{1000, 390} {
			s.b.size(w, 800, false)
			s.b.navigate("/a/pills-" + theme + "/?raw")
			got := s.b.eval(`(() => {
  const middle = (e) => { const r = e.getBoundingClientRect(); return r.top + r.height / 2; };
  const pills = [...document.querySelectorAll('[data-floe-part=asks] .choices [data-floe-part=cell]')];
  if (pills.length !== 2) return pills.length + ' pills';
  for (const c of document.querySelectorAll('[data-floe-part=asks] .check')) {
    const title = c.previousElementSibling.getBoundingClientRect(), r = c.getBoundingClientRect(), card = c.closest('.card');
    const right = card.getBoundingClientRect().right - parseFloat(getComputedStyle(card).paddingRight) - parseFloat(getComputedStyle(card).borderRightWidth);
    if (r.left < title.right) return 'the check stands over the title';
    if (Math.abs(r.right - right) > 1.5) return 'the check stands ' + (right - r.right) + ' px off the right';
    const sign = c.querySelector('path').getBoundingClientRect();
    const dx = sign.left + sign.width / 2 - (r.left + r.width / 2), dy = sign.top + sign.height / 2 - (r.top + r.height / 2);
    if (Math.abs(dx) > 0.75 || Math.abs(dy) > 0.75) return 'the sign stands off its circle by ' + dx + ' ' + dy;
  }
  const beside = pills.filter((p) => p.getBoundingClientRect().top < p.parentElement.firstElementChild.getBoundingClientRect().bottom);
  const off = beside.map((p) => middle(p) - middle(p.parentElement.firstElementChild)).filter((d) => Math.abs(d) > 0.75);
  return off.length ? 'a pill stands off the words by ' + off.join(' ') : '';
})()`)
			if got != "" {
				t.Errorf("%s at %d px: %s", theme, w, got)
			}
		}
	}
}
