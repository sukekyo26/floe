package browsertest_test

import (
	"os"
	"strings"
	"testing"
)

// linksURLs are the URLs linksPage writes, as its rows draw them: html/template writes the href anew, in %xx.
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

// measureLinks returns a line for each thing the rows draw wrong: a row not across the box, what it holds past it, a
// URL not whole, not under the link or not to select, a first row taller than its two lines, words off the title's
// line or left of it, or a link's reach past its words.
const measureLinks = `(() => {
  const out = [], list = document.querySelector('[data-floe-part=links]'), rows = [...list.children];
  const box = list.getBoundingClientRect();
  if (box.right > document.documentElement.clientWidth + 1) out.push('the list reaches past the page');
  rows.forEach((row, i) => {
    const r = row.getBoundingClientRect(), a = row.querySelector('a').getBoundingClientRect(), text = row.querySelector('.text');
    if (r.width < box.width * 0.9) out.push('row ' + i + ': ' + r.width + ' wide in ' + box.width);
    const reach = (e) => { const range = document.createRange(); range.selectNodeContents(e); return range.getBoundingClientRect().right; };
    if (reach(row.querySelector('a')) > r.right + 1) out.push('row ' + i + ': the link past the row');
    const url = row.querySelector('.url'), u = url.getBoundingClientRect();
    if (reach(url) > r.right + 1 || url.scrollWidth > url.clientWidth + 1) out.push('row ' + i + ': the URL cut or past the row');
    if (u.top < a.bottom - 1 || Math.abs(u.left - a.left) > 1) out.push('row ' + i + ': the URL not under the link');
    if (getComputedStyle(url).userSelect === 'none') out.push('row ' + i + ': the URL cannot be selected');
    const link = row.querySelector('a');
    link.scrollIntoView({ block: 'center' });
    const words = document.createRange();
    words.selectNodeContents(link);
    const parts = words.getClientRects(), last = parts[parts.length - 1], end = row.getBoundingClientRect().right;
    if (parts.length > 1 && last.right < end - 8) {
      const past = document.elementFromPoint(last.right + 4, last.top + last.height / 2);
      if (past && past.closest('a')) out.push('row ' + i + ': the blank after a folded title follows its link');
    }
    const sign = getComputedStyle(row, '::before');
    if (parseFloat(sign.maskSize) > parseFloat(sign.height) + 0.5) out.push('row ' + i + ': the sign ' + sign.maskSize + ' in a box ' + sign.height + ' high');
    if (sign.content === 'none' || sign.backgroundColor === 'rgba(0, 0, 0, 0)') out.push('row ' + i + ': no sign');
    if (!getComputedStyle(row.querySelector('a')).textDecorationLine.includes('underline')) out.push('row ' + i + ': the link not underlined');
    const t = text && text.getBoundingClientRect();
    if (text && reach(text) > r.right + 1) out.push('row ' + i + ': the words past the row');
    if (t && t.top >= a.bottom - 1 && Math.abs(t.left - a.left) > 1) out.push('row ' + i + ': words under the link at ' + t.left + ', the link at ' + a.left);
  });
  scrollTo(0, 0);
  const line = (i) => [rows[i].querySelector('a').getBoundingClientRect(), rows[i].querySelector('.text').getBoundingClientRect()];
  const [first, words] = line(0), [, long] = line(4), [longLink] = line(4);
  if (long.top < longLink.bottom - 1) out.push('row 4: the long words beside the link');
  if (innerWidth >= 390) {
    if (words.top >= first.bottom - 1) out.push('row 0: the words under the link');
    if (words.left < first.right) out.push('row 0: the words over the link');
    const pad = getComputedStyle(rows[0]);
    const height = rows[0].getBoundingClientRect().height - parseFloat(pad.paddingTop) - parseFloat(pad.paddingBottom);
    if (height > 4 * parseFloat(getComputedStyle(document.documentElement).fontSize)) out.push('row 0: ' + height + ' tall');
  }
  const row = rows[0].getBoundingClientRect();
  const blank = document.elementFromPoint(row.right - 4, row.top + row.height / 2);
  if (blank && blank.closest('a')) out.push('the blank of a row follows its link');
  return out.join('\n');
})()`

// TestUILinksPart draws a row to each link in every theme: across the box, holding what cannot fold, the words on the
// title's line when they fit and under it when not, the URL whole under them, and the link's reach its own words.
func TestUILinksPart(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	entries, err := os.ReadDir("../kit/floe/themes")
	if err != nil {
		t.Fatal(err)
	}
	file := s.write(t, "links.yaml", linksPage)
	defer s.b.unsize()
	// A theme of one's own draws the rows in lines lower than the sign's 1rem.
	s.own(t, map[string]string{"themes/low/links.css": ":scope > li { font-size: 0.8rem; line-height: 1.1; }\n"})
	s.floe(t, "publish", file, "--slug", "links-low", "--theme", "low")
	s.b.navigate("/a/links-low/?raw")
	if got := s.b.eval(measureLinks); got != "" {
		t.Errorf("low lines:\n%s", got)
	}
	for _, theme := range entries {
		slug := "links-" + theme.Name()
		s.floe(t, "publish", file, "--slug", slug, "--theme", theme.Name())
		for _, w := range []int64{1000, 390, 256} {
			s.b.size(w, 800, false)
			s.b.navigate("/a/" + slug + "/?raw")
			if got := s.b.eval(measureLinks); got != "" {
				t.Errorf("%s at %d px:\n%s", theme.Name(), w, got)
			}
			urls := s.b.eval(`[...document.querySelectorAll('[data-floe-part=links] .url')].map((u) => u.textContent).join('\n')`)
			if want := strings.Join(linksURLs, "\n"); urls != want {
				t.Errorf("%s at %d px: the URLs drawn:\n%s\nwant:\n%s", theme.Name(), w, urls, want)
			}
		}
	}
}
