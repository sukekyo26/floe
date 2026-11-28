package browsertest_test

import (
	"os"
	"strings"
	"testing"
)

// linksURLs are the URLs linksPage writes.
var linksURLs = []string{
	"https://example.com/guide", "/a/old-wiki/", "https://example.com/日本語", "https://example.com/%E3%81%82", "https://example.com/long",
	"https://example.com/a/very/long/path/without/any/break/that/goes/on/and/on/and/on/and/on/and/on/and/on/and/on/and/on/and/on/and/on/and/on/index.html?query=1",
}

// linksPage holds a link of a few words, one of none, one of words too long to stand beside it and one of no break, its
// URL long.
var linksPage = `template: report
lang: en
title: Links
sections:
  - heading: Links
    parts:
      - links:
          - { title: Guide, url: '` + linksURLs[0] + `', text: How it works }
          - { title: Old wiki, url: ` + linksURLs[1] + ` }
          - { title: Japanese, url: '` + linksURLs[2] + `' }
          - { title: Encoded, url: '` + linksURLs[3] + `' }
          - { title: ` + strings.TrimSpace(strings.Repeat("a long title of words ", 8)) + `, url: '` + linksURLs[4] + `', text: ` + strings.TrimSpace(strings.Repeat("words that go on ", 12)) + ` }
          - { title: ` + strings.Repeat("unbroken", 30) + `, url: '` + linksURLs[5] + `', text: ` + strings.Repeat("unbroken", 30) + ` }
`

// linksTo are where linksPage's links go, as their frames say.
var linksTo = []string{"example.com", "docs", "example.com", "example.com", "example.com", "example.com"}

// measureLinks returns a line for each thing the frames draw wrong: the list or a frame past the page or the list, a
// frame with a ground or a shadow (a button's), or without its frame, its destination, its title underlined and, on
// one line, cut to its letters and in the middle of the frame or, for one that leaves floe, its ↗; words not beside their frame; the first two frames not side by side on a wide page; or
// a link's reach not its frame.
const measureLinks = `(() => {
  const out = [], list = document.querySelector('[data-floe-part=links]'), rows = [...list.children];
  const box = list.getBoundingClientRect();
  if (box.right > document.documentElement.clientWidth + 1) out.push('the list reaches past the page');
  rows.forEach((row, i) => {
    const a = row.querySelector('a'), f = a.getBoundingClientRect(), c = getComputedStyle(a);
    if (f.right > box.right + 1 || f.left < box.left - 1) out.push('row ' + i + ': the frame past the list');
    if (parseFloat(c.borderTopWidth) === 0) out.push('row ' + i + ': no frame');
    if (c.backgroundColor !== 'rgba(0, 0, 0, 0)' || c.boxShadow !== 'none') out.push('row ' + i + ': a ground or shadow, as a button');
    const to = a.querySelector('.to'), title = a.querySelector('.title');
    if (!to || !to.textContent || !title) out.push('row ' + i + ': no destination or title');
    else if (to.getBoundingClientRect().left > title.getBoundingClientRect().left) out.push('row ' + i + ': the destination after the title');
    if (title && !getComputedStyle(title).textDecorationLine.includes('underline')) out.push('row ' + i + ': the title not underlined');
    const lines = document.createRange();
    if (title) lines.selectNodeContents(title);
    if (title && new Set([...lines.getClientRects()].map((r) => Math.round(r.top))).size === 1) {
      const t = title.getBoundingClientRect(), edge = parseFloat(c.borderTopWidth), above = t.top - f.top - edge, below = f.bottom - edge - t.bottom;
      if (Math.abs(above - below) > 1) out.push('row ' + i + ': the title ' + above + ' below the frame\'s top, ' + below + ' above its bottom');
      if (t.height > 0.85 * parseFloat(getComputedStyle(title).fontSize)) out.push('row ' + i + ': the title ' + t.height + ' tall, not cut to its letters (a line box holds them low)');
    }
    if (a.getAttribute('href').startsWith('http') && !a.querySelector('.out')) out.push('row ' + i + ': no ↗ out of floe');
    const reach = document.createRange();
    reach.selectNodeContents(a);
    if (reach.getBoundingClientRect().right > f.right + 1) out.push('row ' + i + ': the words past the frame');
    const text = row.querySelector('.text');
    if (text && text.getBoundingClientRect().right > box.right + 1) out.push('row ' + i + ': the words past the list');
  });
  if (innerWidth >= 1000) {
    const [one, two] = [rows[0], rows[1]].map((r) => r.querySelector('a').getBoundingClientRect());
    if (Math.abs(one.top - two.top) > 1) out.push('the first two frames not side by side');
    const words = rows[0].querySelector('.text').getBoundingClientRect();
    if (words.left < one.right || words.top >= one.bottom) out.push('row 0: the words not beside the frame');
  }
  return out.join('\n');
})()`

