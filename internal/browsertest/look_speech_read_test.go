package browsertest_test

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/chromedp"
)

// marked opens every details, marks each element shown whose ::before or ::after draws content with data-speech (one
// in a tab not picked is given a screen reader no more than it is shown), and returns, for each, its part (or its
// tag), its name, whether a pseudo-element's reading (after its /) has a counter, and its text with the hidden words,
// but what a script or style holds; then each hidden word whose parent is a row (li), and each element whose mark
// means what the HTML must tell in hidden words, which has none, both with the part they are in; and how many such
// elements it looked at (worded). A numbered row and a numbered card read their number
// whatever their CSS writes after /; a slide's number and a checklist's count, which no word tells from what is
// around them, are not read.
// LIMIT: worded, numbered and unnumbered list by hand the marks that mean what the HTML must tell, as tokens do not tell a mark
// that means from one that adorns; add a part's or template's to them when it draws one
const marked = `(() => {
  ` + partOfJS + `
  document.querySelectorAll('details').forEach((d) => { d.open = true; });
  const numbered = '[data-floe-look~="numbered"] > li:not(.done), .numbered > .card';
  const unnumbered = '.slides > li, [data-floe-part=checklist]';
  const worded = '[data-floe-part=steps] > li.done > strong, [data-floe-part=checklist] > li > span, .option li > span, ' +
    '[data-floe-part=tree] li:is(.added, .removed, .changed) > .name, [data-floe-part=terminal] .run, [data-floe-part=code] .mark, ' +
    '[data-floe-part=note]:not([class]), [data-floe-part=note].caution > p';
  const reading = (content) => {
    let depth = 0, quote = '', at = -1;
    for (let i = 0; i < content.length; i++) {
      const c = content[i];
      if (quote) { if (c === '\\') i++; else if (c === quote) quote = ''; continue; }
      if (c === '"' || c === "'") quote = c;
      else if (c === '(') depth++;
      else if (c === ')') depth--;
      else if (c === '/' && depth === 0) at = i;
    }
    return at < 0 ? content : content.slice(at + 1);
  };
  const out = [];
  let i = 0;
  for (const e of document.body.querySelectorAll('*')) {
    if (!e.checkVisibility({ visibilityProperty: true })) continue;
    let drawn = false, counted = false;
    for (const p of ['::before', '::after']) {
      const c = getComputedStyle(e, p).content;
      if (!c || c === 'none' || c === 'normal' || c === '""') continue;
      drawn = true;
      if (/counters?\(/.test(reading(c))) counted = true;
    }
    if (!drawn) continue;
    if (e.matches(numbered)) counted = true;
    if (e.matches(unnumbered)) counted = false;
    if (counted) e.dataset.speechCounted = '';
    e.dataset.speech = String(i++);
    // Zoom's tools and hint, hidden by a small image or until the keyboard is on it, are not read: a card holds them
    // when it holds an image (options' parts).
    const zoom = '.floe-zoom-tools, .floe-zoom-hint', copy = e.cloneNode(true), hidden = [...copy.querySelectorAll(zoom)];
    e.querySelectorAll(zoom).forEach((k, j) => { if (getComputedStyle(k).display === 'none') hidden[j].remove(); });
    copy.querySelectorAll('script, style').forEach((s) => s.remove());
    const part = e.closest('[data-floe-part]');
    out.push({ at: partOf(e), name: (part ? part.dataset.floePart + ' ' : '') + e.tagName.toLowerCase() + [...e.classList].map((c) => '.' + c).join(''), counted, text: copy.textContent, e });
  }
  for (const o of out) { o.inner = !!o.e.querySelector('[data-speech-counted]'); delete o.e; }
  const rows = [...document.querySelectorAll('li > .floe-hidden')].map((e) => ({ at: partOf(e), text: e.textContent }));
  const told = (e) => { const h = e.matches('.run, .mark') ? e.previousElementSibling : e.querySelector(':scope > .floe-hidden'); return h?.matches('.floe-hidden') ? h : null; };
  const shown = [...document.querySelectorAll(worded)].filter((e) => e.checkVisibility());
  const unworded = shown.filter((e) => !(told(e)?.textContent.trim()))
    .map((e) => ({ at: partOf(e), text: e.closest('[data-floe-part]').dataset.floePart + ' ' + e.tagName.toLowerCase() + [...(e.closest('li') || e).classList].map((c) => '.' + c).join('') }));
  return JSON.stringify({ out, rows, unworded, worded: shown.length });
})()`

