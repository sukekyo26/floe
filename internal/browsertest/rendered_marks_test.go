package browsertest_test

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"testing"

	"github.com/sukekyo26/floe/internal/kit"
)

// rollbackIn measures the runbook open and returns what is amiss in it: a ring or a line of the rollback's first step
// not in its row's tone (bad), as --tone or, in a theme that fills a tone in its own colour, --tone-fill.
func (b *browser) rollbackIn(where string) []string {
	b.t.Helper()

	got := b.eval(`(() => {
  const li = document.querySelector('.rollback .steps > li');
  const colour = (v) => { const d = document.createElement('i'); d.style.color = v; li.append(d); const c = getComputedStyle(d).color; d.remove(); return c; };
  return JSON.stringify([colour('var(--tone)'), colour('var(--tone-fill)'), getComputedStyle(li, '::before').backgroundColor,
    getComputedStyle(li, '::after').backgroundColor, colour('var(--bad)')]);
})()`)
	var c []string
	if err := json.Unmarshal([]byte(got), &c); err != nil {
		b.t.Fatal(err)
	}
	tone, fill, ring, line, bad := c[0], c[1], c[2], c[3], c[4]
	if tone != bad || (ring != tone && ring != fill) || line != ring {
		return []string{fmt.Sprintf("%s: the rollback's ring is %s and its line %s, its tone %s (fill %s), bad %s", where, ring, line, tone, fill, bad)}
	}
	return nil
}

// numbersIn measures the page open and returns what is amiss in it: a card's number (options' card, #sizedcards' card)
// not as big as --floe-mark, frame within, set on the part's root, unless a theme draws it as text (auto), hides it or
// draws another thing there (notebook's tape).
func (b *browser) numbersIn(where string) []string {
	b.t.Helper()

	var amiss []string
	for _, card := range []string{"[data-floe-part=options] > .card", "#sizedcards > .card"} {
		mark, _ := b.pseudoBoxes(card)
		size := b.eval(`((card) => { ` + pxJS + ` const e = document.querySelector(card), v = getComputedStyle(e, '::before').getPropertyValue('--floe-mark').trim();
  if (v === 'auto' || !getComputedStyle(e, '::before').content.includes('counter')) return 'auto'; return String(px(e, v)); })(` + mustJSON(card) + `)`)
		if mark == nil || size == "auto" {
			continue
		}
		want, err := strconv.ParseFloat(size, 64)
		if err != nil {
			amiss = append(amiss, fmt.Sprintf("%s: %s's number has --floe-mark %q", where, card, size))
			continue
		}
		if w, h := mark[2]-mark[0], mark[3]-mark[1]; math.Abs(w-want) > 0.5 || math.Abs(h-want) > 0.5 {
			amiss = append(amiss, fmt.Sprintf("%s: %s's number is %.1f by %.1f, not --floe-mark %.1f", where, card, w, h, want))
		}
	}
	return amiss
}

// marksLookIn measures the page open and returns what is amiss in it: timeline's date off its title's baseline, the
// paragraph of #wrapped broken into lines or not right of its mark, and a step done whose line is not in its mark's
// colour.
func (b *browser) marksLookIn(where string) []string {
	b.t.Helper()

	var got struct {
		Date, WrappedLeft float64
		Wrapped           int
		Done              []string
	}
	if err := json.Unmarshal([]byte(b.eval(`(() => {
  const base = (el) => { const z = document.createElement('i'); z.style.cssText = 'display:inline-block;width:0;height:0'; el.prepend(z); const y = z.getBoundingClientRect().bottom; z.remove(); return y; };
  const p = document.querySelector('#wrapped p'), done = document.querySelector('[data-floe-part=steps] > li.done');
  return JSON.stringify({ date: base(document.querySelector('[data-floe-part=timeline] > li .date')) - base(document.querySelector('[data-floe-part=timeline] > li strong')),
    wrapped: Math.round(p.getBoundingClientRect().height / parseFloat(getComputedStyle(p).lineHeight)), wrappedLeft: p.getBoundingClientRect().left,
    done: [getComputedStyle(done, '::before').backgroundColor, getComputedStyle(done, '::after').backgroundColor] });
})()`)), &got); err != nil {
		b.t.Fatal(err)
	}
	var amiss []string
	if math.Abs(got.Date) > 0.5 {
		amiss = append(amiss, fmt.Sprintf("%s: timeline's date %.1f px off its title's baseline", where, got.Date))
	}
	if mark, _ := b.pseudoBoxes("#wrapped > li"); got.Wrapped != 1 || mark == nil || got.WrappedLeft <= mark[2] {
		amiss = append(amiss, fmt.Sprintf("%s: #wrapped's text in %d lines, from %.1f, its mark %v", where, got.Wrapped, got.WrappedLeft, mark))
	}
	if got.Done[0] != got.Done[1] {
		amiss = append(amiss, fmt.Sprintf("%s: a step done has its mark %s and its line %s", where, got.Done[0], got.Done[1]))
	}
	return amiss
}

// TestRenderedMarks draws marksPage and runbook's example in every theme: a card's number is as big as --floe-mark
// (numbersIn), timeline's date, #wrapped and a step done are drawn as marksLookIn says, and the runbook's rollback rings
// and lines its steps in bad (rollbackIn).
func TestRenderedMarks(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	file := s.write(t, "marks.yaml", marksPage)
	example, err := os.ReadFile("../kit/floe/templates/runbook/example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	runbook := s.write(t, "runbook.yaml", string(example))
	amiss := make([]string, 0, len(themes))
	for _, theme := range themes {
		s.floe(t, "publish", file, "--slug", "marks-"+theme, "--theme", theme)
		s.b.navigate("/a/marks-" + theme + "/?raw")
		amiss = append(amiss, s.b.numbersIn(theme)...)
		amiss = append(amiss, s.b.marksLookIn(theme)...)
		s.floe(t, "publish", runbook, "--slug", "runbook-"+theme, "--theme", theme)
		s.b.navigate("/a/runbook-" + theme + "/?raw")
		amiss = append(amiss, s.b.rollbackIn(theme)...)
	}
	for _, a := range amiss {
		t.Error(a)
	}
}
