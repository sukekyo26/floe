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

// marked marks each element shown whose ::before or ::after draws content with data-speech (one in a tab not picked
// is given a screen reader no more than it is shown), and returns, for each, its
// name, whether a pseudo-element's reading (after its /) has a counter, and its text with the hidden words, but
// what a script or style holds; then each hidden word whose parent is a row (li), and each element whose mark means
// what the HTML must tell in hidden words, which has none. A numbered row, a numbered card and a slide read their number
// whatever their CSS writes after /.
const marked = `(() => {
  const numbered = '[data-floe-look~="numbered"] > li:not(.done), .numbered > .card, .slides > li';
  const worded = '[data-floe-part=steps] > li.done > strong, [data-floe-part=checklist] > li > span, .option li > span, ' +
    '[data-floe-part=tree] li:is(.added, .removed, .changed) > .name, [data-floe-part=terminal] .run';
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
    e.dataset.speech = String(i++);
    const copy = e.cloneNode(true);
    copy.querySelectorAll('script, style').forEach((s) => s.remove());
    const part = e.closest('[data-floe-part]');
    out.push({ name: (part ? part.dataset.floePart + ' ' : '') + e.tagName.toLowerCase() + [...e.classList].map((c) => '.' + c).join(''), counted, text: copy.textContent });
  }
  const rows = [...document.querySelectorAll('li > .floe-hidden')].map((e) => e.textContent);
  const told = (e) => { const h = e.matches('.run') ? e.previousElementSibling : e.querySelector(':scope > .floe-hidden'); return h?.matches('.floe-hidden') ? h : null; };
  const unworded = [...document.querySelectorAll(worded)].filter((e) => e.checkVisibility() && !(told(e)?.textContent.trim()))
    .map((e) => e.closest('[data-floe-part]').dataset.floePart + ' ' + e.tagName.toLowerCase() + [...(e.closest('li') || e).classList].map((c) => '.' + c).join(''));
  return JSON.stringify({ out, rows, unworded });
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

// TestUISpeech reads, in every page of floe's parts and templates in every theme, what a screen reader is given in
// each element CSS draws content in: a mark is not read and a hidden word is, so that it is the element's text;
// where the reading after / is a counter, its number is read too. A hidden word stands in a row's text, not in the
// row.
func TestUISpeech(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	var amiss []string
	s.floePages(t, func(what string) {
		var found struct {
			Out []struct {
				Name    string
				Counted bool
				Text    string
			}
			Rows     []string
			Unworded []string
		}
		if err := json.Unmarshal([]byte(b.eval(marked)), &found); err != nil {
			t.Fatal(err)
		}
		for _, e := range found.Unworded {
			amiss = append(amiss, what+": "+e+" tells no hidden word")
		}
		for _, row := range found.Rows {
			amiss = append(amiss, what+": hidden "+row+" in a row")
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
		for i, id := range q.NodeIDs {
			d, err := chromedp.Call(b.ctx, dom.DescribeNode, dom.DescribeNodeParams{NodeID: id})
			if err != nil {
				t.Fatal(err)
			}
			e := found.Out[i]
			got, want := bare(read[d.Node.BackendNodeID]), bare(e.Text)
			if e.Counted {
				strip := func(s string) string {
					return strings.Map(func(r rune) rune {
						if unicode.IsDigit(r) {
							return -1
						}
						return r
					}, s)
				}
				if strip(got) != strip(want) || len(got)-len(strip(got)) <= len(want)-len(strip(want)) {
					amiss = append(amiss, what+": "+e.Name+" reads "+got+", want its number and "+want)
				}
			} else if got != want {
				amiss = append(amiss, what+": "+e.Name+" reads "+got+", want "+want)
			}
		}
	})
	check(t, "speech", amiss, "")
}