// spoken returns, by each element's backend node, the text a screen reader is given in it: the text nodes of the
// accessibility tree below it that are not ignored.
func (b *browser) spoken(t *testing.T) map[cdp.BackendNodeID]string {
	t.Helper()

	tree, err := chromedp.Call(b.ctx, accessibility.GetFullAXTree, accessibility.GetFullAXTreeParams{})
	if err != nil {
		t.Fatal(err)
	}
	byID := map[accessibility.NodeID]*accessibility.Node{}
	for _, n := range tree.Nodes {
		byID[n.NodeID] = n
	}
	var text func(n *accessibility.Node) string
	text = func(n *accessibility.Node) string {
		if n.Role != nil && strings.Contains(string(n.Role.Value), "StaticText") {
			var v string
			if n.Ignored || n.Name == nil || json.Unmarshal(n.Name.Value, &v) != nil {
				return ""
			}
			return v
		}
		var s strings.Builder
		for _, c := range n.ChildIDs {
			if k, ok := byID[c]; ok {
				s.WriteString(text(k))
			}
		}
		return s.String()
	}
	out := map[cdp.BackendNodeID]string{}
	for _, n := range tree.Nodes {
		if n.BackendDOMNodeID != 0 {
			out[n.BackendDOMNodeID] = text(n)
		}
	}
	return out
}

// bare is s without its spaces, in lower case: a reading and a text compare so, the way a theme's uppercase or a
// break between text nodes does not tell.
func bare(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), ""))
}

// speechRead reads, in the page open, told as what, what a screen reader is given in each element CSS draws content in
// (marked): a mark is not read and a hidden word is, so that it is the element's text; where the reading after / is a
// counter, its number is read too (and in what holds such an element, a number may be read that its text does not
// have). A hidden word stands in a row's text, not in the row. It returns the elements it read and the worded ones it
// looked at (seen), and each read amiss by its part.
func speechRead(t *testing.T, s *site, what string) measured {
	t.Helper()

	b := s.b
	type told struct{ At, Text string }
	var found struct {
		Out []struct {
			At      string
			Name    string
			Counted bool
			Inner   bool
			Text    string
		}
		Rows     []told
		Unworded []told
		Worded   int
	}
	if err := json.Unmarshal([]byte(b.eval(marked)), &found); err != nil {
		t.Fatal(err)
	}
	m := measured{seen: len(found.Out) + found.Worded}
	add := func(at, text string) { m.amiss = append(m.amiss, amiss{Page: what, At: at, Text: text}) }
	for _, e := range found.Unworded {
		add(e.At, e.Text+" tells no hidden word")
	}
	for _, row := range found.Rows {
		add(row.At, "hidden "+row.Text+" in a row")
	}
	read := b.spoken(t)
	doc, err := chromedp.Call(b.ctx, dom.GetDocument, dom.GetDocumentParams{})
	if err != nil {
		t.Fatal(err)
	}
	q, err := chromedp.Call(b.ctx, dom.QuerySelectorAll, dom.QuerySelectorAllParams{NodeID: doc.Root.NodeID, Selector: "[data-speech]"})
	if err != nil || len(q.NodeIDs) != len(found.Out) {
		t.Fatalf("%s: %d marked, %d found: %v", what, len(found.Out), len(q.NodeIDs), err)
	}
	strip := func(s string) string {
		return strings.Map(func(r rune) rune {
			if unicode.IsDigit(r) {
				return -1
			}
			return r
		}, s)
	}
	for i, id := range q.NodeIDs {
		d, err := chromedp.Call(b.ctx, dom.DescribeNode, dom.DescribeNodeParams{NodeID: id})
		if err != nil {
			t.Fatal(err)
		}
		e := found.Out[i]
		got, want := bare(read[d.Node.BackendNodeID]), bare(e.Text)
		if e.Counted || e.Inner {
			if strip(got) != strip(want) || e.Counted && len(got)-len(strip(got)) <= len(want)-len(strip(want)) {
				add(e.At, e.Name+" reads "+got+", want its number and "+want)
			}
		} else if got != want {
			add(e.At, e.Name+" reads "+got+", want "+want)
		}
	}
	return m
}

const lookSpeechReadFix = "write what a mark means as a hidden word (.floe-hidden, its words in words.yaml), and give a mark drawn " +
	"by CSS an empty reading after / (content: \"✓\" / \"\")"

// lookSpeechRead reads every page of floePages, its details open, as speechRead does.
func lookSpeechRead(t *testing.T, s *site) measured {
	t.Helper()

	var m measured
	s.floePages(t, func(what string) { m.add(speechRead(t, s, what)) })
	return m
}

// brokenSpeechRead draws a part of one's own whose rows' mark is read, written with no reading after /.
func brokenSpeechRead(t *testing.T, s *site) (string, measured) {
	t.Helper()

	html := `<ul data-floe-part="marky"><li>Done</li></ul>`
	s.brokenPage(t, ownPart("marky", html, ":scope li::before { content: \"✓\"; }\n"), htmlPage(html))
	return "marky", speechRead(t, s, "broken")
}
