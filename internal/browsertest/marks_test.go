package browsertest_test

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/chromedp"
)

// marksPage draws every part whose rows have a mark that is no letter, and the look numbered on items of html: plain,
// padded above, under a big title, starting with a block of no text, with a bigger mark, and with its text in a
// paragraph.
const marksPage = `template: report
lang: en
title: Marks
sections:
  - heading: Marks
    parts:
      - links:
          - { title: Docs, url: "https://example.com/docs", text: read }
      - note: { tone: good, title: Heads up, text: a note }
      - steps:
          - { title: One, text: first, state: done }
          - { title: Two, text: second }
      - timeline:
          - { date: "2026-10-01", title: Start, text: begun }
          - { date: "2026-10-02", title: End }
      - html: |
          <ol id="plain" data-floe-look="numbered"><li><strong>Title</strong><p>Text</p></li><li><strong>Next</strong></li></ol>
          <ol id="padded" data-floe-look="numbered"><li style="padding-top: 2rem"><strong>Title</strong><p>Text</p></li><li><strong>Next</strong></li></ol>
          <ol id="big" data-floe-look="numbered"><li><h3 style="font-size: 2rem">Big</h3><p>Text</p></li><li><p>Next</p></li></ol>
          <ol id="block" data-floe-look="numbered"><li><div style="height: 60px"></div><p>Under</p></li><li><p>Next</p></li></ol>
          <ol id="sized" data-floe-look="numbered"><li style="--floe-mark: 3rem"><strong>A</strong></li><li><strong>B</strong></li></ol>
          <ol id="wrapped" data-floe-look="numbered"><li><p>Restart <code>api</code> now</p></li><li><p>Next</p></li></ol>
          <div id="looknote" data-floe-look="note" data-floe-tone="warn"><strong class="title">Title</strong><p>Text</p></div>
`

// markRows are the rows whose mark stands beside a first line, with the element of that line.
var markRows = []struct{ name, row, line string }{
	{"links", "[data-floe-part=links] > li", "[data-floe-part=links] > li > a"},
	{"note", "[data-floe-part=note]", "[data-floe-part=note] > .title"},
	{"note borrowed", "#looknote", "#looknote > .title"},
	{"steps", "[data-floe-part=steps] > li", "[data-floe-part=steps] > li > strong"},
	{"timeline", "[data-floe-part=timeline] > li", "[data-floe-part=timeline] > li strong"},
	{"numbered", "#plain > li", "#plain > li > strong"},
	{"numbered padded", "#padded > li", "#padded > li > strong"},
	{"numbered big", "#big > li", "#big > li > h3"},
	{"numbered sized", "#sized > li", "#sized > li > strong"},
}

// lineRows are the first rows of each list whose line runs to the next mark.
var lineRows = []string{
	"[data-floe-part=steps] > li", "[data-floe-part=timeline] > li", "#plain > li", "#big > li", "#block > li", "#sized > li",
}

// measureMarks returns, as JSON, for each of markRows: the baseline of its first line, how far below its top a box
// drawn as its mark (its ::before's size, frame, font and text, a counter's as "1") has its baseline, the left of the
// element of its first line, the row's --floe-mark-gap and the left of the row's content box; the box of each of lineRows, the
// baselines of timeline's date and title, the lines the paragraph of #wrapped takes (its height in its line height)
// and its left, the root's font size, and the colours of steps' mark and line of a step done. A script reads no box
// of a pseudo-element, the mark's own place, which DevTools gives (pseudoBoxes).
const measureMarks = `((marks, rows) => {
  const zero = () => { const z = document.createElement('i'); z.style.cssText = 'display:inline-block;width:0;height:0'; return z; };
  const base = (el) => { const z = zero(); el.prepend(z); const y = z.getBoundingClientRect().bottom; z.remove(); return y; };
  const ascent = (row) => {
    const p = getComputedStyle(row, '::before');
    if (p.content === 'none') return null;
    const box = document.createElement('div'), mark = document.createElement('span'), z = zero();
    box.style.cssText = 'position:absolute;left:0;top:0';
    for (const k of ['boxSizing', 'width', 'height', 'paddingTop', 'paddingBottom', 'borderTopWidth', 'borderBottomWidth',
      'borderTopStyle', 'borderBottomStyle', 'overflow', 'alignItems', 'justifyItems', 'fontFamily', 'fontSize', 'fontWeight',
      'fontStyle', 'lineHeight']) mark.style[k] = p[k];
    mark.style.display = 'inline-grid';
    mark.textContent = (/^"([^"]*)"/.exec(p.content) || [, '1'])[1];
    box.append(mark, z);
    document.body.append(box);
    const a = z.getBoundingClientRect().bottom - mark.getBoundingClientRect().top;
    box.remove();
    return a;
  };
  const length = (el, v) => { const d = document.createElement('i'); d.style.cssText = 'display:block;position:absolute;width:' + v;
    el.append(d); const w = d.getBoundingClientRect().width; d.remove(); return w; };
  const box = (sel) => { const r = document.querySelector(sel).getBoundingClientRect(); return [r.left, r.top, r.right, r.bottom]; };
  const p = document.querySelector('#wrapped p');
  const done = document.querySelector('[data-floe-part=steps] > li.done');
  return JSON.stringify({
    marks: marks.map(([row, line]) => {
      const r = document.querySelector(row), l = document.querySelector(line), s = getComputedStyle(r);
      return { base: base(l), ascent: ascent(r), left: l.getBoundingClientRect().left, gap: length(r, 'var(--floe-mark-gap)'),
        content: r.getBoundingClientRect().left + parseFloat(s.borderLeftWidth) + parseFloat(s.paddingLeft) };
    }),
    rows: rows.map(box),
    date: base(document.querySelector('[data-floe-part=timeline] > li .date')) - base(document.querySelector('[data-floe-part=timeline] > li strong')),
    wrapped: Math.round(p.getBoundingClientRect().height / parseFloat(getComputedStyle(p).lineHeight)), wrappedLeft: p.getBoundingClientRect().left,
    rem: parseFloat(getComputedStyle(document.documentElement).fontSize),
    done: [getComputedStyle(done, '::before').backgroundColor, getComputedStyle(done, '::after').backgroundColor] });
})`

