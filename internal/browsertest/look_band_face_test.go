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

// measureBand finds what the band draws wrong: a note with no ground of its own or not
// in the page's text colour, a line's code without its frame (a shadow every code has, seen only in a face) or code in
// a fenced block with one, a text not in the band's text colour or its heading marked, a heading in details marked, a
// card with no ground of its own or in the band's text colour, and a picked tab whose ground is not 3:1 off the band's.
// What it looks for and does not find is amiss too, so that a change of the page's shape fails rather than passes.
// It returns {seen, amiss}: the things it measured, and for each wrong the part (or what is missing) and how.
const measureBand = `(() => {` + colours + `
  let seen = 0;
  const amiss = [], push = (at, text) => amiss.push({ at, text });
  const find = (at, q) => { const e = document.querySelector(q); if (!e) push(at, 'none found: ' + q); else seen++; return e; };
  const header = document.querySelector('header');
  const framed = (e) => { const c = getComputedStyle(e).boxShadow.match(/^(rgba?\([^)]*\)|color\([^)]*\))/); return c !== null && rgba(c[1])[3] > 0; };
  const probe = document.body.appendChild(document.createElement('p'));
  const want = getComputedStyle(probe).color;
  probe.remove();
  // A note is on a ground of its own in a band (swiss's too, on the surface there), in the page's text colour.
  for (const note of document.querySelectorAll('[data-floe-part="note"]')) {
    seen++;
    const s = getComputedStyle(note);
    if (rgba(s.backgroundColor)[3] === 0) push('note', 'no ground of its own');
    if (s.color !== want) push('note', 'colour ' + s.color + ', want ' + want);
  }
  const line = find('markdown', 'header > [data-floe-markdown] > p > code');
  if (line && !framed(line)) push('markdown', 'a line\'s code: no frame, ' + getComputedStyle(line).boxShadow);
  const fenced = find('markdown', 'header > [data-floe-markdown] > pre > code');
  if (fenced && framed(fenced)) push('markdown', 'fenced code: frame ' + getComputedStyle(fenced).boxShadow);
  // A text has no ground of its own: its Markdown is in the band's text colour, and its heading takes no mark of the
  // page's (retro's stripes, which its band.css leaves out).
  const said = find('text', 'header > [data-floe-text] p'), heading = find('text', 'header > [data-floe-text] h2');
  const band = getComputedStyle(header).color;
  if (said && getComputedStyle(said).color !== band) push('text', 'colour ' + getComputedStyle(said).color + ', want the band\'s ' + band);
  if (heading && getComputedStyle(heading, '::after').content !== 'none') push('text', 'its heading: a mark after it');
  // A heading held in a part in the band takes no mark either. A card is on a ground of its own in a band (swiss's
  // too, on the surface there), its text off the band's face.
  const held = find('details', 'header [data-floe-part="details"] h2');
  if (held && getComputedStyle(held, '::after').content !== 'none') push('details', 'its heading: a mark after it');
  const card = find('cards', 'header [data-floe-look~="cards"] > .card');
  if (card) {
    if (rgba(getComputedStyle(card).backgroundColor)[3] === 0) push('cards', 'a card: no ground of its own');
    const p = card.querySelector('p');
    if (!p) push('cards', 'none found: a card\'s p');
    else if (getComputedStyle(p).color === band) push('cards', 'a card: in the band\'s text colour');
  }
  // The picked tab is told from the others and from the band: its ground stands 3:1 off the band's.
  const picked = find('tabs', 'header [data-floe-part="tabs"] > label:has(:checked)');
  if (picked) {
    const ground = rgba(getComputedStyle(header).backgroundColor), r = ratio(over(rgba(getComputedStyle(picked).backgroundColor), ground), ground);
    if (r < 3) push('tabs', 'the picked tab: its ground ' + r.toFixed(2) + ':1 off the band\'s');
  }
  return JSON.stringify({ seen, amiss });
})()`

// bandPages publishes bandParts and bandColours in the band of bandKit's template, in every theme, and visits each.
func bandPages(t *testing.T, s *site, visit func(what string)) {
	t.Helper()

	s.own(t, bandKit)
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	for _, theme := range themes {
		bandPage(t, s, "band-"+theme, bandParts+bandColours+"theme: "+theme+"\n")
		visit("the band in " + theme)
	}
}

// bandPage publishes data, a page of bandKit's template, as slug and opens it once its band is drawn.
func bandPage(t *testing.T, s *site, slug, data string) {
	t.Helper()

	s.floe(t, "publish", s.write(t, slug+".yaml", data), "--slug", slug)
	s.b.navigate("/a/" + slug + "/?raw")
	s.b.until("the band", "document.querySelector('header > [data-floe-markdown] > pre') !== null")
}

const lookBandFaceFix = "draw a part in a band in the colours of the band's face: a part on a ground of its own in the page's " +
	"text colour, one of none in the band's; frame a line's code alone; mark no heading; set the picked tab 3:1 off the band"

// lookBandFace draws Markdown, pills and parts a template places in the look band, in every theme (bandPages), as
// measureBand measures: a part on a ground of its own is in the page's text colour (swiss's boxes, of no ground
// elsewhere, are on the surface in a band) and one of none in the band's, only a line's code is framed, a text there
// is in the band's text colour, its heading unmarked. Aurora's glass would let the band through but for its band.css.
// That what is in the band reads on the ground under it is contrast-text's and contrast-ring's, which draw these pages
// too.
func lookBandFace(t *testing.T, s *site) measured {
	t.Helper()

	var m measured
	bandPages(t, s, func(what string) { m.add(s.b.measure(what, measureBand)) })
	return m
}

// brokenBandFace draws the band in a theme of one's own whose note.css paints a note's text in a colour of its own.
func brokenBandFace(t *testing.T, s *site) (string, measured) {
	t.Helper()

	s.own(t, bandKit)
	s.own(t, map[string]string{"themes/drab/note.css": ":scope { color: rgb(1, 2, 3); }\n"})
	bandPage(t, s, "broken", bandParts+bandColours+"theme: drab\n")
	return "note", s.b.measure("broken", measureBand)
}
