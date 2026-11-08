package browsertest_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/chromedp"
)

// barsPage holds bars of a whole each (of), one of large numbers, beside bars without one.
const barsPage = `template: report
lang: en
title: Bars
sections:
  - heading: Bars
    parts:
      - bars:
          - { label: Pages, value: 5, of: 8 }
          - { label: Words, value: 1500000, of: 2000000, tone: good }
          - { label: Kept, value: 2 }
`

// TestRenderedBarsProgress reads the bars with of as a screen reader is given them: each a progressbar named by its label,
// its value and whole as numbers, and its aria-valuetext as the page shows them; a bar without of is no progressbar.
func TestRenderedBarsProgress(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.floe(t, "publish", s.write(t, "bars.yaml", barsPage), "--slug", "bars")
	b.navigate("/a/bars/?raw")

	tree, err := chromedp.Call(b.ctx, accessibility.GetFullAXTree, accessibility.GetFullAXTreeParams{})
	if err != nil {
		t.Fatal(err)
	}
	text := func(v *accessibility.Value) string {
		if v == nil {
			return "-"
		}
		var s string
		if json.Unmarshal(v.Value, &s) != nil {
			return string(v.Value)
		}
		return s
	}
	var got []string
	for _, n := range tree.Nodes {
		if n.Ignored || n.Role == nil || text(n.Role) != "progressbar" {
			continue
		}
		line := text(n.Name) + " now " + text(n.Value)
		for _, p := range n.Properties {
			if p.Name == "valuemax" || p.Name == "valuemin" {
				line += " " + string(p.Name) + " " + text(p.Value)
			}
		}
		got = append(got, line)
	}
	want := "Pages now 5 valuemin 0 valuemax 8\nWords now 1500000 valuemin 0 valuemax 2000000"
	if strings.Join(got, "\n") != want {
		t.Errorf("progressbars:\n%s\nwant\n%s", strings.Join(got, "\n"), want)
	}
	// LIMIT: Chrome gives a progressbar's aria-valuetext to no screen reader (its AX tree has it empty), so the text is
	// read from the page; read it from the AX tree if Chrome comes to give it.
	texts := b.eval(`[...document.querySelectorAll('[role=progressbar]')].map((e) => e.getAttribute('aria-valuetext')).join('|')`)
	if want := "5 / 8|1,500,000 / 2,000,000"; texts != want {
		t.Errorf("aria-valuetext %q, want %q", texts, want)
	}
}