// pseudoBoxes returns the border boxes of the ::before and ::after of the first element sel matches, which a
// script cannot read: DevTools lays them out as nodes of their own.
func (b *browser) pseudoBoxes(sel string) (before, after []float64) {
	b.t.Helper()

	depth, pierce := int64(-1), true
	doc, err := chromedp.Call(b.ctx, dom.GetDocument, dom.GetDocumentParams{Depth: &depth, Pierce: &pierce})
	if err != nil {
		b.t.Fatal(err)
	}
	q, err := chromedp.Call(b.ctx, dom.QuerySelector, dom.QuerySelectorParams{NodeID: doc.Root.NodeID, Selector: sel})
	if err != nil || q.NodeID == 0 {
		b.t.Fatalf("%s: not found: %v", sel, err)
	}
	var find func(n *cdp.Node) *cdp.Node
	find = func(n *cdp.Node) *cdp.Node {
		if n.NodeID == q.NodeID {
			return n
		}
		for _, c := range n.Children {
			if f := find(c); f != nil {
				return f
			}
		}
		return nil
	}
	box := func(id cdp.NodeID) []float64 {
		m, err := chromedp.Call(b.ctx, dom.GetBoxModel, dom.GetBoxModelParams{NodeID: id})
		if err != nil {
			b.t.Fatalf("%s: %v", sel, err)
		}
		q := m.Model.Border
		return []float64{q[0], q[1], q[4], q[5]}
	}
	for _, p := range find(doc.Root).PseudoElements {
		if p.PseudoType == cdp.PseudoTypeBefore {
			before = box(p.NodeID)
		}
		if p.PseudoType == cdp.PseudoTypeAfter {
			after = box(p.NodeID)
		}
	}
	return before, after
}

