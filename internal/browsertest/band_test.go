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

// bandKit is a template that places, in a band, pills (cell) in each tone and in none, and a note, one it places
// itself holding a link and code, a table and code of floe's and Markdown, each when the data file has it.
var bandKit = map[string]string{
	"templates/mine/page.html": "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><meta charset=\"utf-8\">" +
		"<title>{{.Title}}</title></head>\n<body>\n<header data-floe-look=\"band\"><p>" + bandPillsHTML + "</p>{{if .Data.note}}<div data-floe-part=\"note\"><p>A borrowed note, <a href=\"https://example.com/\">a link</a> and <code>code</code>.</p></div>{{end}}{{.Data.note}}{{.Data.text}}{{.Data.table}}{{.Data.code}}</header>\n</body></html>\n",
	"templates/mine/page.css": "header { display: grid; gap: 1rem; padding: 2rem; }\n",
	"templates/mine/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"note": {"$ref": "floe:///parts/note.json"}, "text": {"type": "string", "format": "markdown"},
"table": {"$ref": "floe:///parts/table.json"}, "code": {"$ref": "floe:///parts/code.json"}}}`,
	"templates/mine/example.yaml": "lang: en\ntemplate: mine\n",
}

// bandText is Markdown in a band: a link, a line's code, a quote holding both, and a fenced block wide enough to scroll.
var bandText = "lang: en\ntemplate: mine\ntitle: Band\ntext: |\n  A [link](https://example.com/) and `inline`.\n\n" +
	"  > A quote with [a link](https://example.com/) and `code`.\n\n  ```\n  fenced" + strings.Repeat(" wide", 60) + "\n  ```\n"

// bandParts is parts in a band: a note with a link and a line's code, a table with a link in its heading and as wide
// as to scroll, and code as wide as to scroll.
var bandParts = bandText +
	"note: { text: \"A note with [a link](https://example.com/) and `code`.\" }\n" +
	"table:\n  columns: [{ title: \"Name [t](https://example.com/)\" }" + strings.Repeat(", { title: Wide }", 16) + "]\n" +
	"  rows: [[a" + strings.Repeat(", b", 16) + "]]\n" +
	"code: { file: a.conf, text: \"wide" + strings.Repeat(" wide", 60) + "\\n\" }\n"

// measureBand returns a line for each thing the band draws wrong: a part in it not in the page's text colour, a
// line's code without its frame (a shadow every code has, seen only in a face), or code in a fenced block with one.
const measureBand = `(() => {` + colours + `
  const out = [];
  const framed = (e) => { const c = getComputedStyle(e).boxShadow.match(/^(rgba?\([^)]*\)|color\([^)]*\))/); return c !== null && rgba(c[1])[3] > 0; };
  const probe = document.body.appendChild(document.createElement('p'));
  const want = getComputedStyle(probe).color;
  probe.remove();
  const note = document.querySelector('[data-floe-part="note"]');
  if (note && getComputedStyle(note).color !== want) out.push('note: colour ' + getComputedStyle(note).color + ', want ' + want);
  const line = document.querySelector('header > p > code');
  if (!framed(line)) out.push('a line\'s code: no frame, ' + getComputedStyle(line).boxShadow);
  const fenced = document.querySelector('header > pre > code');
  if (framed(fenced)) out.push('fenced code: frame ' + getComputedStyle(fenced).boxShadow);
  return out.join('\n');
})()`

// TestUIBand draws Markdown and pills a template places in the look band in every theme, and other parts in it in the default one: the
// parts are in the page's text colour, not the band's, only a line's code is framed, and every text and focus ring in
// the band reads on the ground under it, as measureContrast and measureRings measure, pills of a theme that gives
// them no ground too, in the band's text colour. The other parts are in the default theme, as a part with no ground
// of its own (swiss's note) is drawn on the band's colour, which band's LIMIT tells, and in aurora, whose glass would
// let the band through but for its band.css.
func TestUIBand(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	s.own(t, bandKit)
	k, err := kit.Floe()
	if err != nil {
		t.Fatal(err)
	}
	_, _, themes, _ := k.Names()
	pages := map[string]string{"parts": bandParts, "parts-aurora": bandParts + "theme: aurora\n"}
	for _, theme := range themes {
		pages["text-"+theme] = bandText + "theme: " + theme + "\n"
	}
	for slug, page := range pages {
		s.floe(t, "publish", s.write(t, slug+".yaml", page), "--slug", slug)
		s.b.navigate("/a/" + slug + "/?raw")
		s.b.until("the band", "document.querySelector('header > pre') !== null")
		if got := s.b.eval(measureBand); got != "" {
			t.Errorf("in the band of %s:\n  %s", slug, strings.ReplaceAll(got, "\n", "\n  "))
		}
		s.b.measureColours(t, "the band of "+slug)
	}
}
