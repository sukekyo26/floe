package browsertest_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/chromedp"

	"github.com/sukekyo26/floe/internal/kit"
)

// marksPage draws every part whose rows have a mark, a letter's (decision, options, checklist) too, and the look
// numbered on items of html: plain, padded above, under a big title, starting with a block of no text, with a bigger
// mark, and with its text in a paragraph.
const marksPage = `template: report
lang: en
title: Marks
sections:
  - heading: Marks
    parts:
      - links:
          - { title: Docs, url: "https://example.com/docs", text: read }
      - note: { tone: good, title: Heads up, text: a note }
      - note: { tone: warn, text: "A note of Markdown alone.\n\nA second paragraph." }
      - steps:
          - { title: One, text: first, state: done }
          - { title: Two, text: second }
      - timeline:
          - { date: "2026-10-01", title: Start, text: begun }
          - { date: "2026-10-02", title: End }
      - options:
          - { title: One, text: first, pros: ["fast, with ` + "`code`" + ` in it"], cons: [costly] }
          - { title: Two, text: second }
      - decision: { title: Take one, rejected: ["the other, too slow"] }
      - checklist:
          - { text: open it, done: true }
          - { text: close it }
      - html: |
          <ol id="plain" data-floe-look="numbered"><li><strong>Title</strong><p>Text</p></li><li><strong>Next</strong></li></ol>
          <ol id="padded" data-floe-look="numbered"><li style="padding-top: 2rem"><strong>Title</strong><p>Text</p></li><li><strong>Next</strong></li></ol>
          <ol id="big" data-floe-look="numbered"><li><h3 style="font-size: 2rem">Big</h3><p>Text</p></li><li><p>Next</p></li></ol>
          <ol id="block" data-floe-look="numbered"><li><div style="height: 60px"></div><p>Under</p></li><li><p>Next</p></li></ol>
          <ol id="sized" data-floe-look="numbered"><li style="--floe-mark: 3rem"><strong>A</strong></li><li><strong>B</strong></li></ol>
          <ol id="wrapped" data-floe-look="numbered"><li><p>Restart <code>api</code> now</p></li><li><p>Next</p></li></ol>
          <div id="plainnote" data-floe-part="note" data-floe-tone="warn"><p>Text alone</p></div>
          <ol data-floe-look="numbered"><li><p>A step</p><div id="stepnote" data-floe-part="note" data-floe-tone="bad"><strong class="title">If it fails</strong><p>Text</p></div></li></ol>
          <div id="sizedcards" class="numbered" data-floe-look="cards"><div class="card" style="--floe-mark: 3rem" data-floe-look="panel"><p>A</p></div></div>
`

// markRows are the rows whose mark stands beside a first line, with the element of that line and the part or look
// they are of (at); bare when a theme may leave the mark out.
var markRows = []struct {
	name, at, row, line string
	bare                bool
}{
	{"links", "links", "[data-floe-part=links] > li", "[data-floe-part=links] > li > .line > .title > a", false},
	{"note", "note", "[data-floe-part=note]", "[data-floe-part=note] > .title", true},
	{"note without a title", "note", "#plainnote", "#plainnote > p", true},
	{
		"note of Markdown without a title", "note", "[data-floe-part=note]:not(:has(> .title)):not(#plainnote)",
		"[data-floe-part=note]:not(:has(> .title)):not(#plainnote) > [data-floe-text] > p", true,
	},
	{"note in a step", "note", "#stepnote", "#stepnote > .title", true},
	{"steps", "steps", "[data-floe-part=steps] > li", "[data-floe-part=steps] > li > strong", false},
	{"timeline", "timeline", "[data-floe-part=timeline] > li", "[data-floe-part=timeline] > li strong", false},
	{"numbered", "numbered", "#plain > li", "#plain > li > strong", false},
	{"numbered padded", "numbered", "#padded > li", "#padded > li > strong", false},
	{"numbered big", "numbered", "#big > li", "#big > li > h3", false},
	{"numbered sized", "numbered", "#sized > li", "#sized > li > strong", false},
	{"decision", "decision", "[data-floe-part=decision] .rejected > li", "[data-floe-part=decision] .rejected > li > span", true},
	{"options", "options", "[data-floe-part=options] .points > li", "[data-floe-part=options] .points > li > span", true},
	{"checklist", "checklist", "[data-floe-part=checklist] > li", "[data-floe-part=checklist] > li > span", true},
}

