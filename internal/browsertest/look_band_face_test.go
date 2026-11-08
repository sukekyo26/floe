package browsertest_test

import (
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/kit"
)

// bandPillsHTML is the part cell in each tone and in none, as a template places it.
const bandPillsHTML = `<span data-floe-part="cell">plain</span> <span data-floe-part="cell" data-floe-tone="accent">open</span> ` +
	`<span data-floe-part="cell" data-floe-tone="good">ok</span> <span data-floe-part="cell" data-floe-tone="warn">wait</span> ` +
	`<span data-floe-part="cell" data-floe-tone="bad">stop</span> <span data-floe-part="cell" data-floe-tone="muted">later</span>`

// bandKit is a template that places, in a band, pills (cell) in each tone and in none, a card of its own (the look
// cards), and a note, one it places itself holding a link and code, a table and code of floe's and Markdown, each when the data file has it.
var bandKit = map[string]string{
	"templates/mine/page.html": "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><meta charset=\"utf-8\">" +
		"<title>{{.Title}}</title></head>\n<body>\n<header data-floe-look=\"band\"><p>" + bandPillsHTML + "</p>{{if .Data.note}}<div data-floe-part=\"note\"><p>A borrowed note, <a href=\"https://example.com/\">a link</a> and <code>code</code>.</p></div>{{end}}{{.Data.note}}{{.Data.text}}{{.Data.words}}{{.Data.details}}{{.Data.options}}<div data-floe-look=\"cards\"><div class=\"card\" data-floe-look=\"panel\"><p>A card of the template's own.</p></div></div>" +
		"<div data-floe-look=\"panel\"><p>A panel of the template's own, <a href=\"https://example.com/\">a link</a> and <code>code</code>.</p></div>" +
		"<div data-floe-look=\"table\"><table><tr><th>A <a href=\"https://example.com/\">table</a></th></tr><tr><td>of its own, <code>code</code></td></tr></table></div>" +
		"{{.Data.table}}{{.Data.code}}{{.Data.chart}}{{.Data.bars}}{{.Data.tree}}{{.Data.timeline}}{{.Data.tabs}}{{.Data.links}}</header>\n</body></html>\n",
	"templates/mine/page.css": "header { display: grid; gap: 1rem; padding: 2rem; }\n",
	"templates/mine/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"note": {"$ref": "floe:///parts/note.json"}, "text": {"type": "string", "format": "markdown"},
"table": {"$ref": "floe:///parts/table.json"}, "code": {"$ref": "floe:///parts/code.json"},
"words": {"$ref": "floe:///parts/text.json"}, "details": {"$ref": "floe:///parts/details.json"},
"options": {"$ref": "floe:///parts/options.json"}, "bars": {"$ref": "floe:///parts/bars.json"},
"tree": {"$ref": "floe:///parts/tree.json"}, "timeline": {"$ref": "floe:///parts/timeline.json"},
"tabs": {"$ref": "floe:///parts/tabs.json"}, "links": {"$ref": "floe:///parts/links.json"}, "chart": {"$ref": "floe:///parts/chart.json"}}}`,
	"templates/mine/example.yaml": "lang: en\ntemplate: mine\n",
}

// bandText is Markdown in a band, the template's own (in floe's box of it, data-floe-markdown): a link, a line's code, a quote holding both, and a fenced block wide enough to scroll;
// and a part text, a heading over a link and a line's code.
var bandText = "lang: en\ntemplate: mine\ntitle: Band\ntext: |\n  A [link](https://example.com/) and `inline`.\n\n" +
	"  > A quote with [a link](https://example.com/) and `code`.\n\n  ```\n  fenced" + strings.Repeat(" wide", 60) + "\n  ```\n" +
	"words: \"## Said\\n\\nA text with [a link](https://example.com/) and `code`.\"\n"

// bandHeld is parts in a band that hold Markdown: details holding a heading, and a card of options.
var bandHeld = "details: { title: Held, open: true, parts: [{ text: \"## Held\\n\\nIn details.\" }] }\n" +
	"options: [{ title: Card, text: \"A card's text.\" }]\n"

// bandParts is parts in a band: a note with a title, a link and a line's code, a table with a link in its heading and as wide
// as to scroll, and code as wide as to scroll.
var bandParts = bandText + bandHeld + "chart: { kind: line, title: T, x: [a, b], series: [ { name: n, values: [1, 2] }, { name: m, values: [2, 1] } ] }\n" +
	"note: { title: Said, text: \"A note with [a link](https://example.com/) and `code`.\" }\n" +
	"table:\n  columns: [{ title: \"Name [t](https://example.com/)\" }" + strings.Repeat(", { title: Wide }", 16) + "]\n" +
	"  rows: [[a" + strings.Repeat(", b", 16) + "]]\n" +
	"code: { file: a.conf, text: \"wide" + strings.Repeat(" wide", 60) + "\\n\" }\n"

// bandColours is parts in a band that draw some of their text in colours of their own (muted, a tone, the page's
// text colour): bars' label and value, a tree's note and change, a timeline's date; tabs, whose titles have no
// ground, and links, whose title, words and URL draw in colours of their own.
var bandColours = "bars: [{ label: Copied, value: 40, of: 100 }]\n" +
	"tree: [{ path: pages/, note: what export writes }, { path: pages/home.md, change: changed }]\n" +
	"timeline: [{ date: '03-01', title: Started, text: The copy began. }]\n" +
	"tabs: [{ title: One, parts: [{ text: First. }] }, { title: Two, parts: [{ text: Second. }] }]\n" +
	"links: [{ title: Guide, url: 'https://example.com/guide', text: How it works }]\n"

// measureBand returns a line for each thing the band draws wrong: a part in it not in the page's text colour, a
// line's code without its frame (a shadow every code has, seen only in a face), or code in a fenced block with one.
const measureBand = `(() => {` + colours + `
  const out = [];
  document.querySelectorAll('[data-floe-part=links] .url').forEach((url) => {
    if (url.getBoundingClientRect().top - url.previousElementSibling.getBoundingClientRect().bottom < 1) out.push('a links URL against its title');
  });
  const framed = (e) => { const c = getComputedStyle(e).boxShadow.match(/^(rgba?\([^)]*\)|color\([^)]*\))/); return c !== null && rgba(c[1])[3] > 0; };
  const probe = document.body.appendChild(document.createElement('p'));
  const want = getComputedStyle(probe).color;
  probe.remove();
  // A note is on a ground of its own in a band (swiss's too, on the surface there), in the page's text colour.
  for (const note of document.querySelectorAll('[data-floe-part="note"]')) {
    const s = getComputedStyle(note);
    if (rgba(s.backgroundColor)[3] === 0) out.push('note: no ground of its own');
    if (s.color !== want) out.push('note: colour ' + s.color + ', want ' + want);
  }
  const line = document.querySelector('header > [data-floe-markdown] > p > code');
  if (!framed(line)) out.push('a line\'s code: no frame, ' + getComputedStyle(line).boxShadow);
  const fenced = document.querySelector('header > [data-floe-markdown] > pre > code');
  if (framed(fenced)) out.push('fenced code: frame ' + getComputedStyle(fenced).boxShadow);
  // A text has no ground of its own: its Markdown is in the band's text colour, and its heading takes no mark of the
  // page's (retro's stripes, which its band.css leaves out).
  const text = document.querySelector('header > [data-floe-text]');
  if (text) {
    const band = getComputedStyle(document.querySelector('header')).color, said = text.querySelector('p');
    if (getComputedStyle(said).color !== band) out.push('a text: colour ' + getComputedStyle(said).color + ', want the band\'s ' + band);
    if (getComputedStyle(text.querySelector('h2'), '::after').content !== 'none') out.push('a text\'s heading: a mark after it');
  }
  // A heading held in a part in the band takes no mark either. A card is on a ground of its own in a band (swiss's
  // too, on the surface there), its text off the band's face.
  const held = document.querySelector('header [data-floe-part="details"] h2');
  if (held && getComputedStyle(held, '::after').content !== 'none') out.push('a heading in details: a mark after it');
  const card = document.querySelector('header [data-floe-look~="cards"] > .card');
  if (card) {
    if (rgba(getComputedStyle(card).backgroundColor)[3] === 0) out.push('a card: no ground of its own');
    if (getComputedStyle(card.querySelector('p')).color === getComputedStyle(document.querySelector('header')).color) out.push('a card: in the band\'s text colour');
  }
  // The picked tab is told from the others and from the band: its ground stands 3:1 off the band's.
  const picked = document.querySelector('header [data-floe-part="tabs"] > label:has(:checked)');
  if (picked) {
    const band = rgba(getComputedStyle(document.querySelector('header')).backgroundColor), r = ratio(over(rgba(getComputedStyle(picked).backgroundColor), band), band);
    if (r < 3) out.push('the picked tab: its ground ' + r.toFixed(2) + ':1 off the band\'s');
  }
  return out.join('\n');
})()`

// lookBandFace draws Markdown, pills and parts a template places in the look band, in every theme: a part on a ground
// of its own is in the page's text colour (swiss's boxes, of no ground elsewhere, are on the surface in a band) and one
// of none in the band's, only a line's code is framed,
// and every text and focus ring in the band reads on the ground under it, as measureContrast and measureRings measure;
// a text there is in the band's text colour, its heading unmarked, pills of a theme that gives them no ground too. The
// template places a panel and a table of its own too, as looks on elements that are no part's box.
// Aurora's glass would let the band through but for its band.css.
func lookBandFace(t *testing.T) { //nolint:paralleltest,thelper // a subtest of TestLookRule; the browser tests share one Chrome
	s := newSite(t)
	s.own(t, bandKit)
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	pages := map[string]string{}
	for _, theme := range themes {
		pages["parts-"+theme] = bandParts + bandColours + "theme: " + theme + "\n"
	}
	for slug, page := range pages {
		s.floe(t, "publish", s.write(t, slug+".yaml", page), "--slug", slug)
		s.b.navigate("/a/" + slug + "/?raw")
		s.b.until("the band", "document.querySelector('header > [data-floe-markdown] > pre') !== null")
		if got := s.b.eval(measureBand); got != "" {
			t.Errorf("in the band of %s:\n  %s", slug, strings.ReplaceAll(got, "\n", "\n  "))
		}
		s.b.measureColours(t, "the band of "+slug)
	}
}