// marksIn measures the page open and returns what is amiss in it: a mark whose baseline is more than 0.6 px off that
// of the first line beside it, or whose text does not stand its --floe-mark-gap from it (within 0.5 px); a row with no
// mark (but a note, whose mark a theme may leave out) and one with none whose text does not start at its content box,
// as the mark's column takes no room; a line that does not start 0.35rem below its mark and end 0.35rem above its
// row's end, under its mark's middle; timeline's date off its title's baseline, unless narrow puts the date in a row
// of its own; the paragraph of #wrapped broken into lines or not right of its mark; a step done whose line is not in
// its mark's colour.
func (b *browser) marksIn(where string, narrow bool) []string {
	b.t.Helper()

	sels := make([][2]string, len(markRows))
	for i, r := range markRows {
		sels[i] = [2]string{r.row, r.line}
	}
	var m struct {
		Marks []struct {
			Base, Left, Gap, Content float64
			Ascent                   *float64
		}
		Rows                   [][]float64
		WrappedLeft, Rem, Date float64
		Wrapped                int
		Done                   []string
	}
	if err := json.Unmarshal([]byte(b.eval(measureMarks+"("+mustJSON(sels)+", "+mustJSON(lineRows)+")")), &m); err != nil {
		b.t.Fatal(err)
	}
	var amiss []string
	for i, r := range markRows {
		mark, _ := b.pseudoBoxes(r.row)
		got := m.Marks[i]
		switch {
		case mark == nil && !strings.HasPrefix(r.name, "note"):
			amiss = append(amiss, where+": "+r.name+" has no mark")
		case mark == nil:
			if math.Abs(got.Left-got.Content) > 0.5 {
				amiss = append(amiss, fmt.Sprintf("%s: %s with no mark has its text %.1f px into its box", where, r.name, got.Left-got.Content))
			}
		default:
			if off := mark[1] + *got.Ascent - got.Base; math.Abs(off) > 0.6 {
				amiss = append(amiss, fmt.Sprintf("%s: %s's mark %.1f px off its line's baseline", where, r.name, off))
			}
			if gap := got.Left - mark[2]; math.Abs(gap-got.Gap) > 0.5 {
				amiss = append(amiss, fmt.Sprintf("%s: %s's text %.1f px from its mark, not %.1f", where, r.name, gap, got.Gap))
			}
		}
	}
	gap := 0.35 * m.Rem
	for i, row := range lineRows {
		mark, line := b.pseudoBoxes(row)
		start, end := line[1]-mark[3], m.Rows[i][3]-line[3]
		mid := (line[0]+line[2])/2 - (mark[0]+mark[2])/2
		if math.Abs(start-gap) > 0.5 || math.Abs(end-gap) > 0.5 || math.Abs(mid) > 0.5 {
			amiss = append(amiss, fmt.Sprintf("%s: %s's line starts %.1f px below its mark, ends %.1f px above its end, %.1f px off its middle",
				where, row, start, end, mid))
		}
	}
	if !narrow && math.Abs(m.Date) > 0.5 {
		amiss = append(amiss, fmt.Sprintf("%s: timeline's date %.1f px off its title's baseline", where, m.Date))
	}
	if mark, _ := b.pseudoBoxes("#wrapped > li"); m.Wrapped != 1 || m.WrappedLeft <= mark[2] {
		amiss = append(amiss, fmt.Sprintf("%s: #wrapped's text in %d lines, from %.1f, its mark ending at %.1f", where, m.Wrapped, m.WrappedLeft, mark[2]))
	}
	if m.Done[0] != m.Done[1] {
		amiss = append(amiss, fmt.Sprintf("%s: a step done has its mark %s and its line %s", where, m.Done[0], m.Done[1]))
	}
	return amiss
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// TestUIMarks measures the marks that are no letter beside the first line of a row (links, note, steps, timeline and
// the look numbered), in every theme, in a theme of one's own that pads the rows above and sets their line height, and,
// for timeline, on a narrow screen too: each stands on its first line's baseline, wherever a theme or the row's
// padding puts that line, its --floe-mark-gap from the text, or takes no room when a theme leaves it out, and each line to the next mark runs from 0.35rem
// below its mark to 0.35rem above its row's end, however big the mark or the title and whatever the row starts with.
func TestUIMarks(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	themes, err := os.ReadDir("../kit/floe/themes")
	if err != nil {
		t.Fatal(err)
	}
	file := s.write(t, "marks.yaml", marksPage)
	amiss := make([]string, 0, len(themes))
	for _, theme := range themes {
		name := theme.Name()
		s.floe(t, "publish", file, "--slug", "marks-"+name, "--theme", name)
		b.navigate("/a/marks-" + name + "/?raw")
		amiss = append(amiss, b.marksIn(name, false)...)
	}
	// A theme of one's own pads the rows 2rem above, sets their line height 2.5, draws their first line bigger than the
	// mark's letters and moves the note's text off its mark.
	s.own(t, map[string]string{
		"themes/tall/links.css":    ":scope > li { padding-top: 2rem; line-height: 2.5; }\n:scope a { font-size: 1.6rem; }\n",
		"themes/tall/note.css":     ":scope { padding-top: 2rem; line-height: 2.5; --floe-mark-gap: 2rem; }\n.title { font-size: 1.6rem; }\n",
		"themes/tall/steps.css":    ":scope > li { padding-top: 2rem; line-height: 2.5; }\n:scope strong { font-size: 1.6rem; }\n",
		"themes/tall/timeline.css": ":scope > li { padding-top: 2rem; line-height: 2.5; }\n:scope strong { font-size: 1.6rem; }\n",
		"themes/tall/numbered.css": ":scope > li { padding-top: 2rem; line-height: 2.5; }\n:scope strong { font-size: 1.6rem; }\n",
	})
	s.floe(t, "publish", file, "--slug", "marks-tall", "--theme", "tall")
	b.navigate("/a/marks-tall/?raw")
	amiss = append(amiss, b.marksIn("tall", false)...)
	b.size(500, 900, false)
	b.navigate("/a/marks-default/?raw")
	amiss = append(amiss, b.marksIn("narrow", true)...)
	b.unsize()
	check(t, "marks", amiss, "")
}