// lineRows are the first rows of each list whose line runs to the next mark, with the part or look they are of.
var lineRows = []struct{ at, row string }{
	{"steps", "[data-floe-part=steps] > li"},
	{"timeline", "[data-floe-part=timeline] > li"},
	{"numbered", "#plain > li"},
	{"numbered", "#big > li"},
	{"numbered", "#block > li"},
	{"numbered", "#sized > li"},
}

// measureMarks returns, as JSON, for each of markRows: the baseline of its first line, how far below its top a box
// drawn as its mark (its ::before's size, frame, font and text, a counter's as "1") has its baseline, the left of the
// element of its first line, the row's --floe-mark-gap and the left of the row's content box; the box of each of lineRows, the
// baselines of timeline's date and title, the lines the paragraph of #wrapped takes (its height in its line height)
// and its left, the root's font size, and the colours of steps' mark and line of a step done. Each is null where the
// page does not draw what it measures. A script reads no box of a pseudo-element, the mark's own place, which DevTools
// gives (pseudoBoxes).
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
  const box = (sel) => { const e = document.querySelector(sel); if (!e) return null; const r = e.getBoundingClientRect(); return [r.left, r.top, r.right, r.bottom]; };
  const p = document.querySelector('#wrapped p');
  const done = document.querySelector('[data-floe-part=steps] > li.done');
  const date = document.querySelector('[data-floe-part=timeline] > li .date'), title = document.querySelector('[data-floe-part=timeline] > li strong');
  return JSON.stringify({
    marks: marks.map(([row, line]) => {
      const r = document.querySelector(row), l = document.querySelector(line);
      if (!r || !l) return null;
      const s = getComputedStyle(r), size = getComputedStyle(r, '::before').getPropertyValue('--floe-mark').trim();
      return { base: base(l), ascent: ascent(r), left: l.getBoundingClientRect().left, gap: length(r, 'var(--floe-mark-gap)'),
        size: size === 'auto' ? null : length(r, 'var(--floe-mark)'),
        content: r.getBoundingClientRect().left + parseFloat(s.borderLeftWidth) + parseFloat(s.paddingLeft) };
    }),
    rows: rows.map(box),
    date: date && title ? base(date) - base(title) : null,
    wrapped: p ? Math.round(p.getBoundingClientRect().height / parseFloat(getComputedStyle(p).lineHeight)) : null, wrappedLeft: p ? p.getBoundingClientRect().left : 0,
    rem: parseFloat(getComputedStyle(document.documentElement).fontSize),
    done: done ? [getComputedStyle(done, '::before').backgroundColor, getComputedStyle(done, '::after').backgroundColor] : null });
})`

// pseudoBoxes returns the border boxes of the ::before and ::after of the first element sel matches, which a
// script cannot read: DevTools lays them out as nodes of their own.
func (b *browser) pseudoBoxes(sel string) (before, after []float64) {
	b.t.Helper()

	// Just after a page loads, DevTools may still replace the document, and the nodes got of the one before are
	// gone: measure again.
	var err error
	for range 20 {
		if before, after, err = b.tryPseudoBoxes(sel); err == nil {
			return before, after
		}
		time.Sleep(50 * time.Millisecond)
	}
	b.t.Fatalf("%s: %v", sel, err)
	return nil, nil
}

func (b *browser) tryPseudoBoxes(sel string) (before, after []float64, err error) {
	depth, pierce := int64(-1), true
	doc, err := chromedp.Call(b.ctx, dom.GetDocument, dom.GetDocumentParams{Depth: &depth, Pierce: &pierce})
	if err != nil {
		return nil, nil, err
	}
	q, err := chromedp.Call(b.ctx, dom.QuerySelector, dom.QuerySelectorParams{NodeID: doc.Root.NodeID, Selector: sel})
	if err != nil {
		return nil, nil, err
	}
	if q.NodeID == 0 {
		return nil, nil, errors.New("not found")
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
	for _, p := range find(doc.Root).PseudoElements {
		m, err := chromedp.Call(b.ctx, dom.GetBoxModel, dom.GetBoxModelParams{NodeID: p.NodeID})
		if err != nil {
			return nil, nil, err
		}
		q := m.Model.Border
		if p.PseudoType == cdp.PseudoTypeBefore {
			before = []float64{q[0], q[1], q[4], q[5]}
		}
		if p.PseudoType == cdp.PseudoTypeAfter {
			after = []float64{q[0], q[1], q[4], q[5]}
		}
	}
	return before, after, nil
}

// marksIn measures the page open, told as where, and returns what is amiss in it, each at the part or look of its row:
// a mark whose baseline is more than 0.6 px off that of the first line beside it, or whose text does not stand its
// --floe-mark-gap from it (within 0.5 px), or not as wide as its --floe-mark; a row with no mark (but a note, whose mark
// a theme may leave out) and one with none whose text does not start at its content box, as the mark's column takes no
// room; a line that does not start 0.35rem below its mark and end 0.35rem above its row's end, under its mark's middle;
// timeline's date off its title's baseline, unless narrow puts the date in a row of its own; the paragraph of #wrapped
// broken into lines or not right of its mark; a step done whose line is not in its mark's colour. It measures what the
// page draws of markRows and lineRows; whole tells each the page does not draw as amiss too.
func (b *browser) marksIn(where string, narrow, whole bool) measured {
	b.t.Helper()

	sels := make([][2]string, len(markRows))
	for i, r := range markRows {
		sels[i] = [2]string{r.row, r.line}
	}
	lines := make([]string, len(lineRows))
	for i, r := range lineRows {
		lines[i] = r.row
	}
	var got struct {
		Marks []*struct {
			Base, Left, Gap, Content float64
			Ascent, Size             *float64
		}
		Rows        [][]float64
		WrappedLeft float64
		Rem         float64
		Date        *float64
		Wrapped     *int
		Done        []string
	}
	if err := json.Unmarshal([]byte(b.eval(measureMarks+"("+mustJSON(sels)+", "+mustJSON(lines)+")")), &got); err != nil {
		b.t.Fatal(err)
	}
	var m measured
	add := func(at, format string, args ...any) {
		m.amiss = append(m.amiss, amiss{Page: where, At: at, Text: fmt.Sprintf(format, args...)})
	}
	for i, r := range markRows {
		g := got.Marks[i]
		if g == nil {
			if whole {
				add(r.at, "%s not drawn: marksPage draws it", r.name)
			}
			continue
		}
		m.seen++
		mark, _ := b.pseudoBoxes(r.row)
		switch {
		case mark == nil && !r.bare:
			add(r.at, "%s has no mark", r.name)
		case mark == nil:
			if math.Abs(g.Left-g.Content) > 0.5 {
				add(r.at, "%s with no mark has its text %.1f px into its box", r.name, g.Left-g.Content)
			}
		default:
			if off := mark[1] + *g.Ascent - g.Base; math.Abs(off) > 0.6 {
				add(r.at, "%s's mark %.1f px off its line's baseline", r.name, off)
			}
			if gap := g.Left - mark[2]; math.Abs(gap-g.Gap) > 0.5 {
				add(r.at, "%s's text %.1f px from its mark, not %.1f", r.name, gap, g.Gap)
			}
			if w := mark[2] - mark[0]; g.Size != nil && math.Abs(w-*g.Size) > 0.5 {
				add(r.at, "%s's mark %.1f px wide, not --floe-mark %.1f", r.name, w, *g.Size)
			}
		}
	}
	gap := 0.35 * got.Rem
	for i, r := range lineRows {
		if got.Rows[i] == nil {
			if whole {
				add(r.at, "%s not drawn: marksPage draws it", r.row)
			}
			continue
		}
		m.seen++
		mark, line := b.pseudoBoxes(r.row)
		if mark == nil || line == nil {
			add(r.at, "%s draws no mark or no line", r.row)
			continue
		}
		start, end := line[1]-mark[3], got.Rows[i][3]-line[3]
		mid := (line[0]+line[2])/2 - (mark[0]+mark[2])/2
		if math.Abs(start-gap) > 0.5 || math.Abs(end-gap) > 0.5 || math.Abs(mid) > 0.5 {
			add(r.at, "%s's line starts %.1f px below its mark, ends %.1f px above its end, %.1f px off its middle", r.row, start, end, mid)
		}
	}
	if got.Date != nil && !narrow {
		m.seen++
		if math.Abs(*got.Date) > 0.5 {
			add("timeline", "timeline's date %.1f px off its title's baseline", *got.Date)
		}
	}
	if got.Wrapped != nil {
		m.seen++
		if mark, _ := b.pseudoBoxes("#wrapped > li"); *got.Wrapped != 1 || mark == nil || got.WrappedLeft <= mark[2] {
			add("numbered", "#wrapped's text in %d lines, from %.1f, its mark %v", *got.Wrapped, got.WrappedLeft, mark)
		}
	}
	if got.Done != nil {
		m.seen++
		if got.Done[0] != got.Done[1] {
			add("steps", "a step done has its mark %s and its line %s", got.Done[0], got.Done[1])
		}
	}
	return m
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// tallTheme is a theme of one's own that pads the rows 2rem above, sets their line height 2.5, draws their first line
// bigger than the mark's letters and moves the note's text off its mark; bareTheme leaves the look numbered's rings
// out, and so their lines.
var tallTheme = map[string]string{
	"themes/tall/links.css":    ":scope > li { padding-top: 2rem; line-height: 2.5; }\n:scope a { font-size: 1.6rem; }\n",
	"themes/tall/note.css":     ":scope { padding-top: 2rem; line-height: 2.5; --floe-mark-gap: 2rem; }\n.title { font-size: 1.6rem; }\n",
	"themes/tall/steps.css":    ":scope > li { padding-top: 2rem; line-height: 2.5; }\n:scope strong { font-size: 1.6rem; }\n",
	"themes/tall/timeline.css": ":scope > li { padding-top: 2rem; line-height: 2.5; }\n:scope strong { font-size: 1.6rem; }\n",
	"themes/tall/numbered.css": ":scope > li { padding-top: 2rem; line-height: 2.5; }\n:scope strong { font-size: 1.6rem; }\n",
	"themes/tall/decision.css": ":scope .rejected > li::before { content: none; }\n",
	"themes/bare/numbered.css": ":scope > li::before { content: none; }\n",
}

const lookMarksFix = "say a row's mark by part.yaml's marks: and let floe place it; space the text from it by --floe-mark-gap " +
	"and size it by --floe-mark, and write no display, gap or grid-template on a marked row"

// lookMarks measures marksPage (marksIn) in every theme, in tallTheme, and, for timeline, on a narrow screen too: each
// mark stands on its first line's baseline, wherever a theme or the row's padding puts that line, its --floe-mark-gap
// from the text, or takes no room when a theme leaves it out, and each line to the next mark runs from 0.35rem below
// its mark to 0.35rem above its row's end, however big the mark or the title and whatever the row starts with. A theme
// that leaves the rings out leaves out their lines (bareTheme).
func lookMarks(t *testing.T, s *site) measured {
	t.Helper()

	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	s.own(t, tallTheme)
	file := s.write(t, "marks.yaml", marksPage)
	var m measured
	for _, theme := range append(themes, "tall") {
		s.floe(t, "publish", file, "--slug", "marks-"+theme, "--theme", theme)
		s.b.navigate("/a/marks-" + theme + "/?raw")
		m.add(s.b.marksIn("marksPage in "+theme, false, true))
	}
	s.floe(t, "publish", file, "--slug", "marks-bare", "--theme", "bare")
	s.b.navigate("/a/marks-bare/?raw")
	m.seen++
	if _, line := s.b.pseudoBoxes("#plain > li"); line != nil && line[3]-line[1] > 0.5 {
		m.amiss = append(m.amiss, amiss{Page: "marksPage in bare", At: "numbered", Text: fmt.Sprintf("a line %.1f px high under no ring", line[3]-line[1])})
	}
	s.b.size(500, 900, false)
	defer s.b.unsize()
	s.b.navigate("/a/marks-default/?raw")
	m.add(s.b.marksIn("marksPage at 500px", true, true))
	return m
}

// brokenMarks draws steps in a theme of one's own whose numbered.css writes a gap on the marked rows.
func brokenMarks(t *testing.T, s *site) (string, measured) {
	t.Helper()

	s.brokenPage(t, map[string]string{"themes/spread/numbered.css": ":scope > li { column-gap: 30px; }\n"},
		themePage("spread", "- steps:\n    - { title: One, text: first, state: done }\n    - { title: Two, text: second }\n"))
	return "steps", s.b.marksIn("broken", false, false)
}
