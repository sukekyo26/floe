package browsertest_test

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/chromedp"
)

const statsPage = `template: report
lang: ja
title: Stats
sections:
  - heading: A
    parts:
      - stats:
          - { label: One, value: '1.3', from: '3.1', unit: '%', better: lower }
          - { label: Two, value: '120', from: '100', better: lower }
          - { label: Three, value: '5', from: '5.0', better: lower }
          - { label: Four, value: '9', from: '12', better: lower, note: A note, tone: bad }
          - { label: Five, value: '1,840' }
          - { label: Six, value: '123456789012345678901234567890', from: '0.000000000000000001', unit: bytes, better: lower }
          - { label: Seven, value: '5', from: '5', better: lower, tone: bad }
`

// measureStats tells what no other test measures of the numbers before and after: the change's colour, whatever the
// card's tone, what is drawn in its card, and the number after as large as a number with none before it.
const measureStats = `(() => {
  const out = new Set();
  if (document.documentElement.scrollWidth > document.documentElement.clientWidth) out.add('the page scrolls sideways');
  const colour = (tone, card) => { const e = document.createElement('i'); e.dataset.floeTone = tone; e.style.color = 'var(--tone)'; card.append(e); const c = getComputedStyle(e).color; e.remove(); return c; };
  const cards = [...document.querySelectorAll('[data-floe-part=stats] .card')];
  const changes = { 0: 'good', 1: 'bad', 2: 'muted', 3: 'good', 5: 'bad', 6: 'muted' };
  for (const [i, tone] of Object.entries(changes)) {
    const change = cards[i] && cards[i].querySelector('.moved + .note');
    if (!change) { out.add('card ' + i + ' has no change'); continue; }
    if (getComputedStyle(change).color !== colour(tone, cards[i])) out.add(change.textContent + ' in ' + getComputedStyle(change).color + ', not ' + tone);
  }
  const big = getComputedStyle(cards[4].querySelector('.value')).fontSize;
  for (const c of cards) {
    const box = c.getBoundingClientRect();
    for (const e of c.querySelectorAll('dt, dd, dd span')) { if (e.matches('.floe-hidden')) continue; const r = e.getBoundingClientRect();
      if (r.left < box.left - 0.5 || r.right > box.right + 0.5) out.add(c.querySelector('dt').textContent + ' out of its card'); }
    const move = c.querySelector('.moved');
    if (!move) continue;
    if (getComputedStyle(move.lastElementChild).fontSize !== big) out.add('a number after of ' + getComputedStyle(move.lastElementChild).fontSize + ', not ' + big);
  }
  if (cards[4].querySelector('.moved, .from, .arrow')) out.add('a number with no from has what from draws');
  const note = [...cards[3].querySelectorAll('.note')].pop();
  if (getComputedStyle(note).color !== colour('bad', cards[3])) out.add('the note is not in its tone');
  return [...out].join('\n');
})()`

// TestRenderedStats draws the numbers before and after in every theme at the widths that turn how they wrap, and reads them
// as a screen reader is given them: before and after said, the arrow not.
func TestRenderedStats(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	themes, err := os.ReadDir("../kit/floe/themes")
	if err != nil {
		t.Fatal(err)
	}
	file := s.write(t, "stats.yaml", statsPage)
	amiss := make([]string, 0, len(themes))
	for _, th := range themes {
		s.floe(t, "publish", file, "--slug", "stats-"+th.Name(), "--theme", th.Name())
		b.navigate("/a/stats-" + th.Name() + "/?raw")
		for _, w := range []int64{1440, 400, 320} {
			b.size(w, 900, false)
			if got := b.eval(measureStats); got != "" {
				amiss = append(amiss, fmt.Sprintf("%s at %d:\n%s", th.Name(), w, got))
			}
		}
		b.unsize()
	}
	if len(amiss) > 0 {
		t.Error(strings.Join(amiss, "\n"))
	}
	b.navigate("/a/stats-default/?raw")
	tree, err := chromedp.Call(b.ctx, accessibility.GetFullAXTree, accessibility.GetFullAXTreeParams{})
	if err != nil {
		t.Fatal(err)
	}
	var spoken []string
	for _, n := range tree.Nodes {
		if n.Ignored || n.Role == nil || n.Name == nil {
			continue
		}
		var role, name string
		if json.Unmarshal(n.Role.Value, &role) == nil && role == "StaticText" && json.Unmarshal(n.Name.Value, &name) == nil {
			spoken = append(spoken, strings.TrimSpace(name))
		}
	}
	want := []string{"One", "前", "3.1", "後", "1.3", "%", "-1.8 良くなった", "Two"}
	if i := slices.Index(spoken, "One"); i < 0 || i+len(want) > len(spoken) || !slices.Equal(spoken[i:i+len(want)], want) {
		t.Errorf("read as %q, want %q in it", spoken, want)
	}
}