// faceColours are the colours of the first link's words, destination, title, ↗ and frame.
const faceColours = `(() => {
  const row = document.querySelector('[data-floe-part=links] > li'), a = row.querySelector('a');
  return ['.text', '.to', '.title', '.out'].map((sel) => getComputedStyle(row.querySelector(sel)).color)
    .concat(getComputedStyle(a).borderTopColor).join(', ');
})()`

// TestRenderedLinksPart draws a frame to each link in every theme, side by side and wrapping, each within the list
// however narrow the page: its destination, its title underlined, ↗ out of floe, and no ground or shadow to make it a
// button. On a face a template paints, and in the band of a template (bandPages) in every theme, the frame and all it
// says are drawn in the face's text colour, which the frame takes from the part's root.
func TestRenderedLinksPart(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	entries, err := os.ReadDir("../kit/floe/themes")
	if err != nil {
		t.Fatal(err)
	}
	file := s.write(t, "links.yaml", linksPage)
	defer s.b.unsize()
	// A template of one's own holds them in a face it paints, where the panel of a theme of one's own, of no ground,
	// keeps the face's text colour.
	s.own(t, map[string]string{
		"themes/bare/panel.css":       ":scope { background: none; color: inherit; --face: inherit; --on-face: inherit; } :scope > * { --face: inherit; --on-face: inherit; }\n",
		"templates/face/page.html":    "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><meta charset=\"utf-8\"><title>{{.Title}}</title></head>\n<body><div class=\"face\">{{.Data.links}}</div></body></html>\n",
		"templates/face/page.css":     ".face { --face: #123a5c; --on-face: #fff; background: var(--face); color: var(--on-face); }\n",
		"templates/face/schema.json":  `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "properties": {"links": {"$ref": "floe:///parts/links.json"}}}`,
		"templates/face/example.yaml": "lang: en\ntemplate: face\n",
	})
	s.floe(t, "publish", s.write(t, "face.yaml", "lang: en\ntemplate: face\ntheme: bare\ntitle: Face\nlinks:\n  - { title: Guide, url: 'https://example.com/guide', text: How it works }\n"), "--slug", "links-face")
	s.b.navigate("/a/links-face/?raw")
	if got, want := s.b.eval(faceColours), strings.TrimSuffix(strings.Repeat("rgb(255, 255, 255), ", 5), ", "); got != want {
		t.Errorf("words, destination, title, ↗ and frame on a painted face in a theme of no ground = %s, want %s", got, want)
	}
	bandPages(t, s, func(what string) {
		band := s.b.eval(`getComputedStyle(document.querySelector('[data-floe-part=links]')).color`)
		if got, want := s.b.eval(faceColours), strings.TrimSuffix(strings.Repeat(band+", ", 5), ", "); got != want {
			t.Errorf("%s: words, destination, title, ↗ and frame = %s, want the band's %s", what, got, want)
		}
	})
	for _, theme := range entries {
		slug := "links-" + theme.Name()
		s.floe(t, "publish", file, "--slug", slug, "--theme", theme.Name())
		for _, w := range []int64{1000, 390, 256} {
			s.b.size(w, 800, false)
			s.b.navigate("/a/" + slug + "/?raw")
			if got := s.b.eval(measureLinks); got != "" {
				t.Errorf("%s at %d px:\n%s", theme.Name(), w, got)
			}
		}
		drawn := s.b.eval(`[...document.querySelectorAll('[data-floe-part=links] .to')].map((to) => to.textContent).join(' ')`)
		if want := strings.Join(linksTo, " "); drawn != want {
			t.Errorf("%s: where the links go = %s, want %s", theme.Name(), drawn, want)
		}
	}
}
