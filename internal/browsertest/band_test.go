package browsertest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bandKit is a template that places a note of floe's and Markdown with a fenced block in a band.
var bandKit = map[string]string{
	"templates/mine/page.html": "<!doctype html>\n<html lang=\"{{.Lang}}\"><head><meta charset=\"utf-8\">" +
		"<title>{{.Title}}</title></head>\n<body>\n<header data-floe-look=\"band\">{{.Data.note}}{{.Data.text}}</header>\n</body></html>\n",
	"templates/mine/schema.json": `{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object",
"properties": {"note": {"$ref": "floe:///parts/note.json"}, "text": {"type": "string", "format": "markdown"}}}`,
	"templates/mine/example.yaml": "lang: en\ntemplate: mine\n",
}

const bandPage = "lang: en\ntemplate: mine\ntitle: Band\nnote: { text: a note }\ntext: |\n  `inline`\n\n  ```\n  fenced\n  ```\n"

// measureBand returns a line for each thing the band draws wrong: a part in it not in the page's text colour, a
// line's code without its frame, or code in a fenced block with one.
const measureBand = `(() => {
  const out = [];
  const probe = document.body.appendChild(document.createElement('p'));
  const want = getComputedStyle(probe).color;
  probe.remove();
  const note = document.querySelector('[data-floe-part="note"]');
  if (getComputedStyle(note).color !== want) out.push('note: colour ' + getComputedStyle(note).color + ', want ' + want);
  const line = document.querySelector('header :not(pre) > code');
  if (getComputedStyle(line).borderTopWidth !== '1px') out.push('a line\'s code: border ' + getComputedStyle(line).borderTopWidth);
  const fenced = document.querySelector('header pre > code');
  if (getComputedStyle(fenced).borderTopWidth !== '0px') out.push('fenced code: border ' + getComputedStyle(fenced).borderTopWidth);
  return out.join('\n');
})()`

// TestUIBand draws a note and a fenced block in the look band: the note is in the page's text colour, not the
// band's, and only a line's code is framed.
func TestUIBand(t *testing.T) { //nolint:paralleltest // the browser tests share one Chrome, one at a time
	s := newSite(t)
	for name, text := range bandKit {
		at := filepath.Join(s.Dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(at), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	s.floe(t, "publish", s.write(t, "band.yaml", bandPage), "--slug", "band")
	s.b.navigate("/a/band/?raw")
	s.b.until("the note", "document.querySelector('[data-floe-part=\"note\"]') !== null")
	if got := s.b.eval(measureBand); got != "" {
		t.Errorf("in the band:\n  %s", strings.ReplaceAll(got, "\n", "\n  "))
	}
}
