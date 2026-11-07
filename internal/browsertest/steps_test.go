package browsertest_test

import (
	"strings"
	"testing"

	"github.com/sukekyo26/floe/internal/kit"
)

// stepsTagsPage holds steps in each state with tags of every kind (a link, words, code, a pill of no tone and one of
// a tone), a step without tags, a step of tags too long for a narrow page, entries with the same tags to match, and a
// table whose cell's code the tags' code is as large as.
var stepsTagsPage = `template: report
lang: en
title: Steps tags
sections:
  - heading: Steps
    parts:
      - steps:
          - { title: Check the input, state: done, tags: ['[PR #428](https://example.com/428)', words, '` + "`just ci`" + `', { pill: plain }, { pill: merged, tone: good }], text: Both sides }
          - { title: Fix the words, state: doing, tags: ['[PR #431](https://example.com/431)', words, '` + "`just ci`" + `', { pill: plain }], text: In two languages }
          - { title: Remove the old screen, tags: ['[PR #433](https://example.com/433)', words, '` + "`just ci`" + `', { pill: plain }], text: Its links too }
          - { title: No tags, text: As it was }
          - { title: Long tags, tags: ['https://example.com/` + strings.Repeat("unbroken", 12) + `', { pill: ` + strings.TrimSpace(strings.Repeat("a long pill ", 10)) + `, tone: warn }] }
  - heading: Entries
    parts:
      - entries:
          - { title: Same tags, tags: ['[PR #428](https://example.com/428)', words, '` + "`just ci`" + `', { pill: plain }] }
      - table:
          columns: [Run]
          rows: [['` + "`just ci`" + `']]
`

// measureStepsTags returns a line for each thing the steps' tags draw wrong: tags not in the text's column, past the
// row or the page, not between the title and the text (which is in the size of an aside), of a look other than entries' tags, code in them of another size
// than in a table's cell, of colours that change with the step's state, a pill of no tone in another colour than
// outside, or a row of tags on a step without them.
const measureStepsTags = `(() => {
  const out = [], steps = [...document.querySelectorAll('[data-floe-part=steps] > li')];
  const entries = document.querySelector('[data-floe-part=entries] [data-floe-look~=tags]');
  if (!entries) return 'entries draw no tags';
  if (document.documentElement.scrollWidth > document.documentElement.clientWidth) out.push('the page scrolls sideways');
  const look = (e) => { const s = getComputedStyle(e); return [s.display, s.flexWrap, s.alignItems, s.rowGap, s.columnGap, s.fontSize, s.color].join(' '); };
  const colour = (e) => e ? getComputedStyle(e).color : 'none';
  const kinds = (tags) => [tags.querySelector('a'), [...tags.children].find((s) => !s.firstElementChild), tags.querySelector('code'), tags.querySelector('[data-floe-part=cell]:not([data-floe-tone])')].map(colour).join(', ');
  const want = { look: look(entries), code: getComputedStyle(document.querySelector('[data-floe-part=table] td code')).fontSize, kinds: kinds(entries) };
  const own = getComputedStyle(entries.querySelector('code')).fontSize;
  if (own !== want.code) out.push('entries: code ' + own + ', a table cell\'s ' + want.code);
  const probe = document.createElement('span');
  probe.style.fontSize = 'var(--floe-text-aside)';
  document.querySelector('[data-floe-part=steps]').append(probe);
  const aside = getComputedStyle(probe).fontSize;
  probe.remove();
  steps.forEach((li, i) => {
    const words = li.querySelector(':scope > [data-floe-text]');
    if (words && getComputedStyle(words).fontSize !== aside) out.push('step ' + i + ': its text ' + getComputedStyle(words).fontSize + ', not ' + aside);
    const tags = li.querySelector(':scope > [data-floe-look~=tags]');
    if (li.querySelector('strong').textContent === 'No tags') {
      if (li.querySelector('[data-floe-look~=tags], .tags')) out.push('step ' + i + ': tags it was not given');
      return;
    }
    if (!tags) { out.push('step ' + i + ': no tags'); return; }
    const r = li.getBoundingClientRect(), t = tags.getBoundingClientRect(), mark = getComputedStyle(li, '::before');
    const text = r.left + parseFloat(mark.width) + parseFloat(mark.marginRight);
    if (Math.abs(t.left - text) > 1) out.push('step ' + i + ': tags at ' + t.left + ', the text at ' + text);
    if (t.right > r.right + 1) out.push('step ' + i + ': tags past the row');
    const title = li.querySelector('strong').getBoundingClientRect();
    if (t.top < title.bottom - 1) out.push('step ' + i + ': tags beside the title, not under it');
    const next = tags.nextElementSibling;
    if (next && next.getBoundingClientRect().top < t.bottom - 1) out.push('step ' + i + ': tags under the text');
    if (look(tags) !== want.look) out.push('step ' + i + ': ' + look(tags) + ', entries\' tags ' + want.look);
    const code = tags.querySelector('code');
    if (code && getComputedStyle(code).fontSize !== want.code) out.push('step ' + i + ': code ' + getComputedStyle(code).fontSize + ', a table cell\'s ' + want.code);
    if (code && kinds(tags) !== want.kinds) out.push('step ' + i + ' (' + li.className + '): link, words, code, pill ' + kinds(tags) + ', entries\' ' + want.kinds);
  });
  return out.join('\n');
})()`

