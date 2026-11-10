package browsertest_test

import (
	"strings"
	"testing"

	"floe/internal/kit"
)

// pillKit is a part of one's own that lets the pills it holds wrap (--floe-pill-wrap) and holds a table, whose pills
// keep to one line all the same: the part's box stops the variable. A template of one's own places it.
var pillKit = map[string]string{
	"templates/held/page.html": "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><meta charset=\"utf-8\">" +
		"<title>{{.Title}}</title></head>\n<body>\n{{.Data.wrapped}}\n</body></html>\n",
	"templates/held/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"wrapped": {"$ref": "floe:///parts/wrapped.json"}}}`,
	"templates/held/example.yaml": "lang: en\ntemplate: held\n",
	"parts/wrapped/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"table": {"$ref": "floe:///parts/table.json"}}}`,
	"parts/wrapped/part.html": `<div data-floe-part="wrapped"><div class="wrap">{{.table}}</div></div>`,
	"parts/wrapped/part.css":  ".wrap { --floe-pill-wrap: normal; }\n",
}

// pillPage holds pills beside a long text in a table, in its pills column and as a cell's pill in another, in English
// and in Japanese, which breaks anywhere; and entries' long pills, which wrap, and a short one.
const pillPage = `lang: ja
title: Pills
sections:
  - heading: Pills
    parts:
      - table:
          columns: [{ title: what }, { title: state, pills: { in progress: warn, 進行中: warn } }, { title: other }]
          rows:
            - [a long line of words about what is done and what is left to do before the move, in progress, { pill: in progress, tone: warn }]
            - [移行の前に、何が済んで何が残っているかを書いた長い行の文です, 進行中, { pill: 答え待ち, tone: warn }]
      - entries:
          - title: Long pills
            tags: [{ pill: blocked until the platform team reviews the export and the import and the backup of every page, tone: warn }, { pill: 基盤チームが書き出しと読み込みとすべてのページの控えを確かめるまで止めておく, tone: bad }, { pill: short, tone: good }]
            text: Text.
`

// pillHeld is the part of one's own that lets pills wrap, holding such a table.
const pillHeld = `template: held
lang: en
title: Held
wrapped:
  table:
    columns: [{ title: what }, { title: state, pills: { waiting for a review: warn } }]
    rows:
      - [a long line of words about what is done and what is left to do before the move of every page of the wiki to its new host and after it, waiting for a review]
`

// pillDoing is handoff's work in progress, a long name beside a state in Japanese, in the grid's column of 1fr.
const pillDoing = `template: handoff
lang: ja
title: 引き継ぎ
date: 2030-05-16
from: 山田
to: 佐藤
doing:
  - { who: とても長い名前の担当者のチームの代表の人, what: 住所の入力欄, state: waiting }
`

// measurePills returns one line for each pill out of entries (whose tags let them wrap) that wraps, a pill of one line
// whose corners leave straight sides though the theme's --radius-pill rounds them, and a pill that wraps whose corners
// still round it to an oval, each a pixel or two short of the line, as a corner rounds to 1/64 px. A theme that draws
// a pill's corners each its own (notebook's, by hand) is left out of the corners.
const measurePills = `(() => {
  const root = getComputedStyle(document.documentElement), out = [];
  const pill = parseFloat(root.getPropertyValue('--radius-pill'));
  for (const e of document.querySelectorAll('[data-floe-part=cell]')) {
    const r = e.getBoundingClientRect(), s = getComputedStyle(e), half = r.height / 2, round = parseFloat(s.borderTopLeftRadius);
    const line = parseFloat(s.lineHeight), wraps = r.height >= 2 * line, what = e.textContent.trim().slice(0, 20);
    if (wraps && !e.closest('[data-floe-part=entries]')) out.push('wraps: ' + what);
    if (s.borderTopLeftRadius !== s.borderTopRightRadius) continue;
    if (!wraps && pill >= half + 1 && round < half + 1) out.push('one line, straight sides: ' + what + ' ' + round + ' of ' + half);
    if (wraps && round > half - 2) out.push('wraps, an oval: ' + what + ' ' + round + ' of ' + half);
  }
  return out.join('\n');
})()`

// TestRenderedPills draws pills in every theme, as wide as a phone (390 px) and as a narrow window (620 px): a pill keeps to
// one line in a table's column and a grid's, unless what holds it lets it wrap, short of a part it holds, and one that
// wraps is a label of round corners, though a pill of one line stays round at both ends.
func TestRenderedPills(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	b := s.b
	s.own(t, pillKit)
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	for _, theme := range themes {
		for slug, page := range map[string]string{"pills-": "template: report\n" + pillPage, "held-": pillHeld, "doing-": pillDoing} {
			slug += theme
			s.floe(t, "publish", s.write(t, slug+".yaml", page+"theme: "+theme+"\n"), "--slug", slug)
			for _, w := range []int64{390, 620} {
				b.size(w, 844, true)
				b.navigate("/a/" + slug + "/?raw")
				if got := b.eval(measurePills); got != "" {
					t.Errorf("%s at %d px:\n  %s", slug, w, strings.ReplaceAll(got, "\n", "\n  "))
				}
			}
		}
	}
}