// tagsKit is a part of one's own whose box takes the look tags (looks: [tags]), its values its box's children, and a
// template that holds it.
var tagsKit = map[string]string{
	"templates/labelled/page.html": "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><meta charset=\"utf-8\">" +
		"<title>{{.Title}}</title></head>\n<body>\n{{.Data.labels}}\n</body></html>\n",
	"templates/labelled/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"labels": {"$ref": "floe:///parts/labels.json"}}}`,
	"templates/labelled/example.yaml": "lang: en\ntemplate: labelled\n",
	"parts/labels/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "array",
"items": {"$ref": "floe:///parts/cell.json"}}`,
	"parts/labels/part.html": `<div data-floe-part="labels">{{range .}}<span>{{.}}</span>{{end}}</div>`,
	"parts/labels/part.yaml": "looks: [tags]\n",
	"parts/labels/part.css":  ":scope { min-width: 0; }\n",
}

// TestUIStepsTags draws the steps' tags in every theme, on a wide page and a narrow one: under the title in the text's
// column clear of the ring and its line, within the row, drawn as entries' tags are and in the same colours whatever
// the step's state; and a long pill in a part that takes the look by looks: wrapped within the page.
func TestUIStepsTags(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	s.own(t, tagsKit)
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	file := s.write(t, "steps.yaml", stepsTagsPage)
	defer s.b.unsize()
	for _, theme := range themes {
		slug := "steps-tags-" + theme
		s.floe(t, "publish", file, "--slug", slug, "--theme", theme)
		for _, w := range []int64{1440, 320} {
			s.b.size(w, 900, false)
			s.b.navigate("/a/" + slug + "/?raw")
			if got := s.b.eval(measureStepsTags); got != "" {
				t.Errorf("%s at %d px:\n%s", theme, w, got)
			}
		}
	}
	s.floe(t, "publish", s.write(t, "labels.yaml", "lang: en\ntemplate: labelled\ntitle: Labels\nlabels: [a label, { pill: "+
		strings.TrimSpace(strings.Repeat("a long pill ", 12))+", tone: warn }]\n"), "--slug", "labels")
	s.b.size(320, 900, false)
	s.b.navigate("/a/labels/?raw")
	wide := `(() => {
  const pill = document.querySelector('[data-floe-part=labels] [data-floe-part=cell]').getBoundingClientRect();
  return document.documentElement.scrollWidth > document.documentElement.clientWidth || pill.right > document.documentElement.clientWidth ?
    'the long pill past the page, its right at ' + pill.right : '';
})()`
	if got := s.b.eval(wide); got != "" {
		t.Errorf("a part taking tags by looks: at 320 px: %s", got)
	}
}
